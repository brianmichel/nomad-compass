package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	bundleplan "github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
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
	if err := m.validateRepositoryMode(ctx, repoRecord.ID, snapshot.Bundle != nil); err != nil {
		return err
	}
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

func (m *Manager) validateRepositoryMode(ctx context.Context, repoID int64, bundleMode bool) error {
	if bundleMode {
		if m.files == nil {
			return nil
		}
		files, err := m.files.ListByRepo(ctx, repoID)
		if err != nil {
			return err
		}
		if len(files) > 0 {
			return fmt.Errorf("repository %d cannot switch from legacy jobs to a Compass bundle while legacy job ownership exists", repoID)
		}
		return nil
	}
	if m.managed == nil {
		return nil
	}
	resources, err := m.managed.ListByRepo(ctx, repoID)
	if err != nil {
		return err
	}
	if len(resources) > 0 {
		return fmt.Errorf("repository %d cannot switch from a Compass bundle to legacy jobs while managed resource ownership exists", repoID)
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
				source, err := manifest.NativeJobSource(resource)
				if err != nil {
					return false, err
				}
				job, _, err := parseJob(resource.SourcePath, source)
				if err != nil {
					return false, err
				}
				status, err := m.nomad.JobStatus(ctx, resource.Name, desiredJobNamespace(job))
				return status != nil && status.Exists, err
			default:
				adapter, ok := adapterFor(resource.Kind)
				if !ok {
					return false, nil
				}
				return adapter.Lookup(ctx, resourceLookup, resource)
			}
		}
	}
	result, err := bundleplan.CompareTrackedWithLookup(ctx, snapshot.Bundle, tracked, func(ctx context.Context, resource manifest.Resource, tracked storage.ManagedResource) (bundleplan.Observation, error) {
		if resource.Kind == "job" {
			return m.observeBundleJob(ctx, resource, tracked)
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
		result, err := m.adoptBundleJob(ctx, resource)
		if err != nil {
			return err
		}
		nomadID, namespace, subtype = result.NomadID, result.Namespace, result.Subtype
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
	return m.upsertExclusiveManagedResource(ctx, repoID, snapshot, resource, nomadID, namespace, manifest.SpecHash(resource), manifest.ManifestHash(resource), "applied", "", subtype)
}

func (m *Manager) managedResourceInput(repoID int64, snapshot *repo.Snapshot, resource manifest.Resource, nomadID, namespace, specHash, manifestHash, status, lastError, subtype string) (storage.ManagedResourceInput, error) {
	dependsOn, err := json.Marshal(resource.DependsOn)
	if err != nil {
		return storage.ManagedResourceInput{}, err
	}
	return storage.ManagedResourceInput{RepoID: repoID, Address: resource.Address, Kind: resource.Kind, SourcePath: resource.SourcePath, NomadID: nomadID, Namespace: namespace, ContentHash: specHash, ManifestHash: manifestHash, LastCommit: snapshot.CommitHash, Status: status, LastError: lastError, DeleteMode: string(resource.DeleteMode), Subtype: subtype, DependsOn: string(dependsOn)}, nil
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
		if resource.DeleteMode == string(manifest.DeleteModeProtect) && resource.Status == "protected" {
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
		if err := m.nomad.DeregisterJob(ctx, resource.NomadID.String, resource.Namespace.String, true); err != nil {
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
	if m.managed != nil {
		resources, err := m.managed.ListByRepo(ctx, repoID)
		if err != nil {
			return err
		}
		protected := 0
		for _, resource := range resources {
			if resource.DeleteMode == string(manifest.DeleteModeProtect) {
				protected++
			}
		}
		if protected > 0 {
			return fmt.Errorf("repository has %d protected resource(s); remove them from the bundle and reconcile before using repo orphan actions", protected)
		}
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
	ordered, err := managedDeletionOrder(resources)
	if err != nil {
		return err
	}
	for _, resource := range ordered {
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
		if file.Status == "adoption_required" || !file.JobID.Valid || file.JobID.String == "" {
			continue
		}
		if err := m.nomad.DeregisterJob(ctx, file.JobID.String, file.Namespace.String, true); err != nil {
			return err
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
	protectedDependencies := protectedDependencyClosure(removed)
	for _, resource := range orderedRemoved {
		if newAddress, moved := transferred[resource.Address]; moved {
			if err := m.managed.Delete(ctx, repoRecord.ID, resource.Address); err != nil {
				return fmt.Errorf("transfer managed resource %q to %q: %w", resource.Address, newAddress, err)
			}
			continue
		}
		if resource.DeleteMode != string(manifest.DeleteModeProtect) && protectedDependencies[resource.Address] {
			if err := m.managed.Upsert(ctx, storage.ManagedResourceInput{RepoID: repoRecord.ID, Address: resource.Address, Kind: resource.Kind, SourcePath: resource.SourcePath, NomadID: resource.NomadID.String, Namespace: resource.Namespace.String, ContentHash: resource.ContentHash.String, ManifestHash: resource.ManifestHash.String, LastCommit: resource.LastCommit.String, Status: "protected", LastError: "dependency retained because a protected dependent remains", DeleteMode: resource.DeleteMode, Subtype: resource.Subtype.String, DependsOn: resource.DependsOn.String}); err != nil {
				return err
			}
			continue
		}
		if resource.DeleteMode == string(manifest.DeleteModeProtect) {
			if err := m.managed.Upsert(ctx, storage.ManagedResourceInput{
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
			}); err != nil {
				return fmt.Errorf("record protected orphan %q: %w", resource.Address, err)
			}
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
	input, err := m.managedResourceInput(repoID, snapshot, resource, nomadID, namespace, specHash, manifestHash, status, lastError, subtype)
	if err != nil {
		return err
	}
	return m.managed.Upsert(ctx, input)
}

func (m *Manager) upsertExclusiveManagedResource(ctx context.Context, repoID int64, snapshot *repo.Snapshot, resource manifest.Resource, nomadID, namespace, specHash, manifestHash, status, lastError, subtype string) error {
	input, err := m.managedResourceInput(repoID, snapshot, resource, nomadID, namespace, specHash, manifestHash, status, lastError, subtype)
	if err != nil {
		return err
	}
	return m.managed.UpsertExclusive(ctx, input)
}

// managedDeletionOrder returns dependents before their dependencies. The
// dependency JSON is persisted with each managed resource so this remains
// correct even when the current bundle no longer contains the removed nodes.
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
		var dependencies []string
		if resource.DependsOn.Valid && resource.DependsOn.String != "" {
			if err := json.Unmarshal([]byte(resource.DependsOn.String), &dependencies); err != nil {
				return fmt.Errorf("invalid dependencies for %q: %w", address, err)
			}
		}
		for _, dependency := range dependencies {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[address] = 2
		ordered = append(ordered, resource)
		return nil
	}
	for _, resource := range resources {
		if err := visit(resource.Address); err != nil {
			return nil, err
		}
	}
	for left, right := 0, len(ordered)-1; left < right; left, right = left+1, right-1 {
		ordered[left], ordered[right] = ordered[right], ordered[left]
	}
	return ordered, nil
}

func protectedDependencyClosure(resources []storage.ManagedResource) map[string]bool {
	byAddress := make(map[string]storage.ManagedResource, len(resources))
	for _, resource := range resources {
		byAddress[resource.Address] = resource
	}
	protected := make(map[string]bool)
	var visit func(string)
	visit = func(address string) {
		resource, ok := byAddress[address]
		if !ok || protected[address] {
			return
		}
		protected[address] = true
		var deps []string
		if resource.DependsOn.Valid {
			_ = json.Unmarshal([]byte(resource.DependsOn.String), &deps)
		}
		for _, dep := range deps {
			visit(dep)
		}
	}
	for _, resource := range resources {
		if resource.DeleteMode == string(manifest.DeleteModeProtect) {
			var deps []string
			if resource.DependsOn.Valid {
				_ = json.Unmarshal([]byte(resource.DependsOn.String), &deps)
			}
			for _, dep := range deps {
				visit(dep)
			}
		}
	}
	return protected
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
		if resource.Kind == "job" {
			source, err := manifest.NativeJobSource(resource)
			if err != nil {
				return err
			}
			if _, _, err := parseJob(resource.SourcePath, source); err != nil {
				return fmt.Errorf("validate bundle job %q: %w", resource.Address, err)
			}
			continue
		}
		adapter, ok := adapterFor(resource.Kind)
		if !ok {
			return fmt.Errorf("bundle resource %q is not supported yet", resource.Address)
		}
		if err := adapter.Validate(resource); err != nil {
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
		if observation.Present && (!observation.Matches || tracked.ContentHash.Valid && tracked.ContentHash.String != hash) {
			err = adapter.Replace(ctx, client, resource, tracked)
		}
	}
	if err != nil {
		return m.recordManagedResourceFailure(ctx, repoID, snapshot, resource, tracked, failureHash, manifestHash, err, "prepare")
	}
	if resource.Kind == "volume" && tracked.Address == "" {
		spec, compileErr := manifest.CompileVolume(resource)
		if compileErr != nil {
			err = compileErr
		} else if spec.Type == "csi" && spec.CSI != nil {
			if spec.CSI.ID == "" {
				spec.CSI.ID = resource.Name
			}
			existing, observeErr := client.ObserveCSIVolume(ctx, spec.CSI.ID, effectiveNamespace(spec.CSI.Namespace))
			if observeErr != nil {
				err = observeErr
			} else if existing != nil {
				err = fmt.Errorf("CSI volume %q already exists but is not managed by this repository", spec.CSI.ID)
			}
		}
	}
	if err == nil && resource.Kind == "acl_policy" && tracked.Address == "" {
		existing, observeErr := client.ObserveACLPolicy(ctx, resource.Name)
		if observeErr != nil {
			err = observeErr
		} else if existing != nil {
			err = fmt.Errorf("ACL policy %q already exists but is not managed by this repository", resource.Name)
		}
	}
	var result managedResourceResult
	if err == nil {
		result, err = adapter.Apply(ctx, client, resource, tracked)
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
	if resource.Kind == "job" {
		return client.DeregisterJob(ctx, resource.NomadID.String, resource.Namespace.String, true)
	}
	adapter, ok := adapterFor(resource.Kind)
	if !ok {
		return fmt.Errorf("resource kind %q cannot be deleted", resource.Kind)
	}
	return adapter.Delete(ctx, client, resource)
}
