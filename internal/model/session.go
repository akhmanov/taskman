package model

type SessionState struct {
	Version   int            `json:"version" yaml:"version"`
	ID        string         `json:"id" yaml:"id"`
	Task      string         `json:"task" yaml:"task"`
	Status    string         `json:"status" yaml:"status"`
	CreatedAt string         `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
	LastOp    OperationState `json:"last_op,omitempty" yaml:"last_op,omitempty"`
}
