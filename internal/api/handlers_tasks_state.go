// Package api provides the REST API and SSE server for orc.
package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
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
func (s *Server) handleGetTokens(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
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

// handleGetTranscripts returns task transcript files.
func (s *Server) handleGetTranscripts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify task exists
	if exists, err := s.backend.TaskExists(id); err != nil || !exists {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	// Read transcript files
	transcriptsDir := task.TaskDirIn(s.workDir, id) + "/transcripts"
	entries, err := os.ReadDir(transcriptsDir)
	if err != nil {
		// No transcripts yet is OK
		s.jsonResponse(w, []map[string]any{})
		return
	}

	var transcripts []map[string]any
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(transcriptsDir + "/" + entry.Name())
		if err != nil {
			continue
		}

		info, _ := entry.Info()
		transcripts = append(transcripts, map[string]any{
			"filename":   entry.Name(),
			"content":    string(content),
			"created_at": info.ModTime(),
		})
	}

	// Ensure we return an empty array, not null
	if transcripts == nil {
		transcripts = []map[string]any{}
	}

	s.jsonResponse(w, transcripts)
}
