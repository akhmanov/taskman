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
    traits: {}
  task:
    labels: []
    traits:
      mr: required
      worktree: required
traits:
  project:
    preview: [app-api, none]
  task:
    mr: [required, not-needed]
    worktree: [required, optional]
steps:
  task_start:
    - name: noop
      cmd: [./bin/noop]
`)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store := New(root)
	cfg, err := store.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Defaults.Task.Traits["mr"] != "required" {
		t.Fatalf("mr trait = %q", cfg.Defaults.Task.Traits["mr"])
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
		Version:  1,
		Slug:     "cloud-api-auth",
		Project:  "user-permissions",
		Repo:     "cloud",
		Status:   model.TaskStatusActive,
		Session:  model.TaskSessionState{Active: "S-001"},
		MR:       model.TaskMRState{Status: model.MRStatusMissing},
		Worktree: model.TaskWorktreeState{Status: model.WorktreeStatusMissing},
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
}
