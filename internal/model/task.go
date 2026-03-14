package model

type TaskStatus string

type TaskState struct {
	Version   int               `json:"version" yaml:"version"`
	Slug      string            `json:"slug" yaml:"slug"`
	Project   string            `json:"project" yaml:"project"`
	Status    TaskStatus        `json:"status" yaml:"status"`
	Labels    []string          `json:"labels,omitempty" yaml:"labels,omitempty"`
	Vars      map[string]string `json:"vars,omitempty" yaml:"vars,omitempty"`
	CreatedAt string            `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt string            `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	Session   TaskSessionState  `json:"session" yaml:"session"`
	LastOp    OperationState    `json:"last_op,omitempty" yaml:"last_op,omitempty"`
}

type TaskSessionState struct {
	Active        string  `json:"active,omitempty" yaml:"active,omitempty"`
	LastCompleted *string `json:"last_completed,omitempty" yaml:"last_completed,omitempty"`
}
