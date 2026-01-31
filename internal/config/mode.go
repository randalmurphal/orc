package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Mode represents the operational mode for orc.
type Mode string

const (
	// ModeSolo is the default single-user mode with no coordination overhead.
	ModeSolo Mode = "solo"

	// ModeP2P enables peer-to-peer coordination using git and file-based locks.
	// Activated when .orc/shared/ directory exists.
	ModeP2P Mode = "p2p"

	// ModeTeam enables server-based coordination with real-time sync.
	// Activated when team.server_url is configured.
	ModeTeam Mode = "team"
)

// String returns the string representation of the mode.
func (m Mode) String() string {
	return string(m)
}

// IsValid returns true if the mode is a recognized value.
func (m Mode) IsValid() bool {
	switch m {
	case ModeSolo, ModeP2P, ModeTeam:
		return true
	default:
		return false
	}
}

// IsSolo returns true if mode is solo.
func (m Mode) IsSolo() bool {
	return m == ModeSolo
}

// IsP2P returns true if mode is p2p.
func (m Mode) IsP2P() bool {
	return m == ModeP2P
}

// IsTeam returns true if mode is team.
func (m Mode) IsTeam() bool {
	return m == ModeTeam
}

// RequiresLocking returns true if the mode requires task locking.
func (m Mode) RequiresLocking() bool {
	return m == ModeP2P || m == ModeTeam
}

// RequiresPrefixedIDs returns true if the mode requires prefixed task IDs.
func (m Mode) RequiresPrefixedIDs() bool {
	return m == ModeP2P || m == ModeTeam
}

// teamConfigRaw is used for reading team.server_url from config files.
type teamConfigRaw struct {
	Team struct {
		ServerURL string `yaml:"server_url"`
	} `yaml:"team"`
}

// DetectMode determines the operational mode for a project.
//
// Detection priority (highest wins):
//  1. Team mode: team.server_url is configured
//  2. Solo mode: default
//
// This function checks config sources in order:
// - Project config (.orc/config.yaml)
// - User config (~/.orc/config.yaml)
func DetectMode(projectPath string) Mode {
	// Check for team server configuration first (highest priority)
	if hasTeamServer(projectPath) {
		return ModeTeam
	}

	// Default: solo mode
	return ModeSolo
}

// hasTeamServer checks if team.server_url is configured in any config source.
func hasTeamServer(projectPath string) bool {
	// Check project config first
	projectConfig := filepath.Join(projectPath, OrcDir, ConfigFileName)
	if serverURL := readTeamServerURL(projectConfig); serverURL != "" {
		return true
	}

	// Check user config
	if home, err := os.UserHomeDir(); err == nil {
		userConfig := filepath.Join(home, ".orc", ConfigFileName)
		if serverURL := readTeamServerURL(userConfig); serverURL != "" {
			return true
		}
	}

	return false
}

// readTeamServerURL reads the team.server_url from a config file.
// Returns empty string if file doesn't exist or field is not set.
func readTeamServerURL(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var cfg teamConfigRaw
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}

	return cfg.Team.ServerURL
}

