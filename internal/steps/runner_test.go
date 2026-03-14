package steps

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/akhmanov/taskman/internal/model"
)

func TestRunnerExecutesStepWithJSONInputAndStructuredOutput(t *testing.T) {
	tempDir := t.TempDir()
	script := filepath.Join(tempDir, "emit-json.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ninput=\"$2\"\ncat <<EOF\n{\"ok\":true,\"message\":\"step passed\",\"facts\":{\"input\":\"$input\"}}\nEOF\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	runner := New(tempDir)
	result, err := runner.Run(context.Background(), "start", []model.Step{{
		Name: "emit",
		Cmd:  []string{script, "--input", "{{ .input_json_path }}"},
	}}, Context{
		Project: ProjectContext{Slug: "user-permissions", Status: "active"},
		Task:    TaskContext{Slug: "cloud-api-auth", Status: "todo"},
	})
	if err != nil {
		t.Fatalf("run steps: %v", err)
	}

	if !result.OK {
		t.Fatal("expected execution to succeed")
	}

	if len(result.Steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(result.Steps))
	}

	if result.Steps[0].Result.Message != "step passed" {
		t.Fatalf("message = %q", result.Steps[0].Result.Message)
	}
}

func TestRunnerStopsAfterFirstFailingStep(t *testing.T) {
	tempDir := t.TempDir()
	failScript := filepath.Join(tempDir, "fail.sh")
	passScript := filepath.Join(tempDir, "pass.sh")

	if err := os.WriteFile(failScript, []byte("#!/bin/sh\necho '{\"ok\":false,\"message\":\"nope\"}'\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fail script: %v", err)
	}
	if err := os.WriteFile(passScript, []byte("#!/bin/sh\necho '{\"ok\":true,\"message\":\"should not run\"}'\n"), 0o755); err != nil {
		t.Fatalf("write pass script: %v", err)
	}

	runner := New(tempDir)
	result, err := runner.Run(context.Background(), "complete", []model.Step{
		{Name: "first", Cmd: []string{failScript}},
		{Name: "second", Cmd: []string{passScript}},
	}, Context{})
	if err != nil {
		t.Fatalf("run steps: %v", err)
	}

	if result.OK {
		t.Fatal("expected execution failure")
	}

	if result.FailedStep != "first" {
		t.Fatalf("failed step = %q", result.FailedStep)
	}

	if len(result.Steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(result.Steps))
	}
}

func TestRunnerWhenSelectorsUseGenericVarsAndStatuses(t *testing.T) {
	tempDir := t.TempDir()
	script := filepath.Join(tempDir, "emit-json.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho '{\"ok\":true,\"message\":\"matched\"}'\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	runner := New(tempDir)
	result, err := runner.Run(context.Background(), "start", []model.Step{{
		Name: "emit",
		When: map[string]string{
			"task.vars.kind": "feature",
			"task.status":    "todo",
			"transition":     "start",
		},
		Cmd: []string{script},
	}}, Context{
		Project:    ProjectContext{Slug: "user-permissions", Status: "active", Vars: map[string]string{"area": "identity"}},
		Task:       TaskContext{Slug: "cloud-api-auth", Status: "todo", Vars: map[string]string{"kind": "feature"}},
		Transition: "start",
	})
	if err != nil {
		t.Fatalf("run steps: %v", err)
	}

	if len(result.Steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(result.Steps))
	}
}
