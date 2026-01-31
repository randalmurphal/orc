package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/tests/testutil"
)

// TestConfigResolutionShared verifies that project config values are applied
// at the shared level.
func TestConfigResolutionShared(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Set value in project config (loaded at shared level)
	repo.SetConfig("profile", "safe")

	// Create empty user dir to isolate from real ~/.orc/config.yaml
	emptyUserDir := t.TempDir()

	// Create loader pointing to test repo with empty user dir
	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(emptyUserDir)

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Verify project value is applied
	if tc.Config.Profile != "safe" {
		t.Errorf("profile = %q, want %q", tc.Config.Profile, "safe")
	}

	// Check source tracking
	ts := tc.GetTrackedSource("profile")
	if ts.Source != config.SourceShared {
		t.Errorf("profile source = %v, want %v", ts.Source, config.SourceShared)
	}
	// Verify path points to project config
	if ts.Path == "" {
		t.Error("profile path should be set for shared source")
	}
}

// TestConfigResolutionPersonalOverridesShared verifies that personal config
// overrides project config.
func TestConfigResolutionPersonalOverridesShared(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Set value in project config (loaded at shared level)
	repo.SetConfig("profile", "safe")

	// Create personal config with different value
	userHome := testutil.MockUserConfig(t, "AM")

	// Add profile to user config
	userConfigPath := filepath.Join(userHome, ".orc", "config.yaml")
	userConfig := testutil.ReadYAML(t, userConfigPath)
	userConfig["profile"] = "strict"
	testutil.WriteYAML(t, userConfigPath, userConfig)

	// Create loader with custom user dir
	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(filepath.Join(userHome, ".orc"))

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Verify personal value overrides shared
	if tc.Config.Profile != "strict" {
		t.Errorf("profile = %q, want %q (personal should override shared)", tc.Config.Profile, "strict")
	}

	// Check source tracking
	ts := tc.GetTrackedSource("profile")
	if ts.Source != config.SourcePersonal {
		t.Errorf("profile source = %v, want %v", ts.Source, config.SourcePersonal)
	}
}

// TestConfigResolutionEnvOverridesAll verifies that environment variables
// override all other sources.
func TestConfigResolutionEnvOverridesAll(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Set value in project config
	repo.SetConfig("profile", "auto")

	// Set environment variable
	t.Setenv("ORC_PROFILE", "strict")

	// Create loader
	loader := config.NewLoader(repo.RootDir)

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Verify env value overrides everything
	if tc.Config.Profile != "strict" {
		t.Errorf("profile = %q, want %q (env should override all)", tc.Config.Profile, "strict")
	}

	// Check source tracking
	ts := tc.GetTrackedSource("profile")
	if ts.Source != config.SourceEnv {
		t.Errorf("profile source = %v, want %v", ts.Source, config.SourceEnv)
	}
}

// TestConfigResolutionSourceTracking verifies accurate source tracking
// for multiple config values from different sources.
func TestConfigResolutionSourceTracking(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Set values at project level (loaded as shared source)
	repo.SetConfig("profile", "safe")
	repo.SetConfig("model", "claude-sonnet")
	repo.SetConfig("max_iterations", 50)

	// Create personal config
	userHome := testutil.MockUserConfig(t, "AM")
	userConfigPath := filepath.Join(userHome, ".orc", "config.yaml")
	userConfig := testutil.ReadYAML(t, userConfigPath)
	userConfig["timeout"] = "15m"
	testutil.WriteYAML(t, userConfigPath, userConfig)

	// Set env var
	t.Setenv("ORC_RETRY_ENABLED", "false")

	// Create loader
	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(filepath.Join(userHome, ".orc"))

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	tests := []struct {
		key        string
		wantSource config.ConfigSource
	}{
		{"profile", config.SourceShared},        // Set in shared
		{"model", config.SourceShared},          // Set in shared
		{"max_iterations", config.SourceShared}, // Project is treated as shared level
		{"timeout", config.SourcePersonal},      // Set in personal
		{"retry.enabled", config.SourceEnv},     // Set via env
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			ts := tc.GetTrackedSource(tt.key)
			if ts.Source != tt.wantSource {
				t.Errorf("%s source = %v, want %v", tt.key, ts.Source, tt.wantSource)
			}
		})
	}
}

// TestConfigResolutionDefaults verifies that default values are used
// when not overridden.
func TestConfigResolutionDefaults(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Don't set any config values - use all defaults
	// Remove the project config to ensure defaults are used
	_ = os.Remove(filepath.Join(repo.OrcDir, "config.yaml"))

	// Create empty user dir to isolate from real ~/.orc/config.yaml
	emptyUserDir := t.TempDir()

	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(emptyUserDir)

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Verify defaults are applied
	defaults := config.Default()

	if tc.Config.Profile != defaults.Profile {
		t.Errorf("profile = %q, want default %q", tc.Config.Profile, defaults.Profile)
	}
	if tc.Config.MaxIterations != defaults.MaxIterations {
		t.Errorf("max_iterations = %d, want default %d", tc.Config.MaxIterations, defaults.MaxIterations)
	}
	if tc.Config.Retry.Enabled != defaults.Retry.Enabled {
		t.Errorf("retry.enabled = %v, want default %v", tc.Config.Retry.Enabled, defaults.Retry.Enabled)
	}

	// Verify source is default
	ts := tc.GetTrackedSource("profile")
	if ts.Source != config.SourceDefault {
		t.Errorf("profile source = %v, want %v", ts.Source, config.SourceDefault)
	}
}

// TestConfigResolutionPersonalOverridesProject verifies that personal config
// (~/.orc/config.yaml) overrides project config (.orc/config.yaml).
func TestConfigResolutionLocalOverridesProject(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Set value in project config
	repo.SetConfig("profile", "auto")

	// Create personal config with different value
	userHome := testutil.MockUserConfig(t, "AM")
	userConfigPath := filepath.Join(userHome, ".orc", "config.yaml")
	userConfig := testutil.ReadYAML(t, userConfigPath)
	userConfig["profile"] = "strict"
	testutil.WriteYAML(t, userConfigPath, userConfig)

	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(filepath.Join(userHome, ".orc"))

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Verify personal overrides project
	if tc.Config.Profile != "strict" {
		t.Errorf("profile = %q, want %q (personal should override project)", tc.Config.Profile, "strict")
	}
}

// TestConfigResolutionNestedValues verifies resolution of nested config values.
func TestConfigResolutionNestedValues(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Set nested values in project config (loaded at shared level)
	projectConfig := map[string]any{
		"version": 1,
		"gates": map[string]any{
			"default_type": "ai",
			"max_retries":  5,
		},
		"retry": map[string]any{
			"enabled":     true,
			"max_retries": 3,
		},
	}
	testutil.WriteYAML(t, filepath.Join(repo.OrcDir, "config.yaml"), projectConfig)

	// Create empty user dir to isolate from real ~/.orc/config.yaml
	emptyUserDir := t.TempDir()

	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(emptyUserDir)

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Verify nested values
	if tc.Config.Gates.DefaultType != "ai" {
		t.Errorf("gates.default_type = %q, want %q", tc.Config.Gates.DefaultType, "ai")
	}
	if tc.Config.Gates.MaxRetries != 5 {
		t.Errorf("gates.max_retries = %d, want %d", tc.Config.Gates.MaxRetries, 5)
	}
	if tc.Config.Retry.MaxRetries != 3 {
		t.Errorf("retry.max_retries = %d, want %d", tc.Config.Retry.MaxRetries, 3)
	}
}

// TestConfigResolutionIdentity verifies identity configuration loading.
func TestConfigResolutionIdentity(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Create user config with identity
	userHome := testutil.MockUserConfig(t, "AM")
	userConfigPath := filepath.Join(userHome, ".orc", "config.yaml")
	userConfig := testutil.ReadYAML(t, userConfigPath)
	userConfig["identity"] = map[string]any{
		"initials":     "AM",
		"display_name": "Alice Martinez",
		"email":        "alice@example.com",
	}
	testutil.WriteYAML(t, userConfigPath, userConfig)

	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(filepath.Join(userHome, ".orc"))

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Verify identity loaded
	if tc.Config.Identity.Initials != "AM" {
		t.Errorf("identity.initials = %q, want %q", tc.Config.Identity.Initials, "AM")
	}
	if tc.Config.Identity.DisplayName != "Alice Martinez" {
		t.Errorf("identity.display_name = %q, want %q", tc.Config.Identity.DisplayName, "Alice Martinez")
	}
}

// TestConfigResolutionExecutorPrefix verifies ExecutorPrefix() returns correct
// values based on mode and identity.
func TestConfigResolutionExecutorPrefix(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		initials string
		want     string
	}{
		{"solo mode returns empty", "solo", "AM", ""},
		{"p2p mode returns initials", "p2p", "AM", "AM"},
		{"team mode returns initials", "team", "BJ", "BJ"},
		{"p2p with empty initials", "p2p", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.TaskID.Mode = tt.mode
			cfg.Identity.Initials = tt.initials

			got := cfg.ExecutorPrefix()
			if got != tt.want {
				t.Errorf("ExecutorPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
