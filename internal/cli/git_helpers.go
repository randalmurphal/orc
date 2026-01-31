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
