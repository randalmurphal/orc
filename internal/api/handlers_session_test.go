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
	"github.com/randalmurphal/orc/internal/state"
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
			ID:     "TASK-001",
			Title:  "Running task 1",
			Status: task.StatusRunning,
			Weight: task.WeightMedium,
		},
		{
			ID:     "TASK-002",
			Title:  "Running task 2",
			Status: task.StatusRunning,
			Weight: task.WeightSmall,
		},
		{
			ID:     "TASK-003",
			Title:  "Completed task",
			Status: task.StatusCompleted,
			Weight: task.WeightMedium,
		},
		{
			ID:     "TASK-004",
			Title:  "Paused task",
			Status: task.StatusPaused,
			Weight: task.WeightSmall,
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

	// Create states with token usage (some today, some yesterday)
	// Note: Tokens are stored at the PHASE level in the database, not state level.
	// Note: Cost filtering uses state.StartedAt which comes from dbTask.StartedAt,
	// so we need to set task StartedAt when creating tasks.
	now := time.Now().UTC()
	startedToday := now.Add(-1 * time.Hour)
	startedYesterday := now.Add(-25 * time.Hour)

	// Create tasks with proper StartedAt times
	tasks := []*task.Task{
		{ID: "TASK-001", Title: "Test task 1", Status: task.StatusCompleted, Weight: task.WeightMedium, StartedAt: &startedToday},
		{ID: "TASK-002", Title: "Test task 2", Status: task.StatusCompleted, Weight: task.WeightMedium, StartedAt: &startedToday},
		{ID: "TASK-003", Title: "Test task 3", Status: task.StatusCompleted, Weight: task.WeightMedium, StartedAt: &startedYesterday},
	}
	for _, tsk := range tasks {
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task %s: %v", tsk.ID, err)
		}
	}

	// Create properly initialized states with phase-level tokens
	st1 := state.New("TASK-001")
	st1.Status = state.StatusCompleted
	st1.StartedAt = now.Add(-1 * time.Hour) // 1 hour ago (today)
	st1.UpdatedAt = now.Add(-1 * time.Hour)
	st1.Cost = state.CostTracking{
		TotalCostUSD: 0.50,
	}
	// Add a phase with tokens (this is where tokens are actually stored)
	st1.Phases["implement"] = &state.PhaseState{
		Status:    state.StatusCompleted,
		StartedAt: now.Add(-1 * time.Hour),
		Tokens: state.TokenUsage{
			InputTokens:  1000,
			OutputTokens: 2000,
		},
	}

	st2 := state.New("TASK-002")
	st2.Status = state.StatusCompleted
	st2.StartedAt = now.Add(-2 * time.Hour) // 2 hours ago (today)
	st2.UpdatedAt = now.Add(-2 * time.Hour)
	st2.Cost = state.CostTracking{
		TotalCostUSD: 0.75,
	}
	// Add a phase with tokens
	st2.Phases["implement"] = &state.PhaseState{
		Status:    state.StatusCompleted,
		StartedAt: now.Add(-2 * time.Hour),
		Tokens: state.TokenUsage{
			InputTokens:  1500,
			OutputTokens: 2500,
		},
	}

	st3 := state.New("TASK-003")
	st3.Status = state.StatusCompleted
	st3.StartedAt = now.Add(-25 * time.Hour) // 25 hours ago (yesterday, should not be counted)
	st3.UpdatedAt = now.Add(-25 * time.Hour)
	st3.Cost = state.CostTracking{
		TotalCostUSD: 2.00,
	}
	// Add a phase with tokens - yesterday's phase should not be counted
	st3.Phases["implement"] = &state.PhaseState{
		Status:    state.StatusCompleted,
		StartedAt: now.Add(-25 * time.Hour),
		Tokens: state.TokenUsage{
			InputTokens:  5000,
			OutputTokens: 5000,
		},
	}

	states := []*state.State{st1, st2, st3}

	for _, st := range states {
		if err := backend.SaveState(st); err != nil {
			t.Fatalf("save state %s: %v", st.TaskID, err)
		}
	}

	// Verify states were saved
	loadedStates, err := backend.LoadAllStates()
	if err != nil {
		t.Fatalf("load states: %v", err)
	}
	if len(loadedStates) != 3 {
		t.Fatalf("expected 3 states, got %d", len(loadedStates))
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
	expectedCost := 0.50 + 0.75 // TASK-001 + TASK-002 (state-level cost)

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
