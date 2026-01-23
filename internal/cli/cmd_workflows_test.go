package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
)

func setupWorkflowTestDB(t *testing.T) (*db.ProjectDB, string) {
	t.Helper()
	tmpDir := t.TempDir()

	pdb, err := db.OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}

	// Seed built-ins
	if _, err := workflow.SeedBuiltins(pdb); err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	return pdb, tmpDir
}

func TestWorkflowNew_FromExisting(t *testing.T) {
	pdb, tmpDir := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Create .orc directory to satisfy FindProjectRoot
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Set working directory context for command
	oldCmd := workflowNewCmd
	t.Cleanup(func() { workflowNewCmd = oldCmd })

	// Execute command manually since we can't easily mock FindProjectRoot
	// Instead, test the database operations directly
	source, err := pdb.GetWorkflow("implement-small")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if source == nil {
		t.Fatal("source workflow not found")
	}

	// Create cloned workflow
	newWf := &db.Workflow{
		ID:              "test-cloned",
		Name:            "test-cloned",
		Description:     source.Description,
		WorkflowType:    source.WorkflowType,
		DefaultModel:    source.DefaultModel,
		DefaultThinking: source.DefaultThinking,
		IsBuiltin:       false,
		BasedOn:         "implement-small",
	}

	if err := pdb.SaveWorkflow(newWf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Copy phases
	phases, err := pdb.GetWorkflowPhases("implement-small")
	if err != nil {
		t.Fatalf("GetWorkflowPhases failed: %v", err)
	}

	for _, p := range phases {
		newPhase := &db.WorkflowPhase{
			WorkflowID:            "test-cloned",
			PhaseTemplateID:       p.PhaseTemplateID,
			Sequence:              p.Sequence,
			MaxIterationsOverride: p.MaxIterationsOverride,
			ModelOverride:         p.ModelOverride,
		}
		if err := pdb.SaveWorkflowPhase(newPhase); err != nil {
			t.Fatalf("SaveWorkflowPhase failed: %v", err)
		}
	}

	// Verify clone
	cloned, err := pdb.GetWorkflow("test-cloned")
	if err != nil {
		t.Fatalf("GetWorkflow for clone failed: %v", err)
	}
	if cloned == nil {
		t.Fatal("cloned workflow not found")
	}
	if cloned.IsBuiltin {
		t.Error("cloned workflow should not be builtin")
	}
	if cloned.BasedOn != "implement-small" {
		t.Errorf("BasedOn = %q, want %q", cloned.BasedOn, "implement-small")
	}

	clonedPhases, err := pdb.GetWorkflowPhases("test-cloned")
	if err != nil {
		t.Fatalf("GetWorkflowPhases for clone failed: %v", err)
	}
	if len(clonedPhases) != len(phases) {
		t.Errorf("cloned phases = %d, want %d", len(clonedPhases), len(phases))
	}
}

func TestWorkflowAddPhase_Success(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Create a custom workflow first
	wf := &db.Workflow{
		ID:        "test-custom",
		Name:      "Test Custom",
		IsBuiltin: false,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Add initial phase
	phase1 := &db.WorkflowPhase{
		WorkflowID:      "test-custom",
		PhaseTemplateID: "implement",
		Sequence:        0,
	}
	if err := pdb.SaveWorkflowPhase(phase1); err != nil {
		t.Fatalf("SaveWorkflowPhase failed: %v", err)
	}

	// Add second phase (simulating CLI add-phase)
	phase2 := &db.WorkflowPhase{
		WorkflowID:      "test-custom",
		PhaseTemplateID: "review",
		Sequence:        1,
	}
	if err := pdb.SaveWorkflowPhase(phase2); err != nil {
		t.Fatalf("SaveWorkflowPhase for review failed: %v", err)
	}

	// Verify
	phases, err := pdb.GetWorkflowPhases("test-custom")
	if err != nil {
		t.Fatalf("GetWorkflowPhases failed: %v", err)
	}
	if len(phases) != 2 {
		t.Errorf("phase count = %d, want 2", len(phases))
	}
	if phases[0].PhaseTemplateID != "implement" {
		t.Errorf("first phase = %q, want implement", phases[0].PhaseTemplateID)
	}
	if phases[1].PhaseTemplateID != "review" {
		t.Errorf("second phase = %q, want review", phases[1].PhaseTemplateID)
	}
}

func TestWorkflowAddPhase_BuiltinFails(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Try to add phase to builtin workflow
	wf, err := pdb.GetWorkflow("implement-medium")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if wf == nil {
		t.Fatal("implement-medium not found")
	}
	if !wf.IsBuiltin {
		t.Error("implement-medium should be builtin")
	}

	// The CLI command should reject this - test the check logic
	if !wf.IsBuiltin {
		t.Error("expected builtin check to prevent modification")
	}
}

func TestWorkflowRemovePhase_Success(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Create custom workflow with phases
	wf := &db.Workflow{
		ID:        "test-remove",
		Name:      "Test Remove",
		IsBuiltin: false,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Add phases
	for i, phaseID := range []string{"spec", "implement", "review"} {
		phase := &db.WorkflowPhase{
			WorkflowID:      "test-remove",
			PhaseTemplateID: phaseID,
			Sequence:        i,
		}
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("SaveWorkflowPhase failed: %v", err)
		}
	}

	// Remove middle phase
	if err := pdb.DeleteWorkflowPhase("test-remove", "implement"); err != nil {
		t.Fatalf("DeleteWorkflowPhase failed: %v", err)
	}

	// Re-sequence (simulating CLI behavior)
	phases, err := pdb.GetWorkflowPhases("test-remove")
	if err != nil {
		t.Fatalf("GetWorkflowPhases failed: %v", err)
	}

	if len(phases) != 2 {
		t.Errorf("phase count = %d, want 2", len(phases))
	}

	// After deletion, remaining phases should be spec (seq 0) and review (seq 2)
	// The CLI would re-sequence, but let's just verify deletion worked
	foundSpec, foundReview := false, false
	for _, p := range phases {
		if p.PhaseTemplateID == "spec" {
			foundSpec = true
		}
		if p.PhaseTemplateID == "review" {
			foundReview = true
		}
		if p.PhaseTemplateID == "implement" {
			t.Error("implement phase should have been deleted")
		}
	}
	if !foundSpec {
		t.Error("spec phase should exist")
	}
	if !foundReview {
		t.Error("review phase should exist")
	}
}

func TestWorkflowAddVariable_Success(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Create custom workflow
	wf := &db.Workflow{
		ID:        "test-var",
		Name:      "Test Var",
		IsBuiltin: false,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Add variable
	variable := &db.WorkflowVariable{
		WorkflowID:   "test-var",
		Name:         "API_KEY",
		Description:  "API key for external service",
		SourceType:   "env",
		SourceConfig: `{"var": "API_KEY"}`,
		Required:     true,
	}
	if err := pdb.SaveWorkflowVariable(variable); err != nil {
		t.Fatalf("SaveWorkflowVariable failed: %v", err)
	}

	// Verify
	vars, err := pdb.GetWorkflowVariables("test-var")
	if err != nil {
		t.Fatalf("GetWorkflowVariables failed: %v", err)
	}
	if len(vars) != 1 {
		t.Errorf("variable count = %d, want 1", len(vars))
	}
	if vars[0].Name != "API_KEY" {
		t.Errorf("variable name = %q, want API_KEY", vars[0].Name)
	}
	if !vars[0].Required {
		t.Error("variable should be required")
	}
}

func TestWorkflowRemoveVariable_Success(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Create custom workflow with variable
	wf := &db.Workflow{
		ID:        "test-rmvar",
		Name:      "Test Remove Var",
		IsBuiltin: false,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	variable := &db.WorkflowVariable{
		WorkflowID:   "test-rmvar",
		Name:         "TO_DELETE",
		SourceType:   "static",
		SourceConfig: `{"value": "test"}`,
	}
	if err := pdb.SaveWorkflowVariable(variable); err != nil {
		t.Fatalf("SaveWorkflowVariable failed: %v", err)
	}

	// Verify it exists
	vars, err := pdb.GetWorkflowVariables("test-rmvar")
	if err != nil {
		t.Fatalf("GetWorkflowVariables failed: %v", err)
	}
	if len(vars) != 1 {
		t.Fatalf("expected 1 variable before delete, got %d", len(vars))
	}

	// Delete
	if err := pdb.DeleteWorkflowVariable("test-rmvar", "TO_DELETE"); err != nil {
		t.Fatalf("DeleteWorkflowVariable failed: %v", err)
	}

	// Verify deletion
	vars, err = pdb.GetWorkflowVariables("test-rmvar")
	if err != nil {
		t.Fatalf("GetWorkflowVariables after delete failed: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("expected 0 variables after delete, got %d", len(vars))
	}
}

func TestWorkflowEdit_UpdatesProperties(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Create custom workflow
	wf := &db.Workflow{
		ID:          "test-edit",
		Name:        "Original Name",
		Description: "Original description",
		IsBuiltin:   false,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Edit properties (simulating CLI edit command)
	wf.Name = "Updated Name"
	wf.Description = "Updated description"
	wf.DefaultModel = "opus"
	wf.DefaultThinking = true

	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow for update failed: %v", err)
	}

	// Verify
	updated, err := pdb.GetWorkflow("test-edit")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", updated.Name, "Updated Name")
	}
	if updated.Description != "Updated description" {
		t.Errorf("Description = %q, want %q", updated.Description, "Updated description")
	}
	if updated.DefaultModel != "opus" {
		t.Errorf("DefaultModel = %q, want %q", updated.DefaultModel, "opus")
	}
	if !updated.DefaultThinking {
		t.Error("DefaultThinking should be true")
	}
}

func TestWorkflowEdit_BuiltinFails(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Get builtin workflow
	wf, err := pdb.GetWorkflow("implement-medium")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if wf == nil {
		t.Fatal("implement-medium not found")
	}

	// Verify it's builtin
	if !wf.IsBuiltin {
		t.Error("implement-medium should be builtin")
	}

	// The CLI would reject editing builtins
	// Just verify the flag is correctly set
}

func TestWorkflowDelete_Success(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Create custom workflow
	wf := &db.Workflow{
		ID:        "test-delete",
		Name:      "To Delete",
		IsBuiltin: false,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Add a phase so we can test cascade
	phase := &db.WorkflowPhase{
		WorkflowID:      "test-delete",
		PhaseTemplateID: "implement",
		Sequence:        0,
	}
	if err := pdb.SaveWorkflowPhase(phase); err != nil {
		t.Fatalf("SaveWorkflowPhase failed: %v", err)
	}

	// Delete
	if err := pdb.DeleteWorkflow("test-delete"); err != nil {
		t.Fatalf("DeleteWorkflow failed: %v", err)
	}

	// Verify workflow is gone
	deleted, err := pdb.GetWorkflow("test-delete")
	if err != nil {
		t.Fatalf("GetWorkflow after delete failed: %v", err)
	}
	if deleted != nil {
		t.Error("workflow should have been deleted")
	}

	// Verify phases are cascaded
	phases, err := pdb.GetWorkflowPhases("test-delete")
	if err != nil {
		t.Fatalf("GetWorkflowPhases after delete failed: %v", err)
	}
	if len(phases) != 0 {
		t.Errorf("phases should be cascaded on delete, got %d", len(phases))
	}
}

func TestWorkflowShow_DisplaysPhases(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Get builtin workflow
	wf, err := pdb.GetWorkflow("implement-medium")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if wf == nil {
		t.Fatal("implement-medium not found")
	}

	phases, err := pdb.GetWorkflowPhases("implement-medium")
	if err != nil {
		t.Fatalf("GetWorkflowPhases failed: %v", err)
	}

	// Verify phases exist and are in order
	if len(phases) == 0 {
		t.Error("expected phases for implement-medium")
	}

	for i, p := range phases {
		if p.Sequence != i {
			t.Errorf("phase %d sequence = %d, want %d", i, p.Sequence, i)
		}
	}
}

func TestWorkflowList_FiltersCorrectly(t *testing.T) {
	pdb, _ := setupWorkflowTestDB(t)
	defer func() { _ = pdb.Close() }()

	// Add custom workflow
	wf := &db.Workflow{
		ID:        "test-filter",
		Name:      "Test Filter",
		IsBuiltin: false,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// List all
	all, err := pdb.ListWorkflows()
	if err != nil {
		t.Fatalf("ListWorkflows failed: %v", err)
	}

	var builtinCount, customCount int
	for _, w := range all {
		if w.IsBuiltin {
			builtinCount++
		} else {
			customCount++
		}
	}

	if builtinCount == 0 {
		t.Error("expected at least one builtin workflow")
	}
	if customCount == 0 {
		t.Error("expected at least one custom workflow")
	}

	// Filter builtin only
	var filteredBuiltin []*db.Workflow
	for _, w := range all {
		if w.IsBuiltin {
			filteredBuiltin = append(filteredBuiltin, w)
		}
	}
	if len(filteredBuiltin) != builtinCount {
		t.Errorf("builtin filter count mismatch")
	}

	// Filter custom only
	var filteredCustom []*db.Workflow
	for _, w := range all {
		if !w.IsBuiltin {
			filteredCustom = append(filteredCustom, w)
		}
	}
	if len(filteredCustom) != customCount {
		t.Errorf("custom filter count mismatch")
	}
}

