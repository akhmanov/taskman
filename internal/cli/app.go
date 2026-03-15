package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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
			{Name: "list", Usage: "List projects in canonical status order", Flags: []urfavecli.Flag{&urfavecli.StringFlag{Name: "status", Usage: "Include only these statuses (comma-separated)"}, &urfavecli.StringFlag{Name: "exclude-status", Usage: "Hide these statuses (comma-separated)"}, &urfavecli.BoolFlag{Name: "active", Usage: "Shortcut for --exclude-status done,canceled"}}, Action: projectsListAction},
			{Name: "show", Usage: "Show a grouped project workboard", ArgsUsage: "<project>", Flags: []urfavecli.Flag{&urfavecli.BoolFlag{Name: "all", Usage: "Expand done and canceled task groups"}}, Action: projectsShowAction},
			{Name: "add", Usage: "Add a backlog project", ArgsUsage: "<project>", Flags: metadataFlags(), Action: projectsAddAction},
			{Name: "update", Usage: "Update project metadata", ArgsUsage: "<project>", Flags: updateFlags(), Action: projectsUpdateAction},
			{Name: "brief", Usage: "Manage project brief", Commands: []*urfavecli.Command{
				{Name: "show", Usage: "Show project brief", ArgsUsage: "<project>", Action: projectsBriefShowAction},
				{Name: "set", Usage: "Set project brief", ArgsUsage: "<project>", Flags: briefSetFlags(), Action: projectsBriefSetAction},
				{Name: "init", Usage: "Initialize project brief template", ArgsUsage: "<project>", Flags: []urfavecli.Flag{&urfavecli.BoolFlag{Name: "force"}}, Action: projectsBriefInitAction},
				{Name: "edit", Usage: "Edit project brief in $EDITOR", ArgsUsage: "<project>", Action: projectsBriefEditAction},
			}},
			{Name: "event", Usage: "Manage project events", Commands: []*urfavecli.Command{
				{Name: "add", Usage: "Add project event", ArgsUsage: "<project>", Flags: payloadEventFlags(), Action: projectsEventAddAction},
				{Name: "list", Usage: "List project events", ArgsUsage: "<project>", Action: projectsEventListAction},
			}},
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
			{Name: "list", Usage: "List tasks in canonical status order", Flags: []urfavecli.Flag{projectFlag(), &urfavecli.StringFlag{Name: "status", Usage: "Include only these statuses (comma-separated)"}, &urfavecli.StringFlag{Name: "exclude-status", Usage: "Hide these statuses (comma-separated)"}, &urfavecli.BoolFlag{Name: "active", Usage: "Shortcut for --exclude-status done,canceled"}}, Action: tasksListAction},
			{Name: "show", Usage: "Show a task", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: tasksShowAction},
			{Name: "add", Usage: "Add a backlog task", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, metadataFlags()...), Action: tasksAddAction},
			{Name: "update", Usage: "Update task metadata", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, updateFlags()...), Action: tasksUpdateAction},
			{Name: "brief", Usage: "Manage task brief", Commands: []*urfavecli.Command{
				{Name: "show", Usage: "Show task brief", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: tasksBriefShowAction},
				{Name: "set", Usage: "Set task brief", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, briefSetFlags()...), Action: tasksBriefSetAction},
				{Name: "init", Usage: "Initialize task brief template", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag(), &urfavecli.BoolFlag{Name: "force"}}, Action: tasksBriefInitAction},
				{Name: "edit", Usage: "Edit task brief in $EDITOR", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: tasksBriefEditAction},
			}},
			{Name: "event", Usage: "Manage task events", Commands: []*urfavecli.Command{
				{Name: "add", Usage: "Add task event", ArgsUsage: "<task>", Flags: append([]urfavecli.Flag{projectFlag()}, payloadEventFlags()...), Action: tasksEventAddAction},
				{Name: "list", Usage: "List task events", ArgsUsage: "<task>", Flags: []urfavecli.Flag{projectFlag()}, Action: tasksEventListAction},
			}},
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
		Name:        "taskman",
		Usage:       "Opinionated task and project workflow for agent-first development",
		Description: "Built-in lifecycle: backlog -> planned -> in_progress -> paused -> done|canceled. Use list to scan and show to inspect grouped work.",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{Name: "root", Value: "../tasks", Usage: "Path to the runtime root", Sources: urfavecli.EnvVars("TASKMAN_ROOT")},
		},
		Commands: []*urfavecli.Command{
			{Name: "init", Usage: "Create a minimal taskman.yaml", Flags: []urfavecli.Flag{&urfavecli.BoolFlag{Name: "force", Usage: "Overwrite an existing taskman.yaml"}}, Action: initAction},
			project,
			task,
		},
	}
}

func runtimeStore(cmd *urfavecli.Command) store.Store {
	return store.New(cmd.String("root"))
}

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
	} else {
		if err := runtimeStore(cmd).InitConfig(); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(os.Stdout, "Initialized: %s\n", path)
	return err
}

func projectsListAction(_ context.Context, cmd *urfavecli.Command) error {
	if _, err := runtimeStore(cmd).LoadConfig(); err != nil {
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
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s\n", project.Status, project.Slug); err != nil {
			return err
		}
	}
	return nil
}

func tasksListAction(_ context.Context, cmd *urfavecli.Command) error {
	if _, err := runtimeStore(cmd).LoadConfig(); err != nil {
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
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s/%s\n", task.Status, task.Project, task.Slug); err != nil {
			return err
		}
	}
	return nil
}

func projectsShowAction(_ context.Context, cmd *urfavecli.Command) error {
	if _, err := runtimeStore(cmd).LoadConfig(); err != nil {
		return err
	}
	projectSlug := cmd.Args().First()
	s := runtimeStore(cmd)
	project, err := s.LoadProject(projectSlug)
	if err != nil {
		return err
	}
	tasks, err := s.ListTasks(projectSlug)
	if err != nil {
		return err
	}
	sortTasks(tasks)
	brief, _ := s.LoadProjectBrief(projectSlug)
	all := cmd.Bool("all")
	if _, err := fmt.Fprintf(os.Stdout, "Project: %s\n", project.Slug); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Status: %s\n", project.Status); err != nil {
		return err
	}
	if first := firstMeaningfulBriefLine(brief); first != "" {
		if _, err := fmt.Fprintf(os.Stdout, "Overview: %s\n", first); err != nil {
			return err
		}
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
	hasCollapsedTerminal := false
	for _, status := range model.CanonicalStatusOrder() {
		entries := buckets[status]
		if len(entries) == 0 {
			continue
		}
		if model.IsTerminalStatus(status) && !all {
			hasCollapsedTerminal = true
			if _, err := fmt.Fprintf(os.Stdout, "%s: %d %s hidden\n", status, len(entries), pluralize(len(entries), "task", "tasks")); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(os.Stdout, "%s:\n", status); err != nil {
			return err
		}
		for _, task := range entries {
			line := formatGroupedTaskLine(task, all)
			if _, err := fmt.Fprintf(os.Stdout, "- %s\n", line); err != nil {
				return err
			}
		}
	}
	if hasCollapsedTerminal {
		if _, err := fmt.Fprintln(os.Stdout, "Use --all to expand done and canceled work."); err != nil {
			return err
		}
	}
	return nil
}

func tasksShowAction(_ context.Context, cmd *urfavecli.Command) error {
	if _, err := runtimeStore(cmd).LoadConfig(); err != nil {
		return err
	}
	projectSlug := cmd.String("project")
	taskSlug := cmd.Args().First()
	s := runtimeStore(cmd)
	task, err := s.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	brief, _ := s.LoadTaskBrief(projectSlug, taskSlug)
	transitions, _ := s.ListTaskTransitions(projectSlug, taskSlug)
	events, _ := s.ListTaskEvents(projectSlug, taskSlug)
	if _, err := fmt.Fprintf(os.Stdout, "Task: %s/%s\n", projectSlug, task.Slug); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Status: %s\n", task.Status); err != nil {
		return err
	}
	for _, line := range statusDetailLines(task.StatusDetail) {
		if _, err := fmt.Fprintln(os.Stdout, line); err != nil {
			return err
		}
	}
	if first := firstMeaningfulBriefLine(brief); first != "" {
		if _, err := fmt.Fprintf(os.Stdout, "Overview: %s\n", first); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(os.Stdout, "Transitions: %d\n", len(transitions)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(os.Stdout, "Events: %d\n", len(events)); err != nil {
		return err
	}
	return nil
}

func projectsAddAction(_ context.Context, cmd *urfavecli.Command) error {
	vars, err := parseVars(cmd.StringSlice("var"))
	if err != nil {
		return err
	}
	project, err := projectService(cmd).Create(cmd.Args().First(), cmd.StringSlice("label"), vars)
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
	task, err := taskService(cmd).Create(cmd.String("project"), cmd.Args().First(), cmd.StringSlice("label"), vars)
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

func projectTransitionAction(verb string) urfavecli.ActionFunc {
	return func(_ context.Context, cmd *urfavecli.Command) error {
		input := transitionInputFromFlags(cmd)
		project, warnings, err := projectService(cmd).Transition(cmd.Args().First(), verb, input)
		if err != nil {
			return err
		}
		return renderProjectSummary(project, warnings)
	}
}

func taskTransitionAction(verb string) urfavecli.ActionFunc {
	return func(_ context.Context, cmd *urfavecli.Command) error {
		input := transitionInputFromFlags(cmd)
		task, warnings, err := taskService(cmd).Transition(cmd.String("project"), cmd.Args().First(), verb, input)
		if err != nil {
			return err
		}
		return renderTaskSummary(task, warnings)
	}
}

func projectsBriefShowAction(_ context.Context, cmd *urfavecli.Command) error {
	brief, err := projectService(cmd).GetBrief(cmd.Args().First())
	if err != nil {
		return err
	}
	_, err = io.WriteString(os.Stdout, brief)
	return err
}

func tasksBriefShowAction(_ context.Context, cmd *urfavecli.Command) error {
	brief, err := taskService(cmd).GetBrief(cmd.String("project"), cmd.Args().First())
	if err != nil {
		return err
	}
	_, err = io.WriteString(os.Stdout, brief)
	return err
}

func projectsBriefSetAction(_ context.Context, cmd *urfavecli.Command) error {
	brief, _, err := resolveBriefContent(cmd)
	if err != nil {
		return err
	}
	return projectService(cmd).SetBrief(cmd.Args().First(), brief)
}

func tasksBriefSetAction(_ context.Context, cmd *urfavecli.Command) error {
	brief, _, err := resolveBriefContent(cmd)
	if err != nil {
		return err
	}
	return taskService(cmd).SetBrief(cmd.String("project"), cmd.Args().First(), brief)
}

func projectsBriefInitAction(_ context.Context, cmd *urfavecli.Command) error {
	projectSlug := cmd.Args().First()
	current, err := projectService(cmd).GetBrief(projectSlug)
	if err != nil {
		return err
	}
	if strings.TrimSpace(current) != "" && !cmd.Bool("force") {
		return fmt.Errorf("project brief already exists; use --force to overwrite")
	}
	return projectService(cmd).SetBrief(projectSlug, model.ProjectBriefTemplate)
}

func tasksBriefInitAction(_ context.Context, cmd *urfavecli.Command) error {
	taskSlug := cmd.Args().First()
	current, err := taskService(cmd).GetBrief(cmd.String("project"), taskSlug)
	if err != nil {
		return err
	}
	if strings.TrimSpace(current) != "" && !cmd.Bool("force") {
		return fmt.Errorf("task brief already exists; use --force to overwrite")
	}
	return taskService(cmd).SetBrief(cmd.String("project"), taskSlug, model.TaskBriefTemplate)
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

func tasksBriefEditAction(_ context.Context, cmd *urfavecli.Command) error {
	current, err := taskService(cmd).GetBrief(cmd.String("project"), cmd.Args().First())
	if err != nil {
		return err
	}
	edited, err := editBriefContent(current, model.TaskBriefTemplate)
	if err != nil {
		return err
	}
	return taskService(cmd).SetBrief(cmd.String("project"), cmd.Args().First(), edited)
}

func projectsEventAddAction(_ context.Context, cmd *urfavecli.Command) error {
	event, err := payloadEventFromFlags(cmd)
	if err != nil {
		return err
	}
	return projectService(cmd).AddEvent(cmd.Args().First(), event)
}

func tasksEventAddAction(_ context.Context, cmd *urfavecli.Command) error {
	event, err := payloadEventFromFlags(cmd)
	if err != nil {
		return err
	}
	return taskService(cmd).AddEvent(cmd.String("project"), cmd.Args().First(), event)
}

func projectsEventListAction(_ context.Context, cmd *urfavecli.Command) error {
	events, err := projectService(cmd).GetEvents(cmd.Args().First())
	if err != nil {
		return err
	}
	for _, event := range events {
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", event.ID, event.Type, event.Summary); err != nil {
			return err
		}
	}
	return nil
}

func tasksEventListAction(_ context.Context, cmd *urfavecli.Command) error {
	events, err := taskService(cmd).GetEvents(cmd.String("project"), cmd.Args().First())
	if err != nil {
		return err
	}
	for _, event := range events {
		if _, err := fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", event.ID, event.Type, event.Summary); err != nil {
			return err
		}
	}
	return nil
}

func metadataFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringSliceFlag{Name: "label"},
		&urfavecli.StringSliceFlag{Name: "var"},
	}
}

func updateFlags() []urfavecli.Flag {
	flags := metadataFlags()
	return append(flags, &urfavecli.StringSliceFlag{Name: "unset-var"})
}

func projectFlag() urfavecli.Flag {
	return &urfavecli.StringFlag{Name: "project", Aliases: []string{"p"}, Usage: "Project slug for the task command", Required: true}
}

func pauseFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "reason-type", Usage: "Short machine-friendly pause code"},
		&urfavecli.StringFlag{Name: "reason", Usage: "Human explanation for the pause"},
		&urfavecli.StringFlag{Name: "resume-when", Usage: "Condition or date for resuming work"},
	}
}

func completeFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "summary", Usage: "Short outcome summary for a done item"},
	}
}

func cancelFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "reason-type", Usage: "Short machine-friendly cancel code"},
		&urfavecli.StringFlag{Name: "reason", Usage: "Human explanation for the cancellation"},
	}
}

func briefSetFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "content"},
		&urfavecli.StringFlag{Name: "file"},
	}
}

func payloadEventFlags() []urfavecli.Flag {
	return []urfavecli.Flag{
		&urfavecli.StringFlag{Name: "id", Required: true},
		&urfavecli.StringFlag{Name: "at", Required: true},
		&urfavecli.StringFlag{Name: "type", Required: true},
		&urfavecli.StringFlag{Name: "summary", Required: true},
		&urfavecli.StringFlag{Name: "details"},
		&urfavecli.StringFlag{Name: "actor", Required: true},
		&urfavecli.StringSliceFlag{Name: "ref"},
		&urfavecli.StringFlag{Name: "rationale"},
		&urfavecli.StringFlag{Name: "impact"},
		&urfavecli.StringFlag{Name: "event-status"},
	}
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

func optionalLabels(cmd *urfavecli.Command) []string {
	if !cmd.IsSet("label") {
		return nil
	}
	return cmd.StringSlice("label")
}

func transitionInputFromFlags(cmd *urfavecli.Command) lifecycle.TransitionInput {
	return lifecycle.TransitionInput{
		Actor:      "taskman",
		ReasonType: cmd.String("reason-type"),
		Reason:     cmd.String("reason"),
		ResumeWhen: cmd.String("resume-when"),
		Summary:    cmd.String("summary"),
	}
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

func filterTasks(tasks []model.TaskState, include, exclude []model.Status) []model.TaskState {
	includeSet := makeStatusSet(include)
	excludeSet := makeStatusSet(exclude)
	filtered := make([]model.TaskState, 0, len(tasks))
	for _, task := range tasks {
		if len(includeSet) > 0 {
			if _, ok := includeSet[task.Status]; !ok {
				continue
			}
		}
		if _, ok := excludeSet[task.Status]; ok {
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered
}

func filterProjects(projects []model.ProjectState, include, exclude []model.Status) []model.ProjectState {
	includeSet := makeStatusSet(include)
	excludeSet := makeStatusSet(exclude)
	filtered := make([]model.ProjectState, 0, len(projects))
	for _, project := range projects {
		if len(includeSet) > 0 {
			if _, ok := includeSet[project.Status]; !ok {
				continue
			}
		}
		if _, ok := excludeSet[project.Status]; ok {
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

func sortTasks(tasks []model.TaskState) {
	sort.Slice(tasks, func(i, j int) bool {
		left := model.StatusSortIndex(tasks[i].Status)
		right := model.StatusSortIndex(tasks[j].Status)
		if left != right {
			return left < right
		}
		if tasks[i].UpdatedAt != tasks[j].UpdatedAt {
			return tasks[i].UpdatedAt > tasks[j].UpdatedAt
		}
		return tasks[i].Slug < tasks[j].Slug
	})
}

func sortProjects(projects []model.ProjectState) {
	sort.Slice(projects, func(i, j int) bool {
		left := model.StatusSortIndex(projects[i].Status)
		right := model.StatusSortIndex(projects[j].Status)
		if left != right {
			return left < right
		}
		if projects[i].UpdatedAt != projects[j].UpdatedAt {
			return projects[i].UpdatedAt > projects[j].UpdatedAt
		}
		return projects[i].Slug < projects[j].Slug
	})
}

func taskBuckets(tasks []model.TaskState) map[model.Status][]model.TaskState {
	buckets := map[model.Status][]model.TaskState{}
	for _, task := range tasks {
		buckets[task.Status] = append(buckets[task.Status], task)
	}
	return buckets
}

func formatGroupedTaskLine(task model.TaskState, all bool) string {
	parts := []string{task.Slug}
	if detail := task.StatusDetail; task.Status == model.StatusPaused {
		if detail.Reason != "" {
			parts = append(parts, detail.Reason)
		}
	} else if all && (task.Status == model.StatusDone || task.Status == model.StatusCanceled) {
		if detail.Summary != "" {
			parts = append(parts, detail.Summary)
		}
		if detail.Reason != "" {
			parts = append(parts, detail.Reason)
		}
	}
	return strings.Join(parts, " - ")
}

func projectWarnings(project model.ProjectState, tasks []model.TaskState) []string {
	warnings := []string{}
	inProgress := 0
	nonTerminal := 0
	for _, task := range tasks {
		if task.Status == model.StatusInProgress {
			inProgress++
		}
		if !model.IsTerminalStatus(task.Status) {
			nonTerminal++
		}
	}
	if project.Status == model.StatusPaused && inProgress > 0 {
		warnings = append(warnings, fmt.Sprintf("project is paused but has %d in_progress %s", inProgress, pluralize(inProgress, "task", "tasks")))
	}
	if project.Status == model.StatusDone && nonTerminal > 0 {
		warnings = append(warnings, fmt.Sprintf("project is done but has %d non-terminal %s", nonTerminal, pluralize(nonTerminal, "task", "tasks")))
	}
	if project.Status == model.StatusCanceled && nonTerminal > 0 {
		warnings = append(warnings, fmt.Sprintf("project is canceled but has %d non-terminal %s", nonTerminal, pluralize(nonTerminal, "task", "tasks")))
	}
	return warnings
}

func renderProjectSummary(project model.ProjectState, warnings []string) error {
	if _, err := fmt.Fprintf(os.Stdout, "Project %s is now %s.\n", project.Slug, project.Status); err != nil {
		return err
	}
	for _, line := range statusDetailLines(project.StatusDetail) {
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

func renderTaskSummary(task model.TaskState, warnings []string) error {
	if _, err := fmt.Fprintf(os.Stdout, "Task %s/%s is now %s.\n", task.Project, task.Slug, task.Status); err != nil {
		return err
	}
	for _, line := range statusDetailLines(task.StatusDetail) {
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

func firstMeaningfulBriefLine(brief string) string {
	for _, line := range strings.Split(brief, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func resolveBriefContent(cmd *urfavecli.Command) (string, string, error) {
	content := cmd.String("content")
	file := cmd.String("file")
	if strings.TrimSpace(content) == "" && strings.TrimSpace(file) == "" {
		return "", "", fmt.Errorf("provide --content or --file")
	}
	if strings.TrimSpace(content) != "" && strings.TrimSpace(file) != "" {
		return "", "", fmt.Errorf("provide either --content or --file, not both")
	}
	if strings.TrimSpace(content) != "" {
		return content, "content", nil
	}
	if file == "-" {
		data, err := io.ReadAll(os.Stdin)
		return string(data), "stdin", err
	}
	data, err := os.ReadFile(file)
	return string(data), file, err
}

func editBriefContent(current, fallback string) (string, error) {
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		return "", fmt.Errorf("EDITOR or VISUAL must be set")
	}
	initial := current
	if strings.TrimSpace(initial) == "" {
		initial = fallback
	}
	file, err := os.CreateTemp("", "taskman-brief-*.md")
	if err != nil {
		return "", err
	}
	tmpPath := file.Name()
	defer os.Remove(tmpPath)
	if err := file.Close(); err != nil {
		return "", err
	}
	if err := os.WriteFile(tmpPath, []byte(initial), 0o600); err != nil {
		return "", err
	}
	editorCommand := exec.Command(editor, tmpPath)
	editorCommand.Stdin = os.Stdin
	editorCommand.Stdout = os.Stdout
	editorCommand.Stderr = os.Stderr
	if err := editorCommand.Run(); err != nil {
		return "", err
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func payloadEventFromFlags(cmd *urfavecli.Command) (model.PayloadEvent, error) {
	event := model.PayloadEvent{
		ID:        cmd.String("id"),
		At:        cmd.String("at"),
		Type:      model.PayloadEventType(cmd.String("type")),
		Summary:   cmd.String("summary"),
		Details:   cmd.String("details"),
		Actor:     cmd.String("actor"),
		Refs:      cmd.StringSlice("ref"),
		Rationale: cmd.String("rationale"),
		Impact:    cmd.String("impact"),
		Status:    cmd.String("event-status"),
	}
	return event, nil
}
