package lifecycle

import (
	"context"
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

func NewProjectService(s store.Store, runner steps.Runner) ProjectService {
	return ProjectService{store: s, runner: runner, now: time.Now}
}

func (s ProjectService) Create(slug string, labels []string, vars map[string]string) (model.ProjectState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.ProjectState{}, err
	}
	now := s.now().UTC().Format(time.RFC3339)
	project := model.ProjectState{
		Version:   2,
		Slug:      slug,
		Status:    model.StatusBacklog,
		Labels:    append(append([]string{}, cfg.Defaults.Project.Labels...), labels...),
		Vars:      model.MergeVars(cfg.Defaults.Project.Vars, vars),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.ScaffoldProject(project); err != nil {
		return model.ProjectState{}, err
	}
	return project, nil
}

func (s ProjectService) Update(slug string, labels []string, vars map[string]string, unsetVars []string) (model.ProjectState, error) {
	project, err := s.store.LoadProject(slug)
	if err != nil {
		return model.ProjectState{}, err
	}
	if labels != nil {
		project.Labels = append([]string{}, labels...)
	}
	if project.Vars == nil {
		project.Vars = map[string]string{}
	}
	for _, key := range unsetVars {
		delete(project.Vars, key)
	}
	if vars != nil {
		project.Vars = model.MergeVars(project.Vars, vars)
	}
	project.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if err := s.store.SaveProject(project); err != nil {
		return model.ProjectState{}, err
	}
	return project, nil
}

func (s ProjectService) GetBrief(slug string) (string, error) {
	if _, err := s.store.LoadProject(slug); err != nil {
		return "", err
	}
	return s.store.LoadProjectBrief(slug)
}

func (s ProjectService) SetBrief(slug, brief string) error {
	if _, err := s.store.LoadProject(slug); err != nil {
		return err
	}
	if err := validateProjectBrief(brief); err != nil {
		return err
	}
	return s.store.SaveProjectBrief(slug, brief)
}

func (s ProjectService) AddEvent(slug string, event model.PayloadEvent) error {
	if _, err := s.store.LoadProject(slug); err != nil {
		return err
	}
	if err := validatePayloadEvent(event); err != nil {
		return err
	}
	existing, err := s.store.ListProjectEvents(slug)
	if err != nil {
		return err
	}
	if hasEventID(existing, event.ID) {
		return fmt.Errorf("event id %q already exists", event.ID)
	}
	return s.store.AppendProjectEvent(slug, event)
}

func (s ProjectService) GetEvents(slug string) ([]model.PayloadEvent, error) {
	if _, err := s.store.LoadProject(slug); err != nil {
		return nil, err
	}
	return s.store.ListProjectEvents(slug)
}

func (s ProjectService) Transition(slug, verb string, input TransitionInput) (model.ProjectState, []string, error) {
	project, err := s.store.LoadProject(slug)
	if err != nil {
		return model.ProjectState{}, nil, err
	}
	spec, err := validateTransitionAllowed(project.Status, verb)
	if err != nil {
		return model.ProjectState{}, nil, err
	}
	if verb == "complete" {
		tasks, err := s.store.ListTasks(slug)
		if err != nil {
			return model.ProjectState{}, nil, err
		}
		for _, task := range tasks {
			if !model.IsTerminalStatus(task.Status) {
				return model.ProjectState{}, nil, fmt.Errorf("project can be done only when all tasks are done or canceled")
			}
		}
	}
	detail, err := buildStatusDetail(spec.To, input)
	if err != nil {
		return model.ProjectState{}, nil, err
	}
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.ProjectState{}, nil, err
	}
	contextPayload := steps.Context{
		RuntimeRoot: s.store.Root(),
		Project: steps.ProjectContext{
			Slug:   project.Slug,
			Status: string(project.Status),
			Labels: project.Labels,
			Vars:   project.Vars,
		},
		Transition: verb,
	}
	preResult, err := s.runner.Run(context.Background(), verb, cfg.MiddlewareFor("project", verb).Pre, contextPayload)
	if err != nil {
		return model.ProjectState{}, nil, err
	}
	if !preResult.OK {
		return model.ProjectState{}, nil, fmt.Errorf(lastResultMessage(preResult))
	}
	previous := project.Status
	project.Status = spec.To
	project.StatusDetail = detail
	project.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	contextPayload.Project.Status = string(project.Status)
	postResult, err := s.runner.Run(context.Background(), verb, cfg.MiddlewareFor("project", verb).Post, contextPayload)
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
	if err := s.store.SaveProject(project); err != nil {
		return model.ProjectState{}, nil, err
	}
	record := transitionRecord(project.UpdatedAt, input.Actor, verb, previous, spec.To, input, warnings, facts, artifacts)
	if err := s.store.AppendProjectTransition(slug, record); err != nil {
		return model.ProjectState{}, nil, err
	}
	return project, warnings, nil
}
