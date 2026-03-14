package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/assistant-wi/taskman/internal/store"
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
