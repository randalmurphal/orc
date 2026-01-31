# Documentation

Technical documentation for the orc orchestrator.

## Directory Structure

```
docs/
├── API_REFERENCE.md  # REST API endpoints
├── architecture/     # System architecture docs
├── specs/            # Feature specifications
├── decisions/        # Architecture Decision Records (ADRs)
├── plans/            # Design documents and implementation plans
├── research/         # Research notes and analysis
└── guides/           # User guides and troubleshooting
```

## API Reference

| Document | Description |
|----------|-------------|
| `API_REFERENCE.md` | All REST endpoints, WebSocket protocol, error responses |

## Architecture Documents

| Document | Description |
|----------|-------------|
| `OVERVIEW.md` | High-level system architecture |
| `TASK_MODEL.md` | Task structure, weight, lifecycle |
| `PHASE_MODEL.md` | Phase definitions, templates |
| `GIT_INTEGRATION.md` | Branches, checkpoints, worktrees |
| `EXECUTOR.md` | Ralph-style execution loop, completion detection |
| `GATES.md` | Quality gates, approval workflow |

## Specifications

| Document | Description |
|----------|-------------|
| `CLI.md` | Command-line interface specification |
| `FILE_FORMATS.md` | YAML file formats (task, plan, state) |
| `WEB_DASHBOARD.md` | Web UI specification |
| `KEYBOARD_SHORTCUTS.md` | UI keyboard shortcuts |
| `ERROR_STANDARDS.md` | Error handling patterns |
| `COMPLETION_CRITERIA.md` | Phase completion rules |
| `SESSION_INTEROP.md` | Claude Code session handling |
| `COST_TRACKING.md` | Token and cost tracking |
| `CROSS_PROJECT.md` | Multi-project support |
| `INIT_WIZARD.md` | Interactive initialization |
| `TASK_ENHANCEMENT.md` | AI task enhancement |
| `PHASE_SETTINGS_DESIGN.md` | Per-phase Claude Code settings (hooks, skills, hook scripts) |
| `PHASE_SETTINGS_UI_DESIGN.md` | Frontend UI for phase settings management |
| `TASK_TEMPLATES.md` | Task template system |
| `PROJECT_DETECTION.md` | Project type detection |
| `TUI_WATCH.md` | Terminal UI specification |
| `IMPLEMENTATION_GUIDE.md` | Implementation guidelines |
| `ROADMAP.md` | Feature roadmap |
| `DOCUMENTATION.md` | Documentation standards |

## Architecture Decision Records (ADRs)

| ADR | Decision |
|-----|----------|
| `ADR-001` | Language and stack (Go + Svelte 5) |
| `ADR-002` | Storage model (YAML files, git-tracked) |
| `ADR-003` | Git integration (branches, worktrees) |
| `ADR-004` | UI framework (Svelte 5) |
| `ADR-005` | Task weight system |
| `ADR-006` | Execution model (Ralph-style) |
| `ADR-007` | Human gates |

## Research

| Document | Description |
|----------|-------------|
| `CLAUDE_TOOLS.md` | Claude Code tool analysis |
| `AGENT_FRAMEWORKS.md` | Agent framework comparison |
| `PATTERNS.md` | Orchestration patterns |
| `RALPH_WIGGUM.md` | Ralph-style loop research |

## Guides

| Document | Description |
|----------|-------------|
| `TROUBLESHOOTING.md` | Common issues and solutions |

## Design Documents & Plans

| Document | Description |
|----------|-------------|
| `plans/2026-01-29-branch-control-design.md` | Branch control feature design (custom branches, PR options) |
| `plans/2026-01-29-branch-control-plan.md` | Branch control implementation plan |
| `plans/2025-01-29-multi-project-support-design.md` | Multi-project support design |
| `plans/2025-01-29-multi-project-implementation.md` | Multi-project implementation plan |
| `plans/2026-01-30-ai-gates-lifecycle-events-design.md` | AI gates & lifecycle events design (see `architecture/GATES.md` for implemented docs) |

## Quick Reference

### Reading Order (New Contributors)
1. `architecture/OVERVIEW.md` - System overview
2. `architecture/TASK_MODEL.md` - Core concepts
3. `architecture/PHASE_MODEL.md` - Execution flow
4. `specs/CLI.md` - Command reference
5. `specs/FILE_FORMATS.md` - Data formats

### Implementation Reference
1. `architecture/EXECUTOR.md` - Execution engine
2. `specs/COMPLETION_CRITERIA.md` - Phase completion
3. `specs/ERROR_STANDARDS.md` - Error handling
4. `architecture/GATES.md` - Quality gates

### ADR Template
```markdown
# ADR-XXX: Title

## Status
Proposed | Accepted | Deprecated | Superseded

## Context
[Why this decision was needed]

## Decision
[What was decided]

## Consequences
[Positive and negative outcomes]
```
