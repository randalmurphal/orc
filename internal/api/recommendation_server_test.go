package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
)

func TestRecommendationServiceCRUDViaHTTP(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	publisher := &recommendationTestPublisher{}

	recommendationSvc := NewRecommendationServer(backend, slog.Default(), publisher)

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewRecommendationServiceHandler(recommendationSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewRecommendationServiceClient(http.DefaultClient, ts.URL)

	createResp, err := client.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		Recommendation: recommendationProtoForAPI("cleanup:task-001:duplicate-polling"),
	}))
	require.NoError(t, err)
	require.NotNil(t, createResp.Msg.Recommendation)
	require.NotEmpty(t, createResp.Msg.Recommendation.Id)

	getResp, err := client.GetRecommendation(context.Background(), connect.NewRequest(&orcv1.GetRecommendationRequest{
		RecommendationId: createResp.Msg.Recommendation.Id,
	}))
	require.NoError(t, err)
	require.Equal(t, createResp.Msg.Recommendation.Id, getResp.Msg.Recommendation.Id)

	listResp, err := client.ListRecommendations(context.Background(), connect.NewRequest(&orcv1.ListRecommendationsRequest{
		Status: orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Recommendations, 1)

	acceptResp, err := client.AcceptRecommendation(context.Background(), connect.NewRequest(&orcv1.AcceptRecommendationRequest{
		RecommendationId: createResp.Msg.Recommendation.Id,
		DecidedBy:        "randy",
		DecisionReason:   "do it",
	}))
	require.NoError(t, err)
	require.Equal(t, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED, acceptResp.Msg.Recommendation.Status)
	require.Equal(t, "randy", acceptResp.Msg.Recommendation.GetDecidedBy())

	require.Len(t, publisher.events, 2)
	require.Equal(t, events.EventRecommendationCreated, publisher.events[0].Type)
	require.Equal(t, events.EventRecommendationDecided, publisher.events[1].Type)
}

func TestRegisterConnectHandlersIncludesRecommendationService(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	s := &Server{
		mux:              http.NewServeMux(),
		backend:          backend,
		publisher:        publisher,
		logger:           slog.Default(),
		orcConfig:        config.Default(),
		workDir:          t.TempDir(),
		projectDB:        backend.DB(),
		runningTasks:     make(map[string]context.CancelFunc),
		diffCache:        diff.NewCache(10),
		pendingDecisions: gate.NewPendingDecisionStore(),
		projectCache:     NewProjectCache(1),
	}

	s.registerConnectHandlers()

	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	client := orcv1connect.NewRecommendationServiceClient(http.DefaultClient, ts.URL)
	resp, err := client.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		Recommendation: recommendationProtoForAPI("cleanup:task-001:register-connect"),
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Recommendation.Id)
}

func recommendationProtoForAPI(dedupeKey string) *orcv1.Recommendation {
	return &orcv1.Recommendation{
		Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP,
		Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
		Title:          "Clean up duplicate polling",
		Summary:        "Two polling loops are doing the same work.",
		ProposedAction: "Remove the legacy loop after validating the new path.",
		Evidence:       "Both loops hit the same endpoint every 5 seconds.",
		SourceTaskId:   "TASK-001",
		SourceRunId:    "RUN-001",
		DedupeKey:      dedupeKey,
	}
}

func storageFixturesForRecommendation(t *testing.T, backend *storage.DatabaseBackend) {
	t.Helper()

	require.NoError(t, backend.DB().SaveWorkflow(&db.Workflow{
		ID:   "wf-recommendation",
		Name: "Recommendation Workflow",
	}))
	require.NoError(t, backend.DB().SaveTask(&db.Task{
		ID:         "TASK-001",
		Title:      "Recommendation Source Task",
		WorkflowID: "wf-recommendation",
		Status:     "running",
	}))
	taskID := "TASK-001"
	require.NoError(t, backend.DB().SaveWorkflowRun(&db.WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "wf-recommendation",
		ContextType: "task",
		TaskID:      &taskID,
		Status:      "running",
	}))
}

type recommendationTestPublisher struct {
	events []events.Event
}

func (p *recommendationTestPublisher) Publish(event events.Event) {
	p.events = append(p.events, event)
}

func (p *recommendationTestPublisher) Subscribe(taskID string) <-chan events.Event {
	ch := make(chan events.Event)
	close(ch)
	return ch
}

func (p *recommendationTestPublisher) Unsubscribe(taskID string, ch <-chan events.Event) {}

func (p *recommendationTestPublisher) Close() {}
