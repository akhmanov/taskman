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
    traits:
      preview: app-api
  task:
    labels: [backend]
    traits:
      mr: required
      worktree: required
traits:
  project:
    preview: [app-api, none]
  task:
    mr: [required, not-needed]
    worktree: [required, optional]
naming:
  task_slug: "{{ .repo }}-{{ .name }}"
  branch: "task/{{ .project.slug }}/{{ .task.slug }}"
steps:
  task_start:
    - name: create_worktree
      when:
        task.traits.worktree: required
      cmd:
        - ./tasks/_meta/bin/create-worktree
        - --input
        - "{{ .input_json_path }}"
`)

	var cfg Config
	if err := yaml.Unmarshal(input, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if cfg.Version != 1 {
		t.Fatalf("version = %d, want 1", cfg.Version)
	}

	if got := cfg.Defaults.Task.Traits["mr"]; got != "required" {
		t.Fatalf("default task mr trait = %q, want required", got)
	}

	if got := cfg.Naming.TaskSlug; got != "{{ .repo }}-{{ .name }}" {
		t.Fatalf("task slug template = %q", got)
	}

	steps := cfg.Steps[TaskStartPhase]
	if len(steps) != 1 {
		t.Fatalf("task_start steps = %d, want 1", len(steps))
	}

	step := steps[0]
	if step.Name != "create_worktree" {
		t.Fatalf("step name = %q", step.Name)
	}

	if got := step.When["task.traits.worktree"]; got != "required" {
		t.Fatalf("when selector = %q, want required", got)
	}

	if len(step.Cmd) != 3 {
		t.Fatalf("cmd len = %d, want 3", len(step.Cmd))
	}
}

func TestConfigValidateRejectsUnknownDefaultTraitValues(t *testing.T) {
	cfg := Config{
		Version: 1,
		Traits: TraitSchema{
			Task: map[string][]string{
				"mr": {"required", "not-needed"},
			},
		},
		Defaults: Defaults{
			Task: MetadataDefaults{Traits: map[string]string{"mr": "sometimes"}},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}
}
