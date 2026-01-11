package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// DashboardStats represents dashboard statistics.
type DashboardStats struct {
	Running   int     `json:"running"`
	Paused    int     `json:"paused"`
	Blocked   int     `json:"blocked"`
	Completed int     `json:"completed"`
	Failed    int     `json:"failed"`
	Today     int     `json:"today"`
	Total     int     `json:"total"`
	Tokens    int64   `json:"tokens"`
	Cost      float64 `json:"cost"`
}

// handleGetDashboardStats returns dashboard statistics.
func (s *Server) handleGetDashboardStats(w http.ResponseWriter, r *http.Request) {
	tasks, err := task.LoadAll()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load tasks: %v", err), http.StatusInternalServerError)
		return
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	stats := DashboardStats{
		Total: len(tasks),
	}

	for _, t := range tasks {
		switch t.Status {
		case task.StatusRunning:
			stats.Running++
		case task.StatusPaused:
			stats.Paused++
		case task.StatusBlocked:
			stats.Blocked++
		case task.StatusCompleted:
			stats.Completed++
		case task.StatusFailed:
			stats.Failed++
		}

		// Count tasks created or updated today
		if t.CreatedAt.After(today) || (!t.UpdatedAt.IsZero() && t.UpdatedAt.After(today)) {
			stats.Today++
		}

		// Load state for token counts
		if st, err := state.Load(t.ID); err == nil && st != nil {
			stats.Tokens += int64(st.Tokens.TotalTokens)
		}
	}

	s.jsonResponse(w, stats)
}
