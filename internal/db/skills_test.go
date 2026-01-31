package db

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for SC-5: skills table CRUD with supporting_files JSON column
// Tests for SC-6: Migration creates both tables

func TestSkills_CRUD(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	gdb, err := OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gdb.Close() })

	t.Run("save and get with supporting files", func(t *testing.T) {
		s := &Skill{
			ID:          "python-style",
			Name:        "Python Style",
			Description: "Python coding standards",
			Content:     "# Python Style Guide\nUse snake_case",
			SupportingFiles: map[string]string{
				"ruff.toml":  "[tool.ruff]\nline-length = 80",
				"pyright.json": `{"reportMissingImports": true}`,
			},
			IsBuiltin: true,
		}

		err := gdb.SaveSkill(s)
		require.NoError(t, err)

		got, err := gdb.GetSkill("python-style")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "python-style", got.ID)
		assert.Equal(t, "Python Style", got.Name)
		assert.Equal(t, "# Python Style Guide\nUse snake_case", got.Content)
		require.NotNil(t, got.SupportingFiles)
		assert.Equal(t, "[tool.ruff]\nline-length = 80", got.SupportingFiles["ruff.toml"])
		assert.Equal(t, `{"reportMissingImports": true}`, got.SupportingFiles["pyright.json"])
		assert.True(t, got.IsBuiltin)
		assert.NotEmpty(t, got.CreatedAt)
		assert.NotEmpty(t, got.UpdatedAt)
	})

	t.Run("get missing returns nil nil", func(t *testing.T) {
		got, err := gdb.GetSkill("nonexistent")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("empty supporting files stored as null returned as nil", func(t *testing.T) {
		s := &Skill{
			ID:              "minimal-skill",
			Name:            "Minimal",
			Content:         "# Minimal",
			SupportingFiles: nil, // explicitly nil
			IsBuiltin:       false,
		}
		err := gdb.SaveSkill(s)
		require.NoError(t, err)

		got, err := gdb.GetSkill("minimal-skill")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Nil(t, got.SupportingFiles)
	})

	t.Run("upsert on conflict", func(t *testing.T) {
		s := &Skill{
			ID:      "upsert-skill",
			Name:    "Original",
			Content: "original content",
		}
		err := gdb.SaveSkill(s)
		require.NoError(t, err)

		s.Name = "Updated"
		s.Content = "updated content"
		err = gdb.SaveSkill(s)
		require.NoError(t, err)

		got, err := gdb.GetSkill("upsert-skill")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "Updated", got.Name)
		assert.Equal(t, "updated content", got.Content)
	})

	t.Run("list returns all", func(t *testing.T) {
		list, err := gdb.ListSkills()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 3) // from previous subtests
	})

	t.Run("delete", func(t *testing.T) {
		s := &Skill{
			ID:      "deletable-skill",
			Name:    "Deletable",
			Content: "will be deleted",
		}
		err := gdb.SaveSkill(s)
		require.NoError(t, err)

		err = gdb.DeleteSkill("deletable-skill")
		require.NoError(t, err)

		got, err := gdb.GetSkill("deletable-skill")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("supporting files round trip as map", func(t *testing.T) {
		files := map[string]string{
			"helper.py":   "def helper(): pass",
			"config.yaml": "key: value",
			"data.json":   `{"nested": {"key": "value"}}`,
		}
		s := &Skill{
			ID:              "files-roundtrip",
			Name:            "Round Trip",
			Content:         "# Test",
			SupportingFiles: files,
		}
		err := gdb.SaveSkill(s)
		require.NoError(t, err)

		got, err := gdb.GetSkill("files-roundtrip")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, files, got.SupportingFiles)
	})
}

func TestGlobalMigration_HookScriptsAndSkillsTables(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Opening GlobalDB runs all migrations including global_006.sql
	gdb, err := OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gdb.Close() })

	// Verify hook_scripts table exists by doing a query
	var count int
	err = gdb.QueryRow("SELECT COUNT(*) FROM hook_scripts").Scan(&count)
	require.NoError(t, err, "hook_scripts table should exist after migration")

	// Verify skills table exists
	err = gdb.QueryRow("SELECT COUNT(*) FROM skills").Scan(&count)
	require.NoError(t, err, "skills table should exist after migration")
}
