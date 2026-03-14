package lifecycle

import (
	"fmt"
	"strings"

	"github.com/assistant-wi/taskman/internal/model"
)

var projectBriefSections = []string{
	"# Mission",
	"# Boundaries",
	"## In Scope",
	"## Out of Scope",
	"# Glossary",
	"# Shared Decisions",
	"# Active Risks",
	"# Tasking Rules",
	"# References",
}

var taskBriefSections = []string{
	"# Intent",
	"# Scope In",
	"# Scope Out",
	"# Acceptance",
	"# Current Context",
	"# Open Questions",
	"# Next Action",
	"# References",
}

func validateProjectBrief(brief string) error {
	_ = model.ProjectBriefTemplate
	return validateBriefSections("project", brief, projectBriefSections)
}

func validateTaskBrief(brief string) error {
	_ = model.TaskBriefTemplate
	return validateBriefSections("task", brief, taskBriefSections)
}

func validateBriefSections(scope, brief string, sections []string) error {
	trimmed := strings.TrimSpace(brief)
	if trimmed == "" {
		return fmt.Errorf("%s brief is required", scope)
	}
	for _, section := range sections {
		if !strings.Contains(brief, section) {
			return fmt.Errorf("%s brief must contain section %q", scope, section)
		}
	}
	return nil
}
