package lifecycle

import (
	"fmt"
	"strings"

	"github.com/akhmanov/taskman/internal/model"
)

func validatePayloadEvent(event model.PayloadEvent) error {
	if event.Type == "" {
		return fmt.Errorf("event type is required")
	}
	if !isAllowedPayloadEventType(event.Type) {
		return fmt.Errorf("event type %q is not allowed", event.Type)
	}
	if event.ID == "" {
		return fmt.Errorf("event id is required")
	}
	if event.At == "" {
		return fmt.Errorf("event at is required")
	}
	if event.Summary == "" {
		return fmt.Errorf("event summary is required")
	}
	if event.Actor == "" {
		return fmt.Errorf("event actor is required")
	}
	switch event.Type {
	case model.PayloadEventTypeDecision:
		if strings.TrimSpace(event.Rationale) == "" {
			return fmt.Errorf("decision event rationale is required")
		}
		if strings.TrimSpace(event.Impact) == "" {
			return fmt.Errorf("decision event impact is required")
		}
		if event.Status != "active" && event.Status != "superseded" {
			return fmt.Errorf("decision event status must be active or superseded")
		}
	case model.PayloadEventTypeBlocker:
		if event.Status != "active" && event.Status != "resolved" {
			return fmt.Errorf("blocker event status must be active or resolved")
		}
	}
	return nil
}

func isAllowedPayloadEventType(eventType model.PayloadEventType) bool {
	switch eventType {
	case model.PayloadEventTypeDecision,
		model.PayloadEventTypeNote,
		model.PayloadEventTypeBlocker,
		model.PayloadEventTypeHandoff,
		model.PayloadEventTypeReference,
		model.PayloadEventTypeScopeChange:
		return true
	default:
		return false
	}
}

func hasEventID(events []model.PayloadEvent, id string) bool {
	for _, event := range events {
		if event.ID == id {
			return true
		}
	}
	return false
}
