package reconcile

import (
	"context"
	"testing"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	repomodel "github.com/brianmichel/nomad-compass/internal/repo"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestParseJob(t *testing.T) {
	job, err := parseJob(".nomad/job.nomad.hcl", []byte(`job "demo" { datacenters = ["dc1"] }`))
	if err != nil {
		t.Fatalf("parse job: %v", err)
	}
	if job == nil || job.Name == nil || *job.Name != "demo" {
		t.Fatalf("unexpected job: %#v", job)
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

type fakeNomad struct {
	lastJob *api.Job
}

func (f *fakeNomad) RegisterJob(_ context.Context, job *api.Job) error {
	f.lastJob = job
	return nil
}

func (f *fakeNomad) DeregisterJob(_ context.Context, jobID string, _ bool) error {
	if f.lastJob != nil && f.lastJob.ID != nil && *f.lastJob.ID == jobID {
		f.lastJob = nil
	}
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
