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
