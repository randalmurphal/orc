package executor

import (
	"os"
	"path/filepath"
	"testing"

	llmkit "github.com/randalmurphal/llmkit/v2"
	"github.com/randalmurphal/llmkit/v2/claude"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithPhaseRuntimeConfig(t *testing.T) {
	cfg := &PhaseRuntimeConfig{
		Shared: llmkit.SharedRuntimeConfig{
			SystemPrompt:    "You are a spec writer",
			DisallowedTools: []string{"Write", "Edit"},
			MaxBudgetUSD:    5.0,
		},
	}

	exec := NewClaudeExecutor(WithPhaseRuntimeConfig(cfg))

	require.NotNil(t, exec.phaseConfig)
	assert.Equal(t, "You are a spec writer", exec.phaseConfig.Shared.SystemPrompt)
	assert.Equal(t, []string{"Write", "Edit"}, exec.phaseConfig.Shared.DisallowedTools)
	assert.Equal(t, 5.0, exec.phaseConfig.Shared.MaxBudgetUSD)
}

func TestClaudeExecutor_ApplyPhaseConfig_NilAndEmpty(t *testing.T) {
	t.Run("nil phase config returns unchanged options", func(t *testing.T) {
		exec := NewClaudeExecutor()
		opts := []claude.ClaudeOption{}
		result, err := exec.applyPhaseConfig(opts)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("empty phase config returns unchanged options", func(t *testing.T) {
		exec := NewClaudeExecutor(WithPhaseRuntimeConfig(&PhaseRuntimeConfig{}))
		opts := []claude.ClaudeOption{}
		result, err := exec.applyPhaseConfig(opts)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestClaudeExecutor_BuildBaseCLIOptions_WithSharedRuntimeConfig(t *testing.T) {
	exec := NewClaudeExecutor(
		WithClaudeWorkdir("/tmp"),
		WithClaudeModel("opus"),
		WithClaudeMaxTurns(50),
		WithPhaseRuntimeConfig(&PhaseRuntimeConfig{
			Shared: llmkit.SharedRuntimeConfig{
				SystemPrompt:       "Base system prompt",
				AppendSystemPrompt: "Additional instructions",
				AllowedTools:       []string{"Read"},
				DisallowedTools:    []string{"Write"},
				MCPServers: map[string]llmkit.MCPServerConfig{
					"test": {
						Type:    "stdio",
						Command: "test-server",
						Args:    []string{"--serve"},
						Env:     map[string]string{"FOO": "bar"},
					},
				},
				StrictMCPConfig: true,
				MaxBudgetUSD:    5.0,
				MaxTurns:        20,
				Env:             map[string]string{"KEY": "value"},
				AddDirs:         []string{"/extra"},
			},
			Providers: PhaseRuntimeProviderConfig{
				Claude: &llmkit.ClaudeRuntimeConfig{
					AgentRef: "code-reviewer",
					InlineAgents: map[string]llmkit.InlineAgentDef{
						"helper": {
							Description: "Helps with stuff",
							Prompt:      "You help",
						},
					},
				},
			},
		}),
	)

	opts, err := exec.buildBaseCLIOptions()
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_BuildBaseCLIOptions_WithToolRestrictionVariants(t *testing.T) {
	tests := []struct {
		name string
		cfg  *PhaseRuntimeConfig
	}{
		{
			name: "allowed tools",
			cfg: &PhaseRuntimeConfig{
				Shared: llmkit.SharedRuntimeConfig{AllowedTools: []string{"Read", "Glob"}},
			},
		},
		{
			name: "disallowed tools",
			cfg: &PhaseRuntimeConfig{
				Shared: llmkit.SharedRuntimeConfig{DisallowedTools: []string{"Write", "Edit", "Bash"}},
			},
		},
		{
			name: "tools list",
			cfg: &PhaseRuntimeConfig{
				Shared: llmkit.SharedRuntimeConfig{Tools: []string{"Read", "Glob", "Grep"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewClaudeExecutor(WithPhaseRuntimeConfig(tt.cfg))
			opts, err := exec.buildBaseCLIOptions()
			require.NoError(t, err)
			assert.NotEmpty(t, opts)
		})
	}
}

func TestClaudeExecutor_BuildBaseCLIOptions_LoadsPromptFilesViaLLMKit(t *testing.T) {
	root := t.TempDir()
	systemPath := filepath.Join(root, "system.txt")
	appendPath := filepath.Join(root, "append.txt")
	require.NoError(t, os.WriteFile(systemPath, []byte("base"), 0o644))
	require.NoError(t, os.WriteFile(appendPath, []byte("append"), 0o644))

	exec := NewClaudeExecutor(
		WithClaudeWorkdir(root),
		WithPhaseRuntimeConfig(&PhaseRuntimeConfig{
			Providers: PhaseRuntimeProviderConfig{
				Claude: &llmkit.ClaudeRuntimeConfig{
					SystemPromptFile:       "system.txt",
					AppendSystemPromptFile: "append.txt",
				},
			},
		}),
	)

	opts, err := exec.buildBaseCLIOptions()
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestClaudeExecutor_BuildBaseCLIOptions_ErrorsOnMissingPromptFile(t *testing.T) {
	exec := NewClaudeExecutor(
		WithClaudeWorkdir(t.TempDir()),
		WithPhaseRuntimeConfig(&PhaseRuntimeConfig{
			Providers: PhaseRuntimeProviderConfig{
				Claude: &llmkit.ClaudeRuntimeConfig{
					SystemPromptFile: "missing.txt",
				},
			},
		}),
	)

	_, err := exec.buildBaseCLIOptions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build claude runtime config")
	assert.Contains(t, err.Error(), "read prompt file")
}
