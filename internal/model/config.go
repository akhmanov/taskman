package model

import (
	"fmt"
	"maps"
)

type Config struct {
	Defaults   Defaults         `yaml:"defaults,omitempty"`
	Middleware MiddlewareConfig `yaml:"middleware,omitempty"`
}

type Defaults struct {
	Project MetadataDefaults `yaml:"project,omitempty"`
	Task    MetadataDefaults `yaml:"task,omitempty"`
}

type MetadataDefaults struct {
	Labels []string          `yaml:"labels,omitempty"`
	Vars   map[string]string `yaml:"vars,omitempty"`
}

type MiddlewareConfig struct {
	Project map[string]TransitionMiddleware `yaml:"project,omitempty"`
	Task    map[string]TransitionMiddleware `yaml:"task,omitempty"`
}

type TransitionMiddleware struct {
	Pre  []MiddlewareCommand `yaml:"pre,omitempty"`
	Post []MiddlewareCommand `yaml:"post,omitempty"`
}

type MiddlewareCommand struct {
	Name string   `yaml:"name"`
	Cmd  []string `yaml:"cmd"`
}

func (c Config) Validate() error {
	if err := validateMiddlewareScope("middleware.project", c.Middleware.Project); err != nil {
		return err
	}
	if err := validateMiddlewareScope("middleware.task", c.Middleware.Task); err != nil {
		return err
	}
	return nil
}

func validateMiddlewareScope(scope string, transitions map[string]TransitionMiddleware) error {
	resource := "task"
	if scope == "middleware.project" {
		resource = "project"
	}
	for name, middleware := range transitions {
		if name == "" {
			return fmt.Errorf("%s has empty transition name", scope)
		}
		if !IsSupportedTransitionVerb(name) {
			return fmt.Errorf("unknown %s middleware transition %q", resource, name)
		}
		if err := validateMiddlewareCommands(scope+"."+name+".pre", middleware.Pre); err != nil {
			return err
		}
		if err := validateMiddlewareCommands(scope+"."+name+".post", middleware.Post); err != nil {
			return err
		}
	}
	return nil
}

func validateMiddlewareCommands(scope string, commands []MiddlewareCommand) error {
	for index, command := range commands {
		if command.Name == "" {
			return fmt.Errorf("%s command %d has empty name", scope, index)
		}
		if len(command.Cmd) == 0 {
			return fmt.Errorf("%s command %q has empty cmd", scope, command.Name)
		}
	}
	return nil
}

func (c Config) MiddlewareFor(scope, verb string) TransitionMiddleware {
	if scope == "project" {
		return c.Middleware.Project[verb]
	}
	return c.Middleware.Task[verb]
}

func MergeVars(base, override map[string]string) map[string]string {
	merged := map[string]string{}
	maps.Copy(merged, base)
	maps.Copy(merged, override)
	return merged
}

const DefaultConfigYAML = `defaults:
  project:
    labels: []
  task:
    labels: []
middleware:
  project: {}
  task: {}
`
