package github

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNewClient_SSHRemote(t *testing.T) {
	// Create a temporary git repo
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Set remote URL (SSH format)
	cmd = exec.Command("git", "remote", "add", "origin", "git@github.com:randalmurphal/orc.git")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git remote add: %v", err)
	}

	client, err := NewClient(tmpDir)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if client.Owner() != "randalmurphal" {
		t.Errorf("expected owner 'randalmurphal', got '%s'", client.Owner())
	}
	if client.Repo() != "orc" {
		t.Errorf("expected repo 'orc', got '%s'", client.Repo())
	}
}

func TestNewClient_HTTPSRemote(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Set remote URL (HTTPS format)
	cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/foo/bar.git")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git remote add: %v", err)
	}

	client, err := NewClient(tmpDir)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if client.Owner() != "foo" {
		t.Errorf("expected owner 'foo', got '%s'", client.Owner())
	}
	if client.Repo() != "bar" {
		t.Errorf("expected repo 'bar', got '%s'", client.Repo())
	}
}

func TestNewClient_HTTPSRemoteNoGitSuffix(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	// Set remote URL (HTTPS format without .git suffix)
	cmd = exec.Command("git", "remote", "add", "origin", "https://github.com/foo/bar")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git remote add: %v", err)
	}

	client, err := NewClient(tmpDir)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if client.Owner() != "foo" {
		t.Errorf("expected owner 'foo', got '%s'", client.Owner())
	}
	if client.Repo() != "bar" {
		t.Errorf("expected repo 'bar', got '%s'", client.Repo())
	}
}

func TestNewClient_NoRemote(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	_, err := NewClient(tmpDir)
	if err == nil {
		t.Error("expected error for repo without origin remote")
	}
}

func TestNewClient_NonGitDir(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := NewClient(tmpDir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestCheckGHAuth(t *testing.T) {
	// Skip this test if gh is not installed
	_, err := exec.LookPath("gh")
	if err != nil {
		t.Skip("gh CLI not installed")
	}

	// This test just verifies the function runs without panic
	// We don't assert on the result since it depends on local auth state
	_ = CheckGHAuth(context.Background())
}

func TestClient_GetPRByURL(t *testing.T) {
	// Create a mock client for URL parsing test
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	cmd = exec.Command("git", "remote", "add", "origin", "git@github.com:owner/repo.git")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git remote add: %v", err)
	}

	client, err := NewClient(tmpDir)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// Test URL parsing (this will fail to actually fetch since gh isn't authed in test)
	// but we can at least verify URL parsing doesn't crash
	_, err = client.GetPRByURL(context.Background(), "https://github.com/owner/repo/pull/123")
	// We expect an error since gh isn't configured in tests
	// but the URL parsing should work
	if err == nil {
		t.Log("GetPRByURL succeeded (gh might be authenticated)")
	}
}

func TestIntegration_RealRepo(t *testing.T) {
	// Integration test that runs against the actual orc repo
	// Only runs if we're in the orc repo and gh is authenticated

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if we're in the orc repo
	wd, err := os.Getwd()
	if err != nil {
		t.Skip("could not get working directory")
	}

	// Go up to find repo root
	repoRoot := wd
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err == nil {
			break
		}
		repoRoot = filepath.Dir(repoRoot)
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		t.Skip("not in a git repository")
	}

	// Check gh auth
	if err := CheckGHAuth(context.Background()); err != nil {
		t.Skip("gh not authenticated")
	}

	client, err := NewClient(repoRoot)
	if err != nil {
		t.Skipf("could not create client: %v", err)
	}

	t.Logf("Testing against repo: %s/%s", client.Owner(), client.Repo())

	// Test listing PRs (read-only, safe)
	pr, err := client.FindPRByBranch(context.Background(), "main")
	if err != nil {
		if errors.Is(err, ErrNoPRFound) {
			t.Log("No PR found for main branch (expected)")
		} else {
			t.Logf("FindPRByBranch error: %v", err)
		}
	} else {
		t.Logf("Found PR #%d: %s", pr.Number, pr.Title)
	}
}
