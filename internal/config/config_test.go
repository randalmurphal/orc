package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// normalizePath resolves symlinks to get canonical path for comparison.
// On macOS, /var is a symlink to /private/var, which causes path comparison
// issues between t.TempDir() and paths returned by functions that resolve symlinks.
func normalizePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If we can't resolve, return original (might not exist yet)
		return path
	}
	return resolved
}

// pathsEqual compares two paths after normalizing symlinks.
func pathsEqual(t *testing.T, got, want string) bool {
	t.Helper()
	return normalizePath(t, got) == normalizePath(t, want)
}

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

	// CoverageThreshold should default to 85%
	if cfg.Testing.CoverageThreshold != 85 {
		t.Errorf("Testing.CoverageThreshold = %d, want 85", cfg.Testing.CoverageThreshold)
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
	loaded, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile() failed: %v", err)
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
	if err := InitAt(tmpDir, false); err != nil {
		t.Fatalf("InitAt() failed: %v", err)
	}

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
	_ = InitAt(tmpDir, false)

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
				"test":   "implement",
				"review": "implement",
			},
		},
	}

	tests := []struct {
		phase    string
		wantFrom string
	}{
		{"test", "implement"},
		{"review", "implement"},
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

func TestDefaultConfigRetryMap(t *testing.T) {
	// Verify default config has the expected retry mappings
	// These mappings prevent infinite loops when phases fail
	cfg := Default()

	// Phases that should have retry mappings
	retryTests := []struct {
		phase    string
		wantFrom string
	}{
		{"test", "implement"},
		{"test_unit", "implement"},
		{"test_e2e", "implement"},
		{"review", "implement"}, // Critical: prevents review-resume loop
	}

	for _, tt := range retryTests {
		t.Run(tt.phase, func(t *testing.T) {
			from := cfg.ShouldRetryFrom(tt.phase)
			if from != tt.wantFrom {
				t.Errorf("Default().ShouldRetryFrom(%s) = %s, want %s", tt.phase, from, tt.wantFrom)
			}
		})
	}

	// Phases that should NOT have retry mappings (no upstream phase or retry not helpful)
	noRetryPhases := []string{"spec", "implement", "docs", "research"}
	for _, phase := range noRetryPhases {
		t.Run("no_retry_"+phase, func(t *testing.T) {
			from := cfg.ShouldRetryFrom(phase)
			if from != "" {
				t.Errorf("Default().ShouldRetryFrom(%s) = %s, want empty (no retry mapping)", phase, from)
			}
		})
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

func TestLoadFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Write invalid YAML
	invalidYAML := "invalid: yaml: content: [["
	err := os.WriteFile(tmpDir+"/invalid.yaml", []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("failed to write invalid yaml: %v", err)
	}

	_, err = LoadFile(tmpDir + "/invalid.yaml")
	if err == nil {
		t.Error("LoadFile() should fail with invalid YAML")
	}
}

func TestLoadFile_NonExistent(t *testing.T) {
	// LoadFile returns default config when file doesn't exist
	cfg, err := LoadFile("/nonexistent/path/config.yaml")
	if err != nil {
		t.Errorf("LoadFile() should not error with non-existent file: %v", err)
	}
	// Should return default config
	if cfg == nil {
		t.Fatal("LoadFile() should return default config for non-existent file")
	}
	if cfg.Version != 1 {
		t.Errorf("LoadFile() default config should have Version=1, got %d", cfg.Version)
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

func TestShouldSyncOnStart(t *testing.T) {
	tests := []struct {
		name        string
		strategy    SyncStrategy
		syncOnStart bool
		expected    bool
	}{
		// Default behavior: sync on start enabled
		{"completion+enabled", SyncStrategyCompletion, true, true},
		{"phase+enabled", SyncStrategyPhase, true, true},
		{"detect+enabled", SyncStrategyDetect, true, true},
		// Explicitly disabled
		{"completion+disabled", SyncStrategyCompletion, false, false},
		{"phase+disabled", SyncStrategyPhase, false, false},
		// Strategy none disables all sync including sync-on-start
		{"none+enabled", SyncStrategyNone, true, false},
		{"none+disabled", SyncStrategyNone, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Completion.Sync.Strategy = tt.strategy
			cfg.Completion.Sync.SyncOnStart = tt.syncOnStart

			got := cfg.ShouldSyncOnStart()
			if got != tt.expected {
				t.Errorf("ShouldSyncOnStart() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSyncOnStart_DefaultEnabled(t *testing.T) {
	// Verify that the default configuration has sync_on_start enabled
	cfg := Default()
	if !cfg.Completion.Sync.SyncOnStart {
		t.Error("Default config should have Completion.Sync.SyncOnStart = true")
	}
	if !cfg.ShouldSyncOnStart() {
		t.Error("Default config should return ShouldSyncOnStart() = true")
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
	defer func() { _ = os.Chdir(origWd) }()

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
	if !pathsEqual(t, root, tmpDir) {
		t.Errorf("FindProjectRoot() = %s, want %s", root, tmpDir)
	}
}

func TestFindProjectRoot_NotInitialized(t *testing.T) {
	// Save current dir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

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
	defer func() { _ = os.Chdir(origWd) }()

	// Create temp project with .orc and database (simulates freshly initialized project)
	// Note: After orc init, there's always a database file, so just having .orc/ is not enough
	// to be recognized as a project. This prevents worktrees with tracked .orc/ files from
	// being mistakenly identified as projects.
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, OrcDir)
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create a database file to simulate a real initialized project
	// (orc init always creates the database)
	dbPath := filepath.Join(orcDir, "orc.db")
	if err := os.WriteFile(dbPath, []byte("sqlite"), 0644); err != nil {
		t.Fatalf("failed to create db file: %v", err)
	}

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// FindProjectRoot should fallback to current dir (has .orc with database)
	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot() failed: %v", err)
	}
	if !pathsEqual(t, root, tmpDir) {
		t.Errorf("FindProjectRoot() = %s, want %s", root, tmpDir)
	}
}

func TestFindProjectRoot_WorktreePath(t *testing.T) {
	// Save current dir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// Create a structure that simulates a worktree:
	// tmpDir/.orc/orc.db (main project database)
	// tmpDir/.orc/worktrees/task-xxx/.orc/ (worktree with tracked files)
	tmpDir := t.TempDir()
	mainOrcDir := filepath.Join(tmpDir, OrcDir)
	if err := os.MkdirAll(mainOrcDir, 0755); err != nil {
		t.Fatalf("failed to create main .orc dir: %v", err)
	}

	// Create database in main project
	dbPath := filepath.Join(mainOrcDir, "orc.db")
	if err := os.WriteFile(dbPath, []byte("sqlite"), 0644); err != nil {
		t.Fatalf("failed to create db file: %v", err)
	}

	// Create worktree path with .orc/ (simulating tracked files from git checkout)
	worktreeDir := filepath.Join(tmpDir, OrcDir, "worktrees", "task-xxx")
	worktreeOrcDir := filepath.Join(worktreeDir, OrcDir)
	if err := os.MkdirAll(worktreeOrcDir, 0755); err != nil {
		t.Fatalf("failed to create worktree .orc dir: %v", err)
	}

	// Change to worktree dir
	if err := os.Chdir(worktreeDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// FindProjectRoot should detect we're in a worktree and return main project path
	root, err := FindProjectRoot()
	if err != nil {
		t.Fatalf("FindProjectRoot() failed: %v", err)
	}
	if !pathsEqual(t, root, tmpDir) {
		t.Errorf("FindProjectRoot() = %s, want %s (main project, not worktree)", root, tmpDir)
	}
}

func TestFindProjectRoot_WorktreeOrcDirOnly(t *testing.T) {
	// Save current dir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	// Create directory with .orc but no database or tasks (like a worktree with tracked files)
	// This should NOT be recognized as a project
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, OrcDir)
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	// FindProjectRoot should fail because .orc/ alone is not enough
	_, err = FindProjectRoot()
	if err == nil {
		t.Error("FindProjectRoot() should fail when only .orc/ exists without database or tasks")
	}
}

func TestFindProjectRoot_WalkUpDirectories(t *testing.T) {
	// Save current dir
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

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
	if !pathsEqual(t, root, tmpDir) {
		t.Errorf("FindProjectRoot() = %s, want %s", root, tmpDir)
	}
}

// Tests for FinalizeConfig

func TestDefault_FinalizeConfig(t *testing.T) {
	cfg := Default()

	// Enabled should be true by default
	if !cfg.Completion.Finalize.Enabled {
		t.Error("Completion.Finalize.Enabled should default to true")
	}

	// AutoTrigger should be true by default
	if !cfg.Completion.Finalize.AutoTrigger {
		t.Error("Completion.Finalize.AutoTrigger should default to true")
	}

	// Sync strategy should be merge by default
	if cfg.Completion.Finalize.Sync.Strategy != FinalizeSyncMerge {
		t.Errorf("Completion.Finalize.Sync.Strategy = %s, want merge",
			cfg.Completion.Finalize.Sync.Strategy)
	}

	// Conflict resolution should be enabled
	if !cfg.Completion.Finalize.ConflictResolution.Enabled {
		t.Error("Completion.Finalize.ConflictResolution.Enabled should default to true")
	}

	// Risk assessment should be enabled
	if !cfg.Completion.Finalize.RiskAssessment.Enabled {
		t.Error("Completion.Finalize.RiskAssessment.Enabled should default to true")
	}

	// Re-review threshold should be high
	if cfg.Completion.Finalize.RiskAssessment.ReReviewThreshold != "high" {
		t.Errorf("Completion.Finalize.RiskAssessment.ReReviewThreshold = %s, want high",
			cfg.Completion.Finalize.RiskAssessment.ReReviewThreshold)
	}

	// Pre-merge gate should be auto
	if cfg.Completion.Finalize.Gates.PreMerge != "auto" {
		t.Errorf("Completion.Finalize.Gates.PreMerge = %s, want auto",
			cfg.Completion.Finalize.Gates.PreMerge)
	}
}

func TestFinalizePresets_ProfileFast(t *testing.T) {
	finalize := FinalizePresets(ProfileFast)

	// Fast should use rebase strategy
	if finalize.Sync.Strategy != FinalizeSyncRebase {
		t.Errorf("FinalizePresets(fast).Sync.Strategy = %s, want rebase", finalize.Sync.Strategy)
	}

	// Fast should disable risk assessment
	if finalize.RiskAssessment.Enabled {
		t.Error("FinalizePresets(fast).RiskAssessment.Enabled should be false")
	}

	// Fast should have no pre-merge gate
	if finalize.Gates.PreMerge != "none" {
		t.Errorf("FinalizePresets(fast).Gates.PreMerge = %s, want none", finalize.Gates.PreMerge)
	}
}

func TestFinalizePresets_ProfileSafe(t *testing.T) {
	finalize := FinalizePresets(ProfileSafe)

	// Safe should use merge strategy
	if finalize.Sync.Strategy != FinalizeSyncMerge {
		t.Errorf("FinalizePresets(safe).Sync.Strategy = %s, want merge", finalize.Sync.Strategy)
	}

	// Safe should enable risk assessment
	if !finalize.RiskAssessment.Enabled {
		t.Error("FinalizePresets(safe).RiskAssessment.Enabled should be true")
	}

	// Safe should have lower re-review threshold
	if finalize.RiskAssessment.ReReviewThreshold != "medium" {
		t.Errorf("FinalizePresets(safe).RiskAssessment.ReReviewThreshold = %s, want medium",
			finalize.RiskAssessment.ReReviewThreshold)
	}

	// Safe should have human pre-merge gate
	if finalize.Gates.PreMerge != "human" {
		t.Errorf("FinalizePresets(safe).Gates.PreMerge = %s, want human", finalize.Gates.PreMerge)
	}
}

func TestFinalizePresets_ProfileStrict(t *testing.T) {
	finalize := FinalizePresets(ProfileStrict)

	// Strict should use merge strategy
	if finalize.Sync.Strategy != FinalizeSyncMerge {
		t.Errorf("FinalizePresets(strict).Sync.Strategy = %s, want merge", finalize.Sync.Strategy)
	}

	// Strict should have lowest re-review threshold
	if finalize.RiskAssessment.ReReviewThreshold != "low" {
		t.Errorf("FinalizePresets(strict).RiskAssessment.ReReviewThreshold = %s, want low",
			finalize.RiskAssessment.ReReviewThreshold)
	}

	// Strict should have human pre-merge gate
	if finalize.Gates.PreMerge != "human" {
		t.Errorf("FinalizePresets(strict).Gates.PreMerge = %s, want human", finalize.Gates.PreMerge)
	}
}

func TestApplyProfile_AffectsFinalize(t *testing.T) {
	cfg := Default()

	// Apply strict profile
	cfg.ApplyProfile(ProfileStrict)

	// Verify finalize changed
	if cfg.Completion.Finalize.Gates.PreMerge != "human" {
		t.Errorf("After ApplyProfile(strict), Finalize.Gates.PreMerge = %s, want human",
			cfg.Completion.Finalize.Gates.PreMerge)
	}

	// Apply fast profile
	cfg.ApplyProfile(ProfileFast)

	// Verify finalize changed to fast settings
	if cfg.Completion.Finalize.Sync.Strategy != FinalizeSyncRebase {
		t.Errorf("After ApplyProfile(fast), Finalize.Sync.Strategy = %s, want rebase",
			cfg.Completion.Finalize.Sync.Strategy)
	}
}

func TestShouldRunFinalize(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		weight   string
		expected bool
	}{
		{"enabled for large", true, "large", true},
		{"enabled for medium", true, "medium", true},
		{"enabled for small", true, "small", true},
		{"disabled for trivial", true, "trivial", false},
		{"disabled globally", false, "large", false},
		{"disabled globally for small", false, "small", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Completion.Finalize.Enabled = tt.enabled

			got := cfg.ShouldRunFinalize(tt.weight)
			if got != tt.expected {
				t.Errorf("ShouldRunFinalize(%s) = %v, want %v", tt.weight, got, tt.expected)
			}
		})
	}
}

func TestShouldAutoTriggerFinalize(t *testing.T) {
	tests := []struct {
		enabled     bool
		autoTrigger bool
		expected    bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			cfg := Default()
			cfg.Completion.Finalize.Enabled = tt.enabled
			cfg.Completion.Finalize.AutoTrigger = tt.autoTrigger

			got := cfg.ShouldAutoTriggerFinalize()
			if got != tt.expected {
				t.Errorf("ShouldAutoTriggerFinalize() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFinalizeUsesRebase(t *testing.T) {
	cfg := Default()

	// Default should be merge (not rebase)
	if cfg.FinalizeUsesRebase() {
		t.Error("Default should not use rebase")
	}

	// Switch to rebase
	cfg.Completion.Finalize.Sync.Strategy = FinalizeSyncRebase
	if !cfg.FinalizeUsesRebase() {
		t.Error("Should use rebase after setting strategy to rebase")
	}
}

func TestShouldReReview(t *testing.T) {
	tests := []struct {
		name      string
		enabled   bool
		threshold string
		riskLevel RiskLevel
		expected  bool
	}{
		{"low risk, high threshold", true, "high", RiskLow, false},
		{"medium risk, high threshold", true, "high", RiskMedium, false},
		{"high risk, high threshold", true, "high", RiskHigh, true},
		{"critical risk, high threshold", true, "high", RiskCritical, true},
		{"low risk, low threshold", true, "low", RiskLow, true},
		{"disabled assessment", false, "low", RiskCritical, false},
		{"medium risk, medium threshold", true, "medium", RiskMedium, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Completion.Finalize.RiskAssessment.Enabled = tt.enabled
			cfg.Completion.Finalize.RiskAssessment.ReReviewThreshold = tt.threshold

			got := cfg.ShouldReReview(tt.riskLevel)
			if got != tt.expected {
				t.Errorf("ShouldReReview(%s) = %v, want %v", tt.riskLevel, got, tt.expected)
			}
		})
	}
}

func TestParseRiskLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected RiskLevel
	}{
		{"low", RiskLow},
		{"LOW", RiskLow},
		{"Low", RiskLow},
		{"medium", RiskMedium},
		{"MEDIUM", RiskMedium},
		{"high", RiskHigh},
		{"HIGH", RiskHigh},
		{"critical", RiskCritical},
		{"CRITICAL", RiskCritical},
		{"unknown", RiskHigh}, // Defaults to high
		{"", RiskHigh},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseRiskLevel(tt.input)
			if got != tt.expected {
				t.Errorf("ParseRiskLevel(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRiskLevel_String(t *testing.T) {
	tests := []struct {
		level    RiskLevel
		expected string
	}{
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
		{RiskCritical, "critical"},
		{RiskLevel(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("RiskLevel(%d).String() = %s, want %s", tt.level, got, tt.expected)
			}
		})
	}
}

func TestGetPreMergeGateType(t *testing.T) {
	cfg := Default()

	// Default should be auto
	if cfg.GetPreMergeGateType() != "auto" {
		t.Errorf("GetPreMergeGateType() = %s, want auto", cfg.GetPreMergeGateType())
	}

	// Set to human
	cfg.Completion.Finalize.Gates.PreMerge = "human"
	if cfg.GetPreMergeGateType() != "human" {
		t.Errorf("GetPreMergeGateType() = %s, want human", cfg.GetPreMergeGateType())
	}

	// Empty should default to auto
	cfg.Completion.Finalize.Gates.PreMerge = ""
	if cfg.GetPreMergeGateType() != "auto" {
		t.Errorf("GetPreMergeGateType() = %s, want auto (from empty)", cfg.GetPreMergeGateType())
	}
}

func TestValidate_FinalizeConfig(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*Config)
		expectError bool
		errContains string
	}{
		{
			name:        "valid default config",
			setup:       func(c *Config) {},
			expectError: false,
		},
		{
			name: "invalid finalize sync strategy",
			setup: func(c *Config) {
				c.Completion.Finalize.Sync.Strategy = "invalid"
			},
			expectError: true,
			errContains: "completion.finalize.sync.strategy",
		},
		{
			name: "invalid risk threshold",
			setup: func(c *Config) {
				c.Completion.Finalize.RiskAssessment.ReReviewThreshold = "extreme"
			},
			expectError: true,
			errContains: "re_review_threshold",
		},
		{
			name: "invalid pre-merge gate",
			setup: func(c *Config) {
				c.Completion.Finalize.Gates.PreMerge = "robot"
			},
			expectError: true,
			errContains: "pre_merge",
		},
		{
			name: "valid rebase strategy",
			setup: func(c *Config) {
				c.Completion.Finalize.Sync.Strategy = FinalizeSyncRebase
			},
			expectError: false,
		},
		{
			name: "valid merge strategy",
			setup: func(c *Config) {
				c.Completion.Finalize.Sync.Strategy = FinalizeSyncMerge
			},
			expectError: false,
		},
		{
			name: "valid human gate",
			setup: func(c *Config) {
				c.Completion.Finalize.Gates.PreMerge = "human"
			},
			expectError: false,
		},
		{
			name: "invalid ai gate",
			setup: func(c *Config) {
				c.Completion.Finalize.Gates.PreMerge = "ai"
			},
			expectError: true,
			errContains: "pre_merge",
		},
		{
			name: "valid none gate",
			setup: func(c *Config) {
				c.Completion.Finalize.Gates.PreMerge = "none"
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.setup(cfg)

			err := cfg.Validate()
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestShouldResolveConflicts(t *testing.T) {
	cfg := Default()

	// Default should resolve conflicts
	if !cfg.ShouldResolveConflicts() {
		t.Error("Default should resolve conflicts")
	}

	// Disable
	cfg.Completion.Finalize.ConflictResolution.Enabled = false
	if cfg.ShouldResolveConflicts() {
		t.Error("Should not resolve conflicts when disabled")
	}
}

func TestGetConflictInstructions(t *testing.T) {
	cfg := Default()

	// Default should be empty
	if cfg.GetConflictInstructions() != "" {
		t.Errorf("GetConflictInstructions() = %q, want empty", cfg.GetConflictInstructions())
	}

	// Set instructions
	cfg.Completion.Finalize.ConflictResolution.Instructions = "Prefer newer code"
	if cfg.GetConflictInstructions() != "Prefer newer code" {
		t.Errorf("GetConflictInstructions() = %q, want %q",
			cfg.GetConflictInstructions(), "Prefer newer code")
	}
}

func TestShouldAssessRisk(t *testing.T) {
	cfg := Default()

	// Default should assess risk
	if !cfg.ShouldAssessRisk() {
		t.Error("Default should assess risk")
	}

	// Disable
	cfg.Completion.Finalize.RiskAssessment.Enabled = false
	if cfg.ShouldAssessRisk() {
		t.Error("Should not assess risk when disabled")
	}
}

func TestDefault_AutoTriggerOnApproval(t *testing.T) {
	cfg := Default()

	// Default should have auto-trigger on approval enabled
	if !cfg.Completion.Finalize.AutoTriggerOnApproval {
		t.Error("Completion.Finalize.AutoTriggerOnApproval should default to true")
	}
}

func TestShouldAutoTriggerFinalizeOnApproval(t *testing.T) {
	tests := []struct {
		enabled               bool
		autoTriggerOnApproval bool
		expected              bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			cfg := Default()
			cfg.Completion.Finalize.Enabled = tt.enabled
			cfg.Completion.Finalize.AutoTriggerOnApproval = tt.autoTriggerOnApproval

			got := cfg.ShouldAutoTriggerFinalizeOnApproval()
			if got != tt.expected {
				t.Errorf("ShouldAutoTriggerFinalizeOnApproval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFinalizePresets_AutoTriggerOnApproval(t *testing.T) {
	tests := []struct {
		profile  AutomationProfile
		expected bool
	}{
		{ProfileAuto, true},    // Auto profile enables auto-trigger on approval
		{ProfileFast, true},    // Fast profile enables auto-trigger on approval
		{ProfileSafe, false},   // Safe profile disables auto-trigger (human review)
		{ProfileStrict, false}, // Strict profile disables auto-trigger (human decision)
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			finalize := FinalizePresets(tt.profile)
			if finalize.AutoTriggerOnApproval != tt.expected {
				t.Errorf("FinalizePresets(%s).AutoTriggerOnApproval = %v, want %v",
					tt.profile, finalize.AutoTriggerOnApproval, tt.expected)
			}
		})
	}
}

func TestApplyProfile_AffectsAutoTriggerOnApproval(t *testing.T) {
	cfg := Default()

	// Apply safe profile - should disable auto-trigger on approval
	cfg.ApplyProfile(ProfileSafe)
	if cfg.Completion.Finalize.AutoTriggerOnApproval {
		t.Error("After ApplyProfile(safe), AutoTriggerOnApproval should be false")
	}

	// Apply auto profile - should enable auto-trigger on approval
	cfg.ApplyProfile(ProfileAuto)
	if !cfg.Completion.Finalize.AutoTriggerOnApproval {
		t.Error("After ApplyProfile(auto), AutoTriggerOnApproval should be true")
	}

	// Apply strict profile - should disable auto-trigger on approval
	cfg.ApplyProfile(ProfileStrict)
	if cfg.Completion.Finalize.AutoTriggerOnApproval {
		t.Error("After ApplyProfile(strict), AutoTriggerOnApproval should be false")
	}
}

func TestDefault_AutoApprovePR(t *testing.T) {
	cfg := Default()

	// Default should have auto-approve disabled (opt-in)
	if cfg.Completion.PR.AutoApprove {
		t.Error("Completion.PR.AutoApprove should default to false")
	}
}

func TestShouldAutoApprovePR(t *testing.T) {
	tests := []struct {
		name        string
		profile     AutomationProfile
		autoApprove bool
		expected    bool
	}{
		{"auto profile with auto-approve", ProfileAuto, true, true},
		{"auto profile without auto-approve", ProfileAuto, false, false},
		{"fast profile with auto-approve", ProfileFast, true, true},
		{"fast profile without auto-approve", ProfileFast, false, false},
		{"safe profile with auto-approve", ProfileSafe, true, false}, // Safe always returns false
		{"safe profile without auto-approve", ProfileSafe, false, false},
		{"strict profile with auto-approve", ProfileStrict, true, false}, // Strict always returns false
		{"strict profile without auto-approve", ProfileStrict, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Profile = tt.profile
			cfg.Completion.PR.AutoApprove = tt.autoApprove

			got := cfg.ShouldAutoApprovePR()
			if got != tt.expected {
				t.Errorf("ShouldAutoApprovePR() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPRAutoApprovePreset(t *testing.T) {
	tests := []struct {
		profile  AutomationProfile
		expected bool
	}{
		{ProfileAuto, false},   // Auto profile — auto-approve is opt-in
		{ProfileFast, false},   // Fast profile — auto-approve is opt-in
		{ProfileSafe, false},   // Safe profile — human review required
		{ProfileStrict, false}, // Strict profile — human decision required
	}

	for _, tt := range tests {
		t.Run(string(tt.profile), func(t *testing.T) {
			result := PRAutoApprovePreset(tt.profile)
			if result != tt.expected {
				t.Errorf("PRAutoApprovePreset(%s) = %v, want %v",
					tt.profile, result, tt.expected)
			}
		})
	}
}

func TestApplyProfile_AffectsAutoApprovePR(t *testing.T) {
	cfg := Default()

	// Apply safe profile - should disable auto-approve
	cfg.ApplyProfile(ProfileSafe)
	if cfg.Completion.PR.AutoApprove {
		t.Error("After ApplyProfile(safe), AutoApprove should be false")
	}

	// Apply auto profile - auto-approve is now opt-in, profile preset returns false
	cfg.ApplyProfile(ProfileAuto)
	if cfg.Completion.PR.AutoApprove {
		t.Error("After ApplyProfile(auto), AutoApprove should be false (opt-in)")
	}

	// Apply fast profile - auto-approve is now opt-in, profile preset returns false
	cfg.ApplyProfile(ProfileFast)
	if cfg.Completion.PR.AutoApprove {
		t.Error("After ApplyProfile(fast), AutoApprove should be false (opt-in)")
	}

	// Apply strict profile - should disable auto-approve
	cfg.ApplyProfile(ProfileStrict)
	if cfg.Completion.PR.AutoApprove {
		t.Error("After ApplyProfile(strict), AutoApprove should be false")
	}
}

// Tests for CI wait and merge config

func TestDefault_CIMergeConfig(t *testing.T) {
	cfg := Default()

	// WaitForCI should be false by default (opt-in)
	if cfg.Completion.WaitForCI {
		t.Error("Completion.WaitForCI should default to false")
	}

	// CITimeout should be 10 minutes by default
	if cfg.Completion.CITimeout != 10*time.Minute {
		t.Errorf("Completion.CITimeout = %v, want 10m", cfg.Completion.CITimeout)
	}

	// MergeOnCIPass should be false by default (opt-in)
	if cfg.Completion.MergeOnCIPass {
		t.Error("Completion.MergeOnCIPass should default to false")
	}
}

func TestShouldWaitForCI(t *testing.T) {
	tests := []struct {
		name      string
		profile   AutomationProfile
		waitForCI bool
		expected  bool
	}{
		{"auto profile with wait_for_ci", ProfileAuto, true, true},
		{"auto profile without wait_for_ci", ProfileAuto, false, false},
		{"fast profile with wait_for_ci", ProfileFast, true, true},
		{"fast profile without wait_for_ci", ProfileFast, false, false},
		{"safe profile with wait_for_ci", ProfileSafe, true, false}, // Safe always returns false
		{"safe profile without wait_for_ci", ProfileSafe, false, false},
		{"strict profile with wait_for_ci", ProfileStrict, true, false}, // Strict always returns false
		{"strict profile without wait_for_ci", ProfileStrict, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Profile = tt.profile
			cfg.Completion.CI.WaitForCI = tt.waitForCI

			got := cfg.ShouldWaitForCI()
			if got != tt.expected {
				t.Errorf("ShouldWaitForCI() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestShouldMergeOnCIPass(t *testing.T) {
	tests := []struct {
		name          string
		profile       AutomationProfile
		waitForCI     bool
		mergeOnCIPass bool
		expected      bool
	}{
		{"auto with both enabled", ProfileAuto, true, true, true},
		{"auto without wait_for_ci", ProfileAuto, false, true, false},
		{"auto without merge_on_ci_pass", ProfileAuto, true, false, false},
		{"auto with neither enabled", ProfileAuto, false, false, false},
		{"fast with both enabled", ProfileFast, true, true, true},
		{"safe with both enabled", ProfileSafe, true, true, false},     // Safe always returns false
		{"strict with both enabled", ProfileStrict, true, true, false}, // Strict always returns false
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Profile = tt.profile
			cfg.Completion.CI.WaitForCI = tt.waitForCI
			cfg.Completion.CI.MergeOnCIPass = tt.mergeOnCIPass

			got := cfg.ShouldMergeOnCIPass()
			if got != tt.expected {
				t.Errorf("ShouldMergeOnCIPass() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCITimeout(t *testing.T) {
	cfg := Default()

	// Default timeout
	timeout := cfg.CITimeout()
	if timeout != 10*time.Minute {
		t.Errorf("default timeout = %v, want 10m", timeout)
	}

	// Custom timeout
	cfg.Completion.CI.CITimeout = 5 * time.Minute
	timeout = cfg.CITimeout()
	if timeout != 5*time.Minute {
		t.Errorf("custom timeout = %v, want 5m", timeout)
	}

	// Zero timeout falls back to default
	cfg.Completion.CI.CITimeout = 0
	timeout = cfg.CITimeout()
	if timeout != 10*time.Minute {
		t.Errorf("zero timeout = %v, want 10m (default)", timeout)
	}
}

// Tests for DiagnosticsConfig

func TestDefault_DiagnosticsConfig(t *testing.T) {
	cfg := Default()

	// ResourceTracking should be enabled by default
	if !cfg.Diagnostics.ResourceTracking.Enabled {
		t.Error("Diagnostics.ResourceTracking.Enabled should default to true")
	}

	// MemoryThresholdMB should be 500 (not 100)
	if cfg.Diagnostics.ResourceTracking.MemoryThresholdMB != 500 {
		t.Errorf("Diagnostics.ResourceTracking.MemoryThresholdMB = %d, want 500",
			cfg.Diagnostics.ResourceTracking.MemoryThresholdMB)
	}

	// FilterSystemProcesses should be true by default
	if !cfg.Diagnostics.ResourceTracking.FilterSystemProcesses {
		t.Error("Diagnostics.ResourceTracking.FilterSystemProcesses should default to true")
	}
}

func TestDiagnosticsConfig_CustomThreshold(t *testing.T) {
	// Verify that custom threshold from config is respected (backward compatibility)
	cfg := Default()

	// Set a custom threshold
	cfg.Diagnostics.ResourceTracking.MemoryThresholdMB = 200

	// Verify it's preserved
	if cfg.Diagnostics.ResourceTracking.MemoryThresholdMB != 200 {
		t.Errorf("Custom MemoryThresholdMB = %d, want 200",
			cfg.Diagnostics.ResourceTracking.MemoryThresholdMB)
	}
}
