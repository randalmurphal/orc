//go:build integration

package driver

import (
	"context"
	"testing"
	"time"
)

// TestPostgres_PoolConfiguration verifies pgxpool is properly configured.
// Covers SC-1: Open creates pool with MaxConns=10, MinConns=2, MaxConnLifetime=1h.
func TestPostgres_PoolConfiguration(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = drv.Close() }()

	// Verify pool exists and has correct config
	if drv.pool == nil {
		t.Fatal("pool should not be nil after Open")
	}

	config := drv.pool.Config()
	if config.MaxConns != 10 {
		t.Errorf("MaxConns = %d, want 10", config.MaxConns)
	}
	if config.MinConns != 2 {
		t.Errorf("MinConns = %d, want 2", config.MinConns)
	}
	if config.MaxConnLifetime != time.Hour {
		t.Errorf("MaxConnLifetime = %v, want %v", config.MaxConnLifetime, time.Hour)
	}
}

// TestPostgres_PoolOperations verifies all Driver interface methods work through pgxpool.
// Covers SC-3: DB() returns sql.DB backed by pgxpool, all methods work.
func TestPostgres_PoolOperations(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = drv.Close() }()

	ctx := context.Background()

	// DB() should return non-nil sql.DB backed by pgxpool
	if drv.DB() == nil {
		t.Fatal("DB() should return non-nil sql.DB")
	}

	// Exec through pool
	_, err := drv.Exec(ctx, "CREATE TEMPORARY TABLE pgxpool_test (id SERIAL PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Exec CREATE TABLE failed: %v", err)
	}

	// Insert through pool
	result, err := drv.Exec(ctx, "INSERT INTO pgxpool_test (name) VALUES ($1)", "pooled")
	if err != nil {
		t.Fatalf("Exec INSERT failed: %v", err)
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		t.Errorf("RowsAffected = %d, want 1", rows)
	}

	// Query through pool
	qrows, err := drv.Query(ctx, "SELECT id, name FROM pgxpool_test")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer func() { _ = qrows.Close() }()

	if !qrows.Next() {
		t.Fatal("expected row, got none")
	}
	var id int
	var name string
	if err := qrows.Scan(&id, &name); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if name != "pooled" {
		t.Errorf("name = %q, want %q", name, "pooled")
	}

	// QueryRow through pool
	var count int
	err = drv.QueryRow(ctx, "SELECT COUNT(*) FROM pgxpool_test").Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow failed: %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// BeginTx through pool
	tx, err := drv.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO pgxpool_test (name) VALUES ($1)", "txn")
	if err != nil {
		t.Fatalf("tx.Exec failed: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("tx.Commit failed: %v", err)
	}

	// Verify transaction committed
	err = drv.QueryRow(ctx, "SELECT COUNT(*) FROM pgxpool_test").Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow after tx failed: %v", err)
	}
	if count != 2 {
		t.Errorf("count after tx = %d, want 2", count)
	}
}

// TestPostgres_PoolClose verifies close properly shuts down the pool.
// Covers SC-2: Close shuts down the pgxpool.Pool and releases connections.
func TestPostgres_PoolClose(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Verify pool is set
	if drv.pool == nil {
		t.Fatal("pool should not be nil after Open")
	}

	// Close should not error
	if err := drv.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// After close, pool operations should fail
	ctx := context.Background()
	_, err := drv.Exec(ctx, "SELECT 1")
	if err == nil {
		t.Error("Exec after Close should return error")
	}
}
