package model

import (
	"fmt"
	"strings"
)

type Status string

type ProjectStatus = Status
type TaskStatus = Status

const (
	StatusBacklog    Status = "backlog"
	StatusPlanned    Status = "planned"
	StatusInProgress Status = "in_progress"
	StatusPaused     Status = "paused"
	StatusDone       Status = "done"
	StatusCanceled   Status = "canceled"
)

var canonicalStatusOrder = []Status{
	StatusInProgress,
	StatusPaused,
	StatusPlanned,
	StatusBacklog,
	StatusDone,
	StatusCanceled,
}

func CanonicalStatusOrder() []Status {
	return append([]Status{}, canonicalStatusOrder...)
}

func IsValidStatus(status Status) bool {
	for _, allowed := range canonicalStatusOrder {
		if status == allowed {
			return true
		}
	}
	return false
}

func IsTerminalStatus(status Status) bool {
	return status == StatusDone || status == StatusCanceled
}

func ParseStatusCSV(raw string) ([]Status, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	statuses := make([]Status, 0, len(parts))
	seen := map[Status]struct{}{}
	for _, part := range parts {
		status := Status(strings.TrimSpace(part))
		if !IsValidStatus(status) {
			return nil, fmt.Errorf("unknown status %q", status)
		}
		if _, ok := seen[status]; ok {
			continue
		}
		seen[status] = struct{}{}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func StatusSortIndex(status Status) int {
	for index, candidate := range canonicalStatusOrder {
		if status == candidate {
			return index
		}
	}
	return len(canonicalStatusOrder)
}

type StatusDetail struct {
	ReasonType string `json:"reason_type,omitempty" yaml:"reason_type,omitempty"`
	Reason     string `json:"reason,omitempty" yaml:"reason,omitempty"`
	ResumeWhen string `json:"resume_when,omitempty" yaml:"resume_when,omitempty"`
	Summary    string `json:"summary,omitempty" yaml:"summary,omitempty"`
}
