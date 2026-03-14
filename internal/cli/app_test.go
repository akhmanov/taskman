package cli

import (
	"bytes"
	"context"
	"testing"
)

func TestBuildAppHelpListsPrimaryResources(t *testing.T) {
	cmd := BuildApp()

	var stdout bytes.Buffer
	cmd.Writer = &stdout
	cmd.ErrWriter = &stdout

	err := cmd.Run(context.Background(), []string{"taskman", "--help"})
	if err != nil {
		t.Fatalf("run help: %v", err)
	}

	help := stdout.String()
	for _, want := range []string{"projects", "tasks", "doctor"} {
		if !bytes.Contains([]byte(help), []byte(want)) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}

	for _, want := range []string{"get", "create", "describe", "archive", "transition"} {
		if !bytes.Contains([]byte(help), []byte(want)) {
			t.Fatalf("help missing nested verb %q: %s", want, help)
		}
	}
}
