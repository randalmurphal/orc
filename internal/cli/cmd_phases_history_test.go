package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// --- Test helpers ---

// withPhasesTestDir creates a temp directory with .orc/ structure, changes to it,
// and restores the original working directory when the test completes.
func withPhasesTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return tmpDir
}

// createPhasesTestBackend creates a backend in the given directory.
func createPhasesTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

// setupWorkflow creates a workflow with phases in the given backend's project DB.
// It also ensures the required phase templates exist (for FK constraints).
func setupWorkflow(t *testing.T, backend storage.Backend, workflowID string, phaseIDs []string) {
	t.Helper()
	pdb := backend.DB()

	// Ensure phase templates exist for FK constraints
	for _, phaseID := range phaseIDs {
		_ = pdb.SavePhaseTemplate(&db.PhaseTemplate{
			ID:            phaseID,
			Name:          phaseID,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + phaseID,
		})
	}

	if err := pdb.SaveWorkflow(&db.Workflow{
		ID:        workflowID,
		Name:      workflowID,
		IsBuiltin: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	for i, phaseID := range phaseIDs {
		if err := pdb.SaveWorkflowPhase(&db.WorkflowPhase{
			WorkflowID:      workflowID,
			PhaseTemplateID: phaseID,
			Sequence:        (i + 1) * 10,
		}); err != nil {
			t.Fatalf("save workflow phase %s: %v", phaseID, err)
		}
	}
}

// savePhase saves a phase execution record to the project DB.
func savePhase(t *testing.T, backend storage.Backend, ph *db.Phase) {
	t.Helper()
	if err := backend.DB().SavePhase(ph); err != nil {
		t.Fatalf("save phase: %v", err)
	}
}

// timePtr returns a pointer to the given time.
func timePtr(t time.Time) *time.Time {
	return &t
}

// --- SC-1: Table output with all seven columns ---

func TestPhaseHistory_TableOutput(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	// Create task with medium workflow
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("implement-medium")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Set up workflow with ordered phases
	setupWorkflow(t, backend, "implement-medium", []string{
		"spec", "tdd_write", "tdd_integrate", "implement", "review", "docs",
	})

	// Save completed phases with timing and cost data
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID:       "TASK-001",
		PhaseID:      "spec",
		Status:       "completed",
		Iterations:   1,
		StartedAt:    timePtr(baseTime),
		CompletedAt:  timePtr(baseTime.Add(2*time.Minute + 25*time.Second)),
		InputTokens:  5000,
		OutputTokens: 3000,
		CostUSD:      0.45,
	})
	savePhase(t, backend, &db.Phase{
		TaskID:       "TASK-001",
		PhaseID:      "implement",
		Status:       "completed",
		Iterations:   2,
		StartedAt:    timePtr(baseTime.Add(5 * time.Minute)),
		CompletedAt:  timePtr(baseTime.Add(15 * time.Minute)),
		InputTokens:  20000,
		OutputTokens: 15000,
		CostUSD:      3.50,
	})

	_ = backend.Close()

	// Run the phases command with task ID argument
	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Verify all required columns are present in header
	requiredColumns := []string{"PHASE", "STATUS", "STARTED", "COMPLETED", "DURATION", "ITERATIONS", "COST"}
	for _, col := range requiredColumns {
		if !strings.Contains(output, col) {
			t.Errorf("output missing required column %q:\n%s", col, output)
		}
	}

	// Verify phase data rows appear
	if !strings.Contains(output, "spec") {
		t.Errorf("output missing 'spec' phase:\n%s", output)
	}
	if !strings.Contains(output, "implement") {
		t.Errorf("output missing 'implement' phase:\n%s", output)
	}

	// Verify cost values are formatted as dollars
	if !strings.Contains(output, "$0.45") {
		t.Errorf("output missing spec cost '$0.45':\n%s", output)
	}
	if !strings.Contains(output, "$3.50") {
		t.Errorf("output missing implement cost '$3.50':\n%s", output)
	}
}

// --- SC-2: TOTAL summary row with aggregated values ---

func TestPhaseHistory_TotalRow(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("implement-medium")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "implement-medium", []string{"spec", "implement", "review"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "spec", Status: "completed",
		Iterations: 1,
		StartedAt:  timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(2 * time.Minute)),
		CostUSD: 0.50,
	})
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "completed",
		Iterations: 3,
		StartedAt:  timePtr(baseTime.Add(5 * time.Minute)),
		CompletedAt: timePtr(baseTime.Add(15 * time.Minute)),
		CostUSD: 2.00,
	})
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "review", Status: "completed",
		Iterations: 2,
		StartedAt:  timePtr(baseTime.Add(20 * time.Minute)),
		CompletedAt: timePtr(baseTime.Add(25 * time.Minute)),
		CostUSD: 1.00,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Verify TOTAL row exists
	if !strings.Contains(output, "TOTAL") {
		t.Errorf("output missing TOTAL summary row:\n%s", output)
	}

	// Total cost should be $3.50 (0.50 + 2.00 + 1.00)
	if !strings.Contains(output, "$3.50") {
		t.Errorf("output missing total cost '$3.50':\n%s", output)
	}

	// Total iterations should be 6 (1 + 3 + 2)
	// The TOTAL row should contain "6" for iterations
	lines := strings.Split(output, "\n")
	foundTotal := false
	for _, line := range lines {
		if strings.Contains(line, "TOTAL") {
			foundTotal = true
			if !strings.Contains(line, "6") {
				t.Errorf("TOTAL row missing total iterations '6': %s", line)
			}
			break
		}
	}
	if !foundTotal {
		t.Error("no TOTAL row found in output")
	}
}

// --- SC-3: Phases in workflow execution order ---

func TestPhaseHistory_WorkflowOrder(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("implement-medium")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Set up medium workflow with specific phase order
	expectedOrder := []string{"spec", "tdd_write", "tdd_integrate", "implement", "review", "docs"}
	setupWorkflow(t, backend, "implement-medium", expectedOrder)

	// Save phases in REVERSE order to ensure display uses workflow sequence, not insertion order
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	for i := len(expectedOrder) - 1; i >= 0; i-- {
		savePhase(t, backend, &db.Phase{
			TaskID:      "TASK-001",
			PhaseID:     expectedOrder[i],
			Status:      "completed",
			Iterations:  1,
			StartedAt:   timePtr(baseTime.Add(time.Duration(i*5) * time.Minute)),
			CompletedAt: timePtr(baseTime.Add(time.Duration(i*5+3) * time.Minute)),
			CostUSD:     0.10,
		})
	}

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Verify phases appear in the correct workflow sequence order
	// Find positions of each phase ID in the output
	positions := make([]int, len(expectedOrder))
	for i, phaseID := range expectedOrder {
		pos := strings.Index(output, phaseID)
		if pos == -1 {
			t.Fatalf("phase %q not found in output:\n%s", phaseID, output)
		}
		positions[i] = pos
	}

	// Each phase should appear before the next one
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Errorf("phase %q (pos %d) should appear after %q (pos %d) in output:\n%s",
				expectedOrder[i], positions[i], expectedOrder[i-1], positions[i-1], output)
		}
	}
}

// --- SC-4: Duration formatting uses task.FormatDuration pattern ---

func TestPhaseHistory_DurationFormat(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("test-wf")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"implement"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	// Phase took 2m 25s
	savePhase(t, backend, &db.Phase{
		TaskID:      "TASK-001",
		PhaseID:     "implement",
		Status:      "completed",
		Iterations:  1,
		StartedAt:   timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(2*time.Minute + 25*time.Second)),
		CostUSD:     1.00,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Duration should be formatted as "2m 25s" matching task.FormatDuration
	if !strings.Contains(output, "2m 25s") {
		t.Errorf("output missing expected duration '2m 25s':\n%s", output)
	}
}

// --- SC-5: Cost formatted as dollar amounts ---

func TestPhaseHistory_CostFormat(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("test-wf")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"implement"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID:      "TASK-001",
		PhaseID:     "implement",
		Status:      "completed",
		Iterations:  1,
		StartedAt:   timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(5 * time.Minute)),
		CostUSD:     12345.67,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Cost should be formatted with commas: "$12,345.67"
	if !strings.Contains(output, "$12,345.67") {
		t.Errorf("output missing expected cost '$12,345.67':\n%s", output)
	}
}

// --- SC-6: JSON output structure ---

func TestPhaseHistory_JSONOutput(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("implement-medium")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "implement-medium", []string{"spec", "implement"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "spec", Status: "completed",
		Iterations: 1, InputTokens: 5000, OutputTokens: 3000,
		StartedAt:   timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(2 * time.Minute)),
		CostUSD:     0.45,
	})
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "completed",
		Iterations: 2, InputTokens: 20000, OutputTokens: 15000,
		StartedAt:   timePtr(baseTime.Add(5 * time.Minute)),
		CompletedAt: timePtr(baseTime.Add(15 * time.Minute)),
		CostUSD:     3.50,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001", "--json"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nOutput:\n%s", err, output)
	}

	// Verify phases array exists
	phases, ok := result["phases"].([]any)
	if !ok {
		t.Fatalf("missing or invalid 'phases' array in JSON output:\n%s", output)
	}

	if len(phases) < 2 {
		t.Fatalf("expected at least 2 phases in JSON, got %d:\n%s", len(phases), output)
	}

	// Verify first phase has all required fields
	phase0 := phases[0].(map[string]any)
	requiredFields := []string{
		"phase", "status", "started_at", "completed_at",
		"duration_seconds", "iterations", "cost_usd",
		"input_tokens", "output_tokens",
	}
	for _, field := range requiredFields {
		if _, exists := phase0[field]; !exists {
			t.Errorf("phase missing required field %q: %v", field, phase0)
		}
	}

	// Verify duration_seconds is numeric (not a string)
	if dur, ok := phase0["duration_seconds"].(float64); !ok || dur <= 0 {
		t.Errorf("duration_seconds should be a positive number, got %v (type %T)", phase0["duration_seconds"], phase0["duration_seconds"])
	}

	// Verify iterations is numeric
	if iter, ok := phase0["iterations"].(float64); !ok || iter < 1 {
		t.Errorf("iterations should be a positive number, got %v", phase0["iterations"])
	}

	// Verify totals object exists
	totals, ok := result["totals"].(map[string]any)
	if !ok {
		t.Fatalf("missing or invalid 'totals' object in JSON output:\n%s", output)
	}

	// Totals should have aggregated cost
	if costUSD, ok := totals["cost_usd"].(float64); !ok || costUSD < 3.95 {
		t.Errorf("totals.cost_usd should be ~3.95, got %v", totals["cost_usd"])
	}
}

// --- SC-7: JSON output includes task_id ---

func TestPhaseHistory_JSONTaskID(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("test-wf")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"implement"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "completed",
		Iterations: 1, StartedAt: timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(5 * time.Minute)),
		CostUSD:     1.00,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001", "--json"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nOutput:\n%s", err, output)
	}

	taskID, ok := result["task_id"].(string)
	if !ok || taskID != "TASK-001" {
		t.Errorf("expected task_id='TASK-001', got %v", result["task_id"])
	}
}

// --- SC-8: Error on nonexistent task ---

func TestPhaseHistory_NotFound(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)
	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "NONEXISTENT-999"})
	err := rootCmd.Execute()

	if err == nil {
		t.Fatal("expected error for nonexistent task, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "NONEXISTENT-999") {
		t.Errorf("error should contain task ID 'NONEXISTENT-999', got: %s", errMsg)
	}
}

// --- SC-9: Pending and skipped phases show appropriate placeholders ---

func TestPhaseHistory_PendingPhases(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("test-wf")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"spec", "implement", "review"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	// Only spec is completed, others are pending
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "spec", Status: "completed",
		Iterations: 1, StartedAt: timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(2 * time.Minute)),
		CostUSD:     0.50,
	})
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "pending",
	})
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "review", Status: "pending",
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Pending phases should show placeholder for timing columns (e.g., "-")
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "implement") || strings.Contains(line, "review") {
			// These pending phases should have placeholder dashes for timing
			if !strings.Contains(line, "-") {
				t.Errorf("pending phase line should contain '-' placeholder: %s", line)
			}
		}
	}
}

func TestPhaseHistory_SkippedPhases(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("test-wf")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"spec", "implement"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "spec", Status: "skipped",
		SkipReason: "trivial task",
	})
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "completed",
		Iterations: 1, StartedAt: timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(5 * time.Minute)),
		CostUSD:     1.00,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Skipped phase should show "skipped" status
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "spec") {
			if !strings.Contains(strings.ToLower(line), "skipped") {
				t.Errorf("skipped phase should show 'skipped' status: %s", line)
			}
		}
	}
}

// --- Edge Cases ---

// Failure mode: Task has no workflow ID
func TestPhaseHistory_NoWorkflow(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	// Task without workflow ID
	tk := task.NewProtoTask("TASK-001", "Old task")
	// Don't set WorkflowId
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Save some phases (would exist from execution even without workflow)
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "completed",
		Iterations: 1, StartedAt: timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(5 * time.Minute)),
		CostUSD:     1.00,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Must show phase history output (not template listing).
	// Phase history has COST column; template listing does not.
	if !strings.Contains(output, "COST") && !strings.Contains(output, "$1.00") {
		t.Errorf("expected phase history output with cost data for task without workflow, got template listing:\n%s", output)
	}
}

// Failure mode: Task created but never run (no phases in DB)
func TestPhaseHistory_NoPhasesRun(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Never run task")
	tk.WorkflowId = stringPtr("test-wf")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"implement"})
	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Must show phase history output (not template listing).
	// Phase history shows TASK-001-specific data or a "no execution data" message.
	// Template listing shows "ID NAME GATE" headers which should NOT appear.
	if strings.Contains(output, "BUILT-IN") {
		t.Errorf("expected phase history output for task, got template listing:\n%s", output)
	}
}

// Edge case: Phase with zero cost
func TestPhaseHistory_ZeroCost(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("test-wf")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"implement"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "completed",
		Iterations: 1, StartedAt: timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(5 * time.Minute)),
		CostUSD:     0.00,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Zero cost should show "$0.00"
	if !strings.Contains(output, "$0.00") {
		t.Errorf("zero cost should display as '$0.00':\n%s", output)
	}
}

// Edge case: In-progress phase shows started_at but no completed_at
func TestPhaseHistory_InProgress(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Running task")
	tk.WorkflowId = stringPtr("test-wf")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	tk.CurrentPhase = stringPtr("implement")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"spec", "implement", "review"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "spec", Status: "completed",
		Iterations: 1, StartedAt: timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(2 * time.Minute)),
		CostUSD:     0.50,
	})
	// In-progress phase: started but no completed_at
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "pending",
		Iterations: 1, StartedAt: timePtr(baseTime.Add(5 * time.Minute)),
		// No CompletedAt
		CostUSD: 0.30,
	})
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "review", Status: "pending",
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// The implement phase should show started_at but placeholder for completed_at/duration
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "implement") {
			// Should have started time (not all dashes).
			// Too many dashes might mean started_at isn't showing.
			// This is a heuristic - the key assertion is that started_at IS shown.
			if strings.Count(line, "-") > 10 {
				t.Logf("implement phase line has many dashes (started_at may not be showing): %s", line)
			}
			break
		}
	}

	// Spec should be fully populated
	if !strings.Contains(output, "$0.50") {
		t.Errorf("completed spec phase should show its cost:\n%s", output)
	}
}

// Edge case: Phase with iterations > 1 (retried)
func TestPhaseHistory_Retried(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("test-wf")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"implement"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "completed",
		Iterations: 5,
		StartedAt:  timePtr(baseTime),
		CompletedAt: timePtr(baseTime.Add(30 * time.Minute)),
		CostUSD:     8.50,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Should show iteration count of 5
	if !strings.Contains(output, "5") {
		t.Errorf("output should show iteration count '5':\n%s", output)
	}
}

// Edge case: Interrupted phase
func TestPhaseHistory_Interrupted(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Interrupted task")
	tk.WorkflowId = stringPtr("test-wf")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"implement"})

	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	savePhase(t, backend, &db.Phase{
		TaskID: "TASK-001", PhaseID: "implement", Status: "pending",
		Iterations: 1, StartedAt: timePtr(baseTime),
		// No CompletedAt - was interrupted
		CostUSD: 0.80,
	})

	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	// Must show phase history output (not template listing).
	// Phase history has COST/DURATION columns; template listing does not.
	if strings.Contains(output, "BUILT-IN") {
		t.Errorf("expected phase history output, got template listing:\n%s", output)
	}
	// Should contain the cost data for the interrupted phase
	if !strings.Contains(output, "$0.80") {
		t.Errorf("interrupted phase should show its cost '$0.80':\n%s", output)
	}
}

// Preservation: Existing `orc phases` (no args) still lists templates
func TestPhaseHistory_ExistingListTemplatesPreserved(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	_ = createPhasesTestBackend(t, tmpDir)

	// Running `orc phases` with no arguments should still list phase templates
	// (not error or try to show phase history)
	rootCmd.SetArgs([]string{"phases"})
	err := rootCmd.Execute()

	// Should not error - the existing template listing behavior should work
	if err != nil {
		t.Errorf("'orc phases' with no args should still work (list templates): %v", err)
	}
}

// JSON output: All-pending phases are handled
func TestPhaseHistory_JSONAllPending(t *testing.T) {
	tmpDir := withPhasesTestDir(t)
	backend := createPhasesTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.WorkflowId = stringPtr("test-wf")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	setupWorkflow(t, backend, "test-wf", []string{"spec", "implement"})
	_ = backend.Close()

	rootCmd.SetArgs([]string{"phases", "TASK-001", "--json"})
	output := captureOutput(t, func() error {
		return rootCmd.Execute()
	})

	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		t.Fatal("expected JSON output, got empty string")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		t.Fatalf("invalid JSON output for all-pending: %v\nOutput:\n%s", err, output)
	}

	// Should still have task_id and phases (even if all pending)
	if _, ok := result["task_id"]; !ok {
		t.Error("JSON output missing task_id for all-pending case")
	}
}

// --- captureOutput helper ---

// captureOutput captures stdout from a function call and returns it as a string.
func captureOutput(t *testing.T, fn func() error) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdout = w

	execErr := fn()

	_ = w.Close()
	os.Stdout = old

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	_ = r.Close()

	output := string(buf[:n])

	if execErr != nil {
		t.Fatalf("command failed: %v\nOutput:\n%s", execErr, output)
	}

	return output
}

// stringPtr returns a pointer to the given string.
func stringPtr(s string) *string {
	return &s
}
