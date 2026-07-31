package reconcile

import (
	"context"
	"database/sql"
	"testing"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestManagedResourceAdaptersCoverSupportedNonJobKinds(t *testing.T) {
	for _, kind := range []string{"volume", "acl_policy"} {
		adapter, ok := adapterFor(kind)
		if !ok || adapter.Kind() != kind {
			t.Fatalf("missing adapter for %q: %#v", kind, adapter)
		}
	}
}

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

func TestVolumeAdapterRejectsUnmanagedCollision(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    name = "data"
    type = "host"
    plugin_id = "mkdir"
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	fake := &fakeNomad{hostVolume: &api.HostVolume{ID: "data-id", Name: "data", PluginID: "mkdir"}}
	_, err = (volumeAdapter{}).Apply(context.Background(), fake, bundle.Resources[0], storage.ManagedResource{})
	if err == nil {
		t.Fatal("expected unmanaged host volume collision")
	}
}

func TestVolumeAdapterLifecycle(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    name = "data"
    type = "host"
    plugin_id = "mkdir"
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	resource := bundle.Resources[0]
	fake := &fakeNomad{}
	adapter := volumeAdapter{}
	result, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil {
		t.Fatalf("apply volume: %v", err)
	}
	tracked := storage.ManagedResource{
		Address:   resource.Address,
		NomadID:   sql.NullString{String: result.NomadID, Valid: true},
		Subtype:   sql.NullString{String: result.Subtype, Valid: true},
		Namespace: sql.NullString{String: result.Namespace, Valid: true},
	}
	observation, err := adapter.Observe(context.Background(), fake, resource, tracked)
	if err != nil || observation != (plan.Observation{Present: true, Matches: true}) {
		t.Fatalf("observe volume: %v %#v", err, observation)
	}
	if err := adapter.Replace(context.Background(), fake, resource, tracked); err == nil {
		t.Fatal("expected protected volume replacement to fail")
	}
	resource.DeleteMode = manifest.DeleteModeAllow
	if err := adapter.Replace(context.Background(), fake, resource, tracked); err != nil {
		t.Fatalf("replace volume: %v", err)
	}
	if len(fake.resourceCalls) != 2 || fake.resourceCalls[1] != "delete-volume:"+result.NomadID {
		t.Fatalf("unexpected volume lifecycle calls: %v", fake.resourceCalls)
	}
	result, err = adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil {
		t.Fatalf("reapply volume: %v", err)
	}
	adopted, err := adapter.Adopt(context.Background(), fake, resource)
	if err != nil || adopted.NomadID != result.NomadID {
		t.Fatalf("adopt volume: %v %#v", err, adopted)
	}
	tracked.NomadID = sql.NullString{String: result.NomadID, Valid: true}
	if err := adapter.Delete(context.Background(), fake, tracked); err != nil {
		t.Fatalf("delete volume: %v", err)
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

func TestVolumeAdapterRejectsDriftDuringAdoption(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    name = "data"
    type = "host"
    plugin_id = "mkdir"
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	fake := &fakeNomad{hostVolume: &api.HostVolume{ID: "data-id", Name: "data", PluginID: "other"}}
	_, err = (volumeAdapter{}).Adopt(context.Background(), fake, bundle.Resources[0])
	if err == nil {
		t.Fatal("expected host volume drift to reject adoption")
	}
}
