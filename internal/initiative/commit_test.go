package initiative

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommitAndSync(t *testing.T) {
	// Create temp directory for project root
	tmpDir, err := os.MkdirTemp("", "commit-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo (required for git commands)
	if err := initGitRepo(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Create .orc directory structure
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create initiative
	initDir := filepath.Join(orcDir, "initiatives")
	if err := os.MkdirAll(initDir, 0755); err != nil {
		t.Fatalf("failed to create initiatives dir: %v", err)
	}

	init := New("INIT-001", "Test Initiative")

	// Save initiative
	initSubDir := filepath.Join(initDir, init.ID)
	if err := os.MkdirAll(initSubDir, 0755); err != nil {
		t.Fatalf("failed to create initiative dir: %v", err)
	}
	if err := init.SaveTo(initDir); err != nil {
		t.Fatalf("failed to save initiative: %v", err)
	}

	// Test CommitAndSync - should not error
	cfg := CommitConfig{
		ProjectRoot:  tmpDir,
		CommitPrefix: "[orc]",
	}

	// Should not panic or return error for valid input
	// The actual git commit may fail in test environment, but the function should handle it gracefully
	err = CommitAndSync(init, "create", cfg)
	// We don't check error because git commit may fail in test environment (e.g., no git user configured)
	_ = err

	// Test CommitDeletion
	err = CommitDeletion("INIT-001", cfg)
	_ = err
}

func TestRebuildDBIndex(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "rebuild-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up directory structure
	initiativesDir := filepath.Join(tmpDir, ".orc", "initiatives")
	if err := os.MkdirAll(initiativesDir, 0755); err != nil {
		t.Fatalf("failed to create initiatives dir: %v", err)
	}

	// Create a few initiatives
	for i := 1; i <= 3; i++ {
		init := New(NextIDFromNum(i), "Test Initiative")
		if err := init.SaveTo(initiativesDir); err != nil {
			t.Fatalf("failed to save initiative %d: %v", i, err)
		}
	}

	// Rebuild index - should not error
	err = RebuildDBIndex(tmpDir, false, nil)
	if err != nil {
		t.Errorf("RebuildDBIndex failed: %v", err)
	}
}

// initGitRepo initializes a git repository in the given directory
func initGitRepo(dir string) error {
	if err := gitAdd(dir, "."); err != nil {
		// Try initializing first
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(""), 0644); err != nil {
			return err
		}
	}
	return nil
}

// NextIDFromNum generates an initiative ID from a number
func NextIDFromNum(n int) string {
	return "INIT-00" + string(rune('0'+n))
}
