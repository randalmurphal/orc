package diff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
// Returns the path to the repo and a cleanup function.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "diff-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() { _ = os.RemoveAll(dir) }

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("failed to config git email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("failed to config git name: %v", err)
	}

	// Create initial file and commit
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("initial\n"), 0644); err != nil {
		cleanup()
		t.Fatalf("failed to write file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("failed to add file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		cleanup()
		t.Fatalf("failed to commit: %v", err)
	}

	return dir, cleanup
}

func TestResolveRef(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	svc := NewService(dir, nil)
	ctx := context.Background()

	// Test: existing local branch (main/master) should resolve to itself
	t.Run("existing branch resolves to itself", func(t *testing.T) {
		// Get the current branch name
		cmd := exec.Command("git", "branch", "--show-current")
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Skipf("could not get current branch: %v", err)
		}
		branch := string(out[:len(out)-1]) // trim newline

		resolved := svc.ResolveRef(ctx, branch)
		if resolved != branch {
			t.Errorf("expected %q, got %q", branch, resolved)
		}
	})

	// Test: HEAD should resolve to itself
	t.Run("HEAD resolves to itself", func(t *testing.T) {
		resolved := svc.ResolveRef(ctx, "HEAD")
		if resolved != "HEAD" {
			t.Errorf("expected HEAD, got %q", resolved)
		}
	})

	// Test: non-existent branch returns original
	t.Run("non-existent branch returns original", func(t *testing.T) {
		nonExistent := "non-existent-branch-xyz123"
		resolved := svc.ResolveRef(ctx, nonExistent)
		if resolved != nonExistent {
			t.Errorf("expected %q, got %q", nonExistent, resolved)
		}
	})

	// Test: simulated remote branch resolution
	t.Run("remote branch fallback", func(t *testing.T) {
		// Create a branch
		cmd := exec.Command("git", "checkout", "-b", "feature-test")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to create branch: %v", err)
		}

		// Make a commit on the branch
		if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified\n"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		_ = cmd.Run()
		cmd = exec.Command("git", "commit", "-m", "feature commit")
		cmd.Dir = dir
		_ = cmd.Run()

		// Go back to main
		cmd = exec.Command("git", "checkout", "-")
		cmd.Dir = dir
		_ = cmd.Run()

		// Set up a fake remote by creating a ref manually
		cmd = exec.Command("git", "update-ref", "refs/remotes/origin/feature-test", "feature-test")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Skipf("could not create remote ref: %v", err)
		}

		// Delete the local branch
		cmd = exec.Command("git", "branch", "-D", "feature-test")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to delete branch: %v", err)
		}

		// Now ResolveRef should find origin/feature-test
		resolved := svc.ResolveRef(ctx, "feature-test")
		if resolved != "origin/feature-test" {
			t.Errorf("expected origin/feature-test, got %q", resolved)
		}
	})
}

func TestGetStats(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a feature branch with changes
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Add a new file
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new content\nline 2\nline 3\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Modify existing file
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "feature changes")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Get the main branch name
	cmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD~1")
	cmd.Dir = dir
	// This might not work, so just use the commit hash approach
	cmd = exec.Command("git", "rev-parse", "HEAD~1")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get parent commit: %v", err)
	}
	base := string(out[:len(out)-1])

	svc := NewService(dir, nil)
	ctx := context.Background()

	stats, err := svc.GetStats(ctx, base, "HEAD")
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.FilesChanged != 2 {
		t.Errorf("expected 2 files changed, got %d", stats.FilesChanged)
	}

	if stats.Additions < 3 {
		t.Errorf("expected at least 3 additions, got %d", stats.Additions)
	}
}

func TestGetFileList(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a feature branch with changes
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Add files
	if err := os.WriteFile(filepath.Join(dir, "added.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "feature changes")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	cmd = exec.Command("git", "rev-parse", "HEAD~1")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get parent: %v", err)
	}
	base := string(out[:len(out)-1])

	svc := NewService(dir, nil)
	ctx := context.Background()

	files, err := svc.GetFileList(ctx, base, "HEAD")
	if err != nil {
		t.Fatalf("GetFileList failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}

	// Check files are not nil (important for JSON serialization)
	if files == nil {
		t.Error("files should not be nil")
	}
}

func TestGetFileDiff(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a feature branch with changes
	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Modify file with multiple lines
	content := "line1\nline2 modified\nline3\nline4 new\n"
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "modify file")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	cmd = exec.Command("git", "rev-parse", "HEAD~1")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get parent: %v", err)
	}
	base := string(out[:len(out)-1])

	svc := NewService(dir, nil)
	ctx := context.Background()

	diff, err := svc.GetFileDiff(ctx, base, "HEAD", "file.txt")
	if err != nil {
		t.Fatalf("GetFileDiff failed: %v", err)
	}

	if diff.Path != "file.txt" {
		t.Errorf("expected path file.txt, got %s", diff.Path)
	}

	if len(diff.Hunks) == 0 {
		t.Error("expected at least one hunk")
	}

	if diff.Additions == 0 {
		t.Error("expected some additions")
	}
}

func TestDetectSyntax(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"file.go", "go"},
		{"component.tsx", "tsx"},
		{"script.py", "python"},
		{"style.css", "css"},
		{"config.yaml", "yaml"},
		{"data.json", "json"},
		{"README.md", "markdown"},
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{".gitignore", "gitignore"},
		{"unknown.xyz", "text"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			result := detectSyntax(tc.path)
			if result != tc.expected {
				t.Errorf("detectSyntax(%q) = %q, expected %q", tc.path, result, tc.expected)
			}
		})
	}
}

func TestParseStats(t *testing.T) {
	tests := []struct {
		input    string
		expected DiffStats
	}{
		{
			" 5 files changed, 120 insertions(+), 45 deletions(-)",
			DiffStats{FilesChanged: 5, Additions: 120, Deletions: 45},
		},
		{
			" 1 file changed, 1 insertion(+)",
			DiffStats{FilesChanged: 1, Additions: 1, Deletions: 0},
		},
		{
			" 2 files changed, 10 deletions(-)",
			DiffStats{FilesChanged: 2, Additions: 0, Deletions: 10},
		},
		{
			"",
			DiffStats{FilesChanged: 0, Additions: 0, Deletions: 0},
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := parseStats(tc.input)
			if err != nil {
				t.Fatalf("parseStats failed: %v", err)
			}
			if *result != tc.expected {
				t.Errorf("parseStats(%q) = %+v, expected %+v", tc.input, *result, tc.expected)
			}
		})
	}
}

func TestShouldIncludeWorkingTree(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	svc := NewService(dir, nil)
	ctx := context.Background()

	// Get the main branch name
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("could not get current branch: %v", err)
	}
	mainBranch := string(out[:len(out)-1])

	t.Run("no uncommitted changes returns false", func(t *testing.T) {
		shouldInclude, effectiveHead := svc.ShouldIncludeWorkingTree(ctx, mainBranch, "HEAD")
		if shouldInclude {
			t.Error("expected false when no uncommitted changes")
		}
		if effectiveHead != "HEAD" {
			t.Errorf("expected effectiveHead to be HEAD, got %q", effectiveHead)
		}
	})

	t.Run("uncommitted changes returns true", func(t *testing.T) {
		// Modify file without committing
		if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("uncommitted change\n"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		shouldInclude, effectiveHead := svc.ShouldIncludeWorkingTree(ctx, mainBranch, "HEAD")
		if !shouldInclude {
			t.Error("expected true when there are uncommitted changes")
		}
		if effectiveHead != "" {
			t.Errorf("expected effectiveHead to be empty, got %q", effectiveHead)
		}

		// Reset file for other tests
		cmd := exec.Command("git", "checkout", "--", "file.txt")
		cmd.Dir = dir
		_ = cmd.Run()
	})

	t.Run("diverged branch returns false", func(t *testing.T) {
		// Create a new commit on a feature branch
		cmd := exec.Command("git", "checkout", "-b", "feature-diverged")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to create branch: %v", err)
		}

		if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("feature commit\n"), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}

		cmd = exec.Command("git", "add", ".")
		cmd.Dir = dir
		_ = cmd.Run()

		cmd = exec.Command("git", "commit", "-m", "feature commit")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		shouldInclude, effectiveHead := svc.ShouldIncludeWorkingTree(ctx, mainBranch, "feature-diverged")
		if shouldInclude {
			t.Error("expected false when branch has diverged from base")
		}
		if effectiveHead != "feature-diverged" {
			t.Errorf("expected effectiveHead to be feature-diverged, got %q", effectiveHead)
		}

		// Go back to main
		cmd = exec.Command("git", "checkout", mainBranch)
		cmd.Dir = dir
		_ = cmd.Run()
	})
}

func TestWorkingTreeDiff(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	svc := NewService(dir, nil)
	ctx := context.Background()

	// Get the main branch name
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("could not get current branch: %v", err)
	}
	mainBranch := string(out[:len(out)-1])

	// Create uncommitted changes
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified content\nwith multiple lines\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new file content\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	t.Run("GetStats with working tree", func(t *testing.T) {
		stats, err := svc.GetStats(ctx, mainBranch, "")
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}
		// file.txt is modified, new.txt is untracked (won't show in git diff)
		if stats.FilesChanged < 1 {
			t.Errorf("expected at least 1 file changed, got %d", stats.FilesChanged)
		}
	})

	t.Run("GetFileList with working tree", func(t *testing.T) {
		files, err := svc.GetFileList(ctx, mainBranch, "")
		if err != nil {
			t.Fatalf("GetFileList failed: %v", err)
		}
		if len(files) < 1 {
			t.Errorf("expected at least 1 file, got %d", len(files))
		}
		// Check file.txt is in the list
		found := false
		for _, f := range files {
			if f.Path == "file.txt" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected file.txt to be in the list")
		}
	})

	t.Run("GetFileDiff with working tree", func(t *testing.T) {
		diff, err := svc.GetFileDiff(ctx, mainBranch, "", "file.txt")
		if err != nil {
			t.Fatalf("GetFileDiff failed: %v", err)
		}
		if diff.Path != "file.txt" {
			t.Errorf("expected path file.txt, got %s", diff.Path)
		}
		if len(diff.Hunks) == 0 {
			t.Error("expected at least one hunk")
		}
	})
}
