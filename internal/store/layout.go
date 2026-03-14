package store

import "path/filepath"

func (s Store) activeProjectsDir() string {
	return filepath.Join(s.root, "projects", "active")
}

func (s Store) projectDir(slug string) string {
	return filepath.Join(s.activeProjectsDir(), slug)
}

func (s Store) projectBriefPath(slug string) string {
	return filepath.Join(s.projectDir(slug), "brief.md")
}

func (s Store) projectEventsPath(slug string) string {
	return filepath.Join(s.projectDir(slug), "events.yaml")
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

func (s Store) sessionDir(projectSlug, taskSlug, sessionID string) string {
	return filepath.Join(s.taskDir(projectSlug, taskSlug), "sessions", sessionID)
}
