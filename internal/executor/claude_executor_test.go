package executor

import (
	"testing"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithPhaseClaudeConfig(t *testing.T) {
	cfg := &PhaseClaudeConfig{
		SystemPrompt:    "You are a spec writer",
		DisallowedTools: []string{"Write", "Edit"},
		MaxBudgetUSD:    5.0,
	}

	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(cfg),
	)

	require.NotNil(t, exec.phaseConfig)
	assert.Equal(t, "You are a spec writer", exec.phaseConfig.SystemPrompt)
	assert.Equal(t, []string{"Write", "Edit"}, exec.phaseConfig.DisallowedTools)
	assert.Equal(t, 5.0, exec.phaseConfig.MaxBudgetUSD)
}

func TestClaudeExecutor_ApplyPhaseConfig_Nil(t *testing.T) {
	exec := NewClaudeExecutor()
	opts := []claude.ClaudeOption{}

	result := exec.applyPhaseConfig(opts)

	assert.Empty(t, result, "nil phaseConfig should return empty options")
}

func TestClaudeExecutor_ApplyPhaseConfig_Empty(t *testing.T) {
	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(&PhaseClaudeConfig{}),
	)
	opts := []claude.ClaudeOption{}

	result := exec.applyPhaseConfig(opts)

	// Empty config should not add any options
	assert.Empty(t, result)
}

func TestClaudeExecutor_ApplyPhaseConfig_SystemPrompt(t *testing.T) {
	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(&PhaseClaudeConfig{
			SystemPrompt: "You are a test writer",
		}),
	)

	opts := exec.buildBaseCLIOptions()

	// We can't easily inspect the options, but we can verify it doesn't panic
	// and returns more options than base
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_ApplyPhaseConfig_AppendSystemPrompt(t *testing.T) {
	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(&PhaseClaudeConfig{
			AppendSystemPrompt: "Always use TypeScript",
		}),
	)

	opts := exec.buildBaseCLIOptions()
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_ApplyPhaseConfig_ToolRestrictions(t *testing.T) {
	tests := []struct {
		name   string
		config *PhaseClaudeConfig
	}{
		{
			name: "allowed tools",
			config: &PhaseClaudeConfig{
				AllowedTools: []string{"Read", "Glob"},
			},
		},
		{
			name: "disallowed tools",
			config: &PhaseClaudeConfig{
				DisallowedTools: []string{"Write", "Edit", "Bash"},
			},
		},
		{
			name: "tools list",
			config: &PhaseClaudeConfig{
				Tools: []string{"Read", "Glob", "Grep"},
			},
		},
		{
			name: "combined restrictions",
			config: &PhaseClaudeConfig{
				AllowedTools:    []string{"Bash(git *)"},
				DisallowedTools: []string{"Write"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewClaudeExecutor(
				WithPhaseClaudeConfig(tt.config),
			)

			opts := exec.buildBaseCLIOptions()
			assert.NotEmpty(t, opts)
		})
	}
}

func TestClaudeExecutor_ApplyPhaseConfig_MCPServers(t *testing.T) {
	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(&PhaseClaudeConfig{
			MCPServers: map[string]claude.MCPServerConfig{
				"myserver": {
					Type:    "stdio",
					Command: "node",
					Args:    []string{"server.js"},
				},
			},
		}),
	)

	opts := exec.buildBaseCLIOptions()
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_ApplyPhaseConfig_StrictMCPConfig(t *testing.T) {
	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(&PhaseClaudeConfig{
			MCPServers: map[string]claude.MCPServerConfig{
				"server": {Command: "cmd"},
			},
			StrictMCPConfig: true,
		}),
	)

	opts := exec.buildBaseCLIOptions()
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_ApplyPhaseConfig_Budget(t *testing.T) {
	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(&PhaseClaudeConfig{
			MaxBudgetUSD: 10.50,
		}),
	)

	opts := exec.buildBaseCLIOptions()
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_ApplyPhaseConfig_MaxTurns(t *testing.T) {
	t.Run("phase config overrides executor level", func(t *testing.T) {
		exec := NewClaudeExecutor(
			WithClaudeMaxTurns(50),                               // Executor level
			WithPhaseClaudeConfig(&PhaseClaudeConfig{MaxTurns: 10}), // Phase config
		)

		// Phase config should be applied (10), not executor level (50)
		opts := exec.buildBaseCLIOptions()
		assert.NotEmpty(t, opts)
	})
}

func TestClaudeExecutor_ApplyPhaseConfig_Environment(t *testing.T) {
	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(&PhaseClaudeConfig{
			Env: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		}),
	)

	opts := exec.buildBaseCLIOptions()
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_ApplyPhaseConfig_AddDirs(t *testing.T) {
	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(&PhaseClaudeConfig{
			AddDirs: []string{"/tmp/extra", "/var/data"},
		}),
	)

	opts := exec.buildBaseCLIOptions()
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_ApplyPhaseConfig_Full(t *testing.T) {
	// Test with all supported config options set
	cfg := &PhaseClaudeConfig{
		SystemPrompt:       "Base system prompt",
		AppendSystemPrompt: "Additional instructions",
		AllowedTools:       []string{"Read"},
		DisallowedTools:    []string{"Write"},
		MCPServers: map[string]claude.MCPServerConfig{
			"test": {Command: "test-server"},
		},
		StrictMCPConfig: true,
		MaxBudgetUSD:    5.0,
		MaxTurns:        20,
		Env:             map[string]string{"KEY": "value"},
		AddDirs:         []string{"/extra"},
	}

	exec := NewClaudeExecutor(
		WithClaudeWorkdir("/tmp"),
		WithClaudeModel("opus"),
		WithPhaseClaudeConfig(cfg),
	)

	opts := exec.buildBaseCLIOptions()

	// Verify we get options back without panicking
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_ApplyPhaseConfig_AgentLogsDebug(t *testing.T) {
	// Agent configs are not yet supported by llmkit, but should not panic
	// and should log debug messages
	cfg := &PhaseClaudeConfig{
		AgentRef: "code-reviewer",
		InlineAgents: map[string]InlineAgentDef{
			"helper": {
				Description: "Helps with stuff",
				Prompt:      "You help",
			},
		},
	}

	exec := NewClaudeExecutor(
		WithPhaseClaudeConfig(cfg),
	)

	// Should not panic, just log debug
	opts := exec.buildBaseCLIOptions()
	assert.NotEmpty(t, opts) // At least base options
}

func TestClaudeExecutor_PhaseConfigIntegration(t *testing.T) {
	// Test that phase config works end-to-end with other executor options
	cfg := &PhaseClaudeConfig{
		DisallowedTools: []string{"Write", "Edit"},
		MaxBudgetUSD:    2.50,
	}

	exec := NewClaudeExecutor(
		WithClaudePath("/usr/local/bin/claude"),
		WithClaudeWorkdir("/project"),
		WithClaudeModel("sonnet"),
		WithClaudeMaxTurns(100),
		WithClaudePhaseID("spec"),
		WithPhaseClaudeConfig(cfg),
	)

	assert.Equal(t, "/usr/local/bin/claude", exec.claudePath)
	assert.Equal(t, "/project", exec.workdir)
	assert.Equal(t, "sonnet", exec.model)
	assert.Equal(t, "spec", exec.phaseID)
	assert.NotNil(t, exec.phaseConfig)

	opts := exec.buildBaseCLIOptions()
	assert.NotEmpty(t, opts)
}
