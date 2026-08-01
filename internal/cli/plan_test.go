package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBundlePlanProducesCompactHumanSummary(t *testing.T) {
	dir := t.TempDir()
	previousPath := filepath.Join(dir, "previous.bundle.hcl")
	desiredPath := filepath.Join(dir, "desired.bundle.hcl")
	previous := []byte(`bundle "apps" {
  resource "volume" "data" {
    name = "data"
    type = "host"
  }
  resource "volume" "legacy" {
    name = "legacy"
    type = "host"
  }
  resource "acl_policy" "web" {
    rules {
      namespace "default" { policy = "read" }
    }
  }
  resource "job" "legacy" {
    datacenters = ["dc1"]
    group "legacy" {
      task "legacy" {
        driver = "docker"
      }
    }
  }
}`)
	desired := []byte(`bundle "apps" {
  resource "volume" "data" {
    name = "data"
    type = "host"
  }
  resource "acl_policy" "web" {
    description = "updated"
    rules {
      namespace "default" { policy = "write" }
    }
  }
  resource "job" "worker" {
    datacenters = ["dc1"]
    group "worker" {
      task "worker" {
        driver = "docker"
      }
    }
  }
}`)
	if err := os.WriteFile(previousPath, previous, 0o600); err != nil {
		t.Fatalf("write previous bundle: %v", err)
	}
	if err := os.WriteFile(desiredPath, desired, 0o600); err != nil {
		t.Fatalf("write desired bundle: %v", err)
	}

	var output bytes.Buffer
	err := Run(context.Background(), []string{
		"bundle", "plan", "--file", desiredPath, "--against", previousPath,
		"--revision", "a1b2c3d",
	}, nil, &output, nil)
	if err != nil {
		t.Fatalf("plan bundle: %v", err)
	}
	text := output.String()
	for _, expected := range []string{
		"Bundle: apps\nRevision: a1b2c3d",
		"+ job.worker",
		"~ acl_policy.web",
		"= volume.data",
		"- job.legacy",
		"! volume.legacy deletion protected",
		"Plan:\n  1 create\n  1 update\n  1 delete\n  1 protected\n  1 unchanged",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("plan output missing %q:\n%s", expected, text)
		}
	}
}
