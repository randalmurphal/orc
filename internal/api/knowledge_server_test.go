package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/knowledge"
	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
	"github.com/randalmurphal/orc/internal/storage"
)

func TestKnowledgeServer_Query_DelegatesToService(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	svc := &mockKnowledgeService{
		available: true,
		queryResult: &retrieve.PipelineResult{
			TokensUsed: 321,
			Documents: []retrieve.ScoredDocument{
				{
					Document: retrieve.Document{
						ID:       "doc-1",
						Content:  "Gate evaluation flow",
						Summary:  "Gate summary",
						FilePath: "internal/executor/workflow_executor.go",
						Metadata: map[string]interface{}{
							"type":       "code",
							"start_line": 42,
							"end_line":   84,
						},
					},
					FinalScore: 0.93,
				},
			},
		},
	}

	server := NewKnowledgeServer(backend, svc)
	req := connect.NewRequest(&orcv1.QueryKnowledgeRequest{
		Query:       "how does gate evaluation work",
		Preset:      "deep",
		Limit:       7,
		MaxTokens:   4096,
		MinScore:    0.35,
		SummaryOnly: true,
	})

	resp, err := server.Query(context.Background(), req)
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}

	if svc.lastQuery != "how does gate evaluation work" {
		t.Errorf("query = %q, want %q", svc.lastQuery, "how does gate evaluation work")
	}
	if svc.lastOpts.Preset != "deep" {
		t.Errorf("preset = %q, want %q", svc.lastOpts.Preset, "deep")
	}
	if svc.lastOpts.Limit != 7 {
		t.Errorf("limit = %d, want 7", svc.lastOpts.Limit)
	}
	if svc.lastOpts.MaxTokens != 4096 {
		t.Errorf("maxTokens = %d, want 4096", svc.lastOpts.MaxTokens)
	}
	if svc.lastOpts.MinScore != 0.35 {
		t.Errorf("minScore = %f, want 0.35", svc.lastOpts.MinScore)
	}
	if !svc.lastOpts.SummaryOnly {
		t.Error("summaryOnly = false, want true")
	}

	if resp.Msg.TokensUsed != 321 {
		t.Errorf("TokensUsed = %d, want 321", resp.Msg.TokensUsed)
	}
	if len(resp.Msg.Results) != 1 {
		t.Fatalf("results length = %d, want 1", len(resp.Msg.Results))
	}

	got := resp.Msg.Results[0]
	if got.Type != orcv1.KnowledgeResultType_KNOWLEDGE_RESULT_TYPE_CODE {
		t.Errorf("type = %v, want CODE", got.Type)
	}
	if got.FilePath != "internal/executor/workflow_executor.go" {
		t.Errorf("filePath = %q", got.FilePath)
	}
	if got.StartLine != 42 || got.EndLine != 84 {
		t.Errorf("line range = %d-%d, want 42-84", got.StartLine, got.EndLine)
	}
	if got.Score <= 0 {
		t.Errorf("score = %f, want > 0", got.Score)
	}
}

func TestKnowledgeServer_Query_ErrorPropagation(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	svc := &mockKnowledgeService{
		available: true,
		queryErr:  errors.New("vector search failed"),
	}

	server := NewKnowledgeServer(backend, svc)
	_, err := server.Query(context.Background(), connect.NewRequest(&orcv1.QueryKnowledgeRequest{
		Query: "query",
	}))
	if err == nil {
		t.Fatal("Query() should return error")
	}

	connectErr := new(connect.Error)
	if !errors.As(err, &connectErr) {
		t.Fatalf("error should be connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInternal {
		t.Errorf("code = %v, want %v", connectErr.Code(), connect.CodeInternal)
	}
}

func TestKnowledgeServer_Query_UnavailableReturnsFailedPrecondition(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	svc := &mockKnowledgeService{available: false}

	server := NewKnowledgeServer(backend, svc)
	_, err := server.Query(context.Background(), connect.NewRequest(&orcv1.QueryKnowledgeRequest{
		Query: "query",
	}))
	if err == nil {
		t.Fatal("Query() should return error")
	}

	connectErr := new(connect.Error)
	if !errors.As(err, &connectErr) {
		t.Fatalf("error should be connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("code = %v, want %v", connectErr.Code(), connect.CodeFailedPrecondition)
	}
}

func TestKnowledgeServer_GetStatus(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	svc := &mockKnowledgeService{
		status: &knowledge.ServiceStatus{
			Enabled: true,
			Running: true,
			Neo4j:   true,
			Qdrant:  true,
			Redis:   true,
		},
	}

	server := NewKnowledgeServer(backend, svc)
	resp, err := server.GetStatus(context.Background(), connect.NewRequest(&orcv1.GetKnowledgeStatusRequest{}))
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}

	if !resp.Msg.Status.Enabled {
		t.Error("Enabled = false, want true")
	}
	if !resp.Msg.Status.Running {
		t.Error("Running = false, want true")
	}
	if !resp.Msg.Status.Neo4J || !resp.Msg.Status.Qdrant || !resp.Msg.Status.Redis {
		t.Error("expected all dependency flags to be true")
	}
}

func TestKnowledgeServer_GetInsights(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	now := time.Now().UTC()
	svc := &mockKnowledgeService{
		available: true,
		insights: &knowledge.Insights{
			HotFiles: []retrieve.Document{
				{
					FilePath: "internal/api/server.go",
					Summary:  "Most touched this week",
					Metadata: map[string]interface{}{"hit_count": 9},
				},
			},
			Patterns: []retrieve.Document{
				{
					ID:      "pipeline-pattern",
					Content: "Workflow stage fan-out",
					Metadata: map[string]interface{}{
						"name":         "Pipeline Fan-Out",
						"member_count": 4,
					},
				},
			},
			ConstitutionUpdates: []retrieve.Document{
				{
					ID:      "constitution-update-1",
					Content: "Updated error propagation rule",
					Metadata: map[string]interface{}{
						"title":      "Error Handling Rule",
						"source":     ".orc/CONSTITUTION.md",
						"updated_at": now.Format(time.RFC3339),
					},
				},
			},
		},
	}

	server := NewKnowledgeServer(backend, svc)
	resp, err := server.GetInsights(context.Background(), connect.NewRequest(&orcv1.GetKnowledgeInsightsRequest{}))
	if err != nil {
		t.Fatalf("GetInsights() error: %v", err)
	}

	if len(resp.Msg.HotFiles) != 1 {
		t.Fatalf("HotFiles length = %d, want 1", len(resp.Msg.HotFiles))
	}
	if resp.Msg.HotFiles[0].FilePath != "internal/api/server.go" {
		t.Errorf("hot file path = %q", resp.Msg.HotFiles[0].FilePath)
	}
	if resp.Msg.HotFiles[0].HitCount != 9 {
		t.Errorf("hit_count = %d, want 9", resp.Msg.HotFiles[0].HitCount)
	}

	if len(resp.Msg.RecurringPatterns) != 1 {
		t.Fatalf("RecurringPatterns length = %d, want 1", len(resp.Msg.RecurringPatterns))
	}
	if resp.Msg.RecurringPatterns[0].Name != "Pipeline Fan-Out" {
		t.Errorf("pattern name = %q", resp.Msg.RecurringPatterns[0].Name)
	}
	if resp.Msg.RecurringPatterns[0].MemberCount != 4 {
		t.Errorf("member_count = %d, want 4", resp.Msg.RecurringPatterns[0].MemberCount)
	}

	if len(resp.Msg.ConstitutionUpdates) != 1 {
		t.Fatalf("ConstitutionUpdates length = %d, want 1", len(resp.Msg.ConstitutionUpdates))
	}
	if resp.Msg.ConstitutionUpdates[0].Title != "Error Handling Rule" {
		t.Errorf("title = %q", resp.Msg.ConstitutionUpdates[0].Title)
	}
	if resp.Msg.ConstitutionUpdates[0].UpdatedAt == nil {
		t.Error("updated_at is nil")
	}
}

type mockKnowledgeService struct {
	available bool
	status    *knowledge.ServiceStatus
	insights  *knowledge.Insights

	queryResult *retrieve.PipelineResult
	queryErr    error
	lastQuery   string
	lastOpts    retrieve.QueryOpts
}

func (m *mockKnowledgeService) IsAvailable() bool {
	return m.available
}

func (m *mockKnowledgeService) Query(_ context.Context, query string, opts retrieve.QueryOpts) (*retrieve.PipelineResult, error) {
	m.lastQuery = query
	m.lastOpts = opts
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	if m.queryResult == nil {
		return &retrieve.PipelineResult{}, nil
	}
	return m.queryResult, nil
}

func (m *mockKnowledgeService) Status(_ context.Context) (*knowledge.ServiceStatus, error) {
	if m.status == nil {
		return &knowledge.ServiceStatus{}, nil
	}
	return m.status, nil
}

func (m *mockKnowledgeService) Insights(_ context.Context) (*knowledge.Insights, error) {
	if m.insights == nil {
		return &knowledge.Insights{}, nil
	}
	return m.insights, nil
}
