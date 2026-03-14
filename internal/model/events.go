package model

type PayloadEventType string

const (
	PayloadEventTypeDecision    PayloadEventType = "decision"
	PayloadEventTypeNote        PayloadEventType = "note"
	PayloadEventTypeBlocker     PayloadEventType = "blocker"
	PayloadEventTypeHandoff     PayloadEventType = "handoff"
	PayloadEventTypeReference   PayloadEventType = "reference"
	PayloadEventTypeTransition  PayloadEventType = "transition"
	PayloadEventTypeScopeChange PayloadEventType = "scope_change"
)

type PayloadEvent struct {
	ID        string           `json:"id" yaml:"id"`
	At        string           `json:"at" yaml:"at"`
	Type      PayloadEventType `json:"type" yaml:"type"`
	Summary   string           `json:"summary" yaml:"summary"`
	Details   string           `json:"details,omitempty" yaml:"details,omitempty"`
	Actor     string           `json:"actor" yaml:"actor"`
	Session   string           `json:"session,omitempty" yaml:"session,omitempty"`
	Refs      []string         `json:"refs,omitempty" yaml:"refs,omitempty"`
	Rationale string           `json:"rationale,omitempty" yaml:"rationale,omitempty"`
	Impact    string           `json:"impact,omitempty" yaml:"impact,omitempty"`
	Status    string           `json:"status,omitempty" yaml:"status,omitempty"`
}
