package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigUnmarshalKeepsConciseWorkflowShape(t *testing.T) {
	input := []byte(`version: 1
defaults:
  project:
    labels: [platform]
    vars:
      area: identity
  task:
    labels: [backend]
    vars:
      kind: feature
vars:
  project:
    area:
      allowed: [identity, platform]
  task:
    kind:
      allowed: [feature, chore]
naming:
  task_slug: "{{ .project.slug }}-{{ .name }}"
workflow:
  task:
    statuses: [todo, active, blocked, done, cancelled, closed]
    initial_status: todo
    terminal_statuses: [closed]
    transitions:
      start:
        from: [todo]
        to: active
        requires: [kind]
        steps:
          - name: initialize
            when:
              task.vars.kind: feature
            cmd:
              - ./tasks/_meta/bin/noop
              - --input
              - "{{ .input_json_path }}"
      close:
        from: [done, cancelled]
        to: closed
`)

	var cfg Config
	if err := yaml.Unmarshal(input, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if cfg.Version != 1 {
		t.Fatalf("version = %d, want 1", cfg.Version)
	}

	if got := cfg.Defaults.Task.Vars["kind"]; got != "feature" {
		t.Fatalf("default task kind var = %q, want feature", got)
	}

	if got := cfg.Naming.TaskSlug; got != "{{ .project.slug }}-{{ .name }}" {
		t.Fatalf("task slug template = %q", got)
	}

	if len(cfg.Workflow.Task.Statuses) != 6 {
		t.Fatalf("task workflow statuses = %d, want 6", len(cfg.Workflow.Task.Statuses))
	}

	if cfg.Workflow.Task.InitialStatus != "todo" {
		t.Fatalf("initial status = %q, want todo", cfg.Workflow.Task.InitialStatus)
	}

	if len(cfg.Workflow.Task.Transitions) != 2 {
		t.Fatalf("task workflow transitions = %d, want 2", len(cfg.Workflow.Task.Transitions))
	}

	steps := cfg.Workflow.Task.Transitions["start"].Steps
	if len(steps) != 1 {
		t.Fatalf("start steps = %d, want 1", len(steps))
	}

	step := steps[0]
	if step.Name != "initialize" {
		t.Fatalf("step name = %q", step.Name)
	}

	if got := step.When["task.vars.kind"]; got != "feature" {
		t.Fatalf("when selector = %q, want feature", got)
	}

	if len(step.Cmd) != 3 {
		t.Fatalf("cmd len = %d, want 3", len(step.Cmd))
	}
}

func TestConfigValidateRejectsTransitionToUndeclaredStatus(t *testing.T) {
	cfg := Config{
		Version: 1,
		Vars:    VarSchema{Task: map[string]VarRule{"kind": {Allowed: []string{"feature", "chore"}}}},
		Defaults: Defaults{
			Task: MetadataDefaults{Vars: map[string]string{"kind": "feature"}},
		},
		Workflow: WorkflowConfig{Task: TaskWorkflowConfig{
			Statuses:      []TaskStatus{"todo", "active"},
			InitialStatus: "todo",
			Transitions: map[string]TaskTransition{
				"block": {From: []TaskStatus{"todo"}, To: "blocked"},
			},
		}},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for transition status")
	}
}

func TestConfigValidateRejectsSemanticallyUnknownTemplateFields(t *testing.T) {
	cfg := Config{
		Version: 1,
		Vars: VarSchema{
			Project: map[string]VarRule{"area": {Allowed: []string{"identity", "platform"}}},
			Task:    map[string]VarRule{"kind": {Allowed: []string{"feature", "chore"}}},
		},
		Defaults: Defaults{
			Project: MetadataDefaults{Vars: map[string]string{"area": "identity"}},
			Task:    MetadataDefaults{Vars: map[string]string{"kind": "feature"}},
		},
		Naming: Naming{
			TaskSlug: "{{ .project.missing }}-{{ .name }}",
		},
		Workflow: WorkflowConfig{Task: TaskWorkflowConfig{
			Statuses:      []TaskStatus{"todo", "active"},
			InitialStatus: "todo",
			Transitions: map[string]TaskTransition{
				"start": {From: []TaskStatus{"todo"}, To: "active"},
			},
		}},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected semantic template validation error")
	}
}

func TestConfigValidateRejectsUnknownWhenSelector(t *testing.T) {
	cfg := Config{
		Version: 1,
		Workflow: WorkflowConfig{Task: TaskWorkflowConfig{
			Statuses:      []TaskStatus{"todo", "active"},
			InitialStatus: "todo",
			Transitions: map[string]TaskTransition{
				"start": {
					From: []TaskStatus{"todo"},
					To:   "active",
					Steps: []Step{{
						Name: "noop",
						When: map[string]string{"task.repo": "cloud"},
						Cmd:  []string{"./bin/noop"},
					}},
				},
			},
		}},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected invalid when selector error")
	}
}
