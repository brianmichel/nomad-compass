package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
			_ = json.NewEncoder(w).Encode([]repositoryResponse{{ID: 7, Name: "homelab", Branch: "main", RepoURL: "https://example.test/repo"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/repos":
			_ = json.NewEncoder(w).Encode(repositoryResponse{ID: 8, Name: "new-repo"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/repos/8/reconcile":
			w.WriteHeader(http.StatusAccepted)
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
	if !strings.Contains(reposOutput.String(), `"name": "homelab"`) {
		t.Fatalf("unexpected repo output: %s", reposOutput.String())
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
	if err := Run(context.Background(), []string{"--server", server.URL, "repo", "delete", "--id", "8", "--yes"}, nil, &bytes.Buffer{}, nil); err != nil {
		t.Fatalf("repo delete: %v", err)
	}

	want := []string{
		"GET /api/status",
		"GET /api/repos",
		"POST /api/repos",
		"POST /api/repos/8/reconcile",
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
