package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akhmanov/taskman/internal/model"
)

type Store struct {
	root string
}

func New(root string) Store {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	return Store{root: root}
}

func (s Store) Root() string {
	return s.root
}

func (s Store) LoadOptionalConfig() (model.Config, bool, error) {
	var cfg model.Config
	err := readYAML(s.configPath(), &cfg)
	if err != nil {
		if os.IsNotExist(err) {
			return model.Config{}, false, nil
		}
		return model.Config{}, true, err
	}
	if err := cfg.Validate(); err != nil {
		return model.Config{}, true, err
	}
	return cfg, true, nil
}

func (s Store) InitConfig() error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	return writeIfMissing(s.configPath(), []byte(model.DefaultConfigYAML))
}

func (s Store) CreateProject(manifest model.Manifest) error {
	if _, err := os.Stat(s.projectManifestPath(manifest.Slug)); err == nil {
		return fmt.Errorf("project %s already exists", manifest.Slug)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.projectDir(manifest.Slug), "tasks"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.projectEventsDir(manifest.Slug), 0o755); err != nil {
		return err
	}
	return writeJSON(s.projectManifestPath(manifest.Slug), manifest)
}

func (s Store) CreateTask(manifest model.Manifest) error {
	if _, err := os.Stat(s.taskManifestPath(manifest.ProjectSlug, manifest.Slug)); err == nil {
		return fmt.Errorf("task %s/%s already exists", manifest.ProjectSlug, manifest.Slug)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(s.taskEventsDir(manifest.ProjectSlug, manifest.Slug), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.taskArtifactsDir(manifest.ProjectSlug, manifest.Slug), 0o755); err != nil {
		return err
	}
	return writeJSON(s.taskManifestPath(manifest.ProjectSlug, manifest.Slug), manifest)
}

func (s Store) LoadProjectManifest(slug string) (model.Manifest, error) {
	var manifest model.Manifest
	err := readJSON(s.projectManifestPath(slug), &manifest)
	return manifest, err
}

func (s Store) LoadTaskManifest(projectSlug, taskSlug string) (model.Manifest, error) {
	var manifest model.Manifest
	err := readJSON(s.taskManifestPath(projectSlug, taskSlug), &manifest)
	return manifest, err
}

func (s Store) SaveArtifact(projectSlug, taskSlug, kind string, data map[string]any) error {
	state := model.ArtifactState{Data: data}
	return writeJSON(filepath.Join(s.taskArtifactsDir(projectSlug, taskSlug), kind+".json"), state)
}

func (s Store) ListArtifacts(projectSlug, taskSlug string) (map[string]map[string]any, error) {
	dir := s.taskArtifactsDir(projectSlug, taskSlug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]any{}, nil
		}
		return nil, err
	}
	artifacts := map[string]map[string]any{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var state model.ArtifactState
		if err := readJSON(filepath.Join(dir, entry.Name()), &state); err != nil {
			return nil, err
		}
		kind := strings.TrimSuffix(entry.Name(), ".json")
		artifacts[kind] = state.Data
	}
	return artifacts, nil
}

func (s Store) AppendProjectEvent(slug string, event model.Event) error {
	return s.appendEvent(s.projectEventsDir(slug), event)
}

func (s Store) AppendTaskEvent(projectSlug, taskSlug string, event model.Event) error {
	return s.appendEvent(s.taskEventsDir(projectSlug, taskSlug), event)
}

func (s Store) ListProjectEvents(slug string) ([]model.Event, error) {
	return s.listEvents(s.projectEventsDir(slug))
}

func (s Store) ListTaskEvents(projectSlug, taskSlug string) ([]model.Event, error) {
	return s.listEvents(s.taskEventsDir(projectSlug, taskSlug))
}

func (s Store) LoadProject(slug string) (model.ProjectRecord, error) {
	manifest, err := s.LoadProjectManifest(slug)
	if err != nil {
		return model.ProjectRecord{}, err
	}
	events, err := s.ListProjectEvents(slug)
	if err != nil {
		return model.ProjectRecord{}, err
	}
	state := model.ProjectStateFromEvents(events)
	if state.UpdatedAt == "" {
		state.UpdatedAt = manifest.CreatedAt
	}
	return model.ProjectRecord{Manifest: manifest, State: state}, nil
}

func (s Store) LoadTask(projectSlug, taskSlug string) (model.TaskRecord, error) {
	manifest, err := s.LoadTaskManifest(projectSlug, taskSlug)
	if err != nil {
		return model.TaskRecord{}, err
	}
	events, err := s.ListTaskEvents(projectSlug, taskSlug)
	if err != nil {
		return model.TaskRecord{}, err
	}
	state := model.TaskStateFromEvents(events)
	if state.UpdatedAt == "" {
		state.UpdatedAt = manifest.CreatedAt
	}
	return model.TaskRecord{Manifest: manifest, State: state}, nil
}

func (s Store) ListProjects() ([]model.ProjectRecord, error) {
	entries, err := os.ReadDir(s.projectsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	projects := make([]model.ProjectRecord, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record, err := s.LoadProject(entry.Name())
		if err != nil {
			return nil, err
		}
		projects = append(projects, record)
	}
	return projects, nil
}

func (s Store) ListTasks(projectSlug string) ([]model.TaskRecord, error) {
	if strings.TrimSpace(projectSlug) == "" {
		return nil, fmt.Errorf("project is required")
	}
	entries, err := os.ReadDir(filepath.Join(s.projectDir(projectSlug), "tasks"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	tasks := make([]model.TaskRecord, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record, err := s.LoadTask(projectSlug, entry.Name())
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, record)
	}
	return tasks, nil
}

func (s Store) appendEvent(dir string, event model.Event) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := eventFileName(event)
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("event %s already exists", event.ID)
	}
	return writeJSON(path, event)
}

func (s Store) listEvents(dir string) ([]model.Event, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.Event{}, nil
		}
		return nil, err
	}
	events := make([]model.Event, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		var event model.Event
		if err := readJSON(filepath.Join(dir, entry.Name()), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].At != events[j].At {
			return events[i].At < events[j].At
		}
		return events[i].ID < events[j].ID
	})
	return events, nil
}

func eventFileName(event model.Event) string {
	prefix := strings.ReplaceAll(event.At, ":", "")
	prefix = strings.ReplaceAll(prefix, ".", "-")
	return fmt.Sprintf("%s_%s_%s.json", prefix, event.Kind, event.ID)
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
