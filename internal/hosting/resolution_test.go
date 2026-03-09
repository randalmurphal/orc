package hosting

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

func TestLoadAccountsFromMissingFile(t *testing.T) {
	registry, err := LoadAccountsFrom(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("LoadAccountsFrom missing file: %v", err)
	}
	if len(registry.Accounts) != 0 {
		t.Fatalf("LoadAccountsFrom missing file returned %d accounts, want 0", len(registry.Accounts))
	}
}

func TestResolveConfigWithNamedAccount(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	registry := &AccountRegistry{
		Accounts: map[string]Account{
			"nulliti-ghe": {
				Provider:    "github",
				BaseURL:     "https://nulliti.ghe.example.com",
				TokenEnvVar: "ORC_NULLITI_GHE_TOKEN",
			},
		},
	}
	accountsPath, err := AccountsPath()
	if err != nil {
		t.Fatalf("AccountsPath: %v", err)
	}
	if err := registry.Save(accountsPath); err != nil {
		t.Fatalf("Save accounts: %v", err)
	}

	cfg := config.Default()
	cfg.Hosting.Account = "nulliti-ghe"

	resolved, err := ResolveConfig(t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}
	if resolved.AccountName != "nulliti-ghe" {
		t.Fatalf("AccountName = %q, want %q", resolved.AccountName, "nulliti-ghe")
	}
	if resolved.ProviderType != ProviderGitHub {
		t.Fatalf("ProviderType = %q, want %q", resolved.ProviderType, ProviderGitHub)
	}
	if resolved.BaseURL != "https://nulliti.ghe.example.com" {
		t.Fatalf("BaseURL = %q, want %q", resolved.BaseURL, "https://nulliti.ghe.example.com")
	}
	if resolved.TokenEnvVar != "ORC_NULLITI_GHE_TOKEN" {
		t.Fatalf("TokenEnvVar = %q, want %q", resolved.TokenEnvVar, "ORC_NULLITI_GHE_TOKEN")
	}
}

func TestResolveConfigDefaultsTokenEnvVar(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	registry := &AccountRegistry{
		Accounts: map[string]Account{
			"personal": {
				Provider: "github",
			},
		},
	}
	accountsPath, err := AccountsPath()
	if err != nil {
		t.Fatalf("AccountsPath: %v", err)
	}
	if err := registry.Save(accountsPath); err != nil {
		t.Fatalf("Save accounts: %v", err)
	}

	cfg := config.Default()
	cfg.Hosting.Account = "personal"

	resolved, err := ResolveConfig(t.TempDir(), cfg)
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}
	if resolved.TokenEnvVar != "ORC_GITHUB_TOKEN" {
		t.Fatalf("TokenEnvVar = %q, want %q", resolved.TokenEnvVar, "ORC_GITHUB_TOKEN")
	}
}

func TestResolveConfigDetectsAccountConflict(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	registry := &AccountRegistry{
		Accounts: map[string]Account{
			"personal": {
				Provider:    "github",
				TokenEnvVar: "ORC_GITHUB_TOKEN",
			},
		},
	}
	accountsPath, err := AccountsPath()
	if err != nil {
		t.Fatalf("AccountsPath: %v", err)
	}
	if err := registry.Save(accountsPath); err != nil {
		t.Fatalf("Save accounts: %v", err)
	}

	cfg := config.Default()
	cfg.Hosting.Account = "personal"
	cfg.Hosting.TokenEnvVar = "ORC_OTHER_TOKEN"

	_, err = ResolveConfig(t.TempDir(), cfg)
	if err == nil {
		t.Fatal("ResolveConfig should fail on token env var conflict")
	}
	if !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("ResolveConfig conflict error = %v, want conflict detail", err)
	}
}

func TestResolveTokenFromEnv(t *testing.T) {
	t.Setenv("ORC_GITHUB_TOKEN", "")
	t.Setenv("ORC_GITLAB_TOKEN", "")
	t.Setenv("CUSTOM_TOKEN", "")

	t.Run("github default", func(t *testing.T) {
		t.Setenv("ORC_GITHUB_TOKEN", "ghp-test")
		token, err := ResolveTokenFromEnv(Config{}, ProviderGitHub)
		if err != nil {
			t.Fatalf("ResolveTokenFromEnv github: %v", err)
		}
		if token != "ghp-test" {
			t.Fatalf("ResolveTokenFromEnv github = %q, want %q", token, "ghp-test")
		}
	})

	t.Run("gitlab default", func(t *testing.T) {
		t.Setenv("ORC_GITLAB_TOKEN", "glpat-test")
		token, err := ResolveTokenFromEnv(Config{}, ProviderGitLab)
		if err != nil {
			t.Fatalf("ResolveTokenFromEnv gitlab: %v", err)
		}
		if token != "glpat-test" {
			t.Fatalf("ResolveTokenFromEnv gitlab = %q, want %q", token, "glpat-test")
		}
	})

	t.Run("custom override", func(t *testing.T) {
		t.Setenv("CUSTOM_TOKEN", "custom")
		token, err := ResolveTokenFromEnv(Config{TokenEnvVar: "CUSTOM_TOKEN"}, ProviderGitHub)
		if err != nil {
			t.Fatalf("ResolveTokenFromEnv custom: %v", err)
		}
		if token != "custom" {
			t.Fatalf("ResolveTokenFromEnv custom = %q, want %q", token, "custom")
		}
	})
}

func TestAccountRegistrySaveCreatesDirectory(t *testing.T) {
	registry := &AccountRegistry{
		Accounts: map[string]Account{
			"personal": {Provider: "github"},
		},
	}
	path := filepath.Join(t.TempDir(), "nested", "hosting_accounts.yaml")
	if err := registry.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved file missing: %v", err)
	}
}
