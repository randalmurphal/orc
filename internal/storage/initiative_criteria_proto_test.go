package storage

import (
	"fmt"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
)

// ============================================================================
// SC-1: Proto Initiative message includes criteria field and Criterion message
// SC-2: Proto-based SaveInitiativeProto persists criteria and LoadInitiativeProto restores them
// ============================================================================

// TestSaveLoadInitiativeProto_WithCriteria verifies that criteria round-trip
// through the proto save/load path with all fields intact.
// Covers SC-2.
func TestSaveLoadInitiativeProto_WithCriteria(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	// Create proto initiative with criteria via proto helpers.
	// This also implicitly tests SC-1 (proto Criterion message exists).
	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE

	// Add criteria via proto field (requires Criterion proto message to exist)
	init.Criteria = []*orcv1.Criterion{
		{
			Id:          "AC-001",
			Description: "User can log in with JWT",
			Status:      "covered",
			TaskIds:     []string{"TASK-001"},
			VerifiedAt:  "2026-01-15T10:00:00Z",
			VerifiedBy:  "orchestrator",
			Evidence:    "E2E test passes",
		},
		{
			Id:          "AC-002",
			Description: "User can refresh expired token",
			Status:      "uncovered",
			TaskIds:     []string{},
		},
	}

	// Save via proto path
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("SaveInitiativeProto() error = %v", err)
	}

	// Load via proto path
	loaded, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiativeProto() error = %v", err)
	}

	// Verify criteria were persisted
	if len(loaded.Criteria) != 2 {
		t.Fatalf("loaded Criteria len = %d, want 2", len(loaded.Criteria))
	}

	// Verify first criterion - all fields
	c1 := findProtoCriterion(loaded.Criteria, "AC-001")
	if c1 == nil {
		t.Fatal("AC-001 not found after proto load")
	}
	if c1.Description != "User can log in with JWT" {
		t.Errorf("AC-001 Description = %q, want %q", c1.Description, "User can log in with JWT")
	}
	if c1.Status != "covered" {
		t.Errorf("AC-001 Status = %q, want %q", c1.Status, "covered")
	}
	if len(c1.TaskIds) != 1 || c1.TaskIds[0] != "TASK-001" {
		t.Errorf("AC-001 TaskIds = %v, want [TASK-001]", c1.TaskIds)
	}
	if c1.VerifiedAt != "2026-01-15T10:00:00Z" {
		t.Errorf("AC-001 VerifiedAt = %q, want %q", c1.VerifiedAt, "2026-01-15T10:00:00Z")
	}
	if c1.VerifiedBy != "orchestrator" {
		t.Errorf("AC-001 VerifiedBy = %q, want %q", c1.VerifiedBy, "orchestrator")
	}
	if c1.Evidence != "E2E test passes" {
		t.Errorf("AC-001 Evidence = %q, want %q", c1.Evidence, "E2E test passes")
	}

	// Verify second criterion
	c2 := findProtoCriterion(loaded.Criteria, "AC-002")
	if c2 == nil {
		t.Fatal("AC-002 not found after proto load")
	}
	if c2.Description != "User can refresh expired token" {
		t.Errorf("AC-002 Description = %q, want %q", c2.Description, "User can refresh expired token")
	}
	if c2.Status != "uncovered" {
		t.Errorf("AC-002 Status = %q, want %q", c2.Status, "uncovered")
	}
}

// TestSaveLoadInitiativeProto_NoCriteria verifies that initiatives without criteria
// work correctly on the proto path.
func TestSaveLoadInitiativeProto_NoCriteria(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "No Criteria")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE

	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("SaveInitiativeProto() error = %v", err)
	}

	loaded, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiativeProto() error = %v", err)
	}

	if len(loaded.Criteria) != 0 {
		t.Errorf("Criteria len = %d, want 0", len(loaded.Criteria))
	}
}

// TestSaveInitiativeProto_CriteriaUpdate verifies that criteria are replaced
// atomically when saving.
func TestSaveInitiativeProto_CriteriaUpdate(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init.Criteria = []*orcv1.Criterion{
		{Id: "AC-001", Description: "First", Status: "uncovered", TaskIds: []string{}},
		{Id: "AC-002", Description: "Second", Status: "uncovered", TaskIds: []string{}},
	}

	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("first SaveInitiativeProto() error = %v", err)
	}

	// Remove first, add third
	init.Criteria = []*orcv1.Criterion{
		{Id: "AC-002", Description: "Second", Status: "covered", TaskIds: []string{"TASK-001"}},
		{Id: "AC-003", Description: "Third", Status: "uncovered", TaskIds: []string{}},
	}

	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("second SaveInitiativeProto() error = %v", err)
	}

	loaded, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiativeProto() error = %v", err)
	}

	if len(loaded.Criteria) != 2 {
		t.Fatalf("Criteria len = %d, want 2", len(loaded.Criteria))
	}

	// AC-001 should be gone
	if findProtoCriterion(loaded.Criteria, "AC-001") != nil {
		t.Error("AC-001 should have been removed")
	}

	// AC-002 should exist with updated status
	c2 := findProtoCriterion(loaded.Criteria, "AC-002")
	if c2 == nil {
		t.Fatal("AC-002 should still exist")
	}
	if c2.Status != "covered" {
		t.Errorf("AC-002 Status = %q, want %q", c2.Status, "covered")
	}

	// AC-003 should exist
	if findProtoCriterion(loaded.Criteria, "AC-003") == nil {
		t.Error("AC-003 should exist")
	}
}

// TestLoadAllInitiativesProto_WithCriteria verifies batch loading populates criteria.
func TestLoadAllInitiativesProto_WithCriteria(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init1 := initiative.NewProtoInitiative("INIT-001", "First")
	init1.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init1.Criteria = []*orcv1.Criterion{
		{Id: "AC-001", Description: "Criterion A", Status: "uncovered", TaskIds: []string{}},
		{Id: "AC-002", Description: "Criterion B", Status: "covered", TaskIds: []string{"TASK-001"}},
	}

	init2 := initiative.NewProtoInitiative("INIT-002", "Second")
	init2.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init2.Criteria = []*orcv1.Criterion{
		{Id: "AC-001", Description: "Criterion C", Status: "satisfied", TaskIds: []string{"TASK-002"}, Evidence: "Tests pass"},
	}

	if err := backend.SaveInitiativeProto(init1); err != nil {
		t.Fatalf("SaveInitiativeProto(1) error = %v", err)
	}
	if err := backend.SaveInitiativeProto(init2); err != nil {
		t.Fatalf("SaveInitiativeProto(2) error = %v", err)
	}

	all, err := backend.LoadAllInitiativesProto()
	if err != nil {
		t.Fatalf("LoadAllInitiativesProto() error = %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("LoadAllInitiativesProto() returned %d, want 2", len(all))
	}

	for _, init := range all {
		switch init.Id {
		case "INIT-001":
			if len(init.Criteria) != 2 {
				t.Errorf("INIT-001 Criteria len = %d, want 2", len(init.Criteria))
			}
		case "INIT-002":
			if len(init.Criteria) != 1 {
				t.Errorf("INIT-002 Criteria len = %d, want 1", len(init.Criteria))
			}
		default:
			t.Errorf("unexpected initiative ID: %s", init.Id)
		}
	}
}

// TestSaveInitiativeProto_CriteriaWithMultipleTaskIDs verifies JSON array persistence.
func TestSaveInitiativeProto_CriteriaWithMultipleTaskIDs(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init.Criteria = []*orcv1.Criterion{
		{
			Id:          "AC-001",
			Description: "Complex criterion",
			Status:      "covered",
			TaskIds:     []string{"TASK-001", "TASK-002", "TASK-003"},
		},
	}

	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("SaveInitiativeProto() error = %v", err)
	}

	loaded, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiativeProto() error = %v", err)
	}

	if len(loaded.Criteria) != 1 {
		t.Fatalf("Criteria len = %d, want 1", len(loaded.Criteria))
	}

	c := loaded.Criteria[0]
	if len(c.TaskIds) != 3 {
		t.Errorf("TaskIds len = %d, want 3", len(c.TaskIds))
	}
}

// TestSaveInitiativeProto_CriteriaSpecialChars verifies JSON encoding handles special characters.
func TestSaveInitiativeProto_CriteriaSpecialChars(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init.Criteria = []*orcv1.Criterion{
		{
			Id:          "AC-001",
			Description: `User sees "error" with <html> & 'quotes'`,
			Status:      "satisfied",
			TaskIds:     []string{},
			Evidence:    `Test log: "OK"\nLine 2`,
		},
	}

	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("SaveInitiativeProto() error = %v", err)
	}

	loaded, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiativeProto() error = %v", err)
	}

	c := loaded.Criteria[0]
	if c.Description != `User sees "error" with <html> & 'quotes'` {
		t.Errorf("Description = %q, want special chars preserved", c.Description)
	}
	if c.Evidence != `Test log: "OK"\nLine 2` {
		t.Errorf("Evidence = %q, want special chars preserved", c.Evidence)
	}
}

// TestSaveInitiativeProto_CriteriaBulkOperations verifies handling of many criteria.
func TestSaveInitiativeProto_CriteriaBulkOperations(t *testing.T) {
	t.Parallel()
	backend := NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Bulk Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE

	// Create 50+ criteria
	for i := 1; i <= 55; i++ {
		init.Criteria = append(init.Criteria, &orcv1.Criterion{
			Id:          fmt.Sprintf("AC-%03d", i),
			Description: fmt.Sprintf("Criterion %d", i),
			Status:      "uncovered",
			TaskIds:     []string{},
		})
	}

	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("SaveInitiativeProto() error = %v", err)
	}

	loaded, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiativeProto() error = %v", err)
	}

	if len(loaded.Criteria) != 55 {
		t.Errorf("Criteria len = %d, want 55", len(loaded.Criteria))
	}
}

// findProtoCriterion is a test helper to find a criterion by ID in a proto slice.
func findProtoCriterion(criteria []*orcv1.Criterion, id string) *orcv1.Criterion {
	for _, c := range criteria {
		if c.Id == id {
			return c
		}
	}
	return nil
}
