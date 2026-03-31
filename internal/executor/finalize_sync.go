package executor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
)

// fetchTarget fetches the latest changes from remote.
func (e *FinalizeExecutor) fetchTarget() error {
	if e.gitSvc == nil {
		return fmt.Errorf("git service not available")
	}
	return e.gitSvc.Fetch("origin")
}

// checkDivergence returns the number of commits ahead and behind target.
func (e *FinalizeExecutor) checkDivergence(targetBranch string) (ahead int, behind int, err error) {
	if e.gitSvc == nil {
		return 0, 0, fmt.Errorf("git service not available")
	}

	target := "origin/" + targetBranch
	result, err := e.gitSvc.DetectConflicts(target)
	if err != nil {
		return 0, 0, err
	}

	return result.CommitsAhead, result.CommitsBehind, nil
}

// syncWithTarget syncs the task branch with the target branch.
func (e *FinalizeExecutor) syncWithTarget(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	targetBranch string,
	cfg config.FinalizeConfig,
) (*FinalizeResult, error) {
	result := &FinalizeResult{}

	if e.gitSvc == nil {
		return result, fmt.Errorf("git service not available")
	}

	target := "origin/" + targetBranch

	var syncResult *FinalizeResult
	var syncErr error
	switch cfg.Sync.Strategy {
	case config.FinalizeSyncRebase:
		syncResult, syncErr = e.syncViaRebase(ctx, t, p, exec, target, cfg, result)
	case config.FinalizeSyncMerge:
		syncResult, syncErr = e.syncViaMerge(ctx, t, p, exec, target, cfg, result)
	default:
		syncResult, syncErr = e.syncViaMerge(ctx, t, p, exec, target, cfg, result)
	}
	if syncErr != nil {
		return syncResult, syncErr
	}

	if syncResult.Synced {
		restored, restoreErr := e.gitSvc.RestoreOrcDir(target, t.Id)
		if restoreErr != nil {
			e.logger.Warn("failed to restore .orc/ directory", "error", restoreErr)
		} else if restored {
			e.logger.Info("restored .orc/ from target branch",
				"target", targetBranch,
				"reason", "prevent worktree contamination")
		}

		restoredSettings, restoreErr := e.gitSvc.RestoreClaudeSettings(target, t.Id)
		if restoreErr != nil {
			e.logger.Warn("failed to restore .claude/settings.json", "error", restoreErr)
		} else if restoredSettings {
			e.logger.Info("restored .claude/settings.json from target branch",
				"target", targetBranch,
				"reason", "prevent worktree hooks from being merged")
		}
	}

	return syncResult, nil
}

// syncViaMerge syncs by merging target into the task branch.
func (e *FinalizeExecutor) syncViaMerge(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	target string,
	cfg config.FinalizeConfig,
	result *FinalizeResult,
) (*FinalizeResult, error) {
	err := e.gitSvc.Merge(target, true)
	if err == nil {
		result.Synced = true
		return result, nil
	}

	if !strings.Contains(err.Error(), "CONFLICT") && !strings.Contains(err.Error(), "conflict") {
		return result, fmt.Errorf("merge failed: %w", err)
	}

	syncResult, detectErr := e.gitSvc.DetectConflicts(target)
	if detectErr == nil && syncResult.ConflictsDetected {
		result.ConflictFiles = syncResult.ConflictFiles
	}

	if cfg.ConflictResolution.Enabled && len(result.ConflictFiles) > 0 {
		e.logger.Info("conflicts detected, attempting resolution", "files", result.ConflictFiles)

		autoResolved, remaining, autoLogs := e.gitSvc.AutoResolveConflicts(result.ConflictFiles, e.logger)
		for _, log := range autoLogs {
			e.logger.Debug("auto-resolve", "msg", log)
		}

		if len(autoResolved) > 0 {
			e.logger.Info("auto-resolved conflicts", "files", autoResolved, "remaining", remaining)
		}

		if len(remaining) == 0 {
			unmerged, gitErr := e.gitSvc.Context().RunGit("diff", "--name-only", "--diff-filter=U")
			if gitErr != nil {
				return result, fmt.Errorf("check unmerged files: %w", gitErr)
			}
			if strings.TrimSpace(unmerged) == "" {
				_, commitErr := e.gitSvc.Context().RunGit("commit", "--no-edit")
				if commitErr == nil {
					result.ConflictsResolved = len(result.ConflictFiles)
					result.Synced = true
					e.logger.Info("all conflicts auto-resolved successfully")
					return result, nil
				}
			}
		}

		if len(remaining) > 0 {
			resolved, resolveErr := e.resolveConflicts(ctx, t, p, exec, remaining, cfg)
			if resolveErr != nil {
				_, _ = e.gitSvc.Context().RunGit("merge", "--abort")
				return result, fmt.Errorf("conflict resolution failed: %w", resolveErr)
			}

			if resolved {
				result.ConflictsResolved = len(result.ConflictFiles)
				result.Synced = true
				return result, nil
			}
		}
	}

	_, _ = e.gitSvc.Context().RunGit("merge", "--abort")
	return result, fmt.Errorf("merge conflicts could not be resolved: %v", result.ConflictFiles)
}

// syncViaRebase syncs by rebasing onto the target branch.
func (e *FinalizeExecutor) syncViaRebase(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	target string,
	cfg config.FinalizeConfig,
	result *FinalizeResult,
) (*FinalizeResult, error) {
	syncResult, err := e.gitSvc.RebaseWithConflictCheck(target)
	if err == nil {
		result.Synced = true
		return result, nil
	}

	if errors.Is(err, git.ErrMergeConflict) && cfg.ConflictResolution.Enabled {
		result.ConflictFiles = syncResult.ConflictFiles

		e.logger.Info("rebase conflicts detected, attempting resolution", "files", result.ConflictFiles)

		resolved, resolveErr := e.resolveRebaseConflicts(ctx, t, p, exec, result.ConflictFiles, cfg)
		if resolveErr != nil {
			return result, fmt.Errorf("rebase conflict resolution failed: %w", resolveErr)
		}

		if resolved {
			result.ConflictsResolved = len(result.ConflictFiles)
			result.Synced = true
			return result, nil
		}
	}

	return result, fmt.Errorf("rebase failed: %w", err)
}

// resolveConflicts uses Claude to resolve merge conflicts.
func (e *FinalizeExecutor) resolveConflicts(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	_ *orcv1.ExecutionState,
	conflictFiles []string,
	cfg config.FinalizeConfig,
) (bool, error) {
	prompt := buildConflictResolutionPrompt(t, conflictFiles, cfg)

	model := e.config.Model
	if model == "" {
		model = "opus"
	}

	var turnExec TurnExecutor
	sessionID := fmt.Sprintf("%s-conflict-resolution", t.Id)
	if e.turnExecutor != nil {
		turnExec = e.turnExecutor
	} else {
		claudeOpts := []ClaudeExecutorOption{
			WithClaudePath(e.claudePath),
			WithClaudeWorkdir(e.workingDir),
			WithClaudeModel(model),
			WithClaudeSessionID(sessionID),
			WithClaudeMaxTurns(5),
			WithClaudeLogger(e.logger),
			WithClaudePhaseID(p.ID),
			WithClaudeBackend(e.backend),
			WithClaudeTaskID(t.Id),
		}
		turnExec = NewClaudeExecutor(claudeOpts...)
	}

	_, execErr := turnExec.ExecuteTurn(ctx, prompt)
	if execErr != nil {
		return false, fmt.Errorf("conflict resolution turn: %w", execErr)
	}

	unmerged, gitErr := e.gitSvc.Context().RunGit("diff", "--name-only", "--diff-filter=U")
	if gitErr != nil {
		return false, fmt.Errorf("check unmerged files: %w", gitErr)
	}
	if strings.TrimSpace(unmerged) == "" {
		_, commitErr := e.gitSvc.Context().RunGit("commit", "--no-edit")
		return commitErr == nil, commitErr
	}

	remainingFiles := strings.Split(strings.TrimSpace(unmerged), "\n")
	return false, fmt.Errorf("conflict resolution incomplete: %d files still unmerged: %v", len(remainingFiles), remainingFiles)
}

// resolveRebaseConflicts resolves conflicts during rebase.
func (e *FinalizeExecutor) resolveRebaseConflicts(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	_ []string,
	cfg config.FinalizeConfig,
) (bool, error) {
	const maxAttempts = 10

	for attempt := range maxAttempts {
		unmerged, gitErr := e.gitSvc.Context().RunGit("diff", "--name-only", "--diff-filter=U")
		if gitErr != nil {
			_ = e.gitSvc.AbortRebase()
			return false, fmt.Errorf("check unmerged files at attempt %d: %w", attempt, gitErr)
		}
		unmergedFiles := strings.Split(strings.TrimSpace(unmerged), "\n")
		if len(unmergedFiles) == 0 || (len(unmergedFiles) == 1 && unmergedFiles[0] == "") {
			_, err := e.gitSvc.Context().RunGit("rebase", "--continue")
			if err == nil || strings.Contains(err.Error(), "No rebase in progress") {
				return true, nil
			}
			continue
		}

		autoResolved, remaining, autoLogs := e.gitSvc.AutoResolveConflicts(unmergedFiles, e.logger)
		for _, log := range autoLogs {
			e.logger.Debug("auto-resolve during rebase", "msg", log)
		}

		if len(autoResolved) > 0 {
			e.logger.Info("auto-resolved rebase conflicts", "files", autoResolved, "remaining", remaining)
		}

		if len(remaining) > 0 {
			resolved, err := e.resolveConflicts(ctx, t, p, exec, remaining, cfg)
			if err != nil || !resolved {
				_ = e.gitSvc.AbortRebase()
				return false, fmt.Errorf("failed to resolve rebase conflict at attempt %d: %w", attempt, err)
			}
		}

		for _, f := range unmergedFiles {
			if _, stageErr := e.gitSvc.Context().RunGit("add", f); stageErr != nil {
				e.logger.Warn("failed to stage resolved file", "file", f, "error", stageErr)
			}
		}

		_, continueErr := e.gitSvc.Context().RunGit("rebase", "--continue")
		if continueErr == nil || strings.Contains(continueErr.Error(), "No rebase in progress") {
			return true, nil
		}
	}

	_ = e.gitSvc.AbortRebase()
	return false, fmt.Errorf("rebase conflict resolution exceeded max attempts")
}

// buildConflictResolutionPrompt creates the prompt for conflict resolution.
func buildConflictResolutionPrompt(t *orcv1.Task, conflictFiles []string, cfg config.FinalizeConfig) string {
	conflictCfg := cfg.ConflictResolution
	var sb strings.Builder

	sb.WriteString("# Conflict Resolution Task\n\n")
	sb.WriteString("You are resolving merge conflicts for task: ")
	sb.WriteString(t.Id)
	sb.WriteString(" - ")
	sb.WriteString(t.Title)
	sb.WriteString("\n\n")

	sb.WriteString("## Conflicted Files\n\n")
	for _, f := range conflictFiles {
		sb.WriteString("- `")
		sb.WriteString(f)
		sb.WriteString("`\n")
	}

	sb.WriteString("\n## Conflict Resolution Rules\n\n")
	sb.WriteString("**CRITICAL - You MUST follow these rules:**\n\n")
	sb.WriteString("1. **NEVER remove features** - Both your changes AND upstream changes must be preserved\n")
	sb.WriteString("2. **Merge intentions, not text** - Understand what each side was trying to accomplish\n")
	sb.WriteString("3. **Prefer additive resolution** - If in doubt, keep both implementations\n")
	sb.WriteString("4. **Test after every file** - Don't batch conflict resolutions\n\n")

	sb.WriteString("## Prohibited Resolutions\n\n")
	sb.WriteString("- **NEVER**: Just take \"ours\" or \"theirs\" without understanding\n")
	sb.WriteString("- **NEVER**: Remove upstream features to fix conflicts\n")
	sb.WriteString("- **NEVER**: Remove your features to fix conflicts\n")
	sb.WriteString("- **NEVER**: Comment out conflicting code\n\n")

	if conflictCfg.Instructions != "" {
		sb.WriteString("## Additional Instructions\n\n")
		sb.WriteString(conflictCfg.Instructions)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. For each conflicted file, read and understand both sides of the conflict\n")
	sb.WriteString("2. Resolve the conflict by merging both changes appropriately\n")
	sb.WriteString("3. Stage the resolved file with `git add <file>`\n")
	sb.WriteString("4. After all files are resolved, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "complete", "summary": "Resolved X conflicts in files A, B, C"}`)
	sb.WriteString("\n\nIf you cannot resolve a conflict, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "blocked", "reason": "[explanation]"}`)
	sb.WriteString("\n")

	return sb.String()
}
