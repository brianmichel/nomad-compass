// Package manifest parses Compass bundle manifests.
package manifest

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

// DeleteMode controls what Compass may do when a resource disappears from a
// bundle. Destructive resources should normally use protect.
type DeleteMode string

const (
	DeleteModeAllow   DeleteMode = "allow"
	DeleteModeProtect DeleteMode = "protect"
)

// Resource is one desired Nomad resource inside a bundle. Body contains the
// native Nomad HCL body with Compass-only attributes removed.
type Resource struct {
	Kind       string
	Name       string
	Address    string
	SourcePath string
	Body       []byte
	DependsOn  []string
	DeleteMode DeleteMode
}

// Bundle is a validated collection of desired resources.
type Bundle struct {
	Name      string
	Path      string
	Resources []Resource
}

var supportedKinds = map[string]struct{}{
	"job":              {},
	"volume":           {},
	"acl_policy":       {},
	"namespace":        {},
	"node_pool":        {},
	"quota":            {},
	"scaling_policy":   {},
	"variable":         {},
	"sentinel_policy":  {},
	"acl_auth_method":  {},
	"acl_binding_rule": {},
	"acl_token":        {},
}

// Parse validates and parses one Compass bundle manifest.
func Parse(src []byte, path string) (*Bundle, error) {
	file, diags := hclwrite.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("parse bundle %s: %s", path, diags.Error())
	}

	root := file.Body()
	if len(root.Attributes()) != 0 {
		return nil, fmt.Errorf("bundle %s must not contain top-level attributes", path)
	}

	var bundleBlocks []*hclwrite.Block
	for _, block := range root.Blocks() {
		if block.Type() != "bundle" {
			return nil, fmt.Errorf("bundle %s contains unsupported top-level block %q", path, block.Type())
		}
		bundleBlocks = append(bundleBlocks, block)
	}
	if len(bundleBlocks) != 1 {
		return nil, fmt.Errorf("bundle %s must contain exactly one bundle block", path)
	}

	bundleBlock := bundleBlocks[0]
	labels := bundleBlock.Labels()
	if len(labels) != 1 || labels[0] == "" {
		return nil, fmt.Errorf("bundle %s must have exactly one non-empty label", path)
	}

	bundle := &Bundle{Name: labels[0], Path: path}
	seen := make(map[string]struct{})
	for _, block := range bundleBlock.Body().Blocks() {
		if block.Type() != "resource" {
			return nil, fmt.Errorf("bundle %q contains unsupported block %q", bundle.Name, block.Type())
		}
		resource, err := parseResource(block, path)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[resource.Address]; exists {
			return nil, fmt.Errorf("bundle %q contains duplicate resource %q", bundle.Name, resource.Address)
		}
		seen[resource.Address] = struct{}{}
		bundle.Resources = append(bundle.Resources, resource)
	}
	if len(bundle.Resources) == 0 {
		return nil, fmt.Errorf("bundle %q must contain at least one resource", bundle.Name)
	}

	for _, resource := range bundle.Resources {
		for _, dependency := range resource.DependsOn {
			if dependency == resource.Address {
				return nil, fmt.Errorf("resource %q cannot depend on itself", resource.Address)
			}
			if _, exists := seen[dependency]; !exists {
				return nil, fmt.Errorf("resource %q depends on unknown resource %q", resource.Address, dependency)
			}
		}
	}
	if _, err := bundle.OrderedResources(); err != nil {
		return nil, err
	}

	return bundle, nil
}

func parseResource(block *hclwrite.Block, path string) (Resource, error) {
	labels := block.Labels()
	if len(labels) != 2 || labels[0] == "" || labels[1] == "" {
		return Resource{}, fmt.Errorf("resource block in %s must have kind and name labels", path)
	}
	kind, name := labels[0], labels[1]
	if _, ok := supportedKinds[kind]; !ok {
		return Resource{}, fmt.Errorf("resource %q uses unsupported kind %q", name, kind)
	}

	body := block.Body()
	dependsOn, err := stringListAttribute(body, "depends_on", path)
	if err != nil {
		return Resource{}, fmt.Errorf("resource %s.%s: %w", kind, name, err)
	}
	deleteMode := defaultDeleteMode(kind)
	if attr := body.GetAttribute("delete"); attr != nil {
		value, err := stringAttribute(attr, path)
		if err != nil {
			return Resource{}, fmt.Errorf("resource %s.%s delete: %w", kind, name, err)
		}
		deleteMode = DeleteMode(value)
		if deleteMode != DeleteModeAllow && deleteMode != DeleteModeProtect {
			return Resource{}, fmt.Errorf("resource %s.%s has invalid delete mode %q", kind, name, value)
		}
	}

	body.RemoveAttribute("depends_on")
	body.RemoveAttribute("delete")
	return Resource{
		Kind:       kind,
		Name:       name,
		Address:    kind + "." + name,
		SourcePath: path,
		Body:       body.BuildTokens(nil).Bytes(),
		DependsOn:  dependsOn,
		DeleteMode: deleteMode,
	}, nil
}

func defaultDeleteMode(kind string) DeleteMode {
	switch kind {
	case "volume", "acl_policy":
		return DeleteModeProtect
	default:
		return DeleteModeAllow
	}
}

func stringAttribute(attr *hclwrite.Attribute, path string) (string, error) {
	expr, diags := hclsyntax.ParseExpression(attr.Expr().BuildTokens(nil).Bytes(), path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return "", errors.New(diags.Error())
	}
	value, diags := expr.Value(nil)
	if diags.HasErrors() {
		return "", errors.New(diags.Error())
	}
	if !value.IsKnown() || value.IsNull() || value.Type().FriendlyName() != "string" {
		return "", errors.New("expected a known string")
	}
	return value.AsString(), nil
}

func stringListAttribute(body *hclwrite.Body, name string, path string) ([]string, error) {
	attr := body.GetAttribute(name)
	if attr == nil {
		return nil, nil
	}
	expr, diags := hclsyntax.ParseExpression(attr.Expr().BuildTokens(nil).Bytes(), path, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, errors.New(diags.Error())
	}
	value, diags := expr.Value(nil)
	if diags.HasErrors() {
		return nil, errors.New(diags.Error())
	}
	if !value.IsKnown() || value.IsNull() || !value.CanIterateElements() {
		return nil, errors.New("expected a known list of strings")
	}

	result := make([]string, 0)
	iterator := value.ElementIterator()
	for iterator.Next() {
		_, item := iterator.Element()
		if !item.IsKnown() || item.IsNull() || item.Type().FriendlyName() != "string" {
			return nil, errors.New("expected a list of strings")
		}
		result = append(result, item.AsString())
	}
	return result, nil
}

// OrderedResources returns resources in dependency order.
func (b *Bundle) OrderedResources() ([]Resource, error) {
	if b == nil {
		return nil, errors.New("bundle is required")
	}
	byAddress := make(map[string]Resource, len(b.Resources))
	for _, resource := range b.Resources {
		byAddress[resource.Address] = resource
	}

	state := make(map[string]uint8, len(b.Resources))
	ordered := make([]Resource, 0, len(b.Resources))
	var visit func(string) error
	visit = func(address string) error {
		switch state[address] {
		case 1:
			return fmt.Errorf("bundle %q contains a dependency cycle at %q", b.Name, address)
		case 2:
			return nil
		}
		resource, ok := byAddress[address]
		if !ok {
			return fmt.Errorf("resource %q is not in bundle %q", address, b.Name)
		}
		state[address] = 1
		for _, dependency := range resource.DependsOn {
			if err := visit(dependency); err != nil {
				return err
			}
		}
		state[address] = 2
		ordered = append(ordered, resource)
		return nil
	}

	for _, resource := range b.Resources {
		if err := visit(resource.Address); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

// NativeJobSource wraps an embedded job body in the native Nomad job block
// expected by jobspec2.
func NativeJobSource(resource Resource) ([]byte, error) {
	if resource.Kind != "job" {
		return nil, fmt.Errorf("resource %q is not a job", resource.Address)
	}
	return []byte("job " + strconv.Quote(resource.Name) + " {\n" + string(resource.Body) + "}\n"), nil
}
