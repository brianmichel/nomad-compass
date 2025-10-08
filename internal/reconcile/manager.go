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
	nomad    nomadclient.JobRegistrar
	interval time.Duration
	logger   *slog.Logger
}

// New constructs a reconciliation manager.
func New(repos *storage.RepoStore, files *storage.RepoFileStore, creds *storage.CredentialStore, git *repo.Manager, nomad nomadclient.JobRegistrar, interval time.Duration, logger *slog.Logger) *Manager {
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

	if repoRecord.LastCommit.Valid && repoRecord.LastCommit.String == snapshot.CommitHash {
		m.logger.Info("repo already reconciled", "repo", repoRecord.Name, "commit", snapshot.CommitHash)
		return m.repos.UpdatePollTimestamp(ctx, repoRecord.ID)
	}

	for _, jobFile := range snapshot.JobFiles {
		if err := m.applyJob(ctx, repoRecord, jobFile, snapshot); err != nil {
			m.logger.Error("job apply failed", "repo", repoRecord.Name, "file", jobFile.Path, "error", err)
		} else {
			_ = m.files.Upsert(ctx, repoRecord.ID, jobFile.Path, snapshot.CommitHash)
		}
	}

	if err := m.repos.UpdateCommitMetadata(ctx, repoRecord.ID, snapshot.CommitHash, snapshot.CommitAuthor, snapshot.CommitTitle); err != nil {
		return err
	}

	m.logger.Info("repo reconciled", "repo", repoRecord.Name, "commit", snapshot.CommitHash)
	return nil
}

func (m *Manager) applyJob(ctx context.Context, repoRecord *storage.Repository, jobFile repo.JobFile, snapshot *repo.Snapshot) error {
	job, err := parseJob(jobFile.Path, jobFile.Content)
	if err != nil {
		return err
	}

	job.Meta["nomad-compass/repo-url"] = repoRecord.RepoURL
	job.Meta["nomad-compass/repo-name"] = repoRecord.Name
	job.Meta["nomad-compass/job-file"] = jobFile.Path
	job.Meta["nomad-compass/commit"] = snapshot.CommitHash
	job.Meta["nomad-compass/commit-author"] = snapshot.CommitAuthor
	job.Meta["nomad-compass/commit-title"] = snapshot.CommitTitle

	return m.nomad.RegisterJob(ctx, job)
}

func parseJob(path string, contents []byte) (*api.Job, error) {
	cfg := &jobspec2.ParseConfig{Body: contents, Path: path, Strict: true}
	job, err := jobspec2.ParseWithConfig(cfg)
	if err != nil {
		return nil, err
	}
	if job.Meta == nil {
		job.Meta = map[string]string{}
	}
	return job, nil
}
