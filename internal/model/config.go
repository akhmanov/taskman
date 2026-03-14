package model

import (
	"fmt"
	"text/template"
)

type Phase string

const (
	TaskStartPhase      Phase = "task_start"
	TaskDonePhase       Phase = "task_done"
	TaskCleanupPhase    Phase = "task_cleanup"
	ProjectArchivePhase Phase = "project_archive"
)

type Config struct {
	Version  int               `yaml:"version"`
	Repos    map[string]string `yaml:"repos,omitempty"`
	Defaults Defaults          `yaml:"defaults"`
	Traits   TraitSchema       `yaml:"traits"`
	Naming   Naming            `yaml:"naming"`
	Steps    map[Phase][]Step  `yaml:"steps"`
}

type Defaults struct {
	Project MetadataDefaults `yaml:"project"`
	Task    MetadataDefaults `yaml:"task"`
}

type MetadataDefaults struct {
	Labels []string          `yaml:"labels"`
	Traits map[string]string `yaml:"traits"`
}

type TraitSchema struct {
	Project map[string][]string `yaml:"project"`
	Task    map[string][]string `yaml:"task"`
}

type Naming struct {
	TaskSlug string `yaml:"task_slug"`
	Branch   string `yaml:"branch"`
	Worktree string `yaml:"worktree"`
}

type Step struct {
	Name string            `yaml:"name"`
	When map[string]string `yaml:"when,omitempty"`
	Cmd  []string          `yaml:"cmd"`
}

func (c Config) Validate() error {
	if c.Version <= 0 {
		return fmt.Errorf("version must be positive")
	}

	if err := validateDefaultTraits("project", c.Defaults.Project.Traits, c.Traits.Project); err != nil {
		return err
	}

	if err := validateDefaultTraits("task", c.Defaults.Task.Traits, c.Traits.Task); err != nil {
		return err
	}

	for _, candidate := range []struct {
		name    string
		pattern string
	}{
		{name: "naming.task_slug", pattern: c.Naming.TaskSlug},
		{name: "naming.branch", pattern: c.Naming.Branch},
		{name: "naming.worktree", pattern: c.Naming.Worktree},
	} {
		if candidate.pattern == "" {
			continue
		}
		if _, err := template.New(candidate.name).Parse(candidate.pattern); err != nil {
			return fmt.Errorf("%s is invalid: %w", candidate.name, err)
		}
	}

	for phase, steps := range c.Steps {
		for i, step := range steps {
			if step.Name == "" {
				return fmt.Errorf("%s step %d has empty name", phase, i)
			}
			if len(step.Cmd) == 0 {
				return fmt.Errorf("%s step %q has empty cmd", phase, step.Name)
			}
		}
	}

	return nil
}

func (c Config) ValidateTraitOverrides(scope string, overrides map[string]string) error {
	if len(overrides) == 0 {
		return nil
	}

	var schema map[string][]string
	switch scope {
	case "project":
		schema = c.Traits.Project
	case "task":
		schema = c.Traits.Task
	default:
		return fmt.Errorf("unknown trait scope %q", scope)
	}

	for key, value := range overrides {
		allowed, ok := schema[key]
		if !ok {
			return fmt.Errorf("%s trait %q is not declared", scope, key)
		}
		if !contains(allowed, value) {
			return fmt.Errorf("%s trait %q has invalid value %q", scope, key, value)
		}
	}

	return nil
}

func validateDefaultTraits(scope string, values map[string]string, schema map[string][]string) error {
	for key, value := range values {
		allowed, ok := schema[key]
		if !ok {
			return fmt.Errorf("%s trait %q is not declared", scope, key)
		}

		if !contains(allowed, value) {
			return fmt.Errorf("%s trait %q has invalid default %q", scope, key, value)
		}
	}

	return nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}
