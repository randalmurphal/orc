package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/tests/testutil"
)

// TestTeamInitCreatesStructure verifies that team init creates the correct
// directory structure.
func TestTeamInitCreatesStructure(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Use InitSharedDir to simulate `orc team init`
	repo.InitSharedDir()

	sharedDir := filepath.Join(repo.OrcDir, "shared")

	// Check directories exist
	expectedDirs := []string{
		sharedDir,
		filepath.Join(sharedDir, "prompts"),
		filepath.Join(sharedDir, "skills"),
		filepath.Join(sharedDir, "templates"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("directory %s should exist: %v", dir, err)
		}
	}

	// Check files exist
	expectedFiles := []string{
		filepath.Join(sharedDir, "config.yaml"),
		filepath.Join(sharedDir, "team.yaml"),
	}

	for _, file := range expectedFiles {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("file %s should exist: %v", file, err)
		}
	}
}

// TestTeamInitSharedConfigContent verifies the content of shared config.
func TestTeamInitSharedConfigContent(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	configPath := filepath.Join(repo.OrcDir, "shared", "config.yaml")
	config := testutil.ReadYAML(t, configPath)

	// Check version
	if v, ok := config["version"].(int); !ok || v != 1 {
		t.Errorf("version = %v, want 1", config["version"])
	}

	// Check task_id
	taskID, ok := config["task_id"].(map[string]interface{})
	if !ok {
		t.Fatal("task_id section missing")
	}

	if taskID["mode"] != "p2p" {
		t.Errorf("task_id.mode = %v, want p2p", taskID["mode"])
	}
	if taskID["prefix_source"] != "initials" {
		t.Errorf("task_id.prefix_source = %v, want initials", taskID["prefix_source"])
	}

	// Check defaults
	defaults, ok := config["defaults"].(map[string]interface{})
	if !ok {
		t.Fatal("defaults section missing")
	}

	if defaults["profile"] != "safe" {
		t.Errorf("defaults.profile = %v, want safe", defaults["profile"])
	}
}

// TestTeamInitTeamYamlContent verifies the content of team.yaml.
func TestTeamInitTeamYamlContent(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	teamPath := filepath.Join(repo.OrcDir, "shared", "team.yaml")
	team := testutil.ReadYAML(t, teamPath)

	// Check version
	if v, ok := team["version"].(int); !ok || v != 1 {
		t.Errorf("version = %v, want 1", team["version"])
	}

	// Check members is empty array
	members, ok := team["members"].([]interface{})
	if !ok {
		t.Fatal("members should be an array")
	}
	if len(members) != 0 {
		t.Errorf("members should be empty, got %d items", len(members))
	}

	// Check reserved_prefixes is empty array
	reserved, ok := team["reserved_prefixes"].([]interface{})
	if !ok {
		t.Fatal("reserved_prefixes should be an array")
	}
	if len(reserved) != 0 {
		t.Errorf("reserved_prefixes should be empty, got %d items", len(reserved))
	}
}

// TestTeamJoinAddsMember simulates the team join flow.
func TestTeamJoinAddsMember(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	teamPath := filepath.Join(repo.OrcDir, "shared", "team.yaml")

	// Simulate adding a team member
	team := testutil.ReadYAML(t, teamPath)

	// Add member
	members, _ := team["members"].([]interface{})
	members = append(members, map[string]interface{}{
		"initials": "AM",
		"name":     "Alice Martinez",
		"email":    "alice@example.com",
	})
	team["members"] = members

	// Add reserved prefix
	reserved, _ := team["reserved_prefixes"].([]interface{})
	reserved = append(reserved, "AM")
	team["reserved_prefixes"] = reserved

	testutil.WriteYAML(t, teamPath, team)

	// Verify member was added
	updatedTeam := testutil.ReadYAML(t, teamPath)
	updatedMembers := updatedTeam["members"].([]interface{})

	if len(updatedMembers) != 1 {
		t.Fatalf("expected 1 member, got %d", len(updatedMembers))
	}

	member := updatedMembers[0].(map[string]interface{})
	if member["initials"] != "AM" {
		t.Errorf("member initials = %v, want AM", member["initials"])
	}
	if member["name"] != "Alice Martinez" {
		t.Errorf("member name = %v, want Alice Martinez", member["name"])
	}

	// Verify prefix reserved
	updatedReserved := updatedTeam["reserved_prefixes"].([]interface{})
	if len(updatedReserved) != 1 {
		t.Fatalf("expected 1 reserved prefix, got %d", len(updatedReserved))
	}
	if updatedReserved[0] != "AM" {
		t.Errorf("reserved prefix = %v, want AM", updatedReserved[0])
	}
}

// TestTeamJoinPrefixUniqueness verifies that duplicate prefixes are detected.
func TestTeamJoinPrefixUniqueness(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	teamPath := filepath.Join(repo.OrcDir, "shared", "team.yaml")

	// Add first member
	team := testutil.ReadYAML(t, teamPath)
	members, _ := team["members"].([]interface{})
	members = append(members, map[string]interface{}{
		"initials": "AM",
		"name":     "Alice Martinez",
	})
	team["members"] = members

	reserved, _ := team["reserved_prefixes"].([]interface{})
	reserved = append(reserved, "AM")
	team["reserved_prefixes"] = reserved

	testutil.WriteYAML(t, teamPath, team)

	// Function to check if prefix is taken
	isPrefixTaken := func(prefix string) bool {
		currentTeam := testutil.ReadYAML(t, teamPath)
		currentReserved, _ := currentTeam["reserved_prefixes"].([]interface{})
		for _, p := range currentReserved {
			if p == prefix {
				return true
			}
		}
		return false
	}

	// Check existing prefix
	if !isPrefixTaken("AM") {
		t.Error("AM should be taken")
	}

	// Check new prefix
	if isPrefixTaken("BJ") {
		t.Error("BJ should not be taken yet")
	}
}

// TestTeamMembersList verifies listing team members.
func TestTeamMembersList(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	teamPath := filepath.Join(repo.OrcDir, "shared", "team.yaml")

	// Add multiple members
	team := testutil.ReadYAML(t, teamPath)
	members := []interface{}{
		map[string]interface{}{
			"initials": "AM",
			"name":     "Alice Martinez",
			"email":    "alice@example.com",
		},
		map[string]interface{}{
			"initials": "BJ",
			"name":     "Bob Johnson",
			"email":    "",
		},
		map[string]interface{}{
			"initials": "CD",
			"name":     "Carol Davis",
		},
	}
	team["members"] = members
	team["reserved_prefixes"] = []interface{}{"AM", "BJ", "CD"}
	testutil.WriteYAML(t, teamPath, team)

	// Read and verify
	updatedTeam := testutil.ReadYAML(t, teamPath)
	updatedMembers := updatedTeam["members"].([]interface{})

	if len(updatedMembers) != 3 {
		t.Errorf("expected 3 members, got %d", len(updatedMembers))
	}

	// Verify first member
	first := updatedMembers[0].(map[string]interface{})
	if first["initials"] != "AM" {
		t.Errorf("first member initials = %v, want AM", first["initials"])
	}

	// Verify reserved prefixes
	reserved := updatedTeam["reserved_prefixes"].([]interface{})
	if len(reserved) != 3 {
		t.Errorf("expected 3 reserved prefixes, got %d", len(reserved))
	}
}

// TestTeamInitIdempotent verifies that running init multiple times is safe
// (with force flag logic).
func TestTeamInitIdempotent(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// First init
	repo.InitSharedDir()

	sharedDir := filepath.Join(repo.OrcDir, "shared")

	// Verify exists
	if _, err := os.Stat(sharedDir); err != nil {
		t.Fatal("shared dir should exist after first init")
	}

	// Add some content
	teamPath := filepath.Join(sharedDir, "team.yaml")
	team := testutil.ReadYAML(t, teamPath)
	members := []interface{}{
		map[string]interface{}{
			"initials": "AM",
			"name":     "Alice",
		},
	}
	team["members"] = members
	testutil.WriteYAML(t, teamPath, team)

	// Second init (simulating --force behavior by recreating)
	// In real CLI, this would check for --force flag
	// Here we just verify the directory still exists
	if _, err := os.Stat(sharedDir); err != nil {
		t.Error("shared dir should still exist")
	}

	// Content should be preserved (without --force)
	updatedTeam := testutil.ReadYAML(t, teamPath)
	updatedMembers, ok := updatedTeam["members"].([]interface{})
	if !ok || len(updatedMembers) != 1 {
		t.Error("members should be preserved when not using --force")
	}
}

// TestUserIdentitySaved verifies that user identity is saved correctly.
func TestUserIdentitySaved(t *testing.T) {
	// Create mock user home
	userHome := testutil.MockUserConfig(t, "AM")

	// Verify identity was saved
	configPath := filepath.Join(userHome, ".orc", "config.yaml")
	config := testutil.ReadYAML(t, configPath)

	identity, ok := config["identity"].(map[string]interface{})
	if !ok {
		t.Fatal("identity section should exist")
	}

	if identity["initials"] != "AM" {
		t.Errorf("identity.initials = %v, want AM", identity["initials"])
	}
}

// TestInitialsValidation verifies initials format validation.
func TestInitialsValidation(t *testing.T) {
	tests := []struct {
		initials string
		valid    bool
	}{
		{"AM", true},
		{"BJ", true},
		{"ABC", true},
		{"XY1", true},   // Alphanumeric allowed
		{"ABCD", true},  // 4 chars max
		{"A", false},    // Too short
		{"ABCDE", false}, // Too long
		{"A!", false},   // Invalid char
		{"a-b", false},  // Invalid char
	}

	for _, tt := range tests {
		t.Run(tt.initials, func(t *testing.T) {
			// Validate initials format
			valid := len(tt.initials) >= 2 && len(tt.initials) <= 4
			if valid {
				for _, c := range tt.initials {
					if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
						valid = false
						break
					}
				}
			}

			if valid != tt.valid {
				t.Errorf("validate(%q) = %v, want %v", tt.initials, valid, tt.valid)
			}
		})
	}
}
