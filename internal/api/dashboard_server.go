// Package api provides the Connect RPC and REST API server for orc.
// This file implements the DashboardService Connect RPC service.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// DiffServicer defines the interface for diff operations needed by GetTopFiles.
// This allows for testing with mock implementations.
type DiffServicer interface {
	GetFileList(ctx context.Context, base, head string) ([]diff.FileDiff, error)
}

// dashboardServer implements the DashboardServiceHandler interface.
type dashboardServer struct {
	orcv1connect.UnimplementedDashboardServiceHandler
	backend      storage.Backend
	projectCache *ProjectCache
	cache        *dashboardCache
	logger       *slog.Logger
	diffSvc      DiffServicer
}

// NewDashboardServer creates a new DashboardService handler.
func NewDashboardServer(
	backend storage.Backend,
	logger *slog.Logger,
) orcv1connect.DashboardServiceHandler {
	return &dashboardServer{
		backend: backend,
		cache:   newDashboardCache(backend, 30*time.Second),
		logger:  logger,
	}
}

// NewDashboardServerWithDiff creates a DashboardService handler with an injected diff service.
// This is primarily used for testing with mock diff services.
func NewDashboardServerWithDiff(
	backend storage.Backend,
	logger *slog.Logger,
	diffSvc DiffServicer,
) *dashboardServer {
	return &dashboardServer{
		backend: backend,
		cache:   newDashboardCache(backend, 30*time.Second),
		logger:  logger,
		diffSvc: diffSvc,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *dashboardServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
// If projectID is provided and projectCache is available, uses the cache.
// Errors if projectID is provided but cache is not configured (prevents silent data leaks).
// Falls back to legacy single backend only when no projectID is specified.
func (s *dashboardServer) getBackend(projectID string) (storage.Backend, error) {
	if projectID != "" && s.projectCache != nil {
		return s.projectCache.GetBackend(projectID)
	}
	if projectID != "" && s.projectCache == nil {
		return nil, fmt.Errorf("project_id specified but no project cache configured")
	}
	if s.backend == nil {
		return nil, fmt.Errorf("no backend available")
	}
	return s.backend, nil
}

// getTasks returns tasks from cache or directly from backend for a project.
// When projectID is empty, uses the cached tasks from the default backend.
// When projectID is specified, bypasses the cache and loads directly from that project's backend.
func (s *dashboardServer) getTasks(projectID string) ([]*orcv1.Task, error) {
	if projectID == "" {
		return s.cache.Tasks()
	}
	backend, err := s.getBackend(projectID)
	if err != nil {
		return nil, err
	}
	return backend.LoadAllTasks()
}

// GetStats returns dashboard statistics.
func (s *dashboardServer) GetStats(
	ctx context.Context,
	req *connect.Request[orcv1.GetStatsRequest],
) (*connect.Response[orcv1.GetStatsResponse], error) {
	tasks, err := s.getTasks(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Calculate status counts
	statusCounts := &orcv1.StatusCounts{}
	var runningTasks []*orcv1.RunningTaskInfo
	var recentCompletions []*orcv1.RecentCompletion
	var todayCost float64
	todayTokens := &orcv1.TokenUsage{}

	for _, t := range tasks {
		statusCounts.All++

		switch t.Status {
		case orcv1.TaskStatus_TASK_STATUS_RUNNING:
			statusCounts.Running++
			statusCounts.Active++
			runningTasks = append(runningTasks, &orcv1.RunningTaskInfo{
				Id:           t.Id,
				Title:        t.Title,
				CurrentPhase: ptrStringValue(t.CurrentPhase),
				Iteration:    int32(t.Execution.CurrentIteration),
				StartedAt:    t.StartedAt,
			})
		case orcv1.TaskStatus_TASK_STATUS_PLANNED, orcv1.TaskStatus_TASK_STATUS_CREATED:
			statusCounts.Active++
		case orcv1.TaskStatus_TASK_STATUS_PAUSED:
			statusCounts.Active++
		case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
			statusCounts.Blocked++
			statusCounts.Active++
		case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
			statusCounts.Completed++
			if t.CompletedAt != nil && t.CompletedAt.AsTime().After(today.Add(-7*24*time.Hour)) {
				recentCompletions = append(recentCompletions, &orcv1.RecentCompletion{
					Id:          t.Id,
					Title:       t.Title,
					Success:     true,
					CompletedAt: t.CompletedAt,
				})
			}
		case orcv1.TaskStatus_TASK_STATUS_FAILED:
			statusCounts.Failed++
			if t.UpdatedAt != nil && t.UpdatedAt.AsTime().After(today.Add(-7*24*time.Hour)) {
				recentCompletions = append(recentCompletions, &orcv1.RecentCompletion{
					Id:          t.Id,
					Title:       t.Title,
					Success:     false,
					CompletedAt: t.UpdatedAt,
				})
			}
		}

		// Aggregate today's tokens and cost
		if (t.CreatedAt != nil && t.CreatedAt.AsTime().After(today)) || (t.UpdatedAt != nil && t.UpdatedAt.AsTime().After(today)) {
			for _, phase := range t.Execution.Phases {
				todayTokens.InputTokens += phase.Tokens.InputTokens
				todayTokens.OutputTokens += phase.Tokens.OutputTokens
				todayTokens.CacheCreationInputTokens += phase.Tokens.CacheCreationInputTokens
				todayTokens.CacheReadInputTokens += phase.Tokens.CacheReadInputTokens
			}
			todayTokens.TotalTokens = todayTokens.InputTokens + todayTokens.OutputTokens
			todayCost += t.Execution.Cost.TotalCostUsd
		}
	}

	// Sort recent completions by date (newest first), limit to 10
	sort.Slice(recentCompletions, func(i, j int) bool {
		return recentCompletions[i].CompletedAt.AsTime().After(recentCompletions[j].CompletedAt.AsTime())
	})
	if len(recentCompletions) > 10 {
		recentCompletions = recentCompletions[:10]
	}

	stats := &orcv1.DashboardStats{
		TaskCounts:        statusCounts,
		RunningTasks:      runningTasks,
		RecentCompletions: recentCompletions,
		PendingDecisions:  0, // Would need access to pendingDecisions store
		TodayTokens:       todayTokens,
		TodayCostUsd:      todayCost,
	}

	return connect.NewResponse(&orcv1.GetStatsResponse{
		Stats: stats,
	}), nil
}

// GetActivityHeatmap returns activity heatmap data.
func (s *dashboardServer) GetActivityHeatmap(
	ctx context.Context,
	req *connect.Request[orcv1.GetActivityHeatmapRequest],
) (*connect.Response[orcv1.GetActivityHeatmapResponse], error) {
	days := req.Msg.Days
	if days <= 0 {
		days = 90 // Default to 90 days
	}

	tasks, err := s.getTasks(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	now := time.Now()
	startDate := now.Add(-time.Duration(days) * 24 * time.Hour)

	// Aggregate by date
	dayMap := make(map[string]*orcv1.ActivityDay)

	for _, t := range tasks {
		if t.CompletedAt != nil && t.CompletedAt.AsTime().After(startDate) {
			dateStr := t.CompletedAt.AsTime().Format("2006-01-02")
			if dayMap[dateStr] == nil {
				dayMap[dateStr] = &orcv1.ActivityDay{Date: dateStr}
			}
			dayMap[dateStr].TasksCompleted++

			// Count phases completed
			dayMap[dateStr].PhasesCompleted += int32(len(t.Execution.Phases))

			// Sum tokens
			for _, phase := range t.Execution.Phases {
				dayMap[dateStr].Tokens += int32(phase.Tokens.InputTokens + phase.Tokens.OutputTokens)
			}
		}
	}

	// Convert to sorted slice
	var activityDays []*orcv1.ActivityDay
	for _, day := range dayMap {
		activityDays = append(activityDays, day)
	}
	sort.Slice(activityDays, func(i, j int) bool {
		return activityDays[i].Date < activityDays[j].Date
	})

	return connect.NewResponse(&orcv1.GetActivityHeatmapResponse{
		Heatmap: &orcv1.ActivityHeatmap{
			Days: activityDays,
		},
	}), nil
}

// GetCostSummary returns cost summary.
func (s *dashboardServer) GetCostSummary(
	ctx context.Context,
	req *connect.Request[orcv1.GetCostSummaryRequest],
) (*connect.Response[orcv1.GetCostSummaryResponse], error) {
	tasks, err := s.getTasks(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	// Determine time filter
	now := time.Now()
	var since time.Time
	switch req.Msg.Period {
	case "day":
		since = now.Add(-24 * time.Hour)
	case "week":
		since = now.Add(-7 * 24 * time.Hour)
	case "month":
		since = now.Add(-30 * 24 * time.Hour)
	default:
		since = time.Time{} // All time
	}

	// Calculate cost summary
	var totalCost float64
	byModel := make(map[string]float64)
	byCategory := make(map[string]float64)
	periodCosts := make(map[string]float64)

	for _, t := range tasks {
		// Filter by time
		var taskTime time.Time
		if t.CompletedAt != nil {
			taskTime = t.CompletedAt.AsTime()
		} else if t.UpdatedAt != nil {
			taskTime = t.UpdatedAt.AsTime()
		}
		if !since.IsZero() && taskTime.Before(since) {
			continue
		}

		cost := t.Execution.Cost.TotalCostUsd
		totalCost += cost

		// By category
		byCategory[task.CategoryFromProto(t.Category)] += cost

		// By period (day)
		periodKey := taskTime.Format("2006-01-02")
		periodCosts[periodKey] += cost

		// Note: Model tracking per-phase not available in orcv1.PhaseState
		// Cost tracking is at task level via t.Execution.Cost
	}

	// Convert period costs to sorted slice
	var periodCostsList []*orcv1.PeriodCost
	for date, cost := range periodCosts {
		periodCostsList = append(periodCostsList, &orcv1.PeriodCost{
			Period:  "day",
			Label:   date,
			CostUsd: cost,
		})
	}
	sort.Slice(periodCostsList, func(i, j int) bool {
		return periodCostsList[i].Label < periodCostsList[j].Label
	})

	return connect.NewResponse(&orcv1.GetCostSummaryResponse{
		Summary: &orcv1.CostSummary{
			TotalCostUsd: totalCost,
			ByPeriod:     periodCostsList,
			ByModel:      byModel,
			ByCategory:   byCategory,
		},
	}), nil
}

// GetMetrics returns metrics summary.
func (s *dashboardServer) GetMetrics(
	ctx context.Context,
	req *connect.Request[orcv1.GetMetricsRequest],
) (*connect.Response[orcv1.GetMetricsResponse], error) {
	tasks, err := s.getTasks(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	// Determine time filter
	now := time.Now()
	var since time.Time
	if req.Msg.Period != nil {
		switch *req.Msg.Period {
		case "day":
			since = now.Add(-24 * time.Hour)
		case "week":
			since = now.Add(-7 * 24 * time.Hour)
		case "month":
			since = now.Add(-30 * 24 * time.Hour)
		}
	}

	metrics := &orcv1.MetricsSummary{
		TotalTokens: &orcv1.TokenUsage{},
	}

	var completedCount, failedCount int
	var totalDuration float64
	var durationCount int

	for _, t := range tasks {
		// Filter by time
		var taskTime time.Time
		if t.CompletedAt != nil {
			taskTime = t.CompletedAt.AsTime()
		} else if t.UpdatedAt != nil {
			taskTime = t.UpdatedAt.AsTime()
		}
		if !since.IsZero() && taskTime.Before(since) {
			continue
		}

		switch t.Status {
		case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
			completedCount++
			if t.StartedAt != nil && t.CompletedAt != nil {
				totalDuration += t.CompletedAt.AsTime().Sub(t.StartedAt.AsTime()).Seconds()
				durationCount++
			}
		case orcv1.TaskStatus_TASK_STATUS_FAILED:
			failedCount++
		}

		metrics.PhasesExecuted += int32(len(t.Execution.Phases))

		for _, phase := range t.Execution.Phases {
			metrics.TotalTokens.InputTokens += phase.Tokens.InputTokens
			metrics.TotalTokens.OutputTokens += phase.Tokens.OutputTokens
			metrics.TotalTokens.CacheCreationInputTokens += phase.Tokens.CacheCreationInputTokens
			metrics.TotalTokens.CacheReadInputTokens += phase.Tokens.CacheReadInputTokens
		}
	}

	metrics.TotalTokens.TotalTokens = metrics.TotalTokens.InputTokens + metrics.TotalTokens.OutputTokens
	metrics.TasksCompleted = int32(completedCount)

	if durationCount > 0 {
		metrics.AvgTaskDurationSeconds = totalDuration / float64(durationCount)
	}

	totalFinished := completedCount + failedCount
	if totalFinished > 0 {
		metrics.SuccessRate = float64(completedCount) / float64(totalFinished)
	}

	return connect.NewResponse(&orcv1.GetMetricsResponse{
		Metrics: metrics,
	}), nil
}

// GetDailyMetrics returns daily metrics.
func (s *dashboardServer) GetDailyMetrics(
	ctx context.Context,
	req *connect.Request[orcv1.GetDailyMetricsRequest],
) (*connect.Response[orcv1.GetDailyMetricsResponse], error) {
	days := req.Msg.Days
	if days <= 0 {
		days = 30
	}

	tasks, err := s.getTasks(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	now := time.Now()
	startDate := now.Add(-time.Duration(days) * 24 * time.Hour)

	// Aggregate by date
	dayMap := make(map[string]*orcv1.DailyMetrics)

	for _, t := range tasks {
		// Check creation date
		if t.CreatedAt != nil && t.CreatedAt.AsTime().After(startDate) {
			dateStr := t.CreatedAt.AsTime().Format("2006-01-02")
			if dayMap[dateStr] == nil {
				dayMap[dateStr] = &orcv1.DailyMetrics{Date: dateStr}
			}
			dayMap[dateStr].TasksCreated++
		}

		// Check completion date
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED && t.CompletedAt != nil && t.CompletedAt.AsTime().After(startDate) {
			dateStr := t.CompletedAt.AsTime().Format("2006-01-02")
			if dayMap[dateStr] == nil {
				dayMap[dateStr] = &orcv1.DailyMetrics{Date: dateStr}
			}
			dayMap[dateStr].TasksCompleted++
			dayMap[dateStr].CostUsd += t.Execution.Cost.TotalCostUsd
			dayMap[dateStr].PhasesCompleted += int32(len(t.Execution.Phases))

			for _, phase := range t.Execution.Phases {
				dayMap[dateStr].TokensUsed += phase.Tokens.InputTokens + phase.Tokens.OutputTokens
			}
		}

		// Check failed date
		if t.Status == orcv1.TaskStatus_TASK_STATUS_FAILED && t.UpdatedAt != nil && t.UpdatedAt.AsTime().After(startDate) {
			dateStr := t.UpdatedAt.AsTime().Format("2006-01-02")
			if dayMap[dateStr] == nil {
				dayMap[dateStr] = &orcv1.DailyMetrics{Date: dateStr}
			}
			dayMap[dateStr].TasksFailed++
		}
	}

	// Convert to sorted slice
	var dailyMetrics []*orcv1.DailyMetrics
	for _, dm := range dayMap {
		dailyMetrics = append(dailyMetrics, dm)
	}
	sort.Slice(dailyMetrics, func(i, j int) bool {
		return dailyMetrics[i].Date < dailyMetrics[j].Date
	})

	return connect.NewResponse(&orcv1.GetDailyMetricsResponse{
		Stats: &orcv1.PerDayStats{
			Days: dailyMetrics,
		},
	}), nil
}

// GetMetricsByModel returns metrics grouped by model.
// Note: Model tracking per-phase is not available in orcv1.PhaseState.
// This returns a single "unknown" model with aggregated metrics.
func (s *dashboardServer) GetMetricsByModel(
	ctx context.Context,
	req *connect.Request[orcv1.GetMetricsByModelRequest],
) (*connect.Response[orcv1.GetMetricsByModelResponse], error) {
	tasks, err := s.getTasks(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	// Determine time filter
	now := time.Now()
	var since time.Time
	if req.Msg.Period != nil {
		switch *req.Msg.Period {
		case "day":
			since = now.Add(-24 * time.Hour)
		case "week":
			since = now.Add(-7 * 24 * time.Hour)
		case "month":
			since = now.Add(-30 * 24 * time.Hour)
		}
	}

	// Aggregate all metrics (model info not available per-phase)
	mm := &orcv1.ModelMetrics{
		Model:  "unknown",
		Tokens: &orcv1.TokenUsage{},
	}

	for _, t := range tasks {
		var taskTime time.Time
		if t.CompletedAt != nil {
			taskTime = t.CompletedAt.AsTime()
		} else if t.UpdatedAt != nil {
			taskTime = t.UpdatedAt.AsTime()
		}
		if !since.IsZero() && taskTime.Before(since) {
			continue
		}

		mm.Tasks++
		mm.CostUsd += t.Execution.Cost.TotalCostUsd

		for _, phase := range t.Execution.Phases {
			mm.Phases++
			mm.Tokens.InputTokens += phase.Tokens.InputTokens
			mm.Tokens.OutputTokens += phase.Tokens.OutputTokens
			mm.Tokens.CacheCreationInputTokens += phase.Tokens.CacheCreationInputTokens
			mm.Tokens.CacheReadInputTokens += phase.Tokens.CacheReadInputTokens
		}
	}

	mm.Tokens.TotalTokens = mm.Tokens.InputTokens + mm.Tokens.OutputTokens
	if mm.Phases > 0 {
		mm.AvgTokensPerPhase = float64(mm.Tokens.TotalTokens) / float64(mm.Phases)
	}

	var models []*orcv1.ModelMetrics
	if mm.Tasks > 0 {
		models = append(models, mm)
	}

	return connect.NewResponse(&orcv1.GetMetricsByModelResponse{
		Models: models,
	}), nil
}

// GetOutcomes returns outcome statistics.
func (s *dashboardServer) GetOutcomes(
	ctx context.Context,
	req *connect.Request[orcv1.GetOutcomesRequest],
) (*connect.Response[orcv1.GetOutcomesResponse], error) {
	tasks, err := s.getTasks(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	// Determine time filter
	now := time.Now()
	var since time.Time
	if req.Msg.Period != nil {
		switch *req.Msg.Period {
		case "day":
			since = now.Add(-24 * time.Hour)
		case "week":
			since = now.Add(-7 * 24 * time.Hour)
		case "month":
			since = now.Add(-30 * 24 * time.Hour)
		}
	}

	outcomes := &orcv1.OutcomeStats{}

	for _, t := range tasks {
		var taskTime time.Time
		if t.CompletedAt != nil {
			taskTime = t.CompletedAt.AsTime()
		} else if t.UpdatedAt != nil {
			taskTime = t.UpdatedAt.AsTime()
		}
		if !since.IsZero() && taskTime.Before(since) {
			continue
		}

		switch t.Status {
		case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
			outcomes.Completed++
		case orcv1.TaskStatus_TASK_STATUS_FAILED:
			outcomes.Failed++
		case orcv1.TaskStatus_TASK_STATUS_RESOLVED:
			outcomes.Resolved++
		case orcv1.TaskStatus_TASK_STATUS_RUNNING, orcv1.TaskStatus_TASK_STATUS_PAUSED, orcv1.TaskStatus_TASK_STATUS_BLOCKED, orcv1.TaskStatus_TASK_STATUS_PLANNED, orcv1.TaskStatus_TASK_STATUS_CREATED, orcv1.TaskStatus_TASK_STATUS_CLASSIFYING:
			outcomes.InProgress++
		}
	}

	return connect.NewResponse(&orcv1.GetOutcomesResponse{
		Outcomes: outcomes,
	}), nil
}

// GetTopInitiatives returns top initiatives by activity.
func (s *dashboardServer) GetTopInitiatives(
	ctx context.Context,
	req *connect.Request[orcv1.GetTopInitiativesRequest],
) (*connect.Response[orcv1.GetTopInitiativesResponse], error) {
	limit := req.Msg.Limit
	if limit <= 0 {
		limit = 10
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

	tasks, err := s.getTasks(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	// Aggregate by initiative — first pass: collect counts
	initMap := make(map[string]*orcv1.TopInitiative)

	for _, t := range tasks {
		if t.InitiativeId == nil || *t.InitiativeId == "" {
			continue
		}
		initID := *t.InitiativeId

		if initMap[initID] == nil {
			initMap[initID] = &orcv1.TopInitiative{
				Id:    initID,
				Title: initID, // Default to ID, resolved below
			}
		}

		initMap[initID].TaskCount++
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			initMap[initID].CompletedCount++
		}
		initMap[initID].CostUsd += t.Execution.Cost.TotalCostUsd
	}

	// Batch load initiative titles (avoids N+1 LoadInitiative calls)
	if len(initMap) > 0 {
		ids := make([]string, 0, len(initMap))
		for id := range initMap {
			ids = append(ids, id)
		}
		titles, err := backend.DB().GetInitiativeTitlesBatch(ids)
		if err == nil {
			for id, title := range titles {
				if title != "" {
					initMap[id].Title = title
				}
			}
		}
	}

	// Convert to sorted slice
	var initiatives []*orcv1.TopInitiative
	for _, init := range initMap {
		initiatives = append(initiatives, init)
	}
	sort.Slice(initiatives, func(i, j int) bool {
		return initiatives[i].TaskCount > initiatives[j].TaskCount
	})

	// Apply limit
	if int32(len(initiatives)) > limit {
		initiatives = initiatives[:limit]
	}

	return connect.NewResponse(&orcv1.GetTopInitiativesResponse{
		Initiatives: initiatives,
	}), nil
}

// GetTopFiles returns top changed files aggregated across completed tasks.
func (s *dashboardServer) GetTopFiles(
	ctx context.Context,
	req *connect.Request[orcv1.GetTopFilesRequest],
) (*connect.Response[orcv1.GetTopFilesResponse], error) {
	// Apply limit with defaults and max
	limit := req.Msg.Limit
	if limit <= 0 {
		limit = 10 // Default limit
	}
	if limit > 50 {
		limit = 50 // Max limit
	}

	// Check if diff service is available
	if s.diffSvc == nil {
		return connect.NewResponse(&orcv1.GetTopFilesResponse{
			Files: nil,
		}), nil
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

	// Load tasks
	var tasks []*orcv1.Task

	// If task_id is specified, only load that task
	if req.Msg.TaskId != nil && *req.Msg.TaskId != "" {
		task, loadErr := backend.LoadTask(*req.Msg.TaskId)
		if loadErr != nil {
			// Task not found - return empty result (not error per spec)
			return connect.NewResponse(&orcv1.GetTopFilesResponse{
				Files: []*orcv1.TopFile{},
			}), nil
		}
		tasks = []*orcv1.Task{task}
	} else {
		tasks, err = s.getTasks(req.Msg.GetProjectId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
		}
	}

	// Aggregate file statistics across completed tasks with branches
	type fileStats struct {
		changeCount int32
		additions   int32
		deletions   int32
	}
	fileMap := make(map[string]*fileStats)

	for _, t := range tasks {
		// Only include completed tasks
		if t.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			continue
		}

		// Skip tasks without branches
		if t.Branch == "" {
			continue
		}

		// Get file list for this task's branch
		files, diffErr := s.diffSvc.GetFileList(ctx, "main", t.Branch)
		if diffErr != nil {
			// Log warning and skip this task (graceful degradation)
			if s.logger != nil {
				s.logger.Warn("failed to get diff for task",
					"task_id", t.Id,
					"branch", t.Branch,
					"error", diffErr)
			}
			continue
		}

		// Aggregate file stats
		for _, f := range files {
			if fileMap[f.Path] == nil {
				fileMap[f.Path] = &fileStats{}
			}
			fileMap[f.Path].changeCount++
			fileMap[f.Path].additions += int32(f.Additions)
			fileMap[f.Path].deletions += int32(f.Deletions)
		}
	}

	// Convert to slice and sort by change_count descending
	topFiles := make([]*orcv1.TopFile, 0, len(fileMap))
	for path, stats := range fileMap {
		topFiles = append(topFiles, &orcv1.TopFile{
			Path:        path,
			ChangeCount: stats.changeCount,
			Additions:   stats.additions,
			Deletions:   stats.deletions,
		})
	}

	sort.Slice(topFiles, func(i, j int) bool {
		return topFiles[i].ChangeCount > topFiles[j].ChangeCount
	})

	// Apply limit
	if int32(len(topFiles)) > limit {
		topFiles = topFiles[:limit]
	}

	return connect.NewResponse(&orcv1.GetTopFilesResponse{
		Files: topFiles,
	}), nil
}

// GetComparison returns comparison metrics between current and previous period.
func (s *dashboardServer) GetComparison(
	ctx context.Context,
	req *connect.Request[orcv1.GetComparisonRequest],
) (*connect.Response[orcv1.GetComparisonResponse], error) {
	tasks, err := s.getTasks(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	now := time.Now()
	var periodDuration time.Duration

	switch req.Msg.Period {
	case "day":
		periodDuration = 24 * time.Hour
	case "week":
		periodDuration = 7 * 24 * time.Hour
	case "month":
		periodDuration = 30 * 24 * time.Hour
	default:
		periodDuration = 7 * 24 * time.Hour
	}

	currentStart := now.Add(-periodDuration)
	previousStart := now.Add(-2 * periodDuration)

	// Calculate metrics for each period
	current := s.calculateMetricsForPeriod(tasks, currentStart, now)
	previous := s.calculateMetricsForPeriod(tasks, previousStart, currentStart)

	// Calculate percentage changes
	comparison := &orcv1.ComparisonMetrics{
		Current:  current,
		Previous: previous,
	}

	if previous.TasksCompleted > 0 {
		comparison.TasksChangePct = (float64(current.TasksCompleted) - float64(previous.TasksCompleted)) / float64(previous.TasksCompleted) * 100
	}

	// Cost change percentage would require cost tracking
	if previous.SuccessRate > 0 {
		comparison.SuccessRateChangePct = (current.SuccessRate - previous.SuccessRate) * 100
	}

	return connect.NewResponse(&orcv1.GetComparisonResponse{
		Comparison: comparison,
	}), nil
}

// GetTaskMetrics returns metrics for a specific task.
func (s *dashboardServer) GetTaskMetrics(
	ctx context.Context,
	req *connect.Request[orcv1.GetTaskMetricsRequest],
) (*connect.Response[orcv1.GetTaskMetricsResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", req.Msg.TaskId))
	}

	metrics := &orcv1.MetricsSummary{
		TotalTokens: &orcv1.TokenUsage{},
	}

	switch t.Status {
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		metrics.TasksCompleted = 1
		metrics.SuccessRate = 1.0
	case orcv1.TaskStatus_TASK_STATUS_FAILED:
		metrics.SuccessRate = 0.0
	}

	exec := t.GetExecution()
	if exec != nil {
		metrics.PhasesExecuted = int32(len(exec.Phases))

		for _, phase := range exec.Phases {
			if phase.GetTokens() != nil {
				metrics.TotalTokens.InputTokens += phase.Tokens.InputTokens
				metrics.TotalTokens.OutputTokens += phase.Tokens.OutputTokens
				metrics.TotalTokens.CacheCreationInputTokens += phase.Tokens.CacheCreationInputTokens
				metrics.TotalTokens.CacheReadInputTokens += phase.Tokens.CacheReadInputTokens
			}
		}
	}
	metrics.TotalTokens.TotalTokens = metrics.TotalTokens.InputTokens + metrics.TotalTokens.OutputTokens

	if t.StartedAt != nil && t.CompletedAt != nil {
		metrics.AvgTaskDurationSeconds = t.CompletedAt.AsTime().Sub(t.StartedAt.AsTime()).Seconds()
	}

	return connect.NewResponse(&orcv1.GetTaskMetricsResponse{
		Metrics: metrics,
	}), nil
}

// Helper functions

func (s *dashboardServer) calculateMetricsForPeriod(tasks []*orcv1.Task, start, end time.Time) *orcv1.MetricsSummary {
	metrics := &orcv1.MetricsSummary{
		TotalTokens: &orcv1.TokenUsage{},
	}

	var completedCount, failedCount int
	var totalDuration float64
	var durationCount int

	for _, t := range tasks {
		var taskTime time.Time
		if t.CompletedAt != nil {
			taskTime = t.CompletedAt.AsTime()
		} else if t.UpdatedAt != nil {
			taskTime = t.UpdatedAt.AsTime()
		}
		if taskTime.Before(start) || taskTime.After(end) {
			continue
		}

		switch t.Status {
		case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
			completedCount++
			if t.StartedAt != nil && t.CompletedAt != nil {
				totalDuration += t.CompletedAt.AsTime().Sub(t.StartedAt.AsTime()).Seconds()
				durationCount++
			}
		case orcv1.TaskStatus_TASK_STATUS_FAILED:
			failedCount++
		}

		metrics.PhasesExecuted += int32(len(t.Execution.Phases))

		for _, phase := range t.Execution.Phases {
			metrics.TotalTokens.InputTokens += phase.Tokens.InputTokens
			metrics.TotalTokens.OutputTokens += phase.Tokens.OutputTokens
		}
	}

	metrics.TotalTokens.TotalTokens = metrics.TotalTokens.InputTokens + metrics.TotalTokens.OutputTokens
	metrics.TasksCompleted = int32(completedCount)

	if durationCount > 0 {
		metrics.AvgTaskDurationSeconds = totalDuration / float64(durationCount)
	}

	totalFinished := completedCount + failedCount
	if totalFinished > 0 {
		metrics.SuccessRate = float64(completedCount) / float64(totalFinished)
	}

	return metrics
}

// ptrStringValue returns the value of a string pointer, or empty string if nil.
func ptrStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
