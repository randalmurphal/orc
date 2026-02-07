// Package executor implements the task execution engine.
package executor

import (
	"log/slog"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
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

// ResolveTargetBranchWithWorkflow determines the target branch for a task's PR using a 6-level
// priority hierarchy. Higher levels take precedence over lower levels:
//
//  1. Task.TargetBranch (explicit override per task)
//  2. Workflow.TargetBranch (per-workflow default)
//  3. Initiative.BranchBase (inherited from initiative)
//  4. Developer.StagingBranch (personal staging area, when enabled)
//  5. Config.Completion.TargetBranch (project default)
//  6. "main" (hardcoded fallback)
//
// Parameters:
//   - t: The task (may be nil)
//   - wf: The workflow the task uses (may be nil)
//   - init: The initiative the task belongs to (may be nil)
//   - cfg: The orc configuration (may be nil)
//
// Returns the resolved target branch name. If the resolved branch name is invalid,
// falls back to the default branch for safety.
func ResolveTargetBranchWithWorkflow(t *orcv1.Task, wf *workflow.Workflow, init *initiative.Initiative, cfg *config.Config) string {
	branch, _ := ResolveTargetBranchWithWorkflowSource(t, wf, init, cfg)
	return branch
}

// ResolveTargetBranchWithWorkflowSource returns both the resolved target branch and the source
// of that resolution for debugging/logging purposes.
//
// Uses a 6-level priority hierarchy:
//
//  1. Task.TargetBranch (explicit override per task)
//  2. Workflow.TargetBranch (per-workflow default)
//  3. Initiative.BranchBase (inherited from initiative)
//  4. Developer.StagingBranch (personal staging area, when enabled)
//  5. Config.Completion.TargetBranch (project default)
//  6. "main" (hardcoded fallback)
//
// Returns:
//   - branch: The resolved target branch name
//   - source: A human-readable description of where the branch came from
//
// If the resolved branch name is invalid, falls back to the default branch for safety.
func ResolveTargetBranchWithWorkflowSource(t *orcv1.Task, wf *workflow.Workflow, init *initiative.Initiative, cfg *config.Config) (branch, source string) {
	// Level 1: Task explicit override
	targetBranch := task.GetTargetBranchProto(t)
	if t != nil && targetBranch != "" {
		branch = targetBranch
		source = "task override"
	} else if wf != nil && wf.TargetBranch != "" {
		// Level 2: Workflow target branch
		branch = wf.TargetBranch
		source = "workflow default"
	} else if init != nil && init.BranchBase != "" {
		// Level 3: Initiative branch base
		branch = init.BranchBase
		source = "initiative branch"
	} else if cfg != nil && cfg.Developer.StagingEnabled && cfg.Developer.StagingBranch != "" {
		// Level 4: Developer staging branch (personal config)
		branch = cfg.Developer.StagingBranch
		source = "developer staging"
	} else if cfg != nil && cfg.Completion.TargetBranch != "" {
		// Level 5: Project config default
		branch = cfg.Completion.TargetBranch
		source = "project config"
	} else {
		// Level 6: Hardcoded fallback
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

// ResolveTargetBranchWithGlobalDB is a convenience function that loads both the workflow
// (from globalDB) and initiative (from backend) and uses the 6-level resolution chain.
// This is useful in contexts like finalize where you have access to globalDB but not a
// pre-loaded workflow.
//
// Parameters:
//   - t: The task (may be nil)
//   - backend: Storage backend for loading initiatives (may be nil)
//   - globalDB: Global database for loading workflows (may be nil)
//   - cfg: The orc configuration (may be nil)
//
// Returns the resolved target branch name.
func ResolveTargetBranchWithGlobalDB(t *orcv1.Task, backend storage.Backend, globalDB *db.GlobalDB, cfg *config.Config) string {
	branch, _ := ResolveTargetBranchWithGlobalDBSource(t, backend, globalDB, cfg)
	return branch
}

// ResolveTargetBranchWithGlobalDBSource is like ResolveTargetBranchWithGlobalDB but also returns
// the source of the resolution for debugging/logging purposes.
func ResolveTargetBranchWithGlobalDBSource(t *orcv1.Task, backend storage.Backend, globalDB *db.GlobalDB, cfg *config.Config) (branch, source string) {
	var init *initiative.Initiative
	var wf *workflow.Workflow

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

	// Load workflow if task specifies one
	if t != nil && t.GetWorkflowId() != "" && globalDB != nil {
		dbWf, err := globalDB.GetWorkflow(t.GetWorkflowId())
		if err == nil && dbWf != nil {
			wf = workflow.DBWorkflowToWorkflow(dbWf)
		} else if err != nil {
			slog.Debug("failed to load workflow for branch resolution",
				"task_id", t.Id,
				"workflow_id", t.GetWorkflowId(),
				"error", err,
			)
		}
	}

	return ResolveTargetBranchWithWorkflowSource(t, wf, init, cfg)
}

