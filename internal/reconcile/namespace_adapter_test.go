package reconcile

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestNamespaceAdapterLifecycle(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "namespace" {
  resource "namespace" "apps" {
    description = "application namespace"
  }
}`), "namespace.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	resource := bundle.Resources[0]
	adapter := namespaceAdapter{}
	fake := &fakeNomad{}
	result, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || result.NomadID != "apps" {
		t.Fatalf("apply namespace: %v %#v", err, result)
	}
	tracked := storage.ManagedResource{Address: resource.Address}
	observation, err := adapter.Observe(context.Background(), fake, resource, tracked)
	if err != nil || observation != (plan.Observation{Present: true, Matches: true}) {
		t.Fatalf("observe namespace: %v %#v", err, observation)
	}
	if _, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{}); err == nil {
		t.Fatal("expected unmanaged namespace collision")
	}
	if _, err := adapter.Adopt(context.Background(), fake, resource); err != nil {
		t.Fatalf("adopt namespace: %v", err)
	}
	if err := adapter.Delete(context.Background(), fake, storage.ManagedResource{NomadID: sql.NullString{String: result.NomadID, Valid: true}}); err != nil {
		t.Fatalf("delete namespace: %v", err)
	}
}
