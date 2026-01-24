package executor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestSettings creates a .claude/settings.json file with the given agents.
func createTestSettings(t *testing.T, projectRoot string, agents []map[string]any) {
	t.Helper()

	claudeDir := filepath.Join(projectRoot, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	settings := map[string]any{
		"extensions": map[string]any{
			"agents": agents,
		},
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "settings.json"),
		data,
		0644,
	))
}

func TestAgentResolver_ResolveAgentConfig(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		resolver := NewAgentResolver(t.TempDir(), t.TempDir())
		err := resolver.ResolveAgentConfig(nil)

		require.NoError(t, err)
	})

	t.Run("empty agent ref returns nil", func(t *testing.T) {
		resolver := NewAgentResolver(t.TempDir(), t.TempDir())
		cfg := &PhaseClaudeConfig{}
		err := resolver.ResolveAgentConfig(cfg)

		require.NoError(t, err)
	})

	t.Run("agent not found returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestSettings(t, tmpDir, []map[string]any{})

		resolver := NewAgentResolver(tmpDir, tmpDir)
		cfg := &PhaseClaudeConfig{AgentRef: "nonexistent"}

		err := resolver.ResolveAgentConfig(cfg)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent")
	})

	t.Run("merges agent system prompt", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestSettings(t, tmpDir, []map[string]any{
			{
				"name":        "reviewer",
				"description": "Code reviewer",
				"prompt":      "You are a code reviewer",
			},
		})

		resolver := NewAgentResolver(tmpDir, tmpDir)
		cfg := &PhaseClaudeConfig{AgentRef: "reviewer"}

		err := resolver.ResolveAgentConfig(cfg)

		require.NoError(t, err)
		assert.Equal(t, "You are a code reviewer", cfg.SystemPrompt)
	})

	t.Run("config prompt takes precedence over agent", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestSettings(t, tmpDir, []map[string]any{
			{
				"name":        "reviewer",
				"description": "Code reviewer",
				"prompt":      "Agent prompt",
			},
		})

		resolver := NewAgentResolver(tmpDir, tmpDir)
		cfg := &PhaseClaudeConfig{
			AgentRef:     "reviewer",
			SystemPrompt: "Config prompt",
		}

		err := resolver.ResolveAgentConfig(cfg)

		require.NoError(t, err)
		assert.Equal(t, "Config prompt", cfg.SystemPrompt)
	})

	t.Run("merges agent tool restrictions", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestSettings(t, tmpDir, []map[string]any{
			{
				"name":        "safe-agent",
				"description": "Safe agent",
				"tools": map[string]any{
					"allow": []string{"Read", "Glob"},
					"deny":  []string{"Bash", "Write"},
				},
			},
		})

		resolver := NewAgentResolver(tmpDir, tmpDir)
		cfg := &PhaseClaudeConfig{AgentRef: "safe-agent"}

		err := resolver.ResolveAgentConfig(cfg)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Read", "Glob"}, cfg.AllowedTools)
		assert.ElementsMatch(t, []string{"Bash", "Write"}, cfg.DisallowedTools)
	})

	t.Run("config tools take precedence over agent", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestSettings(t, tmpDir, []map[string]any{
			{
				"name":        "safe-agent",
				"description": "Safe agent",
				"tools": map[string]any{
					"deny": []string{"Bash"},
				},
			},
		})

		resolver := NewAgentResolver(tmpDir, tmpDir)
		cfg := &PhaseClaudeConfig{
			AgentRef:        "safe-agent",
			DisallowedTools: []string{"Write", "Edit"},
		}

		err := resolver.ResolveAgentConfig(cfg)

		require.NoError(t, err)
		// Config value preserved, agent value not applied
		assert.ElementsMatch(t, []string{"Write", "Edit"}, cfg.DisallowedTools)
	})

	t.Run("appends agent skill refs", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestSettings(t, tmpDir, []map[string]any{
			{
				"name":        "python-dev",
				"description": "Python developer",
				"skill_refs":  []string{"python-style", "testing"},
			},
		})

		resolver := NewAgentResolver(tmpDir, tmpDir)
		cfg := &PhaseClaudeConfig{
			AgentRef:  "python-dev",
			SkillRefs: []string{"debugging"},
		}

		err := resolver.ResolveAgentConfig(cfg)

		require.NoError(t, err)
		// Both config and agent skills should be present
		assert.Contains(t, cfg.SkillRefs, "debugging")
		assert.Contains(t, cfg.SkillRefs, "python-style")
		assert.Contains(t, cfg.SkillRefs, "testing")
	})

	t.Run("deduplicates skill refs", func(t *testing.T) {
		tmpDir := t.TempDir()
		createTestSettings(t, tmpDir, []map[string]any{
			{
				"name":        "python-dev",
				"description": "Python developer",
				"skill_refs":  []string{"python-style", "testing"},
			},
		})

		resolver := NewAgentResolver(tmpDir, tmpDir)
		cfg := &PhaseClaudeConfig{
			AgentRef:  "python-dev",
			SkillRefs: []string{"python-style"}, // Already have this
		}

		err := resolver.ResolveAgentConfig(cfg)

		require.NoError(t, err)
		// Should not have duplicates
		count := 0
		for _, s := range cfg.SkillRefs {
			if s == "python-style" {
				count++
			}
		}
		assert.Equal(t, 1, count, "python-style should appear only once")
	})
}

func TestAgentResolver_ResolveAgentForConfig(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		resolver := NewAgentResolver(t.TempDir(), t.TempDir())
		err := resolver.ResolveAgentForConfig(nil)

		require.NoError(t, err)
	})

	t.Run("resolves agent and loads skills", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create agent with skill ref
		createTestSettings(t, tmpDir, []map[string]any{
			{
				"name":        "python-dev",
				"description": "Python developer",
				"skill_refs":  []string{"python-style"},
			},
		})

		// Create the skill
		skillDir := filepath.Join(tmpDir, "skills", "python-style")
		require.NoError(t, os.MkdirAll(skillDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte("---\nname: python-style\ndescription: Python standards\n---\nUse snake_case"),
			0644,
		))

		resolver := NewAgentResolver(tmpDir, tmpDir)
		cfg := &PhaseClaudeConfig{AgentRef: "python-dev"}

		err := resolver.ResolveAgentForConfig(cfg)

		require.NoError(t, err)
		// Skills should have been loaded
		assert.Contains(t, cfg.AppendSystemPrompt, "snake_case")
	})
}

func TestResolveAgentConfigSimple(t *testing.T) {
	tmpDir := t.TempDir()
	createTestSettings(t, tmpDir, []map[string]any{
		{
			"name":        "simple-agent",
			"description": "Simple agent",
			"prompt":      "Simple prompt",
		},
	})

	cfg := &PhaseClaudeConfig{AgentRef: "simple-agent"}

	err := ResolveAgentConfigSimple(tmpDir, tmpDir, cfg)

	require.NoError(t, err)
	assert.Equal(t, "Simple prompt", cfg.SystemPrompt)
}
