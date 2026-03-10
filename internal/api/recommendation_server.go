package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
)

type recommendationServer struct {
	orcv1connect.UnimplementedRecommendationServiceHandler
	backend      storage.Backend
	projectCache *ProjectCache
	logger       *slog.Logger
	publisher    events.Publisher
}

func NewRecommendationServer(
	backend storage.Backend,
	logger *slog.Logger,
	publisher events.Publisher,
) orcv1connect.RecommendationServiceHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &recommendationServer{
		backend:   backend,
		logger:    logger,
		publisher: publisher,
	}
}

func (s *recommendationServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

func (s *recommendationServer) getBackend(projectID string) (storage.Backend, error) {
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

func (s *recommendationServer) CreateRecommendation(
	ctx context.Context,
	req *connect.Request[orcv1.CreateRecommendationRequest],
) (*connect.Response[orcv1.CreateRecommendationResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	if req.Msg.Recommendation == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("recommendation is required"))
	}

	rec := req.Msg.Recommendation
	if rec.Status == orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED {
		rec.Status = orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING
	}

	if err := backend.SaveRecommendation(rec); err != nil {
		if isRecommendationDedupeError(err) {
			return nil, connect.NewError(connect.CodeAlreadyExists, err)
		}
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	s.publishRecommendationCreated(rec)
	return connect.NewResponse(&orcv1.CreateRecommendationResponse{Recommendation: rec}), nil
}

func (s *recommendationServer) GetRecommendation(
	ctx context.Context,
	req *connect.Request[orcv1.GetRecommendationRequest],
) (*connect.Response[orcv1.GetRecommendationResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	if req.Msg.RecommendationId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("recommendation_id is required"))
	}

	rec, err := backend.LoadRecommendation(req.Msg.RecommendationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if rec == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("recommendation %s not found", req.Msg.RecommendationId))
	}

	return connect.NewResponse(&orcv1.GetRecommendationResponse{Recommendation: rec}), nil
}

func (s *recommendationServer) ListRecommendations(
	ctx context.Context,
	req *connect.Request[orcv1.ListRecommendationsRequest],
) (*connect.Response[orcv1.ListRecommendationsResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	recommendations, err := backend.LoadAllRecommendations()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	filtered := make([]*orcv1.Recommendation, 0, len(recommendations))
	for _, rec := range recommendations {
		if req.Msg.Status != orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED && rec.Status != req.Msg.Status {
			continue
		}
		if req.Msg.Kind != orcv1.RecommendationKind_RECOMMENDATION_KIND_UNSPECIFIED && rec.Kind != req.Msg.Kind {
			continue
		}
		if req.Msg.SourceTaskId != "" && rec.SourceTaskId != req.Msg.SourceTaskId {
			continue
		}
		filtered = append(filtered, rec)
	}

	return connect.NewResponse(&orcv1.ListRecommendationsResponse{Recommendations: filtered}), nil
}

func (s *recommendationServer) AcceptRecommendation(
	ctx context.Context,
	req *connect.Request[orcv1.AcceptRecommendationRequest],
) (*connect.Response[orcv1.AcceptRecommendationResponse], error) {
	rec, previousStatus, err := s.updateRecommendationDecision(
		req.Msg.GetProjectId(),
		req.Msg.RecommendationId,
		orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED,
		req.Msg.DecidedBy,
		req.Msg.DecisionReason,
	)
	if err != nil {
		return nil, err
	}
	s.publishRecommendationDecided(rec, previousStatus)
	return connect.NewResponse(&orcv1.AcceptRecommendationResponse{Recommendation: rec}), nil
}

func (s *recommendationServer) RejectRecommendation(
	ctx context.Context,
	req *connect.Request[orcv1.RejectRecommendationRequest],
) (*connect.Response[orcv1.RejectRecommendationResponse], error) {
	rec, previousStatus, err := s.updateRecommendationDecision(
		req.Msg.GetProjectId(),
		req.Msg.RecommendationId,
		orcv1.RecommendationStatus_RECOMMENDATION_STATUS_REJECTED,
		req.Msg.DecidedBy,
		req.Msg.DecisionReason,
	)
	if err != nil {
		return nil, err
	}
	s.publishRecommendationDecided(rec, previousStatus)
	return connect.NewResponse(&orcv1.RejectRecommendationResponse{Recommendation: rec}), nil
}

func (s *recommendationServer) DiscussRecommendation(
	ctx context.Context,
	req *connect.Request[orcv1.DiscussRecommendationRequest],
) (*connect.Response[orcv1.DiscussRecommendationResponse], error) {
	rec, previousStatus, err := s.updateRecommendationDecision(
		req.Msg.GetProjectId(),
		req.Msg.RecommendationId,
		orcv1.RecommendationStatus_RECOMMENDATION_STATUS_DISCUSSED,
		req.Msg.DecidedBy,
		req.Msg.DecisionReason,
	)
	if err != nil {
		return nil, err
	}
	s.publishRecommendationDecided(rec, previousStatus)
	return connect.NewResponse(&orcv1.DiscussRecommendationResponse{
		Recommendation: rec,
		ContextPack:    buildRecommendationContextPack(rec),
	}), nil
}

func (s *recommendationServer) updateRecommendationDecision(
	projectID string,
	recommendationID string,
	status orcv1.RecommendationStatus,
	decidedBy string,
	decisionReason string,
) (*orcv1.Recommendation, orcv1.RecommendationStatus, error) {
	backend, err := s.getBackend(projectID)
	if err != nil {
		return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}
	if recommendationID == "" {
		return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeInvalidArgument, errors.New("recommendation_id is required"))
	}
	if decidedBy == "" {
		return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeInvalidArgument, errors.New("decided_by is required"))
	}

	current, err := backend.LoadRecommendation(recommendationID)
	if err != nil {
		return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeInternal, err)
	}
	if current == nil {
		return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeNotFound, fmt.Errorf("recommendation %s not found", recommendationID))
	}

	updated, err := backend.UpdateRecommendationStatus(recommendationID, status, decidedBy, decisionReason)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrRecommendationConflict):
			return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeFailedPrecondition, err)
		case errors.Is(err, db.ErrInvalidRecommendationTransition):
			return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeFailedPrecondition, err)
		default:
			return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeInternal, err)
		}
	}

	return updated, current.Status, nil
}

func (s *recommendationServer) publishRecommendationCreated(rec *orcv1.Recommendation) {
	if s.publisher == nil || rec == nil {
		return
	}
	s.publisher.Publish(events.NewEvent(events.EventRecommendationCreated, rec.SourceTaskId, events.RecommendationCreatedData{
		RecommendationID: rec.Id,
		Kind:             recommendationKindProtoToString(rec.Kind),
		Status:           recommendationStatusProtoToString(rec.Status),
		Title:            rec.Title,
		Summary:          rec.Summary,
		SourceTaskID:     rec.SourceTaskId,
		SourceRunID:      rec.SourceRunId,
	}))
}

func (s *recommendationServer) publishRecommendationDecided(rec *orcv1.Recommendation, previousStatus orcv1.RecommendationStatus) {
	if s.publisher == nil || rec == nil {
		return
	}
	s.publisher.Publish(events.NewEvent(events.EventRecommendationDecided, rec.SourceTaskId, events.RecommendationDecidedData{
		RecommendationID: rec.Id,
		PreviousStatus:   recommendationStatusProtoToString(previousStatus),
		Status:           recommendationStatusProtoToString(rec.Status),
		DecidedBy:        rec.GetDecidedBy(),
		DecisionReason:   rec.GetDecisionReason(),
		SourceTaskID:     rec.SourceTaskId,
	}))
}

func buildRecommendationContextPack(rec *orcv1.Recommendation) string {
	if rec == nil {
		return ""
	}
	return fmt.Sprintf(
		"Recommendation %s\nKind: %s\nTitle: %s\nSummary: %s\nProposed action: %s\nEvidence: %s\nSource task: %s\nSource run: %s",
		rec.Id,
		strings.ToLower(strings.TrimPrefix(rec.Kind.String(), "RECOMMENDATION_KIND_")),
		rec.Title,
		rec.Summary,
		rec.ProposedAction,
		rec.Evidence,
		rec.SourceTaskId,
		rec.SourceRunId,
	)
}

func isRecommendationDedupeError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate")
}

func recommendationStatusProtoToString(status orcv1.RecommendationStatus) string {
	switch status {
	case orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING:
		return db.RecommendationStatusPending
	case orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED:
		return db.RecommendationStatusAccepted
	case orcv1.RecommendationStatus_RECOMMENDATION_STATUS_REJECTED:
		return db.RecommendationStatusRejected
	case orcv1.RecommendationStatus_RECOMMENDATION_STATUS_DISCUSSED:
		return db.RecommendationStatusDiscussed
	default:
		return ""
	}
}

func recommendationKindProtoToString(kind orcv1.RecommendationKind) string {
	switch kind {
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP:
		return db.RecommendationKindCleanup
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK:
		return db.RecommendationKindRisk
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP:
		return db.RecommendationKindFollowUp
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_DECISION_REQUEST:
		return db.RecommendationKindDecisionRequest
	default:
		return ""
	}
}
