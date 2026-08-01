package manifest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	oldhcl "github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/ast"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/nomad/api"
	"github.com/mitchellh/mapstructure"
)

// VolumeSpec is the typed volume payload represented by a bundle resource.
type VolumeSpec struct {
	Type string
	Host *api.HostVolume
	CSI  *api.CSIVolume
}

var hostVolumeKeys = keySet("namespace", "id", "name", "plugin_id", "node_pool", "node_id", "capacity_min", "capacity_max", "capacity", "host_path", "parameters", "constraint", "capability", "type")
var csiVolumeKeys = keySet("id", "name", "external_id", "namespace", "plugin_id", "access_mode", "attachment_mode", "mount_options", "secrets", "parameters", "context", "capacity_min", "capacity_max", "capability", "clone_id", "snapshot_id", "topology_request", "type")

func keySet(keys ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		result[key] = struct{}{}
	}
	return result
}

// CompileVolume decodes the native volume HCL body into the Nomad API model.
// It uses an explicit schema because mapstructure's weak decoder otherwise
// silently drops native Nomad fields such as constraints and mount options.
func CompileVolume(resource Resource) (VolumeSpec, error) {
	if resource.Kind != "volume" {
		return VolumeSpec{}, fmt.Errorf("resource %q is not a volume", resource.Address)
	}
	file, err := oldhcl.Parse(string(resource.Body))
	if err != nil {
		return VolumeSpec{}, fmt.Errorf("parse volume %q: %w", resource.Address, err)
	}
	list, ok := file.Node.(*ast.ObjectList)
	if !ok {
		return VolumeSpec{}, fmt.Errorf("volume %q body must be an object", resource.Address)
	}
	values := map[string]interface{}{}
	if err := oldhcl.DecodeObject(&values, list); err != nil {
		return VolumeSpec{}, fmt.Errorf("decode volume %q: %w", resource.Address, err)
	}
	volumeType, ok := values["type"].(string)
	if !ok || volumeType == "" {
		return VolumeSpec{}, fmt.Errorf("volume %q must define type", resource.Address)
	}
	var allowed map[string]struct{}
	switch volumeType {
	case "host":
		allowed = hostVolumeKeys
	case "csi":
		allowed = csiVolumeKeys
	default:
		return VolumeSpec{}, fmt.Errorf("volume %q has unsupported type %q", resource.Address, volumeType)
	}
	if err := rejectUnknownKeys(values, allowed, "volume "+resource.Address); err != nil {
		return VolumeSpec{}, err
	}
	capabilityValue := values["capability"]
	constraintValue := values["constraint"]
	mountOptionsValue := values["mount_options"]
	topologyRequestValue := values["topology_request"]
	delete(values, "type")
	delete(values, "capability")
	delete(values, "constraint")
	delete(values, "mount_options")
	delete(values, "topology_request")
	capacityMin, err := parseCapacity(values, "capacity_min")
	if err != nil {
		return VolumeSpec{}, fmt.Errorf("volume %q capacity_min: %w", resource.Address, err)
	}
	capacityMax, err := parseCapacity(values, "capacity_max")
	if err != nil {
		return VolumeSpec{}, fmt.Errorf("volume %q capacity_max: %w", resource.Address, err)
	}
	capacity, err := parseCapacity(values, "capacity")
	if err != nil {
		return VolumeSpec{}, fmt.Errorf("volume %q capacity: %w", resource.Address, err)
	}
	delete(values, "capacity_min")
	delete(values, "capacity_max")
	delete(values, "capacity")

	if volumeType == "host" {
		volume := &api.HostVolume{}
		if err := mapstructure.Decode(values, volume); err != nil {
			return VolumeSpec{}, fmt.Errorf("decode host volume %q: %w", resource.Address, err)
		}
		volume.RequestedCapacityMinBytes = capacityMin
		volume.RequestedCapacityMaxBytes = capacityMax
		volume.CapacityBytes = capacity
		if err := decodeHostCapabilities(capabilityValue, &volume.RequestedCapabilities); err != nil {
			return VolumeSpec{}, fmt.Errorf("decode host volume %q capabilities: %w", resource.Address, err)
		}
		if err := decodeConstraints(constraintValue, &volume.Constraints); err != nil {
			return VolumeSpec{}, fmt.Errorf("decode host volume %q constraints: %w", resource.Address, err)
		}
		return VolumeSpec{Type: volumeType, Host: volume}, nil
	}

	volume := &api.CSIVolume{}
	if err := mapstructure.Decode(values, volume); err != nil {
		return VolumeSpec{}, fmt.Errorf("decode CSI volume %q: %w", resource.Address, err)
	}
	volume.RequestedCapacityMin = capacityMin
	volume.RequestedCapacityMax = capacityMax
	if err := decodeCSICapabilities(capabilityValue, &volume.RequestedCapabilities); err != nil {
		return VolumeSpec{}, fmt.Errorf("decode CSI volume %q capabilities: %w", resource.Address, err)
	}
	if volume.MountOptions, err = decodeMountOptions(mountOptionsValue); err != nil {
		return VolumeSpec{}, fmt.Errorf("decode CSI volume %q mount options: %w", resource.Address, err)
	}
	if volume.RequestedTopologies, err = decodeTopologyRequest(topologyRequestValue); err != nil {
		return VolumeSpec{}, fmt.Errorf("decode CSI volume %q topology request: %w", resource.Address, err)
	}
	return VolumeSpec{Type: volumeType, CSI: volume}, nil
}

func rejectUnknownKeys(values map[string]interface{}, allowed map[string]struct{}, context string) error {
	for key := range values {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%s contains unsupported field %q", context, key)
		}
	}
	return nil
}

func parseCapacity(values map[string]interface{}, key string) (int64, error) {
	value, ok := values[key]
	if !ok {
		return 0, nil
	}
	switch value := value.(type) {
	case int:
		if value < 0 {
			return 0, fmt.Errorf("capacity must be non-negative")
		}
		return int64(value), nil
	case int64:
		if value < 0 {
			return 0, fmt.Errorf("capacity must be non-negative")
		}
		return value, nil
	case float64:
		if value < 0 || value != float64(int64(value)) {
			return 0, fmt.Errorf("expected a non-negative whole byte count")
		}
		return int64(value), nil
	case string:
		value = strings.TrimSpace(value)
		if parsed, err := humanize.ParseBytes(value); err == nil {
			if parsed < 0 {
				return 0, fmt.Errorf("capacity must be non-negative")
			}
			return int64(parsed), nil
		}
		return strconv.ParseInt(value, 10, 64)
	default:
		return 0, fmt.Errorf("expected a byte count or human-readable string, got %T", value)
	}
}

func objectList(value interface{}) ([]map[string]interface{}, error) {
	if value == nil {
		return nil, nil
	}
	items, ok := value.([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected blocks, got %T", value)
	}
	return items, nil
}

func decodeHostCapabilities(value interface{}, target *[]*api.HostVolumeCapability) error {
	items, err := objectList(value)
	if err != nil {
		return err
	}
	for _, values := range items {
		if err := rejectUnknownKeys(values, keySet("attachment_mode", "access_mode"), "host capability"); err != nil {
			return err
		}
		capability := &api.HostVolumeCapability{}
		if err := mapstructure.Decode(values, capability); err != nil {
			return err
		}
		*target = append(*target, capability)
	}
	return nil
}

func decodeCSICapabilities(value interface{}, target *[]*api.CSIVolumeCapability) error {
	items, err := objectList(value)
	if err != nil {
		return err
	}
	for _, values := range items {
		if err := rejectUnknownKeys(values, keySet("attachment_mode", "access_mode"), "CSI capability"); err != nil {
			return err
		}
		capability := &api.CSIVolumeCapability{}
		if err := mapstructure.Decode(values, capability); err != nil {
			return err
		}
		*target = append(*target, capability)
	}
	return nil
}

func decodeConstraints(value interface{}, target *[]*api.Constraint) error {
	items, err := objectList(value)
	if err != nil {
		return err
	}
	for _, values := range items {
		if err := rejectUnknownKeys(values, keySet("attribute", "operator", "value"), "volume constraint"); err != nil {
			return err
		}
		constraint := &api.Constraint{}
		if err := mapstructure.Decode(values, constraint); err != nil {
			return err
		}
		*target = append(*target, constraint)
	}
	return nil
}

func decodeMountOptions(value interface{}) (*api.CSIMountOptions, error) {
	items, err := objectList(value)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) != 1 {
		return nil, fmt.Errorf("expected one mount_options block")
	}
	values := items[0]
	if err := rejectUnknownKeys(values, keySet("fs_type", "mount_flags"), "mount options"); err != nil {
		return nil, err
	}
	options := &api.CSIMountOptions{}
	if raw, exists := values["fs_type"]; exists {
		var ok bool
		options.FSType, ok = raw.(string)
		if !ok {
			return nil, fmt.Errorf("fs_type must be a string")
		}
	}
	if raw, exists := values["mount_flags"]; exists {
		items, ok := raw.([]interface{})
		if !ok {
			return nil, fmt.Errorf("mount_flags must be a list of strings")
		}
		for _, item := range items {
			flag, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("mount_flags must contain strings")
			}
			options.MountFlags = append(options.MountFlags, flag)
		}
	}
	return options, nil
}

func decodeTopologyRequest(value interface{}) (*api.CSITopologyRequest, error) {
	items, err := objectList(value)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	if len(items) != 1 {
		return nil, fmt.Errorf("expected one topology_request block")
	}
	request := &api.CSITopologyRequest{}
	for _, kind := range []string{"required", "preferred"} {
		blocks, err := objectList(items[0][kind])
		if err != nil {
			return nil, err
		}
		for _, group := range blocks {
			if err := rejectUnknownKeys(group, keySet("topology"), "topology_request "+kind); err != nil {
				return nil, err
			}
			topologies, err := objectList(group["topology"])
			if err != nil {
				return nil, err
			}
			for _, topologyBlock := range topologies {
				if err := rejectUnknownKeys(topologyBlock, keySet("segments"), "topology"); err != nil {
					return nil, err
				}
				segments, err := stringMap(topologyBlock["segments"])
				if err != nil {
					return nil, err
				}
				topology := &api.CSITopology{Segments: segments}
				if kind == "required" {
					request.Required = append(request.Required, topology)
				} else {
					request.Preferred = append(request.Preferred, topology)
				}
			}
		}
	}
	if err := rejectUnknownKeys(items[0], keySet("required", "preferred"), "topology_request"); err != nil {
		return nil, err
	}
	return request, nil
}

func stringMap(value interface{}) (map[string]string, error) {
	if value == nil {
		return nil, nil
	}
	objects, ok := value.([]map[string]interface{})
	if !ok || len(objects) != 1 {
		return nil, fmt.Errorf("expected one string map")
	}
	result := make(map[string]string, len(objects[0]))
	for key, raw := range objects[0] {
		value, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("map value %q must be a string", key)
		}
		result[key] = value
	}
	return result, nil
}

// CompileACLPolicy converts an embedded policy rules block into the raw HCL
// string required by Nomad's ACL policy API.
func CompileACLPolicy(resource Resource) (*api.ACLPolicy, error) {
	if resource.Kind != "acl_policy" {
		return nil, fmt.Errorf("resource %q is not an ACL policy", resource.Address)
	}
	file, diags := hclwrite.ParseConfig(resource.Body, resource.SourcePath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse ACL policy %q: %s", resource.Address, diags.Error())
	}
	body := file.Body()
	description := ""
	if attr := body.GetAttribute("description"); attr != nil {
		value, err := stringAttribute(attr, resource.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("ACL policy %q description: %w", resource.Address, err)
		}
		description = value
	}
	rules := body.FirstMatchingBlock("rules", nil)
	if rules == nil {
		return nil, fmt.Errorf("ACL policy %q must contain a rules block", resource.Address)
	}
	return &api.ACLPolicy{Name: resource.Name, Description: description, Rules: string(rules.Body().BuildTokens(nil).Bytes())}, nil
}
