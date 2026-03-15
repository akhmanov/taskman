package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akhmanov/taskman/internal/lifecycle"
	"github.com/akhmanov/taskman/internal/model"
	"github.com/akhmanov/taskman/internal/steps"
	"github.com/akhmanov/taskman/internal/store"
	urfavecli "github.com/urfave/cli/v3"
)

func BuildApp() *urfavecli.Command {
	project := &urfavecli.Command{
		Name:  "project",
		Usage: "Manage projects",
		Commands: []*urfavecli.Command{
			{Name: "list", Usage: "List projects in canonical status order", Flags: listFlags(false), Action: projectsListAction},
			{Name: "show", Usage: "Show a project workboard", ArgsUsage: "<project>", Action: projectsShowAction},
			{Name: "add", Usage: "Create a project manifest", ArgsUsage: "<project>", Flags: createFlags(), Action: projectsAddAction},
			{Name: "update", Usage: "Append project metadata changes", ArgsUsage: "<project>", Flags: updateFlags(), Action: projectsUpdateAction},
			{Name: "message", Usage: "Manage project messages", Commands: []*urfavecli.Command{{Name: "add", Usage: "Add project message", ArgsUsage: "<project>", Flags: messageFlags(), Action: projectsMessageAddAction}, {Name: "list", Usage: "List project messages", ArgsUsage: "<project>", Action: projectsMessageListAction}}},
			{Name: "transition", Usage: "Inspect project transitions", Commands: []*urfavecli.Command{{Name: "list", Usage: "List project transitions", ArgsUsage: "<project>", Action: projectsTransitionListAction}}},
			{Name: "plan", Usage: "Move project to planned", ArgsUsage: "<project>", Action: projectTransitionAction("plan")},
			{Name: "start", Usage: "Move project to in_progress", ArgsUsage: "<project>", Action: projectTransitionAction("start")},
			{Name: "pause", Usage: "Pause a project", ArgsUsage: "<project>", Flags: pauseFlags(), Action: projectTransitionAction("pause")},
			{Name: "resume", Usage: "Resume a paused project", ArgsUsage: "<project>", Action: projectTransitionAction("resume")},
			{Name: "complete", Usage: "Complete a project", ArgsUsage: "<project>", Flags: completeFlags(), Action: projectTransitionAction("complete")},
			{Name: "cancel", Usage: "Cancel a project", ArgsUsage: "<project>", Flags: cancelFlags(), Action: projectTransitionAction("cancel")},
			{Name: "reopen", Usage: "Reopen a terminal project", ArgsUsage: "<project>", Action: projectTransitionAction("reopen")},
		},
	}

	task := &urfavecli.Command{
		Name:  "task",
		Usage: "Manage tasks",
		Commands: []*urfavecli.Command{
			{Name: "list", Usage: "List tasks in canonical status order", Flags: append([]urfavecli.Flag{projectFlag()}, listFlags(true)...), Action: tasksListAction},
			{Name: "show", Usage: "Show a task card", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: tasksShowAction},
			{Name: "add", Usage: "Create a task manifest", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, createFlags()...), Action: tasksAddAction},
			{Name: "update", Usage: "Append task metadata changes", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, updateFlags()...), Action: tasksUpdateAction},
			{Name: "message", Usage: "Manage task messages", Commands: []*urfavecli.Command{{Name: "add", Usage: "Add task message", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, messageFlags()...), Action: tasksMessageAddAction}, {Name: "list", Usage: "List task messages", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: tasksMessageListAction}}},
			{Name: "transition", Usage: "Inspect task transitions", Commands: []*urfavecli.Command{{Name: "list", Usage: "List task transitions", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: tasksTransitionListAction}}},
			{Name: "plan", Usage: "Move task to planned", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: taskTransitionAction("plan")},
			{Name: "start", Usage: "Move task to in_progress", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: taskTransitionAction("start")},
			{Name: "pause", Usage: "Pause a task", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, pauseFlags()...), Action: taskTransitionAction("pause")},
			{Name: "resume", Usage: "Resume a paused task", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: taskTransitionAction("resume")},
			{Name: "complete", Usage: "Complete a task", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, completeFlags()...), Action: taskTransitionAction("complete")},
			{Name: "cancel", Usage: "Cancel a task", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, cancelFlags()...), Action: taskTransitionAction("cancel")},
			{Name: "reopen", Usage: "Reopen a terminal task", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: taskTransitionAction("reopen")},
		},
	}

	return &urfavecli.Command{
		Name:     "taskman",
		Usage:    "Append-only project and task workflow for agent-first development",
		Flags:    []urfavecli.Flag{&urfavecli.StringFlag{Name: "root", Value: ".", Usage: "Path to the runtime root", Sources: urfavecli.EnvVars("TASKMAN_ROOT")}},
		Commands: []*urfavecli.Command{{Name: "init", Usage: "Create an optional taskman.yaml overlay", Flags: []urfavecli.Flag{&urfavecli.BoolFlag{Name: "force", Usage: "Overwrite an existing taskman.yaml"}}, Action: initAction}, project, task},
	}
}

func runtimeStore(cmd *urfavecli.Command) store.Store { return store.New(cmd.String("root")) }
func projectService(cmd *urfavecli.Command) lifecycle.ProjectService {
	root := cmd.String("root")
	return lifecycle.NewProjectService(store.New(root), steps.New(root))
}
func taskService(cmd *urfavecli.Command) lifecycle.TaskService {
	root := cmd.String("root")
	return lifecycle.NewTaskService(store.New(root), steps.New(root))
}

func initAction(_ context.Context, cmd *urfavecli.Command) error {
	path := filepath.Join(cmd.String("root"), "taskman.yaml")
	if _, err := os.Stat(path); err == nil && !cmd.Bool("force") {
		return fmt.Errorf("taskman.yaml already exists; use --force to overwrite")
	}
	if cmd.Bool("force") {
		if err := os.MkdirAll(cmd.String("root"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(model.DefaultConfigYAML), 0o644); err != nil {
			return err
		}
	} else if err := runtimeStore(cmd).InitConfig(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(os.Stdout, "Initialized: %s\n", path)
	return err
}

func projectsListAction(_ context.Context, cmd *urfavecli.Command) error {
	if _, _, err := runtimeStore(cmd).LoadOptionalConfig(); err != nil {
		return err
	}
	projects, err := runtimeStore(cmd).ListProjects()
	if err != nil {
		return err
	}
	include, exclude, err := listStatusFilters(cmd)
	if err != nil {
		return err
	}
	projects = filterProjects(projects, include, exclude)
	sortProjects(projects)
	for _, project := range projects {
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s\n", project.State.Status, project.Manifest.Slug); err != nil {
			return err
		}
	}
	return nil
}

func tasksListAction(_ context.Context, cmd *urfavecli.Command) error {
	if _, _, err := runtimeStore(cmd).LoadOptionalConfig(); err != nil {
		return err
	}
	tasks, err := runtimeStore(cmd).ListTasks(cmd.String("project"))
	if err != nil {
		return err
	}
	include, exclude, err := listStatusFilters(cmd)
	if err != nil {
		return err
	}
	tasks = filterTasks(tasks, include, exclude)
	sortTasks(tasks)
	for _, task := range tasks {
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s/%s\n", task.State.Status, task.Manifest.ProjectSlug, task.Manifest.Slug); err != nil {
			return err
		}
	}
	return nil
}

func projectsShowAction(_ context.Context, cmd *urfavecli.Command) error {
	if _, _, err := runtimeStore(cmd).LoadOptionalConfig(); err != nil {
		return err
	}
	project, err := runtimeStore(cmd).LoadProject(cmd.Args().First())
	if err != nil {
		return err
	}
	tasks, err := runtimeStore(cmd).ListTasks(project.Manifest.Slug)
	if err != nil {
		return err
	}
	messages, err := projectService(cmd).GetMessages(project.Manifest.Slug)
	if err != nil {
		return err
	}
	sortTasks(tasks)
	if _, err := fmt.Fprintf(os.Stdout, "Project: %s\n", project.Manifest.Slug); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Name: %s\n", project.Manifest.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Description: %s\n", project.Manifest.Description); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Status: %s\n", project.State.Status); err != nil {
		return err
	}
	for _, warning := range projectWarnings(project, tasks) {
		if _, err := fmt.Fprintf(os.Stdout, "Needs attention: %s\n", warning); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(os.Stdout, "Workboard:"); err != nil {
		return err
	}
	buckets := taskBuckets(tasks)
	for _, status := range model.CanonicalStatusOrder() {
		entries := buckets[status]
		if len(entries) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(os.Stdout, "%s:\n", status); err != nil {
			return err
		}
		for _, task := range entries {
			if _, err := fmt.Fprintf(os.Stdout, "- %s\n", task.Manifest.Slug); err != nil {
				return err
			}
		}
	}
	if len(messages) > 0 {
		if _, err := fmt.Fprintln(os.Stdout, "Recent Project Messages:"); err != nil {
			return err
		}
		for _, message := range tailEvents(messages, 3) {
			if _, err := fmt.Fprintf(os.Stdout, "- %s: %s\n", message.Message.Kind, message.Message.Body); err != nil {
				return err
			}
		}
	}
	return nil
}

func tasksShowAction(_ context.Context, cmd *urfavecli.Command) error {
	if _, _, err := runtimeStore(cmd).LoadOptionalConfig(); err != nil {
		return err
	}
	task, err := runtimeStore(cmd).LoadTask(cmd.String("project"), cmd.Args().First())
	if err != nil {
		return err
	}
	project, err := runtimeStore(cmd).LoadProject(cmd.String("project"))
	if err != nil {
		return err
	}
	transitions, err := taskService(cmd).GetTransitions(cmd.String("project"), cmd.Args().First())
	if err != nil {
		return err
	}
	messages, err := taskService(cmd).GetMessages(cmd.String("project"), cmd.Args().First())
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Task: %s/%s\n", task.Manifest.ProjectSlug, task.Manifest.Slug); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Name: %s\n", task.Manifest.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Description: %s\n", task.Manifest.Description); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Status: %s\n", task.State.Status); err != nil {
		return err
	}
	if len(task.State.Labels) > 0 {
		if _, err := fmt.Fprintf(os.Stdout, "Labels: %s\n", strings.Join(task.State.Labels, ", ")); err != nil {
			return err
		}
	}
	if len(task.State.Vars) > 0 {
		if _, err := fmt.Fprintf(os.Stdout, "Vars: %s\n", formatVars(task.State.Vars)); err != nil {
			return err
		}
	}
	for _, line := range statusDetailLines(task.State.StatusDetail) {
		if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
			return err
		}
	}
	allowed := allowedTaskTransitionVerbs(task.State, project.State.Status)
	if _, err := fmt.Fprintf(os.Stdout, "Allowed Next: %s\n", strings.Join(allowed, ", ")); err != nil {
		return err
	}
	if task.State.HasConflict() {
		if _, err := fmt.Fprintf(os.Stdout, "Conflict: unresolved heads %s\n", strings.Join(task.State.UnresolvedHead, ", ")); err != nil {
			return err
		}
	}
	if len(transitions) > 0 {
		last := transitions[len(transitions)-1]
		if _, err := fmt.Fprintf(os.Stdout, "Transitions: %d\n", len(transitions)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(os.Stdout, "Last Transition: %s -> %s via %s\n", last.Transition.From, last.Transition.To, last.Transition.Verb); err != nil {
			return err
		}
	}
	if len(messages) > 0 {
		if _, err := fmt.Fprintf(os.Stdout, "Messages: %d\n", len(messages)); err != nil {
			return err
		}
		last := messages[len(messages)-1]
		if _, err := fmt.Fprintf(os.Stdout, "Last Message: %s - %s\n", last.Message.Kind, last.Message.Body); err != nil {
			return err
		}
	}
	return nil
}

func projectsAddAction(_ context.Context, cmd *urfavecli.Command) error {
	vars, err := parseVars(cmd.StringSlice("var"))
	if err != nil {
		return err
	}
	project, err := projectService(cmd).Create(cmd.Args().First(), lifecycle.CreateInput{Name: cmd.String("name"), Description: cmd.String("description"), Labels: cmd.StringSlice("label"), Vars: vars})
	if err != nil {
		return err
	}
	return renderProjectSummary(project, nil)
}

func projectsUpdateAction(_ context.Context, cmd *urfavecli.Command) error {
	vars, err := parseVars(cmd.StringSlice("var"))
	if err != nil {
		return err
	}
	project, err := projectService(cmd).Update(cmd.Args().First(), optionalLabels(cmd), vars, cmd.StringSlice("unset-var"))
	if err != nil {
		return err
	}
	return renderProjectSummary(project, nil)
}

func tasksAddAction(_ context.Context, cmd *urfavecli.Command) error {
	vars, err := parseVars(cmd.StringSlice("var"))
	if err != nil {
		return err
	}
	task, err := taskService(cmd).Create(cmd.String("project"), cmd.Args().First(), lifecycle.CreateInput{Name: cmd.String("name"), Description: cmd.String("description"), Labels: cmd.StringSlice("label"), Vars: vars})
	if err != nil {
		return err
	}
	return renderTaskSummary(task, nil)
}

func tasksUpdateAction(_ context.Context, cmd *urfavecli.Command) error {
	vars, err := parseVars(cmd.StringSlice("var"))
	if err != nil {
		return err
	}
	task, err := taskService(cmd).Update(cmd.String("project"), cmd.Args().First(), optionalLabels(cmd), vars, cmd.StringSlice("unset-var"))
	if err != nil {
		return err
	}
	return renderTaskSummary(task, nil)
}

func projectsMessageAddAction(_ context.Context, cmd *urfavecli.Command) error {
	kind, err := parseMessageKind(cmd.String("kind"))
	if err != nil {
		return err
	}
	return projectService(cmd).AddMessage(cmd.Args().First(), lifecycle.MessageInput{Actor: cmd.String("actor"), Kind: kind, Body: cmd.String("body")})
}
func tasksMessageAddAction(_ context.Context, cmd *urfavecli.Command) error {
	kind, err := parseMessageKind(cmd.String("kind"))
	if err != nil {
		return err
	}
	return taskService(cmd).AddMessage(cmd.String("project"), cmd.Args().First(), lifecycle.MessageInput{Actor: cmd.String("actor"), Kind: kind, Body: cmd.String("body")})
}

func projectsMessageListAction(_ context.Context, cmd *urfavecli.Command) error {
	messages, err := projectService(cmd).GetMessages(cmd.Args().First())
	if err != nil {
		return err
	}
	return renderMessages(messages)
}
func tasksMessageListAction(_ context.Context, cmd *urfavecli.Command) error {
	messages, err := taskService(cmd).GetMessages(cmd.String("project"), cmd.Args().First())
	if err != nil {
		return err
	}
	return renderMessages(messages)
}

func projectsTransitionListAction(_ context.Context, cmd *urfavecli.Command) error {
	transitions, err := projectService(cmd).GetTransitions(cmd.Args().First())
	if err != nil {
		return err
	}
	return renderTransitions(transitions)
}
func tasksTransitionListAction(_ context.Context, cmd *urfavecli.Command) error {
	transitions, err := taskService(cmd).GetTransitions(cmd.String("project"), cmd.Args().First())
	if err != nil {
		return err
	}
	return renderTransitions(transitions)
}

func projectTransitionAction(verb string) urfavecli.ActionFunc {
	return func(_ context.Context, cmd *urfavecli.Command) error {
		project, warnings, err := projectService(cmd).Transition(cmd.Args().First(), verb, transitionInputFromFlags(cmd))
		if err != nil {
			return err
		}
		return renderProjectSummary(project, warnings)
	}
}
func taskTransitionAction(verb string) urfavecli.ActionFunc {
	return func(_ context.Context, cmd *urfavecli.Command) error {
		task, warnings, err := taskService(cmd).Transition(cmd.String("project"), cmd.Args().First(), verb, transitionInputFromFlags(cmd))
		if err != nil {
			return err
		}
		return renderTaskSummary(task, warnings)
	}
}

func createFlags() []urfavecli.Flag {
	return []urfavecli.Flag{&urfavecli.StringFlag{Name: "name"}, &urfavecli.StringFlag{Name: "description", Required: true}, &urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "var"}}
}
func updateFlags() []urfavecli.Flag {
	flags := []urfavecli.Flag{&urfavecli.StringSliceFlag{Name: "label"}, &urfavecli.StringSliceFlag{Name: "var"}}
	return append(flags, &urfavecli.StringSliceFlag{Name: "unset-var"})
}
func projectFlag() urfavecli.Flag {
	return &urfavecli.StringFlag{Name: "project", Aliases: []string{"p"}, Usage: "Project slug for the task command", Required: true}
}
func pauseFlags() []urfavecli.Flag {
	return []urfavecli.Flag{&urfavecli.StringFlag{Name: "reason-type"}, &urfavecli.StringFlag{Name: "reason"}, &urfavecli.StringFlag{Name: "resume-when"}}
}
func completeFlags() []urfavecli.Flag {
	return []urfavecli.Flag{&urfavecli.StringFlag{Name: "summary"}}
}
func cancelFlags() []urfavecli.Flag {
	return []urfavecli.Flag{&urfavecli.StringFlag{Name: "reason-type"}, &urfavecli.StringFlag{Name: "reason"}}
}
func messageFlags() []urfavecli.Flag {
	return []urfavecli.Flag{&urfavecli.StringFlag{Name: "actor", Value: "taskman"}, &urfavecli.StringFlag{Name: "kind", Value: string(model.MessageKindComment)}, &urfavecli.StringFlag{Name: "body", Required: true}}
}
func listFlags(projectRequired bool) []urfavecli.Flag {
	return []urfavecli.Flag{&urfavecli.StringFlag{Name: "status", Usage: "Include only these statuses (comma-separated)"}, &urfavecli.StringFlag{Name: "exclude-status", Usage: "Hide these statuses (comma-separated)"}, &urfavecli.BoolFlag{Name: "active", Usage: "Shortcut for --exclude-status done,canceled"}}
}

func parseVars(raw []string) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	vars := map[string]string{}
	for _, entry := range raw {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid --var %q, expected key=value", entry)
		}
		vars[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return vars, nil
}

func parseMessageKind(raw string) (model.MessageKind, error) {
	kind := model.MessageKind(raw)
	if !model.IsValidMessageKind(kind) {
		return "", fmt.Errorf("unknown message kind %q", raw)
	}
	return kind, nil
}

func optionalLabels(cmd *urfavecli.Command) []string {
	if !cmd.IsSet("label") {
		return nil
	}
	return cmd.StringSlice("label")
}

func transitionInputFromFlags(cmd *urfavecli.Command) lifecycle.TransitionInput {
	return lifecycle.TransitionInput{Actor: "taskman", ReasonType: cmd.String("reason-type"), Reason: cmd.String("reason"), ResumeWhen: cmd.String("resume-when"), Summary: cmd.String("summary")}
}

func listStatusFilters(cmd *urfavecli.Command) ([]model.Status, []model.Status, error) {
	include, err := model.ParseStatusCSV(cmd.String("status"))
	if err != nil {
		return nil, nil, err
	}
	exclude, err := model.ParseStatusCSV(cmd.String("exclude-status"))
	if err != nil {
		return nil, nil, err
	}
	if cmd.Bool("active") {
		exclude = append(exclude, model.StatusDone, model.StatusCanceled)
	}
	return include, exclude, nil
}

func filterTasks(tasks []model.TaskRecord, include, exclude []model.Status) []model.TaskRecord {
	includeSet := makeStatusSet(include)
	excludeSet := makeStatusSet(exclude)
	filtered := make([]model.TaskRecord, 0, len(tasks))
	for _, task := range tasks {
		if len(includeSet) > 0 {
			if _, ok := includeSet[task.State.Status]; !ok {
				continue
			}
		}
		if _, ok := excludeSet[task.State.Status]; ok {
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered
}
func filterProjects(projects []model.ProjectRecord, include, exclude []model.Status) []model.ProjectRecord {
	includeSet := makeStatusSet(include)
	excludeSet := makeStatusSet(exclude)
	filtered := make([]model.ProjectRecord, 0, len(projects))
	for _, project := range projects {
		if len(includeSet) > 0 {
			if _, ok := includeSet[project.State.Status]; !ok {
				continue
			}
		}
		if _, ok := excludeSet[project.State.Status]; ok {
			continue
		}
		filtered = append(filtered, project)
	}
	return filtered
}

func makeStatusSet(statuses []model.Status) map[model.Status]struct{} {
	set := map[model.Status]struct{}{}
	for _, status := range statuses {
		set[status] = struct{}{}
	}
	return set
}
func sortTasks(tasks []model.TaskRecord) {
	sort.Slice(tasks, func(i, j int) bool {
		left, right := model.StatusSortIndex(tasks[i].State.Status), model.StatusSortIndex(tasks[j].State.Status)
		if left != right {
			return left < right
		}
		if tasks[i].State.UpdatedAt != tasks[j].State.UpdatedAt {
			return tasks[i].State.UpdatedAt > tasks[j].State.UpdatedAt
		}
		return tasks[i].Manifest.Slug < tasks[j].Manifest.Slug
	})
}
func sortProjects(projects []model.ProjectRecord) {
	sort.Slice(projects, func(i, j int) bool {
		left, right := model.StatusSortIndex(projects[i].State.Status), model.StatusSortIndex(projects[j].State.Status)
		if left != right {
			return left < right
		}
		if projects[i].State.UpdatedAt != projects[j].State.UpdatedAt {
			return projects[i].State.UpdatedAt > projects[j].State.UpdatedAt
		}
		return projects[i].Manifest.Slug < projects[j].Manifest.Slug
	})
}
func taskBuckets(tasks []model.TaskRecord) map[model.Status][]model.TaskRecord {
	buckets := map[model.Status][]model.TaskRecord{}
	for _, task := range tasks {
		buckets[task.State.Status] = append(buckets[task.State.Status], task)
	}
	return buckets
}

func projectWarnings(project model.ProjectRecord, tasks []model.TaskRecord) []string {
	warnings := []string{}
	inProgress, nonTerminal := 0, 0
	for _, task := range tasks {
		if task.State.Status == model.StatusInProgress {
			inProgress++
		}
		if !model.IsTerminalStatus(task.State.Status) {
			nonTerminal++
		}
	}
	if project.State.Status == model.StatusPaused && inProgress > 0 {
		warnings = append(warnings, fmt.Sprintf("project is paused but has %d in_progress tasks", inProgress))
	}
	if project.State.Status == model.StatusDone && nonTerminal > 0 {
		warnings = append(warnings, fmt.Sprintf("project is done but has %d non-terminal tasks", nonTerminal))
	}
	if project.State.HasConflict() {
		warnings = append(warnings, "project has unresolved divergent heads")
	}
	return warnings
}

func renderProjectSummary(project model.ProjectRecord, warnings []string) error {
	if _, err := fmt.Fprintf(os.Stdout, "Project %s is now %s.\n", project.Manifest.Slug, project.State.Status); err != nil {
		return err
	}
	for _, line := range statusDetailLines(project.State.StatusDetail) {
		if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
			return err
		}
	}
	for _, warning := range warnings {
		if _, err := fmt.Fprintf(os.Stdout, "Needs follow-up: %s\n", warning); err != nil {
			return err
		}
	}
	return nil
}
func renderTaskSummary(task model.TaskRecord, warnings []string) error {
	if _, err := fmt.Fprintf(os.Stdout, "Task %s/%s is now %s.\n", task.Manifest.ProjectSlug, task.Manifest.Slug, task.State.Status); err != nil {
		return err
	}
	for _, line := range statusDetailLines(task.State.StatusDetail) {
		if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
			return err
		}
	}
	for _, warning := range warnings {
		if _, err := fmt.Fprintf(os.Stdout, "Needs follow-up: %s\n", warning); err != nil {
			return err
		}
	}
	return nil
}

func renderMessages(messages []model.Event) error {
	for _, message := range messages {
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", message.At, message.Message.Kind, message.Message.Body); err != nil {
			return err
		}
	}
	return nil
}
func renderTransitions(transitions []model.Event) error {
	for _, event := range transitions {
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s\t%s -> %s\n", event.At, event.Transition.Verb, event.Transition.From, event.Transition.To); err != nil {
			return err
		}
	}
	return nil
}

func statusDetailLines(detail model.StatusDetail) []string {
	var lines []string
	if detail.ReasonType != "" {
		lines = append(lines, fmt.Sprintf("Reason Type: %s", detail.ReasonType))
	}
	if detail.Reason != "" {
		lines = append(lines, fmt.Sprintf("Reason: %s", detail.Reason))
	}
	if detail.ResumeWhen != "" {
		lines = append(lines, fmt.Sprintf("Resume When: %s", detail.ResumeWhen))
	}
	if detail.Summary != "" {
		lines = append(lines, fmt.Sprintf("Outcome: %s", detail.Summary))
	}
	return lines
}
func formatVars(vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, vars[key]))
	}
	return strings.Join(parts, ", ")
}
func allowedTransitionVerbs(status model.Status) []string {
	verbs := []string{}
	for _, verb := range []string{"plan", "start", "pause", "resume", "complete", "cancel", "reopen"} {
		if _, err := lifecycle.ValidateTransitionForCLI(status, verb); err == nil {
			verbs = append(verbs, verb)
		}
	}
	return verbs
}

func allowedTaskTransitionVerbs(state model.ProjectionState, projectStatus model.Status) []string {
	if state.HasConflict() {
		return []string{}
	}
	verbs := allowedTransitionVerbs(state.Status)
	filtered := make([]string, 0, len(verbs))
	for _, verb := range verbs {
		if (verb == "start" || verb == "resume") && projectStatus == model.StatusBacklog {
			continue
		}
		if projectStatus == model.StatusDone || projectStatus == model.StatusCanceled {
			if verb != "cancel" && verb != "reopen" {
				continue
			}
		}
		filtered = append(filtered, verb)
	}
	return filtered
}

func tailEvents(events []model.Event, limit int) []model.Event {
	if len(events) <= limit {
		return events
	}
	return events[len(events)-limit:]
}
