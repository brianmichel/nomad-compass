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

func parseResource(t *testing.T, source string, address string) manifest.Resource {
	t.Helper()
	bundle, err := manifest.Parse([]byte(source), "semantics.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	for _, resource := range bundle.Resources {
		if resource.Address == address {
			return resource
		}
	}
	t.Fatalf("resource %q not found", address)
	return manifest.Resource{}
}

func TestVariableReplacementProtectsIdentityAndRejectsDestinationCollision(t *testing.T) {
	resource := parseResource(t, `bundle "variables" {
  resource "variable" "config" {
    path = "new/config"
    items = { environment = "test" }
  }
}`, "variable.config")
	adapter := variableAdapter{}
	tracked := storage.ManagedResource{
		Address:   resource.Address,
		NomadID:   sql.NullString{String: "old/config", Valid: true},
		Namespace: sql.NullString{String: "default", Valid: true},
	}
	fake := &fakeNomad{}
	if err := adapter.Replace(context.Background(), fake, resource, tracked); err == nil {
		t.Fatal("expected protected variable identity change to fail")
	}
	resource.DeleteMode = manifest.DeleteModeAllow
	fake.variable = &api.Variable{Path: "new/config"}
	if err := adapter.Replace(context.Background(), fake, resource, tracked); err == nil {
		t.Fatal("expected replacement collision to fail")
	}
	fake.variable = &api.Variable{Path: "old/config"}
	if err := adapter.Replace(context.Background(), fake, resource, tracked); err != nil {
		t.Fatalf("replace variable identity: %v", err)
	}
	if len(fake.resourceCalls) != 1 || fake.resourceCalls[0] != "delete-variable:old/config" {
		t.Fatalf("unexpected replacement calls: %v", fake.resourceCalls)
	}
}

func TestQuotaComparisonIgnoresServerMetadataAndLimitOrdering(t *testing.T) {
	resource := parseResource(t, `bundle "quotas" {
  resource "quota" "compute" {
    limit {
      region = "global"
      region_limit { cpu = 100 }
    }
    limit {
      region = "secondary"
      region_limit { cpu = 200 }
    }
  }
}`, "quota.compute")
	fake := &fakeNomad{}
	adapter := quotaAdapter{}
	if _, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{}); err != nil {
		t.Fatalf("apply quota: %v", err)
	}
	fake.quota.CreateIndex = 11
	fake.quota.ModifyIndex = 12
	fake.quota.Limits[0], fake.quota.Limits[1] = fake.quota.Limits[1], fake.quota.Limits[0]
	fake.quota.Limits[0].Hash = []byte("server hash")
	observation, err := adapter.Observe(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || observation != (plan.Observation{Present: true, Matches: true}) {
		t.Fatalf("metadata/order-only quota drift: %v %#v", err, observation)
	}
	fake.quota.Description = "changed"
	observation, err = adapter.Observe(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || observation.Matches {
		t.Fatalf("semantic quota drift was not detected: %v %#v", err, observation)
	}
}

func TestSentinelComparisonIgnoresServerIndexesButDetectsPolicyDrift(t *testing.T) {
	resource := parseResource(t, `bundle "policies" {
  resource "sentinel_policy" "safe" {
    scope = "submit-job"
    enforcement_level = "soft-mandatory"
    policy = "main = rule { true }"
  }
}`, "sentinel_policy.safe")
	fake := &fakeNomad{}
	adapter := sentinelPolicyAdapter{}
	if _, err := adapter.Apply(context.Background(), fake, resource, storage.ManagedResource{}); err != nil {
		t.Fatalf("apply Sentinel policy: %v", err)
	}
	fake.sentinelPolicy.CreateIndex = 20
	fake.sentinelPolicy.ModifyIndex = 21
	observation, err := adapter.Observe(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || observation != (plan.Observation{Present: true, Matches: true}) {
		t.Fatalf("index-only Sentinel drift: %v %#v", err, observation)
	}
	fake.sentinelPolicy.Policy = "main = rule { false }"
	observation, err = adapter.Observe(context.Background(), fake, resource, storage.ManagedResource{})
	if err != nil || observation.Matches {
		t.Fatalf("policy drift was not detected: %v %#v", err, observation)
	}
}
