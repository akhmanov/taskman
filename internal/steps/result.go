package steps

type Result struct {
	OK        bool                         `json:"ok"`
	Message   string                       `json:"message,omitempty"`
	Facts     map[string]string            `json:"facts,omitempty"`
	Artifacts map[string]map[string]string `json:"artifacts,omitempty"`
	Warnings  []string                     `json:"warnings,omitempty"`
}

type StepExecution struct {
	Name   string `json:"name"`
	Result Result `json:"result"`
}

type PhaseResult struct {
	OK         bool            `json:"ok"`
	FailedStep string          `json:"failed_step,omitempty"`
	Steps      []StepExecution `json:"steps,omitempty"`
}
