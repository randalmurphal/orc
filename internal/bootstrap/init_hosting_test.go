package bootstrap

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/hosting"
)

// =============================================================================
// Tests for SC-11: ANTHROPIC_API_KEY check shown in verification output
// Tests for SC-12: Verification output shows all checks in consistent format
// =============================================================================

// TestPrintHostingVerification_Format tests SC-12: Verification output format.
// Verifies output shows all checks with ✓/✗ prefixes in correct order:
// git repo, remote, provider, token env, token valid, API key.
func TestPrintHostingVerification_Format(t *testing.T) {
	result := &HostingVerificationResult{
		GitRepoFound:     true,
		RemoteFound:      true,
		RemoteURL:        "git@github.com:owner/repo.git",
		Provider:         hosting.ProviderGitHub,
		ProviderName:     "GitHub",
		TokenEnvVar:      "GITHUB_TOKEN",
		TokenExists:      true,
		TokenValid:       true,
		TokenUsername:    "testuser",
		AnthropicKeySet:  true,
	}

	var buf bytes.Buffer
	PrintHostingVerification(&buf, result)

	output := buf.String()

	// Verify order: git repo, remote, provider, token env, token valid, API key
	checks := []string{
		"✓ Git repository detected",
		"✓ Remote 'origin' found",
		"✓ Hosting provider: GitHub",
		"✓ GITHUB_TOKEN found",
		"✓ Token validated",
		"✓ ANTHROPIC_API_KEY",
	}

	lastIdx := -1
	for _, check := range checks {
		idx := strings.Index(output, check)
		if idx == -1 {
			t.Errorf("expected %q in output, got:\n%s", check, output)
			continue
		}
		if idx < lastIdx {
			t.Errorf("checks out of order: %q appears before previous check", check)
		}
		lastIdx = idx
	}
}

// TestPrintHostingVerification_MissingToken tests SC-3, SC-11: Token warnings.
func TestPrintHostingVerification_MissingToken(t *testing.T) {
	result := &HostingVerificationResult{
		GitRepoFound:    true,
		RemoteFound:     true,
		RemoteURL:       "git@github.com:owner/repo.git",
		Provider:        hosting.ProviderGitHub,
		ProviderName:    "GitHub",
		TokenEnvVar:     "GITHUB_TOKEN",
		TokenExists:     false, // Missing
		TokenValid:      false,
		AnthropicKeySet: false, // Also missing
	}

	var buf bytes.Buffer
	PrintHostingVerification(&buf, result)

	output := buf.String()

	// Should show ✗ for missing items
	if !strings.Contains(output, "✗ GITHUB_TOKEN not set") {
		t.Errorf("expected token warning, got:\n%s", output)
	}
	// SC-11: ANTHROPIC_API_KEY warning
	if !strings.Contains(output, "✗ ANTHROPIC_API_KEY not set") {
		t.Errorf("expected ANTHROPIC_API_KEY warning, got:\n%s", output)
	}
	// Should mention it's required for orc run
	if !strings.Contains(output, "required for orc run") {
		t.Errorf("expected 'required for orc run' message, got:\n%s", output)
	}
}

// TestPrintHostingVerification_InvalidToken tests SC-4: Invalid token feedback.
func TestPrintHostingVerification_InvalidToken(t *testing.T) {
	result := &HostingVerificationResult{
		GitRepoFound:        true,
		RemoteFound:         true,
		RemoteURL:           "git@github.com:owner/repo.git",
		Provider:            hosting.ProviderGitHub,
		ProviderName:        "GitHub",
		TokenEnvVar:         "GITHUB_TOKEN",
		TokenExists:         true,
		TokenValid:          false, // Invalid
		TokenValidationError: "401 Unauthorized",
		AnthropicKeySet:     true,
	}

	var buf bytes.Buffer
	PrintHostingVerification(&buf, result)

	output := buf.String()

	if !strings.Contains(output, "✗ Token validation failed") {
		t.Errorf("expected token validation failure message, got:\n%s", output)
	}
	if !strings.Contains(output, "401 Unauthorized") {
		t.Errorf("expected error reason in output, got:\n%s", output)
	}
}

// TestPrintHostingVerification_NoRemote tests failure mode: No git remote.
func TestPrintHostingVerification_NoRemote(t *testing.T) {
	result := &HostingVerificationResult{
		GitRepoFound: true,
		RemoteFound:  false, // No remote
	}

	var buf bytes.Buffer
	PrintHostingVerification(&buf, result)

	output := buf.String()

	if !strings.Contains(output, "No git remote configured") {
		t.Errorf("expected no remote message, got:\n%s", output)
	}
	// Should still show git repo check as passed
	if !strings.Contains(output, "✓ Git repository detected") {
		t.Errorf("expected git repo check to pass, got:\n%s", output)
	}
}

// TestPrintHostingVerification_SelfHosted tests SC-5: Self-hosted feedback.
func TestPrintHostingVerification_SelfHosted(t *testing.T) {
	result := &HostingVerificationResult{
		GitRepoFound:    true,
		RemoteFound:     true,
		RemoteURL:       "git@gitlab.company.com:org/repo.git",
		Provider:        hosting.ProviderGitLab,
		ProviderName:    "GitLab (self-hosted)",
		IsSelfHosted:    true,
		BaseURL:         "https://gitlab.company.com",
		TokenEnvVar:     "GITLAB_TOKEN",
		TokenExists:     true,
		TokenValid:      true,
		TokenUsername:   "gitlabuser",
		AnthropicKeySet: true,
	}

	var buf bytes.Buffer
	PrintHostingVerification(&buf, result)

	output := buf.String()

	// Should show self-hosted indicator
	if !strings.Contains(output, "self-hosted") || !strings.Contains(output, "gitlab.company.com") {
		t.Errorf("expected self-hosted indication, got:\n%s", output)
	}
}

// TestPrintHostingVerification_GitHubAutoMergeWarning tests SC-8: Auto-merge
// warning during init for GitHub.
func TestPrintHostingVerification_GitHubAutoMergeWarning(t *testing.T) {
	result := &HostingVerificationResult{
		GitRepoFound:       true,
		RemoteFound:        true,
		RemoteURL:          "git@github.com:owner/repo.git",
		Provider:           hosting.ProviderGitHub,
		ProviderName:       "GitHub",
		TokenEnvVar:        "GITHUB_TOKEN",
		TokenExists:        true,
		TokenValid:         true,
		TokenUsername:      "testuser",
		AnthropicKeySet:    true,
		AutoMergeEnabled:   true, // Auto-merge is enabled
		AutoMergeSupported: false, // But not supported on GitHub
	}

	var buf bytes.Buffer
	PrintHostingVerification(&buf, result)

	output := buf.String()

	// SC-8: Warning about auto-merge not supported
	if !strings.Contains(output, "Warning") && !strings.Contains(output, "auto_merge") {
		t.Errorf("expected auto_merge warning, got:\n%s", output)
	}
	// SC-10: Should suggest alternative
	if !strings.Contains(output, "merge_on_ci_pass") {
		t.Errorf("expected merge_on_ci_pass suggestion, got:\n%s", output)
	}
}

// =============================================================================
// Tests for instant init hosting behavior
// =============================================================================

// TestInstantInit_HostingDetection tests edge case: --yes flag auto-accepts
// detected provider.
func TestInstantInit_HostingDetection(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepoWithRemote(t, tmpDir, "git@github.com:owner/repo.git")

	opts := Options{
		WorkDir: tmpDir,
		Force:   true, // Allow re-init
	}

	result, err := Run(opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Instant init should detect hosting and store in config
	configPath := filepath.Join(tmpDir, ".orc", "config.yaml")
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	// Verify hosting was detected and stored
	if cfg.Hosting.Provider != "github" {
		t.Errorf("Hosting.Provider = %q, want %q", cfg.Hosting.Provider, "github")
	}

	_ = result // Use result to prevent unused warning
}

// TestInstantInit_NoRemote tests that instant init works without a remote.
func TestInstantInit_NoRemote(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepoNoRemote(t, tmpDir)

	opts := Options{
		WorkDir: tmpDir,
		Force:   true,
	}

	// Should not error - hosting step is skipped gracefully
	result, err := Run(opts)
	if err != nil {
		t.Fatalf("Run() error = %v, should skip hosting gracefully", err)
	}

	_ = result
}

// =============================================================================
// Tests for HostingVerificationResult structure
// =============================================================================

// TestHostingVerificationResult_Fields tests that the struct has all required fields.
func TestHostingVerificationResult_Fields(t *testing.T) {
	result := HostingVerificationResult{
		GitRepoFound:         true,
		RemoteFound:          true,
		RemoteURL:            "git@github.com:owner/repo.git",
		Provider:             hosting.ProviderGitHub,
		ProviderName:         "GitHub",
		IsSelfHosted:         false,
		BaseURL:              "",
		TokenEnvVar:          "GITHUB_TOKEN",
		TokenExists:          true,
		TokenValid:           true,
		TokenUsername:        "testuser",
		TokenValidationError: "",
		AnthropicKeySet:      true,
		AutoMergeEnabled:     false,
		AutoMergeSupported:   true,
	}

	// Just verify the struct compiles and fields are accessible
	if result.Provider != hosting.ProviderGitHub {
		t.Errorf("Provider = %q, want %q", result.Provider, hosting.ProviderGitHub)
	}
}

// =============================================================================
// Helper functions
// =============================================================================

func setupGitRepoWithRemote(t *testing.T, dir, remoteURL string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "remote", "add", "origin", remoteURL)
}

func setupGitRepoNoRemote(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}
