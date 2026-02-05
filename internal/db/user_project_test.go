package db

import (
	"path/filepath"
	"testing"
	"time"
)

// =============================================================================
// SC-2: User columns added to tasks table (created_by, assigned_to)
// =============================================================================

func TestMigration057_AddsUserColumnsToTasks(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "project.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Migrate project schema (should include 057)
	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	// Verify created_by and assigned_to columns exist in tasks
	expectedColumns := []string{"created_by", "assigned_to"}
	for _, col := range expectedColumns {
		var colCount int
		err = db.QueryRow(`
			SELECT COUNT(*) FROM pragma_table_info('tasks')
			WHERE name = ?
		`, col).Scan(&colCount)
		if err != nil {
			t.Fatalf("check column %s: %v", col, err)
		}
		if colCount != 1 {
			t.Errorf("tasks table missing column: %s", col)
		}
	}
}

func TestProjectDB_SaveTask_WithUserColumns(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "project.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Save task with user columns
	task := &Task{
		ID:          "TASK-001",
		Title:       "Test task",
		Description: "Test description",
		Weight:      "medium",
		Status:      "created",
		CreatedBy:   "user-001",
		AssignedTo:  "user-002",
	}

	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Retrieve and verify
	got, err := pdb.GetTask("TASK-001")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.CreatedBy != "user-001" {
		t.Errorf("CreatedBy = %q, want user-001", got.CreatedBy)
	}
	if got.AssignedTo != "user-002" {
		t.Errorf("AssignedTo = %q, want user-002", got.AssignedTo)
	}
}

// =============================================================================
// SC-3: User columns added to initiatives table (created_by, owned_by)
// =============================================================================

func TestMigration057_AddsUserColumnsToInitiatives(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "project.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	// Verify created_by and owned_by columns exist in initiatives
	expectedColumns := []string{"created_by", "owned_by"}
	for _, col := range expectedColumns {
		var colCount int
		err = db.QueryRow(`
			SELECT COUNT(*) FROM pragma_table_info('initiatives')
			WHERE name = ?
		`, col).Scan(&colCount)
		if err != nil {
			t.Fatalf("check column %s: %v", col, err)
		}
		if colCount != 1 {
			t.Errorf("initiatives table missing column: %s", col)
		}
	}
}

func TestProjectDB_SaveInitiative_WithUserColumns(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "project.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Save initiative with user columns
	init := &Initiative{
		ID:        "INIT-001",
		Title:     "Test initiative",
		Status:    "active",
		CreatedBy: "user-001",
		OwnedBy:   "user-003",
	}

	if err := pdb.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative failed: %v", err)
	}

	// Retrieve and verify
	got, err := pdb.GetInitiative("INIT-001")
	if err != nil {
		t.Fatalf("GetInitiative failed: %v", err)
	}
	if got.CreatedBy != "user-001" {
		t.Errorf("CreatedBy = %q, want user-001", got.CreatedBy)
	}
	if got.OwnedBy != "user-003" {
		t.Errorf("OwnedBy = %q, want user-003", got.OwnedBy)
	}
}

// =============================================================================
// SC-4: User columns added to phases table (executed_by)
// =============================================================================

func TestMigration057_AddsUserColumnsToPhases(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "project.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	// Verify executed_by column exists in phases
	var colCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('phases')
		WHERE name = 'executed_by'
	`).Scan(&colCount)
	if err != nil {
		t.Fatalf("check column: %v", err)
	}
	if colCount != 1 {
		t.Errorf("phases table missing column: executed_by")
	}
}

func TestProjectDB_SavePhase_WithExecutedBy(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "project.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// First create a task (phase references task)
	task := &Task{
		ID:        "TASK-001",
		Title:     "Test task",
		Weight:    "medium",
		Status:    "created",
		CreatedAt: time.Now(),
	}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Save phase with executed_by
	phase := &Phase{
		TaskID:     "TASK-001",
		PhaseID:    "implement",
		Status:     "running",
		ExecutedBy: "user-004",
	}

	if err := pdb.SavePhase(phase); err != nil {
		t.Fatalf("SavePhase failed: %v", err)
	}

	// Retrieve and verify
	phases, err := pdb.GetPhases("TASK-001")
	if err != nil {
		t.Fatalf("GetPhases failed: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("len(phases) = %d, want 1", len(phases))
	}
	if phases[0].ExecutedBy != "user-004" {
		t.Errorf("ExecutedBy = %q, want user-004", phases[0].ExecutedBy)
	}
}

// =============================================================================
// SC-5: User columns added to workflow_runs table (started_by)
// =============================================================================

func TestMigration057_AddsUserColumnsToWorkflowRuns(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "project.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	// Verify started_by column exists in workflow_runs
	var colCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('workflow_runs')
		WHERE name = 'started_by'
	`).Scan(&colCount)
	if err != nil {
		t.Fatalf("check column: %v", err)
	}
	if colCount != 1 {
		t.Errorf("workflow_runs table missing column: started_by")
	}
}

func TestProjectDB_SaveWorkflowRun_WithStartedBy(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "project.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// First create a workflow (workflow_run references workflow)
	workflow := &Workflow{
		ID:   "implement-medium",
		Name: "Implement Medium",
	}
	if err := pdb.SaveWorkflow(workflow); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Save workflow run with started_by
	run := &WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "implement-medium",
		ContextType: "task",
		ContextData: `{"task_id": "TASK-001"}`,
		Prompt:      "Test prompt",
		Status:      "running",
		StartedBy:   "user-005",
	}

	if err := pdb.SaveWorkflowRun(run); err != nil {
		t.Fatalf("SaveWorkflowRun failed: %v", err)
	}

	// Retrieve and verify
	got, err := pdb.GetWorkflowRun("RUN-001")
	if err != nil {
		t.Fatalf("GetWorkflowRun failed: %v", err)
	}
	if got.StartedBy != "user-005" {
		t.Errorf("StartedBy = %q, want user-005", got.StartedBy)
	}
}
