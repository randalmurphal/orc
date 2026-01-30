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
	"github.com/randalmurphal/orc/internal/workflow"
)

// TaskExecutorFunc is the callback type for spawning task executors.
// It takes a task ID and spawns a WorkflowExecutor goroutine.
// Returns error if the executor fails to spawn (not execution errors).
type TaskExecutorFunc func(taskID string) error

// taskServer implements the TaskServiceHandler interface.
type taskServer struct {
	orcv1connect.UnimplementedTaskServiceHandler
	backend      storage.Backend   // Legacy: single project backend (fallback)
	projectCache *ProjectCache     // Multi-project: cache of backends per project
	config       *config.Config
	logger       *slog.Logger
	publisher    events.Publisher
	projectRoot  string
	diffCache    *diff.Cache
	projectDB    *db.ProjectDB
	taskExecutor TaskExecutorFunc // Optional: spawns executor for RunTask
}

// getBackend returns the appropriate backend for a project ID.
// If projectID is provided and projectCache is available, uses the cache.
// Errors if projectID is provided but cache is not configured (prevents silent data leaks).
// Falls back to legacy single backend only when no projectID is specified.
func (s *taskServer) getBackend(projectID string) (storage.Backend, error) {
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

// NewTaskServer creates a new TaskService handler.
// Note: Without an executor callback, RunTask will only update status (legacy behavior).
// Use NewTaskServerWithExecutor for full execution support.
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
		backend:      backend,
		config:       cfg,
		logger:       logger,
		publisher:    publisher,
		projectRoot:  projectRoot,
		diffCache:    diffCache,
		projectDB:    projectDB,
		taskExecutor: nil, // No executor - RunTask validates only
	}
}

// NewTaskServerWithExecutor creates a TaskService handler with execution support.
// The executor callback is called by RunTask to spawn a WorkflowExecutor goroutine.
func NewTaskServerWithExecutor(
	backend storage.Backend,
	cfg *config.Config,
	logger *slog.Logger,
	publisher events.Publisher,
	projectRoot string,
	diffCache *diff.Cache,
	projectDB *db.ProjectDB,
	executor TaskExecutorFunc,
) *taskServer {
	return &taskServer{
		backend:      backend,
		config:       cfg,
		logger:       logger,
		publisher:    publisher,
		projectRoot:  projectRoot,
		diffCache:    diffCache,
		projectDB:    projectDB,
		taskExecutor: executor,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *taskServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// ListTasks returns all tasks with optional filtering.
func (s *taskServer) ListTasks(
	ctx context.Context,
	req *connect.Request[orcv1.ListTasksRequest],
) (*connect.Response[orcv1.ListTasksResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	tasks, err := backend.LoadAllTasks()
	if err != nil {
		// Return empty list if no tasks yet
		return connect.NewResponse(&orcv1.ListTasksResponse{
			Tasks: []*orcv1.Task{},
			Page:  &orcv1.PageResponse{Total: 0},
		}), nil
	}

	if tasks == nil {
		tasks = []*orcv1.Task{}
	}

	// Filter by initiative if requested
	if req.Msg.InitiativeId != nil && *req.Msg.InitiativeId != "" {
		var filtered []*orcv1.Task
		for _, t := range tasks {
			if task.GetInitiativeIDProto(t) == *req.Msg.InitiativeId {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Filter by status if requested
	if len(req.Msg.Statuses) > 0 {
		var filtered []*orcv1.Task
		statusSet := make(map[orcv1.TaskStatus]bool)
		for _, status := range req.Msg.Statuses {
			statusSet[status] = true
		}
		for _, t := range tasks {
			if statusSet[t.Status] {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Filter by queue if requested
	if req.Msg.Queue != nil {
		var filtered []*orcv1.Task
		for _, t := range tasks {
			if task.GetQueueProto(t) == *req.Msg.Queue {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Filter by category if requested
	if req.Msg.Category != nil {
		var filtered []*orcv1.Task
		for _, t := range tasks {
			if task.GetCategoryProto(t) == *req.Msg.Category {
				filtered = append(filtered, t)
			}
		}
		tasks = filtered
	}

	// Populate computed dependency fields
	task.PopulateComputedFieldsProto(tasks)

	// Filter by dependency status if requested
	if req.Msg.DependencyStatus != nil {
		var filtered []*orcv1.Task
		for _, t := range tasks {
			if t.DependencyStatus == *req.Msg.DependencyStatus {
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
		tasks = []*orcv1.Task{}
	}

	// Calculate pagination response
	totalPages := (totalCount + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return connect.NewResponse(&orcv1.ListTasksResponse{
		Tasks: tasks,
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
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Populate computed fields
	allTasks, _ := backend.LoadAllTasks()
	if allTasks != nil {
		t.Blocks = task.ComputeBlocksProto(t.Id, allTasks)
		t.ReferencedBy = task.ComputeReferencedByProto(t.Id, allTasks)
		taskMap := make(map[string]*orcv1.Task)
		for _, at := range allTasks {
			taskMap[at.Id] = at
		}
		t.UnmetBlockers = task.GetUnmetDependenciesProto(t, taskMap)
		t.IsBlocked = len(t.UnmetBlockers) > 0
		t.DependencyStatus = task.ComputeDependencyStatusProto(t)
	}

	return connect.NewResponse(&orcv1.GetTaskResponse{
		Task: t,
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

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Generate a new task ID
	id, err := backend.GetNextTaskID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("generate task ID: %w", err))
	}

	// Create the task using proto type
	t := task.NewProtoTask(id, req.Msg.Title)

	// Set optional fields from request - direct proto assignments
	if req.Msg.Description != nil {
		t.Description = req.Msg.Description
	}
	if req.Msg.Weight != orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED {
		t.Weight = req.Msg.Weight
	}
	if req.Msg.Queue != nil {
		t.Queue = *req.Msg.Queue
	}
	if req.Msg.Priority != nil {
		t.Priority = *req.Msg.Priority
	}
	if req.Msg.Category != nil {
		t.Category = *req.Msg.Category
	}
	if req.Msg.InitiativeId != nil {
		t.InitiativeId = req.Msg.InitiativeId
	}
	// Auto-assign workflow based on weight if not explicitly provided
	if req.Msg.WorkflowId != nil {
		t.WorkflowId = req.Msg.WorkflowId
	} else if t.Weight != orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED {
		wfID := workflow.WeightToWorkflowID(t.Weight)
		if wfID != "" {
			t.WorkflowId = &wfID
		}
	}
	if req.Msg.TargetBranch != nil {
		t.TargetBranch = req.Msg.TargetBranch
	}
	if len(req.Msg.BlockedBy) > 0 {
		t.BlockedBy = req.Msg.BlockedBy
	}
	if len(req.Msg.RelatedTo) > 0 {
		t.RelatedTo = req.Msg.RelatedTo
	}

	// Save the task
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskCreated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.CreateTaskResponse{
		Task: t,
	}), nil
}

// UpdateTask updates an existing task.
func (s *taskServer) UpdateTask(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateTaskRequest],
) (*connect.Response[orcv1.UpdateTaskResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Load existing task
	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Apply updates - direct proto field assignments
	if req.Msg.Title != nil {
		t.Title = *req.Msg.Title
	}
	if req.Msg.Description != nil {
		t.Description = req.Msg.Description
	}
	if req.Msg.Weight != nil {
		t.Weight = *req.Msg.Weight
	}
	if req.Msg.Queue != nil {
		t.Queue = *req.Msg.Queue
	}
	if req.Msg.Priority != nil {
		t.Priority = *req.Msg.Priority
	}
	if req.Msg.Category != nil {
		t.Category = *req.Msg.Category
	}
	if req.Msg.InitiativeId != nil {
		t.InitiativeId = req.Msg.InitiativeId
	}
	if req.Msg.TargetBranch != nil {
		t.TargetBranch = req.Msg.TargetBranch
	}
	if req.Msg.BlockedBy != nil {
		t.BlockedBy = req.Msg.BlockedBy
	}
	if req.Msg.RelatedTo != nil {
		t.RelatedTo = req.Msg.RelatedTo
	}
	if req.Msg.WorkflowId != nil {
		t.WorkflowId = req.Msg.WorkflowId
	}

	// Update timestamp
	task.UpdateTimestampProto(t)

	// Save the task
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.UpdateTaskResponse{
		Task: t,
	}), nil
}

// DeleteTask deletes a task.
func (s *taskServer) DeleteTask(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteTaskRequest],
) (*connect.Response[orcv1.DeleteTaskResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Check task exists
	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check if task is running
	if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("cannot delete running task"))
	}

	// Delete the task
	if err := backend.DeleteTask(req.Msg.TaskId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskDeleted, req.Msg.TaskId, map[string]string{"task_id": req.Msg.TaskId}))
	}

	return connect.NewResponse(&orcv1.DeleteTaskResponse{}), nil
}

// GetTaskState returns the execution state for a task.
func (s *taskServer) GetTaskState(
	ctx context.Context,
	req *connect.Request[orcv1.GetTaskStateRequest],
) (*connect.Response[orcv1.GetTaskStateResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	return connect.NewResponse(&orcv1.GetTaskStateResponse{
		State: t.Execution,
	}), nil
}

// GetTaskPlan returns the plan for a task.
func (s *taskServer) GetTaskPlan(
	ctx context.Context,
	req *connect.Request[orcv1.GetTaskPlanRequest],
) (*connect.Response[orcv1.GetTaskPlanResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Build TaskPlan from execution state
	plan := &orcv1.TaskPlan{
		Version: 1,
		Weight:  t.Weight,
	}
	if t.Description != nil {
		plan.Description = *t.Description
	}

	// Get workflow phases to determine correct order
	var phaseOrder []string
	if t.WorkflowId != nil && *t.WorkflowId != "" {
		workflowPhases, err := s.projectDB.GetWorkflowPhases(*t.WorkflowId)
		if err == nil && len(workflowPhases) > 0 {
			// Build phase order from workflow (already sorted by sequence)
			for _, wp := range workflowPhases {
				phaseOrder = append(phaseOrder, wp.PhaseTemplateID)
			}
		}
	}

	// Build phase list from workflow (show all phases, not just started ones)
	if len(phaseOrder) > 0 {
		// Use workflow order - include ALL workflow phases
		for _, phaseName := range phaseOrder {
			if phaseName == "" {
				continue // Skip empty phase names
			}
			planPhase := &orcv1.PlanPhase{
				Name:   phaseName,
				Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING, // Default
			}
			// If we have execution state for this phase, use it
			if t.Execution != nil && t.Execution.Phases != nil {
				if phase, exists := t.Execution.Phases[phaseName]; exists {
					planPhase.Status = phase.Status
				}
			}
			plan.Phases = append(plan.Phases, planPhase)
		}
		// Add any phases from execution state not in workflow (shouldn't happen but be safe)
		if t.Execution != nil && t.Execution.Phases != nil {
			for phaseName, phase := range t.Execution.Phases {
				if phaseName == "" {
					continue // Skip empty phase names
				}
				found := false
				for _, orderedName := range phaseOrder {
					if phaseName == orderedName {
						found = true
						break
					}
				}
				if !found {
					planPhase := &orcv1.PlanPhase{
						Name:   phaseName,
						Status: phase.Status,
					}
					plan.Phases = append(plan.Phases, planPhase)
				}
			}
		}
	} else if t.Execution != nil && t.Execution.Phases != nil {
		// Fallback: no workflow order available, use map iteration (random)
		for phaseName, phase := range t.Execution.Phases {
			if phaseName == "" {
				continue // Skip empty phase names
			}
			planPhase := &orcv1.PlanPhase{
				Name:   phaseName,
				Status: phase.Status,
			}
			plan.Phases = append(plan.Phases, planPhase)
		}
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
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Load all tasks to compute dependencies
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks: %w", err))
	}

	taskMap := make(map[string]*orcv1.Task)
	for _, at := range allTasks {
		taskMap[at.Id] = at
	}

	// Compute dependency info
	blocks := task.ComputeBlocksProto(t.Id, allTasks)

	// Build dependency graph
	graph := &orcv1.DependencyGraph{
		Nodes: make([]*orcv1.DependencyNode, 0),
		Edges: make([]*orcv1.DependencyEdge, 0),
	}

	// Add the target task as a node
	graph.Nodes = append(graph.Nodes, &orcv1.DependencyNode{
		Id:     t.Id,
		Title:  t.Title,
		Status: t.Status,
	})

	// Add blockers as nodes and edges
	for _, blockerID := range t.BlockedBy {
		blocker, exists := taskMap[blockerID]
		if exists {
			graph.Nodes = append(graph.Nodes, &orcv1.DependencyNode{
				Id:     blocker.Id,
				Title:  blocker.Title,
				Status: blocker.Status,
			})
			graph.Edges = append(graph.Edges, &orcv1.DependencyEdge{
				From: blockerID,
				To:   t.Id,
				Type: "blocks",
			})
		}
	}

	// Add blocked tasks as nodes and edges
	for _, blockID := range blocks {
		blocked, exists := taskMap[blockID]
		if exists {
			graph.Nodes = append(graph.Nodes, &orcv1.DependencyNode{
				Id:     blocked.Id,
				Title:  blocked.Title,
				Status: blocked.Status,
			})
			graph.Edges = append(graph.Edges, &orcv1.DependencyEdge{
				From: t.Id,
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
					Id:     rel.Id,
					Title:  rel.Title,
					Status: rel.Status,
				})
				graph.Edges = append(graph.Edges, &orcv1.DependencyEdge{
					From: t.Id,
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

// AddBlocker adds a blocker relationship to a task.
func (s *taskServer) AddBlocker(
	ctx context.Context,
	req *connect.Request[orcv1.AddBlockerRequest],
) (*connect.Response[orcv1.AddBlockerResponse], error) {
	if req.Msg.TaskId == "" || req.Msg.BlockerId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id and blocker_id are required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check blocker exists
	if _, err := backend.LoadTask(req.Msg.BlockerId); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("blocker task %s not found", req.Msg.BlockerId))
	}

	// Check for self-reference
	if req.Msg.TaskId == req.Msg.BlockerId {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task cannot block itself"))
	}

	// Add blocker if not already present
	for _, existing := range t.BlockedBy {
		if existing == req.Msg.BlockerId {
			return connect.NewResponse(&orcv1.AddBlockerResponse{Task: t}), nil
		}
	}

	t.BlockedBy = append(t.BlockedBy, req.Msg.BlockerId)
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	s.publisher.Publish(events.Event{Type: "task_updated", TaskID: t.Id, Data: t})
	return connect.NewResponse(&orcv1.AddBlockerResponse{Task: t}), nil
}

// RemoveBlocker removes a blocker relationship from a task.
func (s *taskServer) RemoveBlocker(
	ctx context.Context,
	req *connect.Request[orcv1.RemoveBlockerRequest],
) (*connect.Response[orcv1.RemoveBlockerResponse], error) {
	if req.Msg.TaskId == "" || req.Msg.BlockerId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id and blocker_id are required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Remove blocker
	filtered := make([]string, 0, len(t.BlockedBy))
	for _, id := range t.BlockedBy {
		if id != req.Msg.BlockerId {
			filtered = append(filtered, id)
		}
	}
	t.BlockedBy = filtered

	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	s.publisher.Publish(events.Event{Type: "task_updated", TaskID: t.Id, Data: t})
	return connect.NewResponse(&orcv1.RemoveBlockerResponse{}), nil
}

// AddRelated adds a related task relationship.
func (s *taskServer) AddRelated(
	ctx context.Context,
	req *connect.Request[orcv1.AddRelatedRequest],
) (*connect.Response[orcv1.AddRelatedResponse], error) {
	if req.Msg.TaskId == "" || req.Msg.RelatedId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id and related_id are required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check related task exists
	if _, err := backend.LoadTask(req.Msg.RelatedId); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("related task %s not found", req.Msg.RelatedId))
	}

	// Check for self-reference
	if req.Msg.TaskId == req.Msg.RelatedId {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task cannot be related to itself"))
	}

	// Add related if not already present
	for _, existing := range t.RelatedTo {
		if existing == req.Msg.RelatedId {
			return connect.NewResponse(&orcv1.AddRelatedResponse{Task: t}), nil
		}
	}

	t.RelatedTo = append(t.RelatedTo, req.Msg.RelatedId)
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	s.publisher.Publish(events.Event{Type: "task_updated", TaskID: t.Id, Data: t})
	return connect.NewResponse(&orcv1.AddRelatedResponse{Task: t}), nil
}

// RemoveRelated removes a related task relationship.
func (s *taskServer) RemoveRelated(
	ctx context.Context,
	req *connect.Request[orcv1.RemoveRelatedRequest],
) (*connect.Response[orcv1.RemoveRelatedResponse], error) {
	if req.Msg.TaskId == "" || req.Msg.RelatedId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id and related_id are required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Remove related
	filtered := make([]string, 0, len(t.RelatedTo))
	for _, id := range t.RelatedTo {
		if id != req.Msg.RelatedId {
			filtered = append(filtered, id)
		}
	}
	t.RelatedTo = filtered

	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	s.publisher.Publish(events.Event{Type: "task_updated", TaskID: t.Id, Data: t})
	return connect.NewResponse(&orcv1.RemoveRelatedResponse{}), nil
}

// ============================================================================
// Task Control Methods
// ============================================================================

// RunTask starts execution of a task.
// This validates the task can be run and spawns an executor via callback.
func (s *taskServer) RunTask(
	ctx context.Context,
	req *connect.Request[orcv1.RunTaskRequest],
) (*connect.Response[orcv1.RunTaskResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Validate workflow_id BEFORE any status changes
	workflowID := t.GetWorkflowId()
	if workflowID == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task has no workflow_id set"))
	}

	// Validate task status allows running
	switch t.Status {
	case orcv1.TaskStatus_TASK_STATUS_RUNNING:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is already running"))
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is already completed"))
	case orcv1.TaskStatus_TASK_STATUS_PAUSED:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is paused - use resume instead"))
	case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is blocked"))
	case orcv1.TaskStatus_TASK_STATUS_CREATED,
		orcv1.TaskStatus_TASK_STATUS_PLANNED,
		orcv1.TaskStatus_TASK_STATUS_FAILED: // Failed tasks can be retried
		// OK to run
	default:
		// Allow other statuses (FINALIZING, etc.) to proceed for flexibility
	}

	// Check if task is blocked by dependencies
	allTasks, _ := backend.LoadAllTasks()
	if allTasks != nil {
		taskMap := make(map[string]*orcv1.Task)
		for _, at := range allTasks {
			taskMap[at.Id] = at
		}
		unmet := task.GetUnmetDependenciesProto(t, taskMap)
		if len(unmet) > 0 {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task is blocked by: %v", unmet))
		}
	}

	// Store original status for rollback if executor fails
	originalStatus := t.Status

	// Set task to running
	task.MarkStartedProto(t)

	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish status update event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	// Spawn executor if callback is set
	if s.taskExecutor != nil {
		if err := s.taskExecutor(t.Id); err != nil {
			// Executor failed to spawn - revert status
			t.Status = originalStatus
			task.UpdateTimestampProto(t)
			if saveErr := backend.SaveTask(t); saveErr != nil {
				// Log but don't mask the original error
				if s.logger != nil {
					s.logger.Error("failed to revert task status after executor failure",
						"task", t.Id, "error", saveErr)
				}
			}
			// Publish status revert event
			if s.publisher != nil {
				s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
			}
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("spawn executor: %w", err))
		}
	}

	return connect.NewResponse(&orcv1.RunTaskResponse{
		Task: t,
	}), nil
}

// PauseTask pauses a running task.
func (s *taskServer) PauseTask(
	ctx context.Context,
	req *connect.Request[orcv1.PauseTaskRequest],
) (*connect.Response[orcv1.PauseTaskResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check if task is running
	if t.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task is not running"))
	}

	// Set task to paused
	t.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.PauseTaskResponse{
		Task: t,
	}), nil
}

// ResumeTask resumes a paused or failed task.
func (s *taskServer) ResumeTask(
	ctx context.Context,
	req *connect.Request[orcv1.ResumeTaskRequest],
) (*connect.Response[orcv1.ResumeTaskResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check if task can be resumed
	if t.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_FAILED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task cannot be resumed"))
	}

	// Set task to running
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.ResumeTaskResponse{
		Task: t,
	}), nil
}

// SkipBlock removes the dependency blocking the task.
func (s *taskServer) SkipBlock(
	ctx context.Context,
	req *connect.Request[orcv1.SkipBlockRequest],
) (*connect.Response[orcv1.SkipBlockResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Clear blocked_by and reset status if blocked
	if t.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED || len(t.BlockedBy) > 0 {
		t.BlockedBy = nil
		if t.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED {
			t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
		}
		task.UpdateTimestampProto(t)

		if err := backend.SaveTask(t); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
		}

		// Publish event
		if s.publisher != nil {
			s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
		}
	}

	return connect.NewResponse(&orcv1.SkipBlockResponse{
		Task: t,
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
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check if task can be retried
	if t.Status != orcv1.TaskStatus_TASK_STATUS_FAILED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("task cannot be retried"))
	}

	// Set up retry context
	fromPhase := "implement"
	if req.Msg.FromPhase != nil && *req.Msg.FromPhase != "" {
		fromPhase = *req.Msg.FromPhase
	}

	// Ensure execution state is initialized
	task.EnsureExecutionProto(t)

	// Get current retry count
	var currentRetries int32
	if t.Quality != nil {
		currentRetries = t.Quality.TotalRetries
	}

	// Set retry context using proto helper
	instructions := ""
	if req.Msg.Instructions != nil {
		instructions = *req.Msg.Instructions
	}
	task.SetRetryContextProto(t.Execution, fromPhase, "", "manual retry", instructions, currentRetries+1)

	// Reset status
	t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	if t.Execution != nil {
		t.Execution.Error = nil
	}
	task.UpdateTimestampProto(t)

	// Increment retry counter
	task.EnsureQualityMetricsProto(t)
	t.Quality.TotalRetries++

	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	return connect.NewResponse(&orcv1.RetryTaskResponse{
		Task:    t,
		Message: fmt.Sprintf("Task will retry from phase: %s", fromPhase),
	}), nil
}

// RetryPreview returns information about what a retry would do.
func (s *taskServer) RetryPreview(
	ctx context.Context,
	req *connect.Request[orcv1.RetryPreviewRequest],
) (*connect.Response[orcv1.RetryPreviewResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Determine phases that would be rerun
	// Use current_phase as the starting point since phases no longer track running/failed status
	phasesToRerun := []string{}
	currentPhase := task.GetCurrentPhaseProto(t)
	if currentPhase != "" {
		phasesToRerun = append(phasesToRerun, currentPhase)
	}

	// If no failed phases, default to implement
	if len(phasesToRerun) == 0 {
		phasesToRerun = []string{"implement"}
	}

	info := &orcv1.RetryPreviewInfo{
		FromPhase:     phasesToRerun[0],
		PhasesToRerun: phasesToRerun,
	}

	if t.Execution != nil && t.Execution.Error != nil && *t.Execution.Error != "" {
		info.LastError = t.Execution.Error
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
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check if task can be finalized
	if t.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED &&
		t.Status != orcv1.TaskStatus_TASK_STATUS_FINALIZING && !req.Msg.Force {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task status is %s, expected completed", t.Status))
	}

	// Start finalize (this would normally be async, but for the RPC we return immediately)
	t.Status = orcv1.TaskStatus_TASK_STATUS_FINALIZING
	task.UpdateTimestampProto(t)

	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
	}

	// Return initial finalize state (actual finalization runs async in background)
	state := &orcv1.FinalizeState{
		Synced:      false,
		TestsPassed: false,
		NeedsReview: true,
	}

	return connect.NewResponse(&orcv1.FinalizeTaskResponse{
		Task:  t,
		State: state,
	}), nil
}

// GetFinalizeState returns the current finalize state for a task.
func (s *taskServer) GetFinalizeState(
	ctx context.Context,
	req *connect.Request[orcv1.GetFinalizeStateRequest],
) (*connect.Response[orcv1.GetFinalizeStateResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Load task to get finalization state
	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Build finalize state from task's current state
	protoState := &orcv1.FinalizeState{
		Synced:      t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED,
		TestsPassed: true, // Default assumption; real implementation would check test results
		NeedsReview: t.Pr != nil && !t.Pr.Merged,
	}

	// If merged, populate merge details
	if t.Pr != nil && t.Pr.Merged {
		protoState.Merged = true
		if t.Pr.MergeCommitSha != nil && *t.Pr.MergeCommitSha != "" {
			protoState.MergeCommit = t.Pr.MergeCommitSha
		}
		if t.Pr.TargetBranch != nil && *t.Pr.TargetBranch != "" {
			protoState.TargetBranch = t.Pr.TargetBranch
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
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	diffSvc := diff.NewService(s.projectRoot, s.diffCache)

	// Determine which diff strategy to use
	var result *diff.DiffResult

	// Strategy 1: Merged PR
	if t.Pr != nil && t.Pr.Merged && t.Pr.MergeCommitSha != nil && *t.Pr.MergeCommitSha != "" {
		result, err = diffSvc.GetMergeCommitDiff(ctx, *t.Pr.MergeCommitSha)
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
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	diffSvc := diff.NewService(s.projectRoot, s.diffCache)

	var stats *diff.DiffStats

	// Strategy 1: Merged PR
	if t.Pr != nil && t.Pr.Merged && t.Pr.MergeCommitSha != nil && *t.Pr.MergeCommitSha != "" {
		sha := *t.Pr.MergeCommitSha
		stats, err = diffSvc.GetStats(ctx, sha+"^", sha)
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

// GetFileDiff returns the diff for a single file with hunks.
func (s *taskServer) GetFileDiff(
	ctx context.Context,
	req *connect.Request[orcv1.GetFileDiffRequest],
) (*connect.Response[orcv1.GetFileDiffResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.FilePath == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("file_path is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	diffSvc := diff.NewService(s.projectRoot, s.diffCache)

	var fileDiff *diff.FileDiff

	// Strategy 1: Merged PR with merge commit SHA
	if t.Pr != nil && t.Pr.Merged && t.Pr.MergeCommitSha != nil && *t.Pr.MergeCommitSha != "" {
		fileDiff, err = diffSvc.GetMergeCommitFileDiff(ctx, *t.Pr.MergeCommitSha, req.Msg.FilePath)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get merge commit file diff: %w", err))
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

		fileDiff, err = diffSvc.GetFileDiff(ctx, base, head, req.Msg.FilePath)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get file diff: %w", err))
		}
	}

	// Convert to proto
	protoFileDiff := fileDiffToProto(fileDiff)

	return connect.NewResponse(&orcv1.GetFileDiffResponse{
		File: protoFileDiff,
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

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	attachments, err := backend.ListAttachments(req.Msg.TaskId)
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

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	if err := backend.DeleteAttachment(req.Msg.TaskId, req.Msg.Filename); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete attachment: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteAttachmentResponse{
		Message: "Attachment deleted",
	}), nil
}

// ============================================================================
// Conversion Helpers
// ============================================================================

func fileDiffToProto(f *diff.FileDiff) *orcv1.FileDiff {
	if f == nil {
		return nil
	}
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
	return fileDiff
}

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

// ExportTask exports task artifacts to the filesystem or a git branch.
func (s *taskServer) ExportTask(
	ctx context.Context,
	req *connect.Request[orcv1.ExportTaskRequest],
) (*connect.Response[orcv1.ExportTaskResponse], error) {
	taskID := req.Msg.TaskId
	if taskID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task ID required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Check if task exists
	exists, err := backend.TaskExists(taskID)
	if err != nil || !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", taskID))
	}

	// Load config for export settings
	cfg, err := config.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load config: %w", err))
	}

	// Build export options from request or defaults
	resolved := cfg.Storage.ResolveExportConfig()
	opts := &storage.ExportOptions{
		TaskDefinition: resolved.TaskDefinition,
		FinalState:     resolved.FinalState,
		Transcripts:    resolved.Transcripts,
		ContextSummary: resolved.ContextSummary,
	}

	// Override with request values if provided
	if req.Msg.TaskDefinition != nil {
		opts.TaskDefinition = *req.Msg.TaskDefinition
	}
	if req.Msg.FinalState != nil {
		opts.FinalState = *req.Msg.FinalState
	}
	if req.Msg.Transcripts != nil {
		opts.Transcripts = *req.Msg.Transcripts
	}
	if req.Msg.ContextSummary != nil {
		opts.ContextSummary = *req.Msg.ContextSummary
	}

	// Create export service
	exportBackend, err := storage.NewBackend(s.projectRoot, &cfg.Storage)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create storage backend: %w", err))
	}
	defer func() { _ = exportBackend.Close() }()

	exportSvc := storage.NewExportService(exportBackend, &cfg.Storage)

	// Perform export
	if req.Msg.ToBranch {
		// Get current branch for the task
		t, err := backend.LoadTask(taskID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load task: %w", err))
		}

		if err := exportSvc.ExportToBranch(taskID, t.GetBranch(), opts); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to export to branch: %w", err))
		}

		return connect.NewResponse(&orcv1.ExportTaskResponse{
			Success:    true,
			TaskId:     taskID,
			ExportedTo: t.GetBranch(),
		}), nil
	}

	if err := exportSvc.Export(taskID, opts); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to export: %w", err))
	}

	return connect.NewResponse(&orcv1.ExportTaskResponse{
		Success:    true,
		TaskId:     taskID,
		ExportedTo: ".orc/exports/" + taskID,
	}), nil
}

// PauseAllTasks pauses all currently running tasks.
func (s *taskServer) PauseAllTasks(
	ctx context.Context,
	req *connect.Request[orcv1.PauseAllTasksRequest],
) (*connect.Response[orcv1.PauseAllTasksResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	var pausedTasks []*orcv1.Task
	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
			t.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
			if err := backend.SaveTask(t); err != nil {
				s.logger.Warn("failed to pause task", "task_id", t.Id, "error", err)
				continue
			}
			pausedTasks = append(pausedTasks, t)
			s.publisher.Publish(events.Event{
				Type:   events.EventTaskUpdated,
				TaskID: t.Id,
				Data:   t,
			})
		}
	}

	return connect.NewResponse(&orcv1.PauseAllTasksResponse{
		Tasks: pausedTasks,
		Count: int32(len(pausedTasks)),
	}), nil
}

// ResumeAllTasks resumes all paused tasks.
func (s *taskServer) ResumeAllTasks(
	ctx context.Context,
	req *connect.Request[orcv1.ResumeAllTasksRequest],
) (*connect.Response[orcv1.ResumeAllTasksResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load tasks: %w", err))
	}

	var resumedTasks []*orcv1.Task
	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_PAUSED {
			t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
			if err := backend.SaveTask(t); err != nil {
				s.logger.Warn("failed to resume task", "task_id", t.Id, "error", err)
				continue
			}
			resumedTasks = append(resumedTasks, t)
			s.publisher.Publish(events.Event{
				Type:   events.EventTaskUpdated,
				TaskID: t.Id,
				Data:   t,
			})
		}
	}

	return connect.NewResponse(&orcv1.ResumeAllTasksResponse{
		Tasks: resumedTasks,
		Count: int32(len(resumedTasks)),
	}), nil
}
