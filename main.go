package main

import (
	"os"

	"github.com/albertcmiller1/flow/cli/cmd"
)

var version = "dev"

func main() {
	cmd.SetVersion(version)
	// Cobra already prints the error (as `Error: ...`) when Execute returns
	// non-nil; we just propagate the exit code.
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
