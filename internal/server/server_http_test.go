package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

type handlerRepoStore struct {
	mu      sync.Mutex
	repos   []storage.Repository
	created []storage.RepositoryInput
	err     error
}

func (s *handlerRepoStore) List(context.Context) ([]storage.Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]storage.Repository(nil), s.repos...), s.err
}

func (s *handlerRepoStore) Create(_ context.Context, input storage.RepositoryInput) (*storage.Repository, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	s.created = append(s.created, input)
	return &storage.Repository{ID: 42, Name: input.Name, RepoURL: input.RepoURL, Branch: input.Branch, JobPath: input.JobPath}, nil
}

type handlerFileStore struct{}

func (handlerFileStore) ListByRepo(context.Context, int64) ([]storage.RepoFile, error) {
	return nil, nil
}

type handlerCredentialStore struct {
	mu      sync.Mutex
	creds   []storage.Credential
	created []storage.CredentialPayload
	err     error
}

func (s *handlerCredentialStore) List(context.Context) ([]storage.Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]storage.Credential(nil), s.creds...), s.err
}

func (s *handlerCredentialStore) Create(_ context.Context, name string, ctype storage.CredentialType, payload storage.CredentialPayload) (*storage.Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	s.created = append(s.created, payload)
	credential := storage.Credential{ID: 7, Name: name, Type: ctype}
	s.creds = append(s.creds, credential)
	return &credential, nil
}

type handlerReconciler struct {
	mu                sync.Mutex
	reconcileCalls    []int64
	planCalls         []int64
	forgetCalls       []string
	deleteOrphanCalls []string
	adoptCalls        []string
	deleteRepoCalls   []struct {
		id         int64
		unschedule bool
	}
	deleteCredCalls []struct {
		id          int64
		deleteRepos bool
		unschedule  bool
	}
	reconcileCh chan int64
	protected   []storage.ManagedResource
	err         error
	plan        *plan.Report
}

func (r *handlerReconciler) ReconcileRepo(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reconcileCalls = append(r.reconcileCalls, id)
	if r.reconcileCh != nil {
		select {
		case r.reconcileCh <- id:
		default:
		}
	}
	return r.err
}

func (r *handlerReconciler) PlanRepo(_ context.Context, id int64) (*plan.Report, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.planCalls = append(r.planCalls, id)
	if r.err != nil {
		return nil, r.err
	}
	return r.plan, nil
}

func (r *handlerReconciler) ListProtectedResources(context.Context, int64) ([]storage.ManagedResource, error) {
	return r.protected, r.err
}
func (r *handlerReconciler) ForgetProtectedResource(_ context.Context, _ int64, address string) error {
	r.forgetCalls = append(r.forgetCalls, address)
	return r.err
}
func (r *handlerReconciler) DeleteProtectedResource(_ context.Context, _ int64, address string) error {
	r.deleteOrphanCalls = append(r.deleteOrphanCalls, address)
	return r.err
}
func (r *handlerReconciler) AdoptBundleResource(_ context.Context, _ int64, address string) error {
	r.adoptCalls = append(r.adoptCalls, address)
	return r.err
}
func (r *handlerReconciler) DeleteRepository(_ context.Context, id int64, unschedule bool) error {
	r.deleteRepoCalls = append(r.deleteRepoCalls, struct {
		id         int64
		unschedule bool
	}{id, unschedule})
	return r.err
}
func (r *handlerReconciler) DeleteCredential(_ context.Context, id int64, deleteRepos bool, unschedule bool) error {
	r.deleteCredCalls = append(r.deleteCredCalls, struct {
		id          int64
		deleteRepos bool
		unschedule  bool
	}{id, deleteRepos, unschedule})
	return r.err
}

type handlerNomad struct{ pingErr error }

func (n handlerNomad) RegisterJob(context.Context, *api.Job, *api.JobSubmission) error { return nil }
func (n handlerNomad) DeregisterJob(context.Context, string, bool) error               { return nil }
func (n handlerNomad) Ping(context.Context) error                                      { return n.pingErr }
func (n handlerNomad) JobStatus(context.Context, string) (*nomadclient.JobStatus, error) {
	return nil, nil
}
func (n handlerNomad) PlanJob(context.Context, *api.Job) (*api.JobPlanResponse, error) {
	return nil, nil
}

func newHTTPTestServer(t *testing.T, repos *handlerRepoStore, creds *handlerCredentialStore, reconciler *handlerReconciler, nomad handlerNomad) *httptest.Server {
	t.Helper()
	server := New(repos, handlerFileStore{}, creds, reconciler, nomad, "http://nomad.local", slog.New(slog.NewTextHandler(io.Discard, nil)))
	return httptest.NewServer(server.Handler())
}

func request(t *testing.T, serverURL, method, path string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, serverURL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestHandlerValidatesMalformedRequestsAndDispatchesSafely(t *testing.T) {
	repos := &handlerRepoStore{}
	creds := &handlerCredentialStore{}
	reconciler := &handlerReconciler{plan: &plan.Report{}}
	server := newHTTPTestServer(t, repos, creds, reconciler, handlerNomad{})
	defer server.Close()

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"malformed create repo", http.MethodPost, "/api/repos", "{", http.StatusBadRequest},
		{"invalid trigger id", http.MethodPost, "/api/repos/not-an-id/reconcile", "{}", http.StatusBadRequest},
		{"malformed trigger body", http.MethodPost, "/api/repos/42/reconcile", "{", http.StatusBadRequest},
		{"invalid plan id", http.MethodGet, "/api/repos/not-an-id/plan", "", http.StatusBadRequest},
		{"malformed orphan body", http.MethodPost, "/api/repos/42/orphans/forget", "{", http.StatusBadRequest},
		{"missing orphan address", http.MethodPost, "/api/repos/42/orphans/forget", `{}`, http.StatusBadRequest},
		{"invalid orphan id", http.MethodPost, "/api/repos/not-an-id/orphans/delete", `{"address":"volume.data"}`, http.StatusBadRequest},
		{"malformed delete repo", http.MethodDelete, "/api/repos/42", "{", http.StatusBadRequest},
		{"invalid credential id", http.MethodDelete, "/api/credentials/not-an-id", "{}", http.StatusBadRequest},
		{"invalid credential body", http.MethodPost, "/api/credentials", "{", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := requestWithRawBody(t, server.URL, tc.method, tc.path, tc.body)
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}

	resp := request(t, server.URL, http.MethodPut, "/api/repos", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("unsupported method status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func requestWithRawBody(t *testing.T, serverURL, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, serverURL+path, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestHandlerSupportsManualAndAutomaticRepositoryReconcile(t *testing.T) {
	repos := &handlerRepoStore{}
	creds := &handlerCredentialStore{}
	reconciler := &handlerReconciler{}
	server := newHTTPTestServer(t, repos, creds, reconciler, handlerNomad{})
	defer server.Close()

	resp := request(t, server.URL, http.MethodPost, "/api/repos", map[string]any{
		"name": "manual", "repo_url": "https://example.test/repo.git", "branch": "main", "initial_reconcile": false,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manual create status = %d", resp.StatusCode)
	}
	var created repositoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID != 42 || len(reconciler.reconcileCalls) != 0 {
		t.Fatalf("manual create = %#v, reconcile calls = %v", created, reconciler.reconcileCalls)
	}
	if len(repos.created) != 1 || repos.created[0].Name != "manual" {
		t.Fatalf("repository input not recorded: %#v", repos.created)
	}

	reconciler.reconcileCh = make(chan int64, 1)
	resp = request(t, server.URL, http.MethodPost, "/api/repos", map[string]any{
		"name": "automatic", "repo_url": "https://example.test/repo.git", "branch": "main", "initial_reconcile": true,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("automatic create status = %d", resp.StatusCode)
	}
	// The automatic reconcile is intentionally asynchronous. Assert the
	// observable fake call without making the handler synchronous.
	select {
	case id := <-reconciler.reconcileCh:
		if id != 42 {
			t.Fatalf("reconciled repository ID = %d, want 42", id)
		}
	case <-time.After(time.Second):
		t.Fatal("automatic reconcile was not started")
	}
}

func TestHandlerDispatchesMutationsAndMapsBackendErrors(t *testing.T) {
	repos := &handlerRepoStore{}
	creds := &handlerCredentialStore{}
	reconciler := &handlerReconciler{plan: &plan.Report{}}
	reconciler.protected = []storage.ManagedResource{{Address: "volume.data", Kind: "volume", Status: "protected", DeleteMode: "protect"}}
	server := newHTTPTestServer(t, repos, creds, reconciler, handlerNomad{})
	defer server.Close()

	resp := request(t, server.URL, http.MethodGet, "/api/repos/42/orphans", nil)
	var orphans []protectedResourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&orphans); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(orphans) != 1 || orphans[0].Address != "volume.data" {
		t.Fatalf("orphans response = %d %#v", resp.StatusCode, orphans)
	}

	resp = request(t, server.URL, http.MethodPost, "/api/repos/42/reconcile", map[string]any{"dry_run": true})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(reconciler.planCalls) != 1 {
		t.Fatalf("dry-run response = %d, plan calls = %v", resp.StatusCode, reconciler.planCalls)
	}
	resp = request(t, server.URL, http.MethodPost, "/api/repos/42/reconcile", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted || len(reconciler.reconcileCalls) != 1 {
		t.Fatalf("reconcile response = %d, calls = %v", resp.StatusCode, reconciler.reconcileCalls)
	}

	for _, tc := range []struct {
		path string
		body map[string]any
		want []string
	}{
		{"/api/repos/42/orphans/forget", map[string]any{"address": "volume.data"}, []string{"forget"}},
		{"/api/repos/42/orphans/delete", map[string]any{"address": "volume.data"}, []string{"delete"}},
		{"/api/repos/42/adopt", map[string]any{"address": "volume.data"}, []string{"adopt"}},
	} {
		resp = request(t, server.URL, http.MethodPost, tc.path, tc.body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, resp.StatusCode)
		}
	}
	resp = request(t, server.URL, http.MethodDelete, "/api/repos/42", map[string]any{"unschedule": true})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(reconciler.deleteRepoCalls) != 1 || !reconciler.deleteRepoCalls[0].unschedule {
		t.Fatalf("delete repo = %d, calls = %#v", resp.StatusCode, reconciler.deleteRepoCalls)
	}
	resp = request(t, server.URL, http.MethodDelete, "/api/credentials/7", map[string]any{"delete_repos": true, "unschedule": true})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(reconciler.deleteCredCalls) != 1 || !reconciler.deleteCredCalls[0].deleteRepos || !reconciler.deleteCredCalls[0].unschedule {
		t.Fatalf("delete credential = %d, calls = %#v", resp.StatusCode, reconciler.deleteCredCalls)
	}

	reconciler.err = errors.New("backend exploded")
	resp = request(t, server.URL, http.MethodPost, "/api/repos/42/reconcile", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("backend error status = %d, want 500", resp.StatusCode)
	}
}

func TestHandlerCredentialValidationAndStatus(t *testing.T) {
	repos := &handlerRepoStore{}
	creds := &handlerCredentialStore{}
	reconciler := &handlerReconciler{}
	server := newHTTPTestServer(t, repos, creds, reconciler, handlerNomad{pingErr: errors.New("Nomad unavailable")})
	defer server.Close()

	resp := request(t, server.URL, http.MethodPost, "/api/credentials", map[string]any{"name": "bad", "type": "https-token"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid credential status = %d", resp.StatusCode)
	}
	resp = request(t, server.URL, http.MethodPost, "/api/credentials", map[string]any{"name": "token", "type": "https-token", "token": "secret"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(creds.created) != 1 {
		t.Fatalf("valid credential status = %d, created = %#v", resp.StatusCode, creds.created)
	}

	resp = request(t, server.URL, http.MethodGet, "/api/credentials", nil)
	var listed []credentialResponse
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if len(listed) != 1 || listed[0].Name != "token" {
		t.Fatalf("listed credentials = %#v", listed)
	}

	resp = request(t, server.URL, http.MethodGet, "/api/status", nil)
	defer resp.Body.Close()
	var status statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatal(err)
	}
	if status.NomadConnected || status.NomadMessage != "Nomad unavailable" {
		t.Fatalf("unexpected status response: %#v", status)
	}
}

func TestHandlerMapsNotFoundPlanAndStoreErrors(t *testing.T) {
	repos := &handlerRepoStore{}
	creds := &handlerCredentialStore{}
	reconciler := &handlerReconciler{err: errors.New("repository not found")}
	server := newHTTPTestServer(t, repos, creds, reconciler, handlerNomad{})
	defer server.Close()

	resp := request(t, server.URL, http.MethodGet, "/api/repos/42/plan", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("not-found plan status = %d, want 404", resp.StatusCode)
	}

	repos.err = errors.New("database unavailable")
	resp = request(t, server.URL, http.MethodGet, "/api/repos", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("store error status = %d, want 500", resp.StatusCode)
	}
}
