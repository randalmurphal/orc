package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/task"
)

// DashboardStats represents dashboard statistics.
type DashboardStats struct {
	// Existing fields (backward compatible)
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

	// New computed metrics
	AvgTaskTimeSeconds *float64 `json:"avg_task_time_seconds,omitempty"`
	SuccessRate        *float64 `json:"success_rate,omitempty"`

	// Period comparison
	Period         string              `json:"period,omitempty"`
	PreviousPeriod *PreviousPeriodData `json:"previous_period,omitempty"`
	Changes        *ChangeData         `json:"changes,omitempty"`
}

// PreviousPeriodData represents statistics from the previous period.
type PreviousPeriodData struct {
	Completed          int      `json:"completed"`
	Tokens             int64    `json:"tokens"`
	Cost               float64  `json:"cost"`
	AvgTaskTimeSeconds *float64 `json:"avg_task_time_seconds,omitempty"`
	SuccessRate        *float64 `json:"success_rate,omitempty"`
}

// ChangeData represents percentage changes between current and previous period.
type ChangeData struct {
	CompletedPct *float64 `json:"completed_pct,omitempty"`
	TokensPct    *float64 `json:"tokens_pct,omitempty"`
	CostPct      *float64 `json:"cost_pct,omitempty"`
	AvgTimePct   *float64 `json:"avg_time_pct,omitempty"`
	SuccessRatePct *float64 `json:"success_rate_pct,omitempty"` // Percentage point change
}

// handleGetDashboardStats returns dashboard statistics.
func (s *Server) handleGetDashboardStats(w http.ResponseWriter, r *http.Request) {
	// Parse period parameter (default: 7d)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	// Validate period
	validPeriods := map[string]bool{"24h": true, "7d": true, "30d": true, "all": true}
	if !validPeriods[period] {
		s.jsonError(w, "period must be one of: 24h, 7d, 30d, all", http.StatusBadRequest)
		return
	}

	// Calculate time boundaries
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var periodStart, prevPeriodStart, prevPeriodEnd time.Time
	switch period {
	case "24h":
		periodStart = now.Add(-24 * time.Hour)
		prevPeriodStart = now.Add(-48 * time.Hour)
		prevPeriodEnd = periodStart
	case "7d":
		periodStart = now.Add(-7 * 24 * time.Hour)
		prevPeriodStart = now.Add(-14 * 24 * time.Hour)
		prevPeriodEnd = periodStart
	case "30d":
		periodStart = now.Add(-30 * 24 * time.Hour)
		prevPeriodStart = now.Add(-60 * 24 * time.Hour)
		prevPeriodEnd = periodStart
	case "all":
		periodStart = time.Time{} // Zero time = beginning of time
		prevPeriodStart = time.Time{}
		prevPeriodEnd = time.Time{}
	}

	// Load all tasks
	tasks, err := s.backend.LoadAllTasks()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load tasks: %v", err), http.StatusInternalServerError)
		return
	}

	// Calculate current period stats
	stats := s.calculatePeriodStats(tasks, periodStart, now, today)
	stats.Period = period

	// Calculate previous period stats for comparison (if not "all")
	if period != "all" {
		prevStats := s.calculatePeriodStats(tasks, prevPeriodStart, prevPeriodEnd, today)
		stats.PreviousPeriod = &PreviousPeriodData{
			Completed:          prevStats.Completed,
			Tokens:             prevStats.Tokens,
			Cost:               prevStats.Cost,
			AvgTaskTimeSeconds: prevStats.AvgTaskTimeSeconds,
			SuccessRate:        prevStats.SuccessRate,
		}

		// Calculate percentage changes
		stats.Changes = s.calculateChanges(stats, prevStats)
	}

	s.jsonResponse(w, stats)
}

// calculatePeriodStats computes statistics for a given time period.
// For current period (periodEnd == now), include current status counts.
// For historical periods, only compute completion-based metrics.
func (s *Server) calculatePeriodStats(tasks []*task.Task, periodStart, periodEnd, today time.Time) DashboardStats {
	stats := DashboardStats{}
	isCurrentPeriod := periodEnd.After(time.Now().Add(-1 * time.Second)) // Check if this is the current period

	// Track completed tasks for average time calculation
	var completedTaskTimes []float64

	for _, t := range tasks {
		// Load state for token counts, cost, and orphan detection
		st, err := s.backend.LoadState(t.ID)
		if err != nil {
			// State may not exist for all tasks, continue
			st = nil
		}

		// For current period only, count current status
		if isCurrentPeriod {
			stats.Total++

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
			}

			// Count tasks created or updated today
			if t.CreatedAt.After(today) || (!t.UpdatedAt.IsZero() && t.UpdatedAt.After(today)) {
				stats.Today++
			}
		}

		// Period filtering for completed tasks
		if t.Status == task.StatusCompleted {
			// For "all" period, include all completed tasks
			// For specific periods, check if CompletedAt is within the range
			inPeriod := periodStart.IsZero() || (t.CompletedAt != nil && t.CompletedAt.After(periodStart) && !t.CompletedAt.After(periodEnd))

			if inPeriod {
				stats.Completed++

				// Calculate task duration for average
				if t.StartedAt != nil && t.CompletedAt != nil {
					duration := t.CompletedAt.Sub(*t.StartedAt).Seconds()
					completedTaskTimes = append(completedTaskTimes, duration)
				}

				// Add token counts and cost from state
				if st != nil {
					// Sum tokens from all phases
					// Note: DB only stores InputTokens and OutputTokens per phase
					// CacheCreation and CacheRead tokens are not persisted at phase level
					var inputTokens, outputTokens int
					for _, phase := range st.Phases {
						inputTokens += phase.Tokens.InputTokens
						outputTokens += phase.Tokens.OutputTokens
					}

					stats.Tokens += int64(inputTokens + outputTokens)
					// Cache tokens not available from DB phase storage
					stats.Cost += st.Cost.TotalCostUSD
				}
			}
		}

		// Period filtering for failed tasks
		if t.Status == task.StatusFailed {
			// Note: UpdatedAt is not persisted by database (SaveTask doesn't include it in INSERT)
			// Use CreatedAt as proxy for failure time since tasks typically fail soon after creation
			if periodStart.IsZero() || (t.CreatedAt.After(periodStart) && !t.CreatedAt.After(periodEnd)) {
				stats.Failed++
			}
		}
	}

	// Calculate average task time
	if len(completedTaskTimes) > 0 {
		var sum float64
		for _, t := range completedTaskTimes {
			sum += t
		}
		avg := sum / float64(len(completedTaskTimes))
		stats.AvgTaskTimeSeconds = &avg
	}

	// Calculate success rate
	totalFinished := stats.Completed + stats.Failed
	if totalFinished > 0 {
		rate := float64(stats.Completed) / float64(totalFinished)
		stats.SuccessRate = &rate
	}

	return stats
}

// calculateChanges computes percentage changes between current and previous period.
func (s *Server) calculateChanges(current DashboardStats, previous DashboardStats) *ChangeData {
	changes := &ChangeData{}

	// Helper to calculate percentage change
	pctChange := func(current, previous float64) *float64 {
		if previous == 0 {
			if current == 0 {
				return nil // No change, both zero
			}
			// If previous is 0 but current is not, it's infinite growth
			// Return nil to indicate not calculable
			return nil
		}
		pct := ((current - previous) / previous) * 100
		return &pct
	}

	// Completed tasks change
	if prev := previous.Completed; prev > 0 || current.Completed > 0 {
		changes.CompletedPct = pctChange(float64(current.Completed), float64(prev))
	}

	// Tokens change
	if prev := previous.Tokens; prev > 0 || current.Tokens > 0 {
		changes.TokensPct = pctChange(float64(current.Tokens), float64(prev))
	}

	// Cost change
	if prev := previous.Cost; prev > 0 || current.Cost > 0 {
		changes.CostPct = pctChange(current.Cost, prev)
	}

	// Average time change
	if current.AvgTaskTimeSeconds != nil && previous.AvgTaskTimeSeconds != nil {
		changes.AvgTimePct = pctChange(*current.AvgTaskTimeSeconds, *previous.AvgTaskTimeSeconds)
	}

	// Success rate change (percentage point difference, not percentage of percentage)
	if current.SuccessRate != nil && previous.SuccessRate != nil {
		// Convert rates to percentages (0.942 -> 94.2) then subtract
		diff := (*current.SuccessRate - *previous.SuccessRate) * 100
		changes.SuccessRatePct = &diff
	}

	return changes
}
