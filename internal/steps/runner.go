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

	"github.com/assistant-wi/taskman/internal/model"
)

type Runner struct {
	workDir string
}

func New(workDir string) Runner {
	return Runner{workDir: workDir}
}

func (r Runner) Run(ctx context.Context, transition string, steps []model.Step, input Context) (PhaseResult, error) {
	result := PhaseResult{OK: true}
	input.Transition = transition

	inputPath, err := r.writeInput(input)
	if err != nil {
		return PhaseResult{}, err
	}
	defer os.Remove(inputPath)

	for _, step := range steps {
		if !matchesWhen(step.When, input) {
			continue
		}

		runResult, runErr := r.runStep(ctx, step, map[string]string{
			"input_json_path": inputPath,
		})
		if runErr != nil {
			return PhaseResult{}, runErr
		}

		result.Steps = append(result.Steps, StepExecution{Name: step.Name, Result: runResult})
		if !runResult.OK {
			result.OK = false
			result.FailedStep = step.Name
			break
		}
	}

	return result, nil
}

func (r Runner) runStep(ctx context.Context, step model.Step, vars map[string]string) (Result, error) {
	args := make([]string, 0, len(step.Cmd))
	for _, part := range step.Cmd {
		rendered, err := render(part, vars)
		if err != nil {
			return Result{}, err
		}
		args = append(args, rendered)
	}

	if len(args) == 0 {
		return Result{}, fmt.Errorf("step %q has empty command", step.Name)
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
		return Result{}, fmt.Errorf("decode result for %q: %w", step.Name, decodeErr)
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

func matchesWhen(selectors map[string]string, input Context) bool {
	if len(selectors) == 0 {
		return true
	}

	for key, expected := range selectors {
		switch {
		case strings.HasPrefix(key, "task.vars."):
			name := strings.TrimPrefix(key, "task.vars.")
			if input.Task.Vars[name] != expected {
				return false
			}
		case strings.HasPrefix(key, "project.vars."):
			name := strings.TrimPrefix(key, "project.vars.")
			if input.Project.Vars[name] != expected {
				return false
			}
		case key == "task.status":
			if input.Task.Status != expected {
				return false
			}
		case key == "project.status":
			if input.Project.Status != expected {
				return false
			}
		case key == "task.slug":
			if input.Task.Slug != expected {
				return false
			}
		case key == "project.slug":
			if input.Project.Slug != expected {
				return false
			}
		case key == "transition":
			if input.Transition != expected {
				return false
			}
		default:
			return false
		}
	}

	return true
}
