package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/akhmanov/taskman/internal/model"
	"gopkg.in/yaml.v3"
)

type Store struct {
	root string
}

const emptyEventsTemplate = "[]\n"

func New(root string) Store {
	return Store{root: root}
}

func (s Store) Root() string {
	return s.root
}

func (s Store) LoadConfig() (model.Config, error) {
	var cfg model.Config
	err := readYAML(filepath.Join(s.root, "config.yaml"), &cfg)
	if err != nil {
		return model.Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return model.Config{}, err
	}

	return cfg, nil
}

func (s Store) ScaffoldProject(project model.ProjectState) error {
	if project.Slug == "" {
		return fmt.Errorf("project slug is required")
	}

	projectDir := s.projectDir(project.Slug)
	if err := os.MkdirAll(filepath.Join(projectDir, "tasks"), 0o755); err != nil {
		return err
	}

	if err := writeIfMissing(filepath.Join(projectDir, "overview.md"), []byte("\n")); err != nil {
		return err
	}
	if err := writeIfMissing(s.projectBriefPath(project.Slug), []byte(model.ProjectBriefTemplate)); err != nil {
		return err
	}
	if err := writeIfMissing(s.projectEventsPath(project.Slug), []byte(emptyEventsTemplate)); err != nil {
		return err
	}

	return writeYAML(filepath.Join(projectDir, "state.yaml"), project)
}

func (s Store) LoadProject(slug string) (model.ProjectState, error) {
	var state model.ProjectState
	err := readYAML(filepath.Join(s.projectDir(slug), "state.yaml"), &state)
	return state, err
}

func (s Store) SaveProject(project model.ProjectState) error {
	return writeYAML(filepath.Join(s.projectDir(project.Slug), "state.yaml"), project)
}

func (s Store) ScaffoldTask(task model.TaskState) error {
	if task.Project == "" || task.Slug == "" {
		return fmt.Errorf("task project and slug are required")
	}

	taskDir := s.taskDir(task.Project, task.Slug)
	if err := os.MkdirAll(filepath.Join(taskDir, "sessions"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(taskDir, "artifacts"), 0o755); err != nil {
		return err
	}

	if err := writeIfMissing(filepath.Join(taskDir, "overview.md"), []byte("\n")); err != nil {
		return err
	}
	if err := writeIfMissing(s.taskBriefPath(task.Project, task.Slug), []byte(model.TaskBriefTemplate)); err != nil {
		return err
	}
	if err := writeIfMissing(s.taskEventsPath(task.Project, task.Slug), []byte(emptyEventsTemplate)); err != nil {
		return err
	}

	if task.Session.Active != "" {
		if err := os.MkdirAll(filepath.Join(taskDir, "sessions", task.Session.Active), 0o755); err != nil {
			return err
		}
	}

	return writeYAML(filepath.Join(taskDir, "state.yaml"), task)
}

func (s Store) LoadTask(projectSlug, taskSlug string) (model.TaskState, error) {
	var state model.TaskState
	err := readYAML(filepath.Join(s.taskDir(projectSlug, taskSlug), "state.yaml"), &state)
	return state, err
}

func (s Store) SaveTask(task model.TaskState) error {
	return writeYAML(filepath.Join(s.taskDir(task.Project, task.Slug), "state.yaml"), task)
}

func (s Store) LoadProjectBrief(slug string) (string, error) {
	data, err := os.ReadFile(s.projectBriefPath(slug))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func (s Store) SaveProjectBrief(slug, brief string) error {
	if err := os.MkdirAll(s.projectDir(slug), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.projectBriefPath(slug), []byte(brief), 0o644)
}

func (s Store) AppendProjectEvent(slug string, event model.PayloadEvent) error {
	events, err := s.ListProjectEvents(slug)
	if err != nil {
		return err
	}
	events = append(events, event)
	return s.saveProjectEvents(slug, events)
}

func (s Store) ListProjectEvents(slug string) ([]model.PayloadEvent, error) {
	return s.loadEvents(s.projectEventsPath(slug))
}

func (s Store) saveProjectEvents(slug string, events []model.PayloadEvent) error {
	return s.saveEvents(s.projectEventsPath(slug), events)
}

func (s Store) SaveSession(projectSlug, taskSlug string, session model.SessionState) error {
	return writeYAML(filepath.Join(s.sessionDir(projectSlug, taskSlug, session.ID), "state.yaml"), session)
}

func (s Store) LoadTaskBrief(projectSlug, taskSlug string) (string, error) {
	data, err := os.ReadFile(s.taskBriefPath(projectSlug, taskSlug))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func (s Store) SaveTaskBrief(projectSlug, taskSlug, brief string) error {
	if err := os.MkdirAll(s.taskDir(projectSlug, taskSlug), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.taskBriefPath(projectSlug, taskSlug), []byte(brief), 0o644)
}

func (s Store) AppendTaskEvent(projectSlug, taskSlug string, event model.PayloadEvent) error {
	events, err := s.ListTaskEvents(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	events = append(events, event)
	return s.saveTaskEvents(projectSlug, taskSlug, events)
}

func (s Store) ListTaskEvents(projectSlug, taskSlug string) ([]model.PayloadEvent, error) {
	return s.loadEvents(s.taskEventsPath(projectSlug, taskSlug))
}

func (s Store) saveTaskEvents(projectSlug, taskSlug string, events []model.PayloadEvent) error {
	return s.saveEvents(s.taskEventsPath(projectSlug, taskSlug), events)
}

func (s Store) LoadSession(projectSlug, taskSlug, sessionID string) (model.SessionState, error) {
	var state model.SessionState
	err := readYAML(filepath.Join(s.sessionDir(projectSlug, taskSlug, sessionID), "state.yaml"), &state)
	return state, err
}

func (s Store) SaveArtifact(projectSlug, taskSlug, kind string, data map[string]any) error {
	state := model.ArtifactState{Version: 1, Data: data}
	return writeYAML(filepath.Join(s.taskDir(projectSlug, taskSlug), "artifacts", kind+".yaml"), state)
}

func (s Store) LoadArtifact(projectSlug, taskSlug, kind string) (model.ArtifactState, error) {
	var state model.ArtifactState
	err := readYAML(filepath.Join(s.taskDir(projectSlug, taskSlug), "artifacts", kind+".yaml"), &state)
	return state, err
}

func (s Store) ListArtifacts(projectSlug, taskSlug string) (map[string]map[string]any, error) {
	dir := filepath.Join(s.taskDir(projectSlug, taskSlug), "artifacts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]any{}, nil
		}
		return nil, err
	}

	artifacts := map[string]map[string]any{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		kind := strings.TrimSuffix(entry.Name(), ".yaml")
		artifact, err := s.LoadArtifact(projectSlug, taskSlug, kind)
		if err != nil {
			return nil, err
		}
		if len(artifact.Data) > 0 {
			artifacts[kind] = artifact.Data
		}
	}

	return artifacts, nil
}

func (s Store) loadEvents(path string) ([]model.PayloadEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.PayloadEvent{}, nil
		}
		return nil, err
	}

	var events []model.PayloadEvent
	if len(data) == 0 {
		return []model.PayloadEvent{}, nil
	}
	if err := yaml.Unmarshal(data, &events); err != nil {
		return nil, err
	}
	if events == nil {
		return []model.PayloadEvent{}, nil
	}

	return events, nil
}

func (s Store) saveEvents(path string, events []model.PayloadEvent) error {
	data, err := yaml.Marshal(events)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (s Store) ArchiveProject(project model.ProjectState, year int) error {
	if err := writeYAML(filepath.Join(s.projectDir(project.Slug), "state.yaml"), project); err != nil {
		return err
	}

	source := s.projectDir(project.Slug)
	target := filepath.Join(s.root, "projects", "archive", fmt.Sprintf("%d", year), project.Slug)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.Rename(source, target)
}

func (s Store) ListProjects() ([]model.ProjectState, error) {
	entries, err := os.ReadDir(s.activeProjectsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	projects := make([]model.ProjectState, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		project, err := s.LoadProject(entry.Name())
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, nil
}

func (s Store) ListTasks(projectSlug string) ([]model.TaskState, error) {
	if projectSlug == "" {
		projects, err := s.ListProjects()
		if err != nil {
			return nil, err
		}
		all := make([]model.TaskState, 0)
		for _, project := range projects {
			tasks, err := s.ListTasks(project.Slug)
			if err != nil {
				return nil, err
			}
			all = append(all, tasks...)
		}
		return all, nil
	}

	dir := filepath.Join(s.projectDir(projectSlug), "tasks")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	tasks := make([]model.TaskState, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		task, err := s.LoadTask(projectSlug, entry.Name())
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}
