package lifecycle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/steps"
	"github.com/assistant-wi/taskman/internal/store"
)

func TestProjectArchiveFailsWhenTaskStillActive(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
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
  project_archive:
    - name: noop
      cmd: [./bin/noop]
`)
	writeExecutable(t, filepath.Join(root, "bin", "noop"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"archive ok\"}'\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusDone}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "cloud-api-auth", Project: "user-permissions", Repo: "cloud", Status: model.TaskStatusActive, Worktree: model.TaskWorktreeState{Status: model.WorktreeStatusPresent}, MR: model.TaskMRState{Status: model.MRStatusReady}}); err != nil {
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

func TestProjectArchiveMovesProjectWhenLocalGatesPass(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `version: 1
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
  project_archive:
    - name: noop
      cmd: [./bin/noop]
`)
	writeExecutable(t, filepath.Join(root, "bin", "noop"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"archive ok\"}'\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusDone}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "cloud-api-auth", Project: "user-permissions", Repo: "cloud", Status: model.TaskStatusDone, Worktree: model.TaskWorktreeState{Status: model.WorktreeStatusCleaned}, MR: model.TaskMRState{Status: model.MRStatusReady}}); err != nil {
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
