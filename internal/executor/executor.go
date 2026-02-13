// Package executor provides the workflow-based execution engine for orc.
package executor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// commonClaudeLocations contains paths where Claude CLI is commonly installed.
// Order matters - check most common locations first.
var commonClaudeLocations = []string{
	// User-local installs (npm global, homebrew user)
	"~/.local/bin/claude",
	"~/.claude/local/claude",
	"~/.npm-global/bin/claude",
	// System installs (homebrew, apt, manual)
	"/usr/local/bin/claude",
	"/opt/homebrew/bin/claude",
	"/usr/bin/claude",
	// macOS-specific paths
	"/opt/local/bin/claude",
	// Linux snap install
	"/snap/bin/claude",
}

// ResolveClaudePath resolves a Claude CLI path to an absolute path.
// This is necessary because when cmd.Dir is set (e.g., for worktrees),
// Go's exec.Command won't perform PATH lookup for relative executables.
// By resolving to absolute path upfront, execution works regardless of cmd.Dir.
//
// Resolution order:
//  1. Empty string - returned unchanged
//  2. Already absolute - returned unchanged
//  3. PATH lookup - uses exec.LookPath for relative names like "claude"
//  4. Common locations - checks well-known install paths as fallback
func ResolveClaudePath(path string) string {
	if path == "" {
		return path
	}

	// Expand tilde to home directory first
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(homeDir, path[2:])
		}
	}

	if filepath.IsAbs(path) {
		return path
	}

	// Resolve relative path to absolute using PATH lookup
	if absPath, err := exec.LookPath(path); err == nil {
		return absPath
	}

	// If the path is "claude" (the default), try common install locations
	if path == "claude" {
		if found := findClaudeInCommonLocations(); found != "" {
			return found
		}
	}

	return path // Fall back to original if all lookups fail
}

// findClaudeInCommonLocations checks common Claude install paths.
// Returns the first valid executable found, or empty string if none.
func findClaudeInCommonLocations() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "" // Will skip ~ paths
	}

	for _, loc := range commonClaudeLocations {
		path := loc
		// Expand ~ to home directory
		if strings.HasPrefix(path, "~/") && homeDir != "" {
			path = filepath.Join(homeDir, path[2:])
		} else if strings.HasPrefix(path, "~/") {
			continue // Skip ~ paths if we couldn't get home dir
		}

		// Check if file exists and is executable
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// On Unix, check executable bit
			if info.Mode()&0111 != 0 {
				return path
			}
		}
	}
	return ""
}

// Result represents the result of a phase execution.
// Used by FinalizeExecutor for finalize phase results.
type Result struct {
	Phase        string
	Status       orcv1.PhaseStatus
	Iterations   int
	Duration     time.Duration
	Output       string
	Error        error
	Artifacts    []string
	CommitSHA    string
	InputTokens  int
	OutputTokens int
	CostUSD      float64 // Total cost in USD for this phase
	Model        string  // Model used for this phase (e.g., "opus", "sonnet")

	// Cache token tracking (for cost analytics)
	CacheCreationTokens int // Tokens used to create new cache entries
	CacheReadTokens     int // Tokens read from existing cache
}
