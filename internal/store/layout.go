package store

import "path/filepath"

func (s Store) activeProjectsDir() string {
	return filepath.Join(s.root, "projects", "active")
}

func (s Store) projectDir(slug string) string {
	return filepath.Join(s.activeProjectsDir(), slug)
}

func (s Store) taskDir(projectSlug, taskSlug string) string {
	return filepath.Join(s.projectDir(projectSlug), "tasks", taskSlug)
}

func (s Store) sessionDir(projectSlug, taskSlug, sessionID string) string {
	return filepath.Join(s.taskDir(projectSlug, taskSlug), "sessions", sessionID)
}
