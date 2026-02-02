package driver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDriver(t *testing.T) {
	tests := []struct {
		name    string
		dialect Dialect
		wantErr bool
	}{
		{"sqlite", DialectSQLite, false},
		{"postgres", DialectPostgres, false},
		{"invalid", Dialect("invalid"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drv, err := New(tt.dialect)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if drv == nil {
				t.Error("expected driver, got nil")
			}
		})
	}
}

func TestParseDialect(t *testing.T) {
	tests := []struct {
		input   string
		want    Dialect
		wantErr bool
	}{
		{"sqlite", DialectSQLite, false},
		{"sqlite3", DialectSQLite, false},
		{"postgres", DialectPostgres, false},
		{"postgresql", DialectPostgres, false},
		{"pg", DialectPostgres, false},
		{"mysql", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDialect(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSQLiteDriver(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	drv := NewSQLite()

	// Test Open
	if err := drv.Open(dbPath); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = drv.Close() }()

	// Test Dialect
	if drv.Dialect() != DialectSQLite {
		t.Errorf("Dialect() = %v, want %v", drv.Dialect(), DialectSQLite)
	}

	// Test Placeholder
	if drv.Placeholder(1) != "?" {
		t.Errorf("Placeholder(1) = %v, want ?", drv.Placeholder(1))
	}

	// Test Now
	if drv.Now() != "datetime('now')" {
		t.Errorf("Now() = %v, want datetime('now')", drv.Now())
	}

	// Test DB
	if drv.DB() == nil {
		t.Error("DB() returned nil")
	}

	// Test basic Exec
	ctx := context.Background()
	_, err := drv.Exec(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Errorf("Exec CREATE TABLE failed: %v", err)
	}

	// Test Insert
	result, err := drv.Exec(ctx, "INSERT INTO test (name) VALUES (?)", "hello")
	if err != nil {
		t.Errorf("Exec INSERT failed: %v", err)
	}
	id, _ := result.LastInsertId()
	if id != 1 {
		t.Errorf("LastInsertId() = %d, want 1", id)
	}

	// Test Query
	rows, err := drv.Query(ctx, "SELECT id, name FROM test WHERE id = ?", 1)
	if err != nil {
		t.Errorf("Query failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		t.Error("expected row, got none")
	}
	var gotID int
	var gotName string
	if err := rows.Scan(&gotID, &gotName); err != nil {
		t.Errorf("Scan failed: %v", err)
	}
	if gotID != 1 || gotName != "hello" {
		t.Errorf("got (%d, %q), want (1, 'hello')", gotID, gotName)
	}

	// Test QueryRow
	row := drv.QueryRow(ctx, "SELECT name FROM test WHERE id = ?", 1)
	var name string
	if err := row.Scan(&name); err != nil {
		t.Errorf("QueryRow Scan failed: %v", err)
	}
	if name != "hello" {
		t.Errorf("got %q, want 'hello'", name)
	}

	// Test BeginTx
	tx, err := drv.BeginTx(ctx, nil)
	if err != nil {
		t.Errorf("BeginTx failed: %v", err)
	}

	_, err = tx.Exec(ctx, "INSERT INTO test (name) VALUES (?)", "world")
	if err != nil {
		t.Errorf("tx.Exec failed: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Errorf("tx.Commit failed: %v", err)
	}

	// Verify committed
	var count int
	row = drv.QueryRow(ctx, "SELECT COUNT(*) FROM test")
	if err := row.Scan(&count); err != nil {
		t.Errorf("count scan failed: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}

	// Test Rollback
	tx2, _ := drv.BeginTx(ctx, nil)
	_, _ = tx2.Exec(ctx, "INSERT INTO test (name) VALUES (?)", "rollback")
	if err := tx2.Rollback(); err != nil {
		t.Errorf("tx.Rollback failed: %v", err)
	}

	row = drv.QueryRow(ctx, "SELECT COUNT(*) FROM test")
	if err := row.Scan(&count); err != nil {
		t.Errorf("count scan failed: %v", err)
	}
	if count != 2 {
		t.Errorf("count after rollback = %d, want 2", count)
	}
}

func TestSQLiteDriver_Close(t *testing.T) {
	drv := NewSQLite()

	// Close without Open should not error
	if err := drv.Close(); err != nil {
		t.Errorf("Close without Open failed: %v", err)
	}
}

func TestPostgresDriver_Placeholder(t *testing.T) {
	drv := NewPostgres()

	tests := []struct {
		index int
		want  string
	}{
		{1, "$1"},
		{2, "$2"},
		{10, "$10"},
	}

	for _, tt := range tests {
		got := drv.Placeholder(tt.index)
		if got != tt.want {
			t.Errorf("Placeholder(%d) = %q, want %q", tt.index, got, tt.want)
		}
	}
}

func TestPostgresDriver_Dialect(t *testing.T) {
	drv := NewPostgres()

	if drv.Dialect() != DialectPostgres {
		t.Errorf("Dialect() = %v, want %v", drv.Dialect(), DialectPostgres)
	}

	if drv.Now() != "NOW()" {
		t.Errorf("Now() = %v, want NOW()", drv.Now())
	}
}

func TestPostgresDriver_Close(t *testing.T) {
	drv := NewPostgres()

	// Close without Open should not error
	if err := drv.Close(); err != nil {
		t.Errorf("Close without Open failed: %v", err)
	}
}

// TestSQLiteMigrate tests the migration functionality for SQLite
func TestSQLiteMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "migrate_test.db")

	drv := NewSQLite()
	if err := drv.Open(dbPath); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = drv.Close() }()

	// Create a mock schema FS
	schemaDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("create schema dir: %v", err)
	}

	// Write a test migration
	migration := `
		CREATE TABLE IF NOT EXISTS test_table (
			id INTEGER PRIMARY KEY,
			name TEXT
		);
	`
	if err := os.WriteFile(filepath.Join(schemaDir, "test_001.sql"), []byte(migration), 0644); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	// Create a mock SchemaFS
	mockFS := &mockSchemaFS{dir: tmpDir}

	ctx := context.Background()
	if err := drv.Migrate(ctx, mockFS, "test"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Verify table was created
	var name string
	err := drv.QueryRow(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&name)
	if err != nil {
		t.Errorf("test_table not created: %v", err)
	}

	// Run again - should be idempotent
	if err := drv.Migrate(ctx, mockFS, "test"); err != nil {
		t.Errorf("second Migrate failed: %v", err)
	}
}

// mockSchemaFS implements SchemaFS for testing
type mockSchemaFS struct {
	dir string
}

func (m *mockSchemaFS) ReadDir(name string) ([]DirEntry, error) {
	entries, err := os.ReadDir(filepath.Join(m.dir, name))
	if err != nil {
		return nil, err
	}
	result := make([]DirEntry, len(entries))
	for i, e := range entries {
		result[i] = mockDirEntry{e}
	}
	return result, nil
}

func (m *mockSchemaFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(m.dir, name))
}

type mockDirEntry struct {
	os.DirEntry
}

func (m mockDirEntry) Name() string { return m.DirEntry.Name() }
func (m mockDirEntry) IsDir() bool  { return m.DirEntry.IsDir() }

// TestSQLiteMigrateWithFKDisable tests migrations that require FK constraints disabled
func TestSQLiteMigrateWithFKDisable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "migrate_fk_test.db")

	drv := NewSQLite()
	if err := drv.Open(dbPath); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = drv.Close() }()

	schemaDir := filepath.Join(tmpDir, "schema")
	if err := os.MkdirAll(schemaDir, 0755); err != nil {
		t.Fatalf("create schema dir: %v", err)
	}

	// Migration 1: Create parent and child tables with FK
	migration1 := `
		CREATE TABLE parent (id INTEGER PRIMARY KEY, name TEXT);
		CREATE TABLE child (
			id INTEGER PRIMARY KEY,
			parent_id INTEGER,
			FOREIGN KEY (parent_id) REFERENCES parent(id)
		);
		INSERT INTO parent (id, name) VALUES (1, 'test');
		INSERT INTO child (id, parent_id) VALUES (1, 1);
	`
	if err := os.WriteFile(filepath.Join(schemaDir, "test_001.sql"), []byte(migration1), 0644); err != nil {
		t.Fatalf("write migration 1: %v", err)
	}

	// Migration 2: Restructure tables (requires FK disabled)
	// This simulates what project_052.sql does - renaming a referenced table
	migration2 := `-- orc:disable_fk
		-- Rename parent to parent_storage and create a view
		ALTER TABLE parent RENAME TO parent_storage;
		CREATE VIEW parent AS SELECT * FROM parent_storage;

		-- Recreate child without FK (since parent is now a view)
		CREATE TABLE child_new (id INTEGER PRIMARY KEY, parent_id INTEGER);
		INSERT INTO child_new SELECT * FROM child;
		DROP TABLE child;
		ALTER TABLE child_new RENAME TO child;
	`
	if err := os.WriteFile(filepath.Join(schemaDir, "test_002.sql"), []byte(migration2), 0644); err != nil {
		t.Fatalf("write migration 2: %v", err)
	}

	mockFS := &mockSchemaFS{dir: tmpDir}
	ctx := context.Background()

	// Apply migrations
	if err := drv.Migrate(ctx, mockFS, "test"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Verify parent_storage table exists
	var name string
	err := drv.QueryRow(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='parent_storage'").Scan(&name)
	if err != nil {
		t.Errorf("parent_storage table not created: %v", err)
	}

	// Verify parent view exists
	err = drv.QueryRow(ctx, "SELECT name FROM sqlite_master WHERE type='view' AND name='parent'").Scan(&name)
	if err != nil {
		t.Errorf("parent view not created: %v", err)
	}

	// Verify data is still accessible through view
	var count int
	err = drv.QueryRow(ctx, "SELECT COUNT(*) FROM parent").Scan(&count)
	if err != nil || count != 1 {
		t.Errorf("expected 1 row in parent view, got %d: %v", count, err)
	}

	// Verify FKs are re-enabled after migration
	var fkEnabled int
	err = drv.QueryRow(ctx, "PRAGMA foreign_keys").Scan(&fkEnabled)
	if err != nil || fkEnabled != 1 {
		t.Errorf("foreign keys should be re-enabled after migration, got %d", fkEnabled)
	}
}
