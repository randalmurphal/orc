package driver

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // SQLite driver
)

// SQLiteDriver implements the Driver interface for SQLite.
type SQLiteDriver struct {
	db *sql.DB
}

// NewSQLite creates a new SQLite driver.
func NewSQLite() *SQLiteDriver {
	return &SQLiteDriver{}
}

// Open opens a SQLite database at the given path.
func (d *SQLiteDriver) Open(dsn string) error {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	// Enable foreign keys, WAL mode, and busy timeout for concurrent access
	if _, err := db.Exec(`
		PRAGMA foreign_keys = ON;
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA busy_timeout = 5000;
	`); err != nil {
		_ = db.Close()
		return fmt.Errorf("set pragmas: %w", err)
	}

	d.db = db
	return nil
}

// Close closes the database connection.
func (d *SQLiteDriver) Close() error {
	if d.db == nil {
		return nil
	}
	return d.db.Close()
}

// Exec executes a query without returning rows.
func (d *SQLiteDriver) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (d *SQLiteDriver) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (d *SQLiteDriver) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return d.db.QueryRowContext(ctx, query, args...)
}

// BeginTx starts a transaction.
func (d *SQLiteDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := d.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return &sqlTx{tx: tx}, nil
}

// Migrate runs all migrations for the given schema type.
func (d *SQLiteDriver) Migrate(ctx context.Context, schemaFS SchemaFS, schemaType string) error {
	// Create migrations table if it doesn't exist
	if _, err := d.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT (datetime('now'))
		)
	`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Get applied versions
	applied := make(map[int]bool)
	rows, err := d.db.QueryContext(ctx, "SELECT version FROM _migrations")
	if err != nil {
		return fmt.Errorf("query migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

		tx, err := d.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, "INSERT INTO _migrations (version) VALUES (?)", version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}

// Dialect returns the SQLite dialect identifier.
func (d *SQLiteDriver) Dialect() Dialect {
	return DialectSQLite
}

// Placeholder returns the SQLite placeholder (always ?).
func (d *SQLiteDriver) Placeholder(index int) string {
	return "?"
}

// Now returns the SQLite NOW() equivalent.
func (d *SQLiteDriver) Now() string {
	return "datetime('now')"
}

// UpsertConflict returns the SQLite ON CONFLICT syntax prefix.
func (d *SQLiteDriver) UpsertConflict() string {
	return "ON CONFLICT"
}

// DB returns the underlying sql.DB for advanced operations.
func (d *SQLiteDriver) DB() *sql.DB {
	return d.db
}

// extractVersion extracts version number from migration filename.
// e.g., "global_001.sql" with prefix "global_" returns 1
func extractVersion(name, prefix string) int {
	s := strings.TrimPrefix(name, prefix)
	s = strings.TrimSuffix(s, ".sql")
	var v int
	_, _ = fmt.Sscanf(s, "%d", &v)
	return v
}
