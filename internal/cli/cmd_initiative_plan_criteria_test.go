package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/initiative"
)

// ============================================================================
// SC-11: `orc initiative plan manifest.yaml` creates acceptance criteria
// from manifest's acceptance_criteria in create_initiative section.
// ============================================================================

// TestInitiativePlan_CriteriaFromManifest verifies SC-11:
// Manifest with acceptance_criteria creates initiative with criteria
// matching manifest descriptions, each with auto-generated IDs.
func TestInitiativePlan_CriteriaFromManifest(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create manifest with acceptance_criteria
	manifest := `version: 1
create_initiative:
  title: "User Authentication"
  vision: "JWT-based auth with refresh tokens"
  acceptance_criteria:
    - "User can log in with JWT"
    - "User can refresh expired token"
    - "Invalid tokens are rejected with 401"
tasks:
  - id: 1
    title: "Login endpoint"
    weight: small
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_ = backend.Close()

	// Run plan command
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	// Re-open backend to verify
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Load initiative
	initiatives, err := backend.LoadAllInitiatives()
	if err != nil {
		t.Fatalf("LoadAllInitiatives() error = %v", err)
	}
	if len(initiatives) != 1 {
		t.Fatalf("expected 1 initiative, got %d", len(initiatives))
	}

	init := initiatives[0]

	// SC-11: Initiative should have 3 criteria
	if len(init.Criteria) != 3 {
		t.Fatalf("Criteria len = %d, want 3", len(init.Criteria))
	}

	// Verify auto-generated IDs
	expectedCriteria := []struct {
		id          string
		description string
		status      string
	}{
		{"AC-001", "User can log in with JWT", initiative.CriterionStatusUncovered},
		{"AC-002", "User can refresh expired token", initiative.CriterionStatusUncovered},
		{"AC-003", "Invalid tokens are rejected with 401", initiative.CriterionStatusUncovered},
	}

	for _, ec := range expectedCriteria {
		c := init.GetCriterion(ec.id)
		if c == nil {
			t.Errorf("criterion %s not found", ec.id)
			continue
		}
		if c.Description != ec.description {
			t.Errorf("%s Description = %q, want %q", ec.id, c.Description, ec.description)
		}
		if c.Status != ec.status {
			t.Errorf("%s Status = %q, want %q", ec.id, c.Status, ec.status)
		}
	}
}

// TestInitiativePlan_CriteriaWithoutTaskRefs verifies edge case:
// Manifest with acceptance_criteria but no tasks referencing them
// creates criteria all with "uncovered" status.
func TestInitiativePlan_CriteriaWithoutTaskRefs(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	manifest := `version: 1
create_initiative:
  title: "Feature with Uncovered Criteria"
  acceptance_criteria:
    - "First criterion"
    - "Second criterion"
tasks:
  - id: 1
    title: "Task without criteria ref"
    weight: small
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_ = backend.Close()

	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	initiatives, err := backend.LoadAllInitiatives()
	if err != nil {
		t.Fatalf("LoadAllInitiatives() error = %v", err)
	}
	if len(initiatives) != 1 {
		t.Fatalf("expected 1 initiative, got %d", len(initiatives))
	}

	init := initiatives[0]

	// Both criteria should exist and be uncovered
	if len(init.Criteria) != 2 {
		t.Fatalf("Criteria len = %d, want 2", len(init.Criteria))
	}
	for _, c := range init.Criteria {
		if c.Status != initiative.CriterionStatusUncovered {
			t.Errorf("criterion %s Status = %q, want %q", c.ID, c.Status, initiative.CriterionStatusUncovered)
		}
	}
}

// ============================================================================
// SC-12: `orc initiative plan manifest.yaml` maps task criteria indices
// to initiative criteria.
// ============================================================================

// TestInitiativePlan_TaskCriteriaMapped verifies SC-12 and BDD-4:
// Tasks that reference criteria by index show up in criterion's task_ids
// and criteria status transitions to "covered".
func TestInitiativePlan_TaskCriteriaMapped(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// BDD-4 manifest: 2 criteria, task references first one
	manifest := `version: 1
create_initiative:
  title: "Auth Feature"
  acceptance_criteria:
    - "Users can login"
    - "Users can logout"
tasks:
  - id: 1
    title: "Login endpoint"
    weight: small
    criteria: [0]
  - id: 2
    title: "Logout endpoint"
    weight: small
    criteria: [1]
  - id: 3
    title: "Auth middleware"
    weight: small
    criteria: [0, 1]
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_ = backend.Close()

	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	initiatives, err := backend.LoadAllInitiatives()
	if err != nil {
		t.Fatalf("LoadAllInitiatives() error = %v", err)
	}
	if len(initiatives) != 1 {
		t.Fatalf("expected 1 initiative, got %d", len(initiatives))
	}

	init := initiatives[0]

	// Load tasks to find their IDs
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("LoadAllTasks() error = %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Build task title → ID map
	taskIDByTitle := make(map[string]string)
	for _, tk := range tasks {
		taskIDByTitle[tk.Title] = tk.Id
	}

	loginTaskID := taskIDByTitle["Login endpoint"]
	logoutTaskID := taskIDByTitle["Logout endpoint"]
	middlewareTaskID := taskIDByTitle["Auth middleware"]

	// SC-12: AC-001 ("Users can login") should have Login and Auth middleware tasks
	c1 := init.GetCriterion("AC-001")
	if c1 == nil {
		t.Fatal("AC-001 not found")
	}
	if c1.Status != initiative.CriterionStatusCovered {
		t.Errorf("AC-001 Status = %q, want %q (should be covered after task mapping)", c1.Status, initiative.CriterionStatusCovered)
	}
	if !containsString(c1.TaskIDs, loginTaskID) {
		t.Errorf("AC-001 TaskIDs should contain login task %s, got %v", loginTaskID, c1.TaskIDs)
	}
	if !containsString(c1.TaskIDs, middlewareTaskID) {
		t.Errorf("AC-001 TaskIDs should contain middleware task %s, got %v", middlewareTaskID, c1.TaskIDs)
	}

	// SC-12: AC-002 ("Users can logout") should have Logout and Auth middleware tasks
	c2 := init.GetCriterion("AC-002")
	if c2 == nil {
		t.Fatal("AC-002 not found")
	}
	if c2.Status != initiative.CriterionStatusCovered {
		t.Errorf("AC-002 Status = %q, want %q (should be covered after task mapping)", c2.Status, initiative.CriterionStatusCovered)
	}
	if !containsString(c2.TaskIDs, logoutTaskID) {
		t.Errorf("AC-002 TaskIDs should contain logout task %s, got %v", logoutTaskID, c2.TaskIDs)
	}
	if !containsString(c2.TaskIDs, middlewareTaskID) {
		t.Errorf("AC-002 TaskIDs should contain middleware task %s, got %v", middlewareTaskID, c2.TaskIDs)
	}
}

// TestInitiativePlan_PartialCriteriaMapping verifies BDD-4 partial case:
// Only the first criterion is covered when only one task references it.
func TestInitiativePlan_PartialCriteriaMapping(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	manifest := `version: 1
create_initiative:
  title: "Auth Feature"
  acceptance_criteria:
    - "Users can login"
    - "Users can logout"
tasks:
  - id: 1
    title: "Login endpoint"
    weight: small
    criteria: [0]
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_ = backend.Close()

	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	initiatives, err := backend.LoadAllInitiatives()
	if err != nil {
		t.Fatalf("LoadAllInitiatives() error = %v", err)
	}
	init := initiatives[0]

	// AC-001 should be covered (has task)
	c1 := init.GetCriterion("AC-001")
	if c1 == nil {
		t.Fatal("AC-001 not found")
	}
	if c1.Status != initiative.CriterionStatusCovered {
		t.Errorf("AC-001 Status = %q, want %q", c1.Status, initiative.CriterionStatusCovered)
	}
	if len(c1.TaskIDs) != 1 {
		t.Errorf("AC-001 TaskIDs len = %d, want 1", len(c1.TaskIDs))
	}

	// AC-002 should remain uncovered (no task references it)
	c2 := init.GetCriterion("AC-002")
	if c2 == nil {
		t.Fatal("AC-002 not found")
	}
	if c2.Status != initiative.CriterionStatusUncovered {
		t.Errorf("AC-002 Status = %q, want %q (no task references it)", c2.Status, initiative.CriterionStatusUncovered)
	}
}

// TestInitiativePlan_NoCriteriaInManifest verifies that plan works
// without acceptance_criteria (backward compatibility).
func TestInitiativePlan_NoCriteriaInManifest(t *testing.T) {
	tmpDir := withInitiativeTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	manifest := `version: 1
create_initiative:
  title: "Simple Feature"
  vision: "No criteria"
tasks:
  - id: 1
    title: "Do the thing"
    weight: small
`
	manifestPath := filepath.Join(tmpDir, "tasks.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_ = backend.Close()

	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("plan command failed: %v", err)
	}

	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	initiatives, err := backend.LoadAllInitiatives()
	if err != nil {
		t.Fatalf("LoadAllInitiatives() error = %v", err)
	}
	if len(initiatives) != 1 {
		t.Fatalf("expected 1 initiative, got %d", len(initiatives))
	}

	// No criteria should exist
	if len(initiatives[0].Criteria) != 0 {
		t.Errorf("Criteria len = %d, want 0 (no criteria in manifest)", len(initiatives[0].Criteria))
	}

	// Task should still be created
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("LoadAllTasks() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

// containsString checks if a slice contains a given string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
