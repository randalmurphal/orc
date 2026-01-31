# File Format Specifications

**Purpose**: Define data structures and file formats used by orc.

> **Note**: Task data (tasks, plans, states, specs, initiatives) is stored in **SQLite** (`~/.orc/projects/<id>/orc.db`), not YAML files. This document describes the data schemas for reference. Configuration files (`config.yaml`, `prompts/`) remain as files in the project `.orc/` directory. Use `orc export --all-tasks --all` for full portable backup.

---

## Export Format (Cross-Machine Portability)

Export files are YAML with versioning for compatibility. Default location: `~/.orc/projects/<id>/exports/`

### Task Export

```yaml
# .orc/exports/TASK-001.yaml (or stdout from orc export TASK-001)
version: 2                          # Export format version
exported_at: 2026-01-16T10:30:00Z

# Core task data
task:
  id: TASK-001
  title: "Add user auth"
  # ... full task definition

plan:
  # ... phase sequence

spec: |
  # Task Specification
  ...

state:
  # ... execution state

# Execution history (with --transcripts or --all)
transcripts:
  - task_id: TASK-001
    phase: implement
    iteration: 1
    role: combined
    content: |
      # implement - Iteration 1
      ## Prompt
      ...
      ## Response
      ...

# Gate decisions
gate_decisions:
  - task_id: TASK-001
    phase: spec
    gate_type: ai
    approved: true
    reason: "Meets criteria"

# Comments
task_comments:
  - id: comment-001
    author: randy
    content: "Consider edge case..."

review_comments:
  - id: review-001
    file_path: src/auth.go
    line_number: 42
    severity: suggestion
    content: "Could simplify this"

# Attachments (base64-encoded binary)
attachments:
  - filename: screenshot.png
    content_type: image/png
    size_bytes: 12345
    is_image: true
    data: <base64-encoded>
```

### Initiative Export

```yaml
# .orc/exports/initiatives/INIT-001.yaml
version: 2
exported_at: 2026-01-16T10:30:00Z
type: initiative                    # Distinguishes from task exports

initiative:
  id: INIT-001
  title: "User Authentication"
  vision: |
    JWT-based auth with refresh tokens...
  decisions:
    - id: decision-001
      decision: "Use bcrypt for passwords"
      rationale: "Industry standard"
  task_ids:
    - TASK-001
    - TASK-002
  blocked_by:
    - INIT-002
```

### Export Directory Structure

```
.orc/exports/
├── TASK-001.yaml
├── TASK-002.yaml
├── ...
└── initiatives/
    ├── INIT-001.yaml
    └── INIT-002.yaml
```

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
status: running                   # Task execution status (see Task Status Values below)
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

# PR Status (auto-populated when PR is created, updated via polling)
pr:
  url: https://github.com/owner/repo/pull/123
  number: 123
  status: approved               # draft | pending_review | changes_requested | approved | merged | closed
  checks_status: success         # pending | success | failure | none
  mergeable: true
  review_count: 2
  approval_count: 2
  last_checked_at: 2026-01-10T14:00:00Z
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

### PR Status

Tasks can have an associated pull request. PR status is tracked in the `pr` field:

| Field | Description |
|-------|-------------|
| `url` | Full URL to the pull request |
| `number` | PR number (e.g., 123 for PR #123) |
| `status` | Review status (see values below) |
| `checks_status` | CI check results: `pending`, `success`, `failure`, `none` |
| `mergeable` | Whether the PR can be merged (no conflicts) |
| `review_count` | Number of reviews received |
| `approval_count` | Number of approvals |
| `last_checked_at` | When PR status was last polled |

**PR Status Values:**

| Status | Description |
|--------|-------------|
| `draft` | PR is in draft state |
| `pending_review` | PR awaiting review |
| `changes_requested` | Reviewers requested changes |
| `approved` | PR has been approved |
| `merged` | PR has been merged |
| `closed` | PR was closed without merging |

**Automatic polling:**
- PR status is polled every 60 seconds for tasks with open PRs
- Polling skips tasks with merged/closed PRs
- Polling skips tasks polled within the last 30 seconds (rate limiting)
- Manual refresh available via `POST /api/tasks/:id/github/pr/refresh`

### Task Status Values

| Status | Description | UI Column |
|--------|-------------|-----------|
| `created` | Task created, not yet classified | Planning |
| `classifying` | AI classifying task weight | Planning |
| `planned` | Task has plan, ready to run | Planning |
| `running` | Task currently executing | Active phase column |
| `paused` | Task paused by user | Paused in current column |
| `blocked` | Task blocked by dependencies | Blocked |
| `completed` | All phases done, ready for finalize | Done |
| `finalizing` | Branch sync and merge in progress | Done (with progress) |
| `finished` | Task merged to target branch | Done (with merge info) |
| `failed` | Task failed with error | Failed |

**Finalize workflow statuses:**
- `completed` → `finalizing` → `finished`: Normal flow when finalize succeeds
- `completed` → `finalizing` → `failed`: If finalize encounters unresolvable issues
- UI shows different visual states in Done column for each status

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

### Database Storage

Initiatives are stored in SQLite (source of truth):
- `initiatives` table - Core initiative data
- `initiative_tasks` table - Task-to-initiative links
- `initiative_decisions` table - Decisions within initiatives
- `initiative_dependencies` table - Blocked-by relationships

**CLI behavior:**
CLI commands (`new`, `add-task`, `decide`, `activate`, `complete`) write directly to the database.

**Export for inspection:**
```bash
orc initiative show INIT-001 --format yaml
```

---

## Execution State (Embedded in Task)

Execution state is embedded in `orcv1.Task.Execution`, not stored as a separate entity. This consolidates task metadata and execution tracking into a single save operation.

### ExecutionState Structure

| Field | Type | Description |
|-------|------|-------------|
| `current_iteration` | int | Iteration count within current phase |
| `phases` | map[string]*orcv1.PhaseState | Per-phase execution state |
| `gates` | []GateDecision | Gate evaluation results |
| `tokens` | TokenUsage | Aggregate token usage |
| `cost` | CostTracking | Cost tracking |
| `session` | *SessionInfo | Claude session info |
| `error` | string | Last error message |
| `retry_context` | *RetryContext | Cross-phase retry information |

### PhaseState Structure

| Field | Type | Description |
|-------|------|-------------|
| `status` | PhaseStatus | pending, running, completed, failed, paused, interrupted, skipped, blocked |
| `started_at` | time.Time | When phase started |
| `completed_at` | *time.Time | When phase completed |
| `iterations` | int | Iteration count |
| `commit_sha` | string | Checkpoint commit |
| `error` | string | Error message (if failed/skipped) |
| `tokens` | TokenUsage | Per-phase token usage |
| `session_id` | string | Claude session ID for this phase |

### Example Task with Execution

```yaml
id: TASK-001
title: "Add user authentication"
status: running
current_phase: implement
execution:
  current_iteration: 3
  phases:
    spec:
      status: completed
      started_at: 2026-01-10T10:32:00Z
      completed_at: 2026-01-10T10:45:00Z
      iterations: 2
      tokens:
        input_tokens: 15000
        output_tokens: 5000
    implement:
      status: running
      started_at: 2026-01-10T10:45:00Z
      iterations: 3
  gates:
    - phase: spec
      gate_type: ai
      approved: true
      timestamp: 2026-01-10T10:45:00Z
  tokens:
    input_tokens: 45000
    output_tokens: 12000
    cache_read_input_tokens: 8000
    total_tokens: 57000
  cost:
    total_cost_usd: 0.85
```

### Executor Tracking (Database-Only)

Orphan detection uses task-level executor tracking stored in database columns (not in YAML):

| Field | Description |
|-------|-------------|
| `executor_pid` | Process ID of executor |
| `executor_hostname` | Machine running the executor |
| `executor_started_at` | When execution began |
| `last_heartbeat` | Last heartbeat timestamp |

**Persistence:** Call `backend.SaveTask(t)` to save both task metadata and execution state. Use `backend.UpdateTaskHeartbeat()` for periodic heartbeat updates.

### Token Fields

| Field | Description |
|-------|-------------|
| `input_tokens` | Uncached input tokens (billed at full rate) |
| `output_tokens` | Generated output tokens |
| `cache_creation_input_tokens` | Tokens written to cache this session (optional) |
| `cache_read_input_tokens` | Tokens served from cache (90% cheaper than input) |
| `total_tokens` | Sum of all token types |

**Note:** Raw `input_tokens` alone can appear misleadingly low when prompt caching is active. The "effective" input context is `input_tokens + cache_creation_input_tokens + cache_read_input_tokens`. UI displays show the combined cached total for clarity.

### Finalize State

When the finalize phase runs, additional state is tracked:

```yaml
# Added to state.yaml during finalize
finalize:
  status: running                    # not_started | pending | running | completed | failed
  started_at: 2026-01-10T14:30:00Z
  updated_at: 2026-01-10T14:32:00Z
  completed_at: null                 # Set on completion
  step: "Syncing with target"        # Current operation
  progress: "Merging main"           # Detailed progress message
  step_percent: 50                   # Completion percentage (0-100)
  result:                            # Only present on completion
    synced: true
    conflicts_resolved: 2
    conflict_files:
      - src/api/handler.go
      - internal/config/config.go
    tests_passed: true
    risk_level: medium               # low | medium | high
    files_changed: 12
    lines_changed: 350
    needs_review: false
    commit_sha: abc123def456
    target_branch: main
  error: null                        # Error message on failure
```

| Field | Description |
|-------|-------------|
| `status` | Finalize status: `not_started`, `pending`, `running`, `completed`, `failed` |
| `step` | Current operation name (e.g., "Syncing with target", "Running tests") |
| `progress` | Human-readable progress message |
| `step_percent` | Completion percentage (0-100) |
| `result` | Finalize result object (only present on completion) |
| `error` | Error message (only present on failure) |

**Result fields:**

| Field | Type | Description |
|-------|------|-------------|
| `synced` | boolean | Whether branch was synced with target |
| `conflicts_resolved` | number | Number of merge conflicts resolved |
| `conflict_files` | string[] | List of files that had conflicts |
| `tests_passed` | boolean | Whether tests passed after sync |
| `risk_level` | string | Risk assessment: `low`, `medium`, `high` |
| `files_changed` | number | Total files modified in diff |
| `lines_changed` | number | Total lines added/removed |
| `needs_review` | boolean | Whether human review is recommended |
| `commit_sha` | string | Final merged commit SHA |
| `target_branch` | string | Branch merged into |

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

### Spec Content (Database)

Spec content is stored in the SQLite database (`specs` table), not as file artifacts. This avoids merge conflicts when running parallel tasks in worktrees.

**Schema:**
```sql
CREATE TABLE specs (
    task_id TEXT PRIMARY KEY,    -- References tasks.id
    content TEXT NOT NULL,       -- Markdown spec content
    source TEXT NOT NULL,        -- Source identifier (e.g., "spec-phase")
    created_at TEXT NOT NULL,    -- RFC3339 timestamp
    updated_at TEXT NOT NULL     -- RFC3339 timestamp
);
```

**Example content format:**
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

**API access:** `GET /api/tasks/:id/spec` returns spec content; `PUT /api/tasks/:id/spec` saves spec to database.

**Template variable:** `{{SPEC_CONTENT}}` is populated via `WithSpecFromDatabase()` in executor templates.

**Legacy fallback:** For backward compatibility, `ArtifactDetector` checks the database first (via `NewArtifactDetectorWithBackend`), then falls back to legacy `spec.md` files if they exist.

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
1. Enables Playwright MCP server via phase template's `claude_config.mcp_servers`
2. Sets `SCREENSHOT_DIR` to `.orc/tasks/{id}/test-results/screenshots/`
3. Provides UI testing context to prompt templates

---

## Initiative Task Manifest

The manifest format allows bulk creation of tasks for an initiative from a single YAML file. Tasks with inline specs skip the spec phase during execution.

### Command

```bash
orc initiative plan <manifest.yaml>           # Create tasks, prompt for confirm
orc initiative plan <manifest.yaml> --dry-run # Preview without creating
orc initiative plan <manifest.yaml> --yes     # Skip confirmation prompt
orc initiative plan <manifest.yaml> --create-initiative  # Create initiative if missing
```

### Manifest Format

```yaml
# initiative-tasks.yaml
version: 1                        # Required: manifest format version

# Target initiative (use one of these)
initiative: INIT-001              # Existing initiative ID
# OR
create_initiative:                # Create new initiative
  title: "User Authentication"    # Required
  vision: "OAuth2 support"        # Optional

# Task definitions
tasks:
  - id: 1                         # Local ID for dependency references
    title: "Add OAuth2 config"    # Required
    description: |                # Optional
      Add configuration structure for OAuth2 providers.
    weight: small                 # Optional: trivial/small/medium/large/greenfield
    category: feature             # Optional: feature/bug/refactor/chore/docs/test
    priority: normal              # Optional: critical/high/normal/low
    depends_on: []                # Optional: local IDs of prerequisite tasks
    spec: |                       # Optional: inline specification
      # Specification: Add OAuth2 configuration

      ## Success Criteria
      - [ ] Config struct for OAuth2 settings
      - [ ] Environment variable support

  - id: 2
    title: "Implement Google OAuth2"
    weight: medium
    depends_on: [1]               # Depends on task with local ID 1
    spec: |
      # Specification: Google OAuth2
      ...

  - id: 3
    title: "Implement GitHub OAuth2"
    weight: medium
    depends_on: [1]

  - id: 4
    title: "Add auth middleware"
    weight: small
    depends_on: [2, 3]            # Can depend on multiple tasks
    # No spec = will run spec phase during execution
```

### Field Reference

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `version` | Yes | - | Manifest format version (currently 1) |
| `initiative` | One of these | - | ID of existing initiative |
| `create_initiative` | One of these | - | Details for creating new initiative |
| `tasks` | Yes | - | List of task definitions |
| `tasks[].id` | Yes | - | Local ID for dependency references |
| `tasks[].title` | Yes | - | Task title |
| `tasks[].description` | No | - | Task description |
| `tasks[].weight` | No | medium | Task complexity |
| `tasks[].category` | No | feature | Task category |
| `tasks[].priority` | No | normal | Task priority |
| `tasks[].depends_on` | No | [] | Local IDs of prerequisite tasks |
| `tasks[].spec` | No | - | Inline spec (skips spec phase) |

### Validation Rules

1. **Version**: Must be `1` (current version)
2. **Initiative**: Either `initiative` or `create_initiative` must be specified (not both)
3. **Tasks**: At least one task required
4. **Local IDs**: Must be unique positive integers
5. **Dependencies**: Must reference valid local IDs, no circular dependencies
6. **Enum values**: weight/category/priority must be valid values

### Dependency Resolution

Tasks are created in topological order (dependencies first), ensuring:
- Local IDs map to actual TASK-IDs as tasks are created
- Dependencies in the manifest become proper `blocked_by` relationships
- Tasks with satisfied dependencies can run immediately

### Inline Specs

When a task includes the `spec` field:
- The spec content is stored in the database
- The task skips the spec phase during execution
- The task starts directly with the implement phase (or next applicable phase)

### Example Workflow

```bash
# Create manifest file
cat > auth-tasks.yaml << 'EOF'
version: 1
create_initiative:
  title: "User Authentication"
  vision: "OAuth2 support for Google and GitHub"
tasks:
  - id: 1
    title: "Add OAuth config"
    weight: small
    spec: |
      # Specification: Add OAuth config
      ## Success Criteria
      - [ ] Config struct exists
EOF

# Preview
orc initiative plan auth-tasks.yaml --dry-run

# Create tasks
orc initiative plan auth-tasks.yaml --yes

# Output:
# Created initiative: INIT-003
# Created task: TASK-045 - Add OAuth config [small] (spec stored)
#
# Summary: 1 task(s) created in INIT-003
```
