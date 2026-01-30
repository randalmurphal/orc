// Package api provides the Connect RPC and REST API server for orc.
// This file implements the DecisionService Connect RPC service.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// decisionServer implements the DecisionServiceHandler interface.
type decisionServer struct {
	orcv1connect.UnimplementedDecisionServiceHandler
	backend          storage.Backend
	projectCache     *ProjectCache
	pendingDecisions *gate.PendingDecisionStore
	publisher        events.Publisher
	logger           *slog.Logger
}

// SetProjectCache sets the project cache for multi-project support.
func (s *decisionServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
func (s *decisionServer) getBackend(projectID string) (storage.Backend, error) {
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

// NewDecisionServer creates a new DecisionService handler.
func NewDecisionServer(
	backend storage.Backend,
	pendingDecisions *gate.PendingDecisionStore,
	publisher events.Publisher,
	logger *slog.Logger,
) orcv1connect.DecisionServiceHandler {
	return &decisionServer{
		backend:          backend,
		pendingDecisions: pendingDecisions,
		publisher:        publisher,
		logger:           logger,
	}
}

// ListPendingDecisions returns all pending decisions, optionally filtered by task ID.
func (s *decisionServer) ListPendingDecisions(
	ctx context.Context,
	req *connect.Request[orcv1.ListPendingDecisionsRequest],
) (*connect.Response[orcv1.ListPendingDecisionsResponse], error) {
	decisions := s.pendingDecisions.List()

	// Filter by task ID if specified
	taskFilter := ""
	if req.Msg.TaskId != nil {
		taskFilter = *req.Msg.TaskId
	}

	var protoDecisions []*orcv1.PendingDecision
	for _, d := range decisions {
		if taskFilter != "" && d.TaskID != taskFilter {
			continue
		}
		protoDecisions = append(protoDecisions, pendingDecisionToProto(d))
	}

	return connect.NewResponse(&orcv1.ListPendingDecisionsResponse{
		Decisions: protoDecisions,
	}), nil
}

// GetPendingDecision retrieves a specific pending decision by ID.
func (s *decisionServer) GetPendingDecision(
	ctx context.Context,
	req *connect.Request[orcv1.GetPendingDecisionRequest],
) (*connect.Response[orcv1.GetPendingDecisionResponse], error) {
	decision, ok := s.pendingDecisions.Get(req.Msg.Id)
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("decision not found: %s", req.Msg.Id))
	}

	return connect.NewResponse(&orcv1.GetPendingDecisionResponse{
		Decision: pendingDecisionToProto(decision),
	}), nil
}

// ResolveDecision resolves a pending decision (approve or reject).
func (s *decisionServer) ResolveDecision(
	ctx context.Context,
	req *connect.Request[orcv1.ResolveDecisionRequest],
) (*connect.Response[orcv1.ResolveDecisionResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	decisionID := req.Msg.Id

	// Get pending decision
	decision, ok := s.pendingDecisions.Get(decisionID)
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("decision not found: %s", decisionID))
	}

	// Load task
	t, err := backend.LoadTask(decision.TaskID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", decision.TaskID))
	}

	// Verify task is blocked
	if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("task is not blocked (status: %s)", t.Status.String()))
	}

	// Verify phase matches current task phase to prevent stale decisions
	currentPhase := task.GetCurrentPhaseProto(t)
	if currentPhase != decision.Phase {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("decision phase mismatch: task is at phase %q, decision is for phase %q",
				currentPhase, decision.Phase))
	}

	// Extract optional fields
	reason := ""
	if req.Msg.Reason != nil {
		reason = *req.Msg.Reason
	}
	resolvedBy := "api"
	if req.Msg.ResolvedBy != nil && *req.Msg.ResolvedBy != "" {
		resolvedBy = *req.Msg.ResolvedBy
	}
	selectedOption := ""
	if req.Msg.SelectedOption != nil {
		selectedOption = *req.Msg.SelectedOption
	}

	now := time.Now()

	// Record gate decision in task execution state
	task.EnsureExecutionProto(t)
	gateDecision := &orcv1.GateDecision{
		Phase:     decision.Phase,
		GateType:  decision.GateType,
		Approved:  req.Msg.Approved,
		Timestamp: timestamppb.New(now),
	}
	if reason != "" {
		gateDecision.Reason = &reason
	}
	t.Execution.Gates = append(t.Execution.Gates, gateDecision)

	// Save task
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save task: %w", err))
	}

	// Record in database if available
	if dbBackend, ok := backend.(*storage.DatabaseBackend); ok {
		dbDecision := &db.GateDecision{
			TaskID:    decision.TaskID,
			Phase:     decision.Phase,
			GateType:  decision.GateType,
			Approved:  req.Msg.Approved,
			Reason:    reason,
			DecidedBy: resolvedBy,
			DecidedAt: now,
		}
		if err := dbBackend.DB().AddGateDecision(dbDecision); err != nil {
			s.logger.Warn("failed to record gate decision in database", "error", err)
			// Don't fail the request - database recording is optional
		}
	}

	// Update task status based on approval
	var newStatus orcv1.TaskStatus
	if req.Msg.Approved {
		newStatus = orcv1.TaskStatus_TASK_STATUS_PLANNED
	} else {
		newStatus = orcv1.TaskStatus_TASK_STATUS_FAILED
	}

	t.Status = newStatus
	if err := backend.SaveTask(t); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save task: %w", err))
	}

	// Emit decision_resolved event
	resolvedData := events.DecisionResolvedData{
		DecisionID: decisionID,
		TaskID:     decision.TaskID,
		Phase:      decision.Phase,
		Approved:   req.Msg.Approved,
		Reason:     reason,
		ResolvedBy: resolvedBy,
		ResolvedAt: now,
	}

	s.publisher.Publish(events.Event{
		Type:   events.EventDecisionResolved,
		TaskID: decision.TaskID,
		Data:   resolvedData,
		Time:   now,
	})

	// Remove decision from pending store
	s.pendingDecisions.Remove(decisionID)

	// Build response
	resolved := &orcv1.ResolvedDecision{
		Id:         decisionID,
		TaskId:     decision.TaskID,
		Phase:      decision.Phase,
		Approved:   req.Msg.Approved,
		ResolvedBy: resolvedBy,
		ResolvedAt: timestamppb.New(now),
	}
	if selectedOption != "" {
		resolved.SelectedOption = &selectedOption
	}
	if reason != "" {
		resolved.Reason = &reason
	}

	return connect.NewResponse(&orcv1.ResolveDecisionResponse{
		Decision: resolved,
	}), nil
}

// ListResolvedDecisions returns historical resolved decisions from the database.
func (s *decisionServer) ListResolvedDecisions(
	ctx context.Context,
	req *connect.Request[orcv1.ListResolvedDecisionsRequest],
) (*connect.Response[orcv1.ListResolvedDecisionsResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	dbBackend, ok := backend.(*storage.DatabaseBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("resolved decisions require database backend"))
	}

	// Get all gate decisions grouped by task
	allDecisions, err := dbBackend.DB().GetAllGateDecisionsGrouped()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load decisions: %w", err))
	}

	// Filter by task ID if specified
	taskFilter := ""
	if req.Msg.TaskId != nil {
		taskFilter = *req.Msg.TaskId
	}

	// Flatten and convert to proto
	var protoDecisions []*orcv1.ResolvedDecision
	for taskID, decisions := range allDecisions {
		if taskFilter != "" && taskID != taskFilter {
			continue
		}
		for _, d := range decisions {
			protoDecisions = append(protoDecisions, dbGateDecisionToProto(&d))
		}
	}

	// Apply pagination (1-indexed page)
	pageNum := int32(1)
	pageLimit := int32(50)
	if req.Msg.Page != nil {
		if req.Msg.Page.Page > 0 {
			pageNum = req.Msg.Page.Page
		}
		if req.Msg.Page.Limit > 0 {
			pageLimit = req.Msg.Page.Limit
		}
	}

	totalCount := int32(len(protoDecisions))
	totalPages := (totalCount + pageLimit - 1) / pageLimit
	if totalPages == 0 {
		totalPages = 1
	}

	// Apply offset and limit (convert 1-indexed page to 0-indexed offset)
	start := int((pageNum - 1) * pageLimit)
	if start > len(protoDecisions) {
		start = len(protoDecisions)
	}
	end := start + int(pageLimit)
	if end > len(protoDecisions) {
		end = len(protoDecisions)
	}

	paginatedDecisions := protoDecisions[start:end]

	return connect.NewResponse(&orcv1.ListResolvedDecisionsResponse{
		Decisions: paginatedDecisions,
		Page: &orcv1.PageResponse{
			Page:       pageNum,
			Limit:      pageLimit,
			Total:      totalCount,
			TotalPages: totalPages,
			HasMore:    pageNum < totalPages,
		},
	}), nil
}

// pendingDecisionToProto converts a gate.PendingDecision to proto.
func pendingDecisionToProto(d *gate.PendingDecision) *orcv1.PendingDecision {
	return &orcv1.PendingDecision{
		Id:          d.DecisionID,
		TaskId:      d.TaskID,
		TaskTitle:   d.TaskTitle,
		Phase:       d.Phase,
		GateType:    d.GateType,
		Question:    d.Question,
		Context:     d.Context,
		Options:     nil, // Options not stored in gate.PendingDecision
		RequestedAt: timestamppb.New(d.RequestedAt),
	}
}

// dbGateDecisionToProto converts a db.GateDecision to proto ResolvedDecision.
func dbGateDecisionToProto(d *db.GateDecision) *orcv1.ResolvedDecision {
	resolved := &orcv1.ResolvedDecision{
		Id:         fmt.Sprintf("decision-%d", d.ID),
		TaskId:     d.TaskID,
		Phase:      d.Phase,
		Approved:   d.Approved,
		ResolvedBy: d.DecidedBy,
		ResolvedAt: timestamppb.New(d.DecidedAt),
	}
	if d.Reason != "" {
		resolved.Reason = &d.Reason
	}
	return resolved
}
