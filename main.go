package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/akhmanov/taskman/internal/cli"
	urfavecli "github.com/urfave/cli/v3"
)

func main() {
	os.Exit(execute(context.Background(), os.Args, os.Stdout, os.Stderr))
}

func execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cmd := cli.BuildApp()
	cmd.Writer = stdout
	cmd.ErrWriter = stderr
	cmd.ExitErrHandler = func(context.Context, *urfavecli.Command, error) {}
	if err := cmd.Run(ctx, args); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
