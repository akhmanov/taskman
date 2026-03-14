package steps

type Context struct {
	RuntimeRoot  string                    `json:"runtime_root,omitempty"`
	Project      ProjectContext            `json:"project,omitempty"`
	Task         TaskContext               `json:"task,omitempty"`
	TaskDir      string                    `json:"task_dir,omitempty"`
	ArtifactsDir string                    `json:"artifacts_dir,omitempty"`
	Artifacts    map[string]map[string]any `json:"artifacts,omitempty"`
	Transition   string                    `json:"transition,omitempty"`
}

type ProjectContext struct {
	Slug   string            `json:"slug,omitempty"`
	Status string            `json:"status,omitempty"`
	Labels []string          `json:"labels,omitempty"`
	Vars   map[string]string `json:"vars,omitempty"`
}

type TaskContext struct {
	Slug   string            `json:"slug,omitempty"`
	Status string            `json:"status,omitempty"`
	Labels []string          `json:"labels,omitempty"`
	Vars   map[string]string `json:"vars,omitempty"`
}
