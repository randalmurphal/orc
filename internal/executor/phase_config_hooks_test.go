package executor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for SC-1: PhaseClaudeConfig has Hooks field, CaptureHookEvents removed
// Tests for SC-2: Merge() appends Hooks matchers per event key, IsEmpty() checks Hooks
// Tests for SC-3: HookMatcher and HookEntry types with correct JSON tags

func TestHookMatcher_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("matches Claude Code hook format", func(t *testing.T) {
		// The JSON format Claude Code expects for hooks in settings.json:
		// { "matcher": "Edit|Write", "hooks": [{"type": "command", "command": "bash /path/to/hook.sh"}] }
		input := `{
			"matcher": "Edit|Write|MultiEdit",
			"hooks": [
				{"type": "command", "command": "bash /path/to/hook.sh"}
			]
		}`

		var hm HookMatcher
		err := json.Unmarshal([]byte(input), &hm)
		require.NoError(t, err)

		assert.Equal(t, "Edit|Write|MultiEdit", hm.Matcher)
		require.Len(t, hm.Hooks, 1)
		assert.Equal(t, "command", hm.Hooks[0].Type)
		assert.Equal(t, "bash /path/to/hook.sh", hm.Hooks[0].Command)

		// Round-trip: marshal back and verify structure
		data, err := json.Marshal(hm)
		require.NoError(t, err)

		var roundTripped HookMatcher
		err = json.Unmarshal(data, &roundTripped)
		require.NoError(t, err)
		assert.Equal(t, hm, roundTripped)
	})

	t.Run("multiple hooks per matcher", func(t *testing.T) {
		input := `{
			"matcher": "Edit|Write",
			"hooks": [
				{"type": "command", "command": "python3 isolation.py"},
				{"type": "command", "command": "bash tdd-check.sh"}
			]
		}`

		var hm HookMatcher
		err := json.Unmarshal([]byte(input), &hm)
		require.NoError(t, err)
		assert.Len(t, hm.Hooks, 2)
	})

	t.Run("malformed JSON returns error", func(t *testing.T) {
		var hm HookMatcher
		err := json.Unmarshal([]byte(`{invalid`), &hm)
		assert.Error(t, err)
	})
}

func TestHookEntry_JSONTags(t *testing.T) {
	t.Parallel()

	entry := HookEntry{
		Type:    "command",
		Command: "bash /path/to/script.sh",
	}

	data, err := json.Marshal(entry)
	require.NoError(t, err)

	// Verify the JSON keys match Claude Code's format
	var raw map[string]string
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)
	assert.Equal(t, "command", raw["type"])
	assert.Equal(t, "bash /path/to/script.sh", raw["command"])
}

func TestPhaseClaudeConfig_HooksField(t *testing.T) {
	t.Parallel()

	t.Run("parse config with Hooks field", func(t *testing.T) {
		input := `{
			"hooks": {
				"PreToolUse": [
					{
						"matcher": "Edit|Write",
						"hooks": [{"type": "command", "command": "python3 isolation.py"}]
					}
				],
				"PostToolUse": [
					{
						"matcher": "Bash",
						"hooks": [{"type": "command", "command": "bash verify.sh"}]
					}
				]
			}
		}`

		cfg, err := ParsePhaseClaudeConfig(input)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		require.Contains(t, cfg.Hooks, "PreToolUse")
		require.Contains(t, cfg.Hooks, "PostToolUse")
		assert.Len(t, cfg.Hooks["PreToolUse"], 1)
		assert.Equal(t, "Edit|Write", cfg.Hooks["PreToolUse"][0].Matcher)
	})

	t.Run("CaptureHookEvents field does not exist", func(t *testing.T) {
		// SC-1: CaptureHookEvents must be removed.
		// This test verifies that parsing old-format JSON with capture_hook_events
		// does NOT populate anything — the field should not exist on the struct.
		input := `{"capture_hook_events": ["PreToolUse"]}`
		cfg, err := ParsePhaseClaudeConfig(input)
		require.NoError(t, err)
		// If CaptureHookEvents still exists, this would be non-empty.
		// With the field removed, json.Unmarshal ignores unknown keys.
		assert.True(t, cfg.IsEmpty(), "config with only old capture_hook_events should be empty (field removed)")
	})
}

func TestPhaseClaudeConfig_MergeHooks(t *testing.T) {
	t.Parallel()

	t.Run("same event key concatenates matchers", func(t *testing.T) {
		base := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{
				"PreToolUse": {
					{Matcher: "Edit|Write", Hooks: []HookEntry{{Type: "command", Command: "isolation.py"}}},
				},
			},
		}
		override := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{
				"PreToolUse": {
					{Matcher: "Bash", Hooks: []HookEntry{{Type: "command", Command: "tdd.sh"}}},
				},
			},
		}

		result := base.Merge(override)
		require.Contains(t, result.Hooks, "PreToolUse")
		// Must be concatenated (2 matchers), NOT replaced
		assert.Len(t, result.Hooks["PreToolUse"], 2)
		assert.Equal(t, "Edit|Write", result.Hooks["PreToolUse"][0].Matcher)
		assert.Equal(t, "Bash", result.Hooks["PreToolUse"][1].Matcher)
	})

	t.Run("different event keys both preserved", func(t *testing.T) {
		base := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{
				"PreToolUse": {
					{Matcher: "Edit", Hooks: []HookEntry{{Type: "command", Command: "a.sh"}}},
				},
			},
		}
		override := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{
				"PostToolUse": {
					{Matcher: "Bash", Hooks: []HookEntry{{Type: "command", Command: "b.sh"}}},
				},
			},
		}

		result := base.Merge(override)
		assert.Contains(t, result.Hooks, "PreToolUse")
		assert.Contains(t, result.Hooks, "PostToolUse")
	})

	t.Run("nil hooks on base, non-nil on override", func(t *testing.T) {
		base := &PhaseClaudeConfig{SystemPrompt: "test"}
		override := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{
				"PreToolUse": {
					{Matcher: "Edit", Hooks: []HookEntry{{Type: "command", Command: "a.sh"}}},
				},
			},
		}

		result := base.Merge(override)
		require.Contains(t, result.Hooks, "PreToolUse")
		assert.Len(t, result.Hooks["PreToolUse"], 1)
		assert.Equal(t, "test", result.SystemPrompt) // base preserved
	})

	t.Run("non-nil hooks on base, nil on override", func(t *testing.T) {
		base := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{
				"PreToolUse": {
					{Matcher: "Edit", Hooks: []HookEntry{{Type: "command", Command: "a.sh"}}},
				},
			},
		}
		override := &PhaseClaudeConfig{SystemPrompt: "override"}

		result := base.Merge(override)
		require.Contains(t, result.Hooks, "PreToolUse")
		assert.Len(t, result.Hooks["PreToolUse"], 1)
	})

	t.Run("does not modify original configs", func(t *testing.T) {
		base := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{
				"PreToolUse": {
					{Matcher: "Edit", Hooks: []HookEntry{{Type: "command", Command: "a.sh"}}},
				},
			},
		}
		override := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{
				"PreToolUse": {
					{Matcher: "Write", Hooks: []HookEntry{{Type: "command", Command: "b.sh"}}},
				},
			},
		}

		_ = base.Merge(override)

		// Originals must be unmodified
		assert.Len(t, base.Hooks["PreToolUse"], 1)
		assert.Len(t, override.Hooks["PreToolUse"], 1)
	})
}

func TestPhaseClaudeConfig_IsEmpty_Hooks(t *testing.T) {
	t.Parallel()

	t.Run("with hooks is not empty", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{
				"PreToolUse": {{Matcher: "Edit", Hooks: []HookEntry{{Type: "command", Command: "x"}}}},
			},
		}
		assert.False(t, cfg.IsEmpty())
	})

	t.Run("with empty hooks map is empty", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{
			Hooks: map[string][]HookMatcher{},
		}
		assert.True(t, cfg.IsEmpty())
	})
}
