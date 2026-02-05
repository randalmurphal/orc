package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/wizard"
)

// TestBuildHostingStep_Exists tests that buildHostingStep returns a valid wizard step.
// This is the basic wiring test for SC-1, SC-2.
func TestBuildHostingStep_Exists(t *testing.T) {
	state := &InitWizardState{}
	step := buildHostingStep(state)

	if step == nil {
		t.Fatal("buildHostingStep() returned nil")
	}
}

// TestBuildHostingStep_DetectsGitHub tests SC-1: Init wizard detects GitHub from git remote.
func TestBuildHostingStep_DetectsGitHub(t *testing.T) {
	// Create a temp git repo with a github remote
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir, "git@github.com:owner/repo.git")

	state := &InitWizardState{}
	step := buildHostingStep(state)

	// Simulate running the step with auto-detection
	ws := wizard.State{
		"project_path": tmpDir,
	}

	// The step should detect GitHub and set it in state
	result := step.Execute(ws)
	if result.Error != nil {
		t.Fatalf("step.Execute() error = %v", result.Error)
	}

	// Check that GitHub was detected
	if state.DetectedProvider != hosting.ProviderGitHub {
		t.Errorf("DetectedProvider = %q, want %q", state.DetectedProvider, hosting.ProviderGitHub)
	}
}

// TestBuildHostingStep_DetectsGitLab tests SC-2: Init wizard detects GitLab from git remote.
func TestBuildHostingStep_DetectsGitLab(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir, "git@gitlab.com:owner/repo.git")

	state := &InitWizardState{}
	step := buildHostingStep(state)

	ws := wizard.State{
		"project_path": tmpDir,
	}

	result := step.Execute(ws)
	if result.Error != nil {
		t.Fatalf("step.Execute() error = %v", result.Error)
	}

	if state.DetectedProvider != hosting.ProviderGitLab {
		t.Errorf("DetectedProvider = %q, want %q", state.DetectedProvider, hosting.ProviderGitLab)
	}
}

// TestBuildHostingStep_DetectsSelfHostedGitLab tests SC-5: Self-hosted GitLab URL detection.
func TestBuildHostingStep_DetectsSelfHostedGitLab(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir, "git@gitlab.company.com:org/repo.git")

	state := &InitWizardState{}
	step := buildHostingStep(state)

	ws := wizard.State{
		"project_path": tmpDir,
	}

	result := step.Execute(ws)
	if result.Error != nil {
		t.Fatalf("step.Execute() error = %v", result.Error)
	}

	if state.DetectedProvider != hosting.ProviderGitLab {
		t.Errorf("DetectedProvider = %q, want %q", state.DetectedProvider, hosting.ProviderGitLab)
	}
	if !state.IsSelfHosted {
		t.Error("IsSelfHosted should be true for gitlab.company.com")
	}
	if state.DetectedBaseURL != "https://gitlab.company.com" {
		t.Errorf("DetectedBaseURL = %q, want %q", state.DetectedBaseURL, "https://gitlab.company.com")
	}
}

// TestBuildHostingStep_NoRemote tests failure mode: No git remote configured.
func TestBuildHostingStep_NoRemote(t *testing.T) {
	tmpDir := t.TempDir()
	// Create git repo without remote
	setupGitRepoNoRemote(t, tmpDir)

	state := &InitWizardState{}
	step := buildHostingStep(state)

	ws := wizard.State{
		"project_path": tmpDir,
	}

	result := step.Execute(ws)

	// Should skip gracefully, not error
	if result.Error != nil {
		t.Fatalf("step.Execute() should skip gracefully, got error = %v", result.Error)
	}

	// Should indicate hosting step was skipped
	if state.HostingSkipped != true {
		t.Error("HostingSkipped should be true when no remote exists")
	}
}

// TestBuildHostingStep_UnknownProvider tests failure mode: Remote URL unrecognized.
func TestBuildHostingStep_UnknownProvider(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir, "git@bitbucket.org:owner/repo.git")

	state := &InitWizardState{}
	step := buildHostingStep(state)

	ws := wizard.State{
		"project_path": tmpDir,
	}

	result := step.Execute(ws)
	if result.Error != nil {
		t.Fatalf("step.Execute() error = %v", result.Error)
	}

	// Should prompt for manual selection when provider is unknown
	if state.DetectedProvider != hosting.ProviderUnknown {
		t.Errorf("DetectedProvider = %q, want %q", state.DetectedProvider, hosting.ProviderUnknown)
	}
	if !state.RequiresManualSelection {
		t.Error("RequiresManualSelection should be true for unknown provider")
	}
}

// TestHostingStepTokenCheck tests SC-3: Token existence check.
func TestHostingStepTokenCheck(t *testing.T) {
	// Save and restore env
	oldToken := os.Getenv("GITHUB_TOKEN")
	defer func() {
		if oldToken != "" {
			os.Setenv("GITHUB_TOKEN", oldToken)
		} else {
			os.Unsetenv("GITHUB_TOKEN")
		}
	}()

	// Test with token NOT set
	os.Unsetenv("GITHUB_TOKEN")

	state := &InitWizardState{
		DetectedProvider: hosting.ProviderGitHub,
	}

	// Check token existence
	result := checkTokenExists(state)

	if result.TokenExists {
		t.Error("TokenExists should be false when GITHUB_TOKEN not set")
	}
	if result.TokenEnvVar != "GITHUB_TOKEN" {
		t.Errorf("TokenEnvVar = %q, want %q", result.TokenEnvVar, "GITHUB_TOKEN")
	}

	// Test with token set
	os.Setenv("GITHUB_TOKEN", "test-token")

	result2 := checkTokenExists(state)

	if !result2.TokenExists {
		t.Error("TokenExists should be true when GITHUB_TOKEN is set")
	}
}

// TestHostingStepMissingToken tests failure mode: Token env var not set.
func TestHostingStepMissingToken(t *testing.T) {
	// Ensure token is not set
	os.Unsetenv("GITHUB_TOKEN")

	state := &InitWizardState{
		DetectedProvider: hosting.ProviderGitHub,
	}

	result := checkTokenExists(state)

	// Should continue with warning, not error
	if result.TokenExists {
		t.Error("TokenExists should be false when token not set")
	}
	// Warning message should be set
	if result.Warning == "" {
		t.Error("Warning should be set when token is missing")
	}
}

// TestHostingStepMultipleRemotes tests edge case: Repo with multiple remotes.
func TestHostingStepMultipleRemotes(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepoMultipleRemotes(t, tmpDir)

	state := &InitWizardState{}
	step := buildHostingStep(state)

	ws := wizard.State{
		"project_path": tmpDir,
	}

	result := step.Execute(ws)
	if result.Error != nil {
		t.Fatalf("step.Execute() error = %v", result.Error)
	}

	// Should use 'origin' remote only
	if state.DetectedProvider != hosting.ProviderGitHub {
		t.Errorf("DetectedProvider = %q, want %q (from origin)", state.DetectedProvider, hosting.ProviderGitHub)
	}
}

// TestInitWizardState_HostingFields tests that InitWizardState has hosting fields.
func TestInitWizardState_HostingFields(t *testing.T) {
	state := &InitWizardState{
		DetectedProvider:        hosting.ProviderGitHub,
		ConfirmedProvider:       hosting.ProviderGitHub,
		IsSelfHosted:            false,
		DetectedBaseURL:         "",
		HostingSkipped:          false,
		RequiresManualSelection: false,
	}

	// Verify all fields can be set
	if state.DetectedProvider != hosting.ProviderGitHub {
		t.Errorf("DetectedProvider = %q, want %q", state.DetectedProvider, hosting.ProviderGitHub)
	}
}

// TestExtractWizardResults_Hosting tests that extractWizardResults extracts hosting state.
func TestExtractWizardResults_Hosting(t *testing.T) {
	ws := wizard.State{
		"hosting_provider":   "github",
		"hosting_base_url":   "https://github.company.com",
		"hosting_confirmed":  true,
	}

	state := &InitWizardState{}
	extractWizardResults(ws, state)

	if state.ConfirmedProvider != hosting.ProviderGitHub {
		t.Errorf("ConfirmedProvider = %q, want %q", state.ConfirmedProvider, hosting.ProviderGitHub)
	}
}

// Helper functions

func setupGitRepo(t *testing.T, dir, remoteURL string) {
	t.Helper()

	// git init
	runGit(t, dir, "init")
	// git remote add origin
	runGit(t, dir, "remote", "add", "origin", remoteURL)
}

func setupGitRepoNoRemote(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
}

func setupGitRepoMultipleRemotes(t *testing.T, dir string) {
	t.Helper()

	runGit(t, dir, "init")
	// 'origin' points to GitHub
	runGit(t, dir, "remote", "add", "origin", "git@github.com:owner/repo.git")
	// 'upstream' points to GitLab
	runGit(t, dir, "remote", "add", "upstream", "git@gitlab.com:other/repo.git")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	_ = filepath.Join(dir) // silence unused import

	// Actually run git command
	gitCmd := exec.Command("git", args...)
	gitCmd.Dir = dir
	if err := gitCmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}
