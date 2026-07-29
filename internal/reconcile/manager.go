package reconcile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec2"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

const (
	compassMetaRepoURL      = "nomad-compass/repo-url"
	compassMetaRepoName     = "nomad-compass/repo-name"
	compassMetaJobFile      = "nomad-compass/job-file"
	compassMetaCommit       = "nomad-compass/commit"
	compassMetaCommitAuthor = "nomad-compass/commit-author"
	compassMetaCommitTitle  = "nomad-compass/commit-title"
)

// Manager coordinates reconciliation cycles for onboarded repositories.
type Manager struct {
	repos    *storage.RepoStore
	files    *storage.RepoFileStore
	managed  *storage.ManagedResourceStore
	creds    *storage.CredentialStore
	git      *repo.Manager
	nomad    nomadclient.Client
	interval time.Duration
	logger   *slog.Logger
}

// New constructs a reconciliation manager.
func New(repos *storage.RepoStore, files *storage.RepoFileStore, managed *storage.ManagedResourceStore, creds *storage.CredentialStore, git *repo.Manager, nomad nomadclient.Client, interval time.Duration, logger *slog.Logger) *Manager {
	return &Manager{repos: repos, files: files, managed: managed, creds: creds, git: git, nomad: nomad, interval: interval, logger: logger}
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
	if snapshot.Bundle != nil {
		if err := m.ensureBundle(ctx, repoRecord, snapshot, commitChanged); err != nil {
			return err
		}
	} else if err := m.ensureJobs(ctx, repoRecord, snapshot, commitChanged); err != nil {
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

func (m *Manager) applyJob(ctx context.Context, repoRecord *storage.Repository, jobFile repo.JobFile, snapshot *repo.Snapshot, job *api.Job, submission *api.JobSubmission) (string, error) {
	if job == nil || submission == nil {
		return "", errors.New("job and submission are required")
	}

	annotateJob(job, repoRecord, jobFile, snapshot, true)

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
		if err := m.unscheduleManagedResources(ctx, repoRecord.ID); err != nil {
			return err
		}
	}

	if err := m.files.DeleteByRepo(ctx, repoRecord.ID); err != nil {
		return err
	}
	if m.managed != nil {
		if err := m.managed.DeleteByRepo(ctx, repoRecord.ID); err != nil {
			return err
		}
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

func (m *Manager) unscheduleManagedResources(ctx context.Context, repoID int64) error {
	if m.managed == nil {
		return nil
	}
	resources, err := m.managed.ListByRepo(ctx, repoID)
	if err != nil {
		return err
	}
	if len(resources) == 0 {
		return nil
	}
	client, ok := m.nomad.(nomadclient.ResourceClient)
	if !ok {
		return errors.New("Nomad client does not support bundle resource cleanup")
	}
	for _, resource := range resources {
		if resource.DeleteMode == string(manifest.DeleteModeProtect) {
			continue
		}
		if err := deleteManagedResource(ctx, client, resource); err != nil {
			return fmt.Errorf("delete managed resource %q: %w", resource.Address, err)
		}
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
	jobFiles, err := snapshotJobFiles(snapshot)
	if err != nil {
		return err
	}

	repoFiles, err := m.files.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		return err
	}

	fileIndex := make(map[string]storage.RepoFile, len(repoFiles))
	for _, file := range repoFiles {
		fileIndex[file.Path] = file
	}

	seen := make(map[string]struct{}, len(jobFiles))

	for _, jobFile := range jobFiles {
		seen[jobFile.Path] = struct{}{}
		existing, tracked := fileIndex[jobFile.Path]
		job, submission, err := parseJob(jobFile.Path, jobFile.Content)
		if err != nil {
			m.logger.Error("job parse failed", "repo", repoRecord.Name, "file", jobFile.Path, "error", err)
			continue
		}

		needApply := !tracked
		var trackedJobID string
		if tracked && existing.JobID.Valid {
			trackedJobID = existing.JobID.String
		}

		if tracked && !needApply {
			if trackedJobID == "" {
				needApply = true
			} else {
				status, err := m.nomad.JobStatus(ctx, trackedJobID)
				if err != nil {
					m.logger.Warn("job status check failed", "repo", repoRecord.Name, "job_id", trackedJobID, "file", jobFile.Path, "error", err)
					if commitChanged {
						needApply = true
					} else {
						continue
					}
				}
				if status == nil || !status.Exists {
					needApply = true
				}
			}
		}

		if tracked && !needApply {
			annotateJob(job, repoRecord, jobFile, snapshot, false)
			plan, err := m.nomad.PlanJob(ctx, job)
			if err != nil {
				m.logger.Warn("job plan failed", "repo", repoRecord.Name, "job_id", trackedJobID, "file", jobFile.Path, "error", err)
				needApply = true
			} else if !jobPlanHasChanges(plan) {
				if commitChanged {
					if err := m.files.UpsertWithDeleteMode(ctx, repoRecord.ID, jobFile.Path, snapshot.CommitHash, trackedJobID, jobFile.DeleteMode); err != nil {
						return err
					}
				}
				continue
			} else {
				needApply = true
			}
		}

		if !needApply {
			continue
		}

		jobID, err := m.applyJob(ctx, repoRecord, jobFile, snapshot, job, submission)
		if err != nil {
			m.logger.Error("job apply failed", "repo", repoRecord.Name, "file", jobFile.Path, "error", err)
			continue
		}
		if err := m.files.UpsertWithDeleteMode(ctx, repoRecord.ID, jobFile.Path, snapshot.CommitHash, jobID, jobFile.DeleteMode); err != nil {
			return err
		}
	}

	for path, file := range fileIndex {
		if _, ok := seen[path]; ok {
			continue
		}
		// Job file no longer exists in the repo. Unschedule and drop tracking metadata.
		if file.DeleteMode.Valid && file.DeleteMode.String == string(manifest.DeleteModeProtect) {
			if m.logger != nil {
				m.logger.Warn("job removal protected", "repo", repoRecord.Name, "file", path, "job_id", file.JobID.String)
			}
			continue
		}
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

func (m *Manager) ensureBundle(ctx context.Context, repoRecord *storage.Repository, snapshot *repo.Snapshot, commitChanged bool) error {
	if snapshot == nil || snapshot.Bundle == nil {
		return errors.New("bundle snapshot is required")
	}

	ordered, err := snapshot.Bundle.OrderedResources()
	if err != nil {
		return err
	}

	var resourceClient nomadclient.ResourceClient
	for _, resource := range ordered {
		if resource.Kind == "job" {
			continue
		}
		if resource.Kind != "volume" && resource.Kind != "acl_policy" {
			return fmt.Errorf("bundle resource %q is not supported yet", resource.Address)
		}
		if m.managed == nil {
			return errors.New("managed resource store is required for bundle resources")
		}
		if resourceClient == nil {
			var ok bool
			resourceClient, ok = m.nomad.(nomadclient.ResourceClient)
			if !ok {
				return errors.New("Nomad client does not support bundle resources")
			}
		}
	}

	tracked := map[string]storage.ManagedResource{}
	if m.managed != nil {
		tracked, err = m.managedResources(ctx, repoRecord.ID)
		if err != nil {
			return err
		}
		if resourceClient == nil && len(tracked) > 0 {
			var ok bool
			resourceClient, ok = m.nomad.(nomadclient.ResourceClient)
			if !ok {
				return errors.New("Nomad client does not support bundle resource cleanup")
			}
		}
	}
	for _, resource := range ordered {
		if resource.Kind == "job" {
			continue
		}
		if err := m.ensureManagedResource(ctx, repoRecord.ID, snapshot, resourceClient, resource, tracked[resource.Address]); err != nil {
			return err
		}
	}

	jobFiles, err := snapshotJobFiles(snapshot)
	if err != nil {
		return err
	}
	if len(jobFiles) > 0 {
		jobSnapshot := &repo.Snapshot{
			CommitHash:   snapshot.CommitHash,
			CommitAuthor: snapshot.CommitAuthor,
			CommitTitle:  snapshot.CommitTitle,
			JobFiles:     jobFiles,
		}
		if err := m.ensureJobs(ctx, repoRecord, jobSnapshot, commitChanged); err != nil {
			return err
		}
	}

	desired := make(map[string]struct{}, len(snapshot.Bundle.Resources))
	for _, resource := range snapshot.Bundle.Resources {
		desired[resource.Address] = struct{}{}
	}
	for _, resource := range tracked {
		if _, exists := desired[resource.Address]; exists {
			continue
		}
		if resource.DeleteMode == string(manifest.DeleteModeProtect) {
			_ = m.managed.Upsert(ctx, storage.ManagedResourceInput{
				RepoID:      repoRecord.ID,
				Address:     resource.Address,
				Kind:        resource.Kind,
				SourcePath:  resource.SourcePath,
				NomadID:     resource.NomadID.String,
				Namespace:   resource.Namespace.String,
				ContentHash: resource.ContentHash.String,
				LastCommit:  resource.LastCommit.String,
				Status:      "protected",
				LastError:   "resource removed from bundle but deletion is protected",
				DeleteMode:  resource.DeleteMode,
				Subtype:     resource.Subtype.String,
			})
			continue
		}
		if err := deleteManagedResource(ctx, resourceClient, resource); err != nil {
			return fmt.Errorf("delete bundle resource %q: %w", resource.Address, err)
		}
		if err := m.managed.Delete(ctx, repoRecord.ID, resource.Address); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) managedResources(ctx context.Context, repoID int64) (map[string]storage.ManagedResource, error) {
	resources, err := m.managed.ListByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]storage.ManagedResource, len(resources))
	for _, resource := range resources {
		result[resource.Address] = resource
	}
	return result, nil
}

func (m *Manager) ensureManagedResource(ctx context.Context, repoID int64, snapshot *repo.Snapshot, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) error {
	contentHash := sha256.Sum256(resource.Body)
	hash := hex.EncodeToString(contentHash[:])
	if tracked.Address != "" && tracked.ContentHash.Valid && tracked.ContentHash.String == hash && tracked.Status == "applied" {
		exists, err := managedResourceExists(ctx, client, resource, tracked)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
	}

	var nomadID, namespace string
	var err error
	switch resource.Kind {
	case "volume":
		spec, compileErr := manifest.CompileVolume(resource)
		if compileErr != nil {
			err = compileErr
			break
		}
		switch spec.Type {
		case "host":
			var volume *api.HostVolume
			volume, err = client.ApplyHostVolume(ctx, spec.Host)
			if volume != nil {
				nomadID, namespace = volume.ID, volume.Namespace
			}
		case "csi":
			var volume *api.CSIVolume
			volume, err = client.ApplyCSIVolume(ctx, spec.CSI)
			if volume != nil {
				nomadID, namespace = volume.ID, volume.Namespace
			}
		}
	case "acl_policy":
		var policy *api.ACLPolicy
		policy, err = manifest.CompileACLPolicy(resource)
		if err == nil {
			err = client.ApplyACLPolicy(ctx, policy)
			nomadID = resource.Name
		}
	}
	if err != nil {
		_ = m.managed.Upsert(ctx, storage.ManagedResourceInput{
			RepoID:      repoID,
			Address:     resource.Address,
			Kind:        resource.Kind,
			SourcePath:  resource.SourcePath,
			ContentHash: hash,
			LastCommit:  snapshot.CommitHash,
			Status:      "failed",
			LastError:   err.Error(),
			DeleteMode:  string(resource.DeleteMode),
		})
		return fmt.Errorf("apply bundle resource %q: %w", resource.Address, err)
	}

	resourceType := ""
	if resource.Kind == "volume" {
		spec, _ := manifest.CompileVolume(resource)
		resourceType = spec.Type
	}
	return m.managed.Upsert(ctx, storage.ManagedResourceInput{
		RepoID:      repoID,
		Address:     resource.Address,
		Kind:        resource.Kind,
		SourcePath:  resource.SourcePath,
		NomadID:     nomadID,
		Namespace:   namespace,
		ContentHash: hash,
		LastCommit:  snapshot.CommitHash,
		Status:      "applied",
		DeleteMode:  string(resource.DeleteMode),
		Subtype:     resourceType,
	})
}

func managedResourceExists(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (bool, error) {
	switch resource.Kind {
	case "volume":
		if tracked.Subtype.Valid && tracked.Subtype.String == "csi" {
			volume, err := client.ObserveCSIVolume(ctx, tracked.NomadID.String, tracked.Namespace.String)
			return volume != nil, err
		}
		volume, err := client.ObserveHostVolume(ctx, tracked.NomadID.String, tracked.Namespace.String)
		return volume != nil, err
	case "acl_policy":
		policy, err := client.ObserveACLPolicy(ctx, resource.Name)
		if err != nil || policy == nil {
			return false, err
		}
		desired, err := manifest.CompileACLPolicy(resource)
		if err != nil {
			return false, err
		}
		return policy.Description == desired.Description && strings.TrimSpace(policy.Rules) == strings.TrimSpace(desired.Rules), nil
	default:
		return false, fmt.Errorf("resource %q cannot be observed", resource.Address)
	}
}

func deleteManagedResource(ctx context.Context, client nomadclient.ResourceClient, resource storage.ManagedResource) error {
	switch resource.Kind {
	case "volume":
		if resource.Subtype.Valid && resource.Subtype.String == "csi" {
			return client.DeleteCSIVolume(ctx, resource.NomadID.String, resource.Namespace.String, false)
		}
		return client.DeleteHostVolume(ctx, resource.NomadID.String, resource.Namespace.String, false)
	case "acl_policy":
		return client.DeleteACLPolicy(ctx, resource.NomadID.String)
	default:
		return fmt.Errorf("resource kind %q cannot be deleted", resource.Kind)
	}
}

func snapshotJobFiles(snapshot *repo.Snapshot) ([]repo.JobFile, error) {
	if snapshot == nil {
		return nil, errors.New("snapshot is required")
	}
	if snapshot.Bundle == nil {
		return snapshot.JobFiles, nil
	}

	jobFiles := make([]repo.JobFile, 0, len(snapshot.Bundle.Resources))
	for _, resource := range snapshot.Bundle.Resources {
		if resource.Kind != "job" {
			continue
		}
		source, err := manifest.NativeJobSource(resource)
		if err != nil {
			return nil, err
		}
		jobFiles = append(jobFiles, repo.JobFile{
			Path:       resource.SourcePath + "#" + resource.Address,
			Content:    source,
			DeleteMode: string(resource.DeleteMode),
		})
	}
	return jobFiles, nil
}

func jobPlanHasChanges(resp *api.JobPlanResponse) bool {
	if resp == nil {
		return true
	}
	return jobDiffHasChanges(resp.Diff)
}

func jobDiffHasChanges(diff *api.JobDiff) bool {
	if diff == nil {
		return false
	}
	if fieldDiffsHaveChanges(diff.Fields) || len(diff.Objects) > 0 {
		return true
	}
	for _, tg := range diff.TaskGroups {
		if taskGroupDiffHasChanges(tg) {
			return true
		}
	}
	return false
}

func taskGroupDiffHasChanges(diff *api.TaskGroupDiff) bool {
	if diff == nil {
		return false
	}
	if fieldDiffsHaveChanges(diff.Fields) || len(diff.Objects) > 0 {
		return true
	}
	for _, task := range diff.Tasks {
		if taskDiffHasChanges(task) {
			return true
		}
	}
	return false
}

func taskDiffHasChanges(diff *api.TaskDiff) bool {
	if diff == nil {
		return false
	}
	if fieldDiffsHaveChanges(diff.Fields) || len(diff.Objects) > 0 {
		return true
	}
	return false
}

func fieldDiffsHaveChanges(fields []*api.FieldDiff) bool {
	for _, field := range fields {
		if field == nil || isCompassCommitMetadataField(field.Name) {
			continue
		}
		return true
	}
	return false
}

func isCompassCommitMetadataField(name string) bool {
	switch name {
	case nomadMetaFieldName(compassMetaCommit),
		nomadMetaFieldName(compassMetaCommitAuthor),
		nomadMetaFieldName(compassMetaCommitTitle):
		return true
	default:
		return false
	}
}

func nomadMetaFieldName(key string) string {
	return "Meta[" + key + "]"
}

func annotateJob(job *api.Job, repoRecord *storage.Repository, jobFile repo.JobFile, snapshot *repo.Snapshot, includeCommitMetadata bool) {
	if job == nil {
		return
	}
	if job.Meta == nil {
		job.Meta = map[string]string{}
	}
	job.Meta[compassMetaRepoURL] = repoRecord.RepoURL
	job.Meta[compassMetaRepoName] = repoRecord.Name
	job.Meta[compassMetaJobFile] = jobFile.Path
	if includeCommitMetadata && snapshot != nil {
		job.Meta[compassMetaCommit] = snapshot.CommitHash
		job.Meta[compassMetaCommitAuthor] = snapshot.CommitAuthor
		job.Meta[compassMetaCommitTitle] = snapshot.CommitTitle
	}
}
