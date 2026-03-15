package model

type EntityKind string

const (
	EntityKindProject EntityKind = "project"
	EntityKindTask    EntityKind = "task"
)

type Manifest struct {
	ID          string     `json:"id"`
	Kind        EntityKind `json:"kind"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ProjectID   string     `json:"project_id,omitempty"`
	ProjectSlug string     `json:"project_slug,omitempty"`
	CreatedAt   string     `json:"created_at"`
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
