package executor

import (
	"context"
	"log/slog"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// createTestWorkflow creates a workflow record in the backend so that
// workflow runs can reference it (FK constraint).
func createTestWorkflow(t *testing.T, backend storage.Backend, id string) {
	t.Helper()
	wf := &db.Workflow{
		ID:           id,
		Name:         id,
		WorkflowType: "task",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := backend.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
}

// TestPhaseUpdatesTaskCurrentPhase verifies SC-1: executor updates task.CurrentPhase
// when each phase starts. After the executor begins a phase, the task saved to the
// backend must have CurrentPhase set to the phase template ID.
func TestPhaseUpdatesTaskCurrentPhase(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create and save a task
	tsk := task.NewProtoTask("TASK-001", "Test task for phase tracking")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Verify CurrentPhase starts empty
	loaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.GetCurrentPhaseProto(loaded) != "" {
		t.Fatalf("expected empty CurrentPhase initially, got %q", task.GetCurrentPhaseProto(loaded))
	}

	// Set up a minimal WorkflowExecutor
	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Timeouts: config.TimeoutsConfig{
				PhaseMax: 0, // No timeout
			},
		},
		logger:   slog.Default(),
		resolver: variable.NewResolver("/tmp"),
		task:     tsk,
	}

	// Create workflow (FK parent) and workflow run
	createTestWorkflow(t, backend, "test-workflow")
	taskID := "TASK-001"
	run := &db.WorkflowRun{
		ID:         "RUN-001",
		WorkflowID: "test-workflow",
		TaskID:     &taskID,
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save workflow run: %v", err)
	}

	// Create phase template and workflow phase
	tmpl := &db.PhaseTemplate{
		ID:   "implement",
		Name: "Implement",
	}
	phase := &db.WorkflowPhase{
		PhaseTemplateID: "implement",
	}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "RUN-001",
		PhaseTemplateID: "implement",
	}

	// Use mock that returns a complete response
	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we.turnExecutor = mockTE

	ctx := context.Background()
	// Execute the phase — the exact result/error isn't the focus here;
	// we're testing that CurrentPhase was persisted on the task BEFORE execution.
	_, _ = we.executePhaseWithTimeout(ctx, tmpl, phase, map[string]string{}, nil, run, runPhase, tsk)

	// Reload the task from the backend and verify CurrentPhase was set
	reloaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	currentPhase := task.GetCurrentPhaseProto(reloaded)
	if currentPhase != "implement" {
		t.Errorf("task.CurrentPhase = %q, want %q", currentPhase, "implement")
	}
}

// TestPhaseUpdatesTaskCurrentPhase_MultiplePhases verifies SC-1 across multiple
// phases: each phase transition updates CurrentPhase to the new phase's template ID.
func TestPhaseUpdatesTaskCurrentPhase_MultiplePhases(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-002", "Multi-phase task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Timeouts: config.TimeoutsConfig{
				PhaseMax: 0,
			},
		},
		logger:   slog.Default(),
		resolver: variable.NewResolver("/tmp"),
		task:     tsk,
	}

	createTestWorkflow(t, backend, "test-workflow")
	taskID := "TASK-002"
	run := &db.WorkflowRun{
		ID:         "RUN-002",
		WorkflowID: "test-workflow",
		TaskID:     &taskID,
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save workflow run: %v", err)
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we.turnExecutor = mockTE

	phases := []string{"spec", "tdd_write", "implement", "review"}

	for _, phaseID := range phases {
		tmpl := &db.PhaseTemplate{ID: phaseID, Name: phaseID}
		phase := &db.WorkflowPhase{PhaseTemplateID: phaseID}
		runPhase := &db.WorkflowRunPhase{
			WorkflowRunID:   "RUN-002",
			PhaseTemplateID: phaseID,
		}

		// Update run's current phase (simulating what the executor loop does)
		run.CurrentPhase = phaseID
		_, _ = we.executePhaseWithTimeout(context.Background(), tmpl, phase, map[string]string{}, nil, run, runPhase, tsk)

		// After each phase starts, the task record should reflect the current phase
		reloaded, err := backend.LoadTask("TASK-002")
		if err != nil {
			t.Fatalf("reload task after phase %s: %v", phaseID, err)
		}

		currentPhase := task.GetCurrentPhaseProto(reloaded)
		if currentPhase != phaseID {
			t.Errorf("after phase %s: task.CurrentPhase = %q, want %q", phaseID, currentPhase, phaseID)
		}
	}
}

// TestPhaseUpdatesTaskCurrentPhase_SaveError verifies the failure mode from the spec:
// if SaveTask fails when setting CurrentPhase, the executor should log a warning
// and continue execution (non-fatal). We test with nil task (standalone workflow)
// to verify no panic occurs.
func TestPhaseUpdatesTaskCurrentPhase_SaveError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Timeouts: config.TimeoutsConfig{
				PhaseMax: 0,
			},
		},
		logger:   slog.Default(),
		resolver: variable.NewResolver("/tmp"),
		task:     nil, // No task — executor should not panic
	}

	createTestWorkflow(t, backend, "standalone-workflow")
	run := &db.WorkflowRun{
		ID:         "RUN-003",
		WorkflowID: "standalone-workflow",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save workflow run: %v", err)
	}

	tmpl := &db.PhaseTemplate{ID: "implement", Name: "Implement"}
	phase := &db.WorkflowPhase{PhaseTemplateID: "implement"}
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   "RUN-003",
		PhaseTemplateID: "implement",
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we.turnExecutor = mockTE

	// Should not panic even with nil task
	ctx := context.Background()
	_, err := we.executePhaseWithTimeout(ctx, tmpl, phase, map[string]string{}, nil, run, runPhase, nil)
	// We don't care about the specific error — just that it doesn't panic
	_ = err
}

// TestPhaseUpdatesTaskCurrentPhase_Loop verifies edge case: when a phase loops
// back (e.g., QA retry), CurrentPhase updates to the loop-back target.
func TestPhaseUpdatesTaskCurrentPhase_Loop(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	tsk := task.NewProtoTask("TASK-003", "Loop phase task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	we := &WorkflowExecutor{
		backend: backend,
		orcConfig: &config.Config{
			Timeouts: config.TimeoutsConfig{
				PhaseMax: 0,
			},
		},
		logger:   slog.Default(),
		resolver: variable.NewResolver("/tmp"),
		task:     tsk,
	}

	createTestWorkflow(t, backend, "test-workflow")
	taskID := "TASK-003"
	run := &db.WorkflowRun{
		ID:         "RUN-004",
		WorkflowID: "test-workflow",
		TaskID:     &taskID,
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("save workflow run: %v", err)
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	we.turnExecutor = mockTE

	// Simulate: implement → review → implement (loop back)
	executePhase := func(phaseID string) {
		tmpl := &db.PhaseTemplate{ID: phaseID, Name: phaseID}
		phase := &db.WorkflowPhase{PhaseTemplateID: phaseID}
		runPhase := &db.WorkflowRunPhase{
			WorkflowRunID:   "RUN-004",
			PhaseTemplateID: phaseID,
		}
		run.CurrentPhase = phaseID
		_, _ = we.executePhaseWithTimeout(context.Background(), tmpl, phase, map[string]string{}, nil, run, runPhase, tsk)
	}

	executePhase("implement")
	executePhase("review")

	// Now loop back to implement
	executePhase("implement")

	// After loop-back, CurrentPhase should be "implement" again
	reloaded, err := backend.LoadTask("TASK-003")
	if err != nil {
		t.Fatalf("reload task: %v", err)
	}

	currentPhase := task.GetCurrentPhaseProto(reloaded)
	if currentPhase != "implement" {
		t.Errorf("after loop-back: task.CurrentPhase = %q, want %q", currentPhase, "implement")
	}
}
