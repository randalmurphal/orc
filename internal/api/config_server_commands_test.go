// TDD Tests for TASK-610: Discover commands from .claude/commands/ directory
//
// These tests verify that ListSkills and GetConfigStats discover commands
// from .claude/commands/ (flat .md files) in addition to .claude/skills/
// (directories with SKILL.md). Commands from .claude/commands/ should appear
// with appropriate scope tagging.
package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/randalmurphal/llmkit/claudeconfig"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
)

// TestListSkills_IncludesProjectCommands verifies SC-1:
// Project commands from .claude/commands/ are discovered and returned by ListSkills
// alongside traditional skills from .claude/skills/.
func TestListSkills_IncludesProjectCommands(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	// Create a command in .claude/commands/ (flat .md file)
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("create commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "review.md"), []byte("# Code Review\n\nReview the code."), 0644); err != nil {
		t.Fatalf("write review.md: %v", err)
	}

	// Create a skill in .claude/skills/ (directory with SKILL.md)
	skillDir := filepath.Join(projectDir, ".claude", "skills", "test-skill-610")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := claudeconfig.WriteSkillMD(&claudeconfig.Skill{
		Name:        "test-skill-610",
		Description: "A test skill for TASK-610",
		Content:     "Test skill content",
	}, skillDir); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(nil, backend, projectDir, nil)

	// Call ListSkills with PROJECT scope to avoid picking up real global skills
	scope := orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT
	req := connect.NewRequest(&orcv1.ListSkillsRequest{Scope: &scope})
	resp, err := server.ListSkills(context.Background(), req)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	// Should include both the command and the skill
	hasCommand := false
	hasSkill := false
	for _, skill := range resp.Msg.Skills {
		if skill.Name == "review" {
			hasCommand = true
			if skill.Scope != orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT {
				t.Errorf("command 'review' has scope %v, want PROJECT", skill.Scope)
			}
			// Verify content is populated from the .md file
			if skill.Content == "" {
				t.Error("command 'review' has empty content, want file content")
			}
		}
		if skill.Name == "test-skill-610" {
			hasSkill = true
		}
	}

	if !hasCommand {
		t.Error("command 'review' from .claude/commands/review.md not returned by ListSkills")
	}
	if !hasSkill {
		t.Error("skill 'test-skill-610' from .claude/skills/ not returned by ListSkills")
	}
}

// TestListSkills_CommandsHaveCorrectScope verifies SC-1:
// Commands from .claude/commands/ are tagged with the correct scope and
// have their content populated from the .md file.
func TestListSkills_CommandsHaveCorrectScope(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	// Create a command in .claude/commands/
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("create commands dir: %v", err)
	}
	commandContent := "# Test\n\nTest command content."
	if err := os.WriteFile(filepath.Join(commandsDir, "test-cmd.md"), []byte(commandContent), 0644); err != nil {
		t.Fatalf("write test-cmd.md: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(nil, backend, projectDir, nil)

	// Use PROJECT scope to isolate from real global state
	scope := orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT
	req := connect.NewRequest(&orcv1.ListSkillsRequest{Scope: &scope})
	resp, err := server.ListSkills(context.Background(), req)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	// Find the command in the response
	var found *orcv1.Skill
	for _, skill := range resp.Msg.Skills {
		if skill.Name == "test-cmd" {
			found = skill
			break
		}
	}

	if found == nil {
		t.Fatal("command 'test-cmd' not found in ListSkills response")
	}

	if found.Scope != orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT {
		t.Errorf("command scope = %v, want SETTINGS_SCOPE_PROJECT", found.Scope)
	}

	if found.Content != commandContent {
		t.Errorf("command content = %q, want %q", found.Content, commandContent)
	}
}

// TestGetConfigStats_IncludesCommandsCount verifies SC-2:
// GetConfigStats.SlashCommandsCount includes commands from .claude/commands/
// in addition to skills from .claude/skills/.
func TestGetConfigStats_IncludesCommandsCount(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	// Create 2 commands in .claude/commands/
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("create commands dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "cmd-a.md"), []byte("# Command A\n\nFirst command."), 0644); err != nil {
		t.Fatalf("write cmd-a.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "cmd-b.md"), []byte("# Command B\n\nSecond command."), 0644); err != nil {
		t.Fatalf("write cmd-b.md: %v", err)
	}

	// Create 1 skill in .claude/skills/
	skillDir := filepath.Join(projectDir, ".claude", "skills", "skill-a")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := claudeconfig.WriteSkillMD(&claudeconfig.Skill{
		Name:        "skill-a",
		Description: "Test skill A",
		Content:     "Skill A content",
	}, skillDir); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(nil, backend, projectDir, nil)

	// Get stats
	statsReq := connect.NewRequest(&orcv1.GetConfigStatsRequest{})
	statsResp, err := server.GetConfigStats(context.Background(), statsReq)
	if err != nil {
		t.Fatalf("GetConfigStats failed: %v", err)
	}
	badgeCount := int(statsResp.Msg.Stats.SlashCommandsCount)

	// Get list (no scope = all)
	listReq := connect.NewRequest(&orcv1.ListSkillsRequest{})
	listResp, err := server.ListSkills(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}
	listCount := len(listResp.Msg.Skills)

	// Call ListSkills with PROJECT scope to isolate from global skills
	projectScope := orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT
	projectReq := connect.NewRequest(&orcv1.ListSkillsRequest{Scope: &projectScope})
	projectResp, err := server.ListSkills(context.Background(), projectReq)
	if err != nil {
		t.Fatalf("ListSkills (PROJECT scope) failed: %v", err)
	}
	projectCount := len(projectResp.Msg.Skills)

	// Must have at least 3 project-scoped items: 2 commands (cmd-a, cmd-b) + 1 skill (skill-a)
	if projectCount < 3 {
		t.Errorf("ListSkills(PROJECT).count = %d, want at least 3 (2 commands + 1 skill)", projectCount)
	}

	// Verify the 2 command names appear in project-scoped results
	foundCmdA := false
	foundCmdB := false
	for _, skill := range projectResp.Msg.Skills {
		switch skill.Name {
		case "cmd-a":
			foundCmdA = true
		case "cmd-b":
			foundCmdB = true
		}
	}
	if !foundCmdA {
		t.Error("command 'cmd-a' not found in ListSkills(PROJECT) results")
	}
	if !foundCmdB {
		t.Error("command 'cmd-b' not found in ListSkills(PROJECT) results")
	}

	// Badge count must include at least our 3 project items
	// (may also include global skills from the real home directory)
	if badgeCount < 3 {
		t.Errorf("GetConfigStats.SlashCommandsCount = %d, want at least 3 (2 commands + 1 skill)", badgeCount)
	}

	// Consistency: badge count must match unscoped list count
	if badgeCount != listCount {
		t.Errorf("count mismatch: GetConfigStats.SlashCommandsCount=%d, len(ListSkills)=%d",
			badgeCount, listCount)
	}
}

// TestListSkills_CommandsIgnoresNonMd verifies SC-1 edge case:
// Only .md files in .claude/commands/ are discovered. Non-.md files
// and subdirectories are ignored.
func TestListSkills_CommandsIgnoresNonMd(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()

	// Create .claude/commands/ with various file types
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0755); err != nil {
		t.Fatalf("create commands dir: %v", err)
	}

	// Valid .md file - should be discovered
	if err := os.WriteFile(filepath.Join(commandsDir, "valid.md"), []byte("# Valid\n\nA valid command."), 0644); err != nil {
		t.Fatalf("write valid.md: %v", err)
	}

	// Non-.md files - should be ignored
	if err := os.WriteFile(filepath.Join(commandsDir, "readme.txt"), []byte("not a command"), 0644); err != nil {
		t.Fatalf("write readme.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "notes.json"), []byte(`{"not": "a command"}`), 0644); err != nil {
		t.Fatalf("write notes.json: %v", err)
	}

	// Subdirectory - should be ignored
	if err := os.MkdirAll(filepath.Join(commandsDir, "subdir"), 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(nil, backend, projectDir, nil)

	// Use PROJECT scope to isolate from global state
	scope := orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT
	req := connect.NewRequest(&orcv1.ListSkillsRequest{Scope: &scope})
	resp, err := server.ListSkills(context.Background(), req)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	// Count commands that came from .claude/commands/
	// Only "valid" should appear (not readme, notes, or subdir)
	commandNames := make(map[string]bool)
	for _, skill := range resp.Msg.Skills {
		commandNames[skill.Name] = true
	}

	if !commandNames["valid"] {
		t.Error("command 'valid' from valid.md not found in ListSkills response")
	}
	if commandNames["readme"] {
		t.Error("'readme' from readme.txt should not appear in ListSkills (not .md)")
	}
	if commandNames["notes"] {
		t.Error("'notes' from notes.json should not appear in ListSkills (not .md)")
	}
	if commandNames["subdir"] {
		t.Error("'subdir' directory should not appear in ListSkills")
	}
}
