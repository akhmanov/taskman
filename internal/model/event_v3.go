package model

type EventKind string

const (
	EventKindMessage               EventKind = "message"
	EventKindTransition            EventKind = "transition"
	EventKindMetadataPatch         EventKind = "metadata_patch"
	EventKindMiddlewarePhaseStart  EventKind = "middleware_phase_started"
	EventKindMiddlewareStepFinish  EventKind = "middleware_step_finished"
	EventKindMiddlewarePhaseFinish EventKind = "middleware_phase_finished"
)

type MessageKind string

const (
	MessageKindComment  MessageKind = "comment"
	MessageKindDecision MessageKind = "decision"
	MessageKindBlocker  MessageKind = "blocker"
	MessageKindHandoff  MessageKind = "handoff"
	MessageKindNote     MessageKind = "note"
)

type Event struct {
	ID            string               `json:"id"`
	EntityID      string               `json:"entity_id"`
	Kind          EventKind            `json:"kind"`
	At            string               `json:"at"`
	Actor         string               `json:"actor"`
	ParentHeadID  string               `json:"parent_head_id,omitempty"`
	Message       *MessagePayload      `json:"message,omitempty"`
	Transition    *TransitionPayload   `json:"transition,omitempty"`
	MetadataPatch *MetadataPatch       `json:"metadata_patch,omitempty"`
	Middleware    *MiddlewareEventData `json:"middleware,omitempty"`
}

func (e Event) IsStateful() bool {
	return e.Kind == EventKindTransition || e.Kind == EventKindMetadataPatch
}

type MessagePayload struct {
	Kind MessageKind `json:"kind"`
	Body string      `json:"body"`
}

func IsValidMessageKind(kind MessageKind) bool {
	switch kind {
	case MessageKindComment, MessageKindDecision, MessageKindBlocker, MessageKindHandoff, MessageKindNote:
		return true
	default:
		return false
	}
}

type TransitionPayload struct {
	Verb       string         `json:"verb"`
	From       Status         `json:"from"`
	To         Status         `json:"to"`
	ReasonType string         `json:"reason_type,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	ResumeWhen string         `json:"resume_when,omitempty"`
	Summary    string         `json:"summary,omitempty"`
	Warnings   []string       `json:"warnings,omitempty"`
	Facts      map[string]any `json:"facts,omitempty"`
	Artifacts  []string       `json:"artifacts,omitempty"`
}

type MetadataPatch struct {
	Labels    []string          `json:"labels,omitempty"`
	VarsSet   map[string]string `json:"vars_set,omitempty"`
	VarsUnset []string          `json:"vars_unset,omitempty"`
}

type MiddlewareEventData struct {
	Phase     string   `json:"phase"`
	Step      string   `json:"step,omitempty"`
	OK        bool     `json:"ok"`
	Message   string   `json:"message,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
}
