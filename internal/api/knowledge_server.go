// Package api provides the Connect RPC and REST API server for orc.
// This file implements the KnowledgeService Connect RPC service.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// Default staleness threshold in days.
const defaultStalenessDays = 30

// knowledgeServer implements the KnowledgeServiceHandler interface.
type knowledgeServer struct {
	orcv1connect.UnimplementedKnowledgeServiceHandler
	backend storage.Backend
	logger  *slog.Logger
}

// NewKnowledgeServer creates a new KnowledgeService handler.
func NewKnowledgeServer(
	backend storage.Backend,
	logger *slog.Logger,
) orcv1connect.KnowledgeServiceHandler {
	return &knowledgeServer{
		backend: backend,
		logger:  logger,
	}
}

// ListKnowledge returns knowledge entries with optional filtering.
func (s *knowledgeServer) ListKnowledge(
	ctx context.Context,
	req *connect.Request[orcv1.ListKnowledgeRequest],
) (*connect.Response[orcv1.ListKnowledgeResponse], error) {
	pdb := s.backend.DB()

	var entries []*db.KnowledgeEntry
	var err error

	// Determine which query to use based on filters
	filterType := req.Msg.Type
	filterStatus := req.Msg.Status

	hasTypeFilter := filterType != nil && *filterType != orcv1.KnowledgeType_KNOWLEDGE_TYPE_UNSPECIFIED
	hasStatusFilter := filterStatus != nil && *filterStatus != orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_UNSPECIFIED

	if hasTypeFilter && hasStatusFilter {
		// Both filters - use ListKnowledgeByType
		dbType := protoToDBType(*filterType)
		dbStatus := protoToDBStatus(*filterStatus)
		entries, err = pdb.ListKnowledgeByType(dbType, dbStatus)
	} else if hasStatusFilter {
		// Status filter only
		dbStatus := protoToDBStatus(*filterStatus)
		if dbStatus == db.KnowledgePending {
			entries, err = pdb.ListPendingKnowledge()
		} else if *filterStatus == orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_STALE {
			entries, err = pdb.ListStaleKnowledge(defaultStalenessDays)
		} else {
			// Need to query all and filter
			entries, err = listAllKnowledge(pdb)
			if err == nil {
				entries = filterByStatus(entries, dbStatus)
			}
		}
	} else if hasTypeFilter {
		// Type filter only - query all statuses for this type
		// Query approved entries of this type (most common use case)
		entries, err = listAllKnowledge(pdb)
		if err == nil {
			dbType := protoToDBType(*filterType)
			entries = filterByType(entries, dbType)
		}
	} else {
		// No filters - return all
		entries, err = listAllKnowledge(pdb)
	}

	if err != nil {
		return nil, fmt.Errorf("list knowledge: %w", err)
	}

	// Convert to proto
	protoEntries := make([]*orcv1.KnowledgeEntry, len(entries))
	for i, e := range entries {
		protoEntries[i] = dbEntryToProto(e)
	}

	totalCount := int32(len(protoEntries))

	// Apply pagination
	page := int32(1)
	limit := int32(50)
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

	offset := (page - 1) * limit
	endIdx := offset + limit
	if endIdx > totalCount {
		endIdx = totalCount
	}
	if offset < totalCount {
		protoEntries = protoEntries[offset:endIdx]
	} else {
		protoEntries = []*orcv1.KnowledgeEntry{}
	}

	totalPages := (totalCount + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	return connect.NewResponse(&orcv1.ListKnowledgeResponse{
		Entries: protoEntries,
		Page: &orcv1.PageResponse{
			Page:       page,
			Limit:      limit,
			Total:      totalCount,
			TotalPages: totalPages,
			HasMore:    page < totalPages,
		},
	}), nil
}

// GetKnowledgeStatus returns the status summary (counts by status).
func (s *knowledgeServer) GetKnowledgeStatus(
	ctx context.Context,
	req *connect.Request[orcv1.GetKnowledgeStatusRequest],
) (*connect.Response[orcv1.GetKnowledgeStatusResponse], error) {
	pdb := s.backend.DB()

	// Get all entries to count
	entries, err := listAllKnowledge(pdb)
	if err != nil {
		return nil, fmt.Errorf("get knowledge status: %w", err)
	}

	var pending, approved, rejected, stale int32
	for _, e := range entries {
		switch e.Status {
		case db.KnowledgePending:
			pending++
		case db.KnowledgeApproved:
			approved++
			if e.IsStale(defaultStalenessDays) {
				stale++
			}
		case db.KnowledgeRejected:
			rejected++
		}
	}

	return connect.NewResponse(&orcv1.GetKnowledgeStatusResponse{
		Status: &orcv1.KnowledgeStatusSummary{
			PendingCount:  pending,
			ApprovedCount: approved,
			StaleCount:    stale,
			RejectedCount: rejected,
		},
	}), nil
}

// GetStaleEntries returns approved entries that haven't been validated recently.
func (s *knowledgeServer) GetStaleEntries(
	ctx context.Context,
	req *connect.Request[orcv1.GetStaleEntriesRequest],
) (*connect.Response[orcv1.GetStaleEntriesResponse], error) {
	pdb := s.backend.DB()

	days := int(req.Msg.Days)
	if days <= 0 {
		days = defaultStalenessDays
	}

	entries, err := pdb.ListStaleKnowledge(days)
	if err != nil {
		return nil, fmt.Errorf("get stale entries: %w", err)
	}

	protoEntries := make([]*orcv1.KnowledgeEntry, len(entries))
	for i, e := range entries {
		protoEntries[i] = dbEntryToProto(e)
	}

	return connect.NewResponse(&orcv1.GetStaleEntriesResponse{
		Entries: protoEntries,
	}), nil
}

// GetKnowledge returns a single knowledge entry by ID.
func (s *knowledgeServer) GetKnowledge(
	ctx context.Context,
	req *connect.Request[orcv1.GetKnowledgeRequest],
) (*connect.Response[orcv1.GetKnowledgeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	pdb := s.backend.DB()

	entry, err := pdb.GetKnowledgeEntry(req.Msg.Id)
	if err != nil {
		return nil, fmt.Errorf("get knowledge: %w", err)
	}
	if entry == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("knowledge %s not found", req.Msg.Id))
	}

	return connect.NewResponse(&orcv1.GetKnowledgeResponse{
		Entry: dbEntryToProto(entry),
	}), nil
}

// CreateKnowledge creates a new knowledge entry.
func (s *knowledgeServer) CreateKnowledge(
	ctx context.Context,
	req *connect.Request[orcv1.CreateKnowledgeRequest],
) (*connect.Response[orcv1.CreateKnowledgeResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	if req.Msg.Description == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("description is required"))
	}
	if req.Msg.Type == orcv1.KnowledgeType_KNOWLEDGE_TYPE_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("type is required"))
	}

	pdb := s.backend.DB()

	dbType := protoToDBType(req.Msg.Type)
	sourceTask := ""
	if req.Msg.SourceTask != nil {
		sourceTask = *req.Msg.SourceTask
	}
	proposedBy := req.Msg.ProposedBy
	if proposedBy == "" {
		proposedBy = "user"
	}

	entry, err := pdb.QueueKnowledge(dbType, req.Msg.Name, req.Msg.Description, sourceTask, proposedBy)
	if err != nil {
		return nil, fmt.Errorf("create knowledge: %w", err)
	}

	s.logger.Info("created knowledge entry", "id", entry.ID, "type", entry.Type, "name", entry.Name)

	return connect.NewResponse(&orcv1.CreateKnowledgeResponse{
		Entry: dbEntryToProto(entry),
	}), nil
}

// ApproveKnowledge approves a pending knowledge entry.
func (s *knowledgeServer) ApproveKnowledge(
	ctx context.Context,
	req *connect.Request[orcv1.ApproveKnowledgeRequest],
) (*connect.Response[orcv1.ApproveKnowledgeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	pdb := s.backend.DB()

	reviewedBy := "user"
	if req.Msg.ReviewedBy != nil && *req.Msg.ReviewedBy != "" {
		reviewedBy = *req.Msg.ReviewedBy
	}

	entry, err := pdb.ApproveKnowledge(req.Msg.Id, reviewedBy)
	if err != nil {
		// Check if it's a "not found or already processed" error
		if entry == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("knowledge %s not found or already processed", req.Msg.Id))
		}
		return nil, fmt.Errorf("approve knowledge: %w", err)
	}

	s.logger.Info("approved knowledge entry", "id", entry.ID, "by", reviewedBy)

	return connect.NewResponse(&orcv1.ApproveKnowledgeResponse{
		Entry: dbEntryToProto(entry),
	}), nil
}

// ApproveAllKnowledge approves all pending knowledge entries.
func (s *knowledgeServer) ApproveAllKnowledge(
	ctx context.Context,
	req *connect.Request[orcv1.ApproveAllKnowledgeRequest],
) (*connect.Response[orcv1.ApproveAllKnowledgeResponse], error) {
	pdb := s.backend.DB()

	reviewedBy := "user"
	if req.Msg.ReviewedBy != nil && *req.Msg.ReviewedBy != "" {
		reviewedBy = *req.Msg.ReviewedBy
	}

	count, err := pdb.ApproveAllPending(reviewedBy)
	if err != nil {
		return nil, fmt.Errorf("approve all knowledge: %w", err)
	}

	s.logger.Info("approved all pending knowledge entries", "count", count, "by", reviewedBy)

	return connect.NewResponse(&orcv1.ApproveAllKnowledgeResponse{
		ApprovedCount: int32(count),
	}), nil
}

// RejectKnowledge rejects a pending knowledge entry.
func (s *knowledgeServer) RejectKnowledge(
	ctx context.Context,
	req *connect.Request[orcv1.RejectKnowledgeRequest],
) (*connect.Response[orcv1.RejectKnowledgeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	pdb := s.backend.DB()

	reason := ""
	if req.Msg.Reason != nil {
		reason = *req.Msg.Reason
	}

	err := pdb.RejectKnowledge(req.Msg.Id, reason)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("knowledge %s not found or already processed", req.Msg.Id))
	}

	// Reload to get updated entry
	entry, err := pdb.GetKnowledgeEntry(req.Msg.Id)
	if err != nil {
		return nil, fmt.Errorf("get rejected entry: %w", err)
	}

	s.logger.Info("rejected knowledge entry", "id", req.Msg.Id, "reason", reason)

	return connect.NewResponse(&orcv1.RejectKnowledgeResponse{
		Entry: dbEntryToProto(entry),
	}), nil
}

// ValidateKnowledge marks an approved entry as still relevant (resets staleness).
func (s *knowledgeServer) ValidateKnowledge(
	ctx context.Context,
	req *connect.Request[orcv1.ValidateKnowledgeRequest],
) (*connect.Response[orcv1.ValidateKnowledgeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	pdb := s.backend.DB()

	entry, err := pdb.ValidateKnowledge(req.Msg.Id, "user")
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("knowledge %s not found or not approved", req.Msg.Id))
	}

	s.logger.Info("validated knowledge entry", "id", entry.ID)

	return connect.NewResponse(&orcv1.ValidateKnowledgeResponse{
		Entry: dbEntryToProto(entry),
	}), nil
}

// DeleteKnowledge removes a knowledge entry.
func (s *knowledgeServer) DeleteKnowledge(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteKnowledgeRequest],
) (*connect.Response[orcv1.DeleteKnowledgeResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	pdb := s.backend.DB()

	// Check entry exists first
	entry, err := pdb.GetKnowledgeEntry(req.Msg.Id)
	if err != nil {
		return nil, fmt.Errorf("get knowledge: %w", err)
	}
	if entry == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("knowledge %s not found", req.Msg.Id))
	}

	if err := pdb.DeleteKnowledge(req.Msg.Id); err != nil {
		return nil, fmt.Errorf("delete knowledge: %w", err)
	}

	s.logger.Info("deleted knowledge entry", "id", req.Msg.Id)

	return connect.NewResponse(&orcv1.DeleteKnowledgeResponse{
		Message: fmt.Sprintf("knowledge %s deleted", req.Msg.Id),
	}), nil
}

// =============================================================================
// CONVERSION HELPERS
// =============================================================================

// dbEntryToProto converts a db.KnowledgeEntry to orcv1.KnowledgeEntry.
func dbEntryToProto(e *db.KnowledgeEntry) *orcv1.KnowledgeEntry {
	if e == nil {
		return nil
	}

	proto := &orcv1.KnowledgeEntry{
		Id:          e.ID,
		Type:        dbTypeToProto(e.Type),
		Name:        e.Name,
		Description: e.Description,
		ProposedBy:  e.ProposedBy,
		Status:      dbStatusToProto(e, defaultStalenessDays),
		CreatedAt:   timestamppb.New(e.ProposedAt),
		UpdatedAt:   timestamppb.New(e.ProposedAt), // Use proposed_at if no better option
	}

	// Optional fields
	if e.SourceTask != "" {
		proto.SourceTask = &e.SourceTask
	}

	// Set reviewed_by based on status
	if e.ApprovedBy != "" {
		proto.ReviewedBy = &e.ApprovedBy
	}

	// Set review_reason (rejection reason)
	if e.RejectedReason != "" {
		proto.ReviewReason = &e.RejectedReason
	}

	// Set validated_at if present
	if e.ValidatedAt != nil {
		proto.ValidatedAt = timestamppb.New(*e.ValidatedAt)
	}

	// Update updated_at to latest timestamp
	if e.ApprovedAt != nil {
		proto.UpdatedAt = timestamppb.New(*e.ApprovedAt)
	}
	if e.ValidatedAt != nil && (e.ApprovedAt == nil || e.ValidatedAt.After(*e.ApprovedAt)) {
		proto.UpdatedAt = timestamppb.New(*e.ValidatedAt)
	}

	return proto
}

// protoToDBType converts proto KnowledgeType to db KnowledgeType.
func protoToDBType(t orcv1.KnowledgeType) db.KnowledgeType {
	switch t {
	case orcv1.KnowledgeType_KNOWLEDGE_TYPE_PATTERN:
		return db.KnowledgePattern
	case orcv1.KnowledgeType_KNOWLEDGE_TYPE_GOTCHA:
		return db.KnowledgeGotcha
	case orcv1.KnowledgeType_KNOWLEDGE_TYPE_DECISION:
		return db.KnowledgeDecision
	default:
		return db.KnowledgePattern // Default to pattern
	}
}

// dbTypeToProto converts db KnowledgeType to proto KnowledgeType.
func dbTypeToProto(t db.KnowledgeType) orcv1.KnowledgeType {
	switch t {
	case db.KnowledgePattern:
		return orcv1.KnowledgeType_KNOWLEDGE_TYPE_PATTERN
	case db.KnowledgeGotcha:
		return orcv1.KnowledgeType_KNOWLEDGE_TYPE_GOTCHA
	case db.KnowledgeDecision:
		return orcv1.KnowledgeType_KNOWLEDGE_TYPE_DECISION
	default:
		return orcv1.KnowledgeType_KNOWLEDGE_TYPE_UNSPECIFIED
	}
}

// protoToDBStatus converts proto KnowledgeStatus to db KnowledgeStatus.
func protoToDBStatus(s orcv1.KnowledgeStatus) db.KnowledgeStatus {
	switch s {
	case orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_PENDING:
		return db.KnowledgePending
	case orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_APPROVED:
		return db.KnowledgeApproved
	case orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_REJECTED:
		return db.KnowledgeRejected
	case orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_STALE:
		return db.KnowledgeApproved // Stale is a virtual status for approved entries
	default:
		return db.KnowledgePending
	}
}

// dbStatusToProto converts db KnowledgeStatus to proto KnowledgeStatus.
// It also checks for staleness on approved entries.
func dbStatusToProto(e *db.KnowledgeEntry, stalenessDays int) orcv1.KnowledgeStatus {
	switch e.Status {
	case db.KnowledgePending:
		return orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_PENDING
	case db.KnowledgeApproved:
		if e.IsStale(stalenessDays) {
			return orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_STALE
		}
		return orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_APPROVED
	case db.KnowledgeRejected:
		return orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_REJECTED
	default:
		return orcv1.KnowledgeStatus_KNOWLEDGE_STATUS_UNSPECIFIED
	}
}

// listAllKnowledge returns all knowledge entries by querying each status.
func listAllKnowledge(pdb *db.ProjectDB) ([]*db.KnowledgeEntry, error) {
	var all []*db.KnowledgeEntry

	// Get pending
	pending, err := pdb.ListPendingKnowledge()
	if err != nil {
		return nil, err
	}
	all = append(all, pending...)

	// Get approved (query by type with approved status, then dedupe)
	// Since there's no ListApproved, we query by type
	for _, ktype := range []db.KnowledgeType{db.KnowledgePattern, db.KnowledgeGotcha, db.KnowledgeDecision} {
		approved, err := pdb.ListKnowledgeByType(ktype, db.KnowledgeApproved)
		if err != nil {
			return nil, err
		}
		all = append(all, approved...)

		rejected, err := pdb.ListKnowledgeByType(ktype, db.KnowledgeRejected)
		if err != nil {
			return nil, err
		}
		all = append(all, rejected...)
	}

	return all, nil
}

// filterByStatus filters entries to a specific status.
func filterByStatus(entries []*db.KnowledgeEntry, status db.KnowledgeStatus) []*db.KnowledgeEntry {
	var filtered []*db.KnowledgeEntry
	for _, e := range entries {
		if e.Status == status {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// filterByType filters entries to a specific type.
func filterByType(entries []*db.KnowledgeEntry, ktype db.KnowledgeType) []*db.KnowledgeEntry {
	var filtered []*db.KnowledgeEntry
	for _, e := range entries {
		if e.Type == ktype {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
