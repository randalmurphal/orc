package git

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BranchName returns the full branch name for a task.
// Uses executor prefix in p2p/team mode for isolated branches.
func (g *Git) BranchName(taskID string) string {
	return BranchName(taskID, g.executorPrefix)
}

// BranchNameWithInitiativePrefix returns the full branch name for a task with an initiative prefix.
// When initiativePrefix is non-empty, it replaces the default "orc/" prefix.
// Uses executor prefix in p2p/team mode for isolated branches.
func (g *Git) BranchNameWithInitiativePrefix(taskID, initiativePrefix string) string {
	return BranchNameWithPrefix(taskID, g.executorPrefix, initiativePrefix)
}

// tryCreateWorktree attempts to create a worktree, handling stale registrations.
// If the initial attempt fails, it prunes stale worktree entries and retries.
// This handles the case where a worktree directory was deleted but git still has
// a stale registration for it.
//
// This is a compound operation protected by mutex to prevent concurrent worktree
// creation from interfering with each other (e.g., both pruning at the same time).
func (g *Git) tryCreateWorktree(branchName, worktreePath, baseBranch string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// First attempt: create worktree with new branch
	output, err := g.ctx.RunGit("worktree", "add", "-b", branchName, worktreePath, baseBranch)
	if err == nil {
		return output, nil
	}

	// Branch might already exist, try to add worktree for existing branch
	output, err = g.ctx.RunGit("worktree", "add", worktreePath, branchName)
	if err == nil {
		return output, nil
	}

	// Both attempts failed - check if this might be a stale worktree issue
	// Prune stale worktree entries (directories that no longer exist)
	_, _ = g.ctx.RunGit("worktree", "prune")

	// Retry: create worktree with new branch
	output, err = g.ctx.RunGit("worktree", "add", "-b", branchName, worktreePath, baseBranch)
	if err == nil {
		return output, nil
	}

	// Retry: add worktree for existing branch
	output, err = g.ctx.RunGit("worktree", "add", worktreePath, branchName)
	if err == nil {
		return output, nil
	}

	return "", err
}

// CreateWorktree creates an isolated worktree for a task.
// Returns the absolute path to the worktree.
// Uses executor prefix in p2p/team mode for isolated worktrees.
// NOTE: This does NOT modify the main repo's checked-out branch.
//
// After creation, safety hooks are injected into the worktree that:
// - Block pushes to protected branches (main, master, develop, release)
// - Warn if commits are made on unexpected branches
//
// If a stale worktree registration exists (directory deleted but git still tracks it),
// this function will automatically prune stale entries and retry.
func (g *Git) CreateWorktree(taskID, baseBranch string) (string, error) {
	branchName := g.BranchName(taskID)
	worktreePath := WorktreePath(filepath.Join(g.ctx.RepoPath(), g.worktreeDir), taskID, g.executorPrefix)

	// Ensure worktrees directory exists
	worktreesDir := filepath.Join(g.ctx.RepoPath(), g.worktreeDir)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}

	// Try to create worktree, handling stale registrations
	_, err := g.tryCreateWorktree(branchName, worktreePath, baseBranch)
	if err != nil {
		return "", fmt.Errorf("create worktree for %s: %w", taskID, err)
	}

	// Inject safety hooks into the worktree
	hookCfg := HookConfig{
		ProtectedBranches: g.protectedBranches,
		TaskBranch:        branchName,
		TaskID:            taskID,
	}
	if err := g.InjectWorktreeHooks(worktreePath, hookCfg); err != nil {
		// FATAL: Hooks are safety-critical. Without them, the worktree lacks protection
		// against operations on wrong branches. This must be resolved before continuing.
		return "", fmt.Errorf("failed to inject worktree safety hooks (worktree not safe to use without branch protection): %w", err)
	}

	// Ensure .claude/settings.json is untracked before injecting hooks.
	// This is critical for resumed tasks where the branch may have stale
	// machine-specific hooks committed. By untracking and excluding the file,
	// we prevent dirty working tree state that would block git rebase/merge.
	if err := EnsureClaudeSettingsUntracked(worktreePath); err != nil {
		// Log warning but don't fail - sync might still work if file isn't tracked
		fmt.Fprintf(os.Stderr, "\n⚠️  WARNING: Failed to untrack .claude/settings.json: %v\n", err)
		fmt.Fprintf(os.Stderr, "   Git operations like rebase may fail if the file was previously committed.\n\n")
	}

	// Inject Claude Code hooks for worktree isolation
	// These PreToolUse hooks block file operations outside the worktree,
	// preventing accidental modification of the main repository.
	// Also injects user's env vars from ~/.claude/settings.json for PATH, VIRTUAL_ENV, etc.
	claudeHookCfg := ClaudeCodeHookConfig{
		WorktreePath:  worktreePath,
		MainRepoPath:  g.ctx.RepoPath(),
		TaskID:        taskID,
		InjectUserEnv: true, // Load env vars from user's ~/.claude/settings.json
	}
	if err := InjectClaudeCodeHooks(claudeHookCfg); err != nil {
		// Log warning but don't fail - this is defense in depth
		fmt.Fprintf(os.Stderr, "\n⚠️  WARNING: Failed to inject Claude Code isolation hooks: %v\n", err)
		fmt.Fprintf(os.Stderr, "   File operations may not be restricted to the worktree.\n\n")
	}

	return worktreePath, nil
}

// CreateWorktreeWithInitiativePrefix creates an isolated worktree for a task with initiative branch prefix support.
// Returns the absolute path to the worktree.
//
// When initiativePrefix is non-empty (e.g., "feature/auth-"), it replaces the default "orc/" prefix:
//   - Branch name: feature/auth-TASK-001 instead of orc/TASK-001
//   - Worktree dir: feature-auth-TASK-001 instead of orc-TASK-001
//
// This allows tasks belonging to an initiative to be grouped under a custom branch namespace.
//
// After creation, safety hooks are injected into the worktree that:
//   - Block pushes to protected branches (main, master, develop, release)
//   - Warn if commits are made on unexpected branches
//
// Hook injection failure is fatal - the function returns an error if hooks cannot be installed,
// as the worktree would lack branch protection.
func (g *Git) CreateWorktreeWithInitiativePrefix(taskID, baseBranch, initiativePrefix string) (string, error) {
	branchName := g.BranchNameWithInitiativePrefix(taskID, initiativePrefix)
	worktreePath := WorktreePathWithPrefix(filepath.Join(g.ctx.RepoPath(), g.worktreeDir), taskID, g.executorPrefix, initiativePrefix)

	// Ensure worktrees directory exists
	worktreesDir := filepath.Join(g.ctx.RepoPath(), g.worktreeDir)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}

	// Try to create worktree, handling stale registrations
	_, err := g.tryCreateWorktree(branchName, worktreePath, baseBranch)
	if err != nil {
		return "", fmt.Errorf("create worktree for %s: %w", taskID, err)
	}

	// Inject safety hooks into the worktree
	hookCfg := HookConfig{
		ProtectedBranches: g.protectedBranches,
		TaskBranch:        branchName,
		TaskID:            taskID,
	}
	if err := g.InjectWorktreeHooks(worktreePath, hookCfg); err != nil {
		// FATAL: Hooks are safety-critical. Without them, the worktree lacks protection
		// against operations on wrong branches. This must be resolved before continuing.
		return "", fmt.Errorf("failed to inject worktree safety hooks (worktree not safe to use without branch protection): %w", err)
	}

	// Ensure .claude/settings.json is untracked before injecting hooks.
	if err := EnsureClaudeSettingsUntracked(worktreePath); err != nil {
		fmt.Fprintf(os.Stderr, "\n⚠️  WARNING: Failed to untrack .claude/settings.json: %v\n", err)
		fmt.Fprintf(os.Stderr, "   Git operations like rebase may fail if the file was previously committed.\n\n")
	}

	// Inject Claude Code hooks for worktree isolation
	// Also injects user's env vars from ~/.claude/settings.json for PATH, VIRTUAL_ENV, etc.
	claudeHookCfg := ClaudeCodeHookConfig{
		WorktreePath:  worktreePath,
		MainRepoPath:  g.ctx.RepoPath(),
		TaskID:        taskID,
		InjectUserEnv: true, // Load env vars from user's ~/.claude/settings.json
	}
	if err := InjectClaudeCodeHooks(claudeHookCfg); err != nil {
		fmt.Fprintf(os.Stderr, "\n⚠️  WARNING: Failed to inject Claude Code isolation hooks: %v\n", err)
		fmt.Fprintf(os.Stderr, "   File operations may not be restricted to the worktree.\n\n")
	}

	return worktreePath, nil
}

// CleanupWorktree removes a task's worktree.
// Note: This uses the default worktree path calculation. For initiative-prefixed
// worktrees, use CleanupWorktreeAtPath with the actual path.
func (g *Git) CleanupWorktree(taskID string) error {
	worktreePath := WorktreePath(filepath.Join(g.ctx.RepoPath(), g.worktreeDir), taskID, g.executorPrefix)

	if err := g.ctx.CleanupWorktree(worktreePath); err != nil {
		return fmt.Errorf("cleanup worktree for %s: %w", taskID, err)
	}

	return nil
}

// CleanupWorktreeAtPath removes a worktree at the specified path.
// This is the preferred method when the exact worktree path is known,
// as it handles initiative-prefixed worktrees correctly.
func (g *Git) CleanupWorktreeAtPath(worktreePath string) error {
	if worktreePath == "" {
		return nil // Nothing to clean up
	}

	if err := g.ctx.CleanupWorktree(worktreePath); err != nil {
		return fmt.Errorf("cleanup worktree at %s: %w", worktreePath, err)
	}

	return nil
}

// PruneWorktrees removes stale worktree entries from git's internal tracking.
// Stale entries occur when a worktree directory is deleted without using
// `git worktree remove`. This is safe to call at any time.
func (g *Git) PruneWorktrees() error {
	_, err := g.ctx.RunGit("worktree", "prune")
	if err != nil {
		return fmt.Errorf("prune worktrees: %w", err)
	}
	return nil
}

// WorktreePath returns the path to a task's worktree.
// Uses executor prefix in p2p/team mode for isolated worktrees.
func (g *Git) WorktreePath(taskID string) string {
	return WorktreePath(filepath.Join(g.ctx.RepoPath(), g.worktreeDir), taskID, g.executorPrefix)
}

// WorktreePathWithInitiativePrefix returns the path to a task's worktree with initiative prefix support.
// When initiativePrefix is non-empty, it's used in the worktree directory name.
func (g *Git) WorktreePathWithInitiativePrefix(taskID, initiativePrefix string) string {
	return WorktreePathWithPrefix(filepath.Join(g.ctx.RepoPath(), g.worktreeDir), taskID, g.executorPrefix, initiativePrefix)
}

// InWorktree returns a Git instance operating in the specified worktree.
// The returned instance is marked as being in worktree context.
//
// The new instance gets its own mutex (zero-value, unlocked) since it operates
// on a different directory and should not contend with the parent instance.
func (g *Git) InWorktree(worktreePath string) *Git {
	return &Git{
		// mu is intentionally not copied - each instance gets its own mutex
		ctx:               g.ctx.InWorktree(worktreePath),
		branchPrefix:      g.branchPrefix,
		commitPrefix:      g.commitPrefix,
		worktreeDir:       g.worktreeDir,
		executorPrefix:    g.executorPrefix,
		inWorktreeContext: true,
		protectedBranches: g.protectedBranches,
	}
}

// IsInWorktreeContext returns true if this Git instance is operating within a worktree.
func (g *Git) IsInWorktreeContext() bool {
	return g.inWorktreeContext
}

// CreateCheckpoint creates a checkpoint commit for a phase.
//
// This is a compound operation (stage + commit) protected by mutex to ensure
// atomicity when multiple goroutines might be creating checkpoints.
// SAFETY: Requires worktree context - commits should only happen in worktrees.
func (g *Git) CreateCheckpoint(taskID, phase, message string) (*Checkpoint, error) {
	if err := g.RequireWorktreeContext("git commit (checkpoint)"); err != nil {
		return nil, err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Stage all changes
	if err := g.ctx.StageAll(); err != nil {
		return nil, fmt.Errorf("stage changes: %w", err)
	}

	// Build commit message
	commitMsg := fmt.Sprintf("%s %s: %s - %s", g.commitPrefix, taskID, phase, message)

	// Try to commit
	err := g.ctx.Commit(commitMsg)
	if err != nil {
		// If nothing to commit, create empty commit for checkpoint
		if err == ErrNothingToCommit {
			if _, runErr := g.ctx.RunGit("commit", "--allow-empty", "-m", commitMsg); runErr != nil {
				return nil, fmt.Errorf("create empty checkpoint: %w", runErr)
			}
		} else {
			return nil, fmt.Errorf("create commit: %w", err)
		}
	}

	// Get commit SHA
	sha, err := g.ctx.HeadCommit()
	if err != nil {
		return nil, fmt.Errorf("get commit SHA: %w", err)
	}

	return &Checkpoint{
		TaskID:    taskID,
		Phase:     phase,
		CommitSHA: sha,
		Message:   message,
		CreatedAt: time.Now(),
	}, nil
}
