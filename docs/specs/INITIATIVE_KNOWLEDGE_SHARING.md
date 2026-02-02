# Initiative Knowledge Sharing

## Goal

Enable tasks within an initiative to share learnings, patterns, warnings, and handoffs so future tasks benefit from prior work without context rot.

## Approach

Unified notes system at the initiative level. Humans add notes directly; agents add notes via a specialized "knowledge curator" sub-agent spawned during the docs phase. All notes flow into task prompts via `{{INITIATIVE_CONTEXT}}`.

## Success Criteria

- [ ] Initiative notes table exists with proper schema
- [ ] Docs phase spawns knowledge curator sub-agent when task has initiative
- [ ] Sub-agent follows strict guidelines (only captures what meets the bar)
- [ ] Notes are injected into `{{INITIATIVE_CONTEXT}}` for all initiative tasks
- [ ] CLI can list/add/view notes (`orc initiative notes`)
- [ ] Web UI displays notes on initiative and task detail views
- [ ] Existing initiative notes visible to sub-agent (no duplicates)

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| Initiative-level storage (not task comments) | Keeps initiative knowledge together, avoids legacy entanglement |
| Sub-agent for extraction | Focused attention, specialized instructions, clean separation from docs work |
| Sonnet model for sub-agent | Good balance of quality and cost for knowledge extraction |
| Human notes always inject | If a human wrote it, it matters |
| Agent notes must meet strict bar | Prevents context rot from low-value notes |
| Grouped by type in prompts | Easy scanning, clear categories |

## Non-Goals

- Complex semantic search over notes
- Auto-graduation to permanent docs (future enhancement)
- Cross-initiative note sharing
- Note versioning/history

## Data Model

### New Table: `initiative_notes`

```sql
CREATE TABLE initiative_notes (
  id TEXT PRIMARY KEY,
  initiative_id TEXT NOT NULL,

  -- Author
  author TEXT NOT NULL,
  author_type TEXT NOT NULL,      -- 'human' | 'agent'
  source_task TEXT,               -- TASK-001 (if agent-generated)
  source_phase TEXT,              -- 'docs' (if agent-generated)

  -- Content
  note_type TEXT NOT NULL,        -- 'pattern' | 'warning' | 'learning' | 'handoff'
  content TEXT NOT NULL,
  relevant_files TEXT,            -- JSON array, optional

  -- Lifecycle
  graduated BOOL DEFAULT FALSE,

  created_at TEXT DEFAULT (datetime('now')),

  FOREIGN KEY (initiative_id) REFERENCES initiatives(id) ON DELETE CASCADE
);

CREATE INDEX idx_initiative_notes_initiative ON initiative_notes(initiative_id);
CREATE INDEX idx_initiative_notes_type ON initiative_notes(initiative_id, note_type);
```

## Agent Flow

### Docs Phase (when `INITIATIVE_ID` exists)

```
Docs agent starts
    ↓
Spawns "knowledge curator" sub-agent
    ↓
Sub-agent receives:
  - Task context (ID, title, what was implemented)
  - Initiative context (vision, decisions, existing notes)
  - Review summary (what changed after review)
  - Guidelines for what qualifies
    ↓
Sub-agent returns:
  {
    "notes": [
      {"type": "pattern", "content": "...", "relevant_files": [...]}
    ],
    "rationale": "Why these (or none)"
  }
    ↓
Docs agent incorporates into output schema
    ↓
Executor persists notes to initiative_notes table
```

### Knowledge Curator Guidelines

These guidelines are injected into the sub-agent prompt:

```markdown
## Initiative Note Guidelines

Notes are injected into future task prompts. Write for an agent who has NO context
about your task - they only see the note text.

### Format Rule

**Concise but self-contained.** Each note must make sense without reading your code.

| Too vague | Too verbose | Right |
|-----------|-------------|-------|
| "Use the pattern" | "I implemented a repository pattern where all database access goes through a Repository interface defined in internal/repo/ with methods like Get, List, Save, Delete that abstract the underlying SQLite storage..." | "All data access uses Repository pattern (internal/repo/) - don't call DB directly" |
| "Watch out for handler" | "The legacy_handler.go file has some implicit state that gets initialized on first call and if you modify it without understanding the full initialization sequence you might break things" | "legacy_handler.go has implicit init-on-first-call state - read entire file before modifying" |

### When to Add

| Type | Add when... | Example |
|------|-------------|---------|
| **Pattern** | You established a convention to follow | "All validators in pkg/validate/, return (bool, []string) for (valid, errors)" |
| **Warning** | Something non-obvious could cause bugs/waste time | "auth_middleware.go caches user - changes require server restart in dev" |
| **Handoff** | Specific continuation point for dependent task | "TASK-003: validation stubbed at validator.go:47 with TODO marker" |
| **Learning** | Codebase quirk future tasks need | "CI requires Redis running - tests skip gracefully but integration fails" |

### Do NOT Add

- Progress updates ("finished X")
- Obvious observations ("the auth code handles auth")
- Temporary findings ("found bug, fixed it")
- Anything requiring your task's context to understand
```

## Prompt Injection

### Updated `formatInitiativeContext()`

The existing function in `internal/variable/resolver.go` will be extended to include notes:

```markdown
## Initiative Context

This task is part of **User Authentication** (INIT-001).

### Vision

JWT-based auth with refresh tokens, bcrypt for passwords.

### Decisions

- **DEC-001**: Use bcrypt for passwords (Industry standard)
- **DEC-002**: JWT with refresh tokens (Stateless auth)

### Notes from Previous Work

**Patterns:**
- [TASK-001] Using repository pattern for all data access
- [TASK-002] Error responses follow RFC 7807 format

**Warnings:**
- [TASK-001] Don't modify legacy_handler.go - implicit state dependencies

**Learnings:**
- [TASK-002] Test suite requires Redis running locally

**Alignment**: Ensure your work aligns with the initiative vision, respects prior decisions, and follows established patterns.
```

### Implementation

```go
// In loadInitiativeContext() - add after decisions loading
notes, err := backend.GetInitiativeNotes(initiativeID)
if err == nil && len(notes) > 0 {
    rctx.InitiativeNotes = formatInitiativeNotes(notes)
}

// New function in variable/resolver.go
func formatInitiativeNotes(notes []db.InitiativeNote) string {
    // Group by type
    byType := map[string][]db.InitiativeNote{}
    for _, n := range notes {
        byType[n.NoteType] = append(byType[n.NoteType], n)
    }

    var sb strings.Builder
    sb.WriteString("### Notes from Previous Work\n\n")

    for _, noteType := range []string{"pattern", "warning", "learning", "handoff"} {
        if notes, ok := byType[noteType]; ok && len(notes) > 0 {
            sb.WriteString(fmt.Sprintf("**%ss:**\n", strings.Title(noteType)))
            for _, n := range notes {
                fmt.Fprintf(&sb, "- [%s] %s\n", n.SourceTask, n.Content)
            }
            sb.WriteString("\n")
        }
    }
    return sb.String()
}
```

## CLI Commands

```bash
# List notes for initiative
orc initiative notes INIT-001
orc initiative notes INIT-001 --type pattern
orc initiative notes INIT-001 --json

# Add note manually (human)
orc initiative note INIT-001 --type warning "Don't touch legacy_handler.go"
orc initiative note INIT-001 --type pattern "All validators go in pkg/validate/"

# View notes a task generated
orc show TASK-001 --notes

# Delete a note
orc initiative note delete NOTE-001
```

## Web UI

| View | Notes Display |
|------|---------------|
| Initiative detail page | "Knowledge" tab showing all notes grouped by type |
| Task detail page | "Notes Generated" section (if task created any) |
| Task execution log | Note creation events in activity feed |

## Testing Strategy

| Test | Validates |
|------|-----------|
| Unit: note storage CRUD | DB operations work correctly |
| Unit: formatInitiativeNotes | Proper markdown formatting, grouping |
| Unit: injection into context | Notes appear in resolved variables |
| Integration: docs phase with initiative | Sub-agent spawns, notes persisted |
| Integration: note deduplication | Sub-agent sees existing notes, doesn't repeat |
| E2E: full task flow | Notes from TASK-001 visible in TASK-002's prompts |

## References

- [Anthropic: Effective Context Engineering](https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents)
- [Anthropic: Multi-Agent Research System](https://www.anthropic.com/engineering/multi-agent-research-system)
- [Anthropic: Long-Running Agent Harnesses](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents)
