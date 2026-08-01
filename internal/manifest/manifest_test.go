package manifest

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/nomad/jobspec2"
)

func TestParseEmbeddedResources(t *testing.T) {
	src := []byte(`bundle "compass" {
  resource "volume" "data" {
    delete = "protect"
    name = "compass-data"
    type = "host"
    depends_on = []
    capability {
      access_mode = "single-node-single-writer"
    }
  }

  resource "acl_policy" "compass" {
    description = "Compass policy"
    rules {
      namespace "default" {
        capabilities = ["read-job", "submit-job"]
      }
    }
  }

  resource "job" "compass" {
    depends_on = ["volume.data", "acl_policy.compass"]
    datacenters = ["jobs"]
    group "app" {
      task "server" {
        driver = "docker"
      }
    }
  }
}`)

	bundle, err := Parse(src, "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	if bundle.Name != "compass" {
		t.Fatalf("unexpected bundle name %q", bundle.Name)
	}
	if len(bundle.Resources) != 3 {
		t.Fatalf("expected three resources, got %d", len(bundle.Resources))
	}

	resources, err := bundle.OrderedResources()
	if err != nil {
		t.Fatalf("order resources: %v", err)
	}
	if got := resources[0].Address; got != "volume.data" {
		t.Fatalf("expected volume first, got %q", got)
	}
	if got := resources[1].Address; got != "acl_policy.compass" {
		t.Fatalf("expected policy second, got %q", got)
	}
	if got := resources[2].Address; got != "job.compass" {
		t.Fatalf("expected job last, got %q", got)
	}

	var job Resource
	for _, resource := range bundle.Resources {
		if resource.Address == "job.compass" {
			job = resource
		}
	}
	if job.DeleteMode != DeleteModeAllow {
		t.Fatalf("expected jobs to allow deletion by default, got %q", job.DeleteMode)
	}
	var volume, policy Resource
	for _, resource := range bundle.Resources {
		switch resource.Address {
		case "volume.data":
			volume = resource
		case "acl_policy.compass":
			policy = resource
		}
	}
	if volume.DeleteMode != DeleteModeProtect || policy.DeleteMode != DeleteModeProtect {
		t.Fatalf("expected destructive resources to be protected by default, got volume=%q policy=%q", volume.DeleteMode, policy.DeleteMode)
	}
	source, err := NativeJobSource(job)
	if err != nil {
		t.Fatalf("build native job source: %v", err)
	}
	parsed, err := jobspec2.Parse("compass.bundle.hcl#job.compass", bytes.NewReader(source))
	if err != nil {
		t.Fatalf("parse reconstructed job: %v\n%s", err, source)
	}
	if parsed == nil || parsed.Name == nil || *parsed.Name != "compass" {
		t.Fatalf("unexpected reconstructed job: %#v", parsed)
	}
}

func TestResourceHashesTrackSpecAndCompassMetadataSeparately(t *testing.T) {
	parse := func(deleteMode string) Resource {
		bundle, err := Parse([]byte(fmt.Sprintf(`bundle "compass" {
  resource "volume" "data" {
    delete = %q
    name = "compass-data"
    type = "host"
  }
}`, deleteMode)), "compass.bundle.hcl")
		if err != nil {
			t.Fatalf("parse bundle: %v", err)
		}
		return bundle.Resources[0]
	}

	protected := parse("protect")
	allowed := parse("allow")
	if SpecHash(protected) != SpecHash(allowed) {
		t.Fatal("expected deletion metadata to leave the Nomad spec hash unchanged")
	}
	if ManifestHash(protected) == ManifestHash(allowed) {
		t.Fatal("expected manifest hash to change when deletion metadata changes")
	}
}

func TestCompileHostVolumeAndACLPolicy(t *testing.T) {
	bundle, err := Parse([]byte(`bundle "compass" {
  resource "volume" "data" {
    name = "compass-data"
    type = "host"
    plugin_id = "mkdir"
    capability {
      access_mode = "single-node-single-writer"
      attachment_mode = "file-system"
    }
  }
  resource "acl_policy" "compass" {
    description = "Compass policy"
    rules {
      namespace "default" {
        capabilities = ["read-job", "submit-job"]
      }
    }
  }
}`), "compass.bundle.hcl")
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}

	volume, err := CompileVolume(bundle.Resources[0])
	if err != nil {
		t.Fatalf("compile host volume: %v", err)
	}
	if volume.Type != "host" || volume.Host == nil || volume.Host.Name != "compass-data" {
		t.Fatalf("unexpected host volume: %#v", volume)
	}
	if len(volume.Host.RequestedCapabilities) != 1 {
		t.Fatalf("expected one host capability, got %d", len(volume.Host.RequestedCapabilities))
	}

	policy, err := CompileACLPolicy(bundle.Resources[1])
	if err != nil {
		t.Fatalf("compile ACL policy: %v", err)
	}
	if policy.Name != "compass" || policy.Description != "Compass policy" {
		t.Fatalf("unexpected policy: %#v", policy)
	}
	if policy.Rules == "" || !strings.Contains(policy.Rules, `namespace "default"`) {
		t.Fatalf("unexpected policy rules: %q", policy.Rules)
	}
}

func TestCompileVolumeDecodesNativeHostAndCSIFieldsStrictly(t *testing.T) {
	bundle, err := Parse([]byte(`bundle "volumes" {
  resource "volume" "host" {
    type = "host"
    name = "data"
    capacity_min = "10GiB"
    capacity_max = "20GiB"
    node_pool = "apps"
    constraint {
      attribute = "${node.class}"
      operator = "="
      value = "storage"
    }
  }
  resource "volume" "csi" {
    type = "csi"
    id = "csi-data"
    name = "data"
    capacity_min = "1G"
    mount_options {
      fs_type = "ext4"
      mount_flags = ["ro", "noatime"]
    }
    topology_request {
      required {
        topology {
          segments { zone = "a" }
        }
      }
    }
  }
}`), "volumes.hcl")
	if err != nil {
		t.Fatal(err)
	}
	host, err := CompileVolume(bundle.Resources[0])
	if err != nil {
		t.Fatal(err)
	}
	if host.Host.RequestedCapacityMinBytes != 10*1024*1024*1024 || len(host.Host.Constraints) != 1 {
		t.Fatalf("unexpected host volume: %#v", host.Host)
	}
	csi, err := CompileVolume(bundle.Resources[1])
	if err != nil {
		t.Fatal(err)
	}
	if csi.CSI.RequestedCapacityMin == 0 || csi.CSI.MountOptions.FSType != "ext4" || len(csi.CSI.MountOptions.MountFlags) != 2 || len(csi.CSI.RequestedTopologies.Required) != 1 {
		t.Fatalf("unexpected CSI volume: %#v", csi.CSI)
	}

	bad, err := Parse([]byte(`bundle "bad" {
  resource "volume" "data" {
    type = "host"
    name = "data"
    typo = true
  }
}`), "bad.hcl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompileVolume(bad.Resources[0]); err == nil {
		t.Fatal("expected unknown volume field to be rejected")
	}
}

func TestParseRejectsDependencyCycle(t *testing.T) {
	_, err := Parse([]byte(`bundle "cycle" {
  resource "job" "a" { depends_on = ["job.b"] }
  resource "job" "b" { depends_on = ["job.a"] }
}`), "cycle.bundle.hcl")
	if err == nil {
		t.Fatal("expected dependency cycle error")
	}
}

func TestParseRejectsUnknownDependency(t *testing.T) {
	_, err := Parse([]byte(`bundle "invalid" {
  resource "job" "app" { depends_on = ["volume.missing"] }
}`), "invalid.bundle.hcl")
	if err == nil {
		t.Fatal("expected unknown dependency error")
	}
}
