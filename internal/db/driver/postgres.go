package driver

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// PostgresDriver implements the Driver interface for PostgreSQL.
// It uses pgxpool for connection pooling with a sql.DB adapter for interface compatibility.
type PostgresDriver struct {
	db   *sql.DB
	pool *pgxpool.Pool
}

// NewPostgres creates a new PostgreSQL driver.
func NewPostgres() *PostgresDriver {
	return &PostgresDriver{}
}

// Open opens a PostgreSQL database connection using pgxpool.
func (d *PostgresDriver) Open(dsn string) error {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse postgres config: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("create postgres pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}

	d.pool = pool
	d.db = stdlib.OpenDBFromPool(pool)
	return nil
}

// Close closes the database connection and pool.
func (d *PostgresDriver) Close() error {
	if d.pool == nil {
		return nil
	}
	err := d.db.Close()
	d.pool.Close()
	return err
}

// Exec executes a query without returning rows.
func (d *PostgresDriver) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (d *PostgresDriver) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (d *PostgresDriver) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return d.db.QueryRowContext(ctx, query, args...)
}

// BeginTx starts a transaction.
func (d *PostgresDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := d.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return &sqlTx{tx: tx}, nil
}

// Migrate runs all migrations for the given schema type.
// PostgreSQL migrations are read from schema/postgres/{type}_NNN.sql files.
func (d *PostgresDriver) Migrate(ctx context.Context, schemaFS SchemaFS, schemaType string) error {
	// Create migrations table if it doesn't exist
	if _, err := d.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrations (
			version INTEGER PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
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

	// Find and sort PostgreSQL-specific migration files
	entries, err := schemaFS.ReadDir("schema/postgres")
	if err != nil {
		return fmt.Errorf("read postgres schema dir: %w", err)
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

		content, err := schemaFS.ReadFile("schema/postgres/" + name)
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

		if _, err := tx.ExecContext(ctx, "INSERT INTO _migrations (version) VALUES ($1)", version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}

// Dialect returns the PostgreSQL dialect identifier.
func (d *PostgresDriver) Dialect() Dialect {
	return DialectPostgres
}

// Placeholder returns the PostgreSQL placeholder ($1, $2, etc.).
func (d *PostgresDriver) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// Now returns the PostgreSQL NOW() function.
func (d *PostgresDriver) Now() string {
	return "NOW()"
}

// UpsertConflict returns the PostgreSQL ON CONFLICT syntax prefix.
func (d *PostgresDriver) UpsertConflict() string {
	return "ON CONFLICT"
}

// DB returns the underlying sql.DB backed by pgxpool for advanced operations.
func (d *PostgresDriver) DB() *sql.DB {
	return d.db
}
