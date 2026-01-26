// Package api provides the REST API and SSE server for orc.
package api

import (
	"fmt"
	"net/http"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// handleGetState returns task execution state.
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.backend.LoadTaskProto(id)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	exec := t.GetExecution()
	// Return execution state in the format expected by the frontend
	s.jsonResponse(w, map[string]any{
		"task_id":       t.Id,
		"current_phase": task.GetCurrentPhaseProto(t),
		"phases":        exec.GetPhases(),
		"gates":         exec.GetGates(),
		"cost":          exec.GetCost(),
		"retry_context": exec.GetRetryContext(),
	})
}

// handleGetPlan returns task plan (dynamically generated from task weight).
func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Load task to get weight
	t, err := s.backend.LoadTaskProto(id)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Create plan dynamically from task weight
	p := createPlanForWeightState(id, t.Weight)
	s.jsonResponse(w, p)
}

// handleGetSession returns session information for a task.
func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.backend.LoadTaskProto(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	// Session info is now embedded in the task execution state
	// Return minimal session info based on available execution data
	s.jsonResponse(w, map[string]any{
		"task_id":       t.Id,
		"current_phase": task.GetCurrentPhaseProto(t),
		"status":        t.Status.String(),
	})
}

// handleGetTokens returns token usage and cost for a task.
// Prefers DB-aggregated metrics when available, falls back to task execution state.
func (s *Server) handleGetTokens(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Try DB-based metrics first (JSONL-synced data)
	if dbBackend, ok := s.backend.(*storage.DatabaseBackend); ok {
		pdb := dbBackend.DB()
		usage, err := pdb.GetTaskTokenUsage(id)
		if err != nil {
			// Log DB error but fall through to task-based fallback
			s.logger.Debug("db token lookup failed, using task fallback", "task", id, "error", err)
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

	// Fall back to task execution state tokens
	t, err := s.backend.LoadTaskProto(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	// Aggregate tokens from all phases
	var totalInput, totalOutput int
	if exec := t.GetExecution(); exec != nil {
		for _, ps := range exec.GetPhases() {
			if tokens := ps.GetTokens(); tokens != nil {
				totalInput += int(tokens.InputTokens)
				totalOutput += int(tokens.OutputTokens)
			}
		}
	}

	s.jsonResponse(w, map[string]any{
		"tokens": map[string]any{
			"input_tokens":  totalInput,
			"output_tokens": totalOutput,
			"total_tokens":  totalInput + totalOutput,
		},
		"cost": t.GetExecution().GetCost(),
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

	// Load all tasks with execution state
	tasks, err := s.backend.LoadAllTasksProto()
	if err != nil {
		s.jsonError(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}

	// Aggregate costs from task execution state
	var totalCost float64
	var totalInputTokens, totalOutputTokens int
	taskCount := 0
	taskCosts := make([]map[string]any, 0)
	phaseCosts := make(map[string]float64)

	for _, t := range tasks {
		// Filter by time range if specified
		if startedAt := t.GetStartedAt(); !since.IsZero() && startedAt != nil && startedAt.AsTime().Before(since) {
			continue
		}

		exec := t.GetExecution()
		// Aggregate tokens from all phases
		var taskInput, taskOutput int
		if exec != nil {
			for phaseID, ps := range exec.GetPhases() {
				if tokens := ps.GetTokens(); tokens != nil {
					taskInput += int(tokens.InputTokens)
					taskOutput += int(tokens.OutputTokens)
				}
				// Phase costs are not tracked per-phase in new model, skip
				_ = phaseID
			}
		}

		var taskCostUSD float64
		if exec != nil && exec.GetCost() != nil {
			taskCostUSD = exec.GetCost().TotalCostUsd
		}
		totalCost += taskCostUSD
		totalInputTokens += taskInput
		totalOutputTokens += taskOutput
		taskCount++

		// Track per-task cost
		taskCosts = append(taskCosts, map[string]any{
			"task_id":    t.Id,
			"cost_usd":   taskCostUSD,
			"tokens":     taskInput + taskOutput,
			"started_at": t.GetStartedAt().AsTime(),
		})
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
		// Load all tasks - use for both running count and cost/tokens
		tasks, err := s.backend.LoadAllTasksProto()
		if err != nil {
			s.logger.Warn("failed to load tasks for session metrics", "error", err)
		} else {
			// Only count today's usage
			today := time.Now().UTC().Truncate(24 * time.Hour)
			for _, t := range tasks {
				if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
					tasksRunning++
				}

				// Aggregate cost/tokens for tasks started today
				if startedAt := t.GetStartedAt(); startedAt != nil {
					taskStarted := startedAt.AsTime()
					if taskStarted.After(today) || taskStarted.Equal(today) {
						if exec := t.GetExecution(); exec != nil {
							if cost := exec.GetCost(); cost != nil {
								totalCost += cost.TotalCostUsd
							}
							for _, ps := range exec.GetPhases() {
								if tokens := ps.GetTokens(); tokens != nil {
									totalInputTokens += int(tokens.InputTokens)
									totalOutputTokens += int(tokens.OutputTokens)
								}
							}
						}
					}
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

// createPlanForWeightState creates an execution plan based on task weight.
// Plans are created dynamically for execution, not stored.
func createPlanForWeightState(taskID string, weight orcv1.TaskWeight) *executor.Plan {
	var phases []executor.PhaseDisplay

	switch weight {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL:
		phases = []executor.PhaseDisplay{
			{ID: "tiny_spec", Name: "Specification", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case orcv1.TaskWeight_TASK_WEIGHT_SMALL:
		phases = []executor.PhaseDisplay{
			{ID: "tiny_spec", Name: "Specification", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case orcv1.TaskWeight_TASK_WEIGHT_MEDIUM:
		phases = []executor.PhaseDisplay{
			{ID: "spec", Name: "Specification", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		phases = []executor.PhaseDisplay{
			{ID: "spec", Name: "Specification", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "breakdown", Name: "Breakdown", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	default:
		phases = []executor.PhaseDisplay{
			{ID: "spec", Name: "Specification", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: task.PhaseStatusPending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	}

	return &executor.Plan{
		TaskID: taskID,
		Phases: phases,
	}
}
