package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/storage"
)

// newTestBackend creates a test backend using in-memory database for speed.
func newTestBackend(t *testing.T) storage.Backend {
	t.Helper()
	return storage.NewTestBackend(t)
}

func TestResolveClaudePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		wantAbs  bool // Whether result should be absolute
		wantSame bool // Whether result should be same as input
	}{
		{
			name:     "empty string",
			input:    "",
			wantSame: true,
		},
		{
			name:     "already absolute",
			input:    "/usr/local/bin/claude",
			wantAbs:  true,
			wantSame: true,
		},
		{
			name:    "relative claude",
			input:   "claude",
			wantAbs: true, // Should resolve to absolute if claude exists in PATH
		},
		{
			name:     "relative nonexistent",
			input:    "nonexistent-binary-xyz",
			wantSame: true, // Falls back to original if not found
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ResolveClaudePath(tc.input)

			if tc.wantSame && result != tc.input {
				t.Errorf("ResolveClaudePath(%q) = %q, want %q", tc.input, result, tc.input)
			}

			if tc.wantAbs && result != "" && !filepath.IsAbs(result) {
				// Only check for absolute if we expect it AND claude is actually in PATH
				// If claude isn't installed, it should fall back to the original
				if tc.input != "claude" {
					t.Errorf("ResolveClaudePath(%q) = %q, want absolute path", tc.input, result)
				}
			}
		})
	}
}

func TestFindClaudeInCommonLocations(t *testing.T) {
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
	// Create a temp directory with a fake claude binary
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	// Create the fake binary file with executable permissions
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/sh\necho fake"), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	// Save original and replace with test locations
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{
		"/nonexistent/path/claude", // Won't exist
		fakeClaude,                 // Should be found
	}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != fakeClaude {
		t.Errorf("findClaudeInCommonLocations() = %q, want %q", result, fakeClaude)
	}
}

func TestFindClaudeInCommonLocations_HomeExpansion(t *testing.T) {
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
	// This test verifies ~ expansion works
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("could not determine home directory")
	}

	// Create a temp subdir in home
	testDir := filepath.Join(homeDir, ".orc-test-"+t.Name())
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(testDir) }()

	fakeClaude := filepath.Join(testDir, "claude")
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/sh\necho fake"), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	// Save original and replace with test locations using ~
	originalLocations := commonClaudeLocations
	relativePath := "~/" + filepath.Base(testDir) + "/claude"
	commonClaudeLocations = []string{relativePath}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != fakeClaude {
		t.Errorf("findClaudeInCommonLocations() = %q, want %q (expanded from %q)", result, fakeClaude, relativePath)
	}
}

func TestFindClaudeInCommonLocations_NoMatch(t *testing.T) {
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
	// Save original and replace with nonexistent locations
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{
		"/nonexistent/path1/claude",
		"/nonexistent/path2/claude",
	}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != "" {
		t.Errorf("findClaudeInCommonLocations() = %q, want empty string", result)
	}
}

func TestFindClaudeInCommonLocations_SkipsNonExecutable(t *testing.T) {
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
	// Create a temp directory with a non-executable file
	tmpDir := t.TempDir()
	nonExecFile := filepath.Join(tmpDir, "claude")

	// Create file WITHOUT executable permission (0644)
	if err := os.WriteFile(nonExecFile, []byte("#!/bin/sh\necho fake"), 0644); err != nil {
		t.Fatalf("failed to create non-exec file: %v", err)
	}

	// Save original and replace with test location
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{nonExecFile}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != "" {
		t.Errorf("findClaudeInCommonLocations() = %q, want empty (file not executable)", result)
	}
}

func TestFindClaudeInCommonLocations_SkipsDirectories(t *testing.T) {
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
	// Create a directory named "claude" (edge case: something might create a dir with this name)
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, "claude")

	// Create directory with executable permission
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Save original and replace with test location
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{claudeDir}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != "" {
		t.Errorf("findClaudeInCommonLocations() = %q, want empty (directory should be skipped)", result)
	}
}

func TestResolveClaudePath_WithCommonLocations(t *testing.T) {
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
	// Test that ResolveClaudePath falls back to common locations
	// when PATH lookup fails

	// Create a temp directory with a fake claude binary
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/sh\necho fake"), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	// Save original and replace with test locations
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{fakeClaude}
	defer func() { commonClaudeLocations = originalLocations }()

	// Modify PATH to not include claude (use a temp dir)
	originalPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", t.TempDir())
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	result := ResolveClaudePath("claude")
	if result != fakeClaude {
		t.Errorf("ResolveClaudePath(\"claude\") = %q, want %q (from common locations)", result, fakeClaude)
	}
}

func TestResult(t *testing.T) {
	t.Parallel()
	result := &Result{
		Phase:        "implement",
		Iterations:   5,
		Duration:     30 * time.Second,
		Output:       "Implementation complete",
		CommitSHA:    "abc123",
		InputTokens:  1000,
		OutputTokens: 500,
	}

	if result.Phase != "implement" {
		t.Errorf("Phase = %s, want implement", result.Phase)
	}

	if result.Iterations != 5 {
		t.Errorf("Iterations = %d, want 5", result.Iterations)
	}

	if result.Duration != 30*time.Second {
		t.Errorf("Duration = %v, want 30s", result.Duration)
	}

	if result.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %s, want abc123", result.CommitSHA)
	}
}

func TestResultWithError(t *testing.T) {
	t.Parallel()
	testErr := fmt.Errorf("tests failed")
	result := &Result{
		Phase:      "test",
		Iterations: 3,
		Duration:   1 * time.Minute,
		Error:      testErr,
	}

	if result.Error == nil {
		t.Error("Error should not be nil")
	}
	if result.Error.Error() != "tests failed" {
		t.Errorf("Error = %s, want 'tests failed'", result.Error)
	}
}
