package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/steps"
	"github.com/assistant-wi/taskman/internal/store"
)

func TestTaskServiceCreateScaffoldsInitialSession(t *testing.T) {
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
  task_start:
    - name: create_worktree
      cmd: [./bin/noop]
`)
	writeExecutable(t, filepath.Join(root, "bin", "noop"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"created\",\"artifacts\":{\"worktree\":{\"status\":\"present\",\"path\":\"/tmp/worktree\"}}}'\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	task, err := svc.Create("user-permissions", "cloud", "api-auth", nil, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if task.Slug != "cloud-api-auth" {
		t.Fatalf("task slug = %q", task.Slug)
	}

	if task.Session.Active != "S-001" {
		t.Fatalf("active session = %q", task.Session.Active)
	}

	if task.Worktree.Status != model.WorktreeStatusPresent {
		t.Fatalf("worktree status = %q, want present", task.Worktree.Status)
	}

	sessionPath := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "cloud-api-auth", "sessions", "S-001", "state.yaml")
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("session state missing: %v", err)
	}

	artifactPath := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "cloud-api-auth", "artifacts", "worktree.yaml")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read worktree artifact: %v", err)
	}
	if !strings.Contains(string(data), "status: present") {
		t.Fatalf("unexpected worktree artifact: %s", string(data))
	}
}

func TestTaskServiceDonePersistsFailedOperationWithoutChangingStatus(t *testing.T) {
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
  task_done:
    - name: validate_ready_mr
      when:
        task.traits.mr: required
      cmd: [./bin/fail-mr]
`)
	writeExecutable(t, filepath.Join(root, "bin", "fail-mr"), "#!/bin/sh\necho '{\"ok\":false,\"message\":\"merge request still draft\"}'\nexit 1\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{
		Version:  1,
		Slug:     "cloud-api-auth",
		Project:  "user-permissions",
		Repo:     "cloud",
		Status:   model.TaskStatusActive,
		Traits:   map[string]string{"mr": "required", "worktree": "required"},
		Session:  model.TaskSessionState{Active: "S-001"},
		MR:       model.TaskMRState{Status: model.MRStatusDraft},
		Worktree: model.TaskWorktreeState{Status: model.WorktreeStatusPresent},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	_, err := svc.Done("user-permissions", "cloud-api-auth")
	if err == nil {
		t.Fatal("expected done error")
	}

	loaded, err := s.LoadTask("user-permissions", "cloud-api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if loaded.Status != model.TaskStatusActive {
		t.Fatalf("status = %q, want active", loaded.Status)
	}

	if loaded.LastOp.OK {
		t.Fatal("last op should be failed")
	}

	if loaded.LastOp.Step != "validate_ready_mr" {
		t.Fatalf("last op step = %q", loaded.LastOp.Step)
	}
}

func TestTaskServiceCleanupPersistsCleanedWorktreeState(t *testing.T) {
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
  task_cleanup:
    - name: remove_worktree
      when:
        task.traits.worktree: required
      cmd: [./bin/cleanup-worktree]
`)
	writeExecutable(t, filepath.Join(root, "bin", "cleanup-worktree"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"worktree removed\",\"artifacts\":{\"worktree\":{\"status\":\"cleaned\",\"path\":\"/tmp/worktree\"}}}'\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{
		Version:  1,
		Slug:     "cloud-api-auth",
		Project:  "user-permissions",
		Repo:     "cloud",
		Status:   model.TaskStatusDone,
		Traits:   map[string]string{"mr": "required", "worktree": "required"},
		Session:  model.TaskSessionState{},
		MR:       model.TaskMRState{Status: model.MRStatusReady},
		Worktree: model.TaskWorktreeState{Status: model.WorktreeStatusPresent},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	task, err := svc.Cleanup("user-permissions", "cloud-api-auth")
	if err != nil {
		t.Fatalf("cleanup task: %v", err)
	}

	if task.Worktree.Status != model.WorktreeStatusCleaned {
		t.Fatalf("worktree status = %q", task.Worktree.Status)
	}
	if task.LastOp.Cmd != "tasks.cleanup" || !task.LastOp.OK {
		t.Fatalf("last op = %+v", task.LastOp)
	}

	artifactPath := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "cloud-api-auth", "artifacts", "worktree.yaml")
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read worktree artifact: %v", err)
	}
	if !strings.Contains(string(data), "status: cleaned") {
		t.Fatalf("unexpected artifact after cleanup: %s", string(data))
	}
}

func TestTaskServiceCleanupFailsSafelyForDirtyWorktree(t *testing.T) {
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
  task_cleanup:
    - name: remove_worktree
      when:
        task.traits.worktree: required
      cmd: [./bin/cleanup-worktree]
`)
	writeExecutable(t, filepath.Join(root, "bin", "cleanup-worktree"), "#!/bin/sh\necho '{\"ok\":false,\"message\":\"dirty worktree detected\"}'\nexit 1\n")

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{
		Version:  1,
		Slug:     "cloud-api-auth",
		Project:  "user-permissions",
		Repo:     "cloud",
		Status:   model.TaskStatusDone,
		Traits:   map[string]string{"mr": "required", "worktree": "required"},
		Session:  model.TaskSessionState{},
		MR:       model.TaskMRState{Status: model.MRStatusReady},
		Worktree: model.TaskWorktreeState{Status: model.WorktreeStatusPresent},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	_, err := svc.Cleanup("user-permissions", "cloud-api-auth")
	if err == nil {
		t.Fatal("expected cleanup error")
	}

	task, err := s.LoadTask("user-permissions", "cloud-api-auth")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	if task.Worktree.Status != model.WorktreeStatusPresent {
		t.Fatalf("worktree status = %q, want present", task.Worktree.Status)
	}
	if task.LastOp.OK {
		t.Fatal("expected failed cleanup operation")
	}
	if task.LastOp.Step != "remove_worktree" {
		t.Fatalf("cleanup failed step = %q", task.LastOp.Step)
	}
}

func TestTaskServiceCreateFailsForMissingProject(t *testing.T) {
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
naming:
  task_slug: "{{ .repo }}-{{ .name }}"
steps: {}
`)

	svc := NewTaskService(store.New(root), steps.New(root))
	_, err := svc.Create("missing-project", "cloud", "api-auth", nil, nil)
	if err == nil {
		t.Fatal("expected create error")
	}

	taskDir := filepath.Join(root, "projects", "active", "missing-project", "tasks", "cloud-api-auth")
	if _, statErr := os.Stat(taskDir); !os.IsNotExist(statErr) {
		t.Fatalf("task dir should not exist, stat err = %v", statErr)
	}
}

func TestTaskServiceCreateFailsForInvalidTraitOverride(t *testing.T) {
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
naming:
  task_slug: "{{ .repo }}-{{ .name }}"
steps: {}
`)
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	_, err := svc.Create("user-permissions", "cloud", "api-auth", nil, map[string]string{"mr": "sometimes"})
	if err == nil {
		t.Fatal("expected invalid trait error")
	}

	taskDir := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "cloud-api-auth")
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
naming:
  task_slug: "{{ .repo }}-{{ .name }}"
  branch: "task/{{ .project.slug }"
steps:
  task_start:
    - name: noop
      cmd: [./bin/noop]
`)
	writeExecutable(t, filepath.Join(root, "bin", "noop"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"created\"}'\n")
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	svc := NewTaskService(s, steps.New(root))
	_, err := svc.Create("user-permissions", "cloud", "api-auth", nil, nil)
	if err == nil {
		t.Fatal("expected broken template error")
	}

	taskDir := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "cloud-api-auth")
	if _, statErr := os.Stat(taskDir); !os.IsNotExist(statErr) {
		t.Fatalf("task dir should not exist, stat err = %v", statErr)
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
