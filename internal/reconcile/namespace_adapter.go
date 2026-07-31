package reconcile

import (
	"context"
	"fmt"
	"reflect"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

type namespaceAdapter struct{}

func (namespaceAdapter) Kind() string { return "namespace" }

func (namespaceAdapter) Validate(resource manifest.Resource) error {
	if _, err := manifest.CompileNamespace(resource); err != nil {
		return fmt.Errorf("validate namespace %q: %w", resource.Address, err)
	}
	return nil
}

func (namespaceAdapter) Lookup(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (bool, error) {
	namespace, err := lookup.ObserveNamespace(ctx, resource.Name)
	return namespace != nil, err
}

func (namespaceAdapter) Observe(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, _ storage.ManagedResource) (plan.Observation, error) {
	actual, err := client.ObserveNamespace(ctx, resource.Name)
	if err != nil || actual == nil {
		return plan.Observation{Present: actual != nil}, err
	}
	desired, err := manifest.CompileNamespace(resource)
	if err != nil {
		return plan.Observation{}, err
	}
	return plan.Observation{Present: true, Matches: namespaceEquivalent(desired, actual)}, nil
}

func (namespaceAdapter) Replace(context.Context, nomadclient.ResourceClient, manifest.Resource, storage.ManagedResource) error {
	return nil
}

func (namespaceAdapter) Apply(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (managedResourceResult, error) {
	desired, err := manifest.CompileNamespace(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	if tracked.Address == "" {
		actual, err := client.ObserveNamespace(ctx, desired.Name)
		if err != nil {
			return managedResourceResult{}, err
		}
		if actual != nil {
			return managedResourceResult{}, fmt.Errorf("namespace %q already exists but is not managed by this repository", desired.Name)
		}
	}
	if err := client.ApplyNamespace(ctx, desired); err != nil {
		return managedResourceResult{}, err
	}
	return managedResourceResult{NomadID: desired.Name}, nil
}

func (namespaceAdapter) Delete(ctx context.Context, client nomadclient.ResourceClient, resource storage.ManagedResource) error {
	return client.DeleteNamespace(ctx, resource.NomadID.String)
}

func (namespaceAdapter) Adopt(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (managedResourceResult, error) {
	desired, err := manifest.CompileNamespace(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	actual, err := lookup.ObserveNamespace(ctx, desired.Name)
	if err != nil {
		return managedResourceResult{}, fmt.Errorf("find namespace %q: %w", desired.Name, err)
	}
	if actual == nil {
		return managedResourceResult{}, fmt.Errorf("namespace %q not found", desired.Name)
	}
	if !namespaceEquivalent(desired, actual) {
		return managedResourceResult{}, fmt.Errorf("namespace %q does not match desired bundle resource", desired.Name)
	}
	return managedResourceResult{NomadID: desired.Name}, nil
}

func namespaceEquivalent(desired, actual *api.Namespace) bool {
	return desired != nil && actual != nil && desired.Name == actual.Name && desired.Description == actual.Description && desired.Quota == actual.Quota && reflect.DeepEqual(desired.Capabilities, actual.Capabilities) && reflect.DeepEqual(desired.NodePoolConfiguration, actual.NodePoolConfiguration) && reflect.DeepEqual(desired.VaultConfiguration, actual.VaultConfiguration) && reflect.DeepEqual(desired.ConsulConfiguration, actual.ConsulConfiguration) && reflect.DeepEqual(desired.Meta, actual.Meta)
}
