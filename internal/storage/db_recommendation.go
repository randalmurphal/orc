package storage

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

func (d *DatabaseBackend) SaveRecommendation(r *orcv1.Recommendation) error {
	if r == nil {
		return fmt.Errorf("recommendation is required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	rec, err := protoRecommendationToDB(r)
	if err != nil {
		return err
	}
	if err := d.db.CreateRecommendation(rec); err != nil {
		return err
	}

	updated, err := dbRecommendationToProto(rec)
	if err != nil {
		return err
	}
	copyRecommendation(r, updated)
	return nil
}

func (d *DatabaseBackend) LoadRecommendation(id string) (*orcv1.Recommendation, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rec, err := d.db.GetRecommendation(id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	return dbRecommendationToProto(rec)
}

func (d *DatabaseBackend) LoadAllRecommendations() ([]*orcv1.Recommendation, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	items, err := d.db.ListRecommendations(db.RecommendationListOpts{})
	if err != nil {
		return nil, err
	}

	recommendations := make([]*orcv1.Recommendation, 0, len(items))
	for i := range items {
		rec, err := dbRecommendationToProto(&items[i])
		if err != nil {
			return nil, err
		}
		recommendations = append(recommendations, rec)
	}
	return recommendations, nil
}

func (d *DatabaseBackend) UpdateRecommendationStatus(
	id string,
	status orcv1.RecommendationStatus,
	decidedBy string,
	decisionReason string,
) (*orcv1.Recommendation, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbStatus, err := protoRecommendationStatusToDB(status)
	if err != nil {
		return nil, err
	}

	rec, err := d.db.UpdateRecommendationStatus(id, dbStatus, decidedBy, decisionReason)
	if err != nil {
		return nil, err
	}
	return dbRecommendationToProto(rec)
}

func (d *DatabaseBackend) CountRecommendationsByStatus(status orcv1.RecommendationStatus) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbStatus, err := protoRecommendationStatusToDB(status)
	if err != nil {
		return 0, err
	}
	return d.db.CountRecommendationsByStatus(dbStatus)
}

func (d *DatabaseBackend) GetNextRecommendationID() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.GetNextRecommendationID(context.Background())
}

func dbRecommendationToProto(rec *db.Recommendation) (*orcv1.Recommendation, error) {
	if rec == nil {
		return nil, fmt.Errorf("recommendation is required")
	}

	kind, err := dbRecommendationKindToProto(rec.Kind)
	if err != nil {
		return nil, err
	}
	status, err := dbRecommendationStatusToProto(rec.Status)
	if err != nil {
		return nil, err
	}

	protoRec := &orcv1.Recommendation{
		Id:             rec.ID,
		Kind:           kind,
		Status:         status,
		Title:          rec.Title,
		Summary:        rec.Summary,
		ProposedAction: rec.ProposedAction,
		Evidence:       rec.Evidence,
		SourceTaskId:   rec.SourceTaskID,
		SourceRunId:    rec.SourceRunID,
		SourceThreadId: rec.SourceThreadID,
		DedupeKey:      rec.DedupeKey,
		PromotedToType: rec.PromotedToType,
		PromotedToId:   rec.PromotedToID,
		PromotedBy:     rec.PromotedBy,
		CreatedAt:      timestamppb.New(rec.CreatedAt),
		UpdatedAt:      timestamppb.New(rec.UpdatedAt),
	}
	if rec.DecidedBy != "" {
		protoRec.DecidedBy = &rec.DecidedBy
	}
	if rec.DecidedAt != nil {
		protoRec.DecidedAt = timestamppb.New(*rec.DecidedAt)
	}
	if rec.DecisionReason != "" {
		protoRec.DecisionReason = &rec.DecisionReason
	}
	if rec.PromotedAt != nil {
		protoRec.PromotedAt = timestamppb.New(*rec.PromotedAt)
	}
	return protoRec, nil
}

func copyRecommendation(dst *orcv1.Recommendation, src *orcv1.Recommendation) {
	dst.Id = src.Id
	dst.Kind = src.Kind
	dst.Status = src.Status
	dst.Title = src.Title
	dst.Summary = src.Summary
	dst.ProposedAction = src.ProposedAction
	dst.Evidence = src.Evidence
	dst.SourceTaskId = src.SourceTaskId
	dst.SourceRunId = src.SourceRunId
	dst.SourceThreadId = src.SourceThreadId
	dst.DedupeKey = src.DedupeKey
	dst.DecidedBy = src.DecidedBy
	dst.DecidedAt = src.DecidedAt
	dst.DecisionReason = src.DecisionReason
	dst.PromotedToType = src.PromotedToType
	dst.PromotedToId = src.PromotedToId
	dst.PromotedBy = src.PromotedBy
	dst.PromotedAt = src.PromotedAt
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func protoRecommendationToDB(rec *orcv1.Recommendation) (*db.Recommendation, error) {
	if rec == nil {
		return nil, fmt.Errorf("recommendation is required")
	}

	kind, err := protoRecommendationKindToDB(rec.Kind)
	if err != nil {
		return nil, err
	}
	status, err := protoRecommendationStatusToDB(rec.Status)
	if err != nil {
		return nil, err
	}

	dbRec := &db.Recommendation{
		ID:             rec.Id,
		Kind:           kind,
		Status:         status,
		Title:          rec.Title,
		Summary:        rec.Summary,
		ProposedAction: rec.ProposedAction,
		Evidence:       rec.Evidence,
		SourceTaskID:   rec.SourceTaskId,
		SourceRunID:    rec.SourceRunId,
		SourceThreadID: rec.SourceThreadId,
		DedupeKey:      rec.DedupeKey,
		DecidedBy:      rec.GetDecidedBy(),
		DecisionReason: rec.GetDecisionReason(),
		PromotedToType: rec.PromotedToType,
		PromotedToID:   rec.PromotedToId,
		PromotedBy:     rec.PromotedBy,
	}
	if rec.DecidedAt != nil {
		t := rec.DecidedAt.AsTime()
		dbRec.DecidedAt = &t
	}
	if rec.PromotedAt != nil {
		t := rec.PromotedAt.AsTime()
		dbRec.PromotedAt = &t
	}
	return dbRec, nil
}

func dbRecommendationKindToProto(kind string) (orcv1.RecommendationKind, error) {
	switch kind {
	case db.RecommendationKindCleanup:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP, nil
	case db.RecommendationKindRisk:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK, nil
	case db.RecommendationKindFollowUp:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP, nil
	case db.RecommendationKindDecisionRequest:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_DECISION_REQUEST, nil
	default:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_UNSPECIFIED, fmt.Errorf("invalid recommendation kind %q", kind)
	}
}

func protoRecommendationKindToDB(kind orcv1.RecommendationKind) (string, error) {
	switch kind {
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP:
		return db.RecommendationKindCleanup, nil
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK:
		return db.RecommendationKindRisk, nil
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP:
		return db.RecommendationKindFollowUp, nil
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_DECISION_REQUEST:
		return db.RecommendationKindDecisionRequest, nil
	default:
		return "", fmt.Errorf("invalid recommendation kind %v", kind)
	}
}

func dbRecommendationStatusToProto(status string) (orcv1.RecommendationStatus, error) {
	switch status {
	case db.RecommendationStatusPending:
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING, nil
	case db.RecommendationStatusAccepted:
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED, nil
	case db.RecommendationStatusRejected:
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_REJECTED, nil
	case db.RecommendationStatusDiscussed:
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_DISCUSSED, nil
	default:
		return orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, fmt.Errorf("invalid recommendation status %q", status)
	}
}

func protoRecommendationStatusToDB(status orcv1.RecommendationStatus) (string, error) {
	switch status {
	case orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING:
		return db.RecommendationStatusPending, nil
	case orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED:
		return db.RecommendationStatusAccepted, nil
	case orcv1.RecommendationStatus_RECOMMENDATION_STATUS_REJECTED:
		return db.RecommendationStatusRejected, nil
	case orcv1.RecommendationStatus_RECOMMENDATION_STATUS_DISCUSSED:
		return db.RecommendationStatusDiscussed, nil
	default:
		return "", fmt.Errorf("invalid recommendation status %v", status)
	}
}
