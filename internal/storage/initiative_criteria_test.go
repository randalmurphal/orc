package storage

import (
	"testing"

	"github.com/randalmurphal/orc/internal/initiative"
)

// ============================================================================
// SC-3: Database CRUD for initiative_criteria
// These tests verify the storage layer round-trips criteria correctly.
// They use NewTestBackend (in-memory SQLite) per constitution invariant.
// ============================================================================

func TestSaveLoadInitiative_WithCriteria(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	init.AddCriterion("User can log in with JWT")
	init.AddCriterion("User can refresh token")

	// Map first criterion to a task
	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}

	// Save
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	// Load
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiative() error = %v", err)
	}

	// Verify criteria were persisted
	if len(loaded.Criteria) != 2 {
		t.Fatalf("loaded Criteria len = %d, want 2", len(loaded.Criteria))
	}

	// Verify first criterion
	c1 := loaded.GetCriterion("AC-001")
	if c1 == nil {
		t.Fatal("AC-001 not found after load")
	}
	if c1.Description != "User can log in with JWT" {
		t.Errorf("AC-001 Description = %q, want %q", c1.Description, "User can log in with JWT")
	}
	if c1.Status != initiative.CriterionStatusCovered {
		t.Errorf("AC-001 Status = %q, want %q", c1.Status, initiative.CriterionStatusCovered)
	}
	if len(c1.TaskIDs) != 1 || c1.TaskIDs[0] != "TASK-001" {
		t.Errorf("AC-001 TaskIDs = %v, want [TASK-001]", c1.TaskIDs)
	}

	// Verify second criterion
	c2 := loaded.GetCriterion("AC-002")
	if c2 == nil {
		t.Fatal("AC-002 not found after load")
	}
	if c2.Description != "User can refresh token" {
		t.Errorf("AC-002 Description = %q, want %q", c2.Description, "User can refresh token")
	}
	if c2.Status != initiative.CriterionStatusUncovered {
		t.Errorf("AC-002 Status = %q, want %q", c2.Status, initiative.CriterionStatusUncovered)
	}
}

func TestSaveLoadInitiative_CriteriaWithVerification(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	init.AddCriterion("Feature works")

	// Map and verify
	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask() error = %v", err)
	}
	if err := init.VerifyCriterion("AC-001", initiative.CriterionStatusSatisfied, "E2E test passes"); err != nil {
		t.Fatalf("VerifyCriterion() error = %v", err)
	}

	// Save and reload
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiative() error = %v", err)
	}

	c := loaded.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("AC-001 not found after load")
	}
	if c.Status != initiative.CriterionStatusSatisfied {
		t.Errorf("Status = %q, want %q", c.Status, initiative.CriterionStatusSatisfied)
	}
	if c.Evidence != "E2E test passes" {
		t.Errorf("Evidence = %q, want %q", c.Evidence, "E2E test passes")
	}
	if c.VerifiedAt == "" {
		t.Error("VerifiedAt should be persisted")
	}
	if c.VerifiedBy == "" {
		t.Error("VerifiedBy should be persisted")
	}
}

func TestSaveLoadInitiative_NoCriteria(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	// Initiative without criteria should still work
	init := initiative.New("INIT-001", "No Criteria Initiative")
	init.Activate()

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiative() error = %v", err)
	}

	if len(loaded.Criteria) != 0 {
		t.Errorf("Criteria len = %d, want 0", len(loaded.Criteria))
	}
}

func TestSaveInitiative_CriteriaUpdate(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	// Initial save with 2 criteria
	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	init.AddCriterion("First")
	init.AddCriterion("Second")

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("first SaveInitiative() error = %v", err)
	}

	// Add a third and remove the first
	init.AddCriterion("Third")
	init.RemoveCriterion("AC-001")

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("second SaveInitiative() error = %v", err)
	}

	// Verify update
	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiative() error = %v", err)
	}

	if len(loaded.Criteria) != 2 {
		t.Fatalf("Criteria len = %d, want 2", len(loaded.Criteria))
	}

	// AC-001 should be gone
	if loaded.GetCriterion("AC-001") != nil {
		t.Error("AC-001 should have been removed")
	}

	// AC-002 and AC-003 should remain
	if loaded.GetCriterion("AC-002") == nil {
		t.Error("AC-002 should still exist")
	}
	if loaded.GetCriterion("AC-003") == nil {
		t.Error("AC-003 should still exist")
	}
}

func TestLoadAllInitiatives_WithCriteria(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	// Create two initiatives with criteria
	init1 := initiative.New("INIT-001", "First")
	init1.Activate()
	init1.AddCriterion("Criterion A")
	init1.AddCriterion("Criterion B")

	init2 := initiative.New("INIT-002", "Second")
	init2.Activate()
	init2.AddCriterion("Criterion C")

	if err := backend.SaveInitiative(init1); err != nil {
		t.Fatalf("SaveInitiative(1) error = %v", err)
	}
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("SaveInitiative(2) error = %v", err)
	}

	// Load all
	all, err := backend.LoadAllInitiatives()
	if err != nil {
		t.Fatalf("LoadAllInitiatives() error = %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("LoadAllInitiatives() returned %d, want 2", len(all))
	}

	// Find each initiative and verify criteria
	for _, init := range all {
		switch init.ID {
		case "INIT-001":
			if len(init.Criteria) != 2 {
				t.Errorf("INIT-001 Criteria len = %d, want 2", len(init.Criteria))
			}
		case "INIT-002":
			if len(init.Criteria) != 1 {
				t.Errorf("INIT-002 Criteria len = %d, want 1", len(init.Criteria))
			}
		default:
			t.Errorf("unexpected initiative ID: %s", init.ID)
		}
	}
}

func TestDeleteInitiative_CleansUpCriteria(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init := initiative.New("INIT-001", "To Delete")
	init.Activate()
	init.AddCriterion("Some criterion")
	init.AddCriterion("Another criterion")

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	// Delete the initiative
	if err := backend.DeleteInitiative("INIT-001"); err != nil {
		t.Fatalf("DeleteInitiative() error = %v", err)
	}

	// Verify initiative is gone
	_, err := backend.LoadInitiative("INIT-001")
	if err == nil {
		t.Error("LoadInitiative() should error after deletion")
	}

	// Note: Criteria should be cleaned up via CASCADE or explicit delete.
	// The implementation should handle this - if we re-create the initiative,
	// it should not have stale criteria.
	init2 := initiative.New("INIT-001", "Re-created")
	init2.Activate()
	if err := backend.SaveInitiative(init2); err != nil {
		t.Fatalf("SaveInitiative(re-create) error = %v", err)
	}

	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiative(re-created) error = %v", err)
	}
	if len(loaded.Criteria) != 0 {
		t.Errorf("re-created initiative has %d criteria, want 0 (stale data)", len(loaded.Criteria))
	}
}

// ============================================================================
// SC-3 + SC-4: Storage-level coverage operations
// These test that the storage backend properly supports coverage queries.
// ============================================================================

func TestSaveLoadInitiative_CriteriaWithMultipleTaskIDs(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init := initiative.New("INIT-001", "Test Initiative")
	init.Activate()
	init.AddCriterion("Complex criterion")

	// Map multiple tasks
	if err := init.MapCriterionToTask("AC-001", "TASK-001"); err != nil {
		t.Fatalf("MapCriterionToTask(1) error = %v", err)
	}
	if err := init.MapCriterionToTask("AC-001", "TASK-002"); err != nil {
		t.Fatalf("MapCriterionToTask(2) error = %v", err)
	}
	if err := init.MapCriterionToTask("AC-001", "TASK-003"); err != nil {
		t.Fatalf("MapCriterionToTask(3) error = %v", err)
	}

	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative() error = %v", err)
	}

	loaded, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiative() error = %v", err)
	}

	c := loaded.GetCriterion("AC-001")
	if c == nil {
		t.Fatal("AC-001 not found after load")
	}
	if len(c.TaskIDs) != 3 {
		t.Errorf("TaskIDs len = %d, want 3", len(c.TaskIDs))
	}
}
