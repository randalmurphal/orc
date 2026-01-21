package db

import (
	"path/filepath"
	"testing"
	"time"
)

// setupProjectDB creates an in-memory project database for testing.
func setupProjectDB(t *testing.T) *ProjectDB {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	return &ProjectDB{DB: db}
}

func TestSaveAndGetReviewFindings(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create a task first (foreign key constraint)
	task := &Task{
		ID:          "TASK-001",
		Title:       "Test task",
		Description: "Test description",
		Status:      "running",
		Weight:      "medium",
		Queue:       "active",
		Priority:    "normal",
		Category:    "feature",
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Test saving review findings
	findings := &ReviewFindings{
		TaskID:  "TASK-001",
		Round:   1,
		Summary: "Found 3 issues in the implementation",
		Issues: []ReviewFinding{
			{Severity: "high", File: "main.go", Line: 42, Description: "SQL injection vulnerability"},
			{Severity: "medium", File: "utils.go", Line: 100, Description: "Error not handled"},
			{Severity: "low", File: "config.go", Line: 5, Description: "Magic number"},
		},
		Questions: []string{"Why is this implemented synchronously?"},
		Positives: []string{"Good test coverage"},
		Perspective: "security",
	}

	err := db.SaveReviewFindings(findings)
	if err != nil {
		t.Fatalf("SaveReviewFindings failed: %v", err)
	}

	// Test loading
	loaded, err := db.GetReviewFindings("TASK-001", 1)
	if err != nil {
		t.Fatalf("GetReviewFindings failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("GetReviewFindings returned nil")
	}

	// Verify fields
	if loaded.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", loaded.TaskID, "TASK-001")
	}
	if loaded.Round != 1 {
		t.Errorf("Round = %d, want 1", loaded.Round)
	}
	if loaded.Summary != "Found 3 issues in the implementation" {
		t.Errorf("Summary = %q, want %q", loaded.Summary, "Found 3 issues in the implementation")
	}
	if len(loaded.Issues) != 3 {
		t.Errorf("Issues count = %d, want 3", len(loaded.Issues))
	}
	if loaded.Issues[0].Severity != "high" {
		t.Errorf("Issues[0].Severity = %q, want %q", loaded.Issues[0].Severity, "high")
	}
	if loaded.Issues[0].File != "main.go" {
		t.Errorf("Issues[0].File = %q, want %q", loaded.Issues[0].File, "main.go")
	}
	if len(loaded.Questions) != 1 {
		t.Errorf("Questions count = %d, want 1", len(loaded.Questions))
	}
	if len(loaded.Positives) != 1 {
		t.Errorf("Positives count = %d, want 1", len(loaded.Positives))
	}
	if loaded.Perspective != "security" {
		t.Errorf("Perspective = %q, want %q", loaded.Perspective, "security")
	}
	if loaded.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestReviewFindingsUpsert(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create task
	task := &Task{
		ID:       "TASK-002",
		Title:    "Test task",
		Status:   "running",
		Weight:   "medium",
		Queue:    "active",
		Priority: "normal",
		Category: "feature",
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Save initial findings
	findings := &ReviewFindings{
		TaskID:  "TASK-002",
		Round:   1,
		Summary: "Initial summary",
		Issues:  []ReviewFinding{{Severity: "low", Description: "Minor issue"}},
	}
	if err := db.SaveReviewFindings(findings); err != nil {
		t.Fatalf("SaveReviewFindings failed: %v", err)
	}

	// Update with new findings (upsert)
	findings.Summary = "Updated summary"
	findings.Issues = []ReviewFinding{
		{Severity: "high", Description: "Critical issue"},
		{Severity: "medium", Description: "Another issue"},
	}
	if err := db.SaveReviewFindings(findings); err != nil {
		t.Fatalf("SaveReviewFindings (upsert) failed: %v", err)
	}

	// Verify update
	loaded, err := db.GetReviewFindings("TASK-002", 1)
	if err != nil {
		t.Fatalf("GetReviewFindings failed: %v", err)
	}
	if loaded.Summary != "Updated summary" {
		t.Errorf("Summary = %q, want %q", loaded.Summary, "Updated summary")
	}
	if len(loaded.Issues) != 2 {
		t.Errorf("Issues count = %d, want 2", len(loaded.Issues))
	}
}

func TestGetAllReviewFindings(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create task
	task := &Task{
		ID:       "TASK-003",
		Title:    "Test task",
		Status:   "running",
		Weight:   "large",
		Queue:    "active",
		Priority: "normal",
		Category: "feature",
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Save findings for multiple rounds
	round1 := &ReviewFindings{
		TaskID:  "TASK-003",
		Round:   1,
		Summary: "Round 1 findings",
		Issues:  []ReviewFinding{{Severity: "high", Description: "Issue 1"}},
	}
	round2 := &ReviewFindings{
		TaskID:  "TASK-003",
		Round:   2,
		Summary: "Round 2 findings",
		Issues:  []ReviewFinding{{Severity: "low", Description: "Issue 2"}},
	}

	if err := db.SaveReviewFindings(round1); err != nil {
		t.Fatalf("SaveReviewFindings round1 failed: %v", err)
	}
	if err := db.SaveReviewFindings(round2); err != nil {
		t.Fatalf("SaveReviewFindings round2 failed: %v", err)
	}

	// Get all
	all, err := db.GetAllReviewFindings("TASK-003")
	if err != nil {
		t.Fatalf("GetAllReviewFindings failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("GetAllReviewFindings returned %d, want 2", len(all))
	}

	// Verify ordering (should be by round ascending)
	if all[0].Round != 1 {
		t.Errorf("First result Round = %d, want 1", all[0].Round)
	}
	if all[1].Round != 2 {
		t.Errorf("Second result Round = %d, want 2", all[1].Round)
	}
}

func TestGetReviewFindingsNotFound(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Get non-existent findings
	findings, err := db.GetReviewFindings("NONEXISTENT", 1)
	if err != nil {
		t.Fatalf("GetReviewFindings should not error for missing data: %v", err)
	}
	if findings != nil {
		t.Error("GetReviewFindings should return nil for missing data")
	}
}

func TestDeleteReviewFindings(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create task
	task := &Task{
		ID:       "TASK-004",
		Title:    "Test task",
		Status:   "running",
		Weight:   "medium",
		Queue:    "active",
		Priority: "normal",
		Category: "feature",
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Save findings
	findings := &ReviewFindings{
		TaskID:  "TASK-004",
		Round:   1,
		Summary: "Findings to delete",
	}
	if err := db.SaveReviewFindings(findings); err != nil {
		t.Fatalf("SaveReviewFindings failed: %v", err)
	}

	// Verify exists
	loaded, _ := db.GetReviewFindings("TASK-004", 1)
	if loaded == nil {
		t.Fatal("Findings should exist before delete")
	}

	// Delete (deletes all findings for task)
	if err := db.DeleteReviewFindings("TASK-004"); err != nil {
		t.Fatalf("DeleteReviewFindings failed: %v", err)
	}

	// Verify deleted
	loaded, _ = db.GetReviewFindings("TASK-004", 1)
	if loaded != nil {
		t.Error("Findings should be deleted")
	}
}

func TestReviewFindingsNullFields(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create task
	task := &Task{
		ID:       "TASK-005",
		Title:    "Test task",
		Status:   "running",
		Weight:   "small",
		Queue:    "active",
		Priority: "normal",
		Category: "feature",
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Save minimal findings (no issues, questions, positives)
	findings := &ReviewFindings{
		TaskID:  "TASK-005",
		Round:   1,
		Summary: "No issues found",
	}
	if err := db.SaveReviewFindings(findings); err != nil {
		t.Fatalf("SaveReviewFindings failed: %v", err)
	}

	// Load and verify null fields become empty slices
	loaded, err := db.GetReviewFindings("TASK-005", 1)
	if err != nil {
		t.Fatalf("GetReviewFindings failed: %v", err)
	}
	if loaded.Issues == nil {
		t.Error("Issues should be empty slice, not nil")
	}
	if len(loaded.Issues) != 0 {
		t.Errorf("Issues should be empty, got %d", len(loaded.Issues))
	}
	if loaded.Questions == nil {
		t.Error("Questions should be empty slice, not nil")
	}
	if loaded.Positives == nil {
		t.Error("Positives should be empty slice, not nil")
	}
}

func TestReviewFindingsCascadeDelete(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create task
	task := &Task{
		ID:       "TASK-006",
		Title:    "Test task",
		Status:   "running",
		Weight:   "medium",
		Queue:    "active",
		Priority: "normal",
		Category: "feature",
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Save findings
	findings := &ReviewFindings{
		TaskID:  "TASK-006",
		Round:   1,
		Summary: "Test findings",
	}
	if err := db.SaveReviewFindings(findings); err != nil {
		t.Fatalf("SaveReviewFindings failed: %v", err)
	}

	// Delete task (should cascade to findings)
	if err := db.DeleteTask("TASK-006"); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Verify findings are deleted
	loaded, _ := db.GetReviewFindings("TASK-006", 1)
	if loaded != nil {
		t.Error("Findings should be cascade deleted with task")
	}
}

func TestReviewFindingsCreatedAtTimestamp(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create task
	task := &Task{
		ID:       "TASK-007",
		Title:    "Test task",
		Status:   "running",
		Weight:   "medium",
		Queue:    "active",
		Priority: "normal",
		Category: "feature",
	}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	before := time.Now().Add(-time.Second)

	// Save findings
	findings := &ReviewFindings{
		TaskID:  "TASK-007",
		Round:   1,
		Summary: "Test findings",
	}
	if err := db.SaveReviewFindings(findings); err != nil {
		t.Fatalf("SaveReviewFindings failed: %v", err)
	}

	after := time.Now().Add(time.Second)

	// Load and verify timestamp
	loaded, err := db.GetReviewFindings("TASK-007", 1)
	if err != nil {
		t.Fatalf("GetReviewFindings failed: %v", err)
	}

	if loaded.CreatedAt.Before(before) {
		t.Errorf("CreatedAt %v is before test start %v", loaded.CreatedAt, before)
	}
	if loaded.CreatedAt.After(after) {
		t.Errorf("CreatedAt %v is after test end %v", loaded.CreatedAt, after)
	}
}
