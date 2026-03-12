package driver

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // SQLite driver
)

// SQLiteDriver implements the Driver interface for SQLite.
type SQLiteDriver struct {
	db   *sql.DB
	noFK bool // When true, foreign key constraints are disabled
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
	pragmas := `
		PRAGMA foreign_keys = ON;
		PRAGMA journal_mode = WAL;
		PRAGMA synchronous = NORMAL;
		PRAGMA busy_timeout = 5000;
	`
	if d.noFK {
		pragmas = `
			PRAGMA foreign_keys = OFF;
			PRAGMA journal_mode = WAL;
			PRAGMA synchronous = NORMAL;
			PRAGMA busy_timeout = 5000;
		`
	}
	// For NoFK mode, force a single connection so every operation reuses the
	// connection where FK=OFF was set. Without this, the connection pool may
	// hand out new connections that don't have the pragma applied.
	if d.noFK {
		db.SetMaxOpenConns(1)
	}

	if _, err := db.Exec(pragmas); err != nil {
		_ = db.Close()
		return fmt.Errorf("set pragmas: %w", err)
	}

	d.db = db
	return nil
}

// NewSQLiteNoFK creates a SQLite driver with foreign key constraints disabled.
// Used for scratch databases where FK violations don't matter.
func NewSQLiteNoFK() *SQLiteDriver {
	return &SQLiteDriver{noFK: true}
}

// Close closes the database connection.
// In WAL mode, we checkpoint before closing to ensure all writes are
// visible to new connections that open after this one closes.
func (d *SQLiteDriver) Close() error {
	if d.db == nil {
		return nil
	}
	// Checkpoint WAL to ensure all writes are flushed to the main database.
	// This prevents race conditions when multiple connections open/close rapidly.
	// TRUNCATE mode checkpoints and then truncates the WAL file.
	_, _ = d.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
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

		contentStr := string(content)

		// Check if migration needs FK constraints disabled.
		// This is required for complex schema changes that restructure tables with FK references.
		// SQLite's PRAGMA foreign_keys cannot be changed inside a transaction, so these
		// migrations run without a wrapping transaction.
		// Marker: "-- orc:disable_fk" at start of migration file
		needsFKOff := strings.HasPrefix(contentStr, "-- orc:disable_fk")

		if needsFKOff {
			if err := d.applyMigrationWithFKDisabled(ctx, name, contentStr, version); err != nil {
				return err
			}
		} else {
			if err := d.applyMigrationInTx(ctx, name, contentStr, version); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyMigrationInTx applies a migration within a transaction (standard path).
func (d *SQLiteDriver) applyMigrationInTx(ctx context.Context, name, content string, version int) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if _, err := tx.ExecContext(ctx, content); err != nil {
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

	return nil
}

// applyMigrationWithFKDisabled applies a migration with foreign keys disabled.
// Required for migrations that restructure tables with FK references.
// Follows SQLite's recommended pattern: disable FKs, run DDL, re-enable, verify integrity.
func (d *SQLiteDriver) applyMigrationWithFKDisabled(ctx context.Context, name, content string, version int) error {
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for %s: %w", name, err)
	}
	defer func() { _ = conn.Close() }()

	preViolations, err := collectForeignKeyViolations(ctx, conn)
	if err != nil {
		return fmt.Errorf("check foreign keys before %s: %w", name, err)
	}

	// Disable foreign key enforcement (must be outside transaction)
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disable foreign keys for %s: %w", name, err)
	}

	// Re-enable FKs after migration (unless permanently disabled)
	if !d.noFK {
		defer func() {
			_, _ = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON")
		}()
	}

	// Run migration in transaction for atomicity
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if _, err := tx.ExecContext(ctx, content); err != nil {
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

	// Re-enable foreign keys and verify integrity (skip if FK permanently disabled)
	if d.noFK {
		return nil
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("re-enable foreign keys after %s: %w", name, err)
	}

	postViolations, err := collectForeignKeyViolations(ctx, conn)
	if err != nil {
		return fmt.Errorf("check foreign keys after %s: %w", name, err)
	}

	newViolations := difference(postViolations, preViolations)
	if len(newViolations) > 0 {
		return fmt.Errorf("migration %s introduced FK violations: %v", name, newViolations)
	}

	return nil
}

type foreignKeyChecker interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func collectForeignKeyViolations(ctx context.Context, checker foreignKeyChecker) ([]string, error) {
	rows, err := checker.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var violations []string
	for rows.Next() {
		var table, rowid, parent string
		var fkid int
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return nil, fmt.Errorf("scan FK violation: %w", err)
		}
		violations = append(violations, fmt.Sprintf("%s.rowid=%s->%s#fk%d", table, rowid, parent, fkid))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate FK violations: %w", err)
	}

	slices.Sort(violations)
	return violations, nil
}

func difference(items, baseline []string) []string {
	baselineSet := make(map[string]struct{}, len(baseline))
	for _, item := range baseline {
		baselineSet[item] = struct{}{}
	}

	diff := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := baselineSet[item]; ok {
			continue
		}
		diff = append(diff, item)
	}
	return diff
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

// DateFormat returns a SQLite strftime() expression for date formatting.
// Supported formats: day, week, month, rfc3339.
func (d *SQLiteDriver) DateFormat(column, format string) string {
	var fmtStr string
	switch format {
	case "day":
		fmtStr = "%Y-%m-%d"
	case "week":
		fmtStr = "%Y-W%W"
	case "month":
		fmtStr = "%Y-%m"
	case "rfc3339":
		fmtStr = "%Y-%m-%dT%H:%M:%SZ"
	default:
		fmtStr = "%Y-%m-%d"
	}
	return fmt.Sprintf("strftime('%s', %s)", fmtStr, column)
}

// DateTrunc returns a SQLite strftime() expression for date truncation.
// Supported units: day, month, year.
func (d *SQLiteDriver) DateTrunc(unit, column string) string {
	var fmtStr string
	switch unit {
	case "day":
		fmtStr = "%Y-%m-%d"
	case "month":
		fmtStr = "%Y-%m-01"
	case "year":
		fmtStr = "%Y-01-01"
	default:
		fmtStr = "%Y-%m-%d"
	}
	return fmt.Sprintf("strftime('%s', %s)", fmtStr, column)
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
