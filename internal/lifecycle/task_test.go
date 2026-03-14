package lifecycle

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/steps"
	"github.com/assistant-wi/taskman/internal/store"
)

func TestTaskServiceCreateScaffoldsInitialSessionWithoutRunningTransitions(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "start-ran")
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars:
      kind: feature
vars:
  task:
    kind:
      allowed: [feature, chore]
naming:
  task_slug: "{{ .name }}"
workflow:
  task:
    statuses: [todo, active, done, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
        steps:
          - name: initialize
            cmd: [./bin/start]
`)
	writeExecutable(t, filepath.Join(root, "bin", "start"), "#!/bin/sh\ntouch \""+marker+"\"\necho '{\"ok\":true,\"message\":\"started\"}'\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	task, err := svc.Create("user-permissions", "api-auth", nil, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if task.Slug != "api-auth" {
		t.Fatalf("task slug = %q", task.Slug)
	}

	if task.Status != model.TaskStatus("todo") {
		t.Fatalf("task status = %q, want todo", task.Status)
	}

	if task.Session.Active != "S-001" {
		t.Fatalf("active session = %q, want S-001", task.Session.Active)
	}

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("start marker should not exist, stat err = %v", err)
	}

	sessionPath := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "api-auth", "sessions", "S-001", "state.yaml")
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("session state missing: %v", err)
	}

	artifactPath := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "api-auth", "artifacts", "worktree.yaml")
	if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
		t.Fatalf("artifact should not exist after create, stat err = %v", err)
	}
}

func TestTaskServiceTransitionPersistsFailedOperationWithoutChangingStatus(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars:
      kind: feature
vars:
  task:
    kind:
      allowed: [feature, chore]
workflow:
  task:
    statuses: [todo, active, done, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      complete:
        from: [active]
        to: done
        steps:
          - name: validate_ready_review
            cmd: [./bin/fail-review]
`)
	writeExecutable(t, filepath.Join(root, "bin", "fail-review"), "#!/bin/sh\necho '{\"ok\":false,\"message\":\"review still blocked\"}'\nexit 1\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{
		Version: 1,
		Slug:    "api-auth",
		Project: "user-permissions",
		Status:  model.TaskStatus("active"),
		Vars:    map[string]string{"kind": "feature"},
		Session: model.TaskSessionState{Active: "S-001"},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	_, err := svc.Transition("user-permissions", "api-auth", "complete")
	if err == nil {
		t.Fatal("expected transition error")
	}

	loaded, err := s.LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if loaded.Status != model.TaskStatus("active") {
		t.Fatalf("status = %q, want active", loaded.Status)
	}

	if loaded.LastOp.OK {
		t.Fatal("last op should be failed")
	}

	if loaded.LastOp.Step != "validate_ready_review" {
		t.Fatalf("last op step = %q", loaded.LastOp.Step)
	}
}

func TestTaskServiceTransitionPersistsArtifactsAndTerminalSessionState(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars:
      kind: feature
vars:
  task:
    kind:
      allowed: [feature, chore]
workflow:
  task:
    statuses: [todo, active, done, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      close:
        from: [done]
        to: closed
        steps:
          - name: persist_summary
            cmd: [./bin/close-task]
`)
	writeExecutable(t, filepath.Join(root, "bin", "close-task"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"closed\",\"artifacts\":{\"summary\":{\"state\":\"archived\"}}}'\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{
		Version: 1,
		Slug:    "api-auth",
		Project: "user-permissions",
		Status:  model.TaskStatus("done"),
		Vars:    map[string]string{"kind": "feature"},
		Session: model.TaskSessionState{Active: "S-001"},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}
	if err := s.SaveSession("user-permissions", "api-auth", model.SessionState{Version: 1, ID: "S-001", Task: "api-auth", Status: "done"}); err != nil {
		t.Fatalf("save session: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	task, err := svc.Transition("user-permissions", "api-auth", "close")
	if err != nil {
		t.Fatalf("transition task: %v", err)
	}

	if task.Status != model.TaskStatus("closed") {
		t.Fatalf("task status = %q, want closed", task.Status)
	}
	if task.Session.Active != "" {
		t.Fatalf("active session = %q, want empty", task.Session.Active)
	}
	if task.Session.LastCompleted == nil || *task.Session.LastCompleted != "S-001" {
		t.Fatalf("last completed = %#v", task.Session.LastCompleted)
	}

	artifactPath := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "api-auth", "artifacts", "summary.yaml")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read summary artifact: %v", err)
	}
	if !strings.Contains(string(data), "state: archived") {
		t.Fatalf("unexpected artifact after transition: %s", string(data))
	}
}

func TestTaskServiceCreateFailsForMissingProject(t *testing.T) {
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
    statuses: [todo, active, done]
    initial_status: todo
    terminal_statuses: [done]
    transitions:
      start:
        from: [todo]
        to: active
`)

	svc := NewTaskService(store.New(root), steps.New(root))
	_, err := svc.Create("missing-project", "api-auth", nil, nil)
	if err == nil {
		t.Fatal("expected create error")
	}

	taskDir := filepath.Join(root, "projects", "active", "missing-project", "tasks", "api-auth")
	if _, statErr := os.Stat(taskDir); !os.IsNotExist(statErr) {
		t.Fatalf("task dir should not exist, stat err = %v", statErr)
	}
}

func TestTaskServiceCreateFailsForInvalidVarOverride(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars:
      kind: feature
vars:
  task:
    kind:
      allowed: [feature, chore]
workflow:
  task:
    statuses: [todo, active, done]
    initial_status: todo
    terminal_statuses: [done]
    transitions:
      start:
        from: [todo]
        to: active
`)
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	_, err := svc.Create("user-permissions", "api-auth", nil, map[string]string{"kind": "incident"})
	if err == nil {
		t.Fatal("expected invalid var error")
	}

	taskDir := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "api-auth")
	if _, statErr := os.Stat(taskDir); !os.IsNotExist(statErr) {
		t.Fatalf("task dir should not exist, stat err = %v", statErr)
	}
}

func TestTaskServiceCreateFailsBeforeScaffoldingWhenNamingTemplateIsBroken(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars: {}
naming:
  task_slug: "{{ .project.slug }"
workflow:
  task:
    statuses: [todo, active, done]
    initial_status: todo
    terminal_statuses: [done]
    transitions:
      start:
        from: [todo]
        to: active
`)
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	_, err := svc.Create("user-permissions", "api-auth", nil, nil)
	if err == nil {
		t.Fatal("expected broken template error")
	}

	taskDir := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "api-auth")
	if _, statErr := os.Stat(taskDir); !os.IsNotExist(statErr) {
		t.Fatalf("task dir should not exist, stat err = %v", statErr)
	}
}

func TestTaskServiceUpdateBriefAndEventsPersistBoundedPayloadMemory(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: [backend]
    vars:
      kind: feature
vars:
  task:
    kind:
      allowed: [feature, chore]
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
	if err := s.ScaffoldTask(model.TaskState{
		Version: 1,
		Slug:    "api-auth",
		Project: "user-permissions",
		Status:  model.TaskStatus("todo"),
		Labels:  []string{"backend"},
		Vars:    map[string]string{"kind": "feature"},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))

	updated, err := svc.Update("user-permissions", "api-auth", []string{"auth"}, map[string]string{"kind": "chore"}, nil)
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
	if !reflect.DeepEqual(updated.Labels, []string{"auth"}) {
		t.Fatalf("labels = %#v", updated.Labels)
	}
	if updated.Vars["kind"] != "chore" {
		t.Fatalf("kind var = %q", updated.Vars["kind"])
	}

	loadedTask, err := s.LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task after update: %v", err)
	}
	if !reflect.DeepEqual(loadedTask.Labels, []string{"auth"}) {
		t.Fatalf("persisted labels = %#v", loadedTask.Labels)
	}

	wantBrief := "# Intent\n\nDocument runtime decisions.\n\n# Scope In\n\nPayload APIs.\n\n# Scope Out\n\nWorkflow redesign.\n\n# Acceptance\n\nBrief and events persist.\n\n# Current Context\n\nMutation layer implemented.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nImplement describe views.\n\n# References\n\n- plan://payload-layer\n"
	if err := svc.SetBrief("user-permissions", "api-auth", wantBrief); err != nil {
		t.Fatalf("set task brief: %v", err)
	}
	gotBrief, err := svc.GetBrief("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("get task brief: %v", err)
	}
	if gotBrief != wantBrief {
		t.Fatalf("brief = %q", gotBrief)
	}

	event := model.PayloadEvent{
		ID:        "EVT-002",
		At:        "2026-03-14T12:10:00Z",
		Type:      model.PayloadEventTypeTransition,
		Summary:   "Moved to active",
		Actor:     "taskman",
		Rationale: "Unblocked by API contract",
		Impact:    "Ready for integration tests",
	}
	if err := svc.AddEvent("user-permissions", "api-auth", event); err != nil {
		t.Fatalf("add task event: %v", err)
	}
	events, err := svc.GetEvents("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("get task events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d", len(events))
	}
	if events[0].Type != model.PayloadEventTypeTransition {
		t.Fatalf("event type = %q", events[0].Type)
	}
	if events[0].Impact != "Ready for integration tests" {
		t.Fatalf("event impact = %q", events[0].Impact)
	}
}

func TestTaskServiceUpdateSupportsUnsetVarAndRevalidatesRequiredVars(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars:
      kind: feature
      owner: platform
vars:
  task:
    kind:
      required: true
      allowed: [feature, chore]
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
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{
		Version: 1,
		Slug:    "api-auth",
		Project: "user-permissions",
		Status:  model.TaskStatus("todo"),
		Vars:    map[string]string{"kind": "feature", "owner": "platform"},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))

	updated, err := svc.Update("user-permissions", "api-auth", nil, nil, []string{"owner"})
	if err != nil {
		t.Fatalf("update task with unset: %v", err)
	}
	if _, ok := updated.Vars["owner"]; ok {
		t.Fatalf("owner should be unset: %#v", updated.Vars)
	}

	_, err = svc.Update("user-permissions", "api-auth", nil, nil, []string{"kind"})
	if err == nil {
		t.Fatal("expected required var validation error after unset")
	}
}

func TestTaskServiceAddEventRejectsUnknownType(t *testing.T) {
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
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	err := svc.AddEvent("user-permissions", "api-auth", model.PayloadEvent{
		ID:      "EVT-002",
		At:      "2026-03-14T12:10:00Z",
		Type:    model.PayloadEventType("future_type"),
		Summary: "unknown",
		Actor:   "taskman",
	})
	if err == nil {
		t.Fatal("expected unknown type error")
	}
}

func TestTaskServiceSetBriefRejectsNonTemplateContent(t *testing.T) {
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
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	if err := svc.SetBrief("user-permissions", "api-auth", "freeform text"); err == nil {
		t.Fatal("expected invalid task brief error")
	}
}

func TestTaskServiceAddBlockerEventRejectsInvalidStatus(t *testing.T) {
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
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	err := svc.AddEvent("user-permissions", "api-auth", model.PayloadEvent{ID: "EVT-010", At: "2026-03-14T12:00:00Z", Type: model.PayloadEventTypeBlocker, Summary: "Need signoff", Actor: "taskman", Status: "open"})
	if err == nil {
		t.Fatal("expected invalid blocker event error")
	}
}

func writeConfig(t *testing.T, root string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}
