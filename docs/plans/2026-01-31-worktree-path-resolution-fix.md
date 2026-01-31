# Worktree Path Resolution Fix

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Consolidate all git ops creation in CLI commands behind a single helper that uses `config.ResolveWorktreeDir`, eliminating the split between commands that use `DefaultConfig()` (wrong path) and those that build config manually (correct path).

**Architecture:** Create `NewGitOpsFromConfig()` in `internal/cli/git_helpers.go` as the ONE way to create `git.Git` in CLI code. Replace all 9 callsites (6 wrong + 3 correct-but-duplicated). Change `git.DefaultConfig().WorktreeDir` to empty string so it's obviously incomplete. Fix test that asserts wrong fallback behavior. Add CLAUDE.md guardrails.

**Tech Stack:** Go, internal packages (`git`, `config`, `cli`)

---

### Task 1: Create the CLI git helper

**Files:**
- Create: `internal/cli/git_helpers.go`
- Test: `internal/cli/git_helpers_test.go`

**Step 1: Write the failing test**

Create `internal/cli/git_helpers_test.go`:

```go
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

func TestNewGitOpsFromConfig_ResolvesWorktreeDir(t *testing.T) {
	t.Parallel()

	// Create a temporary git repo
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(homeDir, 0755)
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)
	t.Setenv("HOME", homeDir)

	cfg := &config.Config{
		BranchPrefix: "orc/",
		CommitPrefix: "[orc]",
		Worktree: config.WorktreeConfig{
			Enabled: true,
			Dir:     "/custom/worktrees",
		},
	}

	gitOps, err := NewGitOpsFromConfig(repoDir, cfg)
	if err != nil {
		t.Fatalf("NewGitOpsFromConfig() error: %v", err)
	}

	// The worktree path should use the resolved dir, not .orc/worktrees
	path := gitOps.WorktreePath("TASK-001")
	if filepath.Dir(path) != "/custom/worktrees" {
		t.Errorf("WorktreePath base = %q, want /custom/worktrees", filepath.Dir(path))
	}
}

func TestNewGitOpsFromConfig_NilConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)

	gitOps, err := NewGitOpsFromConfig(tmpDir, nil)
	if err != nil {
		t.Fatalf("NewGitOpsFromConfig() error: %v", err)
	}

	// Should still work with nil config (uses defaults)
	if gitOps == nil {
		t.Error("NewGitOpsFromConfig() returned nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd <worktree> && go test ./internal/cli/ -run TestNewGitOpsFromConfig -v -count=1`
Expected: FAIL — `NewGitOpsFromConfig` undefined

**Step 3: Write the implementation**

Create `internal/cli/git_helpers.go`:

```go
package cli

import (
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
)

// NewGitOpsFromConfig creates a git.Git instance with properly resolved
// worktree directory from orc config. This is the ONLY way CLI commands
// should create git.Git instances.
//
// All git configuration (branch prefix, commit prefix, worktree dir,
// executor prefix) is derived from the orc config, ensuring consistency
// across all CLI commands.
func NewGitOpsFromConfig(projectRoot string, cfg *config.Config) (*git.Git, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	gitCfg := git.Config{
		BranchPrefix:   cfg.BranchPrefix,
		CommitPrefix:   cfg.CommitPrefix,
		WorktreeDir:    config.ResolveWorktreeDir(cfg.Worktree.Dir, projectRoot),
		ExecutorPrefix: cfg.ExecutorPrefix(),
	}
	return git.New(projectRoot, gitCfg)
}
```

**Step 4: Run test to verify it passes**

Run: `cd <worktree> && go test ./internal/cli/ -run TestNewGitOpsFromConfig -v -count=1`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/cli/git_helpers.go internal/cli/git_helpers_test.go
git commit -m "feat(cli): add NewGitOpsFromConfig helper for consistent git ops creation"
```

---

### Task 2: Replace DefaultConfig() in cmd_run.go and cmd_resume.go

These are the two critical paths that actually create worktrees.

**Files:**
- Modify: `internal/cli/cmd_run.go:237`
- Modify: `internal/cli/cmd_resume.go:212`

**Step 1: Update cmd_run.go**

Replace:
```go
gitOps, err := git.New(projectRoot, git.DefaultConfig())
```
With:
```go
gitOps, err := NewGitOpsFromConfig(projectRoot, orcConfig)
```

Remove the `git` import if it becomes unused (it won't — other usages may exist; check).

**Step 2: Update cmd_resume.go**

Same replacement. The config variable name may differ — check context. In `cmd_resume.go`, the orc config is likely loaded earlier in the function. Find it and use it.

**Step 3: Run tests**

Run: `cd <worktree> && go test ./internal/cli/ -v -count=1 -short`
Expected: PASS (some tests may need updating — see Task 5)

**Step 4: Commit**

```bash
git add internal/cli/cmd_run.go internal/cli/cmd_resume.go
git commit -m "fix(cli): use NewGitOpsFromConfig in run and resume commands

These were using git.DefaultConfig() which hardcodes .orc/worktrees (relative),
causing worktrees to be created inside the project directory instead of
~/.orc/worktrees/<project-id>/."
```

---

### Task 3: Replace DefaultConfig() in remaining CLI commands

**Files:**
- Modify: `internal/cli/cmd_finalize.go:127`
- Modify: `internal/cli/cmd_staging.go:110`
- Modify: `internal/cli/cmd_staging.go:175`
- Modify: `internal/cli/cmd_branches.go:145`
- Modify: `internal/cli/cmd_branches.go:250`

**Step 1: Update each file**

Same pattern as Task 2. For each `git.New(projectRoot, git.DefaultConfig())`, replace with `NewGitOpsFromConfig(projectRoot, cfg)`.

**Important:** Check what the orc config variable is named in each command's scope. It may be `cfg`, `orcConfig`, or loaded inline. Each command loads config slightly differently — trace the variable.

**Step 2: Run tests**

Run: `cd <worktree> && go test ./internal/cli/ -v -count=1 -short`

**Step 3: Commit**

```bash
git add internal/cli/cmd_finalize.go internal/cli/cmd_staging.go internal/cli/cmd_branches.go
git commit -m "fix(cli): use NewGitOpsFromConfig in finalize, staging, and branches commands"
```

---

### Task 4: Deduplicate the correct callsites

These commands already build the git config correctly but duplicate the 4-line pattern. Replace with the helper.

**Files:**
- Modify: `internal/cli/cmd_cleanup.go:68-75`
- Modify: `internal/cli/cmd_resolve.go:247-253`
- Modify: `internal/cli/cmd_orchestrate.go:84-91`

**Step 1: Replace inline config building with helper**

For `cmd_cleanup.go` (lines 68-75), replace:
```go
gitCfg := git.Config{
    BranchPrefix:   cfg.BranchPrefix,
    CommitPrefix:   cfg.CommitPrefix,
    WorktreeDir:    config.ResolveWorktreeDir(cfg.Worktree.Dir, projectRoot),
    ExecutorPrefix: cfg.ExecutorPrefix(),
}
gitOps, err := git.New(projectRoot, gitCfg)
```
With:
```go
gitOps, err := NewGitOpsFromConfig(projectRoot, cfg)
```

Same for `cmd_resolve.go` and `cmd_orchestrate.go`. Note: `cmd_orchestrate.go` uses `cwd` and `"."` instead of `projectRoot` — normalize it to use `projectRoot` or `cwd` consistently (check what's available in scope).

**Step 2: Clean up unused imports**

Remove `config` import from files that no longer call `ResolveWorktreeDir` directly (if that was the only usage). Remove `git` import if no longer needed.

**Step 3: Run tests**

Run: `cd <worktree> && go test ./internal/cli/ -v -count=1 -short`

**Step 4: Commit**

```bash
git add internal/cli/cmd_cleanup.go internal/cli/cmd_resolve.go internal/cli/cmd_orchestrate.go
git commit -m "refactor(cli): deduplicate git config building with NewGitOpsFromConfig"
```

---

### Task 5: Fix DefaultConfig() and the helpers.go worktree path

**Files:**
- Modify: `internal/git/git.go` (DefaultConfig)
- Modify: `internal/cli/helpers.go:56-58`

**Step 1: Change DefaultConfig WorktreeDir to empty string**

In `internal/git/git.go`, change:
```go
func DefaultConfig() Config {
    return Config{
        BranchPrefix:      "orc/",
        CommitPrefix:      "[orc]",
        WorktreeDir:       ".orc/worktrees",
        ProtectedBranches: DefaultProtectedBranches,
    }
}
```
To:
```go
func DefaultConfig() Config {
    return Config{
        BranchPrefix:      "orc/",
        CommitPrefix:      "[orc]",
        WorktreeDir:       "",
        ProtectedBranches: DefaultProtectedBranches,
    }
}
```

**Step 2: Fix helpers.go buildBlockedContextProto**

In `internal/cli/helpers.go:56-58`, the `cwd` used for worktree resolution is `os.Getwd()` which is fragile. It should use the project root from config context. But since this function only receives `(t, cfg)`, the fix is to use `config.ResolveWorktreeDir` correctly. Check if this function has access to project root — if not, add it as a parameter or use the git naming helper to derive the path.

Actually, looking at it: this function constructs a display path for the `orc status` blocked context. The path should match what the executor actually creates. The fix is to accept `projectRoot string` as a parameter and pass it through.

**Step 3: Run full test suite**

Run: `cd <worktree> && go test ./internal/... -v -count=1 -short`

This will likely break tests that depend on `DefaultConfig().WorktreeDir` being `.orc/worktrees`. Fix them as they surface — they should use explicit config, not rely on the default.

**Step 4: Commit**

```bash
git add internal/git/git.go internal/cli/helpers.go
git commit -m "fix(git): change DefaultConfig WorktreeDir to empty string

DefaultConfig().WorktreeDir was '.orc/worktrees' which is a relative path
resolving to the project directory. This was a trap — any caller using
DefaultConfig() without overriding WorktreeDir got the wrong location.
Empty string makes it obvious the caller must set it explicitly."
```

---

### Task 6: Fix tests

**Files:**
- Modify: `internal/cli/cmd_run_test.go:167-185`
- Modify: any other tests broken by DefaultConfig change (find via test run)

**Step 1: Fix cmd_run_test.go**

The test `TestBuildBlockedContext_WorktreeDefaultDir` at line 167 asserts:
```go
// With empty Dir, ResolveWorktreeDir falls back to <projectDir>/.orc/worktrees
expected := filepath.Join(cwd, ".orc", "worktrees") + "/orc-TASK-002"
```

This test encodes the wrong behavior. After Task 5 changes `buildBlockedContextProto` to accept project root, update this test to use a proper project root and assert the correct global path.

If `buildBlockedContextProto` now requires project root, update all callers in the test file.

**Step 2: Run the full test suite and fix any remaining failures**

Run: `cd <worktree> && go test ./internal/... -count=1 -short 2>&1 | head -100`

Fix failures iteratively. Common patterns:
- Tests using `DefaultConfig()` that relied on `.orc/worktrees` → set `WorktreeDir` explicitly
- Tests calling `buildBlockedContextProto` → pass project root

**Step 3: Run the full test suite clean**

Run: `cd <worktree> && go test ./internal/... -count=1 -short`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add -A
git commit -m "test: update tests for correct worktree path resolution"
```

---

### Task 7: Add CLAUDE.md guardrails

**Files:**
- Modify: `internal/cli/CLAUDE.md`
- Modify: `internal/git/CLAUDE.md` (if exists, otherwise `internal/CLAUDE.md`)

**Step 1: Add to internal/cli/CLAUDE.md**

Add a section after "## Command Pattern":

```markdown
## Git Operations

**ONE way to create git ops in CLI commands:**

```go
gitOps, err := NewGitOpsFromConfig(projectRoot, orcConfig)
```

Located in `git_helpers.go`. This resolves worktree dir via `config.ResolveWorktreeDir`, sets branch/commit prefix, and executor prefix from orc config.

**NEVER use `git.DefaultConfig()` in CLI commands.** It has no worktree dir configured and will create worktrees in the wrong location.

**NEVER inline git.Config construction.** All git configuration derivation from orc config belongs in `NewGitOpsFromConfig`.
```

**Step 2: Add to internal/CLAUDE.md**

In the "Key Patterns" section, add:

```markdown
### Construction Helpers

When multiple packages need the same object built from config, create ONE helper function and use it everywhere. Never let callers build the object inline — config fields get missed, defaults diverge, and bugs like "worktrees created in wrong directory" happen.

| Object | Helper | Location |
|--------|--------|----------|
| `git.Git` (CLI) | `NewGitOpsFromConfig()` | `internal/cli/git_helpers.go` |
| `git.Git` (API) | inline in `server.go` | `internal/api/server.go` |
```

**Step 3: Commit**

```bash
git add internal/cli/CLAUDE.md internal/CLAUDE.md
git commit -m "docs: add guardrails against inconsistent git ops construction"
```

---

### Task 8: Final verification

**Step 1: Run full backend test suite**

Run: `cd <worktree> && go test ./... -count=1 -short`
Expected: ALL PASS

**Step 2: Build**

Run: `cd <worktree> && go build ./cmd/orc/`
Expected: SUCCESS

**Step 3: Verify no remaining DefaultConfig() in CLI**

Run: `grep -rn 'git\.DefaultConfig()' internal/cli/ --include='*.go' | grep -v _test.go`
Expected: NO OUTPUT (zero matches in non-test CLI code)

**Step 4: Verify all CLI git.New calls use helper**

Run: `grep -rn 'git\.New(' internal/cli/ --include='*.go' | grep -v _test.go`
Expected: Only `git_helpers.go` should contain `git.New(` — no other CLI files.

**Step 5: Commit any final fixups, then squash or leave as-is per preference**
