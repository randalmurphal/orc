package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/initiative"
)

// ============================================================================
// SC-7: `orc initiative criteria INIT-XXX` lists all criteria with status
// ============================================================================

// TestInitiativeCriteriaList_ShowsCriteriaWithStatus verifies SC-7:
// Tabular output shows ID, Description, Status, and linked task count.
func TestInitiativeCriteriaList_ShowsCriteriaWithStatus(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative with criteria
	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	init.AddCriterion("User can log in with JWT")
	init.AddCriterion("User can refresh token")
	init.AddCriterion("Invalid tokens rejected")

	// Map first criterion to a task
	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}
	// Verify third criterion
	if err := init.VerifyCriterion("AC-003", initiative.CriterionStatusSatisfied, "E2E passes"); err != nil {
		t.Fatalf("VerifyCriterion() error = %v", err)
	}

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("criteria command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Verify all criteria appear in output
	if !strings.Contains(output, "AC-001") {
		t.Errorf("output should contain AC-001, got:\n%s", output)
	}
	if !strings.Contains(output, "AC-002") {
		t.Errorf("output should contain AC-002, got:\n%s", output)
	}
	if !strings.Contains(output, "AC-003") {
		t.Errorf("output should contain AC-003, got:\n%s", output)
	}

	// Verify statuses appear
	if !strings.Contains(output, "covered") {
		t.Errorf("output should contain 'covered' status, got:\n%s", output)
	}
	if !strings.Contains(output, "uncovered") {
		t.Errorf("output should contain 'uncovered' status, got:\n%s", output)
	}
	if !strings.Contains(output, "satisfied") {
		t.Errorf("output should contain 'satisfied' status, got:\n%s", output)
	}

	// Verify descriptions appear
	if !strings.Contains(output, "User can log in with JWT") {
		t.Errorf("output should contain criterion description, got:\n%s", output)
	}
}

// TestInitiativeCriteriaList_NotFound verifies SC-7 error path.
func TestInitiativeCriteriaList_NotFound(t *testing.T) {
	_ = withInitiativeTestDir(t)

	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-NONEXISTENT"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent initiative")
	}
}

// TestInitiativeCriteriaList_NoCriteria verifies output with empty criteria.
func TestInitiativeCriteriaList_NoCriteria(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("criteria command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should indicate no criteria exist
	if !strings.Contains(strings.ToLower(output), "no criteria") && !strings.Contains(output, "0") {
		t.Errorf("output should indicate no criteria, got:\n%s", output)
	}
}

// ============================================================================
// SC-8: `orc initiative criteria verify AC-001 --satisfied --evidence "text"`
// ============================================================================

// TestInitiativeCriteriaVerify_Success verifies SC-8:
// Verify command updates criterion status and evidence.
func TestInitiativeCriteriaVerify_Success(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	init.AddCriterion("User can log in")
	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	// Run verify command
	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001", "verify", "AC-001", "--status", "satisfied", "--evidence", "E2E test passes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("criteria verify command failed: %v", err)
	}

	// Re-open and verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiative() error = %v", err)
	}

	c := loaded.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("AC-001 not found")
	}
	if c.Status != initiative.CriterionStatusSatisfied {
		t.Errorf("Status = %q, want %q", c.Status, initiative.CriterionStatusSatisfied)
	}
	if c.Evidence != "E2E test passes" {
		t.Errorf("Evidence = %q, want %q", c.Evidence, "E2E test passes")
	}
	if c.VerifiedAt == "" {
		t.Error("VerifiedAt should be set")
	}
}

// TestInitiativeCriteriaVerify_InvalidStatus verifies SC-8 error path.
func TestInitiativeCriteriaVerify_InvalidStatus(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	init.AddCriterion("User can log in")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001", "verify", "AC-001", "--status", "invalid_value", "--evidence", "test"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid status value")
	}
}

// TestInitiativeCriteriaVerify_CriterionNotFound verifies SC-8 error path.
func TestInitiativeCriteriaVerify_CriterionNotFound(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001", "verify", "AC-999", "--status", "satisfied", "--evidence", "test"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent criterion")
	}
}

// ============================================================================
// SC-9: `orc initiative criteria add INIT-XXX "description"`
// ============================================================================

// TestInitiativeCriteriaAdd_Success verifies SC-9:
// Add command creates new criterion with auto-generated ID.
func TestInitiativeCriteriaAdd_Success(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	// Run add command
	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001", "add", "User can log in with JWT"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("criteria add command failed: %v", err)
	}

	// Re-open and verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiative() error = %v", err)
	}

	if len(loaded.Criteria) != 1 {
		t.Fatalf("Criteria len = %d, want 1", len(loaded.Criteria))
	}

	c := loaded.Criteria[0]
	if c.ID != "AC-001" {
		t.Errorf("criterion ID = %q, want AC-001", c.ID)
	}
	if c.Description != "User can log in with JWT" {
		t.Errorf("Description = %q, want %q", c.Description, "User can log in with JWT")
	}
	if c.Status != initiative.CriterionStatusUncovered {
		t.Errorf("Status = %q, want %q", c.Status, initiative.CriterionStatusUncovered)
	}
}

// TestInitiativeCriteriaAdd_EmptyDescription verifies SC-9 error path.
func TestInitiativeCriteriaAdd_EmptyDescription(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001", "add", ""})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty description")
	}
}

// ============================================================================
// SC-10: `orc initiative criteria map AC-001 TASK-XXX`
// ============================================================================

// TestInitiativeCriteriaMap_Success verifies SC-10:
// Map command links criterion to task and transitions status.
func TestInitiativeCriteriaMap_Success(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	init.AddCriterion("User can log in")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	// Run map command
	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001", "map", "AC-001", "TASK-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("criteria map command failed: %v", err)
	}

	// Re-open and verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiative() error = %v", err)
	}

	c := loaded.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("AC-001 not found")
	}
	if len(c.TaskIDs) != 1 || c.TaskIDs[0] != "TASK-001" {
		t.Errorf("TaskIDs = %v, want [TASK-001]", c.TaskIDs)
	}
	if c.Status != initiative.CriterionStatusCovered {
		t.Errorf("Status = %q, want %q", c.Status, initiative.CriterionStatusCovered)
	}
}

// TestInitiativeCriteriaMap_InvalidCriterion verifies SC-10 error path.
func TestInitiativeCriteriaMap_InvalidCriterion(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001", "map", "AC-999", "TASK-001"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent criterion")
	}
}

// ============================================================================
// CLI criteria coverage subcommand
// ============================================================================

// TestInitiativeCriteriaCoverage_ShowsReport verifies coverage summary output.
func TestInitiativeCriteriaCoverage_ShowsReport(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	init.AddCriterion("First")  // uncovered
	init.AddCriterion("Second") // will be covered
	init.AddCriterion("Third")  // will be satisfied

	if err := init.MapCriterionToTask("AC-002", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}
	if err := init.MapCriterionToTask("AC-003", "TASK-002"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}
	if err := init.VerifyCriterion("AC-003", initiative.CriterionStatusSatisfied, "Tests pass"); err != nil {
		t.Fatalf("VerifyCriterion() error = %v", err)
	}

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newInitiativeCriteriaCmd()
	cmd.SetArgs([]string{"INIT-001", "coverage"})
	if err := cmd.Execute(); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("criteria coverage command failed: %v", err)
	}

	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should show total count
	if !strings.Contains(output, "3") {
		t.Errorf("output should contain total count '3', got:\n%s", output)
	}
}

// ============================================================================
// Integration: criteria subcommand is registered on initiative command
// ============================================================================

// TestInitiativeCommand_HasCriteriaSubcommand verifies wiring:
// The criteria subcommand is registered in the initiative command group.
func TestInitiativeCommand_HasCriteriaSubcommand(t *testing.T) {
	cmd := newInitiativeCmd()

	// Check that 'criteria' subcommand exists
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "criteria" {
			found = true
			break
		}
	}
	if !found {
		t.Error("initiative command should have 'criteria' subcommand registered")
	}
}
