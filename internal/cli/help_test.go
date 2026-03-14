package cli

import (
	"bytes"
	"context"
	"testing"
)

func TestTaskAddHelpExplainsCanonicalMetadataFlags(t *testing.T) {
	cmd := BuildApp()
	var stdout bytes.Buffer
	cmd.Writer = &stdout
	cmd.ErrWriter = &stdout

	err := cmd.Run(context.Background(), []string{"taskman", "task", "add", "--help"})
	if err != nil {
		t.Fatalf("run help: %v", err)
	}

	help := stdout.String()
	for _, want := range []string{"--project", "--label", "--var"} {
		if !bytes.Contains([]byte(help), []byte(want)) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}
	if bytes.Contains([]byte(help), []byte("--name")) {
		t.Fatalf("help should not mention legacy --name flag: %s", help)
	}
}

func TestTaskStartHelpShowsCanonicalArguments(t *testing.T) {
	cmd := BuildApp()
	var stdout bytes.Buffer
	cmd.Writer = &stdout
	cmd.ErrWriter = &stdout

	err := cmd.Run(context.Background(), []string{"taskman", "task", "start", "--help"})
	if err != nil {
		t.Fatalf("run help: %v", err)
	}

	help := stdout.String()
	for _, want := range []string{"<task>", "--project", "-p"} {
		if !bytes.Contains([]byte(help), []byte(want)) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}
	if bytes.Contains([]byte(help), []byte("<transition>")) {
		t.Fatalf("help should not mention legacy transition argument: %s", help)
	}
}
