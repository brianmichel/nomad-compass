package plan

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestCompareTrackedUsesLiveStateAndProtection(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "apps" {
  resource "volume" "data" {
    name = "data"
    type = "host"
  }
  resource "job" "worker" {
    datacenters = ["dc1"]
  }
}`), "desired.bundle.hcl")
	if err != nil {
		t.Fatalf("parse desired bundle: %v", err)
	}
	tracked := []storage.ManagedResource{
		{Address: "volume.data", Kind: "volume", NomadID: stringValue("data"), ContentHash: stringValue(manifest.SpecHash(bundle.Resources[0])), ManifestHash: stringValue(manifest.ManifestHash(bundle.Resources[0])), Status: "applied", DeleteMode: "protect", Subtype: stringValue("host")},
		{Address: "volume.legacy", Kind: "volume", ContentHash: stringValue("old-spec"), ManifestHash: stringValue("old-manifest"), DeleteMode: "protect"},
	}
	observed := map[string]Observation{
		"volume.data": {Present: true, Matches: true},
	}
	result, err := CompareTracked(context.Background(), bundle, tracked, func(_ context.Context, resource manifest.Resource, _ storage.ManagedResource) (Observation, error) {
		return observed[resource.Address], nil
	})
	if err != nil {
		t.Fatalf("compare tracked: %v", err)
	}
	if result.Summary.Create != 1 || result.Summary.Unchanged != 1 || result.Summary.Protected != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if result.Resources[0].Action != "unchanged" || result.Resources[1].Action != "create" || result.Resources[2].Action != "protected" {
		t.Fatalf("unexpected actions: %+v", result.Resources)
	}
}

func TestCompareTrackedReportsUnmanagedCollision(t *testing.T) {
	bundle, err := manifest.Parse([]byte(`bundle "apps" {
  resource "job" "worker" {
    datacenters = ["dc1"]
  }
}`), "desired.bundle.hcl")
	if err != nil {
		t.Fatalf("parse desired bundle: %v", err)
	}
	result, err := CompareTrackedWithLookup(context.Background(), bundle, nil,
		func(context.Context, manifest.Resource, storage.ManagedResource) (Observation, error) {
			t.Fatal("observer called for an unmanaged resource")
			return Observation{}, nil
		},
		func(_ context.Context, resource manifest.Resource) (bool, error) {
			return resource.Address == "job.worker", nil
		})
	if err != nil {
		t.Fatalf("compare with lookup: %v", err)
	}
	if result.Summary.Conflict != 1 || result.Resources[0].Action != "conflict" {
		t.Fatalf("unexpected collision result: %+v", result)
	}
}

func TestCompareBundlesRemainsOffline(t *testing.T) {
	previous, err := manifest.Parse([]byte(`bundle "apps" {
  resource "job" "old" { datacenters = ["dc1"] }
}`), "previous.bundle.hcl")
	if err != nil {
		t.Fatalf("parse previous bundle: %v", err)
	}
	desired, err := manifest.Parse([]byte(`bundle "apps" {
  resource "job" "new" { datacenters = ["dc1"] }
}`), "desired.bundle.hcl")
	if err != nil {
		t.Fatalf("parse desired bundle: %v", err)
	}
	result := CompareBundles(desired, previous)
	if result.Summary.Create != 1 || result.Summary.Delete != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
}

func stringValue(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
