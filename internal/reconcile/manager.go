package reconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"sync"
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
	repoMu   sync.Mutex
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
	m.repoMu.Lock()
	defer m.repoMu.Unlock()
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
	m.repoMu.Lock()
	defer m.repoMu.Unlock()
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
	resourceClient, _ := m.nomad.(nomadclient.ResourceClient)
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
				if spec.Type == "csi" && resourceClient != nil {
					id := spec.CSI.ID
					if id == "" {
						id = resource.Name
					}
					volume, err := resourceClient.ObserveCSIVolume(ctx, id, effectiveNamespace(spec.CSI.Namespace))
					return volume != nil, err
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
			candidateID := tracked.NomadID.String
			if candidateID == "" {
				source, sourceErr := manifest.NativeJobSource(resource)
				if sourceErr != nil {
					return bundleplan.Observation{}, sourceErr
				}
				job, _, parseErr := parseJob(resource.SourcePath, source)
				if parseErr != nil {
					return bundleplan.Observation{}, parseErr
				}
				candidateID = jobID(job)
			}
			status, err := m.nomad.JobStatus(ctx, candidateID)
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
			annotateJob(job, repoRecord, repo.JobFile{Path: resource.Address, Content: source}, snapshot, false)
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
		if resource.Kind == "volume" {
			spec, compileErr := manifest.CompileVolume(resource)
			if compileErr != nil {
				return bundleplan.Observation{}, compileErr
			}
			if spec.Type == "csi" && spec.CSI != nil {
				id := spec.CSI.ID
				if id == "" {
					id = resource.Name
				}
				volume, observeErr := resourceClient.ObserveCSIVolume(ctx, id, effectiveNamespace(spec.CSI.Namespace))
				if tracked.NomadID.Valid {
					drifted, present, err := managedVolumeDrifted(ctx, resourceClient, resource, tracked)
					return bundleplan.Observation{Present: present, Matches: present && !drifted}, err
				}
				return bundleplan.Observation{Present: volume != nil, Matches: false}, observeErr
			}
			drifted, present, err := managedVolumeDrifted(ctx, resourceClient, resource, tracked)
			return bundleplan.Observation{Present: present, Matches: present && !drifted}, err
		}
		policy, err := resourceClient.ObserveACLPolicy(ctx, resource.Name)
		if err != nil || policy == nil {
			return bundleplan.Observation{Present: policy != nil}, err
		}
		desiredPolicy, err := manifest.CompileACLPolicy(resource)
		if err != nil {
			return bundleplan.Observation{}, err
		}
		matches := policy.Description == desiredPolicy.Description && strings.TrimSpace(policy.Rules) == strings.TrimSpace(desiredPolicy.Rules)
		return bundleplan.Observation{Present: true, Matches: matches}, nil
	}, lookup)
	if err != nil {
		return nil, err
	}
	result.Revision = snapshot.CommitHash
	return &result, nil
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
		if err := m.preflightManagedDeletion(ctx, repoRecord.ID); err != nil {
			return err
		}
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

func (m *Manager) preflightManagedDeletion(ctx context.Context, repoID int64) error {
	if m.managed == nil {
		return nil
	}
	resources, err := m.managed.ListByRepo(ctx, repoID)
	if err != nil {
		return err
	}
	for _, resource := range resources {
		if resource.DeleteMode == string(manifest.DeleteModeProtect) {
			return fmt.Errorf("managed resource %q is protected; refusing repository deletion", resource.Address)
		}
	}
	_, err = managedDeletionOrder(resources)
	return err
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
			return fmt.Errorf("managed resource %q is protected; refusing repository deletion", resource.Address)
		}
	}
	ordered, err := managedDeletionOrder(resources)
	if err != nil {
		return err
	}
	for _, resource := range ordered {
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
	if m.managed != nil {
		managed, err := m.managed.ListByRepo(ctx, repoRecord.ID)
		if err != nil {
			return err
		}
		if len(managed) > 0 {
			return errors.New("cannot switch from bundle ownership to legacy job ownership without an explicit migration")
		}
	}
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
	legacyFiles, err := m.files.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		return err
	}
	if len(legacyFiles) > 0 {
		return errors.New("cannot switch from legacy job ownership to bundle ownership without an explicit migration")
	}
	ordered, err := snapshot.Bundle.OrderedResources()
	if err != nil {
		return err
	}
	if err := validateBundleResources(ordered); err != nil {
		return err
	}
	if err := validateUniqueVolumeIdentities(ordered); err != nil {
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
	transferred := make(map[string]string)
	for _, resource := range ordered {
		if resource.Kind == "job" {
			continue
		}
		existing := tracked[resource.Address]
		if existing.Address == "" && resource.Kind == "volume" {
			if candidate, ok := findTrackedVolumeIdentity(resource, tracked); ok {
				existing = candidate
				transferred[candidate.Address] = resource.Address
			}
		}
		if err := m.ensureManagedResource(ctx, repoRecord.ID, snapshot, resourceClient, resource, existing); err != nil {
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
	orderedRemoved, err := managedDeletionOrder(removed)
	if err != nil {
		return err
	}
	for _, resource := range orderedRemoved {
		if newAddress, moved := transferred[resource.Address]; moved {
			if err := m.managed.Delete(ctx, repoRecord.ID, resource.Address); err != nil {
				return fmt.Errorf("transfer managed resource %q to %q: %w", resource.Address, newAddress, err)
			}
			continue
		}
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
				DependsOn:    resource.DependsOn,
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
		DependsOn:    resource.DependsOn,
	})
}

// managedDeletionOrder returns dependent-first order. Dependencies that are
// still present in the desired set are intentionally omitted from this list.
func managedDeletionOrder(resources []storage.ManagedResource) ([]storage.ManagedResource, error) {
	byAddress := make(map[string]storage.ManagedResource, len(resources))
	for _, resource := range resources {
		byAddress[resource.Address] = resource
	}
	state := make(map[string]uint8, len(resources))
	ordered := make([]storage.ManagedResource, 0, len(resources))
	var visit func(string) error
	visit = func(address string) error {
		switch state[address] {
		case 1:
			return fmt.Errorf("managed resource dependency cycle at %q", address)
		case 2:
			return nil
		}
		resource, ok := byAddress[address]
		if !ok {
			return nil
		}
		state[address] = 1
		for _, dependency := range resource.DependsOn {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[address] = 2
		// A dependent must be deleted before its dependency. The DFS emits
		// dependencies first, so prepend the resource after all dependencies.
		ordered = append(ordered, resource)
		return nil
	}
	for _, resource := range resources {
		if err := visit(resource.Address); err != nil {
			return nil, err
		}
	}
	// Reverse the dependency-first traversal to obtain dependent-first order.
	for left, right := 0, len(ordered)-1; left < right; left, right = left+1, right-1 {
		ordered[left], ordered[right] = ordered[right], ordered[left]
	}
	return ordered, nil
}

func validateUniqueVolumeIdentities(resources []manifest.Resource) error {
	seen := make(map[string]string)
	for _, resource := range resources {
		if resource.Kind != "volume" {
			continue
		}
		spec, err := manifest.CompileVolume(resource)
		if err != nil {
			return err
		}
		if spec.Type != "csi" || spec.CSI == nil {
			continue
		}
		id := spec.CSI.ID
		if id == "" {
			id = resource.Name
		}
		key := effectiveNamespace(spec.CSI.Namespace) + "\x00" + id
		if previous, exists := seen[key]; exists {
			return fmt.Errorf("bundle CSI volumes %q and %q share canonical identity %q", previous, resource.Address, id)
		}
		seen[key] = resource.Address
	}
	return nil
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
		case "volume":
			if _, err := manifest.CompileVolume(resource); err != nil {
				return fmt.Errorf("validate bundle volume %q: %w", resource.Address, err)
			}
		case "acl_policy":
			if _, err := manifest.CompileACLPolicy(resource); err != nil {
				return fmt.Errorf("validate bundle ACL policy %q: %w", resource.Address, err)
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

func findTrackedVolumeIdentity(resource manifest.Resource, tracked map[string]storage.ManagedResource) (storage.ManagedResource, bool) {
	spec, err := manifest.CompileVolume(resource)
	if err != nil || spec.Type != "csi" || spec.CSI == nil {
		return storage.ManagedResource{}, false
	}
	id := spec.CSI.ID
	if id == "" {
		id = resource.Name
	}
	for _, candidate := range tracked {
		if candidate.Kind == "volume" && candidate.Subtype.Valid && candidate.Subtype.String == "csi" && candidate.NomadID.Valid && candidate.NomadID.String == id && effectiveNamespace(candidate.Namespace.String) == effectiveNamespace(spec.CSI.Namespace) {
			return candidate, true
		}
	}
	return storage.ManagedResource{}, false
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
	hash := manifest.SpecHash(resource)
	manifestHash := manifest.ManifestHash(resource)
	if tracked.Address != "" && tracked.ContentHash.Valid && tracked.ContentHash.String == hash && tracked.Status == "applied" {
		exists, err := managedResourceExists(ctx, client, resource, tracked)
		if err != nil {
			return err
		}
		if exists {
			if tracked.ManifestHash.Valid && tracked.ManifestHash.String == manifestHash {
				return nil
			}
			return m.upsertManagedResource(ctx, repoID, snapshot, resource, tracked.NomadID.String, tracked.Namespace.String, hash, manifestHash, "applied", "", tracked.Subtype.String)
		}
	}

	var nomadID, namespace string
	failureHash := hash
	if tracked.ContentHash.Valid {
		failureHash = tracked.ContentHash.String
	}
	var err error
	replaceVolume := false
	if resource.Kind == "volume" && tracked.Address != "" && tracked.NomadID.Valid {
		drifted, present, observeErr := managedVolumeDrifted(ctx, client, resource, tracked)
		if observeErr != nil {
			return observeErr
		}
		replaceVolume = present && (drifted || tracked.ContentHash.Valid && tracked.ContentHash.String != hash)
	}
	if replaceVolume {
		if _, compileErr := manifest.CompileVolume(resource); compileErr != nil {
			err = compileErr
		} else {
			err = m.replaceManagedVolume(ctx, client, resource, tracked)
		}
	}
	if err != nil {
		// Preserve the existing Nomad identity when a protected or failed
		// replacement remains in place so the next reconcile can recover.
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
			DependsOn:    resource.DependsOn,
		})
		return fmt.Errorf("prepare bundle resource %q: %w", resource.Address, err)
	}
	switch resource.Kind {
	case "volume":
		spec, compileErr := manifest.CompileVolume(resource)
		if compileErr != nil {
			err = compileErr
			break
		}
		if spec.Type == "csi" && spec.CSI != nil {
			if spec.CSI.ID == "" {
				spec.CSI.ID = resource.Name
			}
			if tracked.Address == "" {
				existing, observeErr := client.ObserveCSIVolume(ctx, spec.CSI.ID, effectiveNamespace(spec.CSI.Namespace))
				if observeErr != nil {
					err = observeErr
				} else if existing != nil {
					err = fmt.Errorf("CSI volume %q already exists but is not managed by this repository", spec.CSI.ID)
				}
			}
		}
		if err != nil {
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
		if err == nil && tracked.Address == "" {
			var existing *api.ACLPolicy
			existing, err = client.ObserveACLPolicy(ctx, resource.Name)
			if err == nil && existing != nil {
				err = fmt.Errorf("ACL policy %q already exists but is not managed by this repository", resource.Name)
			}
		}
		if err == nil {
			err = client.ApplyACLPolicy(ctx, policy)
			nomadID = resource.Name
		}
	}
	if err != nil {
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
			DependsOn:    resource.DependsOn,
		})
		return fmt.Errorf("apply bundle resource %q: %w", resource.Address, err)
	}

	resourceType := ""
	if resource.Kind == "volume" {
		spec, _ := manifest.CompileVolume(resource)
		resourceType = spec.Type
	}
	return m.upsertManagedResource(ctx, repoID, snapshot, resource, nomadID, namespace, hash, manifestHash, "applied", "", resourceType)
}

func managedVolumeDrifted(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (drifted, present bool, err error) {
	spec, err := manifest.CompileVolume(resource)
	if err != nil {
		return false, false, err
	}
	if tracked.Subtype.Valid && tracked.Subtype.String == "csi" {
		volume, err := client.ObserveCSIVolume(ctx, tracked.NomadID.String, tracked.Namespace.String)
		if err != nil || volume == nil {
			return false, volume != nil, err
		}
		if spec.Type != "csi" {
			return true, true, nil
		}
		return !csiVolumeEquivalent(spec.CSI, volume), true, nil
	}
	volume, err := client.ObserveHostVolume(ctx, tracked.NomadID.String, tracked.Namespace.String)
	if err != nil || volume == nil {
		return false, volume != nil, err
	}
	if spec.Type != "host" {
		return true, true, nil
	}
	return !hostVolumeEquivalent(spec.Host, volume), true, nil
}

func hostVolumeEquivalent(desired, actual *api.HostVolume) bool {
	if desired == nil || actual == nil || desired.Name != actual.Name || effectiveNamespace(desired.Namespace) != effectiveNamespace(actual.Namespace) {
		return false
	}
	desiredPlugin := desired.PluginID
	if desiredPlugin == "" {
		desiredPlugin = "mkdir"
	}
	actualPlugin := actual.PluginID
	if actualPlugin == "" {
		actualPlugin = "mkdir"
	}
	return actualPlugin == desiredPlugin &&
		(optionalEqual(desired.NodePool, actual.NodePool) && optionalEqual(desired.NodeID, actual.NodeID)) &&
		reflect.DeepEqual(normalizedHostCapabilities(desired.RequestedCapabilities), normalizedHostCapabilities(actual.RequestedCapabilities)) &&
		reflect.DeepEqual(normalizedConstraints(desired.Constraints), normalizedConstraints(actual.Constraints)) &&
		reflect.DeepEqual(normalizedStringMap(desired.Parameters), normalizedStringMap(actual.Parameters)) &&
		desired.RequestedCapacityMinBytes == actual.RequestedCapacityMinBytes &&
		desired.RequestedCapacityMaxBytes == actual.RequestedCapacityMaxBytes &&
		(desired.CapacityBytes == 0 || desired.CapacityBytes == actual.CapacityBytes)
}

func csiVolumeEquivalent(desired, actual *api.CSIVolume) bool {
	if desired == nil || actual == nil || desired.ID != "" && desired.ID != actual.ID || desired.Name != actual.Name || effectiveNamespace(desired.Namespace) != effectiveNamespace(actual.Namespace) {
		return false
	}
	if !optionalEqual(desired.ExternalID, actual.ExternalID) || !optionalEqual(string(desired.AccessMode), string(actual.AccessMode)) || !optionalEqual(string(desired.AttachmentMode), string(actual.AttachmentMode)) || !optionalEqual(desired.PluginID, actual.PluginID) {
		return false
	}
	if !reflect.DeepEqual(normalizedMountOptions(desired.MountOptions), normalizedMountOptions(actual.MountOptions)) ||
		!reflect.DeepEqual(normalizedStringMap(desired.Parameters), normalizedStringMap(actual.Parameters)) ||
		!reflect.DeepEqual(normalizedStringMap(desired.Context), normalizedStringMap(actual.Context)) ||
		!secretsEquivalent(desired.Secrets, actual.Secrets) ||
		!reflect.DeepEqual(normalizedCSICapabilities(desired.RequestedCapabilities), normalizedCSICapabilities(actual.RequestedCapabilities)) ||
		!reflect.DeepEqual(normalizedTopologyRequest(desired.RequestedTopologies), normalizedTopologyRequest(actual.RequestedTopologies)) {
		return false
	}
	return (desired.RequestedCapacityMin == 0 || desired.RequestedCapacityMin == actual.RequestedCapacityMin) && (desired.RequestedCapacityMax == 0 || desired.RequestedCapacityMax == actual.RequestedCapacityMax) && optionalEqual(desired.CloneID, actual.CloneID) && optionalEqual(desired.SnapshotID, actual.SnapshotID)
}

func effectiveNamespace(namespace string) string {
	if namespace == "" {
		return "default"
	}
	return namespace
}

func optionalEqual(desired, actual string) bool { return desired == "" || desired == actual }

func normalizedStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	return values
}

func secretsEquivalent(desired, actual api.CSISecrets) bool {
	if len(actual) == 0 && len(desired) > 0 {
		return true
	}
	if len(desired) != len(actual) {
		return false
	}
	for key, desiredValue := range desired {
		actualValue, ok := actual[key]
		if !ok || actualValue != "<redacted>" && actualValue != desiredValue {
			return false
		}
	}
	return true
}

func normalizedHostCapabilities(values []*api.HostVolumeCapability) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, string(value.AccessMode)+"/"+string(value.AttachmentMode))
		}
	}
	sort.Strings(result)
	return result
}

func normalizedCSICapabilities(values []*api.CSIVolumeCapability) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, string(value.AccessMode)+"/"+string(value.AttachmentMode))
		}
	}
	sort.Strings(result)
	return result
}

func normalizedConstraints(values []*api.Constraint) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil {
			result = append(result, value.LTarget+"/"+value.Operand+"/"+value.RTarget)
		}
	}
	sort.Strings(result)
	return result
}

func normalizedMountOptions(value *api.CSIMountOptions) *api.CSIMountOptions {
	if value == nil {
		return &api.CSIMountOptions{}
	}
	copy := *value
	copy.MountFlags = append([]string(nil), value.MountFlags...)
	sort.Strings(copy.MountFlags)
	return &copy
}

func normalizedTopologyRequest(value *api.CSITopologyRequest) *api.CSITopologyRequest {
	if value == nil {
		return &api.CSITopologyRequest{}
	}
	copy := &api.CSITopologyRequest{}
	for _, group := range []struct {
		source []*api.CSITopology
		target *[]*api.CSITopology
	}{{value.Required, &copy.Required}, {value.Preferred, &copy.Preferred}} {
		for _, topology := range group.source {
			if topology != nil {
				segments := normalizedStringMap(topology.Segments)
				copyTopology := &api.CSITopology{Segments: segments}
				*group.target = append(*group.target, copyTopology)
			}
		}
	}
	return copy
}

func managedResourceExists(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (bool, error) {
	switch resource.Kind {
	case "job":
		status, err := client.JobStatus(ctx, tracked.NomadID.String)
		return status != nil && status.Exists, err
	case "volume":
		drifted, present, err := managedVolumeDrifted(ctx, client, resource, tracked)
		return present && !drifted, err
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
