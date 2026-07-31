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

type quotaAdapter struct{}

func (quotaAdapter) Kind() string { return "quota" }

func (quotaAdapter) Validate(resource manifest.Resource) error {
	if _, err := manifest.CompileQuota(resource); err != nil {
		return fmt.Errorf("validate quota %q: %w", resource.Address, err)
	}
	return nil
}

func (quotaAdapter) Lookup(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (bool, error) {
	quota, err := lookup.ObserveQuota(ctx, resource.Name)
	return quota != nil, err
}

func (quotaAdapter) Observe(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, _ storage.ManagedResource) (plan.Observation, error) {
	actual, err := client.ObserveQuota(ctx, resource.Name)
	if err != nil || actual == nil {
		return plan.Observation{Present: actual != nil}, err
	}
	desired, err := manifest.CompileQuota(resource)
	if err != nil {
		return plan.Observation{}, err
	}
	return plan.Observation{Present: true, Matches: reflect.DeepEqual(desired, actual)}, nil
}

func (quotaAdapter) Replace(context.Context, nomadclient.ResourceClient, manifest.Resource, storage.ManagedResource) error {
	return nil
}

func (quotaAdapter) Apply(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (managedResourceResult, error) {
	desired, err := manifest.CompileQuota(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	if tracked.Address == "" {
		actual, err := client.ObserveQuota(ctx, desired.Name)
		if err != nil {
			return managedResourceResult{}, err
		}
		if actual != nil {
			return managedResourceResult{}, fmt.Errorf("quota %q already exists but is not managed by this repository", desired.Name)
		}
	}
	if err := client.ApplyQuota(ctx, desired); err != nil {
		return managedResourceResult{}, err
	}
	return managedResourceResult{NomadID: desired.Name}, nil
}

func (quotaAdapter) Delete(ctx context.Context, client nomadclient.ResourceClient, resource storage.ManagedResource) error {
	return client.DeleteQuota(ctx, resource.NomadID.String)
}

func (quotaAdapter) Adopt(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (managedResourceResult, error) {
	desired, err := manifest.CompileQuota(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	actual, err := lookup.ObserveQuota(ctx, desired.Name)
	if err != nil {
		return managedResourceResult{}, fmt.Errorf("find quota %q: %w", desired.Name, err)
	}
	if actual == nil {
		return managedResourceResult{}, fmt.Errorf("quota %q not found", desired.Name)
	}
	if !reflect.DeepEqual(desired, actual) {
		return managedResourceResult{}, fmt.Errorf("quota %q does not match desired bundle resource", desired.Name)
	}
	return managedResourceResult{NomadID: desired.Name}, nil
}
