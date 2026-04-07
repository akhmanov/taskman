package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/akhmanov/taskman/internal/model"
)

func TestCreateProjectAndTaskRejectDuplicateManifests(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "taskman.yaml"), []byte(model.DefaultConfigYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	s := New(root)

	project := model.Manifest{ID: "project-1", Kind: model.EntityKindProject, Slug: "alpha", Name: "alpha", Description: "Alpha project", CreatedAt: "2026-03-15T00:00:00Z"}
	if err := s.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := s.CreateProject(project); err == nil {
		t.Fatalf("duplicate project create should fail")
	}

	task := model.Manifest{ID: "task-1", Kind: model.EntityKindTask, Slug: "api-auth", Name: "api-auth", Description: "Implement API auth", ProjectID: project.ID, ProjectSlug: project.Slug, CreatedAt: "2026-03-15T00:00:00Z"}
	if err := s.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.CreateTask(task); err == nil {
		t.Fatalf("duplicate task create should fail")
	}
}

func TestLoadRecordsFallbackUpdatedAtToManifestCreationTime(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "taskman.yaml"), []byte(model.DefaultConfigYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	s := New(root)
	projectCreatedAt := "2026-03-15T00:00:00Z"
	taskCreatedAt := "2026-03-15T00:01:00Z"
	project := model.Manifest{ID: "project-1", Kind: model.EntityKindProject, Slug: "alpha", Name: "alpha", Description: "Alpha project", CreatedAt: projectCreatedAt}
	if err := s.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := model.Manifest{ID: "task-1", Kind: model.EntityKindTask, Slug: "api-auth", Name: "api-auth", Description: "Implement API auth", ProjectID: project.ID, ProjectSlug: project.Slug, CreatedAt: taskCreatedAt}
	if err := s.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	loadedProject, err := s.LoadProject("alpha")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	if loadedProject.State.UpdatedAt != projectCreatedAt {
		t.Fatalf("project UpdatedAt = %q, want %q", loadedProject.State.UpdatedAt, projectCreatedAt)
	}
	loadedTask, err := s.LoadTask("alpha", "api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if loadedTask.State.UpdatedAt != taskCreatedAt {
		t.Fatalf("task UpdatedAt = %q, want %q", loadedTask.State.UpdatedAt, taskCreatedAt)
	}
}

func TestLoadOptionalConfigHandlesMissingAndInvalidFiles(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	cfg, present, err := s.LoadOptionalConfig()
	if err != nil {
		t.Fatalf("missing optional config should not fail: %v", err)
	}
	if present {
		t.Fatalf("missing optional config should report present=false")
	}
	if len(cfg.Defaults.Project.Labels) != 0 || len(cfg.Defaults.Task.Labels) != 0 || len(cfg.Middleware.Project) != 0 || len(cfg.Middleware.Task) != 0 || cfg.Defaults.Project.Vars != nil || cfg.Defaults.Task.Vars != nil {
		t.Fatalf("missing optional config should return zero config, got %#v", cfg)
	}

	if err := os.WriteFile(filepath.Join(root, "taskman.yaml"), []byte("middleware:\n  project:\n    plan:\n      pre:\n        - name: broken\n"), 0o644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}
	if _, present, err = s.LoadOptionalConfig(); err == nil || !present {
		t.Fatalf("invalid existing config should fail and report present=true, err=%v present=%v", err, present)
	}
}

func TestCreateProjectAndTaskUseCanonicalNumberedPaths(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	project := model.Manifest{ID: "project-25", Kind: model.EntityKindProject, Number: 25, Slug: "alpha", Name: "alpha", Description: "Alpha project", CreatedAt: "2026-03-15T00:00:00Z"}
	if err := s.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := model.Manifest{ID: "task-1", Kind: model.EntityKindTask, Number: 1, Slug: "api-auth", Name: "api-auth", Description: "Implement API auth", ProjectID: project.ID, ProjectSlug: project.Slug, CreatedAt: "2026-03-15T00:01:00Z"}
	if err := s.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	for _, path := range []string{
		filepath.Join(root, "projects", "25_alpha", "manifest.json"),
		filepath.Join(root, "projects", "25_alpha", "tasks", "1_api-auth", "manifest.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected numbered path %s: %v", path, err)
		}
	}
}

func TestLoadProjectAcceptsCompositeNumberAndSlugRefs(t *testing.T) {
	root := t.TempDir()
	s := New(root)

	project := model.Manifest{ID: "project-25", Kind: model.EntityKindProject, Number: 25, Slug: "alpha", Name: "alpha", Description: "Alpha project", CreatedAt: "2026-03-15T00:00:00Z"}
	if err := s.CreateProject(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	for _, ref := range []string{"25_alpha", "25", "alpha"} {
		loaded, err := s.LoadProject(ref)
		if err != nil {
			t.Fatalf("load project %s: %v", ref, err)
		}
		if loaded.Manifest.Number != 25 || loaded.Manifest.Slug != "alpha" {
			t.Fatalf("load project %s = %#v", ref, loaded.Manifest)
		}
	}
}
