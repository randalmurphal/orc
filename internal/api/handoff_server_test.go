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

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
)

func TestHandoffServerGenerateHandoffViaHTTP(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	projectCache := testProjectCacheForBackend("proj-001", backend)

	require.NoError(t, backend.SaveRecommendation(recommendationProtoForAPI("cleanup:task-001:handoff")))
	require.NoError(t, backend.SaveAttentionSignal(&controlplane.PersistedAttentionSignal{
		ID:            "SIG-001",
		ProjectID:     "proj-001",
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        controlplane.AttentionSignalStatusFailed,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   "TASK-001",
		Title:         "Recommendation Source Task",
		Summary:       "Verification failed in review.",
		CreatedAt:     time.Date(2026, time.March, 28, 10, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, time.March, 28, 10, 0, 0, 0, time.UTC),
	}))

	client := newHandoffHTTPClient(t, backend, projectCache, gate.NewPendingDecisionStore())
	ctx := context.Background()

	testCases := []struct {
		name       string
		sourceType orcv1.HandoffSourceType
		sourceID   string
	}{
		{name: "task", sourceType: orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_TASK, sourceID: "TASK-001"},
		{name: "thread", sourceType: orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_THREAD, sourceID: "THR-001"},
		{name: "recommendation", sourceType: orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_RECOMMENDATION, sourceID: "REC-001"},
		{name: "attention_item", sourceType: orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_ATTENTION_ITEM, sourceID: attentionItemID("proj-001", "failed-TASK-001")},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp, err := client.GenerateHandoff(ctx, connect.NewRequest(&orcv1.GenerateHandoffRequest{
				ProjectId:  "proj-001",
				SourceType: tc.sourceType,
				SourceId:   tc.sourceID,
				Target:     orcv1.HandoffTarget_HANDOFF_TARGET_CLAUDE_CODE,
			}))
			require.NoError(t, err)
			require.NotEmpty(t, resp.Msg.GetContextPack())
			require.NotEmpty(t, resp.Msg.GetBootstrapPrompt())
			require.NotEmpty(t, resp.Msg.GetCliCommand())
		})
	}
}

func TestHandoffServerErrors(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	projectCache := testProjectCacheForBackend("proj-001", backend)
	client := newHandoffHTTPClient(t, backend, projectCache, gate.NewPendingDecisionStore())

	_, err := client.GenerateHandoff(context.Background(), connect.NewRequest(&orcv1.GenerateHandoffRequest{
		ProjectId:  "proj-001",
		SourceType: orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_TASK,
		SourceId:   "TASK-404",
		Target:     orcv1.HandoffTarget_HANDOFF_TARGET_CLAUDE_CODE,
	}))
	require.Error(t, err)
	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeNotFound, connectErr.Code())

	_, err = client.GenerateHandoff(context.Background(), connect.NewRequest(&orcv1.GenerateHandoffRequest{
		ProjectId:  "proj-001",
		SourceType: orcv1.HandoffSourceType(99),
		SourceId:   "TASK-001",
		Target:     orcv1.HandoffTarget_HANDOFF_TARGET_CLAUDE_CODE,
	}))
	require.Error(t, err)
	connectErr = new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	require.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestRecommendationPackParity(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)
	projectCache := testProjectCacheForBackend("proj-001", backend)

	require.NoError(t, backend.SaveRecommendation(recommendationProtoForAPI("cleanup:task-001:parity")))

	handoffClient := newHandoffHTTPClient(t, backend, projectCache, gate.NewPendingDecisionStore())
	recommendationSvc := NewRecommendationServer(backend, slog.Default(), nil)
	recommendationSvc.(*recommendationServer).SetProjectCache(projectCache)

	handoffResp, err := handoffClient.GenerateHandoff(context.Background(), connect.NewRequest(&orcv1.GenerateHandoffRequest{
		ProjectId:  "proj-001",
		SourceType: orcv1.HandoffSourceType_HANDOFF_SOURCE_TYPE_RECOMMENDATION,
		SourceId:   "REC-001",
		Target:     orcv1.HandoffTarget_HANDOFF_TARGET_CLAUDE_CODE,
	}))
	require.NoError(t, err)

	discussResp, err := recommendationSvc.DiscussRecommendation(context.Background(), connect.NewRequest(&orcv1.DiscussRecommendationRequest{
		ProjectId:        "proj-001",
		RecommendationId: "REC-001",
		DecidedBy:        "operator",
		DecisionReason:   "Needs discussion",
	}))
	require.NoError(t, err)

	require.Equal(t, discussResp.Msg.GetContextPack(), handoffResp.Msg.GetContextPack())
}

func newHandoffHTTPClient(
	t *testing.T,
	backend *storage.DatabaseBackend,
	projectCache *ProjectCache,
	pendingDecisions *gate.PendingDecisionStore,
) orcv1connect.HandoffServiceClient {
	t.Helper()

	handoffSvc := NewHandoffServer(backend, slog.Default(), pendingDecisions)
	handoffSvc.(*handoffServer).SetProjectCache(projectCache)

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewHandoffServiceHandler(handoffSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return orcv1connect.NewHandoffServiceClient(http.DefaultClient, ts.URL)
}
