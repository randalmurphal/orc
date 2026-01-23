// Package executor provides PR/merge completion actions for task execution.
package executor

import (
	"errors"
	"strings"
)

// ErrSyncConflict is returned when sync encounters merge conflicts.
var ErrSyncConflict = errors.New("sync conflict detected")

// ErrTaskBlocked is returned when task execution completes but requires
// user intervention (e.g., sync conflicts, merge failures).
var ErrTaskBlocked = errors.New("task blocked")

// SyncPhase indicates when sync is being performed.
type SyncPhase string

const (
	// SyncPhaseStart indicates sync at task start or phase start
	SyncPhaseStart SyncPhase = "start"
	// SyncPhaseCompletion indicates sync before PR/merge
	SyncPhaseCompletion SyncPhase = "completion"
)

// ErrDirectMergeBlocked is returned when direct merge to a protected branch is blocked.
var ErrDirectMergeBlocked = errors.New("direct merge to protected branch blocked")

// PRReviewResult contains the result of an AI review.
type PRReviewResult struct {
	Approved bool   // Whether the PR should be approved
	Comment  string // Review comment/reason
}

// ErrGHNotAuthenticated is returned when gh CLI is not authenticated.
var ErrGHNotAuthenticated = errors.New("GitHub CLI not authenticated")

// isLabelError checks if an error is related to missing labels.
// GitHub CLI returns errors like "could not add label: <name> not found".
func isLabelError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "label") &&
		(strings.Contains(errStr, "not found") || strings.Contains(errStr, "could not add"))
}

// isAuthError checks if an error is related to gh CLI authentication.
// Common patterns:
// - "gh: not logged in" (older gh versions)
// - "not authenticated" (from CheckGHAuth)
// - "authentication required"
// - "failed to authenticate"
// - "401" or "Unauthorized"
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not logged in") ||
		strings.Contains(errStr, "not authenticated") ||
		strings.Contains(errStr, "authentication required") ||
		strings.Contains(errStr, "failed to authenticate") ||
		strings.Contains(errStr, "401") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "auth token")
}

// isNonFastForwardError checks if an error is a git non-fast-forward push rejection.
// This occurs when the local branch has diverged from the remote branch,
// typically when re-running a completed task from scratch.
// Common patterns:
// - "non-fast-forward" (standard git message)
// - "rejected" + "fetch first" (alternative git message)
// - "failed to push some refs" + "behind" (hint text)
func isNonFastForwardError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "non-fast-forward") ||
		(strings.Contains(errStr, "rejected") && strings.Contains(errStr, "fetch first")) ||
		(strings.Contains(errStr, "failed to push") && strings.Contains(errStr, "behind"))
}

// isAutoMergeConfigError checks if an error is due to auto-merge not being available
// on the repository. This is expected behavior for repos without auto-merge enabled
// (requires branch protection rules or explicit repo settings).
// Common patterns from GitHub CLI:
// - "auto-merge is not allowed" (repo doesn't allow auto-merge)
// - "pull request is not mergeable" (missing required reviews/checks)
// - "auto-merge can not be enabled" (branch protection prevents it)
// - "auto merge is not allowed" (alternative phrasing)
// - "not eligible for auto-merge" (various eligibility issues)
func isAutoMergeConfigError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "auto-merge is not allowed") ||
		strings.Contains(errStr, "auto merge is not allowed") ||
		strings.Contains(errStr, "auto-merge can not be enabled") ||
		strings.Contains(errStr, "auto merge can not be enabled") ||
		strings.Contains(errStr, "not eligible for auto-merge") ||
		strings.Contains(errStr, "not eligible for auto merge") ||
		strings.Contains(errStr, "pull request is not mergeable") ||
		strings.Contains(errStr, "is in clean status") // PR is already clean/merged
}
