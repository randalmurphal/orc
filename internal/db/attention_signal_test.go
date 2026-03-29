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

func TestResolveAttentionSignalsByTaskID(t *testing.T) {
	t.Parallel()

	pdb, err := OpenProjectInMemory()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pdb.Close()
	})

	// Create signals for two different tasks and one for a different reference type
	taskSignals := []*controlplane.PersistedAttentionSignal{
		{
			Kind:          controlplane.AttentionSignalKindBlocker,
			Status:        controlplane.AttentionSignalStatusBlocked,
			ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
			ReferenceID:   "TASK-010",
			Title:         "Blocked on TASK-010",
		},
		{
			Kind:          controlplane.AttentionSignalKindDecisionRequest,
			Status:        controlplane.AttentionSignalStatusActive,
			ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
			ReferenceID:   "TASK-010",
			Title:         "Decision needed for TASK-010",
		},
	}
	otherSignals := []*controlplane.PersistedAttentionSignal{
		{
			Kind:          controlplane.AttentionSignalKindBlocker,
			Status:        controlplane.AttentionSignalStatusBlocked,
			ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
			ReferenceID:   "TASK-020",
			Title:         "Blocked on TASK-020",
		},
		{
			Kind:          controlplane.AttentionSignalKindVerificationSummary,
			Status:        controlplane.AttentionSignalStatusActive,
			ReferenceType: controlplane.AttentionSignalReferenceTypeRun,
			ReferenceID:   "TASK-010",
			Title:         "Run signal referencing TASK-010 as run ID",
		},
	}

	for _, s := range taskSignals {
		require.NoError(t, pdb.CreateAttentionSignal(s))
	}
	for _, s := range otherSignals {
		require.NoError(t, pdb.CreateAttentionSignal(s))
	}

	// All 4 should be active
	count, err := pdb.CountActiveAttentionSignals()
	require.NoError(t, err)
	require.Equal(t, 4, count)

	// Resolve signals for TASK-010 — matches any signal with reference_id=TASK-010
	// regardless of reference_type, so both task signals and the run signal match.
	resolved, err := pdb.ResolveAttentionSignalsByTaskID("TASK-010")
	require.NoError(t, err)
	require.Equal(t, 3, resolved)

	// Verify the TASK-010 task signals are resolved
	for _, s := range taskSignals {
		loaded, loadErr := pdb.GetAttentionSignal(s.ID)
		require.NoError(t, loadErr)
		require.NotNil(t, loaded.ResolvedAt, "signal %s should be resolved", s.ID)
		require.Equal(t, "task-closed", loaded.ResolvedBy)
		require.Equal(t, controlplane.AttentionSignalStatusResolved, loaded.Status)
	}

	// The run signal with reference_id=TASK-010 should also be resolved
	runSignal, err := pdb.GetAttentionSignal(otherSignals[1].ID)
	require.NoError(t, err)
	require.NotNil(t, runSignal.ResolvedAt, "run signal referencing TASK-010 should be resolved")
	require.Equal(t, "task-closed", runSignal.ResolvedBy)

	// The other task's signal should still be active
	loaded, err := pdb.GetAttentionSignal(otherSignals[0].ID)
	require.NoError(t, err)
	require.Nil(t, loaded.ResolvedAt, "TASK-020 signal should remain active")

	// Only the TASK-020 signal should remain active
	activeCount, err := pdb.CountActiveAttentionSignals()
	require.NoError(t, err)
	require.Equal(t, 1, activeCount)

	// Resolving again for the same task should return 0 (already resolved)
	resolved, err = pdb.ResolveAttentionSignalsByTaskID("TASK-010")
	require.NoError(t, err)
	require.Equal(t, 0, resolved)

	// Resolving a non-existent task should return 0 without error
	resolved, err = pdb.ResolveAttentionSignalsByTaskID("TASK-999")
	require.NoError(t, err)
	require.Equal(t, 0, resolved)
}
