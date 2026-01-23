package db

import (
	"testing"
	"time"
)

func TestPhaseTemplateCRUD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create
	pt := &PhaseTemplate{
		ID:               "test-phase",
		Name:             "Test Phase",
		Description:      "A test phase template",
		PromptSource:     "embedded",
		PromptPath:       "prompts/test.md",
		InputVariables:   `["VAR1", "VAR2"]`,
		ProducesArtifact: true,
		ArtifactType:     "test",
		MaxIterations:    10,
		GateType:         "auto",
		Checkpoint:       true,
		IsBuiltin:        false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	err = pdb.SavePhaseTemplate(pt)
	if err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	// Read
	got, err := pdb.GetPhaseTemplate("test-phase")
	if err != nil {
		t.Fatalf("GetPhaseTemplate failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetPhaseTemplate returned nil")
	}
	if got.ID != pt.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, pt.ID)
	}
	if got.Name != pt.Name {
		t.Errorf("Name mismatch: got %s, want %s", got.Name, pt.Name)
	}
	if got.MaxIterations != pt.MaxIterations {
		t.Errorf("MaxIterations mismatch: got %d, want %d", got.MaxIterations, pt.MaxIterations)
	}

	// List
	all, err := pdb.ListPhaseTemplates()
	if err != nil {
		t.Fatalf("ListPhaseTemplates failed: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 phase template, got %d", len(all))
	}

	// Update
	pt.Name = "Updated Test Phase"
	pt.MaxIterations = 20
	err = pdb.SavePhaseTemplate(pt)
	if err != nil {
		t.Fatalf("SavePhaseTemplate (update) failed: %v", err)
	}

	got, err = pdb.GetPhaseTemplate("test-phase")
	if err != nil {
		t.Fatalf("GetPhaseTemplate after update failed: %v", err)
	}
	if got.Name != "Updated Test Phase" {
		t.Errorf("Name not updated: got %s", got.Name)
	}
	if got.MaxIterations != 20 {
		t.Errorf("MaxIterations not updated: got %d", got.MaxIterations)
	}

	// Delete
	err = pdb.DeletePhaseTemplate("test-phase")
	if err != nil {
		t.Fatalf("DeletePhaseTemplate failed: %v", err)
	}

	got, err = pdb.GetPhaseTemplate("test-phase")
	if err != nil {
		t.Fatalf("GetPhaseTemplate after delete failed: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestWorkflowCRUD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create
	wf := &Workflow{
		ID:           "test-workflow",
		Name:         "Test Workflow",
		Description:  "A test workflow",
		WorkflowType: "task",
		IsBuiltin:    false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err = pdb.SaveWorkflow(wf)
	if err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Read
	got, err := pdb.GetWorkflow("test-workflow")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetWorkflow returned nil")
	}
	if got.ID != wf.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, wf.ID)
	}
	if got.Name != wf.Name {
		t.Errorf("Name mismatch: got %s, want %s", got.Name, wf.Name)
	}

	// List
	all, err := pdb.ListWorkflows()
	if err != nil {
		t.Fatalf("ListWorkflows failed: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(all))
	}

	// Update
	wf.Name = "Updated Test Workflow"
	err = pdb.SaveWorkflow(wf)
	if err != nil {
		t.Fatalf("SaveWorkflow (update) failed: %v", err)
	}

	got, err = pdb.GetWorkflow("test-workflow")
	if err != nil {
		t.Fatalf("GetWorkflow after update failed: %v", err)
	}
	if got.Name != "Updated Test Workflow" {
		t.Errorf("Name not updated: got %s", got.Name)
	}

	// Delete
	err = pdb.DeleteWorkflow("test-workflow")
	if err != nil {
		t.Fatalf("DeleteWorkflow failed: %v", err)
	}

	got, err = pdb.GetWorkflow("test-workflow")
	if err != nil {
		t.Fatalf("GetWorkflow after delete failed: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestWorkflowPhases(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create phase templates first
	pt1 := &PhaseTemplate{
		ID:            "phase-1",
		Name:          "Phase 1",
		PromptSource:  "embedded",
		PromptPath:    "prompts/p1.md",
		MaxIterations: 10,
		GateType:      "auto",
		IsBuiltin:     false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	pt2 := &PhaseTemplate{
		ID:            "phase-2",
		Name:          "Phase 2",
		PromptSource:  "embedded",
		PromptPath:    "prompts/p2.md",
		MaxIterations: 10,
		GateType:      "auto",
		IsBuiltin:     false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := pdb.SavePhaseTemplate(pt1); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}
	if err := pdb.SavePhaseTemplate(pt2); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	// Create workflow
	wf := &Workflow{
		ID:           "test-wf",
		Name:         "Test Workflow",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Add phases
	wp1 := &WorkflowPhase{
		WorkflowID:      "test-wf",
		PhaseTemplateID: "phase-1",
		Sequence:        0,
		DependsOn:       "[]",
	}
	wp2 := &WorkflowPhase{
		WorkflowID:      "test-wf",
		PhaseTemplateID: "phase-2",
		Sequence:        1,
		DependsOn:       `["phase-1"]`,
	}

	if err := pdb.SaveWorkflowPhase(wp1); err != nil {
		t.Fatalf("SaveWorkflowPhase failed: %v", err)
	}
	if err := pdb.SaveWorkflowPhase(wp2); err != nil {
		t.Fatalf("SaveWorkflowPhase failed: %v", err)
	}

	// Get phases
	phases, err := pdb.GetWorkflowPhases("test-wf")
	if err != nil {
		t.Fatalf("GetWorkflowPhases failed: %v", err)
	}
	if len(phases) != 2 {
		t.Errorf("expected 2 phases, got %d", len(phases))
	}

	// Verify order
	if phases[0].Sequence != 0 || phases[1].Sequence != 1 {
		t.Error("phases not in expected order")
	}

	// Delete phase
	err = pdb.DeleteWorkflowPhase("test-wf", "phase-1")
	if err != nil {
		t.Fatalf("DeleteWorkflowPhase failed: %v", err)
	}

	phases, err = pdb.GetWorkflowPhases("test-wf")
	if err != nil {
		t.Fatalf("GetWorkflowPhases after delete failed: %v", err)
	}
	if len(phases) != 1 {
		t.Errorf("expected 1 phase after delete, got %d", len(phases))
	}
}

func TestWorkflowVariables(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create workflow first
	wf := &Workflow{
		ID:           "var-test-wf",
		Name:         "Variable Test Workflow",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Add variables
	wv1 := &WorkflowVariable{
		WorkflowID:   "var-test-wf",
		Name:         "CUSTOM_VAR",
		Description:  "A custom variable",
		SourceType:   "static",
		SourceConfig: `{"value": "test-value"}`,
		Required:     true,
	}
	wv2 := &WorkflowVariable{
		WorkflowID:      "var-test-wf",
		Name:            "ENV_VAR",
		SourceType:      "env",
		SourceConfig:    `{"var": "MY_VAR", "default": "fallback"}`,
		Required:        false,
		DefaultValue:    "fallback",
		CacheTTLSeconds: 300,
	}

	if err := pdb.SaveWorkflowVariable(wv1); err != nil {
		t.Fatalf("SaveWorkflowVariable failed: %v", err)
	}
	if err := pdb.SaveWorkflowVariable(wv2); err != nil {
		t.Fatalf("SaveWorkflowVariable failed: %v", err)
	}

	// Get variables
	vars, err := pdb.GetWorkflowVariables("var-test-wf")
	if err != nil {
		t.Fatalf("GetWorkflowVariables failed: %v", err)
	}
	if len(vars) != 2 {
		t.Errorf("expected 2 variables, got %d", len(vars))
	}

	// Check values
	found := make(map[string]bool)
	for _, v := range vars {
		found[v.Name] = true
		if v.Name == "CUSTOM_VAR" && !v.Required {
			t.Error("CUSTOM_VAR should be required")
		}
		if v.Name == "ENV_VAR" && v.CacheTTLSeconds != 300 {
			t.Errorf("ENV_VAR CacheTTLSeconds mismatch: got %d", v.CacheTTLSeconds)
		}
	}

	if !found["CUSTOM_VAR"] {
		t.Error("CUSTOM_VAR not found")
	}
	if !found["ENV_VAR"] {
		t.Error("ENV_VAR not found")
	}

	// Delete variable
	err = pdb.DeleteWorkflowVariable("var-test-wf", "CUSTOM_VAR")
	if err != nil {
		t.Fatalf("DeleteWorkflowVariable failed: %v", err)
	}

	vars, err = pdb.GetWorkflowVariables("var-test-wf")
	if err != nil {
		t.Fatalf("GetWorkflowVariables after delete failed: %v", err)
	}
	if len(vars) != 1 {
		t.Errorf("expected 1 variable after delete, got %d", len(vars))
	}
}

func TestWorkflowRunCRUD(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create workflow first (required for foreign key)
	wf := &Workflow{
		ID:           "run-test-wf",
		Name:         "Run Test Workflow",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Create run (note: TaskID references tasks table which may not exist in test,
	// so we use NULL for TaskID to avoid FK constraint)
	run := &WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "run-test-wf",
		ContextType: "standalone", // Use standalone to avoid task FK
		ContextData: `{"prompt": "Do something"}`,
		TaskID:      nil, // No task reference
		Status:      "pending",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = pdb.SaveWorkflowRun(run)
	if err != nil {
		t.Fatalf("SaveWorkflowRun failed: %v", err)
	}

	// Read
	got, err := pdb.GetWorkflowRun("RUN-001")
	if err != nil {
		t.Fatalf("GetWorkflowRun failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetWorkflowRun returned nil")
	}
	if got.ID != run.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, run.ID)
	}
	if got.Status != "pending" {
		t.Errorf("Status mismatch: got %s, want pending", got.Status)
	}

	// List with filter
	runs, err := pdb.ListWorkflowRuns(WorkflowRunListOpts{
		WorkflowID: "run-test-wf",
	})
	if err != nil {
		t.Fatalf("ListWorkflowRuns failed: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("expected 1 run, got %d", len(runs))
	}

	// Update status
	run.Status = "running"
	startedAt := time.Now()
	run.StartedAt = &startedAt
	run.CurrentPhase = "spec"
	err = pdb.SaveWorkflowRun(run)
	if err != nil {
		t.Fatalf("SaveWorkflowRun (update) failed: %v", err)
	}

	got, err = pdb.GetWorkflowRun("RUN-001")
	if err != nil {
		t.Fatalf("GetWorkflowRun after update failed: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("Status not updated: got %s", got.Status)
	}
	if got.CurrentPhase != "spec" {
		t.Errorf("CurrentPhase not updated: got %s", got.CurrentPhase)
	}

	// Get next ID
	nextID, err := pdb.GetNextWorkflowRunID()
	if err != nil {
		t.Fatalf("GetNextWorkflowRunID failed: %v", err)
	}
	if nextID != "RUN-002" {
		t.Errorf("expected RUN-002, got %s", nextID)
	}

	// Delete
	err = pdb.DeleteWorkflowRun("RUN-001")
	if err != nil {
		t.Fatalf("DeleteWorkflowRun failed: %v", err)
	}

	got, err = pdb.GetWorkflowRun("RUN-001")
	if err != nil {
		t.Fatalf("GetWorkflowRun after delete failed: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestWorkflowRunPhases(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create phase templates first (required for foreign key)
	specPt := &PhaseTemplate{
		ID:            "spec",
		Name:          "Spec",
		PromptSource:  "embedded",
		PromptPath:    "prompts/spec.md",
		MaxIterations: 10,
		GateType:      "auto",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	implPt := &PhaseTemplate{
		ID:            "implement",
		Name:          "Implement",
		PromptSource:  "embedded",
		PromptPath:    "prompts/implement.md",
		MaxIterations: 20,
		GateType:      "auto",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := pdb.SavePhaseTemplate(specPt); err != nil {
		t.Fatalf("SavePhaseTemplate(spec) failed: %v", err)
	}
	if err := pdb.SavePhaseTemplate(implPt); err != nil {
		t.Fatalf("SavePhaseTemplate(implement) failed: %v", err)
	}

	// Setup workflow and run
	wf := &Workflow{
		ID:           "phase-run-wf",
		Name:         "Phase Run Test",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	run := &WorkflowRun{
		ID:          "RUN-PHASE-001",
		WorkflowID:  "phase-run-wf",
		ContextType: "standalone",
		ContextData: `{"prompt": "Test"}`,
		Status:      "running",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := pdb.SaveWorkflowRun(run); err != nil {
		t.Fatalf("SaveWorkflowRun failed: %v", err)
	}

	// Add run phases
	rp1 := &WorkflowRunPhase{
		WorkflowRunID:   "RUN-PHASE-001",
		PhaseTemplateID: "spec",
		Status:          "completed",
		Iterations:      3,
		InputTokens:     1000,
		OutputTokens:    500,
		CostUSD:         0.05,
	}
	rp2 := &WorkflowRunPhase{
		WorkflowRunID:   "RUN-PHASE-001",
		PhaseTemplateID: "implement",
		Status:          "running",
		Iterations:      1,
	}

	if err := pdb.SaveWorkflowRunPhase(rp1); err != nil {
		t.Fatalf("SaveWorkflowRunPhase failed: %v", err)
	}
	if err := pdb.SaveWorkflowRunPhase(rp2); err != nil {
		t.Fatalf("SaveWorkflowRunPhase failed: %v", err)
	}

	// Get run phases
	phases, err := pdb.GetWorkflowRunPhases("RUN-PHASE-001")
	if err != nil {
		t.Fatalf("GetWorkflowRunPhases failed: %v", err)
	}
	if len(phases) != 2 {
		t.Errorf("expected 2 run phases, got %d", len(phases))
	}

	// Verify data
	for _, p := range phases {
		if p.PhaseTemplateID == "spec" {
			if p.Status != "completed" {
				t.Errorf("spec status mismatch: got %s", p.Status)
			}
			if p.Iterations != 3 {
				t.Errorf("spec iterations mismatch: got %d", p.Iterations)
			}
			if p.CostUSD != 0.05 {
				t.Errorf("spec cost mismatch: got %f", p.CostUSD)
			}
		}
	}

	// Update run phase
	rp2.Status = "completed"
	rp2.Iterations = 5
	if err := pdb.SaveWorkflowRunPhase(rp2); err != nil {
		t.Fatalf("SaveWorkflowRunPhase (update) failed: %v", err)
	}

	phases, err = pdb.GetWorkflowRunPhases("RUN-PHASE-001")
	if err != nil {
		t.Fatalf("GetWorkflowRunPhases after update failed: %v", err)
	}

	for _, p := range phases {
		if p.PhaseTemplateID == "implement" {
			if p.Status != "completed" {
				t.Errorf("implement status not updated: got %s", p.Status)
			}
			if p.Iterations != 5 {
				t.Errorf("implement iterations not updated: got %d", p.Iterations)
			}
		}
	}
}

func TestListWorkflowRunsFiltering(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	now := time.Now()

	// Create workflow
	wf := &Workflow{
		ID:           "filter-wf",
		Name:         "Filter Test",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	// Create multiple runs with different statuses
	// Note: Use standalone context type and nil TaskID to avoid FK constraint on tasks table
	runs := []*WorkflowRun{
		{ID: "FRUN-001", WorkflowID: "filter-wf", ContextType: "standalone", ContextData: `{"task_id": "TASK-001"}`, Status: "pending", TaskID: nil, CreatedAt: now, UpdatedAt: now},
		{ID: "FRUN-002", WorkflowID: "filter-wf", ContextType: "standalone", ContextData: `{"task_id": "TASK-002"}`, Status: "running", TaskID: nil, CreatedAt: now, UpdatedAt: now},
		{ID: "FRUN-003", WorkflowID: "filter-wf", ContextType: "branch", ContextData: "{}", Status: "completed", TaskID: nil, CreatedAt: now, UpdatedAt: now},
		{ID: "FRUN-004", WorkflowID: "filter-wf", ContextType: "standalone", ContextData: `{"task_id": "TASK-003"}`, Status: "failed", TaskID: nil, CreatedAt: now, UpdatedAt: now},
	}

	for _, r := range runs {
		if err := pdb.SaveWorkflowRun(r); err != nil {
			t.Fatalf("SaveWorkflowRun failed: %v", err)
		}
	}

	// Test status filter
	result, err := pdb.ListWorkflowRuns(WorkflowRunListOpts{Status: "running"})
	if err != nil {
		t.Fatalf("ListWorkflowRuns with status filter failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 running run, got %d", len(result))
	}

	// Test workflow_id filter
	result, err = pdb.ListWorkflowRuns(WorkflowRunListOpts{WorkflowID: "filter-wf"})
	if err != nil {
		t.Fatalf("ListWorkflowRuns with workflow_id filter failed: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("expected 4 runs for filter-wf, got %d", len(result))
	}

	// Test limit
	result, err = pdb.ListWorkflowRuns(WorkflowRunListOpts{Limit: 2})
	if err != nil {
		t.Fatalf("ListWorkflowRuns with limit failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 runs with limit, got %d", len(result))
	}

	// Test no filter (all)
	result, err = pdb.ListWorkflowRuns(WorkflowRunListOpts{})
	if err != nil {
		t.Fatalf("ListWorkflowRuns without filter failed: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("expected 4 runs total, got %d", len(result))
	}
}
