package git

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// ErrMainRepoModification is returned when a destructive operation is attempted
// on the main repository instead of within a worktree context.
// This is a critical safety check - worktrees exist to isolate task execution.
var ErrMainRepoModification = errors.New("destructive operation on main repository blocked")

// ErrProtectedBranch is returned when attempting to push to a protected branch.
var ErrProtectedBranch = errors.New("push to protected branch blocked")

// CreateBranch creates a new branch for a task.
// NOTE: This function is primarily used in tests. Production worktree creation
// uses CreateWorktree/CreateWorktreeWithInitiativePrefix which handle branch
// creation internally.
func (g *Git) CreateBranch(taskID string) error {
	branchName := g.BranchName(taskID)
	if err := g.ctx.CreateBranch(branchName); err != nil {
		return fmt.Errorf("create branch %s: %w", branchName, err)
	}
	return g.ctx.Checkout(branchName)
}

// SwitchBranch switches to an existing task branch.
// NOTE: This function is primarily used in tests. Production code should use
// CheckoutSafe for worktree operations.
func (g *Git) SwitchBranch(taskID string) error {
	branchName := g.BranchName(taskID)
	return g.ctx.Checkout(branchName)
}

// BranchExists checks if a branch exists locally.
func (g *Git) BranchExists(branch string) (bool, error) {
	// Use git show-ref to check if branch exists
	_, err := g.ctx.RunGit("show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		// Exit code 1 means branch doesn't exist
		// Other errors should be returned
		if strings.Contains(err.Error(), "exit status 1") {
			return false, nil
		}
		return false, fmt.Errorf("check branch %s: %w", branch, err)
	}
	return true, nil
}

// CreateBranchFromBase creates a new branch from the specified base branch.
// Unlike CreateBranch, this allows specifying any base branch (not just current HEAD).
func (g *Git) CreateBranchFromBase(branch, baseBranch string) error {
	// First, make sure base branch is available (might need to fetch)
	if _, err := g.ctx.RunGit("rev-parse", "--verify", baseBranch); err != nil {
		// Try fetching the branch from origin
		_, fetchErr := g.ctx.RunGit("fetch", "origin", baseBranch+":"+baseBranch)
		if fetchErr != nil {
			// Also try with origin/ prefix in case it's a remote tracking branch
			_, fetchErr = g.ctx.RunGit("fetch", "origin", baseBranch)
			if fetchErr != nil {
				return fmt.Errorf("base branch %s not found locally or on remote: %w", baseBranch, err)
			}
		}
	}

	// Create the new branch from the base
	_, err := g.ctx.RunGit("branch", branch, baseBranch)
	if err != nil {
		return fmt.Errorf("create branch %s from %s: %w", branch, baseBranch, err)
	}
	return nil
}

// EnsureBranchExists creates a branch from base if it doesn't exist.
// This is useful for ensuring initiative or staging branches exist before
// creating task worktrees that target them.
//
// If the branch already exists locally, this is a no-op.
// If the branch exists on remote but not locally, it creates a local tracking branch.
// If the branch doesn't exist anywhere, it creates it from the base branch.
func (g *Git) EnsureBranchExists(branch, baseBranch string) error {
	// Check if branch exists locally
	exists, err := g.BranchExists(branch)
	if err != nil {
		return fmt.Errorf("check branch exists: %w", err)
	}
	if exists {
		return nil // Already exists, nothing to do
	}

	// Check if branch exists on remote
	remoteExists, err := g.RemoteBranchExists("origin", branch)
	if err != nil {
		// Log but don't fail - remote might not be accessible
		// We'll try to create from base anyway
	} else if remoteExists {
		// Create local tracking branch from remote
		_, err = g.ctx.RunGit("branch", "--track", branch, "origin/"+branch)
		if err != nil {
			return fmt.Errorf("create tracking branch %s: %w", branch, err)
		}
		return nil
	}

	// Branch doesn't exist anywhere - create from base
	return g.CreateBranchFromBase(branch, baseBranch)
}

// RequireWorktreeContext returns an error if this Git instance is NOT operating
// in a worktree context. This should be called before any destructive operations
// (reset, rebase, merge, checkout) to ensure we don't accidentally modify the main repo.
func (g *Git) RequireWorktreeContext(operation string) error {
	if !g.inWorktreeContext {
		return fmt.Errorf("%w: %s requires worktree context - refusing to modify main repository",
			ErrMainRepoModification, operation)
	}
	return nil
}

// RequireNonProtectedBranch returns an error if the current branch is protected.
// This is an additional safety check for operations that might affect protected branches.
func (g *Git) RequireNonProtectedBranch(operation string) error {
	currentBranch, err := g.GetCurrentBranch()
	if err != nil {
		// Can't determine branch - be safe and allow the operation
		// (this might happen in detached HEAD state)
		return nil
	}
	if IsProtectedBranch(currentBranch, g.protectedBranches) {
		return fmt.Errorf("%w: %s cannot be performed on protected branch '%s'",
			ErrMainRepoModification, operation, currentBranch)
	}
	return nil
}

// CheckoutSafe checks out a branch with worktree context protection.
// This should be used for any checkout that modifies the working tree state
// during automated operations.
//
// SAFETY: This operation requires worktree context to prevent accidental
// modification of the main repository.
func (g *Git) CheckoutSafe(branch string) error {
	// CRITICAL: Prevent checkout operations on main repo during automated tasks
	if err := g.RequireWorktreeContext("git checkout"); err != nil {
		return err
	}
	return g.ctx.Checkout(branch)
}

// Rewind resets to a specific commit.
// SAFETY: This operation requires worktree context to prevent accidental modification
// of the main repository. It also blocks resets on protected branches.
func (g *Git) Rewind(commitSHA string) error {
	// CRITICAL: Prevent reset operations on main repo
	if err := g.RequireWorktreeContext("git reset --hard"); err != nil {
		return err
	}
	// Additional safety: don't reset on protected branches
	if err := g.RequireNonProtectedBranch("git reset --hard"); err != nil {
		return err
	}

	_, err := g.ctx.RunGit("reset", "--hard", commitSHA)
	if err != nil {
		return fmt.Errorf("rewind to %s: %w", commitSHA, err)
	}
	return nil
}

// Push pushes the current branch to remote.
// Returns ErrProtectedBranch if attempting to push to a protected branch.
// SAFETY: Requires worktree context - automated pushes should only happen from worktrees.
func (g *Git) Push(remote, branch string, setUpstream bool) error {
	if err := g.RequireWorktreeContext("git push"); err != nil {
		return err
	}
	if IsProtectedBranch(branch, g.protectedBranches) {
		return fmt.Errorf("%w: cannot push to '%s' - use PR workflow instead", ErrProtectedBranch, branch)
	}
	return g.ctx.Push(remote, branch, setUpstream)
}

// PushForce pushes with --force-with-lease for safety.
// This is safer than --force as it fails if the remote has unexpected commits
// (i.e., commits that weren't fetched yet).
// SAFETY: Requires worktree context and will NOT push to protected branches.
func (g *Git) PushForce(remote, branch string, setUpstream bool) error {
	if err := g.RequireWorktreeContext("git push --force"); err != nil {
		return err
	}
	if IsProtectedBranch(branch, g.protectedBranches) {
		return fmt.Errorf("%w: cannot force push to '%s'", ErrProtectedBranch, branch)
	}
	// Use --force-with-lease instead of --force for extra safety
	args := []string{"push", "--force-with-lease"}
	if setUpstream {
		args = append(args, "-u")
	}
	args = append(args, remote, branch)
	_, err := g.ctx.RunGit(args...)
	return err
}

// PushWithForceFallback attempts a normal push, and if it fails with a
// non-fast-forward error (divergent history), retries with --force-with-lease.
// This is designed for task branches that may have been rebased, causing
// divergence with the remote feature branch.
//
// SAFETY:
// - Requires worktree context
// - NEVER uses force push on protected branches (returns ErrProtectedBranch)
// - Uses --force-with-lease, not --force (fails if remote has unexpected changes)
//
// When force push is used, a warning is logged if logger is non-nil.
func (g *Git) PushWithForceFallback(remote, branch string, setUpstream bool, logger *slog.Logger) error {
	// CRITICAL: Require worktree context for all push operations
	if err := g.RequireWorktreeContext("git push"); err != nil {
		return err
	}

	// CRITICAL: Never force push to protected branches
	if IsProtectedBranch(branch, g.protectedBranches) {
		return fmt.Errorf("%w: cannot push to '%s'", ErrProtectedBranch, branch)
	}

	// Try normal push first
	err := g.ctx.Push(remote, branch, setUpstream)
	if err == nil {
		return nil
	}

	// Check if this is a non-fast-forward error (divergent history)
	if !IsNonFastForwardError(err) {
		// Not a divergence issue - return the original error
		return err
	}

	// Log warning about force push
	if logger != nil {
		logger.Warn("push failed with non-fast-forward, retrying with --force-with-lease",
			"branch", branch,
			"reason", "divergent history from previous run")
	}

	// Retry with --force-with-lease
	return g.PushForce(remote, branch, setUpstream)
}

// IsNonFastForwardError checks if a push error is due to non-fast-forward
// (divergent history) that can be resolved with force push.
func IsNonFastForwardError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "non-fast-forward") ||
		(strings.Contains(errStr, "rejected") && strings.Contains(errStr, "fetch first")) ||
		(strings.Contains(errStr, "failed to push") && strings.Contains(errStr, "behind"))
}

// RemoteBranchExists checks if a branch exists on the remote.
func (g *Git) RemoteBranchExists(remote, branch string) (bool, error) {
	// Use ls-remote to check if branch exists on remote
	output, err := g.ctx.RunGit("ls-remote", "--heads", remote, "refs/heads/"+branch)
	if err != nil {
		return false, fmt.Errorf("ls-remote failed: %w", err)
	}
	// If output is non-empty, the branch exists
	return strings.TrimSpace(output) != "", nil
}

// ProtectedBranches returns the list of protected branch names.
func (g *Git) ProtectedBranches() []string {
	return g.protectedBranches
}

// DeleteBranch deletes a branch.
func (g *Git) DeleteBranch(branch string, force bool) error {
	return g.ctx.DeleteBranch(branch, force)
}

// HasRemote checks if a remote exists in the repository.
// Returns true if the remote is configured, false otherwise.
// This is useful for detecting sandbox/test repositories that don't have remotes.
func (g *Git) HasRemote(remote string) bool {
	_, err := g.ctx.GetRemoteURL(remote)
	return err == nil
}
