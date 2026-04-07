package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
		"Task: 1_alpha/1_api-auth",
		"Description: Implement API auth",
		"Labels: backend",
		"Vars: repo=taskman",
		"Allowed Next: plan, cancel",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("task show missing %q: %s", want, out)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "projects", "1_alpha", "tasks", "1_api-auth", "manifest.json")); err != nil {
		t.Fatalf("manifest.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "1_alpha", "tasks", "1_api-auth", "events")); err != nil {
		t.Fatalf("events dir missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "1_alpha", "tasks", "1_api-auth", "state.yaml")); !os.IsNotExist(err) {
		t.Fatalf("state.yaml should not exist in v3, stat err=%v", err)
	}
}

func TestTaskmanV3CreateUsesNumberedDirectories(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth")

	for _, path := range []string{
		filepath.Join(root, "projects", "1_alpha", "manifest.json"),
		filepath.Join(root, "projects", "1_alpha", "tasks", "1_api-auth", "manifest.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected numbered path %s: %v", path, err)
		}
	}
}

func TestTaskmanV3ProjectListUsesCanonicalRefsAndNumberOrder(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	writeProjectFixtureV3(t, root, fixtureProjectV3{DirName: "25_beta", ID: "project-25", Number: 25, Slug: "beta", Name: "beta", Description: "Beta project", CreatedAt: "2026-03-15T00:02:00Z"})
	writeProjectFixtureV3(t, root, fixtureProjectV3{DirName: "3_alpha", ID: "project-3", Number: 3, Slug: "alpha", Name: "alpha", Description: "Alpha project", CreatedAt: "2026-03-15T00:01:00Z"})

	out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "project", "list"})
	if err != nil {
		t.Fatalf("project list: %v\n%s", err, out)
	}

	if got, want := strings.TrimSpace(out), strings.Join([]string{"backlog\t3_alpha", "backlog\t25_beta"}, "\n"); got != want {
		t.Fatalf("project list = %q, want %q", got, want)
	}
}

func TestTaskmanV3ProjectShowAcceptsCompositeNumberAndSlugRefs(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)
	writeProjectFixtureV3(t, root, fixtureProjectV3{DirName: "25_alpha", ID: "project-25", Number: 25, Slug: "alpha", Name: "alpha", Description: "Alpha project", CreatedAt: "2026-03-15T00:01:00Z"})

	for _, ref := range []string{"25_alpha", "25", "alpha"} {
		out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "project", "show", ref})
		if err != nil {
			t.Fatalf("project show %s: %v\n%s", ref, err, out)
		}
		if !strings.Contains(out, "Project: 25_alpha") {
			t.Fatalf("project show %s should use canonical ref, got %s", ref, out)
		}
	}
}

func TestTaskmanV3ProjectLabelCommandsNormalizeAndRenderLabels(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "project", "label", "add", "alpha", "Feature", "feature", "  cleanup  ")

	out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "project", "show", "alpha"})
	if err != nil {
		t.Fatalf("project show: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Labels: cleanup, feature") {
		t.Fatalf("project show should render normalized labels, got %s", out)
	}

	runCLISuccessV3(t, root, "project", "label", "remove", "alpha", "cleanup")
	out, err = captureCLIResultV3(t, []string{"taskman", "--root", root, "project", "show", "alpha"})
	if err != nil {
		t.Fatalf("project show after remove: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Labels: feature") || strings.Contains(out, "cleanup") {
		t.Fatalf("project show should remove cleanup label, got %s", out)
	}
}

func TestTaskmanV3TaskListFiltersByAnyLabel(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth", "--label", "feature")
	runCLISuccessV3(t, root, "task", "add", "cleanup-docs", "-p", "alpha", "--description", "Cleanup docs", "--label", "cleanup")
	runCLISuccessV3(t, root, "task", "add", "cleanup-api", "-p", "alpha", "--description", "Cleanup api", "--label", "cleanup", "--label", "feature")

	out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "list", "-p", "alpha", "--label", "cleanup"})
	if err != nil {
		t.Fatalf("task list --label cleanup: %v\n%s", err, out)
	}
	if got, want := strings.TrimSpace(out), strings.Join([]string{"backlog\t1_alpha/2_cleanup-docs", "backlog\t1_alpha/3_cleanup-api"}, "\n"); got != want {
		t.Fatalf("task list cleanup = %q, want %q", got, want)
	}

	out, err = captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "list", "-p", "alpha", "--label", "cleanup", "--label", "backend"})
	if err != nil {
		t.Fatalf("task list any-of labels: %v\n%s", err, out)
	}
	if got, want := strings.TrimSpace(out), strings.Join([]string{"backlog\t1_alpha/2_cleanup-docs", "backlog\t1_alpha/3_cleanup-api"}, "\n"); got != want {
		t.Fatalf("task list any-of = %q, want %q", got, want)
	}
}

func TestTaskmanV3ProjectRenameMovesSubtreeAndUpdatesRefs(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth")
	runCLISuccessV3(t, root, "project", "rename", "alpha", "beta")

	if _, err := os.Stat(filepath.Join(root, "projects", "1_beta", "manifest.json")); err != nil {
		t.Fatalf("renamed project manifest missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "1_alpha")); !os.IsNotExist(err) {
		t.Fatalf("old project dir should be gone, stat err=%v", err)
	}
	out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "project", "show", "beta"})
	if err != nil {
		t.Fatalf("project show beta: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Project: 1_beta") {
		t.Fatalf("project show should use renamed ref, got %s", out)
	}
	out, err = captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "show", "api-auth", "-p", "beta"})
	if err != nil {
		t.Fatalf("task show after project rename: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Task: 1_beta/1_api-auth") {
		t.Fatalf("task show should use renamed project ref, got %s", out)
	}
}

func TestTaskmanV3TaskRenameMovesDirectoryAndUpdatesRef(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth")
	runCLISuccessV3(t, root, "task", "rename", "api-auth", "renamed-auth", "-p", "alpha")

	if _, err := os.Stat(filepath.Join(root, "projects", "1_alpha", "tasks", "1_renamed-auth", "manifest.json")); err != nil {
		t.Fatalf("renamed task manifest missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "projects", "1_alpha", "tasks", "1_api-auth")); !os.IsNotExist(err) {
		t.Fatalf("old task dir should be gone, stat err=%v", err)
	}
	out, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "show", "renamed-auth", "-p", "alpha"})
	if err != nil {
		t.Fatalf("task show renamed-auth: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Task: 1_alpha/1_renamed-auth") {
		t.Fatalf("task show should use renamed task ref, got %s", out)
	}
}

func TestTaskmanV3ReadCommandsSupportJSONOutput(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfigV3(t, root, minimalTaskmanConfigV3)

	runCLISuccessV3(t, root, "project", "add", "alpha", "--description", "Alpha project", "--label", "feature")
	runCLISuccessV3(t, root, "project", "plan", "alpha")
	runCLISuccessV3(t, root, "task", "add", "api-auth", "-p", "alpha", "--description", "Implement API auth", "--label", "backend")
	runCLISuccessV3(t, root, "task", "message", "add", "api-auth", "-p", "alpha", "--kind", "decision", "--body", "Use token auth")
	runCLISuccessV3(t, root, "task", "plan", "api-auth", "-p", "alpha")

	projectListOut, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "project", "list", "--output", "json"})
	if err != nil {
		t.Fatalf("project list json: %v\n%s", err, projectListOut)
	}
	for _, want := range []string{`"number": 1`, `"slug": "alpha"`, `"status": "planned"`} {
		if !strings.Contains(strings.ToLower(projectListOut), strings.ToLower(want)) {
			t.Fatalf("project list json missing %q: %s", want, projectListOut)
		}
	}

	taskShowOut, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "show", "api-auth", "-p", "alpha", "--output", "json"})
	if err != nil {
		t.Fatalf("task show json: %v\n%s", err, taskShowOut)
	}
	for _, want := range []string{`"task"`, `"number": 1`, `"project_number": 1`, `"status": "planned"`} {
		if !strings.Contains(strings.ToLower(taskShowOut), strings.ToLower(want)) {
			t.Fatalf("task show json missing %q: %s", want, taskShowOut)
		}
	}

	messageOut, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "message", "list", "api-auth", "-p", "alpha", "--output", "json"})
	if err != nil {
		t.Fatalf("task message list json: %v\n%s", err, messageOut)
	}
	for _, want := range []string{`"kind": "message"`, `"decision"`, `"Use token auth"`} {
		if !strings.Contains(messageOut, want) {
			t.Fatalf("task message json missing %q: %s", want, messageOut)
		}
	}

	transitionOut, err := captureCLIResultV3(t, []string{"taskman", "--root", root, "task", "transition", "list", "api-auth", "-p", "alpha", "--output", "json"})
	if err != nil {
		t.Fatalf("task transition list json: %v\n%s", err, transitionOut)
	}
	for _, want := range []string{`"kind": "transition"`, `"verb": "plan"`, `"to": "planned"`} {
		if !strings.Contains(transitionOut, want) {
			t.Fatalf("task transition json missing %q: %s", want, transitionOut)
		}
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
	eventsDir := filepath.Join(root, "projects", "1_alpha", "events")
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

type fixtureProjectV3 struct {
	DirName      string
	ID           string
	Number       int
	Slug         string
	Name         string
	Description  string
	CreatedAt    string
	TaskFixtures []fixtureTaskV3
}

type fixtureTaskV3 struct {
	DirName     string
	ID          string
	Number      int
	Slug        string
	Name        string
	Description string
	ProjectID   string
	ProjectSlug string
	CreatedAt   string
}

func writeProjectFixtureV3(t *testing.T, root string, fixture fixtureProjectV3) {
	t.Helper()
	projectDir := filepath.Join(root, "projects", fixture.DirName)
	if err := os.MkdirAll(filepath.Join(projectDir, "events"), 0o755); err != nil {
		t.Fatalf("mkdir project events: %v", err)
	}
	manifest := strings.Join([]string{
		"{",
		`  "id": "` + fixture.ID + `",`,
		`  "kind": "project",`,
		`  "number": ` + strconv.Itoa(fixture.Number) + `,`,
		`  "slug": "` + fixture.Slug + `",`,
		`  "name": "` + fixture.Name + `",`,
		`  "description": "` + fixture.Description + `",`,
		`  "created_at": "` + fixture.CreatedAt + `"`,
		"}",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(projectDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write project manifest: %v", err)
	}
	for _, task := range fixture.TaskFixtures {
		writeTaskFixtureV3(t, root, fixture.DirName, task)
	}
}

func writeTaskFixtureV3(t *testing.T, root string, projectDir string, fixture fixtureTaskV3) {
	t.Helper()
	taskDir := filepath.Join(root, "projects", projectDir, "tasks", fixture.DirName)
	if err := os.MkdirAll(filepath.Join(taskDir, "events"), 0o755); err != nil {
		t.Fatalf("mkdir task events: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(taskDir, "artifacts"), 0o755); err != nil {
		t.Fatalf("mkdir task artifacts: %v", err)
	}
	manifest := strings.Join([]string{
		"{",
		`  "id": "` + fixture.ID + `",`,
		`  "kind": "task",`,
		`  "number": ` + strconv.Itoa(fixture.Number) + `,`,
		`  "slug": "` + fixture.Slug + `",`,
		`  "name": "` + fixture.Name + `",`,
		`  "description": "` + fixture.Description + `",`,
		`  "project_id": "` + fixture.ProjectID + `",`,
		`  "project_slug": "` + fixture.ProjectSlug + `",`,
		`  "created_at": "` + fixture.CreatedAt + `"`,
		"}",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(taskDir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write task manifest: %v", err)
	}
}
