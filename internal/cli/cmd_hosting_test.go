package cli

import (
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/project"
)

func TestWriteProjectHostingAccountSelection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	configPath, err := writeProjectHostingAccountSelection("proj-123", "nulliti-ghe")
	if err != nil {
		t.Fatalf("writeProjectHostingAccountSelection() failed: %v", err)
	}

	expectedPath, err := project.ProjectLocalConfigPath("proj-123")
	if err != nil {
		t.Fatalf("ProjectLocalConfigPath() failed: %v", err)
	}
	if configPath != expectedPath {
		t.Fatalf("configPath = %q, want %q", configPath, expectedPath)
	}

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile() failed: %v", err)
	}
	if cfg.Hosting.Account != "nulliti-ghe" {
		t.Fatalf("Hosting.Account = %q, want %q", cfg.Hosting.Account, "nulliti-ghe")
	}
	if cfg.Hosting.Provider != "" {
		t.Fatalf("Hosting.Provider = %q, want empty", cfg.Hosting.Provider)
	}
	if cfg.Hosting.BaseURL != "" {
		t.Fatalf("Hosting.BaseURL = %q, want empty", cfg.Hosting.BaseURL)
	}
	if cfg.Hosting.TokenEnvVar != "" {
		t.Fatalf("Hosting.TokenEnvVar = %q, want empty", cfg.Hosting.TokenEnvVar)
	}
}
