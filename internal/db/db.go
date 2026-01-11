// Package db provides SQLite persistence for orc.
//
// Two databases are used:
//   - Global (~/.orc/orc.db): projects registry, cost tracking, templates
//   - Project (.orc/orc.db): tasks, phases, transcripts with FTS
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// DB wraps a SQLite database connection.
type DB struct {
	db   *sql.DB
	path string
}

// Open opens a SQLite database at the given path.
// Creates the parent directory if it doesn't exist.
func Open(path string) (*DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable foreign keys and WAL mode for better performance
	if _, err := db.Exec(`
		PRAGMA foreign_keys = ON;
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set pragmas: %w", err)
	}

	return &DB{db: db, path: path}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// Path returns the database file path.
func (d *DB) Path() string {
	return d.path
}

// DB returns the underlying sql.DB for advanced operations.
func (d *DB) DB() *sql.DB {
	return d.db
}

// Migrate runs all migrations for the given schema type.
// Schema files are expected to be named: {type}_NNN.sql (e.g., global_001.sql)
func (d *DB) Migrate(schemaType string) error {
	// Create migrations table if it doesn't exist
	if _, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS _migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Get applied versions
	applied := make(map[int]bool)
	rows, err := d.db.Query("SELECT version FROM _migrations")
	if err != nil {
		return fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("scan migration version: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate migrations: %w", err)
	}

	// Find and sort migration files
	entries, err := schemaFS.ReadDir("schema")
	if err != nil {
		return fmt.Errorf("read schema dir: %w", err)
	}

	var migrations []string
	prefix := schemaType + "_"
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".sql") {
			migrations = append(migrations, e.Name())
		}
	}
	sort.Strings(migrations)

	// Apply pending migrations
	for _, name := range migrations {
		version := extractVersion(name, prefix)
		if applied[version] {
			continue
		}

		content, err := schemaFS.ReadFile("schema/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := d.db.Begin()
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := tx.Exec("INSERT INTO _migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}

// extractVersion extracts version number from migration filename.
// e.g., "global_001.sql" with prefix "global_" returns 1
func extractVersion(name, prefix string) int {
	s := strings.TrimPrefix(name, prefix)
	s = strings.TrimSuffix(s, ".sql")
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

// Exec executes a query without returning rows.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	return d.db.Exec(query, args...)
}

// Query executes a query that returns rows.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return d.db.Query(query, args...)
}

// QueryRow executes a query that returns at most one row.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	return d.db.QueryRow(query, args...)
}
