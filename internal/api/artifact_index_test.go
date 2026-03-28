package api

import (
	"context"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
)

func TestArtifactIndex_AcceptedRecommendation(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	server := NewRecommendationServer(backend, slog.Default(), events.NewMemoryPublisher()).(*recommendationServer)
	server.SetProjectCache(testProjectCacheForBackend("proj-001", backend))

	createResp, err := server.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		ProjectId:      "proj-001",
		Recommendation: recommendationProtoForAPI("cleanup:task-001:artifact-index"),
	}))
	require.NoError(t, err)

	_, err = server.AcceptRecommendation(context.Background(), connect.NewRequest(&orcv1.AcceptRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
		DecidedBy:        "operator",
		DecisionReason:   "accepted for indexing",
	}))
	require.NoError(t, err)

	results, err := backend.QueryArtifactIndex(db.ArtifactIndexQueryOpts{
		Kind:         db.ArtifactKindAcceptedRecommendation,
		SourceTaskID: "TASK-001",
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "cleanup:task-001:artifact-index", results[0].DedupeKey)
	require.Equal(t, "INIT-001", results[0].InitiativeID)
	require.Contains(t, results[0].Content, "Evidence:")
}

func TestArtifactIndex_AcceptedRecommendationFallsBackToThreadInitiative(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)

	sourceTask, err := backend.LoadTask("TASK-001")
	require.NoError(t, err)
	sourceTask.InitiativeId = nil
	require.NoError(t, backend.SaveTask(sourceTask))

	server := NewRecommendationServer(backend, slog.Default(), events.NewMemoryPublisher()).(*recommendationServer)
	server.SetProjectCache(testProjectCacheForBackend("proj-001", backend))

	createResp, err := server.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
		ProjectId:      "proj-001",
		Recommendation: recommendationProtoForAPI("cleanup:task-001:artifact-index:thread-fallback"),
	}))
	require.NoError(t, err)

	_, err = server.AcceptRecommendation(context.Background(), connect.NewRequest(&orcv1.AcceptRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
		DecidedBy:        "operator",
		DecisionReason:   "thread initiative is the remaining context",
	}))
	require.NoError(t, err)

	results, err := backend.QueryArtifactIndex(db.ArtifactIndexQueryOpts{
		Kind:         db.ArtifactKindAcceptedRecommendation,
		SourceTaskID: "TASK-001",
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "cleanup:task-001:artifact-index:thread-fallback", results[0].DedupeKey)
	require.Equal(t, "INIT-001", results[0].InitiativeID)
}

func TestArtifactIndex_PromotedDraft(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	mustCreateThreadServerFixtures(t, backend)
	server := NewThreadServer(backend, events.NewMemoryPublisher(), slog.Default())
	server.SetProjectCache(testProjectCacheForBackend("proj-001", backend))

	createResp, err := server.CreateThread(context.Background(), connect.NewRequest(&orcv1.CreateThreadRequest{
		ProjectId: "proj-001",
		Title:     "Artifact draft thread",
		TaskId:    threadStringPtr("TASK-001"),
	}))
	require.NoError(t, err)

	draftResp, err := server.CreateRecommendationDraft(context.Background(), connect.NewRequest(&orcv1.CreateThreadRecommendationDraftRequest{
		ProjectId: "proj-001",
		ThreadId:  createResp.Msg.Thread.Id,
		Draft: &orcv1.ThreadRecommendationDraft{
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP,
			Title:          "Promote draft",
			Summary:        "Promote this draft into the inbox.",
			ProposedAction: "Create the recommendation.",
			Evidence:       "The thread captured the required evidence.",
		},
	}))
	require.NoError(t, err)

	_, err = server.PromoteRecommendationDraft(context.Background(), connect.NewRequest(&orcv1.PromoteThreadRecommendationDraftRequest{
		ProjectId:  "proj-001",
		ThreadId:   createResp.Msg.Thread.Id,
		DraftId:    draftResp.Msg.Draft.Id,
		PromotedBy: "operator",
	}))
	require.NoError(t, err)

	results, err := backend.QueryArtifactIndex(db.ArtifactIndexQueryOpts{
		Kind:           db.ArtifactKindPromotedDraft,
		SourceThreadID: createResp.Msg.Thread.Id,
		Limit:          10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Contains(t, results[0].Content, "Promoted recommendation:")
}

func TestArtifactIndex_InitiativeDecision(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	require.NoError(t, backend.SaveInitiativeProto(&orcv1.Initiative{
		Id:    "INIT-001",
		Title: "Artifact initiative",
	}))

	server := NewInitiativeServer(backend, slog.Default(), events.NewMemoryPublisher()).(*initiativeServer)
	server.SetProjectCache(testProjectCacheForBackend("proj-001", backend))

	_, err := server.AddDecision(context.Background(), connect.NewRequest(&orcv1.AddDecisionRequest{
		ProjectId:    "proj-001",
		InitiativeId: "INIT-001",
		Decision:     "Keep rollout gated",
		Rationale:    stringPtr("Latency regressed"),
		By:           stringPtr("operator"),
	}))
	require.NoError(t, err)

	results, err := backend.QueryArtifactIndex(db.ArtifactIndexQueryOpts{
		Kind:         db.ArtifactKindInitiativeDecision,
		InitiativeID: "INIT-001",
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Contains(t, results[0].Content, "Rationale: Latency regressed")
}

func TestArtifactIndex_AcceptedRecommendationDecisionPromotion(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	server := NewRecommendationServer(backend, slog.Default(), events.NewMemoryPublisher()).(*recommendationServer)
	server.SetProjectCache(testProjectCacheForBackend("proj-001", backend))

	createResp, err := server.CreateRecommendation(context.Background(), connect.NewRequest(&orcv1.CreateRecommendationRequest{
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
			DedupeKey:      "decision:task-001:feature-flag:indexing",
		},
	}))
	require.NoError(t, err)

	acceptResp, err := server.AcceptRecommendation(context.Background(), connect.NewRequest(&orcv1.AcceptRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: createResp.Msg.Recommendation.Id,
		DecidedBy:        "operator",
		DecisionReason:   "until the regression is fixed",
	}))
	require.NoError(t, err)

	results, err := backend.QueryArtifactIndex(db.ArtifactIndexQueryOpts{
		Kind:         db.ArtifactKindInitiativeDecision,
		InitiativeID: "INIT-001",
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "initiative_decision:INIT-001:"+acceptResp.Msg.Recommendation.PromotedToId, results[0].DedupeKey)
	require.Contains(t, results[0].Content, "Operator note: until the regression is fixed")
	require.Contains(t, results[0].Content, "Decided by: operator")
}
