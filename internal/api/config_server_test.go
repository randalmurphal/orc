// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-586: Settings Slash Commands badge/list mismatch
// Updated for TASK-668: ListSkills now reads from GlobalDB instead of file system
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// ============================================================================
// SC-1: ListSkills without scope returns all skills from GlobalDB
// ============================================================================

// TestListSkills_NoScope_ReturnsAllSkills verifies SC-1:
// When ListSkills is called without a scope, it should return all skills from GlobalDB.
func TestListSkills_NoScope_ReturnsAllSkills(t *testing.T) {
	t.Parallel()

	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed skills into GlobalDB
	for _, s := range []struct {
		id, name string
		builtin  bool
	}{
		{"s1", "project-skill-586", false},
		{"s2", "global-skill-586", false},
		{"s3", "builtin-skill", true},
	} {
		err := gdb.SaveSkill(&db.Skill{
			ID: s.id, Name: s.name, Content: "# " + s.name, IsBuiltin: s.builtin,
		})
		if err != nil {
			t.Fatalf("seed skill %s: %v", s.name, err)
		}
	}

	// Call ListSkills WITHOUT specifying a scope (this is what the frontend does)
	noScopeReq := connect.NewRequest(&orcv1.ListSkillsRequest{})
	noScopeResp, err := server.ListSkills(context.Background(), noScopeReq)
	if err != nil {
		t.Fatalf("ListSkills (no scope) failed: %v", err)
	}

	// Should return all 3 skills
	if len(noScopeResp.Msg.Skills) != 3 {
		t.Errorf("ListSkills without scope returned %d skills, want 3", len(noScopeResp.Msg.Skills))
	}

	// Verify specific skill is included
	hasProjectSkill := false
	for _, skill := range noScopeResp.Msg.Skills {
		if skill.Name == "project-skill-586" {
			hasProjectSkill = true
			break
		}
	}
	if !hasProjectSkill {
		t.Error("project-skill-586 not returned by ListSkills without scope")
	}
}

// ============================================================================
// SC-2: ListSkills returns correct counts
// ============================================================================

// TestListSkills_MatchesConfigStatsCount verifies that ListSkills returns
// all skills from GlobalDB with correct count.
func TestListSkills_MatchesConfigStatsCount(t *testing.T) {
	t.Parallel()

	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed 3 skills
	skillNames := []string{"task586-a", "task586-b", "task586-c"}
	for i, name := range skillNames {
		err := gdb.SaveSkill(&db.Skill{
			ID: name, Name: name, Content: "Content for " + name,
			Description: "Test skill " + name, IsBuiltin: false,
		})
		if err != nil {
			t.Fatalf("seed skill %d: %v", i, err)
		}
	}

	// Get skills list (content) - without scope
	listReq := connect.NewRequest(&orcv1.ListSkillsRequest{})
	listResp, err := server.ListSkills(context.Background(), listReq)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}
	listCount := len(listResp.Msg.Skills)

	// Should have exactly 3 skills
	if listCount != len(skillNames) {
		t.Errorf("ListSkills returned %d, expected %d", listCount, len(skillNames))
	}
}

// ============================================================================
// Edge Cases: Scope-Specific Behavior
// ============================================================================

// TestListSkills_ProjectScope_ReturnsProjectSkill verifies that
// ListSkills returns all skills from GlobalDB (scope filtering is handled by frontend).
func TestListSkills_ProjectScope_ReturnsProjectSkill(t *testing.T) {
	t.Parallel()

	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed a skill
	err := gdb.SaveSkill(&db.Skill{
		ID: "proj-only", Name: "test-proj-only-586",
		Content: "Content", Description: "Project", IsBuiltin: false,
	})
	if err != nil {
		t.Fatalf("seed skill: %v", err)
	}

	req := connect.NewRequest(&orcv1.ListSkillsRequest{})
	resp, err := server.ListSkills(context.Background(), req)
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	// Should contain our test skill
	hasSkill := false
	for _, skill := range resp.Msg.Skills {
		if skill.Name == "test-proj-only-586" {
			hasSkill = true
			break
		}
	}
	if !hasSkill {
		t.Error("test-proj-only-586 not returned by ListSkills")
	}
}

// TestListSkills_EmptyProject_StillReturnsGlobal verifies that
// when no skills exist in GlobalDB, empty list is returned.
func TestListSkills_EmptyProject_StillReturnsGlobal(t *testing.T) {
	t.Parallel()

	server, _ := newTestConfigServerWithGlobalDB(t)

	noScopeReq := connect.NewRequest(&orcv1.ListSkillsRequest{})
	noScopeResp, err := server.ListSkills(context.Background(), noScopeReq)
	if err != nil {
		t.Fatalf("ListSkills (no scope) failed: %v", err)
	}

	// With empty GlobalDB, should return 0 skills
	if len(noScopeResp.Msg.Skills) != 0 {
		t.Errorf("ListSkills with empty GlobalDB returned %d skills, want 0",
			len(noScopeResp.Msg.Skills))
	}
}
