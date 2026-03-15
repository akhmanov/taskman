package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskmanV3TaskShowReadsFromManifestAndEvents(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth", "--label", "backend", "--var", "repo=taskman")

	out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "show", "api-auth", "-p", "alpha"})
	if err != nil {
		t.Fatalf("task show: %v\n%s", err, out)
	}

	for _, want := range []string{
		"Task: alpha/api-auth",
		"Description: Implement API auth",
		"Labels: backend",
		"Vars: repo=taskman",
		"Allowed Next: plan, cancel",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("task show missing %q: %s", want, out)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "projects", "alpha", "tasks", "api-auth", "manifest.json")); err != nil {
		t.Fatalf("manifest.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "alpha", "tasks", "api-auth", "events")); err != nil {
		t.Fatalf("events dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "alpha", "tasks", "api-auth", "state.yaml")); !os.IsNotExist(err) {
		t.Fatalf("state.yaml should not exist in v3, stat err=%v", err)
	}
}

func TestTaskmanV3TaskTransitionListAndMessages(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "project", "plan", "alpha")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth")
	runCLISuccessV3(t, root, "task", "message", "add", "api-auth", "-p", "alpha", "--kind", "decision", "--body", "Use token auth")
	runCLISuccessV3(t, root, "task", "plan", "api-auth", "-p", "alpha")
	runCLISuccessV3(t, root, "task", "start", "api-auth", "-p", "alpha")

	transitionOut, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "transition", "list", "api-auth", "-p", "alpha"})
	if err != nil {
		t.Fatalf("task transition list: %v\n%s", err, transitionOut)
	}
	for _, want := range []string{"plan\tbacklog -> planned", "start\tplanned -> in_progress"} {
		if !strings.Contains(transitionOut, want) {
			t.Fatalf("transition list missing %q: %s", want, transitionOut)
		}
	}

	messageOut, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "message", "list", "api-auth", "-p", "alpha"})
	if err != nil {
		t.Fatalf("task message list: %v\n%s", err, messageOut)
	}
	if !strings.Contains(messageOut, "decision\tUse token auth") {
		t.Fatalf("message list missing decision body: %s", messageOut)
	}
}

func TestTaskmanV3ProjectShowAndTaskShowSurfaceRecentContext(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "project", "message", "add", "alpha", "--kind", "note", "--body", "Project scope frozen")
	runCLISuccessV3(t, root, "project", "plan", "alpha")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth")
	runCLISuccessV3(t, root, "task", "message", "add", "api-auth", "-p", "alpha", "--kind", "decision", "--body", "Use token auth")
	runCLISuccessV3(t, root, "task", "plan", "api-auth", "-p", "alpha")

	projectOut, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "project", "show", "alpha"})
	if err != nil {
		t.Fatalf("project show: %v\n%s", err, projectOut)
	}
	for _, want := range []string{
		"Description: Alpha project",
		"Recent Project Messages:",
		"- note: Project scope frozen",
	} {
		if !strings.Contains(projectOut, want) {
			t.Fatalf("project show missing %q: %s", want, projectOut)
		}
	}

	taskOut, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "show", "api-auth", "-p", "alpha"})
	if err != nil {
		t.Fatalf("task show: %v\n%s", err, taskOut)
	}
	for _, want := range []string{
		"Messages: 1",
		"Last Message: decision - Use token auth",
	} {
		if !strings.Contains(taskOut, want) {
			t.Fatalf("task show missing %q: %s", want, taskOut)
		}
	}
}

func TestTaskmanV3TaskShowFiltersAllowedNextByProjectState(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth")
	runCLISuccessV3(t, root, "task", "plan", "api-auth", "-p", "alpha")

	out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "show", "api-auth", "-p", "alpha"})
	if err != nil {
		t.Fatalf("task show: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Allowed Next: complete, cancel") {
		t.Fatalf("task show should hide start while project is backlog: %s", out)
	}
	if strings.Contains(out, "Allowed Next: start") || strings.Contains(out, "start, complete") {
		t.Fatalf("task show should not advertise start while project is backlog: %s", out)
	}
}

func TestTaskmanV3MessageAddRejectsUnknownKind(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth")

	out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "message", "add", "api-auth", "-p", "alpha", "--kind", "typo", "--body", "bad kind"})
	if err == nil || !strings.Contains(out+err.Error(), "unknown message kind") {
		t.Fatalf("unknown message kind should fail, err=%v out=%s", err, out)
	}
}

func TestTaskmanV3WorksWithoutTaskmanConfigOverlay(t *testing.T) {
	root := t.TempDir()

	listOut, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "project", "list"})
	if err != nil {
		t.Fatalf("project list without config should work: %v\n%s", err, listOut)
	}
	if strings.TrimSpace(listOut) != "" {
		t.Fatalf("empty runtime should produce no project list output, got %q", listOut)
	}

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth")
	runCLISuccessV3(t, root, "project", "plan", "alpha")
	runCLISuccessV3(t, root, "task", "plan", "api-auth", "-p", "alpha")

	out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "show", "api-auth", "-p", "alpha"})
	if err != nil {
		t.Fatalf("task show without config should work: %v\n%s", err, out)
	}
	for _, want := range []string{"Description: Implement API auth", "Status: planned"} {
		if !strings.Contains(out, want) {
			t.Fatalf("task show without config missing %q: %s", want, out)
		}
	}
	if strings.Contains(out, "No taskman.yaml found") {
		t.Fatalf("missing config should not be surfaced in runtime output: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "taskman.yaml")); !os.IsNotExist(err) {
		t.Fatalf("runtime without overlay should not create taskman.yaml implicitly, stat err=%v", err)
	}
}

func TestTaskmanV3FailsWhenExistingConfigOverlayIsInvalid(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "taskman.yaml"), []byte("middleware:\n  project:\n    plan:\n      pre:\n        - name: broken\n"), 0o644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "projects", "alpha", "events"), 0o755); err != nil {
		t.Fatalf("mkdir project events: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "alpha", "manifest.json"), []byte("{\n  \"id\": \"project-1\",\n  \"kind\": \"project\",\n  \"slug\": \"alpha\",\n  \"name\": \"alpha\",\n  \"description\": \"Alpha project\",\n  \"created_at\": \"2026-03-15T00:00:00Z\"\n}\n"), 0o644); err != nil {
		t.Fatalf("write project manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "projects", "alpha", "tasks", "api-auth", "events"), 0o755); err != nil {
		t.Fatalf("mkdir task events: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "projects", "alpha", "tasks", "api-auth", "artifacts"), 0o755); err != nil {
		t.Fatalf("mkdir task artifacts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "alpha", "tasks", "api-auth", "manifest.json"), []byte("{\n  \"id\": \"task-1\",\n  \"kind\": \"task\",\n  \"slug\": \"api-auth\",\n  \"name\": \"api-auth\",\n  \"description\": \"Implement API auth\",\n  \"project_id\": \"project-1\",\n  \"project_slug\": \"alpha\",\n  \"created_at\": \"2026-03-15T00:00:00Z\"\n}\n"), 0o644); err != nil {
		t.Fatalf("write task manifest: %v", err)
	}

	for _, args := range [][]string{
		{"taskman", "--root", root, "project", "list"},
		{"taskman", "--root", root, "project", "add", "alpha", "--description", "Alpha project"},
		{"taskman", "--root", root, "project", "message", "list", "alpha"},
		{"taskman", "--root", root, "project", "update", "alpha", "--label", "ops"},
		{"taskman", "--root", root, "task", "message", "list", "api-auth", "-p", "alpha"},
		{"taskman", "--root", root, "task", "update", "api-auth", "-p", "alpha", "--label", "backend"},
	} {
		out, err := captureCLIResultV3(t, args)
		if err == nil {
			t.Fatalf("invalid existing config should fail for %v, output=%s", args, out)
		}
		if !strings.Contains(out+err.Error(), "empty cmd") {
			t.Fatalf("invalid config failure should mention validation problem for %v, err=%v out=%s", args, err, out)
		}
	}
}

func TestTaskmanV3TransitionWithoutOverlayDoesNotWriteMiddlewareEvents(t *testing.T) {
	root := t.TempDir()

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "project", "plan", "alpha")
	eventsDir := filepath.Join(root, "projects", "alpha", "events")
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		t.Fatalf("read project events: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "middleware_") {
			t.Fatalf("missing overlay should not emit middleware events, found %s", entry.Name())
		}
	}
}

const minimalTaskmanConfigV3 = `defaults:
  project:
    labels: []
  task:
    labels: []
middleware:
  project: {}
  task: {}
`

func writeTaskmanConfigV3(t *testing.T, root string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "taskman.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write taskman config: %v", err)
	}
}

func runCLISuccessV3(t *testing.T, root string, args ...string) string {
	t.Helper()
	argv := append([]string{"taskman", "--root", root}, args...)
	out, err := captureCLIResultV3(t, argv)
	if err != nil {
		t.Fatalf("run %v: %v\n%s", argv, err, out)
	}
	return out
}

func captureCLIResultV3(t *testing.T, args []string) (string, error) {
	t.Helper()
	cmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()
	runErr := cmd.Run(context.Background(), args)
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	return string(data), runErr
}
