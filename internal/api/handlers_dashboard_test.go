package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func TestHandleGetDashboardStats_DefaultPeriod(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rr := httptest.NewRecorder()

	server.handleGetDashboardStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response DashboardStats
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should default to 7d period
	if response.Period != "7d" {
		t.Errorf("expected period=7d, got %s", response.Period)
	}

	// Empty database should return zeros
	if response.Total != 0 {
		t.Errorf("expected total=0, got %d", response.Total)
	}
	if response.Completed != 0 {
		t.Errorf("expected completed=0, got %d", response.Completed)
	}
}

func TestHandleGetDashboardStats_InvalidPeriod(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	tests := []struct {
		name   string
		period string
	}{
		{"invalid period", "1h"},
		{"invalid period 90d", "90d"},
		{"invalid period abc", "abc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats?period="+tc.period, nil)
			rr := httptest.NewRecorder()

			server.handleGetDashboardStats(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d", rr.Code)
			}
		})
	}
}

func TestHandleGetDashboardStats_BackwardCompatibility(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	now := time.Now()

	// Create tasks with different statuses
	runningTask := task.New("TASK-001", "Running task")
	runningTask.Status = task.StatusRunning
	runningTask.ExecutorPID = os.Getpid() // Set PID so it's not detected as orphaned
	if err := backend.SaveTask(runningTask); err != nil {
		t.Fatalf("failed to save running task: %v", err)
	}

	pausedTask := task.New("TASK-002", "Paused task")
	pausedTask.Status = task.StatusPaused
	if err := backend.SaveTask(pausedTask); err != nil {
		t.Fatalf("failed to save paused task: %v", err)
	}

	completedTask := task.New("TASK-003", "Completed task")
	completedTask.Status = task.StatusCompleted
	completedAt := now.Add(-1 * time.Hour)
	completedTask.CompletedAt = &completedAt
	if err := backend.SaveTask(completedTask); err != nil {
		t.Fatalf("failed to save completed task: %v", err)
	}

	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rr := httptest.NewRecorder()

	server.handleGetDashboardStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response DashboardStats
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check all existing fields are present
	if response.Running != 1 {
		t.Errorf("expected running=1, got %d", response.Running)
	}
	if response.Paused != 1 {
		t.Errorf("expected paused=1, got %d", response.Paused)
	}
	if response.Completed != 1 {
		t.Errorf("expected completed=1, got %d", response.Completed)
	}
	if response.Total != 3 {
		t.Errorf("expected total=3, got %d", response.Total)
	}
}

func TestHandleGetDashboardStats_AverageTaskTime(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	now := time.Now()

	// Create completed tasks with different durations
	// Task 1: 100 seconds
	task1 := task.New("TASK-001", "Task 1")
	task1.Status = task.StatusCompleted
	started1 := now.Add(-2 * time.Hour)
	completed1 := started1.Add(100 * time.Second)
	task1.StartedAt = &started1
	task1.CompletedAt = &completed1
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("failed to save task 1: %v", err)
	}

	// Task 2: 200 seconds
	task2 := task.New("TASK-002", "Task 2")
	task2.Status = task.StatusCompleted
	started2 := now.Add(-1 * time.Hour)
	completed2 := started2.Add(200 * time.Second)
	task2.StartedAt = &started2
	task2.CompletedAt = &completed2
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("failed to save task 2: %v", err)
	}

	// Task 3: 300 seconds
	task3 := task.New("TASK-003", "Task 3")
	task3.Status = task.StatusCompleted
	started3 := now.Add(-30 * time.Minute)
	completed3 := started3.Add(300 * time.Second)
	task3.StartedAt = &started3
	task3.CompletedAt = &completed3
	if err := backend.SaveTask(task3); err != nil {
		t.Fatalf("failed to save task 3: %v", err)
	}

	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rr := httptest.NewRecorder()

	server.handleGetDashboardStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response DashboardStats
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Average should be (100 + 200 + 300) / 3 = 200
	if response.AvgTaskTimeSeconds == nil {
		t.Fatal("expected avg_task_time_seconds to be set")
	}
	expected := 200.0
	if *response.AvgTaskTimeSeconds < expected-0.1 || *response.AvgTaskTimeSeconds > expected+0.1 {
		t.Errorf("expected avg_task_time_seconds≈%.1f, got %.1f", expected, *response.AvgTaskTimeSeconds)
	}
}

func TestHandleGetDashboardStats_SuccessRate(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	now := time.Now()

	// Create 9 completed tasks
	for i := 1; i <= 9; i++ {
		tsk := task.New(fmt.Sprintf("TASK-%03d", i), fmt.Sprintf("Task %d", i))
		tsk.Status = task.StatusCompleted
		completedAt := now.Add(-time.Duration(i) * time.Hour)
		tsk.CompletedAt = &completedAt
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task %d: %v", i, err)
		}
	}

	// Create 1 failed task
	// Note: UpdatedAt is not persisted by database, so we rely on CreatedAt for period filtering
	failedTask := task.New("TASK-100", "Failed task")
	failedTask.Status = task.StatusFailed
	// CreatedAt is set by task.New() to now, which is within the 7d period
	if err := backend.SaveTask(failedTask); err != nil {
		t.Fatalf("failed to save failed task: %v", err)
	}

	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rr := httptest.NewRecorder()

	server.handleGetDashboardStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response DashboardStats
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Success rate should be 9/10 = 0.9
	if response.SuccessRate == nil {
		t.Fatal("expected success_rate to be set")
	}
	expected := 0.9
	if *response.SuccessRate < expected-0.01 || *response.SuccessRate > expected+0.01 {
		t.Errorf("expected success_rate≈%.2f, got %.2f", expected, *response.SuccessRate)
	}
}

func TestHandleGetDashboardStats_PeriodComparison(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	now := time.Now()

	// Create tasks in current period (last 7 days): 3 completed
	for i := 1; i <= 3; i++ {
		tsk := task.New(fmt.Sprintf("TASK-%03d", i), fmt.Sprintf("Current task %d", i))
		tsk.Status = task.StatusCompleted
		completedAt := now.Add(-time.Duration(i) * 24 * time.Hour)
		startedAt := completedAt.Add(-100 * time.Second)
		tsk.StartedAt = &startedAt
		tsk.CompletedAt = &completedAt
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save current task %d: %v", i, err)
		}

		// Add state with tokens and cost
		st := &state.State{
			TaskID:       tsk.ID,
			CurrentPhase: "implement",
			Status:       state.StatusCompleted,
			StartedAt:    *tsk.StartedAt,
			CompletedAt:  tsk.CompletedAt,
			Cost: state.CostTracking{
				TotalCostUSD: 1.0,
			},
			Phases: map[string]*state.PhaseState{
				"implement": {
					Status: state.StatusCompleted,
					Tokens: state.TokenUsage{
						InputTokens:  8000,
						OutputTokens: 2000,
					},
				},
			},
		}
		if err := backend.SaveState(st); err != nil {
			t.Fatalf("failed to save state for task %d: %v", i, err)
		}
	}

	// Create tasks in previous period (8-14 days ago): 2 completed
	for i := 1; i <= 2; i++ {
		tsk := task.New(fmt.Sprintf("TASK-%03d", 100+i), fmt.Sprintf("Previous task %d", i))
		tsk.Status = task.StatusCompleted
		completedAt := now.Add(-time.Duration(7+i) * 24 * time.Hour)
		startedAt := completedAt.Add(-200 * time.Second)
		tsk.StartedAt = &startedAt
		tsk.CompletedAt = &completedAt
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save previous task %d: %v", i, err)
		}

		// Add state with tokens and cost
		st := &state.State{
			TaskID:       tsk.ID,
			CurrentPhase: "implement",
			Status:       state.StatusCompleted,
			StartedAt:    *tsk.StartedAt,
			CompletedAt:  tsk.CompletedAt,
			Cost: state.CostTracking{
				TotalCostUSD: 0.8,
			},
			Phases: map[string]*state.PhaseState{
				"implement": {
					Status: state.StatusCompleted,
					Tokens: state.TokenUsage{
						InputTokens:  6400,
						OutputTokens: 1600,
					},
				},
			},
		}
		if err := backend.SaveState(st); err != nil {
			t.Fatalf("failed to save state for previous task %d: %v", i, err)
		}
	}

	// Verify states were saved before closing backend
	for i := 1; i <= 3; i++ {
		taskID := fmt.Sprintf("TASK-%03d", i)
		verifyState, err := backend.LoadState(taskID)
		if err != nil {
			t.Fatalf("failed to load state for verification %s: %v", taskID, err)
		}
		if phase, ok := verifyState.Phases["implement"]; !ok {
			t.Fatalf("state for %s missing implement phase", taskID)
		} else if total := phase.Tokens.InputTokens + phase.Tokens.OutputTokens; total != 10000 {
			t.Fatalf("state for %s has wrong tokens: got %d", taskID, total)
		}
	}

	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats?period=7d", nil)
	rr := httptest.NewRecorder()

	server.handleGetDashboardStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response DashboardStats
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Current period: 3 completed, 30000 tokens, $3.00 cost
	if response.Completed != 3 {
		t.Errorf("expected completed=3, got %d", response.Completed)
	}
	if response.Tokens != 30000 {
		t.Errorf("expected tokens=30000, got %d", response.Tokens)
	}
	if response.Cost < 2.99 || response.Cost > 3.01 {
		t.Errorf("expected cost≈3.00, got %.2f", response.Cost)
	}

	// Previous period data should be present
	if response.PreviousPeriod == nil {
		t.Fatal("expected previous_period to be set")
	}
	if response.PreviousPeriod.Completed != 2 {
		t.Errorf("expected previous completed=2, got %d", response.PreviousPeriod.Completed)
	}
	if response.PreviousPeriod.Tokens != 16000 {
		t.Errorf("expected previous tokens=16000, got %d", response.PreviousPeriod.Tokens)
	}

	// Changes should be present
	if response.Changes == nil {
		t.Fatal("expected changes to be set")
	}

	// Completed change: (3-2)/2 * 100 = 50%
	if response.Changes.CompletedPct == nil {
		t.Fatal("expected completed_pct to be set")
	}
	expectedPct := 50.0
	if *response.Changes.CompletedPct < expectedPct-0.1 || *response.Changes.CompletedPct > expectedPct+0.1 {
		t.Errorf("expected completed_pct≈%.1f, got %.1f", expectedPct, *response.Changes.CompletedPct)
	}

	// Tokens change: (30000-16000)/16000 * 100 = 87.5%
	if response.Changes.TokensPct == nil {
		t.Fatal("expected tokens_pct to be set")
	}
	expectedTokensPct := 87.5
	if *response.Changes.TokensPct < expectedTokensPct-0.1 || *response.Changes.TokensPct > expectedTokensPct+0.1 {
		t.Errorf("expected tokens_pct≈%.1f, got %.1f", expectedTokensPct, *response.Changes.TokensPct)
	}
}

func TestHandleGetDashboardStats_NoPreviousPeriodForAll(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats?period=all", nil)
	rr := httptest.NewRecorder()

	server.handleGetDashboardStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response DashboardStats
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// For "all" period, previous_period and changes should not be set
	if response.PreviousPeriod != nil {
		t.Error("expected previous_period to be nil for 'all' period")
	}
	if response.Changes != nil {
		t.Error("expected changes to be nil for 'all' period")
	}
}

func TestHandleGetDashboardStats_EdgeCases(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	now := time.Now()

	// Task with no StartedAt (should not contribute to avg time)
	task1 := task.New("TASK-001", "No start time")
	task1.Status = task.StatusCompleted
	completedAt := now.Add(-1 * time.Hour)
	task1.CompletedAt = &completedAt
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("failed to save task 1: %v", err)
	}

	// Task with StartedAt and CompletedAt (should contribute)
	task2 := task.New("TASK-002", "With times")
	task2.Status = task.StatusCompleted
	started2 := now.Add(-2 * time.Hour)
	completed2 := started2.Add(100 * time.Second)
	task2.StartedAt = &started2
	task2.CompletedAt = &completed2
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("failed to save task 2: %v", err)
	}

	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rr := httptest.NewRecorder()

	server.handleGetDashboardStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response DashboardStats
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 2 completed tasks
	if response.Completed != 2 {
		t.Errorf("expected completed=2, got %d", response.Completed)
	}

	// Average should only include task2 (100 seconds)
	if response.AvgTaskTimeSeconds == nil {
		t.Fatal("expected avg_task_time_seconds to be set")
	}
	expected := 100.0
	if *response.AvgTaskTimeSeconds < expected-0.1 || *response.AvgTaskTimeSeconds > expected+0.1 {
		t.Errorf("expected avg_task_time_seconds≈%.1f, got %.1f", expected, *response.AvgTaskTimeSeconds)
	}
}

func TestHandleGetDashboardStats_ZeroDivision(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create task that's still running (not completed or failed)
	runningTask := task.New("TASK-001", "Running task")
	runningTask.Status = task.StatusRunning
	if err := backend.SaveTask(runningTask); err != nil {
		t.Fatalf("failed to save running task: %v", err)
	}

	_ = backend.Close()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rr := httptest.NewRecorder()

	server.handleGetDashboardStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response DashboardStats
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// No completed or failed tasks, so success rate should not be set
	if response.SuccessRate != nil {
		t.Error("expected success_rate to be nil when no finished tasks")
	}

	// No completed tasks with times, so avg should not be set
	if response.AvgTaskTimeSeconds != nil {
		t.Error("expected avg_task_time_seconds to be nil when no completed tasks with times")
	}
}
