// Package executor implements the task execution engine.
package executor

import (
	"log/slog"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
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
// Returns the resolved target branch name. If the resolved branch name is invalid,
// falls back to the default branch for safety.
func ResolveTargetBranch(t *orcv1.Task, init *initiative.Initiative, cfg *config.Config) string {
	var branch string
	var source string

	// Level 1: Task explicit override
	targetBranch := task.GetTargetBranchProto(t)
	if t != nil && targetBranch != "" {
		branch = targetBranch
		source = "task override"
	} else if init != nil && init.BranchBase != "" {
		// Level 2: Initiative branch base
		branch = init.BranchBase
		source = "initiative branch"
	} else if cfg != nil && cfg.Developer.StagingEnabled && cfg.Developer.StagingBranch != "" {
		// Level 3: Developer staging branch (personal config)
		branch = cfg.Developer.StagingBranch
		source = "developer staging"
	} else if cfg != nil && cfg.Completion.TargetBranch != "" {
		// Level 4: Project config default
		branch = cfg.Completion.TargetBranch
		source = "project config"
	} else {
		// Level 5: Hardcoded fallback
		return DefaultTargetBranch
	}

	// Defense-in-depth: validate resolved branch name
	if err := git.ValidateBranchName(branch); err != nil {
		slog.Warn("invalid branch name in resolution, using default",
			"branch", branch,
			"source", source,
			"error", err,
		)
		return DefaultTargetBranch
	}

	return branch
}

// ResolveTargetBranchSource returns both the resolved target branch and the source
// of that resolution for debugging/logging purposes.
//
// Returns:
//   - branch: The resolved target branch name
//   - source: A human-readable description of where the branch came from
//
// If the resolved branch name is invalid, falls back to the default branch for safety.
func ResolveTargetBranchSource(t *orcv1.Task, init *initiative.Initiative, cfg *config.Config) (branch, source string) {
	// Level 1: Task explicit override
	targetBranch := task.GetTargetBranchProto(t)
	if t != nil && targetBranch != "" {
		branch = targetBranch
		source = "task override"
	} else if init != nil && init.BranchBase != "" {
		// Level 2: Initiative branch base
		branch = init.BranchBase
		source = "initiative branch"
	} else if cfg != nil && cfg.Developer.StagingEnabled && cfg.Developer.StagingBranch != "" {
		// Level 3: Developer staging branch (personal config)
		branch = cfg.Developer.StagingBranch
		source = "developer staging"
	} else if cfg != nil && cfg.Completion.TargetBranch != "" {
		// Level 4: Project config default
		branch = cfg.Completion.TargetBranch
		source = "project config"
	} else {
		// Level 5: Hardcoded fallback
		return DefaultTargetBranch, "default"
	}

	// Defense-in-depth: validate resolved branch name
	if err := git.ValidateBranchName(branch); err != nil {
		slog.Warn("invalid branch name in resolution, using default",
			"branch", branch,
			"source", source,
			"error", err,
		)
		return DefaultTargetBranch, "default (fallback from invalid " + source + ")"
	}

	return branch, source
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
func ResolveTargetBranchForTask(t *orcv1.Task, backend storage.Backend, cfg *config.Config) string {
	branch, _ := ResolveTargetBranchForTaskWithSource(t, backend, cfg)
	return branch
}

// ResolveTargetBranchForTaskWithSource is like ResolveTargetBranchForTask but also returns
// the source of the resolution for debugging/logging purposes.
func ResolveTargetBranchForTaskWithSource(t *orcv1.Task, backend storage.Backend, cfg *config.Config) (branch, source string) {
	var init *initiative.Initiative

	// Load initiative if task belongs to one
	initiativeID := task.GetInitiativeIDProto(t)
	if t != nil && initiativeID != "" && backend != nil {
		var err error
		init, err = backend.LoadInitiative(initiativeID)
		if err != nil {
			slog.Debug("failed to load initiative for branch resolution",
				"task_id", t.Id,
				"initiative_id", initiativeID,
				"error", err,
			)
			// Continue with nil initiative - will fall through to other resolution levels
		}
	}

	return ResolveTargetBranchSource(t, init, cfg)
}

// ResolveBranchName returns the branch name for a task.
// Priority: task.BranchName (if valid) > auto-generated from task ID.
// If the custom branch name is invalid, it logs a warning and falls back to auto-generated.
func ResolveBranchName(t *orcv1.Task, gitSvc *git.Git, initiativePrefix string) string {
	// Check for user-specified branch name
	if branchName := t.GetBranchName(); branchName != "" {
		// Validate the custom branch name
		if err := git.ValidateBranchName(branchName); err != nil {
			slog.Warn("custom branch name is invalid, falling back to auto-generated name",
				"task_id", t.Id,
				"branch_name", branchName,
				"error", err,
			)
		} else {
			return branchName
		}
	}
	// Fall back to auto-generated name
	return gitSvc.BranchNameWithInitiativePrefix(t.Id, initiativePrefix)
}

