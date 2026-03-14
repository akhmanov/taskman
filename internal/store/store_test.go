package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akhmanov/taskman/internal/model"
)

func TestStoreLoadConfig(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	config := []byte(`version: 1
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
    statuses: [todo, active, blocked, done, cancelled, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
      close:
        from: [done, cancelled]
        to: closed
`)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := New(root)
	cfg, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Defaults.Task.Vars["kind"] != "feature" {
		t.Fatalf("task kind var = %q", cfg.Defaults.Task.Vars["kind"])
	}

	if len(cfg.Workflow.Task.Statuses) != 6 {
		t.Fatalf("task workflow statuses = %d, want 6", len(cfg.Workflow.Task.Statuses))
	}
}

func TestStoreScaffoldProjectCreatesExpectedFiles(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	project := model.ProjectState{
		Version: 1,
		Slug:    "user-permissions",
		Status:  model.ProjectStatusActive,
	}

	if err := store.ScaffoldProject(project); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	projectDir := filepath.Join(root, "projects", "active", "user-permissions")
	for _, path := range []string{
		filepath.Join(projectDir, "overview.md"),
		filepath.Join(projectDir, "brief.md"),
		filepath.Join(projectDir, "events.yaml"),
		filepath.Join(projectDir, "state.yaml"),
		filepath.Join(projectDir, "tasks"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	briefData, err := os.ReadFile(filepath.Join(projectDir, "brief.md"))
	if err != nil {
		t.Fatalf("read project brief: %v", err)
	}
	for _, section := range []string{"# Mission", "# Boundaries", "## In Scope", "# References"} {
		if !strings.Contains(string(briefData), section) {
			t.Fatalf("project brief template missing %q: %q", section, string(briefData))
		}
	}

	eventsData, err := os.ReadFile(filepath.Join(projectDir, "events.yaml"))
	if err != nil {
		t.Fatalf("read project events: %v", err)
	}
	if string(eventsData) != "[]\n" {
		t.Fatalf("project events template = %q", string(eventsData))
	}

	loaded, err := store.LoadProject("user-permissions")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if loaded.Slug != "user-permissions" {
		t.Fatalf("loaded slug = %q", loaded.Slug)
	}
}

func TestStoreScaffoldTaskCreatesTaskSessionAndArtifactsDirs(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	if err := store.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	task := model.TaskState{
		Version: 1,
		Slug:    "cloud-api-auth",
		Project: "user-permissions",
		Status:  model.TaskStatus("active"),
		Vars:    map[string]string{"kind": "feature"},
		Session: model.TaskSessionState{Active: "S-001"},
	}

	if err := store.ScaffoldTask(task); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	taskDir := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "cloud-api-auth")
	for _, path := range []string{
		filepath.Join(taskDir, "overview.md"),
		filepath.Join(taskDir, "brief.md"),
		filepath.Join(taskDir, "events.yaml"),
		filepath.Join(taskDir, "state.yaml"),
		filepath.Join(taskDir, "sessions"),
		filepath.Join(taskDir, "artifacts"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	briefData, err := os.ReadFile(filepath.Join(taskDir, "brief.md"))
	if err != nil {
		t.Fatalf("read task brief: %v", err)
	}
	for _, section := range []string{"# Intent", "# Scope In", "# Next Action", "# References"} {
		if !strings.Contains(string(briefData), section) {
			t.Fatalf("task brief template missing %q: %q", section, string(briefData))
		}
	}

	eventsData, err := os.ReadFile(filepath.Join(taskDir, "events.yaml"))
	if err != nil {
		t.Fatalf("read task events: %v", err)
	}
	if string(eventsData) != "[]\n" {
		t.Fatalf("task events template = %q", string(eventsData))
	}

	loaded, err := store.LoadTask("user-permissions", "cloud-api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if loaded.Project != "user-permissions" {
		t.Fatalf("loaded project = %q", loaded.Project)
	}

	for _, path := range []string{
		filepath.Join(taskDir, "artifacts", "repository.yaml"),
		filepath.Join(taskDir, "artifacts", "branch.yaml"),
		filepath.Join(taskDir, "artifacts", "worktree.yaml"),
		filepath.Join(taskDir, "artifacts", "mr.yaml"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("did not expect scaffold artifact file %s", path)
		}
	}
}

func TestStoreArtifactsRemainOpaqueJSONMaps(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	if err := store.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := store.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	if err := store.SaveArtifact("user-permissions", "api-auth", "summary", map[string]any{"state": "ready", "attempts": 2, "ok": true}); err != nil {
		t.Fatalf("save artifact: %v", err)
	}

	artifact, err := store.LoadArtifact("user-permissions", "api-auth", "summary")
	if err != nil {
		t.Fatalf("load artifact: %v", err)
	}

	if artifact.Data["state"] != "ready" {
		t.Fatalf("state = %#v", artifact.Data["state"])
	}
	if artifact.Data["attempts"] != 2 {
		t.Fatalf("attempts = %#v", artifact.Data["attempts"])
	}
	if artifact.Data["ok"] != true {
		t.Fatalf("ok = %#v", artifact.Data["ok"])
	}
}

func TestStoreProjectBriefLoadAndSaveRoundTrip(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	if err := store.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	brief := "# Mission\n\nHarden auth.\n\n# Boundaries\n\n## In Scope\n\nIdentity.\n\n## Out of Scope\n\nBilling.\n\n# Glossary\n\n- Auth\n\n# Shared Decisions\n\n- Use bounded payloads.\n\n# Active Risks\n\n- Drift.\n\n# Tasking Rules\n\n- Keep tasks focused.\n\n# References\n\n- doc://auth\n"
	if err := store.SaveProjectBrief("user-permissions", brief); err != nil {
		t.Fatalf("save project brief: %v", err)
	}

	loaded, err := store.LoadProjectBrief("user-permissions")
	if err != nil {
		t.Fatalf("load project brief: %v", err)
	}

	if loaded != brief {
		t.Fatalf("project brief = %q", loaded)
	}
}

func TestStoreTaskBriefLoadAndSaveRoundTrip(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	if err := store.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := store.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	brief := "# Intent\n\nAdd typed payload storage.\n\n# Scope In\n\nStorage primitives.\n\n# Scope Out\n\nCLI polish.\n\n# Acceptance\n\nBriefs and events persist.\n\n# Current Context\n\nFoundation only.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nImplement APIs.\n\n# References\n\n- plan://payload-layer\n"
	if err := store.SaveTaskBrief("user-permissions", "api-auth", brief); err != nil {
		t.Fatalf("save task brief: %v", err)
	}

	loaded, err := store.LoadTaskBrief("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task brief: %v", err)
	}

	if loaded != brief {
		t.Fatalf("task brief = %q", loaded)
	}
}

func TestStoreProjectEventsAppendAndListPreservesOrder(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	if err := store.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	first := model.PayloadEvent{ID: "EVT-001", Type: model.PayloadEventTypeNote, At: "2026-03-14T10:00:00Z", Actor: "taskman", Summary: "Created project brief"}
	second := model.PayloadEvent{ID: "EVT-002", Type: model.PayloadEventTypeTransition, At: "2026-03-14T10:30:00Z", Actor: "taskman", Summary: "Updated project status summary"}

	if err := store.AppendProjectEvent("user-permissions", first); err != nil {
		t.Fatalf("append first project event: %v", err)
	}
	if err := store.AppendProjectEvent("user-permissions", second); err != nil {
		t.Fatalf("append second project event: %v", err)
	}

	events, err := store.ListProjectEvents("user-permissions")
	if err != nil {
		t.Fatalf("list project events: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("project events len = %d, want 2", len(events))
	}
	if events[0].Summary != "Created project brief" {
		t.Fatalf("first summary = %q", events[0].Summary)
	}
	if events[1].Type != model.PayloadEventTypeTransition {
		t.Fatalf("second type = %q", events[1].Type)
	}
	if events[0].ID != "EVT-001" {
		t.Fatalf("first id = %q", events[0].ID)
	}
}

func TestStoreTaskEventsAppendAndListPreservesOrder(t *testing.T) {
	root := t.TempDir()
	store := New(root)

	if err := store.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := store.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	first := model.PayloadEvent{ID: "EVT-101", Type: model.PayloadEventTypeDecision, At: "2026-03-14T10:05:00Z", Actor: "taskman", Session: "S-001", Summary: "Created task brief", Rationale: "Need durable context", Impact: "Enables richer follow-up", Status: "active"}
	second := model.PayloadEvent{ID: "EVT-102", Type: model.PayloadEventTypeBlocker, At: "2026-03-14T10:40:00Z", Actor: "taskman", Summary: "Updated task status summary", Status: "resolved"}

	if err := store.AppendTaskEvent("user-permissions", "api-auth", first); err != nil {
		t.Fatalf("append first task event: %v", err)
	}
	if err := store.AppendTaskEvent("user-permissions", "api-auth", second); err != nil {
		t.Fatalf("append second task event: %v", err)
	}

	events, err := store.ListTaskEvents("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("list task events: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("task events len = %d, want 2", len(events))
	}
	if events[0].Summary != "Created task brief" {
		t.Fatalf("first summary = %q", events[0].Summary)
	}
	if events[1].Type != model.PayloadEventTypeBlocker {
		t.Fatalf("second type = %q", events[1].Type)
	}
	if events[0].Status != "active" {
		t.Fatalf("first status = %q", events[0].Status)
	}
}
