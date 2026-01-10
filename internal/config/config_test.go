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
		t.Error("LoadFrom() should return default config for non-existent file")
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
