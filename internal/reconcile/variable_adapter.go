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

type variableAdapter struct{}

func (variableAdapter) Kind() string { return "variable" }

func (variableAdapter) Validate(resource manifest.Resource) error {
	if _, err := manifest.CompileVariable(resource); err != nil {
		return fmt.Errorf("validate variable %q: %w", resource.Address, err)
	}
	return nil
}

func (variableAdapter) Lookup(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (bool, error) {
	variable, err := manifest.CompileVariable(resource)
	if err != nil {
		return false, err
	}
	found, err := lookup.ObserveVariable(ctx, variable.Namespace, variable.Path)
	return found != nil, err
}

func (variableAdapter) Observe(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, _ storage.ManagedResource) (plan.Observation, error) {
	desired, err := manifest.CompileVariable(resource)
	if err != nil {
		return plan.Observation{}, err
	}
	actual, err := client.ObserveVariable(ctx, desired.Namespace, desired.Path)
	if err != nil || actual == nil {
		return plan.Observation{Present: actual != nil}, err
	}
	return plan.Observation{Present: true, Matches: variableEquivalent(desired, actual)}, nil
}

func (variableAdapter) Replace(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) error {
	desired, err := manifest.CompileVariable(resource)
	if err != nil {
		return err
	}
	oldNamespace, oldPath := tracked.Namespace.String, tracked.NomadID.String
	if oldNamespace == desired.Namespace && oldPath == desired.Path {
		return nil
	}
	if resource.DeleteMode != manifest.DeleteModeAllow {
		return fmt.Errorf("variable identity changes are protected; set delete = %q to allow replacement", manifest.DeleteModeAllow)
	}
	if existing, err := client.ObserveVariable(ctx, desired.Namespace, desired.Path); err != nil {
		return err
	} else if existing != nil {
		return fmt.Errorf("variable %q already exists at the replacement identity", desired.Path)
	}
	if err := client.DeleteVariable(ctx, oldNamespace, oldPath); err != nil {
		return fmt.Errorf("delete old variable identity: %w", err)
	}
	return nil
}

func (variableAdapter) Apply(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (managedResourceResult, error) {
	desired, err := manifest.CompileVariable(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	if tracked.Address == "" {
		actual, err := client.ObserveVariable(ctx, desired.Namespace, desired.Path)
		if err != nil {
			return managedResourceResult{}, err
		}
		if actual != nil {
			return managedResourceResult{}, fmt.Errorf("variable %q already exists but is not managed by this repository", desired.Path)
		}
	}
	actual, err := client.ApplyVariable(ctx, desired)
	if err != nil {
		return managedResourceResult{}, err
	}
	if actual == nil {
		actual = desired
	}
	return managedResourceResult{NomadID: actual.Path, Namespace: actual.Namespace}, nil
}

func (variableAdapter) Delete(ctx context.Context, client nomadclient.ResourceClient, resource storage.ManagedResource) error {
	return client.DeleteVariable(ctx, resource.Namespace.String, resource.NomadID.String)
}

func (variableAdapter) Adopt(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (managedResourceResult, error) {
	desired, err := manifest.CompileVariable(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	actual, err := lookup.ObserveVariable(ctx, desired.Namespace, desired.Path)
	if err != nil {
		return managedResourceResult{}, fmt.Errorf("find variable %q: %w", desired.Path, err)
	}
	if actual == nil {
		return managedResourceResult{}, fmt.Errorf("variable %q not found", desired.Path)
	}
	if !variableEquivalent(desired, actual) {
		return managedResourceResult{}, fmt.Errorf("variable %q does not match desired bundle resource", desired.Path)
	}
	return managedResourceResult{NomadID: actual.Path, Namespace: actual.Namespace}, nil
}

func variableEquivalent(desired, actual *api.Variable) bool {
	return desired != nil && actual != nil && desired.Namespace == actual.Namespace && desired.Path == actual.Path && reflect.DeepEqual(desired.Items, actual.Items)
}
