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
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/project"
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
	projectID := req.Msg.GetProjectId()
	if projectID == "" && s.projectCache != nil {
		response, err := s.getCrossProjectAttentionDashboardData()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load cross-project attention dashboard data: %w", err))
		}
		return connect.NewResponse(response), nil
	}

	backend, err := s.getBackend(projectID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

	// Load all tasks
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	now := time.Now()

	activeSignals, err := backend.LoadActiveAttentionSignals()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load attention signals: %w", err))
	}
	for _, signal := range activeSignals {
		if signal == nil || signal.ProjectID != "" || projectID == "" {
			continue
		}
		signal.ProjectID = projectID
	}

	// Build running summary
	runningSummary := s.buildRunningSummary(backend, tasks, now)

	// Build attention items (blocked, failed, pending decisions, gate approvals)
	attentionItems, err := s.buildAttentionItems(backend, activeSignals)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to build attention items: %w", err))
	}

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

func (s *attentionDashboardServer) getCrossProjectAttentionDashboardData() (*orcv1.GetAttentionDashboardDataResponse, error) {
	signals, err := s.loadCrossProjectAttentionSignals()
	if err != nil {
		return nil, fmt.Errorf("load cross-project attention signals: %w", err)
	}

	attentionItems, err := s.buildCrossProjectAttentionItems(signals)
	if err != nil {
		return nil, fmt.Errorf("build cross-project attention items: %w", err)
	}

	pendingRecommendations, err := s.countCrossProjectPendingRecommendations()
	if err != nil {
		return nil, fmt.Errorf("count cross-project pending recommendations: %w", err)
	}

	return &orcv1.GetAttentionDashboardDataResponse{
		RunningSummary: &orcv1.RunningSummary{
			TaskCount: 0,
			Tasks:     []*orcv1.RunningTask{},
		},
		AttentionItems: attentionItems,
		QueueSummary: &orcv1.QueueSummary{
			TaskCount:       0,
			Swimlanes:       []*orcv1.InitiativeSwimlane{},
			UnassignedTasks: []*orcv1.QueuedTask{},
		},
		PendingRecommendations: int32(pendingRecommendations),
	}, nil
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

// buildAttentionItems creates attention items from persisted signals and pending decisions.
func (s *attentionDashboardServer) buildAttentionItems(
	backend storage.Backend,
	signals []*controlplane.PersistedAttentionSignal,
) ([]*orcv1.AttentionItem, error) {
	items := make([]*orcv1.AttentionItem, 0, len(signals))

	for _, signal := range signals {
		if signal == nil {
			continue
		}

		item, err := s.attentionItemFromSignal(backend, signal)
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, item)
		}
	}

	// Add pending decisions if available
	if s.pendingDecisions != nil {
		pendingDecisionItems := s.loadPendingDecisionItems()
		items = append(items, pendingDecisionItems...)
	}

	// Sort by priority (highest first - lower enum values = higher priority), then age.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			if items[i].CreatedAt == nil || items[j].CreatedAt == nil {
				return items[i].Id < items[j].Id
			}
			return items[i].CreatedAt.AsTime().Before(items[j].CreatedAt.AsTime())
		}
		return items[i].Priority < items[j].Priority
	})

	return items, nil
}

func (s *attentionDashboardServer) buildCrossProjectAttentionItems(
	signals []*controlplane.PersistedAttentionSignal,
) ([]*orcv1.AttentionItem, error) {
	if s.projectCache == nil {
		return nil, fmt.Errorf("project cache not configured")
	}

	items := make([]*orcv1.AttentionItem, 0, len(signals))
	for _, signal := range signals {
		if signal == nil {
			continue
		}

		backend, err := s.projectCache.GetBackend(signal.ProjectID)
		if err != nil {
			return nil, fmt.Errorf("get backend for project %s: %w", signal.ProjectID, err)
		}

		item, err := s.attentionItemFromSignal(backend, signal)
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, item)
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority == items[j].Priority {
			if items[i].CreatedAt == nil || items[j].CreatedAt == nil {
				return items[i].Id < items[j].Id
			}
			return items[i].CreatedAt.AsTime().Before(items[j].CreatedAt.AsTime())
		}
		return items[i].Priority < items[j].Priority
	})

	return items, nil
}

func (s *attentionDashboardServer) attentionItemFromSignal(
	backend storage.Backend,
	signal *controlplane.PersistedAttentionSignal,
) (*orcv1.AttentionItem, error) {
	if signal == nil {
		return nil, nil
	}

	refTask, err := attentionSignalTaskReference(backend, signal)
	if err != nil {
		return nil, err
	}

	switch signal.Kind {
	case controlplane.AttentionSignalKindBlocker:
		return s.blockerAttentionItem(signal, refTask), nil
	case controlplane.AttentionSignalKindDecisionRequest:
		return s.genericAttentionItem(signal, refTask, orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_PENDING_DECISION), nil
	case controlplane.AttentionSignalKindDiscussionNeeded, controlplane.AttentionSignalKindVerificationSummary:
		return s.genericAttentionItem(signal, refTask, orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_ERROR_STATE), nil
	default:
		return nil, fmt.Errorf("unsupported attention signal kind %q", signal.Kind)
	}
}

func (s *attentionDashboardServer) blockerAttentionItem(
	signal *controlplane.PersistedAttentionSignal,
	refTask *orcv1.Task,
) *orcv1.AttentionItem {
	itemType := orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_BLOCKED_TASK
	description := signal.Summary
	actions := []orcv1.AttentionAction{
		orcv1.AttentionAction_ATTENTION_ACTION_SKIP,
		orcv1.AttentionAction_ATTENTION_ACTION_FORCE,
		orcv1.AttentionAction_ATTENTION_ACTION_VIEW,
	}
	idPrefix := "blocked"

	if signal.Status == controlplane.AttentionSignalStatusFailed {
		itemType = orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_FAILED_TASK
		actions = []orcv1.AttentionAction{
			orcv1.AttentionAction_ATTENTION_ACTION_RETRY,
			orcv1.AttentionAction_ATTENTION_ACTION_RESOLVE,
			orcv1.AttentionAction_ATTENTION_ACTION_VIEW,
		}
		idPrefix = "failed"
		if description == "" {
			description = "Task execution failed and requires attention"
		}
	}

	if description == "" && refTask != nil {
		description = task.GetDescriptionProto(refTask)
	}
	if description == "" {
		description = signal.Title
	}

	item := &orcv1.AttentionItem{
		Id:               attentionItemID(signal.ProjectID, fmt.Sprintf("%s-%s", idPrefix, signal.ReferenceID)),
		Type:             itemType,
		Title:            signal.Title,
		Description:      description,
		Priority:         attentionSignalPriority(refTask),
		CreatedAt:        timestamppb.New(signal.UpdatedAt),
		AvailableActions: actions,
		ProjectId:        signal.ProjectID,
		SignalKind:       string(signal.Kind),
		ReferenceType:    signal.ReferenceType,
		ReferenceId:      signal.ReferenceID,
	}
	if refTask != nil {
		item.TaskId = refTask.GetId()
		if item.Title == "" {
			item.Title = refTask.GetTitle()
		}
		if itemType == orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_BLOCKED_TASK {
			item.BlockedReason = description
		} else {
			item.ErrorMessage = description
		}
	}
	return item
}

func (s *attentionDashboardServer) genericAttentionItem(
	signal *controlplane.PersistedAttentionSignal,
	refTask *orcv1.Task,
	itemType orcv1.AttentionItemType,
) *orcv1.AttentionItem {
	item := &orcv1.AttentionItem{
		Id:               attentionItemID(signal.ProjectID, fmt.Sprintf("%s-%s", signal.Kind, signal.ReferenceID)),
		Type:             itemType,
		Title:            signal.Title,
		Description:      signal.Summary,
		Priority:         attentionSignalPriority(refTask),
		CreatedAt:        timestamppb.New(signal.UpdatedAt),
		AvailableActions: []orcv1.AttentionAction{orcv1.AttentionAction_ATTENTION_ACTION_VIEW},
		ProjectId:        signal.ProjectID,
		SignalKind:       string(signal.Kind),
		ReferenceType:    signal.ReferenceType,
		ReferenceId:      signal.ReferenceID,
	}
	if refTask != nil {
		item.TaskId = refTask.GetId()
		if item.Title == "" {
			item.Title = refTask.GetTitle()
		}
	}
	return item
}

func attentionSignalTaskReference(
	backend storage.Backend,
	signal *controlplane.PersistedAttentionSignal,
) (*orcv1.Task, error) {
	if signal == nil {
		return nil, nil
	}

	switch signal.ReferenceType {
	case controlplane.AttentionSignalReferenceTypeTask:
		taskItem, err := backend.LoadTask(signal.ReferenceID)
		if err != nil {
			return nil, fmt.Errorf("load task %s for attention signal %s: %w", signal.ReferenceID, signal.ID, err)
		}
		if taskItem == nil {
			return nil, fmt.Errorf("task %s for attention signal %s not found", signal.ReferenceID, signal.ID)
		}
		return taskItem, nil

	case controlplane.AttentionSignalReferenceTypeRun:
		run, err := backend.GetWorkflowRun(signal.ReferenceID)
		if err != nil {
			return nil, fmt.Errorf("load run %s for attention signal %s: %w", signal.ReferenceID, signal.ID, err)
		}
		if run == nil {
			return nil, fmt.Errorf("run %s for attention signal %s not found", signal.ReferenceID, signal.ID)
		}
		if run.TaskID == nil || *run.TaskID == "" {
			return nil, nil
		}
		taskItem, err := backend.LoadTask(*run.TaskID)
		if err != nil {
			return nil, fmt.Errorf("load task %s for attention signal %s: %w", *run.TaskID, signal.ID, err)
		}
		if taskItem == nil {
			return nil, fmt.Errorf("task %s for attention signal %s not found", *run.TaskID, signal.ID)
		}
		return taskItem, nil
	}

	return nil, nil
}

func attentionSignalPriority(taskItem *orcv1.Task) orcv1.TaskPriority {
	if taskItem == nil {
		return orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	}
	return taskItem.GetPriority()
}

func attentionItemID(projectID string, baseID string) string {
	if projectID == "" {
		return baseID
	}
	return projectID + "::" + baseID
}

func parseAttentionItemIdentifier(defaultProjectID string, rawID string) (string, string, error) {
	if rawID == "" {
		return "", "", fmt.Errorf("attention item ID is required")
	}

	projectID := defaultProjectID
	baseID := rawID

	if strings.Contains(rawID, "::") {
		parts := strings.SplitN(rawID, "::", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("invalid attention item ID format: %s", rawID)
		}
		projectID = parts[0]
		baseID = parts[1]
	}

	return projectID, baseID, nil
}

func (s *attentionDashboardServer) resolveTaskAttentionSignals(
	backend storage.Backend,
	projectID string,
	taskID string,
	resolvedBy string,
) error {
	signals, err := backend.LoadActiveAttentionSignals()
	if err != nil {
		return fmt.Errorf("load active attention signals: %w", err)
	}

	for _, signal := range signals {
		if signal == nil {
			continue
		}

		referenceTask, err := attentionSignalTaskReference(backend, signal)
		if err != nil {
			return err
		}
		if referenceTask == nil || referenceTask.GetId() != taskID {
			continue
		}

		resolvedSignal, err := backend.ResolveAttentionSignal(signal.ID, resolvedBy)
		if err != nil {
			return fmt.Errorf("resolve attention signal %s: %w", signal.ID, err)
		}
		s.publishAttentionSignalResolved(projectID, taskID, resolvedSignal)
	}

	return nil
}

func (s *attentionDashboardServer) publishAttentionSignalResolved(
	projectID string,
	taskID string,
	signal *controlplane.PersistedAttentionSignal,
) {
	if s.publisher == nil || signal == nil || signal.ResolvedAt == nil {
		return
	}

	s.publisher.Publish(events.NewProjectEvent(
		events.EventAttentionSignalResolved,
		projectID,
		taskID,
		events.AttentionSignalResolvedData{
			SignalID:      signal.ID,
			Kind:          string(signal.Kind),
			ReferenceType: signal.ReferenceType,
			ReferenceID:   signal.ReferenceID,
			ResolvedBy:    signal.ResolvedBy,
			ResolvedAt:    *signal.ResolvedAt,
		},
	))
}

func (s *attentionDashboardServer) loadCrossProjectAttentionSignals() ([]*controlplane.PersistedAttentionSignal, error) {
	if s.projectCache == nil {
		return nil, fmt.Errorf("project cache not configured")
	}

	registry, err := project.LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("load project registry: %w", err)
	}

	type projectSignal struct {
		signal   *controlplane.PersistedAttentionSignal
		priority orcv1.TaskPriority
	}

	merged := make([]projectSignal, 0)
	for _, proj := range registry.ValidProjects() {
		backend, err := s.projectCache.GetBackend(proj.ID)
		if err != nil {
			return nil, fmt.Errorf("get backend for project %s: %w", proj.ID, err)
		}

		signals, err := backend.LoadActiveAttentionSignals()
		if err != nil {
			return nil, fmt.Errorf("load attention signals for project %s: %w", proj.ID, err)
		}

		for _, signal := range signals {
			if signal == nil {
				continue
			}
			copied := *signal
			copied.ProjectID = proj.ID
			refTask, err := attentionSignalTaskReference(backend, &copied)
			if err != nil {
				return nil, fmt.Errorf("resolve task reference for project %s attention signal %s: %w", proj.ID, copied.ID, err)
			}
			merged = append(merged, projectSignal{
				signal:   &copied,
				priority: attentionSignalPriority(refTask),
			})
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].priority == merged[j].priority {
			if merged[i].signal.UpdatedAt.Equal(merged[j].signal.UpdatedAt) {
				return merged[i].signal.ProjectID < merged[j].signal.ProjectID
			}
			return merged[i].signal.UpdatedAt.Before(merged[j].signal.UpdatedAt)
		}
		return merged[i].priority < merged[j].priority
	})

	result := make([]*controlplane.PersistedAttentionSignal, 0, len(merged))
	for _, item := range merged {
		result = append(result, item.signal)
	}

	return result, nil
}

func (s *attentionDashboardServer) countCrossProjectPendingRecommendations() (int, error) {
	if s.projectCache == nil {
		return 0, fmt.Errorf("project cache not configured")
	}

	registry, err := project.LoadRegistry()
	if err != nil {
		return 0, fmt.Errorf("load project registry: %w", err)
	}

	total := 0
	for _, proj := range registry.ValidProjects() {
		backend, err := s.projectCache.GetBackend(proj.ID)
		if err != nil {
			return 0, fmt.Errorf("get backend for project %s: %w", proj.ID, err)
		}

		count, err := backend.CountRecommendationsByStatus(orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING)
		if err != nil {
			return 0, fmt.Errorf("count pending recommendations for project %s: %w", proj.ID, err)
		}
		total += count
	}

	return total, nil
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
	projectID, attentionItemID, err := parseAttentionItemIdentifier(req.Msg.GetProjectId(), req.Msg.AttentionItemId)
	if err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}), nil
	}

	backend, err := s.getBackend(projectID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get backend: %w", err))
	}

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
		return s.handleRetryAction(backend, projectID, targetID)

	case orcv1.AttentionAction_ATTENTION_ACTION_APPROVE:
		return s.handleApproveAction(backend, projectID, targetID)

	case orcv1.AttentionAction_ATTENTION_ACTION_REJECT:
		return s.handleRejectAction(backend, projectID, targetID)

	case orcv1.AttentionAction_ATTENTION_ACTION_SKIP:
		return s.handleSkipAction(backend, projectID, targetID, req.Msg.Reason)

	case orcv1.AttentionAction_ATTENTION_ACTION_FORCE:
		return s.handleForceAction(backend, projectID, targetID, req.Msg.Reason)

	case orcv1.AttentionAction_ATTENTION_ACTION_RESOLVE:
		return s.handleResolveAction(backend, projectID, targetID, req.Msg.Comment)

	default:
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("unknown action: %s", action.String()),
		}), nil
	}
}

// handleRetryAction handles retry actions on failed tasks.
func (s *attentionDashboardServer) handleRetryAction(backend storage.Backend, projectID string, taskID string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
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
	if err := s.resolveTaskAttentionSignals(backend, projectID, taskID, "dashboard_retry"); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to resolve attention signal: %v", err),
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
func (s *attentionDashboardServer) handleApproveAction(backend storage.Backend, projectID string, decisionID string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
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
	if err := s.resolveTaskAttentionSignals(backend, projectID, t.Id, "dashboard_approve"); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to resolve attention signal: %v", err),
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
func (s *attentionDashboardServer) handleRejectAction(backend storage.Backend, projectID string, decisionID string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
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
func (s *attentionDashboardServer) handleSkipAction(backend storage.Backend, projectID, taskID, reason string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
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
	if err := s.resolveTaskAttentionSignals(backend, projectID, taskID, "dashboard_skip"); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to resolve attention signal: %v", err),
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
func (s *attentionDashboardServer) handleForceAction(backend storage.Backend, projectID, taskID, reason string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
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
	if err := s.resolveTaskAttentionSignals(backend, projectID, taskID, "dashboard_force"); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to resolve attention signal: %v", err),
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
func (s *attentionDashboardServer) handleResolveAction(backend storage.Backend, projectID, taskID, comment string) (*connect.Response[orcv1.PerformAttentionActionResponse], error) {
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
	if err := s.resolveTaskAttentionSignals(backend, projectID, taskID, "dashboard_resolve"); err != nil {
		return connect.NewResponse(&orcv1.PerformAttentionActionResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to resolve attention signal: %v", err),
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
