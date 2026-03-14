package model

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"
)

type Config struct {
	Version  int            `yaml:"version"`
	Defaults Defaults       `yaml:"defaults"`
	Vars     VarSchema      `yaml:"vars,omitempty"`
	Naming   Naming         `yaml:"naming,omitempty"`
	Workflow WorkflowConfig `yaml:"workflow"`
}

type Defaults struct {
	Project MetadataDefaults `yaml:"project"`
	Task    MetadataDefaults `yaml:"task"`
}

type MetadataDefaults struct {
	Labels []string          `yaml:"labels"`
	Vars   map[string]string `yaml:"vars"`
}

type VarSchema struct {
	Project map[string]VarRule `yaml:"project,omitempty"`
	Task    map[string]VarRule `yaml:"task,omitempty"`
}

type VarRule struct {
	Required bool     `yaml:"required,omitempty"`
	Allowed  []string `yaml:"allowed,omitempty"`
}

type Naming struct {
	TaskSlug string `yaml:"task_slug,omitempty"`
}

type WorkflowConfig struct {
	Task    TaskWorkflowConfig    `yaml:"task"`
	Project ProjectWorkflowConfig `yaml:"project,omitempty"`
}

type TaskWorkflowConfig struct {
	Statuses         []TaskStatus              `yaml:"statuses"`
	InitialStatus    TaskStatus                `yaml:"initial_status"`
	TerminalStatuses []TaskStatus              `yaml:"terminal_statuses,omitempty"`
	Transitions      map[string]TaskTransition `yaml:"transitions"`
}

type ProjectWorkflowConfig struct {
	Archive ArchiveWorkflow `yaml:"archive,omitempty"`
}

type ArchiveWorkflow struct {
	Steps []Step `yaml:"steps,omitempty"`
}

type TaskTransition struct {
	From     []TaskStatus `yaml:"from"`
	To       TaskStatus   `yaml:"to"`
	Requires []string     `yaml:"requires,omitempty"`
	Steps    []Step       `yaml:"steps,omitempty"`
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

	if err := validateDefaultVars("project", c.Defaults.Project.Vars, c.Vars.Project); err != nil {
		return err
	}

	if err := validateDefaultVars("task", c.Defaults.Task.Vars, c.Vars.Task); err != nil {
		return err
	}

	if c.Naming.TaskSlug != "" {
		if err := validateTemplate("naming.task_slug", c.Naming.TaskSlug, map[string]any{
			"name":    "name",
			"project": map[string]string{"slug": "project"},
			"vars":    map[string]string{"kind": "feature"},
		}); err != nil {
			return fmt.Errorf("naming.task_slug is invalid: %w", err)
		}
	}

	if err := c.Workflow.Validate(c.Vars.Task); err != nil {
		return err
	}

	if err := validateSteps("workflow.project.archive", c.Workflow.Project.Archive.Steps); err != nil {
		return err
	}

	return nil
}

func (w WorkflowConfig) Validate(taskVars map[string]VarRule) error {
	if err := w.Task.Validate(taskVars); err != nil {
		return err
	}
	return nil
}

func (w TaskWorkflowConfig) Validate(taskVars map[string]VarRule) error {
	if len(w.Statuses) == 0 {
		return fmt.Errorf("workflow.task.statuses must not be empty")
	}
	if w.InitialStatus == "" {
		return fmt.Errorf("workflow.task.initial_status must not be empty")
	}
	if !slices.Contains(w.Statuses, w.InitialStatus) {
		return fmt.Errorf("workflow.task.initial_status %q is not declared", w.InitialStatus)
	}
	for _, status := range w.TerminalStatuses {
		if !slices.Contains(w.Statuses, status) {
			return fmt.Errorf("workflow.task.terminal_statuses contains undeclared status %q", status)
		}
	}
	if len(w.Transitions) == 0 {
		return fmt.Errorf("workflow.task.transitions must not be empty")
	}

	for name, transition := range w.Transitions {
		if name == "" {
			return fmt.Errorf("workflow.task.transitions has empty name")
		}
		if len(transition.From) == 0 {
			return fmt.Errorf("workflow.task.transitions.%s must declare at least one from status", name)
		}
		for _, status := range transition.From {
			if !slices.Contains(w.Statuses, status) {
				return fmt.Errorf("workflow.task.transitions.%s references undeclared from status %q", name, status)
			}
		}
		if !slices.Contains(w.Statuses, transition.To) {
			return fmt.Errorf("workflow.task.transitions.%s references undeclared to status %q", name, transition.To)
		}
		for _, required := range transition.Requires {
			if _, ok := taskVars[required]; !ok {
				return fmt.Errorf("workflow.task.transitions.%s requires undeclared task var %q", name, required)
			}
		}
		if err := validateSteps("workflow.task.transitions."+name, transition.Steps); err != nil {
			return err
		}
	}

	return nil
}

func (w TaskWorkflowConfig) IsTerminal(status TaskStatus) bool {
	return slices.Contains(w.TerminalStatuses, status)
}

func validateSteps(scope string, steps []Step) error {
	for i, step := range steps {
		if step.Name == "" {
			return fmt.Errorf("%s step %d has empty name", scope, i)
		}
		if len(step.Cmd) == 0 {
			return fmt.Errorf("%s step %q has empty cmd", scope, step.Name)
		}
		for key := range step.When {
			if !isValidWhenSelector(key) {
				return fmt.Errorf("%s step %q has invalid when selector %q", scope, step.Name, key)
			}
		}
	}
	return nil
}

func isValidWhenSelector(key string) bool {
	switch {
	case strings.HasPrefix(key, "task.vars."):
		return len(strings.TrimPrefix(key, "task.vars.")) > 0
	case strings.HasPrefix(key, "project.vars."):
		return len(strings.TrimPrefix(key, "project.vars.")) > 0
	case key == "task.status", key == "project.status", key == "task.slug", key == "project.slug", key == "transition":
		return true
	default:
		return false
	}
}

func validateTemplate(name, pattern string, values any) error {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(pattern)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, values); err != nil {
		return err
	}
	return nil
}

func (c Config) ValidateVarOverrides(scope string, overrides map[string]string) error {
	if len(overrides) == 0 {
		return nil
	}

	var schema map[string]VarRule
	switch scope {
	case "project":
		schema = c.Vars.Project
	case "task":
		schema = c.Vars.Task
	default:
		return fmt.Errorf("unknown var scope %q", scope)
	}

	return validateVars(scope, overrides, schema)
}

func (c Config) ValidateRequiredVars(scope string, values map[string]string) error {
	var schema map[string]VarRule
	switch scope {
	case "project":
		schema = c.Vars.Project
	case "task":
		schema = c.Vars.Task
	default:
		return fmt.Errorf("unknown var scope %q", scope)
	}

	for key, rule := range schema {
		if rule.Required && values[key] == "" {
			return fmt.Errorf("%s var %q is required", scope, key)
		}
	}
	return nil
}

func validateDefaultVars(scope string, values map[string]string, schema map[string]VarRule) error {
	return validateVars(scope, values, schema)
}

func validateVars(scope string, values map[string]string, schema map[string]VarRule) error {
	for key, value := range values {
		rule, ok := schema[key]
		if !ok {
			return fmt.Errorf("%s var %q is not declared", scope, key)
		}
		if len(rule.Allowed) > 0 && !slices.Contains(rule.Allowed, value) {
			return fmt.Errorf("%s var %q has invalid value %q", scope, key, value)
		}
	}
	return nil
}

func MergeVars(base, override map[string]string) map[string]string {
	merged := map[string]string{}
	maps.Copy(merged, base)
	maps.Copy(merged, override)
	return merged
}
