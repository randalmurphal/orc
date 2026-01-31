// Package db provides database persistence for orc.
//
// Two databases are used:
//   - Global (~/.orc/orc.db): projects registry, cost tracking, templates
//   - Project (.orc/orc.db): tasks, phases, transcripts with FTS
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/randalmurphal/orc/internal/db/driver"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// embedFSAdapter wraps embed.FS to implement driver.SchemaFS.
type embedFSAdapter struct {
	fs embed.FS
}

func (e *embedFSAdapter) ReadDir(name string) ([]driver.DirEntry, error) {
	entries, err := e.fs.ReadDir(name)
	if err != nil {
		return nil, err
	}
	result := make([]driver.DirEntry, len(entries))
	for i, entry := range entries {
		result[i] = dirEntryAdapter{entry}
	}
	return result, nil
}

func (e *embedFSAdapter) ReadFile(name string) ([]byte, error) {
	return e.fs.ReadFile(name)
}

type dirEntryAdapter struct {
	fs.DirEntry
}

func (d dirEntryAdapter) Name() string {
	return d.DirEntry.Name()
}

func (d dirEntryAdapter) IsDir() bool {
	return d.DirEntry.IsDir()
}

// DB wraps a database connection with driver abstraction.
type DB struct {
	driver driver.Driver
	path   string
}

// Open opens a SQLite database at the given path.
// Creates the parent directory if it doesn't exist.
// This is the default constructor that uses SQLite for backward compatibility.
func Open(path string) (*DB, error) {
	return OpenWithDialect(path, driver.DialectSQLite)
}

// OpenInMemory opens an in-memory SQLite database.
// This is much faster than file-based databases and ideal for testing.
// Each call creates a new isolated database.
func OpenInMemory() (*DB, error) {
	drv, err := driver.New(driver.DialectSQLite)
	if err != nil {
		return nil, err
	}

	// Use :memory: for in-memory database
	if err := drv.Open(":memory:"); err != nil {
		return nil, err
	}

	return &DB{driver: drv, path: ":memory:"}, nil
}

// OpenWithDialect opens a database with a specific dialect.
func OpenWithDialect(dsn string, dialect driver.Dialect) (*DB, error) {
	// For SQLite, create parent directory if needed
	if dialect == driver.DialectSQLite {
		dir := filepath.Dir(dsn)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	drv, err := driver.New(dialect)
	if err != nil {
		return nil, err
	}

	if err := drv.Open(dsn); err != nil {
		return nil, err
	}

	return &DB{driver: drv, path: dsn}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.driver.Close()
}

// Path returns the database DSN/path.
func (d *DB) Path() string {
	return d.path
}

// DB returns the underlying sql.DB for advanced operations.
func (d *DB) DB() *sql.DB {
	return d.driver.DB()
}

// Driver returns the underlying driver for dialect-specific operations.
func (d *DB) Driver() driver.Driver {
	return d.driver
}

// Dialect returns the database dialect.
func (d *DB) Dialect() driver.Dialect {
	return d.driver.Dialect()
}

// Migrate runs all migrations for the given schema type.
// Schema files are expected to be named: {type}_NNN.sql (e.g., global_001.sql)
func (d *DB) Migrate(schemaType string) error {
	adapter := &embedFSAdapter{fs: schemaFS}
	return d.driver.Migrate(context.Background(), adapter, schemaType)
}

// Exec executes a query without returning rows.
func (d *DB) Exec(query string, args ...any) (sql.Result, error) {
	return d.driver.Exec(context.Background(), query, args...)
}

// ExecContext executes a query without returning rows with context.
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.driver.Exec(ctx, query, args...)
}

// Query executes a query that returns rows.
func (d *DB) Query(query string, args ...any) (*sql.Rows, error) {
	return d.driver.Query(context.Background(), query, args...)
}

// QueryContext executes a query that returns rows with context.
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.driver.Query(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (d *DB) QueryRow(query string, args ...any) *sql.Row {
	return d.driver.QueryRow(context.Background(), query, args...)
}

// QueryRowContext executes a query that returns at most one row with context.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.driver.QueryRow(ctx, query, args...)
}

// BeginTx starts a transaction.
func (d *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (driver.Tx, error) {
	return d.driver.BeginTx(ctx, opts)
}

// Placeholder returns the appropriate placeholder for the database dialect.
func (d *DB) Placeholder(index int) string {
	return d.driver.Placeholder(index)
}

// Now returns the SQL function for current timestamp.
func (d *DB) Now() string {
	return d.driver.Now()
}
