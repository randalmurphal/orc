package db

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestProjectWorkflowColumns_RemovesDeadFields(t *testing.T) {
	t.Parallel()

	pdb, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	assertWorkflowColumnRemoved(t, pdb.DB, "workflow_type")
	assertWorkflowColumnRemoved(t, pdb.DB, "default_max_iterations")
}

func TestGlobalWorkflowColumns_RemovesDeadFields(t *testing.T) {
	t.Parallel()

	gdb, err := OpenGlobalAt(filepath.Join(t.TempDir(), "orc.db"))
	if err != nil {
		t.Fatalf("open global db: %v", err)
	}
	defer func() { _ = gdb.Close() }()

	assertWorkflowColumnRemoved(t, gdb.DB, "workflow_type")
	assertWorkflowColumnRemoved(t, gdb.DB, "default_max_iterations")
}

func TestProjectWorkflowColumns_MigrationPreservesWorkflowData(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "project.db")
	rawDB := openLegacyWorkflowDB(t, dbPath, "project", 72)
	defer func() { _ = rawDB.Close() }()

	insertLegacyWorkflowRow(t, rawDB)

	if err := rawDB.Migrate("project"); err != nil {
		t.Fatalf("migrate project db: %v", err)
	}

	assertWorkflowColumnRemoved(t, rawDB, "workflow_type")
	assertWorkflowColumnRemoved(t, rawDB, "default_max_iterations")
	assertWorkflowRowPreserved(t, rawDB)
}

func TestGlobalWorkflowColumns_MigrationPreservesWorkflowData(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "global.db")
	rawDB := openLegacyWorkflowDB(t, dbPath, "global", 12)
	defer func() { _ = rawDB.Close() }()

	insertLegacyWorkflowRow(t, rawDB)

	if err := rawDB.Migrate("global"); err != nil {
		t.Fatalf("migrate global db: %v", err)
	}

	assertWorkflowColumnRemoved(t, rawDB, "workflow_type")
	assertWorkflowColumnRemoved(t, rawDB, "default_max_iterations")
	assertWorkflowRowPreserved(t, rawDB)
}

func assertWorkflowColumnRemoved(t *testing.T, db *DB, column string) {
	t.Helper()

	hasColumn, err := db.tableHasColumn(context.Background(), "workflows", column)
	if err != nil {
		t.Fatalf("check workflows.%s: %v", column, err)
	}
	if hasColumn {
		t.Fatalf("workflows.%s should be removed", column)
	}
}

func openLegacyWorkflowDB(t *testing.T, dbPath, schemaType string, latestApplied int) *DB {
	t.Helper()

	rawDB, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}

	if _, err := rawDB.Exec(`
		CREATE TABLE _migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT DEFAULT (datetime('now'))
		)
	`); err != nil {
		t.Fatalf("create _migrations: %v", err)
	}

	for version := 1; version <= latestApplied; version++ {
		if _, err := rawDB.Exec(`INSERT INTO _migrations (version) VALUES (?)`, version); err != nil {
			t.Fatalf("seed _migrations version %d for %s: %v", version, schemaType, err)
		}
	}

	if _, err := rawDB.Exec(`
		CREATE TABLE workflows (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			default_model TEXT,
			workflow_type TEXT DEFAULT 'task',
			default_max_iterations INTEGER DEFAULT 0,
			default_thinking BOOLEAN DEFAULT FALSE,
			is_builtin BOOLEAN DEFAULT FALSE,
			based_on TEXT,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now')),
			triggers TEXT,
			completion_action TEXT DEFAULT '',
			target_branch TEXT DEFAULT '',
			default_provider TEXT DEFAULT '',
			FOREIGN KEY (based_on) REFERENCES workflows(id) ON DELETE SET NULL
		)
	`); err != nil {
		t.Fatalf("create legacy workflows table: %v", err)
	}

	if _, err := rawDB.Exec(`CREATE INDEX idx_workflows_type ON workflows(workflow_type)`); err != nil {
		t.Fatalf("create legacy workflows index: %v", err)
	}

	return rawDB
}

func insertLegacyWorkflowRow(t *testing.T, db *DB) {
	t.Helper()

	_, err := db.Exec(`
		INSERT INTO workflows (
			id,
			name,
			description,
			default_model,
			workflow_type,
			default_max_iterations,
			default_thinking,
			is_builtin,
			based_on,
			created_at,
			updated_at,
			triggers,
			completion_action,
			target_branch,
			default_provider
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"wf-cleanup",
		"Cleanup Workflow",
		"legacy row",
		"sonnet",
		"task",
		7,
		true,
		false,
		nil,
		"2026-03-30T00:00:00Z",
		"2026-03-30T00:05:00Z",
		`[{"event":"on_task_completed","agent_id":"agent-1"}]`,
		"commit",
		"release/main",
		"codex",
	)
	if err != nil {
		t.Fatalf("insert legacy workflow row: %v", err)
	}
}

func assertWorkflowRowPreserved(t *testing.T, db *DB) {
	t.Helper()

	row := db.QueryRow(`
		SELECT
			id,
			name,
			description,
			default_model,
			default_thinking,
			is_builtin,
			created_at,
			updated_at,
			triggers,
			completion_action,
			target_branch,
			default_provider
		FROM workflows
		WHERE id = ?
	`, "wf-cleanup")

	var (
		id               string
		name             string
		description      string
		defaultModel     string
		defaultThinking  bool
		isBuiltin        bool
		createdAt        string
		updatedAt        string
		triggers         string
		completionAction string
		targetBranch     string
		defaultProvider  string
	)
	if err := row.Scan(
		&id,
		&name,
		&description,
		&defaultModel,
		&defaultThinking,
		&isBuiltin,
		&createdAt,
		&updatedAt,
		&triggers,
		&completionAction,
		&targetBranch,
		&defaultProvider,
	); err != nil {
		t.Fatalf("scan migrated workflow row: %v", err)
	}

	got := fmt.Sprintf("%s|%s|%s|%s|%t|%t|%s|%s|%s|%s|%s|%s",
		id,
		name,
		description,
		defaultModel,
		defaultThinking,
		isBuiltin,
		createdAt,
		updatedAt,
		triggers,
		completionAction,
		targetBranch,
		defaultProvider,
	)
	want := "wf-cleanup|Cleanup Workflow|legacy row|sonnet|true|false|2026-03-30T00:00:00Z|2026-03-30T00:05:00Z|[{\"event\":\"on_task_completed\",\"agent_id\":\"agent-1\"}]|commit|release/main|codex"
	if got != want {
		t.Fatalf("migrated workflow row mismatch:\n got: %s\nwant: %s", got, want)
	}
}
