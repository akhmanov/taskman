package model

type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusBlocked  ProjectStatus = "blocked"
	ProjectStatusDone     ProjectStatus = "done"
	ProjectStatusArchived ProjectStatus = "archived"
)

type ProjectState struct {
	Version   int               `json:"version" yaml:"version"`
	Slug      string            `json:"slug" yaml:"slug"`
	Status    ProjectStatus     `json:"status" yaml:"status"`
	Labels    []string          `json:"labels,omitempty" yaml:"labels,omitempty"`
	Traits    map[string]string `json:"traits,omitempty" yaml:"traits,omitempty"`
	CreatedAt string            `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt string            `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	Tasks     TaskCounts        `json:"tasks,omitempty" yaml:"tasks,omitempty"`
	Archive   ArchiveState      `json:"archive,omitempty" yaml:"archive,omitempty"`
	LastOp    OperationState    `json:"last_op,omitempty" yaml:"last_op,omitempty"`
}

type TaskCounts struct {
	Todo      int `json:"todo,omitempty" yaml:"todo,omitempty"`
	Active    int `json:"active,omitempty" yaml:"active,omitempty"`
	Blocked   int `json:"blocked,omitempty" yaml:"blocked,omitempty"`
	Done      int `json:"done,omitempty" yaml:"done,omitempty"`
	Cancelled int `json:"cancelled,omitempty" yaml:"cancelled,omitempty"`
}

type ArchiveState struct {
	Ready    bool     `json:"ready" yaml:"ready"`
	Blockers []string `json:"blockers,omitempty" yaml:"blockers,omitempty"`
}

type OperationState struct {
	Cmd   string `json:"cmd,omitempty" yaml:"cmd,omitempty"`
	OK    bool   `json:"ok" yaml:"ok"`
	Step  string `json:"step,omitempty" yaml:"step,omitempty"`
	Error string `json:"error,omitempty" yaml:"error,omitempty"`
	At    string `json:"at,omitempty" yaml:"at,omitempty"`
}
