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

const (
	runningSummaryOutputLineLimit     = 5
	runningSummaryTranscriptScanLimit = 40
	runningSummaryTranscriptDirection = "desc"
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
	runningSummary, err := s.buildRunningSummary(backend, tasks, now)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to build running summary: %w", err))
	}

	// Build attention items (blocked, failed, pending decisions, gate approvals)
	attentionItems, err := s.buildAttentionItems(backend, tasks, activeSignals, projectID)
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
	runningSummary, err := s.buildCrossProjectRunningSummary()
	if err != nil {
		return nil, fmt.Errorf("build cross-project running summary: %w", err)
	}

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
		RunningSummary: runningSummary,
		AttentionItems: attentionItems,
		QueueSummary: &orcv1.QueueSummary{
			TaskCount:       0,
			Swimlanes:       []*orcv1.InitiativeSwimlane{},
			UnassignedTasks: []*orcv1.QueuedTask{},
		},
		PendingRecommendations: int32(pendingRecommendations),
	}, nil
}

func (s *attentionDashboardServer) buildCrossProjectRunningSummary() (*orcv1.RunningSummary, error) {
	if s.projectCache == nil {
		return nil, fmt.Errorf("project cache not configured")
	}

	projects, err := project.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	now := time.Now()
	runningTasks := make([]*orcv1.RunningTask, 0)
	for _, proj := range projects {
		backend, err := s.projectCache.GetBackend(proj.ID)
		if err != nil {
			return nil, fmt.Errorf("get backend for project %s: %w", proj.ID, err)
		}

		tasks, err := backend.LoadAllTasks()
		if err != nil {
			return nil, fmt.Errorf("load tasks for project %s: %w", proj.ID, err)
		}

		projectSummary, err := s.buildRunningSummary(backend, tasks, now)
		if err != nil {
			return nil, fmt.Errorf("build running summary for project %s: %w", proj.ID, err)
		}
		for _, runningTask := range projectSummary.Tasks {
			runningTask.ProjectId = proj.ID
			runningTask.ProjectName = proj.Name
			runningTasks = append(runningTasks, runningTask)
		}
	}

	sort.Slice(runningTasks, func(i, j int) bool {
		if runningTasks[i].ElapsedTimeSeconds == runningTasks[j].ElapsedTimeSeconds {
			if runningTasks[i].ProjectId == runningTasks[j].ProjectId {
				return runningTasks[i].Id < runningTasks[j].Id
			}
			return runningTasks[i].ProjectId < runningTasks[j].ProjectId
		}
		return runningTasks[i].ElapsedTimeSeconds > runningTasks[j].ElapsedTimeSeconds
	})

	return &orcv1.RunningSummary{
		TaskCount: int32(len(runningTasks)),
		Tasks:     runningTasks,
	}, nil
}

// buildRunningSummary creates the running tasks summary with progress and timing.
func (s *attentionDashboardServer) buildRunningSummary(backend storage.Backend, tasks []*orcv1.Task, now time.Time) (*orcv1.RunningSummary, error) {
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
		outputLines, err := s.loadOutputLines(backend, t.Id)
		if err != nil {
			return nil, fmt.Errorf("load output lines for task %s: %w", t.Id, err)
		}

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
	}, nil
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
func (s *attentionDashboardServer) loadOutputLines(backend storage.Backend, taskID string) ([]string, error) {
	transcripts, _, err := backend.GetTranscriptsPaginated(taskID, storage.TranscriptPaginationOpts{
		Direction: runningSummaryTranscriptDirection,
		Limit:     runningSummaryTranscriptScanLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("get recent transcripts: %w", err)
	}

	var outputLines []string

	// Descending pagination returns newest rows first. Prepend each extracted line so
	// the final slice is oldest-to-newest within the bounded recent window.
	for _, transcript := range transcripts {
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
					outputLines = append([]string{line}, outputLines...)
					if len(outputLines) == runningSummaryOutputLineLimit {
						break
					}
				}
			}
		}
	}

	return outputLines, nil
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
func (s *attentionDashboardServer) loadPendingDecisionItems(projectID string) []*orcv1.AttentionItem {
	var items []*orcv1.AttentionItem

	if s.pendingDecisions == nil {
		return items
	}

	allDecisions := s.pendingDecisions.List(projectID)

	for _, decision := range allDecisions {
		options := make([]*orcv1.DecisionOption, 0, len(decision.Options))
		for _, option := range decision.Options {
			protoOption := &orcv1.DecisionOption{
				Id:          option.ID,
				Label:       option.Label,
				Recommended: option.Recommended,
			}
			if option.Description != "" {
				protoOption.Description = &option.Description
			}
			options = append(options, protoOption)
		}

		item := &orcv1.AttentionItem{
			Id:          attentionItemID(decision.ProjectID, fmt.Sprintf("decision-%s", decision.DecisionID)),
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
			DecisionOptions: options,
			ProjectId:       decision.ProjectID,
		}
		items = append(items, item)
	}

	return items
}

// buildAttentionItems creates attention items from persisted signals and pending decisions.
func (s *attentionDashboardServer) buildAttentionItems(
	backend storage.Backend,
	tasks []*orcv1.Task,
	signals []*controlplane.PersistedAttentionSignal,
	projectID string,
) ([]*orcv1.AttentionItem, error) {
	mergedSignals := controlplane.MergeTaskAttentionSignals(projectID, tasks, signals)
	items := make([]*orcv1.AttentionItem, 0, len(mergedSignals))

	for _, signal := range mergedSignals {
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
		pendingDecisionItems := s.loadPendingDecisionItems(projectID)
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
		tasks, err := backend.LoadAllTasks()
		if err != nil {
			return nil, fmt.Errorf("load tasks for project %s: %w", proj.ID, err)
		}
		signals = controlplane.MergeTaskAttentionSignals(proj.ID, tasks, signals)

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

// ptrStringValue returns the value of a string pointer, or empty string if nil.
func ptrStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
