package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/brpaz/draftsman/internal/app"
)

var (
	Version   = "0.0.0-dev"
	BuildDate = "n/a"
	Commit    = "n/a"
)

func main() {
	appInstance, err := app.New(
		app.WithVersionInfo(app.VersionInfo{
			Version:   Version,
			Commit:    Commit,
			BuildDate: BuildDate,
		}),
	)
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	if err := appInstance.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
