package lifecycle

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/akhmanov/taskman/internal/model"
	"github.com/akhmanov/taskman/internal/steps"
	"github.com/akhmanov/taskman/internal/store"
)

type ProjectService struct {
	store  store.Store
	runner steps.Runner
	now    func() time.Time
}

type CreateInput struct {
	Name        string
	Description string
	Labels      []string
	Vars        map[string]string
}

type MessageInput struct {
	Actor string
	Kind  model.MessageKind
	Body  string
}

func NewProjectService(s store.Store, runner steps.Runner) ProjectService {
	return ProjectService{store: s, runner: runner, now: time.Now}
}

func (s ProjectService) Create(slug string, input CreateInput) (model.ProjectRecord, error) {
	if input.Description == "" {
		return model.ProjectRecord{}, fmt.Errorf("project description is required")
	}
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.ProjectRecord{}, err
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	manifest := model.Manifest{
		ID:          newID(),
		Kind:        model.EntityKindProject,
		Slug:        slug,
		Name:        fallbackName(slug, input.Name),
		Description: input.Description,
		CreatedAt:   now,
	}
	if err := s.store.CreateProject(manifest); err != nil {
		return model.ProjectRecord{}, err
	}
	patch := model.MetadataPatch{Labels: append([]string{}, input.Labels...), VarsSet: model.MergeVars(cfg.Defaults.Project.Vars, input.Vars)}
	if len(cfg.Defaults.Project.Labels) > 0 || len(input.Labels) > 0 || len(patch.VarsSet) > 0 {
		patch.Labels = append(append([]string{}, cfg.Defaults.Project.Labels...), input.Labels...)
		if err := s.store.AppendProjectEvent(slug, model.Event{ID: newID(), EntityID: manifest.ID, Kind: model.EventKindMetadataPatch, At: now, Actor: "taskman", MetadataPatch: &patch}); err != nil {
			return model.ProjectRecord{}, err
		}
	}
	return s.store.LoadProject(slug)
}

func (s ProjectService) Update(slug string, labels []string, vars map[string]string, unsetVars []string) (model.ProjectRecord, error) {
	record, err := s.store.LoadProject(slug)
	if err != nil {
		return model.ProjectRecord{}, err
	}
	if err := rejectConflict(record.State); err != nil {
		return model.ProjectRecord{}, err
	}
	patch := model.MetadataPatch{VarsSet: vars, VarsUnset: unsetVars}
	if labels != nil {
		patch.Labels = append([]string{}, labels...)
	}
	event := model.Event{ID: newID(), EntityID: record.Manifest.ID, Kind: model.EventKindMetadataPatch, At: s.now().UTC().Format(time.RFC3339Nano), Actor: "taskman", ParentHeadID: record.State.CurrentHeadID, MetadataPatch: &patch}
	if err := s.store.AppendProjectEvent(slug, event); err != nil {
		return model.ProjectRecord{}, err
	}
	return s.store.LoadProject(slug)
}

func (s ProjectService) AddMessage(slug string, input MessageInput) error {
	record, err := s.store.LoadProject(slug)
	if err != nil {
		return err
	}
	if input.Body == "" {
		return fmt.Errorf("message body is required")
	}
	if input.Kind == "" {
		input.Kind = model.MessageKindComment
	}
	return s.store.AppendProjectEvent(slug, model.Event{ID: newID(), EntityID: record.Manifest.ID, Kind: model.EventKindMessage, At: s.now().UTC().Format(time.RFC3339Nano), Actor: fallbackActor(input.Actor), Message: &model.MessagePayload{Kind: input.Kind, Body: input.Body}})
}

func (s ProjectService) GetMessages(slug string) ([]model.Event, error) {
	if _, err := s.store.LoadProject(slug); err != nil {
		return nil, err
	}
	events, err := s.store.ListProjectEvents(slug)
	if err != nil {
		return nil, err
	}
	return filterEvents(events, model.EventKindMessage), nil
}

func (s ProjectService) GetTransitions(slug string) ([]model.Event, error) {
	if _, err := s.store.LoadProject(slug); err != nil {
		return nil, err
	}
	events, err := s.store.ListProjectEvents(slug)
	if err != nil {
		return nil, err
	}
	return filterEvents(events, model.EventKindTransition), nil
}

func (s ProjectService) Transition(slug, verb string, input TransitionInput) (model.ProjectRecord, []string, error) {
	record, err := s.store.LoadProject(slug)
	if err != nil {
		return model.ProjectRecord{}, nil, err
	}
	if err := rejectConflict(record.State); err != nil {
		return model.ProjectRecord{}, nil, err
	}
	if verb == "complete" {
		tasks, err := s.store.ListTasks(slug)
		if err != nil {
			return model.ProjectRecord{}, nil, err
		}
		for _, task := range tasks {
			if !model.IsTerminalStatus(task.State.Status) {
				return model.ProjectRecord{}, nil, fmt.Errorf("project can be done only when all tasks are done or canceled")
			}
		}
	}
	spec, err := validateTransitionAllowed(record.State.Status, verb)
	if err != nil {
		return model.ProjectRecord{}, nil, err
	}
	contextPayload := steps.Context{RuntimeRoot: s.store.Root(), Project: steps.ProjectContext{Slug: record.Manifest.Slug, Status: string(record.State.Status), Labels: record.State.Labels, Vars: record.State.Vars}, Transition: verb}
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.ProjectRecord{}, nil, err
	}
	preResult, err := s.runProjectMiddleware(slug, record.Manifest.ID, "pre", verb, cfg.MiddlewareFor("project", verb).Pre, contextPayload)
	if err != nil {
		return model.ProjectRecord{}, nil, err
	}
	if !preResult.OK {
		return model.ProjectRecord{}, nil, errors.New(lastResultMessage(preResult))
	}
	detail, err := buildStatusDetail(spec.To, input)
	if err != nil {
		return model.ProjectRecord{}, nil, err
	}
	warnings := collectWarnings(preResult)
	facts := collectFacts(preResult)
	artifacts := []string{}
	contextPayload.Project.Status = string(spec.To)
	postResult, err := s.runProjectMiddleware(slug, record.Manifest.ID, "post", verb, cfg.MiddlewareFor("project", verb).Post, contextPayload)
	if err != nil {
		warnings = append(warnings, err.Error())
	} else {
		warnings = append(warnings, collectWarnings(postResult)...)
		if !postResult.OK {
			warnings = append(warnings, lastResultMessage(postResult))
		}
		mergeFacts(facts, collectFacts(postResult))
		for key := range collectArtifacts(postResult) {
			artifacts = append(artifacts, key)
		}
	}
	transitionEvent := model.Event{ID: newID(), EntityID: record.Manifest.ID, Kind: model.EventKindTransition, At: s.now().UTC().Format(time.RFC3339Nano), Actor: fallbackActor(input.Actor), ParentHeadID: record.State.CurrentHeadID, Transition: &model.TransitionPayload{Verb: verb, From: record.State.Status, To: spec.To, ReasonType: detail.ReasonType, Reason: detail.Reason, ResumeWhen: detail.ResumeWhen, Summary: detail.Summary, Warnings: warnings, Facts: facts, Artifacts: artifacts}}
	if err := s.store.AppendProjectEvent(slug, transitionEvent); err != nil {
		return model.ProjectRecord{}, nil, err
	}
	recordAfter, err := s.store.LoadProject(slug)
	if err != nil {
		return model.ProjectRecord{}, nil, err
	}
	return recordAfter, warnings, nil
}

func (s ProjectService) runProjectMiddleware(slug, entityID, phase, verb string, commands []model.MiddlewareCommand, input steps.Context) (steps.PhaseResult, error) {
	startedAt := s.now().UTC().Format(time.RFC3339Nano)
	if err := s.store.AppendProjectEvent(slug, model.Event{ID: newID(), EntityID: entityID, Kind: model.EventKindMiddlewarePhaseStart, At: startedAt, Actor: "taskman", Middleware: &model.MiddlewareEventData{Phase: phase, OK: true, Message: verb}}); err != nil {
		return steps.PhaseResult{}, err
	}
	result, err := s.runner.Run(context.Background(), verb, commands, input)
	finishedAt := s.now().UTC().Format(time.RFC3339Nano)
	if err != nil {
		if appendErr := s.store.AppendProjectEvent(slug, model.Event{ID: newID(), EntityID: entityID, Kind: model.EventKindMiddlewarePhaseFinish, At: finishedAt, Actor: "taskman", Middleware: &model.MiddlewareEventData{Phase: phase, OK: false, Message: err.Error()}}); appendErr != nil {
			return steps.PhaseResult{}, appendErr
		}
		return steps.PhaseResult{}, err
	}
	for _, step := range result.Steps {
		artifacts := []string{}
		for key := range step.Result.Artifacts {
			artifacts = append(artifacts, key)
		}
		if err := s.store.AppendProjectEvent(slug, model.Event{ID: newID(), EntityID: entityID, Kind: model.EventKindMiddlewareStepFinish, At: finishedAt, Actor: "taskman", Middleware: &model.MiddlewareEventData{Phase: phase, Step: step.Name, OK: step.Result.OK, Message: step.Result.Message, Warnings: step.Result.Warnings, Artifacts: artifacts}}); err != nil {
			return steps.PhaseResult{}, err
		}
	}
	if err := s.store.AppendProjectEvent(slug, model.Event{ID: newID(), EntityID: entityID, Kind: model.EventKindMiddlewarePhaseFinish, At: finishedAt, Actor: "taskman", Middleware: &model.MiddlewareEventData{Phase: phase, OK: result.OK, Message: lastResultMessage(result), Warnings: collectWarnings(result)}}); err != nil {
		return steps.PhaseResult{}, err
	}
	return result, nil
}

func filterEvents(events []model.Event, kind model.EventKind) []model.Event {
	filtered := []model.Event{}
	for _, event := range events {
		if event.Kind == kind {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func fallbackActor(actor string) string {
	if actor == "" {
		return "taskman"
	}
	return actor
}

func fallbackName(slug, name string) string {
	if name == "" {
		return slug
	}
	return name
}

func rejectConflict(state model.ProjectionState) error {
	if state.HasConflict() {
		return fmt.Errorf("entity has unresolved conflict; resolve divergent heads first")
	}
	return nil
}

func newID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
