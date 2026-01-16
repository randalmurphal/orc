// Package executor implements the task execution engine.
package executor

import (
	"log/slog"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// DefaultTargetBranch is the hardcoded fallback when no other configuration is set.
const DefaultTargetBranch = "main"

// ResolveTargetBranch determines the target branch for a task's PR using a 5-level
// priority hierarchy. Higher levels take precedence over lower levels:
//
//  1. Task.TargetBranch (explicit override per task)
//  2. Initiative.BranchBase (inherited from initiative)
//  3. Developer.StagingBranch (personal staging area, when enabled)
//  4. Config.Completion.TargetBranch (project default)
//  5. "main" (hardcoded fallback)
//
// Parameters:
//   - t: The task (may be nil)
//   - init: The initiative the task belongs to (may be nil)
//   - cfg: The orc configuration (may be nil)
//
// Returns the resolved target branch name.
func ResolveTargetBranch(t *task.Task, init *initiative.Initiative, cfg *config.Config) string {
	// Level 1: Task explicit override
	if t != nil && t.TargetBranch != "" {
		return t.TargetBranch
	}

	// Level 2: Initiative branch base
	if init != nil && init.BranchBase != "" {
		return init.BranchBase
	}

	// Level 3: Developer staging branch (personal config)
	if cfg != nil && cfg.Developer.StagingEnabled && cfg.Developer.StagingBranch != "" {
		return cfg.Developer.StagingBranch
	}

	// Level 4: Project config default
	if cfg != nil && cfg.Completion.TargetBranch != "" {
		return cfg.Completion.TargetBranch
	}

	// Level 5: Hardcoded fallback
	return DefaultTargetBranch
}

// ResolveTargetBranchSource returns both the resolved target branch and the source
// of that resolution for debugging/logging purposes.
//
// Returns:
//   - branch: The resolved target branch name
//   - source: A human-readable description of where the branch came from
func ResolveTargetBranchSource(t *task.Task, init *initiative.Initiative, cfg *config.Config) (branch, source string) {
	// Level 1: Task explicit override
	if t != nil && t.TargetBranch != "" {
		return t.TargetBranch, "task override"
	}

	// Level 2: Initiative branch base
	if init != nil && init.BranchBase != "" {
		return init.BranchBase, "initiative branch"
	}

	// Level 3: Developer staging branch (personal config)
	if cfg != nil && cfg.Developer.StagingEnabled && cfg.Developer.StagingBranch != "" {
		return cfg.Developer.StagingBranch, "developer staging"
	}

	// Level 4: Project config default
	if cfg != nil && cfg.Completion.TargetBranch != "" {
		return cfg.Completion.TargetBranch, "project config"
	}

	// Level 5: Hardcoded fallback
	return DefaultTargetBranch, "default"
}

// IsDefaultBranch returns true if the given branch name is a default/main branch
// that typically already exists (main, master, develop).
func IsDefaultBranch(branch string) bool {
	switch branch {
	case "main", "master", "develop", "development":
		return true
	default:
		return false
	}
}

// ResolveTargetBranchForTask is a convenience function that loads the initiative
// from the backend (if the task belongs to one) and then resolves the target branch.
// This is useful when you have access to the storage backend but not a pre-loaded initiative.
//
// Parameters:
//   - t: The task (may be nil)
//   - backend: Storage backend for loading initiatives (may be nil)
//   - cfg: The orc configuration (may be nil)
//
// Returns the resolved target branch name.
func ResolveTargetBranchForTask(t *task.Task, backend storage.Backend, cfg *config.Config) string {
	branch, _ := ResolveTargetBranchForTaskWithSource(t, backend, cfg)
	return branch
}

// ResolveTargetBranchForTaskWithSource is like ResolveTargetBranchForTask but also returns
// the source of the resolution for debugging/logging purposes.
func ResolveTargetBranchForTaskWithSource(t *task.Task, backend storage.Backend, cfg *config.Config) (branch, source string) {
	var init *initiative.Initiative

	// Load initiative if task belongs to one
	if t != nil && t.InitiativeID != "" && backend != nil {
		var err error
		init, err = backend.LoadInitiative(t.InitiativeID)
		if err != nil {
			slog.Debug("failed to load initiative for branch resolution",
				"task_id", t.ID,
				"initiative_id", t.InitiativeID,
				"error", err,
			)
			// Continue with nil initiative - will fall through to other resolution levels
		}
	}

	return ResolveTargetBranchSource(t, init, cfg)
}
