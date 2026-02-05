//go:build integration

package driver

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// PostgreSQL integration tests require:
// - ORC_TEST_POSTGRES_DSN environment variable set to a test database DSN
// - Run with: go test -tags=integration ./internal/db/driver/...
//
// Example:
//   export ORC_TEST_POSTGRES_DSN="postgres://user:pass@localhost/orc_test?sslmode=disable"
//   go test -tags=integration -run TestPostgres ./internal/db/driver/...

// getTestDSN returns the test PostgreSQL DSN or skips the test if not set.
func getTestDSN(t *testing.T) string {
	dsn := os.Getenv("ORC_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("ORC_TEST_POSTGRES_DSN not set, skipping PostgreSQL integration test")
	}
	return dsn
}

// resetTestDB drops and recreates all orc tables for a clean test state.
func resetTestDB(t *testing.T, db *sql.DB) {
	t.Helper()

	// Drop tables in reverse dependency order
	tables := []string{
		"phase_agents",
		"workflow_variables",
		"workflow_phases",
		"workflows",
		"phase_templates",
		"agents",
		"skills",
		"hook_scripts",
		"cost_budgets",
		"cost_aggregates",
		"cost_log",
		"templates",
		"projects",
		"users",
		"_migrations",
	}

	for _, table := range tables {
		_, _ = db.Exec("DROP TABLE IF EXISTS " + table + " CASCADE")
	}
}

// TestPostgres_MigrationsApply verifies all PostgreSQL migrations apply successfully.
// Covers SC-1: PostgreSQL migrations create identical table structures to SQLite.
// Covers SC-2 (partial): First migration run succeeds.
func TestPostgres_MigrationsApply(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("failed to open PostgreSQL connection: %v", err)
	}
	defer func() { _ = drv.Close() }()

	// Clean slate
	resetTestDB(t, drv.DB())

	// Create a mock SchemaFS that reads from the actual postgres directory
	mockFS := &testSchemaFS{t: t}

	// Apply migrations
	ctx := context.Background()
	if err := drv.Migrate(ctx, mockFS, "global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Verify key tables exist
	expectedTables := []string{
		"projects",
		"cost_log",
		"templates",
		"cost_aggregates",
		"cost_budgets",
		"phase_templates",
		"workflows",
		"workflow_phases",
		"workflow_variables",
		"agents",
		"phase_agents",
		"hook_scripts",
		"skills",
		"users",
	}

	for _, table := range expectedTables {
		var exists bool
		err := drv.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)`, table).Scan(&exists)
		if err != nil {
			t.Errorf("failed to check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %s to exist", table)
		}
	}
}

// TestPostgres_MigrationsIdempotent verifies migrations can be run twice without error.
// Covers SC-2: PostgreSQL migrations are idempotent.
func TestPostgres_MigrationsIdempotent(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("failed to open PostgreSQL connection: %v", err)
	}
	defer func() { _ = drv.Close() }()

	// Clean slate
	resetTestDB(t, drv.DB())

	mockFS := &testSchemaFS{t: t}
	ctx := context.Background()

	// First run
	if err := drv.Migrate(ctx, mockFS, "global"); err != nil {
		t.Fatalf("first Migrate failed: %v", err)
	}

	// Second run - should succeed (idempotent)
	if err := drv.Migrate(ctx, mockFS, "global"); err != nil {
		t.Fatalf("second Migrate failed (not idempotent): %v", err)
	}

	// Third run - just to be sure
	if err := drv.Migrate(ctx, mockFS, "global"); err != nil {
		t.Fatalf("third Migrate failed (not idempotent): %v", err)
	}
}

// TestPostgres_MigrationsPartialApply verifies partial migrations can resume.
// Covers SC-2 edge case: Partially migrated database applies remaining migrations.
func TestPostgres_MigrationsPartialApply(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("failed to open PostgreSQL connection: %v", err)
	}
	defer func() { _ = drv.Close() }()

	// Clean slate
	resetTestDB(t, drv.DB())

	ctx := context.Background()

	// Create migrations table and mark some as applied
	_, err := drv.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("failed to create _migrations table: %v", err)
	}

	// Mark migrations 1-5 as applied (but don't actually run them)
	for i := 1; i <= 5; i++ {
		_, err := drv.Exec(ctx, "INSERT INTO _migrations (version) VALUES ($1)", i)
		if err != nil {
			t.Fatalf("failed to insert migration %d: %v", i, err)
		}
	}

	mockFS := &testSchemaFS{t: t}

	// This should apply migrations 6-10
	if err := drv.Migrate(ctx, mockFS, "global"); err != nil {
		t.Fatalf("partial Migrate failed: %v", err)
	}

	// Verify migrations 6-10 are now recorded
	for i := 6; i <= 10; i++ {
		var version int
		err := drv.QueryRow(ctx, "SELECT version FROM _migrations WHERE version = $1", i).Scan(&version)
		if err != nil {
			t.Errorf("migration %d not recorded after partial apply: %v", i, err)
		}
	}
}

// TestPostgres_TablesHaveCorrectTypes verifies PostgreSQL-specific types are used.
// Covers SC-3: PostgreSQL migrations use correct dialect syntax.
func TestPostgres_TablesHaveCorrectTypes(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("failed to open PostgreSQL connection: %v", err)
	}
	defer func() { _ = drv.Close() }()

	// Clean slate
	resetTestDB(t, drv.DB())

	mockFS := &testSchemaFS{t: t}
	ctx := context.Background()

	if err := drv.Migrate(ctx, mockFS, "global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Verify TIMESTAMP WITH TIME ZONE is used for created_at columns
	var dataType string
	err := drv.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'projects' AND column_name = 'created_at'
	`).Scan(&dataType)
	if err != nil {
		t.Fatalf("failed to query column type: %v", err)
	}

	// PostgreSQL returns "timestamp with time zone" for TIMESTAMPTZ
	if dataType != "timestamp with time zone" {
		t.Errorf("projects.created_at should be TIMESTAMP WITH TIME ZONE, got %s", dataType)
	}

	// Verify SERIAL is used for auto-increment columns
	err = drv.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'cost_log' AND column_name = 'id'
	`).Scan(&dataType)
	if err != nil {
		t.Fatalf("failed to query cost_log.id type: %v", err)
	}

	// SERIAL creates an integer column with a sequence
	if dataType != "integer" && dataType != "bigint" {
		t.Errorf("cost_log.id should be integer (from SERIAL), got %s", dataType)
	}
}

// TestPostgres_IndexesExist verifies all expected indexes are created.
// Covers SC-6: All indexes from SQLite exist in PostgreSQL.
func TestPostgres_IndexesExist(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("failed to open PostgreSQL connection: %v", err)
	}
	defer func() { _ = drv.Close() }()

	// Clean slate
	resetTestDB(t, drv.DB())

	mockFS := &testSchemaFS{t: t}
	ctx := context.Background()

	if err := drv.Migrate(ctx, mockFS, "global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Key indexes that should exist
	expectedIndexes := []string{
		"idx_cost_project",
		"idx_cost_timestamp",
		"idx_cost_model",
		"idx_cost_model_timestamp",
		"idx_cost_initiative",
		"idx_cost_project_timestamp",
		"idx_cost_agg_project_date",
		"idx_cost_agg_model_date",
		"idx_cost_duration",
		"idx_phase_templates_builtin",
		"idx_workflows_builtin",
		"idx_workflows_type",
		"idx_workflow_phases_workflow",
		"idx_workflow_phases_sequence",
		"idx_workflow_variables_workflow",
		"idx_agents_builtin",
		"idx_phase_agents_phase",
		"idx_phase_agents_agent",
		"idx_users_name",
		"idx_cost_log_user",
		"idx_cost_log_project_user",
	}

	for _, idx := range expectedIndexes {
		var exists bool
		err := drv.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT FROM pg_indexes
				WHERE indexname = $1
			)`, idx).Scan(&exists)
		if err != nil {
			t.Errorf("failed to check index %s: %v", idx, err)
		}
		if !exists {
			t.Errorf("expected index %s to exist", idx)
		}
	}
}

// testSchemaFS implements SchemaFS for integration tests using embedded files.
type testSchemaFS struct {
	t *testing.T
}

func (fs *testSchemaFS) ReadDir(name string) ([]DirEntry, error) {
	// This assumes the test is run from the project root or with proper working directory
	// In practice, we need to use the embedded schemaFS from db.go
	// For integration tests, we'll use os.ReadDir
	entries, err := os.ReadDir("../schema/postgres")
	if err != nil {
		// Try alternate path
		entries, err = os.ReadDir("internal/db/schema/postgres")
		if err != nil {
			fs.t.Logf("ReadDir failed for both paths: %v", err)
			return nil, err
		}
	}

	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = testDirEntry{e}
	}
	return result, nil
}

func (fs *testSchemaFS) ReadFile(name string) ([]byte, error) {
	// Try relative path
	content, err := os.ReadFile("../" + name)
	if err != nil {
		// Try from project root
		content, err = os.ReadFile("internal/db/" + name)
	}
	return content, err
}

type testDirEntry struct {
	os.DirEntry
}

func (e testDirEntry) Name() string { return e.DirEntry.Name() }
func (e testDirEntry) IsDir() bool  { return e.DirEntry.IsDir() }
