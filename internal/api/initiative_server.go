// Package api provides the Connect RPC and REST API server for orc.
// This file implements the InitiativeService Connect RPC service.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// initiativeServer implements the InitiativeServiceHandler interface.
type initiativeServer struct {
	orcv1connect.UnimplementedInitiativeServiceHandler
	backend      storage.Backend
	projectCache *ProjectCache
	logger       *slog.Logger
	publisher    events.Publisher
}

// getBackend returns the appropriate backend for a project ID.
// Errors if projectID is provided but cache is not configured (prevents silent data leaks).
func (s *initiativeServer) getBackend(projectID string) (storage.Backend, error) {
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

// SetProjectCache sets the project cache for multi-project support.
func (s *initiativeServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// NewInitiativeServer creates a new InitiativeService handler.
func NewInitiativeServer(
	backend storage.Backend,
	logger *slog.Logger,
	publisher events.Publisher,
) orcv1connect.InitiativeServiceHandler {
	return &initiativeServer{
		backend:   backend,
		logger:    logger,
		publisher: publisher,
	}
}

// NewInitiativeServerWithCache creates an InitiativeService handler with project cache support.
func NewInitiativeServerWithCache(
	backend storage.Backend,
	logger *slog.Logger,
	publisher events.Publisher,
	cache *ProjectCache,
) orcv1connect.InitiativeServiceHandler {
	return &initiativeServer{
		backend:      backend,
		projectCache: cache,
		logger:       logger,
		publisher:    publisher,
	}
}

// ListInitiatives returns all initiatives with optional filtering.
func (s *initiativeServer) ListInitiatives(
	ctx context.Context,
	req *connect.Request[orcv1.ListInitiativesRequest],
) (*connect.Response[orcv1.ListInitiativesResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	initiatives, err := backend.LoadAllInitiativesProto()
	if err != nil {
		// Return empty list if no initiatives yet
		return connect.NewResponse(&orcv1.ListInitiativesResponse{
			Initiatives: []*orcv1.Initiative{},
			Page:        &orcv1.PageResponse{Total: 0},
		}), nil
	}

	if initiatives == nil {
		initiatives = []*orcv1.Initiative{}
	}

	// Filter by status if requested
	if req.Msg.Status != nil && *req.Msg.Status != orcv1.InitiativeStatus_INITIATIVE_STATUS_UNSPECIFIED {
		var filtered []*orcv1.Initiative
		for _, init := range initiatives {
			if init.Status == *req.Msg.Status {
				filtered = append(filtered, init)
			}
		}
		initiatives = filtered
	}

	// Compute blocks (reverse dependency)
	initiative.PopulateComputedFieldsProto(initiatives)

	totalCount := int32(len(initiatives))

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
		initiatives = initiatives[offset:endIdx]
	} else {
		initiatives = []*orcv1.Initiative{}
	}

	// Calculate pagination response
	totalPages := (totalCount + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return connect.NewResponse(&orcv1.ListInitiativesResponse{
		Initiatives: initiatives,
		Page: &orcv1.PageResponse{
			Page:       page,
			Limit:      limit,
			Total:      totalCount,
			TotalPages: totalPages,
			HasMore:    page < totalPages,
		},
	}), nil
}

// GetInitiative returns a single initiative by ID.
func (s *initiativeServer) GetInitiative(
	ctx context.Context,
	req *connect.Request[orcv1.GetInitiativeRequest],
) (*connect.Response[orcv1.GetInitiativeResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("initiative_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	init, err := backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Compute blocks
	allInits, _ := backend.LoadAllInitiativesProto()
	if allInits != nil {
		init.Blocks = initiative.ComputeBlocksProto(init.Id, allInits)
	}

	return connect.NewResponse(&orcv1.GetInitiativeResponse{
		Initiative: init,
	}), nil
}

// CreateInitiative creates a new initiative.
func (s *initiativeServer) CreateInitiative(
	ctx context.Context,
	req *connect.Request[orcv1.CreateInitiativeRequest],
) (*connect.Response[orcv1.CreateInitiativeResponse], error) {
	if req.Msg.Title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("title is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Generate a new initiative ID
	id, err := backend.GetNextInitiativeID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("generate initiative ID: %w", err))
	}

	// Create the initiative using proto types
	init := initiative.NewProtoInitiative(id, req.Msg.Title)

	// Set optional fields
	if req.Msg.Vision != nil {
		init.Vision = req.Msg.Vision
	}
	if req.Msg.Owner != nil {
		init.Owner = req.Msg.Owner
	}
	if req.Msg.BranchBase != nil {
		init.BranchBase = req.Msg.BranchBase
	}
	if req.Msg.BranchPrefix != nil {
		init.BranchPrefix = req.Msg.BranchPrefix
	}
	if len(req.Msg.ContextFiles) > 0 {
		init.ContextFiles = req.Msg.ContextFiles
	}
	if len(req.Msg.BlockedBy) > 0 {
		init.BlockedBy = req.Msg.BlockedBy
	}

	// Save the initiative
	if err := backend.SaveInitiativeProto(init); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save initiative: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventInitiativeCreated, init.Id, init))
	}

	return connect.NewResponse(&orcv1.CreateInitiativeResponse{
		Initiative: init,
	}), nil
}

// UpdateInitiative updates an existing initiative.
func (s *initiativeServer) UpdateInitiative(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateInitiativeRequest],
) (*connect.Response[orcv1.UpdateInitiativeResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("initiative_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Load existing initiative
	init, err := backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Apply updates
	if req.Msg.Title != nil {
		init.Title = *req.Msg.Title
	}
	if req.Msg.Vision != nil {
		init.Vision = req.Msg.Vision
	}
	if req.Msg.Status != nil && *req.Msg.Status != orcv1.InitiativeStatus_INITIATIVE_STATUS_UNSPECIFIED {
		init.Status = *req.Msg.Status
	}
	if req.Msg.Owner != nil {
		init.Owner = req.Msg.Owner
	}
	if req.Msg.BranchBase != nil {
		init.BranchBase = req.Msg.BranchBase
	}
	if req.Msg.BranchPrefix != nil {
		init.BranchPrefix = req.Msg.BranchPrefix
	}
	if req.Msg.ContextFiles != nil {
		init.ContextFiles = req.Msg.ContextFiles
	}
	if req.Msg.BlockedBy != nil {
		init.BlockedBy = req.Msg.BlockedBy
	}

	// Update timestamp
	initiative.UpdateTimestampProto(init)

	// Save the initiative
	if err := backend.SaveInitiativeProto(init); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save initiative: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventInitiativeUpdated, init.Id, init))
	}

	return connect.NewResponse(&orcv1.UpdateInitiativeResponse{
		Initiative: init,
	}), nil
}

// DeleteInitiative deletes an initiative.
func (s *initiativeServer) DeleteInitiative(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteInitiativeRequest],
) (*connect.Response[orcv1.DeleteInitiativeResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("initiative_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Check initiative exists
	_, err = backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Delete the initiative
	if err := backend.DeleteInitiative(req.Msg.InitiativeId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete initiative: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventInitiativeDeleted, req.Msg.InitiativeId, map[string]string{"initiative_id": req.Msg.InitiativeId}))
	}

	return connect.NewResponse(&orcv1.DeleteInitiativeResponse{}), nil
}

// ListInitiativeTasks returns all tasks associated with an initiative.
func (s *initiativeServer) ListInitiativeTasks(
	ctx context.Context,
	req *connect.Request[orcv1.ListInitiativeTasksRequest],
) (*connect.Response[orcv1.ListInitiativeTasksResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Check initiative exists
	_, err = backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Load all tasks and filter by initiative
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks: %w", err))
	}

	var tasks []*orcv1.Task
	for _, t := range allTasks {
		if task.GetInitiativeIDProto(t) == req.Msg.InitiativeId {
			tasks = append(tasks, t)
		}
	}

	// Populate computed fields
	task.PopulateComputedFieldsProto(tasks)

	return connect.NewResponse(&orcv1.ListInitiativeTasksResponse{
		Tasks: tasks,
	}), nil
}

// LinkTasks links tasks to an initiative.
// Updates BOTH task.initiative_id AND initiative_tasks junction table.
func (s *initiativeServer) LinkTasks(
	ctx context.Context,
	req *connect.Request[orcv1.LinkTasksRequest],
) (*connect.Response[orcv1.LinkTasksResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if len(req.Msg.TaskIds) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_ids is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Check initiative exists
	_, err = backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Get current max sequence for the initiative's tasks
	existingTaskIDs, err := backend.DB().GetInitiativeTasks(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get existing tasks: %w", err))
	}
	sequence := len(existingTaskIDs)

	// Update each task's initiative ID and add to junction table
	for _, taskID := range req.Msg.TaskIds {
		t, err := backend.LoadTask(taskID)
		if err != nil {
			continue // Skip non-existent tasks
		}

		// If task was linked to a different initiative, remove from that junction table
		if t.InitiativeId != nil && *t.InitiativeId != req.Msg.InitiativeId {
			if err := backend.DB().RemoveTaskFromInitiative(*t.InitiativeId, taskID); err != nil {
				if s.logger != nil {
					s.logger.Warn("failed to remove task from old initiative", "task_id", taskID, "old_initiative", *t.InitiativeId, "error", err)
				}
			}
		}

		// Update task.initiative_id
		t.InitiativeId = &req.Msg.InitiativeId
		task.UpdateTimestampProto(t)
		if err := backend.SaveTask(t); err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to link task", "task_id", taskID, "error", err)
			}
			continue
		}

		// Add to junction table (uses ON CONFLICT to handle duplicates)
		if err := backend.DB().AddTaskToInitiative(req.Msg.InitiativeId, taskID, sequence); err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to add task to junction table", "task_id", taskID, "error", err)
			}
			continue
		}
		sequence++
	}

	// Reload initiative to include task updates
	init, err := backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("reload initiative: %w", err))
	}

	return connect.NewResponse(&orcv1.LinkTasksResponse{
		Initiative: init,
	}), nil
}

// UnlinkTask unlinks a task from an initiative.
// Clears BOTH task.initiative_id AND removes from initiative_tasks junction table.
func (s *initiativeServer) UnlinkTask(
	ctx context.Context,
	req *connect.Request[orcv1.UnlinkTaskRequest],
) (*connect.Response[orcv1.UnlinkTaskResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Load the task
	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check task is linked to the initiative
	if task.GetInitiativeIDProto(t) != req.Msg.InitiativeId {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task %s is not linked to initiative %s", req.Msg.TaskId, req.Msg.InitiativeId))
	}

	// Clear task.initiative_id
	t.InitiativeId = nil
	task.UpdateTimestampProto(t)
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
	}

	// Remove from junction table
	if err := backend.DB().RemoveTaskFromInitiative(req.Msg.InitiativeId, req.Msg.TaskId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("remove from junction table: %w", err))
	}

	return connect.NewResponse(&orcv1.UnlinkTaskResponse{}), nil
}

// AddDecision adds a decision to an initiative.
func (s *initiativeServer) AddDecision(
	ctx context.Context,
	req *connect.Request[orcv1.AddDecisionRequest],
) (*connect.Response[orcv1.AddDecisionResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if req.Msg.Decision == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("decision is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Load initiative
	init, err := backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Add decision using proto helper
	rationale := ""
	by := ""
	if req.Msg.Rationale != nil {
		rationale = *req.Msg.Rationale
	}
	if req.Msg.By != nil {
		by = *req.Msg.By
	}
	initiative.AddDecisionProto(init, req.Msg.Decision, rationale, by)

	// Save
	if err := backend.SaveInitiativeProto(init); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save initiative: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventInitiativeUpdated, init.Id, init))
	}

	return connect.NewResponse(&orcv1.AddDecisionResponse{
		Initiative: init,
	}), nil
}

// GetReadyTasks returns tasks in an initiative that are ready to run.
func (s *initiativeServer) GetReadyTasks(
	ctx context.Context,
	req *connect.Request[orcv1.GetReadyTasksRequest],
) (*connect.Response[orcv1.GetReadyTasksResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Check initiative exists
	_, err = backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Load all tasks and filter by initiative
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks: %w", err))
	}

	// Build task map for dependency checking
	taskMap := make(map[string]*orcv1.Task)
	var initiativeTasks []*orcv1.Task
	for _, t := range allTasks {
		taskMap[t.Id] = t
		if task.GetInitiativeIDProto(t) == req.Msg.InitiativeId {
			initiativeTasks = append(initiativeTasks, t)
		}
	}

	// Find ready tasks (not completed, not running, no unmet blockers)
	var readyTasks []*orcv1.Task
	for _, t := range initiativeTasks {
		// Skip completed or running tasks
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED || t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
			continue
		}

		// Check if all blockers are satisfied
		unmet := task.GetUnmetDependenciesProto(t, taskMap)
		if len(unmet) == 0 {
			readyTasks = append(readyTasks, t)
		}
	}

	return connect.NewResponse(&orcv1.GetReadyTasksResponse{
		Tasks: readyTasks,
	}), nil
}

// GetDependencyGraph returns the dependency graph for an initiative.
func (s *initiativeServer) GetDependencyGraph(
	ctx context.Context,
	req *connect.Request[orcv1.GetDependencyGraphRequest],
) (*connect.Response[orcv1.GetDependencyGraphResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Check initiative exists
	_, err = backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Load all tasks and filter by initiative
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks: %w", err))
	}

	// Build dependency graph
	graph := &orcv1.DependencyGraph{
		Nodes: make([]*orcv1.DependencyNode, 0),
		Edges: make([]*orcv1.DependencyEdge, 0),
	}

	// Track added nodes to avoid duplicates
	addedNodes := make(map[string]bool)

	for _, t := range allTasks {
		if task.GetInitiativeIDProto(t) != req.Msg.InitiativeId {
			continue
		}

		// Add task as node
		if !addedNodes[t.Id] {
			graph.Nodes = append(graph.Nodes, &orcv1.DependencyNode{
				Id:     t.Id,
				Title:  t.Title,
				Status: t.Status,
			})
			addedNodes[t.Id] = true
		}

		// Add edges for blockers
		for _, blockerID := range t.BlockedBy {
			graph.Edges = append(graph.Edges, &orcv1.DependencyEdge{
				From: blockerID,
				To:   t.Id,
				Type: "blocks",
			})
		}
	}

	return connect.NewResponse(&orcv1.GetDependencyGraphResponse{
		Graph: graph,
	}), nil
}

// RunInitiative identifies and returns tasks ready to run in an initiative.
// Note: Actual task execution should be triggered via RunTask RPC for each task.
// This RPC validates the initiative and returns which tasks are ready.
func (s *initiativeServer) RunInitiative(
	ctx context.Context,
	req *connect.Request[orcv1.RunInitiativeRequest],
) (*connect.Response[orcv1.RunInitiativeResponse], error) {
	if req.Msg.InitiativeId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("initiative_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Load initiative
	init, err := backend.LoadInitiativeProto(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Load all initiatives to check blocking status
	allInits, err := backend.LoadAllInitiativesProto()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load initiatives: %w", err))
	}
	initMap := make(map[string]*orcv1.Initiative)
	for _, i := range allInits {
		initMap[i.Id] = i
	}

	// Check if initiative is blocked by other initiatives
	if initiative.IsBlockedProto(init, initMap) {
		blockers := initiative.GetIncompleteBlockersProto(init, initMap)
		blockerIDs := make([]string, len(blockers))
		for i, b := range blockers {
			blockerIDs[i] = b.ID
		}
		return connect.NewResponse(&orcv1.RunInitiativeResponse{
			Initiative:     init,
			StartedTaskIds: []string{},
			Message:        fmt.Sprintf("Initiative is blocked by: %v. Complete blocking initiatives first.", blockerIDs),
		}), nil
	}

	// Load all tasks and filter by initiative
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks: %w", err))
	}

	// Build task map for dependency checking
	taskMap := make(map[string]*orcv1.Task)
	var initiativeTasks []*orcv1.Task
	for _, t := range allTasks {
		taskMap[t.Id] = t
		if task.GetInitiativeIDProto(t) == req.Msg.InitiativeId {
			initiativeTasks = append(initiativeTasks, t)
		}
	}

	// Find ready tasks (not completed, not running, no unmet blockers)
	var readyTasks []*orcv1.Task
	for _, t := range initiativeTasks {
		// Skip completed or running tasks
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED || t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
			continue
		}

		// Check if all blockers are satisfied
		unmet := task.GetUnmetDependenciesProto(t, taskMap)
		if len(unmet) == 0 {
			readyTasks = append(readyTasks, t)
		}
	}

	if len(readyTasks) == 0 {
		// Provide context about why no tasks are ready
		message := "No tasks ready to run."
		completedCount := 0
		runningCount := 0
		blockedCount := 0
		for _, t := range initiativeTasks {
			switch t.Status {
			case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
				completedCount++
			case orcv1.TaskStatus_TASK_STATUS_RUNNING:
				runningCount++
			default:
				if len(task.GetUnmetDependenciesProto(t, taskMap)) > 0 {
					blockedCount++
				}
			}
		}
		if completedCount == len(initiativeTasks) {
			message = "All tasks are completed."
		} else if runningCount > 0 {
			message = fmt.Sprintf("No tasks ready. %d running, %d completed, %d blocked.", runningCount, completedCount, blockedCount)
		} else if blockedCount > 0 {
			message = fmt.Sprintf("No tasks ready. %d blocked by dependencies, %d completed.", blockedCount, completedCount)
		}

		return connect.NewResponse(&orcv1.RunInitiativeResponse{
			Initiative:     init,
			StartedTaskIds: []string{},
			Message:        message,
		}), nil
	}

	// Apply max_parallel limit if specified
	maxParallel := int(req.Msg.MaxParallel)
	if maxParallel > 0 && maxParallel < len(readyTasks) {
		readyTasks = readyTasks[:maxParallel]
	}

	// Return the ready task IDs - client should call RunTask for each
	readyTaskIDs := make([]string, len(readyTasks))
	for i, t := range readyTasks {
		readyTaskIDs[i] = t.Id
	}

	return connect.NewResponse(&orcv1.RunInitiativeResponse{
		Initiative:     init,
		StartedTaskIds: readyTaskIDs,
		Message:        fmt.Sprintf("%d task(s) ready to run. Call RunTask for each to start execution.", len(readyTasks)),
	}), nil
}


