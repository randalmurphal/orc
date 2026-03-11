// Package api provides the Connect RPC and REST API server for orc.
// This file implements the DecisionService Connect RPC service.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
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
	if req.Msg.GetProjectId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("project_id is required"))
	}
	decisions := s.pendingDecisions.List(req.Msg.GetProjectId())

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
	if req.Msg.GetProjectId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("project_id is required"))
	}
	decision, ok := s.pendingDecisions.Get(req.Msg.GetProjectId(), req.Msg.Id)
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
	if req.Msg.GetProjectId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("project_id is required"))
	}
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
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

	resolved, err := resolvePendingDecision(
		backend,
		s.pendingDecisions,
		s.publisher,
		req.Msg.GetProjectId(),
		req.Msg.Id,
		req.Msg.Approved,
		reason,
		resolvedBy,
		selectedOption,
	)
	if err != nil {
		switch {
		case strings.HasPrefix(err.Error(), "decision not found"):
			return nil, connect.NewError(connect.CodeNotFound, err)
		case strings.HasPrefix(err.Error(), "task not found"):
			return nil, connect.NewError(connect.CodeNotFound, err)
		case strings.HasPrefix(err.Error(), "decision option not found"):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		case strings.HasPrefix(err.Error(), "task is not blocked"),
			strings.HasPrefix(err.Error(), "decision phase mismatch"):
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		default:
			return nil, connect.NewError(connect.CodeInternal, err)
		}
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
	options := make([]*orcv1.DecisionOption, 0, len(d.Options))
	for _, option := range d.Options {
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

	return &orcv1.PendingDecision{
		Id:          d.DecisionID,
		TaskId:      d.TaskID,
		TaskTitle:   d.TaskTitle,
		Phase:       d.Phase,
		GateType:    d.GateType,
		Question:    d.Question,
		Context:     d.Context,
		Options:     options,
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
