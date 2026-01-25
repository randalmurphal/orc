package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
// This helper is used by all test files in this package.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	return tmpDir
}

func TestNew(t *testing.T) {
	tmpDir := setupTestRepo(t)
	cfg := DefaultConfig()

	g, err := New(tmpDir, cfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if g.branchPrefix != "orc/" {
		t.Errorf("branchPrefix = %s, want orc/", g.branchPrefix)
	}

	if g.commitPrefix != "[orc]" {
		t.Errorf("commitPrefix = %s, want [orc]", g.commitPrefix)
	}
}

func TestNewInvalidPath(t *testing.T) {
	_, err := New("/nonexistent/path", DefaultConfig())
	if err == nil {
		t.Error("New() should fail for non-git directory")
	}
}
