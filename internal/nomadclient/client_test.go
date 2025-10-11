package nomadclient

import (
	"testing"
)

func TestDeriveStatus(t *testing.T) {
	cases := []struct {
		name   string
		status *JobStatus
		want   string
	}{
		{name: "nil", status: nil, want: ""},
		{name: "failed allocs", status: &JobStatus{FailedAllocs: 1}, want: "failed"},
		{name: "lost allocs", status: &JobStatus{LostAllocs: 1}, want: "lost"},
		{name: "healthy", status: &JobStatus{RunningAllocs: 2, DesiredAllocs: 2}, want: "healthy"},
		{name: "degraded", status: &JobStatus{RunningAllocs: 1, DesiredAllocs: 2}, want: "degraded"},
		{name: "deploying", status: &JobStatus{StartingAllocs: 1}, want: "deploying"},
		{name: "pending", status: &JobStatus{Status: "pending"}, want: "pending"},
		{name: "dead", status: &JobStatus{Status: "dead"}, want: "dead"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := deriveStatus(tc.status)
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestFormatAllocationSummary(t *testing.T) {
	status := &JobStatus{RunningAllocs: 3, DesiredAllocs: 5}
	if got := formatAllocationSummary(status); got != "3/5 allocations running" {
		t.Fatalf("unexpected summary: %s", got)
	}
	if got := formatAllocationSummary(nil); got != "" {
		t.Fatalf("expected empty summary for nil status")
	}
}

func TestDeriveStatusFromDeployment(t *testing.T) {
	status := &JobStatus{RunningAllocs: 1, DesiredAllocs: 1}
	if got, _ := deriveStatusFromDeployment(status, "successful"); got != "healthy" {
		t.Fatalf("expected healthy for successful deployment, got %q", got)
	}
	if got, _ := deriveStatusFromDeployment(status, "running"); got != "deploying" {
		t.Fatalf("expected deploying for running deployment, got %q", got)
	}
	if got, _ := deriveStatusFromDeployment(status, "failed"); got != "failed" {
		t.Fatalf("expected failed for failed deployment, got %q", got)
	}
	if got, _ := deriveStatusFromDeployment(status, "unknown"); got != "" {
		t.Fatalf("expected empty status for unknown deployment, got %q", got)
	}
}

func TestDerefString(t *testing.T) {
	primary := "value"
	fallback := "fallback"
	if got := derefString(&primary, &fallback); got != "value" {
		t.Fatalf("expected primary value, got %q", got)
	}
	primary = ""
	if got := derefString(&primary, &fallback); got != "fallback" {
		t.Fatalf("expected fallback value, got %q", got)
	}
	if got := derefString(nil, nil); got != "" {
		t.Fatalf("expected empty string when both nil")
	}
}
