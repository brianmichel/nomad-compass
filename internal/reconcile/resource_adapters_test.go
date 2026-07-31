package reconcile

import (
	"context"
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
