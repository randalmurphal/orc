package api

import (
	"net/http"
	"strconv"
	"time"
)

// ============================================================================
// Stats Activity Endpoint Types
// ============================================================================

// ActivityDay represents task activity for a single date.
type ActivityDay struct {
	Date  string `json:"date"`  // YYYY-MM-DD format
	Count int    `json:"count"` // Number of tasks completed
	Level int    `json:"level"` // Activity level 0-4
}

// BusiestDay represents the day with the most task completions.
type BusiestDay struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// ActivityStats contains aggregate statistics for the activity data.
type ActivityStats struct {
	TotalTasks    int         `json:"total_tasks"`
	CurrentStreak int         `json:"current_streak"`
	LongestStreak int         `json:"longest_streak"`
	BusiestDay    *BusiestDay `json:"busiest_day"`
}

// ActivityResponse is the response for GET /api/stats/activity.
type ActivityResponse struct {
	StartDate string        `json:"start_date"`
	EndDate   string        `json:"end_date"`
	Data      []ActivityDay `json:"data"`
	Stats     ActivityStats `json:"stats"`
}

// ============================================================================
// Stats Activity Handler
// ============================================================================

// handleGetActivityStats returns task activity data for heatmap visualization.
// GET /api/stats/activity?weeks=16
func (s *Server) handleGetActivityStats(w http.ResponseWriter, r *http.Request) {
	// Parse weeks parameter (default: 16, max: 52)
	weeksStr := r.URL.Query().Get("weeks")
	weeks := 16
	if weeksStr != "" {
		parsed, err := strconv.Atoi(weeksStr)
		if err != nil || parsed < 1 || parsed > 52 {
			s.jsonError(w, "weeks must be a number between 1 and 52", http.StatusBadRequest)
			return
		}
		weeks = parsed
	}

	// Calculate date range
	now := time.Now()
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	startDate := endDate.AddDate(0, 0, -weeks*7)

	// Query activity data from database
	activityData, err := s.backend.GetTaskActivityByDate(
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
	)
	if err != nil {
		s.jsonError(w, "failed to load activity data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Create a map for quick lookup
	activityMap := make(map[string]int)
	for _, ac := range activityData {
		activityMap[ac.Date] = ac.Count
	}

	// Fill in all dates in the range
	totalDays := weeks * 7
	data := make([]ActivityDay, totalDays)
	totalTasks := 0
	var busiestDay *BusiestDay

	for i := 0; i < totalDays; i++ {
		date := startDate.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")
		count := activityMap[dateStr]
		level := calculateActivityLevel(count)

		data[i] = ActivityDay{
			Date:  dateStr,
			Count: count,
			Level: level,
		}

		totalTasks += count

		// Track busiest day
		if count > 0 && (busiestDay == nil || count > busiestDay.Count) {
			busiestDay = &BusiestDay{
				Date:  dateStr,
				Count: count,
			}
		}
	}

	// Calculate streaks
	currentStreak, longestStreak := calculateStreaks(data, now)

	response := ActivityResponse{
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.AddDate(0, 0, -1).Format("2006-01-02"), // Inclusive end date
		Data:      data,
		Stats: ActivityStats{
			TotalTasks:    totalTasks,
			CurrentStreak: currentStreak,
			LongestStreak: longestStreak,
			BusiestDay:    busiestDay,
		},
	}

	s.jsonResponse(w, response)
}

// calculateActivityLevel returns the activity level (0-4) based on task count.
// Thresholds: 0=none, 1=1-2, 2=3-5, 3=6-10, 4=11+
func calculateActivityLevel(count int) int {
	switch {
	case count == 0:
		return 0
	case count <= 2:
		return 1
	case count <= 5:
		return 2
	case count <= 10:
		return 3
	default:
		return 4
	}
}

// calculateStreaks calculates current and longest streaks from activity data.
// Current streak: consecutive days ending today (or yesterday if today has no activity yet)
// Longest streak: maximum consecutive days with activity in the range
func calculateStreaks(data []ActivityDay, now time.Time) (current, longest int) {
	if len(data) == 0 {
		return 0, 0
	}

	// Calculate longest streak by iterating forward
	currentRun := 0
	for _, day := range data {
		if day.Count > 0 {
			currentRun++
			if currentRun > longest {
				longest = currentRun
			}
		} else {
			currentRun = 0
		}
	}

	// Calculate current streak by iterating backwards from today
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	// Build a map for quick date lookup
	dateToIdx := make(map[string]int, len(data))
	for i, day := range data {
		dateToIdx[day.Date] = i
	}

	// Find the starting point for current streak calculation
	// Start from today if it has activity, otherwise from yesterday
	startIdx := -1

	// Check today first
	if idx, ok := dateToIdx[today]; ok && data[idx].Count > 0 {
		startIdx = idx
	} else if idx, ok := dateToIdx[yesterday]; ok && data[idx].Count > 0 {
		// Today has no activity (or isn't in data), check yesterday
		startIdx = idx
	}

	if startIdx < 0 {
		return 0, longest
	}

	// Count consecutive days with activity going backwards
	for i := startIdx; i >= 0; i-- {
		if data[i].Count > 0 {
			current++
		} else {
			break
		}
	}

	return current, longest
}
