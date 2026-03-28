package storage

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/randalmurphal/orc/internal/db"
)

func TestArtifactIndex_BackendRoundTrip(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)
	entry := &db.ArtifactIndexEntry{
		Kind:      db.ArtifactKindAcceptedRecommendation,
		Title:     "Accepted cleanup",
		Content:   "Summary: Remove duplicate polling.\nEvidence: Accepted by operator.",
		DedupeKey: "cleanup:duplicate-polling",
	}

	require.NoError(t, backend.SaveArtifactIndexEntry(entry))
	require.NotZero(t, entry.ID)

	results, err := backend.QueryArtifactIndexByDedupeKey("cleanup:duplicate-polling")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, db.ArtifactKindAcceptedRecommendation, results[0].Kind)
}
