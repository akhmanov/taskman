package store

import "path/filepath"

func (s Store) configPath() string {
	return filepath.Join(s.root, "taskman.yaml")
}

func (s Store) projectsDir() string {
	return filepath.Join(s.root, "projects")
}

func (s Store) projectDir(slug string) string {
	return filepath.Join(s.projectsDir(), slug)
}

func (s Store) projectManifestPath(slug string) string {
	return filepath.Join(s.projectDir(slug), "manifest.json")
}

func (s Store) projectEventsDir(slug string) string {
	return filepath.Join(s.projectDir(slug), "events")
}

func (s Store) taskDir(projectSlug, taskSlug string) string {
	return filepath.Join(s.projectDir(projectSlug), "tasks", taskSlug)
}

func (s Store) taskManifestPath(projectSlug, taskSlug string) string {
	return filepath.Join(s.taskDir(projectSlug, taskSlug), "manifest.json")
}

func (s Store) taskEventsDir(projectSlug, taskSlug string) string {
	return filepath.Join(s.taskDir(projectSlug, taskSlug), "events")
}

func (s Store) taskArtifactsDir(projectSlug, taskSlug string) string {
	return filepath.Join(s.taskDir(projectSlug, taskSlug), "artifacts")
}
