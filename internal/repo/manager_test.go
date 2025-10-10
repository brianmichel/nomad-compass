package repo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestManagerSync(t *testing.T) {
	tmp := t.TempDir()
	remotePath := filepath.Join(tmp, "remote")
	if err := os.MkdirAll(filepath.Join(remotePath, ".nomad"), 0o755); err != nil {
		t.Fatalf("mkdir remote: %v", err)
	}

	repo, err := gogit.PlainInit(remotePath, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	jobPath := filepath.Join(remotePath, ".nomad", "job.nomad.hcl")
	if err := os.WriteFile(jobPath, []byte(`job "example" {
  datacenters = ["dc1"]
  group "web" {
    task "server" {
      driver = "raw_exec"
      config { command = "sleep" args = ["10"] }
    }
  }
}
`), 0o644); err != nil {
		t.Fatalf("write job: %v", err)
	}

	if _, err := wt.Add(".nomad/job.nomad.hcl"); err != nil {
		t.Fatalf("add: %v", err)
	}

	_, err = wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{Name: "Tester", Email: "tester@example.com", When: time.Now()},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	manager := NewManager(filepath.Join(tmp, "clones"))
	snapshot, err := manager.Sync(context.Background(), storage.Repository{
		ID:      1,
		Name:    "example",
		RepoURL: remotePath,
		Branch:  "master",
		JobPath: ".nomad",
	}, nil, nil)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	if snapshot.CommitHash == "" {
		t.Fatal("expected commit hash")
	}
	if len(snapshot.JobFiles) != 1 {
		t.Fatalf("expected 1 job file, got %d", len(snapshot.JobFiles))
	}

	// Second sync should reuse clone without error.
	if _, err := manager.Sync(context.Background(), storage.Repository{
		ID:      1,
		Name:    "example",
		RepoURL: remotePath,
		Branch:  "master",
		JobPath: ".nomad",
	}, nil, nil); err != nil {
		t.Fatalf("second sync: %v", err)
	}
}

func TestManagerSyncCustomJobPath(t *testing.T) {
	tmp := t.TempDir()
	remotePath := filepath.Join(tmp, "remote-custom")
	jobDir := filepath.Join(remotePath, "jobspecs")
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		t.Fatalf("mkdir remote job dir: %v", err)
	}

	repo, err := gogit.PlainInit(remotePath, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	mainJobPath := filepath.Join(jobDir, "api.nomad")
	if err := os.WriteFile(mainJobPath, []byte(`job "api" {
  datacenters = ["dc1"]
}
`), 0o644); err != nil {
		t.Fatalf("write job: %v", err)
	}

	subDir := filepath.Join(jobDir, "regional")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir regional: %v", err)
	}
	regionalJob := filepath.Join(subDir, "worker.nomad.hcl")
	if err := os.WriteFile(regionalJob, []byte(`job "worker" {
  datacenters = ["dc2"]
}
`), 0o644); err != nil {
		t.Fatalf("write regional job: %v", err)
	}

	if _, err := wt.Add("jobspecs/api.nomad"); err != nil {
		t.Fatalf("add api job: %v", err)
	}
	if _, err := wt.Add("jobspecs/regional/worker.nomad.hcl"); err != nil {
		t.Fatalf("add worker job: %v", err)
	}

	if _, err := wt.Commit("custom jobs", &gogit.CommitOptions{
		Author: &object.Signature{Name: "Tester", Email: "tester@example.com", When: time.Now()},
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	manager := NewManager(filepath.Join(tmp, "clones"))
	snapshot, err := manager.Sync(context.Background(), storage.Repository{
		ID:      2,
		Name:    "custom",
		RepoURL: remotePath,
		Branch:  "master",
		JobPath: "jobspecs",
	}, nil, nil)
	if err != nil {
		t.Fatalf("sync custom: %v", err)
	}

	if len(snapshot.JobFiles) != 2 {
		t.Fatalf("expected 2 job files, got %d", len(snapshot.JobFiles))
	}

	expectedPaths := map[string]struct{}{
		"jobspecs/api.nomad":                 {},
		"jobspecs/regional/worker.nomad.hcl": {},
	}
	for _, file := range snapshot.JobFiles {
		if _, ok := expectedPaths[file.Path]; !ok {
			t.Fatalf("unexpected job file %s", file.Path)
		}
	}
}
