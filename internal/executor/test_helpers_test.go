package executor

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// testGlobalDBFrom creates a GlobalDB wrapping the same underlying *DB
// as the given backend's ProjectDB. Both GlobalDB and ProjectDB embed *DB
// and have identical workflow CRUD methods on identical tables.
// Writes via pdb.SaveWorkflow() are visible through gdb.GetWorkflow().
//
// Accepts storage.Backend (interface) so it works with both
// *storage.DatabaseBackend and test wrapper types.
func testGlobalDBFrom(backend storage.Backend) *db.GlobalDB {
	return &db.GlobalDB{DB: backend.DB().DB}
}

// setupMinimalWorkflowGlobal seeds a minimal workflow + phase into a GlobalDB.
// Used by tests that need a separate GlobalDB (e.g., cost tracking tests) rather
// than the shared testGlobalDBFrom wrapper.
func setupMinimalWorkflowGlobal(t *testing.T, gdb *db.GlobalDB, workflowID string) {
	t.Helper()

	wf := &db.Workflow{
		ID:          workflowID,
		Name:        "Test Workflow",
		Description: "Test",
	}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow to globalDB: %v", err)
	}

	// Save phase template (GlobalDB may not have seeded templates)
	tmpl := &db.PhaseTemplate{
		ID:            "implement",
		Name:          "implement",
		PromptSource:  "db",
		PromptContent: "Test prompt for implement",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("save phase template to globalDB: %v", err)
	}

	wfPhase := &db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := gdb.SaveWorkflowPhase(wfPhase); err != nil {
		t.Fatalf("save workflow phase to globalDB: %v", err)
	}
}
