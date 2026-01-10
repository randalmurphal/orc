// Package git provides git operations for orc, wrapping devflow/git.
package git

import (
	"fmt"
	"os"
	"path/filepath"
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
type Git struct {
	ctx          *devgit.Context
	branchPrefix string
	commitPrefix string
	worktreeDir  string
}

// Config holds git configuration.
type Config struct {
	BranchPrefix string // Prefix for task branches (default: "orc/")
	CommitPrefix string // Prefix for commit messages (default: "[orc]")
	WorktreeDir  string // Directory for worktrees (default: ".orc/worktrees")
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BranchPrefix: "orc/",
		CommitPrefix: "[orc]",
		WorktreeDir:  ".orc/worktrees",
	}
}

// New creates a new Git instance for the repository at workDir.
func New(workDir string, cfg Config) (*Git, error) {
	ctx, err := devgit.NewContext(workDir, devgit.WithWorktreeDir(cfg.WorktreeDir))
	if err != nil {
		return nil, fmt.Errorf("init git context: %w", err)
	}

	return &Git{
		ctx:          ctx,
		branchPrefix: cfg.BranchPrefix,
		commitPrefix: cfg.CommitPrefix,
		worktreeDir:  cfg.WorktreeDir,
	}, nil
}

// BranchName returns the full branch name for a task.
func (g *Git) BranchName(taskID string) string {
	return g.branchPrefix + taskID
}

// CreateWorktree creates an isolated worktree for a task.
// Returns the absolute path to the worktree.
// NOTE: This does NOT modify the main repo's checked-out branch.
func (g *Git) CreateWorktree(taskID, baseBranch string) (string, error) {
	branchName := g.BranchName(taskID)
	safeName := devgit.SanitizeBranchName(branchName)
	worktreePath := filepath.Join(g.ctx.RepoPath(), g.worktreeDir, safeName)

	// Ensure worktrees directory exists
	worktreesDir := filepath.Join(g.ctx.RepoPath(), g.worktreeDir)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}

	// Create worktree with new branch from base branch
	// This does NOT checkout the base branch in the main repo
	_, err := g.ctx.RunGit("worktree", "add", "-b", branchName, worktreePath, baseBranch)
	if err != nil {
		// Branch might already exist, try to add worktree for existing branch
		_, err = g.ctx.RunGit("worktree", "add", worktreePath, branchName)
		if err != nil {
			return "", fmt.Errorf("create worktree for %s: %w", taskID, err)
		}
	}

	return worktreePath, nil
}

// CleanupWorktree removes a task's worktree.
func (g *Git) CleanupWorktree(taskID string) error {
	branchName := g.BranchName(taskID)
	safeName := devgit.SanitizeBranchName(branchName)
	worktreePath := filepath.Join(g.ctx.RepoPath(), g.worktreeDir, safeName)

	if err := g.ctx.CleanupWorktree(worktreePath); err != nil {
		return fmt.Errorf("cleanup worktree for %s: %w", taskID, err)
	}

	return nil
}

// WorktreePath returns the path to a task's worktree.
func (g *Git) WorktreePath(taskID string) string {
	branchName := g.BranchName(taskID)
	safeName := devgit.SanitizeBranchName(branchName)
	return filepath.Join(g.ctx.RepoPath(), g.worktreeDir, safeName)
}

// InWorktree returns a Git instance operating in the specified worktree.
func (g *Git) InWorktree(worktreePath string) *Git {
	return &Git{
		ctx:          g.ctx.InWorktree(worktreePath),
		branchPrefix: g.branchPrefix,
		commitPrefix: g.commitPrefix,
		worktreeDir:  g.worktreeDir,
	}
}

// CreateCheckpoint creates a checkpoint commit for a phase.
func (g *Git) CreateCheckpoint(taskID, phase, message string) (*Checkpoint, error) {
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
func (g *Git) CreateBranch(taskID string) error {
	branchName := g.BranchName(taskID)
	if err := g.ctx.CreateBranch(branchName); err != nil {
		return fmt.Errorf("create branch %s: %w", branchName, err)
	}
	return g.ctx.Checkout(branchName)
}

// SwitchBranch switches to an existing task branch.
func (g *Git) SwitchBranch(taskID string) error {
	branchName := g.BranchName(taskID)
	return g.ctx.Checkout(branchName)
}

// Rewind resets to a specific commit.
func (g *Git) Rewind(commitSHA string) error {
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
func (g *Git) Rebase(target string) error {
	_, err := g.ctx.RunGit("rebase", target)
	return err
}

// Push pushes the current branch to remote.
func (g *Git) Push(remote, branch string, setUpstream bool) error {
	return g.ctx.Push(remote, branch, setUpstream)
}

// Merge merges a branch into current.
func (g *Git) Merge(branch string, noFF bool) error {
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

// Context returns the underlying devflow git context.
func (g *Git) Context() *devgit.Context {
	return g.ctx
}
