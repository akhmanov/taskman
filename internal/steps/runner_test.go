package steps

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/assistant-wi/taskman/internal/model"
)

func TestRunnerExecutesStepWithJSONInputAndStructuredOutput(t *testing.T) {
	tempDir := t.TempDir()
	script := filepath.Join(tempDir, "emit-json.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ninput=\"$2\"\ncat <<EOF\n{\"ok\":true,\"message\":\"step passed\",\"facts\":{\"input\":\"$input\"}}\nEOF\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	runner := New(tempDir)
	result, err := runner.RunPhase(context.Background(), model.TaskStartPhase, []model.Step{{
		Name: "emit",
		Cmd:  []string{script, "--input", "{{ .input_json_path }}"},
	}}, Context{ProjectSlug: "user-permissions", TaskSlug: "cloud-api-auth"})
	if err != nil {
		t.Fatalf("run phase: %v", err)
	}

	if !result.OK {
		t.Fatal("expected phase to succeed")
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
	result, err := runner.RunPhase(context.Background(), model.TaskDonePhase, []model.Step{
		{Name: "first", Cmd: []string{failScript}},
		{Name: "second", Cmd: []string{passScript}},
	}, Context{})
	if err != nil {
		t.Fatalf("run phase: %v", err)
	}

	if result.OK {
		t.Fatal("expected phase failure")
	}

	if result.FailedStep != "first" {
		t.Fatalf("failed step = %q", result.FailedStep)
	}

	if len(result.Steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(result.Steps))
	}
}
