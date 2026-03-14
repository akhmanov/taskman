package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/assistant-wi/taskman/internal/lifecycle"
	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/steps"
	"github.com/assistant-wi/taskman/internal/store"
	urfavecli "github.com/urfave/cli/v3"
)

func BuildApp() *urfavecli.Command {
	projects := &urfavecli.Command{
		Name:  "projects",
		Usage: "Manage project resources",
		Commands: []*urfavecli.Command{
			{Name: "get", Usage: "List projects", Action: projectsGetAction},
			{Name: "describe", Usage: "Describe a project", ArgsUsage: "<project>", Action: projectsDescribeAction},
			{Name: "create", Usage: "Create a project", ArgsUsage: "<project>", Flags: []urfavecli.Flag{&urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "trait"}}, Action: projectsCreateAction},
			{Name: "archive", Usage: "Archive a project", ArgsUsage: "<project>", Action: projectsArchiveAction},
		},
	}

	tasks := &urfavecli.Command{
		Name:  "tasks",
		Usage: "Manage task resources",
		Commands: []*urfavecli.Command{
			{Name: "get", Usage: "List tasks", Flags: []urfavecli.Flag{&urfavecli.StringFlag{Name: "project"}, &urfavecli.StringFlag{Name: "status"}}, Action: tasksGetAction},
			{Name: "describe", Usage: "Describe a task", ArgsUsage: "<project>/<task>", Action: tasksDescribeAction},
			{Name: "create", Usage: "Create a task", Flags: []urfavecli.Flag{&urfavecli.StringFlag{Name: "project", Required: true}, &urfavecli.StringFlag{Name: "repo", Required: true}, &urfavecli.StringFlag{Name: "name", Required: true}, &urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "trait"}}, Action: tasksCreateAction},
			{Name: "block", Usage: "Block a task", ArgsUsage: "<project>/<task>", Action: tasksBlockAction},
			{Name: "unblock", Usage: "Unblock a task", ArgsUsage: "<project>/<task>", Action: tasksUnblockAction},
			{Name: "done", Usage: "Complete a task", ArgsUsage: "<project>/<task>", Action: tasksDoneAction},
			{Name: "cancel", Usage: "Cancel a task", ArgsUsage: "<project>/<task>", Action: tasksCancelAction},
			{Name: "cleanup", Usage: "Cleanup a task", ArgsUsage: "<project>/<task>", Action: tasksCleanupAction},
		},
	}

	return &urfavecli.Command{
		Name:        "taskman",
		Usage:       "Task workflow engine for projects, tasks, and lifecycle operations",
		Description: "Resources: projects, tasks, doctor\n\nExamples:\n  taskman projects get\n  taskman projects create user-permissions\n  taskman projects archive user-permissions\n  taskman tasks get --project user-permissions --status active\n  taskman tasks describe user-permissions/cloud-api-auth\n  taskman tasks done user-permissions/cloud-api-auth\n  taskman tasks cleanup user-permissions/cloud-api-auth",
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
	writer := commandWriter(cmd)
	for _, project := range projects {
		if _, err := fmt.Fprintf(writer, "%s\t%s\n", project.Slug, project.Status); err != nil {
			return err
		}
	}
	return nil
}

func projectsDescribeAction(_ context.Context, cmd *urfavecli.Command) error {
	project, err := runtimeStore(cmd).LoadProject(cmd.Args().First())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(commandWriter(cmd), "%s\t%s\n", project.Slug, project.Status)
	return err
}

func projectsCreateAction(_ context.Context, cmd *urfavecli.Command) error {
	traits, err := parseTraits(cmd.StringSlice("trait"))
	if err != nil {
		return err
	}
	_, err = projectService(cmd).Create(cmd.Args().First(), cmd.StringSlice("label"), traits)
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
	writer := commandWriter(cmd)
	for _, task := range tasks {
		if status != "" && string(task.Status) != status {
			continue
		}
		if _, err := fmt.Fprintf(writer, "%s/%s\t%s\n", task.Project, task.Slug, task.Status); err != nil {
			return err
		}
	}
	return nil
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
	writer := commandWriter(cmd)
	if _, err := fmt.Fprintf(writer, "%s/%s\t%s\n", loaded.Project, loaded.Slug, loaded.Status); err != nil {
		return err
	}

	mrArtifact, err := runtimeStore(cmd).LoadArtifact(project, task, "mr")
	if err == nil {
		if value := mrArtifact.Data["status"]; value != "" {
			if _, err := fmt.Fprintf(writer, "MR Status: %s\n", value); err != nil {
				return err
			}
		}
		if value := mrArtifact.Data["url"]; value != "" {
			if _, err := fmt.Fprintf(writer, "MR URL: %s\n", value); err != nil {
				return err
			}
		}
		if value := mrArtifact.Data["target_branch"]; value != "" {
			if _, err := fmt.Fprintf(writer, "Target Branch: %s\n", value); err != nil {
				return err
			}
		}
		if value := mrArtifact.Data["title"]; value != "" {
			if _, err := fmt.Fprintf(writer, "Title: %s\n", value); err != nil {
				return err
			}
		}
	}

	return nil
}

func tasksCreateAction(_ context.Context, cmd *urfavecli.Command) error {
	traits, err := parseTraits(cmd.StringSlice("trait"))
	if err != nil {
		return err
	}
	_, err = taskService(cmd).Create(cmd.String("project"), cmd.String("repo"), cmd.String("name"), cmd.StringSlice("label"), traits)
	return err
}

func tasksBlockAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	task, err := runtimeStore(cmd).LoadTask(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	reason := "blocked by command"
	task.Status = model.TaskStatusBlocked
	task.Blocker = &reason
	task.LastOp = model.OperationState{Cmd: "tasks.block", OK: true}
	return runtimeStore(cmd).SaveTask(task)
}

func tasksUnblockAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	task, err := runtimeStore(cmd).LoadTask(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	task.Status = model.TaskStatusActive
	task.Blocker = nil
	task.LastOp = model.OperationState{Cmd: "tasks.unblock", OK: true}
	return runtimeStore(cmd).SaveTask(task)
}

func tasksDoneAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	_, err = taskService(cmd).Done(projectSlug, taskSlug)
	return err
}

func tasksCancelAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	task, err := runtimeStore(cmd).LoadTask(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	task.Status = model.TaskStatusCancelled
	task.LastOp = model.OperationState{Cmd: "tasks.cancel", OK: true}
	return runtimeStore(cmd).SaveTask(task)
}

func tasksCleanupAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug, taskSlug, err := splitTaskID(cmd.Args().First())
	if err != nil {
		return err
	}
	_, err = taskService(cmd).Cleanup(projectSlug, taskSlug)
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

func parseTraits(values []string) (map[string]string, error) {
	traits := map[string]string{}
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("trait must be key=value")
		}
		traits[parts[0]] = parts[1]
	}
	return traits, nil
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
