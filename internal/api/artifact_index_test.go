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
