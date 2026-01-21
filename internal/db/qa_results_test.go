package db

import (
	"testing"
	"time"
)

func TestSaveAndGetQAResult(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create a task first (foreign key constraint)
	task := &Task{
		ID:       "TASK-001",
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

	// Test saving QA result
	result := &QAResult{
		TaskID:  "TASK-001",
		Status:  "pass",
		Summary: "All tests pass with good coverage",
		TestsWritten: []QATest{
			{File: "api_test.go", Description: "E2E tests for API", Type: "e2e"},
			{File: "unit_test.go", Description: "Unit tests", Type: "unit"},
		},
		TestsRun: &QATestRun{
			Total:   42,
			Passed:  40,
			Failed:  0,
			Skipped: 2,
		},
		Coverage: &QACoverage{
			Percentage:     85.5,
			UncoveredAreas: "Error handling in edge cases",
		},
		Documentation: []QADoc{
			{File: "docs/api.md", Type: "api"},
		},
		Issues: []QAIssue{
			{Severity: "low", Description: "Consider adding more edge case tests"},
		},
		Recommendation: "Ready for production deployment",
	}

	err := db.SaveQAResult(result)
	if err != nil {
		t.Fatalf("SaveQAResult failed: %v", err)
	}

	// Test loading
	loaded, err := db.GetQAResult("TASK-001")
	if err != nil {
		t.Fatalf("GetQAResult failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("GetQAResult returned nil")
	}

	// Verify fields
	if loaded.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", loaded.TaskID, "TASK-001")
	}
	if loaded.Status != "pass" {
		t.Errorf("Status = %q, want %q", loaded.Status, "pass")
	}
	if loaded.Summary != "All tests pass with good coverage" {
		t.Errorf("Summary = %q", loaded.Summary)
	}
	if len(loaded.TestsWritten) != 2 {
		t.Errorf("TestsWritten count = %d, want 2", len(loaded.TestsWritten))
	}
	if loaded.TestsWritten[0].Type != "e2e" {
		t.Errorf("TestsWritten[0].Type = %q, want e2e", loaded.TestsWritten[0].Type)
	}
	if loaded.TestsRun == nil {
		t.Fatal("TestsRun is nil")
	}
	if loaded.TestsRun.Total != 42 {
		t.Errorf("TestsRun.Total = %d, want 42", loaded.TestsRun.Total)
	}
	if loaded.TestsRun.Passed != 40 {
		t.Errorf("TestsRun.Passed = %d, want 40", loaded.TestsRun.Passed)
	}
	if loaded.Coverage == nil {
		t.Fatal("Coverage is nil")
	}
	if loaded.Coverage.Percentage != 85.5 {
		t.Errorf("Coverage.Percentage = %f, want 85.5", loaded.Coverage.Percentage)
	}
	if len(loaded.Documentation) != 1 {
		t.Errorf("Documentation count = %d, want 1", len(loaded.Documentation))
	}
	if len(loaded.Issues) != 1 {
		t.Errorf("Issues count = %d, want 1", len(loaded.Issues))
	}
	if loaded.Recommendation != "Ready for production deployment" {
		t.Errorf("Recommendation = %q", loaded.Recommendation)
	}
	if loaded.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestQAResultUpsert(t *testing.T) {
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

	// Save initial result
	result := &QAResult{
		TaskID:         "TASK-002",
		Status:         "fail",
		Summary:        "Tests failing",
		Recommendation: "Fix tests",
	}
	if err := db.SaveQAResult(result); err != nil {
		t.Fatalf("SaveQAResult failed: %v", err)
	}

	// Update with new result (upsert)
	result.Status = "pass"
	result.Summary = "All tests now pass"
	result.Recommendation = "Ready to merge"
	if err := db.SaveQAResult(result); err != nil {
		t.Fatalf("SaveQAResult (upsert) failed: %v", err)
	}

	// Verify update
	loaded, err := db.GetQAResult("TASK-002")
	if err != nil {
		t.Fatalf("GetQAResult failed: %v", err)
	}
	if loaded.Status != "pass" {
		t.Errorf("Status = %q, want pass", loaded.Status)
	}
	if loaded.Summary != "All tests now pass" {
		t.Errorf("Summary = %q", loaded.Summary)
	}
}

func TestGetQAResultNotFound(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Get non-existent result
	result, err := db.GetQAResult("NONEXISTENT")
	if err != nil {
		t.Fatalf("GetQAResult should not error for missing data: %v", err)
	}
	if result != nil {
		t.Error("GetQAResult should return nil for missing data")
	}
}

func TestDeleteQAResult(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create task
	task := &Task{
		ID:       "TASK-003",
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

	// Save result
	result := &QAResult{
		TaskID:  "TASK-003",
		Status:  "pass",
		Summary: "Result to delete",
	}
	if err := db.SaveQAResult(result); err != nil {
		t.Fatalf("SaveQAResult failed: %v", err)
	}

	// Verify exists
	loaded, _ := db.GetQAResult("TASK-003")
	if loaded == nil {
		t.Fatal("Result should exist before delete")
	}

	// Delete
	if err := db.DeleteQAResult("TASK-003"); err != nil {
		t.Fatalf("DeleteQAResult failed: %v", err)
	}

	// Verify deleted
	loaded, _ = db.GetQAResult("TASK-003")
	if loaded != nil {
		t.Error("Result should be deleted")
	}
}

func TestQAResultNullFields(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create task
	task := &Task{
		ID:       "TASK-004",
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

	// Save minimal result (no tests_written, tests_run, coverage, documentation, issues)
	result := &QAResult{
		TaskID:         "TASK-004",
		Status:         "pass",
		Summary:        "No issues found",
		Recommendation: "Ship it",
	}
	if err := db.SaveQAResult(result); err != nil {
		t.Fatalf("SaveQAResult failed: %v", err)
	}

	// Load and verify null fields become empty slices
	loaded, err := db.GetQAResult("TASK-004")
	if err != nil {
		t.Fatalf("GetQAResult failed: %v", err)
	}
	if loaded.TestsWritten == nil {
		t.Error("TestsWritten should be empty slice, not nil")
	}
	if len(loaded.TestsWritten) != 0 {
		t.Errorf("TestsWritten should be empty, got %d", len(loaded.TestsWritten))
	}
	if loaded.TestsRun != nil {
		t.Error("TestsRun should be nil (optional)")
	}
	if loaded.Coverage != nil {
		t.Error("Coverage should be nil (optional)")
	}
	if loaded.Documentation == nil {
		t.Error("Documentation should be empty slice, not nil")
	}
	if loaded.Issues == nil {
		t.Error("Issues should be empty slice, not nil")
	}
}

func TestQAResultCascadeDelete(t *testing.T) {
	t.Parallel()
	db := setupProjectDB(t)

	// Create task
	task := &Task{
		ID:       "TASK-005",
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

	// Save result
	result := &QAResult{
		TaskID:  "TASK-005",
		Status:  "pass",
		Summary: "Test result",
	}
	if err := db.SaveQAResult(result); err != nil {
		t.Fatalf("SaveQAResult failed: %v", err)
	}

	// Delete task (should cascade to result)
	if err := db.DeleteTask("TASK-005"); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Verify result is deleted
	loaded, _ := db.GetQAResult("TASK-005")
	if loaded != nil {
		t.Error("Result should be cascade deleted with task")
	}
}

func TestQAResultCreatedAtTimestamp(t *testing.T) {
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

	before := time.Now().Add(-time.Second)

	// Save result
	result := &QAResult{
		TaskID:  "TASK-006",
		Status:  "pass",
		Summary: "Test result",
	}
	if err := db.SaveQAResult(result); err != nil {
		t.Fatalf("SaveQAResult failed: %v", err)
	}

	after := time.Now().Add(time.Second)

	// Load and verify timestamp
	loaded, err := db.GetQAResult("TASK-006")
	if err != nil {
		t.Fatalf("GetQAResult failed: %v", err)
	}

	if loaded.CreatedAt.Before(before) {
		t.Errorf("CreatedAt %v is before test start %v", loaded.CreatedAt, before)
	}
	if loaded.CreatedAt.After(after) {
		t.Errorf("CreatedAt %v is after test end %v", loaded.CreatedAt, after)
	}
}
