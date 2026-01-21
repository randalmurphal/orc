// Package storage provides test utilities for storage backends.
//
// This file contains test helpers that should be used by all tests
// requiring database backends. Using these helpers ensures:
// - In-memory databases for speed (10-100x faster than file-based)
// - Proper cleanup via t.Cleanup()
// - Consistent patterns across the codebase
package storage

import (
	"testing"
)

// NewTestBackend creates an in-memory database backend for testing.
// The backend is automatically closed when the test completes.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel() // Always add for faster tests
//	    backend := storage.NewTestBackend(t)
//	    // use backend...
//	}
func NewTestBackend(t testing.TB) *DatabaseBackend {
	t.Helper()

	backend, err := NewInMemoryBackend()
	if err != nil {
		t.Fatalf("create test backend: %v", err)
	}

	t.Cleanup(func() {
		_ = backend.Close()
	})

	return backend
}
