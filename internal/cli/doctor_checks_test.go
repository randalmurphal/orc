package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/workflow"
)

func TestCompletionHostingChecksWithNamedAccount(t *testing.T) {
	projectDir := setupDoctorTestProject(t, "https://github.com/example/orc.git")

	registry := &hosting.AccountRegistry{
		Accounts: map[string]hosting.Account{
			"nulliti-ghe": {
				Provider:    "github",
				BaseURL:     "https://nulliti.ghe.example.com",
				TokenEnvVar: "ORC_NULLITI_GHE_TOKEN",
			},
		},
	}
	accountsPath, err := hosting.AccountsPath()
	if err != nil {
		t.Fatalf("AccountsPath: %v", err)
	}
	if err := registry.Save(accountsPath); err != nil {
		t.Fatalf("Save accounts: %v", err)
	}

	gdb, err := db.OpenGlobal()
	if err != nil {
		t.Fatalf("OpenGlobal: %v", err)
	}
	defer func() { _ = gdb.Close() }()
	if _, err := workflow.SeedBuiltins(gdb); err != nil {
		t.Fatalf("SeedBuiltins: %v", err)
	}

	cfg := config.Default()
	cfg.Completion.Action = "pr"
	cfg.Hosting.Account = "nulliti-ghe"

	checks, err := completionHostingChecks(cfg, "crossmodel-standard")
	if err != nil {
		t.Fatalf("completionHostingChecks: %v", err)
	}
	if len(checks) != 2 {
		t.Fatalf("completionHostingChecks len = %d, want 2", len(checks))
	}
	if !checks[0].OK {
		t.Fatalf("hosting account check = %+v, want OK", checks[0])
	}
	if !strings.Contains(checks[0].Detail, "nulliti-ghe") {
		t.Fatalf("hosting account detail = %q, want account name", checks[0].Detail)
	}
	if checks[1].OK {
		t.Fatalf("auth check = %+v, want FAIL when token is missing", checks[1])
	}
	if !strings.Contains(checks[1].Detail, "ORC_NULLITI_GHE_TOKEN") {
		t.Fatalf("auth detail = %q, want token env var", checks[1].Detail)
	}

	t.Setenv("ORC_NULLITI_GHE_TOKEN", "secret")
	checks, err = completionHostingChecks(cfg, "crossmodel-standard")
	if err != nil {
		t.Fatalf("completionHostingChecks after token: %v", err)
	}
	if !checks[1].OK {
		t.Fatalf("auth check after token = %+v, want OK", checks[1])
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".git")); err != nil {
		t.Fatalf("expected git repo at %s: %v", projectDir, err)
	}
}

func TestCompletionHostingChecks_UsesProjectWorkflowCompletionOverride(t *testing.T) {
	projectDir := setupDoctorTestProject(t, "https://github.com/example/orc.git")

	gdb, err := db.OpenGlobal()
	if err != nil {
		t.Fatalf("OpenGlobal: %v", err)
	}
	defer func() { _ = gdb.Close() }()
	if _, err := workflow.SeedBuiltins(gdb); err != nil {
		t.Fatalf("SeedBuiltins: %v", err)
	}

	workflowDir := filepath.Join(projectDir, ".orc", "workflows")
	if err := os.MkdirAll(workflowDir, 0755); err != nil {
		t.Fatalf("create workflow dir: %v", err)
	}
	workflowYAML := `id: plan-eval-local
name: Plan Eval Local
description: Project workflow override for doctor regression coverage.
completion_action: none
phases:
  - template: plan
    sequence: 0
`
	if err := os.WriteFile(filepath.Join(workflowDir, "plan-eval-local.yaml"), []byte(workflowYAML), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	cfg := config.Default()
	cfg.Completion.Action = "pr"

	checks, err := completionHostingChecks(cfg, "plan-eval-local")
	if err != nil {
		t.Fatalf("completionHostingChecks: %v", err)
	}
	if len(checks) != 0 {
		t.Fatalf("completionHostingChecks len = %d, want 0 for completion_action=none", len(checks))
	}
}

func setupDoctorTestProject(t *testing.T, remoteURL string) string {
	t.Helper()

	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(filepath.Join(homeDir, ".orc"), 0755); err != nil {
		t.Fatalf("create home dir: %v", err)
	}
	t.Setenv("HOME", homeDir)

	projectDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(filepath.Join(projectDir, ".orc"), 0755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".orc", "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	runDoctorGitCommand(t, projectDir, "init")
	runDoctorGitCommand(t, projectDir, "remote", "add", "origin", remoteURL)
	return projectDir
}

func runDoctorGitCommand(t *testing.T, workDir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = workDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}
