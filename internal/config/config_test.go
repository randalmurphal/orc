package config

import (
	"os"
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

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create config directory
	os.MkdirAll(tmpDir+"/.orc", 0755)

	// Create and save config
	cfg := Default()
	cfg.Model = "test-model"
	cfg.MaxIterations = 50
	cfg.Timeout = 15 * time.Minute

	err := cfg.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load config
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
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

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Init should succeed
	err := Init(false)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	// Verify .orc directory exists
	if _, err := os.Stat(OrcDir); os.IsNotExist(err) {
		t.Error(".orc directory was not created")
	}

	// Verify tasks directory exists
	if _, err := os.Stat(OrcDir + "/tasks"); os.IsNotExist(err) {
		t.Error(".orc/tasks directory was not created")
	}

	// Init again should fail without force
	err = Init(false)
	if err == nil {
		t.Error("Init() should fail when already initialized")
	}

	// Init with force should succeed
	err = Init(true)
	if err != nil {
		t.Fatalf("Init() with force failed: %v", err)
	}
}

func TestIsInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Not initialized
	if IsInitialized() {
		t.Error("IsInitialized() = true before init")
	}

	// Initialize
	Init(false)

	// Now initialized
	if !IsInitialized() {
		t.Error("IsInitialized() = false after init")
	}
}

func TestRequireInit(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Should error before init
	err := RequireInit()
	if err == nil {
		t.Error("RequireInit() should error when not initialized")
	}

	// Initialize
	Init(false)

	// Should succeed after init
	err = RequireInit()
	if err != nil {
		t.Errorf("RequireInit() failed after init: %v", err)
	}
}

func TestResolveGateType(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *Config
		phase      string
		wantGate   GateType
		wantAuto   bool
	}{
		{
			name: "default auto gates",
			cfg: &Config{
				Gates: GateConfig{
					DefaultGate: "auto",
					PhaseGates:  map[string]GateType{},
				},
			},
			phase:    "implement",
			wantGate: GateAuto,
			wantAuto: true,
		},
		{
			name: "human gate for phase",
			cfg: &Config{
				Gates: GateConfig{
					DefaultGate: "auto",
					PhaseGates: map[string]GateType{
						"review": GateHuman,
					},
				},
			},
			phase:    "review",
			wantGate: GateHuman,
			wantAuto: false,
		},
		{
			name: "fallback to default",
			cfg: &Config{
				Gates: GateConfig{
					DefaultGate: "ai_review",
					PhaseGates:  map[string]GateType{},
				},
			},
			phase:    "test",
			wantGate: GateAIReview,
			wantAuto: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateType, isAuto := tt.cfg.ResolveGateType(tt.phase)
			if gateType != tt.wantGate {
				t.Errorf("ResolveGateType() gate = %v, want %v", gateType, tt.wantGate)
			}
			if isAuto != tt.wantAuto {
				t.Errorf("ResolveGateType() isAuto = %v, want %v", isAuto, tt.wantAuto)
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
		phase      string
		wantFrom   string
		wantRetry  bool
	}{
		{"test", "implement", true},
		{"validate", "implement", true},
		{"implement", "", false},
		{"spec", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			from, shouldRetry := cfg.ShouldRetryFrom(tt.phase)
			if from != tt.wantFrom {
				t.Errorf("ShouldRetryFrom(%s) from = %s, want %s", tt.phase, from, tt.wantFrom)
			}
			if shouldRetry != tt.wantRetry {
				t.Errorf("ShouldRetryFrom(%s) shouldRetry = %v, want %v", tt.phase, shouldRetry, tt.wantRetry)
			}
		})
	}

	// Test with disabled retry
	cfgDisabled := &Config{
		Retry: RetryConfig{
			Enabled: false,
		},
	}
	from, shouldRetry := cfgDisabled.ShouldRetryFrom("test")
	if shouldRetry {
		t.Error("ShouldRetryFrom() should return false when retry is disabled")
	}
	if from != "" {
		t.Errorf("ShouldRetryFrom() from = %s, want empty", from)
	}
}

func TestProfilePresets(t *testing.T) {
	presets := ProfilePresets()

	// Check that all expected profiles exist
	expectedProfiles := []string{"auto", "fast", "safe", "strict"}
	for _, profile := range expectedProfiles {
		if _, ok := presets[profile]; !ok {
			t.Errorf("ProfilePresets() missing profile: %s", profile)
		}
	}

	// Check specific preset values
	auto := presets["auto"]
	if auto.Gates.DefaultGate != "auto" {
		t.Errorf("auto profile default gate = %v, want auto", auto.Gates.DefaultGate)
	}

	strict := presets["strict"]
	if strict.Gates.DefaultGate != "human" {
		t.Errorf("strict profile default gate = %v, want human", strict.Gates.DefaultGate)
	}
}

func TestApplyProfile(t *testing.T) {
	cfg := Default()

	// Apply strict profile
	err := cfg.ApplyProfile("strict")
	if err != nil {
		t.Fatalf("ApplyProfile(strict) failed: %v", err)
	}

	// Verify gates changed
	if cfg.Gates.DefaultGate != "human" {
		t.Errorf("After ApplyProfile(strict), DefaultGate = %v, want human", cfg.Gates.DefaultGate)
	}

	// Apply unknown profile should fail
	err = cfg.ApplyProfile("nonexistent")
	if err == nil {
		t.Error("ApplyProfile(nonexistent) should fail")
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
	_, err := LoadFrom("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("LoadFrom() should fail with non-existent file")
	}
}

func TestSaveTo_InvalidPath(t *testing.T) {
	cfg := Default()
	err := cfg.SaveTo("/nonexistent/directory/config.yaml")
	if err == nil {
		t.Error("SaveTo() should fail with invalid path")
	}
}
