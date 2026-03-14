package cli

import (
	"bytes"
	"context"
	"testing"
)

func TestTasksCreateHelpExplainsGenericMetadataFlags(t *testing.T) {
	cmd := BuildApp()
	var stdout bytes.Buffer
	cmd.Writer = &stdout
	cmd.ErrWriter = &stdout

	err := cmd.Run(context.Background(), []string{"taskman", "tasks", "create", "--help"})
	if err != nil {
		t.Fatalf("run help: %v", err)
	}

	help := stdout.String()
	for _, want := range []string{"--project", "--name", "--label", "--var"} {
		if !bytes.Contains([]byte(help), []byte(want)) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}
}

func TestTasksTransitionHelpShowsTransitionArguments(t *testing.T) {
	cmd := BuildApp()
	var stdout bytes.Buffer
	cmd.Writer = &stdout
	cmd.ErrWriter = &stdout

	err := cmd.Run(context.Background(), []string{"taskman", "tasks", "transition", "--help"})
	if err != nil {
		t.Fatalf("run help: %v", err)
	}

	help := stdout.String()
	for _, want := range []string{"<project>/<task>", "<transition>"} {
		if !bytes.Contains([]byte(help), []byte(want)) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}
}
