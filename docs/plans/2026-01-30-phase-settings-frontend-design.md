# Phase Settings Frontend Design

**Date:** 2026-01-30
**Status:** Design
**Depends on:** TASK-666 (infrastructure), TASK-667 (migration)

## Problem Statement

The backend is moving to a unified per-phase settings system (hooks, MCP servers, skills, env vars) managed through `ApplyPhaseSettings`. The frontend needs to support:

1. Managing the library of available hooks, skills, and MCP servers
2. Assigning library items to phase templates as defaults
3. Overriding those defaults per-workflow
4. Exporting/importing between GlobalDB and `.claude/` directories

Currently, environment pages are a mix of CRUD and read-only views that talk directly to `.claude/` files — flaky and incomplete. This redesign shifts them to proper GlobalDB-backed library management.

## Architecture: Three Layers + Export/Import

```
┌─────────────────────────────────────────────────┐
│  Layer 1: Library (Environment Pages)           │
│  GlobalDB CRUD — source of truth                │
│  + Export/Import sub-tab for .claude/ sync      │
├─────────────────────────────────────────────────┤
│  Layer 2: Phase Template Defaults               │
│  Assign library items to phase templates        │
│  (claude_config in phase_templates table)        │
├─────────────────────────────────────────────────┤
│  Layer 3: Workflow Phase Overrides              │
│  Override template defaults per-workflow         │
│  (claude_config_override in workflow phases)     │
└─────────────────────────────────────────────────┘
```

Maps to existing backend merge: `getEffectivePhaseClaudeConfig()` merges template `claude_config` with workflow `claude_config_override`.

## Layer 1: Environment Pages → Library Management

### Page Transformation

| Page | Current State | New Role |
|------|---------------|----------|
| **Hooks** | CRUD against `configClient` (reads .claude/) | Library CRUD for `hook_scripts` in GlobalDB |
| **Skills** | Read-only listing | Library CRUD for `skills` in GlobalDB |
| **MCP** | CRUD against `mcpClient` (reads .claude/) | Library CRUD for MCP server configs in GlobalDB |
| **Agents** | Read-only listing | Stays read-only (agents already seeded properly) |

Each page has two tabs:

1. **Library** (default) — CRUD for GlobalDB entities
2. **Export/Import** — Sync between GlobalDB and filesystem

### Library Tab

Standard CRUD with project/global scope (existing pattern). Each item shows:
- Name, description
- Type-specific details (hook event type, skill frontmatter, MCP command)
- Built-in badge for seeded items (not editable)
- Edit/delete for user-created items

### Export/Import Tab

**Export section:**

```
Export to:  [Project .claude/] [User ~/.claude/]

☑ orc-verify-completion     Stop hook
☑ orc-tdd-discipline        PreToolUse hook
☐ orc-worktree-isolation    PreToolUse hook

[Export Selected]
```

**Import section:**

```
Scan:  [Project .claude/] [User ~/.claude/]  [Scan]

Found 2 items not in library:
  ☑ custom-lint-hook.sh      PreToolUse hook
  ☐ my-skill/SKILL.md        Skill

[Import Selected to Library]
```

**Sync indicators:**
- Items already in library and matching filesystem: "synced" badge
- Items differing between library and filesystem: "modified" warning with diff option
- Items only in one place: "export" or "import" action

## Layer 2: Phase Template Editor

### Collapsible Settings Sections

All sections collapsed by default. Badge shows count of active items.

```
▶ Hooks (2 active)
▶ MCP Servers (1 active)
▶ Skills (0)
▶ Allowed Tools (3)
▶ Disallowed Tools (0)
▶ Env Vars (2)
▶ JSON Override
```

### Section Details

| Section | UI Element | Behavior |
|---------|------------|----------|
| **Hooks** | Multi-select picker | Grouped by event type (Stop, PreToolUse, PostToolUse). Pick from hook_scripts library. |
| **MCP Servers** | Multi-select picker | Pick from MCP library. Shows command preview. |
| **Skills** | Multi-select picker | Pick from skills library. Shows description. |
| **Allowed Tools** | Tag input / chip list | Free-text entry for tool names. |
| **Disallowed Tools** | Tag input / chip list | Free-text entry for tool names. |
| **Env Vars** | Key-value editor | Add/remove key-value pairs. |
| **JSON Override** | Expandable code editor | Full `claude_config` JSON. Stays in sync with structured fields above. |

### JSON Override Behavior

- Opening the JSON editor renders current state from all structured fields
- Editing JSON directly updates the structured fields where parseable
- Unparseable custom JSON shows a "custom JSON" badge on the structured section
- JSON is the escape hatch — no validation beyond valid JSON

## Layer 3: Workflow Phase Overrides

Same collapsible sections as phase template editor, with visual distinction between inherited and overridden values.

### Visual States

| State | Appearance | Meaning |
|-------|------------|---------|
| **Inherited** | Dimmed + "from [template] template" badge | Using template default |
| **Overridden** | Normal + "override" badge | Custom for this workflow |
| **Clear button** | Per-section | Reset to template default |

### Badge Display

```
▶ Hooks (2 — 1 inherited, 1 override)
▶ MCP Servers (1 inherited)
▶ Skills (1 override)
```

Expanding a section shows:
- Inherited items: dimmed, not directly editable (go to template to change)
- Override items: normal, fully editable
- Add button: adds workflow-specific override
- Clear override button: removes all overrides for that section, falls back to template

## Component Architecture

### New Shared Components

| Component | Purpose |
|-----------|---------|
| `CollapsibleSettingsSection` | Collapsible section with badge count. Used in phase template editor and workflow phase editor. |
| `LibraryPicker` | Multi-select picker that lists library items from GlobalDB. Filters by type (hooks by event, etc). |
| `TagInput` | Chip-style input for free-text lists (allowed/disallowed tools). |
| `KeyValueEditor` | Add/remove key-value pairs (env vars). |
| `JsonOverrideEditor` | Code editor with sync-to-structured and sync-from-structured. |
| `ExportImportPanel` | Checkbox list with export/import actions and sync indicators. |

### Page Changes

| Page | Changes |
|------|---------|
| `Hooks.tsx` | Switch from `configClient` to `hookScriptClient` (GlobalDB). Add Export/Import tab. |
| `Skills.tsx` | Switch from read-only `ListSkills` to full CRUD via `skillClient` (GlobalDB). Add Export/Import tab. |
| `Mcp.tsx` | Switch from `mcpClient` to GlobalDB-backed client. Add Export/Import tab. |
| `Agents.tsx` | No changes (already read-only, already GlobalDB-backed). |
| Phase template editor | Add collapsible settings sections with library pickers. |
| Workflow phase editor | Add collapsible settings sections with inherited/override distinction. |

## API Dependencies

### Required from TASK-666/667

| Endpoint | Purpose |
|----------|---------|
| `HookScriptService.List/Create/Update/Delete` | CRUD for hook_scripts table |
| `SkillService.List/Create/Update/Delete` | CRUD for skills table |
| `HookScriptService.Export/Import` | Write to / read from `.claude/` directories |
| `SkillService.Export/Import` | Write to / read from `.claude/` directories |

### Existing endpoints to update

| Endpoint | Change |
|----------|--------|
| `PhaseTemplateService.Update` | Accept `claude_config.hooks`, `claude_config.skill_refs` |
| `WorkflowService.Update` | Accept `claude_config_override.hooks` per phase |

## Migration Notes

- Environment pages currently read `.claude/` files via `configClient` / `mcpClient` — these become library CRUD against GlobalDB
- Export/Import replaces the old direct-file behavior as an explicit user action
- Phase template and workflow editors gain new sections but existing fields unchanged
- No breaking changes to existing workflow JSON — `claude_config` and `claude_config_override` fields already exist, just get new sub-fields

## Testing

- Unit test collapsible sections render correct badge counts
- Unit test library picker filters by type
- Unit test JSON override stays in sync with structured fields
- Integration test export writes correct files to `.claude/`
- Integration test import reads and creates GlobalDB entries
- E2E: create hook in library → assign to phase template → verify in workflow editor
