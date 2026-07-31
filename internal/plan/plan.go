// Package plan computes read-only desired-state plans for bundle resources.
package plan

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

// Observation is the read-only live state of one tracked resource.
type Observation struct {
	Present bool
	Matches bool
}

// Observer reports whether a tracked resource still matches the desired
// resource in the live control plane. It must not mutate state.
type Observer func(context.Context, manifest.Resource, storage.ManagedResource) (Observation, error)

// Lookup reports whether an unmanaged Nomad resource already exists by its
// desired name. It must not mutate state.
type Lookup func(context.Context, manifest.Resource) (bool, error)

// Report is the stable machine-readable representation of a bundle plan.
type Report struct {
	Bundle     string         `json:"bundle"`
	Revision   string         `json:"revision"`
	ComparedTo string         `json:"compared_to,omitempty"`
	Resources  []ResourcePlan `json:"resources"`
	Summary    Summary        `json:"summary"`
}

// ResourcePlan describes one desired-state transition.
type ResourcePlan struct {
	Action       string `json:"action"`
	Address      string `json:"address"`
	Kind         string `json:"kind"`
	DeleteMode   string `json:"delete_mode"`
	SpecHash     string `json:"spec_hash,omitempty"`
	ManifestHash string `json:"manifest_hash,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

// Summary counts the transitions in a plan.
type Summary struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Delete    int `json:"delete"`
	Protected int `json:"protected"`
	Unchanged int `json:"unchanged"`
	Conflict  int `json:"conflict"`
}

// CompareBundles compares two validated bundles without contacting Nomad.
func CompareBundles(desired, previous *manifest.Bundle) Report {
	result := Report{Bundle: desired.Name}
	desiredByAddress := make(map[string]manifest.Resource, len(desired.Resources))
	for _, resource := range desired.Resources {
		desiredByAddress[resource.Address] = resource
	}
	previousByAddress := make(map[string]manifest.Resource)
	if previous != nil {
		for _, resource := range previous.Resources {
			previousByAddress[resource.Address] = resource
		}
	}

	ordered, _ := desired.OrderedResources()
	for _, resource := range ordered {
		action := "create"
		reason := "resource is not present in the previous bundle"
		if old, exists := previousByAddress[resource.Address]; exists {
			action = "unchanged"
			reason = "native spec and Compass metadata are unchanged"
			if manifest.SpecHash(resource) != manifest.SpecHash(old) || manifest.ManifestHash(resource) != manifest.ManifestHash(old) {
				action = "update"
				reason = "native spec or Compass metadata changed"
				if resource.Kind == "volume" && resource.DeleteMode == manifest.DeleteModeProtect && manifest.SpecHash(resource) != manifest.SpecHash(old) {
					action = "protected"
					reason = "volume replacement is protected"
				}
			}
		}
		result.add(resourcePlan(resource, action, reason))
	}
	if previous != nil {
		orderedPrevious, _ := previous.OrderedResources()
		for _, resource := range orderedPrevious {
			if _, exists := desiredByAddress[resource.Address]; exists {
				continue
			}
			action := "delete"
			reason := "resource is absent from the desired bundle"
			if resource.DeleteMode == manifest.DeleteModeProtect {
				action = "protected"
				reason = "resource is absent but deletion is protected"
			}
			result.add(resourcePlan(resource, action, reason))
		}
	}
	return result
}

// CompareTracked compares a desired bundle with Compass's managed-resource
// state and live Nomad observations. It never writes to storage or Nomad.
func CompareTracked(ctx context.Context, desired *manifest.Bundle, tracked []storage.ManagedResource, observe Observer) (Report, error) {
	return CompareTrackedWithLookup(ctx, desired, tracked, observe, nil)
}

// CompareTrackedWithLookup also reports unmanaged Nomad name collisions.
func CompareTrackedWithLookup(ctx context.Context, desired *manifest.Bundle, tracked []storage.ManagedResource, observe Observer, lookup Lookup) (Report, error) {
	if desired == nil {
		return Report{}, fmt.Errorf("desired bundle is required")
	}
	if observe == nil {
		return Report{}, fmt.Errorf("resource observer is required")
	}
	trackedByAddress := make(map[string]storage.ManagedResource, len(tracked))
	for _, resource := range tracked {
		trackedByAddress[resource.Address] = resource
	}
	seen := make(map[string]struct{}, len(desired.Resources))
	ordered, err := desired.OrderedResources()
	if err != nil {
		return Report{}, err
	}
	result := Report{Bundle: desired.Name}
	for _, resource := range ordered {
		seen[resource.Address] = struct{}{}
		trackedResource, exists := trackedByAddress[resource.Address]
		if !exists {
			if lookup != nil {
				alreadyExists, err := lookup(ctx, resource)
				if err != nil {
					return Report{}, fmt.Errorf("lookup %s: %w", resource.Address, err)
				}
				if alreadyExists {
					result.add(resourcePlan(resource, "conflict", "resource exists in Nomad but is not managed by Compass"))
					continue
				}
			} else {
				observation, err := observe(ctx, resource, storage.ManagedResource{})
				if err != nil {
					return Report{}, fmt.Errorf("observe %s: %w", resource.Address, err)
				}
				if observation.Present {
					result.add(resourcePlan(resource, "conflict", "an unmanaged live resource already owns the desired identity"))
					continue
				}
			}
			result.add(resourcePlan(resource, "create", "resource is not managed by Compass"))
			continue
		}
		observation, err := observe(ctx, resource, trackedResource)
		if err != nil {
			return Report{}, fmt.Errorf("observe %s: %w", resource.Address, err)
		}
		action, reason := "unchanged", "native spec, Compass metadata, and live resource are unchanged"
		specChanged := !trackedResource.ContentHash.Valid || trackedResource.ContentHash.String != manifest.SpecHash(resource)
		manifestChanged := !trackedResource.ManifestHash.Valid || trackedResource.ManifestHash.String != manifest.ManifestHash(resource)
		if !observation.Present {
			action = "create"
			reason = "managed resource is missing from Nomad"
		} else if resource.Kind == "volume" && resource.DeleteMode == manifest.DeleteModeProtect && (specChanged || !observation.Matches) {
			action = "protected"
			reason = "volume replacement is protected"
		} else if !observation.Matches {
			action = "update"
			reason = "managed resource has drifted in Nomad"
		} else if specChanged || manifestChanged || trackedResource.Status != "applied" {
			action = "update"
			reason = "native spec, Compass metadata, or recorded status changed"
		}
		result.add(resourcePlan(resource, action, reason))
	}
	for _, resource := range tracked {
		if _, exists := seen[resource.Address]; exists {
			continue
		}
		action := "delete"
		reason := "resource is absent from the desired bundle"
		if resource.DeleteMode == string(manifest.DeleteModeProtect) {
			action = "protected"
			reason = "resource is absent but deletion is protected"
		}
		result.add(ResourcePlan{Action: action, Address: resource.Address, Kind: resource.Kind, DeleteMode: resource.DeleteMode, SpecHash: nullString(resource.ContentHash), ManifestHash: nullString(resource.ManifestHash), Reason: reason})
	}
	return result, nil
}

func resourcePlan(resource manifest.Resource, action, reason string) ResourcePlan {
	return ResourcePlan{Action: action, Address: resource.Address, Kind: resource.Kind, DeleteMode: string(resource.DeleteMode), SpecHash: manifest.SpecHash(resource), ManifestHash: manifest.ManifestHash(resource), Reason: reason}
}

func (r *Report) add(resource ResourcePlan) {
	r.Resources = append(r.Resources, resource)
	switch resource.Action {
	case "create":
		r.Summary.Create++
	case "update":
		r.Summary.Update++
	case "delete":
		r.Summary.Delete++
	case "protected":
		r.Summary.Protected++
	case "unchanged":
		r.Summary.Unchanged++
	case "conflict":
		r.Summary.Conflict++
	}
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
