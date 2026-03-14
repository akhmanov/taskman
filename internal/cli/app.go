package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/akhmanov/taskman/internal/lifecycle"
	"github.com/akhmanov/taskman/internal/model"
	"github.com/akhmanov/taskman/internal/steps"
	"github.com/akhmanov/taskman/internal/store"
	urfavecli "github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

func BuildApp() *urfavecli.Command {
	projects := &urfavecli.Command{
		Name:  "projects",
		Usage: "Manage project resources",
		Commands: []*urfavecli.Command{
			{Name: "get", Usage: "List projects", Flags: []urfavecli.Flag{outputFlag()}, Action: projectsGetAction},
			{Name: "describe", Usage: "Describe a project", ArgsUsage: "<project>", Flags: []urfavecli.Flag{outputFlag(), describeViewFlag()}, Action: projectsDescribeAction},
			{Name: "create", Usage: "Create a project", ArgsUsage: "<project>", Flags: []urfavecli.Flag{&urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "var"}}, Action: projectsCreateAction},
			{Name: "update", Usage: "Update project metadata", ArgsUsage: "<project>", Flags: []urfavecli.Flag{&urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "var"}, &urfavecli.StringSliceFlag{Name: "unset-var"}}, Action: projectsUpdateAction},
			{Name: "brief", Usage: "Manage project brief", Commands: []*urfavecli.Command{
				{Name: "get", Usage: "Get project brief", ArgsUsage: "<project>", Flags: []urfavecli.Flag{outputFlag()}, Action: projectsBriefGetAction},
				{Name: "set", Usage: "Set project brief", ArgsUsage: "<project>", Flags: briefSetFlags(), Action: projectsBriefSetAction},
				{Name: "init", Usage: "Initialize project brief template", ArgsUsage: "<project>", Flags: []urfavecli.Flag{&urfavecli.BoolFlag{Name: "force"}}, Action: projectsBriefInitAction},
				{Name: "edit", Usage: "Edit project brief in $EDITOR", ArgsUsage: "<project>", Action: projectsBriefEditAction},
			}},
			{Name: "events", Aliases: []string{"event"}, Usage: "Manage project events", Commands: []*urfavecli.Command{
				{Name: "add", Usage: "Add project event", ArgsUsage: "<project>", Flags: payloadEventFlags(), Action: projectsEventAddAction},
				{Name: "get", Usage: "Get project events", ArgsUsage: "<project>", Flags: eventGetFlags(), Action: projectsEventGetAction},
			}},
			{Name: "archive", Usage: "Archive a project", ArgsUsage: "<project>", Action: projectsArchiveAction},
		},
	}

	tasks := &urfavecli.Command{
		Name:  "tasks",
		Usage: "Manage task resources",
		Commands: []*urfavecli.Command{
			{Name: "get", Usage: "List tasks", Flags: []urfavecli.Flag{&urfavecli.StringFlag{Name: "project"}, &urfavecli.StringFlag{Name: "status"}, outputFlag()}, Action: tasksGetAction},
			{Name: "describe", Usage: "Describe a task", ArgsUsage: "<project>/<task>", Flags: []urfavecli.Flag{outputFlag(), describeViewFlag()}, Action: tasksDescribeAction},
			{Name: "create", Usage: "Create a task", Flags: []urfavecli.Flag{&urfavecli.StringFlag{Name: "project", Required: true}, &urfavecli.StringFlag{Name: "name", Required: true}, &urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "var"}}, Action: tasksCreateAction},
			{Name: "update", Usage: "Update task metadata", ArgsUsage: "<project>/<task>", Flags: []urfavecli.Flag{&urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "var"}, &urfavecli.StringSliceFlag{Name: "unset-var"}}, Action: tasksUpdateAction},
			{Name: "brief", Usage: "Manage task brief", Commands: []*urfavecli.Command{
				{Name: "get", Usage: "Get task brief", ArgsUsage: "<project>/<task>", Flags: []urfavecli.Flag{outputFlag()}, Action: tasksBriefGetAction},
				{Name: "set", Usage: "Set task brief", ArgsUsage: "<project>/<task>", Flags: briefSetFlags(), Action: tasksBriefSetAction},
				{Name: "init", Usage: "Initialize task brief template", ArgsUsage: "<project>/<task>", Flags: []urfavecli.Flag{&urfavecli.BoolFlag{Name: "force"}}, Action: tasksBriefInitAction},
				{Name: "edit", Usage: "Edit task brief in $EDITOR", ArgsUsage: "<project>/<task>", Action: tasksBriefEditAction},
			}},
			{Name: "events", Aliases: []string{"event"}, Usage: "Manage task events", Commands: []*urfavecli.Command{
				{Name: "add", Usage: "Add task event", ArgsUsage: "<project>/<task>", Flags: payloadEventFlags(), Action: tasksEventAddAction},
				{Name: "get", Usage: "Get task events", ArgsUsage: "<project>/<task>", Flags: eventGetFlags(), Action: tasksEventGetAction},
			}},
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
	s := runtimeStore(cmd)
	project, err := s.LoadProject(cmd.Args().First())
	if err != nil {
		return err
	}
	brief, err := s.LoadProjectBrief(project.Slug)
	if err != nil {
		return err
	}
	events, err := s.ListProjectEvents(project.Slug)
	if err != nil {
		return err
	}
	view, err := describeView(cmd)
	if err != nil {
		return err
	}

	if view == "agent" {
		digest := buildProjectDigest(project, brief, events)
		activeDecisions := filterActiveDecisions(events)
		openBlockers := filterOpenBlockers(events)
		recentEvents := boundedRecentEvents(events, 3)
		nextAction := nextAction(openBlockers, nil)
		payload := map[string]any{
			"view":             "agent",
			"project_digest":   digest,
			"active_decisions": activeDecisions,
			"recent_events":    recentEvents,
			"open_blockers":    openBlockers,
			"next_action":      nextAction,
		}
		return writeOutput(cmd, payload, func(writer io.Writer) error {
			if _, err := fmt.Fprintf(writer, "Project: %s\n", project.Slug); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "Next Action: %s\n", nextAction); err != nil {
				return err
			}
			if digest.Brief != "" {
				if _, err := fmt.Fprintf(writer, "Mission: %s\n", firstMeaningfulBriefLine(digest.Brief)); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "Active Decisions: %d\n", len(activeDecisions)); err != nil {
				return err
			}
			for _, line := range summarizeEvents("Decision", activeDecisions, 2) {
				if _, err := fmt.Fprintln(writer, line); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "Open Blockers: %d\n", len(openBlockers)); err != nil {
				return err
			}
			for _, line := range summarizeEvents("Blocker", openBlockers, 2) {
				if _, err := fmt.Fprintln(writer, line); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "Recent Events: %d\n", len(recentEvents)); err != nil {
				return err
			}
			for _, line := range summarizeEvents("Recent", recentEvents, 2) {
				if _, err := fmt.Fprintln(writer, line); err != nil {
					return err
				}
			}
			return nil
		})
	}

	payload := map[string]any{
		"view":    "raw",
		"project": project,
		"brief":   brief,
		"events":  events,
	}
	return writeOutput(cmd, payload, func(writer io.Writer) error {
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
		if brief != "" {
			if _, err := fmt.Fprintf(writer, "Brief:\n%s\n", brief); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(writer, "Events: %d\n", len(events)); err != nil {
			return err
		}
		for _, event := range events {
			if _, err := fmt.Fprintf(writer, "- %s [%s] %s\n", event.ID, event.Type, event.Summary); err != nil {
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

func projectsUpdateAction(_ context.Context, cmd *urfavecli.Command) error {
	vars, err := parseVars(cmd.StringSlice("var"))
	if err != nil {
		return err
	}
	_, err = projectService(cmd).Update(cmd.Args().First(), cmd.StringSlice("label"), vars, cmd.StringSlice("unset-var"))
	return err
}

func projectsBriefGetAction(_ context.Context, cmd *urfavecli.Command) error {
	brief, err := projectService(cmd).GetBrief(cmd.Args().First())
	if err != nil {
		return err
	}
	return writeOutput(cmd, map[string]string{"brief": brief}, func(writer io.Writer) error {
		_, err := io.WriteString(writer, brief)
		return err
	})
}

func projectsBriefSetAction(_ context.Context, cmd *urfavecli.Command) error {
	brief, err := resolveBriefContent(cmd)
	if err != nil {
		return err
	}
	return projectService(cmd).SetBrief(cmd.Args().First(), brief)
}

func projectsBriefInitAction(_ context.Context, cmd *urfavecli.Command) error {
	current, err := projectService(cmd).GetBrief(cmd.Args().First())
	if err != nil {
		return err
	}
	if strings.TrimSpace(current) != "" && !cmd.Bool("force") {
		return fmt.Errorf("project brief already exists; use --force to overwrite")
	}
	return projectService(cmd).SetBrief(cmd.Args().First(), model.ProjectBriefTemplate)
}

func projectsBriefEditAction(_ context.Context, cmd *urfavecli.Command) error {
	current, err := projectService(cmd).GetBrief(cmd.Args().First())
	if err != nil {
		return err
	}
	edited, err := editBriefContent(current, model.ProjectBriefTemplate)
	if err != nil {
		return err
	}
	return projectService(cmd).SetBrief(cmd.Args().First(), edited)
}

func projectsEventAddAction(_ context.Context, cmd *urfavecli.Command) error {
	event, err := payloadEventFromFlags(cmd)
	if err != nil {
		return err
	}
	return projectService(cmd).AddEvent(cmd.Args().First(), event)
}

func projectsEventGetAction(_ context.Context, cmd *urfavecli.Command) error {
	events, err := projectService(cmd).GetEvents(cmd.Args().First())
	if err != nil {
		return err
	}
	events = filterPayloadEvents(events, cmd.String("type"), cmd.Bool("active-only"))
	return writeOutput(cmd, events, func(writer io.Writer) error {
		for _, event := range events {
			if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", event.ID, event.Type, event.Summary); err != nil {
				return err
			}
		}
		return nil
	})
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
	s := runtimeStore(cmd)
	loaded, err := s.LoadTask(project, task)
	if err != nil {
		return err
	}
	artifacts, err := s.ListArtifacts(project, task)
	if err != nil {
		return err
	}
	brief, err := s.LoadTaskBrief(project, task)
	if err != nil {
		return err
	}
	events, err := s.ListTaskEvents(project, task)
	if err != nil {
		return err
	}
	view, err := describeView(cmd)
	if err != nil {
		return err
	}

	if view == "agent" {
		projectState, err := s.LoadProject(project)
		if err != nil {
			return err
		}
		projectBrief, err := s.LoadProjectBrief(project)
		if err != nil {
			return err
		}
		projectEvents, err := s.ListProjectEvents(project)
		if err != nil {
			return err
		}
		cfg, err := s.LoadConfig()
		if err != nil {
			return err
		}
		allowedTransitions := allowedTransitionsForStatus(cfg, loaded.Status)
		activeDecisions := filterActiveDecisions(events)
		openBlockers := filterOpenBlockers(events)
		recentEvents := boundedRecentEvents(events, 3)
		nextAction := nextAction(openBlockers, allowedTransitions)
		payload := map[string]any{
			"view":                "agent",
			"task_brief":          brief,
			"project_digest":      buildProjectDigest(projectState, projectBrief, projectEvents),
			"active_decisions":    activeDecisions,
			"recent_events":       recentEvents,
			"open_blockers":       openBlockers,
			"next_action":         nextAction,
			"allowed_transitions": allowedTransitions,
		}
		return writeOutput(cmd, payload, func(writer io.Writer) error {
			if _, err := fmt.Fprintf(writer, "%s/%s\t%s\n", loaded.Project, loaded.Slug, loaded.Status); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "Next Action: %s\n", nextAction); err != nil {
				return err
			}
			if brief != "" {
				if _, err := fmt.Fprintf(writer, "Intent: %s\n", firstMeaningfulBriefLine(brief)); err != nil {
					return err
				}
			}
			if projectDigest, ok := payload["project_digest"].(projectDigest); ok && projectDigest.Brief != "" {
				if _, err := fmt.Fprintf(writer, "Project Digest: %s\n", firstMeaningfulBriefLine(projectDigest.Brief)); err != nil {
					return err
				}
			}
			if len(allowedTransitions) > 0 {
				if _, err := fmt.Fprintf(writer, "Allowed Transitions: %s\n", strings.Join(allowedTransitions, ", ")); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "Active Decisions: %d\n", len(activeDecisions)); err != nil {
				return err
			}
			for _, line := range summarizeEvents("Decision", activeDecisions, 2) {
				if _, err := fmt.Fprintln(writer, line); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "Open Blockers: %d\n", len(openBlockers)); err != nil {
				return err
			}
			for _, line := range summarizeEvents("Blocker", openBlockers, 2) {
				if _, err := fmt.Fprintln(writer, line); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(writer, "Recent Events: %d\n", len(recentEvents)); err != nil {
				return err
			}
			for _, line := range summarizeEvents("Recent", recentEvents, 2) {
				if _, err := fmt.Fprintln(writer, line); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return writeOutput(cmd, map[string]any{
		"view":      "raw",
		"task":      loaded,
		"brief":     brief,
		"events":    events,
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
		if brief != "" {
			if _, err := fmt.Fprintf(writer, "Brief:\n%s\n", brief); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(writer, "Events: %d\n", len(events)); err != nil {
			return err
		}
		for _, event := range events {
			if _, err := fmt.Fprintf(writer, "- %s [%s] %s\n", event.ID, event.Type, event.Summary); err != nil {
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

func tasksUpdateAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	vars, err := parseVars(cmd.StringSlice("var"))
	if err != nil {
		return err
	}
	_, err = taskService(cmd).Update(projectSlug, taskSlug, cmd.StringSlice("label"), vars, cmd.StringSlice("unset-var"))
	return err
}

func tasksBriefGetAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	brief, err := taskService(cmd).GetBrief(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	return writeOutput(cmd, map[string]string{"brief": brief}, func(writer io.Writer) error {
		_, err := io.WriteString(writer, brief)
		return err
	})
}

func tasksBriefSetAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	brief, err := resolveBriefContent(cmd)
	if err != nil {
		return err
	}
	return taskService(cmd).SetBrief(projectSlug, taskSlug, brief)
}

func tasksBriefInitAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	current, err := taskService(cmd).GetBrief(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	if strings.TrimSpace(current) != "" && !cmd.Bool("force") {
		return fmt.Errorf("task brief already exists; use --force to overwrite")
	}
	return taskService(cmd).SetBrief(projectSlug, taskSlug, model.TaskBriefTemplate)
}

func tasksBriefEditAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	current, err := taskService(cmd).GetBrief(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	edited, err := editBriefContent(current, model.TaskBriefTemplate)
	if err != nil {
		return err
	}
	return taskService(cmd).SetBrief(projectSlug, taskSlug, edited)
}

func tasksEventAddAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	event, err := payloadEventFromFlags(cmd)
	if err != nil {
		return err
	}
	return taskService(cmd).AddEvent(projectSlug, taskSlug, event)
}

func tasksEventGetAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	events, err := taskService(cmd).GetEvents(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	events = filterPayloadEvents(events, cmd.String("type"), cmd.Bool("active-only"))
	return writeOutput(cmd, events, func(writer io.Writer) error {
		for _, event := range events {
			if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", event.ID, event.Type, event.Summary); err != nil {
				return err
			}
		}
		return nil
	})
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

func briefSetFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "content"},
		&urfavecli.StringFlag{Name: "file"},
	}
}

func resolveBriefContent(cmd *urfavecli.Command) (string, error) {
	content := cmd.String("content")
	file := cmd.String("file")
	if strings.TrimSpace(content) == "" && strings.TrimSpace(file) == "" {
		return "", fmt.Errorf("either --content or --file is required")
	}
	if strings.TrimSpace(content) != "" && strings.TrimSpace(file) != "" {
		return "", fmt.Errorf("use either --content or --file, not both")
	}
	if strings.TrimSpace(file) != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return content, nil
}

func editBriefContent(current, fallback string) (string, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return "", fmt.Errorf("EDITOR is not set")
	}
	initial := current
	if strings.TrimSpace(initial) == "" {
		initial = fallback
	}
	tmpFile, err := os.CreateTemp("", "taskman-brief-*.md")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if err := tmpFile.Close(); err != nil {
		return "", err
	}
	if err := os.WriteFile(tmpPath, []byte(initial), 0o600); err != nil {
		return "", err
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return "", fmt.Errorf("EDITOR is not set")
	}
	command := exec.Command(parts[0], append(parts[1:], tmpPath)...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Clean(tmpPath))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func payloadEventFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "id", Required: true},
		&urfavecli.StringFlag{Name: "at", Required: true},
		&urfavecli.StringFlag{Name: "type", Required: true},
		&urfavecli.StringFlag{Name: "summary", Required: true},
		&urfavecli.StringFlag{Name: "actor", Required: true},
		&urfavecli.StringFlag{Name: "details"},
		&urfavecli.StringFlag{Name: "session"},
		&urfavecli.StringSliceFlag{Name: "ref"},
		&urfavecli.StringFlag{Name: "rationale"},
		&urfavecli.StringFlag{Name: "impact"},
		&urfavecli.StringFlag{Name: "status"},
	}
}

func eventGetFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		outputFlag(),
		&urfavecli.StringFlag{Name: "type"},
		&urfavecli.BoolFlag{Name: "active-only"},
	}
}

func filterPayloadEvents(events []model.PayloadEvent, eventType string, activeOnly bool) []model.PayloadEvent {
	filtered := make([]model.PayloadEvent, 0, len(events))
	for _, event := range events {
		if eventType != "" && string(event.Type) != eventType {
			continue
		}
		if activeOnly && event.Status != "active" {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func payloadEventFromFlags(cmd *urfavecli.Command) (model.PayloadEvent, error) {
	eventType := model.PayloadEventType(cmd.String("type"))
	if eventType == "" {
		return model.PayloadEvent{}, fmt.Errorf("event type is required")
	}
	return model.PayloadEvent{
		ID:        cmd.String("id"),
		At:        cmd.String("at"),
		Type:      eventType,
		Summary:   cmd.String("summary"),
		Details:   cmd.String("details"),
		Actor:     cmd.String("actor"),
		Session:   cmd.String("session"),
		Refs:      cmd.StringSlice("ref"),
		Rationale: cmd.String("rationale"),
		Impact:    cmd.String("impact"),
		Status:    cmd.String("status"),
	}, nil
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

func describeViewFlag() *urfavecli.StringFlag {
	return &urfavecli.StringFlag{Name: "view", Value: "raw", Usage: "Describe view: raw or agent"}
}

func describeView(cmd *urfavecli.Command) (string, error) {
	view := strings.TrimSpace(cmd.String("view"))
	if view == "" {
		return "raw", nil
	}
	if view != "raw" && view != "agent" {
		return "", fmt.Errorf("view must be one of: raw, agent")
	}
	return view, nil
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

type projectDigest struct {
	Slug            string               `json:"slug" yaml:"slug"`
	Status          model.ProjectStatus  `json:"status" yaml:"status"`
	Brief           string               `json:"brief" yaml:"brief"`
	ActiveDecisions []model.PayloadEvent `json:"active_decisions" yaml:"active_decisions"`
	OpenBlockers    []model.PayloadEvent `json:"open_blockers" yaml:"open_blockers"`
}

func buildProjectDigest(project model.ProjectState, brief string, events []model.PayloadEvent) projectDigest {
	return projectDigest{
		Slug:            project.Slug,
		Status:          project.Status,
		Brief:           brief,
		ActiveDecisions: filterActiveDecisions(events),
		OpenBlockers:    filterOpenBlockers(events),
	}
}

func filterActiveDecisions(events []model.PayloadEvent) []model.PayloadEvent {
	decisions := make([]model.PayloadEvent, 0)
	for _, event := range events {
		if event.Type != model.PayloadEventTypeDecision {
			continue
		}
		if isInactiveEventStatus(event.Status) {
			continue
		}
		decisions = append(decisions, event)
	}
	return decisions
}

func filterOpenBlockers(events []model.PayloadEvent) []model.PayloadEvent {
	blockers := make([]model.PayloadEvent, 0)
	for _, event := range events {
		if event.Type != model.PayloadEventTypeBlocker {
			continue
		}
		if isInactiveEventStatus(event.Status) {
			continue
		}
		blockers = append(blockers, event)
	}
	return blockers
}

func boundedRecentEvents(events []model.PayloadEvent, limit int) []model.PayloadEvent {
	if limit <= 0 || len(events) == 0 {
		return []model.PayloadEvent{}
	}
	if len(events) < limit {
		limit = len(events)
	}
	recent := make([]model.PayloadEvent, 0, limit)
	for idx := len(events) - 1; idx >= len(events)-limit; idx-- {
		recent = append(recent, events[idx])
	}
	return recent
}

func nextAction(openBlockers []model.PayloadEvent, allowedTransitions []string) string {
	if len(openBlockers) > 0 {
		return "Resolve blocker: " + openBlockers[0].Summary
	}
	if len(allowedTransitions) > 0 {
		return "Run transition: " + allowedTransitions[0]
	}
	return "Update project/task brief with current truth"
}

func firstMeaningfulBriefLine(brief string) string {
	for _, line := range strings.Split(brief, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
			continue
		}
		return trimmed
	}
	return ""
}

func summarizeEvents(prefix string, events []model.PayloadEvent, limit int) []string {
	if limit <= 0 || len(events) == 0 {
		return nil
	}
	if len(events) < limit {
		limit = len(events)
	}
	lines := make([]string, 0, limit)
	for _, event := range events[:limit] {
		lines = append(lines, fmt.Sprintf("%s: %s", prefix, event.Summary))
	}
	return lines
}

func allowedTransitionsForStatus(cfg model.Config, status model.TaskStatus) []string {
	allowed := make([]string, 0)
	for _, name := range sortedKeys(cfg.Workflow.Task.Transitions) {
		transition := cfg.Workflow.Task.Transitions[name]
		if statusAllowed(status, transition.From) {
			allowed = append(allowed, string(name))
		}
	}
	return allowed
}

func statusAllowed(current model.TaskStatus, allowed []model.TaskStatus) bool {
	for _, status := range allowed {
		if current == status {
			return true
		}
	}
	return false
}

func isInactiveEventStatus(status string) bool {
	value := strings.ToLower(strings.TrimSpace(status))
	switch value {
	case "resolved", "inactive", "closed", "superseded", "rejected", "cancelled":
		return true
	default:
		return false
	}
}
