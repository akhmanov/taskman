package model

type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusBlocked  ProjectStatus = "blocked"
	ProjectStatusDone     ProjectStatus = "done"
	ProjectStatusArchived ProjectStatus = "archived"
)

type ProjectState struct {
	Version   int               `yaml:"version"`
	Slug      string            `yaml:"slug"`
	Status    ProjectStatus     `yaml:"status"`
	Labels    []string          `yaml:"labels,omitempty"`
	Traits    map[string]string `yaml:"traits,omitempty"`
	CreatedAt string            `yaml:"created_at,omitempty"`
	UpdatedAt string            `yaml:"updated_at,omitempty"`
	Tasks     TaskCounts        `yaml:"tasks,omitempty"`
	Archive   ArchiveState      `yaml:"archive,omitempty"`
	LastOp    OperationState    `yaml:"last_op,omitempty"`
}

type TaskCounts struct {
	Todo      int `yaml:"todo,omitempty"`
	Active    int `yaml:"active,omitempty"`
	Blocked   int `yaml:"blocked,omitempty"`
	Done      int `yaml:"done,omitempty"`
	Cancelled int `yaml:"cancelled,omitempty"`
}

type ArchiveState struct {
	Ready    bool     `yaml:"ready"`
	Blockers []string `yaml:"blockers,omitempty"`
}

type OperationState struct {
	Cmd   string `yaml:"cmd,omitempty"`
	OK    bool   `yaml:"ok"`
	Step  string `yaml:"step,omitempty"`
	Error string `yaml:"error,omitempty"`
	At    string `yaml:"at,omitempty"`
}
