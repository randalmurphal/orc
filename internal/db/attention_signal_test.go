package db

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/randalmurphal/orc/internal/controlplane"
)

func TestAttentionSignalCRUD(t *testing.T) {
	t.Parallel()

	pdb, err := OpenProjectInMemory()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pdb.Close()
	})

	signals := []*controlplane.PersistedAttentionSignal{
		{
			Kind:          controlplane.AttentionSignalKindBlocker,
			Status:        controlplane.AttentionSignalStatusBlocked,
			ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
			ReferenceID:   "TASK-001",
			Title:         "Blocked task",
			Summary:       "Waiting on schema review.",
		},
		{
			Kind:          controlplane.AttentionSignalKindDecisionRequest,
			Status:        controlplane.AttentionSignalStatusActive,
			ReferenceType: controlplane.AttentionSignalReferenceTypeRecommendation,
			ReferenceID:   "REC-001",
			Title:         "Pick a cleanup path",
			Summary:       "Operator decision required.",
		},
		{
			Kind:          controlplane.AttentionSignalKindDiscussionNeeded,
			Status:        controlplane.AttentionSignalStatusActive,
			ReferenceType: controlplane.AttentionSignalReferenceTypeRun,
			ReferenceID:   "RUN-001",
			Title:         "Discuss the retry behavior",
			Summary:       "The current output is ambiguous.",
		},
		{
			Kind:          controlplane.AttentionSignalKindVerificationSummary,
			Status:        controlplane.AttentionSignalStatusActive,
			ReferenceType: controlplane.AttentionSignalReferenceTypeInitiative,
			ReferenceID:   "INIT-001",
			Title:         "Verification summary",
			Summary:       "Tests passed with one warning.",
		},
	}

	for _, signal := range signals {
		require.NoError(t, pdb.CreateAttentionSignal(signal))
		require.NotEmpty(t, signal.ID)
		require.False(t, signal.CreatedAt.IsZero())
		require.False(t, signal.UpdatedAt.IsZero())
	}

	loaded, err := pdb.GetAttentionSignal(signals[0].ID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, controlplane.AttentionSignalKindBlocker, loaded.Kind)
	require.Equal(t, controlplane.AttentionSignalReferenceTypeTask, loaded.ReferenceType)
	require.Equal(t, "TASK-001", loaded.ReferenceID)

	activeSignals, err := pdb.ListActiveAttentionSignals()
	require.NoError(t, err)
	require.Len(t, activeSignals, 4)

	count, err := pdb.CountActiveAttentionSignals()
	require.NoError(t, err)
	require.Equal(t, 4, count)

	resolved, err := pdb.ResolveAttentionSignal(signals[0].ID, "operator")
	require.NoError(t, err)
	require.Equal(t, controlplane.AttentionSignalStatusResolved, resolved.Status)
	require.NotNil(t, resolved.ResolvedAt)
	require.Equal(t, "operator", resolved.ResolvedBy)

	activeSignals, err = pdb.ListActiveAttentionSignals()
	require.NoError(t, err)
	require.Len(t, activeSignals, 3)

	count, err = pdb.CountActiveAttentionSignals()
	require.NoError(t, err)
	require.Equal(t, 3, count)
}
