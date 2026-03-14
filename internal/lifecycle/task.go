package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/steps"
	"github.com/assistant-wi/taskman/internal/store"
)

type TaskService struct {
	store  store.Store
	runner steps.Runner
	now    func() time.Time
}

func NewTaskService(s store.Store, runner steps.Runner) TaskService {
	return TaskService{store: s, runner: runner, now: time.Now}
}

func (s TaskService) Update(projectSlug, taskSlug string, labels []string, vars map[string]string, unsetVars []string) (model.TaskState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.TaskState{}, err
	}
	if err := cfg.ValidateVarOverrides("task", vars); err != nil {
		return model.TaskState{}, err
	}

	task, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskState{}, err
	}

	if labels != nil {
		task.Labels = append([]string{}, labels...)
	}
	if len(unsetVars) > 0 {
		if task.Vars == nil {
			task.Vars = map[string]string{}
		}
		for _, key := range unsetVars {
			delete(task.Vars, key)
		}
	}
	if vars != nil {
		task.Vars = model.MergeVars(task.Vars, vars)
	}
	if err := cfg.ValidateRequiredVars("task", task.Vars); err != nil {
		return model.TaskState{}, err
	}

	task.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	task.LastOp = model.OperationState{Cmd: "tasks.update", OK: true, At: task.UpdatedAt}
	if err := s.store.SaveTask(task); err != nil {
		return model.TaskState{}, err
	}

	return task, nil
}

func (s TaskService) GetBrief(projectSlug, taskSlug string) (string, error) {
	if _, err := s.store.LoadTask(projectSlug, taskSlug); err != nil {
		return "", err
	}
	return s.store.LoadTaskBrief(projectSlug, taskSlug)
}

func (s TaskService) SetBrief(projectSlug, taskSlug, brief string) error {
	if _, err := s.store.LoadTask(projectSlug, taskSlug); err != nil {
		return err
	}
	if err := validateTaskBrief(brief); err != nil {
		return err
	}
	return s.store.SaveTaskBrief(projectSlug, taskSlug, brief)
}

func (s TaskService) AddEvent(projectSlug, taskSlug string, event model.PayloadEvent) error {
	if _, err := s.store.LoadTask(projectSlug, taskSlug); err != nil {
		return err
	}
	if err := validatePayloadEvent(event); err != nil {
		return err
	}
	return s.store.AppendTaskEvent(projectSlug, taskSlug, event)
}

func (s TaskService) GetEvents(projectSlug, taskSlug string) ([]model.PayloadEvent, error) {
	if _, err := s.store.LoadTask(projectSlug, taskSlug); err != nil {
		return nil, err
	}
	return s.store.ListTaskEvents(projectSlug, taskSlug)
}

func (s TaskService) Create(projectSlug, name string, labels []string, vars map[string]string) (model.TaskState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.TaskState{}, err
	}
	if _, err := s.store.LoadProject(projectSlug); err != nil {
		return model.TaskState{}, err
	}
	if err := cfg.ValidateVarOverrides("task", vars); err != nil {
		return model.TaskState{}, err
	}

	mergedVars := model.MergeVars(cfg.Defaults.Task.Vars, vars)
	if err := cfg.ValidateRequiredVars("task", mergedVars); err != nil {
		return model.TaskState{}, err
	}

	slug, err := renderTaskSlug(cfg.Naming.TaskSlug, projectSlug, name, mergedVars)
	if err != nil {
		return model.TaskState{}, err
	}

	now := s.now().UTC().Format(time.RFC3339)
	task := model.TaskState{
		Version:   1,
		Slug:      slug,
		Project:   projectSlug,
		Status:    cfg.Workflow.Task.InitialStatus,
		Labels:    append([]string{}, append(cfg.Defaults.Task.Labels, labels...)...),
		Vars:      mergedVars,
		CreatedAt: now,
		UpdatedAt: now,
		Session:   model.TaskSessionState{Active: "S-001"},
		LastOp:    model.OperationState{Cmd: "tasks.create", OK: true, At: now},
	}

	if err := s.store.ScaffoldTask(task); err != nil {
		return model.TaskState{}, err
	}

	session := model.SessionState{
		Version:   1,
		ID:        "S-001",
		Task:      task.Slug,
		Status:    string(task.Status),
		CreatedAt: now,
		UpdatedAt: now,
		LastOp:    task.LastOp,
	}
	if err := s.store.SaveSession(projectSlug, slug, session); err != nil {
		return model.TaskState{}, err
	}

	if err := s.refreshProjectCounts(projectSlug); err != nil {
		return model.TaskState{}, err
	}

	return task, nil
}

func (s TaskService) Transition(projectSlug, taskSlug, transitionName string) (model.TaskState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.TaskState{}, err
	}

	task, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskState{}, err
	}
	project, err := s.store.LoadProject(projectSlug)
	if err != nil {
		return model.TaskState{}, err
	}

	transition, ok := cfg.Workflow.Task.Transitions[transitionName]
	if !ok {
		return task, fmt.Errorf("task transition %q is not declared", transitionName)
	}
	if !statusAllowed(task.Status, transition.From) {
		return task, fmt.Errorf("task transition %q is not allowed from status %q", transitionName, task.Status)
	}
	for _, required := range transition.Requires {
		if task.Vars[required] == "" {
			return task, fmt.Errorf("task transition %q requires task var %q", transitionName, required)
		}
	}

	stepInput, err := s.stepContext(project, task, transitionName)
	if err != nil {
		return model.TaskState{}, err
	}
	result, err := s.runner.Run(context.Background(), transitionName, transition.Steps, stepInput)
	if err != nil {
		return model.TaskState{}, err
	}
	s.syncArtifacts(&task, result)

	if !result.OK {
		applyFailedOperation(&task, "tasks.transition", result, s.now)
		task.UpdatedAt = s.now().UTC().Format(time.RFC3339)
		if saveErr := s.store.SaveTask(task); saveErr != nil {
			return model.TaskState{}, saveErr
		}
		return task, errors.New(task.LastOp.Error)
	}

	task.Status = transition.To
	task.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	applySucceededOperation(&task, "tasks.transition", result, s.now)
	if cfg.Workflow.Task.IsTerminal(task.Status) {
		s.completeSession(projectSlug, taskSlug, &task)
	} else {
		s.syncActiveSession(projectSlug, taskSlug, task.Status)
	}
	if err := s.store.SaveTask(task); err != nil {
		return model.TaskState{}, err
	}
	if err := s.refreshProjectCounts(projectSlug); err != nil {
		return model.TaskState{}, err
	}

	return task, nil
}

func renderTaskSlug(pattern, projectSlug, name string, vars map[string]string) (string, error) {
	if pattern == "" {
		return name, nil
	}

	tmpl, err := template.New("task_slug").Option("missingkey=error").Parse(pattern)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	if err := tmpl.Execute(&builder, map[string]any{
		"name":    name,
		"project": map[string]string{"slug": projectSlug},
		"vars":    vars,
	}); err != nil {
		return "", err
	}

	return builder.String(), nil
}

func (s TaskService) stepContext(project model.ProjectState, task model.TaskState, transition string) (steps.Context, error) {
	artifacts, err := s.store.ListArtifacts(task.Project, task.Slug)
	if err != nil {
		return steps.Context{}, err
	}
	taskDir := filepath.Join(s.store.Root(), "projects", "active", task.Project, "tasks", task.Slug)
	return steps.Context{
		RuntimeRoot: s.store.Root(),
		Project: steps.ProjectContext{
			Slug:   project.Slug,
			Status: string(project.Status),
			Labels: project.Labels,
			Vars:   project.Vars,
		},
		Task: steps.TaskContext{
			Slug:   task.Slug,
			Status: string(task.Status),
			Labels: task.Labels,
			Vars:   task.Vars,
		},
		TaskDir:      taskDir,
		ArtifactsDir: filepath.Join(taskDir, "artifacts"),
		Artifacts:    artifacts,
		Transition:   transition,
	}, nil
}

func (s TaskService) syncArtifacts(task *model.TaskState, result steps.PhaseResult) {
	for _, exec := range result.Steps {
		for kind, data := range exec.Result.Artifacts {
			_ = s.store.SaveArtifact(task.Project, task.Slug, kind, data)
		}
	}
}

func (s TaskService) completeSession(projectSlug, taskSlug string, task *model.TaskState) {
	activeSession := task.Session.Active
	task.Session.Active = ""
	if activeSession == "" {
		return
	}
	task.Session.LastCompleted = &activeSession
	if session, loadErr := s.store.LoadSession(projectSlug, taskSlug, activeSession); loadErr == nil {
		session.Status = string(task.Status)
		session.UpdatedAt = s.now().UTC().Format(time.RFC3339)
		session.LastOp = task.LastOp
		_ = s.store.SaveSession(projectSlug, taskSlug, session)
	}
}

func (s TaskService) syncActiveSession(projectSlug, taskSlug string, status model.TaskStatus) {
	task, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil || task.Session.Active == "" {
		return
	}
	if session, loadErr := s.store.LoadSession(projectSlug, taskSlug, task.Session.Active); loadErr == nil {
		session.Status = string(status)
		session.UpdatedAt = s.now().UTC().Format(time.RFC3339)
		_ = s.store.SaveSession(projectSlug, taskSlug, session)
	}
}

func (s TaskService) refreshProjectCounts(projectSlug string) error {
	project, err := s.store.LoadProject(projectSlug)
	if err != nil {
		return err
	}
	tasks, err := s.store.ListTasks(projectSlug)
	if err != nil {
		return err
	}
	counts := model.TaskCounts{}
	for _, task := range tasks {
		counts[string(task.Status)]++
	}
	project.Tasks = counts
	project.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	return s.store.SaveProject(project)
}

func statusAllowed(current model.TaskStatus, allowed []model.TaskStatus) bool {
	for _, status := range allowed {
		if current == status {
			return true
		}
	}
	return false
}

func applyFailedOperation(task *model.TaskState, cmd string, result steps.PhaseResult, now func() time.Time) {
	message := "step execution failed"
	if len(result.Steps) > 0 {
		message = result.Steps[len(result.Steps)-1].Result.Message
	}
	task.LastOp = model.OperationState{
		Cmd:   cmd,
		OK:    false,
		Step:  result.FailedStep,
		Error: message,
		At:    now().UTC().Format(time.RFC3339),
	}
}

func applySucceededOperation(task *model.TaskState, cmd string, result steps.PhaseResult, now func() time.Time) {
	task.LastOp = model.OperationState{
		Cmd: cmd,
		OK:  true,
		At:  now().UTC().Format(time.RFC3339),
	}
	if len(result.Steps) > 0 {
		task.LastOp.Step = result.Steps[len(result.Steps)-1].Name
	}
}
