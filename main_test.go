package main

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"
)

func TestExecuteWritesPlainErrorsWithoutTimestamp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := execute(context.Background(), []string{"taskman", "project", "list", "docs-refresh"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	got := stderr.String()
	if !strings.Contains(got, "project list does not accept a project id") {
		t.Fatalf("stderr = %q", got)
	}
	if regexp.MustCompile(`\d{4}/\d{2}/\d{2}`).MatchString(got) {
		t.Fatalf("stderr should not contain log timestamp: %q", got)
	}
}

func TestExecuteRewritesHelpTopicErrorsForListMisuse(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := execute(context.Background(), []string{"taskman", "project", "list", "-h", "docs-refresh"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	got := stderr.String()
	if !strings.Contains(got, "project list does not accept a project id") {
		t.Fatalf("stderr = %q", got)
	}
	if strings.Contains(got, "No help topic") {
		t.Fatalf("stderr should not leak framework help topic error: %q", got)
	}
}

func TestExecuteRewritesLegacyProjectsCommandError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := execute(context.Background(), []string{"taskman", "projects", "list"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	got := stderr.String()
	if !strings.Contains(got, "legacy `projects` command was removed; use `taskman project ...`") {
		t.Fatalf("stderr = %q", got)
	}
	if strings.Contains(got, "No help topic") {
		t.Fatalf("stderr should not leak framework help topic error: %q", got)
	}
}

func TestExecuteRewritesLegacyTransitionCommandError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := execute(context.Background(), []string{"taskman", "task", "transition", "demo", "start", "-p", "alpha"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	got := stderr.String()
	if !strings.Contains(got, "legacy `task transition` command was removed; use `taskman task <start|block|unblock|complete|cancel|close> <task> -p <project>`") {
		t.Fatalf("stderr = %q", got)
	}
	if strings.Contains(got, "No help topic") {
		t.Fatalf("stderr should not leak framework help topic error: %q", got)
	}
}
