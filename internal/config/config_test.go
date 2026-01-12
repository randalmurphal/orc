package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}

	if cfg.Model == "" {
		t.Error("Model is empty")
	}

	if cfg.MaxIterations <= 0 {
		t.Errorf("MaxIterations = %d, want > 0", cfg.MaxIterations)
	}

	if cfg.Timeout <= 0 {
		t.Errorf("Timeout = %v, want > 0", cfg.Timeout)
	}

	if cfg.BranchPrefix != "orc/" {
		t.Errorf("BranchPrefix = %s, want orc/", cfg.BranchPrefix)
	}

	if cfg.CommitPrefix != "[orc]" {
		t.Errorf("CommitPrefix = %s, want [orc]", cfg.CommitPrefix)
	}
}

func TestDefault_TestingConfig(t *testing.T) {
	cfg := Default()

	// Required should be true by default
	if !cfg.Testing.Required {
		t.Error("Testing.Required should default to true")
	}

	// CoverageThreshold should be 0 (no threshold)
	if cfg.Testing.CoverageThreshold != 0 {
		t.Errorf("Testing.CoverageThreshold = %d, want 0", cfg.Testing.CoverageThreshold)
	}

	// Types should include "unit"
	if len(cfg.Testing.Types) == 0 {
		t.Fatal("Testing.Types should not be empty")
	}
	if cfg.Testing.Types[0] != "unit" {
		t.Errorf("Testing.Types[0] = %s, want unit", cfg.Testing.Types[0])
	}

	// SkipForWeights should include "trivial"
	if len(cfg.Testing.SkipForWeights) == 0 {
		t.Fatal("Testing.SkipForWeights should not be empty")
	}
	if cfg.Testing.SkipForWeights[0] != "trivial" {
		t.Errorf("Testing.SkipForWeights[0] = %s, want trivial", cfg.Testing.SkipForWeights[0])
	}

	// Commands should have unit test command
	if cfg.Testing.Commands.Unit != "go test ./..." {
		t.Errorf("Testing.Commands.Unit = %s, want 'go test ./...'", cfg.Testing.Commands.Unit)
	}

	// ParseOutput should be true
	if !cfg.Testing.ParseOutput {
		t.Error("Testing.ParseOutput should default to true")
	}
}

func TestDefault_DocumentationConfig(t *testing.T) {
	cfg := Default()

	// Enabled should be true by default
	if !cfg.Documentation.Enabled {
		t.Error("Documentation.Enabled should default to true")
	}

	// AutoUpdateClaudeMD should be true
	if !cfg.Documentation.AutoUpdateClaudeMD {
		t.Error("Documentation.AutoUpdateClaudeMD should default to true")
	}

	// UpdateOn should include "feature" and "api_change"
	if len(cfg.Documentation.UpdateOn) < 2 {
		t.Fatal("Documentation.UpdateOn should have at least 2 items")
	}

	foundFeature := false
	foundAPIChange := false
	for _, item := range cfg.Documentation.UpdateOn {
		if item == "feature" {
			foundFeature = true
		}
		if item == "api_change" {
			foundAPIChange = true
		}
	}
	if !foundFeature {
		t.Error("Documentation.UpdateOn should include 'feature'")
	}
	if !foundAPIChange {
		t.Error("Documentation.UpdateOn should include 'api_change'")
	}

	// SkipForWeights should include "trivial"
	if len(cfg.Documentation.SkipForWeights) == 0 {
		t.Fatal("Documentation.SkipForWeights should not be empty")
	}
	if cfg.Documentation.SkipForWeights[0] != "trivial" {
		t.Errorf("Documentation.SkipForWeights[0] = %s, want trivial", cfg.Documentation.SkipForWeights[0])
	}

	// Sections should have at least api-endpoints
	if len(cfg.Documentation.Sections) == 0 {
		t.Fatal("Documentation.Sections should not be empty")
	}
	foundAPIEndpoints := false
	for _, section := range cfg.Documentation.Sections {
		if section == "api-endpoints" {
			foundAPIEndpoints = true
		}
	}
	if !foundAPIEndpoints {
		t.Error("Documentation.Sections should include 'api-endpoints'")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config directory
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	configPath := filepath.Join(orcDir, "config.yaml")

	// Create and save config
	cfg := Default()
	cfg.Model = "test-model"
	cfg.MaxIterations = 50
	cfg.Timeout = 15 * time.Minute

	err := cfg.SaveTo(configPath)
	if err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	// Load config
	loaded, err := LoadFrom(configPath)
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	if loaded.Model != cfg.Model {
		t.Errorf("loaded Model = %s, want %s", loaded.Model, cfg.Model)
	}

	if loaded.MaxIterations != cfg.MaxIterations {
		t.Errorf("loaded MaxIterations = %d, want %d", loaded.MaxIterations, cfg.MaxIterations)
	}

	if loaded.Timeout != cfg.Timeout {
		t.Errorf("loaded Timeout = %v, want %v", loaded.Timeout, cfg.Timeout)
	}
}

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()

	// Init should succeed
	err := InitAt(tmpDir, false)
	if err != nil {
		t.Fatalf("InitAt() failed: %v", err)
	}

	// Verify .orc directory exists
	orcDir := filepath.Join(tmpDir, OrcDir)
	if _, err := os.Stat(orcDir); os.IsNotExist(err) {
		t.Error(".orc directory was not created")
	}

	// Verify tasks directory exists
	tasksDir := filepath.Join(orcDir, "tasks")
	if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
		t.Error(".orc/tasks directory was not created")
	}

	// Init again should fail without force
	err = InitAt(tmpDir, false)
	if err == nil {
		t.Error("InitAt() should fail when already initialized")
	}

	// Init with force should succeed
	err = InitAt(tmpDir, true)
	if err != nil {
		t.Fatalf("InitAt() with force failed: %v", err)
	}
}

func TestIsInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	// Not initialized
	if IsInitializedAt(tmpDir) {
		t.Error("IsInitializedAt() = true before init")
	}

	// Initialize
	InitAt(tmpDir, false)

	// Now initialized
	if !IsInitializedAt(tmpDir) {
		t.Error("IsInitializedAt() = false after init")
	}
}

func TestRequireInit(t *testing.T) {
	tmpDir := t.TempDir()

	// Should error before init
	err := RequireInitAt(tmpDir)
	if err == nil {
		t.Error("RequireInitAt() should error when not initialized")
	}

	// Initialize
	InitAt(tmpDir, false)

	// Should succeed after init
	err = RequireInitAt(tmpDir)
	if err != nil {
		t.Errorf("RequireInitAt() failed after init: %v", err)
	}
}

func TestResolveGateType(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		phase    string
		weight   string
		wantGate string
	}{
		{
			name: "default auto gates",
			cfg: &Config{
				Gates: GateConfig{
					DefaultType: "auto",
				},
			},
			phase:    "implement",
			weight:   "small",
			wantGate: "auto",
		},
		{
			name: "phase override",
			cfg: &Config{
				Gates: GateConfig{
					DefaultType: "auto",
					PhaseOverrides: map[string]string{
						"review": "human",
					},
				},
			},
			phase:    "review",
			weight:   "small",
			wantGate: "human",
		},
		{
			name: "weight override takes priority",
			cfg: &Config{
				Gates: GateConfig{
					DefaultType: "auto",
					PhaseOverrides: map[string]string{
						"spec": "ai",
					},
					WeightOverrides: map[string]map[string]string{
						"large": {
							"spec": "human",
						},
					},
				},
			},
			phase:    "spec",
			weight:   "large",
			wantGate: "human",
		},
		{
			name: "empty config returns auto",
			cfg: &Config{
				Gates: GateConfig{},
			},
			phase:    "test",
			weight:   "small",
			wantGate: "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateType := tt.cfg.ResolveGateType(tt.phase, tt.weight)
			if gateType != tt.wantGate {
				t.Errorf("ResolveGateType() = %v, want %v", gateType, tt.wantGate)
			}
		})
	}
}

func TestShouldRetryFrom(t *testing.T) {
	cfg := &Config{
		Retry: RetryConfig{
			Enabled:    true,
			MaxRetries: 3,
			RetryMap: map[string]string{
				"test":     "implement",
				"validate": "implement",
			},
		},
	}

	tests := []struct {
		phase    string
		wantFrom string
	}{
		{"test", "implement"},
		{"validate", "implement"},
		{"implement", ""},
		{"spec", ""},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			from := cfg.ShouldRetryFrom(tt.phase)
			if from != tt.wantFrom {
				t.Errorf("ShouldRetryFrom(%s) = %s, want %s", tt.phase, from, tt.wantFrom)
			}
		})
	}

	// Test with disabled retry
	cfgDisabled := &Config{
		Retry: RetryConfig{
			Enabled: false,
		},
	}
	from := cfgDisabled.ShouldRetryFrom("test")
	if from != "" {
		t.Errorf("ShouldRetryFrom() = %s, want empty when retry disabled", from)
	}
}

func TestProfilePresets(t *testing.T) {
	tests := []struct {
		profile     AutomationProfile
		wantDefault string
	}{
		{ProfileAuto, "auto"},
		{ProfileFast, "auto"},
		{ProfileSafe, "auto"},
		{ProfileStrict, "auto"},
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			preset := ProfilePresets(tt.profile)
			if preset.DefaultType != tt.wantDefault {
				t.Errorf("ProfilePresets(%s).DefaultType = %v, want %v", tt.profile, preset.DefaultType, tt.wantDefault)
			}
		})
	}

	// Check strict has phase overrides
	strict := ProfilePresets(ProfileStrict)
	if strict.PhaseOverrides == nil {
		t.Error("ProfilePresets(strict) should have PhaseOverrides")
	}
	if strict.PhaseOverrides["spec"] != "human" {
		t.Errorf("ProfilePresets(strict).PhaseOverrides[spec] = %v, want human", strict.PhaseOverrides["spec"])
	}
}

func TestApplyProfile(t *testing.T) {
	cfg := Default()

	// Apply strict profile
	cfg.ApplyProfile(ProfileStrict)

	// Verify gates changed - strict has human gates for spec
	if cfg.Gates.PhaseOverrides == nil {
		t.Fatal("After ApplyProfile(strict), PhaseOverrides should not be nil")
	}
	if cfg.Gates.PhaseOverrides["spec"] != "human" {
		t.Errorf("After ApplyProfile(strict), PhaseOverrides[spec] = %v, want human", cfg.Gates.PhaseOverrides["spec"])
	}

	// Apply auto profile
	cfg.ApplyProfile(ProfileAuto)
	if cfg.Profile != ProfileAuto {
		t.Errorf("After ApplyProfile(auto), Profile = %v, want auto", cfg.Profile)
	}
}

func TestLoadFrom_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid YAML
	invalidYAML := "invalid: yaml: content: [["
	err := os.WriteFile(tmpDir+"/invalid.yaml", []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid yaml: %v", err)
	}

	_, err = LoadFrom(tmpDir + "/invalid.yaml")
	if err == nil {
		t.Error("LoadFrom() should fail with invalid YAML")
	}
}

func TestLoadFrom_NonExistent(t *testing.T) {
	// LoadFrom returns default config when file doesn't exist
	cfg, err := LoadFrom("/nonexistent/path/config.yaml")
	if err != nil {
		t.Errorf("LoadFrom() should not error with non-existent file: %v", err)
	}
	// Should return default config
	if cfg == nil {
		t.Fatal("LoadFrom() should return default config for non-existent file")
	}
	if cfg.Version != 1 {
		t.Errorf("LoadFrom() default config should have Version=1, got %d", cfg.Version)
	}
}

func TestSaveTo_InvalidPath(t *testing.T) {
	cfg := Default()
	err := cfg.SaveTo("/nonexistent/directory/config.yaml")
	if err == nil {
		t.Error("SaveTo() should fail with invalid path")
	}
}

func TestDefault_DatabaseConfig(t *testing.T) {
	cfg := Default()

	// Driver should be sqlite by default
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("Database.Driver = %s, want sqlite", cfg.Database.Driver)
	}

	// SQLite paths
	if cfg.Database.SQLite.Path != ".orc/orc.db" {
		t.Errorf("Database.SQLite.Path = %s, want .orc/orc.db", cfg.Database.SQLite.Path)
	}
	if cfg.Database.SQLite.GlobalPath != "~/.orc/orc.db" {
		t.Errorf("Database.SQLite.GlobalPath = %s, want ~/.orc/orc.db", cfg.Database.SQLite.GlobalPath)
	}

	// Postgres defaults
	if cfg.Database.Postgres.Host != "localhost" {
		t.Errorf("Database.Postgres.Host = %s, want localhost", cfg.Database.Postgres.Host)
	}
	if cfg.Database.Postgres.Port != 5432 {
		t.Errorf("Database.Postgres.Port = %d, want 5432", cfg.Database.Postgres.Port)
	}
	if cfg.Database.Postgres.Database != "orc" {
		t.Errorf("Database.Postgres.Database = %s, want orc", cfg.Database.Postgres.Database)
	}
	if cfg.Database.Postgres.User != "orc" {
		t.Errorf("Database.Postgres.User = %s, want orc", cfg.Database.Postgres.User)
	}
	if cfg.Database.Postgres.SSLMode != "disable" {
		t.Errorf("Database.Postgres.SSLMode = %s, want disable", cfg.Database.Postgres.SSLMode)
	}
	if cfg.Database.Postgres.PoolMax != 10 {
		t.Errorf("Database.Postgres.PoolMax = %d, want 10", cfg.Database.Postgres.PoolMax)
	}
}

func TestDSN_SQLite(t *testing.T) {
	cfg := Default()

	dsn := cfg.DSN()
	if dsn != ".orc/orc.db" {
		t.Errorf("DSN() = %s, want .orc/orc.db", dsn)
	}

	globalDSN := cfg.GlobalDSN()
	if globalDSN != "~/.orc/orc.db" {
		t.Errorf("GlobalDSN() = %s, want ~/.orc/orc.db", globalDSN)
	}
}

func TestDSN_Postgres(t *testing.T) {
	cfg := Default()
	cfg.Database.Driver = "postgres"
	cfg.Database.Postgres.Password = "secret"

	dsn := cfg.DSN()
	expected := "postgres://orc:secret@localhost:5432/orc?sslmode=disable"
	if dsn != expected {
		t.Errorf("DSN() = %s, want %s", dsn, expected)
	}

	// GlobalDSN should return same as DSN for postgres
	globalDSN := cfg.GlobalDSN()
	if globalDSN != expected {
		t.Errorf("GlobalDSN() = %s, want %s", globalDSN, expected)
	}
}

func TestDefault_PlanConfig(t *testing.T) {
	cfg := Default()

	// RequireSpecForExecution should default to false
	if cfg.Plan.RequireSpecForExecution {
		t.Error("Plan.RequireSpecForExecution should default to false")
	}

	// WarnOnMissingSpec should default to true
	if !cfg.Plan.WarnOnMissingSpec {
		t.Error("Plan.WarnOnMissingSpec should default to true")
	}

	// SkipValidationWeights should default to [trivial]
	if len(cfg.Plan.SkipValidationWeights) != 1 || cfg.Plan.SkipValidationWeights[0] != "trivial" {
		t.Errorf("Plan.SkipValidationWeights = %v, want [trivial]", cfg.Plan.SkipValidationWeights)
	}

	// MinimumSections should default to intent, success_criteria, testing
	expected := []string{"intent", "success_criteria", "testing"}
	if len(cfg.Plan.MinimumSections) != 3 {
		t.Errorf("Plan.MinimumSections = %v, want %v", cfg.Plan.MinimumSections, expected)
	}
	for i, section := range expected {
		if i < len(cfg.Plan.MinimumSections) && cfg.Plan.MinimumSections[i] != section {
			t.Errorf("Plan.MinimumSections[%d] = %s, want %s", i, cfg.Plan.MinimumSections[i], section)
		}
	}
}
