package db

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for SC-4: hook_scripts table CRUD operations
// Tests for SC-6: Migration creates both tables

func TestHookScripts_CRUD(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gdb, err := OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gdb.Close() })

	t.Run("save and get", func(t *testing.T) {
		hs := &HookScript{
			ID:          "orc-verify-completion",
			Name:        "Verify Completion",
			Description: "Verifies phase completion output",
			Content:     "#!/bin/bash\necho verify",
			EventType:   "Stop",
			IsBuiltin:   true,
		}

		err := gdb.SaveHookScript(hs)
		require.NoError(t, err)

		got, err := gdb.GetHookScript("orc-verify-completion")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "orc-verify-completion", got.ID)
		assert.Equal(t, "Verify Completion", got.Name)
		assert.Equal(t, "#!/bin/bash\necho verify", got.Content)
		assert.Equal(t, "Stop", got.EventType)
		assert.True(t, got.IsBuiltin)
		assert.NotEmpty(t, got.CreatedAt)
		assert.NotEmpty(t, got.UpdatedAt)
	})

	t.Run("get missing returns nil nil", func(t *testing.T) {
		got, err := gdb.GetHookScript("nonexistent")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("upsert on conflict", func(t *testing.T) {
		hs := &HookScript{
			ID:          "orc-upsert-test",
			Name:        "Original",
			Description: "Original description",
			Content:     "#!/bin/bash\noriginal",
			EventType:   "PreToolUse",
			IsBuiltin:   true,
		}
		err := gdb.SaveHookScript(hs)
		require.NoError(t, err)

		// Update with same ID
		hs.Name = "Updated"
		hs.Content = "#!/bin/bash\nupdated"
		err = gdb.SaveHookScript(hs)
		require.NoError(t, err)

		got, err := gdb.GetHookScript("orc-upsert-test")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "Updated", got.Name)
		assert.Equal(t, "#!/bin/bash\nupdated", got.Content)
	})

	t.Run("list returns all", func(t *testing.T) {
		// Already have 2 from previous subtests
		list, err := gdb.ListHookScripts()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 2)
	})

	t.Run("delete non-builtin", func(t *testing.T) {
		hs := &HookScript{
			ID:        "user-custom-hook",
			Name:      "Custom",
			Content:   "#!/bin/bash\ncustom",
			EventType: "PreToolUse",
			IsBuiltin: false,
		}
		err := gdb.SaveHookScript(hs)
		require.NoError(t, err)

		err = gdb.DeleteHookScript("user-custom-hook")
		require.NoError(t, err)

		got, err := gdb.GetHookScript("user-custom-hook")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("is_builtin flag preserved", func(t *testing.T) {
		for _, builtin := range []bool{true, false} {
			hs := &HookScript{
				ID:        "builtin-test-" + boolStr(builtin),
				Name:      "test",
				Content:   "#!/bin/bash\ntest",
				EventType: "PreToolUse",
				IsBuiltin: builtin,
			}
			err := gdb.SaveHookScript(hs)
			require.NoError(t, err)

			got, err := gdb.GetHookScript(hs.ID)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, builtin, got.IsBuiltin)
		}
	})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
