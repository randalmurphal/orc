# Task Model

**Purpose**: Define task structure, weight classification, and lifecycle states.

---

## Task Definition

```yaml
# .orc/tasks/TASK-001/task.yaml
id: TASK-001
title: "Add user authentication"
description: |
  Implement OAuth2 authentication with Google and GitHub providers.
  Should integrate with existing user model.

weight: medium           # trivial | small | medium | large | greenfield
status: running          # created | classifying | planned | running | paused | blocked | completed | failed
current_phase: implement # Phase currently being executed (updated by executor)
branch: orc/TASK-001

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

```yaml
# .orc/tasks/TASK-001/state.yaml
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
| Create | `orc new "title"` | Creates task.yaml, branch |
| Run | `orc run TASK-001` | Starts/resumes execution |
| Pause | `orc pause TASK-001` | Checkpoints, stops execution |
| Rewind | `orc rewind TASK-001 --to spec` | Resets to checkpoint |
| Approve | `orc approve TASK-001` | Passes human gate |
| Abort | `orc abort TASK-001` | Marks failed, cleans up |
