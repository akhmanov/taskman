package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/akhmanov/taskman/internal/model"
	"github.com/akhmanov/taskman/internal/steps"
	"github.com/akhmanov/taskman/internal/store"
)

type TaskService struct {
	store  store.Store
	runner steps.Runner
	now    func() time.Time
}

func NewTaskService(s store.Store, runner steps.Runner) TaskService {
	return TaskService{store: s, runner: runner, now: time.Now}
}

func (s TaskService) Create(projectSlug, slug string, labels []string, vars map[string]string) (model.TaskState, error) {
	project, err := s.store.LoadProject(projectSlug)
	if err != nil {
		return model.TaskState{}, err
	}
	if model.IsTerminalStatus(project.Status) {
		return model.TaskState{}, fmt.Errorf("cannot add task to terminal project %q", project.Status)
	}
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.TaskState{}, err
	}
	now := s.now().UTC().Format(time.RFC3339)
	task := model.TaskState{
		Version:   2,
		Slug:      slug,
		Project:   projectSlug,
		Status:    model.StatusBacklog,
		Labels:    append(append([]string{}, cfg.Defaults.Task.Labels...), labels...),
		Vars:      model.MergeVars(cfg.Defaults.Task.Vars, vars),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.ScaffoldTask(task); err != nil {
		return model.TaskState{}, err
	}
	return task, nil
}

func (s TaskService) Update(projectSlug, taskSlug string, labels []string, vars map[string]string, unsetVars []string) (model.TaskState, error) {
	task, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskState{}, err
	}
	if labels != nil {
		task.Labels = append([]string{}, labels...)
	}
	if task.Vars == nil {
		task.Vars = map[string]string{}
	}
	for _, key := range unsetVars {
		delete(task.Vars, key)
	}
	if vars != nil {
		task.Vars = model.MergeVars(task.Vars, vars)
	}
	task.UpdatedAt = s.now().UTC().Format(time.RFC3339)
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
	existing, err := s.store.ListTaskEvents(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	if hasEventID(existing, event.ID) {
		return fmt.Errorf("event id %q already exists", event.ID)
	}
	return s.store.AppendTaskEvent(projectSlug, taskSlug, event)
}

func (s TaskService) GetEvents(projectSlug, taskSlug string) ([]model.PayloadEvent, error) {
	if _, err := s.store.LoadTask(projectSlug, taskSlug); err != nil {
		return nil, err
	}
	return s.store.ListTaskEvents(projectSlug, taskSlug)
}

func (s TaskService) Transition(projectSlug, taskSlug, verb string, input TransitionInput) (model.TaskState, []string, error) {
	task, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskState{}, nil, err
	}
	project, err := s.store.LoadProject(projectSlug)
	if err != nil {
		return model.TaskState{}, nil, err
	}
	if model.IsTerminalStatus(project.Status) && verb != "cancel" && verb != "reopen" {
		return model.TaskState{}, nil, fmt.Errorf("cannot run task transition inside terminal project %q", project.Status)
	}
	if (verb == "start" || verb == "resume") && project.Status == model.StatusBacklog {
		return model.TaskState{}, nil, fmt.Errorf("cannot %s task while project is backlog", verb)
	}
	spec, err := validateTransitionAllowed(task.Status, verb)
	if err != nil {
		return model.TaskState{}, nil, err
	}
	detail, err := buildStatusDetail(spec.To, input)
	if err != nil {
		return model.TaskState{}, nil, err
	}
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.TaskState{}, nil, err
	}
	contextPayload, err := s.stepContext(project, task, verb)
	if err != nil {
		return model.TaskState{}, nil, err
	}
	preResult, err := s.runner.Run(context.Background(), verb, cfg.MiddlewareFor("task", verb).Pre, contextPayload)
	if err != nil {
		return model.TaskState{}, nil, err
	}
	if !preResult.OK {
		return model.TaskState{}, nil, fmt.Errorf(lastResultMessage(preResult))
	}
	previous := task.Status
	task.Status = spec.To
	task.StatusDetail = detail
	task.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	contextPayload, err = s.stepContext(project, task, verb)
	if err != nil {
		return model.TaskState{}, nil, err
	}
	postResult, err := s.runner.Run(context.Background(), verb, cfg.MiddlewareFor("task", verb).Post, contextPayload)
	warnings := collectWarnings(preResult)
	facts := collectFacts(preResult)
	artifacts := collectArtifacts(preResult)
	if err != nil {
		warnings = append(warnings, err.Error())
	} else {
		warnings = append(warnings, collectWarnings(postResult)...)
		if !postResult.OK {
			warnings = append(warnings, lastResultMessage(postResult))
		}
		mergeFacts(facts, collectFacts(postResult))
		mergeArtifacts(artifacts, collectArtifacts(postResult))
	}
	for kind, data := range artifacts {
		if saveErr := s.store.SaveArtifact(projectSlug, taskSlug, kind, data); saveErr != nil {
			warnings = append(warnings, saveErr.Error())
		}
	}
	if err := s.store.SaveTask(task); err != nil {
		return model.TaskState{}, nil, err
	}
	record := transitionRecord(task.UpdatedAt, input.Actor, verb, previous, spec.To, input, warnings, facts, artifacts)
	if err := s.store.AppendTaskTransition(projectSlug, taskSlug, record); err != nil {
		return model.TaskState{}, nil, err
	}
	return task, warnings, nil
}

func (s TaskService) stepContext(project model.ProjectState, task model.TaskState, transition string) (steps.Context, error) {
	artifacts, err := s.store.ListArtifacts(task.Project, task.Slug)
	if err != nil {
		return steps.Context{}, err
	}
	taskDir := filepath.Join(s.store.Root(), "projects", task.Project, "tasks", task.Slug)
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
