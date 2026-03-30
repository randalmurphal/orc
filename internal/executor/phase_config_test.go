package executor

import (
	"testing"

	llmkit "github.com/randalmurphal/llmkit/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePhaseRuntimeConfig(t *testing.T) {
	t.Run("empty string returns nil", func(t *testing.T) {
		cfg, err := ParsePhaseRuntimeConfig("")
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("parses nested shared and provider config", func(t *testing.T) {
		cfg, err := ParsePhaseRuntimeConfig(`{
			"shared": {
				"system_prompt": "You are a reviewer",
				"append_system_prompt": "Always use TypeScript",
				"allowed_tools": ["Read", "Glob"],
				"disallowed_tools": ["Write", "Edit"],
				"max_budget_usd": 5.5,
				"max_turns": 10,
				"env": {"FOO":"bar"},
				"mcp_servers": {"myserver": {"command":"node","args":["server.js"]}}
			},
			"providers": {
				"claude": {
					"skill_refs": ["python-style", "testing"],
					"agent_ref": "code-reviewer",
					"inline_agents": {
						"reviewer": {"description": "Reviews code", "prompt": "Review carefully"}
					},
					"hooks": {
						"PreToolUse": [{"matcher":"Edit","hooks":[{"type":"command","command":"test.sh"}]}]
					}
				},
				"codex": {
					"reasoning_effort": "high",
					"web_search_mode": "cached"
				}
			}
		}`)
		require.NoError(t, err)
		require.NotNil(t, cfg)

		assert.Equal(t, "You are a reviewer", cfg.Shared.SystemPrompt)
		assert.Equal(t, "Always use TypeScript", cfg.Shared.AppendSystemPrompt)
		assert.Equal(t, []string{"Read", "Glob"}, cfg.Shared.AllowedTools)
		assert.Equal(t, []string{"Write", "Edit"}, cfg.Shared.DisallowedTools)
		assert.Equal(t, 5.5, cfg.Shared.MaxBudgetUSD)
		assert.Equal(t, 10, cfg.Shared.MaxTurns)
		assert.Equal(t, "bar", cfg.Shared.Env["FOO"])
		assert.Equal(t, "node", cfg.Shared.MCPServers["myserver"].Command)

		require.NotNil(t, cfg.Providers.Claude)
		assert.Equal(t, []string{"python-style", "testing"}, cfg.Providers.Claude.SkillRefs)
		assert.Equal(t, "code-reviewer", cfg.Providers.Claude.AgentRef)
		require.Len(t, cfg.Providers.Claude.InlineAgents, 1)
		assert.Equal(t, "Reviews code", cfg.Providers.Claude.InlineAgents["reviewer"].Description)
		require.Contains(t, cfg.Providers.Claude.Hooks, "PreToolUse")
		assert.Len(t, cfg.Providers.Claude.Hooks["PreToolUse"], 1)

		require.NotNil(t, cfg.Providers.Codex)
		assert.Equal(t, "high", cfg.Providers.Codex.ReasoningEffort)
		assert.Equal(t, "cached", cfg.Providers.Codex.WebSearchMode)
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		_, err := ParsePhaseRuntimeConfig(`{invalid}`)
		assert.Error(t, err)
	})
}

func TestPhaseRuntimeConfig_Merge(t *testing.T) {
	base := &PhaseRuntimeConfig{
		Shared: llmkit.SharedRuntimeConfig{
			SystemPrompt:    "base prompt",
			DisallowedTools: []string{"Write"},
			MaxBudgetUSD:    5.0,
			MaxTurns:        50,
			Env:             map[string]string{"FOO": "bar"},
			MCPServers: map[string]llmkit.MCPServerConfig{
				"server1": {Command: "cmd1"},
			},
		},
		Providers: PhaseRuntimeProviderConfig{
			Claude: &llmkit.ClaudeRuntimeConfig{
				SkillRefs: []string{"python-style"},
				AgentRef:  "base-agent",
				Hooks: map[string][]llmkit.HookMatcher{
					"PreToolUse": {
						{Matcher: "Edit", Hooks: []llmkit.HookEntry{{Type: "command", Command: "a.sh"}}},
					},
				},
			},
		},
	}
	override := &PhaseRuntimeConfig{
		Shared: llmkit.SharedRuntimeConfig{
			SystemPrompt:    "override prompt",
			DisallowedTools: []string{"Edit", "Bash"},
			Env:             map[string]string{"BAZ": "qux"},
			MCPServers: map[string]llmkit.MCPServerConfig{
				"server2": {Command: "cmd2"},
			},
		},
		Providers: PhaseRuntimeProviderConfig{
			Claude: &llmkit.ClaudeRuntimeConfig{
				SkillRefs: []string{"testing"},
				Hooks: map[string][]llmkit.HookMatcher{
					"PreToolUse": {
						{Matcher: "Write", Hooks: []llmkit.HookEntry{{Type: "command", Command: "b.sh"}}},
					},
					"PostToolUse": {
						{Matcher: "Bash", Hooks: []llmkit.HookEntry{{Type: "command", Command: "c.sh"}}},
					},
				},
			},
			Codex: &llmkit.CodexRuntimeConfig{
				ReasoningEffort: "xhigh",
			},
		},
	}

	result := base.Merge(override)
	require.NotNil(t, result)

	assert.Equal(t, "override prompt", result.Shared.SystemPrompt)
	assert.Equal(t, []string{"Edit", "Bash"}, result.Shared.DisallowedTools)
	assert.Equal(t, 5.0, result.Shared.MaxBudgetUSD)
	assert.Equal(t, 50, result.Shared.MaxTurns)
	assert.Equal(t, "bar", result.Shared.Env["FOO"])
	assert.Equal(t, "qux", result.Shared.Env["BAZ"])
	assert.Equal(t, "cmd1", result.Shared.MCPServers["server1"].Command)
	assert.Equal(t, "cmd2", result.Shared.MCPServers["server2"].Command)

	require.NotNil(t, result.Providers.Claude)
	assert.Equal(t, "base-agent", result.Providers.Claude.AgentRef)
	assert.Equal(t, []string{"python-style", "testing"}, result.Providers.Claude.SkillRefs)
	assert.Len(t, result.Providers.Claude.Hooks["PreToolUse"], 2)
	assert.Len(t, result.Providers.Claude.Hooks["PostToolUse"], 1)

	require.NotNil(t, result.Providers.Codex)
	assert.Equal(t, "xhigh", result.Providers.Codex.ReasoningEffort)
}

func TestPhaseRuntimeConfig_IsEmpty(t *testing.T) {
	assert.True(t, (*PhaseRuntimeConfig)(nil).IsEmpty())
	assert.True(t, (&PhaseRuntimeConfig{}).IsEmpty())
	assert.False(t, (&PhaseRuntimeConfig{
		Shared: llmkit.SharedRuntimeConfig{SystemPrompt: "test"},
	}).IsEmpty())
	assert.False(t, (&PhaseRuntimeConfig{
		Providers: PhaseRuntimeProviderConfig{
			Claude: &llmkit.ClaudeRuntimeConfig{SkillRefs: []string{"python-style"}},
		},
	}).IsEmpty())
}

func TestPhaseRuntimeConfig_ToLLMKit(t *testing.T) {
	cfg := &PhaseRuntimeConfig{
		Shared: llmkit.SharedRuntimeConfig{
			SystemPrompt: "test",
			MaxTurns:     5,
		},
		Providers: PhaseRuntimeProviderConfig{
			Codex: &llmkit.CodexRuntimeConfig{ReasoningEffort: "high"},
		},
	}

	got := cfg.ToLLMKit()
	assert.Equal(t, "test", got.Shared.SystemPrompt)
	assert.Equal(t, 5, got.Shared.MaxTurns)
	require.NotNil(t, got.Providers.Codex)
	assert.Equal(t, "high", got.Providers.Codex.ReasoningEffort)
}
