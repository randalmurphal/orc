package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/task"
)

// DashboardStats represents dashboard statistics.
type DashboardStats struct {
	Running                  int     `json:"running"`
	Orphaned                 int     `json:"orphaned"`
	Paused                   int     `json:"paused"`
	Blocked                  int     `json:"blocked"`
	Completed                int     `json:"completed"`
	Failed                   int     `json:"failed"`
	Today                    int     `json:"today"`
	Total                    int     `json:"total"`
	Tokens                   int64   `json:"tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens,omitempty"`
	Cost                     float64 `json:"cost"`
}

// handleGetDashboardStats returns dashboard statistics.
func (s *Server) handleGetDashboardStats(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.backend.LoadAllTasks()
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
		// Load state for token counts and orphan detection
		st, _ := s.backend.LoadState(t.ID)

		switch t.Status {
		case task.StatusRunning:
			// Check if task is orphaned
			if st != nil {
				if isOrphaned, _ := st.CheckOrphaned(); isOrphaned {
					stats.Orphaned++
				} else {
					stats.Running++
				}
			} else {
				stats.Running++
			}
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

		// Add token counts from state
		if st != nil {
			stats.Tokens += int64(st.Tokens.TotalTokens)
			stats.CacheCreationInputTokens += int64(st.Tokens.CacheCreationInputTokens)
			stats.CacheReadInputTokens += int64(st.Tokens.CacheReadInputTokens)
		}
	}

	s.jsonResponse(w, stats)
}
