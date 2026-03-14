package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/assistant-wi/taskman/internal/model"
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
		filepath.Join(projectDir, "state.yaml"),
		filepath.Join(projectDir, "tasks"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
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
		filepath.Join(taskDir, "state.yaml"),
		filepath.Join(taskDir, "sessions"),
		filepath.Join(taskDir, "artifacts"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
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
