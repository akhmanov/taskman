package lifecycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/akhmanov/taskman/internal/model"
	"github.com/akhmanov/taskman/internal/steps"
	"github.com/akhmanov/taskman/internal/store"
)

func TestTaskServiceRejectsStatefulWritesWhenTaskHasDivergentHeads(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigLifecycle(t, root)

	projectSvc := NewProjectService(store.New(root), steps.New(root))
	taskSvc := NewTaskService(store.New(root), steps.New(root))
	_, err := projectSvc.Create("alpha", CreateInput{Description: "Alpha project"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, err = taskSvc.Create("alpha", "api-auth", CreateInput{Description: "Implement API auth"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	runtimeStore := store.New(root)
	manifest, err := runtimeStore.LoadTaskManifest("alpha", "api-auth")
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, event := range []model.Event{
		{ID: "ev-plan", EntityID: manifest.ID, Kind: model.EventKindTransition, At: now, Actor: "test", Transition: &model.TransitionPayload{Verb: "plan", From: model.StatusBacklog, To: model.StatusPlanned}},
		{ID: "ev-cancel", EntityID: manifest.ID, Kind: model.EventKindTransition, At: now + "-b", Actor: "test", Transition: &model.TransitionPayload{Verb: "cancel", From: model.StatusBacklog, To: model.StatusCanceled, ReasonType: "manual", Reason: "stop"}},
	} {
		if err := runtimeStore.AppendTaskEvent("alpha", "api-auth", event); err != nil {
			t.Fatalf("append conflict event %s: %v", event.ID, err)
		}
	}

	_, err = taskSvc.Update("alpha", "api-auth", []string{"backend"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unresolved conflict") {
		t.Fatalf("task update should reject conflict, err=%v", err)
	}
	_, _, err = taskSvc.Transition("alpha", "api-auth", "plan", TransitionInput{})
	if err == nil || !strings.Contains(err.Error(), "unresolved conflict") {
		t.Fatalf("task transition should reject conflict, err=%v", err)
	}
}

func TestProjectServiceRejectsStatefulWritesWhenProjectHasDivergentHeads(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigLifecycle(t, root)

	projectSvc := NewProjectService(store.New(root), steps.New(root))
	_, err := projectSvc.Create("alpha", CreateInput{Description: "Alpha project"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	runtimeStore := store.New(root)
	manifest, err := runtimeStore.LoadProjectManifest("alpha")
	if err != nil {
		t.Fatalf("load project manifest: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, event := range []model.Event{
		{ID: "ev-plan", EntityID: manifest.ID, Kind: model.EventKindTransition, At: now, Actor: "test", Transition: &model.TransitionPayload{Verb: "plan", From: model.StatusBacklog, To: model.StatusPlanned}},
		{ID: "ev-cancel", EntityID: manifest.ID, Kind: model.EventKindTransition, At: now + "-b", Actor: "test", Transition: &model.TransitionPayload{Verb: "cancel", From: model.StatusBacklog, To: model.StatusCanceled, ReasonType: "manual", Reason: "stop"}},
	} {
		if err := runtimeStore.AppendProjectEvent("alpha", event); err != nil {
			t.Fatalf("append conflict event %s: %v", event.ID, err)
		}
	}

	_, err = projectSvc.Update("alpha", []string{"backend"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unresolved conflict") {
		t.Fatalf("project update should reject conflict, err=%v", err)
	}
	_, _, err = projectSvc.Transition("alpha", "plan", TransitionInput{})
	if err == nil || !strings.Contains(err.Error(), "unresolved conflict") {
		t.Fatalf("project transition should reject conflict, err=%v", err)
	}
}

func TestProjectServiceMiddlewareWriteFailsClosedWhenJournalAppendFails(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigLifecycle(t, root)

	projectSvc := NewProjectService(store.New(root), steps.New(root))
	project, err := projectSvc.Create("alpha", CreateInput{Description: "Alpha project"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	eventsDir := filepath.Join(root, "projects", project.Manifest.Ref(), "events")
	if err := os.Chmod(eventsDir, 0o555); err != nil {
		t.Fatalf("chmod events dir: %v", err)
	}
	defer os.Chmod(eventsDir, 0o755)

	_, err = projectSvc.runProjectMiddleware("alpha", project.Manifest.ID, "pre", "plan", []model.MiddlewareCommand{{Name: "noop", Cmd: []string{"sh", "-c", "true"}}}, steps.Context{})
	if err == nil {
		t.Fatalf("middleware journaling should fail closed when append fails")
	}
}

func writeTaskmanConfigLifecycle(t *testing.T, root string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "taskman.yaml"), []byte(model.DefaultConfigYAML), 0o644); err != nil {
		t.Fatalf("write taskman config: %v", err)
	}
}
