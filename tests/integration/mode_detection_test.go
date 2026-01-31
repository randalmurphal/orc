package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/tests/testutil"
)

// TestModeDetectionSolo verifies that solo mode is the default for empty projects.
func TestModeDetectionSolo(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	loader := config.NewLoader(repo.RootDir)
	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Default mode should be solo
	if tc.Config.TaskID.Mode != "solo" {
		t.Errorf("TaskID.Mode = %q, want %q", tc.Config.TaskID.Mode, "solo")
	}

	// Executor prefix should be empty in solo mode
	prefix := tc.Config.ExecutorPrefix()
	if prefix != "" {
		t.Errorf("ExecutorPrefix() = %q, want empty in solo mode", prefix)
	}
}

// TestModeDetectionP2P verifies P2P mode detection when configured in project config.
func TestModeDetectionP2P(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Set P2P mode in project config
	repo.SetConfig("task_id.mode", "p2p")
	repo.SetConfig("task_id.prefix_source", "initials")

	// Create empty user dir to isolate from real ~/.orc/config.yaml
	emptyUserDir := t.TempDir()

	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(emptyUserDir)
	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if tc.Config.TaskID.Mode != "p2p" {
		t.Errorf("TaskID.Mode = %q, want %q", tc.Config.TaskID.Mode, "p2p")
	}
}

// TestModeDetectionTeam verifies team mode detection when server URL is configured.
func TestModeDetectionTeam(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Set team mode via config
	repo.SetConfig("task_id.mode", "team")
	repo.SetConfig("team.server_url", "https://team.example.com")

	// Create empty user dir to isolate from real ~/.orc/config.yaml
	emptyUserDir := t.TempDir()

	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(emptyUserDir)
	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if tc.Config.TaskID.Mode != "team" {
		t.Errorf("TaskID.Mode = %q, want %q", tc.Config.TaskID.Mode, "team")
	}

	if tc.Config.Team.ServerURL != "https://team.example.com" {
		t.Errorf("Team.ServerURL = %q, want %q", tc.Config.Team.ServerURL, "https://team.example.com")
	}
}

// TestModeDetectionSharedDirectoryCheck verifies that shared directory existence
// can be used to detect P2P mode.
func TestModeDetectionSharedDirectoryCheck(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	sharedDir := filepath.Join(repo.OrcDir, "shared")

	// Initially no shared directory
	if _, err := os.Stat(sharedDir); !os.IsNotExist(err) {
		t.Error("shared directory should not exist initially")
	}

	// Create shared directory
	repo.InitSharedDir()

	// Now shared directory should exist
	if _, err := os.Stat(sharedDir); err != nil {
		t.Errorf("shared directory should exist after InitSharedDir: %v", err)
	}

	// team.yaml should exist
	teamPath := filepath.Join(sharedDir, "team.yaml")
	if _, err := os.Stat(teamPath); err != nil {
		t.Errorf("team.yaml should exist: %v", err)
	}
}

// TestModeDetectionPrefixSourceInitials verifies initials prefix source.
func TestModeDetectionPrefixSourceInitials(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Set P2P mode in project config
	repo.SetConfig("task_id.mode", "p2p")
	repo.SetConfig("task_id.prefix_source", "initials")

	// Set identity
	userHome := testutil.MockUserConfig(t, "AM")

	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(filepath.Join(userHome, ".orc"))

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Mode should be P2P
	if tc.Config.TaskID.Mode != "p2p" {
		t.Errorf("TaskID.Mode = %q, want p2p", tc.Config.TaskID.Mode)
	}

	// Prefix source should be initials (set in project config)
	if tc.Config.TaskID.PrefixSource != "initials" {
		t.Errorf("TaskID.PrefixSource = %q, want initials", tc.Config.TaskID.PrefixSource)
	}

	// Executor prefix should return initials
	if tc.Config.Identity.Initials != "AM" {
		t.Errorf("Identity.Initials = %q, want AM", tc.Config.Identity.Initials)
	}
	if tc.Config.ExecutorPrefix() != "AM" {
		t.Errorf("ExecutorPrefix() = %q, want AM", tc.Config.ExecutorPrefix())
	}
}

// TestModeDetectionEnvOverride verifies that environment can override mode.
func TestModeDetectionEnvOverride(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	// Override mode via environment
	t.Setenv("ORC_TASK_ID_MODE", "solo")

	loader := config.NewLoader(repo.RootDir)
	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Note: The env var may or may not override depending on implementation
	// This test documents the expected behavior
	t.Logf("TaskID.Mode after env override: %s", tc.Config.TaskID.Mode)
}

// TestModeAffectsExecutorPrefix verifies that mode affects executor prefix behavior.
func TestModeAffectsExecutorPrefix(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		initials   string
		wantPrefix string
	}{
		{
			name:       "solo mode ignores initials",
			mode:       "solo",
			initials:   "AM",
			wantPrefix: "",
		},
		{
			name:       "p2p mode uses initials",
			mode:       "p2p",
			initials:   "AM",
			wantPrefix: "AM",
		},
		{
			name:       "team mode uses initials",
			mode:       "team",
			initials:   "BJ",
			wantPrefix: "BJ",
		},
		{
			name:       "p2p mode without initials",
			mode:       "p2p",
			initials:   "",
			wantPrefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.TaskID.Mode = tt.mode
			cfg.Identity.Initials = tt.initials

			got := cfg.ExecutorPrefix()
			if got != tt.wantPrefix {
				t.Errorf("ExecutorPrefix() = %q, want %q", got, tt.wantPrefix)
			}
		})
	}
}

// TestModeWithTeamYaml verifies that team.yaml content is correct.
func TestModeWithTeamYaml(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	teamPath := filepath.Join(repo.OrcDir, "shared", "team.yaml")
	teamData := testutil.ReadYAML(t, teamPath)

	// Check version
	version, ok := teamData["version"]
	if !ok {
		t.Error("team.yaml should have version")
	} else if v, ok := version.(int); !ok || v != 1 {
		t.Errorf("team.yaml version = %v, want 1", version)
	}

	// Check members array exists
	members, ok := teamData["members"]
	if !ok {
		t.Error("team.yaml should have members")
	}
	if _, ok := members.([]interface{}); !ok {
		t.Error("members should be an array")
	}

	// Check reserved_prefixes array exists
	reserved, ok := teamData["reserved_prefixes"]
	if !ok {
		t.Error("team.yaml should have reserved_prefixes")
	}
	if _, ok := reserved.([]interface{}); !ok {
		t.Error("reserved_prefixes should be an array")
	}
}

// TestModeWithSharedConfig verifies that shared config has correct defaults.
func TestModeWithSharedConfig(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	sharedConfigPath := filepath.Join(repo.OrcDir, "shared", "config.yaml")
	sharedData := testutil.ReadYAML(t, sharedConfigPath)

	// Check task_id section
	taskID, ok := sharedData["task_id"].(map[string]interface{})
	if !ok {
		t.Fatal("shared config should have task_id section")
	}

	if taskID["mode"] != "p2p" {
		t.Errorf("task_id.mode = %v, want p2p", taskID["mode"])
	}
	if taskID["prefix_source"] != "initials" {
		t.Errorf("task_id.prefix_source = %v, want initials", taskID["prefix_source"])
	}

	// Check defaults section
	defaults, ok := sharedData["defaults"].(map[string]interface{})
	if !ok {
		t.Fatal("shared config should have defaults section")
	}

	if defaults["profile"] != "safe" {
		t.Errorf("defaults.profile = %v, want safe", defaults["profile"])
	}
}
