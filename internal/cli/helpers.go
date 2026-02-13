// Package cli implements the orc command-line interface.
// This file contains shared helper functions used across multiple commands.
package cli

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/progress"
)

// errorJSON represents an error in JSON output
type errorJSON struct {
	Error string `json:"error"`
}

// outputJSONError outputs an error as JSON to the command's output stream.
// This should be called when jsonOut is true and an error occurs.
func outputJSONError(cmd *cobra.Command, err error) {
	output := errorJSON{Error: err.Error()}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(output)
}

// parseConflictFilesFromError extracts conflict file names from an error string.
// Looks for file list in brackets: [file1 file2 file3]
// This matches the format from ErrSyncConflict error messages.
func parseConflictFilesFromError(errStr string) []string {
	var files []string

	// Look for file list in brackets: [file1 file2 file3]
	startBracket := strings.Index(errStr, "[")
	endBracket := strings.LastIndex(errStr, "]")

	if startBracket >= 0 && endBracket > startBracket {
		fileListStr := errStr[startBracket+1 : endBracket]
		// Split by space, handling empty strings
		for _, f := range strings.Fields(fileListStr) {
			// Clean up any extra punctuation
			f = strings.Trim(f, ",")
			if f != "" {
				files = append(files, f)
			}
		}
	}

	return files
}

// containsPhase checks if a phase ID exists in a list of phases.
func containsPhase(phases []string, phaseID string) bool {
	for _, p := range phases {
		if p == phaseID {
			return true
		}
	}
	return false
}

// getTaskFileChangeStats computes diff statistics for the task branch vs target branch.
// Returns nil if stats cannot be computed (best-effort summary data).
func getTaskFileChangeStats(ctx context.Context, projectRoot, taskBranch string, cfg *config.Config) *progress.FileChangeStats {
	targetBranch := "main"
	if cfg != nil && cfg.Completion.TargetBranch != "" {
		targetBranch = cfg.Completion.TargetBranch
	}

	diffSvc := diff.NewService(projectRoot, nil)
	resolvedBase := diffSvc.ResolveRef(ctx, targetBranch)

	stats, err := diffSvc.GetStats(ctx, resolvedBase, taskBranch)
	if err != nil {
		return nil
	}

	return &progress.FileChangeStats{
		FilesChanged: stats.FilesChanged,
		Additions:    stats.Additions,
		Deletions:    stats.Deletions,
	}
}

// buildBlockedContextProto creates progress context for blocked task display (proto version).
// Used by finalize, resume, and other commands that handle blocked tasks.
func buildBlockedContextProto(t *orcv1.Task, cfg *config.Config, projectRoot string) *progress.BlockedContext {
	ctx := &progress.BlockedContext{}
	if t == nil {
		return ctx
	}

	// Get worktree path from task ID and config
	if cfg != nil && cfg.Worktree.Enabled {
		resolvedDir := config.ResolveWorktreeDir(cfg.Worktree.Dir, projectRoot)
		ctx.WorktreePath = resolvedDir + "/orc-" + t.Id
	}

	// Extract conflict files from task metadata if available
	if t.Metadata != nil {
		if errStr, ok := t.Metadata["blocked_error"]; ok {
			ctx.ConflictFiles = parseConflictFilesFromError(errStr)
		}
	}

	// Set sync strategy based on config
	if cfg != nil {
		if cfg.Completion.Finalize.Sync.Strategy == config.FinalizeSyncMerge {
			ctx.SyncStrategy = progress.SyncStrategyMerge
		} else {
			ctx.SyncStrategy = progress.SyncStrategyRebase
		}

		// Set target branch
		ctx.TargetBranch = cfg.Completion.TargetBranch
		if ctx.TargetBranch == "" {
			ctx.TargetBranch = "main"
		}
	}

	return ctx
}
