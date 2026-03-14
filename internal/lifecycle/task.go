package lifecycle

import (
	"context"
	"errors"
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

func (s TaskService) Create(projectSlug, repo, name string, labels []string, traits map[string]string) (model.TaskState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.TaskState{}, err
	}
	if _, err := s.store.LoadProject(projectSlug); err != nil {
		return model.TaskState{}, err
	}
	if err := cfg.ValidateTraitOverrides("task", traits); err != nil {
		return model.TaskState{}, err
	}

	slug, err := renderTaskSlug(cfg.Naming.TaskSlug, repo, name)
	if err != nil {
		return model.TaskState{}, err
	}

	mergedTraits := mergeMap(cfg.Defaults.Task.Traits, traits)
	now := s.now().UTC().Format(time.RFC3339)
	task := model.TaskState{
		Version:   1,
		Slug:      slug,
		Project:   projectSlug,
		Repo:      repo,
		Status:    model.TaskStatusActive,
		Labels:    append([]string{}, append(cfg.Defaults.Task.Labels, labels...)...),
		Traits:    mergedTraits,
		CreatedAt: now,
		UpdatedAt: now,
		Handoff:   false,
		Session:   model.TaskSessionState{Active: "S-001"},
		MR:        model.TaskMRState{Status: initialMRStatus(mergedTraits)},
		Worktree:  model.TaskWorktreeState{Status: model.WorktreeStatusMissing},
	}

	if err := s.store.ScaffoldTask(task); err != nil {
		return model.TaskState{}, err
	}

	session := model.SessionState{
		Version:   1,
		ID:        "S-001",
		Task:      task.Slug,
		Status:    string(model.TaskStatusActive),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.SaveSession(projectSlug, slug, session); err != nil {
		return model.TaskState{}, err
	}

	stepInput, err := stepContext(s.store.Root(), cfg, task)
	if err != nil {
		return model.TaskState{}, err
	}
	phaseResult, err := s.runner.RunPhase(context.Background(), model.TaskStartPhase, cfg.Steps[model.TaskStartPhase], stepInput)
	if err != nil {
		return model.TaskState{}, err
	}
	s.syncArtifacts(&task, phaseResult)

	if !phaseResult.OK {
		applyFailedOperation(&task, "tasks.create", phaseResult)
		_ = s.store.SaveTask(task)
		return task, errors.New(task.LastOp.Error)
	}

	applySucceededOperation(&task, "tasks.create", phaseResult)
	if err := s.store.SaveTask(task); err != nil {
		return model.TaskState{}, err
	}

	return task, nil
}

func (s TaskService) Done(projectSlug, taskSlug string) (model.TaskState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.TaskState{}, err
	}

	task, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskState{}, err
	}

	stepInput, err := stepContext(s.store.Root(), cfg, task)
	if err != nil {
		return model.TaskState{}, err
	}
	phaseResult, err := s.runner.RunPhase(context.Background(), model.TaskDonePhase, cfg.Steps[model.TaskDonePhase], stepInput)
	if err != nil {
		return model.TaskState{}, err
	}
	s.syncArtifacts(&task, phaseResult)

	if !phaseResult.OK {
		applyFailedOperation(&task, "tasks.done", phaseResult)
		task.UpdatedAt = s.now().UTC().Format(time.RFC3339)
		if saveErr := s.store.SaveTask(task); saveErr != nil {
			return model.TaskState{}, saveErr
		}
		return task, errors.New(task.LastOp.Error)
	}

	activeSession := task.Session.Active
	task.Status = model.TaskStatusDone
	task.Session.Active = ""
	if activeSession != "" {
		task.Session.LastCompleted = &activeSession
		if session, loadErr := s.store.LoadSession(projectSlug, taskSlug, activeSession); loadErr == nil {
			session.Status = string(model.TaskStatusDone)
			session.UpdatedAt = s.now().UTC().Format(time.RFC3339)
			_ = s.store.SaveSession(projectSlug, taskSlug, session)
		}
	}
	task.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	applySucceededOperation(&task, "tasks.done", phaseResult)
	if err := s.store.SaveTask(task); err != nil {
		return model.TaskState{}, err
	}

	return task, nil
}

func (s TaskService) Cleanup(projectSlug, taskSlug string) (model.TaskState, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.TaskState{}, err
	}

	task, err := s.store.LoadTask(projectSlug, taskSlug)
	if err != nil {
		return model.TaskState{}, err
	}

	if task.Status != model.TaskStatusDone && task.Status != model.TaskStatusCancelled {
		return task, errors.New("task cleanup requires done or cancelled status")
	}

	stepInput, err := stepContext(s.store.Root(), cfg, task)
	if err != nil {
		return model.TaskState{}, err
	}
	phaseResult, err := s.runner.RunPhase(context.Background(), model.TaskCleanupPhase, cfg.Steps[model.TaskCleanupPhase], stepInput)
	if err != nil {
		return model.TaskState{}, err
	}
	s.syncArtifacts(&task, phaseResult)

	if !phaseResult.OK {
		applyFailedOperation(&task, "tasks.cleanup", phaseResult)
		task.UpdatedAt = s.now().UTC().Format(time.RFC3339)
		if saveErr := s.store.SaveTask(task); saveErr != nil {
			return model.TaskState{}, saveErr
		}
		return task, errors.New(task.LastOp.Error)
	}

	if task.Worktree.Status == model.WorktreeStatusPresent {
		task.Worktree.Status = model.WorktreeStatusCleaned
	}
	task.UpdatedAt = s.now().UTC().Format(time.RFC3339)
	applySucceededOperation(&task, "tasks.cleanup", phaseResult)
	if err := s.store.SaveTask(task); err != nil {
		return model.TaskState{}, err
	}

	return task, nil
}

func renderTaskSlug(pattern, repo, name string) (string, error) {
	if pattern == "" {
		return repo + "-" + name, nil
	}

	tmpl, err := template.New("task_slug").Option("missingkey=error").Parse(pattern)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	if err := tmpl.Execute(&builder, map[string]string{"repo": repo, "name": name}); err != nil {
		return "", err
	}

	return builder.String(), nil
}

func mergeMap(base, override map[string]string) map[string]string {
	merged := map[string]string{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func initialMRStatus(traits map[string]string) model.MRStatus {
	if traits["mr"] == "not-needed" {
		return model.MRStatusNotNeeded
	}
	return model.MRStatusMissing
}

func stepContext(runtimeRoot string, cfg model.Config, task model.TaskState) (steps.Context, error) {
	repoRoot := resolveRepoRoot(runtimeRoot, cfg, task.Repo)
	branchName, err := renderStringTemplate(cfg.Naming.Branch, map[string]any{
		"project": map[string]string{"slug": task.Project},
		"task":    map[string]string{"slug": task.Slug},
	})
	if err != nil {
		return steps.Context{}, err
	}
	worktreePath, err := renderStringTemplate(cfg.Naming.Worktree, map[string]any{
		"repo_root": repoRoot,
		"project":   map[string]string{"slug": task.Project},
		"task":      map[string]string{"slug": task.Slug},
	})
	if err != nil {
		return steps.Context{}, err
	}
	taskDir := filepath.Join(runtimeRoot, "projects", "active", task.Project, "tasks", task.Slug)
	return steps.Context{
		RuntimeRoot:  runtimeRoot,
		ProjectSlug:  task.Project,
		TaskSlug:     task.Slug,
		TaskRepo:     task.Repo,
		TaskTraits:   task.Traits,
		RepoRoot:     repoRoot,
		BranchName:   branchName,
		WorktreePath: worktreePath,
		TaskDir:      taskDir,
		ArtifactsDir: filepath.Join(taskDir, "artifacts"),
	}, nil
}

func (s TaskService) syncArtifacts(task *model.TaskState, result steps.PhaseResult) {
	for _, exec := range result.Steps {
		for kind, data := range exec.Result.Artifacts {
			_ = s.store.SaveArtifact(task.Project, task.Slug, kind, data)
			s.applyArtifactSummary(task, kind, data)
		}
	}
}

func (s TaskService) applyArtifactSummary(task *model.TaskState, kind string, data map[string]string) {
	switch kind {
	case "worktree":
		if status, ok := data["status"]; ok {
			task.Worktree.Status = model.WorktreeStatus(status)
		}
	case "mr":
		if status, ok := data["status"]; ok {
			task.MR.Status = model.MRStatus(status)
		}
		if reason, ok := data["reason"]; ok && reason != "" {
			task.MR.Reason = &reason
		}
	case "branch":
	case "repository":
	default:
		return
	}
}

func resolveRepoRoot(runtimeRoot string, cfg model.Config, repo string) string {
	if cfg.Repos != nil {
		if rel, ok := cfg.Repos[repo]; ok && rel != "" {
			return filepath.Clean(filepath.Join(runtimeRoot, rel))
		}
	}
	return filepath.Clean(filepath.Join(runtimeRoot, "..", repo))
}

func renderStringTemplate(pattern string, values map[string]any) (string, error) {
	if pattern == "" {
		return "", nil
	}
	tmpl, err := template.New("runtime").Option("missingkey=error").Parse(pattern)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	if err := tmpl.Execute(&builder, values); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func applyFailedOperation(task *model.TaskState, cmd string, result steps.PhaseResult) {
	message := "step execution failed"
	if len(result.Steps) > 0 {
		message = result.Steps[len(result.Steps)-1].Result.Message
	}
	task.LastOp = model.OperationState{
		Cmd:   cmd,
		OK:    false,
		Step:  result.FailedStep,
		Error: message,
		At:    time.Now().UTC().Format(time.RFC3339),
	}
}

func applySucceededOperation(task *model.TaskState, cmd string, result steps.PhaseResult) {
	task.LastOp = model.OperationState{
		Cmd: cmd,
		OK:  true,
		At:  time.Now().UTC().Format(time.RFC3339),
	}
	if len(result.Steps) > 0 {
		task.LastOp.Step = result.Steps[len(result.Steps)-1].Name
	}
}
