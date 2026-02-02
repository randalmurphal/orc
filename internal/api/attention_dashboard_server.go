// Package api provides the Connect RPC and REST API server for orc.
// This file implements the AttentionDashboardService Connect RPC service.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// attentionDashboardServer implements the AttentionDashboardServiceHandler interface.
type attentionDashboardServer struct {
	orcv1connect.UnimplementedAttentionDashboardServiceHandler
	backend      storage.Backend
	projectCache *ProjectCache
	logger       *slog.Logger
}

// NewAttentionDashboardServer creates a new AttentionDashboardService handler.
func NewAttentionDashboardServer(
	backend storage.Backend,
	logger *slog.Logger,
) orcv1connect.AttentionDashboardServiceHandler {
	return &attentionDashboardServer{
		backend: backend,
		logger:  logger,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *attentionDashboardServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
// If projectID is provided and projectCache is available, uses the cache.
// Errors if projectID is provided but cache is not configured (prevents silent data leaks).
// Falls back to legacy single backend only when no projectID is specified.
func (s *attentionDashboardServer) getBackend(projectID string) (storage.Backend, error) {
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

// GetAttentionDashboardData returns dashboard data for the attention management redesign.
func (s *attentionDashboardServer) GetAttentionDashboardData(
	ctx context.Context,
	req *connect.Request[orcv1.GetAttentionDashboardDataRequest],
) (*connect.Response[orcv1.GetAttentionDashboardDataResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

	// Load all tasks
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	now := time.Now()

	// Build running summary
	runningSummary := s.buildRunningSummary(tasks, now)

	// Build attention items (blocked, failed, pending decisions, gate approvals)
	attentionItems := s.buildAttentionItems(tasks, now)

	// Build queue summary (planned tasks organized by initiative)
	queueSummary, err := s.buildQueueSummary(backend, tasks)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to build queue summary: %w", err))
	}

	response := &orcv1.GetAttentionDashboardDataResponse{
		RunningSummary: runningSummary,
		AttentionItems: attentionItems,
		QueueSummary:   queueSummary,
	}

	return connect.NewResponse(response), nil
}

// buildRunningSummary creates the running tasks summary with progress and timing.
func (s *attentionDashboardServer) buildRunningSummary(tasks []*orcv1.Task, now time.Time) *orcv1.RunningSummary {
	var runningTasks []*orcv1.RunningTask

	for _, t := range tasks {
		if t.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
			continue
		}

		// Calculate elapsed time
		var elapsedSeconds int64
		if t.StartedAt != nil {
			elapsedSeconds = int64(now.Sub(t.StartedAt.AsTime()).Seconds())
		}

		// Get initiative details if linked
		var initiativeID, initiativeTitle string
		if t.InitiativeId != nil {
			initiativeID = *t.InitiativeId
			// TODO: Load initiative title from database
			initiativeTitle = initiativeID // Fallback to ID
		}

		// Build phase progress
		phaseProgress := s.buildPhaseProgress(t)

		runningTask := &orcv1.RunningTask{
			Id:                 t.Id,
			Title:              t.Title,
			CurrentPhase:       ptrStringValue(t.CurrentPhase),
			StartedAt:          t.StartedAt,
			ElapsedTimeSeconds: elapsedSeconds,
			InitiativeId:       initiativeID,
			InitiativeTitle:    initiativeTitle,
			PhaseProgress:      phaseProgress,
			OutputLines:        []string{}, // TODO: Load recent output from execution
		}

		runningTasks = append(runningTasks, runningTask)
	}

	return &orcv1.RunningSummary{
		TaskCount: int32(len(runningTasks)),
		Tasks:     runningTasks,
	}
}

// buildPhaseProgress creates phase progress for pipeline visualization.
func (s *attentionDashboardServer) buildPhaseProgress(t *orcv1.Task) *orcv1.PhaseProgress {
	// Define the 5-phase pipeline mapping
	phaseSteps := []*orcv1.PhaseStep{
		{Name: "plan", Status: orcv1.PhaseStepStatus_PHASE_STEP_STATUS_PENDING},
		{Name: "code", Status: orcv1.PhaseStepStatus_PHASE_STEP_STATUS_PENDING},
		{Name: "test", Status: orcv1.PhaseStepStatus_PHASE_STEP_STATUS_PENDING},
		{Name: "review", Status: orcv1.PhaseStepStatus_PHASE_STEP_STATUS_PENDING},
		{Name: "done", Status: orcv1.PhaseStepStatus_PHASE_STEP_STATUS_PENDING},
	}

	// Map current phase to display phase and mark completed phases
	currentPhase := ptrStringValue(t.CurrentPhase)
	displayPhase := mapPhaseToDisplay(currentPhase)

	for i, step := range phaseSteps {
		if step.Name == displayPhase {
			step.Status = orcv1.PhaseStepStatus_PHASE_STEP_STATUS_ACTIVE
			// Mark all previous phases as completed
			for j := 0; j < i; j++ {
				phaseSteps[j].Status = orcv1.PhaseStepStatus_PHASE_STEP_STATUS_COMPLETED
			}
			break
		}
	}

	return &orcv1.PhaseProgress{
		CurrentPhase: currentPhase,
		Steps:        phaseSteps,
	}
}

// mapPhaseToDisplay maps internal phase names to display names for pipeline.
func mapPhaseToDisplay(phase string) string {
	switch phase {
	case "spec", "design", "research":
		return "plan"
	case "implement":
		return "code"
	case "test":
		return "test"
	case "review":
		return "review"
	case "docs", "validate":
		return "done"
	default:
		return phase
	}
}

// buildAttentionItems creates attention items for blocked/failed tasks and pending decisions.
func (s *attentionDashboardServer) buildAttentionItems(tasks []*orcv1.Task, now time.Time) []*orcv1.AttentionItem {
	var items []*orcv1.AttentionItem

	for _, t := range tasks {
		// Add blocked tasks
		if t.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED {
			item := &orcv1.AttentionItem{
				Id:          fmt.Sprintf("blocked-%s", t.Id),
				Type:        orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_BLOCKED_TASK,
				TaskId:      t.Id,
				Title:       t.Title,
				Description: s.buildBlockedDescription(t),
				Priority:    t.Priority,
				CreatedAt:   t.UpdatedAt,
				AvailableActions: []orcv1.AttentionAction{
					orcv1.AttentionAction_ATTENTION_ACTION_SKIP,
					orcv1.AttentionAction_ATTENTION_ACTION_FORCE,
					orcv1.AttentionAction_ATTENTION_ACTION_VIEW,
				},
			}
			items = append(items, item)
		}

		// Add failed tasks
		if t.Status == orcv1.TaskStatus_TASK_STATUS_FAILED {
			item := &orcv1.AttentionItem{
				Id:          fmt.Sprintf("failed-%s", t.Id),
				Type:        orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_FAILED_TASK,
				TaskId:      t.Id,
				Title:       t.Title,
				Description: "Task execution failed and requires attention",
				Priority:    t.Priority,
				CreatedAt:   t.UpdatedAt,
				AvailableActions: []orcv1.AttentionAction{
					orcv1.AttentionAction_ATTENTION_ACTION_RETRY,
					orcv1.AttentionAction_ATTENTION_ACTION_RESOLVE,
					orcv1.AttentionAction_ATTENTION_ACTION_VIEW,
				},
			}
			items = append(items, item)
		}
	}

	// TODO: Add pending decisions and gate approvals from respective stores
	// This would require loading from decision store and gate approval store

	// Sort by priority (highest first)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority > items[j].Priority
	})

	return items
}

// buildBlockedDescription creates a description for blocked tasks.
func (s *attentionDashboardServer) buildBlockedDescription(t *orcv1.Task) string {
	if len(t.BlockedBy) == 1 {
		return fmt.Sprintf("Blocked by task %s", t.BlockedBy[0])
	} else if len(t.BlockedBy) > 1 {
		return fmt.Sprintf("Blocked by %d tasks: %s", len(t.BlockedBy), strings.Join(t.BlockedBy, ", "))
	}
	return "Task is blocked"
}

// buildQueueSummary creates queue summary organized by initiatives.
func (s *attentionDashboardServer) buildQueueSummary(backend storage.Backend, tasks []*orcv1.Task) (*orcv1.QueueSummary, error) {
	// Group planned tasks by initiative
	initiativeMap := make(map[string][]*orcv1.Task)
	var unassignedTasks []*orcv1.Task

	for _, t := range tasks {
		if t.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
			continue
		}

		if t.InitiativeId != nil && *t.InitiativeId != "" {
			initID := *t.InitiativeId
			initiativeMap[initID] = append(initiativeMap[initID], t)
		} else {
			unassignedTasks = append(unassignedTasks, t)
		}
	}

	// Build swimlanes
	var swimlanes []*orcv1.InitiativeSwimlane

	for initID, initTasks := range initiativeMap {
		// TODO: Load initiative title from database
		initTitle := initID // Fallback to ID

		// Convert tasks to queued tasks format
		var queuedTasks []*orcv1.QueuedTask
		for pos, t := range initTasks {
			queuedTask := &orcv1.QueuedTask{
				Id:          t.Id,
				Title:       t.Title,
				Category:    t.Category,
				Priority:    t.Priority,
				Position:    int32(pos + 1),
				CreatedAt:   t.CreatedAt,
				WorkflowId:  ptrStringValue(t.WorkflowId),
				Tags:        []string{}, // TODO: Load task tags if implemented
			}
			queuedTasks = append(queuedTasks, queuedTask)
		}

		swimlane := &orcv1.InitiativeSwimlane{
			InitiativeId:         initID,
			InitiativeTitle:      initTitle,
			TaskCount:            int32(len(initTasks)),
			CompletionPercentage: 0, // TODO: Calculate based on completed vs total tasks
			Tasks:                queuedTasks,
			Collapsed:            false,
		}
		swimlanes = append(swimlanes, swimlane)
	}

	// Convert unassigned tasks
	var unassignedQueuedTasks []*orcv1.QueuedTask
	for pos, t := range unassignedTasks {
		queuedTask := &orcv1.QueuedTask{
			Id:          t.Id,
			Title:       t.Title,
			Category:    t.Category,
			Priority:    t.Priority,
			Position:    int32(pos + 1),
			CreatedAt:   t.CreatedAt,
			WorkflowId:  ptrStringValue(t.WorkflowId),
			Tags:        []string{},
		}
		unassignedQueuedTasks = append(unassignedQueuedTasks, queuedTask)
	}

	totalTasks := len(unassignedTasks)
	for _, tasks := range initiativeMap {
		totalTasks += len(tasks)
	}

	return &orcv1.QueueSummary{
		TaskCount:       int32(totalTasks),
		Swimlanes:       swimlanes,
		UnassignedTasks: unassignedQueuedTasks,
	}, nil
}

// PerformAttentionAction handles actions on attention items.
func (s *attentionDashboardServer) PerformAttentionAction(
	ctx context.Context,
	req *connect.Request[orcv1.PerformAttentionActionRequest],
) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	// TODO: Implement attention action handling
	// This would involve:
	// - Skip/Force blocked tasks
	// - Retry/Resolve failed tasks
	// - Approve/Reject pending decisions
	// - Handle gate approvals

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
		Success:      false,
		ErrorMessage: "PerformAttentionAction not yet implemented",
	}), nil
}

// UpdateQueueOrganization handles queue organization updates.
func (s *attentionDashboardServer) UpdateQueueOrganization(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateQueueOrganizationRequest],
) (*connect.Response[orcv1.UpdateQueueOrganizationResponse], error) {
	// TODO: Implement queue organization updates
	// This would involve:
	// - Collapse/expand swimlanes
	// - Reorder tasks within or between initiatives
	// - Update task initiative assignments

	return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
		Success:      false,
		ErrorMessage: "UpdateQueueOrganization not yet implemented",
	}), nil
}

// ptrStringValue returns the value of a string pointer, or empty string if nil.
func ptrStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}