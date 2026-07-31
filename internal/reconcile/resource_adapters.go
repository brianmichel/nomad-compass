package reconcile

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/nomad/api"

	"github.com/brianmichel/nomad-compass/internal/manifest"
	"github.com/brianmichel/nomad-compass/internal/nomadclient"
	"github.com/brianmichel/nomad-compass/internal/plan"
	"github.com/brianmichel/nomad-compass/internal/storage"
)

// managedResourceAdapter concentrates one Nomad resource kind's native
// decoding, observation, application, and adoption rules. Jobs remain on the
// existing jobspec path because their lifecycle includes deployment planning.
type managedResourceAdapter interface {
	Kind() string
	Validate(manifest.Resource) error
	Observe(context.Context, nomadclient.ResourceClient, manifest.Resource, storage.ManagedResource) (plan.Observation, error)
	Apply(context.Context, nomadclient.ResourceClient, manifest.Resource) (managedResourceResult, error)
	Adopt(context.Context, nomadclient.ResourceLookup, manifest.Resource) (managedResourceResult, error)
}

type managedResourceResult struct {
	NomadID   string
	Namespace string
	Subtype   string
}

var managedResourceAdapters = map[string]managedResourceAdapter{
	"volume":     volumeAdapter{},
	"acl_policy": aclPolicyAdapter{},
}

func adapterFor(kind string) (managedResourceAdapter, bool) {
	adapter, ok := managedResourceAdapters[kind]
	return adapter, ok
}

type volumeAdapter struct{}

func (volumeAdapter) Kind() string { return "volume" }

func (volumeAdapter) Validate(resource manifest.Resource) error {
	if _, err := manifest.CompileVolume(resource); err != nil {
		return fmt.Errorf("validate bundle volume %q: %w", resource.Address, err)
	}
	return nil
}

func (volumeAdapter) Observe(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (plan.Observation, error) {
	drifted, present, err := observeVolumeDrift(ctx, client, resource, tracked)
	return plan.Observation{Present: present, Matches: present && !drifted}, err
}

func (volumeAdapter) Apply(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource) (managedResourceResult, error) {
	spec, err := manifest.CompileVolume(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	switch spec.Type {
	case "host":
		volume, err := client.ApplyHostVolume(ctx, spec.Host)
		if err != nil {
			return managedResourceResult{}, err
		}
		if volume == nil {
			return managedResourceResult{}, fmt.Errorf("Nomad returned an empty host volume response")
		}
		return managedResourceResult{NomadID: volume.ID, Namespace: volume.Namespace, Subtype: "host"}, nil
	case "csi":
		if spec.CSI.ID == "" {
			spec.CSI.ID = resource.Name
		}
		volume, err := client.ApplyCSIVolume(ctx, spec.CSI)
		if err != nil {
			return managedResourceResult{}, err
		}
		if volume == nil {
			return managedResourceResult{}, fmt.Errorf("Nomad returned an empty CSI volume response")
		}
		return managedResourceResult{NomadID: volume.ID, Namespace: volume.Namespace, Subtype: "csi"}, nil
	default:
		return managedResourceResult{}, fmt.Errorf("volume %q has unsupported type %q", resource.Address, spec.Type)
	}
}

func (volumeAdapter) Adopt(ctx context.Context, lookup nomadclient.ResourceLookup, resource manifest.Resource) (managedResourceResult, error) {
	spec, err := manifest.CompileVolume(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	if spec.Type == "host" {
		volume, err := lookup.FindHostVolume(ctx, spec.Host.Name, spec.Host.Namespace)
		if err != nil {
			return managedResourceResult{}, fmt.Errorf("find host volume %q: %w", resource.Name, err)
		}
		if volume == nil {
			return managedResourceResult{}, fmt.Errorf("host volume %q not found", resource.Name)
		}
		if !hostVolumeEquivalent(spec.Host, volume) {
			return managedResourceResult{}, fmt.Errorf("host volume %q does not match desired bundle resource", resource.Name)
		}
		return managedResourceResult{NomadID: volume.ID, Namespace: volume.Namespace, Subtype: "host"}, nil
	}
	volume, err := lookup.FindCSIVolume(ctx, spec.CSI.Name, spec.CSI.Namespace)
	if err != nil {
		return managedResourceResult{}, fmt.Errorf("find CSI volume %q: %w", resource.Name, err)
	}
	if volume == nil {
		return managedResourceResult{}, fmt.Errorf("CSI volume %q not found", resource.Name)
	}
	if !csiVolumeEquivalent(spec.CSI, volume) {
		return managedResourceResult{}, fmt.Errorf("CSI volume %q does not match desired bundle resource", resource.Name)
	}
	return managedResourceResult{NomadID: volume.ID, Namespace: volume.Namespace, Subtype: "csi"}, nil
}

type aclPolicyAdapter struct{}

func (aclPolicyAdapter) Kind() string { return "acl_policy" }

func (aclPolicyAdapter) Validate(resource manifest.Resource) error {
	if _, err := manifest.CompileACLPolicy(resource); err != nil {
		return fmt.Errorf("validate bundle ACL policy %q: %w", resource.Address, err)
	}
	return nil
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

func (aclPolicyAdapter) Apply(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource) (managedResourceResult, error) {
	policy, err := manifest.CompileACLPolicy(resource)
	if err != nil {
		return managedResourceResult{}, err
	}
	if err := client.ApplyACLPolicy(ctx, policy); err != nil {
		return managedResourceResult{}, err
	}
	return managedResourceResult{NomadID: resource.Name}, nil
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

func observeVolumeDrift(ctx context.Context, client nomadclient.ResourceClient, resource manifest.Resource, tracked storage.ManagedResource) (drifted, present bool, err error) {
	spec, err := manifest.CompileVolume(resource)
	if err != nil {
		return false, false, err
	}
	if tracked.Subtype.Valid && tracked.Subtype.String == "csi" {
		volume, err := client.ObserveCSIVolume(ctx, tracked.NomadID.String, tracked.Namespace.String)
		if err != nil || volume == nil {
			return false, volume != nil, err
		}
		if spec.Type != "csi" {
			return true, true, nil
		}
		return !csiVolumeEquivalent(spec.CSI, volume), true, nil
	}
	volume, err := client.ObserveHostVolume(ctx, tracked.NomadID.String, tracked.Namespace.String)
	if err != nil || volume == nil {
		return false, volume != nil, err
	}
	if spec.Type != "host" {
		return true, true, nil
	}
	return !hostVolumeEquivalent(spec.Host, volume), true, nil
}

func hostVolumeEquivalent(desired, actual *api.HostVolume) bool {
	if desired == nil || actual == nil || desired.Name != actual.Name || effectiveNamespace(desired.Namespace) != effectiveNamespace(actual.Namespace) {
		return false
	}
	desiredPlugin, actualPlugin := desired.PluginID, actual.PluginID
	if desiredPlugin == "" {
		desiredPlugin = "mkdir"
	}
	if actualPlugin == "" {
		actualPlugin = "mkdir"
	}
	return actualPlugin == desiredPlugin && optionalEqual(desired.NodePool, actual.NodePool) && optionalEqual(desired.NodeID, actual.NodeID) &&
		reflect.DeepEqual(normalizedHostCapabilities(desired.RequestedCapabilities), normalizedHostCapabilities(actual.RequestedCapabilities)) &&
		reflect.DeepEqual(normalizedConstraints(desired.Constraints), normalizedConstraints(actual.Constraints)) &&
		reflect.DeepEqual(normalizedStringMap(desired.Parameters), normalizedStringMap(actual.Parameters)) &&
		desired.RequestedCapacityMinBytes == actual.RequestedCapacityMinBytes && desired.RequestedCapacityMaxBytes == actual.RequestedCapacityMaxBytes && (desired.CapacityBytes == 0 || desired.CapacityBytes == actual.CapacityBytes)
}

func csiVolumeEquivalent(desired, actual *api.CSIVolume) bool {
	if desired == nil || actual == nil || desired.ID != "" && desired.ID != actual.ID || desired.Name != actual.Name || effectiveNamespace(desired.Namespace) != effectiveNamespace(actual.Namespace) {
		return false
	}
	if !optionalEqual(desired.ExternalID, actual.ExternalID) || !optionalEqual(string(desired.AccessMode), string(actual.AccessMode)) || !optionalEqual(string(desired.AttachmentMode), string(actual.AttachmentMode)) || !optionalEqual(desired.PluginID, actual.PluginID) {
		return false
	}
	return reflect.DeepEqual(normalizedMountOptions(desired.MountOptions), normalizedMountOptions(actual.MountOptions)) && reflect.DeepEqual(normalizedStringMap(desired.Parameters), normalizedStringMap(actual.Parameters)) && reflect.DeepEqual(normalizedStringMap(desired.Context), normalizedStringMap(actual.Context)) && secretsEquivalent(desired.Secrets, actual.Secrets) && reflect.DeepEqual(normalizedCSICapabilities(desired.RequestedCapabilities), normalizedCSICapabilities(actual.RequestedCapabilities)) && reflect.DeepEqual(normalizedTopologyRequest(desired.RequestedTopologies), normalizedTopologyRequest(actual.RequestedTopologies)) && (desired.RequestedCapacityMin == 0 || desired.RequestedCapacityMin == actual.RequestedCapacityMin) && (desired.RequestedCapacityMax == 0 || desired.RequestedCapacityMax == actual.RequestedCapacityMax) && optionalEqual(desired.CloneID, actual.CloneID) && optionalEqual(desired.SnapshotID, actual.SnapshotID)
}

func effectiveNamespace(namespace string) string {
	if namespace == "" {
		return "default"
	}
	return namespace
}
func optionalEqual(desired, actual string) bool { return desired == "" || desired == actual }
func normalizedStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	return values
}
func secretsEquivalent(desired, actual api.CSISecrets) bool {
	if len(actual) == 0 && len(desired) > 0 {
		return true
	}
	if len(desired) != len(actual) {
		return false
	}
	for key, value := range desired {
		actualValue, ok := actual[key]
		if !ok || actualValue != "<redacted>" && actualValue != value {
			return false
		}
	}
	return true
}
func normalizedHostCapabilities(values []*api.HostVolumeCapability) []string {
	result := []string{}
	for _, value := range values {
		if value != nil {
			result = append(result, string(value.AccessMode)+"/"+string(value.AttachmentMode))
		}
	}
	sort.Strings(result)
	return result
}
func normalizedCSICapabilities(values []*api.CSIVolumeCapability) []string {
	result := []string{}
	for _, value := range values {
		if value != nil {
			result = append(result, string(value.AccessMode)+"/"+string(value.AttachmentMode))
		}
	}
	sort.Strings(result)
	return result
}
func normalizedConstraints(values []*api.Constraint) []string {
	result := []string{}
	for _, value := range values {
		if value != nil {
			result = append(result, value.LTarget+"/"+value.Operand+"/"+value.RTarget)
		}
	}
	sort.Strings(result)
	return result
}
func normalizedMountOptions(value *api.CSIMountOptions) *api.CSIMountOptions {
	if value == nil {
		return &api.CSIMountOptions{}
	}
	copy := *value
	copy.MountFlags = append([]string(nil), value.MountFlags...)
	sort.Strings(copy.MountFlags)
	return &copy
}
func normalizedTopologyRequest(value *api.CSITopologyRequest) *api.CSITopologyRequest {
	if value == nil {
		return &api.CSITopologyRequest{}
	}
	copy := &api.CSITopologyRequest{}
	for _, group := range []struct {
		source []*api.CSITopology
		target *[]*api.CSITopology
	}{{value.Required, &copy.Required}, {value.Preferred, &copy.Preferred}} {
		for _, topology := range group.source {
			if topology != nil {
				*group.target = append(*group.target, &api.CSITopology{Segments: normalizedStringMap(topology.Segments)})
			}
		}
	}
	return copy
}
