package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/hashicorp/nomad/api"
	"github.com/hashicorp/nomad/jobspec2"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	bundleplan "github.com/brianmichel/nomad-compass/internal/plan"
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
	snapshot, err := m.syncRepo(ctx, repoRecord)
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

func (m *Manager) syncRepo(ctx context.Context, repoRecord *storage.Repository) (*repo.Snapshot, error) {
	if repoRecord == nil {
		return nil, errors.New("repository is required")
	}
	var cred *storage.Credential
	var payload *storage.CredentialPayload
	if repoRecord.CredentialID.Valid {
		var err error
		cred, err = m.creds.Get(ctx, repoRecord.CredentialID.Int64)
		if err != nil {
			return nil, err
		}
		if cred == nil {
			return nil, errors.New("linked credential not found")
		}
		payload, err = m.creds.DecryptPayload(cred)
		if err != nil {
			return nil, err
		}
	}
	return m.git.Sync(ctx, *repoRecord, cred, payload)
}

// PlanRepo returns a read-only plan for the bundle currently in a repository.
// Git synchronization updates only the local checkout; this method does not
// write Compass tracking state or mutate Nomad.
func (m *Manager) PlanRepo(ctx context.Context, repoID int64) (*bundleplan.Report, error) {
	repoRecord, err := m.repos.Get(ctx, repoID)
	if err != nil {
		return nil, err
	}
	if repoRecord == nil {
		return nil, errors.New("repository not found")
	}
	snapshot, err := m.syncRepo(ctx, repoRecord)
	if err != nil {
		return nil, err
	}
	if snapshot.Bundle == nil {
		return nil, errors.New("repository does not contain a Compass bundle")
	}
	ordered, err := snapshot.Bundle.OrderedResources()
	if err != nil {
		return nil, err
	}
	if err := validateBundleResources(ordered); err != nil {
		return nil, err
	}
	if m.managed == nil {
		return nil, errors.New("managed resource store is required for bundle planning")
	}
	tracked, err := m.managed.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		return nil, err
	}
	var resourceClient nomadclient.ResourceClient
	if len(tracked) > 0 {
		resourceClient, _ = m.nomad.(nomadclient.ResourceClient)
	}
	var lookup bundleplan.Lookup
	if resourceLookup, ok := m.nomad.(nomadclient.ResourceLookup); ok {
		lookup = func(ctx context.Context, resource manifest.Resource) (bool, error) {
			switch resource.Kind {
			case "job":
				status, err := m.nomad.JobStatus(ctx, resource.Name)
				return status != nil && status.Exists, err
			case "acl_policy":
				policy, err := resourceLookup.ObserveACLPolicy(ctx, resource.Name)
				return policy != nil, err
			case "volume":
				spec, err := manifest.CompileVolume(resource)
				if err != nil {
					return false, err
				}
				if spec.Type == "host" {
					volume, err := resourceLookup.FindHostVolume(ctx, spec.Host.Name, spec.Host.Namespace)
					return volume != nil, err
				}
				volume, err := resourceLookup.FindCSIVolume(ctx, spec.CSI.Name, spec.CSI.Namespace)
				return volume != nil, err
			default:
				return false, nil
			}
		}
	}
	result, err := bundleplan.CompareTrackedWithLookup(ctx, snapshot.Bundle, tracked, func(ctx context.Context, resource manifest.Resource, tracked storage.ManagedResource) (bundleplan.Observation, error) {
		if resource.Kind == "job" {
			status, err := m.nomad.JobStatus(ctx, tracked.NomadID.String)
			if err != nil || status == nil || !status.Exists {
				return bundleplan.Observation{Present: status != nil && status.Exists}, err
			}
			source, err := manifest.NativeJobSource(resource)
			if err != nil {
				return bundleplan.Observation{}, err
			}
			job, _, err := parseJob(resource.SourcePath, source)
			if err != nil {
				return bundleplan.Observation{}, err
			}
			job.ID = &tracked.NomadID.String
			jobPlan, err := m.nomad.PlanJob(ctx, job)
			if err != nil {
				return bundleplan.Observation{}, err
			}
			return bundleplan.Observation{Present: true, Matches: !jobPlanHasChanges(jobPlan)}, nil
		}
		if resourceClient == nil {
			return bundleplan.Observation{}, errors.New("Nomad client does not support bundle resources")
		}
		adapter, ok := adapterFor(resource.Kind)
		if !ok {
			return bundleplan.Observation{}, fmt.Errorf("resource kind %q cannot be observed", resource.Kind)
		}
		return adapter.Observe(ctx, resourceClient, resource, tracked)
	}, lookup)
	if err != nil {
		return nil, err
	}
	result.Revision = snapshot.CommitHash
	return &result, nil
}

// AdoptBundleResource records explicit ownership of an unmanaged resource
// after verifying that the live Nomad object matches the desired bundle body.
// Adoption never changes the Nomad object.
func (m *Manager) AdoptBundleResource(ctx context.Context, repoID int64, address string) error {
	repoRecord, err := m.repos.Get(ctx, repoID)
	if err != nil {
		return err
	}
	if repoRecord == nil {
		return errors.New("repository not found")
	}
	snapshot, err := m.syncRepo(ctx, repoRecord)
	if err != nil {
		return err
	}
	return m.adoptBundleResource(ctx, repoID, snapshot, address)
}

func (m *Manager) adoptBundleResource(ctx context.Context, repoID int64, snapshot *repo.Snapshot, address string) error {
	if snapshot == nil || snapshot.Bundle == nil {
		return errors.New("repository does not contain a Compass bundle")
	}
	ordered, err := snapshot.Bundle.OrderedResources()
	if err != nil {
		return err
	}
	if err := validateBundleResources(ordered); err != nil {
		return err
	}
	var resource manifest.Resource
	found := false
	for _, candidate := range ordered {
		if candidate.Address == address {
			resource = candidate
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("bundle resource %q not found", address)
	}
	if m.managed == nil {
		return errors.New("managed resource store is required for adoption")
	}
	tracked, err := m.managedResources(ctx, repoID)
	if err != nil {
		return err
	}
	if _, exists := tracked[address]; exists {
		return fmt.Errorf("resource %q is already managed by Compass", address)
	}

	var nomadID, namespace, subtype string
	if resource.Kind == "job" {
		status, err := m.nomad.JobStatus(ctx, resource.Name)
		if err != nil {
			return err
		}
		if status == nil || !status.Exists {
			return fmt.Errorf("Nomad job %q does not exist", resource.Name)
		}
		source, err := manifest.NativeJobSource(resource)
		if err != nil {
			return err
		}
		job, _, err := parseJob(resource.SourcePath, source)
		if err != nil {
			return err
		}
		job.ID = &resource.Name
		jobPlan, err := m.nomad.PlanJob(ctx, job)
		if err != nil {
			return err
		}
		if jobPlanHasChanges(jobPlan) {
			return fmt.Errorf("Nomad job %q does not match desired bundle resource", resource.Name)
		}
		nomadID = resource.Name
	} else {
		lookup, ok := m.nomad.(nomadclient.ResourceLookup)
		if !ok {
			return errors.New("Nomad client does not support resource adoption lookups")
		}
		adapter, ok := adapterFor(resource.Kind)
		if !ok {
			return fmt.Errorf("resource kind %q cannot be adopted", resource.Kind)
		}
		result, err := adapter.Adopt(ctx, lookup, resource)
		if err != nil {
			return err
		}
		nomadID, namespace, subtype = result.NomadID, result.Namespace, result.Subtype
	}
	return m.upsertManagedResource(ctx, repoID, snapshot, resource, nomadID, namespace, manifest.SpecHash(resource), manifest.ManifestHash(resource), "applied", "", subtype)
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

// ListProtectedResources returns resources that Compass retained after they
// were removed from a bundle or otherwise require explicit lifecycle action.
func (m *Manager) ListProtectedResources(ctx context.Context, repoID int64) ([]storage.ManagedResource, error) {
	if m.managed == nil {
		return nil, errors.New("managed resource store is required")
	}
	resources, err := m.managed.ListByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	protected := make([]storage.ManagedResource, 0)
	for _, resource := range resources {
		if resource.DeleteMode == string(manifest.DeleteModeProtect) {
			protected = append(protected, resource)
		}
	}
	return protected, nil
}

// ForgetProtectedResource removes Compass ownership metadata without deleting
// the Nomad resource. This is intentionally explicit because it creates an
// unmanaged resource by design.
func (m *Manager) ForgetProtectedResource(ctx context.Context, repoID int64, address string) error {
	resource, err := m.protectedResource(ctx, repoID, address)
	if err != nil {
		return err
	}
	if resource.Status != "protected" {
		return fmt.Errorf("resource %q is not a protected orphan", address)
	}
	return m.managed.Delete(ctx, repoID, address)
}

// DeleteProtectedResource removes a protected orphan from Nomad and Compass.
func (m *Manager) DeleteProtectedResource(ctx context.Context, repoID int64, address string) error {
	resource, err := m.protectedResource(ctx, repoID, address)
	if err != nil {
		return err
	}
	if resource.Status != "protected" {
		return fmt.Errorf("resource %q is not a protected orphan", address)
	}
	if resource.Kind == "job" {
		if err := m.nomad.DeregisterJob(ctx, resource.NomadID.String, true); err != nil {
			return err
		}
	} else {
		client, ok := m.nomad.(nomadclient.ResourceClient)
		if !ok {
			return errors.New("Nomad client does not support bundle resource cleanup")
		}
		if err := deleteManagedResource(ctx, client, resource); err != nil {
			return err
		}
	}
	return m.managed.Delete(ctx, repoID, address)
}

func (m *Manager) protectedResource(ctx context.Context, repoID int64, address string) (storage.ManagedResource, error) {
	resources, err := m.ListProtectedResources(ctx, repoID)
	if err != nil {
		return storage.ManagedResource{}, err
	}
	for _, resource := range resources {
		if resource.Address == address {
			return resource, nil
		}
	}
	return storage.ManagedResource{}, fmt.Errorf("protected resource %q not found", address)
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
	if protected, err := m.ListProtectedResources(ctx, repoID); err != nil {
		return err
	} else if len(protected) > 0 {
		return fmt.Errorf("repository has %d protected resource(s); use repo orphan list, forget, or delete first", len(protected))
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
	for _, resource := range managedDeletionOrder(resources) {
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

	if m.managed == nil {
		return errors.New("managed resource store is required for bundle resources")
	}
	ordered, err := snapshot.Bundle.OrderedResources()
	if err != nil {
		return err
	}
	if err := validateBundleResources(ordered); err != nil {
		return err
	}

	var resourceClient nomadclient.ResourceClient
	for _, resource := range ordered {
		if resource.Kind == "job" {
			continue
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

	if err := m.ensureBundleJobs(ctx, repoRecord, snapshot, ordered, tracked); err != nil {
		return err
	}

	desired := make(map[string]struct{}, len(snapshot.Bundle.Resources))
	for _, resource := range snapshot.Bundle.Resources {
		desired[resource.Address] = struct{}{}
	}
	removed := make([]storage.ManagedResource, 0)
	for _, resource := range tracked {
		if _, exists := desired[resource.Address]; !exists {
			removed = append(removed, resource)
		}
	}
	for _, resource := range managedDeletionOrder(removed) {
		if resource.DeleteMode == string(manifest.DeleteModeProtect) {
			_ = m.managed.Upsert(ctx, storage.ManagedResourceInput{
				RepoID:       repoRecord.ID,
				Address:      resource.Address,
				Kind:         resource.Kind,
				SourcePath:   resource.SourcePath,
				NomadID:      resource.NomadID.String,
				Namespace:    resource.Namespace.String,
				ContentHash:  resource.ContentHash.String,
				ManifestHash: resource.ManifestHash.String,
				LastCommit:   resource.LastCommit.String,
				Status:       "protected",
				LastError:    "resource removed from bundle but deletion is protected",
				DeleteMode:   resource.DeleteMode,
				Subtype:      resource.Subtype.String,
				DependsOn:    resource.DependsOn.String,
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

func (m *Manager) upsertManagedResource(ctx context.Context, repoID int64, snapshot *repo.Snapshot, resource manifest.Resource, nomadID, namespace, specHash, manifestHash, status, lastError, subtype string) error {
	dependsOn, err := json.Marshal(resource.DependsOn)
	if err != nil {
		return err
	}
	return m.managed.Upsert(ctx, storage.ManagedResourceInput{
		RepoID:       repoID,
		Address:      resource.Address,
		Kind:         resource.Kind,
		SourcePath:   resource.SourcePath,
		NomadID:      nomadID,
		Namespace:    namespace,
		ContentHash:  specHash,
		ManifestHash: manifestHash,
		LastCommit:   snapshot.CommitHash,
		Status:       status,
		LastError:    lastError,
		DeleteMode:   string(resource.DeleteMode),
		Subtype:      subtype,
		DependsOn:    string(dependsOn),
	})
}

// managedDeletionOrder returns dependents before their dependencies. The
// dependency JSON is persisted with each managed resource so this remains
// correct even when the current bundle no longer contains the removed nodes.
func managedDeletionOrder(resources []storage.ManagedResource) []storage.ManagedResource {
	byAddress := make(map[string]storage.ManagedResource, len(resources))
	ordered := append([]storage.ManagedResource(nil), resources...)
	sort.SliceStable(ordered, func(i, j int) bool {
		rankI, rankJ := managedDeletionRank(ordered[i].Kind), managedDeletionRank(ordered[j].Kind)
		if rankI != rankJ {
			return rankI > rankJ
		}
		return ordered[i].Address > ordered[j].Address
	})
	for _, resource := range ordered {
		byAddress[resource.Address] = resource
	}

	state := make(map[string]uint8, len(ordered))
	dependencyFirst := make([]storage.ManagedResource, 0, len(ordered))
	var visit func(string)
	visit = func(address string) {
		switch state[address] {
		case 1:
			return
		case 2:
			return
		}
		resource, exists := byAddress[address]
		if !exists {
			return
		}
		state[address] = 1
		var dependencies []string
		if resource.DependsOn.Valid {
			_ = json.Unmarshal([]byte(resource.DependsOn.String), &dependencies)
		}
		for _, dependency := range dependencies {
			visit(dependency)
		}
		state[address] = 2
		dependencyFirst = append(dependencyFirst, resource)
	}
	for _, resource := range ordered {
		visit(resource.Address)
	}

	result := make([]storage.ManagedResource, 0, len(dependencyFirst))
	for i := len(dependencyFirst) - 1; i >= 0; i-- {
		result = append(result, dependencyFirst[i])
	}
	return result
}

func managedDeletionRank(kind string) int {
	if kind == "job" {
		return 0
	}
	return 1
}

func encodedDependencies(resource manifest.Resource) string {
	encoded, _ := json.Marshal(resource.DependsOn)
	return string(encoded)
}

func validateBundleResources(resources []manifest.Resource) error {
	for _, resource := range resources {
		switch resource.Kind {
		case "job":
			source, err := manifest.NativeJobSource(resource)
			if err != nil {
				return err
			}
			if _, _, err := parseJob(resource.SourcePath, source); err != nil {
				return fmt.Errorf("validate bundle job %q: %w", resource.Address, err)
			}
		case "volume", "acl_policy":
			adapter, ok := adapterFor(resource.Kind)
			if !ok {
				return fmt.Errorf("bundle resource %q is not supported yet", resource.Address)
			}
			if err := adapter.Validate(resource); err != nil {
				return err
			}
		default:
			return fmt.Errorf("bundle resource %q is not supported yet", resource.Address)
		}
	}
	return nil
}

func (m *Manager) ensureBundleJobs(ctx context.Context, repoRecord *storage.Repository, snapshot *repo.Snapshot, ordered []manifest.Resource, tracked map[string]storage.ManagedResource) error {
	for _, resource := range ordered {
		if resource.Kind != "job" {
			continue
		}

		source, err := manifest.NativeJobSource(resource)
		if err != nil {
			return err
		}
		job, submission, err := parseJob(resource.Address, source)
		if err != nil {
			return fmt.Errorf("parse bundle job %q: %w", resource.Address, err)
		}

		hash := manifest.SpecHash(resource)
		manifestHash := manifest.ManifestHash(resource)
		existing := tracked[resource.Address]
		jobFile := repo.JobFile{Path: resource.Address, Content: source, DeleteMode: string(resource.DeleteMode)}
		trackedJobID := existing.NomadID.String

		if existing.Address == "" {
			candidateID := jobID(job)
			status, statusErr := m.nomad.JobStatus(ctx, candidateID)
			if statusErr != nil {
				return fmt.Errorf("check ownership for bundle job %q: %w", resource.Address, statusErr)
			}
			if status != nil && status.Exists {
				return fmt.Errorf("bundle job %q collides with unmanaged Nomad job %q", resource.Address, candidateID)
			}
		}

		if existing.Address != "" && trackedJobID != "" {
			status, statusErr := m.nomad.JobStatus(ctx, trackedJobID)
			if statusErr != nil {
				if m.logger != nil {
					m.logger.Warn("bundle job status check failed", "repo", repoRecord.Name, "job", resource.Address, "error", statusErr)
				}
			} else if status != nil && status.Exists {
				job.ID = &trackedJobID
				annotateJob(job, repoRecord, jobFile, snapshot, false)
				plan, planErr := m.nomad.PlanJob(ctx, job)
				if planErr == nil && !jobPlanHasChanges(plan) {
					if err := m.upsertManagedResource(ctx, repoRecord.ID, snapshot, resource, trackedJobID, "", hash, manifestHash, "applied", "", ""); err != nil {
						return err
					}
					continue
				}
			}
		}

		appliedID, applyErr := m.applyJob(ctx, repoRecord, jobFile, snapshot, job, submission)
		if applyErr != nil {
			_ = m.upsertManagedResource(ctx, repoRecord.ID, snapshot, resource, trackedJobID, "", hash, manifestHash, "failed", applyErr.Error(), "")
			return fmt.Errorf("apply bundle job %q: %w", resource.Address, applyErr)
		}
		if err := m.upsertManagedResource(ctx, repoRecord.ID, snapshot, resource, appliedID, "", hash, manifestHash, "applied", "", ""); err != nil {
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

func (m *Manager) replaceManagedVolume(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) error {
	if resource.DeleteMode != manifest.DeleteModeAllow {
		return fmt.Errorf("volume changes are protected; use a new resource address or set delete = %q to allow replacement", manifest.DeleteModeAllow)
	}
	if err := deleteManagedResource(ctx, client, tracked); err != nil {
		return fmt.Errorf("replace volume %q: delete existing volume: %w", resource.Address, err)
	}
	return nil
}

func (m *Manager) ensureManagedResource(ctx context.Context, repoID int64, snapshot *repo.Snapshot, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) error {
	adapter, ok := adapterFor(resource.Kind)
	if !ok {
		return fmt.Errorf("resource kind %q cannot be managed as a bundle resource", resource.Kind)
	}
	hash := manifest.SpecHash(resource)
	manifestHash := manifest.ManifestHash(resource)
	if tracked.Address != "" && tracked.ContentHash.Valid && tracked.ContentHash.String == hash && tracked.Status == "applied" {
		observation, err := adapter.Observe(ctx, client, resource, tracked)
		if err != nil {
			return err
		}
		if observation.Present && observation.Matches {
			if tracked.ManifestHash.Valid && tracked.ManifestHash.String == manifestHash {
				return nil
			}
			return m.upsertManagedResource(ctx, repoID, snapshot, resource, tracked.NomadID.String, tracked.Namespace.String, hash, manifestHash, "applied", "", tracked.Subtype.String)
		}
	}

	failureHash := hash
	if tracked.ContentHash.Valid {
		failureHash = tracked.ContentHash.String
	}
	var err error
	if tracked.Address != "" && tracked.NomadID.Valid {
		observation, observeErr := adapter.Observe(ctx, client, resource, tracked)
		if observeErr != nil {
			return observeErr
		}
		if resource.Kind == "volume" && observation.Present && (!observation.Matches || tracked.ContentHash.Valid && tracked.ContentHash.String != hash) {
			err = m.replaceManagedVolume(ctx, client, resource, tracked)
		}
	}
	if err != nil {
		return m.recordManagedResourceFailure(ctx, repoID, snapshot, resource, tracked, failureHash, manifestHash, err, "prepare")
	}
	if resource.Kind == "acl_policy" && tracked.Address == "" {
		existing, observeErr := client.ObserveACLPolicy(ctx, resource.Name)
		if observeErr != nil {
			err = observeErr
		} else if existing != nil {
			err = fmt.Errorf("ACL policy %q already exists but is not managed by this repository", resource.Name)
		}
	}
	var result managedResourceResult
	if err == nil {
		result, err = adapter.Apply(ctx, client, resource)
	}
	if err != nil {
		return m.recordManagedResourceFailure(ctx, repoID, snapshot, resource, tracked, failureHash, manifestHash, err, "apply")
	}
	return m.upsertManagedResource(ctx, repoID, snapshot, resource, result.NomadID, result.Namespace, hash, manifestHash, "applied", "", result.Subtype)
}

func (m *Manager) recordManagedResourceFailure(ctx context.Context, repoID int64, snapshot *repo.Snapshot, resource manifest.Resource, tracked storage.ManagedResource, failureHash, manifestHash string, err error, phase string) error {
	_ = m.managed.Upsert(ctx, storage.ManagedResourceInput{
		RepoID:       repoID,
		Address:      resource.Address,
		Kind:         resource.Kind,
		SourcePath:   resource.SourcePath,
		NomadID:      tracked.NomadID.String,
		Namespace:    tracked.Namespace.String,
		ContentHash:  failureHash,
		ManifestHash: manifestHash,
		LastCommit:   snapshot.CommitHash,
		Status:       "failed",
		LastError:    err.Error(),
		DeleteMode:   string(resource.DeleteMode),
		Subtype:      tracked.Subtype.String,
		DependsOn:    encodedDependencies(resource),
	})
	return fmt.Errorf("%s bundle resource %q: %w", phase, resource.Address, err)
}

func deleteManagedResource(ctx context.Context, client nomadclient.ResourceClient, resource storage.ManagedResource) error {
	switch resource.Kind {
	case "job":
		return client.DeregisterJob(ctx, resource.NomadID.String, true)
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
	if snapshot.Bundle != nil {
		return nil, errors.New("bundle jobs use managed-resource tracking")
	}
	return snapshot.JobFiles, nil
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
