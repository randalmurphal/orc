// Package git provides git operations for orc, wrapping devflow/git.
package git

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	devgit "github.com/randalmurphal/devflow/git"
)

// Checkpoint represents a git checkpoint (commit) for a phase.
type Checkpoint struct {
	TaskID    string    `yaml:"task_id" json:"task_id"`
	Phase     string    `yaml:"phase" json:"phase"`
	CommitSHA string    `yaml:"commit_sha" json:"commit_sha"`
	Message   string    `yaml:"message" json:"message"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

// Git provides git operations for orc tasks.
// The mutex protects compound operations that must be atomic (e.g., rebase+abort,
// worktree creation with cleanup). Individual git commands don't need locking
// as they are atomic at the process level.
type Git struct {
	mu                sync.Mutex       // Protects compound operations that must be atomic
	ctx               *devgit.Context
	branchPrefix      string
	commitPrefix      string
	worktreeDir       string
	executorPrefix    string   // For multi-user branch/worktree naming (empty in solo mode)
	inWorktreeContext bool     // True when operating within a worktree
	protectedBranches []string // Branches that cannot be pushed to directly
}

// Config holds git configuration.
type Config struct {
	BranchPrefix      string   // Prefix for task branches (default: "orc/")
	CommitPrefix      string   // Prefix for commit messages (default: "[orc]")
	WorktreeDir       string   // Directory for worktrees (default: ".orc/worktrees")
	ExecutorPrefix    string   // Executor prefix for multi-user mode (empty in solo mode)
	ProtectedBranches []string // Branches protected from direct push (default: main, master, develop, release)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BranchPrefix:      "orc/",
		CommitPrefix:      "[orc]",
		WorktreeDir:       ".orc/worktrees",
		ProtectedBranches: DefaultProtectedBranches,
	}
}

// New creates a new Git instance for the repository at workDir.
func New(workDir string, cfg Config) (*Git, error) {
	ctx, err := devgit.NewContext(workDir, devgit.WithWorktreeDir(cfg.WorktreeDir))
	if err != nil {
		return nil, fmt.Errorf("init git context: %w", err)
	}

	protectedBranches := cfg.ProtectedBranches
	if len(protectedBranches) == 0 {
		protectedBranches = DefaultProtectedBranches
	}

	return &Git{
		ctx:               ctx,
		branchPrefix:      cfg.BranchPrefix,
		commitPrefix:      cfg.CommitPrefix,
		worktreeDir:       cfg.WorktreeDir,
		executorPrefix:    cfg.ExecutorPrefix,
		protectedBranches: protectedBranches,
	}, nil
}

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
	claudeHookCfg := ClaudeCodeHookConfig{
		WorktreePath: worktreePath,
		MainRepoPath: g.ctx.RepoPath(),
		TaskID:       taskID,
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
	claudeHookCfg := ClaudeCodeHookConfig{
		WorktreePath: worktreePath,
		MainRepoPath: g.ctx.RepoPath(),
		TaskID:       taskID,
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
func (g *Git) CreateCheckpoint(taskID, phase, message string) (*Checkpoint, error) {
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
		if err == devgit.ErrNothingToCommit {
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

// ErrMainRepoModification is returned when a destructive operation is attempted
// on the main repository instead of within a worktree context.
// This is a critical safety check - worktrees exist to isolate task execution.
var ErrMainRepoModification = errors.New("destructive operation on main repository blocked")

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

// GetCurrentBranch returns the current branch name.
func (g *Git) GetCurrentBranch() (string, error) {
	return g.ctx.CurrentBranch()
}

// IsClean returns true if the working directory is clean.
func (g *Git) IsClean() (bool, error) {
	return g.ctx.IsClean()
}

// Fetch fetches from the remote.
func (g *Git) Fetch(remote string) error {
	return g.ctx.Fetch(remote)
}

// Rebase rebases onto the target ref.
// SAFETY: This operation requires worktree context to prevent accidental modification
// of the main repository.
func (g *Git) Rebase(target string) error {
	// CRITICAL: Prevent rebase operations on main repo
	if err := g.RequireWorktreeContext("git rebase"); err != nil {
		return err
	}
	_, err := g.ctx.RunGit("rebase", target)
	return err
}

// ErrProtectedBranch is returned when attempting to push to a protected branch.
var ErrProtectedBranch = errors.New("push to protected branch blocked")

// Push pushes the current branch to remote.
// Returns ErrProtectedBranch if attempting to push to a protected branch.
func (g *Git) Push(remote, branch string, setUpstream bool) error {
	if IsProtectedBranch(branch, g.protectedBranches) {
		return fmt.Errorf("%w: cannot push to '%s' - use PR workflow instead", ErrProtectedBranch, branch)
	}
	return g.ctx.Push(remote, branch, setUpstream)
}

// PushForce pushes with --force-with-lease for safety.
// This is safer than --force as it fails if the remote has unexpected commits
// (i.e., commits that weren't fetched yet).
// SAFETY: This will NOT push to protected branches.
func (g *Git) PushForce(remote, branch string, setUpstream bool) error {
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

// PushUnsafe pushes to remote without branch protection checks.
// This should only be used by PR merge operations that have explicit user approval.
// DANGER: Use with caution - this bypasses safety checks.
func (g *Git) PushUnsafe(remote, branch string, setUpstream bool) error {
	return g.ctx.Push(remote, branch, setUpstream)
}

// ProtectedBranches returns the list of protected branch names.
func (g *Git) ProtectedBranches() []string {
	return g.protectedBranches
}

// Merge merges a branch into current.
//
// SAFETY: This operation requires worktree context to prevent accidental modification
// of the main repository.
func (g *Git) Merge(branch string, noFF bool) error {
	// CRITICAL: Prevent merge operations on main repo
	if err := g.RequireWorktreeContext("git merge"); err != nil {
		return err
	}
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)
	_, err := g.ctx.RunGit(args...)
	return err
}

// DeleteBranch deletes a branch.
func (g *Git) DeleteBranch(branch string, force bool) error {
	return g.ctx.DeleteBranch(branch, force)
}

// GetRemoteURL returns the URL of the origin remote.
func (g *Git) GetRemoteURL() (string, error) {
	return g.ctx.GetRemoteURL("origin")
}

// HasRemote checks if a remote exists in the repository.
// Returns true if the remote is configured, false otherwise.
// This is useful for detecting sandbox/test repositories that don't have remotes.
func (g *Git) HasRemote(remote string) bool {
	_, err := g.ctx.GetRemoteURL(remote)
	return err == nil
}

// Context returns the underlying devflow git context.
func (g *Git) Context() *devgit.Context {
	return g.ctx
}

// SyncResult contains the result of a sync operation.
type SyncResult struct {
	// Synced indicates whether sync was performed successfully
	Synced bool
	// ConflictsDetected indicates merge conflicts were found
	ConflictsDetected bool
	// ConflictFiles lists files with conflicts
	ConflictFiles []string
	// CommitsBehind is the number of commits the branch is behind target
	CommitsBehind int
	// CommitsAhead is the number of commits the branch is ahead of target
	CommitsAhead int
}

// ErrMergeConflict is returned when a merge/rebase encounters conflicts.
var ErrMergeConflict = errors.New("merge conflict detected")

// DetectConflicts checks if the current branch would have conflicts when merged with target.
// This performs a dry-run merge without modifying the working tree.
func (g *Git) DetectConflicts(target string) (*SyncResult, error) {
	result := &SyncResult{}

	// Get commit counts
	ahead, behind, err := g.GetCommitCounts(target)
	if err != nil {
		return nil, fmt.Errorf("get commit counts: %w", err)
	}
	result.CommitsAhead = ahead
	result.CommitsBehind = behind

	// If up-to-date, no conflicts possible
	if behind == 0 {
		result.Synced = true
		return result, nil
	}

	// Use git merge-tree to detect conflicts without modifying working tree
	// This requires git 2.38+ with the --write-tree option
	currentBranch, err := g.ctx.CurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("get current branch: %w", err)
	}

	// Get merge base
	mergeBase, err := g.ctx.RunGit("merge-base", currentBranch, target)
	if err != nil {
		return nil, fmt.Errorf("get merge base: %w", err)
	}
	mergeBase = strings.TrimSpace(mergeBase)

	// Try merge-tree with --write-tree (git 2.38+)
	output, err := g.ctx.RunGit("merge-tree", "--write-tree", "--no-messages", mergeBase, currentBranch, target)
	if err != nil {
		// If merge-tree fails, fall back to actual merge attempt
		return g.detectConflictsViaMerge(target)
	}

	// Parse output - if there are conflict markers, we have conflicts
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "CONFLICT") {
			result.ConflictsDetected = true
			// Extract file name from conflict line if possible
			// Format: "CONFLICT (content): Merge conflict in <file>"
			if idx := strings.Index(line, " in "); idx != -1 {
				file := strings.TrimSpace(line[idx+4:])
				result.ConflictFiles = append(result.ConflictFiles, file)
			}
		}
	}

	return result, nil
}

// detectConflictsViaMerge performs conflict detection via an actual merge attempt.
// Falls back for older git versions that don't support merge-tree --write-tree.
//
// SAFETY: This function performs merge and reset operations. While it attempts to
// restore the original state, it MUST only be called in worktree context.
//
// This is a compound operation (merge + diff + abort + reset) protected by mutex
// to prevent concurrent conflict detection from interfering with each other.
func (g *Git) detectConflictsViaMerge(target string) (*SyncResult, error) {
	// CRITICAL: This function does merge and reset - MUST be in worktree context
	if err := g.RequireWorktreeContext("conflict detection via merge"); err != nil {
		return nil, err
	}
	// Additional check: don't do this on protected branches
	if err := g.RequireNonProtectedBranch("conflict detection via merge"); err != nil {
		return nil, err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	result := &SyncResult{}

	// Get current HEAD for potential abort
	head, err := g.ctx.HeadCommit()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	// Defer cleanup BEFORE merge attempt - guaranteed to run even on error/panic.
	// These operations are idempotent: merge --abort and reset --hard are safe
	// to call even if no merge was started or if already at the target state.
	defer func() {
		// Abort any in-progress merge (idempotent - safe if no merge)
		_, _ = g.ctx.RunGit("merge", "--abort")
		// Reset to original HEAD just in case (idempotent - safe if already at HEAD)
		_, _ = g.ctx.RunGit("reset", "--hard", head)
	}()

	// Attempt merge with --no-commit to detect conflicts
	_, mergeErr := g.ctx.RunGit("merge", "--no-commit", "--no-ff", target)

	// Check for conflicts by looking at unmerged files
	if mergeErr != nil {
		// List unmerged files
		output, _ := g.ctx.RunGit("diff", "--name-only", "--diff-filter=U")
		if output != "" {
			result.ConflictsDetected = true
			result.ConflictFiles = strings.Split(strings.TrimSpace(output), "\n")
		}
	}

	return result, nil
}

// GetCommitCounts returns (ahead, behind) commit counts relative to target.
// Ahead is how many commits HEAD has that target doesn't.
// Behind is how many commits target has that HEAD doesn't.
func (g *Git) GetCommitCounts(target string) (int, int, error) {
	// git rev-list --count --left-right HEAD...target
	output, err := g.ctx.RunGit("rev-list", "--count", "--left-right", "HEAD..."+target)
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output: %s", output)
	}

	ahead := 0
	behind := 0
	_, _ = fmt.Sscanf(parts[0], "%d", &ahead)
	_, _ = fmt.Sscanf(parts[1], "%d", &behind)

	return ahead, behind, nil
}

// RebaseWithConflictCheck rebases onto target and returns details about any conflicts.
// If conflicts occur, the rebase is aborted and ErrMergeConflict is returned.
//
// SAFETY: This operation requires worktree context to prevent accidental modification
// of the main repository.
//
// This is a compound operation (commit counts + rebase + diff + abort) protected by
// mutex to prevent concurrent rebase operations from interfering with each other.
func (g *Git) RebaseWithConflictCheck(target string) (*SyncResult, error) {
	// CRITICAL: Prevent rebase operations on main repo
	if err := g.RequireWorktreeContext("rebase with conflict check"); err != nil {
		return nil, err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	result := &SyncResult{}

	// Get initial state
	ahead, behind, err := g.GetCommitCounts(target)
	if err != nil {
		return nil, fmt.Errorf("get commit counts: %w", err)
	}
	result.CommitsAhead = ahead
	result.CommitsBehind = behind

	// If already up-to-date, no rebase needed
	if behind == 0 {
		result.Synced = true
		return result, nil
	}

	// Attempt rebase
	_, rebaseErr := g.ctx.RunGit("rebase", target)
	if rebaseErr != nil {
		// Check for conflicts
		output, _ := g.ctx.RunGit("diff", "--name-only", "--diff-filter=U")
		if output != "" {
			result.ConflictsDetected = true
			result.ConflictFiles = strings.Split(strings.TrimSpace(output), "\n")
		}

		// Abort the rebase
		_, _ = g.ctx.RunGit("rebase", "--abort")

		// Only return ErrMergeConflict if we actually detected conflicts
		if result.ConflictsDetected {
			return result, fmt.Errorf("%w: %d files have conflicts", ErrMergeConflict, len(result.ConflictFiles))
		}
		// Rebase failed for another reason (dirty tree, uncommitted changes, etc.)
		return result, fmt.Errorf("rebase failed: %w", rebaseErr)
	}

	result.Synced = true
	return result, nil
}

// AbortRebase aborts any in-progress rebase.
func (g *Git) AbortRebase() error {
	_, err := g.ctx.RunGit("rebase", "--abort")
	return err
}

// AbortMerge aborts any in-progress merge.
func (g *Git) AbortMerge() error {
	_, err := g.ctx.RunGit("merge", "--abort")
	return err
}

// IsRebaseInProgress checks if a rebase is in progress.
// Checks for .git/rebase-merge/ or .git/rebase-apply/ directories.
// Works in both regular repos and worktrees.
func (g *Git) IsRebaseInProgress() (bool, error) {
	workDir := g.ctx.WorkDir()

	// For worktrees, .git is a file pointing to the actual git dir
	gitPath := filepath.Join(workDir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false, fmt.Errorf("stat .git: %w", err)
	}

	var gitDir string
	if info.IsDir() {
		// Regular repo - .git is a directory
		gitDir = gitPath
	} else {
		// Worktree - .git is a file containing "gitdir: <path>"
		content, err := os.ReadFile(gitPath)
		if err != nil {
			return false, fmt.Errorf("read .git file: %w", err)
		}
		line := strings.TrimSpace(string(content))
		if !strings.HasPrefix(line, "gitdir: ") {
			return false, fmt.Errorf("unexpected .git file format: %s", line)
		}
		gitDir = strings.TrimPrefix(line, "gitdir: ")
	}

	// Check for rebase-merge directory (interactive rebase)
	rebaseMergeDir := filepath.Join(gitDir, "rebase-merge")
	if _, err := os.Stat(rebaseMergeDir); err == nil {
		return true, nil
	}

	// Check for rebase-apply directory (non-interactive rebase, am)
	rebaseApplyDir := filepath.Join(gitDir, "rebase-apply")
	if _, err := os.Stat(rebaseApplyDir); err == nil {
		return true, nil
	}

	return false, nil
}

// IsMergeInProgress checks if a merge is in progress.
// Checks for .git/MERGE_HEAD file.
// Works in both regular repos and worktrees.
func (g *Git) IsMergeInProgress() (bool, error) {
	workDir := g.ctx.WorkDir()

	// For worktrees, .git is a file pointing to the actual git dir
	gitPath := filepath.Join(workDir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false, fmt.Errorf("stat .git: %w", err)
	}

	var gitDir string
	if info.IsDir() {
		// Regular repo - .git is a directory
		gitDir = gitPath
	} else {
		// Worktree - .git is a file containing "gitdir: <path>"
		content, err := os.ReadFile(gitPath)
		if err != nil {
			return false, fmt.Errorf("read .git file: %w", err)
		}
		line := strings.TrimSpace(string(content))
		if !strings.HasPrefix(line, "gitdir: ") {
			return false, fmt.Errorf("unexpected .git file format: %s", line)
		}
		gitDir = strings.TrimPrefix(line, "gitdir: ")
	}

	// Check for MERGE_HEAD file
	mergeHeadFile := filepath.Join(gitDir, "MERGE_HEAD")
	if _, err := os.Stat(mergeHeadFile); err == nil {
		return true, nil
	}

	return false, nil
}

// DiscardChanges discards all uncommitted changes in the working directory.
// This includes both staged and unstaged changes.
// SAFETY: This operation is destructive and should only be used when explicitly requested.
func (g *Git) DiscardChanges() error {
	// Reset staged changes (ignore error - might fail if no HEAD exists yet)
	_, _ = g.ctx.RunGit("reset", "HEAD")

	// Discard unstaged changes to tracked files
	if _, err := g.ctx.RunGit("checkout", "--", "."); err != nil {
		return fmt.Errorf("discard tracked changes: %w", err)
	}

	// Remove untracked files and directories
	if _, err := g.ctx.RunGit("clean", "-fd"); err != nil {
		return fmt.Errorf("remove untracked files: %w", err)
	}

	return nil
}

// RestoreOrcDir restores the .orc/ directory from a target ref (e.g., "origin/main").
// This is used during completion sync to prevent worktree modifications to .orc/ from
// contaminating the target branch when merged.
//
// The function:
// 1. Checks if .orc/ has changes compared to the target
// 2. If changes exist, removes the current .orc/ and restores from target
// 3. Commits the restoration if needed
//
// Returns true if restoration was performed, false if no changes were found.
//
// This is a compound operation (diff + rm + checkout + add + commit) protected by
// mutex to ensure atomicity.
func (g *Git) RestoreOrcDir(target string, taskID string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if .orc/ directory exists
	// Use WorkDir() not RepoPath() - in worktree context, WorkDir is the worktree path
	orcDir := filepath.Join(g.ctx.WorkDir(), ".orc")
	if _, err := os.Stat(orcDir); os.IsNotExist(err) {
		// No .orc/ directory, nothing to restore
		return false, nil
	}

	// Check if .orc/ has changes compared to target
	// Use git diff to detect changes between current state and target
	output, err := g.ctx.RunGit("diff", "--name-only", target, "--", ".orc/")
	if err != nil {
		// If target ref doesn't exist or diff fails, skip restoration
		// This handles cases where target branch doesn't have .orc/ yet
		return false, nil
	}

	// If no changes, nothing to restore
	changedFiles := strings.TrimSpace(output)
	if changedFiles == "" {
		return false, nil
	}

	// Remove tracked .orc/ files that differ from target
	// This handles both modifications AND additions (files that don't exist in target)
	// We need to use rm for files that were added (won't exist in target)
	// and checkout for files that were modified

	// First, remove .orc/ from working tree (but keep in index for now)
	if err := os.RemoveAll(orcDir); err != nil {
		return false, fmt.Errorf("remove .orc/ directory: %w", err)
	}

	// Restore .orc/ from target using checkout
	// This will recreate .orc/ with the target's content
	_, err = g.ctx.RunGit("checkout", target, "--", ".orc/")
	if err != nil {
		return false, fmt.Errorf("restore .orc/ from %s: %w", target, err)
	}

	// Check if there are changes to commit (added files that were removed will show as deleted)
	status, err := g.ctx.RunGit("status", "--porcelain", ".orc/")
	if err != nil {
		return false, fmt.Errorf("check status after restore: %w", err)
	}

	// If there are changes after restoration, commit them
	if strings.TrimSpace(status) != "" {
		// Stage all .orc/ changes (including deletions of files not in target)
		if _, err := g.ctx.RunGit("add", ".orc/"); err != nil {
			return false, fmt.Errorf("stage .orc/ restore: %w", err)
		}

		// Commit the restoration
		commitMsg := fmt.Sprintf("%s %s: restore .orc/ from %s", g.commitPrefix, taskID, target)
		if _, err := g.ctx.RunGit("commit", "-m", commitMsg); err != nil {
			// If commit fails due to nothing to commit, that's OK
			if !strings.Contains(err.Error(), "nothing to commit") {
				return false, fmt.Errorf("commit .orc/ restore: %w", err)
			}
			return false, nil
		}
	}

	return true, nil
}

// RestoreClaudeSettings restores .claude/settings.json from a target ref.
// This prevents worktree isolation hooks from being merged into the target branch.
// Worktrees inject Claude Code hooks that reference machine-specific paths - these
// should never be merged to shared branches.
//
// Returns true if restoration was performed, false if no changes were needed.
//
// This is a compound operation (diff + checkout/rm + add + commit) protected by
// mutex to ensure atomicity.
func (g *Git) RestoreClaudeSettings(target string, taskID string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	settingsPath := filepath.Join(g.ctx.WorkDir(), ".claude", "settings.json")

	// Check if settings.json exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return false, nil
	}

	// Check if settings.json differs from target
	output, err := g.ctx.RunGit("diff", "--name-only", target, "--", ".claude/settings.json")
	if err != nil {
		// Target might not have .claude/settings.json - that's fine
		return false, nil
	}

	if strings.TrimSpace(output) == "" {
		return false, nil
	}

	// Check if target has .claude/settings.json
	_, err = g.ctx.RunGit("cat-file", "-e", target+":.claude/settings.json")
	if err != nil {
		// Target doesn't have settings.json - remove ours entirely
		if err := os.Remove(settingsPath); err != nil && !os.IsNotExist(err) {
			return false, fmt.Errorf("remove .claude/settings.json: %w", err)
		}
		// Stage the deletion
		if _, err := g.ctx.RunGit("add", ".claude/settings.json"); err != nil {
			return false, fmt.Errorf("stage settings.json deletion: %w", err)
		}
	} else {
		// Restore from target
		if _, err := g.ctx.RunGit("checkout", target, "--", ".claude/settings.json"); err != nil {
			return false, fmt.Errorf("restore .claude/settings.json from %s: %w", target, err)
		}
		// Stage the restoration
		if _, err := g.ctx.RunGit("add", ".claude/settings.json"); err != nil {
			return false, fmt.Errorf("stage settings.json restore: %w", err)
		}
	}

	// Commit if there are staged changes
	status, _ := g.ctx.RunGit("diff", "--cached", "--name-only", "--", ".claude/settings.json")
	if strings.TrimSpace(status) != "" {
		commitMsg := fmt.Sprintf("%s %s: restore .claude/settings.json from %s", g.commitPrefix, taskID, target)
		if _, err := g.ctx.RunGit("commit", "-m", commitMsg); err != nil {
			if !strings.Contains(err.Error(), "nothing to commit") {
				return false, fmt.Errorf("commit settings.json restore: %w", err)
			}
			return false, nil
		}
		return true, nil
	}

	return false, nil
}

// TryAutoResolveClaudeMD attempts to auto-resolve CLAUDE.md conflicts.
// This is called when CLAUDE.md is detected as a conflicted file.
// Returns true if resolution succeeded, false if manual resolution is needed.
//
// Auto-resolution only works for append-only conflicts in the knowledge section:
// - Both sides add new rows to the same table
// - No overlapping edits to the same row
// - Conflict is within orc:knowledge:begin/end markers
func (g *Git) TryAutoResolveClaudeMD(logger *slog.Logger) (bool, []string) {
	// Use WorkDir() not RepoPath() - in worktree context, WorkDir is the worktree path
	claudeMDPath := filepath.Join(g.ctx.WorkDir(), "CLAUDE.md")

	// Read the conflicted file
	content, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return false, []string{fmt.Sprintf("failed to read CLAUDE.md: %v", err)}
	}

	// Check for conflict markers
	if !strings.Contains(string(content), "<<<<<<<") {
		return false, []string{"no conflict markers found in CLAUDE.md"}
	}

	// Attempt auto-resolution
	resolved, success, logs := ResolveClaudeMDConflict(string(content), logger)
	if !success {
		return false, logs
	}

	// Write the resolved content
	if err := os.WriteFile(claudeMDPath, []byte(resolved), 0644); err != nil {
		return false, append(logs, fmt.Sprintf("failed to write resolved CLAUDE.md: %v", err))
	}

	// Stage the resolved file
	if _, err := g.ctx.RunGit("add", "CLAUDE.md"); err != nil {
		return false, append(logs, fmt.Sprintf("failed to stage resolved CLAUDE.md: %v", err))
	}

	logs = append(logs, "CLAUDE.md auto-resolved and staged")
	return true, logs
}

// AutoResolveConflicts attempts to auto-resolve known conflict patterns.
// Currently handles:
// - CLAUDE.md knowledge section (append-only table rows)
//
// Returns the list of files that were auto-resolved and the remaining conflicts.
func (g *Git) AutoResolveConflicts(conflictFiles []string, logger *slog.Logger) (resolved []string, remaining []string, logs []string) {
	for _, file := range conflictFiles {
		if IsClaudeMDFile(file) {
			success, resolveLogs := g.TryAutoResolveClaudeMD(logger)
			logs = append(logs, resolveLogs...)
			if success {
				resolved = append(resolved, file)
				if logger != nil {
					logger.Info("auto-resolved CLAUDE.md conflict",
						"file", file,
					)
				}
			} else {
				remaining = append(remaining, file)
				if logger != nil {
					logger.Debug("CLAUDE.md auto-resolve failed, requires manual resolution",
						"file", file,
						"reason", strings.Join(resolveLogs, "; "),
					)
				}
			}
		} else {
			remaining = append(remaining, file)
		}
	}
	return resolved, remaining, logs
}
