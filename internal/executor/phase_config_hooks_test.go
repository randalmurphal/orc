package executor

import (
	"encoding/json"
	"testing"

	llmkit "github.com/randalmurphal/llmkit/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookMatcher_JSONRoundTrip(t *testing.T) {
	input := `{
		"matcher": "Edit|Write|MultiEdit",
		"hooks": [{"type": "command", "command": "bash /path/to/hook.sh"}]
	}`

	var hm HookMatcher
	require.NoError(t, json.Unmarshal([]byte(input), &hm))
	assert.Equal(t, "Edit|Write|MultiEdit", hm.Matcher)
	require.Len(t, hm.Hooks, 1)
	assert.Equal(t, "command", hm.Hooks[0].Type)
	assert.Equal(t, "bash /path/to/hook.sh", hm.Hooks[0].Command)

	data, err := json.Marshal(hm)
	require.NoError(t, err)

	var roundTripped HookMatcher
	require.NoError(t, json.Unmarshal(data, &roundTripped))
	assert.Equal(t, hm, roundTripped)
}

func TestPhaseRuntimeConfig_HooksField(t *testing.T) {
	cfg, err := ParsePhaseRuntimeConfig(`{
		"providers": {
			"claude": {
				"hooks": {
					"PreToolUse": [{
						"matcher": "Edit|Write",
						"hooks": [{"type": "command", "command": "python3 isolation.py"}]
					}],
					"PostToolUse": [{
						"matcher": "Bash",
						"hooks": [{"type": "command", "command": "bash verify.sh"}]
					}]
				}
			}
		}
	}`)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Providers.Claude)
	require.Contains(t, cfg.Providers.Claude.Hooks, "PreToolUse")
	require.Contains(t, cfg.Providers.Claude.Hooks, "PostToolUse")
	assert.Len(t, cfg.Providers.Claude.Hooks["PreToolUse"], 1)
	assert.Equal(t, "Edit|Write", cfg.Providers.Claude.Hooks["PreToolUse"][0].Matcher)
}

func TestPhaseRuntimeConfig_MergeHooks(t *testing.T) {
	base := &PhaseRuntimeConfig{
		Providers: PhaseRuntimeProviderConfig{
			Claude: &llmkit.ClaudeRuntimeConfig{
				Hooks: map[string][]llmkit.HookMatcher{
					"PreToolUse": {
						{Matcher: "Edit|Write", Hooks: []llmkit.HookEntry{{Type: "command", Command: "isolation.py"}}},
					},
				},
			},
		},
	}
	override := &PhaseRuntimeConfig{
		Providers: PhaseRuntimeProviderConfig{
			Claude: &llmkit.ClaudeRuntimeConfig{
				Hooks: map[string][]llmkit.HookMatcher{
					"PreToolUse": {
						{Matcher: "Bash", Hooks: []llmkit.HookEntry{{Type: "command", Command: "tdd.sh"}}},
					},
					"PostToolUse": {
						{Matcher: "Stop", Hooks: []llmkit.HookEntry{{Type: "command", Command: "verify.sh"}}},
					},
				},
			},
		},
	}

	result := base.Merge(override)
	require.NotNil(t, result.Providers.Claude)
	require.Contains(t, result.Providers.Claude.Hooks, "PreToolUse")
	require.Contains(t, result.Providers.Claude.Hooks, "PostToolUse")
	assert.Len(t, result.Providers.Claude.Hooks["PreToolUse"], 2)
	assert.Equal(t, "Edit|Write", result.Providers.Claude.Hooks["PreToolUse"][0].Matcher)
	assert.Equal(t, "Bash", result.Providers.Claude.Hooks["PreToolUse"][1].Matcher)
}
