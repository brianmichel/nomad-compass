package manifest

import (
	"fmt"

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

// CompileVolume decodes the native volume HCL body into the Nomad API model.
// It intentionally accepts the same top-level shape used by nomad volume
// create/register files.
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
	delete(values, "type")
	delete(values, "capability")

	switch volumeType {
	case "host":
		volume := &api.HostVolume{}
		if err := mapstructure.WeakDecode(values, volume); err != nil {
			return VolumeSpec{}, fmt.Errorf("decode host volume %q: %w", resource.Address, err)
		}
		if err := decodeHostCapabilities(list, &volume.RequestedCapabilities); err != nil {
			return VolumeSpec{}, fmt.Errorf("decode host volume %q capabilities: %w", resource.Address, err)
		}
		return VolumeSpec{Type: volumeType, Host: volume}, nil
	case "csi":
		volume := &api.CSIVolume{}
		if err := mapstructure.WeakDecode(values, volume); err != nil {
			return VolumeSpec{}, fmt.Errorf("decode CSI volume %q: %w", resource.Address, err)
		}
		if err := decodeCSICapabilities(list, &volume.RequestedCapabilities); err != nil {
			return VolumeSpec{}, fmt.Errorf("decode CSI volume %q capabilities: %w", resource.Address, err)
		}
		return VolumeSpec{Type: volumeType, CSI: volume}, nil
	default:
		return VolumeSpec{}, fmt.Errorf("volume %q has unsupported type %q", resource.Address, volumeType)
	}
}

func decodeHostCapabilities(list *ast.ObjectList, target *[]*api.HostVolumeCapability) error {
	for _, item := range list.Filter("capability").Elem().Items {
		values := map[string]interface{}{}
		if err := oldhcl.DecodeObject(&values, item.Val); err != nil {
			return err
		}
		capability := &api.HostVolumeCapability{}
		if err := mapstructure.WeakDecode(values, capability); err != nil {
			return err
		}
		*target = append(*target, capability)
	}
	return nil
}

func decodeCSICapabilities(list *ast.ObjectList, target *[]*api.CSIVolumeCapability) error {
	for _, item := range list.Filter("capability").Elem().Items {
		values := map[string]interface{}{}
		if err := oldhcl.DecodeObject(&values, item.Val); err != nil {
			return err
		}
		capability := &api.CSIVolumeCapability{}
		if err := mapstructure.WeakDecode(values, capability); err != nil {
			return err
		}
		*target = append(*target, capability)
	}
	return nil
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
	return &api.ACLPolicy{
		Name:        resource.Name,
		Description: description,
		Rules:       string(rules.Body().BuildTokens(nil).Bytes()),
	}, nil
}
