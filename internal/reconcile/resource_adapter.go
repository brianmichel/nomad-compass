package reconcile

import (
	"context"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

// managedResourceAdapter owns the lifecycle rules for one Nomad resource kind.
// Keeping one concrete adapter per kind makes decoding, comparison, adoption,
// replacement, and deletion changes local to that resource.
type managedResourceAdapter interface {
	Kind() string
	Validate(manifest.Resource) error
	Lookup(context.Context, nomadclient.ResourceLookup, manifest.Resource) (bool, error)
	Observe(context.Context, nomadclient.ResourceClient, manifest.Resource, storage.ManagedResource) (plan.Observation, error)
	Replace(context.Context, nomadclient.ResourceClient, manifest.Resource, storage.ManagedResource) error
	Apply(context.Context, nomadclient.ResourceClient, manifest.Resource, storage.ManagedResource) (managedResourceResult, error)
	Adopt(context.Context, nomadclient.ResourceLookup, manifest.Resource) (managedResourceResult, error)
	Delete(context.Context, nomadclient.ResourceClient, storage.ManagedResource) error
}

type managedResourceResult struct {
	NomadID   string
	Namespace string
	Subtype   string
}

var managedResourceAdapters = map[string]managedResourceAdapter{
	"volume":          volumeAdapter{},
	"acl_policy":      aclPolicyAdapter{},
	"namespace":       namespaceAdapter{},
	"quota":           quotaAdapter{},
	"variable":        variableAdapter{},
	"sentinel_policy": sentinelPolicyAdapter{},
}

func adapterFor(kind string) (managedResourceAdapter, bool) {
	adapter, ok := managedResourceAdapters[kind]
	return adapter, ok
}
