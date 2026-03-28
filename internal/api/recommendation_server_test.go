package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	projectCache := testProjectCacheForBackend("proj-001", backend)

	recommendationSvc := NewRecommendationServer(backend, slog.Default(), publisher)
	recommendationSvc.(*recommendationServer).SetProjectCache(projectCache)

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewRecommendationServiceHandler(recommendationSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewRecommendationServiceClient(http.DefaultClient, ts.URL)

	createResp, err := client.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		ProjectId:      "proj-001",
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
	require.Equal(t, "THR-001", getResp.Msg.Recommendation.SourceThreadId)
	require.Empty(t, getResp.Msg.Recommendation.PromotedToType)
	require.Empty(t, getResp.Msg.Recommendation.PromotedToId)

	listResp, err := client.ListRecommendations(context.Background(), connect.NewRequest(&orcv1.ListRecommendationsRequest{
		Status: orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Recommendations, 1)

	acceptResp, err := client.AcceptRecommendation(context.Background(), connect.NewRequest(&orcv1.AcceptRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
		DecidedBy:        "randy",
		DecisionReason:   "do it",
	}))
	require.NoError(t, err)
	require.Equal(t, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED, acceptResp.Msg.Recommendation.Status)
	require.Equal(t, "randy", acceptResp.Msg.Recommendation.GetDecidedBy())
	require.Equal(t, "task", acceptResp.Msg.Recommendation.PromotedToType)
	require.NotEmpty(t, acceptResp.Msg.Recommendation.PromotedToId)

	promotedTask, err := backend.LoadTask(acceptResp.Msg.Recommendation.PromotedToId)
	require.NoError(t, err)
	require.NotNil(t, promotedTask)
	require.Equal(t, "Clean up duplicate polling", promotedTask.Title)
	require.Equal(t, orcv1.TaskQueue_TASK_QUEUE_BACKLOG, promotedTask.Queue)
	require.Contains(t, promotedTask.GetDescription(), "Accepted from recommendation")
	require.Equal(t, "INIT-001", promotedTask.GetInitiativeId())

	require.Len(t, publisher.events, 3)
	require.Equal(t, events.EventRecommendationCreated, publisher.events[0].Type)
	require.Equal(t, "proj-001", publisher.events[0].ProjectID)
	require.Equal(t, events.EventRecommendationDecided, publisher.events[1].Type)
	require.Equal(t, "proj-001", publisher.events[1].ProjectID)
	require.Equal(t, events.EventTaskCreated, publisher.events[2].Type)
	require.Equal(t, "proj-001", publisher.events[2].ProjectID)

	historyResp, err := client.ListRecommendationHistory(context.Background(), connect.NewRequest(&orcv1.ListRecommendationHistoryRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
	}))
	require.NoError(t, err)
	require.Len(t, historyResp.Msg.History, 2)
	require.Equal(t, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED, historyResp.Msg.History[0].ToStatus)
	require.Equal(t, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING, historyResp.Msg.History[1].ToStatus)

	idempotentAcceptResp, err := client.AcceptRecommendation(context.Background(), connect.NewRequest(&orcv1.AcceptRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
		DecidedBy:        "randy",
		DecisionReason:   "do it again",
	}))
	require.NoError(t, err)
	require.Equal(t, acceptResp.Msg.Recommendation.PromotedToId, idempotentAcceptResp.Msg.Recommendation.PromotedToId)
	require.Len(t, publisher.events, 3)
}

func TestRecommendationServiceAcceptDecisionRequestPromotesToInitiativeDecision(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	publisher := &recommendationTestPublisher{}
	projectCache := testProjectCacheForBackend("proj-001", backend)

	recommendationSvc := NewRecommendationServer(backend, slog.Default(), publisher)
	recommendationSvc.(*recommendationServer).SetProjectCache(projectCache)

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewRecommendationServiceHandler(recommendationSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewRecommendationServiceClient(http.DefaultClient, ts.URL)

	createResp, err := client.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		ProjectId: "proj-001",
		Recommendation: &orcv1.Recommendation{
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_DECISION_REQUEST,
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Title:          "Freeze the rollout behind a feature flag",
			Summary:        "The operator should decide whether the rollout stays gated.",
			ProposedAction: "Keep the feature flag enabled until the latency regression is gone.",
			Evidence:       "Latency climbed 18% on the last canary run.",
			SourceTaskId:   "TASK-001",
			SourceRunId:    "RUN-001",
			SourceThreadId: "THR-001",
			DedupeKey:      "decision:task-001:feature-flag",
		},
	}))
	require.NoError(t, err)

	acceptResp, err := client.AcceptRecommendation(context.Background(), connect.NewRequest(&orcv1.AcceptRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
		DecidedBy:        "randy",
		DecisionReason:   "until the regression is fixed",
	}))
	require.NoError(t, err)
	require.Equal(t, "initiative_decision", acceptResp.Msg.Recommendation.PromotedToType)
	require.Equal(t, "DEC-"+createResp.Msg.Recommendation.Id, acceptResp.Msg.Recommendation.PromotedToId)

	initRecord, err := backend.LoadInitiativeProto("INIT-001")
	require.NoError(t, err)
	require.Len(t, initRecord.Decisions, 1)
	require.Equal(t, "Keep the feature flag enabled until the latency regression is gone.", initRecord.Decisions[0].Decision)
	require.Contains(t, initRecord.Decisions[0].GetRationale(), "until the regression is fixed")
}

func TestRecommendationServiceRejectsPrePromotedCreate(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	projectCache := testProjectCacheForBackend("proj-001", backend)

	recommendationSvc := NewRecommendationServer(backend, slog.Default(), nil)
	recommendationSvc.(*recommendationServer).SetProjectCache(projectCache)

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewRecommendationServiceHandler(recommendationSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewRecommendationServiceClient(http.DefaultClient, ts.URL)

	rec := recommendationProtoForAPI("cleanup:task-001:pre-promoted")
	rec.PromotedToType = "task"
	rec.PromotedToId = "TASK-999"
	rec.PromotedBy = "operator"
	rec.PromotedAt = timestamppb.Now()
	_, err := client.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		ProjectId:      "proj-001",
		Recommendation: rec,
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestRecommendationServiceListHistoryReturnsNotFoundForMissingRecommendation(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	projectCache := testProjectCacheForBackend("proj-001", backend)

	recommendationSvc := NewRecommendationServer(backend, slog.Default(), nil)
	recommendationSvc.(*recommendationServer).SetProjectCache(projectCache)

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewRecommendationServiceHandler(recommendationSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewRecommendationServiceClient(http.DefaultClient, ts.URL)
	_, err := client.ListRecommendationHistory(context.Background(), connect.NewRequest(&orcv1.ListRecommendationHistoryRequest{
		ProjectId:        "proj-001",
		RecommendationId: "REC-404",
	}))
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestRecommendationServiceRejectAndDiscussAreIdempotent(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	publisher := &recommendationTestPublisher{}
	projectCache := testProjectCacheForBackend("proj-001", backend)

	recommendationSvc := NewRecommendationServer(backend, slog.Default(), publisher)
	recommendationSvc.(*recommendationServer).SetProjectCache(projectCache)

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewRecommendationServiceHandler(recommendationSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewRecommendationServiceClient(http.DefaultClient, ts.URL)

	createResp, err := client.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		ProjectId:      "proj-001",
		Recommendation: recommendationProtoForAPI("cleanup:task-001:discuss-idempotent"),
	}))
	require.NoError(t, err)

	discussResp, err := client.DiscussRecommendation(context.Background(), connect.NewRequest(&orcv1.DiscussRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
		DecidedBy:        "randy",
		DecisionReason:   "talk it through",
	}))
	require.NoError(t, err)
	require.Equal(t, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_DISCUSSED, discussResp.Msg.Recommendation.Status)

	discussResp, err = client.DiscussRecommendation(context.Background(), connect.NewRequest(&orcv1.DiscussRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
		DecidedBy:        "randy",
		DecisionReason:   "same request",
	}))
	require.NoError(t, err)
	require.Equal(t, "talk it through", discussResp.Msg.Recommendation.GetDecisionReason())

	historyResp, err := client.ListRecommendationHistory(context.Background(), connect.NewRequest(&orcv1.ListRecommendationHistoryRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
	}))
	require.NoError(t, err)
	require.Len(t, historyResp.Msg.History, 2)

	rejectCreateResp, err := client.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		ProjectId:      "proj-001",
		Recommendation: recommendationProtoForAPI("cleanup:task-001:reject-idempotent"),
	}))
	require.NoError(t, err)

	rejectResp, err := client.RejectRecommendation(context.Background(), connect.NewRequest(&orcv1.RejectRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: rejectCreateResp.Msg.Recommendation.Id,
		DecidedBy:        "randy",
		DecisionReason:   "not worth it",
	}))
	require.NoError(t, err)
	require.Equal(t, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_REJECTED, rejectResp.Msg.Recommendation.Status)

	rejectResp, err = client.RejectRecommendation(context.Background(), connect.NewRequest(&orcv1.RejectRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: rejectCreateResp.Msg.Recommendation.Id,
		DecidedBy:        "randy",
		DecisionReason:   "same request",
	}))
	require.NoError(t, err)
	require.Equal(t, "not worth it", rejectResp.Msg.Recommendation.GetDecisionReason())
}

func TestRecommendationServiceDefaultsDecisionActorViaHTTP(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	publisher := &recommendationTestPublisher{}
	projectCache := testProjectCacheForBackend("proj-001", backend)

	recommendationSvc := NewRecommendationServer(backend, slog.Default(), publisher)
	recommendationSvc.(*recommendationServer).SetProjectCache(projectCache)

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewRecommendationServiceHandler(recommendationSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewRecommendationServiceClient(http.DefaultClient, ts.URL)

	createResp, err := client.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		ProjectId:      "proj-001",
		Recommendation: recommendationProtoForAPI("cleanup:task-001:default-actor"),
	}))
	require.NoError(t, err)

	discussResp, err := client.DiscussRecommendation(context.Background(), connect.NewRequest(&orcv1.DiscussRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, discussResp.Msg.Recommendation.GetDecidedBy())
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
		projectCache:     testProjectCacheForBackend("proj-001", backend),
	}

	s.registerConnectHandlers()

	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	client := orcv1connect.NewRecommendationServiceClient(http.DefaultClient, ts.URL)
	resp, err := client.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		ProjectId:      "proj-001",
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
		SourceThreadId: "THR-001",
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
	thread := &db.Thread{
		Title:        "Recommendation discussion",
		TaskID:       taskID,
		InitiativeID: "INIT-001",
	}
	require.NoError(t, backend.DB().CreateThread(thread))
	require.NoError(t, backend.DB().SaveInitiative(&db.Initiative{
		ID:        "INIT-001",
		Title:     "Recommendation Initiative",
		Status:    "active",
		CreatedAt: time.Date(2026, time.March, 9, 17, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.March, 9, 17, 0, 0, 0, time.UTC),
	}))

	sourceTask, err := backend.LoadTask("TASK-001")
	require.NoError(t, err)
	sourceTask.InitiativeId = stringPointer("INIT-001")
	require.NoError(t, backend.SaveTask(sourceTask))
}

func stringPointer(value string) *string {
	return &value
}

func testProjectCacheForBackend(projectID string, backend *storage.DatabaseBackend) *ProjectCache {
	cache := NewProjectCache(1)
	cache.entries[projectID] = &cacheEntry{
		db:      backend.DB(),
		backend: backend,
		path:    "",
	}
	cache.order = append(cache.order, projectID)
	return cache
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
