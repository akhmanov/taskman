package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/assistant-wi/taskman/internal/lifecycle"
	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/steps"
	"github.com/assistant-wi/taskman/internal/store"
	urfavecli "github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

func BuildApp() *urfavecli.Command {
	projects := &urfavecli.Command{
		Name:  "projects",
		Usage: "Manage project resources",
		Commands: []*urfavecli.Command{
			{Name: "get", Usage: "List projects", Flags: []urfavecli.Flag{outputFlag()}, Action: projectsGetAction},
			{Name: "describe", Usage: "Describe a project", ArgsUsage: "<project>", Flags: []urfavecli.Flag{outputFlag()}, Action: projectsDescribeAction},
			{Name: "create", Usage: "Create a project", ArgsUsage: "<project>", Flags: []urfavecli.Flag{&urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "var"}}, Action: projectsCreateAction},
			{Name: "archive", Usage: "Archive a project", ArgsUsage: "<project>", Action: projectsArchiveAction},
		},
	}

	tasks := &urfavecli.Command{
		Name:  "tasks",
		Usage: "Manage task resources",
		Commands: []*urfavecli.Command{
			{Name: "get", Usage: "List tasks", Flags: []urfavecli.Flag{&urfavecli.StringFlag{Name: "project"}, &urfavecli.StringFlag{Name: "status"}, outputFlag()}, Action: tasksGetAction},
			{Name: "describe", Usage: "Describe a task", ArgsUsage: "<project>/<task>", Flags: []urfavecli.Flag{outputFlag()}, Action: tasksDescribeAction},
			{Name: "create", Usage: "Create a task", Flags: []urfavecli.Flag{&urfavecli.StringFlag{Name: "project", Required: true}, &urfavecli.StringFlag{Name: "name", Required: true}, &urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "var"}}, Action: tasksCreateAction},
			{Name: "transition", Usage: "Run a task transition", ArgsUsage: "<project>/<task> <transition>", Action: tasksTransitionAction},
		},
	}

	return &urfavecli.Command{
		Name:        "taskman",
		Usage:       "Task workflow engine for projects, tasks, and lifecycle operations",
		Description: "Resources: projects, tasks, doctor\n\nExamples:\n  taskman projects get\n  taskman projects create user-permissions\n  taskman projects archive user-permissions\n  taskman tasks get --project user-permissions --status active\n  taskman tasks describe user-permissions/api-auth\n  taskman tasks transition user-permissions/api-auth start",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{Name: "root", Value: "../tasks", Usage: "Path to the runtime tasks root"},
		},
		Commands: []*urfavecli.Command{
			projects,
			tasks,
			{Name: "doctor", Usage: "Inspect taskman health", Action: doctorAction},
		},
	}
}

func runtimeStore(cmd *urfavecli.Command) store.Store {
	return store.New(cmd.String("root"))
}

func projectService(cmd *urfavecli.Command) lifecycle.ProjectService {
	s := runtimeStore(cmd)
	return lifecycle.NewProjectService(s, steps.New(cmd.String("root")))
}

func taskService(cmd *urfavecli.Command) lifecycle.TaskService {
	s := runtimeStore(cmd)
	return lifecycle.NewTaskService(s, steps.New(cmd.String("root")))
}

func projectsGetAction(_ context.Context, cmd *urfavecli.Command) error {
	projects, err := runtimeStore(cmd).ListProjects()
	if err != nil {
		return err
	}
	return writeOutput(cmd, projects, func(writer io.Writer) error {
		for _, project := range projects {
			if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", project.Slug, project.Status, formatCounts(project.Tasks)); err != nil {
				return err
			}
		}
		return nil
	})
}

func projectsDescribeAction(_ context.Context, cmd *urfavecli.Command) error {
	project, err := runtimeStore(cmd).LoadProject(cmd.Args().First())
	if err != nil {
		return err
	}
	return writeOutput(cmd, project, func(writer io.Writer) error {
		if _, err := fmt.Fprintf(writer, "Project: %s\n", project.Slug); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "Status: %s\n", project.Status); err != nil {
			return err
		}
		if len(project.Labels) > 0 {
			if _, err := fmt.Fprintf(writer, "Labels: %s\n", strings.Join(project.Labels, ", ")); err != nil {
				return err
			}
		}
		if len(project.Vars) > 0 {
			if _, err := fmt.Fprintf(writer, "Vars: %s\n", formatStringMap(project.Vars)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(writer, "Tasks: %s\n", formatCounts(project.Tasks)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "Archive Ready: %t\n", project.Archive.Ready); err != nil {
			return err
		}
		if len(project.Archive.Blockers) > 0 {
			if _, err := fmt.Fprintf(writer, "Archive Blockers: %s\n", strings.Join(project.Archive.Blockers, "; ")); err != nil {
				return err
			}
		}
		return nil
	})
}

func projectsCreateAction(_ context.Context, cmd *urfavecli.Command) error {
	vars, err := parseVars(cmd.StringSlice("var"))
	if err != nil {
		return err
	}
	_, err = projectService(cmd).Create(cmd.Args().First(), cmd.StringSlice("label"), vars)
	return err
}

func projectsArchiveAction(_ context.Context, cmd *urfavecli.Command) error {
	_, err := projectService(cmd).Archive(cmd.Args().First())
	return err
}

func tasksGetAction(_ context.Context, cmd *urfavecli.Command) error {
	tasks, err := runtimeStore(cmd).ListTasks(cmd.String("project"))
	if err != nil {
		return err
	}
	status := cmd.String("status")
	filtered := make([]model.TaskState, 0, len(tasks))
	for _, task := range tasks {
		if status != "" && string(task.Status) != status {
			continue
		}
		filtered = append(filtered, task)
	}
	return writeOutput(cmd, filtered, func(writer io.Writer) error {
		for _, task := range filtered {
			if _, err := fmt.Fprintf(writer, "%s/%s\t%s\n", task.Project, task.Slug, task.Status); err != nil {
				return err
			}
		}
		return nil
	})
}

func tasksDescribeAction(_ context.Context, cmd *urfavecli.Command) error {
	project, task, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	loaded, err := runtimeStore(cmd).LoadTask(project, task)
	if err != nil {
		return err
	}
	artifacts, err := runtimeStore(cmd).ListArtifacts(project, task)
	if err != nil {
		return err
	}
	return writeOutput(cmd, map[string]any{
		"task":      loaded,
		"artifacts": artifacts,
	}, func(writer io.Writer) error {
		if _, err := fmt.Fprintf(writer, "%s/%s\t%s\n", loaded.Project, loaded.Slug, loaded.Status); err != nil {
			return err
		}
		if len(loaded.Vars) > 0 {
			if _, err := fmt.Fprintf(writer, "Vars: %s\n", formatStringMap(loaded.Vars)); err != nil {
				return err
			}
		}
		for _, kind := range sortedKeys(artifacts) {
			if _, err := fmt.Fprintf(writer, "Artifact %s: %s\n", kind, formatMap(artifacts[kind])); err != nil {
				return err
			}
		}
		return nil
	})
}

func tasksCreateAction(_ context.Context, cmd *urfavecli.Command) error {
	vars, err := parseVars(cmd.StringSlice("var"))
	if err != nil {
		return err
	}
	_, err = taskService(cmd).Create(cmd.String("project"), cmd.String("name"), cmd.StringSlice("label"), vars)
	return err
}

func tasksTransitionAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().Get(0))
	if err != nil {
		return err
	}
	transition := cmd.Args().Get(1)
	if transition == "" {
		return fmt.Errorf("transition name is required")
	}
	_, err = taskService(cmd).Transition(projectSlug, taskSlug, transition)
	return err
}

func doctorAction(_ context.Context, cmd *urfavecli.Command) error {
	_, err := runtimeStore(cmd).LoadConfig()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(commandWriter(cmd), "ok")
	return err
}

func splitTaskID(value string) (string, string, error) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("task id must be <project>/<task>")
	}
	return parts[0], parts[1], nil
}

func parseVars(values []string) (map[string]string, error) {
	vars := map[string]string{}
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("var must be key=value")
		}
		vars[parts[0]] = parts[1]
	}
	return vars, nil
}

func commandWriter(cmd *urfavecli.Command) io.Writer {
	if cmd.Writer != nil {
		return cmd.Writer
	}
	if root := cmd.Root(); root != nil && root.Writer != nil {
		return root.Writer
	}
	return os.Stdout
}

func outputFlag() *urfavecli.StringFlag {
	return &urfavecli.StringFlag{Name: "output", Value: "text", Usage: "Output format: text, json, yaml"}
}

func writeOutput(cmd *urfavecli.Command, value any, renderText func(io.Writer) error) error {
	writer := commandWriter(cmd)
	switch cmd.String("output") {
	case "", "text":
		return renderText(writer)
	case "json":
		enc := json.NewEncoder(writer)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	case "yaml":
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	default:
		return fmt.Errorf("output must be one of: text, json, yaml")
	}
}

func formatCounts(counts model.TaskCounts) string {
	if len(counts) == 0 {
		return ""
	}
	parts := make([]string, 0, len(counts))
	for _, key := range sortedKeys(counts) {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}

func formatMap(values map[string]any) string {
	parts := make([]string, 0, len(values))
	for _, key := range sortedKeys(values) {
		parts = append(parts, fmt.Sprintf("%s=%v", key, values[key]))
	}
	return strings.Join(parts, ", ")
}

func formatStringMap(values map[string]string) string {
	parts := make([]string, 0, len(values))
	for _, key := range sortedKeys(values) {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(parts, ", ")
}

func sortedKeys[K ~string, V any](values map[K]V) []K {
	keys := slices.Collect(maps.Keys(values))
	slices.Sort(keys)
	return keys
}
