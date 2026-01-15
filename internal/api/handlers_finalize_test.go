package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
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

// createTestServerWithContext creates a server with proper context for testing.
func createTestServerWithContext(t *testing.T, backend storage.Backend, orcCfg *config.Config) *Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel() })

	return &Server{
		workDir:         t.TempDir(),
		mux:             http.NewServeMux(),
		orcConfig:       orcCfg,
		logger:          testLogger(),
		publisher:       events.NewNopPublisher(),
		backend:         backend,
		serverCtx:       ctx,
		serverCtxCancel: cancel,
	}
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

	// Create server with context for finalize goroutine management
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel() })

	srv := &Server{
		workDir:         t.TempDir(),
		mux:             http.NewServeMux(),
		orcConfig:       orcCfg,
		logger:          testLogger(),
		publisher:       events.NewNopPublisher(),
		backend:         backend,
		serverCtx:       ctx,
		serverCtxCancel: cancel,
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

func TestFinalizeTrackerCleanup(t *testing.T) {
	// Create a fresh tracker for this test to avoid interference
	tracker := &finalizeTracker{
		states: make(map[string]*FinalizeState),
	}

	// Add some test entries with different states and ages
	now := time.Now()
	retention := 5 * time.Minute

	// Old completed entry (should be removed)
	tracker.set("old-completed", &FinalizeState{
		TaskID:    "old-completed",
		Status:    FinalizeStatusCompleted,
		UpdatedAt: now.Add(-10 * time.Minute),
	})

	// Old failed entry (should be removed)
	tracker.set("old-failed", &FinalizeState{
		TaskID:    "old-failed",
		Status:    FinalizeStatusFailed,
		UpdatedAt: now.Add(-10 * time.Minute),
	})

	// Recent completed entry (should be preserved)
	tracker.set("recent-completed", &FinalizeState{
		TaskID:    "recent-completed",
		Status:    FinalizeStatusCompleted,
		UpdatedAt: now.Add(-1 * time.Minute),
	})

	// Old running entry (should be preserved - still active)
	tracker.set("old-running", &FinalizeState{
		TaskID:    "old-running",
		Status:    FinalizeStatusRunning,
		UpdatedAt: now.Add(-10 * time.Minute),
	})

	// Old pending entry (should be preserved - still active)
	tracker.set("old-pending", &FinalizeState{
		TaskID:    "old-pending",
		Status:    FinalizeStatusPending,
		UpdatedAt: now.Add(-10 * time.Minute),
	})

	// Run cleanup
	removed := tracker.cleanupStale(retention)

	// Verify correct number removed
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	// Verify old completed is gone
	if tracker.get("old-completed") != nil {
		t.Error("old-completed should have been removed")
	}

	// Verify old failed is gone
	if tracker.get("old-failed") != nil {
		t.Error("old-failed should have been removed")
	}

	// Verify recent completed is preserved
	if tracker.get("recent-completed") == nil {
		t.Error("recent-completed should be preserved")
	}

	// Verify running entries are preserved regardless of age
	if tracker.get("old-running") == nil {
		t.Error("old-running should be preserved (active operation)")
	}

	// Verify pending entries are preserved regardless of age
	if tracker.get("old-pending") == nil {
		t.Error("old-pending should be preserved (active operation)")
	}
}

func TestFinalizeTrackerCleanupPreservesRunning(t *testing.T) {
	tracker := &finalizeTracker{
		states: make(map[string]*FinalizeState),
	}

	now := time.Now()
	retention := 1 * time.Minute // Very short retention

	// Add running entries that are well past retention
	tracker.set("running-1", &FinalizeState{
		TaskID:    "running-1",
		Status:    FinalizeStatusRunning,
		UpdatedAt: now.Add(-1 * time.Hour),
	})

	tracker.set("pending-1", &FinalizeState{
		TaskID:    "pending-1",
		Status:    FinalizeStatusPending,
		UpdatedAt: now.Add(-1 * time.Hour),
	})

	// Run cleanup
	removed := tracker.cleanupStale(retention)

	// Verify nothing was removed (running/pending are never cleaned up)
	if removed != 0 {
		t.Errorf("removed = %d, want 0 (running/pending should never be removed)", removed)
	}

	if tracker.get("running-1") == nil {
		t.Error("running-1 should not be removed")
	}

	if tracker.get("pending-1") == nil {
		t.Error("pending-1 should not be removed")
	}
}

func TestFinalizeTrackerCleanupShutdown(t *testing.T) {
	tracker := &finalizeTracker{
		states: make(map[string]*FinalizeState),
	}

	// Add an old completed entry
	tracker.set("cleanup-shutdown-test", &FinalizeState{
		TaskID:    "cleanup-shutdown-test",
		Status:    FinalizeStatusCompleted,
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	})

	// Create context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start cleanup with short interval (10ms interval, 1 min retention)
	tracker.startCleanup(ctx, 10*time.Millisecond, 1*time.Minute)

	// Poll for cleanup to complete (more reliable than fixed sleep)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if tracker.get("cleanup-shutdown-test") == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify the entry was cleaned up
	if tracker.get("cleanup-shutdown-test") != nil {
		t.Error("cleanup-shutdown-test should have been cleaned up")
	}

	// Cancel context to stop cleanup goroutine
	cancel()

	// Give goroutine time to exit
	time.Sleep(50 * time.Millisecond)

	// Add another entry after cancellation
	tracker.set("post-cancel-shutdown", &FinalizeState{
		TaskID:    "post-cancel-shutdown",
		Status:    FinalizeStatusCompleted,
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	})

	// Wait a bit - cleanup should not run since context was cancelled
	time.Sleep(50 * time.Millisecond)

	// Entry should still exist (cleanup goroutine stopped)
	if tracker.get("post-cancel-shutdown") == nil {
		t.Error("post-cancel-shutdown should still exist (cleanup stopped)")
	}

	// Cleanup
	tracker.delete("post-cancel-shutdown")
}

func TestFinalizeTrackerCancelAll(t *testing.T) {
	// Create a fresh tracker for this test
	tracker := &finalizeTracker{
		states:  make(map[string]*FinalizeState),
		cancels: make(map[string]context.CancelFunc),
	}

	// Track which cancels were called
	canceledTasks := make(map[string]bool)
	var cancelMu sync.Mutex

	// Add some tasks with cancel functions
	for _, taskID := range []string{"task-1", "task-2", "task-3"} {
		tracker.set(taskID, &FinalizeState{
			TaskID: taskID,
			Status: FinalizeStatusRunning,
		})
		taskIDCopy := taskID
		tracker.setCancel(taskID, func() {
			cancelMu.Lock()
			canceledTasks[taskIDCopy] = true
			cancelMu.Unlock()
		})
	}

	// Verify cancels are tracked
	tracker.mu.RLock()
	if len(tracker.cancels) != 3 {
		t.Errorf("expected 3 cancels, got %d", len(tracker.cancels))
	}
	tracker.mu.RUnlock()

	// Call cancelAll
	tracker.cancelAll()

	// Verify all cancel functions were called
	cancelMu.Lock()
	for _, taskID := range []string{"task-1", "task-2", "task-3"} {
		if !canceledTasks[taskID] {
			t.Errorf("cancel function for %s was not called", taskID)
		}
	}
	cancelMu.Unlock()

	// Verify cancels map is empty
	tracker.mu.RLock()
	if len(tracker.cancels) != 0 {
		t.Errorf("expected 0 cancels after cancelAll, got %d", len(tracker.cancels))
	}
	tracker.mu.RUnlock()
}

func TestFinalizeTrackerSetCancelAndCancel(t *testing.T) {
	// Create a fresh tracker for this test
	tracker := &finalizeTracker{
		states:  make(map[string]*FinalizeState),
		cancels: make(map[string]context.CancelFunc),
	}

	t.Run("setCancel stores cancel function", func(t *testing.T) {
		called := false
		tracker.setCancel("test-task", func() { called = true })

		// Verify it's stored
		tracker.mu.RLock()
		_, exists := tracker.cancels["test-task"]
		tracker.mu.RUnlock()

		if !exists {
			t.Error("cancel function should be stored")
		}

		// Call cancel
		tracker.cancel("test-task")

		if !called {
			t.Error("cancel function should have been called")
		}

		// Verify it's removed after cancel
		tracker.mu.RLock()
		_, exists = tracker.cancels["test-task"]
		tracker.mu.RUnlock()

		if exists {
			t.Error("cancel function should be removed after cancel")
		}
	})

	t.Run("cancel is idempotent", func(t *testing.T) {
		callCount := 0
		tracker.setCancel("idempotent-task", func() { callCount++ })

		// Call cancel multiple times
		tracker.cancel("idempotent-task")
		tracker.cancel("idempotent-task")
		tracker.cancel("idempotent-task")

		// Should only be called once
		if callCount != 1 {
			t.Errorf("cancel should only be called once, got %d", callCount)
		}
	})

	t.Run("cancel on non-existent task is safe", func(t *testing.T) {
		// Should not panic
		tracker.cancel("non-existent-task")
	})

	t.Run("delete removes cancel function", func(t *testing.T) {
		called := false
		tracker.set("delete-test", &FinalizeState{TaskID: "delete-test"})
		tracker.setCancel("delete-test", func() { called = true })

		// Delete the task
		tracker.delete("delete-test")

		// Verify cancel is also removed
		tracker.mu.RLock()
		_, exists := tracker.cancels["delete-test"]
		tracker.mu.RUnlock()

		if exists {
			t.Error("cancel function should be removed by delete")
		}

		// The cancel function should not have been called by delete
		if called {
			t.Error("delete should not call the cancel function")
		}
	})
}

func TestServerContextOnShutdown(t *testing.T) {
	// Create server with a context we control
	ctx, cancel := context.WithCancel(context.Background())
	serverCtx, serverCancel := context.WithCancel(ctx)
	defer serverCancel()

	taskID := "TASK-SHUTDOWN-TEST"

	// Ensure clean state
	finTracker.delete(taskID)

	// Track if cancel was called via context
	finState := &FinalizeState{
		TaskID:    taskID,
		Status:    FinalizeStatusPending,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	finTracker.set(taskID, finState)

	// Create a derived context and store its cancel
	derivedCtx, derivedCancel := context.WithCancel(serverCtx)
	finTracker.setCancel(taskID, derivedCancel)

	// Verify context is not cancelled yet
	if derivedCtx.Err() != nil {
		t.Error("derived context should not be cancelled yet")
	}

	// Cancel the server context (simulating shutdown)
	cancel()

	// The derived context should now be cancelled (because it's derived from server context)
	// Wait a moment for propagation
	time.Sleep(10 * time.Millisecond)

	if derivedCtx.Err() == nil {
		t.Error("derived context should be cancelled after server context cancel")
	}

	// Clean up
	finTracker.delete(taskID)
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

		// Create server with context for finalize goroutine management
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(func() { cancel() })

		srv := &Server{
			workDir:         t.TempDir(),
			orcConfig:       orcCfg,
			logger:          testLogger(),
			publisher:       events.NewNopPublisher(),
			backend:         backend,
			serverCtx:       ctx,
			serverCtxCancel: cancel,
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
