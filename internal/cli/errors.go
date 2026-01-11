// Package cli provides error handling utilities for CLI output.
package cli

import (
	"fmt"
	"os"

	orcerrors "github.com/randalmurphal/orc/internal/errors"
)

// PrintError prints an error to stderr with appropriate formatting.
// If the error is an OrcError, it uses the user-friendly format.
// Otherwise, it prints a simple error message.
func PrintError(err error) {
	if orcErr := orcerrors.AsOrcError(err); orcErr != nil {
		fmt.Fprintln(os.Stderr, orcErr.UserMessage())
		if verbose {
			// In verbose mode, also print the error code and cause
			fmt.Fprintf(os.Stderr, "\nCode: %s\n", orcErr.Code)
			if orcErr.Cause != nil {
				fmt.Fprintf(os.Stderr, "Cause: %v\n", orcErr.Cause)
			}
		}
		return
	}
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

// wrapTaskNotFound returns a task not found error.
func wrapTaskNotFound(id string) error {
	return orcerrors.ErrTaskNotFound(id)
}

// wrapNotInitialized returns a not initialized error.
func wrapNotInitialized() error {
	return orcerrors.ErrNotInitialized()
}
