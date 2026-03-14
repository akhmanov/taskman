package lifecycle

import (
	"context"
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

func NewProjectService(s store.Store, runner steps.Runner) ProjectService {
	return ProjectService{store: s, runner: runner, now: time.Now}
}

func (s ProjectService) Update(slug string, labels []string, vars map[string]string, unsetVars []string) (model.ProjectState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.ProjectState{}, err
	}
	if err := cfg.ValidateVarOverrides("project", vars); err != nil {
		return model.ProjectState{}, err
	}

	project, err := s.store.LoadProject(slug)
	if err != nil {
		return model.ProjectState{}, err
	}

	if labels != nil {
		project.Labels = append([]string{}, labels...)
	}
	if len(unsetVars) > 0 {
		if project.Vars == nil {
			project.Vars = map[string]string{}
		}
		for _, key := range unsetVars {
			delete(project.Vars, key)
		}
	}
	if vars != nil {
		project.Vars = model.MergeVars(project.Vars, vars)
	}
	if err := cfg.ValidateRequiredVars("project", project.Vars); err != nil {
		return model.ProjectState{}, err
	}

	project.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	project.LastOp = model.OperationState{Cmd: "projects.update", OK: true, At: project.UpdatedAt}
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

func (s ProjectService) Create(slug string, labels []string, vars map[string]string) (model.ProjectState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.ProjectState{}, err
	}
	if err := cfg.ValidateVarOverrides("project", vars); err != nil {
		return model.ProjectState{}, err
	}

	mergedVars := model.MergeVars(cfg.Defaults.Project.Vars, vars)
	if err := cfg.ValidateRequiredVars("project", mergedVars); err != nil {
		return model.ProjectState{}, err
	}

	now := s.now().UTC().Format(time.RFC3339)
	project := model.ProjectState{
		Version:   1,
		Slug:      slug,
		Status:    model.ProjectStatusActive,
		Labels:    append([]string{}, append(cfg.Defaults.Project.Labels, labels...)...),
		Vars:      mergedVars,
		CreatedAt: now,
		UpdatedAt: now,
		Tasks:     model.TaskCounts{},
		Archive:   model.ArchiveState{Ready: false},
		LastOp:    model.OperationState{Cmd: "projects.create", OK: true, At: now},
	}

	if err := s.store.ScaffoldProject(project); err != nil {
		return model.ProjectState{}, err
	}

	return project, nil
}

func (s ProjectService) Archive(slug string) (model.ProjectState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.ProjectState{}, err
	}

	project, err := s.store.LoadProject(slug)
	if err != nil {
		return model.ProjectState{}, err
	}

	project.Archive = s.evaluateArchiveState(slug, cfg.Workflow.Task.TerminalStatuses)
	project.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if err := s.store.SaveProject(project); err != nil {
		return model.ProjectState{}, err
	}

	if !project.Archive.Ready {
		return project, errors.New("project is not ready to archive")
	}

	result, err := s.runner.Run(context.Background(), "archive", cfg.Workflow.Project.Archive.Steps, steps.Context{
		RuntimeRoot: s.store.Root(),
		Project: steps.ProjectContext{
			Slug:   project.Slug,
			Status: string(project.Status),
			Labels: project.Labels,
			Vars:   project.Vars,
		},
		Transition: "archive",
	})
	if err != nil {
		return model.ProjectState{}, err
	}
	if !result.OK {
		project.LastOp = model.OperationState{Cmd: "projects.archive", OK: false, Step: result.FailedStep, At: s.now().UTC().Format(time.RFC3339)}
		if len(result.Steps) > 0 {
			project.LastOp.Error = result.Steps[len(result.Steps)-1].Result.Message
		}
		_ = s.store.SaveProject(project)
		return project, errors.New(project.LastOp.Error)
	}

	project.Status = model.ProjectStatusArchived
	project.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	project.LastOp = model.OperationState{Cmd: "projects.archive", OK: true, At: project.UpdatedAt}
	if err := s.store.ArchiveProject(project, s.now().UTC().Year()); err != nil {
		return model.ProjectState{}, err
	}
	return project, nil
}

func (s ProjectService) evaluateArchiveState(projectSlug string, terminalStatuses []model.TaskStatus) model.ArchiveState {
	tasks, err := s.store.ListTasks(projectSlug)
	if err != nil {
		return model.ArchiveState{Ready: false, Blockers: []string{err.Error()}}
	}

	allowed := map[model.TaskStatus]struct{}{}
	for _, status := range terminalStatuses {
		allowed[status] = struct{}{}
	}

	blockers := make([]string, 0)
	for _, task := range tasks {
		if _, ok := allowed[task.Status]; !ok {
			blockers = append(blockers, fmt.Sprintf("task %s is not terminal", task.Slug))
		}
	}

	return model.ArchiveState{
		Ready:    len(blockers) == 0,
		Blockers: blockers,
	}
}
