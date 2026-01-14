// Package diff provides git diff computation and caching for the web UI.
package diff

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DiffStats contains summary statistics for a diff.
type DiffStats struct {
	FilesChanged int `json:"files_changed"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
}

// FileDiff represents changes to a single file.
type FileDiff struct {
	Path      string `json:"path"`
	Status    string `json:"status"`             // modified, added, deleted, renamed
	OldPath   string `json:"old_path,omitempty"` // For renames
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Binary    bool   `json:"binary"`
	Syntax    string `json:"syntax"` // Language for highlighting
	Hunks     []Hunk `json:"hunks,omitempty"`
}

// Hunk represents a continuous block of changes.
type Hunk struct {
	OldStart int    `json:"old_start"`
	OldLines int    `json:"old_lines"`
	NewStart int    `json:"new_start"`
	NewLines int    `json:"new_lines"`
	Lines    []Line `json:"lines"`
}

// Line represents a single line in the diff.
type Line struct {
	Type    string `json:"type"` // context, addition, deletion
	Content string `json:"content"`
	OldLine int    `json:"old_line,omitempty"`
	NewLine int    `json:"new_line,omitempty"`
}

// DiffResult contains the complete diff response.
type DiffResult struct {
	Base  string     `json:"base"`
	Head  string     `json:"head"`
	Stats DiffStats  `json:"stats"`
	Files []FileDiff `json:"files"`
}

// Service provides diff computation.
type Service struct {
	repoPath string
	cache    *Cache
}

// NewService creates a new diff service.
func NewService(repoPath string, cache *Cache) *Service {
	return &Service{repoPath: repoPath, cache: cache}
}

// ResolveRef resolves a git reference, trying multiple fallbacks:
// 1. The ref as given (works for local branches and remote refs)
// 2. origin/<ref> for remote tracking branches
// Returns the resolved ref or the original if no better option found.
func (s *Service) ResolveRef(ctx context.Context, ref string) string {
	// Check if ref exists directly
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--quiet", ref)
	cmd.Dir = s.repoPath
	if err := cmd.Run(); err == nil {
		return ref
	}

	// Try origin/<ref> for remote tracking branches
	remoteRef := "origin/" + ref
	cmd = exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--quiet", remoteRef)
	cmd.Dir = s.repoPath
	if err := cmd.Run(); err == nil {
		return remoteRef
	}

	// Return original ref - git commands will fail with appropriate error
	return ref
}

// ShouldIncludeWorkingTree checks if the diff should include uncommitted changes.
// This is true when:
// 1. head branch has not diverged from base (same commit)
// 2. There are uncommitted changes in the working tree
// Returns true if working tree should be included, along with the effective head ref.
func (s *Service) ShouldIncludeWorkingTree(ctx context.Context, base, head string) (bool, string) {
	// Get commit SHAs for base and head
	baseCmd := exec.CommandContext(ctx, "git", "rev-parse", base)
	baseCmd.Dir = s.repoPath
	baseOut, err := baseCmd.Output()
	if err != nil {
		return false, head
	}
	baseSHA := strings.TrimSpace(string(baseOut))

	headCmd := exec.CommandContext(ctx, "git", "rev-parse", head)
	headCmd.Dir = s.repoPath
	headOut, err := headCmd.Output()
	if err != nil {
		return false, head
	}
	headSHA := strings.TrimSpace(string(headOut))

	// If head has diverged from base, use normal branch comparison
	if baseSHA != headSHA {
		return false, head
	}

	// Check if there are uncommitted changes relative to base
	// Using git diff with no args against base shows working tree changes
	diffCmd := exec.CommandContext(ctx, "git", "diff", "--quiet", base, "--")
	diffCmd.Dir = s.repoPath
	if err := diffCmd.Run(); err != nil {
		// Non-zero exit means there are differences - working tree has changes
		return true, ""
	}

	// Also check for staged changes
	stagedCmd := exec.CommandContext(ctx, "git", "diff", "--quiet", "--cached", base, "--")
	stagedCmd.Dir = s.repoPath
	if err := stagedCmd.Run(); err != nil {
		// Non-zero exit means there are staged changes
		return true, ""
	}

	// No uncommitted changes
	return false, head
}

// GetStats returns diff statistics without file contents.
// If head is empty, compares base against the working tree (uncommitted changes).
func (s *Service) GetStats(ctx context.Context, base, head string) (*DiffStats, error) {
	var cmd *exec.Cmd

	if head == "" {
		// Compare base to working tree (uncommitted changes)
		cmd = exec.CommandContext(ctx, "git", "diff", "--shortstat", base, "--")
	} else {
		// Use git diff --stat with shortstat for summary
		cmd = exec.CommandContext(ctx, "git", "diff", "--shortstat", base+"..."+head)
	}
	cmd.Dir = s.repoPath
	output, err := cmd.Output()
	if err != nil && head != "" {
		// Try without three-dot notation for unrelated branches
		cmd = exec.CommandContext(ctx, "git", "diff", "--shortstat", base, head)
		cmd.Dir = s.repoPath
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git diff stat: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("git diff stat: %w", err)
	}
	return parseStats(string(output))
}

// GetFileList returns the list of changed files without content.
// If head is empty, compares base against the working tree (uncommitted changes).
func (s *Service) GetFileList(ctx context.Context, base, head string) ([]FileDiff, error) {
	var cmd *exec.Cmd
	var statusCmd *exec.Cmd

	if head == "" {
		// Compare base to working tree (uncommitted changes)
		cmd = exec.CommandContext(ctx, "git", "diff", "--numstat", base, "--")
		statusCmd = exec.CommandContext(ctx, "git", "diff", "--name-status", "-M", base, "--")
	} else {
		// Use git diff --numstat for file list with additions/deletions count
		cmd = exec.CommandContext(ctx, "git", "diff", "--numstat", base+"..."+head)
		statusCmd = exec.CommandContext(ctx, "git", "diff", "--name-status", "-M", base+"..."+head)
	}
	cmd.Dir = s.repoPath
	output, err := cmd.Output()
	if err != nil && head != "" {
		// Try without three-dot notation
		cmd = exec.CommandContext(ctx, "git", "diff", "--numstat", base, head)
		cmd.Dir = s.repoPath
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git diff numstat: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("git diff numstat: %w", err)
	}

	files, err := parseNumstat(string(output))
	if err != nil {
		return nil, err
	}

	// Get status (added, modified, deleted, renamed) using --name-status
	statusCmd.Dir = s.repoPath
	statusOutput, err := statusCmd.Output()
	if err != nil && head != "" {
		statusCmd = exec.CommandContext(ctx, "git", "diff", "--name-status", "-M", base, head)
		statusCmd.Dir = s.repoPath
		statusOutput, _ = statusCmd.Output()
	}

	statusMap := parseNameStatus(string(statusOutput))
	for i := range files {
		if info, ok := statusMap[files[i].Path]; ok {
			files[i].Status = info.Status
			files[i].OldPath = info.OldPath
		}
	}

	// Ensure we never return nil (Go JSON serializes nil slices as null, which crashes frontend)
	if files == nil {
		files = []FileDiff{}
	}

	return files, nil
}

// GetFileDiff returns the diff for a single file with hunks.
// If head is empty, compares base against the working tree (uncommitted changes).
func (s *Service) GetFileDiff(ctx context.Context, base, head, filePath string) (*FileDiff, error) {
	// Check cache first (don't cache working tree diffs as they can change)
	if s.cache != nil && head != "" {
		cacheKey := fmt.Sprintf("%s..%s:%s", base, head, filePath)
		if cached := s.cache.Get(cacheKey); cached != nil {
			return cached, nil
		}
	}

	var cmd *exec.Cmd
	if head == "" {
		// Compare base to working tree (uncommitted changes)
		cmd = exec.CommandContext(ctx, "git", "diff", "--histogram", "-U3", base, "--", filePath)
	} else {
		// Use git diff with histogram algorithm for better diffs
		cmd = exec.CommandContext(ctx, "git", "diff", "--histogram", "-U3", base+"..."+head, "--", filePath)
	}
	cmd.Dir = s.repoPath
	output, err := cmd.Output()
	if err != nil && head != "" {
		// Try without three-dot notation
		cmd = exec.CommandContext(ctx, "git", "diff", "--histogram", "-U3", base, head, "--", filePath)
		cmd.Dir = s.repoPath
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git diff file: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("git diff file: %w", err)
	}

	diff := parseFileDiff(string(output), filePath)
	diff.Syntax = detectSyntax(filePath)

	// Cache result if cache available (don't cache working tree diffs)
	if s.cache != nil && head != "" {
		cacheKey := fmt.Sprintf("%s..%s:%s", base, head, filePath)
		s.cache.Set(cacheKey, diff)
	}

	return diff, nil
}

// GetFullDiff returns the complete diff with all files and hunks.
// For large diffs, prefer GetFileList + GetFileDiff for individual files.
func (s *Service) GetFullDiff(ctx context.Context, base, head string) (*DiffResult, error) {
	stats, err := s.GetStats(ctx, base, head)
	if err != nil {
		return nil, err
	}

	files, err := s.GetFileList(ctx, base, head)
	if err != nil {
		return nil, err
	}

	// Get hunks for each file
	for i := range files {
		if files[i].Binary {
			continue
		}
		fileDiff, err := s.GetFileDiff(ctx, base, head, files[i].Path)
		if err == nil {
			files[i].Hunks = fileDiff.Hunks
		}
	}

	return &DiffResult{
		Base:  base,
		Head:  head,
		Stats: *stats,
		Files: files,
	}, nil
}

// FileStatus contains status information for a file.
type FileStatus struct {
	Status  string
	OldPath string
}

// parseNameStatus parses git diff --name-status output.
func parseNameStatus(output string) map[string]FileStatus {
	result := make(map[string]FileStatus)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		statusCode := parts[0]
		var status, path, oldPath string

		switch {
		case statusCode == "A":
			status = "added"
			path = parts[1]
		case statusCode == "D":
			status = "deleted"
			path = parts[1]
		case statusCode == "M":
			status = "modified"
			path = parts[1]
		case strings.HasPrefix(statusCode, "R"):
			status = "renamed"
			if len(parts) >= 3 {
				oldPath = parts[1]
				path = parts[2]
			} else {
				path = parts[1]
			}
		case strings.HasPrefix(statusCode, "C"):
			status = "copied"
			if len(parts) >= 3 {
				oldPath = parts[1]
				path = parts[2]
			} else {
				path = parts[1]
			}
		default:
			status = "modified"
			path = parts[1]
		}

		result[path] = FileStatus{Status: status, OldPath: oldPath}
	}

	return result
}
