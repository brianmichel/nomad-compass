package reconcile

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestObserveBundleJobReportsMissingAndNomadPlanDrift(t *testing.T) {
	resource := manifest.Resource{Kind: "job", Name: "api", Address: "job.api", SourcePath: "bundle.hcl", Body: []byte("datacenters = [\"dc1\"]\n")}
	tracked := storage.ManagedResource{NomadID: sql.NullString{String: "api", Valid: true}}
	fake := &fakeNomad{jobStatuses: map[string]*nomadclient.JobStatus{"api": {ID: "api", Exists: true}}, planResponses: map[string]*api.JobPlanResponse{"api": {}}}
	manager := &Manager{nomad: fake}
	observation, err := manager.observeBundleJob(context.Background(), resource, tracked)
	if err != nil || observation != (plan.Observation{Present: true, Matches: true}) {
		t.Fatalf("unchanged bundle job = %#v, %v", observation, err)
	}
	fake.planResponses["api"] = &api.JobPlanResponse{Diff: &api.JobDiff{Fields: []*api.FieldDiff{{Name: "Datacenters"}}}}
	observation, err = manager.observeBundleJob(context.Background(), resource, tracked)
	if err != nil || observation.Matches {
		t.Fatalf("drifted bundle job = %#v, %v", observation, err)
	}
	fake.jobStatuses["api"] = &nomadclient.JobStatus{ID: "api", Exists: false}
	observation, err = manager.observeBundleJob(context.Background(), resource, tracked)
	if err != nil || observation.Present {
		t.Fatalf("missing bundle job = %#v, %v", observation, err)
	}
	fake.jobStatusErr = errors.New("permission denied")
	fake.jobStatuses["api"] = &nomadclient.JobStatus{ID: "api", Exists: true}
	if _, err := manager.observeBundleJob(context.Background(), resource, tracked); err == nil {
		t.Fatal("expected job status error")
	}
}
