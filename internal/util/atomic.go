// Package util provides common utility functions for orc.
package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// AtomicWriteFile writes data to a file atomically by first writing to a
// temporary file in the same directory, syncing it, then renaming it to the
// target path. This prevents partial writes from corrupting data if the
// process crashes mid-write.
//
// The atomic rename operation is guaranteed by POSIX on the same filesystem.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Create temp file in same directory (required for atomic rename)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on any error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write data
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	// Sync to disk before rename
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Set permissions
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp to final: %w", err)
	}

	success = true
	return nil
}

// AtomicWriteFileString is a convenience wrapper for AtomicWriteFile that
// accepts a string instead of a byte slice.
func AtomicWriteFileString(path string, content string, perm os.FileMode) error {
	return AtomicWriteFile(path, []byte(content), perm)
}
