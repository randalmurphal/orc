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
	backend   storage.Backend
	logger    *slog.Logger
	publisher events.Publisher
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

// ListInitiatives returns all initiatives with optional filtering.
func (s *initiativeServer) ListInitiatives(
	ctx context.Context,
	req *connect.Request[orcv1.ListInitiativesRequest],
) (*connect.Response[orcv1.ListInitiativesResponse], error) {
	initiatives, err := s.backend.LoadAllInitiatives()
	if err != nil {
		// Return empty list if no initiatives yet
		return connect.NewResponse(&orcv1.ListInitiativesResponse{
			Initiatives: []*orcv1.Initiative{},
			Page:        &orcv1.PageResponse{Total: 0},
		}), nil
	}

	if initiatives == nil {
		initiatives = []*initiative.Initiative{}
	}

	// Filter by status if requested
	if req.Msg.Status != nil && *req.Msg.Status != orcv1.InitiativeStatus_INITIATIVE_STATUS_UNSPECIFIED {
		var filtered []*initiative.Initiative
		targetStatus := protoToInitiativeStatus(*req.Msg.Status)
		for _, init := range initiatives {
			if init.Status == targetStatus {
				filtered = append(filtered, init)
			}
		}
		initiatives = filtered
	}

	// Compute blocks (reverse dependency)
	s.computeInitiativeBlocks(initiatives)

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
		initiatives = []*initiative.Initiative{}
	}

	// Convert to proto
	protoInitiatives := make([]*orcv1.Initiative, len(initiatives))
	for i, init := range initiatives {
		protoInitiatives[i] = InitiativeToProto(init)
	}

	// Calculate pagination response
	totalPages := (totalCount + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return connect.NewResponse(&orcv1.ListInitiativesResponse{
		Initiatives: protoInitiatives,
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
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	init, err := s.backend.LoadInitiative(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.Id))
	}

	// Compute blocks
	allInits, _ := s.backend.LoadAllInitiatives()
	if allInits != nil {
		init.Blocks = s.computeBlocksForInitiative(init.ID, allInits)
	}

	return connect.NewResponse(&orcv1.GetInitiativeResponse{
		Initiative: InitiativeToProto(init),
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

	// Generate a new initiative ID
	id, err := s.backend.GetNextInitiativeID()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("generate initiative ID: %w", err))
	}

	// Create the initiative
	init := &initiative.Initiative{
		Version: 1,
		ID:      id,
		Title:   req.Msg.Title,
		Status:  initiative.StatusDraft,
	}

	// Set optional fields
	if req.Msg.Vision != nil {
		init.Vision = *req.Msg.Vision
	}
	if req.Msg.Owner != nil {
		init.Owner = initiative.Identity{
			Initials: req.Msg.Owner.Initials,
		}
		if req.Msg.Owner.DisplayName != nil {
			init.Owner.DisplayName = *req.Msg.Owner.DisplayName
		}
		if req.Msg.Owner.Email != nil {
			init.Owner.Email = *req.Msg.Owner.Email
		}
	}
	if req.Msg.BranchBase != nil {
		init.BranchBase = *req.Msg.BranchBase
	}
	if req.Msg.BranchPrefix != nil {
		init.BranchPrefix = *req.Msg.BranchPrefix
	}
	if len(req.Msg.ContextFiles) > 0 {
		init.ContextFiles = req.Msg.ContextFiles
	}
	if len(req.Msg.BlockedBy) > 0 {
		init.BlockedBy = req.Msg.BlockedBy
	}

	// Save the initiative
	if err := s.backend.SaveInitiative(init); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save initiative: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventInitiativeCreated, init.ID, init))
	}

	return connect.NewResponse(&orcv1.CreateInitiativeResponse{
		Initiative: InitiativeToProto(init),
	}), nil
}

// UpdateInitiative updates an existing initiative.
func (s *initiativeServer) UpdateInitiative(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateInitiativeRequest],
) (*connect.Response[orcv1.UpdateInitiativeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	// Load existing initiative
	init, err := s.backend.LoadInitiative(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.Id))
	}

	// Apply updates
	if req.Msg.Title != nil {
		init.Title = *req.Msg.Title
	}
	if req.Msg.Vision != nil {
		init.Vision = *req.Msg.Vision
	}
	if req.Msg.Status != nil && *req.Msg.Status != orcv1.InitiativeStatus_INITIATIVE_STATUS_UNSPECIFIED {
		init.Status = protoToInitiativeStatus(*req.Msg.Status)
	}
	if req.Msg.Owner != nil {
		init.Owner = initiative.Identity{
			Initials: req.Msg.Owner.Initials,
		}
		if req.Msg.Owner.DisplayName != nil {
			init.Owner.DisplayName = *req.Msg.Owner.DisplayName
		}
		if req.Msg.Owner.Email != nil {
			init.Owner.Email = *req.Msg.Owner.Email
		}
	}
	if req.Msg.BranchBase != nil {
		init.BranchBase = *req.Msg.BranchBase
	}
	if req.Msg.BranchPrefix != nil {
		init.BranchPrefix = *req.Msg.BranchPrefix
	}
	if req.Msg.ContextFiles != nil {
		init.ContextFiles = req.Msg.ContextFiles
	}
	if req.Msg.BlockedBy != nil {
		init.BlockedBy = req.Msg.BlockedBy
	}

	// Save the initiative
	if err := s.backend.SaveInitiative(init); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save initiative: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventInitiativeUpdated, init.ID, init))
	}

	return connect.NewResponse(&orcv1.UpdateInitiativeResponse{
		Initiative: InitiativeToProto(init),
	}), nil
}

// DeleteInitiative deletes an initiative.
func (s *initiativeServer) DeleteInitiative(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteInitiativeRequest],
) (*connect.Response[orcv1.DeleteInitiativeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	// Check initiative exists
	_, err := s.backend.LoadInitiative(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.Id))
	}

	// Delete the initiative
	if err := s.backend.DeleteInitiative(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete initiative: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventInitiativeDeleted, req.Msg.Id, map[string]string{"initiative_id": req.Msg.Id}))
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

	// Check initiative exists
	_, err := s.backend.LoadInitiative(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Load all tasks and filter by initiative
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks: %w", err))
	}

	var tasks []*task.Task
	for _, t := range allTasks {
		if t.InitiativeID == req.Msg.InitiativeId {
			tasks = append(tasks, t)
		}
	}

	// Populate computed fields
	task.PopulateComputedFields(tasks)

	// Convert to proto
	protoTasks := make([]*orcv1.Task, len(tasks))
	for i, t := range tasks {
		protoTasks[i] = TaskToProto(t)
	}

	return connect.NewResponse(&orcv1.ListInitiativeTasksResponse{
		Tasks: protoTasks,
	}), nil
}

// LinkTasks links tasks to an initiative.
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

	// Check initiative exists
	_, err := s.backend.LoadInitiative(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Update each task's initiative ID
	for _, taskID := range req.Msg.TaskIds {
		t, err := s.backend.LoadTask(taskID)
		if err != nil {
			continue // Skip non-existent tasks
		}
		t.InitiativeID = req.Msg.InitiativeId
		if err := s.backend.SaveTask(t); err != nil {
			s.logger.Warn("failed to link task", "task_id", taskID, "error", err)
			continue
		}
	}

	// Reload initiative to include task updates
	init, err := s.backend.LoadInitiative(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("reload initiative: %w", err))
	}

	return connect.NewResponse(&orcv1.LinkTasksResponse{
		Initiative: InitiativeToProto(init),
	}), nil
}

// UnlinkTask unlinks a task from an initiative.
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

	// Load the task
	t, err := s.backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task %s not found", req.Msg.TaskId))
	}

	// Check task is linked to the initiative
	if t.InitiativeID != req.Msg.InitiativeId {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("task %s is not linked to initiative %s", req.Msg.TaskId, req.Msg.InitiativeId))
	}

	// Unlink
	t.InitiativeID = ""
	if err := s.backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save task: %w", err))
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

	// Load initiative
	init, err := s.backend.LoadInitiative(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Create decision
	decision := initiative.Decision{
		ID:       fmt.Sprintf("DEC-%03d", len(init.Decisions)+1),
		Decision: req.Msg.Decision,
	}
	if req.Msg.Rationale != nil {
		decision.Rationale = *req.Msg.Rationale
	}
	if req.Msg.By != nil {
		decision.By = *req.Msg.By
	}

	// Add to initiative
	init.Decisions = append(init.Decisions, decision)

	// Save
	if err := s.backend.SaveInitiative(init); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save initiative: %w", err))
	}

	// Publish event
	if s.publisher != nil {
		s.publisher.Publish(events.NewEvent(events.EventInitiativeUpdated, init.ID, init))
	}

	return connect.NewResponse(&orcv1.AddDecisionResponse{
		Initiative: InitiativeToProto(init),
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

	// Check initiative exists
	_, err := s.backend.LoadInitiative(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Load all tasks and filter by initiative
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks: %w", err))
	}

	// Build task map for dependency checking
	taskMap := make(map[string]*task.Task)
	var initiativeTasks []*task.Task
	for _, t := range allTasks {
		taskMap[t.ID] = t
		if t.InitiativeID == req.Msg.InitiativeId {
			initiativeTasks = append(initiativeTasks, t)
		}
	}

	// Find ready tasks (not completed, not running, no unmet blockers)
	var readyTasks []*task.Task
	for _, t := range initiativeTasks {
		// Skip completed or running tasks
		if t.Status == task.StatusCompleted || t.Status == task.StatusRunning {
			continue
		}

		// Check if all blockers are satisfied
		unmet := t.GetUnmetDependencies(taskMap)
		if len(unmet) == 0 {
			readyTasks = append(readyTasks, t)
		}
	}

	// Convert to proto
	protoTasks := make([]*orcv1.Task, len(readyTasks))
	for i, t := range readyTasks {
		protoTasks[i] = TaskToProto(t)
	}

	return connect.NewResponse(&orcv1.GetReadyTasksResponse{
		Tasks: protoTasks,
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

	// Check initiative exists
	_, err := s.backend.LoadInitiative(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Load all tasks and filter by initiative
	allTasks, err := s.backend.LoadAllTasks()
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
		if t.InitiativeID != req.Msg.InitiativeId {
			continue
		}

		// Add task as node
		if !addedNodes[t.ID] {
			graph.Nodes = append(graph.Nodes, &orcv1.DependencyNode{
				Id:     t.ID,
				Title:  t.Title,
				Status: taskStatusToProto(t.Status),
			})
			addedNodes[t.ID] = true
		}

		// Add edges for blockers
		for _, blockerID := range t.BlockedBy {
			graph.Edges = append(graph.Edges, &orcv1.DependencyEdge{
				From: blockerID,
				To:   t.ID,
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

	// Load initiative
	init, err := s.backend.LoadInitiative(req.Msg.InitiativeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("initiative %s not found", req.Msg.InitiativeId))
	}

	// Load all initiatives to check blocking status
	allInits, err := s.backend.LoadAllInitiatives()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load initiatives: %w", err))
	}
	initMap := make(map[string]*initiative.Initiative)
	for _, i := range allInits {
		initMap[i.ID] = i
	}

	// Check if initiative is blocked by other initiatives
	if init.IsBlocked(initMap) {
		blockers := init.GetIncompleteBlockers(initMap)
		blockerIDs := make([]string, len(blockers))
		for i, b := range blockers {
			blockerIDs[i] = b.ID
		}
		return connect.NewResponse(&orcv1.RunInitiativeResponse{
			Initiative:     InitiativeToProto(init),
			StartedTaskIds: []string{},
			Message:        fmt.Sprintf("Initiative is blocked by: %v. Complete blocking initiatives first.", blockerIDs),
		}), nil
	}

	// Load all tasks and filter by initiative
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load tasks: %w", err))
	}

	// Build task map for dependency checking
	taskMap := make(map[string]*task.Task)
	var initiativeTasks []*task.Task
	for _, t := range allTasks {
		taskMap[t.ID] = t
		if t.InitiativeID == req.Msg.InitiativeId {
			initiativeTasks = append(initiativeTasks, t)
		}
	}

	// Find ready tasks (not completed, not running, no unmet blockers)
	var readyTasks []*task.Task
	for _, t := range initiativeTasks {
		// Skip completed or running tasks
		if t.Status == task.StatusCompleted || t.Status == task.StatusRunning {
			continue
		}

		// Check if all blockers are satisfied
		unmet := t.GetUnmetDependencies(taskMap)
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
			case task.StatusCompleted:
				completedCount++
			case task.StatusRunning:
				runningCount++
			default:
				if len(t.GetUnmetDependencies(taskMap)) > 0 {
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
			Initiative:     InitiativeToProto(init),
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
		readyTaskIDs[i] = t.ID
	}

	return connect.NewResponse(&orcv1.RunInitiativeResponse{
		Initiative:     InitiativeToProto(init),
		StartedTaskIds: readyTaskIDs,
		Message:        fmt.Sprintf("%d task(s) ready to run. Call RunTask for each to start execution.", len(readyTasks)),
	}), nil
}

// Helper functions

// computeInitiativeBlocks computes the Blocks field for all initiatives.
func (s *initiativeServer) computeInitiativeBlocks(initiatives []*initiative.Initiative) {
	// Build map of initiative ID -> []IDs that it blocks
	blocksMap := make(map[string][]string)
	for _, init := range initiatives {
		for _, blockedBy := range init.BlockedBy {
			blocksMap[blockedBy] = append(blocksMap[blockedBy], init.ID)
		}
	}

	// Apply to each initiative
	for _, init := range initiatives {
		init.Blocks = blocksMap[init.ID]
	}
}

// computeBlocksForInitiative computes what initiatives this one blocks.
func (s *initiativeServer) computeBlocksForInitiative(id string, allInits []*initiative.Initiative) []string {
	var blocks []string
	for _, init := range allInits {
		for _, blockedBy := range init.BlockedBy {
			if blockedBy == id {
				blocks = append(blocks, init.ID)
				break
			}
		}
	}
	return blocks
}

