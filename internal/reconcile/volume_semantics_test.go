package reconcile

import (
	"testing"

	"github.com/hashicorp/nomad/api"
)

func TestCSIVolumeEquivalentNormalizesServerRepresentations(t *testing.T) {
	desired := &api.CSIVolume{
		Name:                 "data",
		Namespace:            "default",
		ExternalID:           "external-1",
		AccessMode:           api.CSIVolumeAccessMode("single-node-single-writer"),
		AttachmentMode:       api.CSIVolumeAttachmentMode("file-system"),
		MountOptions:         &api.CSIMountOptions{FSType: "ext4", MountFlags: []string{"ro", "noatime"}},
		Secrets:              api.CSISecrets{"user": "secret"},
		Parameters:           map[string]string{"class": "fast"},
		Context:              map[string]string{"zone": "a"},
		RequestedCapacityMin: 100,
		RequestedCapabilities: []*api.CSIVolumeCapability{{
			AccessMode: api.CSIVolumeAccessMode("single-node-single-writer"), AttachmentMode: api.CSIVolumeAttachmentMode("file-system"),
		}},
		RequestedTopologies: &api.CSITopologyRequest{Required: []*api.CSITopology{{Segments: map[string]string{"zone": "a"}}}},
	}
	actual := *desired
	actual.ID = "server-assigned-id"
	actual.Namespace = ""
	actual.MountOptions = &api.CSIMountOptions{FSType: "ext4", MountFlags: []string{"noatime", "ro"}}
	actual.Secrets = api.CSISecrets{"user": "<redacted>"}
	actual.RequestedCapabilities = []*api.CSIVolumeCapability{nil, desired.RequestedCapabilities[0]}
	actual.CreateIndex = 42
	actual.ModifyIndex = 43
	if !csiVolumeEquivalent(desired, &actual) {
		t.Fatal("equivalent CSI representations were treated as drift")
	}

	cases := []struct {
		name   string
		mutate func(*api.CSIVolume)
	}{
		{"name", func(v *api.CSIVolume) { v.Name = "other" }},
		{"external ID", func(v *api.CSIVolume) { v.ExternalID = "other" }},
		{"mount options", func(v *api.CSIVolume) { v.MountOptions.MountFlags = []string{"rw"} }},
		{"secrets", func(v *api.CSIVolume) { v.Secrets = api.CSISecrets{"user": "wrong"} }},
		{"topology", func(v *api.CSIVolume) { v.RequestedTopologies.Required[0].Segments["zone"] = "b" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := actual
			changed.MountOptions = &api.CSIMountOptions{FSType: actual.MountOptions.FSType, MountFlags: append([]string(nil), actual.MountOptions.MountFlags...)}
			changed.Secrets = map[string]string{"user": "<redacted>"}
			changed.RequestedTopologies = &api.CSITopologyRequest{Required: []*api.CSITopology{{Segments: map[string]string{"zone": "a"}}}}
			tc.mutate(&changed)
			if csiVolumeEquivalent(desired, &changed) {
				t.Fatal("semantic CSI drift was not detected")
			}
		})
	}
}

func TestSecretsEquivalentAllowsRedactionAndOmittedLiveSecrets(t *testing.T) {
	if !secretsEquivalent(api.CSISecrets{"token": "secret"}, api.CSISecrets{}) {
		t.Fatal("omitted live secrets should not be treated as drift")
	}
	if !secretsEquivalent(api.CSISecrets{"token": "secret"}, api.CSISecrets{"token": "<redacted>"}) {
		t.Fatal("redacted live secret should not be treated as drift")
	}
	if secretsEquivalent(api.CSISecrets{"token": "secret"}, api.CSISecrets{"token": "wrong"}) {
		t.Fatal("different live secret should be treated as drift")
	}
}
