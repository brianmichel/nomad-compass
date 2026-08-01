package reconcile

import (
	"testing"

	"github.com/hashicorp/nomad/api"
)

func TestJobDiffChangeDetectionTraversesNestedTasksAndIgnoresCompassMetadata(t *testing.T) {
	metadataOnly := &api.JobDiff{Fields: []*api.FieldDiff{
		{Name: "Meta[" + compassMetaCommit + "]"},
		{Name: "Meta[" + compassMetaCommitAuthor + "]"},
		{Name: "Meta[" + compassMetaCommitTitle + "]"},
	}}
	if jobDiffHasChanges(metadataOnly) || jobPlanHasChanges(&api.JobPlanResponse{Diff: metadataOnly}) {
		t.Fatal("Compass commit metadata should not trigger job changes")
	}
	if !jobPlanHasChanges(nil) {
		t.Fatal("nil plan should be treated as changed")
	}

	nestedChange := &api.JobDiff{TaskGroups: []*api.TaskGroupDiff{{Tasks: []*api.TaskDiff{{Fields: []*api.FieldDiff{{Name: "Config[image]"}}}}}}}
	if !jobDiffHasChanges(nestedChange) || !taskGroupDiffHasChanges(nestedChange.TaskGroups[0]) || !taskDiffHasChanges(nestedChange.TaskGroups[0].Tasks[0]) {
		t.Fatal("nested task field change was not detected")
	}
	if jobDiffHasChanges(&api.JobDiff{TaskGroups: []*api.TaskGroupDiff{{Tasks: []*api.TaskDiff{{Fields: metadataOnly.Fields}}}}}) {
		t.Fatal("nested Compass metadata should not trigger job changes")
	}

	for _, diff := range []*api.TaskGroupDiff{nil, {}, {Fields: []*api.FieldDiff{{Name: "Meta[" + compassMetaCommit + "]"}}}} {
		if taskGroupDiffHasChanges(diff) {
			t.Fatalf("metadata-only task group reported change: %#v", diff)
		}
	}
	for _, diff := range []*api.TaskDiff{nil, {}, {Fields: []*api.FieldDiff{{Name: "Meta[" + compassMetaCommitTitle + "]"}}}} {
		if taskDiffHasChanges(diff) {
			t.Fatalf("metadata-only task reported change: %#v", diff)
		}
	}
	if !taskDiffHasChanges(&api.TaskDiff{Objects: []*api.ObjectDiff{{}}}) {
		t.Fatal("task object change was not detected")
	}
}
