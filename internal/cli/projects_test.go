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
	"gopkg.in/yaml.v3"
)

func TestProjectsCreateScaffoldsProject(t *testing.T) {
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

func TestProjectsCreatePersistsLabelsAndVars(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: [platform]
    vars:
      area: identity
  task:
    labels: []
    vars: {}
vars:
  project:
    area:
      allowed: [identity, product]
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
	err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions", "--label", "auth", "--var", "area=product"})
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
	if project.Vars["area"] != "product" {
		t.Fatalf("area var = %q", project.Vars["area"])
	}
}

func TestProjectsCreateRejectsInvalidVarOverride(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars:
      area: identity
  task:
    labels: []
    vars: {}
vars:
  project:
    area:
      allowed: [identity, product]
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
	err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions", "--var", "area=unknown"})
	if err == nil {
		t.Fatal("expected invalid var error")
	}
}

func TestProjectsDescribeSupportsYAMLOutput(t *testing.T) {
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
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "create", "user-permissions"}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "describe", "user-permissions", "--view", "raw", "--output", "yaml"}); err != nil {
		t.Fatalf("projects describe yaml: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var payload map[string]any
	if err := yaml.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal yaml output: %v\n%s", err, string(data))
	}
	if payload["view"] != "raw" {
		t.Fatalf("view = %#v", payload["view"])
	}
	project, ok := payload["project"].(map[string]any)
	if !ok {
		t.Fatalf("project payload missing: %#v", payload)
	}
	if project["slug"] != "user-permissions" {
		t.Fatalf("slug = %#v", project["slug"])
	}
	if project["status"] != "active" {
		t.Fatalf("status = %#v", project["status"])
	}
}

func TestProjectsGetShowsTaskCountSummaryInTextMode(t *testing.T) {
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
    statuses: [todo, active, in_review, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
`)
	s := store.New(root)
	if err := s.ScaffoldProject(projectStateForText()); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "get"}); err != nil {
		t.Fatalf("projects get: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	out := string(data)
	for _, want := range []string{"user-permissions", "active", "active=2", "closed=3", "todo=1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("projects get output missing %q: %s", want, out)
		}
	}
}

func TestProjectsDescribeShowsRicherTextDetails(t *testing.T) {
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
    statuses: [todo, active, in_review, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
`)
	s := store.New(root)
	if err := s.ScaffoldProject(projectStateForText()); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "describe", "user-permissions"}); err != nil {
		t.Fatalf("projects describe: %v", err)
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
		"Project: user-permissions",
		"Status: active",
		"Labels: auth, platform",
		"Vars: area=product",
		"Tasks: active=2, closed=3, todo=1",
		"Archive Ready: false",
		"Archive Blockers: task api-auth is not terminal",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("projects describe output missing %q: %s", want, out)
		}
	}
}

func TestProjectsDescribeRawViewExposesPersistedMemorySurfaces(t *testing.T) {
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
	if err := s.SaveProjectBrief("user-permissions", "# Mission\n\nKeep auth decisions bounded.\n\n# Boundaries\n\n## In Scope\n\nAuth memory.\n\n## Out of Scope\n\nBilling.\n\n# Glossary\n\n- Decision\n\n# Shared Decisions\n\n- Keep bounded.\n\n# Active Risks\n\n- Drift.\n\n# Tasking Rules\n\n- No copied context.\n\n# References\n\n- doc://auth\n"); err != nil {
		t.Fatalf("save project brief: %v", err)
	}
	if err := s.AppendProjectEvent("user-permissions", model.PayloadEvent{ID: "EVT-001", At: "2026-03-14T10:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "Adopt bounded memory", Actor: "taskman", Status: "active"}); err != nil {
		t.Fatalf("append project event: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "describe", "user-permissions", "--view", "raw", "--output", "json"}); err != nil {
		t.Fatalf("projects describe raw: %v", err)
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
	project, ok := payload["project"].(map[string]any)
	if !ok {
		t.Fatalf("project payload missing: %#v", payload)
	}
	if project["slug"] != "user-permissions" {
		t.Fatalf("slug = %#v", project["slug"])
	}
	if brief, ok := payload["brief"].(string); !ok || !strings.Contains(brief, "# Mission") {
		t.Fatalf("brief = %#v", payload["brief"])
	}
	events, ok := payload["events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("events = %#v", payload["events"])
	}
}

func TestProjectsDescribeAgentViewBuildsBoundedDigest(t *testing.T) {
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
	if err := s.SaveProjectBrief("user-permissions", "# Mission\n\nShip payload memory views.\n\n# Boundaries\n\n## In Scope\n\nPayload layer.\n\n## Out of Scope\n\nGeneral notes.\n\n# Glossary\n\n- Digest\n\n# Shared Decisions\n\n- Reference project context.\n\n# Active Risks\n\n- Event drift.\n\n# Tasking Rules\n\n- Keep agent views short.\n\n# References\n\n- plan://payload-layer\n"); err != nil {
		t.Fatalf("save project brief: %v", err)
	}
	for _, event := range []model.PayloadEvent{
		{ID: "EVT-001", At: "2026-03-14T10:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "Use typed events", Actor: "taskman", Status: "active"},
		{ID: "EVT-002", At: "2026-03-14T10:10:00Z", Type: model.PayloadEventTypeBlocker, Summary: "Await schema migration", Actor: "taskman", Status: "active"},
		{ID: "EVT-003", At: "2026-03-14T10:20:00Z", Type: model.PayloadEventTypeNote, Summary: "Captured current context", Actor: "taskman"},
		{ID: "EVT-004", At: "2026-03-14T10:30:00Z", Type: model.PayloadEventTypeDecision, Summary: "Legacy payload removed", Actor: "taskman", Status: "superseded"},
	} {
		if err := s.AppendProjectEvent("user-permissions", event); err != nil {
			t.Fatalf("append project event: %v", err)
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

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "describe", "user-permissions", "--view", "agent", "--output", "json"}); err != nil {
		t.Fatalf("projects describe agent: %v", err)
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
	digest, ok := payload["project_digest"].(map[string]any)
	if !ok {
		t.Fatalf("project digest missing: %#v", payload)
	}
	if brief, ok := digest["brief"].(string); !ok || !strings.Contains(brief, "# Mission") {
		t.Fatalf("project digest brief = %#v", digest["brief"])
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
}

func TestProjectsDescribeAgentTextShowsSummaryLines(t *testing.T) {
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
	if err := s.SaveProjectBrief("user-permissions", "# Mission\n\nShip bounded project memory.\n\n# Boundaries\n\n## In Scope\n\nProject digest.\n\n## Out of Scope\n\nLong docs.\n\n# Glossary\n\n- Digest\n\n# Shared Decisions\n\n- Keep things compact.\n\n# Active Risks\n\n- Drift.\n\n# Tasking Rules\n\n- Prefer brief.\n\n# References\n\n- doc://memory\n"); err != nil {
		t.Fatalf("save project brief: %v", err)
	}
	for _, event := range []model.PayloadEvent{{ID: "EVT-1", At: "2026-03-14T10:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "Use typed events", Actor: "taskman", Rationale: "Need bounded memory", Impact: "Digest stays structured", Status: "active"}, {ID: "EVT-2", At: "2026-03-14T10:10:00Z", Type: model.PayloadEventTypeBlocker, Summary: "Await product signoff", Actor: "taskman", Status: "active"}} {
		if err := s.AppendProjectEvent("user-permissions", event); err != nil {
			t.Fatalf("append project event: %v", err)
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
	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "describe", "user-permissions", "--view", "agent"}); err != nil {
		t.Fatalf("projects describe agent text: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	out := string(data)
	for _, want := range []string{"Project: user-permissions", "Next Action:", "Decision: Use typed events", "Blocker: Await product signoff"} {
		if !strings.Contains(out, want) {
			t.Fatalf("agent text missing %q: %s", want, out)
		}
	}
}

func projectStateForText() model.ProjectState {
	return model.ProjectState{
		Version: 1,
		Slug:    "user-permissions",
		Status:  model.ProjectStatusActive,
		Labels:  []string{"auth", "platform"},
		Vars:    map[string]string{"area": "product"},
		Tasks: model.TaskCounts{
			"todo":   1,
			"active": 2,
			"closed": 3,
		},
		Archive: model.ArchiveState{
			Ready:    false,
			Blockers: []string{"task api-auth is not terminal"},
		},
	}
}

func TestProjectsUpdatePersistsMetadata(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars:
      area: identity
  task:
    labels: []
    vars: {}
vars:
  project:
    area:
      allowed: [identity, product]
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

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "update", "user-permissions", "--label", "auth", "--label", "api", "--var", "area=product"}); err != nil {
		t.Fatalf("projects update: %v", err)
	}

	project, err := s.LoadProject("user-permissions")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	if len(project.Labels) != 2 || project.Labels[0] != "auth" || project.Labels[1] != "api" {
		t.Fatalf("labels = %#v", project.Labels)
	}
	if project.Vars["area"] != "product" {
		t.Fatalf("area = %q", project.Vars["area"])
	}
}

func TestProjectsBriefSetAndGet(t *testing.T) {
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

	want := "# Mission\n\nUse bounded payload memory.\n\n# Boundaries\n\n## In Scope\n\nTask context.\n\n## Out of Scope\n\nLong-form docs.\n\n# Glossary\n\n- Brief\n\n# Shared Decisions\n\n- Keep current truth separate.\n\n# Active Risks\n\n- Comment sprawl.\n\n# Tasking Rules\n\n- Prefer describe --view agent.\n\n# References\n\n- plan://payload-layer\n"
	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "brief", "set", "user-permissions", "--content", want}); err != nil {
		t.Fatalf("projects brief set: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "brief", "get", "user-permissions"}); err != nil {
		t.Fatalf("projects brief get: %v", err)
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

func TestProjectsBriefSetFromFile(t *testing.T) {
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

	briefPath := filepath.Join(root, "project-brief.md")
	want := "# Mission\n\nUse file-based brief updates.\n\n# Boundaries\n\n## In Scope\n\nCLI UX.\n\n## Out of Scope\n\nEditor workflow.\n\n# Glossary\n\n- Brief\n\n# Shared Decisions\n\n- Accept file input.\n\n# Active Risks\n\n- None.\n\n# Tasking Rules\n\n- Keep templates fixed.\n\n# References\n\n- file://not-used\n"
	if err := os.WriteFile(briefPath, []byte(want), 0o644); err != nil {
		t.Fatalf("write brief file: %v", err)
	}

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "brief", "set", "user-permissions", "--file", briefPath}); err != nil {
		t.Fatalf("projects brief set --file: %v", err)
	}

	got, err := s.LoadProjectBrief("user-permissions")
	if err != nil {
		t.Fatalf("load project brief: %v", err)
	}
	if got != want {
		t.Fatalf("project brief = %q", got)
	}
}

func TestProjectsBriefInitResetsTemplateWithForce(t *testing.T) {
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
	if err := s.SaveProjectBrief("user-permissions", "# Mission\n\nCustomized.\n\n# Boundaries\n\n## In Scope\n\nX\n\n## Out of Scope\n\nY\n\n# Glossary\n\nZ\n\n# Shared Decisions\n\nA\n\n# Active Risks\n\nB\n\n# Tasking Rules\n\nC\n\n# References\n\nD\n"); err != nil {
		t.Fatalf("seed project brief: %v", err)
	}

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "brief", "init", "user-permissions", "--force"}); err != nil {
		t.Fatalf("projects brief init: %v", err)
	}

	got, err := s.LoadProjectBrief("user-permissions")
	if err != nil {
		t.Fatalf("load project brief: %v", err)
	}
	if !strings.Contains(got, "# Mission") || !strings.Contains(got, "# Tasking Rules") {
		t.Fatalf("project brief template not restored: %q", got)
	}
}

func TestProjectsBriefEditUsesEditor(t *testing.T) {
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
	editorPath := filepath.Join(root, "editor.sh")
	edited := "# Mission\n\nEdited through editor.\n\n# Boundaries\n\n## In Scope\n\nCLI.\n\n## Out of Scope\n\nAutomation.\n\n# Glossary\n\n- Brief\n\n# Shared Decisions\n\n- Use editor.\n\n# Active Risks\n\n- None.\n\n# Tasking Rules\n\n- Save file.\n\n# References\n\n- doc://editor\n"
	script := "#!/bin/sh\ncat <<'EOF' > \"$1\"\n" + edited + "EOF\n"
	if err := os.WriteFile(editorPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}
	t.Setenv("EDITOR", editorPath)

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "brief", "edit", "user-permissions"}); err != nil {
		t.Fatalf("projects brief edit: %v", err)
	}

	got, err := s.LoadProjectBrief("user-permissions")
	if err != nil {
		t.Fatalf("load project brief: %v", err)
	}
	if got != edited {
		t.Fatalf("edited brief = %q", got)
	}
}

func TestProjectsEventAddAndGet(t *testing.T) {
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

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{
		"taskman", "--root", root,
		"projects", "event", "add", "user-permissions",
		"--id", "EVT-001",
		"--at", "2026-03-14T10:35:00Z",
		"--type", "decision",
		"--summary", "Adopt bounded payload memory",
		"--actor", "taskman",
		"--session", "S-001",
		"--ref", "doc://plan",
		"--rationale", "Decision history needs bounded typed events",
		"--impact", "Project digest can surface active decisions",
		"--status", "active",
	}); err != nil {
		t.Fatalf("projects event add: %v", err)
	}

	readCmd := BuildApp()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = originalStdout }()

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "event", "get", "user-permissions", "--output", "json"}); err != nil {
		t.Fatalf("projects event get: %v", err)
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
	if events[0]["type"] != "decision" {
		t.Fatalf("event type = %#v", events[0]["type"])
	}
	if events[0]["session"] != "S-001" {
		t.Fatalf("event session = %#v", events[0]["session"])
	}
}

func TestProjectsUpdateSupportsUnsetVar(t *testing.T) {
	root := t.TempDir()
	writeCLIConfig(t, root, `version: 1
defaults:
  project:
    labels: []
    vars:
      area: identity
      owner: platform
  task:
    labels: []
    vars: {}
vars:
  project:
    area:
      required: true
      allowed: [identity, product]
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
	if err := s.ScaffoldProject(model.ProjectState{
		Version: 1,
		Slug:    "user-permissions",
		Status:  model.ProjectStatusActive,
		Vars:    map[string]string{"area": "identity", "owner": "platform"},
	}); err != nil {
		t.Fatalf("scaffold project: %v", err)
	}

	cmd := BuildApp()
	if err := cmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "update", "user-permissions", "--unset-var", "owner"}); err != nil {
		t.Fatalf("projects update unset-var: %v", err)
	}

	project, err := s.LoadProject("user-permissions")
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	if _, ok := project.Vars["owner"]; ok {
		t.Fatalf("owner should be unset: %#v", project.Vars)
	}
}

func TestProjectsEventsGetSupportsTypeAndActiveOnlyFilters(t *testing.T) {
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
	for _, event := range []model.PayloadEvent{
		{ID: "EVT-1", At: "2026-03-14T10:00:00Z", Type: model.PayloadEventTypeBlocker, Summary: "blocked", Actor: "taskman", Status: "active"},
		{ID: "EVT-2", At: "2026-03-14T11:00:00Z", Type: model.PayloadEventTypeBlocker, Summary: "resolved", Actor: "taskman", Status: "resolved"},
		{ID: "EVT-3", At: "2026-03-14T12:00:00Z", Type: model.PayloadEventTypeDecision, Summary: "decision", Actor: "taskman", Status: "active"},
	} {
		if err := s.AppendProjectEvent("user-permissions", event); err != nil {
			t.Fatalf("append project event: %v", err)
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

	if err := readCmd.Run(context.Background(), []string{"taskman", "--root", root, "projects", "events", "get", "user-permissions", "--type", "blocker", "--active-only", "--output", "json"}); err != nil {
		t.Fatalf("projects events get with filters: %v", err)
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
