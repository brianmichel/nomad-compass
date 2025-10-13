package reconcile

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec2"

	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

// Manager coordinates reconciliation cycles for onboarded repositories.
type Manager struct {
	repos    *storage.RepoStore
	files    *storage.RepoFileStore
	creds    *storage.CredentialStore
	git      *repo.Manager
	nomad    nomadclient.Client
	interval time.Duration
	logger   *slog.Logger
}

// New constructs a reconciliation manager.
func New(repos *storage.RepoStore, files *storage.RepoFileStore, creds *storage.CredentialStore, git *repo.Manager, nomad nomadclient.Client, interval time.Duration, logger *slog.Logger) *Manager {
	return &Manager{repos: repos, files: files, creds: creds, git: git, nomad: nomad, interval: interval, logger: logger}
}

// Run executes reconciliation loops until the context is cancelled.
func (m *Manager) Run(ctx context.Context) error {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.logger.Info("reconciler started", "interval", m.interval)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := m.reconcileAll(ctx); err != nil {
				m.logger.Error("reconciliation cycle failed", "error", err)
			}
		}
	}
}

// RunOnce executes a single reconciliation cycle. Useful for manual triggers and tests.
func (m *Manager) RunOnce(ctx context.Context) error {
	return m.reconcileAll(ctx)
}

// ReconcileRepo triggers reconciliation for a single repository.
func (m *Manager) ReconcileRepo(ctx context.Context, repoID int64) error {
	repo, err := m.repos.Get(ctx, repoID)
	if err != nil {
		return err
	}
	if repo == nil {
		return errors.New("repository not found")
	}
	return m.reconcileRepo(ctx, repo)
}

func (m *Manager) reconcileAll(ctx context.Context) error {
	repos, err := m.repos.List(ctx)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		if err := m.reconcileRepo(ctx, &repo); err != nil {
			m.logger.Error("repo reconciliation failed", "repo", repo.Name, "error", err)
		}
	}
	return nil
}

func (m *Manager) reconcileRepo(ctx context.Context, repoRecord *storage.Repository) error {
	var cred *storage.Credential
	var payload *storage.CredentialPayload
	if repoRecord.CredentialID.Valid {
		var err error
		cred, err = m.creds.Get(ctx, repoRecord.CredentialID.Int64)
		if err != nil {
			return err
		}
		if cred == nil {
			return errors.New("linked credential not found")
		}
		payload, err = m.creds.DecryptPayload(cred)
		if err != nil {
			return err
		}
	}

	snapshot, err := m.git.Sync(ctx, *repoRecord, cred, payload)
	if err != nil {
		// Partial failures should still record the poll event
		_ = m.repos.UpdatePollTimestamp(ctx, repoRecord.ID)
		return err
	}

	commitChanged := !repoRecord.LastCommit.Valid || repoRecord.LastCommit.String != snapshot.CommitHash
	if err := m.ensureJobs(ctx, repoRecord, snapshot, commitChanged); err != nil {
		return err
	}

	if commitChanged {
		if err := m.repos.UpdateCommitMetadata(ctx, repoRecord.ID, snapshot.CommitHash, snapshot.CommitAuthor, snapshot.CommitTitle); err != nil {
			return err
		}
		m.logger.Info("repo reconciled", "repo", repoRecord.Name, "commit", snapshot.CommitHash)
	} else {
		if err := m.repos.UpdatePollTimestamp(ctx, repoRecord.ID); err != nil {
			return err
		}
		m.logger.Info("repo state enforced", "repo", repoRecord.Name, "commit", snapshot.CommitHash)
	}

	return nil
}

func (m *Manager) applyJob(ctx context.Context, repoRecord *storage.Repository, jobFile repo.JobFile, snapshot *repo.Snapshot) (string, error) {
	job, submission, err := parseJob(jobFile.Path, jobFile.Content)
	if err != nil {
		return "", err
	}

	job.Meta["nomad-compass/repo-url"] = repoRecord.RepoURL
	job.Meta["nomad-compass/repo-name"] = repoRecord.Name
	job.Meta["nomad-compass/job-file"] = jobFile.Path
	job.Meta["nomad-compass/commit"] = snapshot.CommitHash
	job.Meta["nomad-compass/commit-author"] = snapshot.CommitAuthor
	job.Meta["nomad-compass/commit-title"] = snapshot.CommitTitle

	if err := m.nomad.RegisterJob(ctx, job, submission); err != nil {
		return "", err
	}
	return jobID(job), nil
}

// DeleteRepository removes repository metadata and optionally unschedules jobs.
func (m *Manager) DeleteRepository(ctx context.Context, repoID int64, unschedule bool) error {
	repoRecord, err := m.repos.Get(ctx, repoID)
	if err != nil {
		return err
	}
	if repoRecord == nil {
		return errors.New("repository not found")
	}

	if unschedule {
		if err := m.unscheduleJobs(ctx, repoRecord.ID); err != nil {
			return err
		}
	}

	if err := m.files.DeleteByRepo(ctx, repoRecord.ID); err != nil {
		return err
	}
	if err := m.repos.Delete(ctx, repoRecord.ID); err != nil {
		return err
	}
	if err := m.git.RemoveRepo(repoRecord.ID); err != nil {
		return err
	}
	return nil
}

// DeleteCredential removes a credential and optionally deletes & unschedules repos that depend on it.
func (m *Manager) DeleteCredential(ctx context.Context, credentialID int64, deleteRepos bool, unschedule bool) error {
	credential, err := m.creds.Get(ctx, credentialID)
	if err != nil {
		return err
	}
	if credential == nil {
		return errors.New("credential not found")
	}

	repos, err := m.repos.ListByCredential(ctx, credentialID)
	if err != nil {
		return err
	}

	if deleteRepos {
		for _, repo := range repos {
			if err := m.DeleteRepository(ctx, repo.ID, unschedule); err != nil {
				return err
			}
		}
	} else if len(repos) > 0 {
		if err := m.repos.ClearCredential(ctx, credentialID); err != nil {
			return err
		}
	}

	if err := m.creds.Delete(ctx, credentialID); err != nil {
		return err
	}

	return nil
}

func (m *Manager) unscheduleJobs(ctx context.Context, repoID int64) error {
	files, err := m.files.ListByRepo(ctx, repoID)
	if err != nil {
		return err
	}
	for _, file := range files {
		if !file.JobID.Valid || file.JobID.String == "" {
			continue
		}
		if err := m.nomad.DeregisterJob(ctx, file.JobID.String, true); err != nil {
			return err
		}
	}
	return nil
}

func jobID(job *api.Job) string {
	if job == nil {
		return ""
	}
	if job.ID != nil && *job.ID != "" {
		return *job.ID
	}
	if job.Name != nil {
		return *job.Name
	}
	return ""
}

func parseJob(path string, contents []byte) (*api.Job, *api.JobSubmission, error) {
	cfg := &jobspec2.ParseConfig{Body: contents, Path: path, Strict: true}
	job, err := jobspec2.ParseWithConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	if job.Meta == nil {
		job.Meta = map[string]string{}
	}
	submission := &api.JobSubmission{
		Source: string(contents),
		Format: "hcl2",
	}
	return job, submission, nil
}

func (m *Manager) ensureJobs(ctx context.Context, repoRecord *storage.Repository, snapshot *repo.Snapshot, commitChanged bool) error {
	repoFiles, err := m.files.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		return err
	}

	fileIndex := make(map[string]storage.RepoFile, len(repoFiles))
	for _, file := range repoFiles {
		fileIndex[file.Path] = file
	}

	seen := make(map[string]struct{}, len(snapshot.JobFiles))

	for _, jobFile := range snapshot.JobFiles {
		seen[jobFile.Path] = struct{}{}
		existing, tracked := fileIndex[jobFile.Path]
		needApply := commitChanged || !tracked

		var trackedJobID string
		if tracked && existing.JobID.Valid {
			trackedJobID = existing.JobID.String
		}

		if !needApply {
			if trackedJobID == "" {
				needApply = true
			} else {
				status, err := m.nomad.JobStatus(ctx, trackedJobID)
				if err != nil {
					m.logger.Warn("job status check failed", "repo", repoRecord.Name, "job_id", trackedJobID, "file", jobFile.Path, "error", err)
					continue
				}
				if status == nil || !status.Exists {
					needApply = true
				}
			}
		}

		if !needApply {
			continue
		}

		jobID, err := m.applyJob(ctx, repoRecord, jobFile, snapshot)
		if err != nil {
			m.logger.Error("job apply failed", "repo", repoRecord.Name, "file", jobFile.Path, "error", err)
			continue
		}
		if err := m.files.Upsert(ctx, repoRecord.ID, jobFile.Path, snapshot.CommitHash, jobID); err != nil {
			return err
		}
	}

	for path, file := range fileIndex {
		if _, ok := seen[path]; ok {
			continue
		}
		// Job file no longer exists in the repo. Unschedule and drop tracking metadata.
		if file.JobID.Valid && file.JobID.String != "" {
			if err := m.nomad.DeregisterJob(ctx, file.JobID.String, true); err != nil {
				if m.logger != nil {
					m.logger.Error("job deregister failed", "repo", repoRecord.Name, "job_id", file.JobID.String, "file", path, "error", err)
				}
				continue
			}
		}
		if err := m.files.Delete(ctx, repoRecord.ID, path); err != nil {
			return err
		}
		if m.logger != nil {
			m.logger.Info("job removed", "repo", repoRecord.Name, "file", path, "job_id", file.JobID.String)
		}
	}

	return nil
}
