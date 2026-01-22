// Package api provides the REST API and SSE server for orc.
package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// handleGetState returns task execution state.
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, err := s.backend.LoadState(id)
	if err != nil {
		s.jsonError(w, "state not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, st)
}

// handleGetPlan returns task plan.
func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := s.backend.LoadPlan(id)
	if err != nil {
		s.jsonError(w, "plan not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, p)
}

// handleGetSession returns session information for a task.
func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, err := s.backend.LoadState(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	if st.Session == nil {
		s.jsonResponse(w, map[string]any{"session": nil})
		return
	}

	s.jsonResponse(w, st.Session)
}

// handleGetTokens returns token usage and cost for a task.
// Prefers DB-aggregated metrics when available, falls back to state.
func (s *Server) handleGetTokens(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Try DB-based metrics first (JSONL-synced data)
	if dbBackend, ok := s.backend.(*storage.DatabaseBackend); ok {
		pdb := dbBackend.DB()
		usage, err := pdb.GetTaskTokenUsage(id)
		if err != nil {
			// Log DB error but fall through to state-based fallback
			s.logger.Debug("db token lookup failed, using state fallback", "task", id, "error", err)
		} else if usage.MessageCount > 0 {
			s.jsonResponse(w, map[string]any{
				"input_tokens":          usage.TotalInput,
				"output_tokens":         usage.TotalOutput,
				"cache_read_tokens":     usage.TotalCacheRead,
				"cache_creation_tokens": usage.TotalCacheCreation,
				"message_count":         usage.MessageCount,
			})
			return
		}
	}

	// Fall back to state-based tokens (legacy or running tasks)
	st, err := s.backend.LoadState(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	s.jsonResponse(w, map[string]any{
		"tokens": st.Tokens,
		"cost":   st.Cost,
	})
}

// handleGetCostSummary returns aggregated cost information with optional period filtering.
// Supports query params:
//   - period: day, week, month, all (default: all)
//   - since: RFC3339 timestamp for custom start date
func (s *Server) handleGetCostSummary(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	period := r.URL.Query().Get("period")
	sinceStr := r.URL.Query().Get("since")

	// Calculate the time range
	var since time.Time
	now := time.Now()

	switch period {
	case "day":
		since = now.AddDate(0, 0, -1)
	case "week":
		since = now.AddDate(0, 0, -7)
	case "month":
		since = now.AddDate(0, -1, 0)
	case "all", "":
		since = time.Time{} // Zero time = no filter
	default:
		// Try parsing custom since parameter
		if sinceStr != "" {
			var err error
			since, err = time.Parse(time.RFC3339, sinceStr)
			if err != nil {
				s.jsonError(w, "invalid 'since' parameter: use RFC3339 format", http.StatusBadRequest)
				return
			}
		}
	}

	// Load all states
	states, err := s.backend.LoadAllStates()
	if err != nil {
		s.jsonError(w, "failed to load task states", http.StatusInternalServerError)
		return
	}

	// Aggregate costs
	var totalCost float64
	var totalInputTokens, totalOutputTokens int
	taskCount := 0
	taskCosts := make([]map[string]any, 0)
	phaseCosts := make(map[string]float64)

	for _, st := range states {
		// Filter by time range if specified
		if !since.IsZero() && st.StartedAt.Before(since) {
			continue
		}

		totalCost += st.Cost.TotalCostUSD
		totalInputTokens += st.Tokens.InputTokens
		totalOutputTokens += st.Tokens.OutputTokens
		taskCount++

		// Track per-task cost
		taskCosts = append(taskCosts, map[string]any{
			"task_id":    st.TaskID,
			"cost_usd":   st.Cost.TotalCostUSD,
			"tokens":     st.Tokens.TotalTokens,
			"started_at": st.StartedAt,
			"status":     st.Status,
		})

		// Aggregate phase costs
		for phase, cost := range st.Cost.PhaseCosts {
			phaseCosts[phase] += cost
		}
	}

	// Check budget threshold from config
	cfg, err := config.Load()
	if err != nil {
		s.logger.Warn("failed to load config for budget check", "error", err)
	}
	var budgetWarning *string
	if cfg != nil && cfg.Budget.ThresholdUSD > 0 && totalCost >= cfg.Budget.ThresholdUSD {
		warning := fmt.Sprintf("Budget threshold of $%.2f reached (current: $%.4f)", cfg.Budget.ThresholdUSD, totalCost)
		budgetWarning = &warning
	}

	response := map[string]any{
		"period":     period,
		"since":      since,
		"task_count": taskCount,
		"total": map[string]any{
			"cost_usd":      totalCost,
			"input_tokens":  totalInputTokens,
			"output_tokens": totalOutputTokens,
			"total_tokens":  totalInputTokens + totalOutputTokens,
		},
		"by_phase": phaseCosts,
		"tasks":    taskCosts,
	}

	if budgetWarning != nil {
		response["budget_warning"] = *budgetWarning
	}

	s.jsonResponse(w, response)
}

// GetSessionMetrics returns current session metrics for WebSocket initial state.
// This is used to provide initial session_update to reconnecting clients.
func (s *Server) GetSessionMetrics() map[string]any {
	var tasksRunning int
	var totalCost float64
	var totalInputTokens, totalOutputTokens int

	// Skip backend queries if not available (e.g., in tests)
	if s.backend != nil {
		// Count running tasks
		tasks, err := s.backend.LoadAllTasks()
		if err != nil {
			s.logger.Warn("failed to load tasks for session metrics", "error", err)
		} else {
			for _, t := range tasks {
				if t.Status == task.StatusRunning {
					tasksRunning++
				}
			}
		}

		// Load today's aggregated state data for cost/tokens
		states, err := s.backend.LoadAllStates()
		if err != nil {
			s.logger.Warn("failed to load states for session metrics", "error", err)
		} else {
			// Only count today's usage
			today := time.Now().UTC().Truncate(24 * time.Hour)
			for _, st := range states {
				if st.StartedAt.After(today) || st.StartedAt.Equal(today) {
					totalCost += st.Cost.TotalCostUSD
					totalInputTokens += st.Tokens.InputTokens
					totalOutputTokens += st.Tokens.OutputTokens
				}
			}
		}
	}

	return map[string]any{
		"duration_seconds":   0, // Session duration is executor-specific, set to 0 for API
		"total_tokens":       totalInputTokens + totalOutputTokens,
		"estimated_cost_usd": totalCost,
		"input_tokens":       totalInputTokens,
		"output_tokens":      totalOutputTokens,
		"tasks_running":      tasksRunning,
		"is_paused":          false, // Pause state is executor-specific
	}
}

// handleGetTranscripts returns task transcripts from the database.
// Supports pagination via query parameters:
//   - limit: max results (default: 50, max: 200)
//   - cursor: transcript ID to start from
//   - direction: 'asc' (default) or 'desc'
//   - phase: filter by phase name
func (s *Server) handleGetTranscripts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify task exists
	if exists, err := s.backend.TaskExists(id); err != nil || !exists {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	// Parse pagination parameters
	query := r.URL.Query()

	// Parse pagination options (defaults applied by DB layer)
	opts := storage.TranscriptPaginationOpts{
		Phase:     query.Get("phase"),
		Direction: query.Get("direction"),
	}

	// Parse limit
	if limitStr := query.Get("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
			s.jsonError(w, "invalid limit parameter", http.StatusBadRequest)
			return
		}
		if limit < 1 || limit > 200 {
			s.jsonError(w, "limit must be between 1 and 200", http.StatusBadRequest)
			return
		}
		opts.Limit = limit
	}

	// Parse cursor
	if cursorStr := query.Get("cursor"); cursorStr != "" {
		var cursor int64
		if _, err := fmt.Sscanf(cursorStr, "%d", &cursor); err != nil {
			s.jsonError(w, "invalid cursor format", http.StatusBadRequest)
			return
		}
		opts.Cursor = cursor
	}

	// Validate direction
	if opts.Direction != "" && opts.Direction != "asc" && opts.Direction != "desc" {
		s.jsonError(w, "direction must be 'asc' or 'desc'", http.StatusBadRequest)
		return
	}

	// Get paginated transcripts
	transcripts, pagination, err := s.backend.GetTranscriptsPaginated(id, opts)
	if err != nil {
		s.jsonError(w, "failed to load transcripts: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get phase summary
	phases, err := s.backend.GetPhaseSummary(id)
	if err != nil {
		s.jsonError(w, "failed to load phase summary: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Transform transcripts to API response format
	transcriptsResponse := make([]map[string]any, len(transcripts))
	for i, t := range transcripts {
		transcriptsResponse[i] = s.transcriptToMap(t)
	}

	// Build response
	response := map[string]any{
		"transcripts": transcriptsResponse,
		"pagination": map[string]any{
			"next_cursor": pagination.NextCursor,
			"prev_cursor": pagination.PrevCursor,
			"has_more":    pagination.HasMore,
			"total_count": pagination.TotalCount,
		},
		"phases": phases,
	}

	s.jsonResponse(w, response)
}

// transcriptToMap converts a storage.Transcript to the API response format.
func (s *Server) transcriptToMap(t storage.Transcript) map[string]any {
	return map[string]any{
		"id":                    t.ID,
		"task_id":               t.TaskID,
		"phase":                 t.Phase,
		"session_id":            t.SessionID,
		"message_uuid":          t.MessageUUID,
		"parent_uuid":           t.ParentUUID,
		"type":                  t.Type,
		"role":                  t.Role,
		"content":               t.Content,
		"model":                 t.Model,
		"input_tokens":          t.InputTokens,
		"output_tokens":         t.OutputTokens,
		"cache_creation_tokens": t.CacheCreationTokens,
		"cache_read_tokens":     t.CacheReadTokens,
		"tool_calls":            t.ToolCalls,
		"tool_results":          t.ToolResults,
		"timestamp":             t.Timestamp,
	}
}
