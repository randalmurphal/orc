package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithSources_DefaultsOnly(t *testing.T) {
	// Use a temp dir with no config files
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	tc, err := LoadWithSources()
	if err != nil {
		t.Fatalf("LoadWithSources failed: %v", err)
	}

	// Check defaults are loaded
	if tc.Config.Profile != ProfileAuto {
		t.Errorf("Profile = %q, want %q", tc.Config.Profile, ProfileAuto)
	}

	// Check sources are all default
	if tc.GetSource("profile") != SourceDefault {
		t.Errorf("profile source = %q, want default", tc.GetSource("profile"))
	}
}

func TestLoadWithSources_ProjectConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create project config
	os.MkdirAll(".orc", 0755)
	projectConfig := `
profile: strict
model: claude-sonnet
gates:
  default_type: human
`
	os.WriteFile(".orc/config.yaml", []byte(projectConfig), 0644)

	tc, err := LoadWithSources()
	if err != nil {
		t.Fatalf("LoadWithSources failed: %v", err)
	}

	// Check project config is loaded
	if tc.Config.Profile != ProfileStrict {
		t.Errorf("Profile = %q, want strict", tc.Config.Profile)
	}
	if tc.Config.Model != "claude-sonnet" {
		t.Errorf("Model = %q, want claude-sonnet", tc.Config.Model)
	}
	if tc.Config.Gates.DefaultType != "human" {
		t.Errorf("Gates.DefaultType = %q, want human", tc.Config.Gates.DefaultType)
	}

	// Check sources
	if tc.GetSource("profile") != SourceProject {
		t.Errorf("profile source = %q, want project", tc.GetSource("profile"))
	}
	if tc.GetSource("model") != SourceProject {
		t.Errorf("model source = %q, want project", tc.GetSource("model"))
	}
	if tc.GetSource("gates.default_type") != SourceProject {
		t.Errorf("gates.default_type source = %q, want project", tc.GetSource("gates.default_type"))
	}

	// Check defaults for unset values
	if tc.GetSource("timeout") != SourceDefault {
		t.Errorf("timeout source = %q, want default", tc.GetSource("timeout"))
	}
}

func TestLoadWithSources_UserConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create a fake home directory
	fakeHome := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(fakeHome, ".orc"), 0755)

	// Set HOME temporarily
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)
	defer os.Setenv("HOME", origHome)

	// Create user config
	userConfig := `
profile: safe
retry:
  enabled: false
`
	os.WriteFile(filepath.Join(fakeHome, ".orc", "config.yaml"), []byte(userConfig), 0644)

	tc, err := LoadWithSources()
	if err != nil {
		t.Fatalf("LoadWithSources failed: %v", err)
	}

	// Check user config is loaded
	if tc.Config.Profile != ProfileSafe {
		t.Errorf("Profile = %q, want safe", tc.Config.Profile)
	}
	if tc.Config.Retry.Enabled {
		t.Error("Retry.Enabled = true, want false")
	}

	// Check sources
	if tc.GetSource("profile") != SourceUser {
		t.Errorf("profile source = %q, want user", tc.GetSource("profile"))
	}
	if tc.GetSource("retry.enabled") != SourceUser {
		t.Errorf("retry.enabled source = %q, want user", tc.GetSource("retry.enabled"))
	}
}

func TestLoadWithSources_EnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create project config
	os.MkdirAll(".orc", 0755)
	projectConfig := `profile: auto`
	os.WriteFile(".orc/config.yaml", []byte(projectConfig), 0644)

	// Set env var
	t.Setenv("ORC_PROFILE", "strict")
	t.Setenv("ORC_MODEL", "claude-sonnet")
	t.Setenv("ORC_RETRY_ENABLED", "false")

	tc, err := LoadWithSources()
	if err != nil {
		t.Fatalf("LoadWithSources failed: %v", err)
	}

	// Check env overrides project
	if tc.Config.Profile != ProfileStrict {
		t.Errorf("Profile = %q, want strict (from env)", tc.Config.Profile)
	}
	if tc.Config.Model != "claude-sonnet" {
		t.Errorf("Model = %q, want claude-sonnet", tc.Config.Model)
	}
	if tc.Config.Retry.Enabled {
		t.Error("Retry.Enabled = true, want false")
	}

	// Check sources
	if tc.GetSource("profile") != SourceEnv {
		t.Errorf("profile source = %q, want env", tc.GetSource("profile"))
	}
	if tc.GetSource("model") != SourceEnv {
		t.Errorf("model source = %q, want env", tc.GetSource("model"))
	}
	if tc.GetSource("retry.enabled") != SourceEnv {
		t.Errorf("retry.enabled source = %q, want env", tc.GetSource("retry.enabled"))
	}
}

func TestLoadWithSources_HierarchyOrder(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create fake home
	fakeHome := filepath.Join(tmpDir, "home")
	os.MkdirAll(filepath.Join(fakeHome, ".orc"), 0755)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", fakeHome)
	defer os.Setenv("HOME", origHome)

	// User config sets profile to safe
	os.WriteFile(filepath.Join(fakeHome, ".orc", "config.yaml"),
		[]byte("profile: safe"), 0644)

	// Project config sets profile to strict (should override user)
	os.MkdirAll(".orc", 0755)
	os.WriteFile(".orc/config.yaml",
		[]byte("profile: strict"), 0644)

	tc, err := LoadWithSources()
	if err != nil {
		t.Fatalf("LoadWithSources failed: %v", err)
	}

	// Project should override user
	if tc.Config.Profile != ProfileStrict {
		t.Errorf("Profile = %q, want strict (project overrides user)", tc.Config.Profile)
	}
	if tc.GetSource("profile") != SourceProject {
		t.Errorf("profile source = %q, want project", tc.GetSource("profile"))
	}
}

func TestApplyEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		value    string
		check    func(*Config) bool
		wantPath string
	}{
		{
			name:     "profile",
			envVar:   "ORC_PROFILE",
			value:    "strict",
			check:    func(c *Config) bool { return c.Profile == ProfileStrict },
			wantPath: "profile",
		},
		{
			name:     "max_iterations",
			envVar:   "ORC_MAX_ITERATIONS",
			value:    "50",
			check:    func(c *Config) bool { return c.MaxIterations == 50 },
			wantPath: "max_iterations",
		},
		{
			name:     "timeout",
			envVar:   "ORC_TIMEOUT",
			value:    "5m",
			check:    func(c *Config) bool { return c.Timeout.Minutes() == 5 },
			wantPath: "timeout",
		},
		{
			name:     "retry_enabled_false",
			envVar:   "ORC_RETRY_ENABLED",
			value:    "false",
			check:    func(c *Config) bool { return !c.Retry.Enabled },
			wantPath: "retry.enabled",
		},
		{
			name:     "retry_enabled_true",
			envVar:   "ORC_RETRY_ENABLED",
			value:    "true",
			check:    func(c *Config) bool { return c.Retry.Enabled },
			wantPath: "retry.enabled",
		},
		{
			name:     "gates_default",
			envVar:   "ORC_GATES_DEFAULT",
			value:    "human",
			check:    func(c *Config) bool { return c.Gates.DefaultType == "human" },
			wantPath: "gates.default_type",
		},
		{
			name:     "worktree_enabled",
			envVar:   "ORC_WORKTREE_ENABLED",
			value:    "false",
			check:    func(c *Config) bool { return !c.Worktree.Enabled },
			wantPath: "worktree.enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all ORC env vars first
			for envVar := range EnvVarMapping {
				os.Unsetenv(envVar)
			}

			t.Setenv(tt.envVar, tt.value)

			tc := NewTrackedConfig()
			overridden := ApplyEnvVars(tc)

			if !tt.check(tc.Config) {
				t.Errorf("config not set correctly for %s=%s", tt.envVar, tt.value)
			}

			found := false
			for _, path := range overridden {
				if path == tt.wantPath {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("path %q not in overridden list: %v", tt.wantPath, overridden)
			}

			if tc.GetSource(tt.wantPath) != SourceEnv {
				t.Errorf("source for %q = %q, want env", tt.wantPath, tc.GetSource(tt.wantPath))
			}
		})
	}
}
