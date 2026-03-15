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

func (s Store) projectBriefPath(slug string) string {
	return filepath.Join(s.projectDir(slug), "brief.md")
}

func (s Store) projectEventsPath(slug string) string {
	return filepath.Join(s.projectDir(slug), "events.yaml")
}

func (s Store) projectTransitionsPath(slug string) string {
	return filepath.Join(s.projectDir(slug), "transitions.yaml")
}

func (s Store) taskDir(projectSlug, taskSlug string) string {
	return filepath.Join(s.projectDir(projectSlug), "tasks", taskSlug)
}

func (s Store) taskBriefPath(projectSlug, taskSlug string) string {
	return filepath.Join(s.taskDir(projectSlug, taskSlug), "brief.md")
}

func (s Store) taskEventsPath(projectSlug, taskSlug string) string {
	return filepath.Join(s.taskDir(projectSlug, taskSlug), "events.yaml")
}

func (s Store) taskTransitionsPath(projectSlug, taskSlug string) string {
	return filepath.Join(s.taskDir(projectSlug, taskSlug), "transitions.yaml")
}
