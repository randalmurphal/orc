package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/task"
)

// SessionMetricsResponse represents the response for GET /api/session.
type SessionMetricsResponse struct {
	SessionID        string    `json:"session_id"`
	StartedAt        time.Time `json:"started_at"`
	DurationSeconds  int64     `json:"duration_seconds"`
	TotalTokens      int       `json:"total_tokens"`
	InputTokens      int       `json:"input_tokens"`
	OutputTokens     int       `json:"output_tokens"`
	EstimatedCostUSD float64   `json:"estimated_cost_usd"`
	TasksCompleted   int       `json:"tasks_completed"`
	TasksRunning     int       `json:"tasks_running"`
	IsPaused         bool      `json:"is_paused"`
}

// handleGetSessionMetrics returns current session metrics for the TopBar.
// GET /api/session
//
// Response includes:
//   - session_id: UUID generated at server startup
//   - started_at: Server start time
//   - duration_seconds: Time elapsed since server start
//   - total_tokens: Sum of input + output tokens for today
//   - input_tokens: Total input tokens for today
//   - output_tokens: Total output tokens for today
//   - estimated_cost_usd: Total cost for today
//   - tasks_completed: Count of completed tasks
//   - tasks_running: Count of running tasks
//   - is_paused: Always false (executor-level pause not exposed to API)
func (s *Server) handleGetSessionMetrics(w http.ResponseWriter, r *http.Request) {
	// Calculate duration since server start
	duration := int64(time.Since(s.sessionStart).Seconds())

	// Load all tasks
	tasks, err := s.backend.LoadAllTasks()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load tasks: %v", err), http.StatusInternalServerError)
		return
	}

	// Count tasks by status and aggregate today's token/cost data
	// Tokens are stored at the phase level in task.Execution.Phases
	today := time.Now().UTC().Truncate(24 * time.Hour)
	var running, completed int
	var totalInput, totalOutput int
	var totalCost float64
	for _, t := range tasks {
		switch t.Status {
		case task.StatusRunning:
			running++
		case task.StatusCompleted:
			completed++
		}

		// Aggregate tokens from phases that started today
		// Skip if task has no phases (uninitialized execution state)
		for _, ps := range t.Execution.Phases {
			if ps != nil && !ps.StartedAt.IsZero() && (ps.StartedAt.After(today) || ps.StartedAt.Equal(today)) {
				totalInput += ps.Tokens.InputTokens
				totalOutput += ps.Tokens.OutputTokens
			}
		}
		// Cost is tracked at the task level
		if t.StartedAt != nil && (t.StartedAt.After(today) || t.StartedAt.Equal(today)) {
			totalCost += t.Execution.Cost.TotalCostUSD
		}
	}

	response := SessionMetricsResponse{
		SessionID:        s.sessionID,
		StartedAt:        s.sessionStart,
		DurationSeconds:  duration,
		TotalTokens:      totalInput + totalOutput,
		InputTokens:      totalInput,
		OutputTokens:     totalOutput,
		EstimatedCostUSD: totalCost,
		TasksCompleted:   completed,
		TasksRunning:     running,
		IsPaused:         false, // Executor-level pause not exposed to API
	}

	s.jsonResponse(w, response)
}
