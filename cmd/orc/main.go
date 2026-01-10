// Package main provides the entry point for the orc CLI.
package main

import (
	"os"

	"github.com/randalmurphal/orc/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
