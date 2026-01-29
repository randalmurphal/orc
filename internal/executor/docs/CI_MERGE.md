# CI Merger Reference

Handles CI polling and auto-merge after finalize phase completes. Bypasses hosting provider auto-merge features (which require branch protection on GitHub or pipeline configuration on GitLab).

## Types

```go
type CIStatus string

const (
    CIStatusPending  CIStatus = "pending"
    CIStatusPassed   CIStatus = "passed"
    CIStatusFailed   CIStatus = "failed"
    CIStatusNoChecks CIStatus = "no_checks"
)

type CICheckResult struct {
    Status       CIStatus  // Overall status
    TotalChecks  int       // Number of CI checks
    PassedChecks int       // Passed count
    FailedChecks int       // Failed count
    PendingChecks int      // Pending count
    FailedNames  []string  // Failed check names
    PendingNames []string  // Pending check names
    Details      string    // Status message
}
```

## Usage

```go
merger := NewCIMerger(cfg,
    WithCIMergerPublisher(publisher),
    WithCIMergerLogger(logger),
    WithCIMergerWorkDir(worktreePath),
)

// Main entry point - waits for CI then merges
err := merger.WaitForCIAndMerge(ctx, task)

// Or use individual methods
result, err := merger.WaitForCI(ctx, prURL, taskID)
result, err := merger.CheckCIStatus(ctx, prURL)
err := merger.MergePR(ctx, task)
```

## Flow

```
WaitForCIAndMerge
├── Check config (ShouldWaitForCI)
├── Get PR URL from task
├── WaitForCI(ctx, prURL, taskID)
│   ├── CheckCIStatus()  # Initial check
│   └── Poll loop (30s interval, 10m timeout)
│       └── CheckCIStatus() → passed|failed|pending
├── Check config (ShouldMergeOnCIPass)
└── MergePR(ctx, task)
    ├── Build merge options (commit title, templates, SHA verification)
    ├── Provider.MergePR(ctx, number, opts)
    └── Update task with merge info
```

## Configuration

```go
// Methods on *config.Config
cfg.ShouldWaitForCI()      // true for auto/fast profiles
cfg.ShouldMergeOnCIPass()  // true for auto/fast profiles
cfg.CITimeout()            // Default: 10m
cfg.CIPollInterval()       // Default: 30s
cfg.MergeMethod()          // Default: "squash"
cfg.Completion.CI.MergeCommitTemplate   // Template for merge commit message
cfg.Completion.CI.SquashCommitTemplate  // Template for squash commit message
cfg.Completion.CI.VerifySHAOnMerge      // Verify HEAD SHA before merge (default: true)
```

## CI Check Buckets

| Bucket | Treatment |
|--------|-----------|
| `pass` | Passed |
| `skipping` | Passed (treated as success) |
| `fail` | Failed |
| `cancel` | Failed (treated as failure) |
| `pending` | Pending |

## WebSocket Events

Progress is broadcast via `Transcript()` with phase="ci_merge":
- "Waiting for CI checks to pass..."
- "Waiting for CI... 3/5 passed, 2 pending"
- "CI checks passed. Merging PR..."
- "PR merged successfully!"

## Error Handling

| Scenario | Action |
|----------|--------|
| CI timeout | Log warning, return `ErrCITimeout`, PR remains open |
| CI failed | Log error with check names, return `ErrCIFailed`, PR remains open |
| Merge fails | Return wrapped error, PR remains open |

Errors don't fail the task - finalize succeeded and PR exists. User can merge manually.

## Commit Message Templates

Templates support variables: `{{TASK_ID}}`, `{{TASK_TITLE}}`, `{{TASK_BRANCH}}`

Example config:
```yaml
ci:
  merge_commit_template: "[{{TASK_ID}}] {{TASK_TITLE}}"
  squash_commit_template: "{{TASK_ID}}: {{TASK_TITLE}}"
```

## SHA Verification

When `verify_sha_on_merge: true` (default), MergePR fetches the current HEAD SHA and passes it to the merge request. This prevents merging stale PRs when the branch has been updated between CI check and merge.

If the SHA fetch fails (e.g., API rate limit), merge proceeds without verification (warning logged).
