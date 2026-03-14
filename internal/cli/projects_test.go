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

func TestProjectsCreatePersistsLabelsAndTraits(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: [platform]
    traits:
      preview: none
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

	cmd := BuildApp()
	err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions", "--label", "auth", "--trait", "preview=app-api"})
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
	if project.Traits["preview"] != "app-api" {
		t.Fatalf("preview trait = %q", project.Traits["preview"])
	}
}

func TestProjectsCreateRejectsInvalidTraitOverride(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    traits:
      preview: none
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

	cmd := BuildApp()
	err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions", "--trait", "preview=maybe"})
	if err == nil {
		t.Fatal("expected invalid trait error")
	}
}

func TestProjectsDescribeSupportsYAMLOutput(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    traits:
      preview: none
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
    traits:
      preview: none
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
	for _, want := range []string{"user-permissions", "active", "todo=1", "active=2", "done=3"} {
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
    traits:
      preview: none
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
		"Traits: preview=app-api",
		"Tasks: todo=1 active=2 blocked=0 done=3 cancelled=0",
		"Archive Ready: false",
		"Archive Blockers: task cloud-api-auth is still active",
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
		Traits:  map[string]string{"preview": "app-api"},
		Tasks: model.TaskCounts{
			Todo:      1,
			Active:    2,
			Blocked:   0,
			Done:      3,
			Cancelled: 0,
		},
		Archive: model.ArchiveState{
			Ready:    false,
			Blockers: []string{"task cloud-api-auth is still active"},
		},
	}
}
