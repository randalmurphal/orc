// Package api provides the Connect RPC and REST API server for orc.
// This file implements the TaskService Connect RPC service.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// taskServer implements the TaskServiceHandler interface.
type taskServer struct {
	orcv1connect.UnimplementedTaskServiceHandler
	backend     storage.Backend
	config      *config.Config
	logger      *slog.Logger
	publisher   events.Publisher
	projectRoot string
	diffCache   *diff.Cache
	projectDB   *db.ProjectDB
}

// NewTaskServer creates a new TaskService handler.
func NewTaskServer(
	backend storage.Backend,
	cfg *config.Config,
	logger *slog.Logger,
	publisher events.Publisher,
	projectRoot string,
	diffCache *diff.Cache,
	projectDB *db.ProjectDB,
) orcv1connect.TaskServiceHandler {
	return &taskServer{
		backend:     backend,
		config:      cfg,
		logger:      logger,
		publisher:   publisher,
		projectRoot: projectRoot,
		diffCache:   diffCache,
		projectDB:   projectDB,
	}
}

// ListTasks returns all tasks with optional filtering.
func (s *taskServer) ListTasks(
	ctx context.Context,
	req *connect.Request[orcv1.ListTasksRequest],
) (*connect.Response[orcv1.ListTasksResponse], error) {
	tasks, err := s.backend.LoadAllTasks()
	if err != nil {
		// Return empty list if no tasks yet
		return connect.NewResponse(&orcv1.ListTasksResponse{
			Tasks: []*orcv1.Task{},
			Page:  &orcv1.PageResponse{Total: 0},
		}), nil
	}

	if tasks == nil {
		tasks = []*task.Task{}
	}

	// Filter by initiative if requested
	if req.Msg.InitiativeId != nil && *req.Msg.InitiativeId != "" {
		var filtered []*task.Task
		for _, t := range tasks {
			if t.InitiativeID == *req.Msg.InitiativeId {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Filter by status if requested
	if len(req.Msg.Statuses) > 0 {
		var filtered []*task.Task
		statusSet := make(map[orcv1.TaskStatus]bool)
		for _, status := range req.Msg.Statuses {
			statusSet[status] = true
		}
		for _, t := range tasks {
			if statusSet[taskStatusToProto(t.Status)] {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Filter by queue if requested
	if req.Msg.Queue != nil {
		var filtered []*task.Task
		targetQueue := protoToTaskQueue(*req.Msg.Queue)
		for _, t := range tasks {
			if t.GetQueue() == targetQueue {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Filter by category if requested
	if req.Msg.Category != nil {
		var filtered []*task.Task
		targetCategory := protoToTaskCategory(*req.Msg.Category)
		for _, t := range tasks {
			if t.Category == targetCategory {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Populate computed dependency fields
	task.PopulateComputedFields(tasks)

	// Filter by dependency status if requested
	if req.Msg.DependencyStatus != nil {
		var filtered []*task.Task
		for _, t := range tasks {
			if dependencyStatusToProto(t.DependencyStatus) == *req.Msg.DependencyStatus {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	totalCount := int32(len(tasks))

	// Apply pagination
	page := int32(1)
	limit := int32(20)
	if req.Msg.Page != nil {
		if req.Msg.Page.Page > 0 {
			page = req.Msg.Page.Page
		}
		if req.Msg.Page.Limit > 0 {
			limit = req.Msg.Page.Limit
		}
	}
	if limit > 100 {
		limit = 100
	}

	// Calculate offset and slice
	offset := (page - 1) * limit
	endIdx := offset + limit
	if endIdx > totalCount {
		endIdx = totalCount
	}
	if offset < totalCount {
		tasks = tasks[offset:endIdx]
	} else {
		tasks = []*task.Task{}
	}

	// Convert to proto
	protoTasks := make([]*orcv1.Task, len(tasks))
	for i, t := range tasks {
		protoTasks[i] = TaskToProto(t)
	}

	// Calculate pagination response
	totalPages := (totalCount + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return connect.NewResponse(&orcv1.ListTasksResponse{
		Tasks: protoTasks,
		Page: &orcv1.PageResponse{
			Page:       page,
			Limit:      limit,
			Total:      totalCount,
			TotalPages: totalPages,
			HasMore:    page < totalPages,
		},
	}), nil
}

// GetTask returns a single task by ID.
func (s *taskServer) GetTask(
	ctx context.Context,
	req *connect.Request[orcv1.GetTaskRequest],
) (*connect.Response[orcv1.GetTaskResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Populate computed fields
	allTasks, _ := s.backend.LoadAllTasks()
	if allTasks != nil {
		t.Blocks = task.ComputeBlocks(t.ID, allTasks)
		t.ReferencedBy = task.ComputeReferencedBy(t.ID, allTasks)
		taskMap := make(map[string]*task.Task)
		for _, at := range allTasks {
			taskMap[at.ID] = at
		}
		t.UnmetBlockers = t.GetUnmetDependencies(taskMap)
		t.IsBlocked = len(t.UnmetBlockers) > 0
		t.DependencyStatus = t.ComputeDependencyStatus()
	}

	return connect.NewResponse(&orcv1.GetTaskResponse{
		Task: TaskToProto(t),
	}), nil
}

// CreateTask creates a new task.
func (s *taskServer) CreateTask(
	ctx context.Context,
	req *connect.Request[orcv1.CreateTaskRequest],
) (*connect.Response[orcv1.CreateTaskResponse], error) {
	if req.Msg.Title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("title is required"))
	}

	// Generate a new task ID
	id, err := s.backend.GetNextTaskID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("generate task ID: %w", err))
	}

	// Create the task
	t := task.New(id, req.Msg.Title)

	// Set optional fields from request
	if req.Msg.Description != nil {
		t.Description = *req.Msg.Description
	}
	if req.Msg.Weight != orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED {
		t.Weight = protoToTaskWeight(req.Msg.Weight)
	}
	if req.Msg.Queue != nil {
		t.Queue = protoToTaskQueue(*req.Msg.Queue)
	}
	if req.Msg.Priority != nil {
		t.Priority = protoToTaskPriority(*req.Msg.Priority)
	}
	if req.Msg.Category != nil {
		t.Category = protoToTaskCategory(*req.Msg.Category)
	}
	if req.Msg.InitiativeId != nil {
		t.InitiativeID = *req.Msg.InitiativeId
	}
	if req.Msg.WorkflowId != nil {
		t.WorkflowID = *req.Msg.WorkflowId
	}
	if req.Msg.TargetBranch != nil {
		t.TargetBranch = *req.Msg.TargetBranch
	}
	if len(req.Msg.BlockedBy) > 0 {
		t.BlockedBy = req.Msg.BlockedBy
	}
	if len(req.Msg.RelatedTo) > 0 {
		t.RelatedTo = req.Msg.RelatedTo
	}

	// Save the task
	if err := s.backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskCreated, t.ID, t))
	}

	return connect.NewResponse(&orcv1.CreateTaskResponse{
		Task: TaskToProto(t),
	}), nil
}

// UpdateTask updates an existing task.
func (s *taskServer) UpdateTask(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateTaskRequest],
) (*connect.Response[orcv1.UpdateTaskResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	// Load existing task
	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Apply updates
	if req.Msg.Title != nil {
		t.Title = *req.Msg.Title
	}
	if req.Msg.Description != nil {
		t.Description = *req.Msg.Description
	}
	if req.Msg.Weight != nil {
		t.Weight = protoToTaskWeight(*req.Msg.Weight)
	}
	if req.Msg.Queue != nil {
		t.Queue = protoToTaskQueue(*req.Msg.Queue)
	}
	if req.Msg.Priority != nil {
		t.Priority = protoToTaskPriority(*req.Msg.Priority)
	}
	if req.Msg.Category != nil {
		t.Category = protoToTaskCategory(*req.Msg.Category)
	}
	if req.Msg.InitiativeId != nil {
		t.InitiativeID = *req.Msg.InitiativeId
	}
	if req.Msg.TargetBranch != nil {
		t.TargetBranch = *req.Msg.TargetBranch
	}
	if req.Msg.BlockedBy != nil {
		t.BlockedBy = req.Msg.BlockedBy
	}
	if req.Msg.RelatedTo != nil {
		t.RelatedTo = req.Msg.RelatedTo
	}

	// Save the task
	if err := s.backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.ID, t))
	}

	return connect.NewResponse(&orcv1.UpdateTaskResponse{
		Task: TaskToProto(t),
	}), nil
}

// DeleteTask deletes a task.
func (s *taskServer) DeleteTask(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteTaskRequest],
) (*connect.Response[orcv1.DeleteTaskResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	// Check task exists
	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Check if task is running
	if t.Status == task.StatusRunning {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot delete running task"))
	}

	// Delete the task
	if err := s.backend.DeleteTask(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskDeleted, req.Msg.Id, map[string]string{"task_id": req.Msg.Id}))
	}

	return connect.NewResponse(&orcv1.DeleteTaskResponse{}), nil
}

// GetTaskState returns the execution state for a task.
func (s *taskServer) GetTaskState(
	ctx context.Context,
	req *connect.Request[orcv1.GetTaskStateRequest],
) (*connect.Response[orcv1.GetTaskStateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	return connect.NewResponse(&orcv1.GetTaskStateResponse{
		State: executionStateToProto(&t.Execution),
	}), nil
}

// GetTaskPlan returns the plan for a task.
func (s *taskServer) GetTaskPlan(
	ctx context.Context,
	req *connect.Request[orcv1.GetTaskPlanRequest],
) (*connect.Response[orcv1.GetTaskPlanResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Build TaskPlan from execution state
	plan := &orcv1.TaskPlan{
		Version:     1,
		Weight:      taskWeightToProto(t.Weight),
		Description: t.Description,
	}

	// Add phases from execution state (map iteration)
	for phaseName, phase := range t.Execution.Phases {
		planPhase := &orcv1.PlanPhase{
			Name:   phaseName,
			Status: phaseStatusToProto(phase.Status),
		}
		plan.Phases = append(plan.Phases, planPhase)
	}

	return connect.NewResponse(&orcv1.GetTaskPlanResponse{
		Plan: plan,
	}), nil
}

// GetDependencies returns dependency information for a task.
func (s *taskServer) GetDependencies(
	ctx context.Context,
	req *connect.Request[orcv1.GetDependenciesRequest],
) (*connect.Response[orcv1.GetDependenciesResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Load all tasks to compute dependencies
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks: %w", err))
	}

	taskMap := make(map[string]*task.Task)
	for _, at := range allTasks {
		taskMap[at.ID] = at
	}

	// Compute dependency info
	t.Blocks = task.ComputeBlocks(t.ID, allTasks)

	// Build dependency graph
	graph := &orcv1.DependencyGraph{
		Nodes: make([]*orcv1.DependencyNode, 0),
		Edges: make([]*orcv1.DependencyEdge, 0),
	}

	// Add the target task as a node
	graph.Nodes = append(graph.Nodes, &orcv1.DependencyNode{
		Id:     t.ID,
		Title:  t.Title,
		Status: taskStatusToProto(t.Status),
	})

	// Add blockers as nodes and edges
	for _, blockerID := range t.BlockedBy {
		blocker, exists := taskMap[blockerID]
		if exists {
			graph.Nodes = append(graph.Nodes, &orcv1.DependencyNode{
				Id:     blocker.ID,
				Title:  blocker.Title,
				Status: taskStatusToProto(blocker.Status),
			})
			graph.Edges = append(graph.Edges, &orcv1.DependencyEdge{
				From: blockerID,
				To:   t.ID,
				Type: "blocks",
			})
		}
	}

	// Add blocked tasks as nodes and edges
	for _, blockID := range t.Blocks {
		blocked, exists := taskMap[blockID]
		if exists {
			graph.Nodes = append(graph.Nodes, &orcv1.DependencyNode{
				Id:     blocked.ID,
				Title:  blocked.Title,
				Status: taskStatusToProto(blocked.Status),
			})
			graph.Edges = append(graph.Edges, &orcv1.DependencyEdge{
				From: t.ID,
				To:   blockID,
				Type: "blocks",
			})
		}
	}

	// Add related tasks if transitive
	if req.Msg.Transitive {
		for _, relID := range t.RelatedTo {
			rel, exists := taskMap[relID]
			if exists {
				graph.Nodes = append(graph.Nodes, &orcv1.DependencyNode{
					Id:     rel.ID,
					Title:  rel.Title,
					Status: taskStatusToProto(rel.Status),
				})
				graph.Edges = append(graph.Edges, &orcv1.DependencyEdge{
					From: t.ID,
					To:   relID,
					Type: "related",
				})
			}
		}
	}

	return connect.NewResponse(&orcv1.GetDependenciesResponse{
		Graph: graph,
	}), nil
}

// ============================================================================
// Task Control Methods
// ============================================================================

// RunTask starts execution of a task.
func (s *taskServer) RunTask(
	ctx context.Context,
	req *connect.Request[orcv1.RunTaskRequest],
) (*connect.Response[orcv1.RunTaskResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Check if task is already running
	if t.Status == task.StatusRunning {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is already running"))
	}

	// Check if task is blocked by dependencies
	allTasks, _ := s.backend.LoadAllTasks()
	if allTasks != nil {
		taskMap := make(map[string]*task.Task)
		for _, at := range allTasks {
			taskMap[at.ID] = at
		}
		unmet := t.GetUnmetDependencies(taskMap)
		if len(unmet) > 0 {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task is blocked by: %v", unmet))
		}
	}

	// Set task to running
	t.Status = task.StatusRunning
	now := time.Now()
	t.StartedAt = &now
	t.UpdatedAt = now

	if err := s.backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.ID, t))
	}

	return connect.NewResponse(&orcv1.RunTaskResponse{
		Task: TaskToProto(t),
	}), nil
}

// PauseTask pauses a running task.
func (s *taskServer) PauseTask(
	ctx context.Context,
	req *connect.Request[orcv1.PauseTaskRequest],
) (*connect.Response[orcv1.PauseTaskResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Check if task is running
	if t.Status != task.StatusRunning {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is not running"))
	}

	// Set task to paused
	t.Status = task.StatusPaused
	t.UpdatedAt = time.Now()

	if err := s.backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.ID, t))
	}

	return connect.NewResponse(&orcv1.PauseTaskResponse{
		Task: TaskToProto(t),
	}), nil
}

// ResumeTask resumes a paused or failed task.
func (s *taskServer) ResumeTask(
	ctx context.Context,
	req *connect.Request[orcv1.ResumeTaskRequest],
) (*connect.Response[orcv1.ResumeTaskResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Check if task can be resumed
	if t.Status != task.StatusPaused && t.Status != task.StatusFailed && t.Status != task.StatusBlocked {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task cannot be resumed"))
	}

	// Set task to running
	t.Status = task.StatusRunning
	t.UpdatedAt = time.Now()

	if err := s.backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.ID, t))
	}

	return connect.NewResponse(&orcv1.ResumeTaskResponse{
		Task: TaskToProto(t),
	}), nil
}

// SkipBlock removes the dependency blocking the task.
func (s *taskServer) SkipBlock(
	ctx context.Context,
	req *connect.Request[orcv1.SkipBlockRequest],
) (*connect.Response[orcv1.SkipBlockResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Clear blocked_by and reset status if blocked
	if t.Status == task.StatusBlocked || len(t.BlockedBy) > 0 {
		t.BlockedBy = nil
		if t.Status == task.StatusBlocked {
			t.Status = task.StatusPlanned
		}
		t.UpdatedAt = time.Now()

		if err := s.backend.SaveTask(t); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
		}

		// Publish event
		if s.publisher != nil {
			s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.ID, t))
		}
	}

	return connect.NewResponse(&orcv1.SkipBlockResponse{
		Task: TaskToProto(t),
	}), nil
}

// ============================================================================
// Retry Methods
// ============================================================================

// RetryTask retries a failed task from a specific phase.
func (s *taskServer) RetryTask(
	ctx context.Context,
	req *connect.Request[orcv1.RetryTaskRequest],
) (*connect.Response[orcv1.RetryTaskResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Check if task can be retried
	if t.Status != task.StatusFailed && t.Status != task.StatusCompleted && t.Status != task.StatusPaused {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task cannot be retried"))
	}

	// Set up retry context
	fromPhase := "implement"
	if req.Msg.FromPhase != nil && *req.Msg.FromPhase != "" {
		fromPhase = *req.Msg.FromPhase
	}

	t.Execution.RetryContext = &task.RetryContext{
		FromPhase: fromPhase,
		ToPhase:   "",
		Reason:    "manual retry",
		Attempt:   t.Quality.TotalRetries + 1,
		Timestamp: time.Now(),
	}

	if req.Msg.Instructions != nil {
		t.Execution.RetryContext.FailureOutput = *req.Msg.Instructions
	}

	// Reset status
	t.Status = task.StatusPlanned
	t.Execution.Error = ""
	t.UpdatedAt = time.Now()

	// Increment retry counter
	if t.Quality == nil {
		t.Quality = &task.QualityMetrics{}
	}
	t.Quality.TotalRetries++

	if err := s.backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.ID, t))
	}

	return connect.NewResponse(&orcv1.RetryTaskResponse{
		Task:    TaskToProto(t),
		Message: fmt.Sprintf("Task will retry from phase: %s", fromPhase),
	}), nil
}

// RetryPreview returns information about what a retry would do.
func (s *taskServer) RetryPreview(
	ctx context.Context,
	req *connect.Request[orcv1.RetryPreviewRequest],
) (*connect.Response[orcv1.RetryPreviewResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Determine phases that would be rerun
	phasesToRerun := []string{}
	for phaseName, phaseState := range t.Execution.Phases {
		if phaseState != nil && (phaseState.Status == task.PhaseStatusFailed || phaseState.Status == task.PhaseStatusInterrupted) {
			phasesToRerun = append(phasesToRerun, phaseName)
		}
	}

	// If no failed phases, default to implement
	if len(phasesToRerun) == 0 {
		phasesToRerun = []string{"implement"}
	}

	info := &orcv1.RetryPreviewInfo{
		FromPhase:     phasesToRerun[0],
		PhasesToRerun: phasesToRerun,
	}

	if t.Execution.Error != "" {
		info.LastError = &t.Execution.Error
	}

	return connect.NewResponse(&orcv1.RetryPreviewResponse{
		Info: info,
	}), nil
}

// ============================================================================
// Finalize Methods
// ============================================================================

// FinalizeTask starts the finalize process for a completed task.
func (s *taskServer) FinalizeTask(
	ctx context.Context,
	req *connect.Request[orcv1.FinalizeTaskRequest],
) (*connect.Response[orcv1.FinalizeTaskResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Check if task can be finalized
	if t.Status != task.StatusCompleted && t.Status != task.StatusFinalizing && !req.Msg.Force {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task status is %s, expected completed", t.Status))
	}

	// Start finalize (this would normally be async, but for the RPC we return immediately)
	t.Status = task.StatusFinalizing
	t.UpdatedAt = time.Now()

	if err := s.backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.ID, t))
	}

	// Return initial finalize state (actual finalization runs async in background)
	state := &orcv1.FinalizeState{
		Synced:      false,
		TestsPassed: false,
		NeedsReview: true,
	}

	return connect.NewResponse(&orcv1.FinalizeTaskResponse{
		Task:  TaskToProto(t),
		State: state,
	}), nil
}

// GetFinalizeState returns the current finalize state for a task.
func (s *taskServer) GetFinalizeState(
	ctx context.Context,
	req *connect.Request[orcv1.GetFinalizeStateRequest],
) (*connect.Response[orcv1.GetFinalizeStateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	// Load task to get finalization state
	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	// Build finalize state from task's current state
	protoState := &orcv1.FinalizeState{
		Synced:      t.Status == task.StatusCompleted,
		TestsPassed: true, // Default assumption; real implementation would check test results
		NeedsReview: t.PR != nil && !t.PR.Merged,
	}

	// If merged, populate merge details
	if t.PR != nil && t.PR.Merged {
		protoState.Merged = true
		if t.PR.MergeCommitSHA != "" {
			protoState.MergeCommit = &t.PR.MergeCommitSHA
		}
		if t.PR.TargetBranch != "" {
			protoState.TargetBranch = &t.PR.TargetBranch
		}
	}

	return connect.NewResponse(&orcv1.GetFinalizeStateResponse{
		State: protoState,
	}), nil
}

// ============================================================================
// Diff Methods
// ============================================================================

// GetDiff returns the diff for a task's changes.
func (s *taskServer) GetDiff(
	ctx context.Context,
	req *connect.Request[orcv1.GetDiffRequest],
) (*connect.Response[orcv1.GetDiffResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	diffSvc := diff.NewService(s.projectRoot, s.diffCache)

	// Determine which diff strategy to use
	var result *diff.DiffResult

	// Strategy 1: Merged PR
	if t.PR != nil && t.PR.Merged && t.PR.MergeCommitSHA != "" {
		result, err = diffSvc.GetMergeCommitDiff(ctx, t.PR.MergeCommitSHA)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get merge commit diff: %w", err))
		}
	} else {
		// Strategy 2: Branch comparison
		base := "main"
		head := t.Branch
		if head == "" {
			head = "HEAD"
		}

		base = diffSvc.ResolveRef(ctx, base)
		head = diffSvc.ResolveRef(ctx, head)

		useWorkingTree, effectiveHead := diffSvc.ShouldIncludeWorkingTree(ctx, base, head)
		if useWorkingTree {
			head = effectiveHead
		}

		result, err = diffSvc.GetFullDiff(ctx, base, head)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get diff: %w", err))
		}
	}

	// Convert to proto
	protoDiff := diffResultToProto(result)

	return connect.NewResponse(&orcv1.GetDiffResponse{
		Diff: protoDiff,
	}), nil
}

// GetDiffStats returns just the diff statistics.
func (s *taskServer) GetDiffStats(
	ctx context.Context,
	req *connect.Request[orcv1.GetDiffStatsRequest],
) (*connect.Response[orcv1.GetDiffStatsResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	t, err := s.backend.LoadTask(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.Id))
	}

	diffSvc := diff.NewService(s.projectRoot, s.diffCache)

	var stats *diff.DiffStats

	// Strategy 1: Merged PR
	if t.PR != nil && t.PR.Merged && t.PR.MergeCommitSHA != "" {
		stats, err = diffSvc.GetStats(ctx, t.PR.MergeCommitSHA+"^", t.PR.MergeCommitSHA)
	} else {
		// Strategy 2: Branch comparison
		base := "main"
		head := t.Branch
		if head == "" {
			head = "HEAD"
		}

		base = diffSvc.ResolveRef(ctx, base)
		head = diffSvc.ResolveRef(ctx, head)

		useWorkingTree, effectiveHead := diffSvc.ShouldIncludeWorkingTree(ctx, base, head)
		if useWorkingTree {
			head = effectiveHead
		}

		stats, err = diffSvc.GetStats(ctx, base, head)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get diff stats: %w", err))
	}

	return connect.NewResponse(&orcv1.GetDiffStatsResponse{
		Stats: &orcv1.DiffStats{
			FilesChanged: int32(stats.FilesChanged),
			Additions:    int32(stats.Additions),
			Deletions:    int32(stats.Deletions),
		},
	}), nil
}

// ============================================================================
// Comment Methods
// ============================================================================

// ListComments returns all comments for a task.
func (s *taskServer) ListComments(
	ctx context.Context,
	req *connect.Request[orcv1.ListCommentsRequest],
) (*connect.Response[orcv1.ListCommentsResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	pdb, err := db.OpenProject(s.projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

	var comments []db.TaskComment

	// Filter by author type if specified
	if req.Msg.AuthorType != nil && *req.Msg.AuthorType != orcv1.AuthorType_AUTHOR_TYPE_UNSPECIFIED {
		authorType := protoToAuthorType(*req.Msg.AuthorType)
		comments, err = pdb.ListTaskCommentsByAuthorType(req.Msg.TaskId, authorType)
	} else if req.Msg.Phase != nil && *req.Msg.Phase != "" {
		comments, err = pdb.ListTaskCommentsByPhase(req.Msg.TaskId, *req.Msg.Phase)
	} else {
		comments, err = pdb.ListTaskComments(req.Msg.TaskId)
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list comments: %w", err))
	}

	protoComments := make([]*orcv1.TaskComment, len(comments))
	for i, c := range comments {
		protoComments[i] = taskCommentToProto(&c)
	}

	return connect.NewResponse(&orcv1.ListCommentsResponse{
		Comments: protoComments,
	}), nil
}

// CreateComment creates a new comment on a task.
func (s *taskServer) CreateComment(
	ctx context.Context,
	req *connect.Request[orcv1.CreateCommentRequest],
) (*connect.Response[orcv1.CreateCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content is required"))
	}

	pdb, err := db.OpenProject(s.projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

	author := "user"
	if req.Msg.Author != nil {
		author = *req.Msg.Author
	}

	authorType := db.AuthorTypeHuman
	if req.Msg.AuthorType != nil {
		authorType = protoToAuthorType(*req.Msg.AuthorType)
	}

	comment := &db.TaskComment{
		TaskID:     req.Msg.TaskId,
		Content:    req.Msg.Content,
		Author:     author,
		AuthorType: authorType,
	}

	if req.Msg.Phase != nil {
		comment.Phase = *req.Msg.Phase
	}

	if err := pdb.CreateTaskComment(comment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save comment: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateCommentResponse{
		Comment: taskCommentToProto(comment),
	}), nil
}

// UpdateComment updates an existing comment.
func (s *taskServer) UpdateComment(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateCommentRequest],
) (*connect.Response[orcv1.UpdateCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.CommentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment_id is required"))
	}

	pdb, err := db.OpenProject(s.projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

	comment, err := pdb.GetTaskComment(req.Msg.CommentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get comment: %w", err))
	}
	if comment == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("comment not found"))
	}

	if req.Msg.Content != nil {
		comment.Content = *req.Msg.Content
	}
	if req.Msg.Phase != nil {
		comment.Phase = *req.Msg.Phase
	}

	if err := pdb.UpdateTaskComment(comment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update comment: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateCommentResponse{
		Comment: taskCommentToProto(comment),
	}), nil
}

// DeleteComment deletes a comment.
func (s *taskServer) DeleteComment(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteCommentRequest],
) (*connect.Response[orcv1.DeleteCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.CommentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment_id is required"))
	}

	pdb, err := db.OpenProject(s.projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

	if err := pdb.DeleteTaskComment(req.Msg.CommentId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete comment: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteCommentResponse{
		Message: "Comment deleted",
	}), nil
}

// ============================================================================
// Review Comment Methods
// ============================================================================

// ListReviewComments returns all review comments for a task.
func (s *taskServer) ListReviewComments(
	ctx context.Context,
	req *connect.Request[orcv1.ListReviewCommentsRequest],
) (*connect.Response[orcv1.ListReviewCommentsResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	pdb, err := db.OpenProject(s.projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

	status := ""
	if req.Msg.Status != nil && *req.Msg.Status != orcv1.CommentStatus_COMMENT_STATUS_UNSPECIFIED {
		status = protoToCommentStatus(*req.Msg.Status)
	}

	comments, err := pdb.ListReviewComments(req.Msg.TaskId, status)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list review comments: %w", err))
	}

	protoComments := make([]*orcv1.ReviewComment, len(comments))
	for i, c := range comments {
		protoComments[i] = reviewCommentToProto(&c)
	}

	return connect.NewResponse(&orcv1.ListReviewCommentsResponse{
		Comments: protoComments,
	}), nil
}

// CreateReviewComment creates a new review comment.
func (s *taskServer) CreateReviewComment(
	ctx context.Context,
	req *connect.Request[orcv1.CreateReviewCommentRequest],
) (*connect.Response[orcv1.CreateReviewCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content is required"))
	}

	pdb, err := db.OpenProject(s.projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

	severity := db.SeveritySuggestion
	if req.Msg.Severity != orcv1.CommentSeverity_COMMENT_SEVERITY_UNSPECIFIED {
		severity = protoToCommentSeverity(req.Msg.Severity)
	}

	reviewRound := int(req.Msg.ReviewRound)
	if reviewRound == 0 {
		// Get latest review round
		latest, err := pdb.GetLatestReviewRound(req.Msg.TaskId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get review round: %w", err))
		}
		reviewRound = latest
		if reviewRound == 0 {
			reviewRound = 1
		}
	}

	comment := &db.ReviewComment{
		TaskID:      req.Msg.TaskId,
		ReviewRound: reviewRound,
		Content:     req.Msg.Content,
		Severity:    severity,
	}

	if req.Msg.FilePath != nil {
		comment.FilePath = *req.Msg.FilePath
	}
	if req.Msg.LineNumber != nil {
		comment.LineNumber = int(*req.Msg.LineNumber)
	}

	if err := pdb.CreateReviewComment(comment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create review comment: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateReviewCommentResponse{
		Comment: reviewCommentToProto(comment),
	}), nil
}

// UpdateReviewComment updates an existing review comment.
func (s *taskServer) UpdateReviewComment(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateReviewCommentRequest],
) (*connect.Response[orcv1.UpdateReviewCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.CommentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment_id is required"))
	}

	pdb, err := db.OpenProject(s.projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

	comment, err := pdb.GetReviewComment(req.Msg.CommentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get comment: %w", err))
	}
	if comment == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("comment not found"))
	}

	if req.Msg.Content != nil {
		comment.Content = *req.Msg.Content
	}
	if req.Msg.Status != nil && *req.Msg.Status != orcv1.CommentStatus_COMMENT_STATUS_UNSPECIFIED {
		comment.Status = db.ReviewCommentStatus(protoToCommentStatus(*req.Msg.Status))
		if comment.Status == db.CommentStatusResolved || comment.Status == db.CommentStatusWontFix {
			now := time.Now()
			comment.ResolvedAt = &now
		}
	}

	if err := pdb.UpdateReviewComment(comment); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update comment: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateReviewCommentResponse{
		Comment: reviewCommentToProto(comment),
	}), nil
}

// DeleteReviewComment deletes a review comment.
func (s *taskServer) DeleteReviewComment(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteReviewCommentRequest],
) (*connect.Response[orcv1.DeleteReviewCommentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.CommentId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("comment_id is required"))
	}

	pdb, err := db.OpenProject(s.projectRoot)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("open database: %w", err))
	}
	defer func() { _ = pdb.Close() }()

	if err := pdb.DeleteReviewComment(req.Msg.CommentId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete comment: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteReviewCommentResponse{
		Message: "Review comment deleted",
	}), nil
}

// ============================================================================
// Attachment Methods
// ============================================================================

// ListAttachments returns all attachments for a task.
func (s *taskServer) ListAttachments(
	ctx context.Context,
	req *connect.Request[orcv1.ListAttachmentsRequest],
) (*connect.Response[orcv1.ListAttachmentsResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	attachments, err := s.backend.ListAttachments(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list attachments: %w", err))
	}

	protoAttachments := make([]*orcv1.Attachment, len(attachments))
	for i, a := range attachments {
		protoAttachments[i] = attachmentToProto(a)
	}

	return connect.NewResponse(&orcv1.ListAttachmentsResponse{
		Attachments: protoAttachments,
	}), nil
}

// UploadAttachment uploads a file attachment (client streaming).
func (s *taskServer) UploadAttachment(
	ctx context.Context,
	stream *connect.ClientStream[orcv1.UploadAttachmentRequest],
) (*connect.Response[orcv1.UploadAttachmentResponse], error) {
	var taskID, filename, contentType string
	var data []byte

	// Receive the stream
	for stream.Receive() {
		msg := stream.Msg()
		switch d := msg.Data.(type) {
		case *orcv1.UploadAttachmentRequest_Metadata:
			taskID = d.Metadata.TaskId
			filename = d.Metadata.Filename
			contentType = d.Metadata.ContentType
		case *orcv1.UploadAttachmentRequest_Chunk:
			data = append(data, d.Chunk...)
		}
	}

	if err := stream.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("stream error: %w", err))
	}

	if taskID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if filename == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("filename is required"))
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	attachment, err := s.backend.SaveAttachment(taskID, filename, contentType, data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save attachment: %w", err))
	}

	return connect.NewResponse(&orcv1.UploadAttachmentResponse{
		Attachment: attachmentToProto(attachment),
	}), nil
}

// DownloadAttachment downloads a file attachment (server streaming).
func (s *taskServer) DownloadAttachment(
	ctx context.Context,
	req *connect.Request[orcv1.DownloadAttachmentRequest],
	stream *connect.ServerStream[orcv1.DownloadAttachmentResponse],
) error {
	if req.Msg.TaskId == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.Filename == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("filename is required"))
	}

	_, data, err := s.backend.GetAttachment(req.Msg.TaskId, req.Msg.Filename)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("attachment not found: %w", err))
	}

	// Send data in chunks
	chunkSize := 64 * 1024 // 64KB chunks
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		if err := stream.Send(&orcv1.DownloadAttachmentResponse{
			Chunk: data[i:end],
		}); err != nil {
			return err
		}
	}

	return nil
}

// DeleteAttachment deletes a file attachment.
func (s *taskServer) DeleteAttachment(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteAttachmentRequest],
) (*connect.Response[orcv1.DeleteAttachmentResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.Filename == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("filename is required"))
	}

	if err := s.backend.DeleteAttachment(req.Msg.TaskId, req.Msg.Filename); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete attachment: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteAttachmentResponse{
		Message: "Attachment deleted",
	}), nil
}

// ============================================================================
// Conversion Helpers
// ============================================================================

func diffResultToProto(d *diff.DiffResult) *orcv1.DiffResult {
	if d == nil {
		return nil
	}
	result := &orcv1.DiffResult{
		Base: d.Base,
		Head: d.Head,
		Stats: &orcv1.DiffStats{
			FilesChanged: int32(d.Stats.FilesChanged),
			Additions:    int32(d.Stats.Additions),
			Deletions:    int32(d.Stats.Deletions),
		},
	}

	result.Files = make([]*orcv1.FileDiff, len(d.Files))
	for i, f := range d.Files {
		fileDiff := &orcv1.FileDiff{
			Path:      f.Path,
			Status:    f.Status,
			Additions: int32(f.Additions),
			Deletions: int32(f.Deletions),
			Binary:    f.Binary,
			Syntax:    f.Syntax,
		}
		if f.OldPath != "" {
			fileDiff.OldPath = &f.OldPath
		}
		fileDiff.Hunks = make([]*orcv1.DiffHunk, len(f.Hunks))
		for j, h := range f.Hunks {
			fileDiff.Hunks[j] = &orcv1.DiffHunk{
				OldStart: int32(h.OldStart),
				OldLines: int32(h.OldLines),
				NewStart: int32(h.NewStart),
				NewLines: int32(h.NewLines),
			}
			fileDiff.Hunks[j].Lines = make([]*orcv1.DiffLine, len(h.Lines))
			for k, l := range h.Lines {
				line := &orcv1.DiffLine{
					Type:    l.Type,
					Content: l.Content,
				}
				if l.OldLine > 0 {
					oldLine := int32(l.OldLine)
					line.OldLine = &oldLine
				}
				if l.NewLine > 0 {
					newLine := int32(l.NewLine)
					line.NewLine = &newLine
				}
				fileDiff.Hunks[j].Lines[k] = line
			}
		}
		result.Files[i] = fileDiff
	}

	return result
}

func protoToAuthorType(at orcv1.AuthorType) db.AuthorType {
	switch at {
	case orcv1.AuthorType_AUTHOR_TYPE_HUMAN:
		return db.AuthorTypeHuman
	case orcv1.AuthorType_AUTHOR_TYPE_AGENT:
		return db.AuthorTypeAgent
	case orcv1.AuthorType_AUTHOR_TYPE_SYSTEM:
		return db.AuthorTypeSystem
	default:
		return db.AuthorTypeHuman
	}
}

func authorTypeToProto(at db.AuthorType) orcv1.AuthorType {
	switch at {
	case db.AuthorTypeHuman:
		return orcv1.AuthorType_AUTHOR_TYPE_HUMAN
	case db.AuthorTypeAgent:
		return orcv1.AuthorType_AUTHOR_TYPE_AGENT
	case db.AuthorTypeSystem:
		return orcv1.AuthorType_AUTHOR_TYPE_SYSTEM
	default:
		return orcv1.AuthorType_AUTHOR_TYPE_UNSPECIFIED
	}
}

func taskCommentToProto(c *db.TaskComment) *orcv1.TaskComment {
	if c == nil {
		return nil
	}
	pb := &orcv1.TaskComment{
		Id:         c.ID,
		TaskId:     c.TaskID,
		Content:    c.Content,
		Author:     c.Author,
		AuthorType: authorTypeToProto(c.AuthorType),
		CreatedAt:  timestamppb.New(c.CreatedAt),
	}
	if c.Phase != "" {
		pb.Phase = &c.Phase
	}
	return pb
}

func protoToCommentStatus(s orcv1.CommentStatus) string {
	switch s {
	case orcv1.CommentStatus_COMMENT_STATUS_OPEN:
		return string(db.CommentStatusOpen)
	case orcv1.CommentStatus_COMMENT_STATUS_RESOLVED:
		return string(db.CommentStatusResolved)
	case orcv1.CommentStatus_COMMENT_STATUS_WONT_FIX:
		return string(db.CommentStatusWontFix)
	default:
		return ""
	}
}

func commentStatusToProto(s db.ReviewCommentStatus) orcv1.CommentStatus {
	switch s {
	case db.CommentStatusOpen:
		return orcv1.CommentStatus_COMMENT_STATUS_OPEN
	case db.CommentStatusResolved:
		return orcv1.CommentStatus_COMMENT_STATUS_RESOLVED
	case db.CommentStatusWontFix:
		return orcv1.CommentStatus_COMMENT_STATUS_WONT_FIX
	default:
		return orcv1.CommentStatus_COMMENT_STATUS_UNSPECIFIED
	}
}

func protoToCommentSeverity(s orcv1.CommentSeverity) db.ReviewCommentSeverity {
	switch s {
	case orcv1.CommentSeverity_COMMENT_SEVERITY_SUGGESTION:
		return db.SeveritySuggestion
	case orcv1.CommentSeverity_COMMENT_SEVERITY_ISSUE:
		return db.SeverityIssue
	case orcv1.CommentSeverity_COMMENT_SEVERITY_BLOCKER:
		return db.SeverityBlocker
	default:
		return db.SeveritySuggestion
	}
}

func commentSeverityToProto(s db.ReviewCommentSeverity) orcv1.CommentSeverity {
	switch s {
	case db.SeveritySuggestion:
		return orcv1.CommentSeverity_COMMENT_SEVERITY_SUGGESTION
	case db.SeverityIssue:
		return orcv1.CommentSeverity_COMMENT_SEVERITY_ISSUE
	case db.SeverityBlocker:
		return orcv1.CommentSeverity_COMMENT_SEVERITY_BLOCKER
	default:
		return orcv1.CommentSeverity_COMMENT_SEVERITY_UNSPECIFIED
	}
}

func reviewCommentToProto(c *db.ReviewComment) *orcv1.ReviewComment {
	if c == nil {
		return nil
	}
	pb := &orcv1.ReviewComment{
		Id:          c.ID,
		TaskId:      c.TaskID,
		Content:     c.Content,
		Severity:    commentSeverityToProto(c.Severity),
		Status:      commentStatusToProto(c.Status),
		ReviewRound: int32(c.ReviewRound),
		CreatedAt:   timestamppb.New(c.CreatedAt),
	}
	if c.FilePath != "" {
		pb.FilePath = &c.FilePath
	}
	if c.LineNumber > 0 {
		ln := int32(c.LineNumber)
		pb.LineNumber = &ln
	}
	if c.ResolvedAt != nil {
		pb.ResolvedAt = timestamppb.New(*c.ResolvedAt)
	}
	if c.ResolvedBy != "" {
		pb.ResolvedBy = &c.ResolvedBy
	}
	return pb
}

func attachmentToProto(a *task.Attachment) *orcv1.Attachment {
	if a == nil {
		return nil
	}
	return &orcv1.Attachment{
		Filename:    a.Filename,
		Size:        a.Size,
		ContentType: a.ContentType,
		CreatedAt:   timestamppb.New(a.CreatedAt),
		IsImage:     a.IsImage,
	}
}
