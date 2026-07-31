package reconcile

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	repomodel "github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestManagedDeletionOrderDeletesDependentsFirst(t *testing.T) {
	ordered, err := managedDeletionOrder([]storage.ManagedResource{
		{Address: "volume.data", Kind: "volume"},
		{Address: "job.app", Kind: "job", DependsOn: sql.NullString{String: `["volume.data"]`, Valid: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) != 2 || ordered[0].Address != "job.app" || ordered[1].Address != "volume.data" {
		t.Fatalf("order = %#v", ordered)
	}
	_, err = managedDeletionOrder([]storage.ManagedResource{
		{Address: "a", DependsOn: sql.NullString{String: `["b"]`, Valid: true}},
		{Address: "b", DependsOn: sql.NullString{String: `["a"]`, Valid: true}},
	})
	if err == nil {
		t.Fatal("expected dependency cycle to fail closed")
	}
}

func TestParseJob(t *testing.T) {
	job, submission, err := parseJob(".nomad/job.nomad.hcl", []byte(`job "demo" { datacenters = ["dc1"] }`))
	if err != nil {
		t.Fatalf("parse job: %v", err)
	}
	if job == nil || job.Name == nil || *job.Name != "demo" {
		t.Fatalf("unexpected job: %#v", job)
	}
	if submission == nil {
		t.Fatal("expected submission metadata")
	}
	if submission.Source == "" || submission.Format != "hcl2" {
		t.Fatalf("unexpected submission: %#v", submission)
	}
}

func TestNativeJobSourceUsesStableBundleAddress(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "demo" {
  resource "job" "api" {
    datacenters = ["dc1"]
  }
}`), ".nomad/compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}

	var resource manifest.Resource
	for _, candidate := range bundle.Resources {
		if candidate.Address == "job.api" {
			resource = candidate
		}
	}
	source, err := manifest.NativeJobSource(resource)
	if err != nil {
		t.Fatalf("build native job source: %v", err)
	}
	job, _, err := parseJob(resource.Address, source)
	if err != nil {
		t.Fatalf("parse embedded job: %v", err)
	}
	if job.Name == nil || *job.Name != "api" {
		t.Fatalf("unexpected embedded job: %#v", job)
	}
}

func TestEnsureBundleAppliesResourcesInDependencyOrder(t *testing.T) {
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "bundle.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)
	managedStore := storage.NewManagedResourceStore(db)
	repoRecord, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "bundle",
		RepoURL: "https://example.com/bundle.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    delete = "protect"
    name = "compass-data"
    type = "host"
    plugin_id = "mkdir"
    capability {
      access_mode = "single-node-single-writer"
      attachment_mode = "file-system"
    }
  }
  resource "acl_policy" "compass" {
    delete = "protect"
    rules {
      namespace "default" {
        capabilities = ["read-job", "submit-job"]
      }
    }
  }
  resource "job" "compass" {
    depends_on = ["volume.data", "acl_policy.compass"]
    datacenters = ["dc1"]
  }
}`), ".nomad/compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}

	fake := &fakeNomad{}
	manager := &Manager{
		files:   fileStore,
		managed: managedStore,
		nomad:   fake,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := manager.ensureBundle(ctx, repoRecord, &repomodel.Snapshot{
		CommitHash: "commit-1",
		Bundle:     bundle,
	}, true); err != nil {
		t.Fatalf("ensure bundle: %v", err)
	}

	wantCalls := []string{"volume:compass-data", "policy:compass", "job:compass"}
	if !reflect.DeepEqual(fake.resourceCalls, wantCalls) {
		t.Fatalf("resource calls = %v, want %v", fake.resourceCalls, wantCalls)
	}
	tracked, err := managedStore.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		t.Fatalf("list managed resources: %v", err)
	}
	if len(tracked) != 3 {
		t.Fatalf("expected all bundle resources tracked, got %d", len(tracked))
	}
	for _, resource := range tracked {
		if resource.Address == "job.compass" && (!resource.DependsOn.Valid || resource.DependsOn.String != `["volume.data","acl_policy.compass"]`) {
			t.Fatalf("expected persisted job dependencies, got %#v", resource.DependsOn)
		}
	}

	remainingBundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "job" "compass" {
    datacenters = ["dc1"]
  }
}`), ".nomad/compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse remaining bundle: %v", err)
	}
	if err := manager.ensureBundle(ctx, repoRecord, &repomodel.Snapshot{
		CommitHash: "commit-2",
		Bundle:     remainingBundle,
	}, true); err != nil {
		t.Fatalf("ensure remaining bundle: %v", err)
	}
	if slices.Contains(fake.resourceCalls, "delete-volume:host-volume-id") || slices.Contains(fake.resourceCalls, "delete-policy:compass") {
		t.Fatalf("protected resources were deleted: %v", fake.resourceCalls)
	}
}

func newBundleManager(t *testing.T) (*Manager, *storage.Repository, *storage.ManagedResourceStore, *fakeNomad) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "bundle.sqlite"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	repoRecord, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "bundle",
		RepoURL: "https://example.com/bundle.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}
	fake := &fakeNomad{}
	return &Manager{
		repos:   repoStore,
		files:   storage.NewRepoFileStore(db),
		managed: storage.NewManagedResourceStore(db),
		nomad:   fake,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, repoRecord, storage.NewManagedResourceStore(db), fake
}

func TestProtectedResourceLifecycleRequiresExplicitAction(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, _ := newBundleManager(t)
	if err := managed.Upsert(ctx, storage.ManagedResourceInput{
		RepoID:     repoRecord.ID,
		Address:    "volume.legacy",
		Kind:       "volume",
		NomadID:    "legacy-id",
		Status:     "protected",
		DeleteMode: string(manifest.DeleteModeProtect),
	}); err != nil {
		t.Fatalf("upsert protected resource: %v", err)
	}

	protected, err := manager.ListProtectedResources(ctx, repoRecord.ID)
	if err != nil || len(protected) != 1 || protected[0].Address != "volume.legacy" {
		t.Fatalf("unexpected protected resources: %v %#v", err, protected)
	}
	if err := manager.DeleteRepository(ctx, repoRecord.ID, false); err == nil || !strings.Contains(err.Error(), "protected resource") {
		t.Fatalf("expected repository deletion guard, got %v", err)
	}
	if err := manager.ForgetProtectedResource(ctx, repoRecord.ID, "volume.legacy"); err != nil {
		t.Fatalf("forget protected resource: %v", err)
	}
	protected, err = manager.ListProtectedResources(ctx, repoRecord.ID)
	if err != nil || len(protected) != 0 {
		t.Fatalf("protected resource was not forgotten: %v %#v", err, protected)
	}
}

func TestManagedDeletionOrderUsesPersistedDependencies(t *testing.T) {
	resources := []storage.ManagedResource{
		{Address: "acl_policy.web", Kind: "acl_policy"},
		{Address: "volume.data", Kind: "volume", DependsOn: sql.NullString{String: `["acl_policy.web"]`, Valid: true}},
		{Address: "job.web", Kind: "job", DependsOn: sql.NullString{String: `["volume.data"]`, Valid: true}},
	}
	ordered, err := managedDeletionOrder(resources)
	if err != nil {
		t.Fatal(err)
	}
	addresses := make([]string, 0, len(ordered))
	for _, resource := range ordered {
		addresses = append(addresses, resource.Address)
	}
	if !reflect.DeepEqual(addresses, []string{"job.web", "volume.data", "acl_policy.web"}) {
		t.Fatalf("deletion order = %v", addresses)
	}
	_, err = managedDeletionOrder([]storage.ManagedResource{
		{Address: "volume.data", Kind: "volume", DependsOn: sql.NullString{String: "not-json", Valid: true}},
		{Address: "job.web", Kind: "job"},
	})
	if err == nil {
		t.Fatal("expected malformed dependency metadata to fail closed")
	}
}

func TestUnscheduleManagedResourcesDeletesDependentsFirst(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, fake := newBundleManager(t)
	resources := []storage.ManagedResource{
		{RepoID: repoRecord.ID, Address: "acl_policy.web", Kind: "acl_policy", NomadID: sql.NullString{String: "web", Valid: true}, DeleteMode: "allow"},
		{RepoID: repoRecord.ID, Address: "volume.data", Kind: "volume", NomadID: sql.NullString{String: "data", Valid: true}, DeleteMode: "allow", Subtype: sql.NullString{String: "host", Valid: true}, DependsOn: sql.NullString{String: `["acl_policy.web"]`, Valid: true}},
		{RepoID: repoRecord.ID, Address: "job.web", Kind: "job", NomadID: sql.NullString{String: "web", Valid: true}, DeleteMode: "allow", DependsOn: sql.NullString{String: `["volume.data"]`, Valid: true}},
	}
	for _, resource := range resources {
		if err := managed.Upsert(ctx, storage.ManagedResourceInput{RepoID: resource.RepoID, Address: resource.Address, Kind: resource.Kind, NomadID: resource.NomadID.String, DeleteMode: resource.DeleteMode, Subtype: resource.Subtype.String, DependsOn: resource.DependsOn.String, Status: "applied"}); err != nil {
			t.Fatalf("upsert %s: %v", resource.Address, err)
		}
	}
	if err := manager.unscheduleManagedResources(ctx, repoRecord.ID); err != nil {
		t.Fatalf("unschedule resources: %v", err)
	}
	if !reflect.DeepEqual(fake.deregistered, []string{"web"}) || !reflect.DeepEqual(fake.resourceCalls, []string{"delete-job:web", "delete-volume:data", "delete-policy:web"}) {
		t.Fatalf("deletion calls were not dependency ordered: jobs=%v resources=%v", fake.deregistered, fake.resourceCalls)
	}
}

func TestAdoptBundleResourceRecordsMatchingHostVolume(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, fake := newBundleManager(t)
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    name = "data"
    type = "host"
    plugin_id = "mkdir"
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	fake.hostVolume = &api.HostVolume{ID: "host-volume-id", Name: "data", PluginID: "mkdir"}
	if err := manager.adoptBundleResource(ctx, repoRecord.ID, &repomodel.Snapshot{CommitHash: "commit-1", Bundle: bundle}, "volume.data"); err != nil {
		t.Fatalf("adopt host volume: %v", err)
	}
	resources, err := managed.ListByRepo(ctx, repoRecord.ID)
	if err != nil || len(resources) != 1 || resources[0].NomadID.String != "host-volume-id" {
		t.Fatalf("unexpected adopted resource: %v %#v", err, resources)
	}
	if len(fake.resourceCalls) != 0 {
		t.Fatalf("adoption mutated Nomad: %v", fake.resourceCalls)
	}
}

func TestAdoptBundleResourceRejectsMismatchedHostVolume(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, fake := newBundleManager(t)
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    name = "data"
    type = "host"
    plugin_id = "mkdir"
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	fake.hostVolume = &api.HostVolume{ID: "host-volume-id", Name: "data", PluginID: "other"}
	if err := manager.adoptBundleResource(ctx, repoRecord.ID, &repomodel.Snapshot{CommitHash: "commit-1", Bundle: bundle}, "volume.data"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected mismatch error, got %v", err)
	}
	resources, err := managed.ListByRepo(ctx, repoRecord.ID)
	if err != nil || len(resources) != 0 {
		t.Fatalf("mismatched resource was adopted: %v %#v", err, resources)
	}
}

func TestAdoptBundleResourceRecordsMatchingJob(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, fake := newBundleManager(t)
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "job" "worker" {
    datacenters = ["dc1"]
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	fake.jobStatuses = map[string]*nomadclient.JobStatus{"worker": {ID: "worker", Exists: true}}
	fake.planResponses = map[string]*api.JobPlanResponse{"worker": {}}
	if err := manager.adoptBundleResource(ctx, repoRecord.ID, &repomodel.Snapshot{CommitHash: "commit-1", Bundle: bundle}, "job.worker"); err != nil {
		t.Fatalf("adopt job: %v", err)
	}
	resources, err := managed.ListByRepo(ctx, repoRecord.ID)
	if err != nil || len(resources) != 1 || resources[0].NomadID.String != "worker" {
		t.Fatalf("unexpected adopted job: %v %#v", err, resources)
	}
	if fake.registerCalls != 0 {
		t.Fatalf("adoption registered a job: %d", fake.registerCalls)
	}
}

func TestAdoptBundleResourceRejectsJobDrift(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, fake := newBundleManager(t)
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "job" "worker" {
    datacenters = ["dc1"]
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	fake.jobStatuses = map[string]*nomadclient.JobStatus{"worker": {ID: "worker", Exists: true}}
	fake.planResponses = map[string]*api.JobPlanResponse{"worker": {Diff: &api.JobDiff{Fields: []*api.FieldDiff{{Name: "datacenters", Old: "dc1", New: "dc2"}}}}}
	if err := manager.adoptBundleResource(ctx, repoRecord.ID, &repomodel.Snapshot{CommitHash: "commit-1", Bundle: bundle}, "job.worker"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected job mismatch error, got %v", err)
	}
	resources, err := managed.ListByRepo(ctx, repoRecord.ID)
	if err != nil || len(resources) != 0 {
		t.Fatalf("drifted job was adopted: %v %#v", err, resources)
	}
}

func TestBundleJobIdentitySurvivesBundlePathChange(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, fake := newBundleManager(t)

	first, err := manifest.Parse([]byte(`bundle "compass" {
  resource "job" "api" {
    datacenters = ["dc1"]
  }
}`), ".nomad/compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse first bundle: %v", err)
	}
	if err := manager.ensureBundle(ctx, repoRecord, &repomodel.Snapshot{CommitHash: "commit-1", Bundle: first}, true); err != nil {
		t.Fatalf("ensure first bundle: %v", err)
	}

	second, err := manifest.Parse([]byte(`bundle "compass" {
  resource "job" "api" {
    datacenters = ["dc1"]
  }
}`), ".nomad/compass.hcl")
	if err != nil {
		t.Fatalf("parse second bundle: %v", err)
	}
	if err := manager.ensureBundle(ctx, repoRecord, &repomodel.Snapshot{CommitHash: "commit-2", Bundle: second}, true); err != nil {
		t.Fatalf("ensure second bundle: %v", err)
	}

	if fake.registerCalls != 1 {
		t.Fatalf("expected stable bundle job to be planned without re-registering, got %d registrations", fake.registerCalls)
	}
	tracked, err := managed.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		t.Fatalf("list managed resources: %v", err)
	}
	if len(tracked) != 1 || tracked[0].Address != "job.api" {
		t.Fatalf("unexpected managed bundle jobs: %#v", tracked)
	}
	if tracked[0].SourcePath != ".nomad/compass.hcl" {
		t.Fatalf("expected source path to update without changing identity, got %q", tracked[0].SourcePath)
	}
}

func TestBundleVolumeChangesRequireExplicitReplacement(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, _, fake := newBundleManager(t)

	first, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    name = "compass-data"
    type = "host"
    plugin_id = "mkdir"
    capability {
      access_mode = "single-node-single-writer"
      attachment_mode = "file-system"
    }
  }
}`), ".nomad/compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse first volume: %v", err)
	}
	if err := manager.ensureBundle(ctx, repoRecord, &repomodel.Snapshot{CommitHash: "commit-1", Bundle: first}, true); err != nil {
		t.Fatalf("ensure first volume: %v", err)
	}

	changed, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    name = "renamed-data"
    type = "host"
    plugin_id = "mkdir"
    capability {
      access_mode = "single-node-single-writer"
      attachment_mode = "file-system"
    }
  }
}`), ".nomad/compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse changed volume: %v", err)
	}
	if err := manager.ensureBundle(ctx, repoRecord, &repomodel.Snapshot{CommitHash: "commit-2", Bundle: changed}, true); err == nil {
		t.Fatal("expected protected volume change to fail")
	}
	if slices.Contains(fake.resourceCalls, "delete-volume:host-volume-id") {
		t.Fatalf("protected volume was deleted: %v", fake.resourceCalls)
	}

	allowed, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    delete = "allow"
    name = "renamed-data"
    type = "host"
    plugin_id = "mkdir"
    capability {
      access_mode = "single-node-single-writer"
      attachment_mode = "file-system"
    }
  }
}`), ".nomad/compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse allowed volume: %v", err)
	}
	if err := manager.ensureBundle(ctx, repoRecord, &repomodel.Snapshot{CommitHash: "commit-3", Bundle: allowed}, true); err != nil {
		t.Fatalf("ensure explicitly replaceable volume: %v", err)
	}
	if !slices.Contains(fake.resourceCalls, "delete-volume:host-volume-id") {
		t.Fatalf("expected explicit volume replacement, got %v", fake.resourceCalls)
	}
}

func TestEnsureBundleValidatesBeforeApplying(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, _, fake := newBundleManager(t)
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    name = "compass-data"
    type = "host"
  }
  resource "acl_policy" "invalid" {
    description = "missing rules"
  }
}`), ".nomad/compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	if err := manager.ensureBundle(ctx, repoRecord, &repomodel.Snapshot{CommitHash: "commit-1", Bundle: bundle}, true); err == nil {
		t.Fatal("expected invalid bundle validation to fail")
	}
	if len(fake.resourceCalls) != 0 {
		t.Fatalf("expected no Nomad mutations before validation, got %v", fake.resourceCalls)
	}
}

func TestBundleRejectsUnmanagedACLPolicyCollision(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, _, fake := newBundleManager(t)
	fake.aclPolicy = &api.ACLPolicy{Name: "existing"}
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "acl_policy" "existing" {
    rules {
      namespace "default" { capabilities = ["read-job"] }
    }
  }
}`), ".nomad/compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	if err := manager.ensureBundle(ctx, repoRecord, &repomodel.Snapshot{CommitHash: "commit-1", Bundle: bundle}, true); err == nil {
		t.Fatal("expected unmanaged ACL policy collision to fail")
	}
	if slices.Contains(fake.resourceCalls, "policy:existing") {
		t.Fatalf("unmanaged ACL policy was overwritten: %v", fake.resourceCalls)
	}
}

func TestApplyJobAddsMetadata(t *testing.T) {
	fake := &fakeNomad{}
	m := &Manager{nomad: fake}

	repo := &storage.Repository{RepoURL: "git@example.com/foo.git", Name: "foo"}
	snapshot := &repomodel.Snapshot{CommitHash: "abc123", CommitAuthor: "Tester <test@example.com>", CommitTitle: "Initial"}
	jobFile := repomodel.JobFile{Path: ".nomad/job.nomad.hcl", Content: []byte(`job "demo" { datacenters = ["dc1"] }`)}

	job, submission, err := parseJob(jobFile.Path, jobFile.Content)
	if err != nil {
		t.Fatalf("parse job: %v", err)
	}

	id, err := m.applyJob(context.Background(), repo, jobFile, snapshot, job, submission)
	if err != nil {
		t.Fatalf("apply job: %v", err)
	}

	if fake.lastJob == nil {
		t.Fatal("expected job to be registered")
	}
	if fake.lastSubmission == nil {
		t.Fatal("expected submission to be captured")
	}
	if fake.lastSubmission.Format != "hcl2" {
		t.Fatalf("unexpected submission format: %s", fake.lastSubmission.Format)
	}
	if fake.lastSubmission.Source != string(jobFile.Content) {
		t.Fatalf("unexpected submission source: %q", fake.lastSubmission.Source)
	}

	if id == "" {
		t.Fatal("expected job id")
	}

	meta := fake.lastJob.Meta
	cases := map[string]string{
		compassMetaRepoURL:      repo.RepoURL,
		compassMetaRepoName:     repo.Name,
		compassMetaJobFile:      jobFile.Path,
		compassMetaCommit:       snapshot.CommitHash,
		compassMetaCommitAuthor: snapshot.CommitAuthor,
		compassMetaCommitTitle:  snapshot.CommitTitle,
	}

	for key, want := range cases {
		if got := meta[key]; got != want {
			t.Fatalf("meta[%q]=%q want %q", key, got, want)
		}
	}
}

func TestEnsureJobsRemovesDeletedJobs(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)

	repoRecord, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	if err := fileStore.Upsert(ctx, repoRecord.ID, ".nomad/removed.nomad.hcl", "old", "demo-job"); err != nil {
		t.Fatalf("upsert repo file: %v", err)
	}

	fake := &fakeNomad{}
	m := &Manager{
		files:  fileStore,
		nomad:  fake,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	snapshot := &repomodel.Snapshot{JobFiles: nil, CommitHash: "new"}
	if err := m.ensureJobs(ctx, repoRecord, snapshot, true); err != nil {
		t.Fatalf("ensure jobs: %v", err)
	}

	if len(fake.deregistered) != 1 || fake.deregistered[0] != "demo-job" {
		t.Fatalf("expected job deregistered, got %v", fake.deregistered)
	}

	files, err := fileStore.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		t.Fatalf("list repo files: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected repo file tracking removed, got %v", files)
	}
}

func TestEnsureJobsSkipsUnchangedJobs(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)

	repoRecord, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	jobContent := []byte(`job "demo" { datacenters = ["dc1"] }`)
	jobPath := ".nomad/demo.nomad.hcl"
	if err := fileStore.Upsert(ctx, repoRecord.ID, jobPath, "old", "demo"); err != nil {
		t.Fatalf("upsert repo file: %v", err)
	}

	fake := &fakeNomad{
		lastJob: &api.Job{ID: strPtr("demo"), Name: strPtr("demo")},
		planResponses: map[string]*api.JobPlanResponse{
			"demo": {},
		},
	}
	m := &Manager{
		files:  fileStore,
		nomad:  fake,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	snapshot := &repomodel.Snapshot{
		CommitHash: "new",
		JobFiles: []repomodel.JobFile{{
			Path:    jobPath,
			Content: jobContent,
		}},
	}

	if err := m.ensureJobs(ctx, repoRecord, snapshot, true); err != nil {
		t.Fatalf("ensure jobs: %v", err)
	}

	if fake.registerCalls != 0 {
		t.Fatalf("expected no job re-registrations, got %d", fake.registerCalls)
	}

	files, err := fileStore.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		t.Fatalf("list repo files: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected repo file tracking retained, got %v", files)
	}
	file := files[0]
	if !file.LastCommit.Valid || file.LastCommit.String != "new" {
		t.Fatalf("expected last commit updated to new, got %+v", file.LastCommit)
	}
}

func TestEnsureJobsReappliesChangedJobs(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)

	repoRecord, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	jobPath := ".nomad/demo.nomad.hcl"
	if err := fileStore.Upsert(ctx, repoRecord.ID, jobPath, "old", "demo"); err != nil {
		t.Fatalf("upsert repo file: %v", err)
	}

	fake := &fakeNomad{
		lastJob: &api.Job{ID: strPtr("demo"), Name: strPtr("demo")},
		planResponses: map[string]*api.JobPlanResponse{
			"demo": {
				Diff: &api.JobDiff{Fields: []*api.FieldDiff{{Name: "datacenters", Old: "[\"dc1\"]", New: "[\"dc1\",\"dc2\"]"}}},
			},
		},
	}
	m := &Manager{
		files:  fileStore,
		nomad:  fake,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	updatedContent := []byte(`job "demo" {
  datacenters = ["dc1"]
  meta = { version = "2" }
}`)
	snapshot := &repomodel.Snapshot{
		CommitHash: "new",
		JobFiles: []repomodel.JobFile{{
			Path:    jobPath,
			Content: updatedContent,
		}},
	}

	if err := m.ensureJobs(ctx, repoRecord, snapshot, true); err != nil {
		t.Fatalf("ensure jobs: %v", err)
	}

	if fake.registerCalls != 1 {
		t.Fatalf("expected job re-registered once, got %d (last submission: %#v)", fake.registerCalls, fake.lastSubmission)
	}
	if fake.lastSubmission == nil || fake.lastSubmission.Source != string(updatedContent) {
		t.Fatalf("expected submission with updated content, got %#v", fake.lastSubmission)
	}

	files, err := fileStore.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		t.Fatalf("list repo files: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected repo file tracking retained, got %v", files)
	}
	file := files[0]
	if !file.LastCommit.Valid || file.LastCommit.String != "new" {
		t.Fatalf("expected last commit updated to new, got %+v", file.LastCommit)
	}
}

func TestEnsureJobsOnlyReappliesChangedJobAcrossRepo(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)

	repoRecord, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	changedPath := ".nomad/changed.nomad.hcl"
	unchangedPath := ".nomad/unchanged.nomad.hcl"
	if err := fileStore.Upsert(ctx, repoRecord.ID, changedPath, "old", "job-changed"); err != nil {
		t.Fatalf("upsert changed repo file: %v", err)
	}
	if err := fileStore.Upsert(ctx, repoRecord.ID, unchangedPath, "old", "job-unchanged"); err != nil {
		t.Fatalf("upsert unchanged repo file: %v", err)
	}

	fake := &fakeNomad{
		planResponses: map[string]*api.JobPlanResponse{
			"job-changed": {
				Diff: &api.JobDiff{Fields: []*api.FieldDiff{{Name: "task.env.NEW_VAR", Old: "", New: "1"}}},
			},
			"job-unchanged": {},
		},
		jobStatuses: map[string]*nomadclient.JobStatus{
			"job-changed":   {ID: "job-changed", Exists: true, Status: "running"},
			"job-unchanged": {ID: "job-unchanged", Exists: true, Status: "running"},
		},
	}
	m := &Manager{
		files:  fileStore,
		nomad:  fake,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	snapshot := &repomodel.Snapshot{
		CommitHash: "new",
		JobFiles: []repomodel.JobFile{
			{
				Path: changedPath,
				Content: []byte(`job "job-changed" {
  datacenters = ["dc1"]
  group "g" {
    task "t" {
      driver = "docker"
      config { image = "alpine:3.19" }
      env { NEW_VAR = "1" }
    }
  }
}`),
			},
			{
				Path: unchangedPath,
				Content: []byte(`job "job-unchanged" {
  datacenters = ["dc1"]
}`),
			},
		},
	}

	if err := m.ensureJobs(ctx, repoRecord, snapshot, true); err != nil {
		t.Fatalf("ensure jobs: %v", err)
	}

	if fake.registerCalls != 1 {
		t.Fatalf("expected only changed job to be re-registered, got %d", fake.registerCalls)
	}
	if len(fake.registeredJobIDs) != 1 || fake.registeredJobIDs[0] != "job-changed" {
		t.Fatalf("expected job-changed to be the only re-registered job, got %v", fake.registeredJobIDs)
	}

	files, err := fileStore.ListByRepo(ctx, repoRecord.ID)
	if err != nil {
		t.Fatalf("list repo files: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected two tracked files, got %d", len(files))
	}
	for _, file := range files {
		if !file.LastCommit.Valid || file.LastCommit.String != "new" {
			t.Fatalf("expected updated last commit for %s, got %+v", file.Path, file.LastCommit)
		}
	}
}

func TestEnsureJobsIgnoresCompassCommitMetadataDiffsAcrossRepo(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)

	repoRecord, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	paths := map[string]string{
		"job-a": ".nomad/a.nomad.hcl",
		"job-b": ".nomad/b.nomad.hcl",
		"job-c": ".nomad/c.nomad.hcl",
	}
	for jobID, path := range paths {
		if err := fileStore.Upsert(ctx, repoRecord.ID, path, "old", jobID); err != nil {
			t.Fatalf("upsert %s repo file: %v", jobID, err)
		}
	}

	commitMetadataDiff := &api.JobDiff{Fields: []*api.FieldDiff{
		{Name: nomadMetaFieldName(compassMetaCommit), Old: "old", New: "new"},
		{Name: nomadMetaFieldName(compassMetaCommitAuthor), Old: "Old <old@example.com>", New: "New <new@example.com>"},
		{Name: nomadMetaFieldName(compassMetaCommitTitle), Old: "Initial", New: "Update job B"},
	}}
	fake := &fakeNomad{
		planResponses: map[string]*api.JobPlanResponse{
			"job-a": {Diff: commitMetadataDiff},
			"job-b": {Diff: &api.JobDiff{Fields: []*api.FieldDiff{
				{Name: nomadMetaFieldName(compassMetaCommit), Old: "old", New: "new"},
				{Name: nomadMetaFieldName(compassMetaCommitAuthor), Old: "Old <old@example.com>", New: "New <new@example.com>"},
				{Name: nomadMetaFieldName(compassMetaCommitTitle), Old: "Initial", New: "Update job B"},
				{Name: "TaskGroups[web].Tasks[app].Config.image", Old: "alpine:3.18", New: "alpine:3.19"},
			}}},
			"job-c": {Diff: commitMetadataDiff},
		},
		jobStatuses: map[string]*nomadclient.JobStatus{
			"job-a": {ID: "job-a", Exists: true, Status: "running"},
			"job-b": {ID: "job-b", Exists: true, Status: "running"},
			"job-c": {ID: "job-c", Exists: true, Status: "running"},
		},
	}
	m := &Manager{
		files:  fileStore,
		nomad:  fake,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	snapshot := &repomodel.Snapshot{
		CommitHash:   "new",
		CommitAuthor: "New <new@example.com>",
		CommitTitle:  "Update job B",
		JobFiles: []repomodel.JobFile{
			{Path: paths["job-a"], Content: []byte(`job "job-a" { datacenters = ["dc1"] }`)},
			{Path: paths["job-b"], Content: []byte(`job "job-b" {
  datacenters = ["dc1"]
  group "web" {
    task "app" {
      driver = "docker"
      config { image = "alpine:3.19" }
    }
  }
}`)},
			{Path: paths["job-c"], Content: []byte(`job "job-c" { datacenters = ["dc1"] }`)},
		},
	}

	if err := m.ensureJobs(ctx, repoRecord, snapshot, true); err != nil {
		t.Fatalf("ensure jobs: %v", err)
	}

	if fake.registerCalls != 1 {
		t.Fatalf("expected only job with non-metadata diff to be re-registered, got %d", fake.registerCalls)
	}
	if len(fake.registeredJobIDs) != 1 || fake.registeredJobIDs[0] != "job-b" {
		t.Fatalf("expected job-b to be the only re-registered job, got %v", fake.registeredJobIDs)
	}
}

func TestEnsureJobsPlanUsesStableAnnotations(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)

	repoRecord, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	jobPath := ".nomad/demo.nomad.hcl"
	if err := fileStore.Upsert(ctx, repoRecord.ID, jobPath, "old", "demo"); err != nil {
		t.Fatalf("upsert repo file: %v", err)
	}

	metadataPlanned := false
	fake := &fakeNomad{
		lastJob: &api.Job{ID: strPtr("demo"), Name: strPtr("demo")},
		planFn: func(job *api.Job) (*api.JobPlanResponse, error) {
			if job != nil &&
				job.Meta[compassMetaRepoURL] == "https://example.com/demo.git" &&
				job.Meta[compassMetaRepoName] == "demo" &&
				job.Meta[compassMetaJobFile] == jobPath &&
				job.Meta[compassMetaCommit] == "" &&
				job.Meta[compassMetaCommitAuthor] == "" &&
				job.Meta[compassMetaCommitTitle] == "" {
				metadataPlanned = true
				return &api.JobPlanResponse{}, nil
			}
			return &api.JobPlanResponse{
				Diff: &api.JobDiff{
					Fields: []*api.FieldDiff{{Name: "Meta", Old: "annotated", New: "missing"}},
				},
			}, nil
		},
	}
	m := &Manager{
		files:  fileStore,
		nomad:  fake,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	snapshot := &repomodel.Snapshot{
		CommitHash:   "new",
		CommitAuthor: "Tester <test@example.com>",
		CommitTitle:  "No-op commit",
		JobFiles: []repomodel.JobFile{{
			Path:    jobPath,
			Content: []byte(`job "demo" { datacenters = ["dc1"] }`),
		}},
	}

	if err := m.ensureJobs(ctx, repoRecord, snapshot, true); err != nil {
		t.Fatalf("ensure jobs: %v", err)
	}

	if !metadataPlanned {
		t.Fatalf("expected plan call to include injected metadata")
	}
	if fake.registerCalls != 0 {
		t.Fatalf("expected unchanged job not to be re-registered, got %d", fake.registerCalls)
	}
}

func TestEnsureJobsReappliesOnStatusErrorWhenCommitChanged(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)

	repoRecord, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	jobPath := ".nomad/demo.nomad.hcl"
	if err := fileStore.Upsert(ctx, repoRecord.ID, jobPath, "old", "demo"); err != nil {
		t.Fatalf("upsert repo file: %v", err)
	}

	fake := &fakeNomad{
		lastJob:       &api.Job{ID: strPtr("demo"), Name: strPtr("demo")},
		jobStatusErr:  errors.New("nomad read denied"),
		planResponses: map[string]*api.JobPlanResponse{"demo": {}},
	}
	m := &Manager{
		files:  fileStore,
		nomad:  fake,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	updatedContent := []byte(`job "demo" {
  datacenters = ["dc1"]
  meta = { version = "2" }
}`)
	snapshot := &repomodel.Snapshot{
		CommitHash: "new",
		JobFiles: []repomodel.JobFile{{
			Path:    jobPath,
			Content: updatedContent,
		}},
	}

	if err := m.ensureJobs(ctx, repoRecord, snapshot, true); err != nil {
		t.Fatalf("ensure jobs: %v", err)
	}

	if fake.registerCalls != 1 {
		t.Fatalf("expected job re-registered when status check fails on commit change, got %d", fake.registerCalls)
	}
	if fake.planCalls != 0 {
		t.Fatalf("expected no plan calls when status check forces apply, got %d", fake.planCalls)
	}
}

type fakeNomad struct {
	lastJob          *api.Job
	lastSubmission   *api.JobSubmission
	registeredJobIDs []string
	deregistered     []string
	registerCalls    int
	planResponses    map[string]*api.JobPlanResponse
	planErr          error
	planFn           func(job *api.Job) (*api.JobPlanResponse, error)
	planCalls        int
	jobStatusErr     error
	jobStatuses      map[string]*nomadclient.JobStatus
	hostVolume       *api.HostVolume
	aclPolicy        *api.ACLPolicy
	namespace        *api.Namespace
	quota            *api.QuotaSpec
	variable         *api.Variable
	sentinelPolicy   *api.SentinelPolicy
	resourceCalls    []string
}

func strPtr(s string) *string {
	return &s
}

func (f *fakeNomad) RegisterJob(_ context.Context, job *api.Job, submission *api.JobSubmission) error {
	f.resourceCalls = append(f.resourceCalls, "job:"+jobID(job))
	f.lastJob = job
	f.lastSubmission = submission
	if id := jobID(job); id != "" {
		f.registeredJobIDs = append(f.registeredJobIDs, id)
	}
	f.registerCalls++
	return nil
}

func (f *fakeNomad) DeregisterJob(_ context.Context, jobID string, _ bool) error {
	f.resourceCalls = append(f.resourceCalls, "delete-job:"+jobID)
	if f.lastJob != nil && f.lastJob.ID != nil && *f.lastJob.ID == jobID {
		f.lastJob = nil
	}
	f.deregistered = append(f.deregistered, jobID)
	return nil
}

func (f *fakeNomad) Ping(context.Context) error {
	return nil
}

func (f *fakeNomad) JobStatus(_ context.Context, jobID string) (*nomadclient.JobStatus, error) {
	if f.jobStatusErr != nil {
		return nil, f.jobStatusErr
	}
	if f.jobStatuses != nil {
		if status, ok := f.jobStatuses[jobID]; ok {
			return status, nil
		}
		return &nomadclient.JobStatus{ID: jobID, Exists: false}, nil
	}
	if f.lastJob == nil || f.lastJob.ID == nil || *f.lastJob.ID != jobID {
		return &nomadclient.JobStatus{ID: jobID, Exists: false}, nil
	}

	var name string
	if f.lastJob.Name != nil {
		name = *f.lastJob.Name
	}

	return &nomadclient.JobStatus{
		ID:     jobID,
		Name:   name,
		Status: "running",
		Exists: true,
	}, nil
}

func (f *fakeNomad) ApplyHostVolume(_ context.Context, volume *api.HostVolume) (*api.HostVolume, error) {
	f.resourceCalls = append(f.resourceCalls, "volume:"+volume.Name)
	copy := *volume
	if copy.ID == "" {
		copy.ID = "host-volume-id"
	}
	f.hostVolume = &copy
	return &copy, nil
}

func (f *fakeNomad) FindHostVolume(_ context.Context, name, _ string) (*api.HostVolume, error) {
	if f.hostVolume == nil || f.hostVolume.Name != name {
		return nil, nil
	}
	return f.hostVolume, nil
}

func (f *fakeNomad) FindCSIVolume(context.Context, string, string) (*api.CSIVolume, error) {
	return nil, nil
}

func (f *fakeNomad) ObserveHostVolume(_ context.Context, id, _ string) (*api.HostVolume, error) {
	if f.hostVolume == nil || f.hostVolume.ID != id {
		return nil, nil
	}
	return f.hostVolume, nil
}

func (f *fakeNomad) DeleteHostVolume(_ context.Context, id, _ string, _ bool) error {
	f.resourceCalls = append(f.resourceCalls, "delete-volume:"+id)
	f.hostVolume = nil
	return nil
}

func (f *fakeNomad) ApplyCSIVolume(_ context.Context, volume *api.CSIVolume) (*api.CSIVolume, error) {
	return volume, nil
}

func (f *fakeNomad) ObserveCSIVolume(context.Context, string, string) (*api.CSIVolume, error) {
	return nil, nil
}

func (f *fakeNomad) DeleteCSIVolume(context.Context, string, string, bool) error {
	return nil
}

func (f *fakeNomad) ApplyACLPolicy(_ context.Context, policy *api.ACLPolicy) error {
	f.resourceCalls = append(f.resourceCalls, "policy:"+policy.Name)
	copy := *policy
	f.aclPolicy = &copy
	return nil
}

func (f *fakeNomad) ObserveACLPolicy(_ context.Context, name string) (*api.ACLPolicy, error) {
	if f.aclPolicy == nil || f.aclPolicy.Name != name {
		return nil, nil
	}
	return f.aclPolicy, nil
}

func (f *fakeNomad) DeleteACLPolicy(_ context.Context, name string) error {
	f.resourceCalls = append(f.resourceCalls, "delete-policy:"+name)
	f.aclPolicy = nil
	return nil
}

func (f *fakeNomad) ApplyNamespace(_ context.Context, namespace *api.Namespace) error {
	f.resourceCalls = append(f.resourceCalls, "namespace:"+namespace.Name)
	copy := *namespace
	f.namespace = &copy
	return nil
}

func (f *fakeNomad) ObserveNamespace(_ context.Context, name string) (*api.Namespace, error) {
	if f.namespace == nil || f.namespace.Name != name {
		return nil, nil
	}
	return f.namespace, nil
}

func (f *fakeNomad) DeleteNamespace(_ context.Context, name string) error {
	f.resourceCalls = append(f.resourceCalls, "delete-namespace:"+name)
	f.namespace = nil
	return nil
}

func (f *fakeNomad) ApplyQuota(_ context.Context, quota *api.QuotaSpec) error {
	f.resourceCalls = append(f.resourceCalls, "quota:"+quota.Name)
	copy := *quota
	f.quota = &copy
	return nil
}

func (f *fakeNomad) ObserveQuota(_ context.Context, name string) (*api.QuotaSpec, error) {
	if f.quota == nil || f.quota.Name != name {
		return nil, nil
	}
	return f.quota, nil
}

func (f *fakeNomad) DeleteQuota(_ context.Context, name string) error {
	f.resourceCalls = append(f.resourceCalls, "delete-quota:"+name)
	f.quota = nil
	return nil
}

func (f *fakeNomad) ApplyVariable(_ context.Context, variable *api.Variable) (*api.Variable, error) {
	f.resourceCalls = append(f.resourceCalls, "variable:"+variable.Path)
	copy := *variable
	f.variable = &copy
	return &copy, nil
}

func (f *fakeNomad) ObserveVariable(_ context.Context, _, path string) (*api.Variable, error) {
	if f.variable == nil || f.variable.Path != path {
		return nil, nil
	}
	return f.variable, nil
}

func (f *fakeNomad) DeleteVariable(_ context.Context, _, path string) error {
	f.resourceCalls = append(f.resourceCalls, "delete-variable:"+path)
	f.variable = nil
	return nil
}

func (f *fakeNomad) ApplySentinelPolicy(_ context.Context, policy *api.SentinelPolicy) error {
	f.resourceCalls = append(f.resourceCalls, "sentinel:"+policy.Name)
	copy := *policy
	f.sentinelPolicy = &copy
	return nil
}

func (f *fakeNomad) ObserveSentinelPolicy(_ context.Context, name string) (*api.SentinelPolicy, error) {
	if f.sentinelPolicy == nil || f.sentinelPolicy.Name != name {
		return nil, nil
	}
	return f.sentinelPolicy, nil
}

func (f *fakeNomad) DeleteSentinelPolicy(_ context.Context, name string) error {
	f.resourceCalls = append(f.resourceCalls, "delete-sentinel:"+name)
	f.sentinelPolicy = nil
	return nil
}

func (f *fakeNomad) PlanJob(_ context.Context, job *api.Job) (*api.JobPlanResponse, error) {
	f.planCalls++
	if f.planErr != nil {
		return nil, f.planErr
	}
	if f.planFn != nil {
		return f.planFn(job)
	}
	if job == nil || job.ID == nil {
		return nil, errors.New("job id required")
	}
	if resp, ok := f.planResponses[*job.ID]; ok {
		return resp, nil
	}
	return &api.JobPlanResponse{}, nil
}
