package brief

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

func TestBriefWithArtifactIndex(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	require.NoError(t, backend.DB().SaveInitiative(&db.Initiative{
		ID:     "INIT-001",
		Title:  "Brief Artifacts",
		Status: "active",
	}))
	require.NoError(t, backend.SaveArtifactIndexEntry(&db.ArtifactIndexEntry{
		Kind:         db.ArtifactKindAcceptedRecommendation,
		Title:        "Accepted cleanup",
		Content:      strings.Repeat("Remove duplicate polling guards. ", 20),
		DedupeKey:    "cleanup:duplicate-polling",
		InitiativeID: "INIT-001",
	}))
	require.NoError(t, backend.SaveArtifactIndexEntry(&db.ArtifactIndexEntry{
		Kind:         db.ArtifactKindInitiativeDecision,
		Title:        "Keep rollout gated",
		Content:      strings.Repeat("Feature flag stays enabled while latency is unstable. ", 16),
		DedupeKey:    "initiative_decision:INIT-001:DEC-001",
		InitiativeID: "INIT-001",
	}))

	gen := NewGenerator(backend, DefaultConfig())
	brief, err := gen.Generate(context.Background())
	require.NoError(t, err)
	require.NotNil(t, brief)

	section := findSection(brief, CategoryIndexedArtifacts)
	require.NotNil(t, section)
	require.NotEmpty(t, section.Entries)
	require.LessOrEqual(t, brief.TokenCount, DefaultConfig().MaxTokens)

	formatted := FormatBrief(brief)
	require.Contains(t, formatted, "### Indexed Artifacts")
	require.Contains(t, formatted, "Accepted cleanup")
}
