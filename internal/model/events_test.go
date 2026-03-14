package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPayloadEventRoundTripPreservesTypedEventFields(t *testing.T) {
	input := []byte(`id: EVT-001
at: 2026-03-14T10:35:00Z
type: decision
summary: Adopt append-only payload events
details: Keep state.yaml compact and durable
actor: taskman
session: S-001
refs: ["doc://plan", "task://user-permissions/api-auth"]
rationale: Reduces coupling between runtime summaries and durable memory
impact: Future describe and agent views can compose from bounded events
status: active
`)

	var event PayloadEvent
	if err := yaml.Unmarshal(input, &event); err != nil {
		t.Fatalf("unmarshal payload event: %v", err)
	}

	if event.Type != PayloadEventTypeDecision {
		t.Fatalf("event type = %q", event.Type)
	}
	if event.ID != "EVT-001" {
		t.Fatalf("event id = %q", event.ID)
	}

	if event.Actor != "taskman" {
		t.Fatalf("actor = %q", event.Actor)
	}

	output, err := yaml.Marshal(event)
	if err != nil {
		t.Fatalf("marshal payload event: %v", err)
	}

	var roundTrip PayloadEvent
	if err := yaml.Unmarshal(output, &roundTrip); err != nil {
		t.Fatalf("round trip unmarshal: %v", err)
	}

	if roundTrip.Session != "S-001" {
		t.Fatalf("session = %q", roundTrip.Session)
	}
	if roundTrip.Status != "active" {
		t.Fatalf("status = %q", roundTrip.Status)
	}
	if len(roundTrip.Refs) != 2 {
		t.Fatalf("refs len = %d", len(roundTrip.Refs))
	}
}

func TestPayloadEventTypeConstantsAreBoundedToApprovedSet(t *testing.T) {
	types := []PayloadEventType{
		PayloadEventTypeDecision,
		PayloadEventTypeNote,
		PayloadEventTypeBlocker,
		PayloadEventTypeHandoff,
		PayloadEventTypeReference,
		PayloadEventTypeTransition,
		PayloadEventTypeScopeChange,
	}

	if len(types) != 7 {
		t.Fatalf("event type constants len = %d", len(types))
	}

	if types[0] != "decision" || types[6] != "scope_change" {
		t.Fatalf("unexpected bounded event constants: %#v", types)
	}
}
