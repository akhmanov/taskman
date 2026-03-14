package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/assistant-wi/taskman/internal/model"
	"github.com/assistant-wi/taskman/internal/steps"
	"github.com/assistant-wi/taskman/internal/store"
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
