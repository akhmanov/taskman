package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/store"
)

func TestTasksCreateAndDoneFlow(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
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
steps:
  task_start:
    - name: noop
      cmd: [./bin/noop]
  task_done:
    - name: ready_mr
      when:
        task.traits.mr: required
      cmd: [./bin/ready-mr]
  task_cleanup:
    - name: cleanup_worktree
      cmd: [./bin/cleanup-worktree]
`)
	writeCLIExecutable(t, filepath.Join(root, "bin", "noop"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"created\"}'\n")
	writeCLIExecutable(t, filepath.Join(root, "bin", "ready-mr"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"mr ready\"}'\n")
	writeCLIExecutable(t, filepath.Join(root, "bin", "cleanup-worktree"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"cleaned\",\"artifacts\":{\"worktree\":{\"status\":\"cleaned\",\"path\":\"/tmp/worktree\"}}}'\n")

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "user-permissions", "--repo", "cloud", "--name", "api-auth"}); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "done", "user-permissions/cloud-api-auth"}); err != nil {
		t.Fatalf("done task: %v", err)
	}

	s := store.New(root)
	task, err := s.LoadTask("user-permissions", "cloud-api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if task.Status != model.TaskStatusDone {
		t.Fatalf("status = %q, want done", task.Status)
	}

	if task.LastOp.Cmd != "tasks.done" || !task.LastOp.OK {
		t.Fatalf("last op = %+v", task.LastOp)
	}
	if task.Session.Active != "" {
		t.Fatalf("active session = %q, want empty", task.Session.Active)
	}
	if task.Session.LastCompleted == nil || *task.Session.LastCompleted != "S-001" {
		t.Fatalf("last completed = %#v", task.Session.LastCompleted)
	}

	sessionPath := filepath.Join(root, "projects", "active", "user-permissions", "tasks", "cloud-api-auth", "sessions", "S-001", "state.yaml")
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("session state missing: %v", err)
	}

	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "cleanup", "user-permissions/cloud-api-auth"}); err != nil {
		t.Fatalf("cleanup task: %v", err)
	}

	task, err = s.LoadTask("user-permissions", "cloud-api-auth")
	if err != nil {
		t.Fatalf("reload task after cleanup: %v", err)
	}

	if task.Worktree.Status != model.WorktreeStatusCleaned {
		t.Fatalf("cleanup worktree status = %q", task.Worktree.Status)
	}
	if task.LastOp.Cmd != "tasks.cleanup" || !task.LastOp.OK {
		t.Fatalf("cleanup last op = %+v", task.LastOp)
	}
}

func TestTasksCreatePersistsLabelsAndTraits(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    traits: {}
  task:
    labels: [backend]
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
steps:
  task_start:
    - name: noop
      cmd: [./bin/noop]
`)
	writeCLIExecutable(t, filepath.Join(root, "bin", "noop"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"created\"}'\n")

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "user-permissions", "--repo", "cloud", "--name", "api-auth", "--label", "auth", "--trait", "mr=not-needed"}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	task, err := store.New(root).LoadTask("user-permissions", "cloud-api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if len(task.Labels) != 2 || task.Labels[0] != "backend" || task.Labels[1] != "auth" {
		t.Fatalf("labels = %#v", task.Labels)
	}
	if task.Traits["mr"] != "not-needed" {
		t.Fatalf("mr trait = %q", task.Traits["mr"])
	}
	if task.MR.Status != model.MRStatusNotNeeded {
		t.Fatalf("mr status = %q", task.MR.Status)
	}
}

func TestTasksGetWithoutProjectListsAcrossProjects(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
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
steps:
  task_start:
    - name: noop
      cmd: [./bin/noop]
`)
	writeCLIExecutable(t, filepath.Join(root, "bin", "noop"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"created\"}'\n")

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "alpha"}); err != nil {
		t.Fatalf("create alpha project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "beta"}); err != nil {
		t.Fatalf("create beta project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "alpha", "--repo", "cloud", "--name", "api-auth"}); err != nil {
		t.Fatalf("create alpha task: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "beta", "--repo", "taskman", "--name", "engine"}); err != nil {
		t.Fatalf("create beta task: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "get", "--status", "active"}); err != nil {
		t.Fatalf("tasks get global: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "alpha/cloud-api-auth") || !strings.Contains(out, "beta/taskman-engine") {
		t.Fatalf("global tasks get missing expected tasks: %s", out)
	}
}

func TestTasksDescribeShowsRicherMergeRequestArtifactData(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
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
steps: {}
`)

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
		Worktree: model.TaskWorktreeState{Status: model.WorktreeStatusCleaned},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}
	if err := s.SaveArtifact("user-permissions", "cloud-api-auth", "mr", map[string]string{
		"status":        "ready",
		"iid":           "123",
		"url":           "https://gitlab.com/assistant-wi/cloud/-/merge_requests/123",
		"state":         "opened",
		"draft":         "false",
		"source_branch": "task/user-permissions/cloud-api-auth",
		"target_branch": "main",
		"title":         "Add auth flow",
	}); err != nil {
		t.Fatalf("save mr artifact: %v", err)
	}
	if err := s.SaveArtifact("user-permissions", "cloud-api-auth", "repository", map[string]string{
		"root": "/repo/cloud",
		"name": "cloud",
	}); err != nil {
		t.Fatalf("save repository artifact: %v", err)
	}
	if err := s.SaveArtifact("user-permissions", "cloud-api-auth", "branch", map[string]string{
		"name": "task/user-permissions/cloud-api-auth",
	}); err != nil {
		t.Fatalf("save branch artifact: %v", err)
	}
	if err := s.SaveArtifact("user-permissions", "cloud-api-auth", "worktree", map[string]string{
		"path":   "/repo/cloud/.worktrees/user-permissions/cloud-api-auth",
		"status": "cleaned",
	}); err != nil {
		t.Fatalf("save worktree artifact: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "describe", "user-permissions/cloud-api-auth"}); err != nil {
		t.Fatalf("tasks describe: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	out := string(data)
	for _, want := range []string{
		"user-permissions/cloud-api-auth",
		"MR Status: ready",
		"MR URL: https://gitlab.com/assistant-wi/cloud/-/merge_requests/123",
		"Repository Root: /repo/cloud",
		"Branch: task/user-permissions/cloud-api-auth",
		"Worktree Path: /repo/cloud/.worktrees/user-permissions/cloud-api-auth",
		"Target Branch: main",
		"Title: Add auth flow",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("describe output missing %q: %s", want, out)
		}
	}
}

func TestTasksGetSupportsJSONOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
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
steps: {}
`)
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "alpha", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "cloud-api-auth", Project: "alpha", Repo: "cloud", Status: model.TaskStatusActive}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "get", "--output", "json"}); err != nil {
		t.Fatalf("tasks get json: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal json output: %v\n%s", err, string(data))
	}
	if len(payload) != 1 {
		t.Fatalf("payload len = %d", len(payload))
	}
	if payload[0]["slug"] != "cloud-api-auth" {
		t.Fatalf("slug = %#v", payload[0]["slug"])
	}
}

func writeCLIConfig(t *testing.T, root string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeCLIExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir executable dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}
}
