// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-671: Export/Import for hooks and skills between GlobalDB and .claude directories
//
// These tests verify that ConfigServer export/import/scan methods correctly:
// - Export hooks/skills from GlobalDB to .claude/ directories (project or user)
// - Scan .claude/ directories for hooks/skills not in GlobalDB
// - Import discovered hooks/skills into GlobalDB
//
// Tests will NOT COMPILE until:
// 1. Proto types added: ExportHooksRequest/Response, ImportHooksRequest/Response,
//    ExportSkillsRequest/Response, ImportSkillsRequest/Response,
//    ScanClaudeDirRequest/Response, DiscoveredItem
// 2. RPCs added to ConfigService: ExportHooks, ImportHooks, ExportSkills,
//    ImportSkills, ScanClaudeDir
// 3. Handler methods implemented on configServer
package api

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// --- Test helpers for export/import tests ---

// newTestConfigServerForExport creates a ConfigServer with an in-memory GlobalDB
// and a temp directory as the project workDir (used as PROJECT destination).
// Returns the server, GlobalDB, and the project directory path.
func newTestConfigServerForExport(t *testing.T) (*configServer, *db.GlobalDB, string) {
	t.Helper()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	projectDir := t.TempDir()

	server := NewConfigServer(nil, backend, projectDir, nil)
	cs := server.(*configServer)
	cs.SetGlobalDB(globalDB)

	return cs, globalDB, projectDir
}

// seedHook inserts a non-builtin hook into GlobalDB.
func seedHook(t *testing.T, gdb *db.GlobalDB, id, name, eventType, content string) {
	t.Helper()
	err := gdb.SaveHookScript(&db.HookScript{
		ID:        id,
		Name:      name,
		EventType: eventType,
		Content:   content,
		IsBuiltin: false,
	})
	require.NoError(t, err)
}

// seedSkill inserts a non-builtin skill into GlobalDB.
func seedSkill(t *testing.T, gdb *db.GlobalDB, id, name, content string, supportingFiles map[string]string) {
	t.Helper()
	err := gdb.SaveSkill(&db.Skill{
		ID:              id,
		Name:            name,
		Content:         content,
		SupportingFiles: supportingFiles,
		IsBuiltin:       false,
	})
	require.NoError(t, err)
}

// ============================================================================
// SC-1: ExportHooks writes executable script files to the correct destination
// ============================================================================

func TestExportHooks_WritesFilesWithCorrectPermissions(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	seedHook(t, gdb, "hook-1", "custom-lint", "PreToolUse", "#!/bin/bash\necho lint")
	seedHook(t, gdb, "hook-2", "custom-format", "Stop", "#!/bin/bash\necho format")

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"hook-1", "hook-2"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ExportHooks(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Msg.WrittenPaths, 2)

	// Verify files exist at project .claude/hooks/
	for _, writtenPath := range resp.Msg.WrittenPaths {
		assert.True(t, strings.HasPrefix(writtenPath, filepath.Join(projectDir, ".claude", "hooks")),
			"written path %s should be under project .claude/hooks/", writtenPath)

		info, err := os.Stat(writtenPath)
		require.NoError(t, err, "file should exist: %s", writtenPath)
		assert.Equal(t, os.FileMode(0755), info.Mode().Perm(),
			"hook file should have 0755 permissions")
	}

	// Verify content matches DB
	lintPath := filepath.Join(projectDir, ".claude", "hooks", "custom-lint")
	content, err := os.ReadFile(lintPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/bash\necho lint", string(content))

	formatPath := filepath.Join(projectDir, ".claude", "hooks", "custom-format")
	content, err = os.ReadFile(formatPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/bash\necho format", string(content))
}

// ============================================================================
// SC-2: ExportHooks supports both project and user destinations
// ============================================================================

func TestExportHooks_ProjectDestination(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	seedHook(t, gdb, "hook-1", "my-hook", "Stop", "#!/bin/bash\nexit 0")

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"hook-1"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ExportHooks(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Msg.WrittenPaths, 1)

	expectedPath := filepath.Join(projectDir, ".claude", "hooks", "my-hook")
	assert.Equal(t, expectedPath, resp.Msg.WrittenPaths[0])

	_, err = os.Stat(expectedPath)
	require.NoError(t, err, "file should exist at project destination")
}

func TestExportHooks_GlobalDestination(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	// Override home dir to a temp dir for isolation
	homeDir := t.TempDir()
	server.testHomeDir = homeDir

	seedHook(t, gdb, "hook-1", "my-hook", "Stop", "#!/bin/bash\nexit 0")

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"hook-1"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL,
	})

	resp, err := server.ExportHooks(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Msg.WrittenPaths, 1)

	expectedPath := filepath.Join(homeDir, ".claude", "hooks", "my-hook")
	assert.Equal(t, expectedPath, resp.Msg.WrittenPaths[0])

	_, err = os.Stat(expectedPath)
	require.NoError(t, err, "file should exist at global destination")
}

func TestExportHooks_UnspecifiedDestination_ReturnsError(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	seedHook(t, gdb, "hook-1", "my-hook", "Stop", "#!/bin/bash\nexit 0")

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"hook-1"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_UNSPECIFIED,
	})

	_, err := server.ExportHooks(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// ============================================================================
// SC-3: ExportSkills writes SKILL.md and supporting files
// ============================================================================

func TestExportSkills_WritesSkillMdAndSupportingFiles(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	supportingFiles := map[string]string{
		"ruff.toml":    "[tool.ruff]\nline-length = 80",
		"pyright.json": `{"reportMissingImports": true}`,
	}
	seedSkill(t, gdb, "skill-1", "python-style", "# Python Style\nUse snake_case", supportingFiles)

	req := connect.NewRequest(&orcv1.ExportSkillsRequest{
		SkillIds:    []string{"skill-1"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ExportSkills(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.WrittenPaths)

	// Verify SKILL.md exists with correct content
	skillMdPath := filepath.Join(projectDir, ".claude", "skills", "python-style", "SKILL.md")
	content, err := os.ReadFile(skillMdPath)
	require.NoError(t, err)
	assert.Equal(t, "# Python Style\nUse snake_case", string(content))

	// Verify supporting files exist with correct content
	ruffPath := filepath.Join(projectDir, ".claude", "skills", "python-style", "ruff.toml")
	content, err = os.ReadFile(ruffPath)
	require.NoError(t, err)
	assert.Equal(t, "[tool.ruff]\nline-length = 80", string(content))

	pyrightPath := filepath.Join(projectDir, ".claude", "skills", "python-style", "pyright.json")
	content, err = os.ReadFile(pyrightPath)
	require.NoError(t, err)
	assert.Equal(t, `{"reportMissingImports": true}`, string(content))
}

func TestExportSkills_NoSupportingFiles(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	seedSkill(t, gdb, "skill-1", "simple-skill", "# Simple\nJust content", nil)

	req := connect.NewRequest(&orcv1.ExportSkillsRequest{
		SkillIds:    []string{"skill-1"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ExportSkills(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.WrittenPaths)

	// SKILL.md should exist
	skillMdPath := filepath.Join(projectDir, ".claude", "skills", "simple-skill", "SKILL.md")
	content, err := os.ReadFile(skillMdPath)
	require.NoError(t, err)
	assert.Equal(t, "# Simple\nJust content", string(content))
}

// ============================================================================
// SC-4: ExportSkills rejects path traversal in skill IDs
// ============================================================================

func TestExportSkills_PathTraversal_DotDot(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	// Seed a skill with a safe ID, then try to export with a traversal ID
	seedSkill(t, gdb, "../etc/passwd", "evil-skill", "pwned", nil)

	req := connect.NewRequest(&orcv1.ExportSkillsRequest{
		SkillIds:    []string{"../etc/passwd"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := server.ExportSkills(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	assert.Contains(t, connectErr.Message(), "invalid skill ID")
}

func TestExportSkills_PathTraversal_Slash(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	seedSkill(t, gdb, "foo/bar", "nested-skill", "content", nil)

	req := connect.NewRequest(&orcv1.ExportSkillsRequest{
		SkillIds:    []string{"foo/bar"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := server.ExportSkills(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// ============================================================================
// SC-5: ScanClaudeDir discovers hook scripts not present in library
// ============================================================================

func TestScanClaudeDir_DiscoverNewHooks(t *testing.T) {
	t.Parallel()
	server, _, projectDir := newTestConfigServerForExport(t)

	// Create a hook file on disk that's NOT in GlobalDB
	hooksDir := filepath.Join(projectDir, ".claude", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "my-hook.sh"),
		[]byte("#!/bin/bash\necho discovered"),
		0755,
	))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)

	// Find the hook in results
	var found *orcv1.DiscoveredItem
	for _, item := range resp.Msg.Items {
		if item.ItemType == "hook" && item.Name == "my-hook" {
			found = item
			break
		}
	}
	require.NotNil(t, found, "should discover my-hook.sh")
	assert.Equal(t, "new", found.Status)
	assert.Contains(t, found.Content, "#!/bin/bash")
}

func TestScanClaudeDir_DiscoverModifiedHook(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	// Hook exists in GlobalDB with certain content
	seedHook(t, gdb, "hook-1", "my-hook", "Stop", "#!/bin/bash\necho original")

	// Same-named hook on disk has different content
	hooksDir := filepath.Join(projectDir, ".claude", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "my-hook"),
		[]byte("#!/bin/bash\necho modified"),
		0755,
	))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)

	var found *orcv1.DiscoveredItem
	for _, item := range resp.Msg.Items {
		if item.ItemType == "hook" && item.Name == "my-hook" {
			found = item
			break
		}
	}
	require.NotNil(t, found, "should discover modified my-hook")
	assert.Equal(t, "modified", found.Status)
}

func TestScanClaudeDir_SkipsAlreadySyncedHook(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	content := "#!/bin/bash\necho synced"
	seedHook(t, gdb, "hook-1", "synced-hook", "Stop", content)

	// Same content on disk
	hooksDir := filepath.Join(projectDir, ".claude", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "synced-hook"),
		[]byte(content),
		0755,
	))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)

	// Should NOT appear in results (already synced)
	for _, item := range resp.Msg.Items {
		if item.ItemType == "hook" && item.Name == "synced-hook" {
			t.Fatal("already-synced hook should not appear in scan results")
		}
	}
}

func TestScanClaudeDir_HookWithoutExtension(t *testing.T) {
	t.Parallel()
	server, _, projectDir := newTestConfigServerForExport(t)

	hooksDir := filepath.Join(projectDir, ".claude", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))
	// File without .sh extension - should still be discovered
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "my-hook"),
		[]byte("#!/bin/bash\necho no-ext"),
		0755,
	))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)

	var found bool
	for _, item := range resp.Msg.Items {
		if item.ItemType == "hook" && item.Name == "my-hook" {
			found = true
			break
		}
	}
	assert.True(t, found, "hook without .sh extension should still be discovered")
}

// ============================================================================
// SC-6: ScanClaudeDir discovers skills not present in library
// ============================================================================

func TestScanClaudeDir_DiscoverNewSkill(t *testing.T) {
	t.Parallel()
	server, _, projectDir := newTestConfigServerForExport(t)

	// Create a skill directory with SKILL.md on disk
	skillDir := filepath.Join(projectDir, ".claude", "skills", "python-style")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte("# Python Style\nUse snake_case"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "ruff.toml"),
		[]byte("[tool.ruff]\nline-length = 80"),
		0644,
	))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)

	var found *orcv1.DiscoveredItem
	for _, item := range resp.Msg.Items {
		if item.ItemType == "skill" && item.Name == "python-style" {
			found = item
			break
		}
	}
	require.NotNil(t, found, "should discover python-style skill")
	assert.Equal(t, "new", found.Status)
	assert.Contains(t, found.Content, "# Python Style")
	assert.Contains(t, found.SupportingFiles, "ruff.toml")
	assert.Equal(t, "[tool.ruff]\nline-length = 80", found.SupportingFiles["ruff.toml"])
}

func TestScanClaudeDir_SkipsDirWithoutSkillMd(t *testing.T) {
	t.Parallel()
	server, _, projectDir := newTestConfigServerForExport(t)

	// Directory without SKILL.md should be skipped
	noSkillDir := filepath.Join(projectDir, ".claude", "skills", "not-a-skill")
	require.NoError(t, os.MkdirAll(noSkillDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(noSkillDir, "README.md"),
		[]byte("This is not a skill"),
		0644,
	))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)

	for _, item := range resp.Msg.Items {
		if item.Name == "not-a-skill" {
			t.Fatal("directory without SKILL.md should not be discovered")
		}
	}
}

func TestScanClaudeDir_DiscoverModifiedSkill(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	// Skill in DB
	seedSkill(t, gdb, "skill-1", "my-skill", "# Original content", nil)

	// Same-named skill on disk with different content
	skillDir := filepath.Join(projectDir, ".claude", "skills", "my-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte("# Modified content"),
		0644,
	))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)

	var found *orcv1.DiscoveredItem
	for _, item := range resp.Msg.Items {
		if item.ItemType == "skill" && item.Name == "my-skill" {
			found = item
			break
		}
	}
	require.NotNil(t, found, "should discover modified my-skill")
	assert.Equal(t, "modified", found.Status)
}

// ============================================================================
// SC-7: ImportHooks creates GlobalDB entries from discovered hook files
// ============================================================================

func TestImportHooks_CreatesGlobalDBEntries(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	req := connect.NewRequest(&orcv1.ImportHooksRequest{
		Items: []*orcv1.DiscoveredItem{
			{
				Name:     "imported-hook",
				Content:  "#!/bin/bash\necho imported",
				ItemType: "hook",
				Status:   "new",
			},
		},
	})

	resp, err := server.ImportHooks(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Msg.Imported, 1)
	assert.Equal(t, "imported-hook", resp.Msg.Imported[0].Name)
	assert.Equal(t, "#!/bin/bash\necho imported", resp.Msg.Imported[0].Content)
	assert.False(t, resp.Msg.Imported[0].IsBuiltin)

	// Verify in DB
	scripts, err := gdb.ListHookScripts()
	require.NoError(t, err)

	var found bool
	for _, s := range scripts {
		if s.Name == "imported-hook" {
			found = true
			assert.Equal(t, "#!/bin/bash\necho imported", s.Content)
			assert.False(t, s.IsBuiltin)
			break
		}
	}
	assert.True(t, found, "imported hook should appear in GlobalDB")
}

func TestImportHooks_MultipleItems(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	req := connect.NewRequest(&orcv1.ImportHooksRequest{
		Items: []*orcv1.DiscoveredItem{
			{Name: "hook-a", Content: "#!/bin/bash\necho a", ItemType: "hook", Status: "new"},
			{Name: "hook-b", Content: "#!/bin/bash\necho b", ItemType: "hook", Status: "new"},
		},
	})

	resp, err := server.ImportHooks(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Imported, 2)

	scripts, err := gdb.ListHookScripts()
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, s := range scripts {
		names[s.Name] = true
	}
	assert.True(t, names["hook-a"])
	assert.True(t, names["hook-b"])
}

// ============================================================================
// SC-8: ImportSkills creates GlobalDB entries from discovered skill directories
// ============================================================================

func TestImportSkills_CreatesGlobalDBEntries(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	req := connect.NewRequest(&orcv1.ImportSkillsRequest{
		Items: []*orcv1.DiscoveredItem{
			{
				Name:     "python-style",
				Content:  "# Python Style\nUse snake_case",
				ItemType: "skill",
				Status:   "new",
				SupportingFiles: map[string]string{
					"ruff.toml": "[tool.ruff]\nline-length = 80",
				},
			},
		},
	})

	resp, err := server.ImportSkills(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Msg.Imported, 1)
	assert.Equal(t, "python-style", resp.Msg.Imported[0].Name)
	assert.Equal(t, "# Python Style\nUse snake_case", resp.Msg.Imported[0].Content)
	assert.False(t, resp.Msg.Imported[0].IsBuiltin)
	assert.Equal(t, "[tool.ruff]\nline-length = 80", resp.Msg.Imported[0].SupportingFiles["ruff.toml"])

	// Verify in DB
	skills, err := gdb.ListSkills()
	require.NoError(t, err)

	var found *db.Skill
	for _, s := range skills {
		if s.Name == "python-style" {
			found = s
			break
		}
	}
	require.NotNil(t, found, "imported skill should appear in GlobalDB")
	assert.Equal(t, "# Python Style\nUse snake_case", found.Content)
	assert.Equal(t, "[tool.ruff]\nline-length = 80", found.SupportingFiles["ruff.toml"])
	assert.False(t, found.IsBuiltin)
}

// ============================================================================
// Failure Modes
// ============================================================================

func TestExportHooks_NotFound(t *testing.T) {
	t.Parallel()
	server, _, _ := newTestConfigServerForExport(t)

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"nonexistent-id"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := server.ExportHooks(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestExportHooks_PermissionError(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	seedHook(t, gdb, "hook-1", "my-hook", "Stop", "#!/bin/bash\nexit 0")

	// Make destination read-only
	readOnlyDir := filepath.Join(projectDir, ".claude", "hooks")
	require.NoError(t, os.MkdirAll(readOnlyDir, 0755))
	require.NoError(t, os.Chmod(readOnlyDir, 0444))
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0755) })

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"hook-1"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := server.ExportHooks(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}

func TestExportSkills_NotFound(t *testing.T) {
	t.Parallel()
	server, _, _ := newTestConfigServerForExport(t)

	req := connect.NewRequest(&orcv1.ExportSkillsRequest{
		SkillIds:    []string{"nonexistent-id"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := server.ExportSkills(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

func TestScanClaudeDir_EmptyDirectory(t *testing.T) {
	t.Parallel()
	server, _, projectDir := newTestConfigServerForExport(t)

	// Create empty .claude/ with empty hooks/ and skills/
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".claude", "hooks"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".claude", "skills"), 0755))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Items)
}

func TestScanClaudeDir_MissingDirectory(t *testing.T) {
	t.Parallel()
	server, _, _ := newTestConfigServerForExport(t)

	// projectDir has no .claude/ at all - should return empty, not error
	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Items)
}

func TestScanClaudeDir_SkipsBinaryFiles(t *testing.T) {
	t.Parallel()
	server, _, projectDir := newTestConfigServerForExport(t)

	hooksDir := filepath.Join(projectDir, ".claude", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Write a binary file (contains null bytes)
	binaryContent := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "binary-file"), binaryContent, 0755))

	// Also write a valid script
	require.NoError(t, os.WriteFile(
		filepath.Join(hooksDir, "valid-hook"),
		[]byte("#!/bin/bash\necho valid"),
		0755,
	))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)

	for _, item := range resp.Msg.Items {
		assert.NotEqual(t, "binary-file", item.Name, "binary files should be skipped")
	}

	// Valid hook should still be discovered
	var foundValid bool
	for _, item := range resp.Msg.Items {
		if item.Name == "valid-hook" {
			foundValid = true
			break
		}
	}
	assert.True(t, foundValid, "valid text hook should still be discovered")
}

func TestImportHooks_DuplicateName(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	// Seed an existing hook with name "existing-hook"
	seedHook(t, gdb, "hook-1", "existing-hook", "Stop", "#!/bin/bash\noriginal")

	req := connect.NewRequest(&orcv1.ImportHooksRequest{
		Items: []*orcv1.DiscoveredItem{
			{
				Name:     "existing-hook",
				Content:  "#!/bin/bash\nnew content",
				ItemType: "hook",
				Status:   "new",
			},
		},
	})

	_, err := server.ImportHooks(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())

	// Verify original is unchanged
	got, err := gdb.GetHookScript("hook-1")
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/bash\noriginal", got.Content)
}

func TestImportHooks_BuiltInCollision(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	// Seed a built-in hook
	seedBuiltinHook(t, gdb, "orc-verify", "orc-verify", "Stop", "#!/bin/bash\nverify")

	req := connect.NewRequest(&orcv1.ImportHooksRequest{
		Items: []*orcv1.DiscoveredItem{
			{
				Name:     "orc-verify",
				Content:  "#!/bin/bash\nhijacked",
				ItemType: "hook",
				Status:   "new",
			},
		},
	})

	_, err := server.ImportHooks(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())
}

func TestImportHooks_EmptyContent(t *testing.T) {
	t.Parallel()
	server, _, _ := newTestConfigServerForExport(t)

	req := connect.NewRequest(&orcv1.ImportHooksRequest{
		Items: []*orcv1.DiscoveredItem{
			{
				Name:     "empty-hook",
				Content:  "",
				ItemType: "hook",
				Status:   "new",
			},
		},
	})

	_, err := server.ImportHooks(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestImportSkills_DuplicateName(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	seedSkill(t, gdb, "skill-1", "existing-skill", "# Original", nil)

	req := connect.NewRequest(&orcv1.ImportSkillsRequest{
		Items: []*orcv1.DiscoveredItem{
			{
				Name:     "existing-skill",
				Content:  "# Different",
				ItemType: "skill",
				Status:   "new",
			},
		},
	})

	_, err := server.ImportSkills(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())
}

// ============================================================================
// Edge Cases
// ============================================================================

func TestExportHooks_EmptyIDsList(t *testing.T) {
	t.Parallel()
	server, _, _ := newTestConfigServerForExport(t)

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := server.ExportHooks(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestExportSkills_EmptyIDsList(t *testing.T) {
	t.Parallel()
	server, _, _ := newTestConfigServerForExport(t)

	req := connect.NewRequest(&orcv1.ExportSkillsRequest{
		SkillIds:    []string{},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := server.ExportSkills(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestExportHooks_BuiltInAllowed(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	// Built-in hooks CAN be exported (export is read-only copy)
	seedBuiltinHook(t, gdb, "orc-verify", "orc-verify", "Stop", "#!/bin/bash\nverify")

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"orc-verify"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ExportHooks(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.Msg.WrittenPaths, 1)

	content, err := os.ReadFile(filepath.Join(projectDir, ".claude", "hooks", "orc-verify"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/bash\nverify", string(content))
}

func TestScanClaudeDir_LargeFile(t *testing.T) {
	t.Parallel()
	server, _, projectDir := newTestConfigServerForExport(t)

	hooksDir := filepath.Join(projectDir, ".claude", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Write a >1MB script
	largeContent := "#!/bin/bash\n" + strings.Repeat("echo line\n", 120000) // ~1.2MB
	require.Greater(t, len(largeContent), 1024*1024)
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "large-hook"), []byte(largeContent), 0755))

	req := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	resp, err := server.ScanClaudeDir(context.Background(), req)
	require.NoError(t, err)

	var found *orcv1.DiscoveredItem
	for _, item := range resp.Msg.Items {
		if item.Name == "large-hook" {
			found = item
			break
		}
	}
	require.NotNil(t, found, "large hook should be discovered")
	// Content should be truncated in preview (first 10KB per spec)
	assert.LessOrEqual(t, len(found.Content), 10240+100, // small margin for rounding
		"scan preview should truncate content to ~10KB")
}

func TestExportHooks_NilGlobalDB(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	projectDir := t.TempDir()

	server := NewConfigServer(nil, backend, projectDir, nil)
	cs := server.(*configServer)
	// Do NOT set GlobalDB

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"hook-1"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := cs.ExportHooks(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}

// ============================================================================
// Integration: Full round-trip export → scan → import
// ============================================================================

func TestExportImportRoundTrip_Hooks(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	// 1. Seed hooks in GlobalDB
	seedHook(t, gdb, "hook-export", "roundtrip-hook", "Stop", "#!/bin/bash\necho roundtrip")

	// 2. Export to project .claude/
	exportReq := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"hook-export"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})
	exportResp, err := server.ExportHooks(context.Background(), exportReq)
	require.NoError(t, err)
	require.Len(t, exportResp.Msg.WrittenPaths, 1)

	// 3. Verify file exists on disk
	hookPath := filepath.Join(projectDir, ".claude", "hooks", "roundtrip-hook")
	_, err = os.Stat(hookPath)
	require.NoError(t, err)

	// 4. Remove the hook from GlobalDB to simulate fresh import
	require.NoError(t, gdb.DeleteHookScript("hook-export"))

	// 5. Scan should discover it as "new"
	scanReq := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})
	scanResp, err := server.ScanClaudeDir(context.Background(), scanReq)
	require.NoError(t, err)

	var discovered *orcv1.DiscoveredItem
	for _, item := range scanResp.Msg.Items {
		if item.ItemType == "hook" && item.Name == "roundtrip-hook" {
			discovered = item
			break
		}
	}
	require.NotNil(t, discovered, "exported hook should be discoverable after DB deletion")
	assert.Equal(t, "new", discovered.Status)

	// 6. Import it back
	importReq := connect.NewRequest(&orcv1.ImportHooksRequest{
		Items: []*orcv1.DiscoveredItem{discovered},
	})
	importResp, err := server.ImportHooks(context.Background(), importReq)
	require.NoError(t, err)
	require.Len(t, importResp.Msg.Imported, 1)
	assert.Equal(t, "roundtrip-hook", importResp.Msg.Imported[0].Name)
	assert.Equal(t, "#!/bin/bash\necho roundtrip", importResp.Msg.Imported[0].Content)

	// 7. Verify in DB
	scripts, err := gdb.ListHookScripts()
	require.NoError(t, err)
	var found bool
	for _, s := range scripts {
		if s.Name == "roundtrip-hook" {
			found = true
			assert.Equal(t, "#!/bin/bash\necho roundtrip", s.Content)
			break
		}
	}
	assert.True(t, found, "re-imported hook should be in GlobalDB")
}

func TestExportImportRoundTrip_Skills(t *testing.T) {
	t.Parallel()
	server, gdb, projectDir := newTestConfigServerForExport(t)

	// 1. Seed skill in GlobalDB
	supportingFiles := map[string]string{
		"config.toml": "key = value",
	}
	seedSkill(t, gdb, "skill-export", "roundtrip-skill", "# Roundtrip\nContent", supportingFiles)

	// 2. Export to project .claude/
	exportReq := connect.NewRequest(&orcv1.ExportSkillsRequest{
		SkillIds:    []string{"skill-export"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})
	exportResp, err := server.ExportSkills(context.Background(), exportReq)
	require.NoError(t, err)
	require.NotEmpty(t, exportResp.Msg.WrittenPaths)

	// 3. Verify files on disk
	skillMdPath := filepath.Join(projectDir, ".claude", "skills", "roundtrip-skill", "SKILL.md")
	content, err := os.ReadFile(skillMdPath)
	require.NoError(t, err)
	assert.Equal(t, "# Roundtrip\nContent", string(content))

	configPath := filepath.Join(projectDir, ".claude", "skills", "roundtrip-skill", "config.toml")
	content, err = os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, "key = value", string(content))

	// 4. Remove from GlobalDB
	require.NoError(t, gdb.DeleteSkill("skill-export"))

	// 5. Scan should discover it as "new"
	scanReq := connect.NewRequest(&orcv1.ScanClaudeDirRequest{
		Source: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})
	scanResp, err := server.ScanClaudeDir(context.Background(), scanReq)
	require.NoError(t, err)

	var discovered *orcv1.DiscoveredItem
	for _, item := range scanResp.Msg.Items {
		if item.ItemType == "skill" && item.Name == "roundtrip-skill" {
			discovered = item
			break
		}
	}
	require.NotNil(t, discovered, "exported skill should be discoverable")
	assert.Equal(t, "new", discovered.Status)
	assert.Equal(t, "key = value", discovered.SupportingFiles["config.toml"])

	// 6. Import it back
	importReq := connect.NewRequest(&orcv1.ImportSkillsRequest{
		Items: []*orcv1.DiscoveredItem{discovered},
	})
	importResp, err := server.ImportSkills(context.Background(), importReq)
	require.NoError(t, err)
	require.Len(t, importResp.Msg.Imported, 1)

	// 7. Verify in DB
	skills, err := gdb.ListSkills()
	require.NoError(t, err)
	var foundSkill *db.Skill
	for _, s := range skills {
		if s.Name == "roundtrip-skill" {
			foundSkill = s
			break
		}
	}
	require.NotNil(t, foundSkill)
	assert.Equal(t, "# Roundtrip\nContent", foundSkill.Content)
	assert.Equal(t, "key = value", foundSkill.SupportingFiles["config.toml"])
}

// ============================================================================
// Path Traversal: DB-sourced names
// ============================================================================

func TestExportHooks_PathTraversal(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	// Seed a hook whose Name contains a path traversal sequence
	seedHook(t, gdb, "hook-evil", "../evil", "Stop", "#!/bin/bash\necho pwned")

	req := connect.NewRequest(&orcv1.ExportHooksRequest{
		HookIds:     []string{"hook-evil"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := server.ExportHooks(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	assert.Contains(t, connectErr.Message(), "invalid hook name")
}

func TestExportSkills_SupportingFilePathTraversal(t *testing.T) {
	t.Parallel()
	server, gdb, _ := newTestConfigServerForExport(t)

	// Seed a skill with a supporting file whose name contains path traversal
	supportingFiles := map[string]string{
		"../../evil.sh": "#!/bin/bash\necho pwned",
	}
	seedSkill(t, gdb, "skill-evil", "safe-name", "# Skill content", supportingFiles)

	req := connect.NewRequest(&orcv1.ExportSkillsRequest{
		SkillIds:    []string{"skill-evil"},
		Destination: orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT,
	})

	_, err := server.ExportSkills(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
	assert.Contains(t, connectErr.Message(), "invalid supporting filename")
}
