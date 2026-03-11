package storage

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/randalmurphal/orc/internal/controlplane"
)

func TestAttentionSignalBackendRoundTrip(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	signal := &controlplane.PersistedAttentionSignal{
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        controlplane.AttentionSignalStatusFailed,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   "TASK-001",
		Title:         "Failed task",
		Summary:       "The review phase failed.",
	}

	require.NoError(t, backend.SaveAttentionSignal(signal))
	require.NotEmpty(t, signal.ID)

	loaded, err := backend.LoadAttentionSignal(signal.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, controlplane.AttentionSignalStatusFailed, loaded.Status)
	require.Equal(t, "TASK-001", loaded.ReferenceID)

	activeSignals, err := backend.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Len(t, activeSignals, 1)

	count, err := backend.CountActiveAttentionSignals()
	require.NoError(t, err)
	require.Equal(t, 1, count)

	signal.Summary = "The review phase failed after retry."
	require.NoError(t, backend.SaveAttentionSignal(signal))

	activeSignals, err = backend.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Len(t, activeSignals, 1)
	require.Equal(t, "The review phase failed after retry.", activeSignals[0].Summary)

	resolved, err := backend.ResolveAttentionSignal(signal.ID, "operator")
	require.NoError(t, err)
	require.Equal(t, controlplane.AttentionSignalStatusResolved, resolved.Status)

	activeSignals, err = backend.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Empty(t, activeSignals)

	count, err = backend.CountActiveAttentionSignals()
	require.NoError(t, err)
	require.Zero(t, count)
}
