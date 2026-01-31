// TDD Tests for TASK-610: Discover commands from .claude/commands/ directory
// Updated for TASK-668: ListSkills now reads from GlobalDB instead of file system.
// GetConfigStats still uses file-based discovery for legacy slash commands counting.
package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"github.com/randalmurphal/llmkit/claudeconfig"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// TestListSkills_IncludesProjectCommands verifies that skills seeded into
// GlobalDB are returned by ListSkills.
func TestListSkills_IncludesProjectCommands(t *testing.T) {
	t.Parallel()

	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed skills into GlobalDB
	err := gdb.SaveSkill(&db.Skill{
		ID: "review-skill", Name: "review",
		Content: "# Code Review\n\nReview the code.", IsBuiltin: false,
	})
	if err != nil {
		t.Fatalf("seed review skill: %v", err)
	}
	err = gdb.SaveSkill(&db.Skill{
		ID: "test-skill-610", Name: "test-skill-610",
		Description: "A test skill for TASK-610",
		Content:     "Test skill content", IsBuiltin: false,
	})
	if err != nil {
		t.Fatalf("seed test skill: %v", err)
	}

	req := connect.NewRequest(&orcv1.ListSkillsRequest{})
	resp, err := server.ListSkills(context.Background(), req)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	// Should include both skills
	hasReview := false
	hasSkill := false
	for _, skill := range resp.Msg.Skills {
		if skill.Name == "review" {
			hasReview = true
			if skill.Content == "" {
				t.Error("skill 'review' has empty content")
			}
		}
		if skill.Name == "test-skill-610" {
			hasSkill = true
		}
	}

	if !hasReview {
		t.Error("skill 'review' not returned by ListSkills")
	}
	if !hasSkill {
		t.Error("skill 'test-skill-610' not returned by ListSkills")
	}
}

// TestListSkills_CommandsHaveCorrectScope verifies that skills from GlobalDB
// have their content populated correctly.
func TestListSkills_CommandsHaveCorrectScope(t *testing.T) {
	t.Parallel()

	server, gdb := newTestConfigServerWithGlobalDB(t)

	content := "# Test\n\nTest command content."
	err := gdb.SaveSkill(&db.Skill{
		ID: "test-cmd", Name: "test-cmd",
		Content: content, IsBuiltin: false,
	})
	if err != nil {
		t.Fatalf("seed skill: %v", err)
	}

	req := connect.NewRequest(&orcv1.ListSkillsRequest{})
	resp, err := server.ListSkills(context.Background(), req)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	var found *orcv1.Skill
	for _, skill := range resp.Msg.Skills {
		if skill.Name == "test-cmd" {
			found = skill
			break
		}
	}

	if found == nil {
		t.Fatal("skill 'test-cmd' not found in ListSkills response")
	}

	if found.Content != content {
		t.Errorf("skill content = %q, want %q", found.Content, content)
	}
}

// TestGetConfigStats_IncludesCommandsCount verifies that GetConfigStats still
// counts file-based skills and commands from .claude/ directories.
// Note: GetConfigStats was NOT changed by TASK-668, it still uses file-based discovery.
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

	// Get stats - still uses file-based discovery
	statsReq := connect.NewRequest(&orcv1.GetConfigStatsRequest{})
	statsResp, err := server.GetConfigStats(context.Background(), statsReq)
	if err != nil {
		t.Fatalf("GetConfigStats failed: %v", err)
	}
	badgeCount := int(statsResp.Msg.Stats.SlashCommandsCount)

	// Badge count should include at least our 3 project items
	if badgeCount < 3 {
		t.Errorf("GetConfigStats.SlashCommandsCount = %d, want at least 3 (2 commands + 1 skill)", badgeCount)
	}
}

// TestListSkills_CommandsIgnoresNonMd verifies that only skills seeded to
// GlobalDB are returned by ListSkills.
func TestListSkills_CommandsIgnoresNonMd(t *testing.T) {
	t.Parallel()

	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Only seed one valid skill
	err := gdb.SaveSkill(&db.Skill{
		ID: "valid", Name: "valid",
		Content: "# Valid\n\nA valid skill.", IsBuiltin: false,
	})
	if err != nil {
		t.Fatalf("seed skill: %v", err)
	}

	req := connect.NewRequest(&orcv1.ListSkillsRequest{})
	resp, err := server.ListSkills(context.Background(), req)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	if len(resp.Msg.Skills) != 1 {
		t.Errorf("ListSkills returned %d skills, want 1", len(resp.Msg.Skills))
	}

	if resp.Msg.Skills[0].Name != "valid" {
		t.Errorf("expected skill name 'valid', got %q", resp.Msg.Skills[0].Name)
	}
}
