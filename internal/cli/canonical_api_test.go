package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akhmanov/taskman/internal/model"
	"github.com/akhmanov/taskman/internal/store"
	urfavecli "github.com/urfave/cli/v3"
)

func TestBuildAppHelpListsCanonicalResourcesAndVerbs(t *testing.T) {
	cmd := BuildApp()
	var stdout bytes.Buffer
	cmd.Writer = &stdout
	cmd.ErrWriter = &stdout

	if err := cmd.Run(context.Background(), []string{"taskman", "--help"}); err != nil {
		t.Fatalf("run help: %v", err)
	}

	help := stdout.String()
	for _, want := range []string{"project", "task", "doctor", "add", "list", "show", "start", "complete", "close"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}
}

func TestTaskAddHelpIncludesShortProjectFlag(t *testing.T) {
	cmd := BuildApp()
	var stdout bytes.Buffer
	cmd.Writer = &stdout
	cmd.ErrWriter = &stdout

	if err := cmd.Run(context.Background(), []string{"taskman", "task", "add", "--help"}); err != nil {
		t.Fatalf("run help: %v", err)
	}

	help := stdout.String()
	for _, want := range []string{"-p", "--project"} {
		if !strings.Contains(help, want) {
			t.Fatalf("task add help missing %q: %s", want, help)
		}
	}
}

func TestCanonicalProjectTaskFlowUsesProjectFlagAddressing(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars:
      kind: feature
vars:
  task:
    kind:
      allowed: [feature, chore]
workflow:
  task:
    statuses: [todo, active, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
        steps:
          - name: write-summary
            cmd: [./bin/start-task]
`)
	writeCLIExecutable(t, filepath.Join(root, "bin", "start-task"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"started\",\"artifacts\":{\"summary\":{\"state\":\"started\"}}}'\n")

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "project", "add", "user-permissions"}); err != nil {
		t.Fatalf("project add: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "task", "add", "api-auth", "-p", "user-permissions"}); err != nil {
		t.Fatalf("task add: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "task", "start", "api-auth", "-p", "user-permissions"}); err != nil {
		t.Fatalf("task start: %v", err)
	}

	listOutput := captureCLIOutput(t, []string{"taskman", "--root", root, "task", "list", "-p", "user-permissions"})
	if !strings.Contains(listOutput, "user-permissions/api-auth") {
		t.Fatalf("task list missing canonical task id: %s", listOutput)
	}

	showOutput := captureCLIOutput(t, []string{"taskman", "--root", root, "task", "show", "api-auth", "-p", "user-permissions"})
	if !strings.Contains(showOutput, "user-permissions/api-auth") {
		t.Fatalf("task show missing task details: %s", showOutput)
	}

	s := store.New(root)
	task, err := s.LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Status != model.TaskStatus("active") {
		t.Fatalf("status = %q, want active", task.Status)
	}
	artifact, err := s.LoadArtifact("user-permissions", "api-auth", "summary")
	if err != nil {
		t.Fatalf("load summary artifact: %v", err)
	}
	if artifact.Data["state"] != "started" {
		t.Fatalf("summary state = %#v", artifact.Data["state"])
	}
}

func TestDoctorUsesTaskmanRootEnv(t *testing.T) {
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
	t.Setenv("TASKMAN_ROOT", root)

	out := captureCLIOutput(t, []string{"taskman", "doctor"})
	if !strings.Contains(out, "ok") {
		t.Fatalf("doctor output = %q", out)
	}
}

func TestTaskBriefSetReadsFromStdinWithFileDash(t *testing.T) {
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
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	brief := "# Intent\n\nStream from stdin.\n\n# Scope In\n\nCLI UX.\n\n# Scope Out\n\nEditor.\n\n# Acceptance\n\nBrief saves.\n\n# Current Context\n\nTask exists.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nList tasks.\n\n# References\n\n- stdin://test\n"
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	if _, err := io.WriteString(w, brief); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	originalStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = originalStdin }()

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "task", "brief", "set", "api-auth", "-p", "user-permissions", "--file", "-"}); err != nil {
		t.Fatalf("task brief set from stdin: %v", err)
	}

	got, err := s.LoadTaskBrief("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task brief: %v", err)
	}
	if got != brief {
		t.Fatalf("brief = %q", got)
	}
}

func TestTaskEventAddRejectsDuplicateIDs(t *testing.T) {
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
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	cmd := BuildApp()
	args := []string{"taskman", "--root", root, "task", "event", "add", "api-auth", "-p", "user-permissions", "--id", "EVT-001", "--at", "2026-03-14T13:00:00Z", "--type", "note", "--summary", "first", "--actor", "taskman"}
	if err := cmd.Run(context.Background(), args); err != nil {
		t.Fatalf("first task event add: %v", err)
	}
	if err := cmd.Run(context.Background(), args); err == nil {
		t.Fatal("expected duplicate event id error")
	}
}

func TestTaskShowAgentViewPrefersStartOverCancel(t *testing.T) {
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
    statuses: [todo, active, cancelled, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
      cancel:
        from: [todo, active]
        to: cancelled
      close:
        from: [cancelled]
        to: closed
`)
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "alpha", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.SaveProjectBrief("alpha", model.ProjectBriefTemplate); err != nil {
		t.Fatalf("save project brief: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "demo", Project: "alpha", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}
	if err := s.SaveTaskBrief("alpha", "demo", model.TaskBriefTemplate); err != nil {
		t.Fatalf("save task brief: %v", err)
	}

	out := captureCLIOutput(t, []string{"taskman", "--root", root, "task", "show", "demo", "-p", "alpha", "--view", "agent"})
	if !strings.Contains(out, "Next Action: Run transition: start") {
		t.Fatalf("unexpected next action output: %s", out)
	}
	if strings.Contains(out, "Allowed Transitions: cancel, start") {
		t.Fatalf("unexpected allowed transition ordering: %s", out)
	}
}

func TestProjectAddPrintsSuccessSummary(t *testing.T) {
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

	out := captureCLIOutput(t, []string{"taskman", "--root", root, "project", "add", "user-permissions"})
	for _, want := range []string{"Project: user-permissions", "Status: active"} {
		if !strings.Contains(out, want) {
			t.Fatalf("project add summary missing %q: %s", want, out)
		}
	}
}

func TestTaskAddAndStartPrintSuccessSummaries(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars:
      kind: feature
vars:
  task:
    kind:
      allowed: [feature, chore]
workflow:
  task:
    statuses: [todo, active, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
        steps:
          - name: create_worktree
            cmd: [./bin/start-task]
`)
	writeCLIExecutable(t, filepath.Join(root, "bin", "start-task"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"worktree created\",\"artifacts\":{\"branch\":{\"name\":\"task/user-permissions/api-auth\"},\"worktree\":{\"path\":\"/tmp/demo\",\"status\":\"present\"}}}'\n")

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "project", "add", "user-permissions"}); err != nil {
		t.Fatalf("project add: %v", err)
	}

	addOut := captureCLIOutput(t, []string{"taskman", "--root", root, "task", "add", "api-auth", "-p", "user-permissions"})
	for _, want := range []string{"Task: user-permissions/api-auth", "Status: todo"} {
		if !strings.Contains(addOut, want) {
			t.Fatalf("task add summary missing %q: %s", want, addOut)
		}
	}

	startOut := captureCLIOutput(t, []string{"taskman", "--root", root, "task", "start", "api-auth", "-p", "user-permissions"})
	for _, want := range []string{"Task: user-permissions/api-auth", "Transition: start", "Status: active", "Step create_worktree: worktree created", "branch=task/user-permissions/api-auth", "path=/tmp/demo"} {
		if !strings.Contains(startOut, want) {
			t.Fatalf("task start summary missing %q: %s", want, startOut)
		}
	}
}

func TestProjectListUnexpectedArgReturnsShowHint(t *testing.T) {
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
	err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "project", "list", "docs-refresh"})
	if err == nil {
		t.Fatal("expected project list arg validation error")
	}
	if !strings.Contains(err.Error(), "project list does not accept a project id; use `taskman project show <project>`") {
		t.Fatalf("unexpected project list error: %v", err)
	}
}

func TestTaskCloseShowsWarningsFromBestEffortCleanup(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: []
    vars:
      worktree: required
vars:
  task:
    worktree:
      allowed: [required, optional]
workflow:
  task:
    statuses: [todo, done, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      close:
        from: [done]
        to: closed
        steps:
          - name: remove_worktree
            cmd: [./bin/close-task]
`)
	writeCLIExecutable(t, filepath.Join(root, "bin", "close-task"), "#!/bin/sh\necho '{\"ok\":true,\"message\":\"worktree cleanup skipped\",\"warnings\":[\"dirty worktree detected at /tmp/demo\"],\"artifacts\":{\"worktree\":{\"path\":\"/tmp/demo\",\"status\":\"dirty\"}}}'\n")
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("done"), Vars: map[string]string{"worktree": "required"}, Session: model.TaskSessionState{Active: "S-001"}}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}
	if err := s.SaveSession("user-permissions", "api-auth", model.SessionState{Version: 1, ID: "S-001", Task: "api-auth", Status: "done"}); err != nil {
		t.Fatalf("save session: %v", err)
	}

	out := captureCLIOutput(t, []string{"taskman", "--root", root, "task", "close", "api-auth", "-p", "user-permissions"})
	for _, want := range []string{"Task: user-permissions/api-auth", "Transition: close", "Status: closed", "Warning: dirty worktree detected at /tmp/demo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("task close output missing %q: %s", want, out)
		}
	}
}

func TestDoctorPrintsResolvedRoot(t *testing.T) {
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
	t.Setenv("TASKMAN_ROOT", root)

	out := captureCLIOutput(t, []string{"taskman", "doctor"})
	for _, want := range []string{"ok", "Root: " + root} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q: %s", want, out)
		}
	}
}

func TestProjectBriefInitPrintsSummary(t *testing.T) {
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
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "project", "add", "user-permissions"}); err != nil {
		t.Fatalf("project add: %v", err)
	}

	out := captureCLIOutput(t, []string{"taskman", "--root", root, "project", "brief", "init", "user-permissions", "--force"})
	for _, want := range []string{"Project Brief: user-permissions", "Source: template"} {
		if !strings.Contains(out, want) {
			t.Fatalf("project brief init output missing %q: %s", want, out)
		}
	}
}

func TestTaskBriefSetPrintsSummaryForStdinSource(t *testing.T) {
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
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	brief := "# Intent\n\nStream from stdin.\n\n# Scope In\n\nCLI UX.\n\n# Scope Out\n\nEditor.\n\n# Acceptance\n\nBrief saves.\n\n# Current Context\n\nTask exists.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nList tasks.\n\n# References\n\n- stdin://test\n"
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	if _, err := io.WriteString(w, brief); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	originalStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = originalStdin }()

	out := captureCLIOutput(t, []string{"taskman", "--root", root, "task", "brief", "set", "api-auth", "-p", "user-permissions", "--file", "-"})
	for _, want := range []string{"Task Brief: user-permissions/api-auth", "Source: stdin"} {
		if !strings.Contains(out, want) {
			t.Fatalf("task brief set output missing %q: %s", want, out)
		}
	}
}

func TestCanonicalTaskCommandsRequireProjectFlagInErrors(t *testing.T) {
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
	for _, args := range [][]string{
		{"taskman", "--root", root, "task", "show", "api-auth"},
		{"taskman", "--root", root, "task", "show", "user-permissions/api-auth"},
		{"taskman", "--root", root, "task", "update", "api-auth"},
		{"taskman", "--root", root, "task", "brief", "show", "api-auth"},
		{"taskman", "--root", root, "task", "event", "list", "api-auth"},
		{"taskman", "--root", root, "task", "start", "api-auth"},
	} {
		err := cmd.Run(context.Background(), args)
		if err == nil {
			t.Fatalf("expected project flag error for %v", args)
		}
		if !strings.Contains(err.Error(), "task must be addressed as <task> -p <project>") {
			t.Fatalf("unexpected error for %v: %v", args, err)
		}
	}
}

func TestLegacyCLIFormsAreRejected(t *testing.T) {
	for _, args := range [][]string{
		{"taskman", "projects", "list"},
		{"taskman", "tasks", "get"},
		{"taskman", "task", "transition", "demo", "start", "-p", "alpha"},
		{"taskman", "task", "add", "--name", "demo", "-p", "alpha"},
	} {
		cmd := BuildApp()
		var stdout bytes.Buffer
		cmd.Writer = &stdout
		cmd.ErrWriter = &stdout
		cmd.ExitErrHandler = func(context.Context, *urfavecli.Command, error) {}
		if err := cmd.Run(context.Background(), args); err == nil {
			t.Fatalf("expected legacy form to fail: %v", args)
		}
	}
}

func TestRemainingMutationsPrintSuccessSummaries(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars:
      area: identity
  task:
    labels: []
    vars:
      kind: feature
vars:
  project:
    area:
      allowed: [identity, product]
  task:
    kind:
      allowed: [feature, chore]
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
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "project", "add", "user-permissions"}); err != nil {
		t.Fatalf("project add: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "task", "add", "api-auth", "-p", "user-permissions"}); err != nil {
		t.Fatalf("task add: %v", err)
	}

	projectUpdateOut := captureCLIOutput(t, []string{"taskman", "--root", root, "project", "update", "user-permissions", "--var", "area=product"})
	if !strings.Contains(projectUpdateOut, "Project: user-permissions") || !strings.Contains(projectUpdateOut, "Status: active") {
		t.Fatalf("project update summary missing: %s", projectUpdateOut)
	}

	taskUpdateOut := captureCLIOutput(t, []string{"taskman", "--root", root, "task", "update", "api-auth", "-p", "user-permissions", "--var", "kind=chore"})
	if !strings.Contains(taskUpdateOut, "Task: user-permissions/api-auth") || !strings.Contains(taskUpdateOut, "Status: todo") {
		t.Fatalf("task update summary missing: %s", taskUpdateOut)
	}

	projectEventOut := captureCLIOutput(t, []string{"taskman", "--root", root, "project", "event", "add", "user-permissions", "--id", "PEVT-1", "--at", "2026-03-14T13:00:00Z", "--type", "note", "--summary", "created", "--actor", "taskman"})
	if !strings.Contains(projectEventOut, "Project Event: user-permissions") || !strings.Contains(projectEventOut, "Event ID: PEVT-1") {
		t.Fatalf("project event summary missing: %s", projectEventOut)
	}

	taskEventOut := captureCLIOutput(t, []string{"taskman", "--root", root, "task", "event", "add", "api-auth", "-p", "user-permissions", "--id", "TEVT-1", "--at", "2026-03-14T13:00:01Z", "--type", "note", "--summary", "created", "--actor", "taskman"})
	if !strings.Contains(taskEventOut, "Task Event: user-permissions/api-auth") || !strings.Contains(taskEventOut, "Event ID: TEVT-1") {
		t.Fatalf("task event summary missing: %s", taskEventOut)
	}
}

func captureCLIOutput(t *testing.T, args []string) string {
	t.Helper()
	cmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()
	if err := cmd.Run(context.Background(), args); err != nil {
		t.Fatalf("run %v: %v", args, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	return string(data)
}
