package api

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
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

// ============================================================================
// Stats Per-Day Endpoint Types
// ============================================================================

// PerDayData represents task count for a single day in the bar chart.
type PerDayData struct {
	Date  string `json:"date"`  // YYYY-MM-DD format
	Day   string `json:"day"`   // Short day name (Mon, Tue, etc.)
	Count int    `json:"count"` // Number of tasks completed
}

// PerDayResponse is the response for GET /api/stats/per-day.
type PerDayResponse struct {
	Period  string       `json:"period"`  // e.g., "7d"
	Data    []PerDayData `json:"data"`    // Daily counts
	Max     int          `json:"max"`     // Highest count in data
	Average float64      `json:"average"` // Average count across all days
}

// ============================================================================
// Stats Per-Day Handler
// ============================================================================

// handleGetPerDayStats returns daily task counts for bar chart visualization.
// GET /api/stats/per-day?days=7
func (s *Server) handleGetPerDayStats(w http.ResponseWriter, r *http.Request) {
	// Parse days parameter (default: 7, max: 30)
	daysStr := r.URL.Query().Get("days")
	days := 7
	if daysStr != "" {
		parsed, err := strconv.Atoi(daysStr)
		if err != nil || parsed < 1 || parsed > 30 {
			s.jsonError(w, "days must be a number between 1 and 30", http.StatusBadRequest)
			return
		}
		days = parsed
	}

	// Calculate date range - most recent day is today
	now := time.Now()
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	startDate := endDate.AddDate(0, 0, -days)

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

	// Fill in all dates in the range (exactly N days)
	data := make([]PerDayData, days)
	totalCount := 0
	maxCount := 0

	for i := 0; i < days; i++ {
		date := startDate.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")
		dayName := date.Format("Mon") // Short day name
		count := activityMap[dateStr]

		data[i] = PerDayData{
			Date:  dateStr,
			Day:   dayName,
			Count: count,
		}

		totalCount += count
		if count > maxCount {
			maxCount = count
		}
	}

	// Calculate average
	average := 0.0
	if days > 0 {
		average = float64(totalCount) / float64(days)
	}

	response := PerDayResponse{
		Period:  strconv.Itoa(days) + "d",
		Data:    data,
		Max:     maxCount,
		Average: average,
	}

	s.jsonResponse(w, response)
}

// ============================================================================
// Stats Outcomes Endpoint Types
// ============================================================================

// OutcomeCount represents a single outcome category with count and percentage.
type OutcomeCount struct {
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// OutcomesResponse is the response for GET /api/stats/outcomes.
type OutcomesResponse struct {
	Period   string                  `json:"period"`
	Total    int                     `json:"total"`
	Outcomes map[string]OutcomeCount `json:"outcomes"`
}

// ============================================================================
// Stats Outcomes Handler
// ============================================================================

// handleGetOutcomesStats returns task outcome distribution for donut chart.
// GET /api/stats/outcomes?period=30d
func (s *Server) handleGetOutcomesStats(w http.ResponseWriter, r *http.Request) {
	// Parse period parameter (24h, 7d, 30d, all) - default: all
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all"
	}

	// Validate period
	var cutoffTime *time.Time
	now := time.Now()
	switch period {
	case "24h":
		t := now.Add(-24 * time.Hour)
		cutoffTime = &t
	case "7d":
		t := now.AddDate(0, 0, -7)
		cutoffTime = &t
	case "30d":
		t := now.AddDate(0, 0, -30)
		cutoffTime = &t
	case "all":
		// No cutoff
		cutoffTime = nil
	default:
		s.jsonError(w, "period must be one of: 24h, 7d, 30d, all", http.StatusBadRequest)
		return
	}

	// Load all tasks
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		s.jsonError(w, "failed to load tasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter tasks by completion time and count outcomes
	var completed, withRetries, failed int

	for _, t := range allTasks {
		// Apply time filter - only include tasks completed within the period
		// Skip tasks with no completion time or completed before the cutoff
		if cutoffTime != nil && (t.CompletedAt == nil || t.CompletedAt.Before(*cutoffTime)) {
			continue
		}

		// Categorize by outcome
		switch t.Status {
		case task.StatusCompleted:
			// Check execution state for retries (execution state is embedded in task)
			exec := &t.Execution

			// Check if any phase has status "failed" (indicating a retry occurred)
			hasRetry := false
			for _, ps := range exec.Phases {
				// A phase that failed and was later retried will have status "failed"
				// in the phase history even if task ultimately completed
				if ps.Status == task.PhaseStatusFailed && ps.Error != "" {
					hasRetry = true
					break
				}
			}

			// Also check if there's retry context
			if exec.RetryContext != nil {
				hasRetry = true
			}

			if hasRetry {
				withRetries++
			} else {
				completed++
			}

		case task.StatusFailed:
			failed++
		}
	}

	// Calculate total and percentages
	total := completed + withRetries + failed
	outcomes := make(map[string]OutcomeCount)

	if total > 0 {
		outcomes["completed"] = OutcomeCount{
			Count:      completed,
			Percentage: float64(completed) / float64(total) * 100,
		}
		outcomes["with_retries"] = OutcomeCount{
			Count:      withRetries,
			Percentage: float64(withRetries) / float64(total) * 100,
		}
		outcomes["failed"] = OutcomeCount{
			Count:      failed,
			Percentage: float64(failed) / float64(total) * 100,
		}
	} else {
		// Return zeros gracefully
		outcomes["completed"] = OutcomeCount{Count: 0, Percentage: 0}
		outcomes["with_retries"] = OutcomeCount{Count: 0, Percentage: 0}
		outcomes["failed"] = OutcomeCount{Count: 0, Percentage: 0}
	}

	response := OutcomesResponse{
		Period:   period,
		Total:    total,
		Outcomes: outcomes,
	}

	s.jsonResponse(w, response)
}

// ============================================================================
// Stats Top Initiatives Endpoint Types
// ============================================================================

// TopInitiativeData represents a single initiative in the leaderboard.
type TopInitiativeData struct {
	Rank           int     `json:"rank"`
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	TaskCount      int     `json:"task_count"`
	CompletedCount int     `json:"completed_count"`
	CompletionRate float64 `json:"completion_rate"`
	TotalTokens    int     `json:"total_tokens"`
	TotalCostUSD   float64 `json:"total_cost_usd"`
}

// TopInitiativesResponse is the response for GET /api/stats/top-initiatives.
type TopInitiativesResponse struct {
	Period      string              `json:"period"`
	Initiatives []TopInitiativeData `json:"initiatives"`
}

// ============================================================================
// Stats Top Initiatives Handler
// ============================================================================

// handleGetTopInitiatives returns most active initiatives for leaderboard.
// GET /api/stats/top-initiatives?limit=10&period=all
func (s *Server) handleGetTopInitiatives(w http.ResponseWriter, r *http.Request) {
	// Parse limit parameter (default: 10, max: 25)
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 1 || parsed > 25 {
			s.jsonError(w, "limit must be a number between 1 and 25", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	// Parse period parameter (24h, 7d, 30d, all) - default: all
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all"
	}

	// Validate period
	var cutoffTime *time.Time
	now := time.Now()
	switch period {
	case "24h":
		t := now.Add(-24 * time.Hour)
		cutoffTime = &t
	case "7d":
		t := now.AddDate(0, 0, -7)
		cutoffTime = &t
	case "30d":
		t := now.AddDate(0, 0, -30)
		cutoffTime = &t
	case "all":
		// No cutoff
		cutoffTime = nil
	default:
		s.jsonError(w, "period must be one of: 24h, 7d, 30d, all", http.StatusBadRequest)
		return
	}

	// Load all initiatives
	initiatives, err := s.backend.LoadAllInitiatives()
	if err != nil {
		s.jsonError(w, "failed to load initiatives: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Load all tasks to build the statistics
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		s.jsonError(w, "failed to load tasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build map of task stats
	taskStatsMap := make(map[string]struct {
		totalTokens  int
		totalCostUSD float64
		isCompleted  bool
		completedAt  *time.Time
	})

	for _, t := range allTasks {
		// Get token and cost data from task's execution state (embedded in task)
		taskStatsMap[t.ID] = struct {
			totalTokens  int
			totalCostUSD float64
			isCompleted  bool
			completedAt  *time.Time
		}{
			totalTokens:  t.Execution.Tokens.TotalTokens,
			totalCostUSD: t.Execution.Cost.TotalCostUSD,
			isCompleted:  t.Status == task.StatusCompleted,
			completedAt:  t.CompletedAt,
		}
	}

	// Calculate stats for each initiative
	type initiativeStats struct {
		initiative     *initiative.Initiative
		taskCount      int
		completedCount int
		totalTokens    int
		totalCostUSD   float64
	}

	var stats []initiativeStats

	for _, init := range initiatives {
		var taskCount, completedCount, totalTokens int
		var totalCostUSD float64

		for _, taskRef := range init.Tasks {
			taskStat, exists := taskStatsMap[taskRef.ID]
			if !exists {
				continue
			}

			// Apply period filter to task completion dates
			if cutoffTime != nil {
				if !taskStat.isCompleted || taskStat.completedAt == nil || taskStat.completedAt.Before(*cutoffTime) {
					continue
				}
			}

			taskCount++
			totalTokens += taskStat.totalTokens
			totalCostUSD += taskStat.totalCostUSD

			if taskStat.isCompleted {
				completedCount++
			}
		}

		// Skip initiatives with 0 tasks (after period filter)
		if taskCount == 0 {
			continue
		}

		stats = append(stats, initiativeStats{
			initiative:     init,
			taskCount:      taskCount,
			completedCount: completedCount,
			totalTokens:    totalTokens,
			totalCostUSD:   totalCostUSD,
		})
	}

	// Sort by task count descending (O(n log n))
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].taskCount > stats[j].taskCount
	})

	// Take top N initiatives (up to limit)
	if len(stats) > limit {
		stats = stats[:limit]
	}

	// Build response
	initiativesData := make([]TopInitiativeData, len(stats))
	for i, stat := range stats {
		completionRate := 0.0
		if stat.taskCount > 0 {
			completionRate = float64(stat.completedCount) / float64(stat.taskCount) * 100
		}

		initiativesData[i] = TopInitiativeData{
			Rank:           i + 1,
			ID:             stat.initiative.ID,
			Title:          stat.initiative.Title,
			TaskCount:      stat.taskCount,
			CompletedCount: stat.completedCount,
			CompletionRate: completionRate,
			TotalTokens:    stat.totalTokens,
			TotalCostUSD:   stat.totalCostUSD,
		}
	}

	response := TopInitiativesResponse{
		Period:      period,
		Initiatives: initiativesData,
	}

	s.jsonResponse(w, response)
}

// ============================================================================
// Stats Top Files Endpoint Types
// ============================================================================

// TopFile represents a single file in the leaderboard.
type TopFile struct {
	Rank              int       `json:"rank"`
	Path              string    `json:"path"`
	ModificationCount int       `json:"modification_count"`
	LastModified      time.Time `json:"last_modified"`
	Tasks             []string  `json:"tasks"`
}

// TopFilesResponse is the response for GET /api/stats/top-files.
type TopFilesResponse struct {
	Period string    `json:"period"`
	Files  []TopFile `json:"files"`
}

// ============================================================================
// Stats Top Files Handler
// ============================================================================

// handleGetTopFiles returns ranked list of most frequently modified files.
// GET /api/stats/top-files?limit=10&period=all
func (s *Server) handleGetTopFiles(w http.ResponseWriter, r *http.Request) {
	// Parse limit parameter (default: 10, max: 50)
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 1 || parsed > 50 {
			s.jsonError(w, "limit must be a number between 1 and 50", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	// Parse period parameter (24h, 7d, 30d, all) - default: all
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "all"
	}

	// Validate period
	var cutoffTime *time.Time
	now := time.Now()
	switch period {
	case "24h":
		t := now.Add(-24 * time.Hour)
		cutoffTime = &t
	case "7d":
		t := now.AddDate(0, 0, -7)
		cutoffTime = &t
	case "30d":
		t := now.AddDate(0, 0, -30)
		cutoffTime = &t
	case "all":
		// No cutoff
		cutoffTime = nil
	default:
		s.jsonError(w, "period must be one of: 24h, 7d, 30d, all", http.StatusBadRequest)
		return
	}

	// Load all tasks
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		s.jsonError(w, "failed to load tasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter to completed tasks within period and aggregate file modifications
	fileAgg := s.aggregateFileModifications(r.Context(), allTasks, cutoffTime)

	// Convert to slice and sort by modification count
	files := make([]TopFile, 0, len(fileAgg))
	for path, info := range fileAgg {
		files = append(files, TopFile{
			Path:              path,
			ModificationCount: info.count,
			LastModified:      info.lastModified,
			Tasks:             info.tasks,
		})
	}

	// Sort by modification count descending
	sortTopFiles(files)

	// Take top N
	if len(files) > limit {
		files = files[:limit]
	}

	// Assign ranks
	for i := range files {
		files[i].Rank = i + 1
	}

	response := TopFilesResponse{
		Period: period,
		Files:  files,
	}

	s.jsonResponse(w, response)
}

// ============================================================================
// Stats Top Files Helpers
// ============================================================================

// fileModInfo tracks aggregation info for a single file.
type fileModInfo struct {
	count        int
	tasks        []string
	lastModified time.Time
}

// aggregateFileModifications aggregates file modification data across tasks.
// Returns a map of file path -> modification info.
func (s *Server) aggregateFileModifications(ctx context.Context, allTasks []*task.Task, cutoffTime *time.Time) map[string]*fileModInfo {
	fileMap := make(map[string]*fileModInfo)

	for _, t := range allTasks {
		// Only include completed tasks
		if t.Status != task.StatusCompleted {
			continue
		}

		// Apply time filter
		if cutoffTime != nil && t.CompletedAt != nil && t.CompletedAt.Before(*cutoffTime) {
			continue
		}

		// Get file list for this task
		files, err := s.getTaskFileList(ctx, t)
		if err != nil {
			// Log and skip tasks where we can't get diff data
			s.logger.Debug("failed to get file list for task", "task", t.ID, "error", err)
			continue
		}

		// Aggregate each file
		for _, file := range files {
			info, exists := fileMap[file.Path]
			if !exists {
				info = &fileModInfo{
					tasks: make([]string, 0, 1),
				}
				fileMap[file.Path] = info
			}

			info.count++
			info.tasks = append(info.tasks, t.ID)

			// Update last modified time
			if t.CompletedAt != nil && (info.lastModified.IsZero() || t.CompletedAt.After(info.lastModified)) {
				info.lastModified = *t.CompletedAt
			}
		}
	}

	return fileMap
}

// getTaskFileList gets the file list for a task using the appropriate diff strategy.
// Consolidates the three diff strategies from handlers_diff.go.
func (s *Server) getTaskFileList(ctx context.Context, t *task.Task) ([]diff.FileDiff, error) {
	diffSvc := diff.NewService(s.getProjectRoot(), s.diffCache)

	// Strategy 1: Merged PR with merge commit SHA
	if t.PR != nil && t.PR.Merged && t.PR.MergeCommitSHA != "" {
		files, _, err := diffSvc.GetMergeCommitFileList(ctx, t.PR.MergeCommitSHA)
		return files, err
	}

	// Strategy 2: Use commit SHAs from task state if available
	firstCommit, lastCommit := s.getTaskCommitRange(t.ID)
	if firstCommit != "" && lastCommit != "" {
		files, _, err := diffSvc.GetCommitRangeFileList(ctx, firstCommit, lastCommit)
		return files, err
	}

	// Strategy 3: Fall back to branch comparison
	base := "main"
	head := t.Branch
	if head == "" {
		head = "HEAD"
	}

	// Resolve refs (handles remote-only branches)
	base = diffSvc.ResolveRef(ctx, base)
	head = diffSvc.ResolveRef(ctx, head)

	// Check if we should include uncommitted working tree changes
	useWorkingTree, effectiveHead := diffSvc.ShouldIncludeWorkingTree(ctx, base, head)
	if useWorkingTree {
		head = effectiveHead // Will be "" to indicate working tree comparison
	}

	return diffSvc.GetFileList(ctx, base, head)
}

// sortTopFiles sorts files by modification count descending, then by path ascending (for stability).
func sortTopFiles(files []TopFile) {
	// Simple bubble sort is fine for typical sizes (10-50 files)
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			// Sort by count descending, then path ascending
			if files[j].ModificationCount > files[i].ModificationCount ||
				(files[j].ModificationCount == files[i].ModificationCount && files[j].Path < files[i].Path) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}
}

// ============================================================================
// Stats Comparison Endpoint Types
// ============================================================================

// PeriodStats represents statistics for a single period.
type PeriodStats struct {
	Tasks       int     `json:"tasks"`
	Tokens      int     `json:"tokens"`
	Cost        float64 `json:"cost"`
	SuccessRate float64 `json:"success_rate"`
}

// ChangeStats represents percentage changes between periods.
type ChangeStats struct {
	Tasks       float64 `json:"tasks"`
	Tokens      float64 `json:"tokens"`
	Cost        float64 `json:"cost"`
	SuccessRate float64 `json:"success_rate"`
}

// ComparisonResponse is the response for GET /api/stats/comparison.
type ComparisonResponse struct {
	Current  PeriodStats `json:"current"`
	Previous PeriodStats `json:"previous"`
	Changes  ChangeStats `json:"changes"`
}

// ============================================================================
// Stats Comparison Handler
// ============================================================================

// handleGetComparisonStats returns comparison between current and previous period.
// GET /api/stats/comparison?period=7d
func (s *Server) handleGetComparisonStats(w http.ResponseWriter, r *http.Request) {
	// Parse period parameter (7d, 30d) - default: 7d
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	// Calculate period duration in days
	var days int
	switch period {
	case "7d":
		days = 7
	case "30d":
		days = 30
	default:
		s.jsonError(w, "period must be one of: 7d, 30d", http.StatusBadRequest)
		return
	}

	// Calculate date ranges
	now := time.Now()
	currentEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	currentStart := currentEnd.AddDate(0, 0, -days)
	previousEnd := currentStart
	previousStart := previousEnd.AddDate(0, 0, -days)

	// Load all tasks
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		s.jsonError(w, "failed to load tasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build task map for accessing execution state (tasks now contain embedded execution state)
	tasksByID := make(map[string]*task.Task, len(allTasks))
	for _, t := range allTasks {
		tasksByID[t.ID] = t
	}

	// Calculate stats for both periods
	// Use calculatePeriodStats from handlers_dashboard.go (requires 5 params: tasks, tasksByID, periodStart, periodEnd, today)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	currentDashStats := s.calculatePeriodStats(allTasks, tasksByID, currentStart, currentEnd, today)
	previousDashStats := s.calculatePeriodStats(allTasks, tasksByID, previousStart, previousEnd, today)

	// Convert DashboardStats to PeriodStats for response
	currentStats := PeriodStats{
		Tasks:       currentDashStats.Completed,
		Tokens:      int(currentDashStats.Tokens),
		Cost:        currentDashStats.Cost,
		SuccessRate: 0, // Will calculate below
	}
	previousStats := PeriodStats{
		Tasks:       previousDashStats.Completed,
		Tokens:      int(previousDashStats.Tokens),
		Cost:        previousDashStats.Cost,
		SuccessRate: 0, // Will calculate below
	}

	// Calculate success rates: completed / (completed + failed) * 100
	currentTotal := currentDashStats.Completed + currentDashStats.Failed
	if currentTotal > 0 {
		currentStats.SuccessRate = (float64(currentDashStats.Completed) / float64(currentTotal)) * 100
	}
	previousTotal := previousDashStats.Completed + previousDashStats.Failed
	if previousTotal > 0 {
		previousStats.SuccessRate = (float64(previousDashStats.Completed) / float64(previousTotal)) * 100
	}

	// Calculate percentage changes
	changes := ChangeStats{
		Tasks:       calculatePercentageChange(float64(previousStats.Tasks), float64(currentStats.Tasks)),
		Tokens:      calculatePercentageChange(float64(previousStats.Tokens), float64(currentStats.Tokens)),
		Cost:        calculatePercentageChange(previousStats.Cost, currentStats.Cost),
		SuccessRate: calculatePercentageChange(previousStats.SuccessRate, currentStats.SuccessRate),
	}

	response := ComparisonResponse{
		Current:  currentStats,
		Previous: previousStats,
		Changes:  changes,
	}

	s.jsonResponse(w, response)
}

// calculatePercentageChange computes percentage change from previous to current.
// Returns ((current - previous) / previous) * 100
func calculatePercentageChange(previous, current float64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		// Previous was 0, current is non-zero: treat as 100% increase
		return 100
	}
	return ((current - previous) / previous) * 100
}
