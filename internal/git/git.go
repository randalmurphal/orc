// Package git provides git operations for orc.
package git

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"
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
	mu                sync.Mutex // Protects compound operations that must be atomic
	ctx               *Context
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
		WorktreeDir:       "",
		ProtectedBranches: DefaultProtectedBranches,
	}
}

// New creates a new Git instance for the repository at workDir.
func New(workDir string, cfg Config) (*Git, error) {
	ctx, err := NewContext(workDir, WithWorktreeDir(cfg.WorktreeDir))
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

// worktreeBasePath returns the absolute base directory for worktrees.
// Handles both absolute and relative worktree directory configurations.
func (g *Git) worktreeBasePath() string {
	if filepath.IsAbs(g.worktreeDir) {
		return g.worktreeDir
	}
	return filepath.Join(g.ctx.RepoPath(), g.worktreeDir)
}

// Context returns the underlying git context.
func (g *Git) Context() *Context {
	return g.ctx
}
