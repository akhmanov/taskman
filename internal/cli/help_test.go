package cli

import (
	"bytes"
	"context"
	"testing"
)

func TestTasksCreateHelpExplainsMetadataFlags(t *testing.T) {
	cmd := BuildApp()
	var stdout bytes.Buffer
	cmd.Writer = &stdout
	cmd.ErrWriter = &stdout

	err := cmd.Run(context.Background(), []string{"taskman", "tasks", "create", "--help"})
	if err != nil {
		t.Fatalf("run help: %v", err)
	}

	help := stdout.String()
	for _, want := range []string{"--project", "--repo", "--name", "--label", "--trait"} {
		if !bytes.Contains([]byte(help), []byte(want)) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}
}

func TestTasksDoneHelpShowsResourceIDShape(t *testing.T) {
	cmd := BuildApp()
	var stdout bytes.Buffer
	cmd.Writer = &stdout
	cmd.ErrWriter = &stdout

	err := cmd.Run(context.Background(), []string{"taskman", "tasks", "done", "--help"})
	if err != nil {
		t.Fatalf("run help: %v", err)
	}

	help := stdout.String()
	if !bytes.Contains([]byte(help), []byte("<project>/<task>")) {
		t.Fatalf("help missing task id shape: %s", help)
	}
}
