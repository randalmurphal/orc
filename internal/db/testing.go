// Package db provides test utilities for database operations.
//
// This file contains test helpers that should be used by all tests
// requiring database access. Using these helpers ensures:
// - In-memory databases for speed (10-100x faster than file-based)
// - Proper cleanup via t.Cleanup()
// - Consistent patterns across the codebase
package db

import (
	"testing"
)

// NewTestProjectDB creates an in-memory project database for testing.
// The database is automatically closed when the test completes.
// Schema migrations are applied automatically.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//	    t.Parallel() // Always add for faster tests
//	    pdb := db.NewTestProjectDB(t)
//	    // use pdb...
//	}
func NewTestProjectDB(t testing.TB) *ProjectDB {
	t.Helper()

	pdb, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("create test project db: %v", err)
	}

	t.Cleanup(func() {
		_ = pdb.Close()
	})

	return pdb
}
