package reconcile

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestSentinelPolicyAdapterLifecycle(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "sentinel" {
  resource "sentinel_policy" "safe" {
    scope = "submit-job"
    enforcement_level = "soft-mandatory"
    policy = "main = rule { true }"
  }
}`), "sentinel.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	resource := bundle.Resources[0]
	adapter := sentinelPolicyAdapter{}
	fake := &fakeNomad{}
	result, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || result.NomadID != "safe" {
		t.Fatalf("apply Sentinel policy: %v %#v", err, result)
	}
	observation, err := adapter.Observe(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || observation != (plan.Observation{Present: true, Matches: true}) {
		t.Fatalf("observe Sentinel policy: %v %#v", err, observation)
	}
	if _, err := adapter.Adopt(context.Background(), fake, resource); err != nil {
		t.Fatalf("adopt Sentinel policy: %v", err)
	}
	if err := adapter.Delete(context.Background(), fake, storage.ManagedResource{NomadID: sql.NullString{String: result.NomadID, Valid: true}}); err != nil {
		t.Fatalf("delete Sentinel policy: %v", err)
	}
}
