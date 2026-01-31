package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeSolo, "solo"},
		{ModeP2P, "p2p"},
		{ModeTeam, "team"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("Mode.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestMode_IsValid(t *testing.T) {
	tests := []struct {
		mode Mode
		want bool
	}{
		{ModeSolo, true},
		{ModeP2P, true},
		{ModeTeam, true},
		{Mode("invalid"), false},
		{Mode(""), false},
	}

	for _, tt := range tests {
		if got := tt.mode.IsValid(); got != tt.want {
			t.Errorf("Mode(%q).IsValid() = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestMode_Helpers(t *testing.T) {
	t.Run("IsSolo", func(t *testing.T) {
		if !ModeSolo.IsSolo() {
			t.Error("ModeSolo.IsSolo() should be true")
		}
		if ModeP2P.IsSolo() {
			t.Error("ModeP2P.IsSolo() should be false")
		}
	})

	t.Run("IsP2P", func(t *testing.T) {
		if !ModeP2P.IsP2P() {
			t.Error("ModeP2P.IsP2P() should be true")
		}
		if ModeSolo.IsP2P() {
			t.Error("ModeSolo.IsP2P() should be false")
		}
	})

	t.Run("IsTeam", func(t *testing.T) {
		if !ModeTeam.IsTeam() {
			t.Error("ModeTeam.IsTeam() should be true")
		}
		if ModeSolo.IsTeam() {
			t.Error("ModeSolo.IsTeam() should be false")
		}
	})

	t.Run("RequiresLocking", func(t *testing.T) {
		if ModeSolo.RequiresLocking() {
			t.Error("ModeSolo should not require locking")
		}
		if !ModeP2P.RequiresLocking() {
			t.Error("ModeP2P should require locking")
		}
		if !ModeTeam.RequiresLocking() {
			t.Error("ModeTeam should require locking")
		}
	})

	t.Run("RequiresPrefixedIDs", func(t *testing.T) {
		if ModeSolo.RequiresPrefixedIDs() {
			t.Error("ModeSolo should not require prefixed IDs")
		}
		if !ModeP2P.RequiresPrefixedIDs() {
			t.Error("ModeP2P should require prefixed IDs")
		}
		if !ModeTeam.RequiresPrefixedIDs() {
			t.Error("ModeTeam should require prefixed IDs")
		}
	})
}

func TestDetectMode_Solo(t *testing.T) {
	// Create a temporary project directory with no shared dir or team config
	tmpDir := t.TempDir()

	// Create .orc directory but no shared directory
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatal(err)
	}

	mode := DetectMode(tmpDir)
	if mode != ModeSolo {
		t.Errorf("DetectMode() = %v, want %v", mode, ModeSolo)
	}
}

func TestDetectMode_SharedDirNoLongerTriggersP2P(t *testing.T) {
	// P2P auto-detection via .orc/shared/ was removed.
	// Having .orc/shared/ should now result in solo mode.
	tmpDir := t.TempDir()

	// Create .orc/shared directory
	sharedDir := filepath.Join(tmpDir, ".orc", "shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}

	mode := DetectMode(tmpDir)
	if mode != ModeSolo {
		t.Errorf("DetectMode() = %v, want %v", mode, ModeSolo)
	}
}

func TestDetectMode_Team(t *testing.T) {
	// Create a temporary project directory with team.server_url configured
	tmpDir := t.TempDir()

	// Create .orc directory with config containing team.server_url
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(orcDir, "config.yaml")
	configContent := `version: 1
team:
  server_url: https://orc.example.com
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	mode := DetectMode(tmpDir)
	if mode != ModeTeam {
		t.Errorf("DetectMode() = %v, want %v", mode, ModeTeam)
	}
}

func TestDetectMode_TeamWithSharedDir(t *testing.T) {
	// Team mode should be detected even with .orc/shared/ present
	tmpDir := t.TempDir()

	// Create both .orc/shared/ AND team.server_url config
	sharedDir := filepath.Join(tmpDir, ".orc", "shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tmpDir, ".orc", "config.yaml")
	configContent := `version: 1
team:
  server_url: https://orc.example.com
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	mode := DetectMode(tmpDir)
	if mode != ModeTeam {
		t.Errorf("DetectMode() with shared dir and team config = %v, want %v", mode, ModeTeam)
	}
}

func TestDetectMode_EmptyServerURL(t *testing.T) {
	// Empty server_url should not trigger team mode
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(orcDir, "config.yaml")
	configContent := `version: 1
team:
  server_url: ""
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	mode := DetectMode(tmpDir)
	if mode != ModeSolo {
		t.Errorf("DetectMode() with empty server_url = %v, want %v", mode, ModeSolo)
	}
}

func TestDetectMode_UserConfig(t *testing.T) {
	// Test that team.server_url in user config is detected
	tmpDir := t.TempDir()
	homeDir := t.TempDir()

	// Override home directory for this test
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", homeDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Create user config with team.server_url
	userOrcDir := filepath.Join(homeDir, ".orc")
	if err := os.MkdirAll(userOrcDir, 0755); err != nil {
		t.Fatal(err)
	}

	userConfigPath := filepath.Join(userOrcDir, "config.yaml")
	userConfigContent := `version: 1
team:
  server_url: https://orc.company.com
`
	if err := os.WriteFile(userConfigPath, []byte(userConfigContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create empty project .orc directory
	projectOrcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(projectOrcDir, 0755); err != nil {
		t.Fatal(err)
	}

	mode := DetectMode(tmpDir)
	if mode != ModeTeam {
		t.Errorf("DetectMode() with user config team.server_url = %v, want %v", mode, ModeTeam)
	}
}

func TestDetectMode_NoOrcDir(t *testing.T) {
	// Test detection on a directory without .orc/
	tmpDir := t.TempDir()

	mode := DetectMode(tmpDir)
	if mode != ModeSolo {
		t.Errorf("DetectMode() on non-orc project = %v, want %v", mode, ModeSolo)
	}
}

func TestDetectMode_InvalidYAML(t *testing.T) {
	// Test that invalid YAML doesn't cause a crash
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(orcDir, "config.yaml")
	invalidContent := `not: valid: yaml: [[[`
	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Should not panic, should default to solo
	mode := DetectMode(tmpDir)
	if mode != ModeSolo {
		t.Errorf("DetectMode() with invalid YAML = %v, want %v", mode, ModeSolo)
	}
}

func TestReadTeamServerURL(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantURL     string
		shouldExist bool
	}{
		{
			name: "with server_url",
			content: `team:
  server_url: https://example.com`,
			wantURL:     "https://example.com",
			shouldExist: true,
		},
		{
			name: "empty server_url",
			content: `team:
  server_url: ""`,
			wantURL:     "",
			shouldExist: true,
		},
		{
			name: "no team section",
			content: `version: 1
profile: auto`,
			wantURL:     "",
			shouldExist: true,
		},
		{
			name:        "file not exists",
			content:     "",
			wantURL:     "",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "config.yaml")

			if tt.shouldExist {
				if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			got := readTeamServerURL(path)
			if got != tt.wantURL {
				t.Errorf("readTeamServerURL() = %q, want %q", got, tt.wantURL)
			}
		})
	}
}
