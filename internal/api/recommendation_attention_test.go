package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
)

func TestAttentionDashboardRecommendationCount(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	storageFixturesForRecommendation(t, backend)

	require.NoError(t, backend.SaveRecommendation(recommendationProtoForAPI("cleanup:task-001:attention")))

	server := NewAttentionDashboardServer(backend, nil, nil, nil)
	resp, err := server.GetAttentionDashboardData(context.Background(), connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.PendingRecommendations)
}
