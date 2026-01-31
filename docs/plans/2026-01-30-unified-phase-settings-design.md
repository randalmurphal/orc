# Unified Phase Settings Management

**Date:** 2026-01-30
**Status:** Design

## Problem Statement

Per-phase Claude Code configuration (hooks, MCP servers, skills, env) is managed through scattered, piecemeal injection:

- `InjectClaudeCodeHooks()` writes isolation + TDD hooks once at worktree creation
- `InjectMCPServersToWorktree()` patches settings.json per-phase with no revert
- `SkillLoader` hacks skill content into `--append-system-prompt` instead of writing real skill files
- TDD discipline hook is always injected but queries the DB to check if it should do anything
- No cleanup between phases — settings accumulate

This prevents adding new per-phase features (like a Stop hook for implement verification) without more special-casing.

## Solution

Unify all `.claude/` directory management into a single per-phase lifecycle:

```
For each phase:
  1. git checkout <source-branch> -- .claude/    # Reset to clean project state
  2. ApplyPhaseSettings(worktreePath, config)    # Write everything this phase needs
  3. executeWithClaude(...)                       # Run the phase
  4. git checkout <source-branch> -- .claude/    # Reset for next phase
```

No state persists between phases. Each phase declares what it needs via `PhaseClaudeConfig`. The same logic runs for every phase.

## Data Model

### PhaseClaudeConfig Changes

Add `Hooks` field. Fix `SkillRefs` to write real skill files instead of prompt injection:

```go
type PhaseClaudeConfig struct {
    // --- CLI flag fields (unchanged, handled by applyPhaseConfig → ClaudeExecutor) ---
    SystemPrompt       string
    AppendSystemPrompt string
    AllowedTools       []string
    DisallowedTools    []string
    Tools              []string
    MaxBudgetUSD       float64
    MaxTurns           int
    AgentRef           string
    InlineAgents       map[string]InlineAgentDef

    // --- Settings.json fields (written to .claude/settings.json per-phase) ---
    MCPServers map[string]claude.MCPServerConfig `json:"mcp_servers,omitempty"`
    Hooks      map[string][]HookMatcher           `json:"hooks,omitempty"`
    Env        map[string]string                   `json:"env,omitempty"`

    // --- Skill files (written to .claude/skills/ per-phase) ---
    SkillRefs []string `json:"skill_refs,omitempty"`

    // --- Removed ---
    // CaptureHookEvents - replaced by proper Hooks field
}
```

### New Types

```go
// HookMatcher mirrors Claude Code's hook configuration format exactly.
// Keys in PhaseClaudeConfig.Hooks are event names: "Stop", "PreToolUse", "PostToolUse", etc.
type HookMatcher struct {
    Matcher string      `json:"matcher,omitempty"` // Tool pattern (only for PreToolUse/PostToolUse)
    Hooks   []HookEntry `json:"hooks"`
}

type HookEntry struct {
    Type    string `json:"type"`              // "command" or "prompt"
    Command string `json:"command,omitempty"` // For type: "command"
    Prompt  string `json:"prompt,omitempty"`  // For type: "prompt"
    Timeout int    `json:"timeout,omitempty"` // Optional timeout in seconds
    Once    bool   `json:"once,omitempty"`    // Fire once then remove (skills only)
}
```

### WorktreeBaseConfig

Replaces `ClaudeCodeHookConfig`. Contains safety-critical settings injected for every phase:

```go
type WorktreeBaseConfig struct {
    WorktreePath  string            // Absolute path to worktree
    MainRepoPath  string            // Absolute path to main repo (for isolation blocking)
    TaskID        string            // For logging and context
    InjectUserEnv bool              // Load env vars from ~/.claude/settings.json
    AdditionalEnv map[string]string // Extra env vars (e.g., ORC_TASK_ID, ORC_DB_PATH)
}
```

## Settings.json Merge Strategy

`ApplyPhaseSettings` reads the project's `.claude/settings.json` (restored by git checkout) and layers on orc's configuration:

```
Project's .claude/settings.json (from source branch)
  ├── hooks: project hooks preserved
  │   += isolation PreToolUse hook (always, from base config)
  │   += phase hooks from PhaseClaudeConfig.Hooks
  ├── mcpServers: project servers preserved
  │   += phase MCP servers (phase wins on key collision)
  ├── env: project env preserved
  │   += user env from ~/.claude/settings.json (if InjectUserEnv)
  │   += phase env (phase wins on key collision)
  └── other fields: preserved as-is
```

**Hooks merge:** Append orc's matchers to existing project matchers for the same event. Never overwrite project hooks.

**MCP/env merge:** Phase config wins on key collision (same as current behavior).

## Hook Script Storage

Hook scripts follow the same pattern as agents:

1. **Embedded source:** `templates/hooks/*.sh` (or `*.py`)
2. **Seeded to GlobalDB:** `SeedHookScripts()` on startup, stored in `hook_scripts` table
3. **Written at runtime:** `ApplyPhaseSettings` writes referenced scripts to `.claude/hooks/`
4. **Cleaned up:** `git checkout` reset removes them

### DB Schema

```sql
CREATE TABLE hook_scripts (
    id TEXT PRIMARY KEY,        -- "orc-verify-completion"
    name TEXT NOT NULL,         -- Display name
    description TEXT,           -- What it does
    filename TEXT NOT NULL,     -- "orc-verify-completion.sh"
    content TEXT NOT NULL,      -- Full script content
    is_builtin BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

Phase templates reference hook scripts by ID in their `claude_config` JSON. The `command` field uses `$CLAUDE_PROJECT_DIR/.claude/hooks/<filename>` — `ApplyPhaseSettings` writes the script to that path.

### Built-in Hook Scripts

#### `orc-verify-completion.sh` (implement phase Stop hook)

Forces the implement agent to study the spec and verify all success criteria before stopping:

```bash
#!/bin/bash
# Implement phase verification hook.
# First stop: force re-verification against spec.
# Second stop: allow (stop_hook_active = true).

HOOK_INPUT=$(cat)
STOP_HOOK_ACTIVE=$(echo "$HOOK_INPUT" | jq -r '.stop_hook_active // false')

if [ "$STOP_HOOK_ACTIVE" = "true" ]; then
    exit 0
fi

jq -n '{
  "decision": "block",
  "reason": "BEFORE COMPLETING: Study the original specification and success criteria carefully. For each SC-X, verify you have implemented it and can point to the specific code that satisfies it. If this is a bug fix, grep for the buggy pattern across the entire codebase to ensure no instances were missed. Only stop when you have verified every criterion is met."
}'
```

#### `orc-tdd-discipline.sh` (tdd_write phase PreToolUse hook)

Existing TDD hook, simplified — no longer needs DB query since it only runs during tdd_write:

```bash
#!/bin/bash
# TDD discipline hook. Blocks non-test file writes.
# Only injected during tdd_write phase via phase config.

HOOK_INPUT=$(cat)
TOOL_NAME=$(echo "$HOOK_INPUT" | jq -r '.tool_name // empty')

case "$TOOL_NAME" in
    Write|Edit|MultiEdit) ;;
    *) exit 0 ;;
esac

FILE_PATH=$(echo "$HOOK_INPUT" | jq -r '.tool_input.file_path // empty')
if [ -z "$FILE_PATH" ]; then
    exit 0
fi

# [existing is_test_file() logic — unchanged]

if is_test_file "$FILE_PATH"; then
    exit 0
fi

jq -n --arg file "$FILE_PATH" '{
    "decision": "block",
    "reason": ("TDD discipline: During tdd_write phase, only test files can be modified. Blocked: " + $file + "\nWrite your tests first.")
}'
```

#### `orc-worktree-isolation.py` (all phases PreToolUse hook)

Existing isolation hook — moved from hardcoded generation to embedded template. Content unchanged.

## Skill Storage

Skills follow the same embedded → DB → runtime pattern:

1. **Embedded source:** `templates/skills/<name>/SKILL.md` (+ supporting files)
2. **Seeded to GlobalDB:** `SeedSkills()` on startup, stored in `skills` table
3. **Written at runtime:** `ApplyPhaseSettings` writes to `.claude/skills/<name>/SKILL.md`
4. **Cleaned up:** `git checkout` reset removes them

### DB Schema

```sql
CREATE TABLE skills (
    id TEXT PRIMARY KEY,          -- "python-style"
    name TEXT NOT NULL,           -- Display name
    description TEXT,             -- What the skill does
    content TEXT NOT NULL,        -- Full SKILL.md content (frontmatter + body)
    supporting_files JSON,        -- {"template.md": "content...", "scripts/validate.sh": "content..."}
    is_builtin BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

Phase templates reference skills by ID in their `claude_config.skill_refs`. `ApplyPhaseSettings` writes each skill's `SKILL.md` and supporting files to `.claude/skills/<id>/`.

## Phase Template Configuration Examples

### implement phase template `claude_config`:

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/orc-verify-completion.sh"
          }
        ]
      }
    ]
  }
}
```

### tdd_write phase template `claude_config`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/orc-tdd-discipline.sh"
          }
        ]
      }
    ]
  }
}
```

### Phase with MCP + hooks + skills:

```json
{
  "mcp_servers": {
    "playwright": {
      "command": "npx",
      "args": ["@anthropic/mcp-playwright"]
    }
  },
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "$CLAUDE_PROJECT_DIR/.claude/hooks/orc-verify-completion.sh"
          }
        ]
      }
    ]
  },
  "skill_refs": ["python-style"]
}
```

## Execution Flow

### Current (scattered):

```
setupWorktree()
├── InjectClaudeCodeHooks()        # Writes isolation + TDD hooks once
│
for each phase:
├── InjectMCPServersToWorktree()   # Patches MCP into existing settings
├── SkillLoader.LoadSkillsForConfig()  # Hacks skills into system prompt
├── executeWithClaude()
│   (no cleanup between phases)
│
cleanupWorktree()                  # Nukes everything
```

### New (unified):

```
setupWorktree()
├── (no settings injection at setup — each phase handles its own)
│
for each phase:
├── resetClaudeDir(worktreePath, sourceBranch)  # git checkout .claude/
├── ApplyPhaseSettings(worktreePath, phaseCfg, baseCfg)
│   ├── Read project's .claude/settings.json
│   ├── Merge isolation hooks (from base config)
│   ├── Merge phase hooks (from PhaseClaudeConfig.Hooks)
│   ├── Merge phase MCP servers
│   ├── Merge env vars
│   ├── Write merged settings.json
│   ├── Write hook scripts to .claude/hooks/
│   └── Write skills to .claude/skills/
├── executeWithClaude()
├── resetClaudeDir(worktreePath, sourceBranch)  # Clean for next phase
│
cleanupWorktree()
```

### `resetClaudeDir` Implementation

```go
func resetClaudeDir(worktreePath, sourceBranch string) error {
    // git checkout <source-branch> -- .claude/
    // This restores .claude/ to whatever is committed on the source branch.
    // If .claude/ doesn't exist on the branch, it removes injected files.
    cmd := exec.Command("git", "checkout", sourceBranch, "--", ".claude/")
    cmd.Dir = worktreePath
    return cmd.Run()
}
```

Edge case: if `.claude/` doesn't exist on the source branch, `git checkout` will error. Handle by falling back to `rm -rf .claude/` + recreating the directory.

## What Gets Removed

| Current Code | Replacement |
|---|---|
| `InjectClaudeCodeHooks()` | `ApplyPhaseSettings()` |
| `InjectMCPServersToWorktree()` | `ApplyPhaseSettings()` |
| `generateClaudeCodeSettings()` | `ApplyPhaseSettings()` |
| `RemoveClaudeCodeHooks()` (unused) | `resetClaudeDir()` |
| `ClaudeCodeHookConfig` struct | `WorktreeBaseConfig` |
| `SkillLoader.LoadSkillsForConfig()` | Real skill file writing |
| `worktreeSettings` struct | New merge logic in `ApplyPhaseSettings` |
| TDD hook DB query logic | Phase-scoped injection (no query needed) |

## Override Hierarchy

Follows existing pattern for other phase config:

```
phase_template.claude_config        # Template default (e.g., implement has Stop hook)
  → workflow_phase.claude_config_override  # Workflow-level override
  → Merged result                   # What ApplyPhaseSettings receives
```

This is already how `getEffectivePhaseClaudeConfig()` works. The `Hooks` field just merges alongside everything else.

## Migration

1. Add `Hooks` field to `PhaseClaudeConfig` + new types
2. Add `hook_scripts` and `skills` tables to GlobalDB
3. Implement `SeedHookScripts()` and `SeedSkills()`
4. Implement `ApplyPhaseSettings()` and `resetClaudeDir()`
5. Update `executePhase()` to call reset → apply → execute → reset
6. Move isolation hook to embedded template
7. Move TDD hook to tdd_write phase config
8. Add verify-completion hook to implement phase config
9. Remove old injection functions
10. Update `SkillRefs` handling to write files instead of prompt injection
11. Remove `CaptureHookEvents` field (replaced by `Hooks`)

## Testing

- Unit test `ApplyPhaseSettings` with various merge scenarios
- Unit test `resetClaudeDir` with existing/missing `.claude/`
- Integration test: phase hooks only present during their phase
- Integration test: skills written and cleaned up correctly
- E2E: run implement phase, verify Stop hook fires and forces re-verification
