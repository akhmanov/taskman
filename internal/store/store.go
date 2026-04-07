package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	if err := s.ensureProjectCreateAllowed(manifest); err != nil {
		return err
	}
	if _, err := os.Stat(s.projectManifestPath(manifest.Ref())); err == nil {
		return fmt.Errorf("project %s already exists", manifest.Slug)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Join(s.projectDir(manifest.Ref()), "tasks"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.projectEventsDir(manifest.Ref()), 0o755); err != nil {
		return err
	}
	return writeJSON(s.projectManifestPath(manifest.Ref()), manifest)
}

func (s Store) CreateTask(manifest model.Manifest) error {
	projectRef := manifest.ProjectRef()
	if projectRef == "" {
		projectRef = manifest.ProjectSlug
	}
	if projectRef != "" {
		project, err := s.LoadProject(projectRef)
		if err == nil {
			manifest.ProjectID = project.Manifest.ID
			manifest.ProjectNumber = project.Manifest.Number
			manifest.ProjectSlug = project.Manifest.Slug
		}
	}
	if err := s.ensureTaskCreateAllowed(manifest); err != nil {
		return err
	}
	if _, err := os.Stat(s.taskManifestPath(manifest.ProjectRef(), manifest.Ref())); err == nil {
		return fmt.Errorf("task %s/%s already exists", manifest.ProjectSlug, manifest.Slug)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(s.taskEventsDir(manifest.ProjectRef(), manifest.Ref()), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(s.taskArtifactsDir(manifest.ProjectRef(), manifest.Ref()), 0o755); err != nil {
		return err
	}
	return writeJSON(s.taskManifestPath(manifest.ProjectRef(), manifest.Ref()), manifest)
}

func (s Store) LoadProjectManifest(slug string) (model.Manifest, error) {
	dirName, err := s.resolveProjectDirName(slug)
	if err != nil {
		return model.Manifest{}, err
	}
	var manifest model.Manifest
	err = readJSON(s.projectManifestPath(dirName), &manifest)
	return manifest, err
}

func (s Store) LoadTaskManifest(projectSlug, taskSlug string) (model.Manifest, error) {
	projectDirName, taskDirName, err := s.resolveTaskDirNames(projectSlug, taskSlug)
	if err != nil {
		return model.Manifest{}, err
	}
	var manifest model.Manifest
	err = readJSON(s.taskManifestPath(projectDirName, taskDirName), &manifest)
	return manifest, err
}

func (s Store) SaveArtifact(projectSlug, taskSlug, kind string, data map[string]any) error {
	projectDirName, taskDirName, err := s.resolveTaskDirNames(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	state := model.ArtifactState{Data: data}
	return writeJSON(filepath.Join(s.taskArtifactsDir(projectDirName, taskDirName), kind+".json"), state)
}

func (s Store) ListArtifacts(projectSlug, taskSlug string) (map[string]map[string]any, error) {
	projectDirName, taskDirName, err := s.resolveTaskDirNames(projectSlug, taskSlug)
	if err != nil {
		return nil, err
	}
	dir := s.taskArtifactsDir(projectDirName, taskDirName)
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
	dirName, err := s.resolveProjectDirName(slug)
	if err != nil {
		return err
	}
	return s.appendEvent(s.projectEventsDir(dirName), event)
}

func (s Store) AppendTaskEvent(projectSlug, taskSlug string, event model.Event) error {
	projectDirName, taskDirName, err := s.resolveTaskDirNames(projectSlug, taskSlug)
	if err != nil {
		return err
	}
	return s.appendEvent(s.taskEventsDir(projectDirName, taskDirName), event)
}

func (s Store) ListProjectEvents(slug string) ([]model.Event, error) {
	dirName, err := s.resolveProjectDirName(slug)
	if err != nil {
		return nil, err
	}
	return s.listEvents(s.projectEventsDir(dirName))
}

func (s Store) ListTaskEvents(projectSlug, taskSlug string) ([]model.Event, error) {
	projectDirName, taskDirName, err := s.resolveTaskDirNames(projectSlug, taskSlug)
	if err != nil {
		return nil, err
	}
	return s.listEvents(s.taskEventsDir(projectDirName, taskDirName))
}

func (s Store) LoadProject(slug string) (model.ProjectRecord, error) {
	dirName, err := s.resolveProjectDirName(slug)
	if err != nil {
		return model.ProjectRecord{}, err
	}
	return s.loadProjectByDirName(dirName)
}

func (s Store) LoadTask(projectSlug, taskSlug string) (model.TaskRecord, error) {
	projectDirName, taskDirName, err := s.resolveTaskDirNames(projectSlug, taskSlug)
	if err != nil {
		return model.TaskRecord{}, err
	}
	return s.loadTaskByDirNames(projectDirName, taskDirName)
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
		record, err := s.loadProjectByDirName(entry.Name())
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
	projectDirName, err := s.resolveProjectDirName(projectSlug)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(s.projectDir(projectDirName), "tasks"))
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
		record, err := s.loadTaskByDirNames(projectDirName, entry.Name())
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, record)
	}
	return tasks, nil
}

func (s Store) NextProjectNumber() (int, error) {
	projects, err := s.ListProjects()
	if err != nil {
		return 0, err
	}
	maxNumber := 0
	for _, project := range projects {
		if project.Manifest.Number > maxNumber {
			maxNumber = project.Manifest.Number
		}
	}
	return maxNumber + 1, nil
}

func (s Store) NextTaskNumber(projectRef string) (int, error) {
	tasks, err := s.ListTasks(projectRef)
	if err != nil {
		return 0, err
	}
	maxNumber := 0
	for _, task := range tasks {
		if task.Manifest.Number > maxNumber {
			maxNumber = task.Manifest.Number
		}
	}
	return maxNumber + 1, nil
}

func (s Store) RenameProject(ref, newSlug string) (model.ProjectRecord, error) {
	oldDirName, err := s.resolveProjectDirName(ref)
	if err != nil {
		return model.ProjectRecord{}, err
	}
	record, err := s.loadProjectByDirName(oldDirName)
	if err != nil {
		return model.ProjectRecord{}, err
	}
	manifest := record.Manifest
	manifest.Slug = newSlug
	if err := s.ensureProjectRenameAllowed(record.Manifest.ID, manifest); err != nil {
		return model.ProjectRecord{}, err
	}
	newDirName := manifest.Ref()
	if oldDirName != newDirName {
		if err := os.Rename(s.projectDir(oldDirName), s.projectDir(newDirName)); err != nil {
			return model.ProjectRecord{}, err
		}
	}
	if err := writeJSON(s.projectManifestPath(newDirName), manifest); err != nil {
		return model.ProjectRecord{}, err
	}
	tasks, err := s.taskEntries(newDirName)
	if err != nil {
		return model.ProjectRecord{}, err
	}
	for _, entry := range tasks {
		taskManifest := entry.manifest
		taskManifest.ProjectSlug = manifest.Slug
		taskManifest.ProjectNumber = manifest.Number
		if err := writeJSON(s.taskManifestPath(newDirName, entry.dirName), taskManifest); err != nil {
			return model.ProjectRecord{}, err
		}
	}
	return s.loadProjectByDirName(newDirName)
}

func (s Store) RenameTask(projectRef, taskRef, newSlug string) (model.TaskRecord, error) {
	projectDirName, oldTaskDirName, err := s.resolveTaskDirNames(projectRef, taskRef)
	if err != nil {
		return model.TaskRecord{}, err
	}
	record, err := s.loadTaskByDirNames(projectDirName, oldTaskDirName)
	if err != nil {
		return model.TaskRecord{}, err
	}
	manifest := record.Manifest
	manifest.Slug = newSlug
	if err := s.ensureTaskRenameAllowed(projectDirName, record.Manifest.ID, manifest); err != nil {
		return model.TaskRecord{}, err
	}
	newTaskDirName := manifest.Ref()
	if oldTaskDirName != newTaskDirName {
		if err := os.Rename(s.taskDir(projectDirName, oldTaskDirName), s.taskDir(projectDirName, newTaskDirName)); err != nil {
			return model.TaskRecord{}, err
		}
	}
	if err := writeJSON(s.taskManifestPath(projectDirName, newTaskDirName), manifest); err != nil {
		return model.TaskRecord{}, err
	}
	return s.loadTaskByDirNames(projectDirName, newTaskDirName)
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

type manifestEntry struct {
	dirName  string
	manifest model.Manifest
}

func (s Store) loadProjectByDirName(dirName string) (model.ProjectRecord, error) {
	var manifest model.Manifest
	if err := readJSON(s.projectManifestPath(dirName), &manifest); err != nil {
		return model.ProjectRecord{}, err
	}
	events, err := s.listEvents(s.projectEventsDir(dirName))
	if err != nil {
		return model.ProjectRecord{}, err
	}
	state := model.ProjectStateFromEvents(events)
	if state.UpdatedAt == "" {
		state.UpdatedAt = manifest.CreatedAt
	}
	return model.ProjectRecord{Manifest: manifest, State: state}, nil
}

func (s Store) loadTaskByDirNames(projectDirName, taskDirName string) (model.TaskRecord, error) {
	var manifest model.Manifest
	if err := readJSON(s.taskManifestPath(projectDirName, taskDirName), &manifest); err != nil {
		return model.TaskRecord{}, err
	}
	events, err := s.listEvents(s.taskEventsDir(projectDirName, taskDirName))
	if err != nil {
		return model.TaskRecord{}, err
	}
	state := model.TaskStateFromEvents(events)
	if state.UpdatedAt == "" {
		state.UpdatedAt = manifest.CreatedAt
	}
	return model.TaskRecord{Manifest: manifest, State: state}, nil
}

func (s Store) ensureProjectCreateAllowed(manifest model.Manifest) error {
	entries, err := s.projectEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.manifest.Slug == manifest.Slug || entry.manifest.Number == manifest.Number || entry.manifest.Ref() == manifest.Ref() {
			return fmt.Errorf("project %s already exists", manifest.Slug)
		}
	}
	return nil
}

func (s Store) ensureProjectRenameAllowed(currentID string, manifest model.Manifest) error {
	entries, err := s.projectEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.manifest.ID == currentID {
			continue
		}
		if entry.manifest.Slug == manifest.Slug || entry.manifest.Ref() == manifest.Ref() {
			return fmt.Errorf("project %s already exists", manifest.Slug)
		}
	}
	return nil
}

func (s Store) ensureTaskCreateAllowed(manifest model.Manifest) error {
	entries, err := s.taskEntries(manifest.ProjectRef())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.manifest.Slug == manifest.Slug || entry.manifest.Number == manifest.Number || entry.manifest.Ref() == manifest.Ref() {
			return fmt.Errorf("task %s/%s already exists", manifest.ProjectSlug, manifest.Slug)
		}
	}
	return nil
}

func (s Store) ensureTaskRenameAllowed(projectRef, currentID string, manifest model.Manifest) error {
	entries, err := s.taskEntries(projectRef)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.manifest.ID == currentID {
			continue
		}
		if entry.manifest.Slug == manifest.Slug || entry.manifest.Ref() == manifest.Ref() {
			return fmt.Errorf("task %s/%s already exists", manifest.ProjectSlug, manifest.Slug)
		}
	}
	return nil
}

func (s Store) resolveProjectDirName(ref string) (string, error) {
	entries, err := s.projectEntries()
	if err != nil {
		return "", err
	}
	matches := []string{}
	for _, entry := range entries {
		if manifestMatchesRef(entry.manifest, entry.dirName, ref) {
			matches = append(matches, entry.dirName)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("project %s not found", ref)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("project ref %s is ambiguous", ref)
	}
	return matches[0], nil
}

func (s Store) resolveTaskDirNames(projectRef, taskRef string) (string, string, error) {
	projectDirName, err := s.resolveProjectDirName(projectRef)
	if err != nil {
		return "", "", err
	}
	entries, err := s.taskEntries(projectDirName)
	if err != nil {
		return "", "", err
	}
	matches := []string{}
	for _, entry := range entries {
		if manifestMatchesRef(entry.manifest, entry.dirName, taskRef) {
			matches = append(matches, entry.dirName)
		}
	}
	if len(matches) == 0 {
		return "", "", fmt.Errorf("task %s/%s not found", projectRef, taskRef)
	}
	if len(matches) > 1 {
		return "", "", fmt.Errorf("task ref %s is ambiguous in project %s", taskRef, projectRef)
	}
	return projectDirName, matches[0], nil
}

func (s Store) projectEntries() ([]manifestEntry, error) {
	entries, err := os.ReadDir(s.projectsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	manifests := make([]manifestEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var manifest model.Manifest
		if err := readJSON(s.projectManifestPath(entry.Name()), &manifest); err != nil {
			return nil, err
		}
		manifests = append(manifests, manifestEntry{dirName: entry.Name(), manifest: manifest})
	}
	return manifests, nil
}

func (s Store) taskEntries(projectRef string) ([]manifestEntry, error) {
	projectDirName, err := s.resolveProjectDirName(projectRef)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(s.projectDir(projectDirName), "tasks"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	manifests := make([]manifestEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var manifest model.Manifest
		if err := readJSON(s.taskManifestPath(projectDirName, entry.Name()), &manifest); err != nil {
			return nil, err
		}
		manifests = append(manifests, manifestEntry{dirName: entry.Name(), manifest: manifest})
	}
	return manifests, nil
}

func manifestMatchesRef(manifest model.Manifest, dirName, ref string) bool {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return false
	}
	if trimmed == dirName || trimmed == manifest.Ref() || trimmed == manifest.Slug {
		return true
	}
	shortNumber := strings.TrimPrefix(trimmed, "#")
	return manifest.Number > 0 && shortNumber == strconv.Itoa(manifest.Number)
}
