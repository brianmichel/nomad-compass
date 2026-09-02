package reconcile

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	repomodel "github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func createLocalRemote(t *testing.T, content string) string {
	t.Helper()
	remotePath := filepath.Join(t.TempDir(), "remote")
	if err := os.MkdirAll(filepath.Join(remotePath, ".nomad"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepo, err := gogit.PlainInit(remotePath, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := gitRepo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remotePath, ".nomad", "job.nomad.hcl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add(".nomad/job.nomad.hcl"); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("initial", &gogit.CommitOptions{Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	return remotePath
}

func newLifecycleManager(t *testing.T, remotePath string) (*Manager, *storage.Repository, *fakeNomad) {
	t.Helper()
	ctx := context.Background()
	db, err := storage.Open(filepath.Join(t.TempDir(), "manager.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	repos := storage.NewRepoStore(db)
	record, err := repos.Create(ctx, storage.RepositoryInput{
		Name: "local", RepoURL: remotePath, Branch: "master", JobPath: ".nomad",
	})
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeNomad{}
	manager := New(repos, storage.NewRepoFileStore(db), storage.NewManagedResourceStore(db), nil, repomodel.NewManager(filepath.Join(t.TempDir(), "clones")), fake, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return manager, record, fake
}

func TestReconcileRepoSyncsAppliesAndThenOnlyPollsUnchangedRepository(t *testing.T) {
	remote := createLocalRemote(t, `job "demo" {
  datacenters = ["dc1"]
}`)
	manager, record, fake := newLifecycleManager(t, remote)
	ctx := context.Background()

	if err := manager.ReconcileRepo(ctx, record.ID); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if fake.registerCalls != 1 || len(fake.registeredJobIDs) != 1 || fake.registeredJobIDs[0] != "demo" {
		t.Fatalf("first reconcile registered jobs = %d (%v)", fake.registerCalls, fake.registeredJobIDs)
	}
	updated, err := manager.repos.Get(ctx, record.ID)
	if err != nil || updated == nil || !updated.LastCommit.Valid {
		t.Fatalf("commit metadata not persisted: %v %#v", err, updated)
	}
	firstCommit := updated.LastCommit.String

	if err := manager.ReconcileRepo(ctx, record.ID); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	updated, err = manager.repos.Get(ctx, record.ID)
	if err != nil || updated == nil || !updated.LastPolledAt.Valid {
		t.Fatalf("poll timestamp not persisted: %v %#v", err, updated)
	}
	if updated.LastCommit.String != firstCommit || fake.registerCalls != 1 {
		t.Fatalf("unchanged repository was reapplied: commit=%q calls=%d", updated.LastCommit.String, fake.registerCalls)
	}
	if err := manager.RunOnce(ctx); err != nil {
		t.Fatalf("run once: %v", err)
	}
	if fake.registerCalls != 1 {
		t.Fatalf("RunOnce reapplied unchanged repository: %d registrations", fake.registerCalls)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := manager.Run(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Run error = %v, want context canceled", err)
	}
}

func TestReconcileRepoRequiresExplicitAdoptionForExistingLegacyJob(t *testing.T) {
	remote := createLocalRemote(t, `job "demo" {
  namespace = "team-a"
  datacenters = ["dc1"]
}`)
	manager, record, fake := newLifecycleManager(t, remote)
	fake.jobStatuses = map[string]*nomadclient.JobStatus{
		"demo": {ID: "demo", Exists: true, Namespace: "team-a", Status: "running"},
	}
	fake.planResponses = map[string]*api.JobPlanResponse{"demo": {}}
	ctx := context.Background()

	if err := manager.ReconcileRepo(ctx, record.ID); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if fake.registerCalls != 0 {
		t.Fatalf("existing job was overwritten before adoption: %d registrations", fake.registerCalls)
	}
	files, err := manager.files.ListByRepo(ctx, record.ID)
	if err != nil || len(files) != 1 || files[0].Status != "adoption_required" || files[0].Namespace.String != "team-a" {
		t.Fatalf("unexpected adoption state: %v %#v", err, files)
	}

	if err := manager.AdoptJob(ctx, record.ID, files[0].Path); err != nil {
		t.Fatalf("adopt job: %v", err)
	}
	files, err = manager.files.ListByRepo(ctx, record.ID)
	if err != nil || len(files) != 1 || files[0].Status != "applied" {
		t.Fatalf("job was not adopted: %v %#v", err, files)
	}
	if fake.registerCalls != 0 {
		t.Fatalf("adoption registered a job: %d", fake.registerCalls)
	}

	if err := manager.ReconcileRepo(ctx, record.ID); err != nil {
		t.Fatalf("reconcile after adoption: %v", err)
	}
	if fake.registerCalls != 0 {
		t.Fatalf("unchanged adopted job was reapplied: %d", fake.registerCalls)
	}
}

func TestReconcileRepoRecordsPollAfterSyncFailureAndRejectsUnknownRepository(t *testing.T) {
	remote := createLocalRemote(t, `job "demo" { datacenters = ["dc1"] }`)
	manager, _, _ := newLifecycleManager(t, remote)
	ctx := context.Background()
	if err := manager.ReconcileRepo(ctx, 9999); err == nil || err.Error() != "repository not found" {
		t.Fatalf("unknown repository error = %v", err)
	}

	badManager, badRecord, _ := newLifecycleManager(t, filepath.Join(t.TempDir(), "missing-remote"))
	if err := badManager.ReconcileRepo(ctx, badRecord.ID); err == nil {
		t.Fatal("expected sync failure")
	}
	updated, err := badManager.repos.Get(ctx, badRecord.ID)
	if err != nil || updated == nil || !updated.LastPolledAt.Valid {
		t.Fatalf("sync failure did not record poll timestamp: %v %#v", err, updated)
	}
}

func TestPlanRepoIsReadOnlyAndReturnsBundleRevision(t *testing.T) {
	remotePath := filepath.Join(t.TempDir(), "bundle-remote")
	if err := os.MkdirAll(filepath.Join(remotePath, ".nomad"), 0o755); err != nil {
		t.Fatal(err)
	}
	gitRepo, err := gogit.PlainInit(remotePath, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := gitRepo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	bundle := `bundle "demo" {
  resource "namespace" "apps" {
    description = "Apps"
  }
}`
	if err := os.WriteFile(filepath.Join(remotePath, ".nomad", "compass.bundle.hcl"), []byte(bundle), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add(".nomad/compass.bundle.hcl"); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("bundle", &gogit.CommitOptions{Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()}}); err != nil {
		t.Fatal(err)
	}

	manager, record, fake := newLifecycleManager(t, remotePath)
	report, err := manager.PlanRepo(context.Background(), record.ID)
	if err != nil {
		t.Fatalf("plan bundle: %v", err)
	}
	if report == nil || report.Revision == "" || len(report.Resources) != 1 {
		t.Fatalf("unexpected plan report: %#v", report)
	}
	if len(fake.resourceCalls) != 0 || fake.registerCalls != 0 {
		t.Fatalf("planning mutated Nomad: calls=%v registrations=%d", fake.resourceCalls, fake.registerCalls)
	}

}
