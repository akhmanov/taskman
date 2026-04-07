package model

import (
	"sort"
	"strings"
)

func NormalizeLabels(labels []string) []string {
	if len(labels) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(labels))
	for _, label := range labels {
		value := strings.ToLower(strings.TrimSpace(label))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return nil
	}
	sort.Strings(normalized)
	return normalized
}

func MergeLabels(base, extra []string) []string {
	combined := append([]string{}, base...)
	combined = append(combined, extra...)
	return NormalizeLabels(combined)
}

func RemoveLabels(current, remove []string) []string {
	removeSet := map[string]struct{}{}
	for _, label := range NormalizeLabels(remove) {
		removeSet[label] = struct{}{}
	}
	kept := make([]string, 0, len(current))
	for _, label := range NormalizeLabels(current) {
		if _, ok := removeSet[label]; ok {
			continue
		}
		kept = append(kept, label)
	}
	return NormalizeLabels(kept)
}

func HasAnyLabel(current, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	currentSet := map[string]struct{}{}
	for _, label := range NormalizeLabels(current) {
		currentSet[label] = struct{}{}
	}
	for _, label := range NormalizeLabels(filter) {
		if _, ok := currentSet[label]; ok {
			return true
		}
	}
	return false
}
