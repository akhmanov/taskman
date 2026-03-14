package lifecycle

import (
	"context"
	"errors"
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

func (s ProjectService) Create(slug string, labels []string, traits map[string]string) (model.ProjectState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.ProjectState{}, err
	}
	if err := cfg.ValidateTraitOverrides("project", traits); err != nil {
		return model.ProjectState{}, err
	}

	now := s.now().UTC().Format(time.RFC3339)
	project := model.ProjectState{
		Version:   1,
		Slug:      slug,
		Status:    model.ProjectStatusActive,
		Labels:    append([]string{}, append(cfg.Defaults.Project.Labels, labels...)...),
		Traits:    mergeMap(cfg.Defaults.Project.Traits, traits),
		CreatedAt: now,
		UpdatedAt: now,
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

	project.Archive = s.evaluateArchiveState(slug)
	project.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	if err := s.store.SaveProject(project); err != nil {
		return model.ProjectState{}, err
	}

	if !project.Archive.Ready {
		return project, errors.New("project is not ready to archive")
	}

	result, err := s.runner.RunPhase(context.Background(), model.ProjectArchivePhase, cfg.Steps[model.ProjectArchivePhase], steps.Context{RuntimeRoot: s.store.Root(), ProjectSlug: slug, ProjectTraits: project.Traits})
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

func (s ProjectService) evaluateArchiveState(projectSlug string) model.ArchiveState {
	tasks, err := s.store.ListTasks(projectSlug)
	if err != nil {
		return model.ArchiveState{Ready: false, Blockers: []string{err.Error()}}
	}

	blockers := make([]string, 0)
	for _, task := range tasks {
		if task.Status != model.TaskStatusDone && task.Status != model.TaskStatusCancelled {
			blockers = append(blockers, "task "+task.Slug+" is not terminal")
		}
		if task.Worktree.Status == model.WorktreeStatusPresent {
			blockers = append(blockers, "task "+task.Slug+" worktree is not cleaned")
		}
	}

	return model.ArchiveState{
		Ready:    len(blockers) == 0,
		Blockers: blockers,
	}
}
