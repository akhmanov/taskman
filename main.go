package main

import (
	"context"
	"log"
	"os"

	"github.com/assistant-wi/taskman/internal/cli"
)

func main() {
	if err := cli.BuildApp().Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
