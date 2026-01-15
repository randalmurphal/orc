package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// testLogger returns a logger that discards output.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// createTestBackend creates a backend for testing.
func createTestBackend(t *testing.T) storage.Backend {
	t.Helper()
	tmpDir := t.TempDir()
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	t.Cleanup(func() {
		backend.Close()
	})
	return backend
}

func TestHandleFinalizeTask(t *testing.T) {
	backend := createTestBackend(t)

	// Create a task
	taskID := "TASK-001"
	tsk := &task.Task{
		ID:     taskID,
		Title:  "Test task",
		Status: task.StatusCompleted,
		Weight: task.WeightMedium,
	}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create a plan with finalize phase
	p := &plan.Plan{
		TaskID: taskID,
		Phases: []plan.Phase{
			{ID: "implement", Status: plan.PhaseCompleted},
			{ID: "test", Status: plan.PhaseCompleted},
			{ID: "finalize", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(p, taskID); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// Create default config
	orcCfg := config.Default()

	// Create server
	srv := &Server{
		workDir:   t.TempDir(),
		mux:       http.NewServeMux(),
		orcConfig: orcCfg,
		logger:    testLogger(),
		publisher: events.NewNopPublisher(),
		backend:   backend,
	}

	// Register route
	srv.mux.HandleFunc("POST /api/tasks/{id}/finalize", srv.handleFinalizeTask)

	t.Run("returns acknowledgment for valid task", func(t *testing.T) {
		// Clear any previous state
		finTracker.delete(taskID)

		req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
		}

		var resp FinalizeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if resp.TaskID != taskID {
			t.Errorf("task_id = %s, want %s", resp.TaskID, taskID)
		}

		if resp.Status != FinalizeStatusPending {
			t.Errorf("status = %s, want %s", resp.Status, FinalizeStatusPending)
		}

		if resp.Message != "Finalize started" {
			t.Errorf("message = %s, want 'Finalize started'", resp.Message)
		}
	})

	t.Run("returns already in progress for duplicate request", func(t *testing.T) {
		// Set up a running finalize state
		finTracker.set(taskID, &FinalizeState{
			TaskID:    taskID,
			Status:    FinalizeStatusRunning,
			StartedAt: time.Now(),
			UpdatedAt: time.Now(),
			Step:      "Testing",
		})

		req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
		}

		var resp FinalizeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if resp.Status != FinalizeStatusRunning {
			t.Errorf("status = %s, want %s", resp.Status, FinalizeStatusRunning)
		}

		if resp.Message != "Finalize already in progress" {
			t.Errorf("message = %s, want 'Finalize already in progress'", resp.Message)
		}
	})

	t.Run("accepts request with options", func(t *testing.T) {
		// Clear any previous state
		finTracker.delete(taskID)

		body := FinalizeRequest{
			Force:        true,
			GateOverride: true,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/finalize", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
		}

		var resp FinalizeResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if resp.TaskID != taskID {
			t.Errorf("task_id = %s, want %s", resp.TaskID, taskID)
		}
	})

	t.Run("returns 404 for non-existent task", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/tasks/TASK-999/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("rejects non-finalizable task status without force", func(t *testing.T) {
		// Create a running task
		runningTaskID := "TASK-002"
		runningTask := &task.Task{
			ID:     runningTaskID,
			Title:  "Running task",
			Status: task.StatusRunning,
			Weight: task.WeightMedium,
		}
		if err := backend.SaveTask(runningTask); err != nil {
			t.Fatalf("save task: %v", err)
		}

		req := httptest.NewRequest("POST", "/api/tasks/"+runningTaskID+"/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestHandleGetFinalizeStatus(t *testing.T) {
	backend := createTestBackend(t)

	// Create a task
	taskID := "TASK-001"
	tsk := &task.Task{
		ID:     taskID,
		Title:  "Test task",
		Status: task.StatusCompleted,
		Weight: task.WeightMedium,
	}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create default config
	orcCfg := config.Default()

	// Create server
	srv := &Server{
		workDir:   t.TempDir(),
		mux:       http.NewServeMux(),
		orcConfig: orcCfg,
		logger:    testLogger(),
		publisher: events.NewNopPublisher(),
		backend:   backend,
	}

	// Register route
	srv.mux.HandleFunc("GET /api/tasks/{id}/finalize", srv.handleGetFinalizeStatus)

	t.Run("returns not_started when no finalize in progress", func(t *testing.T) {
		// Clear any previous state
		finTracker.delete(taskID)

		req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if resp["status"] != "not_started" {
			t.Errorf("status = %v, want not_started", resp["status"])
		}
	})

	t.Run("returns current state when finalize in progress", func(t *testing.T) {
		// Set up a running finalize state
		now := time.Now()
		finTracker.set(taskID, &FinalizeState{
			TaskID:      taskID,
			Status:      FinalizeStatusRunning,
			StartedAt:   now,
			UpdatedAt:   now,
			Step:        "Syncing with target",
			Progress:    "Merging changes",
			StepPercent: 50,
		})

		req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if resp["status"] != string(FinalizeStatusRunning) {
			t.Errorf("status = %v, want %s", resp["status"], FinalizeStatusRunning)
		}

		if resp["step"] != "Syncing with target" {
			t.Errorf("step = %v, want 'Syncing with target'", resp["step"])
		}

		if resp["step_percent"].(float64) != 50 {
			t.Errorf("step_percent = %v, want 50", resp["step_percent"])
		}
	})

	t.Run("returns completed state with result", func(t *testing.T) {
		// Set up a completed finalize state
		now := time.Now()
		finTracker.set(taskID, &FinalizeState{
			TaskID:      taskID,
			Status:      FinalizeStatusCompleted,
			StartedAt:   now.Add(-5 * time.Minute),
			UpdatedAt:   now,
			Step:        "Complete",
			Progress:    "Finalize completed successfully",
			StepPercent: 100,
			Result: &FinalizeResult{
				Synced:       true,
				CommitSHA:    "abc123",
				TargetBranch: "main",
				RiskLevel:    "low",
			},
		})

		req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if resp["status"] != string(FinalizeStatusCompleted) {
			t.Errorf("status = %v, want %s", resp["status"], FinalizeStatusCompleted)
		}

		result, ok := resp["result"].(map[string]any)
		if !ok {
			t.Fatal("result not found in response")
		}

		if result["synced"] != true {
			t.Errorf("synced = %v, want true", result["synced"])
		}

		if result["commit_sha"] != "abc123" {
			t.Errorf("commit_sha = %v, want abc123", result["commit_sha"])
		}
	})

	t.Run("returns failed state with error", func(t *testing.T) {
		// Set up a failed finalize state
		now := time.Now()
		finTracker.set(taskID, &FinalizeState{
			TaskID:    taskID,
			Status:    FinalizeStatusFailed,
			StartedAt: now.Add(-2 * time.Minute),
			UpdatedAt: now,
			Step:      "Failed",
			Error:     "merge conflict in main.go",
		})

		req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if resp["status"] != string(FinalizeStatusFailed) {
			t.Errorf("status = %v, want %s", resp["status"], FinalizeStatusFailed)
		}

		if resp["error"] != "merge conflict in main.go" {
			t.Errorf("error = %v, want 'merge conflict in main.go'", resp["error"])
		}
	})

	t.Run("returns from state when no tracker state", func(t *testing.T) {
		// Clear tracker state
		finTracker.delete(taskID)

		// Create state with completed finalize phase
		now := time.Now()
		st := state.New(taskID)
		st.Phases["finalize"] = &state.PhaseState{
			Status:      state.StatusCompleted,
			StartedAt:   now.Add(-5 * time.Minute),
			CompletedAt: &now,
			CommitSHA:   "def456",
		}
		if err := backend.SaveState(st); err != nil {
			t.Fatalf("save state: %v", err)
		}

		req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}

		if resp["status"] != string(FinalizeStatusCompleted) {
			t.Errorf("status = %v, want %s", resp["status"], FinalizeStatusCompleted)
		}

		if resp["commit_sha"] != "def456" {
			t.Errorf("commit_sha = %v, want def456", resp["commit_sha"])
		}
	})

	t.Run("returns 404 for non-existent task", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/tasks/TASK-999/finalize", nil)
		w := httptest.NewRecorder()

		srv.mux.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("status code = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}

func TestFinalizeTracker(t *testing.T) {
	t.Run("get returns nil for unknown task", func(t *testing.T) {
		state := finTracker.get("unknown-task")
		if state != nil {
			t.Error("expected nil for unknown task")
		}
	})

	t.Run("set and get work correctly", func(t *testing.T) {
		taskID := "test-task-1"
		state := &FinalizeState{
			TaskID: taskID,
			Status: FinalizeStatusRunning,
		}

		finTracker.set(taskID, state)
		defer finTracker.delete(taskID)

		got := finTracker.get(taskID)
		if got == nil {
			t.Fatal("expected non-nil state")
		}

		if got.TaskID != taskID {
			t.Errorf("task_id = %s, want %s", got.TaskID, taskID)
		}
	})

	t.Run("delete removes state", func(t *testing.T) {
		taskID := "test-task-2"
		finTracker.set(taskID, &FinalizeState{TaskID: taskID})

		finTracker.delete(taskID)

		got := finTracker.get(taskID)
		if got != nil {
			t.Error("expected nil after delete")
		}
	})
}

func TestTriggerFinalizeOnApproval(t *testing.T) {
	t.Run("does not trigger when config disabled", func(t *testing.T) {
		backend := createTestBackend(t)

		// Create a completed task
		taskID := "TASK-001"
		tsk := &task.Task{
			ID:     taskID,
			Title:  "Test task",
			Status: task.StatusCompleted,
			Weight: task.WeightMedium,
		}
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		// Create config with auto-trigger disabled
		orcCfg := config.Default()
		orcCfg.Completion.Finalize.AutoTriggerOnApproval = false

		srv := &Server{
			workDir:   t.TempDir(),
			orcConfig: orcCfg,
			logger:    testLogger(),
			publisher: events.NewNopPublisher(),
			backend:   backend,
		}

		triggered, err := srv.TriggerFinalizeOnApproval(taskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if triggered {
			t.Error("should not trigger when config disabled")
		}
	})

	t.Run("does not trigger when finalize already running", func(t *testing.T) {
		backend := createTestBackend(t)

		taskID := "TASK-002"
		tsk := &task.Task{
			ID:     taskID,
			Title:  "Test task",
			Status: task.StatusCompleted,
			Weight: task.WeightMedium,
		}
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		// Set up running finalize
		finTracker.set(taskID, &FinalizeState{
			TaskID: taskID,
			Status: FinalizeStatusRunning,
		})
		defer finTracker.delete(taskID)

		orcCfg := config.Default()
		srv := &Server{
			workDir:   t.TempDir(),
			orcConfig: orcCfg,
			logger:    testLogger(),
			publisher: events.NewNopPublisher(),
			backend:   backend,
		}

		triggered, err := srv.TriggerFinalizeOnApproval(taskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if triggered {
			t.Error("should not trigger when finalize already running")
		}
	})

	t.Run("does not trigger for trivial weight tasks", func(t *testing.T) {
		backend := createTestBackend(t)

		taskID := "TASK-003"
		tsk := &task.Task{
			ID:     taskID,
			Title:  "Trivial task",
			Status: task.StatusCompleted,
			Weight: task.WeightTrivial, // Trivial weight
		}
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		orcCfg := config.Default()
		srv := &Server{
			workDir:   t.TempDir(),
			orcConfig: orcCfg,
			logger:    testLogger(),
			publisher: events.NewNopPublisher(),
			backend:   backend,
		}

		triggered, err := srv.TriggerFinalizeOnApproval(taskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if triggered {
			t.Error("should not trigger for trivial weight tasks")
		}
	})

	t.Run("does not trigger for non-completed tasks", func(t *testing.T) {
		backend := createTestBackend(t)

		taskID := "TASK-004"
		tsk := &task.Task{
			ID:     taskID,
			Title:  "Running task",
			Status: task.StatusRunning, // Not completed
			Weight: task.WeightMedium,
		}
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		orcCfg := config.Default()
		srv := &Server{
			workDir:   t.TempDir(),
			orcConfig: orcCfg,
			logger:    testLogger(),
			publisher: events.NewNopPublisher(),
			backend:   backend,
		}

		triggered, err := srv.TriggerFinalizeOnApproval(taskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if triggered {
			t.Error("should not trigger for non-completed tasks")
		}
	})

	t.Run("does not trigger when finalize already completed", func(t *testing.T) {
		backend := createTestBackend(t)

		taskID := "TASK-005"
		tsk := &task.Task{
			ID:     taskID,
			Title:  "Completed task",
			Status: task.StatusCompleted,
			Weight: task.WeightMedium,
		}
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		// Create state with completed finalize
		st := state.New(taskID)
		st.Phases["finalize"] = &state.PhaseState{
			Status: state.StatusCompleted,
		}
		if err := backend.SaveState(st); err != nil {
			t.Fatalf("save state: %v", err)
		}

		orcCfg := config.Default()
		srv := &Server{
			workDir:   t.TempDir(),
			orcConfig: orcCfg,
			logger:    testLogger(),
			publisher: events.NewNopPublisher(),
			backend:   backend,
		}

		triggered, err := srv.TriggerFinalizeOnApproval(taskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if triggered {
			t.Error("should not trigger when finalize already completed")
		}
	})

	t.Run("triggers finalize for valid task", func(t *testing.T) {
		backend := createTestBackend(t)

		taskID := "TASK-006"
		// Create a completed task with medium weight
		tsk := &task.Task{
			ID:     taskID,
			Title:  "Valid task",
			Status: task.StatusCompleted,
			Weight: task.WeightMedium,
		}
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task: %v", err)
		}

		// Create plan
		p := &plan.Plan{
			TaskID: taskID,
			Phases: []plan.Phase{
				{ID: "implement", Status: plan.PhaseCompleted},
				{ID: "finalize", Status: plan.PhasePending},
			},
		}
		if err := backend.SavePlan(p, taskID); err != nil {
			t.Fatalf("save plan: %v", err)
		}

		// Ensure no prior finalize tracker state
		finTracker.delete(taskID)

		orcCfg := config.Default()
		// Ensure auto-trigger is enabled
		orcCfg.Completion.Finalize.AutoTriggerOnApproval = true

		srv := &Server{
			workDir:   t.TempDir(),
			orcConfig: orcCfg,
			logger:    testLogger(),
			publisher: events.NewNopPublisher(),
			backend:   backend,
		}

		triggered, err := srv.TriggerFinalizeOnApproval(taskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !triggered {
			t.Error("should trigger finalize for valid task")
		}

		// Clean up: check that finalize state was created
		finState := finTracker.get(taskID)
		if finState == nil {
			t.Error("finalize state should be created")
		} else {
			finTracker.delete(taskID)
		}
	})

	t.Run("returns error for non-existent task", func(t *testing.T) {
		backend := createTestBackend(t)

		orcCfg := config.Default()
		srv := &Server{
			workDir:   t.TempDir(),
			orcConfig: orcCfg,
			logger:    testLogger(),
			publisher: events.NewNopPublisher(),
			backend:   backend,
		}

		_, err := srv.TriggerFinalizeOnApproval("TASK-NONEXISTENT")
		if err == nil {
			t.Error("should return error for non-existent task")
		}
	})
}
