package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/task"
)

func TestHandleGetSessionMetrics_EmptyState(t *testing.T) {
	t.Parallel()
	backend := createTestBackend(t)

	sessionID := uuid.New().String()
	sessionStart := time.Now().Add(-5 * time.Minute)

	srv := &Server{
		workDir:      t.TempDir(),
		mux:          http.NewServeMux(),
		orcConfig:    config.Default(),
		logger:       testLogger(),
		publisher:    events.NewNopPublisher(),
		backend:      backend,
		sessionID:    sessionID,
		sessionStart: sessionStart,
	}

	// Register route
	srv.mux.HandleFunc("GET /api/session", srv.handleGetSessionMetrics)

	req := httptest.NewRequest("GET", "/api/session", nil)
	w := httptest.NewRecorder()

	srv.handleGetSessionMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response SessionMetricsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify all fields are present
	if response.SessionID != sessionID {
		t.Errorf("expected session_id %q, got %q", sessionID, response.SessionID)
	}
	if !response.StartedAt.Equal(sessionStart) {
		t.Errorf("expected started_at %v, got %v", sessionStart, response.StartedAt)
	}
	if response.DurationSeconds < 299 || response.DurationSeconds > 301 {
		t.Errorf("expected duration ~300 seconds, got %d", response.DurationSeconds)
	}
	if response.TotalTokens != 0 {
		t.Errorf("expected total_tokens 0, got %d", response.TotalTokens)
	}
	if response.InputTokens != 0 {
		t.Errorf("expected input_tokens 0, got %d", response.InputTokens)
	}
	if response.OutputTokens != 0 {
		t.Errorf("expected output_tokens 0, got %d", response.OutputTokens)
	}
	if response.EstimatedCostUSD != 0 {
		t.Errorf("expected estimated_cost_usd 0, got %f", response.EstimatedCostUSD)
	}
	if response.TasksCompleted != 0 {
		t.Errorf("expected tasks_completed 0, got %d", response.TasksCompleted)
	}
	if response.TasksRunning != 0 {
		t.Errorf("expected tasks_running 0, got %d", response.TasksRunning)
	}
	if response.IsPaused != false {
		t.Errorf("expected is_paused false, got %v", response.IsPaused)
	}
}

func TestHandleGetSessionMetrics_WithRunningTasks(t *testing.T) {
	t.Parallel()
	backend := createTestBackend(t)

	// Create tasks with different statuses
	tasks := []*task.Task{
		{
			ID:        "TASK-001",
			Title:     "Running task 1",
			Status:    task.StatusRunning,
			Weight:    task.WeightMedium,
			Execution: task.InitExecutionState(),
		},
		{
			ID:        "TASK-002",
			Title:     "Running task 2",
			Status:    task.StatusRunning,
			Weight:    task.WeightSmall,
			Execution: task.InitExecutionState(),
		},
		{
			ID:        "TASK-003",
			Title:     "Completed task",
			Status:    task.StatusCompleted,
			Weight:    task.WeightMedium,
			Execution: task.InitExecutionState(),
		},
		{
			ID:        "TASK-004",
			Title:     "Paused task",
			Status:    task.StatusPaused,
			Weight:    task.WeightSmall,
			Execution: task.InitExecutionState(),
		},
	}

	for _, tsk := range tasks {
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task %s: %v", tsk.ID, err)
		}
	}

	sessionID := uuid.New().String()
	sessionStart := time.Now().Add(-10 * time.Minute)

	srv := &Server{
		workDir:      t.TempDir(),
		mux:          http.NewServeMux(),
		orcConfig:    config.Default(),
		logger:       testLogger(),
		publisher:    events.NewNopPublisher(),
		backend:      backend,
		sessionID:    sessionID,
		sessionStart: sessionStart,
	}

	// Register route
	srv.mux.HandleFunc("GET /api/session", srv.handleGetSessionMetrics)

	req := httptest.NewRequest("GET", "/api/session", nil)
	w := httptest.NewRecorder()

	srv.handleGetSessionMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response SessionMetricsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify task counts
	if response.TasksRunning != 2 {
		t.Errorf("expected tasks_running 2, got %d", response.TasksRunning)
	}
	if response.TasksCompleted != 1 {
		t.Errorf("expected tasks_completed 1, got %d", response.TasksCompleted)
	}
}

func TestHandleGetSessionMetrics_TokenAggregation(t *testing.T) {
	t.Parallel()
	backend := createTestBackend(t)

	// Create tasks with token usage (some today, some yesterday)
	// Tokens are stored at the PHASE level in task.Execution.Phases.
	// Cost filtering uses task.StartedAt, so we need to set task StartedAt.
	//
	// IMPORTANT: Use times relative to "today at midnight" to avoid day boundary issues.
	// The handler calculates "today" as time.Now().UTC().Truncate(24 * time.Hour), so we
	// must use times that are clearly within today (after midnight) or yesterday (before).
	now := time.Now().UTC()
	today := now.Truncate(24 * time.Hour)
	startedToday := today.Add(2 * time.Hour)      // 2am today (clearly within today)
	startedToday2 := today.Add(4 * time.Hour)     // 4am today (clearly within today)
	startedYesterday := today.Add(-1 * time.Hour) // 11pm yesterday (clearly before today)

	// Create tasks with execution state containing tokens and cost
	task1 := &task.Task{
		ID:        "TASK-001",
		Title:     "Test task 1",
		Status:    task.StatusCompleted,
		Weight:    task.WeightMedium,
		StartedAt: &startedToday,
		Execution: task.InitExecutionState(),
	}
	task1.Execution.Cost.TotalCostUSD = 0.50
	task1.Execution.Phases["implement"] = &task.PhaseState{
		Status:    task.PhaseStatusCompleted,
		StartedAt: startedToday,
		Tokens: task.TokenUsage{
			InputTokens:  1000,
			OutputTokens: 2000,
		},
	}

	task2 := &task.Task{
		ID:        "TASK-002",
		Title:     "Test task 2",
		Status:    task.StatusCompleted,
		Weight:    task.WeightMedium,
		StartedAt: &startedToday2,
		Execution: task.InitExecutionState(),
	}
	task2.Execution.Cost.TotalCostUSD = 0.75
	task2.Execution.Phases["implement"] = &task.PhaseState{
		Status:    task.PhaseStatusCompleted,
		StartedAt: startedToday2,
		Tokens: task.TokenUsage{
			InputTokens:  1500,
			OutputTokens: 2500,
		},
	}

	task3 := &task.Task{
		ID:        "TASK-003",
		Title:     "Test task 3",
		Status:    task.StatusCompleted,
		Weight:    task.WeightMedium,
		StartedAt: &startedYesterday,
		Execution: task.InitExecutionState(),
	}
	task3.Execution.Cost.TotalCostUSD = 2.00
	// Yesterday's phase should not be counted
	task3.Execution.Phases["implement"] = &task.PhaseState{
		Status:    task.PhaseStatusCompleted,
		StartedAt: startedYesterday,
		Tokens: task.TokenUsage{
			InputTokens:  5000,
			OutputTokens: 5000,
		},
	}

	tasks := []*task.Task{task1, task2, task3}
	for _, tsk := range tasks {
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task %s: %v", tsk.ID, err)
		}
	}

	// Verify tasks were saved with execution state
	loadedTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(loadedTasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(loadedTasks))
	}

	sessionID := uuid.New().String()
	sessionStart := time.Now().Add(-1 * time.Hour)

	srv := &Server{
		workDir:      t.TempDir(),
		mux:          http.NewServeMux(),
		orcConfig:    config.Default(),
		logger:       testLogger(),
		publisher:    events.NewNopPublisher(),
		backend:      backend,
		sessionID:    sessionID,
		sessionStart: sessionStart,
	}

	// Register route
	srv.mux.HandleFunc("GET /api/session", srv.handleGetSessionMetrics)

	req := httptest.NewRequest("GET", "/api/session", nil)
	w := httptest.NewRecorder()

	srv.handleGetSessionMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response SessionMetricsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Verify token aggregation (only today's phases - TASK-001 and TASK-002)
	expectedInput := 1000 + 1500  // TASK-001 + TASK-002
	expectedOutput := 2000 + 2500 // TASK-001 + TASK-002
	expectedTotal := expectedInput + expectedOutput
	expectedCost := 0.50 + 0.75 // TASK-001 + TASK-002 (task-level cost)

	if response.InputTokens != expectedInput {
		t.Errorf("expected input_tokens %d, got %d", expectedInput, response.InputTokens)
	}
	if response.OutputTokens != expectedOutput {
		t.Errorf("expected output_tokens %d, got %d", expectedOutput, response.OutputTokens)
	}
	if response.TotalTokens != expectedTotal {
		t.Errorf("expected total_tokens %d, got %d", expectedTotal, response.TotalTokens)
	}
	if response.EstimatedCostUSD != expectedCost {
		t.Errorf("expected estimated_cost_usd %.2f, got %.2f", expectedCost, response.EstimatedCostUSD)
	}
}

func TestHandleGetSessionMetrics_DurationCalculation(t *testing.T) {
	t.Parallel()
	backend := createTestBackend(t)

	sessionID := uuid.New().String()
	sessionStart := time.Now().Add(-30 * time.Minute)

	srv := &Server{
		workDir:      t.TempDir(),
		mux:          http.NewServeMux(),
		orcConfig:    config.Default(),
		logger:       testLogger(),
		publisher:    events.NewNopPublisher(),
		backend:      backend,
		sessionID:    sessionID,
		sessionStart: sessionStart,
	}

	// Register route
	srv.mux.HandleFunc("GET /api/session", srv.handleGetSessionMetrics)

	// Make first request
	req1 := httptest.NewRequest("GET", "/api/session", nil)
	w1 := httptest.NewRecorder()
	srv.handleGetSessionMetrics(w1, req1)

	var response1 SessionMetricsResponse
	if err := json.NewDecoder(w1.Body).Decode(&response1); err != nil {
		t.Fatalf("decode response1: %v", err)
	}

	// Wait long enough to see duration change (need >1 second for int64 seconds to increment)
	time.Sleep(1100 * time.Millisecond)

	// Make second request
	req2 := httptest.NewRequest("GET", "/api/session", nil)
	w2 := httptest.NewRecorder()
	srv.handleGetSessionMetrics(w2, req2)

	var response2 SessionMetricsResponse
	if err := json.NewDecoder(w2.Body).Decode(&response2); err != nil {
		t.Fatalf("decode response2: %v", err)
	}

	// Duration should increase by at least 1 second
	if response2.DurationSeconds <= response1.DurationSeconds {
		t.Errorf("expected duration to increase, got %d then %d",
			response1.DurationSeconds, response2.DurationSeconds)
	}

	// Should be roughly 30 minutes (1800 seconds)
	expectedDuration := int64(1800)
	if response2.DurationSeconds < expectedDuration-5 || response2.DurationSeconds > expectedDuration+5 {
		t.Errorf("expected duration ~%d seconds, got %d", expectedDuration, response2.DurationSeconds)
	}
}
