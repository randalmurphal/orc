# Orc Feature Roadmap

**Status**: Planning
**Last Updated**: 2026-01-10

---

## Overview

This document outlines the comprehensive feature set for orc v1.0. Each feature has a detailed specification in its own document.

## Priority Tiers

| Tier | Focus | Timeline |
|------|-------|----------|
| **P0** | Core functionality, must-have for v1.0 | Immediate |
| **P1** | Essential UX, significantly improves experience | Near-term |
| **P2** | Polish, nice-to-have | Post-v1.0 |

---

## Feature Matrix

| Feature | Priority | Spec | Status |
|---------|----------|------|--------|
| [Task Enhancement Flow](#task-enhancement-flow) | P0 | [TASK_ENHANCEMENT.md](./TASK_ENHANCEMENT.md) | Planning |
| [Session Interoperability](#session-interoperability) | P0 | [SESSION_INTEROP.md](./SESSION_INTEROP.md) | Planning |
| [Interactive Init Wizard](#interactive-init-wizard) | P0 | [INIT_WIZARD.md](./INIT_WIZARD.md) | Planning |
| [Error Message Standards](#error-message-standards) | P0 | [ERROR_STANDARDS.md](./ERROR_STANDARDS.md) | Planning |
| [Cost Tracking](#cost-tracking) | P1 | [COST_TRACKING.md](./COST_TRACKING.md) | Planning |
| [Task Templates](#task-templates) | P1 | [TASK_TEMPLATES.md](./TASK_TEMPLATES.md) | Planning |
| [Web UI Dashboard](#web-ui-dashboard) | P1 | [WEB_DASHBOARD.md](./WEB_DASHBOARD.md) | Planning |
| [Project Detection](#project-detection) | P1 | [PROJECT_DETECTION.md](./PROJECT_DETECTION.md) | Planning |
| [Keyboard Shortcuts](#keyboard-shortcuts) | P1 | [KEYBOARD_SHORTCUTS.md](./KEYBOARD_SHORTCUTS.md) | Planning |
| [TUI Watch Mode](#tui-watch-mode) | P2 | [TUI_WATCH.md](./TUI_WATCH.md) | Planning |
| [Cross-Project Resources](#cross-project-resources) | P2 | [CROSS_PROJECT.md](./CROSS_PROJECT.md) | Planning |

---

## Feature Summaries

### Task Enhancement Flow
**Priority: P0**

Instead of simple weight classification, tasks go through Claude-powered enhancement. Either:
1. Human sets weight explicitly (`--weight large`)
2. Task starts with a "planning" phase that uses Claude to deeply analyze and enhance the task

The enhancement phase:
- Analyzes codebase to understand scope
- Expands title into detailed specification
- Identifies affected files and components
- Sets appropriate weight based on analysis
- Can use project scripts for context

### Session Interoperability
**Priority: P0**

Seamless handoff between Web UI and CLI:
- Start task in UI, pause, get session ID
- Resume in Claude Code: `claude --resume <session-id>`
- Start in CLI, monitor in UI
- Session IDs stored in task state

### Interactive Init Wizard
**Priority: P0**

Guided project setup:
- Detect project type (Go/Node/Python/etc)
- Choose automation profile
- Configure completion actions
- Optionally spawn Claude session for advanced setup
- Install relevant skills/plugins

### Error Message Standards
**Priority: P0**

Every error must include:
1. What went wrong (clear description)
2. Why it happened (context)
3. How to fix it (actionable steps)

### Cost Tracking
**Priority: P1**

Track and display token usage:
- Per-task token counts (input/output)
- Cost estimation based on model pricing
- Historical trends
- Budget alerts (optional)

### Task Templates
**Priority: P1**

Reusable task patterns:
- Save task as template: `orc template save bugfix`
- Create from template: `orc new --template bugfix "Fix auth timeout"`
- Templates include: default weight, custom prompts, context files

### Web UI Dashboard
**Priority: P1**

Landing page with:
- Active task summary
- Recent activity feed
- Quick stats (running, blocked, completed today)
- Token usage overview
- Quick actions

### Project Detection
**Priority: P1**

Auto-detect project type on init:
- Language detection (go.mod, package.json, pyproject.toml)
- Framework detection (React, FastAPI, Gin, etc)
- Suggest relevant skills/plugins
- Configure default prompts

### Keyboard Shortcuts
**Priority: P1**

Full keyboard navigation:
- Global: `n` new task, `/` search, `?` help
- Task list: `j/k` navigate, `Enter` open, `r` run
- Task detail: `p` pause, `c` cancel, `Esc` back

### TUI Watch Mode
**Priority: P2**

Lazygit-style terminal UI:
- Real-time task monitoring
- Multiple task views
- Vim-style navigation
- Quick actions via keybindings
- Multiple pages/panels

### Cross-Project Resources
**Priority: P2**

Shared resources across projects:
- Global skills in `~/.orc/skills/`
- Shared templates in `~/.orc/templates/`
- Common scripts in `~/.orc/scripts/`
- Project can extend or override

---

## Implementation Order

```
Phase 1: Foundation
├── Error Message Standards (enables better debugging)
├── Session Interoperability (core UX requirement)
└── Interactive Init Wizard (first-run experience)

Phase 2: Task Flow
├── Task Enhancement Flow (replaces naive classification)
├── Cost Tracking (visibility into usage)
└── Task Templates (efficiency)

Phase 3: UI Polish
├── Web UI Dashboard (landing experience)
├── Keyboard Shortcuts (power users)
└── Project Detection (smart defaults)

Phase 4: Advanced
├── TUI Watch Mode (terminal power users)
└── Cross-Project Resources (multi-project workflows)
```

---

## Non-Goals (Explicit Exclusions)

| Feature | Reason |
|---------|--------|
| GitHub/GitLab integration | Focus on core first |
| Jira integration | Focus on core first |
| VS Code extension | Focus on core first |
| Desktop notifications | Web UI notifications sufficient |
| SQLite for primary storage | YAML + git is sufficient |
| Team/collaboration features | Single-user focus for v1.0 |

---

## Open Questions (Resolved)

| Question | Resolution |
|----------|------------|
| Task Enhancement mode | Automatic by default, `-i` for interactive |
| Session storage | In state.yaml (simpler, one file per task) |
| Cost tracking | Estimated based on configurable pricing.yaml |
| Cross-project conflicts | Project always wins, global is fallback |

---

## Implementation Guide

See [IMPLEMENTATION_GUIDE.md](./IMPLEMENTATION_GUIDE.md) for:
- Dependency graph
- Phase-by-phase implementation order
- Effort estimates (~12 weeks total)
- Testing strategy
- Migration notes

---

## Spec Index

| Spec | Priority | Status |
|------|----------|--------|
| [TASK_ENHANCEMENT.md](./TASK_ENHANCEMENT.md) | P0 | Planning |
| [SESSION_INTEROP.md](./SESSION_INTEROP.md) | P0 | Planning |
| [INIT_WIZARD.md](./INIT_WIZARD.md) | P0 | Planning |
| [ERROR_STANDARDS.md](./ERROR_STANDARDS.md) | P0 | Planning |
| [COST_TRACKING.md](./COST_TRACKING.md) | P1 | Planning |
| [TASK_TEMPLATES.md](./TASK_TEMPLATES.md) | P1 | Planning |
| [WEB_DASHBOARD.md](./WEB_DASHBOARD.md) | P1 | Planning |
| [PROJECT_DETECTION.md](./PROJECT_DETECTION.md) | P1 | Planning |
| [KEYBOARD_SHORTCUTS.md](./KEYBOARD_SHORTCUTS.md) | P1 | Planning |
| [TUI_WATCH.md](./TUI_WATCH.md) | P2 | Planning |
| [CROSS_PROJECT.md](./CROSS_PROJECT.md) | P2 | Planning |
