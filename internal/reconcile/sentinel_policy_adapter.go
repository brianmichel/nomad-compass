package reconcile

import (
	"context"
	"fmt"
	"reflect"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

type sentinelPolicyAdapter struct{}

func (sentinelPolicyAdapter) Kind() string { return "sentinel_policy" }

func (sentinelPolicyAdapter) Validate(resource manifest.Resource) error {
	if _, err := manifest.CompileSentinelPolicy(resource); err != nil {
		return fmt.Errorf("validate Sentinel policy %q: %w", resource.Address, err)
	}
	return nil
}

func (sentinelPolicyAdapter) Lookup(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (bool, error) {
	policy, err := lookup.ObserveSentinelPolicy(ctx, resource.Name)
	return policy != nil, err
}

func (sentinelPolicyAdapter) Observe(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, _ storage.ManagedResource) (plan.Observation, error) {
	actual, err := client.ObserveSentinelPolicy(ctx, resource.Name)
	if err != nil || actual == nil {
		return plan.Observation{Present: actual != nil}, err
	}
	desired, err := manifest.CompileSentinelPolicy(resource)
	if err != nil {
		return plan.Observation{}, err
	}
	return plan.Observation{Present: true, Matches: reflect.DeepEqual(desired, actual)}, nil
}

func (sentinelPolicyAdapter) Replace(context.Context, nomadclient.ResourceClient, manifest.Resource, storage.ManagedResource) error {
	return nil
}

func (sentinelPolicyAdapter) Apply(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (managedResourceResult, error) {
	desired, err := manifest.CompileSentinelPolicy(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	if tracked.Address == "" {
		actual, err := client.ObserveSentinelPolicy(ctx, desired.Name)
		if err != nil {
			return managedResourceResult{}, err
		}
		if actual != nil {
			return managedResourceResult{}, fmt.Errorf("Sentinel policy %q already exists but is not managed by this repository", desired.Name)
		}
	}
	if err := client.ApplySentinelPolicy(ctx, desired); err != nil {
		return managedResourceResult{}, err
	}
	return managedResourceResult{NomadID: desired.Name}, nil
}

func (sentinelPolicyAdapter) Delete(ctx context.Context, client nomadclient.ResourceClient, resource storage.ManagedResource) error {
	return client.DeleteSentinelPolicy(ctx, resource.NomadID.String)
}

func (sentinelPolicyAdapter) Adopt(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (managedResourceResult, error) {
	desired, err := manifest.CompileSentinelPolicy(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	actual, err := lookup.ObserveSentinelPolicy(ctx, desired.Name)
	if err != nil {
		return managedResourceResult{}, fmt.Errorf("find Sentinel policy %q: %w", desired.Name, err)
	}
	if actual == nil {
		return managedResourceResult{}, fmt.Errorf("Sentinel policy %q not found", desired.Name)
	}
	if !reflect.DeepEqual(desired, actual) {
		return managedResourceResult{}, fmt.Errorf("Sentinel policy %q does not match desired bundle resource", desired.Name)
	}
	return managedResourceResult{NomadID: desired.Name}, nil
}
