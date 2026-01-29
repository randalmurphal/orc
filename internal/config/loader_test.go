package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithSources_DefaultsOnly(t *testing.T) {
	// Use a temp dir with no config files
	tmpDir := t.TempDir()

	// Use empty home to avoid picking up real user config
	t.Setenv("HOME", filepath.Join(tmpDir, "nonexistent"))

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
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

func TestLoadWithSources_SharedConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Use empty home to avoid picking up real user config
	t.Setenv("HOME", filepath.Join(tmpDir, "nonexistent"))

	// Create shared config (.orc/config.yaml)
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	sharedConfig := `
profile: strict
model: claude-sonnet
gates:
  default_type: human
`
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(sharedConfig), 0644)

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Check shared config is loaded
	if tc.Config.Profile != ProfileStrict {
		t.Errorf("Profile = %q, want strict", tc.Config.Profile)
	}
	if tc.Config.Model != "claude-sonnet" {
		t.Errorf("Model = %q, want claude-sonnet", tc.Config.Model)
	}
	if tc.Config.Gates.DefaultType != "human" {
		t.Errorf("Gates.DefaultType = %q, want human", tc.Config.Gates.DefaultType)
	}

	// Check sources - should be SourceShared
	if tc.GetSource("profile") != SourceShared {
		t.Errorf("profile source = %q, want shared", tc.GetSource("profile"))
	}
	if tc.GetSource("model") != SourceShared {
		t.Errorf("model source = %q, want shared", tc.GetSource("model"))
	}
	if tc.GetSource("gates.default_type") != SourceShared {
		t.Errorf("gates.default_type source = %q, want shared", tc.GetSource("gates.default_type"))
	}

	// Check defaults for unset values
	if tc.GetSource("timeout") != SourceDefault {
		t.Errorf("timeout source = %q, want default", tc.GetSource("timeout"))
	}
}

func TestLoadWithSources_SharedDirConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Use empty home
	t.Setenv("HOME", filepath.Join(tmpDir, "nonexistent"))

	// Create .orc/config.yaml with one value
	orcDir := filepath.Join(tmpDir, ".orc")
	sharedDir := filepath.Join(orcDir, "shared")
	_ = os.MkdirAll(sharedDir, 0755)
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("profile: safe\nmodel: model-a"), 0644)

	// Create .orc/shared/config.yaml that overrides
	_ = os.WriteFile(filepath.Join(sharedDir, "config.yaml"), []byte("model: model-b"), 0644)

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Profile from .orc/config.yaml (not overridden)
	if tc.Config.Profile != ProfileSafe {
		t.Errorf("Profile = %q, want safe", tc.Config.Profile)
	}

	// Model from .orc/shared/config.yaml (overrides .orc/config.yaml)
	if tc.Config.Model != "model-b" {
		t.Errorf("Model = %q, want model-b", tc.Config.Model)
	}

	// Both should be SourceShared
	if tc.GetSource("profile") != SourceShared {
		t.Errorf("profile source = %q, want shared", tc.GetSource("profile"))
	}
	if tc.GetSource("model") != SourceShared {
		t.Errorf("model source = %q, want shared", tc.GetSource("model"))
	}
}

func TestLoadWithSources_PersonalConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake home directory
	fakeHome := filepath.Join(tmpDir, "home")
	_ = os.MkdirAll(filepath.Join(fakeHome, ".orc"), 0755)

	// Set HOME temporarily
	t.Setenv("HOME", fakeHome)

	// Create user config (~/.orc/config.yaml)
	userConfig := `
profile: safe
retry:
  enabled: false
`
	_ = os.WriteFile(filepath.Join(fakeHome, ".orc", "config.yaml"), []byte(userConfig), 0644)

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Check user config is loaded
	if tc.Config.Profile != ProfileSafe {
		t.Errorf("Profile = %q, want safe", tc.Config.Profile)
	}
	if tc.Config.Retry.Enabled {
		t.Error("Retry.Enabled = true, want false")
	}

	// Check sources - should be SourcePersonal
	if tc.GetSource("profile") != SourcePersonal {
		t.Errorf("profile source = %q, want personal", tc.GetSource("profile"))
	}
	if tc.GetSource("retry.enabled") != SourcePersonal {
		t.Errorf("retry.enabled source = %q, want personal", tc.GetSource("retry.enabled"))
	}
}

func TestLoadWithSources_LocalConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake home with global personal config
	fakeHome := filepath.Join(tmpDir, "home")
	_ = os.MkdirAll(filepath.Join(fakeHome, ".orc"), 0755)
	t.Setenv("HOME", fakeHome)
	_ = os.WriteFile(filepath.Join(fakeHome, ".orc", "config.yaml"),
		[]byte("profile: safe\nmodel: global-model"), 0644)

	// Create local personal config (.orc/local/config.yaml)
	localDir := filepath.Join(tmpDir, ".orc", "local")
	_ = os.MkdirAll(localDir, 0755)
	_ = os.WriteFile(filepath.Join(localDir, "config.yaml"), []byte("model: local-model"), 0644)

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Profile from ~/.orc/config.yaml (not overridden)
	if tc.Config.Profile != ProfileSafe {
		t.Errorf("Profile = %q, want safe", tc.Config.Profile)
	}

	// Model from .orc/local/config.yaml (overrides ~/.orc/config.yaml)
	if tc.Config.Model != "local-model" {
		t.Errorf("Model = %q, want local-model", tc.Config.Model)
	}

	// Both should be SourcePersonal
	if tc.GetSource("profile") != SourcePersonal {
		t.Errorf("profile source = %q, want personal", tc.GetSource("profile"))
	}
	if tc.GetSource("model") != SourcePersonal {
		t.Errorf("model source = %q, want personal", tc.GetSource("model"))
	}
}

func TestLoadWithSources_EnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	// Use empty home
	t.Setenv("HOME", filepath.Join(tmpDir, "nonexistent"))

	// Create shared config
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("profile: auto"), 0644)

	// Set env var
	t.Setenv("ORC_PROFILE", "strict")
	t.Setenv("ORC_MODEL", "claude-sonnet")
	t.Setenv("ORC_RETRY_ENABLED", "false")

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Check env overrides everything
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

// TestLoadWithSources_PersonalBeatsShared verifies the key 4-level hierarchy behavior:
// Personal settings (user preferences) override shared settings (team defaults).
func TestLoadWithSources_PersonalBeatsShared(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake home
	fakeHome := filepath.Join(tmpDir, "home")
	_ = os.MkdirAll(filepath.Join(fakeHome, ".orc"), 0755)
	t.Setenv("HOME", fakeHome)

	// Personal config sets profile to safe (user's preference)
	_ = os.WriteFile(filepath.Join(fakeHome, ".orc", "config.yaml"),
		[]byte("profile: safe"), 0644)

	// Shared config sets profile to strict (team default)
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"),
		[]byte("profile: strict"), 0644)

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Personal should override shared
	if tc.Config.Profile != ProfileSafe {
		t.Errorf("Profile = %q, want safe (personal overrides shared)", tc.Config.Profile)
	}
	if tc.GetSource("profile") != SourcePersonal {
		t.Errorf("profile source = %q, want personal", tc.GetSource("profile"))
	}
}

// TestLoadWithSources_RuntimeBeatsPersonal verifies runtime (env) beats personal.
func TestLoadWithSources_RuntimeBeatsPersonal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake home
	fakeHome := filepath.Join(tmpDir, "home")
	_ = os.MkdirAll(filepath.Join(fakeHome, ".orc"), 0755)
	t.Setenv("HOME", fakeHome)

	// Personal config sets profile
	_ = os.WriteFile(filepath.Join(fakeHome, ".orc", "config.yaml"),
		[]byte("profile: safe"), 0644)

	// Runtime (env) should win
	t.Setenv("ORC_PROFILE", "strict")

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Runtime (env) should override personal
	if tc.Config.Profile != ProfileStrict {
		t.Errorf("Profile = %q, want strict (runtime overrides personal)", tc.Config.Profile)
	}
	if tc.GetSource("profile") != SourceEnv {
		t.Errorf("profile source = %q, want env", tc.GetSource("profile"))
	}
}

// TestLoadWithSources_FullHierarchy tests all 4 levels together.
func TestLoadWithSources_FullHierarchy(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake home
	fakeHome := filepath.Join(tmpDir, "home")
	_ = os.MkdirAll(filepath.Join(fakeHome, ".orc"), 0755)
	t.Setenv("HOME", fakeHome)

	// Level 3 (Shared): team defaults
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(`
profile: auto
model: shared-model
max_iterations: 10
branch_prefix: team/
`), 0644)

	// Level 2 (Personal): user preferences
	_ = os.WriteFile(filepath.Join(fakeHome, ".orc", "config.yaml"), []byte(`
profile: safe
model: personal-model
`), 0644)

	// Level 1 (Runtime): env override
	t.Setenv("ORC_MODEL", "runtime-model")

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Profile: personal beats shared
	if tc.Config.Profile != ProfileSafe {
		t.Errorf("Profile = %q, want safe", tc.Config.Profile)
	}
	if tc.GetSource("profile") != SourcePersonal {
		t.Errorf("profile source = %q, want personal", tc.GetSource("profile"))
	}

	// Model: runtime beats personal
	if tc.Config.Model != "runtime-model" {
		t.Errorf("Model = %q, want runtime-model", tc.Config.Model)
	}
	if tc.GetSource("model") != SourceEnv {
		t.Errorf("model source = %q, want env", tc.GetSource("model"))
	}

	// MaxIterations: only in shared
	if tc.Config.MaxIterations != 10 {
		t.Errorf("MaxIterations = %d, want 10", tc.Config.MaxIterations)
	}
	if tc.GetSource("max_iterations") != SourceShared {
		t.Errorf("max_iterations source = %q, want shared", tc.GetSource("max_iterations"))
	}

	// BranchPrefix: only in shared
	if tc.Config.BranchPrefix != "team/" {
		t.Errorf("BranchPrefix = %q, want team/", tc.Config.BranchPrefix)
	}
	if tc.GetSource("branch_prefix") != SourceShared {
		t.Errorf("branch_prefix source = %q, want shared", tc.GetSource("branch_prefix"))
	}

	// Timeout: not set anywhere, should be default
	if tc.GetSource("timeout") != SourceDefault {
		t.Errorf("timeout source = %q, want default", tc.GetSource("timeout"))
	}
}

func TestLoadWithSources_SourcePathTracking(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake home
	fakeHome := filepath.Join(tmpDir, "home")
	_ = os.MkdirAll(filepath.Join(fakeHome, ".orc"), 0755)
	t.Setenv("HOME", fakeHome)

	// Create configs
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("profile: strict"), 0644)
	_ = os.WriteFile(filepath.Join(fakeHome, ".orc", "config.yaml"),
		[]byte("model: my-model"), 0644)

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Check TrackedSource includes file path
	profileTS := tc.GetTrackedSource("profile")
	if profileTS.Source != SourceShared {
		t.Errorf("profile TrackedSource.Source = %q, want shared", profileTS.Source)
	}
	if profileTS.Path == "" {
		t.Error("profile TrackedSource.Path is empty, want file path")
	}

	modelTS := tc.GetTrackedSource("model")
	if modelTS.Source != SourcePersonal {
		t.Errorf("model TrackedSource.Source = %q, want personal", modelTS.Source)
	}
	if modelTS.Path == "" {
		t.Error("model TrackedSource.Path is empty, want file path")
	}
}

func TestLoadWithSources_MissingFilesOK(t *testing.T) {
	tmpDir := t.TempDir()

	// Use empty home - no config files anywhere
	t.Setenv("HOME", filepath.Join(tmpDir, "nonexistent"))

	// No .orc directory, no config files - should still work with defaults
	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed with missing files: %v", err)
	}

	// Should have defaults
	if tc.Config.Profile != ProfileAuto {
		t.Errorf("Profile = %q, want auto (default)", tc.Config.Profile)
	}
	if tc.Config.Model != Default().Model {
		t.Errorf("Model = %q, want %q (default)", tc.Config.Model, Default().Model)
	}
}

func TestLoader_SetDirectories(t *testing.T) {
	loader := NewLoader("/project")

	loader.SetUserDir("/custom/user")
	loader.SetProjectDir("/custom/project")

	paths := loader.GetConfigPaths()

	// Check personal paths include custom user dir
	found := false
	for _, p := range paths[LevelPersonal] {
		if p == "/custom/user/config.yaml" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Personal paths don't include custom user dir: %v", paths[LevelPersonal])
	}

	// Check shared paths include custom project dir
	found = false
	for _, p := range paths[LevelShared] {
		if p == "/custom/project/.orc/config.yaml" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Shared paths don't include custom project dir: %v", paths[LevelShared])
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
				_ = os.Unsetenv(envVar)
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

func TestConfigLevel_String(t *testing.T) {
	tests := []struct {
		level ConfigLevel
		want  string
	}{
		{LevelDefaults, "default"},
		{LevelShared, "shared"},
		{LevelPersonal, "personal"},
		{LevelRuntime, "runtime"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("ConfigLevel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigSource_Level(t *testing.T) {
	tests := []struct {
		source ConfigSource
		want   ConfigLevel
	}{
		{SourceDefault, LevelDefaults},
		{SourceShared, LevelShared},
		{SourcePersonal, LevelPersonal},
		{SourceEnv, LevelRuntime},
		{SourceFlag, LevelRuntime},
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			if got := tt.source.Level(); got != tt.want {
				t.Errorf("ConfigSource.Level() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrackedSource_String(t *testing.T) {
	tests := []struct {
		ts   TrackedSource
		want string
	}{
		{TrackedSource{Source: SourceDefault}, "default"},
		{TrackedSource{Source: SourceShared, Path: ".orc/config.yaml"}, "shared: .orc/config.yaml"},
		{TrackedSource{Source: SourcePersonal, Path: "~/.orc/config.yaml"}, "personal: ~/.orc/config.yaml"},
		{TrackedSource{Source: SourceEnv}, "env"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ts.String(); got != tt.want {
				t.Errorf("TrackedSource.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestLoadWithSources_EnvSourceHasNoPath verifies that env var sources have empty path.
func TestLoadWithSources_EnvSourceHasNoPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Use empty home
	t.Setenv("HOME", filepath.Join(tmpDir, "nonexistent"))

	// Set env var
	t.Setenv("ORC_PROFILE", "strict")

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Env source should have empty path
	ts := tc.GetTrackedSource("profile")
	if ts.Source != SourceEnv {
		t.Errorf("profile source = %q, want env", ts.Source)
	}
	if ts.Path != "" {
		t.Errorf("env source should have empty path, got %q", ts.Path)
	}
}

func TestApplyEnvVars_Database(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		value    string
		check    func(*Config) bool
		wantPath string
	}{
		{
			name:     "db_driver",
			envVar:   "ORC_DB_DRIVER",
			value:    "postgres",
			check:    func(c *Config) bool { return c.Database.Driver == "postgres" },
			wantPath: "database.driver",
		},
		{
			name:     "db_host",
			envVar:   "ORC_DB_HOST",
			value:    "db.example.com",
			check:    func(c *Config) bool { return c.Database.Postgres.Host == "db.example.com" },
			wantPath: "database.postgres.host",
		},
		{
			name:     "db_port",
			envVar:   "ORC_DB_PORT",
			value:    "5433",
			check:    func(c *Config) bool { return c.Database.Postgres.Port == 5433 },
			wantPath: "database.postgres.port",
		},
		{
			name:     "db_name",
			envVar:   "ORC_DB_NAME",
			value:    "mydb",
			check:    func(c *Config) bool { return c.Database.Postgres.Database == "mydb" },
			wantPath: "database.postgres.database",
		},
		{
			name:     "db_user",
			envVar:   "ORC_DB_USER",
			value:    "myuser",
			check:    func(c *Config) bool { return c.Database.Postgres.User == "myuser" },
			wantPath: "database.postgres.user",
		},
		{
			name:     "db_password",
			envVar:   "ORC_DB_PASSWORD",
			value:    "secret123",
			check:    func(c *Config) bool { return c.Database.Postgres.Password == "secret123" },
			wantPath: "database.postgres.password",
		},
		{
			name:     "db_ssl_mode",
			envVar:   "ORC_DB_SSL_MODE",
			value:    "require",
			check:    func(c *Config) bool { return c.Database.Postgres.SSLMode == "require" },
			wantPath: "database.postgres.ssl_mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all ORC env vars first
			for envVar := range EnvVarMapping {
				_ = os.Unsetenv(envVar)
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

func TestLoadWithSources_DatabaseConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Use empty home
	t.Setenv("HOME", filepath.Join(tmpDir, "nonexistent"))

	// Create shared config with database settings
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	dbConfig := `
database:
  driver: postgres
  postgres:
    host: db.team.local
    port: 5432
    database: team_orc
    user: team_user
    ssl_mode: require
`
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(dbConfig), 0644)

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Check database config is loaded
	if tc.Config.Database.Driver != "postgres" {
		t.Errorf("Database.Driver = %q, want postgres", tc.Config.Database.Driver)
	}
	if tc.Config.Database.Postgres.Host != "db.team.local" {
		t.Errorf("Database.Postgres.Host = %q, want db.team.local", tc.Config.Database.Postgres.Host)
	}
	if tc.Config.Database.Postgres.Database != "team_orc" {
		t.Errorf("Database.Postgres.Database = %q, want team_orc", tc.Config.Database.Postgres.Database)
	}
	if tc.Config.Database.Postgres.SSLMode != "require" {
		t.Errorf("Database.Postgres.SSLMode = %q, want require", tc.Config.Database.Postgres.SSLMode)
	}

	// Check sources
	if tc.GetSource("database.driver") != SourceShared {
		t.Errorf("database.driver source = %q, want shared", tc.GetSource("database.driver"))
	}
	if tc.GetSource("database.postgres.host") != SourceShared {
		t.Errorf("database.postgres.host source = %q, want shared", tc.GetSource("database.postgres.host"))
	}
}

func TestLoadWithSources_PRAutoApprove(t *testing.T) {
	tmpDir := t.TempDir()

	// Use empty home to avoid picking up real user config
	t.Setenv("HOME", filepath.Join(tmpDir, "nonexistent"))

	// Create config that disables auto_approve
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	configYAML := `
profile: auto
completion:
  pr:
    auto_approve: false
`
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configYAML), 0644)

	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Check that auto_approve was loaded from config
	if tc.Config.Completion.PR.AutoApprove {
		t.Error("Completion.PR.AutoApprove should be false (loaded from config)")
	}

	// Check source
	if tc.GetSource("completion.pr.auto_approve") != SourceShared {
		t.Errorf("completion.pr.auto_approve source = %q, want shared",
			tc.GetSource("completion.pr.auto_approve"))
	}
}

func TestLoadWithSources_PRAutoApprove_Default(t *testing.T) {
	tmpDir := t.TempDir()

	// Use empty home to avoid picking up real user config
	t.Setenv("HOME", filepath.Join(tmpDir, "nonexistent"))

	// No config file - should use default
	tc, err := LoadWithSourcesFrom(tmpDir)
	if err != nil {
		t.Fatalf("LoadWithSourcesFrom failed: %v", err)
	}

	// Default should be false (auto-approve is opt-in)
	if tc.Config.Completion.PR.AutoApprove {
		t.Error("Completion.PR.AutoApprove should default to false")
	}

	// Check source is default
	if tc.GetSource("completion.pr.auto_approve") != SourceDefault {
		t.Errorf("completion.pr.auto_approve source = %q, want default",
			tc.GetSource("completion.pr.auto_approve"))
	}
}
