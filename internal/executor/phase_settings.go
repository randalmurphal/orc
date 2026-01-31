package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
)

// HookScriptGetter retrieves hook scripts by ID.
type HookScriptGetter interface {
	GetHookScript(id string) (*db.HookScript, error)
}

// SkillGetter retrieves skills by ID.
type SkillGetter interface {
	GetSkill(id string) (*db.Skill, error)
}

// hookRefPattern matches {{hook:some-id}} references in hook commands.
var hookRefPattern = regexp.MustCompile(`\{\{hook:([^}]+)\}\}`)

// ApplyPhaseSettings writes the merged .claude/ configuration for a phase.
// It reads existing settings.json (if any), merges in phase hooks, MCP servers,
// env vars, writes hook script files, and writes skill files.
//
// The merge strategy for hooks is: project hooks (from existing settings.json)
// are preserved, and phase hooks are appended per event key.
func ApplyPhaseSettings(
	worktreePath string,
	phaseCfg *PhaseClaudeConfig,
	baseCfg *WorktreeBaseConfig,
	hsGetter HookScriptGetter,
	sGetter SkillGetter,
) error {
	// Verify worktree path exists
	if _, err := os.Stat(worktreePath); err != nil {
		return fmt.Errorf("worktree path: %w", err)
	}

	claudeDir := filepath.Join(worktreePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	// Read existing settings.json (if any)
	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings := make(map[string]any)

	data, err := os.ReadFile(settingsPath)
	if err == nil {
		// File exists — parse it
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse existing settings.json: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read settings.json: %w", err)
	}

	// Always write the isolation hook script (safety-critical, every phase needs it)
	if baseCfg != nil && hsGetter != nil {
		if err := writeIsolationScript(worktreePath, hsGetter); err != nil {
			return err
		}
	}

	// Collect all hook script IDs referenced in phase config, resolve them,
	// and write script files to .claude/hooks/
	if err := writeHookScriptFiles(worktreePath, phaseCfg, hsGetter); err != nil {
		return err
	}

	// Write skill files to .claude/skills/<id>/
	if err := writeSkillFiles(worktreePath, phaseCfg, sGetter); err != nil {
		return err
	}

	// Merge hooks into settings
	mergeHooksIntoSettings(settings, phaseCfg, baseCfg, worktreePath)

	// Merge MCP servers
	if phaseCfg != nil && len(phaseCfg.MCPServers) > 0 {
		existing, _ := settings["mcpServers"].(map[string]any)
		if existing == nil {
			existing = make(map[string]any)
		}
		for name, cfg := range phaseCfg.MCPServers {
			existing[name] = cfg
		}
		settings["mcpServers"] = existing
	}

	// Merge env vars
	mergeEnvVars(settings, phaseCfg, baseCfg)

	// Write the merged settings.json
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings.json: %w", err)
	}
	if err := os.WriteFile(settingsPath, output, 0644); err != nil {
		return fmt.Errorf("write settings.json: %w", err)
	}

	return nil
}

// mergeHooksIntoSettings merges phase hooks and isolation hooks into the settings map.
// Project hooks from existing settings.json are preserved; new hooks are appended.
func mergeHooksIntoSettings(settings map[string]any, phaseCfg *PhaseClaudeConfig, baseCfg *WorktreeBaseConfig, worktreePath string) {
	// Get existing hooks from settings
	existingHooks, _ := settings["hooks"].(map[string]any)
	if existingHooks == nil {
		existingHooks = make(map[string]any)
	}

	// Add phase hooks first (appending to existing project hooks, never replacing)
	if phaseCfg != nil {
		for event, matchers := range phaseCfg.Hooks {
			existing, _ := existingHooks[event].([]any)
			for _, m := range matchers {
				resolved := resolveHookMatcher(m, worktreePath)
				existing = append(existing, resolved)
			}
			existingHooks[event] = existing
		}
	}

	// Append isolation hook last (always added, runs after phase hooks)
	isolationScriptPath := filepath.Join(worktreePath, ".claude", "hooks", "orc-worktree-isolation.py")
	isolationHook := map[string]any{
		"matcher": "Edit|Write|Read|Glob|Grep|MultiEdit",
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": fmt.Sprintf(`ORC_WORKTREE_PATH="%s" ORC_MAIN_REPO_PATH="%s" ORC_TASK_ID="%s" python3 "%s"`, baseCfg.WorktreePath, baseCfg.MainRepoPath, baseCfg.TaskID, isolationScriptPath),
			},
		},
	}
	preToolUse, _ := existingHooks["PreToolUse"].([]any)
	preToolUse = append(preToolUse, isolationHook)
	existingHooks["PreToolUse"] = preToolUse

	settings["hooks"] = existingHooks
}

// resolveHookMatcher converts a HookMatcher to a map[string]any for JSON,
// resolving {{hook:id}} references in commands to actual file paths.
func resolveHookMatcher(m HookMatcher, worktreePath string) map[string]any {
	hooks := make([]any, 0, len(m.Hooks))
	for _, h := range m.Hooks {
		cmd := h.Command
		// Replace {{hook:id}} with actual path
		cmd = hookRefPattern.ReplaceAllStringFunc(cmd, func(match string) string {
			id := hookRefPattern.FindStringSubmatch(match)[1]
			return filepath.Join(worktreePath, ".claude", "hooks", id)
		})
		hooks = append(hooks, map[string]any{
			"type":    h.Type,
			"command": cmd,
		})
	}
	return map[string]any{
		"matcher": m.Matcher,
		"hooks":   hooks,
	}
}

// mergeEnvVars merges environment variables from base config and phase config into settings.
// Existing project env vars from settings.json are preserved; orc env vars are layered on top.
func mergeEnvVars(settings map[string]any, phaseCfg *PhaseClaudeConfig, baseCfg *WorktreeBaseConfig) {
	// Start with existing project env vars (preserve them)
	env := make(map[string]any)
	if existingEnv, ok := settings["env"].(map[string]any); ok {
		for k, v := range existingEnv {
			env[k] = v
		}
	}

	// Base config additional env (layers on top of project env)
	if baseCfg != nil {
		for k, v := range baseCfg.AdditionalEnv {
			env[k] = v
		}
	}

	// Phase config env (highest precedence)
	if phaseCfg != nil {
		for k, v := range phaseCfg.Env {
			env[k] = v
		}
	}

	if len(env) > 0 {
		settings["env"] = env
	}
}

// writeHookScriptFiles writes hook script files referenced in the phase config
// to .claude/hooks/ in the worktree.
func writeHookScriptFiles(worktreePath string, phaseCfg *PhaseClaudeConfig, hsGetter HookScriptGetter) error {
	if phaseCfg == nil {
		return nil
	}

	// Collect all hook script IDs referenced in commands
	hookIDs := collectHookScriptIDs(phaseCfg)
	if len(hookIDs) == 0 {
		return nil
	}

	hooksDir := filepath.Join(worktreePath, ".claude", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}

	for _, id := range hookIDs {
		if strings.Contains(id, "..") || strings.Contains(id, "/") || filepath.IsAbs(id) {
			return fmt.Errorf("invalid hook script ID %q: must not contain path separators", id)
		}
		hs, err := hsGetter.GetHookScript(id)
		if err != nil {
			return fmt.Errorf("get hook script %s: %w", id, err)
		}
		if hs == nil {
			return fmt.Errorf("hook script %q not found in database", id)
		}

		hookPath := filepath.Join(hooksDir, id)
		if err := os.WriteFile(hookPath, []byte(hs.Content), 0755); err != nil {
			return fmt.Errorf("write hook script %s: %w", id, err)
		}
	}

	return nil
}

// collectHookScriptIDs extracts all {{hook:id}} references from hook commands.
func collectHookScriptIDs(phaseCfg *PhaseClaudeConfig) []string {
	seen := make(map[string]bool)
	var ids []string

	for _, matchers := range phaseCfg.Hooks {
		for _, m := range matchers {
			for _, h := range m.Hooks {
				matches := hookRefPattern.FindAllStringSubmatch(h.Command, -1)
				for _, match := range matches {
					id := match[1]
					if !seen[id] {
						seen[id] = true
						ids = append(ids, id)
					}
				}
			}
		}
	}

	return ids
}

// writeSkillFiles writes skill files to .claude/skills/<id>/ in the worktree.
func writeSkillFiles(worktreePath string, phaseCfg *PhaseClaudeConfig, sGetter SkillGetter) error {
	if phaseCfg == nil || len(phaseCfg.SkillRefs) == 0 {
		return nil
	}

	for _, skillID := range phaseCfg.SkillRefs {
		if strings.Contains(skillID, "..") || strings.Contains(skillID, "/") || filepath.IsAbs(skillID) {
			return fmt.Errorf("invalid skill ID %q: must not contain path separators", skillID)
		}
		skill, err := sGetter.GetSkill(skillID)
		if err != nil {
			return fmt.Errorf("get skill %s: %w", skillID, err)
		}
		if skill == nil {
			return fmt.Errorf("skill %q not found in database", skillID)
		}

		skillDir := filepath.Join(worktreePath, ".claude", "skills", skillID)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return fmt.Errorf("create skill dir %s: %w", skillID, err)
		}

		// Write SKILL.md
		skillPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(skill.Content), 0644); err != nil {
			return fmt.Errorf("write skill %s: %w", skillID, err)
		}

		// Write supporting files
		for filename, content := range skill.SupportingFiles {
			// Prevent path traversal
			if strings.Contains(filename, "..") || filepath.IsAbs(filename) {
				return fmt.Errorf("invalid supporting filename %q in skill %s: must be relative without path traversal", filename, skillID)
			}
			filePath := filepath.Join(skillDir, filename)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write skill supporting file %s/%s: %w", skillID, filename, err)
			}
		}
	}

	return nil
}

// resetClaudeDir restores the .claude/ directory from the source branch.
// Uses `git checkout <sourceBranch> -- .claude/` to reset to clean state.
// Falls back to rm + mkdir if .claude/ doesn't exist on the source branch.
func resetClaudeDir(worktreePath, sourceBranch string) error {
	claudeDir := filepath.Join(worktreePath, ".claude")

	// Try git checkout to restore from source branch
	cmd := exec.Command("git", "checkout", sourceBranch, "--", ".claude/")
	cmd.Dir = worktreePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := strings.TrimSpace(string(output))
		// Check if the error is because .claude/ doesn't exist on the branch
		if strings.Contains(errMsg, "did not match any") ||
			strings.Contains(errMsg, "pathspec") ||
			strings.Contains(errMsg, "error: pathspec") {
			// .claude/ doesn't exist on source branch — fallback to rm + mkdir
			if err := os.RemoveAll(claudeDir); err != nil {
				return fmt.Errorf("remove .claude dir: %w", err)
			}
			if err := os.MkdirAll(claudeDir, 0755); err != nil {
				return fmt.Errorf("create .claude dir: %w", err)
			}
			return nil
		}
		return fmt.Errorf("git checkout %s -- .claude/: %s: %w", sourceBranch, errMsg, err)
	}

	// Clean up any extra files that were added but not on the branch
	// git checkout only restores tracked files, so remove untracked ones
	cmd = exec.Command("git", "clean", "-fd", ".claude/")
	cmd.Dir = worktreePath
	if cleanOutput, cleanErr := cmd.CombinedOutput(); cleanErr != nil {
		return fmt.Errorf("git clean .claude/: %s: %w", strings.TrimSpace(string(cleanOutput)), cleanErr)
	}

	return nil
}

// writeIsolationScript writes the worktree isolation hook script to .claude/hooks/.
// This is always written for every phase as a safety measure.
func writeIsolationScript(worktreePath string, hsGetter HookScriptGetter) error {
	isoScript, err := hsGetter.GetHookScript("orc-worktree-isolation")
	if err != nil {
		return fmt.Errorf("get isolation hook script: %w", err)
	}
	if isoScript == nil {
		// Isolation script not seeded yet — not a fatal error during early bootstrap
		return nil
	}

	hooksDir := filepath.Join(worktreePath, ".claude", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks dir for isolation script: %w", err)
	}

	scriptPath := filepath.Join(hooksDir, "orc-worktree-isolation.py")
	if err := os.WriteFile(scriptPath, []byte(isoScript.Content), 0755); err != nil {
		return fmt.Errorf("write isolation script: %w", err)
	}

	return nil
}
