// Package cli provides error handling utilities for CLI output.
package cli

import (
	orcerrors "github.com/randalmurphal/orc/internal/errors"
)

// wrapNotInitialized returns a not initialized error.
func wrapNotInitialized() error {
	return orcerrors.ErrNotInitialized()
}
