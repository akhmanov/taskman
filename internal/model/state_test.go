package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTaskStateRoundTripPreservesGenericLifecycleSummary(t *testing.T) {
	input := []byte(`version: 1
slug: cloud-api-auth
project: user-permissions
status: active
labels: [auth, backend]
vars:
  kind: feature
  service: auth
created_at: 2026-03-14T10:05:00Z
updated_at: 2026-03-14T10:35:00Z
session:
  active: S-001
  last_completed: null
last_op:
  cmd: tasks.transition
  ok: false
  step: validate_ready_review
  error: review transition still blocked
  at: 2026-03-14T10:35:00Z
`)

	var state TaskState
	if err := yaml.Unmarshal(input, &state); err != nil {
		t.Fatalf("unmarshal task state: %v", err)
	}

	if state.Slug != "cloud-api-auth" {
		t.Fatalf("slug = %q", state.Slug)
	}

	if state.Session.Active != "S-001" {
		t.Fatalf("active session = %q", state.Session.Active)
	}

	if state.Vars["kind"] != "feature" {
		t.Fatalf("task kind var = %q", state.Vars["kind"])
	}

	output, err := yaml.Marshal(state)
	if err != nil {
		t.Fatalf("marshal task state: %v", err)
	}

	var roundTrip TaskState
	if err := yaml.Unmarshal(output, &roundTrip); err != nil {
		t.Fatalf("round trip unmarshal: %v", err)
	}

	if roundTrip.LastOp.Step != "validate_ready_review" {
		t.Fatalf("last op step = %q", roundTrip.LastOp.Step)
	}
}

func TestProjectStateArchiveReadySummarySupportsGenericTaskCounts(t *testing.T) {
	state := ProjectState{
		Version: 1,
		Slug:    "user-permissions",
		Status:  ProjectStatusArchived,
		Tasks:   TaskCounts{"todo": 2, "in_review": 1, "closed": 4},
		Archive: ArchiveState{
			Ready: true,
		},
	}

	if !state.Archive.Ready {
		t.Fatal("archive ready should be true")
	}

	if got := state.Tasks["in_review"]; got != 1 {
		t.Fatalf("in_review count = %d, want 1", got)
	}
}
