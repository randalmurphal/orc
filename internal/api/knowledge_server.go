package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/knowledge"
	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
	"github.com/randalmurphal/orc/internal/storage"
)

type knowledgeQueryService interface {
	IsAvailable() bool
	Query(ctx context.Context, query string, opts retrieve.QueryOpts) (*retrieve.PipelineResult, error)
	Status(ctx context.Context) (*knowledge.ServiceStatus, error)
	Insights(ctx context.Context) (*knowledge.Insights, error)
}

type knowledgeServer struct {
	orcv1connect.UnimplementedKnowledgeServiceHandler
	backend      storage.Backend
	projectCache *ProjectCache
	knowledgeSvc knowledgeQueryService
}

// NewKnowledgeServer creates a KnowledgeService handler.
func NewKnowledgeServer(backend storage.Backend, knowledgeSvc knowledgeQueryService) orcv1connect.KnowledgeServiceHandler {
	return &knowledgeServer{
		backend:      backend,
		knowledgeSvc: knowledgeSvc,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *knowledgeServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

func (s *knowledgeServer) getBackend(projectID string) (storage.Backend, error) {
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

func (s *knowledgeServer) Query(
	ctx context.Context,
	req *connect.Request[orcv1.QueryKnowledgeRequest],
) (*connect.Response[orcv1.QueryKnowledgeResponse], error) {
	if _, err := s.getBackend(req.Msg.GetProjectId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	query := strings.TrimSpace(req.Msg.GetQuery())
	if query == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("query is required"))
	}

	if s.knowledgeSvc == nil || !s.knowledgeSvc.IsAvailable() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("knowledge service is not available"))
	}

	opts := retrieve.QueryOpts{
		Preset:      req.Msg.GetPreset(),
		MaxTokens:   int(req.Msg.GetMaxTokens()),
		MinScore:    req.Msg.GetMinScore(),
		SummaryOnly: req.Msg.GetSummaryOnly(),
		Limit:       int(req.Msg.GetLimit()),
	}

	result, err := s.knowledgeSvc.Query(ctx, query, opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("knowledge query failed: %w", err))
	}

	resp := &orcv1.QueryKnowledgeResponse{
		TokensUsed: int32(result.TokensUsed),
	}
	for _, doc := range result.Documents {
		resp.Results = append(resp.Results, scoredDocToProto(doc))
	}

	return connect.NewResponse(resp), nil
}

func (s *knowledgeServer) GetStatus(
	ctx context.Context,
	req *connect.Request[orcv1.GetKnowledgeStatusRequest],
) (*connect.Response[orcv1.GetKnowledgeStatusResponse], error) {
	if _, err := s.getBackend(req.Msg.GetProjectId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	if s.knowledgeSvc == nil {
		return connect.NewResponse(&orcv1.GetKnowledgeStatusResponse{
			Status: &orcv1.KnowledgeStatus{},
		}), nil
	}

	status, err := s.knowledgeSvc.Status(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get knowledge status: %w", err))
	}

	return connect.NewResponse(&orcv1.GetKnowledgeStatusResponse{
		Status: &orcv1.KnowledgeStatus{
			Enabled: status.Enabled,
			Running: status.Running,
			Neo4J:   status.Neo4j,
			Qdrant:  status.Qdrant,
			Redis:   status.Redis,
		},
	}), nil
}

func (s *knowledgeServer) GetInsights(
	ctx context.Context,
	req *connect.Request[orcv1.GetKnowledgeInsightsRequest],
) (*connect.Response[orcv1.GetKnowledgeInsightsResponse], error) {
	if _, err := s.getBackend(req.Msg.GetProjectId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	if s.knowledgeSvc == nil || !s.knowledgeSvc.IsAvailable() {
		return connect.NewResponse(&orcv1.GetKnowledgeInsightsResponse{}), nil
	}

	insights, err := s.knowledgeSvc.Insights(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get knowledge insights: %w", err))
	}

	resp := &orcv1.GetKnowledgeInsightsResponse{}
	for _, hot := range insights.HotFiles {
		resp.HotFiles = append(resp.HotFiles, hotFileToProto(hot))
	}
	for _, pat := range insights.Patterns {
		resp.RecurringPatterns = append(resp.RecurringPatterns, patternToProto(pat))
	}
	for _, update := range insights.ConstitutionUpdates {
		resp.ConstitutionUpdates = append(resp.ConstitutionUpdates, constitutionUpdateToProto(update))
	}

	return connect.NewResponse(resp), nil
}

func scoredDocToProto(doc retrieve.ScoredDocument) *orcv1.KnowledgeResult {
	metadata := doc.Metadata
	result := &orcv1.KnowledgeResult{
		Id:              doc.ID,
		Type:            metadataTypeToProto(metadata),
		Title:           firstNonEmpty(metadataString(metadata, "title"), doc.ID),
		Content:         doc.Content,
		Summary:         doc.Summary,
		FilePath:        firstNonEmpty(doc.FilePath, metadataString(metadata, "file_path"), metadataString(metadata, "path")),
		StartLine:       int32(metadataInt(metadata, "start_line")),
		EndLine:         int32(metadataInt(metadata, "end_line")),
		Score:           doc.FinalScore,
		Severity:        metadataString(metadata, "severity"),
		Status:          metadataString(metadata, "status"),
		InitiativeId:    metadataString(metadata, "initiative_id"),
		InitiativeTitle: metadataString(metadata, "initiative_title"),
		Rationale:       metadataString(metadata, "rationale"),
		MemberCount:     int32(metadataInt(metadata, "member_count")),
	}

	if !doc.UpdatedAt.IsZero() {
		result.UpdatedAt = timestamppb.New(doc.UpdatedAt)
	} else if ts := metadataTime(metadata, "updated_at", "last_updated"); !ts.IsZero() {
		result.UpdatedAt = timestamppb.New(ts)
	}

	return result
}

func hotFileToProto(doc retrieve.Document) *orcv1.KnowledgeHotFileInsight {
	return &orcv1.KnowledgeHotFileInsight{
		FilePath: firstNonEmpty(doc.FilePath, metadataString(doc.Metadata, "file_path"), metadataString(doc.Metadata, "path")),
		HitCount: int32(metadataInt(doc.Metadata, "hit_count", "count", "frequency")),
		Summary:  summaryContent(doc),
	}
}

func patternToProto(doc retrieve.Document) *orcv1.KnowledgePatternInsight {
	return &orcv1.KnowledgePatternInsight{
		Name:        firstNonEmpty(metadataString(doc.Metadata, "name"), doc.ID),
		MemberCount: int32(metadataInt(doc.Metadata, "member_count")),
		Summary:     summaryContent(doc),
	}
}

func constitutionUpdateToProto(doc retrieve.Document) *orcv1.KnowledgeConstitutionUpdate {
	update := &orcv1.KnowledgeConstitutionUpdate{
		Title:   firstNonEmpty(metadataString(doc.Metadata, "title"), doc.ID),
		Summary: summaryContent(doc),
		Source:  firstNonEmpty(metadataString(doc.Metadata, "source"), doc.FilePath),
	}
	if ts := metadataTime(doc.Metadata, "updated_at", "date"); !ts.IsZero() {
		update.UpdatedAt = timestamppb.New(ts)
	}
	return update
}

func summaryContent(doc retrieve.Document) string {
	return firstNonEmpty(doc.Summary, doc.Content)
}

func metadataTypeToProto(metadata map[string]interface{}) orcv1.KnowledgeResultType {
	switch strings.ToLower(metadataString(metadata, "type")) {
	case "finding", "warning", "issue", "known_issue":
		return orcv1.KnowledgeResultType_KNOWLEDGE_RESULT_TYPE_FINDING
	case "decision":
		return orcv1.KnowledgeResultType_KNOWLEDGE_RESULT_TYPE_DECISION
	case "pattern":
		return orcv1.KnowledgeResultType_KNOWLEDGE_RESULT_TYPE_PATTERN
	case "code", "chunk", "file", "file_history":
		return orcv1.KnowledgeResultType_KNOWLEDGE_RESULT_TYPE_CODE
	default:
		if metadataInt(metadata, "member_count") > 0 {
			return orcv1.KnowledgeResultType_KNOWLEDGE_RESULT_TYPE_PATTERN
		}
		return orcv1.KnowledgeResultType_KNOWLEDGE_RESULT_TYPE_CODE
	}
}

func metadataString(metadata map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		val, ok := metadata[key]
		if !ok {
			continue
		}
		switch v := val.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return v
			}
		}
	}
	return ""
}

func metadataInt(metadata map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		val, ok := metadata[key]
		if !ok {
			continue
		}
		switch v := val.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float32:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

func metadataTime(metadata map[string]interface{}, keys ...string) time.Time {
	for _, key := range keys {
		val, ok := metadata[key]
		if !ok {
			continue
		}
		switch v := val.(type) {
		case time.Time:
			return v
		case string:
			ts, err := time.Parse(time.RFC3339, v)
			if err == nil {
				return ts
			}
		}
	}
	return time.Time{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
