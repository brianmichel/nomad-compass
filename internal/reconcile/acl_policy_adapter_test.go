package reconcile

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestACLPolicyAdapterAppliesAndObserves(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "acl_policy" "web" {
    description = "web access"
    rules {
      namespace "default" {
        policy = "read"
      }
    }
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	resource := bundle.Resources[0]
	adapter := aclPolicyAdapter{}
	fake := &fakeNomad{}
	result, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || result.NomadID != "web" {
		t.Fatalf("apply ACL policy: %v %#v", err, result)
	}
	observation, err := adapter.Observe(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || observation != (plan.Observation{Present: true, Matches: true}) {
		t.Fatalf("observe ACL policy: %v %#v", err, observation)
	}
}

func TestACLPolicyAdapterLifecycle(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "acl_policy" "web" {
    description = "web access"
    rules {
      namespace "default" {
        policy = "read"
      }
    }
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	resource := bundle.Resources[0]
	fake := &fakeNomad{}
	adapter := aclPolicyAdapter{}
	result, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || result.NomadID != "web" {
		t.Fatalf("apply ACL policy: %v %#v", err, result)
	}
	if _, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{}); err == nil {
		t.Fatal("expected unmanaged ACL policy collision")
	}
	tracked := storage.ManagedResource{Address: resource.Address, NomadID: sql.NullString{String: result.NomadID, Valid: true}}
	observation, err := adapter.Observe(context.Background(), fake, resource, tracked)
	if err != nil || !observation.Matches {
		t.Fatalf("observe ACL policy: %v %#v", err, observation)
	}
	if err := adapter.Replace(context.Background(), fake, resource, tracked); err != nil {
		t.Fatalf("replace ACL policy: %v", err)
	}
	adopted, err := adapter.Adopt(context.Background(), fake, resource)
	if err != nil || adopted.NomadID != result.NomadID {
		t.Fatalf("adopt ACL policy: %v %#v", err, adopted)
	}
	if err := adapter.Delete(context.Background(), fake, tracked); err != nil {
		t.Fatalf("delete ACL policy: %v", err)
	}
}
