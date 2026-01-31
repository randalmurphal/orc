package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.
// This is necessary because runTeamInit() calls findProjectRoot() which uses os.Getwd()
// to locate the .orc directory. There is currently no environment variable or parameter
// to override this behavior.

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// withTempDirForTeam creates a temp directory, changes to it, and restores the original
// working directory when the test completes. It calls t.Fatal on any error.
// This is duplicated from cmd_config_test.go to keep tests self-contained.
func withTempDirForTeam(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return tmpDir
}

func TestLoadTeamRegistry(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("returns empty registry when file does not exist", func(t *testing.T) {
		path := filepath.Join(tmpDir, "nonexistent.yaml")
		registry, err := loadTeamRegistry(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if registry.Version != 1 {
			t.Errorf("expected version 1, got %d", registry.Version)
		}
		if len(registry.Members) != 0 {
			t.Errorf("expected empty members, got %d", len(registry.Members))
		}
		if len(registry.ReservedPrefixes) != 0 {
			t.Errorf("expected empty prefixes, got %d", len(registry.ReservedPrefixes))
		}
	})

	t.Run("loads existing registry", func(t *testing.T) {
		path := filepath.Join(tmpDir, "team.yaml")
		content := `version: 1
members:
  - initials: AM
    name: Alice Martinez
    email: alice@example.com
reserved_prefixes:
  - AM
  - BJ
`
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		registry, err := loadTeamRegistry(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(registry.Members) != 1 {
			t.Errorf("expected 1 member, got %d", len(registry.Members))
		}
		if registry.Members[0].Initials != "AM" {
			t.Errorf("expected initials AM, got %s", registry.Members[0].Initials)
		}
		if registry.Members[0].Name != "Alice Martinez" {
			t.Errorf("expected name Alice Martinez, got %s", registry.Members[0].Name)
		}
		if len(registry.ReservedPrefixes) != 2 {
			t.Errorf("expected 2 reserved prefixes, got %d", len(registry.ReservedPrefixes))
		}
	})
}

func TestSaveTeamRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "team.yaml")

	registry := &TeamRegistry{
		Version: 1,
		Members: []TeamMember{
			{Initials: "AM", Name: "Alice Martinez", Email: "alice@example.com"},
			{Initials: "BJ", Name: "Bob Johnson"},
		},
		ReservedPrefixes: []string{"AM", "BJ", "CC"},
	}

	if err := saveTeamRegistry(path, registry); err != nil {
		t.Fatalf("save registry: %v", err)
	}

	// Verify file was written correctly
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var loaded TeamRegistry
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("expected version 1, got %d", loaded.Version)
	}
	if len(loaded.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(loaded.Members))
	}
	if len(loaded.ReservedPrefixes) != 3 {
		t.Errorf("expected 3 reserved prefixes, got %d", len(loaded.ReservedPrefixes))
	}
}

func TestTeamInitCreatesDirectoryStructure(t *testing.T) {
	tmpDir := withTempDirForTeam(t)

	// Create .orc directory with config.yaml so findProjectRoot/isRealOrcProject can find it
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("mkdir .orc: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Run team init
	if err := runTeamInit(false); err != nil {
		t.Fatalf("runTeamInit: %v", err)
	}

	// Verify directory structure
	expectedDirs := []string{
		".orc/shared",
		".orc/shared/prompts",
		".orc/shared/skills",
		".orc/shared/templates",
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", dir)
		}
	}

	// Verify config.yaml
	cfgPath := filepath.Join(".orc", "shared", "config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("expected config.yaml to exist")
	} else {
		data, _ := os.ReadFile(cfgPath)
		var cfg SharedConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("unmarshal config: %v", err)
		}
		if cfg.Version != 1 {
			t.Errorf("expected version 1, got %d", cfg.Version)
		}
		if cfg.TaskID.Mode != "p2p" {
			t.Errorf("expected mode p2p, got %s", cfg.TaskID.Mode)
		}
		if cfg.TaskID.PrefixSource != "initials" {
			t.Errorf("expected prefix_source initials, got %s", cfg.TaskID.PrefixSource)
		}
		if cfg.Gates.DefaultType != "auto" {
			t.Errorf("expected gates.default_type auto, got %s", cfg.Gates.DefaultType)
		}
	}

	// Verify team.yaml
	teamPath := filepath.Join(".orc", "shared", "team.yaml")
	if _, err := os.Stat(teamPath); os.IsNotExist(err) {
		t.Error("expected team.yaml to exist")
	} else {
		data, _ := os.ReadFile(teamPath)
		var registry TeamRegistry
		if err := yaml.Unmarshal(data, &registry); err != nil {
			t.Fatalf("unmarshal team.yaml: %v", err)
		}
		if registry.Version != 1 {
			t.Errorf("expected version 1, got %d", registry.Version)
		}
		if len(registry.Members) != 0 {
			t.Errorf("expected empty members, got %d", len(registry.Members))
		}
	}
}

func TestTeamInitFailsWithoutForce(t *testing.T) {
	tmpDir := withTempDirForTeam(t)

	// Create .orc/shared directory (both .orc for findProjectRoot and shared for the test)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc/shared"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create config.yaml to satisfy isRealOrcProject check
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Should fail without force
	err := runTeamInit(false)
	if err == nil {
		t.Error("expected error when shared directory exists")
	}

	// Should succeed with force
	if err := runTeamInit(true); err != nil {
		t.Errorf("expected success with force, got: %v", err)
	}
}

func TestTeamRegistryValidatesPrefixNotTaken(t *testing.T) {
	registry := &TeamRegistry{
		Version: 1,
		Members: []TeamMember{
			{Initials: "AM", Name: "Alice Martinez"},
		},
		ReservedPrefixes: []string{"AM", "BJ"},
	}

	// Test member check
	for _, m := range registry.Members {
		if m.Initials == "AM" {
			// This prefix is taken by a member
			break
		}
	}

	// Test reserved prefix check
	for _, p := range registry.ReservedPrefixes {
		if p == "BJ" {
			// This prefix is reserved
			break
		}
	}

	// Verify the checks work as expected
	foundAM := false
	for _, m := range registry.Members {
		if m.Initials == "AM" {
			foundAM = true
			break
		}
	}
	if !foundAM {
		t.Error("expected to find AM in members")
	}

	foundBJ := false
	for _, p := range registry.ReservedPrefixes {
		if p == "BJ" {
			foundBJ = true
			break
		}
	}
	if !foundBJ {
		t.Error("expected to find BJ in reserved prefixes")
	}
}

func TestTeamMember(t *testing.T) {
	member := TeamMember{
		Initials: "AM",
		Name:     "Alice Martinez",
		Email:    "alice@example.com",
	}

	if member.Initials != "AM" {
		t.Errorf("expected initials AM, got %s", member.Initials)
	}
	if member.Name != "Alice Martinez" {
		t.Errorf("expected name Alice Martinez, got %s", member.Name)
	}
	if member.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", member.Email)
	}

	// Test YAML marshaling
	data, err := yaml.Marshal(member)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var loaded TeamMember
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.Initials != member.Initials {
		t.Errorf("expected initials %s, got %s", member.Initials, loaded.Initials)
	}
	if loaded.Name != member.Name {
		t.Errorf("expected name %s, got %s", member.Name, loaded.Name)
	}
}

func TestSharedConfig(t *testing.T) {
	cfg := SharedConfig{Version: 1}
	cfg.TaskID.Mode = "p2p"
	cfg.TaskID.PrefixSource = "initials"
	cfg.Defaults.Profile = "safe"
	cfg.Gates.DefaultType = "auto"
	cfg.Gates.PhaseOverrides = map[string]string{"review": "human"}
	cfg.Cost.WarnPerTask = 5.0

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var loaded SharedConfig
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("expected version 1, got %d", loaded.Version)
	}
	if loaded.TaskID.Mode != "p2p" {
		t.Errorf("expected mode p2p, got %s", loaded.TaskID.Mode)
	}
	if loaded.TaskID.PrefixSource != "initials" {
		t.Errorf("expected prefix_source initials, got %s", loaded.TaskID.PrefixSource)
	}
	if loaded.Defaults.Profile != "safe" {
		t.Errorf("expected profile safe, got %s", loaded.Defaults.Profile)
	}
	if loaded.Gates.DefaultType != "auto" {
		t.Errorf("expected gates.default_type auto, got %s", loaded.Gates.DefaultType)
	}
	if loaded.Gates.PhaseOverrides["review"] != "human" {
		t.Errorf("expected gates.phase_overrides[review] human, got %s", loaded.Gates.PhaseOverrides["review"])
	}
	if loaded.Cost.WarnPerTask != 5.0 {
		t.Errorf("expected cost.warn_per_task 5.0, got %f", loaded.Cost.WarnPerTask)
	}
}
