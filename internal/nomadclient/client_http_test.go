package nomadclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/config"
)

func testAPI(t *testing.T, handler http.Handler) *API {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := New(config.NomadConfig{Address: server.URL, Token: "test-token", Namespace: "team-a", Region: "global"})
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	return client
}

func TestResourceObservesTreatOnlyNotFoundAsAbsence(t *testing.T) {
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "forbidden") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	ctx := context.Background()

	absent := []struct {
		name string
		get  func() (any, error)
	}{
		{"namespace", func() (any, error) { return client.ObserveNamespace(ctx, "missing") }},
		{"quota", func() (any, error) { return client.ObserveQuota(ctx, "missing") }},
		{"variable", func() (any, error) { return client.ObserveVariable(ctx, "team-a", "missing") }},
		{"sentinel", func() (any, error) { return client.ObserveSentinelPolicy(ctx, "missing") }},
		{"acl policy", func() (any, error) { return client.ObserveACLPolicy(ctx, "missing") }},
		{"host volume", func() (any, error) { return client.ObserveHostVolume(ctx, "missing", "team-a") }},
		{"CSI volume", func() (any, error) { return client.ObserveCSIVolume(ctx, "missing", "team-a") }},
	}
	for _, tc := range absent {
		t.Run(tc.name, func(t *testing.T) {
			value, err := tc.get()
			if err != nil || (value != nil && !reflect.ValueOf(value).IsNil()) {
				t.Fatalf("missing resource = %#v, err = %v; want nil, nil", value, err)
			}
		})
	}

	for _, tc := range []struct {
		name string
		get  func() error
	}{
		{"namespace", func() error { _, err := client.ObserveNamespace(ctx, "forbidden"); return err }},
		{"variable", func() error { _, err := client.ObserveVariable(ctx, "team-a", "forbidden"); return err }},
		{"host volume", func() error { _, err := client.ObserveHostVolume(ctx, "forbidden", "team-a"); return err }},
	} {
		t.Run(tc.name+" preserves permission errors", func(t *testing.T) {
			if err := tc.get(); err == nil {
				t.Fatal("expected permission error, got nil")
			}
		})
	}
}

func TestResourceDeletesTreatNotFoundAsIdempotent(t *testing.T) {
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	ctx := context.Background()
	deletes := []struct {
		name string
		call func() error
	}{
		{"namespace", func() error { return client.DeleteNamespace(ctx, "missing") }},
		{"quota", func() error { return client.DeleteQuota(ctx, "missing") }},
		{"variable", func() error { return client.DeleteVariable(ctx, "team-a", "missing") }},
		{"sentinel", func() error { return client.DeleteSentinelPolicy(ctx, "missing") }},
		{"acl policy", func() error { return client.DeleteACLPolicy(ctx, "missing") }},
		{"host volume", func() error { return client.DeleteHostVolume(ctx, "missing", "team-a", false) }},
		{"CSI volume", func() error { return client.DeleteCSIVolume(ctx, "missing", "team-a", false) }},
	}
	for _, tc := range deletes {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err != nil {
				t.Fatalf("delete missing resource: %v", err)
			}
		})
	}
}

func TestResourceValidationRejectsEmptyInputsBeforeHTTP(t *testing.T) {
	called := false
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	ctx := context.Background()
	checks := []struct {
		name string
		call func() error
	}{
		{"namespace", func() error { return client.ApplyNamespace(ctx, nil) }},
		{"quota", func() error { return client.ApplyQuota(ctx, &api.QuotaSpec{}) }},
		{"variable", func() error { _, err := client.ApplyVariable(ctx, &api.Variable{}); return err }},
		{"sentinel", func() error { return client.ApplySentinelPolicy(ctx, &api.SentinelPolicy{}) }},
		{"ACL policy", func() error { return client.ApplyACLPolicy(ctx, &api.ACLPolicy{}) }},
		{"host volume", func() error { _, err := client.ApplyHostVolume(ctx, nil); return err }},
		{"CSI volume", func() error { _, err := client.ApplyCSIVolume(ctx, &api.CSIVolume{}); return err }},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if called {
		t.Fatal("validation errors should not make HTTP requests")
	}

	if status, err := client.JobStatus(ctx, "", ""); err != nil || status != nil {
		t.Fatalf("empty job ID = %#v, %v; want nil, nil", status, err)
	}
	if _, err := client.PlanJob(ctx, nil); err == nil {
		t.Fatal("expected nil job planning error")
	}
	job := &api.Job{}
	if _, err := client.PlanJob(ctx, job); err == nil {
		t.Fatal("expected missing job ID planning error")
	}
}

func TestJobStatusAggregatesNomadResponsesAndToleratesOptionalFailures(t *testing.T) {
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("namespace") != "team-b" {
			t.Fatalf("request namespace = %q, want team-b", r.URL.Query().Get("namespace"))
		}
		switch {
		case r.URL.Path == "/v1/job/job-1":
			_, _ = w.Write([]byte(`{"ID":"job-1","Name":"web","Namespace":"team-b","Type":"service","Status":"running","TaskGroups":[{"Count":2}]}`))
		case strings.HasSuffix(r.URL.Path, "/summary"):
			_, _ = w.Write([]byte(`{"Summary":{"web":{"Running":1,"Starting":1,"Failed":1}}}`))
		case strings.HasSuffix(r.URL.Path, "/deployment"):
			_, _ = w.Write([]byte(`{"ID":"deployment-1","Status":"successful"}`))
		case strings.HasSuffix(r.URL.Path, "/allocations"):
			_, _ = w.Write([]byte(`[{"ID":"alloc-1","Name":"web.alloc","NodeName":"node-1","ClientStatus":"running","DesiredStatus":"run","TaskGroup":"web","DeploymentStatus":{"Healthy":true}}]`))
		default:
			http.NotFound(w, r)
		}
	}))

	status, err := client.JobStatus(context.Background(), "job-1", "team-b")
	if err != nil {
		t.Fatalf("job status: %v", err)
	}
	if !status.Exists || status.Name != "web" || status.Namespace != "team-b" || status.Type != "service" {
		t.Fatalf("unexpected identity: %#v", status)
	}
	if status.DesiredAllocs != 2 || status.RunningAllocs != 1 || status.StartingAllocs != 1 || status.FailedAllocs != 1 {
		t.Fatalf("unexpected allocation totals: %#v", status)
	}
	if status.DerivedStatus != "degraded" || status.LatestDeploymentID != "deployment-1" || len(status.Allocations) != 1 {
		t.Fatalf("unexpected derived status: %#v", status)
	}

	degradedClient := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/job/job-2" {
			_, _ = w.Write([]byte(`{"ID":"job-2","Status":"running"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	status, err = degradedClient.JobStatus(context.Background(), "job-2", "team-a")
	if err != nil || status == nil || status.DerivedStatus != "running" {
		t.Fatalf("optional status endpoint failure should preserve base status: %#v, %v", status, err)
	}
}

func TestJobWritesUseExplicitJobNamespace(t *testing.T) {
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("namespace") != "team-b" {
			t.Fatalf("write namespace = %q, want team-b", r.URL.Query().Get("namespace"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"EvalID":"eval-1"}`))
	}))
	jobID := "job-1"
	namespace := "team-b"
	job := &api.Job{ID: &jobID, Namespace: &namespace}
	if err := client.RegisterJob(context.Background(), job, nil); err != nil {
		t.Fatalf("register job: %v", err)
	}
	if _, err := client.PlanJob(context.Background(), job); err != nil {
		t.Fatalf("plan job: %v", err)
	}
	if err := client.DeregisterJob(context.Background(), "job-1", "team-b", true); err != nil {
		t.Fatalf("deregister job: %v", err)
	}
}

func TestJobStatusMapsNotFoundButPropagatesForbidden(t *testing.T) {
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/missing") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	missing, err := client.JobStatus(context.Background(), "missing", "team-a")
	if err != nil || missing == nil || missing.Exists {
		t.Fatalf("missing job = %#v, %v; want non-existing status", missing, err)
	}
	if _, err := client.JobStatus(context.Background(), "forbidden", "team-a"); err == nil {
		t.Fatal("expected forbidden job lookup to fail")
	}
}

func TestNewRejectsInvalidNomadAddress(t *testing.T) {
	_, err := New(config.NomadConfig{Address: "://invalid"})
	if err == nil {
		t.Fatal("expected invalid Nomad address error")
	}
}

func TestVariableWritesUseCheckedCreateAndUpdate(t *testing.T) {
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/var/config" || r.Method != http.MethodPut {
			t.Fatalf("unexpected variable request: %s %s", r.Method, r.URL.String())
		}
		if r.URL.Query().Get("cas") != "0" {
			t.Fatalf("create CAS = %q, want 0", r.URL.Query().Get("cas"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Path":"config","ModifyIndex":1}`))
	}))
	created, err := client.CreateVariable(context.Background(), &api.Variable{Path: "config", Items: map[string]string{"a": "b"}})
	if err != nil || created == nil || created.Path != "config" {
		t.Fatalf("checked create = %#v, %v", created, err)
	}

	updateClient := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cas") != "7" {
			t.Fatalf("update CAS = %q, want 7", r.URL.Query().Get("cas"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Path":"config","ModifyIndex":8}`))
	}))
	updated, err := updateClient.ApplyVariable(context.Background(), &api.Variable{Path: "config", ModifyIndex: 7, Items: map[string]string{"a": "c"}})
	if err != nil || updated == nil || updated.ModifyIndex != 8 {
		t.Fatalf("checked update = %#v, %v", updated, err)
	}
}

func TestResourceErrorResponsesAreReturned(t *testing.T) {
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	ctx := context.Background()
	if _, err := client.ObserveQuota(ctx, "broken"); err == nil {
		t.Fatal("expected quota lookup error")
	}
	if err := client.DeleteVariable(ctx, "team-a", "broken"); err == nil {
		t.Fatal("expected variable deletion error")
	}
	if err := client.ApplySentinelPolicy(ctx, &api.SentinelPolicy{Name: "broken"}); err == nil {
		t.Fatal("expected Sentinel apply error")
	}
}

func TestAPIClientCanDecodeSuccessfulNamespaceResponse(t *testing.T) {
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet || r.URL.Path != "/v1/namespace/team-a" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(&api.Namespace{Name: "team-a", Description: "Team namespace"})
	}))
	namespace, err := client.ObserveNamespace(context.Background(), "team-a")
	if err != nil {
		t.Fatalf("observe namespace: %v", err)
	}
	if namespace == nil || namespace.Name != "team-a" || namespace.Description != "Team namespace" {
		t.Fatalf("unexpected namespace: %#v", namespace)
	}
}

func TestResourceDiscoveryMatchesEffectiveNamespaces(t *testing.T) {
	client := testAPI(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/volumes" && r.URL.Query().Get("type") == "host" {
			_, _ = w.Write([]byte(`[{"ID":"host-1","Name":"data","Namespace":""}]`))
			return
		}
		if r.URL.Path == "/v1/volume/host/host-1" {
			_, _ = w.Write([]byte(`{"ID":"host-1","Name":"data","Namespace":"","PluginID":"mkdir"}`))
			return
		}
		if r.URL.Path == "/v1/volumes" && r.URL.Query().Get("type") == "csi" {
			_, _ = w.Write([]byte(`[{"ID":"csi-1","Name":"data","Namespace":""}]`))
			return
		}
		if r.URL.Path == "/v1/volume/csi/csi-1" {
			_, _ = w.Write([]byte(`{"ID":"csi-1","Name":"data","Namespace":""}`))
			return
		}
		http.NotFound(w, r)
	}))
	ctx := context.Background()
	host, err := client.FindHostVolume(ctx, "data", "default")
	if err != nil || host == nil || host.ID != "host-1" {
		t.Fatalf("host discovery = %#v, %v", host, err)
	}
	csi, err := client.FindCSIVolume(ctx, "data", "")
	if err != nil || csi == nil || csi.ID != "csi-1" {
		t.Fatalf("CSI discovery = %#v, %v", csi, err)
	}
	if found, err := client.FindHostVolume(ctx, "missing", "default"); err != nil || found != nil {
		t.Fatalf("missing host discovery = %#v, %v", found, err)
	}
}

func TestEffectiveNamespaceUsesNomadDefault(t *testing.T) {
	for input, want := range map[string]string{"": "default", "default": "default", "team-a": "team-a"} {
		if got := effectiveNamespace(input); got != want {
			t.Fatalf("effectiveNamespace(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestIsNotFoundHandlesOnlyNomad404Errors(t *testing.T) {
	if isNotFound(nil) {
		t.Fatal("nil error reported as not found")
	}
	if isNotFound(errors.New("not found")) {
		t.Fatal("arbitrary error reported as Nomad not found")
	}
}
