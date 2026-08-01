package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/nomadclient"
)

func TestRunAPICommandsUseGlobalOptionsBeforeVerbs(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/status":
			_ = json.NewEncoder(w).Encode(statusResponse{NomadConnected: true})
		case r.Method == http.MethodGet && r.URL.Path == "/api/repos":
			credentialID := int64(9)
			commit := "commit-1"
			author := "Tester <tester@example.com>"
			_ = json.NewEncoder(w).Encode([]repositoryResponse{{
				ID: 7, Name: "homelab", Branch: "main", RepoURL: "https://example.test/repo", CredentialID: &credentialID,
				LastCommit: &commit, LastCommitAuthor: &author,
				Jobs: []repositoryJob{{Path: "jobs/api.nomad", JobName: "api", Namespace: "apps", StatusDescription: "healthy", Allocations: []nomadclient.AllocationStatus{{ID: "alloc-1", Status: "running"}}}},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/repos":
			_ = json.NewEncoder(w).Encode(repositoryResponse{ID: 8, Name: "new-repo"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/repos/8/reconcile":
			var request reconcileRequest
			_ = json.NewDecoder(r.Body).Decode(&request)
			if request.DryRun {
				_ = json.NewEncoder(w).Encode(PlanReport{Bundle: "apps", Revision: "a1b2c3d", Summary: PlanSummary{Unchanged: 1}})
			} else {
				w.WriteHeader(http.StatusAccepted)
			}
		case r.Method == http.MethodGet && r.URL.Path == "/api/repos/8/plan":
			_ = json.NewEncoder(w).Encode(PlanReport{Bundle: "apps", Revision: "a1b2c3d", Summary: PlanSummary{Unchanged: 1}})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/repos/8":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var statusOutput bytes.Buffer
	if err := Run(context.Background(), []string{"--server", server.URL, "--format", "json", "status"}, nil, &statusOutput, nil); err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(statusOutput.String(), `"nomad_connected": true`) {
		t.Fatalf("unexpected status output: %s", statusOutput.String())
	}

	var reposOutput bytes.Buffer
	if err := Run(context.Background(), []string{"--server", server.URL, "repo", "list", "--format", "json"}, nil, &reposOutput, nil); err != nil {
		t.Fatalf("repo list: %v", err)
	}
	for _, field := range []string{`"credential_id": 9`, `"last_commit_author": "Tester `, `"job_name": "api"`, `"status_description": "healthy"`, `"id": "alloc-1"`} {
		if !strings.Contains(reposOutput.String(), field) {
			t.Fatalf("repository JSON lost %s: %s", field, reposOutput.String())
		}
	}

	var addOutput bytes.Buffer
	if err := Run(context.Background(), []string{"--server", server.URL, "repo", "add", "--name", "new-repo", "--url", "https://example.test/repo"}, nil, &addOutput, nil); err != nil {
		t.Fatalf("repo add: %v", err)
	}
	if !strings.Contains(addOutput.String(), "repository 8 created") {
		t.Fatalf("unexpected add output: %s", addOutput.String())
	}

	if err := Run(context.Background(), []string{"--server", server.URL, "repo", "reconcile", "--id", "8"}, nil, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("repo reconcile: %v", err)
	}
	var dryRunOutput bytes.Buffer
	if err := Run(context.Background(), []string{"--server", server.URL, "--format", "json", "repo", "reconcile", "--id", "8", "--dry-run"}, nil, &dryRunOutput, nil); err != nil {
		t.Fatalf("repo reconcile dry-run: %v", err)
	}
	if !strings.Contains(dryRunOutput.String(), `"revision": "a1b2c3d"`) {
		t.Fatalf("unexpected dry-run output: %s", dryRunOutput.String())
	}
	var planOutput bytes.Buffer
	if err := Run(context.Background(), []string{"--server", server.URL, "--format", "json", "repo", "plan", "--id", "8"}, nil, &planOutput, nil); err != nil {
		t.Fatalf("repo plan: %v", err)
	}
	if !strings.Contains(planOutput.String(), `"revision": "a1b2c3d"`) {
		t.Fatalf("unexpected plan output: %s", planOutput.String())
	}
	if err := Run(context.Background(), []string{"--server", server.URL, "repo", "delete", "--id", "8", "--yes"}, nil, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("repo delete: %v", err)
	}

	want := []string{
		"GET /api/status",
		"GET /api/repos",
		"POST /api/repos",
		"POST /api/repos/8/reconcile",
		"POST /api/repos/8/reconcile",
		"GET /api/repos/8/plan",
		"DELETE /api/repos/8",
	}
	if len(requests) != len(want) {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
	for i := range want {
		if requests[i] != want[i] {
			t.Errorf("request %d = %q, want %q", i, requests[i], want[i])
		}
	}
}

func TestRunDestructiveCommandsRequireConfirmation(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	err := Run(context.Background(), []string{"--server", server.URL, "repo", "delete", "--id", "8"}, nil, &bytes.Buffer{}, nil)
	if err == nil || !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
	if called {
		t.Fatal("confirmation failure made an API request")
	}
}
