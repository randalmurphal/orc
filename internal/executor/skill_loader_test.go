package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillLoader_LoadSkillsContent(t *testing.T) {
	t.Run("empty refs returns empty", func(t *testing.T) {
		loader := NewSkillLoader(t.TempDir())
		content, err := loader.LoadSkillsContent(nil)

		require.NoError(t, err)
		assert.Empty(t, content)
	})

	t.Run("loads skill content", func(t *testing.T) {
		// Setup temp skill
		tmpDir := t.TempDir()
		skillDir := filepath.Join(tmpDir, "skills", "python-style")
		require.NoError(t, os.MkdirAll(skillDir, 0755))

		skillContent := `---
name: python-style
description: Python coding standards
---
# Python Style Guide

Use snake_case for variables.
Use type hints for all functions.`

		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte(skillContent),
			0644,
		))

		loader := NewSkillLoader(tmpDir)
		content, err := loader.LoadSkillsContent([]string{"python-style"})

		require.NoError(t, err)
		assert.Contains(t, content, "## Skill: python-style")
		assert.Contains(t, content, "Python coding standards")
		assert.Contains(t, content, "snake_case")
		assert.Contains(t, content, "type hints")
	})

	t.Run("loads multiple skills", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create two skills
		for _, name := range []string{"skill-a", "skill-b"} {
			skillDir := filepath.Join(tmpDir, "skills", name)
			require.NoError(t, os.MkdirAll(skillDir, 0755))

			content := "---\nname: " + name + "\ndescription: " + name + " desc\n---\nContent for " + name
			require.NoError(t, os.WriteFile(
				filepath.Join(skillDir, "SKILL.md"),
				[]byte(content),
				0644,
			))
		}

		loader := NewSkillLoader(tmpDir)
		content, err := loader.LoadSkillsContent([]string{"skill-a", "skill-b"})

		require.NoError(t, err)
		assert.Contains(t, content, "## Skill: skill-a")
		assert.Contains(t, content, "## Skill: skill-b")
		assert.Contains(t, content, "Content for skill-a")
		assert.Contains(t, content, "Content for skill-b")
	})

	t.Run("skips missing skills gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create one valid skill
		skillDir := filepath.Join(tmpDir, "skills", "valid")
		require.NoError(t, os.MkdirAll(skillDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte("---\nname: valid\ndescription: valid\n---\nValid content"),
			0644,
		))

		loader := NewSkillLoader(tmpDir)
		content, err := loader.LoadSkillsContent([]string{"missing", "valid"})

		require.NoError(t, err)
		assert.Contains(t, content, "Valid content")
		assert.NotContains(t, content, "missing")
	})

	t.Run("returns error if all skills fail", func(t *testing.T) {
		loader := NewSkillLoader(t.TempDir())
		_, err := loader.LoadSkillsContent([]string{"nonexistent1", "nonexistent2"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load any skills")
	})
}

func TestSkillLoader_LoadSkillAllowedTools(t *testing.T) {
	t.Run("empty refs returns nil", func(t *testing.T) {
		loader := NewSkillLoader(t.TempDir())
		tools, err := loader.LoadSkillAllowedTools(nil)

		require.NoError(t, err)
		assert.Nil(t, tools)
	})

	t.Run("loads allowed tools from skill", func(t *testing.T) {
		tmpDir := t.TempDir()
		skillDir := filepath.Join(tmpDir, "skills", "test")
		require.NoError(t, os.MkdirAll(skillDir, 0755))

		skillContent := `---
name: test
description: test skill
allowed-tools:
  - Read
  - Glob
  - Grep
---
Skill content`

		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte(skillContent),
			0644,
		))

		loader := NewSkillLoader(tmpDir)
		tools, err := loader.LoadSkillAllowedTools([]string{"test"})

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Read", "Glob", "Grep"}, tools)
	})

	t.Run("deduplicates tools across skills", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create two skills with overlapping tools
		for _, data := range []struct {
			name  string
			tools string
		}{
			{"skill1", "  - Read\n  - Write"},
			{"skill2", "  - Read\n  - Edit"},
		} {
			skillDir := filepath.Join(tmpDir, "skills", data.name)
			require.NoError(t, os.MkdirAll(skillDir, 0755))
			require.NoError(t, os.WriteFile(
				filepath.Join(skillDir, "SKILL.md"),
				[]byte("---\nname: "+data.name+"\ndescription: test\nallowed-tools:\n"+data.tools+"\n---\nContent"),
				0644,
			))
		}

		loader := NewSkillLoader(tmpDir)
		tools, err := loader.LoadSkillAllowedTools([]string{"skill1", "skill2"})

		require.NoError(t, err)
		// Should have Read, Write, Edit (Read deduplicated)
		assert.Len(t, tools, 3)
		assert.Contains(t, tools, "Read")
		assert.Contains(t, tools, "Write")
		assert.Contains(t, tools, "Edit")
	})
}

func TestSkillLoader_LoadSkillsForConfig(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		loader := NewSkillLoader(t.TempDir())
		err := loader.LoadSkillsForConfig(nil)

		require.NoError(t, err)
	})

	t.Run("empty skill refs returns nil", func(t *testing.T) {
		loader := NewSkillLoader(t.TempDir())
		cfg := &PhaseClaudeConfig{}
		err := loader.LoadSkillsForConfig(cfg)

		require.NoError(t, err)
	})

	t.Run("appends skill content to system prompt", func(t *testing.T) {
		tmpDir := t.TempDir()
		skillDir := filepath.Join(tmpDir, "skills", "test")
		require.NoError(t, os.MkdirAll(skillDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte("---\nname: test\ndescription: test\n---\nSkill content here"),
			0644,
		))

		loader := NewSkillLoader(tmpDir)
		cfg := &PhaseClaudeConfig{
			AppendSystemPrompt: "Existing prompt",
			SkillRefs:          []string{"test"},
		}

		err := loader.LoadSkillsForConfig(cfg)

		require.NoError(t, err)
		assert.Contains(t, cfg.AppendSystemPrompt, "Existing prompt")
		assert.Contains(t, cfg.AppendSystemPrompt, "Skill content here")
	})

	t.Run("merges allowed tools", func(t *testing.T) {
		tmpDir := t.TempDir()
		skillDir := filepath.Join(tmpDir, "skills", "test")
		require.NoError(t, os.MkdirAll(skillDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(skillDir, "SKILL.md"),
			[]byte("---\nname: test\ndescription: test\nallowed-tools:\n  - Glob\n  - Grep\n---\nContent"),
			0644,
		))

		loader := NewSkillLoader(tmpDir)
		cfg := &PhaseClaudeConfig{
			AllowedTools: []string{"Read"},
			SkillRefs:    []string{"test"},
		}

		err := loader.LoadSkillsForConfig(cfg)

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"Read", "Glob", "Grep"}, cfg.AllowedTools)
	})
}

func TestLoadSkillsContentSimple(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "skills", "simple")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte("---\nname: simple\ndescription: simple\n---\nSimple content"),
		0644,
	))

	content, err := LoadSkillsContentSimple(tmpDir, []string{"simple"})

	require.NoError(t, err)
	assert.Contains(t, content, "Simple content")
}
