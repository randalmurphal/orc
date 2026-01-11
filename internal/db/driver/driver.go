// Package driver provides database driver abstraction for SQLite and PostgreSQL.
package driver

import (
	"context"
	"database/sql"
	"fmt"
)

// Dialect represents the database dialect.
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

// Driver abstracts database operations for SQLite and PostgreSQL.
type Driver interface {
	// Connection
	Open(dsn string) error
	Close() error

	// Queries
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row

	// Transactions
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)

	// Migrations
	Migrate(ctx context.Context, schemaFS SchemaFS, schemaType string) error

	// Dialect-specific
	Dialect() Dialect
	Placeholder(index int) string // $1 for Postgres, ? for SQLite

	// SQL helpers for dialect differences
	Now() string            // datetime('now') for SQLite, NOW() for Postgres
	UpsertConflict() string // ON CONFLICT syntax varies

	// Raw access (for advanced operations)
	DB() *sql.DB
}

// Tx wraps database transactions.
type Tx interface {
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Commit() error
	Rollback() error
}

// SchemaFS provides access to embedded schema files.
type SchemaFS interface {
	ReadDir(name string) ([]DirEntry, error)
	ReadFile(name string) ([]byte, error)
}

// DirEntry represents a directory entry.
type DirEntry interface {
	Name() string
	IsDir() bool
}

// Config holds driver configuration.
type Config struct {
	Dialect Dialect
	DSN     string
}

// New creates a driver based on configuration.
func New(dialect Dialect) (Driver, error) {
	switch dialect {
	case DialectSQLite:
		return NewSQLite(), nil
	case DialectPostgres:
		return NewPostgres(), nil
	default:
		return nil, fmt.Errorf("unsupported dialect: %s", dialect)
	}
}

// ParseDialect parses a dialect string.
func ParseDialect(s string) (Dialect, error) {
	switch s {
	case "sqlite", "sqlite3":
		return DialectSQLite, nil
	case "postgres", "postgresql", "pg":
		return DialectPostgres, nil
	default:
		return "", fmt.Errorf("unknown dialect: %s", s)
	}
}

// sqlTx wraps a sql.Tx to implement the Tx interface.
type sqlTx struct {
	tx *sql.Tx
}

func (t *sqlTx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(ctx, query, args...)
}

func (t *sqlTx) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.tx.QueryContext(ctx, query, args...)
}

func (t *sqlTx) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(ctx, query, args...)
}

func (t *sqlTx) Commit() error {
	return t.tx.Commit()
}

func (t *sqlTx) Rollback() error {
	return t.tx.Rollback()
}
