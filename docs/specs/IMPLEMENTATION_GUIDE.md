# Implementation Guide

**Status**: Planning
**Last Updated**: 2026-01-10

---

## Overview

This guide outlines how to implement the orc v1.0 features in a logical order, with dependencies mapped.

---

## Dependency Graph

```
                    ┌─────────────────────────────────────┐
                    │         Error Standards             │
                    │           (foundation)              │
                    └─────────────┬───────────────────────┘
                                  │
           ┌──────────────────────┼──────────────────────┐
           │                      │                      │
           ▼                      ▼                      ▼
┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐
│   Init Wizard       │ │ Session Interop     │ │   Cost Tracking     │
│                     │ │                     │ │                     │
└─────────┬───────────┘ └─────────┬───────────┘ └─────────┬───────────┘
          │                       │                       │
          ▼                       ▼                       ▼
┌─────────────────────┐ ┌─────────────────────┐ ┌─────────────────────┐
│ Project Detection   │ │ Task Enhancement    │ │  Web Dashboard      │
│                     │ │                     │ │                     │
└─────────┬───────────┘ └─────────┬───────────┘ └─────────────────────┘
          │                       │
          ▼                       ▼
┌─────────────────────┐ ┌─────────────────────┐
│  Skill Installation │ │  Task Templates     │
│                     │ │                     │
└─────────────────────┘ └─────────────────────┘
          │                       │
          └───────────┬───────────┘
                      ▼
          ┌─────────────────────┐
          │ Cross-Project       │
          │ Resources           │
          └─────────────────────┘
                      │
                      ▼
          ┌─────────────────────┐
          │ Keyboard Shortcuts  │
          │ (UI polish)         │
          └─────────────────────┘
                      │
                      ▼
          ┌─────────────────────┐
          │ TUI Watch Mode      │
          │ (advanced)          │
          └─────────────────────┘
```

---

## Phase 1: Foundation (Week 1-2)

### 1.1 Error Standards

**Why First**: Every subsequent feature needs good error handling.

**Files to Modify**:
- Create `internal/errors/errors.go` - Error types and constructors
- Update `internal/cli/*.go` - Use new error types
- Update `internal/api/server.go` - JSON error responses

**Deliverables**:
- [ ] `OrcError` type with code, what, why, fix
- [ ] Error constructors for all error codes
- [ ] CLI prints user-friendly errors
- [ ] API returns structured JSON errors
- [ ] At least 20 error messages updated

**Acceptance Test**:
```bash
# Should show helpful error
$ orc run TASK-999
❌ Task TASK-999 not found

No task with this ID exists in the current project.

To fix:
  orc list                  # See all tasks
  orc new "title"           # Create a new task
```

---

### 1.2 Session Interoperability

**Why Early**: Core UX requirement for CLI/UI switching.

**Files to Modify**:
- Update `internal/state/state.go` - Add session fields
- Update `internal/executor/executor.go` - Capture session ID
- Create `internal/cli/session.go` - Session commands
- Update `internal/api/server.go` - Session endpoint

**Deliverables**:
- [ ] Session ID captured on Claude start
- [ ] Session ID stored in state.yaml
- [ ] `orc session TASK-ID` shows session info
- [ ] `GET /api/tasks/:id/session` endpoint
- [ ] Web UI shows resume options when paused

**Acceptance Test**:
```bash
# Run, pause, check session
$ orc run TASK-001
# Ctrl+C to pause

$ orc session TASK-001
Session: 550e8400-e29b-41d4-a716-446655440000
Resume with: claude --resume 550e8400-...
```

---

### 1.3 Cost Tracking

**Why Early**: Needed for dashboard and visibility.

**Files to Modify**:
- Update `internal/state/state.go` - Token tracking
- Update `internal/executor/executor.go` - Capture token usage
- Create `internal/cost/cost.go` - Cost calculations
- Create `internal/cli/cost.go` - Cost command
- Update `internal/api/server.go` - Token endpoints

**Deliverables**:
- [ ] Tokens captured per iteration
- [ ] Aggregation by phase and task
- [ ] `orc show TASK-ID` displays tokens/cost
- [ ] `orc cost` shows summary
- [ ] API returns token data

**Acceptance Test**:
```bash
$ orc show TASK-001
...
Token Usage:
  Input:    45,234 tokens
  Output:   12,456 tokens
  Total:    67,690 tokens
Estimated Cost: $1.47 USD
```

---

## Phase 2: Task Flow (Week 3-4)

### 2.1 Init Wizard

**Dependencies**: Error Standards

**Files to Modify**:
- Rewrite `internal/cli/commands.go:newInitCmd`
- Create `internal/wizard/wizard.go` - Interactive prompts
- Create `internal/detect/detect.go` - Project detection
- Update `internal/config/config.go` - Profile application

**Deliverables**:
- [ ] Interactive prompts with arrow key selection
- [ ] Project type detection
- [ ] Profile selection
- [ ] Completion action selection
- [ ] `--quick` flag for non-interactive
- [ ] CLAUDE.md section generation

**Acceptance Test**:
```bash
$ orc init
# Interactive wizard runs
# .orc/config.yaml created with selections
# CLAUDE.md updated with orc section
```

---

### 2.2 Project Detection

**Dependencies**: Init Wizard (integrated)

**Files to Create**:
- `internal/detect/language.go` - Language detection
- `internal/detect/framework.go` - Framework detection
- `internal/detect/tools.go` - Tool detection

**Deliverables**:
- [ ] Detect Go, TypeScript, Python, Rust
- [ ] Detect major frameworks
- [ ] Set appropriate test/lint commands
- [ ] Works during init wizard

---

### 2.3 Task Enhancement

**Dependencies**: Session Interoperability (uses sessions)

**Files to Modify**:
- Rewrite `internal/cli/commands.go:newNewCmd`
- Create `internal/enhance/enhance.go` - Enhancement logic
- Create `templates/prompts/enhance.md` - Enhancement prompt
- Update `internal/task/task.go` - Enhanced task fields

**Deliverables**:
- [ ] `--weight` skips enhancement
- [ ] Default mode runs automatic enhancement
- [ ] `-i` opens interactive Claude session
- [ ] Enhanced data stored in task.yaml
- [ ] Web UI supports enhancement modes

**Acceptance Test**:
```bash
$ orc new "Fix auth timeout bug"
Starting task enhancement...
# Claude analyzes, suggests weight, expands description
Task created: TASK-001 (small)
```

---

### 2.4 Task Templates

**Dependencies**: None (can be parallel)

**Files to Create**:
- `internal/template/template.go` - Template loading
- `internal/template/save.go` - Save from task
- `internal/cli/template.go` - Template commands
- `internal/api/templates.go` - API handlers

**Deliverables**:
- [ ] `orc template save TASK-ID --name X`
- [ ] `orc template list`
- [ ] `orc new --template X "title"`
- [ ] Variables in templates
- [ ] Global vs project templates

---

## Phase 3: UI Polish (Week 5-6)

### 3.1 Web Dashboard

**Dependencies**: Cost Tracking (shows tokens)

**Files to Create**:
- `web/src/routes/+page.svelte` - Dashboard (replace task list)
- `web/src/lib/components/dashboard/` - Dashboard components
- Create `internal/api/dashboard.go` - Dashboard endpoint

**Deliverables**:
- [ ] Quick stats widget
- [ ] Active tasks section
- [ ] Recent activity feed
- [ ] Quick actions bar
- [ ] Real-time updates via WebSocket

---

### 3.2 Keyboard Shortcuts

**Dependencies**: None

**Files to Create**:
- `web/src/lib/shortcuts.ts` - Shortcut manager
- `web/src/lib/components/overlays/ShortcutsHelp.svelte`
- Update all pages with shortcuts

**Deliverables**:
- [ ] Global shortcuts (⌘K, n, g+letter)
- [ ] Task list navigation (j/k)
- [ ] `?` shows help
- [ ] Visual hints on buttons

---

### 3.3 Web UI Notifications

**Dependencies**: Dashboard

**Files to Create**:
- `web/src/lib/components/overlays/Notifications.svelte`
- `web/src/lib/stores/notifications.ts`

**Deliverables**:
- [ ] Toast notifications for events
- [ ] Notification center
- [ ] Clear all button
- [ ] Persists across page navigation

---

## Phase 4: Advanced (Week 7-8)

### 4.1 Cross-Project Resources

**Dependencies**: Init Wizard, Templates

**Files to Modify**:
- Update all resource loaders for hierarchy
- Create `internal/global/global.go` - Global resource management
- Update CLI commands to show sources

**Deliverables**:
- [ ] Global skills in ~/.orc/skills/
- [ ] Global templates in ~/.orc/templates/
- [ ] Config merging
- [ ] `--global` flag on relevant commands
- [ ] Source indicators in UI

---

### 4.2 TUI Watch Mode

**Dependencies**: All other features (uses them all)

**Files to Create**:
- `internal/tui/` - Full TUI implementation
- Uses bubbletea, lipgloss, bubbles

**Deliverables**:
- [ ] `orc watch` command
- [ ] Task list with vim navigation
- [ ] Transcript viewer
- [ ] Real-time updates
- [ ] Help modal

---

## Testing Strategy

### Unit Tests

Each feature needs unit tests:
- Error constructors
- Config merging
- Detection logic
- Template rendering
- Cost calculations

### Integration Tests

End-to-end scenarios:
- Init wizard flow
- Task enhancement flow
- Session resume flow
- Template create/use flow

### E2E Tests (Playwright)

Web UI tests:
- Dashboard loads correctly
- Keyboard shortcuts work
- Notifications appear
- Task actions work

---

## Migration Notes

### Existing Users

For users with existing `.orc/` directories:

1. **Config Format**: Add new fields with defaults
2. **State Format**: Add session field with null default
3. **No Breaking Changes**: Old configs continue to work

### Database Consideration

Per ADR-002, we're staying with YAML files:
- Sufficient for single-user workflows
- Git-trackable
- No dependency

SQLite only if:
- Query performance becomes an issue
- Need cross-task search
- Users have 100+ tasks

---

## Estimates

| Feature | Effort | Dependencies |
|---------|--------|--------------|
| Error Standards | 1 week | None |
| Session Interop | 1 week | Errors |
| Cost Tracking | 1 week | Errors |
| Init Wizard | 1 week | Errors |
| Project Detection | 0.5 week | Init Wizard |
| Task Enhancement | 1 week | Session Interop |
| Task Templates | 1 week | None |
| Web Dashboard | 1 week | Cost Tracking |
| Keyboard Shortcuts | 0.5 week | None |
| Notifications | 0.5 week | Dashboard |
| Cross-Project | 1 week | Templates |
| TUI Watch | 2 weeks | All |

**Total**: ~12 weeks for full implementation

---

## Open Questions to Resolve

1. **Task Enhancement Mode**: Default to automatic or interactive?
   - Suggestion: Automatic by default, `-i` for interactive

2. **Session Storage**: Separate file or in state.yaml?
   - Suggestion: In state.yaml (simpler)

3. **Cost Display**: Show estimated or actual?
   - Suggestion: Estimated based on pricing.yaml

4. **Skill Sources**: Fetch from Anthropic or bundle?
   - Suggestion: Bundle common, fetch optional

5. **Config Merge Strategy**: Deep merge or shallow?
   - Suggestion: Deep merge with explicit null for removal

---

## Rollout Plan

### Alpha (Internal Testing)

- Implement P0 features
- Test with orc project itself
- Iterate on UX

### Beta (Early Users)

- Implement P1 features
- Documentation
- Bug fixes

### v1.0 (Public Release)

- All P0 and P1 features
- P2 features as available
- Full documentation
- Example configurations
