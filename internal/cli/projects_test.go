package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/store"
	"gopkg.in/yaml.v3"
)

func TestProjectsCreateScaffoldsProject(t *testing.T) {
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
	err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions"})
	if err != nil {
		t.Fatalf("run command: %v", err)
	}

	projectDir := filepath.Join(root, "projects", "active", "user-permissions")
	if _, err := os.Stat(filepath.Join(projectDir, "state.yaml")); err != nil {
		t.Fatalf("project state missing: %v", err)
	}

	s := store.New(root)
	project, err := s.LoadProject("user-permissions")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if project.Slug != "user-permissions" {
		t.Fatalf("slug = %q", project.Slug)
	}
}

func TestProjectsCreatePersistsLabelsAndVars(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: [platform]
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

	cmd := BuildApp()
	err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions", "--label", "auth", "--var", "area=product"})
	if err != nil {
		t.Fatalf("run command: %v", err)
	}

	project, err := store.New(root).LoadProject("user-permissions")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}

	if len(project.Labels) != 2 || project.Labels[0] != "platform" || project.Labels[1] != "auth" {
		t.Fatalf("labels = %#v", project.Labels)
	}
	if project.Vars["area"] != "product" {
		t.Fatalf("area var = %q", project.Vars["area"])
	}
}

func TestProjectsCreateRejectsInvalidVarOverride(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
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

	cmd := BuildApp()
	err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions", "--var", "area=unknown"})
	if err == nil {
		t.Fatal("expected invalid var error")
	}
}

func TestProjectsDescribeSupportsYAMLOutput(t *testing.T) {
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
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions"}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "describe", "user-permissions", "--output", "yaml"}); err != nil {
		t.Fatalf("projects describe yaml: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var payload map[string]any
	if err := yaml.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal yaml output: %v\n%s", err, string(data))
	}
	if payload["slug"] != "user-permissions" {
		t.Fatalf("slug = %#v", payload["slug"])
	}
	if payload["status"] != "active" {
		t.Fatalf("status = %#v", payload["status"])
	}
}

func TestProjectsGetShowsTaskCountSummaryInTextMode(t *testing.T) {
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
    statuses: [todo, active, in_review, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
`)
	s := store.New(root)
	if err := s.ScaffoldProject(projectStateForText()); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "get"}); err != nil {
		t.Fatalf("projects get: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	out := string(data)
	for _, want := range []string{"user-permissions", "active", "active=2", "closed=3", "todo=1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("projects get output missing %q: %s", want, out)
		}
	}
}

func TestProjectsDescribeShowsRicherTextDetails(t *testing.T) {
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
    statuses: [todo, active, in_review, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
`)
	s := store.New(root)
	if err := s.ScaffoldProject(projectStateForText()); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "describe", "user-permissions"}); err != nil {
		t.Fatalf("projects describe: %v", err)
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
		"Project: user-permissions",
		"Status: active",
		"Labels: auth, platform",
		"Vars: area=product",
		"Tasks: active=2, closed=3, todo=1",
		"Archive Ready: false",
		"Archive Blockers: task api-auth is not terminal",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("projects describe output missing %q: %s", want, out)
		}
	}
}

func projectStateForText() model.ProjectState {
	return model.ProjectState{
		Version: 1,
		Slug:    "user-permissions",
		Status:  model.ProjectStatusActive,
		Labels:  []string{"auth", "platform"},
		Vars:    map[string]string{"area": "product"},
		Tasks: model.TaskCounts{
			"todo":   1,
			"active": 2,
			"closed": 3,
		},
		Archive: model.ArchiveState{
			Ready:    false,
			Blockers: []string{"task api-auth is not terminal"},
		},
	}
}
