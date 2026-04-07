package lifecycle

import (
	"context"
	"errors"
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

func (s TaskService) Create(projectSlug, slug string, input CreateInput) (model.TaskRecord, error) {
	project, err := s.store.LoadProject(projectSlug)
	if err != nil {
		return model.TaskRecord{}, err
	}
	if model.IsTerminalStatus(project.State.Status) {
		return model.TaskRecord{}, fmt.Errorf("Can't add task %s to project %s because the project is %s.", slug, projectSlug, project.State.Status)
	}
	if input.Description == "" {
		return model.TaskRecord{}, fmt.Errorf("task description is required")
	}
	cfg, _, err := s.store.LoadOptionalConfig()
	if err != nil {
		return model.TaskRecord{}, err
	}
	number, err := s.store.NextTaskNumber(projectSlug)
	if err != nil {
		return model.TaskRecord{}, err
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	manifest := model.Manifest{ID: newID(), Kind: model.EntityKindTask, Number: number, Slug: slug, Name: fallbackName(slug, input.Name), Description: input.Description, ProjectID: project.Manifest.ID, ProjectNumber: project.Manifest.Number, ProjectSlug: project.Manifest.Slug, CreatedAt: now}
	if err := s.store.CreateTask(manifest); err != nil {
		return model.TaskRecord{}, err
	}
	patch := model.MetadataPatch{VarsSet: model.MergeVars(cfg.Defaults.Task.Vars, input.Vars)}
	labels := model.MergeLabels(cfg.Defaults.Task.Labels, input.Labels)
	if len(labels) > 0 || len(patch.VarsSet) > 0 {
		patch.Labels = labels
		if err := s.store.AppendTaskEvent(projectSlug, slug, model.Event{ID: newID(), EntityID: manifest.ID, Kind: model.EventKindMetadataPatch, At: now, Actor: "taskman", MetadataPatch: &patch}); err != nil {
			return model.TaskRecord{}, err
		}
	}
	return s.store.LoadTask(projectSlug, slug)
}

func (s TaskService) Update(projectSlug, taskSlug string, labels []string, vars map[string]string, unsetVars []string) (model.TaskRecord, error) {
	if _, _, err := s.store.LoadOptionalConfig(); err != nil {
		return model.TaskRecord{}, err
	}
	record, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskRecord{}, err
	}
	if err := rejectConflict(record.State); err != nil {
		return model.TaskRecord{}, err
	}
	patch := model.MetadataPatch{VarsSet: vars, VarsUnset: unsetVars}
	if labels != nil {
		patch.Labels = model.NormalizeLabels(labels)
	}
	event := model.Event{ID: newID(), EntityID: record.Manifest.ID, Kind: model.EventKindMetadataPatch, At: s.now().UTC().Format(time.RFC3339Nano), Actor: "taskman", ParentHeadID: record.State.CurrentHeadID, MetadataPatch: &patch}
	if err := s.store.AppendTaskEvent(projectSlug, taskSlug, event); err != nil {
		return model.TaskRecord{}, err
	}
	return s.store.LoadTask(projectSlug, taskSlug)
}

func (s TaskService) AddLabels(projectSlug, taskSlug string, labels []string) (model.TaskRecord, error) {
	record, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskRecord{}, err
	}
	return s.Update(projectSlug, taskSlug, model.MergeLabels(record.State.Labels, labels), nil, nil)
}

func (s TaskService) RemoveLabels(projectSlug, taskSlug string, labels []string) (model.TaskRecord, error) {
	record, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskRecord{}, err
	}
	return s.Update(projectSlug, taskSlug, model.RemoveLabels(record.State.Labels, labels), nil, nil)
}

func (s TaskService) Rename(projectSlug, taskSlug, newSlug string) (model.TaskRecord, error) {
	if _, _, err := s.store.LoadOptionalConfig(); err != nil {
		return model.TaskRecord{}, err
	}
	if newSlug == "" {
		return model.TaskRecord{}, fmt.Errorf("task slug is required")
	}
	return s.store.RenameTask(projectSlug, taskSlug, newSlug)
}

func (s TaskService) AddMessage(projectSlug, taskSlug string, input MessageInput) error {
	if _, _, err := s.store.LoadOptionalConfig(); err != nil {
		return err
	}
	record, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	if input.Body == "" {
		return fmt.Errorf("message body is required")
	}
	if input.Kind == "" {
		input.Kind = model.MessageKindComment
	}
	return s.store.AppendTaskEvent(projectSlug, taskSlug, model.Event{ID: newID(), EntityID: record.Manifest.ID, Kind: model.EventKindMessage, At: s.now().UTC().Format(time.RFC3339Nano), Actor: fallbackActor(input.Actor), Message: &model.MessagePayload{Kind: input.Kind, Body: input.Body}})
}

func (s TaskService) GetMessages(projectSlug, taskSlug string) ([]model.Event, error) {
	if _, _, err := s.store.LoadOptionalConfig(); err != nil {
		return nil, err
	}
	if _, err := s.store.LoadTask(projectSlug, taskSlug); err != nil {
		return nil, err
	}
	events, err := s.store.ListTaskEvents(projectSlug, taskSlug)
	if err != nil {
		return nil, err
	}
	return filterEvents(events, model.EventKindMessage), nil
}

func (s TaskService) GetTransitions(projectSlug, taskSlug string) ([]model.Event, error) {
	if _, _, err := s.store.LoadOptionalConfig(); err != nil {
		return nil, err
	}
	if _, err := s.store.LoadTask(projectSlug, taskSlug); err != nil {
		return nil, err
	}
	events, err := s.store.ListTaskEvents(projectSlug, taskSlug)
	if err != nil {
		return nil, err
	}
	return filterEvents(events, model.EventKindTransition), nil
}

func (s TaskService) Transition(projectSlug, taskSlug, verb string, input TransitionInput) (model.TaskRecord, []string, error) {
	task, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskRecord{}, nil, err
	}
	if err := rejectConflict(task.State); err != nil {
		return model.TaskRecord{}, nil, err
	}
	project, err := s.store.LoadProject(projectSlug)
	if err != nil {
		return model.TaskRecord{}, nil, err
	}
	if model.IsTerminalStatus(project.State.Status) && verb != "cancel" && verb != "reopen" {
		return model.TaskRecord{}, nil, fmt.Errorf("Can't %s task %s/%s because project %s is %s.", verb, projectSlug, taskSlug, projectSlug, project.State.Status)
	}
	if (verb == "start" || verb == "resume") && project.State.Status == model.StatusBacklog {
		return model.TaskRecord{}, nil, fmt.Errorf("Can't %s task %s/%s while project %s is still backlog. Move the project to planned or in_progress first.", verb, projectSlug, taskSlug, projectSlug)
	}
	spec, err := validateTransitionAllowed(task.State.Status, verb)
	if err != nil {
		return model.TaskRecord{}, nil, err
	}
	detail, err := buildStatusDetail(spec.To, input)
	if err != nil {
		return model.TaskRecord{}, nil, err
	}
	cfg, _, err := s.store.LoadOptionalConfig()
	if err != nil {
		return model.TaskRecord{}, nil, err
	}
	contextPayload, err := s.stepContext(project, task, verb)
	if err != nil {
		return model.TaskRecord{}, nil, err
	}
	preResult, err := s.runTaskMiddleware(projectSlug, taskSlug, task.Manifest.ID, "pre", verb, cfg.MiddlewareFor("task", verb).Pre, contextPayload)
	if err != nil {
		return model.TaskRecord{}, nil, err
	}
	if !preResult.OK {
		return model.TaskRecord{}, nil, errors.New(lastResultMessage(preResult))
	}
	warnings := collectWarnings(preResult)
	facts := collectFacts(preResult)
	artifacts := []string{}
	contextPayload.Task.Status = string(spec.To)
	postResult, err := s.runTaskMiddleware(projectSlug, taskSlug, task.Manifest.ID, "post", verb, cfg.MiddlewareFor("task", verb).Post, contextPayload)
	if err != nil {
		warnings = append(warnings, err.Error())
	} else {
		warnings = append(warnings, collectWarnings(postResult)...)
		if !postResult.OK {
			warnings = append(warnings, lastResultMessage(postResult))
		}
		mergeFacts(facts, collectFacts(postResult))
		for kind, data := range collectArtifacts(postResult) {
			if saveErr := s.store.SaveArtifact(projectSlug, taskSlug, kind, data); saveErr != nil {
				warnings = append(warnings, saveErr.Error())
				continue
			}
			artifacts = append(artifacts, kind)
		}
	}
	event := model.Event{ID: newID(), EntityID: task.Manifest.ID, Kind: model.EventKindTransition, At: s.now().UTC().Format(time.RFC3339Nano), Actor: fallbackActor(input.Actor), ParentHeadID: task.State.CurrentHeadID, Transition: &model.TransitionPayload{Verb: verb, From: task.State.Status, To: spec.To, ReasonType: detail.ReasonType, Reason: detail.Reason, ResumeWhen: detail.ResumeWhen, Summary: detail.Summary, Warnings: warnings, Facts: facts, Artifacts: artifacts}}
	if err := s.store.AppendTaskEvent(projectSlug, taskSlug, event); err != nil {
		return model.TaskRecord{}, nil, err
	}
	recordAfter, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskRecord{}, nil, err
	}
	return recordAfter, warnings, nil
}

func (s TaskService) stepContext(project model.ProjectRecord, task model.TaskRecord, transition string) (steps.Context, error) {
	artifacts, err := s.store.ListArtifacts(task.Manifest.ProjectSlug, task.Manifest.Slug)
	if err != nil {
		return steps.Context{}, err
	}
	taskDir := filepath.Join(s.store.Root(), "projects", task.Manifest.ProjectSlug, "tasks", task.Manifest.Slug)
	return steps.Context{RuntimeRoot: s.store.Root(), Project: steps.ProjectContext{Slug: project.Manifest.Slug, Status: string(project.State.Status), Labels: project.State.Labels, Vars: project.State.Vars}, Task: steps.TaskContext{Slug: task.Manifest.Slug, Status: string(task.State.Status), Labels: task.State.Labels, Vars: task.State.Vars}, TaskDir: taskDir, ArtifactsDir: filepath.Join(taskDir, "artifacts"), Artifacts: artifacts, Transition: transition}, nil
}

func (s TaskService) runTaskMiddleware(projectSlug, taskSlug, entityID, phase, verb string, commands []model.MiddlewareCommand, input steps.Context) (steps.PhaseResult, error) {
	if len(commands) == 0 {
		return steps.PhaseResult{OK: true}, nil
	}
	startedAt := s.now().UTC().Format(time.RFC3339Nano)
	if err := s.store.AppendTaskEvent(projectSlug, taskSlug, model.Event{ID: newID(), EntityID: entityID, Kind: model.EventKindMiddlewarePhaseStart, At: startedAt, Actor: "taskman", Middleware: &model.MiddlewareEventData{Phase: phase, OK: true, Message: verb}}); err != nil {
		return steps.PhaseResult{}, err
	}
	result, err := s.runner.Run(context.Background(), verb, commands, input)
	finishedAt := s.now().UTC().Format(time.RFC3339Nano)
	if err != nil {
		if appendErr := s.store.AppendTaskEvent(projectSlug, taskSlug, model.Event{ID: newID(), EntityID: entityID, Kind: model.EventKindMiddlewarePhaseFinish, At: finishedAt, Actor: "taskman", Middleware: &model.MiddlewareEventData{Phase: phase, OK: false, Message: err.Error()}}); appendErr != nil {
			return steps.PhaseResult{}, appendErr
		}
		return steps.PhaseResult{}, err
	}
	for _, step := range result.Steps {
		artifacts := []string{}
		for key := range step.Result.Artifacts {
			artifacts = append(artifacts, key)
		}
		if err := s.store.AppendTaskEvent(projectSlug, taskSlug, model.Event{ID: newID(), EntityID: entityID, Kind: model.EventKindMiddlewareStepFinish, At: finishedAt, Actor: "taskman", Middleware: &model.MiddlewareEventData{Phase: phase, Step: step.Name, OK: step.Result.OK, Message: step.Result.Message, Warnings: step.Result.Warnings, Artifacts: artifacts}}); err != nil {
			return steps.PhaseResult{}, err
		}
	}
	if err := s.store.AppendTaskEvent(projectSlug, taskSlug, model.Event{ID: newID(), EntityID: entityID, Kind: model.EventKindMiddlewarePhaseFinish, At: finishedAt, Actor: "taskman", Middleware: &model.MiddlewareEventData{Phase: phase, OK: result.OK, Message: lastResultMessage(result), Warnings: collectWarnings(result)}}); err != nil {
		return steps.PhaseResult{}, err
	}
	return result, nil
}
