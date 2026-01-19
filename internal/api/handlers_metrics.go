package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// handleGetMetricsSummary returns aggregated metrics for a time period.
// GET /api/metrics/summary?since=7d
func (s *Server) handleGetMetricsSummary(w http.ResponseWriter, r *http.Request) {
	pdb := s.getProjectDB()
	if pdb == nil {
		s.jsonError(w, "metrics not available", http.StatusServiceUnavailable)
		return
	}

	since := parseSinceParam(r)
	summary, err := pdb.GetMetricsSummary(since)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("get metrics summary: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, summary)
}

// handleGetDailyMetrics returns daily aggregated metrics.
// GET /api/metrics/daily?since=30d
func (s *Server) handleGetDailyMetrics(w http.ResponseWriter, r *http.Request) {
	pdb := s.getProjectDB()
	if pdb == nil {
		s.jsonError(w, "metrics not available", http.StatusServiceUnavailable)
		return
	}

	since := parseSinceParam(r)
	metrics, err := pdb.GetDailyMetrics(since)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("get daily metrics: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, metrics)
}

// handleGetMetricsByModel returns aggregated metrics grouped by model.
// GET /api/metrics/by-model?since=7d
func (s *Server) handleGetMetricsByModel(w http.ResponseWriter, r *http.Request) {
	pdb := s.getProjectDB()
	if pdb == nil {
		s.jsonError(w, "metrics not available", http.StatusServiceUnavailable)
		return
	}

	since := parseSinceParam(r)
	summary, err := pdb.GetMetricsSummary(since)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("get metrics by model: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert map to slice for easier frontend consumption
	// Initialize with capacity to avoid null JSON (returns [] instead of null)
	models := make([]db.ModelMetrics, 0, len(summary.ByModel))
	for _, m := range summary.ByModel {
		models = append(models, m)
	}

	s.jsonResponse(w, models)
}

// handleGetTaskMetrics returns metrics for a specific task.
// GET /api/tasks/{id}/metrics
func (s *Server) handleGetTaskMetrics(w http.ResponseWriter, r *http.Request) {
	pdb := s.getProjectDB()
	if pdb == nil {
		s.jsonError(w, "metrics not available", http.StatusServiceUnavailable)
		return
	}

	taskID := r.PathValue("id")
	metrics, err := pdb.GetTaskMetrics(taskID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("get task metrics: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, metrics)
}

// =============================================================================
// Todo Endpoints
// =============================================================================

// handleGetTaskTodos returns the latest todo snapshot for a task.
// GET /api/tasks/{id}/todos
func (s *Server) handleGetTaskTodos(w http.ResponseWriter, r *http.Request) {
	pdb := s.getProjectDB()
	if pdb == nil {
		s.jsonError(w, "todos not available", http.StatusServiceUnavailable)
		return
	}

	taskID := r.PathValue("id")
	snapshot, err := pdb.GetLatestTodos(taskID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("get task todos: %v", err), http.StatusInternalServerError)
		return
	}

	// Return null if no todos exist (frontend expects nullable)
	s.jsonResponse(w, snapshot)
}

// handleGetTaskTodoHistory returns all todo snapshots for a task.
// GET /api/tasks/{id}/todos/history
func (s *Server) handleGetTaskTodoHistory(w http.ResponseWriter, r *http.Request) {
	pdb := s.getProjectDB()
	if pdb == nil {
		s.jsonError(w, "todos not available", http.StatusServiceUnavailable)
		return
	}

	taskID := r.PathValue("id")
	history, err := pdb.GetTodoHistory(taskID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("get todo history: %v", err), http.StatusInternalServerError)
		return
	}

	// Return empty array if no history (not null)
	if history == nil {
		history = []db.TodoSnapshot{}
	}
	s.jsonResponse(w, history)
}

// =============================================================================
// Helpers
// =============================================================================

// getProjectDB returns the underlying ProjectDB for metrics queries.
// Returns nil if the backend doesn't support direct DB access.
func (s *Server) getProjectDB() *db.ProjectDB {
	if dbBackend, ok := s.backend.(*storage.DatabaseBackend); ok {
		return dbBackend.DB()
	}
	return nil
}

// parseSinceParam parses the "since" query parameter.
// Supports formats like "7d", "30d", "1h", "2w".
// Defaults to 7 days if not specified or invalid.
func parseSinceParam(r *http.Request) time.Time {
	sinceStr := r.URL.Query().Get("since")
	if sinceStr == "" {
		return time.Now().AddDate(0, 0, -7) // Default: 7 days
	}

	// Parse duration string
	var duration time.Duration
	var value int
	var unit rune

	_, err := fmt.Sscanf(sinceStr, "%d%c", &value, &unit)
	if err != nil || value <= 0 {
		return time.Now().AddDate(0, 0, -7) // Default on invalid
	}

	switch unit {
	case 'h', 'H':
		duration = time.Duration(value) * time.Hour
	case 'd', 'D':
		duration = time.Duration(value) * 24 * time.Hour
	case 'w', 'W':
		duration = time.Duration(value) * 7 * 24 * time.Hour
	case 'm', 'M':
		// Approximate months as 30 days
		duration = time.Duration(value) * 30 * 24 * time.Hour
	default:
		return time.Now().AddDate(0, 0, -7) // Default on unknown unit
	}

	return time.Now().Add(-duration)
}
