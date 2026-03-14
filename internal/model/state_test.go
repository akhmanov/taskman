package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTaskStateRoundTripPreservesLifecycleSummary(t *testing.T) {
	input := []byte(`version: 1
slug: cloud-api-auth
project: user-permissions
repo: cloud
status: active
labels: [auth, backend]
traits:
  mr: required
  worktree: required
created_at: 2026-03-14T10:05:00Z
updated_at: 2026-03-14T10:35:00Z
blocker: null
handoff: false
session:
  active: S-001
  last_completed: null
mr:
  status: draft
  reason: null
worktree:
  status: present
last_op:
  cmd: tasks.done
  ok: false
  step: validate_ready_mr
  error: merge request 123 is still draft
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

	if state.MR.Status != MRStatusDraft {
		t.Fatalf("mr status = %q", state.MR.Status)
	}

	if state.Worktree.Status != WorktreeStatusPresent {
		t.Fatalf("worktree status = %q", state.Worktree.Status)
	}

	output, err := yaml.Marshal(state)
	if err != nil {
		t.Fatalf("marshal task state: %v", err)
	}

	var roundTrip TaskState
	if err := yaml.Unmarshal(output, &roundTrip); err != nil {
		t.Fatalf("round trip unmarshal: %v", err)
	}

	if roundTrip.LastOp.Step != "validate_ready_mr" {
		t.Fatalf("last op step = %q", roundTrip.LastOp.Step)
	}
}

func TestProjectStateArchiveReadySummary(t *testing.T) {
	state := ProjectState{
		Version: 1,
		Slug:    "user-permissions",
		Status:  ProjectStatusDone,
		Archive: ArchiveState{
			Ready: true,
		},
	}

	if !state.Archive.Ready {
		t.Fatal("archive ready should be true")
	}
}
