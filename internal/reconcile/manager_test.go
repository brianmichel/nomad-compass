package reconcile

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	repomodel "github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

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

func TestApplyJobAddsMetadata(t *testing.T) {
	fake := &fakeNomad{}
	m := &Manager{nomad: fake}

	repo := &storage.Repository{RepoURL: "git@example.com/foo.git", Name: "foo"}
	snapshot := &repomodel.Snapshot{CommitHash: "abc123", CommitAuthor: "Tester <test@example.com>", CommitTitle: "Initial"}
	jobFile := repomodel.JobFile{Path: ".nomad/job.nomad.hcl", Content: []byte(`job "demo" { datacenters = ["dc1"] }`)}

	id, err := m.applyJob(context.Background(), repo, jobFile, snapshot)
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
		"nomad-compass/repo-url":      repo.RepoURL,
		"nomad-compass/repo-name":     repo.Name,
		"nomad-compass/job-file":      jobFile.Path,
		"nomad-compass/commit":        snapshot.CommitHash,
		"nomad-compass/commit-author": snapshot.CommitAuthor,
		"nomad-compass/commit-title":  snapshot.CommitTitle,
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

type fakeNomad struct {
	lastJob        *api.Job
	lastSubmission *api.JobSubmission
	deregistered   []string
}

func (f *fakeNomad) RegisterJob(_ context.Context, job *api.Job, submission *api.JobSubmission) error {
	f.lastJob = job
	f.lastSubmission = submission
	return nil
}

func (f *fakeNomad) DeregisterJob(_ context.Context, jobID string, _ bool) error {
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
