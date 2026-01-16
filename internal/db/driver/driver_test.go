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
