package manifest

import (
	"strings"
	"testing"
)

func nativeResource(kind, name, body string) Resource {
	return Resource{Kind: kind, Name: name, Address: kind + "." + name, SourcePath: "bundle.hcl", Body: []byte(body)}
}

func TestCompileNamespaceDecodesNestedConfigurationAndMetadata(t *testing.T) {
	resource := nativeResource("namespace", "apps", `description = "Applications"
quota = "compute"
meta = { team = "platform" }

capabilities {
  enabled_task_drivers = ["docker"]
  disabled_network_modes = ["host"]
}

node_pool_config {
  default = "general"
  allowed = ["general", "gpu"]
  denied = ["legacy"]
}`)
	namespace, err := CompileNamespace(resource)
	if err != nil {
		t.Fatalf("compile namespace: %v", err)
	}
	if namespace.Name != "apps" || namespace.Description != "Applications" || namespace.Quota != "compute" || namespace.Meta["team"] != "platform" {
		t.Fatalf("unexpected namespace identity: %#v", namespace)
	}
	if namespace.Capabilities == nil || len(namespace.Capabilities.EnabledTaskDrivers) != 1 || namespace.NodePoolConfiguration == nil || namespace.NodePoolConfiguration.Default != "general" {
		t.Fatalf("nested namespace configuration was not decoded: %#v", namespace)
	}
}

func TestCompileNamespaceRejectsUnknownAndMalformedInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"unknown field", `description = "x"
unsupported = true`},
		{"malformed HCL", `description = [`},
		{"wrong nested field", `capabilities {
  typo = ["docker"]
}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := CompileNamespace(nativeResource("namespace", "apps", tc.body)); err == nil {
				t.Fatal("expected compilation error")
			}
		})
	}
}

func TestCompileQuotaDecodesLimitsAndRejectsIncompleteDefinitions(t *testing.T) {
	resource := nativeResource("quota", "compute", `description = "Compute quota"
limit {
  region = "global"
  variables_limit = 7
  region_limit {
    cpu = 500
    memory = 1024
    storage {
      variables = 32
      host_volumes = 64
    }
  }
}`)
	quota, err := CompileQuota(resource)
	if err != nil {
		t.Fatalf("compile quota: %v", err)
	}
	if quota.Name != "compute" || quota.Description != "Compute quota" || len(quota.Limits) != 1 {
		t.Fatalf("unexpected quota: %#v", quota)
	}
	limit := quota.Limits[0]
	if limit.Region != "global" || limit.VariablesLimit == nil || *limit.VariablesLimit != 7 || limit.RegionLimit == nil || limit.RegionLimit.CPU == nil || *limit.RegionLimit.CPU != 500 {
		t.Fatalf("unexpected quota limit: %#v", limit)
	}
	if limit.RegionLimit.Storage == nil || limit.RegionLimit.Storage.VariablesMB != 32 {
		t.Fatalf("storage quota was not decoded: %#v", limit.RegionLimit)
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{"missing limit", `description = "x"`},
		{"unknown quota field", `typo = true
limit { region = "global" }`},
		{"unknown limit field", `limit {
  region = "global"
  typo = true
}`},
		{"unknown region field", `limit {
  region = "global"
  region_limit { typo = 1 }
}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := CompileQuota(nativeResource("quota", "compute", tc.body)); err == nil {
				t.Fatal("expected quota compilation error")
			}
		})
	}
}

func TestCompileVariableDefaultsNamespaceAndRequiresItems(t *testing.T) {
	variable, err := CompileVariable(nativeResource("variable", "apps/config", `items = {
  environment = "test"
}`))
	if err != nil {
		t.Fatalf("compile variable: %v", err)
	}
	if variable.Path != "apps/config" || variable.Namespace != "default" || variable.Items["environment"] != "test" {
		t.Fatalf("unexpected defaulted variable: %#v", variable)
	}

	explicit, err := CompileVariable(nativeResource("variable", "config", `namespace = "apps"
path = "apps/config"
items = { enabled = true }`))
	if err != nil {
		t.Fatalf("compile explicit variable: %v", err)
	}
	if explicit.Namespace != "apps" || explicit.Path != "apps/config" {
		t.Fatalf("unexpected explicit identity: %#v", explicit)
	}

	for _, body := range []string{`namespace = "apps"`, `items = { key = "value" }
unknown = true`, `items = {}`} {
		if _, err := CompileVariable(nativeResource("variable", "config", body)); err == nil {
			t.Fatalf("expected variable compilation error for %q", body)
		}
	}
}

func TestCompileSentinelPolicyRequiresStringsAndRejectsBlocks(t *testing.T) {
	policy, err := CompileSentinelPolicy(nativeResource("sentinel_policy", "production", `description = "Production guard"
scope = "submit-job"
enforcement_level = "hard-mandatory"
policy = "main = rule { true }"`))
	if err != nil {
		t.Fatalf("compile Sentinel policy: %v", err)
	}
	if policy.Name != "production" || policy.Scope != "submit-job" || policy.EnforcementLevel != "hard-mandatory" || !strings.Contains(policy.Policy, "main") {
		t.Fatalf("unexpected Sentinel policy: %#v", policy)
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{"missing scope", `enforcement_level = "hard-mandatory"
policy = "main = rule { true }"`},
		{"missing policy", `scope = "submit-job"
enforcement_level = "hard-mandatory"`},
		{"unknown attribute", `scope = "submit-job"
enforcement_level = "hard-mandatory"
policy = "x"
typo = true`},
		{"unsupported block", `scope = "submit-job"
enforcement_level = "hard-mandatory"
policy = "x"
extra { value = true }`},
		{"non-string value", `scope = "submit-job"
enforcement_level = 1
policy = "x"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := CompileSentinelPolicy(nativeResource("sentinel_policy", "production", tc.body)); err == nil {
				t.Fatal("expected Sentinel compilation error")
			}
		})
	}
}

func TestNativeCompilersRejectWrongResourceKinds(t *testing.T) {
	resource := nativeResource("job", "wrong", "")
	checks := []func() error{
		func() error { _, err := CompileNamespace(resource); return err },
		func() error { _, err := CompileQuota(resource); return err },
		func() error { _, err := CompileVariable(resource); return err },
		func() error { _, err := CompileSentinelPolicy(resource); return err },
	}
	for _, check := range checks {
		if err := check(); err == nil {
			t.Fatal("expected wrong resource kind error")
		}
	}
}
