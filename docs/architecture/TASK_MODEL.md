# Task Model

**Purpose**: Define task structure, weight classification, and lifecycle states.

---

## Task Definition

Tasks are stored in the SQLite database (`tasks` table). Example:

```yaml
id: TASK-001
title: "Add user authentication"
description: |
  Implement OAuth2 authentication with Google and GitHub providers.
  Should integrate with existing user model.

weight: medium           # trivial | small | medium | large | greenfield
status: running          # created | classifying | planned | running | paused | blocked | completed | failed
current_phase: implement # Phase currently being executed (updated by executor)
branch: orc/TASK-001

# Task organization (for UI display and filtering)
queue: active            # active | backlog
priority: normal         # critical | high | normal | low
category: feature        # feature | bug | refactor | chore | docs | test

# Initiative linking (optional)
initiative_id: INIT-001  # Links to initiative, empty = standalone

# Task dependencies
blocked_by:              # Tasks that must complete first (stored)
  - TASK-060
  - TASK-061
related_to:              # Related tasks for reference (stored)
  - TASK-063

# Testing flags (auto-detected during task creation)
requires_ui_testing: true        # Auto-set when task mentions UI keywords
testing_requirements:
  unit: true                     # Always true for non-trivial tasks
  e2e: true                      # Set for frontend projects with UI tasks
  visual: false                  # Set for design/style/theme tasks

created_at: 2026-01-10T10:30:00Z
created_by: randy
updated_at: 2026-01-10T12:45:00Z

metadata:
  source: cli            # cli | api | import
  tags: [auth, feature]
```

---

## Task Organization

Tasks support queue, priority, and category properties to help manage and organize work.

### Queue

| Queue | Purpose | UI Display |
|-------|---------|------------|
| `active` | Current work (default) | Shown prominently in each column |
| `backlog` | "Someday" items | Collapsed section with dashed borders |

Tasks in backlog remain in their status column but are visually de-emphasized. The backlog is collapsible in the UI.

### Priority

| Priority | Sort Order | Indicator |
|----------|------------|-----------|
| `critical` | 1 (first) | Pulsing red icon |
| `high` | 2 | Orange up arrow |
| `normal` | 3 (default) | None |
| `low` | 4 (last) | Gray down arrow |

Tasks are sorted by priority within each column. Higher priority tasks appear first in both active and backlog sections.

### Category

| Category | Description | Use Case |
|----------|-------------|----------|
| `feature` | New functionality (default) | Adding capabilities |
| `bug` | Bug fix | Error correction |
| `refactor` | Code restructuring | No behavior change |
| `chore` | Maintenance | Dependencies, cleanup |
| `docs` | Documentation | README, API docs |
| `test` | Test-related | Test coverage |

Categories help classify the type of work for organization and filtering. Set via `--category` flag on `orc new` or through the web UI.

**Note:** Queue, priority, and category are orthogonal to task status. A `backlog` task can still be `running` or `blocked`, and a `critical` task can be in any status.

### Initiative Linking

Tasks can optionally belong to an initiative—a grouping of related tasks that share decisions and context.

| Field | Values | Default | Purpose |
|-------|--------|---------|---------|
| `initiative_id` | Initiative ID (e.g., `INIT-001`) | empty | Groups task under an initiative |

**Behavior:**
- Empty/omitted `initiative_id` means the task is standalone
- Set via `orc new --initiative INIT-001` or `orc edit TASK-001 --initiative INIT-001`
- Unlink via `orc edit TASK-001 --initiative ""`
- Bidirectional sync: setting initiative_id auto-adds the task to the initiative's task list
- When a task is deleted, it's automatically removed from its linked initiative

**API support:**
- Include `initiative_id` in POST/PATCH task requests
- Filter tasks by initiative: `GET /api/tasks?initiative=INIT-001`

---

## Task Dependencies

Tasks support dependency relationships for ordering work and tracking relationships.

### Dependency Fields

| Field | Type | Stored | Description |
|-------|------|--------|-------------|
| `blocked_by` | `[]string` | Yes | Task IDs that must complete before this task can run |
| `blocks` | `[]string` | No | Task IDs waiting on this task (computed inverse) |
| `related_to` | `[]string` | Yes | Related task IDs (informational, no execution impact) |
| `referenced_by` | `[]string` | No | Task IDs whose descriptions mention this task (auto-detected) |

### Stored vs Computed

**Stored fields** (`blocked_by`, `related_to`):
- Saved to database
- User-editable via CLI or API
- Persist across sessions

**Computed fields** (`blocks`, `referenced_by`):
- Calculated when tasks are loaded
- Not stored in database
- Derived from scanning all tasks

### Validation

Dependencies are validated on create/update:

| Check | Behavior |
|-------|----------|
| Task ID exists | Error returned if referencing non-existent task |
| Self-reference | Error: "task cannot block itself" |
| Circular dependency | Error: "circular dependency detected: A -> B -> A" |

### CLI Usage

```bash
# Create with dependencies
orc new "Part 2" --blocked-by TASK-001
orc new "Feature" --blocked-by TASK-001,TASK-002 --related-to TASK-003

# Edit dependencies
orc edit TASK-005 --blocked-by TASK-003,TASK-004    # Replace list
orc edit TASK-005 --add-blocker TASK-006            # Add to list
orc edit TASK-005 --remove-blocker TASK-003         # Remove from list
orc edit TASK-005 --related-to TASK-007             # Set related tasks
```

### Execution Behavior

| Scenario | Behavior |
|----------|----------|
| Unmet dependencies | Task can be started but will show warning |
| `HasUnmetDependencies()` | Returns true if any blocker is not completed |
| `GetUnmetDependencies()` | Returns list of incomplete blocker IDs |

Dependencies are informational by default - they don't prevent task execution but help track work ordering.

---

## Weight Classification

| Weight | Scope | Duration | Phases |
|--------|-------|----------|--------|
| **trivial** | 1 file, <10 lines | Minutes | implement |
| **small** | 1 component, <100 lines | <1 hour | implement → test |
| **medium** | Multiple files | Hours | spec → implement → review → test |
| **large** | Cross-cutting | Days | research → spec → design → implement → review → test |
| **greenfield** | New system | Weeks | research → spec → design → scaffold → implement → test → docs |

### Classification Criteria

| Dimension | Trivial | Small | Medium | Large | Greenfield |
|-----------|---------|-------|--------|-------|------------|
| Files | 1-2 | 1-5 | 3-10 | 10+ | New structure |
| Uncertainty | None | Low | Medium | High | Very high |
| Risk | None | Low | Medium | High | High |
| Dependencies | None | Internal | Some external | Many | Unknown |

---

## Status Lifecycle

```
created ──► classifying ──► planned ──► running ◄─┐
                                          │       │
                                          ▼       │
                                       paused ────┘
                                          │
                              ┌───────────┼───────────┐
                              ▼           ▼           ▼
                          completed    failed      blocked
```

| Status | Description | Transitions To |
|--------|-------------|----------------|
| `created` | Task defined, not started | classifying |
| `classifying` | AI determining weight | planned |
| `planned` | Plan generated, ready to run | running |
| `running` | Actively executing phases | paused, completed, failed, blocked |
| `paused` | Manually paused | running |
| `blocked` | Waiting for human gate | running |
| `completed` | All phases done, merged | - |
| `failed` | Unrecoverable error | - |

---

## Task State

State is stored in the SQLite database (`states` table). Example:

```yaml
current_phase: implement
current_iteration: 3
status: running

phases:
  classify:
    status: completed
    started_at: 2026-01-10T10:31:00Z
    completed_at: 2026-01-10T10:32:00Z
    result:
      weight: medium
      confidence: 0.88
    checkpoint: abc123

  spec:
    status: completed
    started_at: 2026-01-10T10:32:00Z
    completed_at: 2026-01-10T10:45:00Z
    iterations: 2
    checkpoint: def456

  implement:
    status: running
    started_at: 2026-01-10T10:45:00Z
    iterations: 3
    last_checkpoint: ghi789

gates:
  - phase: spec
    type: ai
    decision: approved
    timestamp: 2026-01-10T10:45:00Z

errors: []
```

---

## UI Testing Detection

During task creation, orc automatically detects if the task involves UI work and configures testing requirements accordingly.

### UI Keyword Detection

The `requires_ui_testing` flag is auto-set when the task title or description contains UI-related keywords:

| Category | Keywords |
|----------|----------|
| Components | button, form, modal, dialog, component, widget, input, dropdown, select, checkbox, radio, tooltip, popover, toast, notification, alert |
| Layout | page, layout, navigation, menu, sidebar, header, footer, dashboard, table, grid, card |
| Styling | style, css, design, responsive, mobile, desktop, theme, dark mode, light mode, animation, transition |
| Interaction | click, hover, focus, scroll, drag, drop |
| Accessibility | accessibility, a11y, screen reader, keyboard navigation |

### Testing Requirements

The `testing_requirements` object is auto-configured based on task weight and project type:

| Requirement | Condition |
|-------------|-----------|
| `unit` | Always `true` for non-trivial tasks (weight > trivial) |
| `e2e` | `true` if project has frontend AND task requires UI testing |
| `visual` | `true` if task mentions visual/design/style/css/theme/layout/responsive |

### Example Output

```bash
$ orc new "Add dark mode toggle button"

Task created: TASK-042
   Title:  Add dark mode toggle button
   Weight: medium
   Phases: 3
   UI Testing: required (detected from task description)
   Testing: unit, e2e, visual
```

---

## Task Operations

| Operation | Command | Effect |
|-----------|---------|--------|
| Create | `orc new "title"` | Creates task in database, branch |
| Run | `orc run TASK-001` | Starts/resumes execution |
| Pause | `orc pause TASK-001` | Checkpoints, stops execution |
| Rewind | `orc rewind TASK-001 --to spec` | Resets to checkpoint |
| Approve | `orc approve TASK-001` | Passes human gate |
| Abort | `orc abort TASK-001` | Marks failed, cleans up |
