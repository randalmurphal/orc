# File Format Specifications

**Purpose**: Define all YAML file formats used by orc.

---

## orc.yaml (Project Config)

```yaml
# orc.yaml - Project configuration
version: 1

# Claude Code settings
claude:
  path: claude                    # CLI path
  model: claude-opus-4-5-20251101        # Default model (Opus 4.5 for best judgment)
  timeout: 600                    # Phase timeout (seconds)
  max_tokens: 100000              # Max tokens per session

# Gate defaults
gates:
  spec: ai
  design: ai
  review: ai
  merge: human                    # Human approval for merge

  weight_overrides:
    large:
      spec: human
      design: human
    greenfield:
      spec: human
      design: human
      review: human

# Weight classification
weights:
  default: medium                 # Fallback if classification fails
  allow_override: true            # User can override AI classification

# Git settings
git:
  branch_prefix: orc/             # Branch naming: orc/TASK-ID
  checkpoint_prefix: "[orc]"      # Commit message prefix
  merge_strategy: squash          # squash | preserve | rebase
  worktrees: true                 # Enable worktree isolation
  cleanup_on_complete: true       # Delete branch after merge

# Prompt customization
prompts:
  directory: .orc/prompts         # Override prompt templates
```

---

## task.yaml (Task Definition)

```yaml
# .orc/tasks/TASK-001/task.yaml
id: TASK-001
title: "Add user authentication"
description: |
  Implement OAuth2 authentication with Google and GitHub.
  Should integrate with existing user model.

weight: medium
status: running
current_phase: implement          # Phase currently being executed (updated by executor)
branch: orc/TASK-001

# Task Organization
queue: active                     # active (default) | backlog
priority: normal                  # critical | high | normal (default) | low
category: feature                 # feature (default) | bug | refactor | chore | docs | test

# Initiative linking (optional)
initiative_id: INIT-001           # Links task to initiative, empty/omitted = standalone

# Task Dependencies
blocked_by:                       # Tasks that must complete before this task
  - TASK-060
  - TASK-061
related_to:                       # Related tasks (informational, soft connection)
  - TASK-063

# UI Testing Detection (auto-detected from task content)
requires_ui_testing: true         # Set when task mentions UI/frontend/button/form/page

# Testing Requirements (auto-configured based on weight and project type)
testing_requirements:
  unit: true                      # Unit tests (default true for non-trivial tasks)
  e2e: true                       # E2E tests (set for frontend projects with UI tasks)
  visual: false                   # Visual regression tests (set for design/style tasks)

created_at: 2026-01-10T10:30:00Z
created_by: randy
updated_at: 2026-01-10T12:45:00Z

metadata:
  source: cli
  tags: [auth, feature]
  external_id: JIRA-123          # Link to external tracker
  # Resolution metadata (set by `orc resolve` command)
  resolved: "true"               # Task was resolved, not executed to completion
  resolved_at: 2026-01-10T14:00:00Z  # When task was resolved
  resolution_message: "Fixed manually outside of orc"  # Optional explanation
```

### Queue, Priority, Category, and Initiative

| Field | Values | Default | Purpose |
|-------|--------|---------|---------|
| `queue` | `active`, `backlog` | `active` | Separates current work from deferred items |
| `priority` | `critical`, `high`, `normal`, `low` | `normal` | Urgency within a queue |
| `category` | `feature`, `bug`, `refactor`, `chore`, `docs`, `test` | `feature` | Type of work for organization and filtering |
| `initiative_id` | Initiative ID (e.g., `INIT-001`) | empty | Links task to an initiative for grouping |

**Initiative linking:**
- Tasks can optionally belong to an initiative (a group of related tasks)
- Empty/omitted `initiative_id` means the task is standalone
- Set via `orc new --initiative INIT-001` or `orc edit TASK-001 --initiative INIT-001`
- Unlink via `orc edit TASK-001 --initiative ""`
- Bidirectional sync: when initiative_id is set, the task is auto-added to the initiative's task list

**Queue behavior:**
- **active**: Tasks shown prominently on the board
- **backlog**: Tasks collapsed by default in each column, shown with dashed borders

**Priority sort order:** Tasks within each column are sorted by priority (critical first, then high, normal, low).

**Category types:**
- **feature**: New functionality or capability
- **bug**: Bug fix or error correction
- **refactor**: Code restructuring without behavior change
- **chore**: Maintenance tasks (dependencies, cleanup, config)
- **docs**: Documentation changes
- **test**: Test-related changes

### Task Dependencies

Tasks support dependency relationships for ordering and organization:

| Field | Stored | Purpose |
|-------|--------|---------|
| `blocked_by` | Yes | Task IDs that must complete before this task can run |
| `blocks` | No (computed) | Task IDs waiting on this task (inverse of blocked_by) |
| `related_to` | Yes | Related task IDs (soft connection, informational) |
| `referenced_by` | No (computed) | Task IDs whose descriptions mention this task |

**Stored vs Computed fields:**
- `blocked_by` and `related_to` are stored in task.yaml and user-editable
- `blocks` and `referenced_by` are computed on load by scanning all tasks

**Validation rules:**
- Referenced task IDs must exist (warning logged for missing references)
- Tasks cannot block themselves
- Circular dependencies are rejected (A blocks B blocks A)

**Example:**
```yaml
blocked_by:
  - TASK-060       # Must complete before this task runs
  - TASK-061
related_to:
  - TASK-063       # Informational link only
```

---

## initiative.yaml (Initiative Definition)

```yaml
# .orc/initiatives/INIT-001/initiative.yaml
id: INIT-001
title: "User Authentication System"
status: active
vision: |
  Implement comprehensive user authentication with OAuth2 support
  for Google and GitHub providers.

owner:
  initials: JD
  display_name: John Doe
  email: john@example.com

# Initiative Dependencies
blocked_by:
  - INIT-000  # Infrastructure Setup must complete first

# Note: blocks is computed, not stored
# blocks:
#   - INIT-002  # React Migration depends on this

decisions:
  - id: DEC-001
    timestamp: 2026-01-10T10:30:00Z
    decision: "Use JWT tokens for session management"
    rationale: "Better for microservices, stateless authentication"
    made_by: JD
    task_context: TASK-001

context_files:
  - docs/auth-spec.md
  - .env.example

tasks:
  - id: TASK-001
    title: "Add OAuth2 providers"
    status: completed
  - id: TASK-002
    title: "Implement JWT session"
    status: running

created_at: 2026-01-10T10:30:00Z
updated_at: 2026-01-15T14:22:00Z
```

### Initiative Dependencies

Initiatives can depend on other initiatives completing first:

| Field | Stored | Purpose |
|-------|--------|---------|
| `blocked_by` | Yes | Initiative IDs that must complete before this initiative |
| `blocks` | No (computed) | Initiative IDs waiting on this initiative |

**Stored vs Computed fields:**
- `blocked_by` is stored in initiative.yaml and user-editable
- `blocks` is computed on load by scanning all initiatives

**Validation rules:**
- Referenced initiative IDs must exist
- Initiatives cannot block themselves
- Circular dependencies are rejected (A blocks B blocks A)

**Blocking behavior:**
- Initiative is blocked if ANY initiative in `blocked_by` is not `completed`
- `orc initiative list` shows `[BLOCKED]` status indicator
- `orc initiative show` displays dependency chain
- `orc initiative run` warns if blocked, use `--force` to override

**Example:**
```yaml
# INIT-002 depends on INIT-001 completing first
id: INIT-002
title: "React Migration"
status: active
blocked_by:
  - INIT-001  # Build System Upgrade must complete first
```

### Initiative Status Values

| Status | Description |
|--------|-------------|
| `draft` | Initiative created but not started |
| `active` | Initiative in progress |
| `completed` | Initiative finished successfully |
| `archived` | Initiative archived (no longer relevant) |

---

## state.yaml (Execution State)

```yaml
# .orc/tasks/TASK-001/state.yaml
current_phase: implement
current_iteration: 3
status: running

started_at: 2026-01-10T10:31:00Z
total_duration: 2h15m

# Execution tracking (for orphan detection)
execution:
  pid: 12345                        # Process ID of executor
  hostname: dev-laptop              # Machine running the executor
  started_at: 2026-01-10T10:31:00Z  # When this execution began
  last_heartbeat: 2026-01-10T12:30:00Z  # Last executor heartbeat

# Claude session info (for resume support)
session:
  id: sess_abc123                   # Claude session ID
  model: claude-opus-4-5-20251101
  status: active
  created_at: 2026-01-10T10:31:00Z
  last_activity: 2026-01-10T12:30:00Z
  turn_count: 15

phases:
  classify:
    status: completed
    started_at: 2026-01-10T10:31:00Z
    completed_at: 2026-01-10T10:32:00Z
    iterations: 1
    duration: 45s
    checkpoint: abc123def
    result:
      weight: medium
      confidence: 0.88
      rationale: "Multiple files, moderate complexity"

  spec:
    status: completed
    started_at: 2026-01-10T10:32:00Z
    completed_at: 2026-01-10T10:45:00Z
    iterations: 2
    duration: 13m
    checkpoint: def456ghi
    artifacts:
      - .orc/tasks/TASK-001/artifacts/spec.md

  # Example of a skipped phase (artifact already existed)
  research:
    status: skipped
    completed_at: 2026-01-10T10:31:30Z
    iterations: 0
    error: "skipped: artifact exists: research content found in spec.md"

  implement:
    status: running
    started_at: 2026-01-10T10:45:00Z
    iterations: 3
    last_checkpoint: ghi789jkl
    files_changed:
      - src/auth/oauth.go
      - src/auth/oauth_test.go
      - src/config/auth.go

gates:
  - phase: spec
    type: ai
    decision: approved
    timestamp: 2026-01-10T10:45:00Z
    rationale: "Spec covers all requirements"
  # Skip decisions are also recorded as gates for audit trail
  - phase: research
    type: skip
    decision: approved
    timestamp: 2026-01-10T10:31:30Z
    rationale: "artifact exists: research content found in spec.md"

errors: []

tokens:
  input_tokens: 45000
  output_tokens: 12000
  cache_creation_input_tokens: 2000      # Tokens written to cache
  cache_read_input_tokens: 8000          # Tokens served from cache
  total_tokens: 67000
  by_phase:
    classify: 2000
    spec: 15000
    implement: 28000
```

### Token Fields

| Field | Description |
|-------|-------------|
| `input_tokens` | Uncached input tokens (billed at full rate) |
| `output_tokens` | Generated output tokens |
| `cache_creation_input_tokens` | Tokens written to cache this session (optional) |
| `cache_read_input_tokens` | Tokens served from cache (90% cheaper than input) |
| `total_tokens` | Sum of all token types |

**Note:** Raw `input_tokens` alone can appear misleadingly low when prompt caching is active. The "effective" input context is `input_tokens + cache_creation_input_tokens + cache_read_input_tokens`. UI displays show the combined cached total for clarity.

---

## plan.yaml (Execution Plan)

```yaml
# .orc/tasks/TASK-001/plan.yaml
task_id: TASK-001
weight: medium
generated_at: 2026-01-10T10:32:00Z

phases:
  - name: spec
    type: spec
    prompt_template: prompts/spec.md
    max_iterations: 3
    timeout: 300s
    completion_criteria:
      - claude_confirms
    checkpoint: true
    gate:
      type: ai
      criteria: [spec_complete]

  - name: implement
    type: implement
    prompt_template: prompts/implement.md
    max_iterations: 10
    timeout: 600s
    completion_criteria:
      - all_tests_pass
      - no_lint_errors
    checkpoint: true
    checkpoint_frequency: 3
    gate:
      type: auto

  - name: review
    type: review
    prompt_template: prompts/review.md
    max_iterations: 3
    timeout: 300s
    completion_criteria:
      - claude_confirms
    gate:
      type: ai
      criteria: [review_approved]

  - name: test
    type: test
    prompt_template: prompts/test.md
    max_iterations: 3
    timeout: 300s
    completion_criteria:
      - all_tests_pass
      - coverage_above: 80
    gate:
      type: auto
```

---

## Transcript Format

```markdown
<!-- .orc/tasks/TASK-001/transcripts/02-implement-003.md -->
# Transcript: TASK-001 / implement / iteration 3

**Timestamp**: 2026-01-10T11:15:00Z
**Duration**: 5m 32s
**Tokens**: 8500
**Status**: running

---

## Prompt

[Full prompt content here]

---

## Response

[Full Claude response here]

---

## Completion

- Tests passing: yes
- Lint clean: yes
- Phase complete: no (continuing to next iteration)

---

## Files Changed

| File | Action | Lines |
|------|--------|-------|
| src/auth/oauth.go | modified | +45, -12 |
| src/auth/oauth_test.go | created | +120 |
```

---

## Task Attachments

Attachments are stored in `.orc/tasks/TASK-XXX/attachments/` directory. Files are stored directly on disk with metadata derived from file system.

### Directory Structure

```
.orc/tasks/TASK-001/
├── task.yaml
├── spec.md
├── plan.yaml
├── state.yaml
├── transcripts/
├── attachments/
│   ├── screenshot-001.png
│   ├── error-log.txt
│   └── api-response.json
└── test-results/           # Playwright test results
    ├── report.json         # Structured test results
    ├── index.html          # Playwright HTML report
    ├── screenshots/        # Test screenshots
    │   ├── dashboard-initial.png
    │   └── login-success.png
    └── traces/             # Playwright traces
        └── trace-1.zip
```

### Attachment Metadata (API Response)

```json
{
  "filename": "screenshot-001.png",
  "size": 245678,
  "content_type": "image/png",
  "created_at": "2026-01-12T10:30:00Z",
  "is_image": true
}
```

### Supported File Types

| Category | MIME Types |
|----------|------------|
| Images | `image/png`, `image/jpeg`, `image/gif`, `image/webp`, `image/svg+xml` |
| Text | `text/plain`, `text/markdown`, `text/csv` |
| Documents | `application/pdf`, `application/json` |
| Archives | `application/zip` |

### Filename Sanitization

Uploaded filenames are sanitized to prevent path traversal and filesystem issues:
- Path separators (`/`, `\`) are rejected
- Special directory names (`.`, `..`) are rejected
- Filenames are stored as-is after validation

---

## Artifact Formats

### spec.md

```markdown
# Specification: Add User Authentication

## Problem Statement
Users cannot authenticate; we need OAuth2 support.

## Success Criteria
- [ ] Google OAuth2 login works
- [ ] GitHub OAuth2 login works
- [ ] Session persists across page reload
- [ ] Logout clears session

## Scope
### In Scope
- OAuth2 authentication
- Session management

### Out of Scope
- Password authentication
- MFA

## Technical Approach
Use oauth2 library with provider-specific configs.
```

### review.md

```markdown
# Review: TASK-001 / implement

## Verdict: APPROVED

## Findings
### Major
- None

### Minor
- Line 45: Consider using constant for timeout value

## Tests
- 24/24 passing
- Coverage: 87%
```

### finalization-report.md

Generated by the `finalize` phase, documenting the sync with target branch and merge readiness.

```markdown
# Finalization Report: TASK-001

## Sync Summary

| Metric | Value |
|--------|-------|
| Target Branch | main |
| Commits Behind (before sync) | 5 |
| Conflicts Resolved | 2 |
| Files Changed (total) | 12 |
| Lines Changed (total) | 350 |

## Conflict Resolution

| File | Conflict Type | Resolution | Verified |
|------|---------------|------------|----------|
| src/api/handler.go | Same function | Merged both changes | ✓ |
| internal/config/config.go | Import conflicts | Combined imports | ✓ |

## Test Results

| Suite | Result | Notes |
|-------|--------|-------|
| Unit Tests | ✓ PASS | 156 tests |
| Integration Tests | ✓ PASS | 24 tests |
| Build | ✓ PASS | No warnings |

## Risk Assessment

| Factor | Value | Risk |
|--------|-------|------|
| Files Changed | 12 | Medium |
| Lines Changed | 350 | Medium |
| Conflicts Resolved | 2 | Low |
| **Overall Risk** | | **Medium** |

## Merge Decision

**Ready for Merge**: YES
**Recommended Action**: review-then-merge
```

#### Risk Classification

| Files Changed | Lines Changed | Risk Level | Recommended Action |
|---------------|---------------|------------|-------------------|
| 1-5 | <100 | Low | Auto-merge safe |
| 6-15 | 100-500 | Medium | Review recommended |
| 16-30 | 500-1000 | High | Careful review required |
| >30 | >1000 | Critical | Senior review mandatory |

#### Conflict Resolution Rules

The finalize phase enforces strict conflict resolution rules:

| Rule | Description |
|------|-------------|
| **NEVER remove features** | Both task changes AND upstream changes must be preserved |
| **Merge intentions** | Understand what each side was trying to accomplish |
| **Prefer additive** | When in doubt, keep both implementations |
| **Test per file** | Run tests after resolving each conflicted file |

---

## Task Comments (Database)

Comments and notes are stored in the SQLite database (`orc.db`), not in YAML files.

### Schema

```sql
CREATE TABLE task_comments (
    id TEXT PRIMARY KEY,           -- TC-{8 hex chars}
    task_id TEXT NOT NULL,         -- References tasks.id
    author TEXT NOT NULL,          -- Author name (default: "anonymous")
    author_type TEXT NOT NULL,     -- human | agent | system
    content TEXT NOT NULL,         -- Comment content
    phase TEXT,                    -- Optional: phase this relates to
    created_at TEXT NOT NULL,      -- RFC3339 timestamp
    updated_at TEXT NOT NULL       -- RFC3339 timestamp
);
```

### JSON Format (API Response)

```json
{
  "id": "TC-a1b2c3d4",
  "task_id": "TASK-001",
  "author": "claude",
  "author_type": "agent",
  "content": "This approach uses the existing auth flow\nwhich simplifies the implementation.",
  "phase": "implement",
  "created_at": "2026-01-10T10:30:00Z",
  "updated_at": "2026-01-10T10:30:00Z"
}
```

### Author Types

| Type | Description | Use Case |
|------|-------------|----------|
| `human` | Human user (default) | Review feedback, questions, notes |
| `agent` | AI agent | Claude notes during execution |
| `system` | System-generated | Automated process logs |

### Comment Statistics

```json
{
  "task_id": "TASK-001",
  "total_comments": 5,
  "human_count": 2,
  "agent_count": 2,
  "system_count": 1
}
```

---

## Test Results (Playwright)

Test results from Playwright E2E testing are stored in `.orc/tasks/TASK-XXX/test-results/`.

### Directory Structure

```
.orc/tasks/TASK-001/test-results/
├── report.json            # Structured test results
├── index.html             # Playwright HTML report (optional)
├── screenshots/           # Test screenshots
│   ├── dashboard-initial.png
│   ├── login-form.png
│   └── validate-success.png
└── traces/                # Playwright traces (optional)
    └── trace-1.zip
```

### Test Report Format (report.json)

```json
{
  "version": 1,
  "framework": "playwright",
  "started_at": "2026-01-10T10:30:00Z",
  "completed_at": "2026-01-10T10:35:00Z",
  "duration": 300000,
  "summary": {
    "total": 10,
    "passed": 9,
    "failed": 1,
    "skipped": 0
  },
  "suites": [
    {
      "name": "Login Flow",
      "tests": [
        {
          "name": "should login successfully",
          "status": "passed",
          "duration": 1500,
          "screenshots": ["login-success.png"],
          "trace": "trace-1.zip"
        },
        {
          "name": "should show error for invalid credentials",
          "status": "failed",
          "duration": 2000,
          "error": "Expected error message not found",
          "screenshots": ["login-error.png"]
        }
      ]
    }
  ],
  "coverage": {
    "percentage": 85.5,
    "lines": {
      "total": 1000,
      "covered": 855,
      "percent": 85.5
    },
    "branches": {
      "total": 200,
      "covered": 170,
      "percent": 85.0
    }
  }
}
```

### Test Result Status Values

| Status | Description |
|--------|-------------|
| `passed` | Test passed successfully |
| `failed` | Test failed with errors |
| `skipped` | Test was skipped |
| `pending` | Test not yet executed |

### Screenshot Naming Conventions

Screenshots should use descriptive names for easy identification:

| Pattern | Use Case | Example |
|---------|----------|---------|
| `{component}-initial.png` | Initial state | `dashboard-initial.png` |
| `{component}-{action}.png` | After action | `login-submit.png` |
| `{component}-error.png` | Error state | `form-validation-error.png` |
| `validate-{component}-{state}.png` | Validation phase | `validate-dashboard-after.png` |

### UI Testing Detection

Tasks automatically detect if UI testing is required based on keywords in the title and description:

| Keywords Detected | `requires_ui_testing` Set |
|-------------------|--------------------------|
| `ui`, `frontend`, `button`, `form`, `page` | `true` |
| `modal`, `dialog`, `component`, `widget` | `true` |
| `style`, `css`, `theme`, `responsive` | `true` |
| `click`, `hover`, `navigation`, `menu` | `true` |

When `requires_ui_testing: true`, the executor:
1. Configures Playwright MCP server in `.mcp.json`
2. Sets `SCREENSHOT_DIR` to `.orc/tasks/{id}/test-results/screenshots/`
3. Provides UI testing context to prompt templates
