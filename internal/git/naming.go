// Package git provides git operations for orc.
package git

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// MaxBranchNameLength is the maximum allowed length for branch names.
const MaxBranchNameLength = 256

// ErrInvalidBranchName indicates a branch name failed validation.
var ErrInvalidBranchName = errors.New("invalid branch name")

// branchNamePattern validates branch names: alphanumeric, slash, hyphen, underscore, dot.
// Must start with alphanumeric.
var branchNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9/_.-]*$`)

// gitReservedNames contains branch names reserved by git.
var gitReservedNames = map[string]bool{
	"head": true, // HEAD (case-insensitive)
}

// ValidateBranchName validates a branch name for security and git compatibility.
// Returns an error describing the validation failure, or nil if valid.
//
// Validation rules:
//   - Must not be empty
//   - Must not exceed MaxBranchNameLength characters
//   - Must start with alphanumeric character
//   - May only contain: a-z, A-Z, 0-9, /, -, _, .
//   - Must not contain: spaces, shell metacharacters ($`|;&()<>!?*[]{}\)
//   - Must not contain path traversal (..)
//   - Must not start with - or .
//   - Must not end with .lock or .
//   - Must not be a git reserved name (HEAD)
//   - Must not contain git revision syntax (@{)
//   - Components must not start or end with .
func ValidateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: cannot be empty", ErrInvalidBranchName)
	}
	if len(name) > MaxBranchNameLength {
		return fmt.Errorf("%w: exceeds maximum length of %d characters", ErrInvalidBranchName, MaxBranchNameLength)
	}

	// Check git reserved names (case-insensitive)
	if gitReservedNames[strings.ToLower(name)] {
		return fmt.Errorf("%w: '%s' is a reserved name", ErrInvalidBranchName, name)
	}

	// Check for git revision syntax
	if strings.Contains(name, "@{") {
		return fmt.Errorf("%w: cannot contain '@{' (git revision syntax)", ErrInvalidBranchName)
	}
	// Single @ is not allowed (it's shorthand for HEAD)
	if name == "@" {
		return fmt.Errorf("%w: '@' alone is not allowed (it's shorthand for HEAD)", ErrInvalidBranchName)
	}

	if strings.Contains(name, "..") {
		return fmt.Errorf("%w: cannot contain '..'", ErrInvalidBranchName)
	}
	if strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("%w: cannot end with '.lock'", ErrInvalidBranchName)
	}
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("%w: cannot end with '.'", ErrInvalidBranchName)
	}
	if strings.HasSuffix(name, "/") {
		return fmt.Errorf("%w: cannot end with '/'", ErrInvalidBranchName)
	}
	if strings.Contains(name, "//") {
		return fmt.Errorf("%w: cannot contain '//'", ErrInvalidBranchName)
	}
	if strings.Contains(name, "/.") {
		return fmt.Errorf("%w: path components cannot start with '.'", ErrInvalidBranchName)
	}
	if strings.Contains(name, "./") {
		return fmt.Errorf("%w: path components cannot end with '.'", ErrInvalidBranchName)
	}
	if !branchNamePattern.MatchString(name) {
		return fmt.Errorf("%w: contains invalid characters (allowed: a-z, A-Z, 0-9, /, -, _, .)", ErrInvalidBranchName)
	}
	return nil
}

// DefaultBranchPrefix is the default prefix for task branches when no initiative is set.
const DefaultBranchPrefix = "orc/"

// BranchName returns the full branch name for a task with optional executor prefix.
// Solo mode: orc/TASK-001
// P2P/Team mode: orc/TASK-001-am (where "am" is executor's initials)
func BranchName(taskID, executorPrefix string) string {
	return BranchNameWithPrefix(taskID, executorPrefix, "")
}

// BranchNameWithPrefix returns the full branch name for a task with an optional
// initiative prefix and executor prefix.
//
// When initiativePrefix is empty, uses the default "orc/" prefix:
//   - Solo mode: orc/TASK-001
//   - P2P/Team mode: orc/TASK-001-am (where "am" is executor's initials)
//
// When initiativePrefix is set (e.g., "feature/auth-"), it replaces the default prefix:
//   - Solo mode: feature/auth-TASK-001
//   - P2P/Team mode: feature/auth-TASK-001-am
//
// This allows tasks belonging to an initiative to be grouped under a custom branch namespace.
func BranchNameWithPrefix(taskID, executorPrefix, initiativePrefix string) string {
	prefix := DefaultBranchPrefix
	if initiativePrefix != "" {
		prefix = initiativePrefix
	}

	if executorPrefix == "" {
		return fmt.Sprintf("%s%s", prefix, taskID)
	}
	return fmt.Sprintf("%s%s-%s", prefix, taskID, strings.ToLower(executorPrefix))
}

// WorktreeDirName returns the directory name for a task's worktree.
// Solo mode: orc-TASK-001
// P2P/Team mode: orc-TASK-001-am
func WorktreeDirName(taskID, executorPrefix string) string {
	return WorktreeDirNameWithPrefix(taskID, executorPrefix, "")
}

// WorktreeDirNameWithPrefix returns the directory name for a task's worktree
// with an optional initiative prefix.
//
// The directory name is derived from the branch name with slashes replaced by hyphens
// to create a valid directory name.
//
// Examples:
//   - Default (no initiative): orc-TASK-001 or orc-TASK-001-am
//   - With initiative prefix "feature/auth-": feature-auth-TASK-001 or feature-auth-TASK-001-am
func WorktreeDirNameWithPrefix(taskID, executorPrefix, initiativePrefix string) string {
	// Get the branch name, then convert slashes to hyphens for directory safety
	branchName := BranchNameWithPrefix(taskID, executorPrefix, initiativePrefix)
	// Replace / with - to make it a valid directory name
	return strings.ReplaceAll(branchName, "/", "-")
}

// WorktreePath returns the full path to a task's worktree.
// worktreeDir is the base directory for worktrees (e.g., ".orc/worktrees")
func WorktreePath(worktreeDir, taskID, executorPrefix string) string {
	return WorktreePathWithPrefix(worktreeDir, taskID, executorPrefix, "")
}

// WorktreePathWithPrefix returns the full path to a task's worktree with initiative prefix support.
// worktreeDir is the base directory for worktrees (e.g., ".orc/worktrees")
// initiativePrefix is the branch prefix from the initiative (e.g., "feature/auth-")
func WorktreePathWithPrefix(worktreeDir, taskID, executorPrefix, initiativePrefix string) string {
	return filepath.Join(worktreeDir, WorktreeDirNameWithPrefix(taskID, executorPrefix, initiativePrefix))
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
