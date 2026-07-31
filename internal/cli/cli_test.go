package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBundleValidateJSONReportsDependencyOrderAndHashes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compass.bundle.hcl")
	contents := []byte(`bundle "example" {
  resource "volume" "data" {
    name = "example-data"
    type = "host"
  }

  resource "job" "app" {
    depends_on = ["volume.data"]
    datacenters = ["dc1"]
    group "app" {
      task "server" {
        driver = "docker"
      }
    }
  }
}`)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	var output bytes.Buffer
	if err := Run(context.Background(), []string{"bundle", "validate", "--file", path, "--format", "json"}, nil, &output, nil); err != nil {
		t.Fatalf("validate bundle: %v", err)
	}

	var report ValidationReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, output.String())
	}
	if !report.Valid || report.Bundle != "example" || len(report.Resources) != 2 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if report.Resources[0].Address != "volume.data" || report.Resources[1].Address != "job.app" {
		t.Fatalf("resources are not in dependency order: %#v", report.Resources)
	}
	if report.Resources[1].ManifestHash == "" || report.Resources[1].SpecHash == "" {
		t.Fatalf("expected hashes in report: %#v", report.Resources[1])
	}
	if len(report.Resources[1].DependsOn) != 1 || report.Resources[1].DependsOn[0] != "volume.data" {
		t.Fatalf("unexpected dependencies: %#v", report.Resources[1].DependsOn)
	}
}

func TestRunBundleValidateReadsStdinAndWritesText(t *testing.T) {
	input := strings.NewReader(`bundle "stdin" {
  resource "acl_policy" "read" {
    rules {
      namespace "default" {
        policy = "read"
      }
    }
  }
}`)
	var output bytes.Buffer
	if err := Run(context.Background(), []string{"bundle", "validate", "--file", "-"}, input, &output, nil); err != nil {
		t.Fatalf("validate stdin bundle: %v", err)
	}
	if !strings.Contains(output.String(), `valid bundle "stdin" (<stdin>)`) || !strings.Contains(output.String(), "acl_policy.read") {
		t.Fatalf("unexpected text report: %s", output.String())
	}
}

func TestRunBundleValidateRejectsInvalidResource(t *testing.T) {
	input := strings.NewReader(`bundle "invalid" {
  resource "acl_policy" "missing-rules" {
    description = "invalid"
  }
}`)
	var output bytes.Buffer
	err := Run(context.Background(), []string{"bundle", "validate", "--file", "-"}, input, &output, nil)
	if err == nil || !strings.Contains(err.Error(), "must contain a rules block") {
		t.Fatalf("expected validation error, got %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("expected no success output for invalid bundle: %s", output.String())
	}
}
