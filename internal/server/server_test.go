package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

type fakeNomadClient struct {
	statusByID map[string]*nomadclient.JobStatus
	errByID    map[string]error
	calls      []string
}

func (f *fakeNomadClient) RegisterJob(ctx context.Context, job *api.Job) error {
	return nil
}

func (f *fakeNomadClient) DeregisterJob(ctx context.Context, jobID string, purge bool) error {
	return nil
}

func (f *fakeNomadClient) Ping(ctx context.Context) error {
	return nil
}

func (f *fakeNomadClient) JobStatus(ctx context.Context, jobID string) (*nomadclient.JobStatus, error) {
	f.calls = append(f.calls, jobID)
	if f.errByID != nil {
		if err, ok := f.errByID[jobID]; ok {
			return nil, err
		}
	}
	if f.statusByID != nil {
		if status, ok := f.statusByID[jobID]; ok {
			return status, nil
		}
	}
	return nil, nil
}

func setupServer(t *testing.T) (*Server, context.Context, *storage.RepoStore, *storage.RepoFileStore, *fakeNomadClient) {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	repoStore := storage.NewRepoStore(db)
	fileStore := storage.NewRepoFileStore(db)
	nomad := &fakeNomadClient{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := &Server{
		repos:     repoStore,
		files:     fileStore,
		nomad:     nomad,
		logger:    logger,
		nomadAddr: "http://nomad.local",
	}
	return srv, ctx, repoStore, fileStore, nomad
}

func TestListRepositoryResponsesWithoutJobStatus(t *testing.T) {
	srv, ctx, repoStore, fileStore, nomad := setupServer(t)

	repo, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	if err := fileStore.Upsert(ctx, repo.ID, "jobs/api.nomad", "abcd1234", ""); err != nil {
		t.Fatalf("upsert file: %v", err)
	}

	responses, err := srv.listRepositoryResponses(ctx)
	if err != nil {
		t.Fatalf("list responses: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 repo response, got %d", len(responses))
	}
	if len(responses[0].Jobs) != 1 {
		t.Fatalf("expected 1 job response, got %d", len(responses[0].Jobs))
	}

	job := responses[0].Jobs[0]
	if job.Path != "jobs/api.nomad" {
		t.Fatalf("expected path jobs/api.nomad, got %s", job.Path)
	}
	if job.JobID != "" {
		t.Fatalf("expected empty job id, got %s", job.JobID)
	}
	if len(nomad.calls) != 0 {
		t.Fatalf("expected no nomad calls, got %v", nomad.calls)
	}
}

func TestListRepositoryResponsesWithJobStatus(t *testing.T) {
	srv, ctx, repoStore, fileStore, nomad := setupServer(t)

	repo, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	if err := fileStore.Upsert(ctx, repo.ID, "jobs/api.nomad", "abcd1234", "job-123"); err != nil {
		t.Fatalf("upsert file: %v", err)
	}

	nomad.statusByID = map[string]*nomadclient.JobStatus{
		"job-123": {
			ID:                   "job-123",
			Name:                 "api",
			Namespace:            "default",
			Type:                 "service",
			Status:               "running",
			StatusDescription:    "Running",
			DerivedStatus:        "healthy",
			DesiredAllocs:        1,
			RunningAllocs:        1,
			LatestDeploymentID:   "deploy-1",
			LatestAllocationID:   "alloc-1",
			LatestAllocationName: "alloc-name",
			Allocations:          []nomadclient.AllocationStatus{{ID: "alloc-1", Status: "running"}},
			Exists:               true,
		},
	}

	responses, err := srv.listRepositoryResponses(ctx)
	if err != nil {
		t.Fatalf("list responses: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 repo response, got %d", len(responses))
	}
	if len(responses[0].Jobs) != 1 {
		t.Fatalf("expected 1 job response, got %d", len(responses[0].Jobs))
	}

	job := responses[0].Jobs[0]
	if job.JobID != "job-123" {
		t.Fatalf("expected job id job-123, got %s", job.JobID)
	}
	if job.Status != "healthy" {
		t.Fatalf("expected status healthy, got %s", job.Status)
	}
	if job.JobName != "api" {
		t.Fatalf("expected job name api, got %s", job.JobName)
	}
	if job.JobType != "service" {
		t.Fatalf("expected job type service, got %s", job.JobType)
	}
	expectedURL := "http://nomad.local/ui/jobs/job-123@default"
	if job.JobURL != expectedURL {
		t.Fatalf("expected job url %s, got %s", expectedURL, job.JobURL)
	}
	if len(job.Allocations) != 1 || job.Allocations[0].ID != "alloc-1" {
		t.Fatalf("unexpected allocations: %+v", job.Allocations)
	}
	if len(nomad.calls) != 1 || nomad.calls[0] != "job-123" {
		t.Fatalf("unexpected nomad calls: %v", nomad.calls)
	}
}

func TestListRepositoryResponsesMissingJob(t *testing.T) {
	srv, ctx, repoStore, fileStore, nomad := setupServer(t)

	repo, err := repoStore.Create(ctx, storage.RepositoryInput{
		Name:    "demo",
		RepoURL: "https://example.com/demo.git",
		Branch:  "main",
	})
	if err != nil {
		t.Fatalf("create repo: %v", err)
	}

	if err := fileStore.Upsert(ctx, repo.ID, "jobs/api.nomad", "abcd1234", "job-123"); err != nil {
		t.Fatalf("upsert file: %v", err)
	}

	nomad.statusByID = map[string]*nomadclient.JobStatus{
		"job-123": {ID: "job-123", Exists: false},
	}

	responses, err := srv.listRepositoryResponses(ctx)
	if err != nil {
		t.Fatalf("list responses: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 repo response, got %d", len(responses))
	}
	job := responses[0].Jobs[0]
	if job.Status != "missing" {
		t.Fatalf("expected status missing, got %s", job.Status)
	}
	if job.StatusDescription != "Job not found in Nomad" {
		t.Fatalf("unexpected status description: %s", job.StatusDescription)
	}
}

func TestListRepositoryResponsesPropagatesErrors(t *testing.T) {
	errStore := errors.New("store error")
	srv := &Server{
		repos: &staticRepoStore{repos: []storage.Repository{{ID: 1}}, err: nil},
		files: &failingRepoFileStore{err: errStore},
		nomad: &fakeNomadClient{},
	}
	_, err := srv.listRepositoryResponses(context.Background())
	if !errors.Is(err, errStore) {
		t.Fatalf("expected error %v, got %v", errStore, err)
	}
}

func TestJobURL(t *testing.T) {
	cases := []struct {
		name      string
		base      string
		namespace string
		jobID     string
		expected  string
	}{
		{name: "empty base", jobID: "job-1", expected: ""},
		{name: "empty job", base: "http://nomad.local", expected: ""},
		{name: "default namespace", base: "http://nomad.local/", jobID: "job-1", expected: "http://nomad.local/ui/jobs/job-1@default"},
		{name: "custom namespace", base: "http://nomad.local", namespace: "team-a", jobID: "job-1", expected: "http://nomad.local/ui/jobs/job-1@team-a"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jobURL(tc.base, tc.namespace, tc.jobID); got != tc.expected {
				t.Fatalf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

type staticRepoStore struct {
	repos []storage.Repository
	err   error
}

func (s *staticRepoStore) List(ctx context.Context) ([]storage.Repository, error) {
	return s.repos, s.err
}

func (s *staticRepoStore) Create(ctx context.Context, input storage.RepositoryInput) (*storage.Repository, error) {
	return nil, errors.New("not implemented")
}

type failingRepoFileStore struct {
	err error
}

func (f *failingRepoFileStore) ListByRepo(ctx context.Context, repoID int64) ([]storage.RepoFile, error) {
	return nil, f.err
}
