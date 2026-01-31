package executor

import (
	"testing"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePhaseClaudeConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNil  bool
		wantErr  bool
		validate func(t *testing.T, cfg *PhaseClaudeConfig)
	}{
		{
			name:    "empty string returns nil",
			input:   "",
			wantNil: true,
		},
		{
			name:  "system prompt",
			input: `{"system_prompt": "You are a reviewer"}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				assert.Equal(t, "You are a reviewer", cfg.SystemPrompt)
			},
		},
		{
			name:  "append system prompt",
			input: `{"append_system_prompt": "Always use TypeScript"}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				assert.Equal(t, "Always use TypeScript", cfg.AppendSystemPrompt)
			},
		},
		{
			name:  "disallowed tools",
			input: `{"disallowed_tools": ["Write", "Edit"]}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				assert.Equal(t, []string{"Write", "Edit"}, cfg.DisallowedTools)
			},
		},
		{
			name:  "allowed tools",
			input: `{"allowed_tools": ["Read", "Glob"]}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				assert.Equal(t, []string{"Read", "Glob"}, cfg.AllowedTools)
			},
		},
		{
			name:  "max budget",
			input: `{"max_budget_usd": 5.50}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				assert.Equal(t, 5.50, cfg.MaxBudgetUSD)
			},
		},
		{
			name:  "max turns",
			input: `{"max_turns": 10}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				assert.Equal(t, 10, cfg.MaxTurns)
			},
		},
		{
			name:  "skill refs",
			input: `{"skill_refs": ["python-style", "testing"]}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				assert.Equal(t, []string{"python-style", "testing"}, cfg.SkillRefs)
			},
		},
		{
			name:  "agent ref",
			input: `{"agent_ref": "code-reviewer"}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				assert.Equal(t, "code-reviewer", cfg.AgentRef)
			},
		},
		{
			name:  "inline agents",
			input: `{"inline_agents": {"reviewer": {"description": "Reviews code", "prompt": "You are a reviewer"}}}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				require.Len(t, cfg.InlineAgents, 1)
				agent := cfg.InlineAgents["reviewer"]
				assert.Equal(t, "Reviews code", agent.Description)
				assert.Equal(t, "You are a reviewer", agent.Prompt)
			},
		},
		{
			name:  "hooks field",
			input: `{"hooks": {"PreToolUse": [{"matcher": "Edit", "hooks": [{"type": "command", "command": "test.sh"}]}]}}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				require.Contains(t, cfg.Hooks, "PreToolUse")
				assert.Len(t, cfg.Hooks["PreToolUse"], 1)
			},
		},
		{
			name:  "mcp servers",
			input: `{"mcp_servers": {"myserver": {"command": "node", "args": ["server.js"]}}}`,
			validate: func(t *testing.T, cfg *PhaseClaudeConfig) {
				require.Len(t, cfg.MCPServers, 1)
				server := cfg.MCPServers["myserver"]
				assert.Equal(t, "node", server.Command)
				assert.Equal(t, []string{"server.js"}, server.Args)
			},
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParsePhaseClaudeConfig(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, cfg)
				return
			}

			require.NotNil(t, cfg)
			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestPhaseClaudeConfig_Merge(t *testing.T) {
	t.Run("nil base returns override", func(t *testing.T) {
		var base *PhaseClaudeConfig
		override := &PhaseClaudeConfig{SystemPrompt: "override"}
		result := base.Merge(override)
		assert.Equal(t, "override", result.SystemPrompt)
	})

	t.Run("nil override returns base", func(t *testing.T) {
		base := &PhaseClaudeConfig{SystemPrompt: "base"}
		result := base.Merge(nil)
		assert.Equal(t, "base", result.SystemPrompt)
	})

	t.Run("override takes precedence for strings", func(t *testing.T) {
		base := &PhaseClaudeConfig{
			SystemPrompt: "base prompt",
			AgentRef:     "base-agent",
		}
		override := &PhaseClaudeConfig{
			SystemPrompt: "override prompt",
		}
		result := base.Merge(override)
		assert.Equal(t, "override prompt", result.SystemPrompt)
		assert.Equal(t, "base-agent", result.AgentRef) // Preserved from base
	})

	t.Run("override replaces slices", func(t *testing.T) {
		base := &PhaseClaudeConfig{
			DisallowedTools: []string{"Write"},
		}
		override := &PhaseClaudeConfig{
			DisallowedTools: []string{"Edit", "Bash"},
		}
		result := base.Merge(override)
		assert.Equal(t, []string{"Edit", "Bash"}, result.DisallowedTools)
	})

	t.Run("skills are appended", func(t *testing.T) {
		base := &PhaseClaudeConfig{
			SkillRefs: []string{"python-style"},
		}
		override := &PhaseClaudeConfig{
			SkillRefs: []string{"testing"},
		}
		result := base.Merge(override)
		assert.Equal(t, []string{"python-style", "testing"}, result.SkillRefs)
	})

	t.Run("maps are merged", func(t *testing.T) {
		base := &PhaseClaudeConfig{
			Env: map[string]string{"FOO": "bar"},
			MCPServers: map[string]claude.MCPServerConfig{
				"server1": {Command: "cmd1"},
			},
		}
		override := &PhaseClaudeConfig{
			Env: map[string]string{"BAZ": "qux"},
			MCPServers: map[string]claude.MCPServerConfig{
				"server2": {Command: "cmd2"},
			},
		}
		result := base.Merge(override)
		assert.Equal(t, "bar", result.Env["FOO"])
		assert.Equal(t, "qux", result.Env["BAZ"])
		assert.Equal(t, "cmd1", result.MCPServers["server1"].Command)
		assert.Equal(t, "cmd2", result.MCPServers["server2"].Command)
	})

	t.Run("hooks merge appends per event key", func(t *testing.T) {
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
				"PostToolUse": {
					{Matcher: "Bash", Hooks: []HookEntry{{Type: "command", Command: "c.sh"}}},
				},
			},
		}
		result := base.Merge(override)
		assert.Len(t, result.Hooks["PreToolUse"], 2)
		assert.Len(t, result.Hooks["PostToolUse"], 1)
	})

	t.Run("budget preserved from base if not overridden", func(t *testing.T) {
		base := &PhaseClaudeConfig{MaxBudgetUSD: 5.0}
		override := &PhaseClaudeConfig{MaxTurns: 10}
		result := base.Merge(override)
		assert.Equal(t, 5.0, result.MaxBudgetUSD)
		assert.Equal(t, 10, result.MaxTurns)
	})
}

func TestPhaseClaudeConfig_IsEmpty(t *testing.T) {
	t.Run("nil is empty", func(t *testing.T) {
		var cfg *PhaseClaudeConfig
		assert.True(t, cfg.IsEmpty())
	})

	t.Run("zero value is empty", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{}
		assert.True(t, cfg.IsEmpty())
	})

	t.Run("with system prompt is not empty", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{SystemPrompt: "test"}
		assert.False(t, cfg.IsEmpty())
	})

	t.Run("with disallowed tools is not empty", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{DisallowedTools: []string{"Write"}}
		assert.False(t, cfg.IsEmpty())
	})

	t.Run("with budget is not empty", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{MaxBudgetUSD: 1.0}
		assert.False(t, cfg.IsEmpty())
	})
}

func TestPhaseClaudeConfig_JSON(t *testing.T) {
	t.Run("nil returns empty", func(t *testing.T) {
		var cfg *PhaseClaudeConfig
		assert.Equal(t, "", cfg.JSON())
	})

	t.Run("empty returns empty", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{}
		assert.Equal(t, "", cfg.JSON())
	})

	t.Run("with values returns json", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{
			DisallowedTools: []string{"Write", "Edit"},
		}
		json := cfg.JSON()
		assert.Contains(t, json, "disallowed_tools")
		assert.Contains(t, json, "Write")
	})
}

func TestPhaseClaudeConfig_InlineAgentsJSON(t *testing.T) {
	t.Run("nil returns empty", func(t *testing.T) {
		var cfg *PhaseClaudeConfig
		assert.Equal(t, "", cfg.InlineAgentsJSON())
	})

	t.Run("no agents returns empty", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{}
		assert.Equal(t, "", cfg.InlineAgentsJSON())
	})

	t.Run("with agents returns json", func(t *testing.T) {
		cfg := &PhaseClaudeConfig{
			InlineAgents: map[string]InlineAgentDef{
				"reviewer": {
					Description: "Reviews code",
					Prompt:      "You are a reviewer",
				},
			},
		}
		json := cfg.InlineAgentsJSON()
		assert.Contains(t, json, "reviewer")
		assert.Contains(t, json, "Reviews code")
	})
}
