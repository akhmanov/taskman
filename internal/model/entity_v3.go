package model

import "fmt"

type EntityKind string

const (
	EntityKindProject EntityKind = "project"
	EntityKindTask    EntityKind = "task"
)

type Manifest struct {
	ID            string     `json:"id"`
	Kind          EntityKind `json:"kind"`
	Number        int        `json:"number,omitempty"`
	Slug          string     `json:"slug"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	ProjectID     string     `json:"project_id,omitempty"`
	ProjectNumber int        `json:"project_number,omitempty"`
	ProjectSlug   string     `json:"project_slug,omitempty"`
	CreatedAt     string     `json:"created_at"`
}

func CanonicalRef(number int, slug string) string {
	if number <= 0 || slug == "" {
		return slug
	}
	return fmt.Sprintf("%d_%s", number, slug)
}

func (m Manifest) Ref() string {
	return CanonicalRef(m.Number, m.Slug)
}

func (m Manifest) ProjectRef() string {
	return CanonicalRef(m.ProjectNumber, m.ProjectSlug)
}

type ProjectionState struct {
	Status         Status
	StatusDetail   StatusDetail
	Labels         []string
	Vars           map[string]string
	UpdatedAt      string
	CurrentHeadID  string
	UnresolvedHead []string
	Warnings       []string
}

func (p ProjectionState) HasConflict() bool {
	return len(p.UnresolvedHead) > 1
}

type ProjectRecord struct {
	Manifest Manifest
	State    ProjectionState
}

type TaskRecord struct {
	Manifest Manifest
	State    ProjectionState
}
