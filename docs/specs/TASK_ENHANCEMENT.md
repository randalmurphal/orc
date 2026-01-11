# Task Enhancement Flow

**Status**: Planning
**Priority**: P0
**Last Updated**: 2026-01-10

---

## Problem Statement

Current task creation is too simplistic:
1. User provides title
2. Weight defaults to "medium" (AI classification not implemented)
3. Task runs with generic prompts

This leads to:
- Under-specified tasks that require multiple retries
- Over-engineered trivial tasks
- No project-specific context in prompts

---

## Solution: Claude-Powered Task Enhancement

Replace naive weight classification with a **Task Enhancement Phase** that uses Claude to deeply analyze and prepare the task before execution begins.

---

## User Experience

### Option A: Quick Mode (Human Sets Weight)

```bash
$ orc new "Fix auth timeout bug" --weight small

Task created: TASK-001
Weight: small (user-specified)
Phases: implement → test

Run: orc run TASK-001
```

No enhancement phase - user knows what they want.

### Option B: Standard Mode (AI Enhancement)

```bash
$ orc new "Fix auth timeout bug"

Starting task enhancement...
Claude is analyzing your codebase...

┌─ Enhancement Analysis ─────────────────────────────────────────┐
│                                                                │
│ Task: Fix auth timeout bug                                     │
│                                                                │
│ Analysis:                                                      │
│ • Found authClient.go with 5s hardcoded timeout (line 234)     │
│ • Related files: auth_test.go, middleware/auth.go              │
│ • Existing test coverage: 67%                                  │
│ • No breaking changes expected                                 │
│                                                                │
│ Recommended weight: small                                      │
│ Estimated scope: 2-3 files, ~50 lines changed                  │
│                                                                │
│ Enhanced description:                                          │
│ > Make auth timeout configurable via environment variable.     │
│ > Default to 30s, allow override via AUTH_TIMEOUT_SECONDS.     │
│ > Update existing tests to cover timeout configuration.        │
│                                                                │
└────────────────────────────────────────────────────────────────┘

Accept enhancement? [Y/n/edit]: y

Task created: TASK-001
Weight: small
Phases: implement → test

Run: orc run TASK-001
```

### Option C: Interactive Mode (Claude Session)

```bash
$ orc new -i "Implement user dashboard"

Starting interactive enhancement session...
Opening Claude Code for task planning...

# Claude session opens with enhancement prompt
# User can interact, ask questions, refine scope
# Session ends when user is satisfied

Task created: TASK-002
Weight: large
Phases: spec → implement → test → validate
Session ID: 550e8400-e29b-41d4-a716-446655440000

Enhanced spec saved to: .orc/tasks/TASK-002/spec.md
Run: orc run TASK-002
```

---

## Enhancement Phase Implementation

### Trigger Conditions

| Condition | Enhancement Behavior |
|-----------|---------------------|
| `--weight` flag provided | Skip enhancement, use specified weight |
| `--quick` flag | Skip enhancement, default to medium |
| `-i` / `--interactive` flag | Full interactive Claude session |
| No flags (default) | Automatic enhancement (non-interactive) |

### Enhancement Prompt

```markdown
# Task Enhancement

You are enhancing a task for the orc orchestrator.

## Task Title
{{TASK_TITLE}}

## Available Context
- Project: {{PROJECT_NAME}} ({{PROJECT_TYPE}})
- CLAUDE.md: Available in context
- Scripts: {{AVAILABLE_SCRIPTS}}

## Your Job

1. **Analyze the codebase** to understand the scope:
   - Find relevant files
   - Understand existing patterns
   - Identify affected components

2. **Classify the weight** based on:
   | Weight | Criteria |
   |--------|----------|
   | trivial | <10 lines, single file, no tests needed |
   | small | 1-3 files, isolated change, unit tests |
   | medium | 3-10 files, feature work, integration tests |
   | large | 10+ files, cross-cutting, E2E tests |
   | greenfield | New system/service from scratch |

3. **Enhance the description** with:
   - Specific files to modify
   - Technical approach
   - Success criteria
   - Testing requirements

4. **Output your analysis** in this format:

```yaml
weight: small
files:
  - path: internal/auth/client.go
    reason: Contains hardcoded timeout
  - path: internal/auth/client_test.go
    reason: Needs test coverage for timeout config

description: |
  Make auth client timeout configurable via AUTH_TIMEOUT_SECONDS
  environment variable. Default to 30 seconds. Update tests.

success_criteria:
  - Timeout is configurable
  - Default timeout is 30s
  - Tests cover timeout configuration
  - No regression in existing auth tests

estimated_scope:
  files: 2
  lines: ~50
```

When done, output:
<enhancement_complete>true</enhancement_complete>
```

### Script Integration

Enhancement phase can use project scripts:

```yaml
# .orc/config.yaml
enhancement:
  scripts:
    - name: analyze-deps
      command: .claude/scripts/analyze-dependencies
      description: Analyze project dependencies
    - name: find-tests
      command: .claude/scripts/find-related-tests
      description: Find tests related to changed files
```

Scripts are made available to Claude during enhancement:

```markdown
## Available Scripts

You can use these scripts to gather information:

- `analyze-deps <file>`: Analyze dependencies for a file
- `find-tests <pattern>`: Find tests matching pattern

Use the Bash tool to run scripts as needed.
```

---

## CLAUDE.md Integration

On `orc init`, add an orc section to the project's CLAUDE.md:

```markdown
## Orc Orchestrator

This project uses orc for task orchestration.

### Available Commands
- `orc status` - Show running tasks
- `orc run TASK-ID` - Execute a task
- `orc pause TASK-ID` - Pause execution
- `orc resume TASK-ID` - Resume execution

### Scripts
The following scripts are available for task analysis:
- `.claude/scripts/python-code-quality` - Run linters and type checks
- `.claude/scripts/find-callers` - Find all callers of a function

### Task Enhancement
When enhancing tasks, consider:
- Project uses {{FRAMEWORK}}
- Test with: {{TEST_COMMAND}}
- Lint with: {{LINT_COMMAND}}
```

This section is:
- **Idempotent**: Can be regenerated without conflicts
- **Clearly marked**: Uses `## Orc Orchestrator` header
- **Machine-readable**: Structured format for extraction

---

## State Storage

Enhanced task data stored in `task.yaml`:

```yaml
id: TASK-001
title: Fix auth timeout bug
weight: small
status: planned
branch: orc/TASK-001

# Enhancement data
enhanced: true
enhanced_at: 2026-01-10T14:30:00Z
enhancement_session_id: 550e8400-e29b-41d4-a716-446655440000

description: |
  Make auth client timeout configurable via AUTH_TIMEOUT_SECONDS
  environment variable. Default to 30 seconds.

files:
  - path: internal/auth/client.go
    reason: Contains hardcoded timeout
  - path: internal/auth/client_test.go
    reason: Needs test coverage

success_criteria:
  - Timeout is configurable
  - Default timeout is 30s
  - Tests cover timeout configuration

estimated_scope:
  files: 2
  lines: 50
```

---

## CLI Interface

```bash
# Quick mode - skip enhancement
orc new "Fix typo in README" --weight trivial

# Standard mode - automatic enhancement
orc new "Fix auth timeout bug"

# Interactive mode - Claude session for planning
orc new -i "Implement user dashboard"
orc new --interactive "Implement user dashboard"

# With description
orc new "Fix auth timeout" -d "Timeout should be configurable via env var"

# From file
orc new --from spec.md
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--weight` | `-w` | Skip enhancement, use specified weight |
| `--quick` | `-q` | Skip enhancement, default to medium |
| `--interactive` | `-i` | Full interactive Claude session |
| `--description` | `-d` | Provide initial description |
| `--from` | `-f` | Create from specification file |

---

## API Endpoints

```
POST /api/tasks
{
  "title": "Fix auth timeout bug",
  "mode": "enhanced" | "quick" | "interactive"
}

Response:
{
  "id": "TASK-001",
  "status": "enhancing" | "planned",
  "enhancement_session_id": "..."  // If interactive
}

GET /api/tasks/:id/enhancement
{
  "status": "running" | "complete" | "failed",
  "analysis": { ... },
  "recommended_weight": "small"
}
```

---

## Web UI Flow

1. **New Task Modal** with mode selector:
   ```
   ┌─ Create New Task ─────────────────────────────────────────┐
   │                                                           │
   │ Title: [Fix auth timeout bug                          ]   │
   │                                                           │
   │ Creation Mode:                                            │
   │ (•) Enhanced - Claude analyzes and plans the task         │
   │ ( ) Quick - Specify weight manually                       │
   │ ( ) Interactive - Open Claude session for planning        │
   │                                                           │
   │ Weight (for Quick mode): [medium ▼]                       │
   │                                                           │
   │                            [Cancel] [Create Task]         │
   └───────────────────────────────────────────────────────────┘
   ```

2. **Enhancement Progress** (for Enhanced mode):
   ```
   ┌─ Enhancing Task ──────────────────────────────────────────┐
   │                                                           │
   │ ⏳ Analyzing codebase...                                  │
   │                                                           │
   │ Found:                                                    │
   │ • authClient.go - timeout configuration                   │
   │ • auth_test.go - related tests                            │
   │                                                           │
   │ Recommended weight: small                                 │
   │                                                           │
   │                     [Cancel] [Accept] [Edit]              │
   └───────────────────────────────────────────────────────────┘
   ```

---

## Implementation Notes

### Enhancement is Optional

Users who know what they want can skip it:
- `--weight` bypasses enhancement entirely
- `--quick` uses sensible defaults
- Power users aren't slowed down

### Enhancement is Cacheable

If the same title is used again:
- Show previous enhancement as suggestion
- Allow quick re-use or re-enhancement

### Enhancement Uses Project Context

- Reads CLAUDE.md for project conventions
- Uses configured scripts for analysis
- Respects project-specific prompts if configured

### Enhancement Produces Artifacts

- `spec.md` for large/greenfield tasks
- Enhanced `task.yaml` with analysis data
- Session ID for potential resume

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for enhancement code
- 100% coverage for mode selection logic

### Unit Tests

| Test | Description |
|------|-------------|
| `TestWeightFlagSkipsEnhancement` | `--weight` bypasses enhancement entirely |
| `TestQuickFlagSkipsEnhancement` | `--quick` uses medium weight default |
| `TestInteractiveFlagStartsSession` | `-i` triggers interactive mode |
| `TestEnhancementPromptRendering` | Template variables substitute correctly |
| `TestEnhancementYAMLParsing` | Parse Claude's YAML output correctly |
| `TestEnhancementOutputValidation` | Validate weight, files, criteria fields |
| `TestEnhancementCache` | Cache hit returns previous enhancement |
| `TestScriptIntegration` | Scripts available in enhancement context |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestEnhancementFlowWithMock` | Full flow with mocked Claude response |
| `TestStatePersistence` | Enhanced task.yaml has all enhancement fields |
| `TestAPIEnhancedMode` | `POST /api/tasks` with `mode: enhanced` |
| `TestAPIQuickMode` | `POST /api/tasks` with `mode: quick` |
| `TestAPIInteractiveMode` | `POST /api/tasks` with `mode: interactive` |
| `TestGetEnhancementStatus` | `GET /api/tasks/:id/enhancement` returns status |
| `TestCLINewWithWeight` | `orc new --weight small "title"` skips enhancement |
| `TestCLINewDefault` | `orc new "title"` runs enhancement |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_enhanced_mode_selected` | `browser_click`, `browser_snapshot` | Select "Enhanced" in mode selector |
| `test_enhancement_progress_ui` | `browser_wait_for`, `browser_snapshot` | Progress UI appears during enhancement |
| `test_enhancement_accept_button` | `browser_click` | Accept creates task with enhanced data |
| `test_enhancement_edit_button` | `browser_click`, `browser_snapshot` | Edit opens editor with enhancement |
| `test_enhancement_cancel_button` | `browser_click` | Cancel returns to modal |
| `test_quick_mode_no_progress` | `browser_snapshot` | Quick mode skips enhancement UI |

### Test Fixtures
- Sample enhancement Claude output (YAML format)
- Mock project context for enhancement prompt
- Sample task.yaml with enhancement data

---

## Success Criteria

- [ ] Tasks with `--weight` skip enhancement (no slowdown for simple cases)
- [ ] Enhanced tasks have better first-attempt success rate
- [ ] Interactive mode produces high-quality specs
- [ ] Enhancement uses project scripts when available
- [ ] CLAUDE.md is updated idempotently on init
- [ ] Web UI shows enhancement progress
- [ ] API supports all three modes
- [ ] 80%+ test coverage on enhancement code
- [ ] All E2E tests pass
