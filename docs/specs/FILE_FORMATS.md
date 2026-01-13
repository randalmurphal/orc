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
branch: orc/TASK-001

created_at: 2026-01-10T10:30:00Z
created_by: randy
updated_at: 2026-01-10T12:45:00Z

metadata:
  source: cli
  tags: [auth, feature]
  priority: high
  external_id: JIRA-123          # Link to external tracker
```

---

## state.yaml (Execution State)

```yaml
# .orc/tasks/TASK-001/state.yaml
current_phase: implement
current_iteration: 3
status: running

started_at: 2026-01-10T10:31:00Z
total_duration: 2h15m

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

errors: []

tokens:
  total: 45000
  by_phase:
    classify: 2000
    spec: 15000
    implement: 28000
```

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
└── attachments/
    ├── screenshot-001.png
    ├── error-log.txt
    └── api-response.json
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
