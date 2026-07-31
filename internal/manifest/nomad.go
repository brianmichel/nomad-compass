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
		if err := strictDecode(values, volume); err != nil {
			return VolumeSpec{}, fmt.Errorf("decode host volume %q: %w", resource.Address, err)
		}
		if err := decodeHostCapabilities(list, &volume.RequestedCapabilities); err != nil {
			return VolumeSpec{}, fmt.Errorf("decode host volume %q capabilities: %w", resource.Address, err)
		}
		return VolumeSpec{Type: volumeType, Host: volume}, nil
	case "csi":
		volume := &api.CSIVolume{}
		if err := strictDecode(values, volume); err != nil {
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

func strictDecode(values map[string]interface{}, result interface{}) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		ErrorUnused:      true,
		Result:           result,
	})
	if err != nil {
		return err
	}
	return decoder.Decode(values)
}

func decodeHostCapabilities(list *ast.ObjectList, target *[]*api.HostVolumeCapability) error {
	for _, item := range list.Filter("capability").Elem().Items {
		values := map[string]interface{}{}
		if err := oldhcl.DecodeObject(&values, item.Val); err != nil {
			return err
		}
		capability := &api.HostVolumeCapability{}
		if err := strictDecode(values, capability); err != nil {
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
		if err := strictDecode(values, capability); err != nil {
			return err
		}
		*target = append(*target, capability)
	}
	return nil
}

// CompileNamespace decodes a native Nomad namespace body.
func CompileNamespace(resource Resource) (*api.Namespace, error) {
	if resource.Kind != "namespace" {
		return nil, fmt.Errorf("resource %q is not a namespace", resource.Address)
	}
	values, err := decodeNativeValues(resource, map[string]struct{}{
		"description": {}, "quota": {}, "capabilities": {}, "node_pool_config": {}, "vault": {}, "consul": {}, "meta": {},
	})
	if err != nil {
		return nil, err
	}
	namespace := &api.Namespace{Name: resource.Name}
	if err := strictDecode(values, namespace); err != nil {
		return nil, fmt.Errorf("decode namespace %q: %w", resource.Address, err)
	}
	return namespace, nil
}

// CompileQuota decodes a native Nomad quota body.
func CompileQuota(resource Resource) (*api.QuotaSpec, error) {
	if resource.Kind != "quota" {
		return nil, fmt.Errorf("resource %q is not a quota", resource.Address)
	}
	values, err := decodeNativeValues(resource, map[string]struct{}{"description": {}, "limit": {}})
	if err != nil {
		return nil, err
	}
	quota := &api.QuotaSpec{Name: resource.Name}
	delete(values, "limit")
	if err := strictDecode(values, quota); err != nil {
		return nil, fmt.Errorf("decode quota %q: %w", resource.Address, err)
	}
	file, err := oldhcl.Parse(string(resource.Body))
	if err != nil {
		return nil, fmt.Errorf("parse quota %q: %w", resource.Address, err)
	}
	list, ok := file.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("quota %q body must be an object", resource.Address)
	}
	for _, item := range list.Filter("limit").Elem().Items {
		limitValues := map[string]interface{}{}
		if err := oldhcl.DecodeObject(&limitValues, item.Val); err != nil {
			return nil, fmt.Errorf("decode quota %q limit: %w", resource.Address, err)
		}
		var regionLimit *api.QuotaResources
		if nested, ok := item.Val.(*ast.ObjectList); ok {
			for _, regionItem := range nested.Filter("region_limit").Elem().Items {
				regionValues := map[string]interface{}{}
				if err := oldhcl.DecodeObject(&regionValues, regionItem.Val); err != nil {
					return nil, fmt.Errorf("decode quota %q region limit: %w", resource.Address, err)
				}
				regionLimit = &api.QuotaResources{}
				if err := strictDecode(regionValues, regionLimit); err != nil {
					return nil, fmt.Errorf("decode quota %q region limit: %w", resource.Address, err)
				}
				break
			}
		}
		delete(limitValues, "region_limit")
		var input struct {
			Region         string `mapstructure:"region"`
			VariablesLimit *int   `mapstructure:"variables_limit"`
		}
		if err := strictDecode(limitValues, &input); err != nil {
			return nil, fmt.Errorf("decode quota %q limit: %w", resource.Address, err)
		}
		quota.Limits = append(quota.Limits, &api.QuotaLimit{Region: input.Region, RegionLimit: regionLimit, VariablesLimit: input.VariablesLimit})
	}
	if len(quota.Limits) == 0 {
		return nil, fmt.Errorf("quota %q must contain a limit block", resource.Address)
	}
	return quota, nil
}

// CompileVariable decodes a native Nomad variable body. The resource name is
// the default path, allowing the Compass address to remain stable if the body
// is moved between files.
func CompileVariable(resource Resource) (*api.Variable, error) {
	if resource.Kind != "variable" {
		return nil, fmt.Errorf("resource %q is not a variable", resource.Address)
	}
	values, err := decodeNativeValues(resource, map[string]struct{}{"namespace": {}, "path": {}, "items": {}})
	if err != nil {
		return nil, err
	}
	variable := &api.Variable{Path: resource.Name}
	if err := strictDecode(values, variable); err != nil {
		return nil, fmt.Errorf("decode variable %q: %w", resource.Address, err)
	}
	if variable.Path == "" {
		variable.Path = resource.Name
	}
	if len(variable.Items) == 0 {
		return nil, fmt.Errorf("variable %q must contain items", resource.Address)
	}
	return variable, nil
}

func decodeNativeValues(resource Resource, allowed map[string]struct{}) (map[string]interface{}, error) {
	file, err := oldhcl.Parse(string(resource.Body))
	if err != nil {
		return nil, fmt.Errorf("parse %s %q: %w", resource.Kind, resource.Address, err)
	}
	list, ok := file.Node.(*ast.ObjectList)
	if !ok {
		return nil, fmt.Errorf("%s %q body must be an object", resource.Kind, resource.Address)
	}
	values := map[string]interface{}{}
	if err := oldhcl.DecodeObject(&values, list); err != nil {
		return nil, fmt.Errorf("decode %s %q: %w", resource.Kind, resource.Address, err)
	}
	for name := range values {
		if _, ok := allowed[name]; !ok {
			return nil, fmt.Errorf("%s %q has unsupported attribute or block %q", resource.Kind, resource.Address, name)
		}
	}
	return values, nil
}

// CompileSentinelPolicy decodes a native Nomad Sentinel policy body.
func CompileSentinelPolicy(resource Resource) (*api.SentinelPolicy, error) {
	if resource.Kind != "sentinel_policy" {
		return nil, fmt.Errorf("resource %q is not a Sentinel policy", resource.Address)
	}
	file, diags := hclwrite.ParseConfig(resource.Body, resource.SourcePath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse Sentinel policy %q: %s", resource.Address, diags.Error())
	}
	body := file.Body()
	allowed := map[string]struct{}{"description": {}, "scope": {}, "enforcement_level": {}, "policy": {}}
	for name := range body.Attributes() {
		if _, ok := allowed[name]; !ok {
			return nil, fmt.Errorf("Sentinel policy %q has unsupported attribute %q", resource.Address, name)
		}
	}
	read := func(name string, required bool) (string, error) {
		attr := body.GetAttribute(name)
		if attr == nil {
			if required {
				return "", fmt.Errorf("Sentinel policy %q must define %s", resource.Address, name)
			}
			return "", nil
		}
		value, err := stringAttribute(attr, resource.SourcePath)
		if err != nil {
			return "", fmt.Errorf("Sentinel policy %q %s: %w", resource.Address, name, err)
		}
		return value, nil
	}
	description, err := read("description", false)
	if err != nil {
		return nil, err
	}
	scope, err := read("scope", true)
	if err != nil {
		return nil, err
	}
	enforcement, err := read("enforcement_level", true)
	if err != nil {
		return nil, err
	}
	policy, err := read("policy", true)
	if err != nil {
		return nil, err
	}
	return &api.SentinelPolicy{Name: resource.Name, Description: description, Scope: scope, EnforcementLevel: enforcement, Policy: policy}, nil
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
	for name := range body.Attributes() {
		if name != "description" {
			return nil, fmt.Errorf("ACL policy %q has unsupported attribute %q", resource.Address, name)
		}
	}
	rulesBlocks := 0
	for _, block := range body.Blocks() {
		if block.Type() != "rules" {
			return nil, fmt.Errorf("ACL policy %q has unsupported block %q", resource.Address, block.Type())
		}
		rulesBlocks++
	}
	if rulesBlocks == 0 {
		return nil, fmt.Errorf("ACL policy %q must contain a rules block", resource.Address)
	}
	if rulesBlocks > 1 {
		return nil, fmt.Errorf("ACL policy %q must contain exactly one rules block", resource.Address)
	}
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
