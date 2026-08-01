package reconcile

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	repomodel "github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestAdoptionRejectsNomadIdentityOwnedByAnotherRepository(t *testing.T) {
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
		t.Fatal(err)
	}
	if err := managed.Upsert(ctx, storage.ManagedResourceInput{RepoID: repoRecord.ID + 1, Address: "volume.other", Kind: "volume", NomadID: "host-volume-id", Namespace: "default", Status: "applied", DeleteMode: string(manifest.DeleteModeAllow), Subtype: "host"}); err != nil {
		t.Fatal(err)
	}
	fake.hostVolume = &api.HostVolume{ID: "host-volume-id", Name: "data", PluginID: "mkdir"}
	if err := manager.adoptBundleResource(ctx, repoRecord.ID, &repomodel.Snapshot{CommitHash: "commit", Bundle: bundle}, "volume.data"); err == nil {
		t.Fatal("expected adoption ownership collision")
	}
}

func TestRepositoryModeSwitchesFailClosedBeforeMutation(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, _ := newBundleManager(t)
	if err := manager.files.Upsert(ctx, repoRecord.ID, "job.nomad", "commit", "demo"); err != nil {
		t.Fatal(err)
	}
	if err := manager.validateRepositoryMode(ctx, repoRecord.ID, true); err == nil {
		t.Fatal("expected legacy-to-bundle transition to be rejected")
	}
	if err := manager.files.DeleteByRepo(ctx, repoRecord.ID); err != nil {
		t.Fatal(err)
	}
	if err := managed.Upsert(ctx, storage.ManagedResourceInput{RepoID: repoRecord.ID, Address: "namespace.apps", Kind: "namespace", Status: "applied", DeleteMode: string(manifest.DeleteModeProtect)}); err != nil {
		t.Fatal(err)
	}
	if err := manager.validateRepositoryMode(ctx, repoRecord.ID, false); err == nil {
		t.Fatal("expected bundle-to-legacy transition to be rejected")
	}
}

func TestDeleteProtectedResourcePerformsExplicitNomadCleanup(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, fake := newBundleManager(t)
	if err := managed.Upsert(ctx, storage.ManagedResourceInput{RepoID: repoRecord.ID, Address: "volume.data", Kind: "volume", NomadID: "data", Subtype: "host", Status: "protected", DeleteMode: string(manifest.DeleteModeProtect)}); err != nil {
		t.Fatal(err)
	}
	fake.hostVolume = &api.HostVolume{ID: "data", Name: "data", PluginID: "mkdir"}
	if err := manager.DeleteProtectedResource(ctx, repoRecord.ID, "volume.data"); err != nil {
		t.Fatalf("delete protected volume: %v", err)
	}
	if len(fake.resourceCalls) != 1 || fake.resourceCalls[0] != "delete-volume:data" {
		t.Fatalf("cleanup calls = %v", fake.resourceCalls)
	}
	remaining, err := managed.ListByRepo(ctx, repoRecord.ID)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("protected metadata remains: %#v, %v", remaining, err)
	}
}

func TestDeleteRepositoryUnschedulesJobsBeforeRemovingMetadata(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, fake := newBundleManager(t)
	manager.git = repomodel.NewManager(filepath.Join(t.TempDir(), "clones"))
	if err := manager.files.Upsert(ctx, repoRecord.ID, "job.nomad", "commit", "demo"); err != nil {
		t.Fatal(err)
	}
	fake.hostVolume = &api.HostVolume{ID: "data", Name: "data", PluginID: "mkdir"}
	if err := managed.Upsert(ctx, storage.ManagedResourceInput{RepoID: repoRecord.ID, Address: "volume.data", Kind: "volume", NomadID: "data", Subtype: "host", Status: "applied", DeleteMode: string(manifest.DeleteModeAllow)}); err != nil {
		t.Fatal(err)
	}
	if err := manager.DeleteRepository(ctx, repoRecord.ID, true); err != nil {
		t.Fatalf("delete repository: %v", err)
	}
	if len(fake.deregistered) != 1 || fake.deregistered[0] != "demo" || len(fake.resourceCalls) != 2 || fake.resourceCalls[0] != "delete-job:demo" || fake.resourceCalls[1] != "delete-volume:data" {
		t.Fatalf("resources were not unscheduled: jobs=%v resources=%v", fake.deregistered, fake.resourceCalls)
	}
	if repo, err := manager.repos.Get(ctx, repoRecord.ID); err != nil || repo != nil {
		t.Fatalf("repository remains: %#v, %v", repo, err)
	}
	if files, err := manager.files.ListByRepo(ctx, repoRecord.ID); err != nil || len(files) != 0 {
		t.Fatalf("repo files remain: %#v, %v", files, err)
	}
}

func TestDeleteRepositoryRequiresProtectedResourcesToBeResolved(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, _ := newBundleManager(t)
	if err := managed.Upsert(ctx, storage.ManagedResourceInput{RepoID: repoRecord.ID, Address: "volume.data", Kind: "volume", Status: "protected", DeleteMode: string(manifest.DeleteModeProtect)}); err != nil {
		t.Fatal(err)
	}
	if protected, err := manager.ListProtectedResources(ctx, repoRecord.ID); err != nil || len(protected) != 1 {
		t.Fatalf("actionable protected resources = %#v, %v", protected, err)
	}
	if err := manager.DeleteRepository(ctx, repoRecord.ID, true); err == nil {
		t.Fatal("expected protected-resource deletion guard")
	}

	manager, repoRecord, managed, _ = newBundleManager(t)
	if err := managed.Upsert(ctx, storage.ManagedResourceInput{RepoID: repoRecord.ID, Address: "volume.active", Kind: "volume", Status: "applied", DeleteMode: string(manifest.DeleteModeProtect)}); err != nil {
		t.Fatal(err)
	}
	if protected, err := manager.ListProtectedResources(ctx, repoRecord.ID); err != nil || len(protected) != 0 {
		t.Fatalf("active resource incorrectly exposed as orphan: %#v, %v", protected, err)
	}
	if err := manager.DeleteRepository(ctx, repoRecord.ID, true); err == nil {
		t.Fatal("expected active protected resource deletion guard")
	}
}

func TestDeleteProtectedResourceRejectsUnknownOrUnmanagedAddresses(t *testing.T) {
	ctx := context.Background()
	manager, repoRecord, managed, _ := newBundleManager(t)
	if err := managed.Upsert(ctx, storage.ManagedResourceInput{RepoID: repoRecord.ID, Address: "job.demo", Kind: "job", NomadID: "demo", Status: "applied", DeleteMode: string(manifest.DeleteModeAllow)}); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"missing", "job.demo"} {
		if err := manager.DeleteProtectedResource(ctx, repoRecord.ID, address); err == nil {
			t.Fatalf("expected protected resource error for %q", address)
		}
	}
}
