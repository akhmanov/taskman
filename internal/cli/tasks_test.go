package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akhmanov/taskman/internal/model"
	"github.com/akhmanov/taskman/internal/store"
)

func TestTasksCreateAndTransitionFlow(t *testing.T) {
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
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "user-permissions", "--name", "api-auth"}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	s := store.New(root)
	task, err := s.LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task after create: %v", err)
	}
	if task.Status != model.TaskStatus("todo") {
		t.Fatalf("status after create = %q, want todo", task.Status)
	}

	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "transition", "user-permissions/api-auth", "start"}); err != nil {
		t.Fatalf("transition task: %v", err)
	}

	task, err = s.LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task after transition: %v", err)
	}
	if task.Status != model.TaskStatus("active") {
		t.Fatalf("status after transition = %q, want active", task.Status)
	}
	if task.LastOp.Cmd != "tasks.transition" || !task.LastOp.OK {
		t.Fatalf("last op = %+v", task.LastOp)
	}

	artifact, err := s.LoadArtifact("user-permissions", "api-auth", "summary")
	if err != nil {
		t.Fatalf("load artifact: %v", err)
	}
	if artifact.Data["state"] != "started" {
		t.Fatalf("summary artifact state = %q", artifact.Data["state"])
	}
}

func TestTasksCreatePersistsLabelsAndVars(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars: {}
  task:
    labels: [backend]
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
`)

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions"}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "user-permissions", "--name", "api-auth", "--label", "auth", "--var", "kind=chore"}); err != nil {
		t.Fatalf("create task: %v", err)
	}

	task, err := store.New(root).LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}

	if len(task.Labels) != 2 || task.Labels[0] != "backend" || task.Labels[1] != "auth" {
		t.Fatalf("labels = %#v", task.Labels)
	}
	if task.Vars["kind"] != "chore" {
		t.Fatalf("kind var = %q", task.Vars["kind"])
	}
}

func TestTasksGetWithoutProjectListsAcrossProjects(t *testing.T) {
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
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "alpha"}); err != nil {
		t.Fatalf("create alpha project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "beta"}); err != nil {
		t.Fatalf("create beta project: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "alpha", "--name", "api-auth"}); err != nil {
		t.Fatalf("create alpha task: %v", err)
	}
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "create", "--project", "beta", "--name", "engine"}); err != nil {
		t.Fatalf("create beta task: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "get", "--status", "todo"}); err != nil {
		t.Fatalf("tasks get global: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "alpha/api-auth") || !strings.Contains(out, "beta/engine") {
		t.Fatalf("global tasks get missing expected tasks: %s", out)
	}
}

func TestTasksDescribeShowsGenericArtifactData(t *testing.T) {
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
	if err := s.ScaffoldTask(model.TaskState{
		Version: 1,
		Slug:    "api-auth",
		Project: "user-permissions",
		Status:  model.TaskStatus("active"),
		Vars:    map[string]string{"kind": "feature"},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}
	if err := s.SaveArtifact("user-permissions", "api-auth", "summary", map[string]any{
		"state": "ready",
		"url":   "https://example.test/tasks/api-auth",
	}); err != nil {
		t.Fatalf("save summary artifact: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "describe", "user-permissions/api-auth"}); err != nil {
		t.Fatalf("tasks describe: %v", err)
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
		"user-permissions/api-auth",
		"Vars: kind=feature",
		"Artifact summary: state=ready, url=https://example.test/tasks/api-auth",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("describe output missing %q: %s", want, out)
		}
	}
}

func TestTasksDescribeRawViewExposesPersistedMemorySurfaces(t *testing.T) {
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
	if err := s.SaveTaskBrief("user-permissions", "api-auth", "# Intent\n\nImplement raw view.\n\n# Scope In\n\nRaw memory rendering.\n\n# Scope Out\n\nAgent digest.\n\n# Acceptance\n\nRaw surfaces are visible.\n\n# Current Context\n\nPersisted memory exists.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nRender raw payload.\n\n# References\n\n- test://raw-view\n"); err != nil {
		t.Fatalf("save task brief: %v", err)
	}
	if err := s.AppendTaskEvent("user-permissions", "api-auth", model.PayloadEvent{ID: "EVT-101", At: "2026-03-14T11:00:00Z", Type: model.PayloadEventTypeNote, Summary: "Started implementation", Actor: "taskman"}); err != nil {
		t.Fatalf("append task event: %v", err)
	}
	if err := s.SaveArtifact("user-permissions", "api-auth", "summary", map[string]any{"state": "ready"}); err != nil {
		t.Fatalf("save artifact: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "describe", "user-permissions/api-auth", "--view", "raw", "--output", "json"}); err != nil {
		t.Fatalf("tasks describe raw: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal json output: %v\n%s", err, string(data))
	}
	if payload["view"] != "raw" {
		t.Fatalf("view = %#v", payload["view"])
	}
	if brief, ok := payload["brief"].(string); !ok || !strings.Contains(brief, "# Intent") {
		t.Fatalf("brief = %#v", payload["brief"])
	}
	events, ok := payload["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("events = %#v", payload["events"])
	}
	if _, ok := payload["project_digest"]; ok {
		t.Fatalf("raw view should not embed project digest: %#v", payload)
	}
}

func TestTasksDescribeAgentViewComposesProjectDigestAndBoundedTaskContext(t *testing.T) {
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
      close:
        from: [active]
        to: closed
`)

	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.SaveProjectBrief("user-permissions", "# Mission\n\nBound task context for short-memory agents.\n\n# Boundaries\n\n## In Scope\n\nTask context.\n\n## Out of Scope\n\nLong-form research.\n\n# Glossary\n\n- Digest\n\n# Shared Decisions\n\n- Reference project context.\n\n# Active Risks\n\n- Agent drift.\n\n# Tasking Rules\n\n- Keep context compact.\n\n# References\n\n- plan://payload-layer\n"); err != nil {
		t.Fatalf("save project brief: %v", err)
	}
	for _, event := range []model.PayloadEvent{
		{ID: "PEVT-1", At: "2026-03-14T10:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "Use referenced project digest", Actor: "taskman", Status: "active"},
		{ID: "PEVT-2", At: "2026-03-14T10:10:00Z", Type: model.PayloadEventTypeBlocker, Summary: "Await product signoff", Actor: "taskman", Status: "active"},
	} {
		if err := s.AppendProjectEvent("user-permissions", event); err != nil {
			t.Fatalf("append project event: %v", err)
		}
	}

	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}
	if err := s.SaveTaskBrief("user-permissions", "api-auth", "# Intent\n\nImplement task describe views.\n\n# Scope In\n\nAgent describe payload.\n\n# Scope Out\n\nWorkflow changes.\n\n# Acceptance\n\nTask describe is bounded.\n\n# Current Context\n\nProject digest exists.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nRender agent view.\n\n# References\n\n- plan://payload-layer\n"); err != nil {
		t.Fatalf("save task brief: %v", err)
	}
	for _, event := range []model.PayloadEvent{
		{ID: "TEVT-1", At: "2026-03-14T11:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "Render bounded event windows", Actor: "taskman", Status: "active"},
		{ID: "TEVT-2", At: "2026-03-14T11:10:00Z", Type: model.PayloadEventTypeBlocker, Summary: "Need agent output schema", Actor: "taskman", Status: "active"},
		{ID: "TEVT-3", At: "2026-03-14T11:20:00Z", Type: model.PayloadEventTypeNote, Summary: "Drafted tests", Actor: "taskman"},
		{ID: "TEVT-4", At: "2026-03-14T11:30:00Z", Type: model.PayloadEventTypeTransition, Summary: "Prepared implementation", Actor: "taskman"},
	} {
		if err := s.AppendTaskEvent("user-permissions", "api-auth", event); err != nil {
			t.Fatalf("append task event: %v", err)
		}
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "describe", "user-permissions/api-auth", "--view", "agent", "--output", "json"}); err != nil {
		t.Fatalf("tasks describe agent: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal json output: %v\n%s", err, string(data))
	}
	if payload["view"] != "agent" {
		t.Fatalf("view = %#v", payload["view"])
	}
	if brief, ok := payload["task_brief"].(string); !ok || !strings.Contains(brief, "# Intent") {
		t.Fatalf("task brief = %#v", payload["task_brief"])
	}
	digest, ok := payload["project_digest"].(map[string]any)
	if !ok {
		t.Fatalf("project digest missing: %#v", payload)
	}
	if brief, ok := digest["brief"].(string); !ok || !strings.Contains(brief, "# Mission") {
		t.Fatalf("project digest brief = %#v", digest["brief"])
	}
	if _, ok := payload["task"]; ok {
		t.Fatalf("agent view should not expose raw task state: %#v", payload)
	}
	allowed, ok := payload["allowed_transitions"].([]any)
	if !ok || len(allowed) != 1 || allowed[0] != "start" {
		t.Fatalf("allowed transitions = %#v", payload["allowed_transitions"])
	}
	decisions, ok := payload["active_decisions"].([]any)
	if !ok || len(decisions) != 1 {
		t.Fatalf("active decisions = %#v", payload["active_decisions"])
	}
	blockers, ok := payload["open_blockers"].([]any)
	if !ok || len(blockers) != 1 {
		t.Fatalf("open blockers = %#v", payload["open_blockers"])
	}
	recent, ok := payload["recent_events"].([]any)
	if !ok {
		t.Fatalf("recent events = %#v", payload["recent_events"])
	}
	if len(recent) == 0 || len(recent) > 3 {
		t.Fatalf("recent events len = %d", len(recent))
	}
	if payload["next_action"] == "" {
		t.Fatalf("next action should not be empty: %#v", payload)
	}
}

func TestTasksDescribeAgentTextShowsDigestAndRecentContext(t *testing.T) {
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
      close:
        from: [active]
        to: closed
`)
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.SaveProjectBrief("user-permissions", "# Mission\n\nBound task context.\n\n# Boundaries\n\n## In Scope\n\nTask digest.\n\n## Out of Scope\n\nLong docs.\n\n# Glossary\n\n- Digest\n\n# Shared Decisions\n\n- Reference project context.\n\n# Active Risks\n\n- Drift.\n\n# Tasking Rules\n\n- Keep agent views short.\n\n# References\n\n- doc://memory\n"); err != nil {
		t.Fatalf("save project brief: %v", err)
	}
	if err := s.AppendProjectEvent("user-permissions", model.PayloadEvent{ID: "PEVT-1", At: "2026-03-14T10:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "Use referenced project digest", Actor: "taskman", Rationale: "Avoid duplication", Impact: "Tasks stay local", Status: "active"}); err != nil {
		t.Fatalf("append project event: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}
	if err := s.SaveTaskBrief("user-permissions", "api-auth", "# Intent\n\nImplement agent text rendering.\n\n# Scope In\n\nText UX.\n\n# Scope Out\n\nStorage.\n\n# Acceptance\n\nAgent text shows useful lines.\n\n# Current Context\n\nDescribe exists.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nCommit changes.\n\n# References\n\n- plan://payload-layer\n"); err != nil {
		t.Fatalf("save task brief: %v", err)
	}
	for _, event := range []model.PayloadEvent{{ID: "TEVT-1", At: "2026-03-14T11:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "Render bounded event windows", Actor: "taskman", Rationale: "Agents need short views", Impact: "Text mode more useful", Status: "active"}, {ID: "TEVT-2", At: "2026-03-14T11:10:00Z", Type: model.PayloadEventTypeBlocker, Summary: "Need renderer wording", Actor: "taskman", Status: "active"}} {
		if err := s.AppendTaskEvent("user-permissions", "api-auth", event); err != nil {
			t.Fatalf("append task event: %v", err)
		}
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()
	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "describe", "user-permissions/api-auth", "--view", "agent"}); err != nil {
		t.Fatalf("tasks describe agent text: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	out := string(data)
	for _, want := range []string{"user-permissions/api-auth", "Allowed Transitions: start", "Decision: Render bounded event windows", "Blocker: Need renderer wording"} {
		if !strings.Contains(out, want) {
			t.Fatalf("agent text missing %q: %s", want, out)
		}
	}
}

func TestTasksGetSupportsJSONOutput(t *testing.T) {
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
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "alpha", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "alpha", Status: model.TaskStatus("todo")}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "get", "--output", "json"}); err != nil {
		t.Fatalf("tasks get json: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal json output: %v\n%s", err, string(data))
	}
	if len(payload) != 1 {
		t.Fatalf("payload len = %d", len(payload))
	}
	if payload[0]["slug"] != "api-auth" {
		t.Fatalf("slug = %#v", payload[0]["slug"])
	}
}

func writeCLIConfig(t *testing.T, root string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
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

func TestTasksUpdatePersistsMetadata(t *testing.T) {
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
`)
	s := store.New(root)
	if err := s.ScaffoldProject(model.ProjectState{Version: 1, Slug: "user-permissions", Status: model.ProjectStatusActive}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}
	if err := s.ScaffoldTask(model.TaskState{Version: 1, Slug: "api-auth", Project: "user-permissions", Status: model.TaskStatus("todo"), Vars: map[string]string{"kind": "feature"}}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "update", "user-permissions/api-auth", "--label", "auth", "--var", "kind=chore"}); err != nil {
		t.Fatalf("tasks update: %v", err)
	}

	task, err := s.LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if len(task.Labels) != 1 || task.Labels[0] != "auth" {
		t.Fatalf("labels = %#v", task.Labels)
	}
	if task.Vars["kind"] != "chore" {
		t.Fatalf("kind = %q", task.Vars["kind"])
	}
}

func TestTasksBriefSetAndGet(t *testing.T) {
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

	want := "# Intent\n\nCapture decisions for api-auth.\n\n# Scope In\n\nDecision memory.\n\n# Scope Out\n\nLarge docs.\n\n# Acceptance\n\nBrief stored.\n\n# Current Context\n\nTask API exists.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nAdd events.\n\n# References\n\n- plan://payload-layer\n"
	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "brief", "set", "user-permissions/api-auth", "--content", want}); err != nil {
		t.Fatalf("tasks brief set: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "brief", "get", "user-permissions/api-auth"}); err != nil {
		t.Fatalf("tasks brief get: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != want {
		t.Fatalf("brief output = %q", string(data))
	}
}

func TestTasksBriefSetFromFile(t *testing.T) {
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

	briefPath := filepath.Join(root, "task-brief.md")
	want := "# Intent\n\nUse file-based updates.\n\n# Scope In\n\nCLI UX.\n\n# Scope Out\n\nEditor integration.\n\n# Acceptance\n\nFile content persists.\n\n# Current Context\n\nTask exists.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nRender richer views.\n\n# References\n\n- plan://payload-layer\n"
	if err := os.WriteFile(briefPath, []byte(want), 0o644); err != nil {
		t.Fatalf("write task brief file: %v", err)
	}

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "brief", "set", "user-permissions/api-auth", "--file", briefPath}); err != nil {
		t.Fatalf("tasks brief set --file: %v", err)
	}

	got, err := s.LoadTaskBrief("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task brief: %v", err)
	}
	if got != want {
		t.Fatalf("task brief = %q", got)
	}
}

func TestTasksBriefInitResetsTemplateWithForce(t *testing.T) {
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
	if err := s.SaveTaskBrief("user-permissions", "api-auth", "# Intent\n\nCustom.\n\n# Scope In\n\nA\n\n# Scope Out\n\nB\n\n# Acceptance\n\nC\n\n# Current Context\n\nD\n\n# Open Questions\n\nE\n\n# Next Action\n\nF\n\n# References\n\nG\n"); err != nil {
		t.Fatalf("seed task brief: %v", err)
	}

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "brief", "init", "user-permissions/api-auth", "--force"}); err != nil {
		t.Fatalf("tasks brief init: %v", err)
	}

	got, err := s.LoadTaskBrief("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task brief: %v", err)
	}
	if !strings.Contains(got, "# Intent") || !strings.Contains(got, "# Next Action") {
		t.Fatalf("task brief template not restored: %q", got)
	}
}

func TestTasksBriefEditUsesEditor(t *testing.T) {
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
	editorPath := filepath.Join(root, "editor.sh")
	edited := "# Intent\n\nEdited through editor.\n\n# Scope In\n\nCLI.\n\n# Scope Out\n\nAutomation.\n\n# Acceptance\n\nSaved.\n\n# Current Context\n\nTask exists.\n\n# Open Questions\n\nNone.\n\n# Next Action\n\nCommit changes.\n\n# References\n\n- doc://editor\n"
	script := "#!/bin/sh\ncat <<'EOF' > \"$1\"\n" + edited + "EOF\n"
	if err := os.WriteFile(editorPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", editorPath)

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "brief", "edit", "user-permissions/api-auth"}); err != nil {
		t.Fatalf("tasks brief edit: %v", err)
	}

	got, err := s.LoadTaskBrief("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task brief: %v", err)
	}
	if got != edited {
		t.Fatalf("edited task brief = %q", got)
	}
}

func TestTasksEventAddAndGet(t *testing.T) {
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
	if err := cmd.Run(context.Background(), []string{
		"taskman", "--root", root,
		"tasks", "event", "add", "user-permissions/api-auth",
		"--id", "EVT-002",
		"--at", "2026-03-14T12:10:00Z",
		"--type", "transition",
		"--summary", "Moved to active",
		"--actor", "taskman",
		"--rationale", "Dependencies merged",
		"--impact", "Ready for review",
	}); err != nil {
		t.Fatalf("tasks event add: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "event", "get", "user-permissions/api-auth", "--output", "json"}); err != nil {
		t.Fatalf("tasks event get: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var events []map[string]any
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("unmarshal json output: %v\n%s", err, string(data))
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d", len(events))
	}
	if events[0]["type"] != "transition" {
		t.Fatalf("event type = %#v", events[0]["type"])
	}
	if events[0]["impact"] != "Ready for review" {
		t.Fatalf("event impact = %#v", events[0]["impact"])
	}
}

func TestTasksUpdateSupportsUnsetVar(t *testing.T) {
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
      owner: platform
vars:
  task:
    kind:
      required: true
      allowed: [feature, chore]
    owner:
      allowed: [platform, core]
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
	if err := s.ScaffoldTask(model.TaskState{
		Version: 1,
		Slug:    "api-auth",
		Project: "user-permissions",
		Status:  model.TaskStatus("todo"),
		Vars:    map[string]string{"kind": "feature", "owner": "platform"},
	}); err != nil {
		t.Fatalf("scaffold task: %v", err)
	}

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "update", "user-permissions/api-auth", "--unset-var", "owner"}); err != nil {
		t.Fatalf("tasks update unset-var: %v", err)
	}

	task, err := s.LoadTask("user-permissions", "api-auth")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if _, ok := task.Vars["owner"]; ok {
		t.Fatalf("owner should be unset: %#v", task.Vars)
	}
}

func TestTasksEventsGetSupportsTypeAndActiveOnlyFilters(t *testing.T) {
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
	for _, event := range []model.PayloadEvent{
		{ID: "EVT-1", At: "2026-03-14T10:00:00Z", Type: model.PayloadEventTypeBlocker, Summary: "blocked", Actor: "taskman", Status: "active"},
		{ID: "EVT-2", At: "2026-03-14T11:00:00Z", Type: model.PayloadEventTypeBlocker, Summary: "resolved", Actor: "taskman", Status: "resolved"},
		{ID: "EVT-3", At: "2026-03-14T12:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "decision", Actor: "taskman", Status: "active"},
	} {
		if err := s.AppendTaskEvent("user-permissions", "api-auth", event); err != nil {
			t.Fatalf("append task event: %v", err)
		}
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "tasks", "events", "get", "user-permissions/api-auth", "--type", "blocker", "--active-only", "--output", "json"}); err != nil {
		t.Fatalf("tasks events get with filters: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var events []map[string]any
	if err := json.Unmarshal(data, &events); err != nil {
		t.Fatalf("unmarshal json output: %v\n%s", err, string(data))
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d", len(events))
	}
	if events[0]["id"] != "EVT-1" {
		t.Fatalf("event id = %#v", events[0]["id"])
	}
}
