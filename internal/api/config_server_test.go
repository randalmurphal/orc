// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-586: Settings Slash Commands badge/list mismatch
//
// Bug: Badge shows "6" (from GetConfigStats) but list shows "No commands" (from ListSkills).
//
// Root Cause: GetConfigStats counts skills from BOTH global + project directories,
// but ListSkills without a scope defaults to PROJECT scope only.
//
// Fix: ListSkills should return all skills (global + project) when no scope is specified.
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

// ============================================================================
// SC-1: ListSkills without scope returns all skills (global + project)
// ============================================================================

// TestListSkills_NoScope_ReturnsAllSkills verifies SC-1:
// When ListSkills is called without a scope, it should return BOTH global and project skills.
// This must match the count returned by GetConfigStats.slashCommandsCount.
func TestListSkills_NoScope_ReturnsAllSkills(t *testing.T) {
	t.Parallel()

	// Create temp project directory with a unique skill name
	projectDir := t.TempDir()
	projectClaudeDir := filepath.Join(projectDir, ".claude")
	projectSkillDir := filepath.Join(projectClaudeDir, "skills", "test-project-skill-586")

	if err := os.MkdirAll(projectSkillDir, 0755); err != nil {
		t.Fatalf("create project skill dir: %v", err)
	}

	// Write project skill with unique name
	projectSkill := &claudeconfig.Skill{
		Name:        "test-project-skill-586",
		Description: "A test project skill for TASK-586",
		Content:     "Project skill content",
	}
	if err := claudeconfig.WriteSkillMD(projectSkill, projectSkillDir); err != nil {
		t.Fatalf("write project skill: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(nil, backend, projectDir, nil)

	// Get global skills count first (whatever is in real home)
	globalScope := orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL
	globalReq := connect.NewRequest(&orcv1.ListSkillsRequest{Scope: &globalScope})
	globalResp, err := server.ListSkills(context.Background(), globalReq)
	if err != nil {
		t.Fatalf("ListSkills (global) failed: %v", err)
	}
	globalCount := len(globalResp.Msg.Skills)

	// Get project skills count
	projectScope := orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT
	projectReq := connect.NewRequest(&orcv1.ListSkillsRequest{Scope: &projectScope})
	projectResp, err := server.ListSkills(context.Background(), projectReq)
	if err != nil {
		t.Fatalf("ListSkills (project) failed: %v", err)
	}
	projectCount := len(projectResp.Msg.Skills)

	// Call ListSkills WITHOUT specifying a scope (this is what the frontend does)
	noScopeReq := connect.NewRequest(&orcv1.ListSkillsRequest{
		// No scope specified - should return all skills
	})
	noScopeResp, err := server.ListSkills(context.Background(), noScopeReq)
	if err != nil {
		t.Fatalf("ListSkills (no scope) failed: %v", err)
	}
	noScopeCount := len(noScopeResp.Msg.Skills)

	// CRITICAL TEST: ListSkills without scope should return ALL skills (global + project)
	expectedTotal := globalCount + projectCount
	if noScopeCount != expectedTotal {
		t.Errorf("ListSkills without scope returned %d skills, want %d (global=%d + project=%d). This is the bug!",
			noScopeCount, expectedTotal, globalCount, projectCount)
	}

	// Verify the project skill is included when no scope
	hasProjectSkill := false
	for _, skill := range noScopeResp.Msg.Skills {
		if skill.Name == "test-project-skill-586" {
			hasProjectSkill = true
			break
		}
	}
	if !hasProjectSkill {
		t.Error("test-project-skill-586 not returned by ListSkills without scope")
	}
}

// ============================================================================
// SC-2: GetConfigStats and ListSkills return consistent counts
// ============================================================================

// TestListSkills_MatchesConfigStatsCount verifies SC-2:
// The count from GetConfigStats.slashCommandsCount must match len(ListSkills().skills)
// when ListSkills is called without a scope.
func TestListSkills_MatchesConfigStatsCount(t *testing.T) {
	t.Parallel()

	// Create temp project directory with unique skills
	projectDir := t.TempDir()
	projectClaudeDir := filepath.Join(projectDir, ".claude")

	// Create 3 unique project skills
	skillNames := []string{"task586-a", "task586-b", "task586-c"}
	for _, name := range skillNames {
		skillDir := filepath.Join(projectClaudeDir, "skills", name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("create skill dir %s: %v", name, err)
		}
		skill := &claudeconfig.Skill{
			Name:        name,
			Description: "Test skill " + name,
			Content:     "Content for " + name,
		}
		if err := claudeconfig.WriteSkillMD(skill, skillDir); err != nil {
			t.Fatalf("write skill %s: %v", name, err)
		}
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(nil, backend, projectDir, nil)

	// Get stats (badge count)
	statsReq := connect.NewRequest(&orcv1.GetConfigStatsRequest{})
	statsResp, err := server.GetConfigStats(context.Background(), statsReq)
	if err != nil {
		t.Fatalf("GetConfigStats failed: %v", err)
	}
	badgeCount := int(statsResp.Msg.Stats.SlashCommandsCount)

	// Get skills list (content) - without scope
	listReq := connect.NewRequest(&orcv1.ListSkillsRequest{
		// No scope - should return all
	})
	listResp, err := server.ListSkills(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}
	listCount := len(listResp.Msg.Skills)

	// CRITICAL: Badge count must match list count
	if badgeCount != listCount {
		t.Errorf("Count mismatch: GetConfigStats.slashCommandsCount=%d, len(ListSkills)=%d (this is the bug!)",
			badgeCount, listCount)
	}

	// The badge count should include at least our 3 project skills
	if badgeCount < len(skillNames) {
		t.Errorf("GetConfigStats returned %d, expected at least %d (our test skills)", badgeCount, len(skillNames))
	}
}

// ============================================================================
// Edge Cases: Scope-Specific Behavior
// ============================================================================

// TestListSkills_ProjectScope_ReturnsProjectSkill verifies that
// explicitly specifying PROJECT scope returns project skills.
func TestListSkills_ProjectScope_ReturnsProjectSkill(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	projectSkillDir := filepath.Join(projectDir, ".claude", "skills", "test-proj-only-586")

	if err := os.MkdirAll(projectSkillDir, 0755); err != nil {
		t.Fatalf("create project skill dir: %v", err)
	}
	if err := claudeconfig.WriteSkillMD(&claudeconfig.Skill{
		Name: "test-proj-only-586", Description: "Project", Content: "Content",
	}, projectSkillDir); err != nil {
		t.Fatalf("write project skill: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(nil, backend, projectDir, nil)

	scope := orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT
	req := connect.NewRequest(&orcv1.ListSkillsRequest{Scope: &scope})

	resp, err := server.ListSkills(context.Background(), req)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	// Should contain our test skill
	hasSkill := false
	for _, skill := range resp.Msg.Skills {
		if skill.Name == "test-proj-only-586" {
			hasSkill = true
			if skill.Scope != orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT {
				t.Errorf("Skill has scope %v, want PROJECT", skill.Scope)
			}
			break
		}
	}
	if !hasSkill {
		t.Error("test-proj-only-586 not returned by ListSkills with PROJECT scope")
	}
}

// TestListSkills_EmptyProject_StillReturnsGlobal verifies that
// when project has no skills but global does, no-scope request returns global skills.
func TestListSkills_EmptyProject_StillReturnsGlobal(t *testing.T) {
	t.Parallel()

	// Create empty project dir (no .claude/skills)
	projectDir := t.TempDir()

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(nil, backend, projectDir, nil)

	// Get global count first
	globalScope := orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL
	globalReq := connect.NewRequest(&orcv1.ListSkillsRequest{Scope: &globalScope})
	globalResp, err := server.ListSkills(context.Background(), globalReq)
	if err != nil {
		t.Fatalf("ListSkills (global) failed: %v", err)
	}
	globalCount := len(globalResp.Msg.Skills)

	// Skip test if no global skills (can't test the behavior)
	if globalCount == 0 {
		t.Skip("No global skills to test with")
	}

	// No-scope request should return global skills even with empty project
	noScopeReq := connect.NewRequest(&orcv1.ListSkillsRequest{})
	noScopeResp, err := server.ListSkills(context.Background(), noScopeReq)
	if err != nil {
		t.Fatalf("ListSkills (no scope) failed: %v", err)
	}

	// With empty project, no-scope should still return global skills
	if len(noScopeResp.Msg.Skills) != globalCount {
		t.Errorf("ListSkills (no scope) with empty project returned %d skills, want %d (global count)",
			len(noScopeResp.Msg.Skills), globalCount)
	}
}
