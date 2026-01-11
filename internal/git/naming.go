// Package git provides git operations for orc, wrapping devflow/git.
package git

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BranchName returns the full branch name for a task with optional executor prefix.
// Solo mode: orc/TASK-001
// P2P/Team mode: orc/TASK-001-am (where "am" is executor's initials)
func BranchName(taskID, executorPrefix string) string {
	if executorPrefix == "" {
		return fmt.Sprintf("orc/%s", taskID)
	}
	return fmt.Sprintf("orc/%s-%s", taskID, strings.ToLower(executorPrefix))
}

// WorktreeDirName returns the directory name for a task's worktree.
// Solo mode: orc-TASK-001
// P2P/Team mode: orc-TASK-001-am
func WorktreeDirName(taskID, executorPrefix string) string {
	if executorPrefix == "" {
		return fmt.Sprintf("orc-%s", taskID)
	}
	return fmt.Sprintf("orc-%s-%s", taskID, strings.ToLower(executorPrefix))
}

// WorktreePath returns the full path to a task's worktree.
// worktreeDir is the base directory for worktrees (e.g., ".orc/worktrees")
func WorktreePath(worktreeDir, taskID, executorPrefix string) string {
	return filepath.Join(worktreeDir, WorktreeDirName(taskID, executorPrefix))
}

// ParseBranchName extracts the task ID and executor prefix from a branch name.
// Returns (taskID, executorPrefix, ok).
// Examples:
//   - "orc/TASK-001" -> ("TASK-001", "", true)
//   - "orc/TASK-001-am" -> ("TASK-001", "am", true)
//   - "orc/TASK-AM-001-bj" -> ("TASK-AM-001", "bj", true)
//   - "main" -> ("", "", false)
func ParseBranchName(branch string) (taskID, executorPrefix string, ok bool) {
	if !strings.HasPrefix(branch, "orc/") {
		return "", "", false
	}

	name := strings.TrimPrefix(branch, "orc/")

	// Check for executor suffix pattern: ends with -XX where XX is 2-3 lowercase letters
	// We need to distinguish between:
	// - TASK-001-am (task=TASK-001, executor=am)
	// - TASK-AM-001 (task=TASK-AM-001, no executor)
	// - TASK-AM-001-bj (task=TASK-AM-001, executor=bj)

	// Find the last hyphen-separated segment
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return name, "", true
	}

	lastPart := parts[len(parts)-1]

	// Check if last part looks like an executor prefix (2-3 lowercase letters)
	if len(lastPart) >= 2 && len(lastPart) <= 3 && isLowerAlpha(lastPart) {
		// Check if removing this part would leave a valid task ID
		// Valid task IDs end with a number (e.g., TASK-001, TASK-AM-001)
		remaining := strings.Join(parts[:len(parts)-1], "-")
		if len(remaining) > 0 {
			remainingParts := strings.Split(remaining, "-")
			lastRemaining := remainingParts[len(remainingParts)-1]
			if isNumeric(lastRemaining) {
				// This looks like taskID-executor pattern
				return remaining, lastPart, true
			}
		}
	}

	// No executor suffix found
	return name, "", true
}

// isLowerAlpha checks if a string contains only lowercase letters.
func isLowerAlpha(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// isNumeric checks if a string contains only digits.
func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
