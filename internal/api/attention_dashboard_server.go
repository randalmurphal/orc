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
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// attentionDashboardServer implements the AttentionDashboardServiceHandler interface.
type attentionDashboardServer struct {
	orcv1connect.UnimplementedAttentionDashboardServiceHandler
	backend          storage.Backend
	projectCache     *ProjectCache
	logger           *slog.Logger
	publisher        events.Publisher
	pendingDecisions *gate.PendingDecisionStore
}

// NewAttentionDashboardServer creates a new AttentionDashboardService handler.
func NewAttentionDashboardServer(
	backend storage.Backend,
	publisher events.Publisher,
	pendingDecisions *gate.PendingDecisionStore,
	logger *slog.Logger,
) orcv1connect.AttentionDashboardServiceHandler {
	return &attentionDashboardServer{
		backend:          backend,
		publisher:        publisher,
		pendingDecisions: pendingDecisions,
		logger:           logger,
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
	runningSummary := s.buildRunningSummary(backend, tasks, now)

	// Build attention items (blocked, failed, pending decisions, gate approvals)
	attentionItems := s.buildAttentionItems(tasks, now)

	// Build queue summary (planned tasks organized by initiative)
	queueSummary, err := s.buildQueueSummary(backend, tasks)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to build queue summary: %w", err))
	}

	pendingRecommendations, err := backend.CountRecommendationsByStatus(orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to count pending recommendations: %w", err))
	}

	response := &orcv1.GetAttentionDashboardDataResponse{
		RunningSummary:         runningSummary,
		AttentionItems:         attentionItems,
		QueueSummary:           queueSummary,
		PendingRecommendations: int32(pendingRecommendations),
	}

	return connect.NewResponse(response), nil
}

// buildRunningSummary creates the running tasks summary with progress and timing.
func (s *attentionDashboardServer) buildRunningSummary(backend storage.Backend, tasks []*orcv1.Task, now time.Time) *orcv1.RunningSummary {
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
			// Load initiative title from database
			if initiative, err := backend.LoadInitiativeProto(initiativeID); err == nil && initiative != nil {
				initiativeTitle = initiative.Title
			} else {
				initiativeTitle = initiativeID // Fallback to ID if load fails
			}
		}

		// Build phase progress
		phaseProgress := s.buildPhaseProgress(t)

		// Load recent output lines from transcripts
		outputLines := s.loadOutputLines(backend, t.Id)

		runningTask := &orcv1.RunningTask{
			Id:                 t.Id,
			Title:              t.Title,
			CurrentPhase:       ptrStringValue(t.CurrentPhase),
			StartedAt:          t.StartedAt,
			ElapsedTimeSeconds: elapsedSeconds,
			InitiativeId:       initiativeID,
			InitiativeTitle:    initiativeTitle,
			PhaseProgress:      phaseProgress,
			OutputLines:        outputLines,
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

// loadOutputLines loads recent output lines from transcripts for a task.
func (s *attentionDashboardServer) loadOutputLines(backend storage.Backend, taskID string) []string {
	transcripts, err := backend.GetTranscripts(taskID)
	if err != nil {
		// If we can't load transcripts, return empty lines
		return []string{}
	}

	var outputLines []string

	// Find recent assistant messages (limit to last 5-10)
	for i := len(transcripts) - 1; i >= 0 && len(outputLines) < 5; i-- {
		transcript := transcripts[i]
		if transcript.Role == "assistant" && strings.TrimSpace(transcript.Content) != "" {
			// Take first line or first 100 chars of content as summary
			content := strings.TrimSpace(transcript.Content)
			lines := strings.Split(content, "\n")
			if len(lines) > 0 {
				line := strings.TrimSpace(lines[0])
				if len(line) > 100 {
					line = line[:97] + "..."
				}
				if line != "" {
					outputLines = append([]string{line}, outputLines...) // Prepend to maintain chronological order
				}
			}
		}
	}

	return outputLines
}

// calculateInitiativeCompletion calculates the completion percentage for an initiative.
func (s *attentionDashboardServer) calculateInitiativeCompletion(backend storage.Backend, initiativeID string) float32 {
	// Load all tasks for this initiative (regardless of status)
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		return 0.0 // Return 0% if we can't load tasks
	}

	var totalTasks, completedTasks int
	for _, t := range allTasks {
		if t.InitiativeId != nil && *t.InitiativeId == initiativeID {
			totalTasks++
			if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
				completedTasks++
			}
		}
	}

	if totalTasks == 0 {
		return 0.0
	}

	// Calculate percentage
	percentage := float32(completedTasks*100) / float32(totalTasks)
	return percentage
}

// loadPendingDecisionItems creates attention items for pending decisions.
func (s *attentionDashboardServer) loadPendingDecisionItems() []*orcv1.AttentionItem {
	var items []*orcv1.AttentionItem

	if s.pendingDecisions == nil {
		return items
	}

	// Get all pending decisions from the store
	allDecisions := s.pendingDecisions.List()

	for _, decision := range allDecisions {
		item := &orcv1.AttentionItem{
			Id:          fmt.Sprintf("decision-%s", decision.DecisionID),
			Type:        orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_PENDING_DECISION,
			TaskId:      decision.TaskID,
			Title:       decision.TaskTitle,
			Description: decision.Question,
			Priority:    orcv1.TaskPriority_TASK_PRIORITY_NORMAL, // Default priority for decisions
			CreatedAt: &timestamppb.Timestamp{
				Seconds: decision.RequestedAt.Unix(),
				Nanos:   int32(decision.RequestedAt.Nanosecond()),
			},
			AvailableActions: []orcv1.AttentionAction{
				orcv1.AttentionAction_ATTENTION_ACTION_APPROVE,
				orcv1.AttentionAction_ATTENTION_ACTION_REJECT,
				orcv1.AttentionAction_ATTENTION_ACTION_VIEW,
			},
		}
		items = append(items, item)
	}

	return items
}

// buildAttentionItems creates attention items for blocked/failed tasks and pending decisions.
func (s *attentionDashboardServer) buildAttentionItems(tasks []*orcv1.Task, now time.Time) []*orcv1.AttentionItem {
	items := make([]*orcv1.AttentionItem, 0)

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

	// Add pending decisions if available
	if s.pendingDecisions != nil {
		pendingDecisionItems := s.loadPendingDecisionItems()
		items = append(items, pendingDecisionItems...)
	}

	// Sort by priority (highest first - lower enum values = higher priority)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority < items[j].Priority
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
		// Load initiative title from database
		initTitle := initID // Fallback to ID
		if initiative, err := backend.LoadInitiativeProto(initID); err == nil && initiative != nil {
			initTitle = initiative.Title
		}

		// Convert tasks to queued tasks format
		var queuedTasks []*orcv1.QueuedTask
		for pos, t := range initTasks {
			queuedTask := &orcv1.QueuedTask{
				Id:         t.Id,
				Title:      t.Title,
				Category:   t.Category,
				Priority:   t.Priority,
				Position:   int32(pos + 1),
				CreatedAt:  t.CreatedAt,
				WorkflowId: ptrStringValue(t.WorkflowId),
				Tags:       []string{}, // TODO: Load task tags if implemented
			}
			queuedTasks = append(queuedTasks, queuedTask)
		}

		// Calculate completion percentage for this initiative
		completionPercentage := s.calculateInitiativeCompletion(backend, initID)

		swimlane := &orcv1.InitiativeSwimlane{
			InitiativeId:         initID,
			InitiativeTitle:      initTitle,
			TaskCount:            int32(len(initTasks)),
			CompletionPercentage: completionPercentage,
			Tasks:                queuedTasks,
			Collapsed:            false,
		}
		swimlanes = append(swimlanes, swimlane)
	}

	// Convert unassigned tasks
	var unassignedQueuedTasks []*orcv1.QueuedTask
	for pos, t := range unassignedTasks {
		queuedTask := &orcv1.QueuedTask{
			Id:         t.Id,
			Title:      t.Title,
			Category:   t.Category,
			Priority:   t.Priority,
			Position:   int32(pos + 1),
			CreatedAt:  t.CreatedAt,
			WorkflowId: ptrStringValue(t.WorkflowId),
			Tags:       []string{},
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
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

	attentionItemID := req.Msg.AttentionItemId
	action := req.Msg.Action

	// Parse attention item ID to determine type and target
	// Expected formats: "retry-TASK-001", "failed-TASK-001", "blocked-TASK-001", "decision-DEC-001"
	parts := strings.SplitN(attentionItemID, "-", 2)
	if len(parts) != 2 {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("invalid attention item ID format: %s", attentionItemID),
		}), nil
	}

	// itemType := parts[0] // Could be "failed", "blocked", "decision", etc.
	targetID := parts[1]

	switch action {
	case orcv1.AttentionAction_ATTENTION_ACTION_VIEW:
		// View action - always succeeds, no side effects
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success: true,
		}), nil

	case orcv1.AttentionAction_ATTENTION_ACTION_RETRY:
		return s.handleRetryAction(backend, targetID)

	case orcv1.AttentionAction_ATTENTION_ACTION_APPROVE:
		return s.handleApproveAction(backend, targetID)

	case orcv1.AttentionAction_ATTENTION_ACTION_REJECT:
		return s.handleRejectAction(backend, targetID)

	case orcv1.AttentionAction_ATTENTION_ACTION_SKIP:
		return s.handleSkipAction(backend, targetID, req.Msg.Reason)

	case orcv1.AttentionAction_ATTENTION_ACTION_FORCE:
		return s.handleForceAction(backend, targetID, req.Msg.Reason)

	case orcv1.AttentionAction_ATTENTION_ACTION_RESOLVE:
		return s.handleResolveAction(backend, targetID, req.Msg.Comment)

	default:
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("unknown action: %s", action.String()),
		}), nil
	}
}

// handleRetryAction handles retry actions on failed tasks.
func (s *attentionDashboardServer) handleRetryAction(backend storage.Backend, taskID string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	// Load the task
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskID),
		}), nil
	}

	// Check if task can be retried (similar to ResumeTask logic)
	if t.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be retried (status: %s)", taskID, t.Status.String()),
		}), nil
	}

	// Set task to running (like ResumeTask does)
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to save task: %v", err),
		}), nil
	}

	// Publish event if publisher is available
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
		Success: true,
	}), nil
}

// handleApproveAction handles approval of pending decisions.
func (s *attentionDashboardServer) handleApproveAction(backend storage.Backend, decisionID string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	// If no pending decisions store, cannot handle decision actions
	if s.pendingDecisions == nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: "pending decisions not available",
		}), nil
	}

	// Get pending decision
	decision, ok := s.pendingDecisions.Get(decisionID)
	if !ok {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("decision not found: %s", decisionID),
		}), nil
	}

	// Load task
	t, err := backend.LoadTask(decision.TaskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task not found: %s", decision.TaskID),
		}), nil
	}

	// Verify task is blocked
	if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task is not blocked (status: %s)", t.Status.String()),
		}), nil
	}

	// Remove from pending decisions (approval means proceeding)
	s.pendingDecisions.Remove(decisionID)

	// Unblock the task (set to running)
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to save task: %v", err),
		}), nil
	}

	// Publish event if publisher is available
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
		Success: true,
	}), nil
}

// handleRejectAction handles rejection of pending decisions.
func (s *attentionDashboardServer) handleRejectAction(backend storage.Backend, decisionID string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	// If no pending decisions store, cannot handle decision actions
	if s.pendingDecisions == nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: "pending decisions not available",
		}), nil
	}

	// Get pending decision
	decision, ok := s.pendingDecisions.Get(decisionID)
	if !ok {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("decision not found: %s", decisionID),
		}), nil
	}

	// Load task
	t, err := backend.LoadTask(decision.TaskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task not found: %s", decision.TaskID),
		}), nil
	}

	// Remove from pending decisions (rejection means canceling)
	s.pendingDecisions.Remove(decisionID)

	// Task remains blocked or could be set to failed - for now keep it blocked
	// This behavior might need to be refined based on requirements

	// Publish event if publisher is available
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
		Success: true,
	}), nil
}

// handleSkipAction handles skipping a blocked task (moves it back to planned).
func (s *attentionDashboardServer) handleSkipAction(backend storage.Backend, taskID, reason string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	// Load the task
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskID),
		}), nil
	}

	// Check if task can be skipped (should be blocked)
	if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be skipped (status: %s)", taskID, t.Status.String()),
		}), nil
	}

	// Skip task by setting it back to planned status
	// Clear blockers since user explicitly chose to skip
	t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	t.BlockedBy = nil
	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to save task: %v", err),
		}), nil
	}

	// Publish event if publisher is available
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
		Success: true,
	}), nil
}

// handleForceAction handles forcing a blocked task to continue (sets to running).
func (s *attentionDashboardServer) handleForceAction(backend storage.Backend, taskID, reason string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	// Load the task
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskID),
		}), nil
	}

	// Check if task can be forced (should be blocked)
	if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be forced (status: %s)", taskID, t.Status.String()),
		}), nil
	}

	// Force task by setting it to running despite blockage
	// Keep blockers in case we need to track what was overridden
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to save task: %v", err),
		}), nil
	}

	// Publish event if publisher is available
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
		Success: true,
	}), nil
}

// handleResolveAction handles resolving a failed task (sets to planned for retry).
func (s *attentionDashboardServer) handleResolveAction(backend storage.Backend, taskID, comment string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
	// Load the task
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskID),
		}), nil
	}

	// Check if task can be resolved (should be failed or error state)
	if t.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be resolved (status: %s)", taskID, t.Status.String()),
		}), nil
	}

	// Resolve task by setting it back to planned for potential retry
	t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to save task: %v", err),
		}), nil
	}

	// Publish event if publisher is available
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
		Success: true,
	}), nil
}

// UpdateQueueOrganization handles queue organization updates.
func (s *attentionDashboardServer) UpdateQueueOrganization(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateQueueOrganizationRequest],
) (*connect.Response[orcv1.UpdateQueueOrganizationResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

	switch update := req.Msg.Update.(type) {
	case *orcv1.UpdateQueueOrganizationRequest_SwimlaneState:
		return s.handleSwimlaneStateUpdate(backend, update.SwimlaneState)

	case *orcv1.UpdateQueueOrganizationRequest_TaskReorder:
		return s.handleTaskReorderUpdate(backend, update.TaskReorder)

	default:
		return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
			Success:      false,
			ErrorMessage: "unknown update type",
		}), nil
	}
}

// handleSwimlaneStateUpdate handles updating swimlane collapsed/expanded state.
func (s *attentionDashboardServer) handleSwimlaneStateUpdate(backend storage.Backend, swimlaneState *orcv1.SwimlaneStateUpdate) (*connect.Response[orcv1.UpdateQueueOrganizationResponse], error) {
	// For now, we'll just return success as swimlane state is primarily UI state
	// In a more complete implementation, this could be stored in:
	// 1. User preferences table
	// 2. Initiative metadata
	// 3. Separate UI state storage

	if s.logger != nil {
		s.logger.Info("Swimlane state updated",
			"initiative_id", swimlaneState.InitiativeId,
			"collapsed", swimlaneState.Collapsed,
		)
	}

	return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
		Success: true,
	}), nil
}

// handleTaskReorderUpdate handles reordering tasks within or between initiatives.
func (s *attentionDashboardServer) handleTaskReorderUpdate(backend storage.Backend, taskReorder *orcv1.TaskReorderUpdate) (*connect.Response[orcv1.UpdateQueueOrganizationResponse], error) {
	// Load the task to be reordered
	t, err := backend.LoadTask(taskReorder.TaskId)
	if err != nil {
		return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s not found", taskReorder.TaskId),
		}), nil
	}

	// Only allow reordering of planned tasks (tasks in the queue)
	if t.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
		return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("task %s cannot be reordered (status: %s)", taskReorder.TaskId, t.Status.String()),
		}), nil
	}

	// Update task initiative assignment
	targetInitiativeID := taskReorder.TargetInitiativeId
	if targetInitiativeID == "" {
		// Moving to unassigned
		t.InitiativeId = nil
	} else {
		// Moving to specific initiative - validate initiative exists
		if _, err := backend.LoadInitiativeProto(targetInitiativeID); err != nil {
			return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("target initiative %s not found", targetInitiativeID),
			}), nil
		}
		t.InitiativeId = &targetInitiativeID
	}

	// Note: Position ordering within initiatives is not currently implemented
	// in the task storage model. This would require either:
	// 1. Adding an "order" field to tasks
	// 2. Using creation timestamps for ordering
	// 3. Storing ordering separately in initiative metadata
	// For now, we'll just handle the initiative assignment

	// Update task timestamp
	task.UpdateTimestampProto(t)

	// Save the updated task
	if err := backend.SaveTask(t); err != nil {
		return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to save task: %v", err),
		}), nil
	}

	// Publish event for real-time updates
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	if s.logger != nil {
		s.logger.Info("Task reordered",
			"task_id", taskReorder.TaskId,
			"target_initiative", targetInitiativeID,
			"position", taskReorder.NewPosition,
		)
	}

	return connect.NewResponse(&orcv1.UpdateQueueOrganizationResponse{
		Success: true,
	}), nil
}

// ptrStringValue returns the value of a string pointer, or empty string if nil.
func ptrStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
