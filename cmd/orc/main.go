// Package main provides the entry point for the orc CLI.
package main

import (
	"os"

	"github.com/randalmurphal/orc/internal/cli"
)

func main() {
	// main runs the orc CLI command tree and exits non-zero on execution errors.
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
