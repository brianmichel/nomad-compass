package reconcile

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestVariableAdapterLifecycle(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "variable" {
  resource "variable" "apps/config" {
    namespace = "default"
    items = {
      environment = "test"
    }
  }
}`), "variable.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	resource := bundle.Resources[0]
	adapter := variableAdapter{}
	fake := &fakeNomad{}
	result, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || result.NomadID != "apps/config" {
		t.Fatalf("apply variable: %v %#v", err, result)
	}
	tracked := storage.ManagedResource{Address: resource.Address, NomadID: sql.NullString{String: result.NomadID, Valid: true}, Namespace: sql.NullString{String: result.Namespace, Valid: true}}
	observation, err := adapter.Observe(context.Background(), fake, resource, tracked)
	if err != nil || observation != (plan.Observation{Present: true, Matches: true}) {
		t.Fatalf("observe variable: %v %#v", err, observation)
	}
	if _, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{}); err == nil {
		t.Fatal("expected unmanaged variable collision")
	}
	if _, err := adapter.Adopt(context.Background(), fake, resource); err != nil {
		t.Fatalf("adopt variable: %v", err)
	}
	if err := adapter.Delete(context.Background(), fake, tracked); err != nil {
		t.Fatalf("delete variable: %v", err)
	}
}
