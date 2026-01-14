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

func TestDefault_SyncConfig(t *testing.T) {
	cfg := Default()

	// Strategy should default to completion
	if cfg.Completion.Sync.Strategy != SyncStrategyCompletion {
		t.Errorf("Completion.Sync.Strategy = %s, want completion", cfg.Completion.Sync.Strategy)
	}

	// FailOnConflict should default to true
	if !cfg.Completion.Sync.FailOnConflict {
		t.Error("Completion.Sync.FailOnConflict should default to true")
	}

	// MaxConflictFiles should default to 0 (unlimited)
	if cfg.Completion.Sync.MaxConflictFiles != 0 {
		t.Errorf("Completion.Sync.MaxConflictFiles = %d, want 0", cfg.Completion.Sync.MaxConflictFiles)
	}

	// SkipForWeights should include trivial
	if len(cfg.Completion.Sync.SkipForWeights) == 0 {
		t.Fatal("Completion.Sync.SkipForWeights should not be empty")
	}
	if cfg.Completion.Sync.SkipForWeights[0] != "trivial" {
		t.Errorf("Completion.Sync.SkipForWeights[0] = %s, want trivial", cfg.Completion.Sync.SkipForWeights[0])
	}
}

func TestShouldSyncForWeight(t *testing.T) {
	cfg := Default()

	// Should sync for medium weight
	if !cfg.ShouldSyncForWeight("medium") {
		t.Error("ShouldSyncForWeight(medium) should return true")
	}

	// Should not sync for trivial weight (in skip list)
	if cfg.ShouldSyncForWeight("trivial") {
		t.Error("ShouldSyncForWeight(trivial) should return false")
	}

	// Should sync for large weight
	if !cfg.ShouldSyncForWeight("large") {
		t.Error("ShouldSyncForWeight(large) should return true")
	}
}

func TestShouldSyncForWeight_StrategyNone(t *testing.T) {
	cfg := Default()
	cfg.Completion.Sync.Strategy = SyncStrategyNone

	// Should not sync for any weight when strategy is none
	if cfg.ShouldSyncForWeight("medium") {
		t.Error("ShouldSyncForWeight should return false when strategy is none")
	}
	if cfg.ShouldSyncForWeight("large") {
		t.Error("ShouldSyncForWeight should return false when strategy is none")
	}
}

func TestShouldSyncBeforePhase(t *testing.T) {
	tests := []struct {
		strategy SyncStrategy
		expected bool
	}{
		{SyncStrategyNone, false},
		{SyncStrategyPhase, true},
		{SyncStrategyCompletion, false},
		{SyncStrategyDetect, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			cfg := Default()
			cfg.Completion.Sync.Strategy = tt.strategy

			got := cfg.ShouldSyncBeforePhase()
			if got != tt.expected {
				t.Errorf("ShouldSyncBeforePhase() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestShouldSyncAtCompletion(t *testing.T) {
	tests := []struct {
		strategy SyncStrategy
		expected bool
	}{
		{SyncStrategyNone, false},
		{SyncStrategyPhase, false},
		{SyncStrategyCompletion, true},
		{SyncStrategyDetect, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			cfg := Default()
			cfg.Completion.Sync.Strategy = tt.strategy

			got := cfg.ShouldSyncAtCompletion()
			if got != tt.expected {
				t.Errorf("ShouldSyncAtCompletion() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestShouldDetectConflictsOnly(t *testing.T) {
	tests := []struct {
		strategy SyncStrategy
		expected bool
	}{
		{SyncStrategyNone, false},
		{SyncStrategyPhase, false},
		{SyncStrategyCompletion, false},
		{SyncStrategyDetect, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.strategy), func(t *testing.T) {
			cfg := Default()
			cfg.Completion.Sync.Strategy = tt.strategy

			got := cfg.ShouldDetectConflictsOnly()
			if got != tt.expected {
				t.Errorf("ShouldDetectConflictsOnly() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidate_InvalidSyncStrategy(t *testing.T) {
	cfg := Default()
	cfg.Completion.Sync.Strategy = "invalid"

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should fail for invalid sync strategy")
	}
	if err != nil && !contains([]string{"completion.sync.strategy"}, "completion.sync.strategy") {
		// Just check error is returned
		t.Logf("Got expected error: %v", err)
	}
}

func TestValidate_ValidSyncStrategies(t *testing.T) {
	strategies := []SyncStrategy{
		SyncStrategyNone,
		SyncStrategyPhase,
		SyncStrategyCompletion,
		SyncStrategyDetect,
		"", // empty should be valid
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			cfg := Default()
			cfg.Completion.Sync.Strategy = strategy

			// The config has worktree.enabled = true by default which is required
			// so we shouldn't get a validation error for sync strategy
			err := cfg.Validate()
			if err != nil {
				t.Errorf("Validate() should succeed for strategy %q, got: %v", strategy, err)
			}
		})
	}
}

func TestDefault_ExecutorMaxRetries(t *testing.T) {
	cfg := Default()

	// Default max retries should be 5
	if cfg.Execution.MaxRetries != 5 {
		t.Errorf("Execution.MaxRetries = %d, want 5", cfg.Execution.MaxRetries)
	}
}

func TestEffectiveMaxRetries(t *testing.T) {
	tests := []struct {
		name            string
		executorRetries int
		retryMaxRetries int
		expectedRetries int
	}{
		{
			name:            "executor.max_retries takes precedence",
			executorRetries: 3,
			retryMaxRetries: 2,
			expectedRetries: 3,
		},
		{
			name:            "falls back to retry.max_retries when executor is 0",
			executorRetries: 0,
			retryMaxRetries: 4,
			expectedRetries: 4,
		},
		{
			name:            "returns default 5 when both are 0",
			executorRetries: 0,
			retryMaxRetries: 0,
			expectedRetries: 5,
		},
		{
			name:            "executor.max_retries of 1 is used",
			executorRetries: 1,
			retryMaxRetries: 10,
			expectedRetries: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Execution: ExecutionConfig{
					MaxRetries: tt.executorRetries,
				},
				Retry: RetryConfig{
					MaxRetries: tt.retryMaxRetries,
				},
			}

			result := cfg.EffectiveMaxRetries()
			if result != tt.expectedRetries {
				t.Errorf("EffectiveMaxRetries() = %d, want %d", result, tt.expectedRetries)
			}
		})
	}
}

func TestDefault_RetryAndGatesMaxRetries(t *testing.T) {
	cfg := Default()

	// All retry-related defaults should be 5
	if cfg.Retry.MaxRetries != 5 {
		t.Errorf("Retry.MaxRetries = %d, want 5", cfg.Retry.MaxRetries)
	}
	if cfg.Gates.MaxRetries != 5 {
		t.Errorf("Gates.MaxRetries = %d, want 5", cfg.Gates.MaxRetries)
	}
}

func TestDefault_ArtifactSkipConfig(t *testing.T) {
	cfg := Default()

	// Enabled should be true by default
	if !cfg.ArtifactSkip.Enabled {
		t.Error("ArtifactSkip.Enabled should default to true")
	}

	// AutoSkip should be false by default (prompt user)
	if cfg.ArtifactSkip.AutoSkip {
		t.Error("ArtifactSkip.AutoSkip should default to false")
	}

	// Default phases to check
	expectedPhases := []string{"spec", "research", "docs"}
	if len(cfg.ArtifactSkip.Phases) != len(expectedPhases) {
		t.Errorf("ArtifactSkip.Phases = %v, want %v", cfg.ArtifactSkip.Phases, expectedPhases)
	}
	for i, want := range expectedPhases {
		if cfg.ArtifactSkip.Phases[i] != want {
			t.Errorf("ArtifactSkip.Phases[%d] = %s, want %s", i, cfg.ArtifactSkip.Phases[i], want)
		}
	}
}

func TestHasTasksDir(t *testing.T) {
	tmpDir := t.TempDir()

	// No .orc/tasks directory
	if hasTasksDir(tmpDir) {
		t.Error("hasTasksDir should return false when no tasks dir exists")
	}

	// Create .orc/tasks
	tasksDir := filepath.Join(tmpDir, OrcDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	// Now should return true
	if !hasTasksDir(tmpDir) {
		t.Error("hasTasksDir should return true when tasks dir exists")
	}
}

func TestFindProjectRoot_CurrentDir(t *testing.T) {
	// Save current dir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(origWd)

	// Create temp project with tasks
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// FindProjectRoot should return current dir
	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot() failed: %v", err)
	}
	if root != tmpDir {
		t.Errorf("FindProjectRoot() = %s, want %s", root, tmpDir)
	}
}

func TestFindProjectRoot_NotInitialized(t *testing.T) {
	// Save current dir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(origWd)

	// Create temp dir without .orc
	tmpDir := t.TempDir()

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// FindProjectRoot should fail
	_, err = FindProjectRoot()
	if err == nil {
		t.Error("FindProjectRoot() should fail when not in an orc project")
	}
}

func TestFindProjectRoot_FallbackToOrcDir(t *testing.T) {
	// Save current dir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(origWd)

	// Create temp project with .orc but no tasks dir (freshly initialized)
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, OrcDir)
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// FindProjectRoot should fallback to current dir (has .orc)
	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot() failed: %v", err)
	}
	if root != tmpDir {
		t.Errorf("FindProjectRoot() = %s, want %s", root, tmpDir)
	}
}

func TestFindProjectRoot_WalkUpDirectories(t *testing.T) {
	// Save current dir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer os.Chdir(origWd)

	// Create temp project structure:
	// tmpDir/.orc/tasks (project root)
	// tmpDir/subdir/subsubdir (current dir)
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}
	subsubdir := filepath.Join(tmpDir, "subdir", "subsubdir")
	if err := os.MkdirAll(subsubdir, 0755); err != nil {
		t.Fatalf("failed to create subsubdir: %v", err)
	}

	// Change to nested dir
	if err := os.Chdir(subsubdir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// FindProjectRoot should walk up and find tmpDir
	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot() failed: %v", err)
	}
	if root != tmpDir {
		t.Errorf("FindProjectRoot() = %s, want %s", root, tmpDir)
	}
}
