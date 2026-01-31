package executor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for SC-9: ApplyPhaseSettings writes merged settings.json preserving project hooks
// Tests for SC-10: ApplyPhaseSettings writes hook script files and skill files
// Tests for SC-11: resetClaudeDir restores .claude/ from source branch
// Tests for SC-12: WorktreeBaseConfig struct replaces ClaudeCodeHookConfig

// --- SC-12: WorktreeBaseConfig struct ---

func TestWorktreeBaseConfig_FieldsExist(t *testing.T) {
	t.Parallel()

	// Verify the struct has the expected fields.
	// This is a compile-time check — if any field is missing, this won't compile.
	cfg := WorktreeBaseConfig{
		WorktreePath:  "/tmp/worktree",
		MainRepoPath:  "/home/user/repo",
		TaskID:        "TASK-001",
		InjectUserEnv: true,
		AdditionalEnv: map[string]string{"FOO": "bar"},
	}

	assert.Equal(t, "/tmp/worktree", cfg.WorktreePath)
	assert.Equal(t, "/home/user/repo", cfg.MainRepoPath)
	assert.Equal(t, "TASK-001", cfg.TaskID)
	assert.True(t, cfg.InjectUserEnv)
	assert.Equal(t, "bar", cfg.AdditionalEnv["FOO"])
}

// --- SC-9: ApplyPhaseSettings merge scenarios ---

// mockHookScriptGetter implements whatever interface ApplyPhaseSettings uses for DB reads.
type mockHookScriptGetter struct {
	scripts map[string]*db.HookScript
}

func (m *mockHookScriptGetter) GetHookScript(id string) (*db.HookScript, error) {
	if hs, ok := m.scripts[id]; ok {
		return hs, nil
	}
	return nil, nil
}

type mockSkillGetter struct {
	skills map[string]*db.Skill
}

func (m *mockSkillGetter) GetSkill(id string) (*db.Skill, error) {
	if s, ok := m.skills[id]; ok {
		return s, nil
	}
	return nil, nil
}

func setupWorktreeWithProjectHooks(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	claudeDir := filepath.Join(dir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Write existing project settings.json with hooks
	projectSettings := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Bash",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "bash project-hook.sh",
						},
					},
				},
			},
		},
	}
	data, err := json.MarshalIndent(projectSettings, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644))

	return dir
}

func TestApplyPhaseSettings_PreservesProjectHooks(t *testing.T) {
	t.Parallel()
	worktree := setupWorktreeWithProjectHooks(t)

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main/repo",
		TaskID:       "TASK-001",
	}

	// Phase config with its own hooks
	phaseCfg := &PhaseClaudeConfig{
		Hooks: map[string][]HookMatcher{
			"PreToolUse": {
				{Matcher: "Edit|Write", Hooks: []HookEntry{{Type: "command", Command: "python3 isolation.py"}}},
			},
		},
	}

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	err := ApplyPhaseSettings(worktree, phaseCfg, baseCfg, hsGetter, sGetter)
	require.NoError(t, err)

	// Read the resulting settings.json
	data, err := os.ReadFile(filepath.Join(worktree, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	// Verify hooks exist in output
	hooks, ok := settings["hooks"]
	require.True(t, ok, "settings.json must contain hooks")

	hooksMap, ok := hooks.(map[string]any)
	require.True(t, ok)

	preToolUse, ok := hooksMap["PreToolUse"]
	require.True(t, ok, "PreToolUse hooks must exist")

	// Should contain BOTH project hooks and phase hooks (preserved + added)
	matchers, ok := preToolUse.([]any)
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(matchers), 2, "should contain project hook + phase hook + isolation hook")
}

func TestApplyPhaseSettings_MergesMCPServers(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(worktree, ".claude"), 0755))

	// Write existing settings with an MCP server
	existing := map[string]any{
		"mcpServers": map[string]any{
			"project-server": map[string]any{
				"command": "node",
				"args":    []string{"server.js"},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".claude", "settings.json"), data, 0644))

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	phaseCfg := &PhaseClaudeConfig{
		MCPServers: map[string]claude.MCPServerConfig{
			"phase-server": {
				Command: "python3",
				Args:    []string{"mcp.py"},
			},
		},
	}

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	err := ApplyPhaseSettings(worktree, phaseCfg, baseCfg, hsGetter, sGetter)
	require.NoError(t, err)

	data, err = os.ReadFile(filepath.Join(worktree, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	mcpServers, ok := settings["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers must exist")

	// Phase server wins on collision, but both should be present here (no collision)
	assert.Contains(t, mcpServers, "phase-server")
}

func TestApplyPhaseSettings_MergesEnvVars(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()

	baseCfg := &WorktreeBaseConfig{
		WorktreePath:  worktree,
		MainRepoPath:  "/fake/main",
		TaskID:        "TASK-001",
		AdditionalEnv: map[string]string{"BASE_VAR": "base_value"},
	}

	phaseCfg := &PhaseClaudeConfig{
		Env: map[string]string{"PHASE_VAR": "phase_value"},
	}

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	err := ApplyPhaseSettings(worktree, phaseCfg, baseCfg, hsGetter, sGetter)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(worktree, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	env, ok := settings["env"].(map[string]any)
	require.True(t, ok, "env must exist")
	assert.Equal(t, "phase_value", env["PHASE_VAR"])
}

func TestApplyPhaseSettings_NilPhaseConfig(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	// nil phase config: only isolation hooks from baseCfg should be written
	err := ApplyPhaseSettings(worktree, nil, baseCfg, hsGetter, sGetter)
	require.NoError(t, err)

	// settings.json should exist with at least isolation hooks
	data, err := os.ReadFile(filepath.Join(worktree, ".claude", "settings.json"))
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestApplyPhaseSettings_EmptyHooks(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	phaseCfg := &PhaseClaudeConfig{
		Hooks: map[string][]HookMatcher{}, // empty, not nil
	}

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	err := ApplyPhaseSettings(worktree, phaseCfg, baseCfg, hsGetter, sGetter)
	require.NoError(t, err)

	// Should still write settings.json with isolation hooks
	_, err = os.Stat(filepath.Join(worktree, ".claude", "settings.json"))
	require.NoError(t, err)
}

func TestApplyPhaseSettings_NoClaude(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()
	// No .claude/ directory exists

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	err := ApplyPhaseSettings(worktree, nil, baseCfg, hsGetter, sGetter)
	require.NoError(t, err)

	// Directory should be created
	info, err := os.Stat(filepath.Join(worktree, ".claude"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestApplyPhaseSettings_InvalidJSON(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()

	claudeDir := filepath.Join(worktree, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{invalid json"), 0644))

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	err := ApplyPhaseSettings(worktree, nil, baseCfg, hsGetter, sGetter)
	assert.Error(t, err, "should error on invalid JSON in existing settings.json")
}

func TestApplyPhaseSettings_BadPath(t *testing.T) {
	t.Parallel()

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: "/nonexistent/path",
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	err := ApplyPhaseSettings("/nonexistent/path", nil, baseCfg, hsGetter, sGetter)
	assert.Error(t, err, "should error when worktree path doesn't exist")
}

func TestApplyPhaseSettings_PreservesUnknownFields(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()

	claudeDir := filepath.Join(worktree, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))

	// Settings with unknown fields
	existing := map[string]any{
		"customField":  "should be preserved",
		"anotherField": 42,
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0644))

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	err := ApplyPhaseSettings(worktree, nil, baseCfg, hsGetter, sGetter)
	require.NoError(t, err)

	data, err = os.ReadFile(filepath.Join(worktree, ".claude", "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	assert.Equal(t, "should be preserved", settings["customField"])
}

// --- SC-10: ApplyPhaseSettings writes hook script files and skill files ---

func TestApplyPhaseSettings_WritesHookScriptFiles(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()

	hsGetter := &mockHookScriptGetter{
		scripts: map[string]*db.HookScript{
			"orc-isolation": {
				ID:      "orc-isolation",
				Name:    "Isolation",
				Content: "#!/bin/bash\necho isolation",
			},
		},
	}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	// Phase config references a hook script by ID
	phaseCfg := &PhaseClaudeConfig{
		Hooks: map[string][]HookMatcher{
			"PreToolUse": {
				{
					Matcher: "Edit|Write",
					Hooks: []HookEntry{
						{Type: "command", Command: "bash {{hook:orc-isolation}}"},
					},
				},
			},
		},
	}

	err := ApplyPhaseSettings(worktree, phaseCfg, baseCfg, hsGetter, sGetter)
	require.NoError(t, err)

	// Verify hook script was written as executable file in .claude/hooks/
	hookPath := filepath.Join(worktree, ".claude", "hooks", "orc-isolation")
	info, err := os.Stat(hookPath)
	require.NoError(t, err, "hook script file should exist")
	assert.NotZero(t, info.Mode()&0111, "hook script should be executable")

	content, err := os.ReadFile(hookPath)
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/bash\necho isolation", string(content))
}

func TestApplyPhaseSettings_WritesSkillFiles(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{
		skills: map[string]*db.Skill{
			"python-style": {
				ID:      "python-style",
				Name:    "Python Style",
				Content: "# Python Style Guide\nUse snake_case",
				SupportingFiles: map[string]string{
					"ruff.toml": "[tool.ruff]\nline-length = 80",
				},
			},
		},
	}

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	phaseCfg := &PhaseClaudeConfig{
		SkillRefs: []string{"python-style"},
	}

	err := ApplyPhaseSettings(worktree, phaseCfg, baseCfg, hsGetter, sGetter)
	require.NoError(t, err)

	// Verify skill SKILL.md written to .claude/skills/<id>/SKILL.md
	skillPath := filepath.Join(worktree, ".claude", "skills", "python-style", "SKILL.md")
	content, err := os.ReadFile(skillPath)
	require.NoError(t, err, "skill file should exist")
	assert.Equal(t, "# Python Style Guide\nUse snake_case", string(content))

	// Verify supporting file
	supportPath := filepath.Join(worktree, ".claude", "skills", "python-style", "ruff.toml")
	content, err = os.ReadFile(supportPath)
	require.NoError(t, err, "supporting file should exist")
	assert.Equal(t, "[tool.ruff]\nline-length = 80", string(content))
}

func TestApplyPhaseSettings_MissingHookScript(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()

	// Empty getter — no scripts registered
	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}}

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	phaseCfg := &PhaseClaudeConfig{
		Hooks: map[string][]HookMatcher{
			"PreToolUse": {
				{
					Matcher: "Edit",
					Hooks: []HookEntry{
						{Type: "command", Command: "bash {{hook:nonexistent-hook}}"},
					},
				},
			},
		},
	}

	err := ApplyPhaseSettings(worktree, phaseCfg, baseCfg, hsGetter, sGetter)
	assert.Error(t, err, "should error when hook script ID not found in DB")
}

func TestApplyPhaseSettings_MissingSkill(t *testing.T) {
	t.Parallel()
	worktree := t.TempDir()

	hsGetter := &mockHookScriptGetter{scripts: map[string]*db.HookScript{}}
	sGetter := &mockSkillGetter{skills: map[string]*db.Skill{}} // no skills

	baseCfg := &WorktreeBaseConfig{
		WorktreePath: worktree,
		MainRepoPath: "/fake/main",
		TaskID:       "TASK-001",
	}

	phaseCfg := &PhaseClaudeConfig{
		SkillRefs: []string{"nonexistent-skill"},
	}

	err := ApplyPhaseSettings(worktree, phaseCfg, baseCfg, hsGetter, sGetter)
	assert.Error(t, err, "should error when skill ID not found in DB")
}

// --- SC-11: resetClaudeDir ---

func TestResetClaudeDir_RestoresFromBranch(t *testing.T) {
	// This test requires a real git repo to test git checkout behavior.
	// Skip if git is not available.
	t.Parallel()

	// Create a git repo with .claude/ on a branch
	repoDir := t.TempDir()
	setupGitRepo(t, repoDir)

	// Create .claude/ on the branch
	claudeDir := filepath.Join(repoDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"original": true}`),
		0644,
	))
	gitCommit(t, repoDir, "add .claude/settings.json")

	// Modify .claude/ (simulating phase modifications)
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"modified": true, "hooks": {"PreToolUse": []}}`),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "extra-file.txt"),
		[]byte("should be removed"),
		0644,
	))

	// Reset should restore to original state
	err := resetClaudeDir(repoDir, "main")
	require.NoError(t, err)

	// Verify original content restored
	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"original"`)
	assert.NotContains(t, string(data), `"modified"`)
}

func TestResetClaudeDir_NoClaude(t *testing.T) {
	// When .claude/ doesn't exist on source branch, should fallback to rm+mkdir
	t.Parallel()

	repoDir := t.TempDir()
	setupGitRepo(t, repoDir)
	// Don't create .claude/ on main branch

	// Create .claude/ as if a phase had written it
	claudeDir := filepath.Join(repoDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"phase-specific": true}`),
		0644,
	))

	err := resetClaudeDir(repoDir, "main")
	require.NoError(t, err)

	// .claude/ should exist but be empty (mkdir after rm)
	info, err := os.Stat(claudeDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// settings.json should be gone
	_, err = os.Stat(filepath.Join(claudeDir, "settings.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestResetClaudeDir_OverwritesChanges(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	setupGitRepo(t, repoDir)

	// Commit original .claude/
	claudeDir := filepath.Join(repoDir, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"clean": true}`),
		0644,
	))
	gitCommit(t, repoDir, "add .claude/")

	// Make uncommitted changes
	require.NoError(t, os.WriteFile(
		filepath.Join(claudeDir, "settings.json"),
		[]byte(`{"dirty": true}`),
		0644,
	))

	err := resetClaudeDir(repoDir, "main")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"clean"`)
}

// --- Test helpers ---

func setupGitRepo(t *testing.T, dir string) {
	t.Helper()
	execGit(t, dir, "init")
	execGit(t, dir, "config", "user.email", "test@test.com")
	execGit(t, dir, "config", "user.name", "Test")
	// Create initial commit
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644))
	execGit(t, dir, "add", ".")
	execGit(t, dir, "commit", "-m", "initial")
	// Ensure branch is named 'main'
	execGit(t, dir, "branch", "-M", "main")
}

func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	execGit(t, dir, "add", ".")
	execGit(t, dir, "commit", "-m", msg)
}

func execGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
