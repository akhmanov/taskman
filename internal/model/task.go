package model

type TaskState struct {
	Slug         string            `json:"slug" yaml:"slug"`
	Project      string            `json:"project" yaml:"project"`
	Status       TaskStatus        `json:"status" yaml:"status"`
	StatusDetail StatusDetail      `json:"status_detail,omitempty" yaml:"status_detail,omitempty"`
	Labels       []string          `json:"labels,omitempty" yaml:"labels,omitempty"`
	Vars         map[string]string `json:"vars,omitempty" yaml:"vars,omitempty"`
	CreatedAt    string            `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt    string            `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}
