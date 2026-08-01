package reconcile

import (
	"context"
	"fmt"
	"strings"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

type aclPolicyAdapter struct{}

func (aclPolicyAdapter) Kind() string { return "acl_policy" }

func (aclPolicyAdapter) Validate(resource manifest.Resource) error {
	if _, err := manifest.CompileACLPolicy(resource); err != nil {
		return fmt.Errorf("validate bundle ACL policy %q: %w", resource.Address, err)
	}
	return nil
}

func (aclPolicyAdapter) Lookup(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (bool, error) {
	policy, err := lookup.ObserveACLPolicy(ctx, resource.Name)
	return policy != nil, err
}

func (aclPolicyAdapter) Observe(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, _ storage.ManagedResource) (plan.Observation, error) {
	policy, err := client.ObserveACLPolicy(ctx, resource.Name)
	if err != nil || policy == nil {
		return plan.Observation{Present: policy != nil}, err
	}
	desired, err := manifest.CompileACLPolicy(resource)
	if err != nil {
		return plan.Observation{}, err
	}
	matches := policy.Description == desired.Description && strings.TrimSpace(policy.Rules) == strings.TrimSpace(desired.Rules)
	return plan.Observation{Present: true, Matches: matches}, nil
}

func (aclPolicyAdapter) Replace(context.Context, nomadclient.ResourceClient, manifest.Resource, storage.ManagedResource) error {
	return nil
}

func (aclPolicyAdapter) Apply(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (managedResourceResult, error) {
	policy, err := manifest.CompileACLPolicy(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	if tracked.Address == "" {
		existing, err := client.ObserveACLPolicy(ctx, resource.Name)
		if err != nil {
			return managedResourceResult{}, err
		}
		if existing != nil {
			return managedResourceResult{}, fmt.Errorf("ACL policy %q already exists but is not managed by this repository", resource.Name)
		}
	}
	if err := client.ApplyACLPolicy(ctx, policy); err != nil {
		return managedResourceResult{}, err
	}
	return managedResourceResult{NomadID: resource.Name}, nil
}

func (aclPolicyAdapter) Delete(ctx context.Context, client nomadclient.ResourceClient, resource storage.ManagedResource) error {
	return client.DeleteACLPolicy(ctx, resource.NomadID.String)
}

func (aclPolicyAdapter) Adopt(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (managedResourceResult, error) {
	policy, err := lookup.ObserveACLPolicy(ctx, resource.Name)
	if err != nil {
		return managedResourceResult{}, fmt.Errorf("find ACL policy %q: %w", resource.Name, err)
	}
	if policy == nil {
		return managedResourceResult{}, fmt.Errorf("ACL policy %q not found", resource.Name)
	}
	desired, err := manifest.CompileACLPolicy(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	if policy.Description != desired.Description || strings.TrimSpace(policy.Rules) != strings.TrimSpace(desired.Rules) {
		return managedResourceResult{}, fmt.Errorf("ACL policy %q does not match desired bundle resource", resource.Name)
	}
	return managedResourceResult{NomadID: resource.Name}, nil
}
