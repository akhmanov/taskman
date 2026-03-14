package lifecycle

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/akhmanov/taskman/internal/model"
	"github.com/akhmanov/taskman/internal/steps"
	"github.com/akhmanov/taskman/internal/store"
)

func TestProjectArchiveFailsWhenTaskIsNotTerminal(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars: {}
workflow:
  task:
    statuses: [todo, active, done, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
  project:
    archive:
      steps:
        - name: noop
          cmd: [./bin/noop]
`)
	writeExecutable(t, filepath.Join(root, "bin", "noop"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"archive ok\"}'\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("active")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	service := NewProjectService(s, steps.New(root))
	_, err := service.Archive("user-permissions")
	if err == nil {
		t.Fatal("expected archive failure")
	}

	project, err := s.LoadProject("user-permissions")
	if err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if project.Archive.Ready {
		t.Fatal("archive should not be ready")
	}
	if len(project.Archive.Blockers) == 0 {
		t.Fatal("expected archive blockers")
	}
}

func TestProjectArchiveMovesProjectWhenTasksAreTerminal(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars: {}
workflow:
  task:
    statuses: [todo, active, done, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      close:
        from: [done]
        to: closed
  project:
    archive:
      steps:
        - name: noop
          cmd: [./bin/noop]
`)
	writeExecutable(t, filepath.Join(root, "bin", "noop"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"archive ok\"}'\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("closed")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	service := NewProjectService(s, steps.New(root))
	project, err := service.Archive("user-permissions")
	if err != nil {
		t.Fatalf("archive project: %v", err)
	}

	if project.Status != model.ProjectStatusArchived {
		t.Fatalf("project status = %q", project.Status)
	}

	archivedPath := filepath.Join(root, "projects", "archive", "2026", "user-permissions", "state.yaml")
	if _, err := os.Stat(archivedPath); err != nil {
		t.Fatalf("archived project state missing: %v", err)
	}
}

func TestProjectServiceUpdateBriefAndEventsPersistBoundedPayloadMemory(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars:
      area: identity
  task:
    labels: []
    vars: {}
vars:
  project:
    area:
      allowed: [identity, product]
workflow:
  task:
    statuses: [todo, active, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
`)

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{
		Version: 1,
		Slug:    "user-permissions",
		Status:  model.ProjectStatusActive,
		Labels:  []string{"platform"},
		Vars:    map[string]string{"area": "identity"},
	}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewProjectService(s, steps.New(root))

	updated, err := svc.Update("user-permissions", []string{"auth", "api"}, map[string]string{"area": "product"}, nil)
	if err != nil {
		t.Fatalf("update project: %v", err)
	}
	if !reflect.DeepEqual(updated.Labels, []string{"auth", "api"}) {
		t.Fatalf("labels = %#v", updated.Labels)
	}
	if updated.Vars["area"] != "product" {
		t.Fatalf("area var = %q", updated.Vars["area"])
	}

	loadedProject, err := s.LoadProject("user-permissions")
	if err != nil {
		t.Fatalf("load project after update: %v", err)
	}
	if !reflect.DeepEqual(loadedProject.Labels, []string{"auth", "api"}) {
		t.Fatalf("persisted labels = %#v", loadedProject.Labels)
	}

	wantBrief := "# Mission\n\nUpdate scope and constraints.\n\n# Boundaries\n\n## In Scope\n\nPayload memory.\n\n## Out of Scope\n\nKnowledge base.\n\n# Glossary\n\n- Brief\n\n# Shared Decisions\n\n- Use typed events.\n\n# Active Risks\n\n- Drift.\n\n# Tasking Rules\n\n- Keep project digest derived.\n\n# References\n\n- plan://payload-layer\n"
	if err := svc.SetBrief("user-permissions", wantBrief); err != nil {
		t.Fatalf("set project brief: %v", err)
	}
	gotBrief, err := svc.GetBrief("user-permissions")
	if err != nil {
		t.Fatalf("get project brief: %v", err)
	}
	if gotBrief != wantBrief {
		t.Fatalf("brief = %q", gotBrief)
	}

	event := model.PayloadEvent{
		ID:        "EVT-001",
		At:        "2026-03-14T10:35:00Z",
		Type:      model.PayloadEventTypeDecision,
		Summary:   "Adopt bounded payload memory",
		Actor:     "taskman",
		Session:   "S-001",
		Refs:      []string{"doc://plan", "task://user-permissions/api-auth"},
		Rationale: "Bounded decision memory needs explicit history",
		Impact:    "Tasks can reference project digest",
		Status:    "active",
	}
	if err := svc.AddEvent("user-permissions", event); err != nil {
		t.Fatalf("add project event: %v", err)
	}
	events, err := svc.GetEvents("user-permissions")
	if err != nil {
		t.Fatalf("get project events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d", len(events))
	}
	if events[0].Type != model.PayloadEventTypeDecision {
		t.Fatalf("event type = %q", events[0].Type)
	}
	if events[0].Session != "S-001" {
		t.Fatalf("event session = %q", events[0].Session)
	}
}

func TestProjectServiceUpdateSupportsUnsetVarAndRevalidatesRequiredVars(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars:
      area: identity
      owner: platform
  task:
    labels: []
    vars: {}
vars:
  project:
    area:
      required: true
      allowed: [identity, product]
    owner:
      allowed: [platform, core]
workflow:
  task:
    statuses: [todo, active, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
`)

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{
		Version: 1,
		Slug:    "user-permissions",
		Status:  model.ProjectStatusActive,
		Vars:    map[string]string{"area": "identity", "owner": "platform"},
	}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewProjectService(s, steps.New(root))

	updated, err := svc.Update("user-permissions", nil, nil, []string{"owner"})
	if err != nil {
		t.Fatalf("update project with unset: %v", err)
	}
	if _, ok := updated.Vars["owner"]; ok {
		t.Fatalf("owner should be unset: %#v", updated.Vars)
	}

	_, err = svc.Update("user-permissions", nil, nil, []string{"area"})
	if err == nil {
		t.Fatal("expected required var validation error after unset")
	}
}

func TestProjectServiceAddEventRejectsUnknownType(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars: {}
workflow:
  task:
    statuses: [todo, active, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
`)

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewProjectService(s, steps.New(root))
	err := svc.AddEvent("user-permissions", model.PayloadEvent{
		ID:      "EVT-001",
		At:      "2026-03-14T10:35:00Z",
		Type:    model.PayloadEventType("unknown"),
		Summary: "unknown",
		Actor:   "taskman",
	})
	if err == nil {
		t.Fatal("expected unknown type error")
	}
}

func TestProjectServiceSetBriefRejectsNonTemplateContent(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars: {}
workflow:
  task:
    statuses: [todo, active, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
`)
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewProjectService(s, steps.New(root))
	if err := svc.SetBrief("user-permissions", "freeform text"); err == nil {
		t.Fatal("expected invalid project brief error")
	}
}

func TestProjectServiceAddDecisionEventRejectsMissingDecisionFields(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars: {}
workflow:
  task:
    statuses: [todo, active, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
`)
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewProjectService(s, steps.New(root))
	err := svc.AddEvent("user-permissions", model.PayloadEvent{ID: "EVT-009", At: "2026-03-14T12:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "Adopt digest", Actor: "taskman", Status: "active"})
	if err == nil {
		t.Fatal("expected invalid decision event error")
	}
}
