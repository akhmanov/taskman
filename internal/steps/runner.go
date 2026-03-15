package steps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/akhmanov/taskman/internal/model"
)

type Runner struct {
	workDir string
}

func New(workDir string) Runner {
	return Runner{workDir: workDir}
}

func (r Runner) Run(ctx context.Context, transition string, commands []model.MiddlewareCommand, input Context) (PhaseResult, error) {
	result := PhaseResult{OK: true}
	input.Transition = transition

	inputPath, err := r.writeInput(input)
	if err != nil {
		return PhaseResult{}, err
	}
	defer os.Remove(inputPath)

	for _, command := range commands {
		runResult, runErr := r.runStep(ctx, command, map[string]string{
			"input_json_path": inputPath,
		})
		if runErr != nil {
			return PhaseResult{}, runErr
		}

		result.Steps = append(result.Steps, StepExecution{Name: command.Name, Result: runResult})
		if !runResult.OK {
			result.OK = false
			result.FailedStep = command.Name
			break
		}
	}

	return result, nil
}

func (r Runner) runStep(ctx context.Context, command model.MiddlewareCommand, vars map[string]string) (Result, error) {
	args := make([]string, 0, len(command.Cmd))
	for _, part := range command.Cmd {
		rendered, err := render(part, vars)
		if err != nil {
			return Result{}, err
		}
		args = append(args, rendered)
	}

	if len(args) == 0 {
		return Result{}, fmt.Errorf("middleware %q has empty command", command.Name)
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = r.workDir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var result Result
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		if err != nil {
			return Result{OK: false, Message: strings.TrimSpace(stderr.String())}, nil
		}
		return Result{}, fmt.Errorf("decode result for %q: %w", command.Name, decodeErr)
	}

	if err != nil && result.Message == "" {
		result.Message = strings.TrimSpace(stderr.String())
	}

	if err != nil {
		result.OK = false
	}

	return result, nil
}

func (r Runner) writeInput(input Context) (string, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	file, err := os.CreateTemp(r.workDir, "taskman-input-*.json")
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return "", err
	}

	return filepath.Clean(file.Name()), nil
}

func render(source string, vars map[string]string) (string, error) {
	tmpl, err := template.New("cmd").Parse(source)
	if err != nil {
		return "", err
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, vars); err != nil {
		return "", err
	}

	return out.String(), nil
}
