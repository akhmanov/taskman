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
