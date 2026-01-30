# Branch Control for Tasks

Design for branch naming, target branch selection, and PR settings at project and task levels.

## Problem

Branch and PR settings exist in orc but have discoverability and flexibility issues:
- No custom branch names per task (stuck with `orc/TASK-001` pattern)
- PR settings (draft, reviewers, labels) only configurable at project level
- Frontend doesn't expose git settings prominently
- Users can't easily override defaults for specific tasks

## Goals

1. **Custom branch names** - Tasks can specify explicit branch names
2. **Task-level PR overrides** - Draft mode, reviewers, labels per task
3. **Project-level defaults** - Clear place to configure git defaults
4. **Frontend discoverability** - Git settings visible in settings page and task forms

## Non-Goals

- Branch naming templates (just explicit names or auto-generated)
- Merge strategies (out of scope)
- CI configuration (out of scope)
- Sync strategies (out of scope)

## Design

### Data Model

#### Task Fields (new)

| Field | Type | Purpose |
|-------|------|---------|
| `branch_name` | `string?` | User-specified branch name (empty = auto from task ID) |
| `pr_draft` | `bool?` | Override draft mode (null = use project default) |
| `pr_labels` | `[]string` | Override labels |
| `pr_reviewers` | `[]string` | Override reviewers |
| `pr_labels_set` | `bool` | True = use `pr_labels` even if empty |
| `pr_reviewers_set` | `bool` | True = use `pr_reviewers` even if empty |

The `*_set` flags distinguish "not configured" (inherit default) from "explicitly empty" (no labels/reviewers).

#### Project Settings (existing, ensure exposed)

```yaml
completion:
  target_branch: main
  pr:
    draft: false
    labels: [automated, orc]
    reviewers: [alice, bob]
    team_reviewers: [platform-team]
```

### Resolution Logic

```
Branch Name:
  task.branch_name → default: git.BranchName(task.ID, prefix)

Target Branch:
  task.target_branch → initiative.branch_base → config.completion.target_branch → "main"

PR Draft:
  task.pr_draft → config.completion.pr.draft → false

PR Labels:
  task.pr_labels_set ? task.pr_labels : config.completion.pr.labels

PR Reviewers:
  task.pr_reviewers_set ? task.pr_reviewers : config.completion.pr.reviewers
```

### Proto Changes

**`proto/orc/v1/task.proto`**

Add to `Task` message:
```protobuf
message Task {
  // ... existing fields ...
  optional string branch_name = 25;       // User-specified branch name
  optional bool pr_draft = 26;            // Override draft mode
  repeated string pr_labels = 27;         // Override labels
  repeated string pr_reviewers = 28;      // Override reviewers
  bool pr_labels_set = 29;                // True = use pr_labels
  bool pr_reviewers_set = 30;             // True = use pr_reviewers
}
```

Add to `CreateTaskRequest`:
```protobuf
message CreateTaskRequest {
  // ... existing fields (target_branch already at 10) ...
  optional string branch_name = 14;
  optional bool pr_draft = 15;
  repeated string pr_labels = 16;
  repeated string pr_reviewers = 17;
  optional bool pr_labels_set = 18;
  optional bool pr_reviewers_set = 19;
}
```

Add to `UpdateTaskRequest`:
```protobuf
message UpdateTaskRequest {
  // ... existing fields ...
  optional string branch_name = 14;       // Only modifiable before execution
  optional bool pr_draft = 15;
  repeated string pr_labels = 16;
  repeated string pr_reviewers = 17;
  optional bool pr_labels_set = 18;
  optional bool pr_reviewers_set = 19;
}
```

### CLI Changes

**`orc new`**
```bash
orc new "Add login" --branch feature/JIRA-123-login
orc new "Feature" --target-branch develop          # Already exists
orc new "Feature" --draft
orc new "Feature" --reviewers alice,bob
orc new "Feature" --labels bug,urgent
```

**`orc edit`**
```bash
orc edit TASK-001 --branch feature/new-name        # Before execution only
orc edit TASK-001 --target-branch main
orc edit TASK-001 --draft=false
orc edit TASK-001 --reviewers ""                   # Explicit empty
```

**`orc go`**
Inherits same flags.

**Validation:**
- `--branch` uses `git.ValidateBranchName()`
- `--branch` rejected if task execution already started
- Warning if `--branch` conflicts with initiative prefix

### Backend Changes

**Branch Resolution** (`internal/executor/branch.go`)

```go
func ResolveBranchName(task *Task, cfg *Config) string {
    if task.BranchName != nil && *task.BranchName != "" {
        return *task.BranchName
    }
    return git.BranchName(task.ID, cfg.BranchPrefix)
}
```

**PR Options Resolution** (`internal/executor/workflow_completion.go`)

```go
func ResolvePROptions(task *Task, cfg *Config) hosting.PRCreateOptions {
    opts := hosting.PRCreateOptions{
        Draft:     cfg.Completion.PR.Draft,
        Labels:    cfg.Completion.PR.Labels,
        Reviewers: cfg.Completion.PR.Reviewers,
    }

    if task.PrDraft != nil {
        opts.Draft = *task.PrDraft
    }
    if task.PrLabelsSet {
        opts.Labels = task.PrLabels
    }
    if task.PrReviewersSet {
        opts.Reviewers = task.PrReviewers
    }
    return opts
}
```

**Task Handlers** (`internal/api/task_handlers.go`)
- `CreateTask`: Copy new fields from request to task
- `UpdateTask`: Validate `branch_name` not changed after execution, copy fields

### Frontend Changes

#### Project Settings Page (`/settings/git`)

New dedicated page for git configuration:

```
┌─────────────────────────────────────────────────────────────┐
│ Git & Pull Requests                                         │
├─────────────────────────────────────────────────────────────┤
│   Hosting Provider    [Auto-detect ▼]                       │
│                                                             │
│   ─── Branches ───                                          │
│   Default Target Branch    [main        ]                   │
│   Branch Prefix            [orc/        ]                   │
│                                                             │
│   ─── Pull Request Defaults ───                             │
│   Create as Draft          [ ]                              │
│   Default Labels           [automated, orc]                 │
│   Default Reviewers        [alice, bob    ]                 │
│   Team Reviewers           [platform-team ]                 │
│                                                             │
│   ─── PR Templates ───                                      │
│   Title Template           [[orc] {{TASK_TITLE}}]           │
│   Body Template Path       [templates/pr-body.md]           │
└─────────────────────────────────────────────────────────────┘
```

#### Task Creation Modal

Add Git section (visible by default, not hidden):

```
┌─────────────────────────────────────────────────────────────┐
│ New Task                                                    │
├─────────────────────────────────────────────────────────────┤
│   Title         [Add user authentication               ]    │
│   Weight        [medium ▼]     Category  [feature ▼]        │
│                                                             │
│   ─── Git ───                                               │
│   Branch        [                 ] (placeholder: auto)     │
│   Target Branch [develop          ]                         │
│                                                             │
│   ▶ PR Settings (click to expand)                           │
└─────────────────────────────────────────────────────────────┘
```

Expanded PR Settings:
```
│   ▼ PR Settings                                             │
│   Draft         [Use default ▼]                             │
│   Labels        [         ]                                 │
│   Reviewers     [         ]                                 │
└─────────────────────────────────────────────────────────────┘
```

#### Task Edit Modal

Same fields as creation, but:
- `Branch` field disabled after execution starts
- Shows "(from project)" hint when inheriting defaults

#### Task Detail View

Display git info prominently:

```
┌─────────────────────────────────────────────────────────────┐
│ TASK-001: Add user authentication                           │
├─────────────────────────────────────────────────────────────┤
│ Branch: feature/JIRA-123-login  →  Target: develop          │
│ PR: Draft, Labels: [auth], Reviewers: alice                 │
└─────────────────────────────────────────────────────────────┘
```

### Files to Modify

| Layer | Files |
|-------|-------|
| **Proto** | `proto/orc/v1/task.proto` |
| **Backend** | `internal/api/task_handlers.go`, `internal/executor/branch.go`, `internal/executor/workflow_completion.go`, `internal/task/proto_helpers.go` |
| **CLI** | `internal/cli/cmd_new.go`, `internal/cli/cmd_edit.go`, `internal/cli/cmd_go.go` |
| **Web** | New: `web/src/pages/settings/GitSettings.tsx`; Modify: task creation/edit modals, task detail, settings layout |

### Implementation Order

1. **Proto changes** - Add fields, regenerate clients
2. **Backend** - Storage, task handlers, branch/PR resolution
3. **CLI** - Add flags to `new`, `edit`, `go`
4. **Web UI** - Settings page, then task forms

### Testing

- Unit tests for resolution logic (branch name, PR options)
- Integration tests for CLI flags
- E2E tests for settings page and task creation with overrides
