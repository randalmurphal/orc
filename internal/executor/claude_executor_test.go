package executor

import (
	"os"
	"path/filepath"
	"testing"

	llmkit "github.com/randalmurphal/llmkit/v2"
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

func TestClaudeExecutor_BuildClientConfig_NilAndEmpty(t *testing.T) {
	t.Run("nil phase config returns default config", func(t *testing.T) {
		exec := NewClaudeExecutor()
		result, err := exec.buildClientConfig()
		require.NoError(t, err)
		assert.Equal(t, ProviderClaude, result.Provider)
	})

	t.Run("empty phase config returns default config", func(t *testing.T) {
		exec := NewClaudeExecutor(WithPhaseRuntimeConfig(&PhaseRuntimeConfig{}))
		result, err := exec.buildClientConfig()
		require.NoError(t, err)
		assert.Equal(t, ProviderClaude, result.Provider)
	})
}

func TestClaudeExecutor_BuildClientConfig_WithSharedRuntimeConfig(t *testing.T) {
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

	cfg, err := exec.buildClientConfig()
	require.NoError(t, err)
	assert.Equal(t, ProviderClaude, cfg.Provider)
	assert.Equal(t, "opus", cfg.Model)
	assert.Equal(t, "/tmp", cfg.WorkDir)
	assert.Equal(t, []string{"Read"}, cfg.AllowedTools)
	assert.Equal(t, []string{"Write"}, cfg.DisallowedTools)
	assert.Equal(t, 5.0, cfg.MaxBudgetUSD)
}

func TestClaudeExecutor_BuildClientConfig_WithToolRestrictionVariants(t *testing.T) {
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
			cfg, err := exec.buildClientConfig()
			require.NoError(t, err)
			assert.Equal(t, ProviderClaude, cfg.Provider)
		})
	}
}

func TestClaudeExecutor_BuildClientConfig_LoadsPromptFilesViaLLMKit(t *testing.T) {
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

	cfg, err := exec.buildClientConfig()
	require.NoError(t, err)
	assert.Contains(t, cfg.SystemPrompt, "base")
	assert.Contains(t, cfg.SystemPrompt, "append")
}

func TestClaudeExecutor_BuildClientConfig_ErrorsOnMissingPromptFile(t *testing.T) {
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

	_, err := exec.buildClientConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "build claude runtime config")
	assert.Contains(t, err.Error(), "read prompt file")
}
