package db

import (
	"testing"
	"time"
)

func TestGetAllPhasesGrouped_Empty(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	phases, err := db.GetAllPhasesGrouped()
	if err != nil {
		t.Fatalf("GetAllPhasesGrouped failed: %v", err)
	}

	if phases == nil {
		t.Error("expected empty map, got nil")
	}
	if len(phases) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(phases))
	}
}

func TestGetAllPhasesGrouped_SingleTask(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create a task with phases
	task := &Task{ID: "TASK-001", Title: "Test Task", Status: "running"}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	now := time.Now()
	phase1 := &Phase{TaskID: "TASK-001", PhaseID: "spec", Status: "completed", StartedAt: &now}
	phase2 := &Phase{TaskID: "TASK-001", PhaseID: "implement", Status: "running", StartedAt: &now}
	if err := db.SavePhase(phase1); err != nil {
		t.Fatalf("SavePhase failed: %v", err)
	}
	if err := db.SavePhase(phase2); err != nil {
		t.Fatalf("SavePhase failed: %v", err)
	}

	phases, err := db.GetAllPhasesGrouped()
	if err != nil {
		t.Fatalf("GetAllPhasesGrouped failed: %v", err)
	}

	if len(phases) != 1 {
		t.Errorf("expected 1 task, got %d", len(phases))
	}

	taskPhases := phases["TASK-001"]
	if len(taskPhases) != 2 {
		t.Errorf("expected 2 phases for TASK-001, got %d", len(taskPhases))
	}
}

func TestGetAllPhasesGrouped_MultipleTasks(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create multiple tasks with phases
	for _, taskID := range []string{"TASK-001", "TASK-002", "TASK-003"} {
		task := &Task{ID: taskID, Title: "Test Task", Status: "running"}
		if err := db.SaveTask(task); err != nil {
			t.Fatalf("SaveTask failed: %v", err)
		}

		now := time.Now()
		phase := &Phase{TaskID: taskID, PhaseID: "spec", Status: "completed", StartedAt: &now}
		if err := db.SavePhase(phase); err != nil {
			t.Fatalf("SavePhase failed: %v", err)
		}
	}

	phases, err := db.GetAllPhasesGrouped()
	if err != nil {
		t.Fatalf("GetAllPhasesGrouped failed: %v", err)
	}

	if len(phases) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(phases))
	}

	for _, taskID := range []string{"TASK-001", "TASK-002", "TASK-003"} {
		taskPhases := phases[taskID]
		if len(taskPhases) != 1 {
			t.Errorf("expected 1 phase for %s, got %d", taskID, len(taskPhases))
		}
	}
}

func TestGetAllPhasesGrouped_TaskWithoutPhases(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create a task without phases
	task := &Task{ID: "TASK-001", Title: "Test Task", Status: "created"}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	phases, err := db.GetAllPhasesGrouped()
	if err != nil {
		t.Fatalf("GetAllPhasesGrouped failed: %v", err)
	}

	// Task without phases should not appear in the map
	if len(phases) != 0 {
		t.Errorf("expected 0 tasks in phases map, got %d", len(phases))
	}

	// Accessing a non-existent key should return nil/empty slice
	taskPhases := phases["TASK-001"]
	if len(taskPhases) != 0 {
		t.Errorf("expected 0 phases for TASK-001, got %d", len(taskPhases))
	}
}

func TestGetAllGateDecisionsGrouped_Empty(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	gates, err := db.GetAllGateDecisionsGrouped()
	if err != nil {
		t.Fatalf("GetAllGateDecisionsGrouped failed: %v", err)
	}

	if gates == nil {
		t.Error("expected empty map, got nil")
	}
	if len(gates) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(gates))
	}
}

func TestGetAllGateDecisionsGrouped_SingleTask(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create a task with gate decisions
	task := &Task{ID: "TASK-001", Title: "Test Task", Status: "running"}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	gate1 := &GateDecision{TaskID: "TASK-001", Phase: "spec", GateType: "auto", Approved: true}
	gate2 := &GateDecision{TaskID: "TASK-001", Phase: "implement", GateType: "human", Approved: true}
	if err := db.AddGateDecision(gate1); err != nil {
		t.Fatalf("AddGateDecision failed: %v", err)
	}
	if err := db.AddGateDecision(gate2); err != nil {
		t.Fatalf("AddGateDecision failed: %v", err)
	}

	gates, err := db.GetAllGateDecisionsGrouped()
	if err != nil {
		t.Fatalf("GetAllGateDecisionsGrouped failed: %v", err)
	}

	if len(gates) != 1 {
		t.Errorf("expected 1 task, got %d", len(gates))
	}

	taskGates := gates["TASK-001"]
	if len(taskGates) != 2 {
		t.Errorf("expected 2 gate decisions for TASK-001, got %d", len(taskGates))
	}
}

func TestGetAllGateDecisionsGrouped_MultipleTasks(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create multiple tasks with gate decisions
	for _, taskID := range []string{"TASK-001", "TASK-002"} {
		task := &Task{ID: taskID, Title: "Test Task", Status: "running"}
		if err := db.SaveTask(task); err != nil {
			t.Fatalf("SaveTask failed: %v", err)
		}

		gate := &GateDecision{TaskID: taskID, Phase: "spec", GateType: "auto", Approved: true}
		if err := db.AddGateDecision(gate); err != nil {
			t.Fatalf("AddGateDecision failed: %v", err)
		}
	}

	gates, err := db.GetAllGateDecisionsGrouped()
	if err != nil {
		t.Fatalf("GetAllGateDecisionsGrouped failed: %v", err)
	}

	if len(gates) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(gates))
	}

	for _, taskID := range []string{"TASK-001", "TASK-002"} {
		taskGates := gates[taskID]
		if len(taskGates) != 1 {
			t.Errorf("expected 1 gate for %s, got %d", taskID, len(taskGates))
		}
	}
}

func TestGetAllGateDecisionsGrouped_Order(t *testing.T) {
	t.Parallel()
	db := NewTestProjectDB(t)

	// Create a task with multiple gate decisions
	task := &Task{ID: "TASK-001", Title: "Test Task", Status: "running"}
	if err := db.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Add gates with explicit timestamps to verify ordering
	now := time.Now()
	gate1 := &GateDecision{TaskID: "TASK-001", Phase: "spec", GateType: "auto", Approved: true, DecidedAt: now.Add(-2 * time.Hour)}
	gate2 := &GateDecision{TaskID: "TASK-001", Phase: "implement", GateType: "human", Approved: true, DecidedAt: now.Add(-1 * time.Hour)}
	gate3 := &GateDecision{TaskID: "TASK-001", Phase: "review", GateType: "ai", Approved: true, DecidedAt: now}

	if err := db.AddGateDecision(gate1); err != nil {
		t.Fatalf("AddGateDecision failed: %v", err)
	}
	if err := db.AddGateDecision(gate2); err != nil {
		t.Fatalf("AddGateDecision failed: %v", err)
	}
	if err := db.AddGateDecision(gate3); err != nil {
		t.Fatalf("AddGateDecision failed: %v", err)
	}

	gates, err := db.GetAllGateDecisionsGrouped()
	if err != nil {
		t.Fatalf("GetAllGateDecisionsGrouped failed: %v", err)
	}

	taskGates := gates["TASK-001"]
	if len(taskGates) != 3 {
		t.Fatalf("expected 3 gate decisions, got %d", len(taskGates))
	}

	// Verify ordering by decided_at
	if taskGates[0].Phase != "spec" {
		t.Errorf("expected first gate to be 'spec', got %s", taskGates[0].Phase)
	}
	if taskGates[1].Phase != "implement" {
		t.Errorf("expected second gate to be 'implement', got %s", taskGates[1].Phase)
	}
	if taskGates[2].Phase != "review" {
		t.Errorf("expected third gate to be 'review', got %s", taskGates[2].Phase)
	}
}
