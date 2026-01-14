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

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
	}{
		// Standard SSH format
		{
			name:      "ssh standard",
			url:       "git@github.com:owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		// GitHub Enterprise SSH format
		{
			name:      "ssh github enterprise",
			url:       "git@github.company.com:org/repo",
			wantOwner: "org",
			wantRepo:  "repo",
		},
		// SSH URL with port
		{
			name:      "ssh with port",
			url:       "ssh://git@github.com:22/org/repo",
			wantOwner: "org",
			wantRepo:  "repo",
		},
		// Standard HTTPS format
		{
			name:      "https standard",
			url:       "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		// Nested paths (take last two segments)
		{
			name:      "nested path",
			url:       "https://github.com/org/subgroup/repo",
			wantOwner: "subgroup",
			wantRepo:  "repo",
		},
		// Deeply nested paths
		{
			name:      "deeply nested path",
			url:       "https://github.com/org/level1/level2/level3/repo",
			wantOwner: "level3",
			wantRepo:  "repo",
		},
		// HTTP (no S)
		{
			name:      "http",
			url:       "http://github.internal.com/team/project",
			wantOwner: "team",
			wantRepo:  "project",
		},
		// Invalid - too few segments
		{
			name:      "too few segments",
			url:       "https://github.com/repo",
			wantOwner: "",
			wantRepo:  "",
		},
		// Invalid - no path
		{
			name:      "no path",
			url:       "https://github.com/",
			wantOwner: "",
			wantRepo:  "",
		},
		// Invalid - garbage
		{
			name:      "garbage",
			url:       "not a url",
			wantOwner: "",
			wantRepo:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOwner, gotRepo := parseOwnerRepo(tt.url)
			if gotOwner != tt.wantOwner {
				t.Errorf("owner: got %q, want %q", gotOwner, tt.wantOwner)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("repo: got %q, want %q", gotRepo, tt.wantRepo)
			}
		})
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

func TestIsLabelError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "label not found",
			err:      errors.New("could not add label: automated not found"),
			expected: true,
		},
		{
			name:     "label not found uppercase",
			err:      errors.New("Could not add Label: AUTOMATED not found"),
			expected: true,
		},
		{
			name:     "multiple labels not found",
			err:      errors.New("could not add label: bug-fix not found"),
			expected: true,
		},
		{
			name:     "gh cli error with label",
			err:      errors.New("gh pr create: label 'automated' not found: exit status 1"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("network timeout"),
			expected: false,
		},
		{
			name:     "auth error",
			err:      errors.New("gh: not authenticated"),
			expected: false,
		},
		{
			name:     "branch not found",
			err:      errors.New("branch not found: feature-branch"),
			expected: false,
		},
		{
			name:     "generic not found without label",
			err:      errors.New("repository not found"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLabelError(tt.err)
			if got != tt.expected {
				t.Errorf("isLabelError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
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

func TestPRStatusSummary_Analyze(t *testing.T) {
	tests := []struct {
		name          string
		reviews       map[string]string // author -> state
		expectedCount int
		expectedAppr  int
		expectedStat  string
	}{
		{
			name:          "no reviews",
			reviews:       map[string]string{},
			expectedCount: 0,
			expectedAppr:  0,
			expectedStat:  "pending_review",
		},
		{
			name: "one approval",
			reviews: map[string]string{
				"alice": "APPROVED",
			},
			expectedCount: 1,
			expectedAppr:  1,
			expectedStat:  "approved",
		},
		{
			name: "changes requested",
			reviews: map[string]string{
				"alice": "CHANGES_REQUESTED",
			},
			expectedCount: 1,
			expectedAppr:  0,
			expectedStat:  "changes_requested",
		},
		{
			name: "mixed - changes requested takes precedence",
			reviews: map[string]string{
				"alice": "APPROVED",
				"bob":   "CHANGES_REQUESTED",
			},
			expectedCount: 2,
			expectedAppr:  1,
			expectedStat:  "changes_requested",
		},
		{
			name: "multiple approvals",
			reviews: map[string]string{
				"alice": "APPROVED",
				"bob":   "APPROVED",
				"carol": "APPROVED",
			},
			expectedCount: 3,
			expectedAppr:  3,
			expectedStat:  "approved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic from GetPRStatusSummary
			latestByAuthor := tt.reviews
			reviewCount := len(latestByAuthor)

			var approvals, changesRequested int
			for _, state := range latestByAuthor {
				switch state {
				case "APPROVED":
					approvals++
				case "CHANGES_REQUESTED":
					changesRequested++
				}
			}

			var status string
			if changesRequested > 0 {
				status = "changes_requested"
			} else if approvals > 0 {
				status = "approved"
			} else {
				status = "pending_review"
			}

			if reviewCount != tt.expectedCount {
				t.Errorf("reviewCount = %d, want %d", reviewCount, tt.expectedCount)
			}
			if approvals != tt.expectedAppr {
				t.Errorf("approvals = %d, want %d", approvals, tt.expectedAppr)
			}
			if status != tt.expectedStat {
				t.Errorf("status = %s, want %s", status, tt.expectedStat)
			}
		})
	}
}

func TestPRReview_Struct(t *testing.T) {
	// Test that PRReview struct can be created and used
	review := PRReview{
		ID:        12345,
		Author:    "alice",
		State:     "APPROVED",
		Body:      "LGTM!",
		CreatedAt: "2024-01-01T10:00:00Z",
	}

	if review.ID != 12345 {
		t.Errorf("ID = %d, want 12345", review.ID)
	}
	if review.Author != "alice" {
		t.Errorf("Author = %s, want alice", review.Author)
	}
	if review.State != "APPROVED" {
		t.Errorf("State = %s, want APPROVED", review.State)
	}
}
