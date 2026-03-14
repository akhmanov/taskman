package model

type SessionState struct {
	Version   int            `yaml:"version"`
	ID        string         `yaml:"id"`
	Task      string         `yaml:"task"`
	Status    string         `yaml:"status"`
	CreatedAt string         `yaml:"created_at,omitempty"`
	UpdatedAt string         `yaml:"updated_at,omitempty"`
	LastOp    OperationState `yaml:"last_op,omitempty"`
}
