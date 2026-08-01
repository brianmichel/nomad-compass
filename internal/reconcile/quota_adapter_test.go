package reconcile

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestQuotaAdapterLifecycle(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "quota" {
  resource "quota" "apps" {
    description = "application quota"
    limit {
      region = "global"
      region_limit {
        cpu = 1000
        memory = 512
      }
    }
  }
}`), "quota.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	resource := bundle.Resources[0]
	adapter := quotaAdapter{}
	fake := &fakeNomad{}
	result, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || result.NomadID != "apps" {
		t.Fatalf("apply quota: %v %#v", err, result)
	}
	observation, err := adapter.Observe(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || observation != (plan.Observation{Present: true, Matches: true}) {
		t.Fatalf("observe quota: %v %#v", err, observation)
	}
	if _, err := adapter.Adopt(context.Background(), fake, resource); err != nil {
		t.Fatalf("adopt quota: %v", err)
	}
	if err := adapter.Delete(context.Background(), fake, storage.ManagedResource{NomadID: sql.NullString{String: result.NomadID, Valid: true}}); err != nil {
		t.Fatalf("delete quota: %v", err)
	}
}
