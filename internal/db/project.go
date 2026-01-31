package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
	"github.com/randalmurphal/orc/internal/project"
)

// TxRunner provides a transactional execution interface.
// This allows operations to run within a transaction context,
// ensuring atomicity of multi-table operations.
type TxRunner interface {
	// RunInTx executes the given function within a transaction.
	// If fn returns an error, the transaction is rolled back.
	// If fn returns nil, the transaction is committed.
	RunInTx(ctx context.Context, fn func(tx *TxOps) error) error
}

// TxOps provides database operations within a transaction.
// It wraps a driver.Tx to provide the same interface as ProjectDB
// but executes all operations within the transaction.
// The context is stored and used for all operations, enabling cancellation
// and timeout propagation through the entire transaction.
type TxOps struct {
	tx      driver.Tx
	dialect driver.Dialect
	ctx     context.Context
}

// Exec executes a query within the transaction.
// Uses the context passed when the transaction was created.
func (t *TxOps) Exec(query string, args ...any) (sql.Result, error) {
	return t.tx.Exec(t.ctx, query, args...)
}

// Query executes a query that returns rows within the transaction.
// Uses the context passed when the transaction was created.
func (t *TxOps) Query(query string, args ...any) (*sql.Rows, error) {
	return t.tx.Query(t.ctx, query, args...)
}

// QueryRow executes a query that returns at most one row within the transaction.
// Uses the context passed when the transaction was created.
func (t *TxOps) QueryRow(query string, args ...any) *sql.Row {
	return t.tx.QueryRow(t.ctx, query, args...)
}

// Context returns the context associated with this transaction.
func (t *TxOps) Context() context.Context {
	return t.ctx
}

// Dialect returns the database dialect.
func (t *TxOps) Dialect() driver.Dialect {
	return t.dialect
}

// ProjectDB provides operations on a project database.
// The database is stored at ~/.orc/projects/<project-id>/orc.db.
type ProjectDB struct {
	*DB
	// projectDir is the project's working directory (e.g., /home/user/repos/myproject).
	// Used to locate git-tracked files like CONSTITUTION.md that live in <project>/.orc/.
	// Empty when opened via OpenProjectAtPath (tests) or OpenInMemory.
	projectDir string
}

// ProjectDir returns the project's working directory.
// Returns empty string if unknown (e.g., in-memory or test databases).
func (p *ProjectDB) ProjectDir() string {
	return p.projectDir
}

// OpenProject opens the project database for the project at projectPath.
// The database is resolved to ~/.orc/projects/<id>/orc.db via the project registry.
// If the project has an old-layout database at <project>/.orc/orc.db, it is auto-migrated.
// Falls back to the legacy path if the project is not yet registered (e.g., during first init).
func OpenProject(projectPath string) (*ProjectDB, error) {
	projectID, err := project.ResolveProjectID(projectPath)
	if err != nil {
		// Project not registered yet (first init, or test).
		// Fall back to legacy path for backwards compatibility.
		legacyPath := filepath.Join(projectPath, ".orc", "orc.db")
		pdb, legacyErr := OpenProjectAtPath(legacyPath)
		if legacyErr != nil {
			return nil, legacyErr
		}
		pdb.projectDir = projectPath
		return pdb, nil
	}

	// Auto-migrate from old layout if needed
	if migrated, err := project.MigrateIfNeeded(projectPath, projectID); err != nil {
		slog.Warn("failed to migrate project data, using new path anyway",
			"project_id", projectID,
			"error", err,
		)
	} else if migrated {
		slog.Info("migrated project data to ~/.orc/projects/",
			"project_id", projectID,
		)
	}

	dbPath, err := project.ProjectDBPath(projectID)
	if err != nil {
		return nil, fmt.Errorf("resolve project db path: %w", err)
	}

	pdb, err := OpenProjectAtPath(dbPath)
	if err != nil {
		return nil, err
	}
	pdb.projectDir = projectPath
	return pdb, nil
}

// OpenProjectByID opens the project database using only the project ID.
// The database is at ~/.orc/projects/<id>/orc.db.
func OpenProjectByID(projectID string) (*ProjectDB, error) {
	dbPath, err := project.ProjectDBPath(projectID)
	if err != nil {
		return nil, fmt.Errorf("resolve project db path: %w", err)
	}
	pdb, err := OpenProjectAtPath(dbPath)
	if err != nil {
		return nil, err
	}
	// Look up project path from registry for constitution file access.
	reg, regErr := project.LoadRegistry()
	if regErr == nil {
		for _, p := range reg.Projects {
			if p.ID == projectID {
				pdb.projectDir = p.Path
				break
			}
		}
	}
	return pdb, nil
}

// OpenProjectAtPath opens a project database at an explicit file path.
// Used by tests and when the caller has already resolved the path.
func OpenProjectAtPath(dbPath string) (*ProjectDB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db directory %s: %w", filepath.Dir(dbPath), err)
	}

	db, err := Open(dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate("project"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate project db: %w", err)
	}

	return &ProjectDB{DB: db}, nil
}

// OpenProjectWithDialect opens the project database with a specific dialect.
// For SQLite, dsn is the file path. For PostgreSQL, dsn is the connection string.
func OpenProjectWithDialect(dsn string, dialect driver.Dialect) (*ProjectDB, error) {
	db, err := OpenWithDialect(dsn, dialect)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate("project"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate project db: %w", err)
	}

	return &ProjectDB{DB: db}, nil
}

// OpenProjectInMemory opens an in-memory project database.
// This is much faster than file-based databases and ideal for testing.
func OpenProjectInMemory() (*ProjectDB, error) {
	db, err := OpenInMemory()
	if err != nil {
		return nil, err
	}

	if err := db.Migrate("project"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate project db: %w", err)
	}

	return &ProjectDB{DB: db}, nil
}

// RunInTx executes the given function within a database transaction.
// If fn returns an error, the transaction is rolled back.
// If fn returns nil, the transaction is committed.
// The context is propagated to all database operations within the transaction,
// enabling proper cancellation and timeout handling.
func (p *ProjectDB) RunInTx(ctx context.Context, fn func(tx *TxOps) error) error {
	tx, err := p.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txOps := &TxOps{
		tx:      tx,
		dialect: p.Dialect(),
		ctx:     ctx,
	}

	if err := fn(txOps); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// Ensure ProjectDB implements TxRunner
var _ TxRunner = (*ProjectDB)(nil)

// Detection stores project detection results.
type Detection struct {
	ID          int64
	Language    string
	Frameworks  []string
	BuildTools  []string
	HasTests    bool
	TestCommand string
	LintCommand string
	DetectedAt  time.Time
}

// StoreDetection saves detection results.
func (p *ProjectDB) StoreDetection(d *Detection) error {
	frameworks, err := json.Marshal(d.Frameworks)
	if err != nil {
		return fmt.Errorf("marshal frameworks: %w", err)
	}
	buildTools, err2 := json.Marshal(d.BuildTools)
	if err2 != nil {
		return fmt.Errorf("marshal build_tools: %w", err2)
	}

	hasTests := 0
	if d.HasTests {
		hasTests = 1
	}

	// Use dialect-aware upsert
	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `
			INSERT OR REPLACE INTO detection (id, language, frameworks, build_tools, has_tests, test_command, lint_command, detected_at)
			VALUES (1, ?, ?, ?, ?, ?, ?, datetime('now'))
		`
	} else {
		query = `
			INSERT INTO detection (id, language, frameworks, build_tools, has_tests, test_command, lint_command, detected_at)
			VALUES (1, $1, $2, $3, $4, $5, $6, NOW())
			ON CONFLICT (id) DO UPDATE SET
				language = EXCLUDED.language,
				frameworks = EXCLUDED.frameworks,
				build_tools = EXCLUDED.build_tools,
				has_tests = EXCLUDED.has_tests,
				test_command = EXCLUDED.test_command,
				lint_command = EXCLUDED.lint_command,
				detected_at = NOW()
		`
	}

	_, err = p.Exec(query, d.Language, string(frameworks), string(buildTools), hasTests, d.TestCommand, d.LintCommand)
	if err != nil {
		return fmt.Errorf("store detection: %w", err)
	}
	return nil
}

// LoadDetection retrieves the stored detection results.
func (p *ProjectDB) LoadDetection() (*Detection, error) {
	row := p.QueryRow(`
		SELECT id, language, frameworks, build_tools, has_tests, test_command, lint_command, detected_at
		FROM detection WHERE id = 1
	`)

	var d Detection
	var frameworks, buildTools, detectedAt string
	var hasTests int
	if err := row.Scan(&d.ID, &d.Language, &frameworks, &buildTools, &hasTests, &d.TestCommand, &d.LintCommand, &detectedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("load detection: %w", err)
	}

	d.HasTests = hasTests == 1
	if err := json.Unmarshal([]byte(frameworks), &d.Frameworks); err != nil {
		return nil, fmt.Errorf("unmarshal frameworks: %w", err)
	}
	if err := json.Unmarshal([]byte(buildTools), &d.BuildTools); err != nil {
		return nil, fmt.Errorf("unmarshal build_tools: %w", err)
	}
	if t, err := time.Parse("2006-01-02 15:04:05", detectedAt); err == nil {
		d.DetectedAt = t
	}

	return &d, nil
}
