package lifecycle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/steps"
	"github.com/assistant-wi/taskman/internal/store"
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
