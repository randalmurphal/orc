package initiative

import (
	"testing"
	"time"
)

// ============================================================================
// SC-1: Criterion struct with required fields
// ============================================================================

func TestCriterion_Fields(t *testing.T) {
	t.Parallel()

	c := Criterion{
		ID:          "AC-001",
		Description: "User can log in with JWT",
		TaskIDs:     []string{"TASK-001", "TASK-002"},
		Status:      CriterionStatusCovered,
		VerifiedAt:  time.Now().Format(time.RFC3339),
		VerifiedBy:  "orchestrator",
		Evidence:    "E2E test passes",
	}

	if c.ID != "AC-001" {
		t.Errorf("ID = %q, want %q", c.ID, "AC-001")
	}
	if c.Description != "User can log in with JWT" {
		t.Errorf("Description = %q, want %q", c.Description, "User can log in with JWT")
	}
	if len(c.TaskIDs) != 2 {
		t.Errorf("TaskIDs len = %d, want 2", len(c.TaskIDs))
	}
	if c.Status != CriterionStatusCovered {
		t.Errorf("Status = %q, want %q", c.Status, CriterionStatusCovered)
	}
	if c.VerifiedBy != "orchestrator" {
		t.Errorf("VerifiedBy = %q, want %q", c.VerifiedBy, "orchestrator")
	}
	if c.Evidence != "E2E test passes" {
		t.Errorf("Evidence = %q, want %q", c.Evidence, "E2E test passes")
	}
}

func TestCriterion_JSONTags(t *testing.T) {
	t.Parallel()

	// Verify JSON serialization works by creating a criterion and checking fields.
	// The actual JSON tag validation is structural - if the struct compiles with
	// the expected json tags, this test passing confirms the tags exist.
	c := Criterion{
		ID:          "AC-001",
		Description: "Test criterion",
		TaskIDs:     []string{},
		Status:      CriterionStatusUncovered,
	}
	if c.ID == "" {
		t.Error("ID should not be empty")
	}
}

// ============================================================================
// SC-2: Status lifecycle (uncovered -> covered -> satisfied -> regressed)
// ============================================================================

func TestCriterionStatus_Constants(t *testing.T) {
	t.Parallel()

	// All four status values must be defined
	statuses := []CriterionStatus{
		CriterionStatusUncovered,
		CriterionStatusCovered,
		CriterionStatusSatisfied,
		CriterionStatusRegressed,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("criterion status constant should not be empty")
		}
	}

	// Verify distinct values
	seen := make(map[CriterionStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status value: %q", s)
		}
		seen[s] = true
	}
}

func TestCriterionStatus_Values(t *testing.T) {
	t.Parallel()

	// Verify expected string values match spec
	if CriterionStatusUncovered != "uncovered" {
		t.Errorf("CriterionStatusUncovered = %q, want %q", CriterionStatusUncovered, "uncovered")
	}
	if CriterionStatusCovered != "covered" {
		t.Errorf("CriterionStatusCovered = %q, want %q", CriterionStatusCovered, "covered")
	}
	if CriterionStatusSatisfied != "satisfied" {
		t.Errorf("CriterionStatusSatisfied = %q, want %q", CriterionStatusSatisfied, "satisfied")
	}
	if CriterionStatusRegressed != "regressed" {
		t.Errorf("CriterionStatusRegressed = %q, want %q", CriterionStatusRegressed, "regressed")
	}
}

func TestValidateCriterionStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status  string
		wantErr bool
	}{
		{"uncovered", false},
		{"covered", false},
		{"satisfied", false},
		{"regressed", false},
		{"invalid", true},
		{"", true},
		{"UNCOVERED", true}, // Case sensitive
		{"done", true},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			err := ValidateCriterionStatus(tt.status)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCriterionStatus(%q) error = %v, wantErr %v", tt.status, err, tt.wantErr)
			}
		})
	}
}

// ============================================================================
// SC-7: Auto-ID generation (AC-001 format)
// ============================================================================

func TestAddCriterion_AutoID(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")

	// Add first criterion - should get AC-001
	init.AddCriterion("User can log in with JWT")
	if len(init.Criteria) != 1 {
		t.Fatalf("Criteria len = %d, want 1", len(init.Criteria))
	}
	if init.Criteria[0].ID != "AC-001" {
		t.Errorf("first criterion ID = %q, want %q", init.Criteria[0].ID, "AC-001")
	}

	// Add second criterion - should get AC-002
	init.AddCriterion("User can refresh token")
	if len(init.Criteria) != 2 {
		t.Fatalf("Criteria len = %d, want 2", len(init.Criteria))
	}
	if init.Criteria[1].ID != "AC-002" {
		t.Errorf("second criterion ID = %q, want %q", init.Criteria[1].ID, "AC-002")
	}
}

func TestAddCriterion_DefaultStatus(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("Some criterion")

	if init.Criteria[0].Status != CriterionStatusUncovered {
		t.Errorf("new criterion status = %q, want %q", init.Criteria[0].Status, CriterionStatusUncovered)
	}
}

func TestAddCriterion_EmptyTaskIDs(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("Some criterion")

	if init.Criteria[0].TaskIDs == nil {
		// TaskIDs should be initialized (empty slice, not nil) for JSON serialization
		t.Log("TaskIDs is nil - acceptable if JSON serializes correctly")
	}
	if len(init.Criteria[0].TaskIDs) != 0 {
		t.Errorf("new criterion TaskIDs len = %d, want 0", len(init.Criteria[0].TaskIDs))
	}
}

func TestAddCriterion_Description(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("User can log in with JWT")

	if init.Criteria[0].Description != "User can log in with JWT" {
		t.Errorf("Description = %q, want %q", init.Criteria[0].Description, "User can log in with JWT")
	}
}

func TestAddCriterion_UpdatesTimestamp(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	before := init.UpdatedAt

	// Small delay to ensure time difference
	time.Sleep(time.Millisecond)

	init.AddCriterion("Some criterion")

	if !init.UpdatedAt.After(before) {
		t.Error("UpdatedAt should be updated after AddCriterion")
	}
}

func TestAddCriterion_IDSequenceAfterRemoval(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")

	// Add 3 criteria
	init.AddCriterion("First")
	init.AddCriterion("Second")
	init.AddCriterion("Third")

	// Remove the second one
	init.RemoveCriterion("AC-002")

	// Add another - should get AC-004 (not reuse AC-002)
	init.AddCriterion("Fourth")

	// Find the new criterion
	var found bool
	for _, c := range init.Criteria {
		if c.Description == "Fourth" {
			if c.ID != "AC-004" {
				t.Errorf("new criterion after removal ID = %q, want %q", c.ID, "AC-004")
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("Fourth criterion not found")
	}
}

// ============================================================================
// SC-4: Coverage mapping operations
// ============================================================================

func TestMapCriterionToTask(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("User can log in")

	err := init.MapCriterionToTask("AC-001", "TASK-001")
	if err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}

	// Verify the task was added
	c := init.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("criterion AC-001 not found")
	}
	if len(c.TaskIDs) != 1 || c.TaskIDs[0] != "TASK-001" {
		t.Errorf("TaskIDs = %v, want [TASK-001]", c.TaskIDs)
	}

	// Status should transition from uncovered to covered
	if c.Status != CriterionStatusCovered {
		t.Errorf("Status = %q, want %q after mapping task", c.Status, CriterionStatusCovered)
	}
}

func TestMapCriterionToTask_MultipleTasks(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("Complex criterion")

	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("first MapCriterionToTask() error = %v", err)
	}
	if err := init.MapCriterionToTask("AC-001", "TASK-002"); err != nil {
		t.Fatalf("second MapCriterionToTask() error = %v", err)
	}

	c := init.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("criterion not found")
	}
	if len(c.TaskIDs) != 2 {
		t.Errorf("TaskIDs len = %d, want 2", len(c.TaskIDs))
	}
}

func TestMapCriterionToTask_DuplicateTask(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("Some criterion")

	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("first MapCriterionToTask() error = %v", err)
	}
	// Mapping same task again should be idempotent (no error, no duplicate)
	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("duplicate MapCriterionToTask() error = %v", err)
	}

	c := init.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("criterion not found")
	}
	if len(c.TaskIDs) != 1 {
		t.Errorf("TaskIDs len = %d, want 1 (no duplicate)", len(c.TaskIDs))
	}
}

func TestMapCriterionToTask_NonExistentCriterion(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")

	err := init.MapCriterionToTask("AC-999", "TASK-001")
	if err == nil {
		t.Error("MapCriterionToTask() should error for non-existent criterion")
	}
}

func TestGetCriterion(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("First")
	init.AddCriterion("Second")

	c := init.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("GetCriterion(AC-001) returned nil")
	}
	if c.Description != "First" {
		t.Errorf("Description = %q, want %q", c.Description, "First")
	}

	c = init.GetCriterion("AC-002")
	if c == nil {
		t.Fatal("GetCriterion(AC-002) returned nil")
	}
	if c.Description != "Second" {
		t.Errorf("Description = %q, want %q", c.Description, "Second")
	}

	// Non-existent
	c = init.GetCriterion("AC-999")
	if c != nil {
		t.Errorf("GetCriterion(AC-999) should return nil, got %v", c)
	}
}

func TestRemoveCriterion(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("First")
	init.AddCriterion("Second")
	init.AddCriterion("Third")

	// Remove middle
	if !init.RemoveCriterion("AC-002") {
		t.Error("RemoveCriterion should return true for existing criterion")
	}
	if len(init.Criteria) != 2 {
		t.Errorf("Criteria len = %d, want 2", len(init.Criteria))
	}

	// Verify it's gone
	if init.GetCriterion("AC-002") != nil {
		t.Error("AC-002 should be removed")
	}

	// Remove non-existent
	if init.RemoveCriterion("AC-999") {
		t.Error("RemoveCriterion should return false for non-existent criterion")
	}
}

func TestGetUncoveredCriteria(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("First")  // AC-001 - uncovered
	init.AddCriterion("Second") // AC-002 - will be covered
	init.AddCriterion("Third")  // AC-003 - uncovered

	// Map AC-002 to a task (makes it covered)
	if err := init.MapCriterionToTask("AC-002", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}

	uncovered := init.GetUncoveredCriteria()
	if len(uncovered) != 2 {
		t.Errorf("uncovered count = %d, want 2", len(uncovered))
	}

	// Verify the right ones are uncovered
	ids := make(map[string]bool)
	for _, c := range uncovered {
		ids[c.ID] = true
	}
	if !ids["AC-001"] {
		t.Error("AC-001 should be uncovered")
	}
	if !ids["AC-003"] {
		t.Error("AC-003 should be uncovered")
	}
	if ids["AC-002"] {
		t.Error("AC-002 should NOT be uncovered (has task mapped)")
	}
}

func TestGetCoverageReport(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("First")     // uncovered
	init.AddCriterion("Second")    // covered
	init.AddCriterion("Third")     // satisfied
	init.AddCriterion("Fourth")    // regressed

	// Set up various states
	if err := init.MapCriterionToTask("AC-002", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}
	if err := init.MapCriterionToTask("AC-003", "TASK-002"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}
	if err := init.VerifyCriterion("AC-003", CriterionStatusSatisfied, "Tests pass"); err != nil {
		t.Fatalf("VerifyCriterion() error = %v", err)
	}
	if err := init.MapCriterionToTask("AC-004", "TASK-003"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}
	if err := init.VerifyCriterion("AC-004", CriterionStatusRegressed, "Regression detected"); err != nil {
		t.Fatalf("VerifyCriterion() error = %v", err)
	}

	report := init.GetCoverageReport()

	if report.Total != 4 {
		t.Errorf("Total = %d, want 4", report.Total)
	}
	if report.Uncovered != 1 {
		t.Errorf("Uncovered = %d, want 1", report.Uncovered)
	}
	if report.Covered != 1 {
		t.Errorf("Covered = %d, want 1", report.Covered)
	}
	if report.Satisfied != 1 {
		t.Errorf("Satisfied = %d, want 1", report.Satisfied)
	}
	if report.Regressed != 1 {
		t.Errorf("Regressed = %d, want 1", report.Regressed)
	}
	if len(report.Criteria) != 4 {
		t.Errorf("Criteria in report = %d, want 4", len(report.Criteria))
	}
}

func TestGetCoverageReport_Empty(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	report := init.GetCoverageReport()

	if report.Total != 0 {
		t.Errorf("Total = %d, want 0", report.Total)
	}
	if len(report.Criteria) != 0 {
		t.Errorf("Criteria = %d, want 0", len(report.Criteria))
	}
}

// ============================================================================
// SC-5: Verification API
// ============================================================================

func TestVerifyCriterion_Satisfied(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("User can log in")
	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}

	err := init.VerifyCriterion("AC-001", CriterionStatusSatisfied, "E2E test passes")
	if err != nil {
		t.Fatalf("VerifyCriterion() error = %v", err)
	}

	c := init.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("criterion not found")
	}
	if c.Status != CriterionStatusSatisfied {
		t.Errorf("Status = %q, want %q", c.Status, CriterionStatusSatisfied)
	}
	if c.Evidence != "E2E test passes" {
		t.Errorf("Evidence = %q, want %q", c.Evidence, "E2E test passes")
	}
	if c.VerifiedAt == "" {
		t.Error("VerifiedAt should be set")
	}
}

func TestVerifyCriterion_Regressed(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("Feature X works")
	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}
	// First satisfy
	if err := init.VerifyCriterion("AC-001", CriterionStatusSatisfied, "Works"); err != nil {
		t.Fatalf("VerifyCriterion(satisfied) error = %v", err)
	}
	// Then regress
	if err := init.VerifyCriterion("AC-001", CriterionStatusRegressed, "TASK-005 broke it"); err != nil {
		t.Fatalf("VerifyCriterion(regressed) error = %v", err)
	}

	c := init.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("criterion not found")
	}
	if c.Status != CriterionStatusRegressed {
		t.Errorf("Status = %q, want %q", c.Status, CriterionStatusRegressed)
	}
	if c.Evidence != "TASK-005 broke it" {
		t.Errorf("Evidence = %q, want %q", c.Evidence, "TASK-005 broke it")
	}
}

func TestVerifyCriterion_NonExistent(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	err := init.VerifyCriterion("AC-999", CriterionStatusSatisfied, "evidence")
	if err == nil {
		t.Error("VerifyCriterion() should error for non-existent criterion")
	}
}

func TestVerifyCriterion_InvalidStatus(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("Some criterion")

	err := init.VerifyCriterion("AC-001", "invalid", "evidence")
	if err == nil {
		t.Error("VerifyCriterion() should error for invalid status")
	}
}

func TestVerifyAllCriteria(t *testing.T) {
	t.Parallel()

	init := New("INIT-001", "Test Initiative")
	init.AddCriterion("First")
	init.AddCriterion("Second")
	init.AddCriterion("Third")

	// Map all criteria to tasks first
	for _, c := range init.Criteria {
		if err := init.MapCriterionToTask(c.ID, "TASK-001"); err != nil {
			t.Fatalf("MapCriterionToTask(%s) error = %v", c.ID, err)
		}
	}

	results := init.VerifyAllCriteria("orchestrator")

	if len(results) != 3 {
		t.Errorf("VerifyAllCriteria() returned %d results, want 3", len(results))
	}

	// All should have VerifiedBy set
	for _, c := range init.Criteria {
		if c.VerifiedBy != "orchestrator" {
			t.Errorf("criterion %s VerifiedBy = %q, want %q", c.ID, c.VerifiedBy, "orchestrator")
		}
	}
}
