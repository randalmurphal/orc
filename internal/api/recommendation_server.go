package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	taskproto "github.com/randalmurphal/orc/internal/task"
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

	s.publishRecommendationCreated(req.Msg.GetProjectId(), rec)
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

func (s *recommendationServer) ListRecommendationHistory(
	ctx context.Context,
	req *connect.Request[orcv1.ListRecommendationHistoryRequest],
) (*connect.Response[orcv1.ListRecommendationHistoryResponse], error) {
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

	history, err := backend.LoadRecommendationHistory(req.Msg.RecommendationId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&orcv1.ListRecommendationHistoryResponse{History: history}), nil
}

func (s *recommendationServer) AcceptRecommendation(
	ctx context.Context,
	req *connect.Request[orcv1.AcceptRecommendationRequest],
) (*connect.Response[orcv1.AcceptRecommendationResponse], error) {
	rec, previousStatus, err := s.acceptRecommendationWithPromotion(
		req.Msg.GetProjectId(),
		req.Msg.RecommendationId,
		req.Msg.DecidedBy,
		req.Msg.DecisionReason,
	)
	if err != nil {
		return nil, err
	}
	if previousStatus != rec.Status {
		s.publishRecommendationDecided(req.Msg.GetProjectId(), rec, previousStatus)
		s.publishPromotedArtifact(req.Msg.GetProjectId(), rec)
	}
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
	if previousStatus != rec.Status {
		s.publishRecommendationDecided(req.Msg.GetProjectId(), rec, previousStatus)
	}
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
	if previousStatus != rec.Status {
		s.publishRecommendationDecided(req.Msg.GetProjectId(), rec, previousStatus)
	}
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

func (s *recommendationServer) acceptRecommendationWithPromotion(
	projectID string,
	recommendationID string,
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

	sourceTask, sourceThread, err := s.loadPromotionContext(backend, current)
	if err != nil {
		return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	var updated *orcv1.Recommendation
	switch current.Kind {
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_DECISION_REQUEST:
		decision, buildErr := s.buildPromotedDecision(backend.DB(), current, sourceTask, sourceThread, decidedBy, decisionReason)
		if buildErr != nil {
			return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeFailedPrecondition, buildErr)
		}
		_, promoteErr := backend.DB().AcceptRecommendationWithDecision(
			recommendationID,
			decidedBy,
			decisionReason,
			decision,
		)
		if promoteErr != nil {
			return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connectRecommendationPromotionError(promoteErr)
		}
	default:
		taskItem, buildErr := s.buildPromotedTask(backend, current, sourceTask, decidedBy, decisionReason)
		if buildErr != nil {
			return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeFailedPrecondition, buildErr)
		}
		_, promoteErr := backend.DB().AcceptRecommendationWithTask(
			recommendationID,
			decidedBy,
			decisionReason,
			taskItem,
		)
		if promoteErr != nil {
			return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connectRecommendationPromotionError(promoteErr)
		}
	}

	updated, err = backend.LoadRecommendation(recommendationID)
	if err != nil {
		return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeInternal, fmt.Errorf("reload promoted recommendation %s: %w", recommendationID, err))
	}
	if updated == nil {
		return nil, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_UNSPECIFIED, connect.NewError(connect.CodeInternal, fmt.Errorf("promoted recommendation %s disappeared after update", recommendationID))
	}

	return updated, current.Status, nil
}

func (s *recommendationServer) loadPromotionContext(
	backend storage.Backend,
	rec *orcv1.Recommendation,
) (*orcv1.Task, *db.Thread, error) {
	sourceTask, err := backend.LoadTask(rec.SourceTaskId)
	if err != nil {
		return nil, nil, fmt.Errorf("load source task %s: %w", rec.SourceTaskId, err)
	}
	if sourceTask == nil {
		return nil, nil, fmt.Errorf("source task %s not found", rec.SourceTaskId)
	}

	var sourceThread *db.Thread
	if rec.SourceThreadId != "" {
		sourceThread, err = backend.DB().GetThread(rec.SourceThreadId)
		if err != nil {
			return nil, nil, fmt.Errorf("load source thread %s: %w", rec.SourceThreadId, err)
		}
		if sourceThread == nil {
			return nil, nil, fmt.Errorf("source thread %s not found", rec.SourceThreadId)
		}
	}

	return sourceTask, sourceThread, nil
}

func (s *recommendationServer) buildPromotedTask(
	backend storage.Backend,
	rec *orcv1.Recommendation,
	sourceTask *orcv1.Task,
	decidedBy string,
	decisionReason string,
) (*db.Task, error) {
	taskID, err := backend.GetNextTaskID()
	if err != nil {
		return nil, fmt.Errorf("generate promoted task ID: %w", err)
	}

	category := promotedTaskCategory(rec, sourceTask)
	workflowID, err := resolvePromotedTaskWorkflow(backend.DB(), sourceTask, category)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	metadataBytes, err := json.Marshal(map[string]string{
		"source_recommendation_id": rec.Id,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal promoted task metadata: %w", err)
	}

	return &db.Task{
		ID:           taskID,
		Title:        rec.Title,
		Description:  buildPromotedTaskDescription(rec, decisionReason),
		WorkflowID:   workflowID,
		Status:       taskproto.StatusFromProto(orcv1.TaskStatus_TASK_STATUS_CREATED),
		StateStatus:  "pending",
		Branch:       "orc/" + taskID,
		Queue:        taskproto.QueueFromProto(orcv1.TaskQueue_TASK_QUEUE_BACKLOG),
		Priority:     promotedTaskPriority(rec, sourceTask),
		Category:     category,
		InitiativeID: taskproto.GetInitiativeIDProto(sourceTask),
		TargetBranch: taskproto.GetTargetBranchProto(sourceTask),
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata:     string(metadataBytes),
		CreatedBy:    decidedBy,
	}, nil
}

func (s *recommendationServer) buildPromotedDecision(
	pdb *db.ProjectDB,
	rec *orcv1.Recommendation,
	sourceTask *orcv1.Task,
	sourceThread *db.Thread,
	decidedBy string,
	decisionReason string,
) (*db.InitiativeDecision, error) {
	initiativeID := ""
	if sourceThread != nil && sourceThread.InitiativeID != "" {
		initiativeID = sourceThread.InitiativeID
	}
	if initiativeID == "" {
		initiativeID = taskproto.GetInitiativeIDProto(sourceTask)
	}
	if initiativeID == "" {
		return nil, fmt.Errorf("recommendation %s has no linked initiative for decision promotion", rec.Id)
	}

	initRecord, err := pdb.GetInitiative(initiativeID)
	if err != nil {
		return nil, fmt.Errorf("load initiative %s: %w", initiativeID, err)
	}
	if initRecord == nil {
		return nil, fmt.Errorf("initiative %s not found", initiativeID)
	}

	decisionText := strings.TrimSpace(rec.ProposedAction)
	if decisionText == "" {
		decisionText = rec.Title
	}

	return &db.InitiativeDecision{
		ID:           "DEC-" + rec.Id,
		InitiativeID: initiativeID,
		Decision:     decisionText,
		Rationale:    buildPromotedDecisionRationale(rec, decisionReason),
		DecidedBy:    decidedBy,
		DecidedAt:    time.Now(),
	}, nil
}

func (s *recommendationServer) publishPromotedArtifact(projectID string, rec *orcv1.Recommendation) {
	if s.publisher == nil || rec == nil {
		return
	}

	if rec.PromotedToType == db.RecommendationPromotionTypeTask && rec.PromotedToId != "" {
		taskItem, err := s.mustLoadPromotedTask(projectID, rec.PromotedToId)
		if err != nil {
			s.logger.Warn("load promoted task for publication", "project_id", projectID, "task_id", rec.PromotedToId, "error", err)
			return
		}
		if taskItem == nil {
			s.logger.Warn("promoted task missing during publication", "project_id", projectID, "task_id", rec.PromotedToId)
			return
		}
		s.publisher.Publish(events.NewProjectEvent(events.EventTaskCreated, projectID, taskItem.Id, taskItem))
	}
}

func (s *recommendationServer) mustLoadPromotedTask(projectID string, taskID string) (*orcv1.Task, error) {
	backend, err := s.getBackend(projectID)
	if err != nil {
		return nil, err
	}
	return backend.LoadTask(taskID)
}

func (s *recommendationServer) publishRecommendationCreated(projectID string, rec *orcv1.Recommendation) {
	if s.publisher == nil || rec == nil {
		return
	}
	s.publisher.Publish(events.NewProjectEvent(events.EventRecommendationCreated, projectID, rec.SourceTaskId, events.RecommendationCreatedData{
		RecommendationID: rec.Id,
		Kind:             recommendationKindProtoToString(rec.Kind),
		Status:           recommendationStatusProtoToString(rec.Status),
		Title:            rec.Title,
		Summary:          rec.Summary,
		SourceTaskID:     rec.SourceTaskId,
		SourceRunID:      rec.SourceRunId,
		SourceThreadID:   rec.SourceThreadId,
		PromotedToType:   rec.PromotedToType,
		PromotedToID:     rec.PromotedToId,
		PromotedBy:       rec.PromotedBy,
		PromotedAt:       recommendationTimestampString(rec.PromotedAt),
	}))
}

func (s *recommendationServer) publishRecommendationDecided(projectID string, rec *orcv1.Recommendation, previousStatus orcv1.RecommendationStatus) {
	if s.publisher == nil || rec == nil {
		return
	}
	s.publisher.Publish(events.NewProjectEvent(events.EventRecommendationDecided, projectID, rec.SourceTaskId, events.RecommendationDecidedData{
		RecommendationID: rec.Id,
		PreviousStatus:   recommendationStatusProtoToString(previousStatus),
		Status:           recommendationStatusProtoToString(rec.Status),
		DecidedBy:        rec.GetDecidedBy(),
		DecisionReason:   rec.GetDecisionReason(),
		SourceTaskID:     rec.SourceTaskId,
		SourceThreadID:   rec.SourceThreadId,
		PromotedToType:   rec.PromotedToType,
		PromotedToID:     rec.PromotedToId,
		PromotedBy:       rec.PromotedBy,
		PromotedAt:       recommendationTimestampString(rec.PromotedAt),
	}))
}

func buildRecommendationContextPack(rec *orcv1.Recommendation) string {
	if rec == nil {
		return ""
	}
	contextPack := fmt.Sprintf(
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
	if rec.SourceThreadId != "" {
		contextPack += fmt.Sprintf("\nSource thread: %s", rec.SourceThreadId)
	}
	if rec.PromotedToType != "" || rec.PromotedToId != "" {
		contextPack += fmt.Sprintf("\nPromoted to: %s %s", rec.PromotedToType, rec.PromotedToId)
	}
	return contextPack
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

func recommendationTimestampString(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().Format(time.RFC3339)
}

func connectRecommendationPromotionError(err error) error {
	switch {
	case errors.Is(err, db.ErrRecommendationConflict):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, db.ErrInvalidRecommendationTransition):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func promotedTaskCategory(rec *orcv1.Recommendation, sourceTask *orcv1.Task) string {
	if sourceTask != nil && sourceTask.Category != orcv1.TaskCategory_TASK_CATEGORY_UNSPECIFIED {
		return taskproto.CategoryFromProto(sourceTask.Category)
	}

	switch rec.Kind {
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP:
		return taskproto.CategoryFromProto(orcv1.TaskCategory_TASK_CATEGORY_CHORE)
	case orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK:
		return taskproto.CategoryFromProto(orcv1.TaskCategory_TASK_CATEGORY_BUG)
	default:
		return taskproto.CategoryFromProto(orcv1.TaskCategory_TASK_CATEGORY_FEATURE)
	}
}

func promotedTaskPriority(rec *orcv1.Recommendation, sourceTask *orcv1.Task) string {
	if sourceTask != nil && sourceTask.Priority != orcv1.TaskPriority_TASK_PRIORITY_UNSPECIFIED {
		return taskproto.PriorityFromProto(sourceTask.Priority)
	}
	if rec.Kind == orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK {
		return taskproto.PriorityFromProto(orcv1.TaskPriority_TASK_PRIORITY_HIGH)
	}
	return taskproto.PriorityFromProto(orcv1.TaskPriority_TASK_PRIORITY_NORMAL)
}

func resolvePromotedTaskWorkflow(pdb *db.ProjectDB, sourceTask *orcv1.Task, category string) (string, error) {
	if sourceTask != nil && sourceTask.WorkflowId != nil && *sourceTask.WorkflowId != "" {
		return *sourceTask.WorkflowId, nil
	}

	cfg := config.Default()
	projectDir := ""
	if pdb != nil {
		projectDir = pdb.ProjectDir()
	}
	if projectDir != "" {
		if loaded, err := config.LoadFrom(projectDir); err == nil {
			cfg = loaded
		}
	} else if loaded, err := config.Load(); err == nil {
		cfg = loaded
	}

	workflowID, _ := cfg.ResolveWorkflow("", category)
	if workflowID == "" {
		return "", fmt.Errorf("no workflow configured for promoted %s task", category)
	}
	return workflowID, nil
}

func buildPromotedTaskDescription(rec *orcv1.Recommendation, decisionReason string) string {
	lines := []string{
		fmt.Sprintf("Accepted from recommendation %s.", rec.Id),
		"",
		fmt.Sprintf("Summary: %s", rec.Summary),
		fmt.Sprintf("Proposed action: %s", rec.ProposedAction),
		fmt.Sprintf("Evidence: %s", rec.Evidence),
		fmt.Sprintf("Source task: %s", rec.SourceTaskId),
		fmt.Sprintf("Source run: %s", rec.SourceRunId),
	}
	if rec.SourceThreadId != "" {
		lines = append(lines, fmt.Sprintf("Source thread: %s", rec.SourceThreadId))
	}
	if strings.TrimSpace(decisionReason) != "" {
		lines = append(lines, fmt.Sprintf("Operator note: %s", strings.TrimSpace(decisionReason)))
	}
	return strings.Join(lines, "\n")
}

func buildPromotedDecisionRationale(rec *orcv1.Recommendation, decisionReason string) string {
	parts := []string{
		rec.Summary,
		"Evidence: " + rec.Evidence,
		"Source recommendation: " + rec.Id,
	}
	if strings.TrimSpace(decisionReason) != "" {
		parts = append(parts, "Operator note: "+strings.TrimSpace(decisionReason))
	}
	return strings.Join(parts, "\n\n")
}
