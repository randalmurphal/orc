// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-668: Transform Hooks and Skills to GlobalDB CRUD
//
// These tests verify that ConfigServer hook/skill CRUD methods use GlobalDB
// (hook_scripts and skills tables) instead of file-based storage (.claude/settings.json
// and .claude/skills/ directories).
//
// Tests will NOT COMPILE until:
// 1. Proto types updated (Hook gains id/description/content/event_type/is_builtin;
//    Skill gains id/is_builtin/supporting_files)
// 2. ConfigServer gains SetGlobalDB(*db.GlobalDB) method
// 3. ConfigServer methods rewired to use GlobalDB
// 4. internal/db/skills.go created with SaveSkill/GetSkill/ListSkills/DeleteSkill
package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// --- Test helpers ---

// newTestConfigServerWithGlobalDB creates a ConfigServer with an in-memory GlobalDB.
// Returns the server and GlobalDB for seeding test data.
func newTestConfigServerWithGlobalDB(t *testing.T) (orcv1connect.ConfigServiceHandler, *db.GlobalDB) {
	t.Helper()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	projectDir := t.TempDir()

	server := NewConfigServer(nil, backend, projectDir, nil)
	// Inject GlobalDB dependency - method does not exist yet (TDD)
	server.(*configServer).SetGlobalDB(globalDB)

	return server, globalDB
}

// seedBuiltinHook inserts a built-in hook directly into GlobalDB.
func seedBuiltinHook(t *testing.T, gdb *db.GlobalDB, id, name, eventType, content string) {
	t.Helper()
	err := gdb.SaveHookScript(&db.HookScript{
		ID:        id,
		Name:      name,
		EventType: eventType,
		Content:   content,
		IsBuiltin: true,
	})
	require.NoError(t, err)
}

// seedBuiltinSkill inserts a built-in skill directly into GlobalDB.
func seedBuiltinSkill(t *testing.T, gdb *db.GlobalDB, id, name, content string) {
	t.Helper()
	err := gdb.SaveSkill(&db.Skill{
		ID:        id,
		Name:      name,
		Content:   content,
		IsBuiltin: true,
	})
	require.NoError(t, err)
}

// ============================================================================
// SC-1: Creating a hook via the UI inserts a row into hook_scripts table
// ============================================================================

func TestCreateHook_InsertsToGlobalDB(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	req := connect.NewRequest(&orcv1.CreateHookRequest{
		Name:        "test-hook",
		Description: "A test hook",
		Content:     "#!/bin/bash\nexit 0",
		EventType:   "Stop",
	})

	resp, err := server.CreateHook(context.Background(), req)
	require.NoError(t, err)

	// Verify response has correct fields
	hook := resp.Msg.Hook
	require.NotNil(t, hook)
	assert.Equal(t, "test-hook", hook.Name)
	assert.Equal(t, "A test hook", hook.Description)
	assert.Equal(t, "#!/bin/bash\nexit 0", hook.Content)
	assert.Equal(t, "Stop", hook.EventType)
	assert.False(t, hook.IsBuiltin, "newly created hook must not be built-in")
	assert.NotEmpty(t, hook.Id, "hook must have a generated ID")

	// Verify row exists in GlobalDB
	scripts, err := gdb.ListHookScripts()
	require.NoError(t, err)

	var found bool
	for _, s := range scripts {
		if s.Name == "test-hook" {
			found = true
			assert.Equal(t, "A test hook", s.Description)
			assert.Equal(t, "#!/bin/bash\nexit 0", s.Content)
			assert.Equal(t, "Stop", s.EventType)
			assert.False(t, s.IsBuiltin)
			break
		}
	}
	assert.True(t, found, "hook 'test-hook' not found in GlobalDB hook_scripts table")
}

// ============================================================================
// SC-2: Editing a hook via the UI updates the corresponding hook_scripts row
// ============================================================================

func TestUpdateHook_UpdatesGlobalDB(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed a custom hook
	err := gdb.SaveHookScript(&db.HookScript{
		ID:          "custom-hook-1",
		Name:        "my-hook",
		Description: "Original description",
		Content:     "#!/bin/bash\noriginal",
		EventType:   "Stop",
		IsBuiltin:   false,
	})
	require.NoError(t, err)

	// Update via API
	newDesc := "Updated description"
	newContent := "#!/bin/bash\nupdated"
	req := connect.NewRequest(&orcv1.UpdateHookRequest{
		Id:          "custom-hook-1",
		Description: &newDesc,
		Content:     &newContent,
	})

	resp, err := server.UpdateHook(context.Background(), req)
	require.NoError(t, err)

	hook := resp.Msg.Hook
	require.NotNil(t, hook)
	assert.Equal(t, "Updated description", hook.Description)
	assert.Equal(t, "#!/bin/bash\nupdated", hook.Content)

	// Verify DB row updated
	got, err := gdb.GetHookScript("custom-hook-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Updated description", got.Description)
	assert.Equal(t, "#!/bin/bash\nupdated", got.Content)
}

// ============================================================================
// SC-3: Deleting a hook via the UI removes it from hook_scripts table
// ============================================================================

func TestDeleteHook_RemovesFromGlobalDB(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed a custom hook
	err := gdb.SaveHookScript(&db.HookScript{
		ID:        "delete-me",
		Name:      "delete-me",
		Content:   "#!/bin/bash\necho delete",
		EventType: "PreToolUse",
		IsBuiltin: false,
	})
	require.NoError(t, err)

	req := connect.NewRequest(&orcv1.DeleteHookRequest{
		Id: "delete-me",
	})

	_, err = server.DeleteHook(context.Background(), req)
	require.NoError(t, err)

	// Verify row gone from DB
	got, err := gdb.GetHookScript("delete-me")
	require.NoError(t, err)
	assert.Nil(t, got, "hook should be removed from GlobalDB after delete")
}

// ============================================================================
// SC-4: Listing hooks loads from hook_scripts table, not .claude/settings.json
// ============================================================================

func TestListHooks_LoadsFromGlobalDB(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed hooks directly to GlobalDB (no .claude/settings.json involved)
	seedBuiltinHook(t, gdb, "orc-verify-completion", "Verify Completion", "Stop", "#!/bin/bash\nverify")
	err := gdb.SaveHookScript(&db.HookScript{
		ID:        "user-hook-1",
		Name:      "Custom Hook",
		Content:   "#!/bin/bash\ncustom",
		EventType: "PreToolUse",
		IsBuiltin: false,
	})
	require.NoError(t, err)

	// List hooks - should come from DB, not filesystem
	req := connect.NewRequest(&orcv1.ListHooksRequest{})
	resp, err := server.ListHooks(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, resp.Msg)
	assert.Len(t, resp.Msg.Hooks, 2)

	// Verify both hooks are present with correct fields
	hookNames := make(map[string]bool)
	for _, h := range resp.Msg.Hooks {
		hookNames[h.Name] = true
		// Every hook from DB must have an ID
		assert.NotEmpty(t, h.Id, "hook %s missing ID", h.Name)
	}
	assert.True(t, hookNames["Verify Completion"], "built-in hook missing")
	assert.True(t, hookNames["Custom Hook"], "custom hook missing")
}

// ============================================================================
// SC-5: Creating a skill inserts a row into skills table in GlobalDB
// ============================================================================

func TestCreateSkill_InsertsToGlobalDB(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	req := connect.NewRequest(&orcv1.CreateSkillRequest{
		Name:        "my-skill",
		Description: "A custom skill",
		Content:     "# My Skill\n\nDo the thing.",
	})

	resp, err := server.CreateSkill(context.Background(), req)
	require.NoError(t, err)

	skill := resp.Msg.Skill
	require.NotNil(t, skill)
	assert.Equal(t, "my-skill", skill.Name)
	assert.Equal(t, "A custom skill", skill.Description)
	assert.Equal(t, "# My Skill\n\nDo the thing.", skill.Content)
	assert.False(t, skill.IsBuiltin, "newly created skill must not be built-in")
	assert.NotEmpty(t, skill.Id, "skill must have a generated ID")

	// Verify row in DB
	got, err := gdb.GetSkill(skill.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "my-skill", got.Name)
	assert.Equal(t, "A custom skill", got.Description)
}

// ============================================================================
// SC-6: Editing a skill updates content and metadata in the skills table
// ============================================================================

func TestUpdateSkill_UpdatesGlobalDB(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed a custom skill
	err := gdb.SaveSkill(&db.Skill{
		ID:          "edit-skill",
		Name:        "edit-skill",
		Description: "Original",
		Content:     "# Original",
		IsBuiltin:   false,
	})
	require.NoError(t, err)

	newContent := "# Updated Content\n\nWith more detail."
	newDesc := "Updated description"
	req := connect.NewRequest(&orcv1.UpdateSkillRequest{
		Id:          "edit-skill",
		Description: &newDesc,
		Content:     &newContent,
	})

	resp, err := server.UpdateSkill(context.Background(), req)
	require.NoError(t, err)

	skill := resp.Msg.Skill
	require.NotNil(t, skill)
	assert.Equal(t, "Updated description", skill.Description)
	assert.Equal(t, "# Updated Content\n\nWith more detail.", skill.Content)

	// Verify DB
	got, err := gdb.GetSkill("edit-skill")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Updated description", got.Description)
	assert.Equal(t, "# Updated Content\n\nWith more detail.", got.Content)
}

// ============================================================================
// SC-7: Deleting a skill removes it from the skills table
// ============================================================================

func TestDeleteSkill_RemovesFromGlobalDB(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	err := gdb.SaveSkill(&db.Skill{
		ID:        "delete-skill",
		Name:      "delete-skill",
		Content:   "# Delete Me",
		IsBuiltin: false,
	})
	require.NoError(t, err)

	req := connect.NewRequest(&orcv1.DeleteSkillRequest{
		Id: "delete-skill",
	})

	_, err = server.DeleteSkill(context.Background(), req)
	require.NoError(t, err)

	got, err := gdb.GetSkill("delete-skill")
	require.NoError(t, err)
	assert.Nil(t, got, "skill should be removed from GlobalDB after delete")
}

// ============================================================================
// SC-8: Skills page shows Create button - form validation
// ============================================================================

func TestCreateSkill_RequiresName(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	req := connect.NewRequest(&orcv1.CreateSkillRequest{
		Name:    "", // empty name
		Content: "# Something",
	})

	_, err := server.CreateSkill(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestCreateSkill_RequiresContent(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	req := connect.NewRequest(&orcv1.CreateSkillRequest{
		Name:    "valid-name",
		Content: "", // empty content
	})

	_, err := server.CreateSkill(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// ============================================================================
// SC-9: Built-in hooks show "Built-in" badge, edit/delete disabled
// ============================================================================

func TestDeleteHook_BuiltinProtected(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	seedBuiltinHook(t, gdb, "orc-verify", "Verify", "Stop", "#!/bin/bash\nverify")

	req := connect.NewRequest(&orcv1.DeleteHookRequest{
		Id: "orc-verify",
	})

	_, err := server.DeleteHook(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())

	// Verify hook still exists in DB
	got, err := gdb.GetHookScript("orc-verify")
	require.NoError(t, err)
	assert.NotNil(t, got, "built-in hook must not be deleted")
}

func TestUpdateHook_BuiltinProtected(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	seedBuiltinHook(t, gdb, "orc-builtin-update", "Builtin Hook", "Stop", "#!/bin/bash\noriginal")

	newContent := "#!/bin/bash\nhacked"
	req := connect.NewRequest(&orcv1.UpdateHookRequest{
		Id:      "orc-builtin-update",
		Content: &newContent,
	})

	_, err := server.UpdateHook(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())

	// Verify hook unchanged
	got, err := gdb.GetHookScript("orc-builtin-update")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "#!/bin/bash\noriginal", got.Content)
}

func TestUpdateSkill_BuiltinProtected(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	seedBuiltinSkill(t, gdb, "orc-builtin-skill", "Builtin Skill", "# Original")

	newContent := "# Modified"
	req := connect.NewRequest(&orcv1.UpdateSkillRequest{
		Id:      "orc-builtin-skill",
		Content: &newContent,
	})

	_, err := server.UpdateSkill(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())

	// Verify skill unchanged
	got, err := gdb.GetSkill("orc-builtin-skill")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "# Original", got.Content)
}

func TestDeleteSkill_BuiltinProtected(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	seedBuiltinSkill(t, gdb, "orc-protected-skill", "Protected Skill", "# Protected")

	req := connect.NewRequest(&orcv1.DeleteSkillRequest{
		Id: "orc-protected-skill",
	})

	_, err := server.DeleteSkill(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code())

	// Verify skill still exists
	got, err := gdb.GetSkill("orc-protected-skill")
	require.NoError(t, err)
	assert.NotNil(t, got, "built-in skill must not be deleted")
}

// ============================================================================
// SC-9: ListHooks response includes is_builtin field
// ============================================================================

func TestListHooks_IncludesBuiltinFlag(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	seedBuiltinHook(t, gdb, "orc-builtin-list", "Builtin", "Stop", "#!/bin/bash\nbuiltin")
	err := gdb.SaveHookScript(&db.HookScript{
		ID: "custom-list", Name: "Custom", EventType: "PreToolUse",
		Content: "#!/bin/bash\ncustom", IsBuiltin: false,
	})
	require.NoError(t, err)

	resp, err := server.ListHooks(context.Background(), connect.NewRequest(&orcv1.ListHooksRequest{}))
	require.NoError(t, err)

	var builtinCount, customCount int
	for _, h := range resp.Msg.Hooks {
		if h.IsBuiltin {
			builtinCount++
		} else {
			customCount++
		}
	}
	assert.Equal(t, 1, builtinCount, "should have 1 built-in hook")
	assert.Equal(t, 1, customCount, "should have 1 custom hook")
}

// ============================================================================
// SC-10: Cloning a built-in item creates a new editable copy
// ============================================================================

func TestCloneHook_CreatesEditableCopy(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	seedBuiltinHook(t, gdb, "orc-clone-source", "Verify Completion", "Stop", "#!/bin/bash\nverify")

	// Clone by creating a new hook with content from the built-in
	req := connect.NewRequest(&orcv1.CreateHookRequest{
		Name:        "Verify Completion (Copy)",
		Description: "Cloned from built-in",
		Content:     "#!/bin/bash\nverify",
		EventType:   "Stop",
	})

	resp, err := server.CreateHook(context.Background(), req)
	require.NoError(t, err)

	clone := resp.Msg.Hook
	require.NotNil(t, clone)
	assert.Equal(t, "Verify Completion (Copy)", clone.Name)
	assert.False(t, clone.IsBuiltin, "cloned hook must not be built-in")
	assert.Equal(t, "#!/bin/bash\nverify", clone.Content)

	// Original still exists and is still built-in
	original, err := gdb.GetHookScript("orc-clone-source")
	require.NoError(t, err)
	require.NotNil(t, original)
	assert.True(t, original.IsBuiltin)
}

func TestCloneSkill_CreatesEditableCopy(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	seedBuiltinSkill(t, gdb, "orc-tdd-discipline", "TDD Discipline", "# TDD\n\nWrite tests first.")

	req := connect.NewRequest(&orcv1.CreateSkillRequest{
		Name:        "TDD Discipline (Copy)",
		Description: "Cloned from built-in",
		Content:     "# TDD\n\nWrite tests first.",
	})

	resp, err := server.CreateSkill(context.Background(), req)
	require.NoError(t, err)

	clone := resp.Msg.Skill
	require.NotNil(t, clone)
	assert.Equal(t, "TDD Discipline (Copy)", clone.Name)
	assert.False(t, clone.IsBuiltin, "cloned skill must not be built-in")

	// Original unchanged
	original, err := gdb.GetSkill("orc-tdd-discipline")
	require.NoError(t, err)
	require.NotNil(t, original)
	assert.True(t, original.IsBuiltin)
}

// ============================================================================
// SC-11: Hooks grouped by event_type
// ============================================================================

func TestListHooks_GroupedByEventType(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed hooks of different event types
	for _, h := range []struct {
		id, name, eventType string
	}{
		{"stop-hook", "Stop Hook", "Stop"},
		{"pre-hook-1", "Pre Hook 1", "PreToolUse"},
		{"pre-hook-2", "Pre Hook 2", "PreToolUse"},
		{"post-hook", "Post Hook", "PostToolUse"},
	} {
		err := gdb.SaveHookScript(&db.HookScript{
			ID: h.id, Name: h.name, EventType: h.eventType,
			Content: "#!/bin/bash\necho " + h.name, IsBuiltin: false,
		})
		require.NoError(t, err)
	}

	resp, err := server.ListHooks(context.Background(), connect.NewRequest(&orcv1.ListHooksRequest{}))
	require.NoError(t, err)
	assert.Len(t, resp.Msg.Hooks, 4)

	// Verify event_type is set correctly on each hook
	eventCounts := make(map[string]int)
	for _, h := range resp.Msg.Hooks {
		assert.NotEmpty(t, h.EventType, "hook %s missing event_type", h.Name)
		eventCounts[h.EventType]++
	}
	assert.Equal(t, 1, eventCounts["Stop"])
	assert.Equal(t, 2, eventCounts["PreToolUse"])
	assert.Equal(t, 1, eventCounts["PostToolUse"])
}

// ============================================================================
// SC-12: Skills page displays skills in a card grid with built-in badge
// ============================================================================

func TestListSkills_IncludesBuiltinAndCustom(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	seedBuiltinSkill(t, gdb, "builtin-skill-list", "Built-in Skill", "# Builtin")
	err := gdb.SaveSkill(&db.Skill{
		ID: "custom-skill-list", Name: "Custom Skill",
		Description: "A custom skill", Content: "# Custom", IsBuiltin: false,
	})
	require.NoError(t, err)

	resp, err := server.ListSkills(context.Background(), connect.NewRequest(&orcv1.ListSkillsRequest{}))
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(resp.Msg.Skills), 2)

	var foundBuiltin, foundCustom bool
	for _, s := range resp.Msg.Skills {
		if s.Name == "Built-in Skill" {
			foundBuiltin = true
			assert.True(t, s.IsBuiltin)
			assert.NotEmpty(t, s.Id)
		}
		if s.Name == "Custom Skill" {
			foundCustom = true
			assert.False(t, s.IsBuiltin)
			assert.Equal(t, "A custom skill", s.Description)
		}
	}
	assert.True(t, foundBuiltin, "built-in skill missing from list")
	assert.True(t, foundCustom, "custom skill missing from list")
}

// ============================================================================
// Failure Modes
// ============================================================================

// TestCreateHook_DuplicateName verifies that creating a hook with a name
// that already exists returns an error.
func TestCreateHook_DuplicateName(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	// Seed existing hook
	err := gdb.SaveHookScript(&db.HookScript{
		ID: "existing-hook", Name: "existing-hook",
		Content: "#!/bin/bash\nexisting", EventType: "Stop", IsBuiltin: false,
	})
	require.NoError(t, err)

	req := connect.NewRequest(&orcv1.CreateHookRequest{
		Name:      "existing-hook",
		Content:   "#!/bin/bash\nduplicate",
		EventType: "Stop",
	})

	_, err = server.CreateHook(context.Background(), req)
	require.Error(t, err, "creating hook with duplicate name should fail")

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())
}

// TestCreateSkill_DuplicateName verifies that creating a skill with a name
// that already exists returns an error.
func TestCreateSkill_DuplicateName(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	err := gdb.SaveSkill(&db.Skill{
		ID: "existing-skill", Name: "existing-skill",
		Content: "# Existing", IsBuiltin: false,
	})
	require.NoError(t, err)

	req := connect.NewRequest(&orcv1.CreateSkillRequest{
		Name:    "existing-skill",
		Content: "# Duplicate",
	})

	_, err = server.CreateSkill(context.Background(), req)
	require.Error(t, err, "creating skill with duplicate name should fail")

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeAlreadyExists, connectErr.Code())
}

// TestUpdateHook_NotFound verifies updating a non-existent hook returns NotFound.
func TestUpdateHook_NotFound(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	newContent := "#!/bin/bash\nnew"
	req := connect.NewRequest(&orcv1.UpdateHookRequest{
		Id:      "nonexistent-hook",
		Content: &newContent,
	})

	_, err := server.UpdateHook(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

// TestDeleteHook_NotFound verifies deleting a non-existent hook returns NotFound.
func TestDeleteHook_NotFound(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	req := connect.NewRequest(&orcv1.DeleteHookRequest{
		Id: "nonexistent-hook",
	})

	_, err := server.DeleteHook(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

// TestUpdateSkill_NotFound verifies updating a non-existent skill returns NotFound.
func TestUpdateSkill_NotFound(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	newContent := "# Updated"
	req := connect.NewRequest(&orcv1.UpdateSkillRequest{
		Id:      "nonexistent-skill",
		Content: &newContent,
	})

	_, err := server.UpdateSkill(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

// TestDeleteSkill_NotFound verifies deleting a non-existent skill returns NotFound.
func TestDeleteSkill_NotFound(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	req := connect.NewRequest(&orcv1.DeleteSkillRequest{
		Id: "nonexistent-skill",
	})

	_, err := server.DeleteSkill(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeNotFound, connectErr.Code())
}

// ============================================================================
// Edge Cases
// ============================================================================

// TestListHooks_EmptyDB returns empty list when no hooks exist.
func TestListHooks_EmptyDB(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	resp, err := server.ListHooks(context.Background(), connect.NewRequest(&orcv1.ListHooksRequest{}))
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Hooks)
}

// TestListSkills_EmptyDB returns empty list when no skills exist.
func TestListSkills_EmptyDB(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	resp, err := server.ListSkills(context.Background(), connect.NewRequest(&orcv1.ListSkillsRequest{}))
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.Skills)
}

// TestCreateSkill_LargeContent verifies >10KB content is saved correctly.
func TestCreateSkill_LargeContent(t *testing.T) {
	t.Parallel()
	server, gdb := newTestConfigServerWithGlobalDB(t)

	largeContent := ""
	for i := 0; i < 200; i++ {
		largeContent += "## Section " + string(rune('A'+i%26)) + "\n\nThis is a large skill with lots of content for testing purposes.\n\n"
	}
	require.Greater(t, len(largeContent), 10000)

	req := connect.NewRequest(&orcv1.CreateSkillRequest{
		Name:    "large-skill",
		Content: largeContent,
	})

	resp, err := server.CreateSkill(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, largeContent, resp.Msg.Skill.Content)

	// Verify in DB
	got, err := gdb.GetSkill(resp.Msg.Skill.Id)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, largeContent, got.Content)
}

// TestCreateHook_RequiresName verifies empty name is rejected.
func TestCreateHook_RequiresName(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	req := connect.NewRequest(&orcv1.CreateHookRequest{
		Name:      "",
		Content:   "#!/bin/bash\necho test",
		EventType: "Stop",
	})

	_, err := server.CreateHook(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// TestCreateHook_RequiresContent verifies empty content is rejected.
func TestCreateHook_RequiresContent(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	req := connect.NewRequest(&orcv1.CreateHookRequest{
		Name:      "no-content-hook",
		Content:   "",
		EventType: "Stop",
	})

	_, err := server.CreateHook(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// TestCreateHook_RequiresEventType verifies empty event_type is rejected.
func TestCreateHook_RequiresEventType(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	req := connect.NewRequest(&orcv1.CreateHookRequest{
		Name:      "no-event-hook",
		Content:   "#!/bin/bash\necho test",
		EventType: "",
	})

	_, err := server.CreateHook(context.Background(), req)
	require.Error(t, err)

	connectErr := new(connect.Error)
	require.ErrorAs(t, err, &connectErr)
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

// ============================================================================
// Integration: Full CRUD cycle
// ============================================================================

// TestHookCRUDIntegration verifies the full create → list → update → delete cycle.
func TestHookCRUDIntegration(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	// Create
	createResp, err := server.CreateHook(context.Background(), connect.NewRequest(&orcv1.CreateHookRequest{
		Name:        "integration-hook",
		Description: "Integration test hook",
		Content:     "#!/bin/bash\necho integration",
		EventType:   "Stop",
	}))
	require.NoError(t, err)
	hookID := createResp.Msg.Hook.Id
	require.NotEmpty(t, hookID)

	// List - verify it appears
	listResp, err := server.ListHooks(context.Background(), connect.NewRequest(&orcv1.ListHooksRequest{}))
	require.NoError(t, err)
	var found bool
	for _, h := range listResp.Msg.Hooks {
		if h.Id == hookID {
			found = true
			assert.Equal(t, "integration-hook", h.Name)
			break
		}
	}
	require.True(t, found, "created hook not found in list")

	// Update
	newDesc := "Updated integration hook"
	_, err = server.UpdateHook(context.Background(), connect.NewRequest(&orcv1.UpdateHookRequest{
		Id:          hookID,
		Description: &newDesc,
	}))
	require.NoError(t, err)

	// List again - verify update
	listResp2, err := server.ListHooks(context.Background(), connect.NewRequest(&orcv1.ListHooksRequest{}))
	require.NoError(t, err)
	for _, h := range listResp2.Msg.Hooks {
		if h.Id == hookID {
			assert.Equal(t, "Updated integration hook", h.Description)
			break
		}
	}

	// Delete
	_, err = server.DeleteHook(context.Background(), connect.NewRequest(&orcv1.DeleteHookRequest{
		Id: hookID,
	}))
	require.NoError(t, err)

	// List again - verify gone
	listResp3, err := server.ListHooks(context.Background(), connect.NewRequest(&orcv1.ListHooksRequest{}))
	require.NoError(t, err)
	for _, h := range listResp3.Msg.Hooks {
		assert.NotEqual(t, hookID, h.Id, "deleted hook still in list")
	}
}

// TestSkillCRUDIntegration verifies the full create → list → update → delete cycle.
func TestSkillCRUDIntegration(t *testing.T) {
	t.Parallel()
	server, _ := newTestConfigServerWithGlobalDB(t)

	// Create
	createResp, err := server.CreateSkill(context.Background(), connect.NewRequest(&orcv1.CreateSkillRequest{
		Name:        "integration-skill",
		Description: "Integration test skill",
		Content:     "# Integration Skill\n\nContent here.",
	}))
	require.NoError(t, err)
	skillID := createResp.Msg.Skill.Id
	require.NotEmpty(t, skillID)

	// List - verify it appears
	listResp, err := server.ListSkills(context.Background(), connect.NewRequest(&orcv1.ListSkillsRequest{}))
	require.NoError(t, err)
	var found bool
	for _, s := range listResp.Msg.Skills {
		if s.Id == skillID {
			found = true
			assert.Equal(t, "integration-skill", s.Name)
			break
		}
	}
	require.True(t, found, "created skill not found in list")

	// Update
	newContent := "# Updated Integration Skill\n\nNew content."
	_, err = server.UpdateSkill(context.Background(), connect.NewRequest(&orcv1.UpdateSkillRequest{
		Id:      skillID,
		Content: &newContent,
	}))
	require.NoError(t, err)

	// Delete
	_, err = server.DeleteSkill(context.Background(), connect.NewRequest(&orcv1.DeleteSkillRequest{
		Id: skillID,
	}))
	require.NoError(t, err)

	// List again - verify gone
	listResp2, err := server.ListSkills(context.Background(), connect.NewRequest(&orcv1.ListSkillsRequest{}))
	require.NoError(t, err)
	for _, s := range listResp2.Msg.Skills {
		assert.NotEqual(t, skillID, s.Id, "deleted skill still in list")
	}
}
