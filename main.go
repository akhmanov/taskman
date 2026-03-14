package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/akhmanov/taskman/internal/cli"
	urfavecli "github.com/urfave/cli/v3"
)

func main() {
	os.Exit(execute(context.Background(), os.Args, os.Stdout, os.Stderr))
}

func execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cmd := cli.BuildApp()
	cmd.Writer = stdout
	cmd.ErrWriter = stderr
	cmd.ExitErrHandler = func(context.Context, *urfavecli.Command, error) {}
	if err := cmd.Run(ctx, args); err != nil {
		_, _ = fmt.Fprintln(stderr, rewriteCLIError(args, err))
		return 1
	}
	return 0
}

func rewriteCLIError(args []string, err error) error {
	message := err.Error()
	if strings.Contains(message, "No help topic for") && isLegacyProjectsCommand(args) {
		return fmt.Errorf("legacy `projects` command was removed; use `taskman project ...`")
	}
	if strings.Contains(message, "No help topic for") && isLegacyTasksCommand(args) {
		return fmt.Errorf("legacy `tasks` command was removed; use `taskman task ...`")
	}
	if strings.Contains(message, "No help topic for") && isLegacyTaskTransitionCommand(args) {
		return fmt.Errorf("legacy `task transition` command was removed; use `taskman task <start|block|unblock|complete|cancel|close> <task> -p <project>`")
	}
	if strings.Contains(message, "No help topic for") && isProjectListMisuse(args) {
		return fmt.Errorf("project list does not accept a project id; use `taskman project show <project>`")
	}
	if strings.Contains(message, "No help topic for") && isTaskListMisuse(args) {
		return fmt.Errorf("task list does not accept a task id; use `taskman task show <task> -p <project>`")
	}
	return err
}

func isProjectListMisuse(args []string) bool {
	return matchesCommand(args, "project", "list")
}

func isTaskListMisuse(args []string) bool {
	return matchesCommand(args, "task", "list")
}

func isLegacyProjectsCommand(args []string) bool {
	return len(args) >= 2 && args[1] == "projects"
}

func isLegacyTasksCommand(args []string) bool {
	return len(args) >= 2 && args[1] == "tasks"
}

func isLegacyTaskTransitionCommand(args []string) bool {
	return len(args) >= 3 && args[1] == "task" && args[2] == "transition"
}

func matchesCommand(args []string, resource, verb string) bool {
	if len(args) < 3 {
		return false
	}
	return args[1] == resource && args[2] == verb
}
