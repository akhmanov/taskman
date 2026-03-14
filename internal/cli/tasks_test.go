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

func TestTasksCreateAndTransitionFlow(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
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
    statuses: [todo, active, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
        steps:
          - name: write-summary
            cmd: [./bin/start-task]
`)
	writeCLIExecutable(t, filepath.Join(root, "bin", "start-task"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"started\",\"artifacts\":{\"summary\":{\"state\":\"started\"}}}'\n")

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "user-permissions", "--name", "api-auth"}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	s := store.New(root)
	task, err := s.LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task after create: %v", err)
	}
	if task.Status != model.TaskStatus("todo") {
		t.Fatalf("status after create = %q, want todo", task.Status)
	}

	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "transition", "user-permissions/api-auth", "start"}); err != nil {
		t.Fatalf("transition task: %v", err)
	}

	task, err = s.LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task after transition: %v", err)
	}
	if task.Status != model.TaskStatus("active") {
		t.Fatalf("status after transition = %q, want active", task.Status)
	}
	if task.LastOp.Cmd != "tasks.transition" || !task.LastOp.OK {
		t.Fatalf("last op = %+v", task.LastOp)
	}

	artifact, err := s.LoadArtifact("user-permissions", "api-auth", "summary")
	if err != nil {
		t.Fatalf("load artifact: %v", err)
	}
	if artifact.Data["state"] != "started" {
		t.Fatalf("summary artifact state = %q", artifact.Data["state"])
	}
}

func TestTasksCreatePersistsLabelsAndVars(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
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

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "user-permissions", "--name", "api-auth", "--label", "auth", "--var", "kind=chore"}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	task, err := store.New(root).LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if len(task.Labels) != 2 || task.Labels[0] != "backend" || task.Labels[1] != "auth" {
		t.Fatalf("labels = %#v", task.Labels)
	}
	if task.Vars["kind"] != "chore" {
		t.Fatalf("kind var = %q", task.Vars["kind"])
	}
}

func TestTasksGetWithoutProjectListsAcrossProjects(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
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

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "alpha"}); err != nil {
		t.Fatalf("create alpha project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "beta"}); err != nil {
		t.Fatalf("create beta project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "alpha", "--name", "api-auth"}); err != nil {
		t.Fatalf("create alpha task: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "beta", "--name", "engine"}); err != nil {
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

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "get", "--status", "todo"}); err != nil {
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
	if !strings.Contains(out, "alpha/api-auth") || !strings.Contains(out, "beta/engine") {
		t.Fatalf("global tasks get missing expected tasks: %s", out)
	}
}

func TestTasksDescribeShowsGenericArtifactData(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
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
	if err := s.ScaffoldTask(model.TaskState{
		Version: 1,
		Slug:    "api-auth",
		Project: "user-permissions",
		Status:  model.TaskStatus("active"),
		Vars:    map[string]string{"kind": "feature"},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}
	if err := s.SaveArtifact("user-permissions", "api-auth", "summary", map[string]any{
		"state": "ready",
		"url":   "https://example.test/tasks/api-auth",
	}); err != nil {
		t.Fatalf("save summary artifact: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "describe", "user-permissions/api-auth"}); err != nil {
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
		"user-permissions/api-auth",
		"Vars: kind=feature",
		"Artifact summary: state=ready, url=https://example.test/tasks/api-auth",
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
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "alpha", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "alpha", Status: model.TaskStatus("todo")}); err != nil {
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
	if payload[0]["slug"] != "api-auth" {
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
