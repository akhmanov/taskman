package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV2InitCreatesMinimalTaskmanConfig(t *testing.T) {
	root := t.TempDir()

	if _, err := captureCLIResult(t, []string{"taskman", "--root", root, "init"}); err != nil {
		t.Fatalf("init: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "taskman.yaml"))
	if err != nil {
		t.Fatalf("read taskman.yaml: %v", err)
	}
	text := string(data)
	for _, want := range []string{"version: 2", "defaults:", "middleware:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("taskman.yaml missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "workflow:") {
		t.Fatalf("taskman.yaml should not contain workflow config: %s", text)
	}
	if _, err := os.Stat(filepath.Join(root, "projects")); !os.IsNotExist(err) {
		t.Fatalf("init should not create runtime directories, stat err=%v", err)
	}
}

func TestV2HelpExplainsFiltersAndExpansionFlags(t *testing.T) {
	taskListHelp, err := captureCLIResult(t, []string{"taskman", "task", "list", "--help"})
	if err != nil {
		t.Fatalf("task list help: %v", err)
	}
	for _, want := range []string{
		"Project slug for the task command",
		"Include only these statuses",
		"Hide these statuses",
		"Shortcut for --exclude-status done,canceled",
	} {
		if !strings.Contains(taskListHelp, want) {
			t.Fatalf("task list help missing %q: %s", want, taskListHelp)
		}
	}

	projectShowHelp, err := captureCLIResult(t, []string{"taskman", "project", "show", "--help"})
	if err != nil {
		t.Fatalf("project show help: %v", err)
	}
	if !strings.Contains(projectShowHelp, "Expand done and canceled task groups") {
		t.Fatalf("project show help missing expanded-group description: %s", projectShowHelp)
	}

	initHelp, err := captureCLIResult(t, []string{"taskman", "init", "--help"})
	if err != nil {
		t.Fatalf("init help: %v", err)
	}
	if !strings.Contains(initHelp, "Overwrite an existing taskman.yaml") {
		t.Fatalf("init help missing force description: %s", initHelp)
	}
}

func TestV2TaskListSupportsCanonicalOrderingAndFilters(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfig(t, root, minimalTaskmanConfig)
	seedProjectWithCanonicalTasks(t, root, "alpha")

	out, err := captureCLIResult(t, []string{"taskman", "--root", root, "task", "list", "-p", "alpha"})
	if err != nil {
		t.Fatalf("task list: %v", err)
	}
	assertOrdered(t, out, []string{"active-task", "paused-task", "planned-task", "backlog-task", "done-task", "canceled-task"})

	activeOut, err := captureCLIResult(t, []string{"taskman", "--root", root, "task", "list", "-p", "alpha", "--active"})
	if err != nil {
		t.Fatalf("task list --active: %v", err)
	}
	for _, forbidden := range []string{"done-task", "canceled-task"} {
		if strings.Contains(activeOut, forbidden) {
			t.Fatalf("task list --active should hide %q: %s", forbidden, activeOut)
		}
	}

	filteredOut, err := captureCLIResult(t, []string{"taskman", "--root", root, "task", "list", "-p", "alpha", "--status", "paused,done"})
	if err != nil {
		t.Fatalf("task list --status: %v", err)
	}
	for _, want := range []string{"paused-task", "done-task"} {
		if !strings.Contains(filteredOut, want) {
			t.Fatalf("task list --status missing %q: %s", want, filteredOut)
		}
	}
	for _, forbidden := range []string{"active-task", "planned-task", "backlog-task", "canceled-task"} {
		if strings.Contains(filteredOut, forbidden) {
			t.Fatalf("task list --status should hide %q: %s", forbidden, filteredOut)
		}
	}

	excludedOut, err := captureCLIResult(t, []string{"taskman", "--root", root, "task", "list", "-p", "alpha", "--exclude-status", "paused,done"})
	if err != nil {
		t.Fatalf("task list --exclude-status: %v", err)
	}
	for _, forbidden := range []string{"paused-task", "done-task"} {
		if strings.Contains(excludedOut, forbidden) {
			t.Fatalf("task list --exclude-status should hide %q: %s", forbidden, excludedOut)
		}
	}
}

func TestV2ProjectListSupportsCanonicalOrderingAndFilters(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfig(t, root, minimalTaskmanConfig)
	runCLISuccess(t, root, "project", "add", "backlog-project")
	runCLISuccess(t, root, "project", "add", "planned-project")
	runCLISuccess(t, root, "project", "add", "active-project")
	runCLISuccess(t, root, "project", "add", "paused-project")
	runCLISuccess(t, root, "project", "add", "done-project")
	runCLISuccess(t, root, "project", "add", "canceled-project")

	runCLISuccess(t, root, "project", "plan", "planned-project")
	runCLISuccess(t, root, "project", "plan", "active-project")
	runCLISuccess(t, root, "project", "start", "active-project")
	runCLISuccess(t, root, "project", "plan", "paused-project")
	runCLISuccess(t, root, "project", "start", "paused-project")
	runCLISuccess(t, root, "project", "pause", "paused-project", "--reason-type", "waiting_feedback", "--reason", "Waiting", "--resume-when", "Soon")
	runCLISuccess(t, root, "project", "plan", "done-project")
	runCLISuccess(t, root, "project", "complete", "done-project", "--summary", "Shipped")
	runCLISuccess(t, root, "project", "cancel", "canceled-project", "--reason-type", "deprioritized", "--reason", "Dropped")

	out, err := captureCLIResult(t, []string{"taskman", "--root", root, "project", "list"})
	if err != nil {
		t.Fatalf("project list: %v", err)
	}
	assertOrdered(t, out, []string{"active-project", "paused-project", "planned-project", "backlog-project", "done-project", "canceled-project"})

	activeOut, err := captureCLIResult(t, []string{"taskman", "--root", root, "project", "list", "--active"})
	if err != nil {
		t.Fatalf("project list --active: %v", err)
	}
	for _, forbidden := range []string{"done-project", "canceled-project"} {
		if strings.Contains(activeOut, forbidden) {
			t.Fatalf("project list --active should hide %q: %s", forbidden, activeOut)
		}
	}

	filteredOut, err := captureCLIResult(t, []string{"taskman", "--root", root, "project", "list", "--status", "paused,done"})
	if err != nil {
		t.Fatalf("project list --status: %v", err)
	}
	for _, want := range []string{"paused-project", "done-project"} {
		if !strings.Contains(filteredOut, want) {
			t.Fatalf("project list --status missing %q: %s", want, filteredOut)
		}
	}
}

func TestV2ProjectShowGroupsTasksAndCollapsesTerminalBuckets(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfig(t, root, minimalTaskmanConfig)
	seedProjectWithCanonicalTasks(t, root, "alpha")

	out, err := captureCLIResult(t, []string{"taskman", "--root", root, "project", "show", "alpha"})
	if err != nil {
		t.Fatalf("project show: %v", err)
	}
	assertOrdered(t, out, []string{"in_progress:", "paused:", "planned:", "backlog:", "done: 1 task", "canceled: 1 task"})
	if !strings.Contains(out, "Waiting for API review") {
		t.Fatalf("project show should include pause reason: %s", out)
	}
	for _, forbidden := range []string{"Merged cleanly", "Cut from scope"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("project show should collapse terminal details by default, found %q in: %s", forbidden, out)
		}
	}

	allOut, err := captureCLIResult(t, []string{"taskman", "--root", root, "project", "show", "alpha", "--all"})
	if err != nil {
		t.Fatalf("project show --all: %v", err)
	}
	for _, want := range []string{"Merged cleanly", "Cut from scope"} {
		if !strings.Contains(allOut, want) {
			t.Fatalf("project show --all missing %q: %s", want, allOut)
		}
	}
}

func TestV2TransitionMiddlewareSeparatesAuditTrailAndDoesNotRollbackPostFailures(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfig(t, root, preMiddlewareTaskmanConfig)
	writeCLIExecutable(t, filepath.Join(root, "bin", "pre-start"), "#!/bin/sh\nprintf '{\"ok\":false,\"message\":\"branch missing\"}'\n")
	writeCLIExecutable(t, filepath.Join(root, "bin", "post-complete"), "#!/bin/sh\nprintf '{\"ok\":false,\"message\":\"dirty worktree\",\"warnings\":[\"dirty worktree detected\"]}'\n")

	runCLISuccess(t, root, "project", "add", "alpha")
	runCLISuccess(t, root, "project", "plan", "alpha")
	runCLISuccess(t, root, "project", "start", "alpha")
	runCLISuccess(t, root, "task", "add", "blocked-task", "-p", "alpha")
	runCLISuccess(t, root, "task", "plan", "blocked-task", "-p", "alpha")

	if _, err := captureCLIResult(t, []string{"taskman", "--root", root, "task", "start", "blocked-task", "-p", "alpha"}); err == nil || !strings.Contains(err.Error(), "branch missing") {
		t.Fatalf("task start should fail with pre-middleware message, err=%v", err)
	}
	blockedState, err := os.ReadFile(filepath.Join(root, "projects", "alpha", "tasks", "blocked-task", "state.yaml"))
	if err != nil {
		t.Fatalf("read blocked task state: %v", err)
	}
	if !strings.Contains(string(blockedState), "status: planned") {
		t.Fatalf("pre-middleware failure should keep task planned: %s", string(blockedState))
	}

	writeTaskmanConfig(t, root, postMiddlewareTaskmanConfig)
	runCLISuccess(t, root, "task", "add", "done-task", "-p", "alpha")
	runCLISuccess(t, root, "task", "plan", "done-task", "-p", "alpha")
	runCLISuccess(t, root, "task", "start", "done-task", "-p", "alpha")
	out, err := captureCLIResult(t, []string{"taskman", "--root", root, "task", "complete", "done-task", "-p", "alpha", "--summary", "Shipped"})
	if err != nil {
		t.Fatalf("task complete should succeed despite post failure: %v", err)
	}
	if !strings.Contains(out, "dirty worktree detected") {
		t.Fatalf("task complete should surface post-middleware warning: %s", out)
	}
	doneState, err := os.ReadFile(filepath.Join(root, "projects", "alpha", "tasks", "done-task", "state.yaml"))
	if err != nil {
		t.Fatalf("read done task state: %v", err)
	}
	if !strings.Contains(string(doneState), "status: done") {
		t.Fatalf("post-middleware failure must not rollback status: %s", string(doneState))
	}
	transitionsData, err := os.ReadFile(filepath.Join(root, "projects", "alpha", "tasks", "done-task", "transitions.yaml"))
	if err != nil {
		t.Fatalf("read transitions: %v", err)
	}
	if !strings.Contains(string(transitionsData), "to: done") || !strings.Contains(string(transitionsData), "dirty worktree") {
		t.Fatalf("transition history missing completion audit details: %s", string(transitionsData))
	}
	eventsData, err := os.ReadFile(filepath.Join(root, "projects", "alpha", "tasks", "done-task", "events.yaml"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if strings.Contains(string(eventsData), "to: done") {
		t.Fatalf("events and transitions must remain separate: %s", string(eventsData))
	}
}

func TestV2PostMiddlewareReceivesUpdatedStatusContext(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfig(t, root, postStatusCheckConfig)
	writeCLIExecutable(t, filepath.Join(root, "bin", "check-post-status"), "#!/bin/sh\nif grep -q '\"status\":\"done\"' \"$1\"; then\n  printf '{\"ok\":true,\"message\":\"saw done\"}'\nelse\n  printf '{\"ok\":false,\"message\":\"post saw stale status\"}'\nfi\n")

	runCLISuccess(t, root, "project", "add", "alpha")
	runCLISuccess(t, root, "project", "plan", "alpha")
	runCLISuccess(t, root, "project", "start", "alpha")
	runCLISuccess(t, root, "task", "add", "done-task", "-p", "alpha")
	runCLISuccess(t, root, "task", "plan", "done-task", "-p", "alpha")
	runCLISuccess(t, root, "task", "start", "done-task", "-p", "alpha")
	out, err := captureCLIResult(t, []string{"taskman", "--root", root, "task", "complete", "done-task", "-p", "alpha", "--summary", "Shipped"})
	if err != nil {
		t.Fatalf("task complete: %v\n%s", err, out)
	}
	if strings.Contains(out, "post saw stale status") {
		t.Fatalf("post middleware should receive updated status context: %s", out)
	}
}

func TestV2ConfigRejectsUnknownMiddlewareTransitions(t *testing.T) {
	root := t.TempDir()
	writeTaskmanConfig(t, root, `version: 2
defaults:
  project:
    labels: []
  task:
    labels: []
middleware:
  project: {}
  task:
    invent:
      pre:
        - name: bad
          cmd: [./bin/nope]
`)
	_, err := captureCLIResult(t, []string{"taskman", "--root", root, "project", "list"})
	if err == nil || !strings.Contains(err.Error(), "unknown task middleware transition") {
		t.Fatalf("expected unknown middleware transition error, got %v", err)
	}
}

const minimalTaskmanConfig = `version: 2
defaults:
  project:
    labels: []
  task:
    labels: []
middleware:
  project: {}
  task: {}
`

const preMiddlewareTaskmanConfig = `version: 2
defaults:
  project:
    labels: []
  task:
    labels: []
middleware:
  project: {}
  task:
    start:
      pre:
        - name: pre-start
          cmd: [./bin/pre-start]
`

const postMiddlewareTaskmanConfig = `version: 2
defaults:
  project:
    labels: []
  task:
    labels: []
middleware:
  project: {}
  task:
    complete:
      post:
        - name: post-complete
          cmd: [./bin/post-complete]
`

const postStatusCheckConfig = `version: 2
defaults:
  project:
    labels: []
  task:
    labels: []
middleware:
  project: {}
  task:
    complete:
      post:
        - name: check-post-status
          cmd: [./bin/check-post-status, "{{.input_json_path}}"]
`

func seedProjectWithCanonicalTasks(t *testing.T, root string, project string) {
	t.Helper()
	runCLISuccess(t, root, "project", "add", project)
	runCLISuccess(t, root, "project", "plan", project)
	runCLISuccess(t, root, "project", "start", project)

	runCLISuccess(t, root, "task", "add", "active-task", "-p", project)
	runCLISuccess(t, root, "task", "plan", "active-task", "-p", project)
	runCLISuccess(t, root, "task", "start", "active-task", "-p", project)

	runCLISuccess(t, root, "task", "add", "paused-task", "-p", project)
	runCLISuccess(t, root, "task", "plan", "paused-task", "-p", project)
	runCLISuccess(t, root, "task", "start", "paused-task", "-p", project)
	runCLISuccess(t, root, "task", "pause", "paused-task", "-p", project, "--reason-type", "waiting_feedback", "--reason", "Waiting for API review", "--resume-when", "After schema approval")

	runCLISuccess(t, root, "task", "add", "planned-task", "-p", project)
	runCLISuccess(t, root, "task", "plan", "planned-task", "-p", project)

	runCLISuccess(t, root, "task", "add", "backlog-task", "-p", project)

	runCLISuccess(t, root, "task", "add", "done-task", "-p", project)
	runCLISuccess(t, root, "task", "plan", "done-task", "-p", project)
	runCLISuccess(t, root, "task", "start", "done-task", "-p", project)
	runCLISuccess(t, root, "task", "complete", "done-task", "-p", project, "--summary", "Merged cleanly")

	runCLISuccess(t, root, "task", "add", "canceled-task", "-p", project)
	runCLISuccess(t, root, "task", "cancel", "canceled-task", "-p", project, "--reason-type", "deprioritized", "--reason", "Cut from scope")
}

func writeTaskmanConfig(t *testing.T, root string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "taskman.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write taskman config: %v", err)
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

func runCLISuccess(t *testing.T, root string, args ...string) string {
	t.Helper()
	argv := append([]string{"taskman", "--root", root}, args...)
	out, err := captureCLIResult(t, argv)
	if err != nil {
		t.Fatalf("run %v: %v\n%s", argv, err, out)
	}
	return out
}

func captureCLIResult(t *testing.T, args []string) (string, error) {
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

func assertOrdered(t *testing.T, text string, fragments []string) {
	t.Helper()
	last := -1
	for _, fragment := range fragments {
		idx := strings.Index(text, fragment)
		if idx == -1 {
			t.Fatalf("missing fragment %q in output: %s", fragment, text)
		}
		if idx <= last {
			t.Fatalf("fragment %q appeared out of order in output: %s", fragment, text)
		}
		last = idx
	}
}
