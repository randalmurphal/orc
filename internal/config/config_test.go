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
