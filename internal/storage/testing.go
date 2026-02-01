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

	"github.com/randalmurphal/orc/internal/db"
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

	// Seed standard phase templates so FK constraints on workflow_run_phases work.
	// In production these are seeded via SeedBuiltins during orc init.
	seedTestPhaseTemplates(backend)

	t.Cleanup(func() {
		_ = backend.Close()
	})

	return backend
}

// seedTestPhaseTemplates creates minimal phase template entries for FK constraints.
// Uses PromptSource "db" with inline content so integration tests don't require
// embedded template files on the filesystem.
func seedTestPhaseTemplates(backend *DatabaseBackend) {
	ids := []string{
		"spec", "tiny_spec", "tdd_write", "breakdown",
		"implement", "review", "docs", "research",
	}
	for _, id := range ids {
		_ = backend.SavePhaseTemplate(&db.PhaseTemplate{
			ID:            id,
			Name:          id,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + id,
			MaxIterations: 10,
		})
	}
}

// NewTestGlobalDB creates an in-memory global database for testing.
// The database is automatically closed when the test completes.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel() // Always add for faster tests
//	    globalDB := storage.NewTestGlobalDB(t)
//	    // use globalDB for workflows, phases, agents...
//	}
func NewTestGlobalDB(t testing.TB) *db.GlobalDB {
	t.Helper()

	// Create in-memory database and migrate with global schema
	inMemDB, err := db.OpenInMemory()
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	if err := inMemDB.Migrate("global"); err != nil {
		_ = inMemDB.Close()
		t.Fatalf("migrate global db: %v", err)
	}

	globalDB := &db.GlobalDB{DB: inMemDB}

	t.Cleanup(func() {
		_ = globalDB.Close()
	})

	return globalDB
}
