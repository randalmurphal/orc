package db

import (
	"testing"
	"time"
)

func TestWorkflowPhasePositionFields(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create phase template
	pt := &PhaseTemplate{
		ID:            "pos-phase-1",
		Name:          "Position Phase",
		PromptSource:  "embedded",
		PromptPath:    "prompts/p1.md",
		MaxIterations: 10,
		GateType:      "auto",
		IsBuiltin:     false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := pdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	// Create workflow
	wf := &Workflow{
		ID:           "pos-wf",
		Name:         "Position Test Workflow",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Save phase WITH position coordinates
	x := 100.5
	y := 200.75
	wp := &WorkflowPhase{
		WorkflowID:      "pos-wf",
		PhaseTemplateID: "pos-phase-1",
		Sequence:        0,
		DependsOn:       "[]",
		PositionX:       &x,
		PositionY:       &y,
	}
	if err := pdb.SaveWorkflowPhase(wp); err != nil {
		t.Fatalf("SaveWorkflowPhase with positions failed: %v", err)
	}

	// Retrieve and verify positions round-trip
	phases, err := pdb.GetWorkflowPhases("pos-wf")
	if err != nil {
		t.Fatalf("GetWorkflowPhases failed: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}

	got := phases[0]
	if got.PositionX == nil || *got.PositionX != 100.5 {
		t.Errorf("PositionX: want 100.5, got %v", got.PositionX)
	}
	if got.PositionY == nil || *got.PositionY != 200.75 {
		t.Errorf("PositionY: want 200.75, got %v", got.PositionY)
	}
}

func TestWorkflowPhaseNullPositions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	pt := &PhaseTemplate{
		ID:            "null-pos-phase",
		Name:          "Null Position Phase",
		PromptSource:  "embedded",
		PromptPath:    "prompts/p1.md",
		MaxIterations: 10,
		GateType:      "auto",
		IsBuiltin:     false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := pdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	wf := &Workflow{
		ID:           "null-pos-wf",
		Name:         "Null Position Workflow",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Save phase WITHOUT positions (NULL = auto-layout)
	wp := &WorkflowPhase{
		WorkflowID:      "null-pos-wf",
		PhaseTemplateID: "null-pos-phase",
		Sequence:        0,
		DependsOn:       "[]",
		// PositionX and PositionY intentionally nil
	}
	if err := pdb.SaveWorkflowPhase(wp); err != nil {
		t.Fatalf("SaveWorkflowPhase with null positions failed: %v", err)
	}

	phases, err := pdb.GetWorkflowPhases("null-pos-wf")
	if err != nil {
		t.Fatalf("GetWorkflowPhases failed: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}

	got := phases[0]
	if got.PositionX != nil {
		t.Errorf("PositionX: want nil (auto-layout), got %v", *got.PositionX)
	}
	if got.PositionY != nil {
		t.Errorf("PositionY: want nil (auto-layout), got %v", *got.PositionY)
	}
}

func TestWorkflowPhaseDuplicateSequenceAllowed(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create two phase templates
	for _, id := range []string{"dup-seq-1", "dup-seq-2"} {
		pt := &PhaseTemplate{
			ID:            id,
			Name:          id,
			PromptSource:  "embedded",
			PromptPath:    "prompts/p.md",
			MaxIterations: 10,
			GateType:      "auto",
			IsBuiltin:     false,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := pdb.SavePhaseTemplate(pt); err != nil {
			t.Fatalf("SavePhaseTemplate %s failed: %v", id, err)
		}
	}

	wf := &Workflow{
		ID:           "dup-seq-wf",
		Name:         "Duplicate Sequence Workflow",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Both phases share sequence=0 (parallel phases)
	wp1 := &WorkflowPhase{
		WorkflowID:      "dup-seq-wf",
		PhaseTemplateID: "dup-seq-1",
		Sequence:        0,
		DependsOn:       "[]",
	}
	wp2 := &WorkflowPhase{
		WorkflowID:      "dup-seq-wf",
		PhaseTemplateID: "dup-seq-2",
		Sequence:        0, // Same sequence - should be allowed after migration
		DependsOn:       "[]",
	}

	if err := pdb.SaveWorkflowPhase(wp1); err != nil {
		t.Fatalf("SaveWorkflowPhase 1 failed: %v", err)
	}
	if err := pdb.SaveWorkflowPhase(wp2); err != nil {
		t.Fatalf("SaveWorkflowPhase 2 failed (duplicate sequence should be allowed): %v", err)
	}

	phases, err := pdb.GetWorkflowPhases("dup-seq-wf")
	if err != nil {
		t.Fatalf("GetWorkflowPhases failed: %v", err)
	}
	if len(phases) != 2 {
		t.Errorf("expected 2 phases with same sequence, got %d", len(phases))
	}
}

func TestUpdateWorkflowPhasePositions(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create two phase templates
	for _, id := range []string{"bulk-pos-1", "bulk-pos-2"} {
		pt := &PhaseTemplate{
			ID:            id,
			Name:          id,
			PromptSource:  "embedded",
			PromptPath:    "prompts/p.md",
			MaxIterations: 10,
			GateType:      "auto",
			IsBuiltin:     false,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := pdb.SavePhaseTemplate(pt); err != nil {
			t.Fatalf("SavePhaseTemplate %s failed: %v", id, err)
		}
	}

	wf := &Workflow{
		ID:           "bulk-pos-wf",
		Name:         "Bulk Position Workflow",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Create phases without positions
	for i, id := range []string{"bulk-pos-1", "bulk-pos-2"} {
		wp := &WorkflowPhase{
			WorkflowID:      "bulk-pos-wf",
			PhaseTemplateID: id,
			Sequence:        i,
			DependsOn:       "[]",
		}
		if err := pdb.SaveWorkflowPhase(wp); err != nil {
			t.Fatalf("SaveWorkflowPhase %s failed: %v", id, err)
		}
	}

	// Bulk update positions (keyed by phase_template_id)
	positions := map[string][2]float64{
		"bulk-pos-1": {50.0, 100.0},
		"bulk-pos-2": {250.0, 100.0},
	}
	if err := pdb.UpdateWorkflowPhasePositions("bulk-pos-wf", positions); err != nil {
		t.Fatalf("UpdateWorkflowPhasePositions failed: %v", err)
	}

	// Verify positions were saved
	phases, err := pdb.GetWorkflowPhases("bulk-pos-wf")
	if err != nil {
		t.Fatalf("GetWorkflowPhases failed: %v", err)
	}

	for _, p := range phases {
		expected, ok := positions[p.PhaseTemplateID]
		if !ok {
			t.Errorf("unexpected phase %s", p.PhaseTemplateID)
			continue
		}
		if p.PositionX == nil || *p.PositionX != expected[0] {
			t.Errorf("phase %s PositionX: want %f, got %v", p.PhaseTemplateID, expected[0], p.PositionX)
		}
		if p.PositionY == nil || *p.PositionY != expected[1] {
			t.Errorf("phase %s PositionY: want %f, got %v", p.PhaseTemplateID, expected[1], p.PositionY)
		}
	}
}
