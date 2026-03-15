package lifecycle

import "github.com/akhmanov/taskman/internal/steps"

func lastResultMessage(result steps.PhaseResult) string {
	if len(result.Steps) == 0 {
		return "middleware failed"
	}
	message := result.Steps[len(result.Steps)-1].Result.Message
	if message == "" {
		return "middleware failed"
	}
	return message
}

func collectWarnings(result steps.PhaseResult) []string {
	var warnings []string
	for _, step := range result.Steps {
		warnings = append(warnings, step.Result.Warnings...)
	}
	return warnings
}

func collectFacts(result steps.PhaseResult) map[string]any {
	facts := map[string]any{}
	for _, step := range result.Steps {
		for key, value := range step.Result.Facts {
			facts[key] = value
		}
	}
	return facts
}

func collectArtifacts(result steps.PhaseResult) map[string]map[string]any {
	artifacts := map[string]map[string]any{}
	for _, step := range result.Steps {
		for key, value := range step.Result.Artifacts {
			artifacts[key] = value
		}
	}
	return artifacts
}

func mergeFacts(base map[string]any, extra map[string]any) {
	for key, value := range extra {
		base[key] = value
	}
}

func mergeArtifacts(base map[string]map[string]any, extra map[string]map[string]any) {
	for key, value := range extra {
		base[key] = value
	}
}
