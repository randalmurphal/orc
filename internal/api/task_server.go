// Package api provides the Connect RPC and REST API server for orc.
// This file implements the TaskService Connect RPC service.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// TaskExecutorFunc is the callback type for spawning task executors.
// It takes a task ID and spawns a WorkflowExecutor goroutine.
// Returns error if the executor fails to spawn (not execution errors).
type TaskExecutorFunc func(taskID, projectID string) error

// TaskLifecycleTriggerRunner evaluates lifecycle triggers for task creation events.
type TaskLifecycleTriggerRunner interface {
	RunLifecycleTriggers(ctx context.Context, event workflow.WorkflowTriggerEvent, triggers []workflow.WorkflowTrigger, task *orcv1.Task) error
}

// taskServer implements the TaskServiceHandler interface.
type taskServer struct {
	orcv1connect.UnimplementedTaskServiceHandler
	backend       storage.Backend // Legacy: single project backend (fallback)
	projectCache  *ProjectCache   // Multi-project: cache of backends per project
	config        *config.Config
	logger        *slog.Logger
	publisher     events.Publisher
	projectRoot   string
	diffCache     *diff.Cache
	projectDB     *db.ProjectDB
	taskExecutor  TaskExecutorFunc           // Optional: spawns executor for RunTask
	triggerRunner TaskLifecycleTriggerRunner // Optional: evaluates lifecycle triggers
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

// NewTaskServerWithTriggerRunner creates a TaskService handler with trigger runner support.
func NewTaskServerWithTriggerRunner(
	backend storage.Backend,
	cfg *config.Config,
	logger *slog.Logger,
	publisher events.Publisher,
	projectRoot string,
	diffCache *diff.Cache,
	projectDB *db.ProjectDB,
	triggerRunner TaskLifecycleTriggerRunner,
) *taskServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &taskServer{
		backend:       backend,
		config:        cfg,
		logger:        logger,
		publisher:     publisher,
		projectRoot:   projectRoot,
		diffCache:     diffCache,
		projectDB:     projectDB,
		triggerRunner: triggerRunner,
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

	// Validate branch names before creating
	if req.Msg.BranchName != nil && *req.Msg.BranchName != "" {
		if err := git.ValidateBranchName(*req.Msg.BranchName); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid branch_name: %w", err))
		}
	}
	if req.Msg.TargetBranch != nil && *req.Msg.TargetBranch != "" {
		if err := git.ValidateBranchName(*req.Msg.TargetBranch); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target_branch: %w", err))
		}
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
	// Set workflow ID directly
	if req.Msg.WorkflowId != nil {
		t.WorkflowId = req.Msg.WorkflowId
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

	// Branch control overrides
	if req.Msg.BranchName != nil {
		t.BranchName = req.Msg.BranchName
	}
	if req.Msg.PrDraft != nil {
		t.PrDraft = req.Msg.PrDraft
	}
	// Labels: pr_labels_set=true sets labels (including empty list to override defaults)
	if req.Msg.PrLabelsSet != nil && *req.Msg.PrLabelsSet {
		t.PrLabels = req.Msg.PrLabels
		t.PrLabelsSet = true
	}
	// Reviewers: pr_reviewers_set=true sets reviewers (including empty list to override defaults)
	if req.Msg.PrReviewersSet != nil && *req.Msg.PrReviewersSet {
		t.PrReviewers = req.Msg.PrReviewers
		t.PrReviewersSet = true
	}

	// Save the task
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventTaskCreated, t.Id, t))
	}

	// Fire on_task_created lifecycle triggers if a workflow is assigned
	if s.triggerRunner != nil && t.WorkflowId != nil && *t.WorkflowId != "" {
		triggers := s.loadWorkflowTriggers(*t.WorkflowId)
		if err := s.triggerRunner.RunLifecycleTriggers(ctx, workflow.WorkflowTriggerEventOnTaskCreated, triggers, t); err != nil {
			// Gate rejection: set task to BLOCKED
			t.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
			task.UpdateTimestampProto(t)
			if saveErr := backend.SaveTask(t); saveErr != nil {
				s.logger.Error("failed to save blocked task after gate rejection",
					"task_id", t.Id, "error", saveErr)
			}
			// Reload and return the blocked task
			updated, loadErr := backend.LoadTask(t.Id)
			if loadErr != nil {
				return connect.NewResponse(&orcv1.CreateTaskResponse{Task: t}), nil
			}
			return connect.NewResponse(&orcv1.CreateTaskResponse{Task: updated}), nil
		}
	}

	return connect.NewResponse(&orcv1.CreateTaskResponse{
		Task: t,
	}), nil
}

// loadWorkflowTriggers loads and parses triggers from a workflow ID.
// Returns nil if the workflow can't be loaded or has no triggers.
func (s *taskServer) loadWorkflowTriggers(workflowID string) []workflow.WorkflowTrigger {
	if s.projectDB == nil {
		return nil
	}
	wf, err := s.projectDB.GetWorkflow(workflowID)
	if err != nil {
		s.logger.Debug("could not load workflow for triggers",
			"workflow_id", workflowID, "error", err)
		return nil
	}
	if wf.Triggers == "" {
		return nil
	}
	dbTriggers, err := db.ParseWorkflowTriggers(wf.Triggers)
	if err != nil {
		s.logger.Warn("failed to parse workflow triggers",
			"workflow_id", workflowID, "error", err)
		return nil
	}
	// Convert db.WorkflowTrigger to workflow.WorkflowTrigger
	result := make([]workflow.WorkflowTrigger, len(dbTriggers))
	for i, dt := range dbTriggers {
		result[i] = workflow.WorkflowTrigger{
			Event:   workflow.WorkflowTriggerEvent(dt.Event),
			AgentID: dt.AgentID,
			Mode:    workflow.GateMode(dt.Mode),
			Enabled: dt.Enabled,
		}
	}
	return result
}

// UpdateTask updates an existing task.
func (s *taskServer) UpdateTask(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateTaskRequest],
) (*connect.Response[orcv1.UpdateTaskResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	// Validate branch names before loading
	if req.Msg.BranchName != nil && *req.Msg.BranchName != "" {
		if err := git.ValidateBranchName(*req.Msg.BranchName); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid branch_name: %w", err))
		}
	}
	if req.Msg.TargetBranch != nil && *req.Msg.TargetBranch != "" {
		if err := git.ValidateBranchName(*req.Msg.TargetBranch); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target_branch: %w", err))
		}
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

	// Prevent changes on running tasks
	if t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		// Branch settings cannot be changed while running (branch already checked out)
		hasBranchChange := req.Msg.BranchName != nil || req.Msg.TargetBranch != nil
		if hasBranchChange {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("cannot change branch settings on a running task - pause it first"))
		}
		// Status cannot be changed via UpdateTask while running
		if req.Msg.Status != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("cannot change status of a running task via UpdateTask - use pause/resume commands"))
		}
	}

	// Apply updates - direct proto field assignments
	if req.Msg.Title != nil {
		t.Title = *req.Msg.Title
	}
	if req.Msg.Description != nil {
		t.Description = req.Msg.Description
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

	// Branch control overrides
	if req.Msg.BranchName != nil {
		t.BranchName = req.Msg.BranchName
	}
	if req.Msg.PrDraft != nil {
		t.PrDraft = req.Msg.PrDraft
	}
	// Labels: pr_labels_set=true sets labels, pr_labels_set=false clears the override
	if req.Msg.PrLabelsSet != nil {
		if *req.Msg.PrLabelsSet {
			t.PrLabels = req.Msg.PrLabels
			t.PrLabelsSet = true
		} else {
			// Clear the override - return to default behavior
			t.PrLabels = nil
			t.PrLabelsSet = false
		}
	}
	// Reviewers: pr_reviewers_set=true sets reviewers, pr_reviewers_set=false clears the override
	if req.Msg.PrReviewersSet != nil {
		if *req.Msg.PrReviewersSet {
			t.PrReviewers = req.Msg.PrReviewers
			t.PrReviewersSet = true
		} else {
			// Clear the override - return to default behavior
			t.PrReviewers = nil
			t.PrReviewersSet = false
		}
	}

	// Status change (TASK-776)
	// Running task check is handled above; this only applies to non-running tasks
	if req.Msg.Status != nil {
		t.Status = *req.Msg.Status
	}

	// Manual fix flag (TASK-776)
	// Sets quality.manual_intervention when true
	if req.Msg.ManualFix != nil && *req.Msg.ManualFix {
		if t.Quality == nil {
			t.Quality = &orcv1.QualityMetrics{}
		}
		t.Quality.ManualIntervention = true
	}

	// Update timestamp
	task.UpdateTimestampProto(t)

	// Save the task
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		publishTaskUpdatedEvent(s.publisher, req.Msg.GetProjectId(), t)
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
	}
	if t.Description != nil {
		plan.Description = *t.Description
	}

	// Get workflow phases to determine correct order
	var phaseOrder []string
	if t.WorkflowId != nil && *t.WorkflowId != "" {
		workflowPhases, err := backend.GetWorkflowPhases(*t.WorkflowId)
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

	publishTaskUpdatedEvent(s.publisher, req.Msg.GetProjectId(), t)
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

	publishTaskUpdatedEvent(s.publisher, req.Msg.GetProjectId(), t)
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

	publishTaskUpdatedEvent(s.publisher, req.Msg.GetProjectId(), t)
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

	publishTaskUpdatedEvent(s.publisher, req.Msg.GetProjectId(), t)
	return connect.NewResponse(&orcv1.RemoveRelatedResponse{}), nil
}
