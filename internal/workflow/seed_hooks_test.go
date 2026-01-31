package workflow

import (
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for SC-7: SeedHookScripts populates GlobalDB with 3 built-in hook scripts
// Tests for SC-8: SeedSkills exists as infrastructure (seeds 0 skills)

func TestSeedHookScripts(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gdb.Close() })

	// First seed: should return 3
	seeded, err := SeedHookScripts(gdb)
	require.NoError(t, err)
	assert.Equal(t, 3, seeded)

	// Verify all 3 exist with correct IDs and is_builtin=true
	expectedIDs := []string{
		"orc-verify-completion",
		"orc-tdd-discipline",
		"orc-worktree-isolation",
	}

	for _, id := range expectedIDs {
		hs, err := gdb.GetHookScript(id)
		require.NoError(t, err, "GetHookScript(%s) should not error", id)
		require.NotNil(t, hs, "hook script %s should exist", id)
		assert.True(t, hs.IsBuiltin, "hook script %s should be builtin", id)
		assert.NotEmpty(t, hs.Content, "hook script %s should have content", id)
		assert.NotEmpty(t, hs.Name, "hook script %s should have name", id)
	}
}

func TestSeedHookScripts_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gdb.Close() })

	// First seed
	seeded1, err := SeedHookScripts(gdb)
	require.NoError(t, err)
	assert.Equal(t, 3, seeded1)

	// Second seed: should return 0 (all already exist)
	seeded2, err := SeedHookScripts(gdb)
	require.NoError(t, err)
	assert.Equal(t, 0, seeded2)

	// Verify no duplicates
	list, err := gdb.ListHookScripts()
	require.NoError(t, err)
	// Should have exactly 3, not 6
	builtinCount := 0
	for _, hs := range list {
		if hs.IsBuiltin {
			builtinCount++
		}
	}
	assert.Equal(t, 3, builtinCount)
}

func TestSeedSkills(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gdb.Close() })

	// SeedSkills should exist and return 0 (infrastructure only, no actual skills)
	seeded, err := SeedSkills(gdb)
	require.NoError(t, err)
	assert.Equal(t, 0, seeded)
}
