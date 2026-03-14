package main

import (
	"context"
	"log"
	"os"

	"github.com/akhmanov/taskman/internal/cli"
)

func main() {
	if err := cli.BuildApp().Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
