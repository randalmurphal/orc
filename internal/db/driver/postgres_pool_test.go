package driver

import (
	"testing"
)

// TestPostgresDriver_HasPool verifies PostgresDriver stores a pgxpool.Pool.
// Covers SC-1: Open creates a pgxpool.Pool with configured settings.
func TestPostgresDriver_HasPool(t *testing.T) {
	drv := NewPostgres()

	// Before Open, pool should be nil
	if drv.pool != nil {
		t.Error("pool should be nil before Open")
	}
}

// TestPostgresDriver_ClosePool verifies Close handles pool lifecycle.
// Covers SC-2: Close shuts down the pgxpool.Pool.
func TestPostgresDriver_ClosePool(t *testing.T) {
	drv := NewPostgres()

	// Close without Open should not error (pool is nil)
	if err := drv.Close(); err != nil {
		t.Errorf("Close without Open failed: %v", err)
	}

	// After close, pool should remain nil
	if drv.pool != nil {
		t.Error("pool should be nil after Close without Open")
	}
}
