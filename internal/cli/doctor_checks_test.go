package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

func TestResolveHostingTokenEnvVar(t *testing.T) {
	cfg := config.Default()

	if got := resolveHostingTokenEnvVar(cfg, "github"); got != "GITHUB_TOKEN" {
		t.Fatalf("resolveHostingTokenEnvVar(github) = %q, want %q", got, "GITHUB_TOKEN")
	}

	if got := resolveHostingTokenEnvVar(cfg, "gitlab"); got != "GITLAB_TOKEN" {
		t.Fatalf("resolveHostingTokenEnvVar(gitlab) = %q, want %q", got, "GITLAB_TOKEN")
	}

	cfg.Hosting.TokenEnvVar = "CUSTOM_TOKEN"
	if got := resolveHostingTokenEnvVar(cfg, "github"); got != "CUSTOM_TOKEN" {
		t.Fatalf("custom token env var = %q, want %q", got, "CUSTOM_TOKEN")
	}
}

func TestResolveHostingProviderForDoctor_ExplicitProvider(t *testing.T) {
	cfg := config.Default()
	cfg.Hosting.Provider = "gitlab"

	got, err := resolveHostingProviderForDoctor(t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("resolveHostingProviderForDoctor explicit provider: %v", err)
	}
	if got != "gitlab" {
		t.Fatalf("resolveHostingProviderForDoctor explicit provider = %q, want %q", got, "gitlab")
	}
}

func TestResolveHostingProviderForDoctor_AutoDetectGitHub(t *testing.T) {
	workDir := t.TempDir()
	initGitRepoForDoctorTest(t, workDir, "https://github.com/example/orc.git")

	cfg := config.Default()
	cfg.Hosting.Provider = "auto"

	got, err := resolveHostingProviderForDoctor(workDir, cfg)
	if err != nil {
		t.Fatalf("resolveHostingProviderForDoctor auto-detect: %v", err)
	}
	if got != "github" {
		t.Fatalf("resolveHostingProviderForDoctor auto-detect = %q, want %q", got, "github")
	}
}

func initGitRepoForDoctorTest(t *testing.T, workDir string, remoteURL string) {
	t.Helper()

	runDoctorGitCommand(t, workDir, "init")
	runDoctorGitCommand(t, workDir, "remote", "add", "origin", remoteURL)

	gitDir := filepath.Join(workDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Fatalf("expected git repo at %s: %v", gitDir, err)
	}
}

func runDoctorGitCommand(t *testing.T, workDir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
