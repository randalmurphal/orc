//go:build integration

package driver

import (
	"context"
	"database/sql"
	"os"
	"testing"
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

	// Drop tables in reverse dependency order (global + project tables)
	tables := []string{
		// Project tables (reverse dependency order)
		"qa_results",
		"review_findings",
		"trigger_metrics",
		"trigger_counters",
		"trigger_executions",
		"automation_triggers",
		"notifications",
		"branches",
		"sync_state",
		"task_attachments",
		"gate_decisions",
		"specs",
		"plans",
		"initiative_dependencies",
		"task_comments",
		"knowledge_queue",
		"activity_log",
		"task_claims",
		"team_members",
		"review_comments",
		"subtask_queue",
		"task_dependencies",
		"initiative_tasks",
		"initiative_decisions",
		"initiative_decisions_new",
		"initiatives",
		"transcripts",
		"phases",
		"detection",
		"tasks",
		// Global tables
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

// ============================================================================
// Project Migration Integration Tests (SC-6, SC-8, SC-9)
// ============================================================================

// TestPostgres_ProjectMigrationsApply verifies all 20 project migrations apply successfully
// on a PostgreSQL instance.
// Covers SC-8: All project tables from migrations 001-020 exist in information_schema.
// Covers SC-6: Migration 017 trigger fires correctly — updating a task row causes updated_at to change.
func TestPostgres_ProjectMigrationsApply(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("failed to open PostgreSQL connection: %v", err)
	}
	defer func() { _ = drv.Close() }()

	resetTestDB(t, drv.DB())

	mockFS := &testSchemaFS{t: t}
	ctx := context.Background()

	// Apply project migrations
	if err := drv.Migrate(ctx, mockFS, "project"); err != nil {
		t.Fatalf("project Migrate failed: %v", err)
	}

	// Verify all expected project tables exist (BDD-1)
	expectedTables := []string{
		"detection",
		"tasks",
		"phases",
		"transcripts",
		"initiatives",
		"initiative_decisions",
		"initiative_tasks",
		"task_dependencies",
		"subtask_queue",
		"review_comments",
		"team_members",
		"task_claims",
		"activity_log",
		"knowledge_queue",
		"task_comments",
		"initiative_dependencies",
		"plans",
		"specs",
		"gate_decisions",
		"task_attachments",
		"sync_state",
		"branches",
		"automation_triggers",
		"trigger_executions",
		"trigger_counters",
		"trigger_metrics",
		"notifications",
		"review_findings",
		"qa_results",
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
			t.Errorf("expected project table %s to exist after migrations", table)
		}
	}

	// Verify migration versions 1-20 are recorded
	for i := 1; i <= 20; i++ {
		var version int
		err := drv.QueryRow(ctx, "SELECT version FROM _migrations WHERE version = $1", i).Scan(&version)
		if err != nil {
			t.Errorf("migration version %d not recorded: %v", i, err)
		}
	}

	// BDD-3: Verify sync_state was initialized with a hex site_id
	var siteID string
	err := drv.QueryRow(ctx, "SELECT site_id FROM sync_state WHERE id = 1").Scan(&siteID)
	if err != nil {
		t.Errorf("failed to query sync_state: %v", err)
	} else if len(siteID) == 0 {
		t.Error("sync_state.site_id should be non-empty")
	}

	// BDD-4: Verify initiative_decisions has composite primary key (id, initiative_id)
	var constraintName string
	err = drv.QueryRow(ctx, `
		SELECT constraint_name FROM information_schema.table_constraints
		WHERE table_name = 'initiative_decisions'
		AND constraint_type = 'PRIMARY KEY'
	`).Scan(&constraintName)
	if err != nil {
		t.Errorf("failed to query initiative_decisions primary key: %v", err)
	}

	// Verify PK columns
	var pkCols []string
	rows, err := drv.Query(ctx, `
		SELECT column_name FROM information_schema.key_column_usage
		WHERE table_name = 'initiative_decisions'
		AND constraint_name = $1
		ORDER BY ordinal_position
	`, constraintName)
	if err != nil {
		t.Fatalf("failed to query PK columns: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			t.Fatalf("failed to scan PK column: %v", err)
		}
		pkCols = append(pkCols, col)
	}
	if len(pkCols) != 2 || pkCols[0] != "id" || pkCols[1] != "initiative_id" {
		t.Errorf("initiative_decisions should have composite PK (id, initiative_id), got %v", pkCols)
	}

	// SC-6 / BDD-2: Verify updated_at trigger fires on task UPDATE
	_, err = drv.Exec(ctx, `
		INSERT INTO tasks (id, title, status, created_at, updated_at)
		VALUES ('TEST-001', 'Test task', 'pending', NOW() - interval '1 hour', NOW() - interval '1 hour')
	`)
	if err != nil {
		t.Fatalf("failed to insert test task: %v", err)
	}

	var beforeUpdate string
	err = drv.QueryRow(ctx, "SELECT updated_at::text FROM tasks WHERE id = 'TEST-001'").Scan(&beforeUpdate)
	if err != nil {
		t.Fatalf("failed to read updated_at before update: %v", err)
	}

	// Wait briefly to ensure timestamp difference
	_, err = drv.Exec(ctx, "SELECT pg_sleep(0.1)")
	if err != nil {
		t.Fatalf("pg_sleep failed: %v", err)
	}

	// Update the task (should trigger updated_at auto-update)
	_, err = drv.Exec(ctx, "UPDATE tasks SET title = 'Updated task' WHERE id = 'TEST-001'")
	if err != nil {
		t.Fatalf("failed to update test task: %v", err)
	}

	var afterUpdate string
	err = drv.QueryRow(ctx, "SELECT updated_at::text FROM tasks WHERE id = 'TEST-001'").Scan(&afterUpdate)
	if err != nil {
		t.Fatalf("failed to read updated_at after update: %v", err)
	}

	if beforeUpdate == afterUpdate {
		t.Errorf("updated_at trigger did not fire: before=%s, after=%s", beforeUpdate, afterUpdate)
	}
}

// TestPostgres_ProjectMigrationsIdempotent verifies project migrations can be run twice.
// Covers SC-9: Running Migrate("project") twice succeeds without errors.
func TestPostgres_ProjectMigrationsIdempotent(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("failed to open PostgreSQL connection: %v", err)
	}
	defer func() { _ = drv.Close() }()

	resetTestDB(t, drv.DB())

	mockFS := &testSchemaFS{t: t}
	ctx := context.Background()

	// First run
	if err := drv.Migrate(ctx, mockFS, "project"); err != nil {
		t.Fatalf("first project Migrate failed: %v", err)
	}

	// Second run — should succeed (idempotent)
	if err := drv.Migrate(ctx, mockFS, "project"); err != nil {
		t.Fatalf("second project Migrate failed (not idempotent): %v", err)
	}
}

// TestPostgres_ProjectTablesHaveCorrectTypes verifies PostgreSQL-specific types in project tables.
// Covers SC-8 (partial): Validates that dialect conversion produced correct column types.
func TestPostgres_ProjectTablesHaveCorrectTypes(t *testing.T) {
	dsn := getTestDSN(t)

	drv := NewPostgres()
	if err := drv.Open(dsn); err != nil {
		t.Fatalf("failed to open PostgreSQL connection: %v", err)
	}
	defer func() { _ = drv.Close() }()

	resetTestDB(t, drv.DB())

	mockFS := &testSchemaFS{t: t}
	ctx := context.Background()

	if err := drv.Migrate(ctx, mockFS, "project"); err != nil {
		t.Fatalf("project Migrate failed: %v", err)
	}

	// Verify TIMESTAMP WITH TIME ZONE for tasks.created_at
	var dataType string
	err := drv.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'tasks' AND column_name = 'created_at'
	`).Scan(&dataType)
	if err != nil {
		t.Fatalf("failed to query tasks.created_at type: %v", err)
	}
	if dataType != "timestamp with time zone" {
		t.Errorf("tasks.created_at should be TIMESTAMP WITH TIME ZONE, got %s", dataType)
	}

	// Verify SERIAL (integer) for transcripts.id
	err = drv.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'transcripts' AND column_name = 'id'
	`).Scan(&dataType)
	if err != nil {
		t.Fatalf("failed to query transcripts.id type: %v", err)
	}
	if dataType != "integer" && dataType != "bigint" {
		t.Errorf("transcripts.id should be integer (from SERIAL), got %s", dataType)
	}

	// Verify BOOLEAN for detection.has_tests
	err = drv.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'detection' AND column_name = 'has_tests'
	`).Scan(&dataType)
	if err != nil {
		t.Fatalf("failed to query detection.has_tests type: %v", err)
	}
	if dataType != "boolean" {
		t.Errorf("detection.has_tests should be BOOLEAN, got %s", dataType)
	}

	// Verify BYTEA for task_attachments.data
	err = drv.QueryRow(ctx, `
		SELECT data_type FROM information_schema.columns
		WHERE table_name = 'task_attachments' AND column_name = 'data'
	`).Scan(&dataType)
	if err != nil {
		t.Fatalf("failed to query task_attachments.data type: %v", err)
	}
	if dataType != "bytea" {
		t.Errorf("task_attachments.data should be BYTEA, got %s", dataType)
	}
}
