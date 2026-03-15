package model

type TransitionRecord struct {
	At         string                    `json:"at" yaml:"at"`
	Actor      string                    `json:"actor,omitempty" yaml:"actor,omitempty"`
	Verb       string                    `json:"verb" yaml:"verb"`
	From       Status                    `json:"from" yaml:"from"`
	To         Status                    `json:"to" yaml:"to"`
	ReasonType string                    `json:"reason_type,omitempty" yaml:"reason_type,omitempty"`
	Reason     string                    `json:"reason,omitempty" yaml:"reason,omitempty"`
	ResumeWhen string                    `json:"resume_when,omitempty" yaml:"resume_when,omitempty"`
	Summary    string                    `json:"summary,omitempty" yaml:"summary,omitempty"`
	Warnings   []string                  `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Artifacts  map[string]map[string]any `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	Facts      map[string]any            `json:"facts,omitempty" yaml:"facts,omitempty"`
}
