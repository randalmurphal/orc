# Executor Model

**Purpose**: Ralph-style execution loops within structured phases.

> **Code Reference**: See `internal/executor/CLAUDE.md` for implementation details.

---

## Module Structure

The executor package is organized into focused modules:

| Module | File | Responsibility |
|--------|------|----------------|
| **Core** | `executor.go` | Main orchestrator, task lifecycle |
| **Task Execution** | `task_execution.go` | ExecuteTask, ResumeFromPhase, gate evaluation |
| **Phase Execution** | `phase.go` | ExecutePhase, executor dispatch |
| **Execution Context** | `execution_context.go` | `BuildExecutionContext()` - centralized context building |
| **Claude Executor** | `claude_executor.go` | `TurnExecutor` interface, ClaudeCLI wrapper |
| **Trivial Executor** | `trivial.go` | ClaudeExecutor, no session persistence |
| **Standard Executor** | `standard.go` | ClaudeExecutor per phase, iteration loop |
| **Full Executor** | `full.go` | ClaudeExecutor with per-iteration checkpointing |
| **Finalize Executor** | `finalize.go` | Branch sync, conflict resolution, risk assessment |
| **Publishing** | `publish.go` | Nil-safe EventPublisher |
| **Templates** | `template.go` | Prompt variable substitution |
| **Retry** | `retry.go` | Cross-phase retry context |
| **Worktree** | `worktree.go` | Git worktree isolation |
| **Completion** | `completion.go` | Phase completion detection |

---

## Executor Strategies

All executors use the unified `ClaudeExecutor` via `TurnExecutor` interface. They differ in session handling and checkpointing:

| Executor | Session | Checkpointing | Max Iterations | Best For |
|----------|---------|---------------|----------------|----------|
| **Trivial** | None | None | 5 | Quick single-prompt tasks |
| **Standard** | Per-phase | On completion | 20 | Small/medium tasks |
| **Full** | Persistent | Every iteration | 30-50 | Large/greenfield |

### Specialized Executors

The **Finalize Executor** handles the finalize phase specifically:

| Aspect | Behavior |
|--------|----------|
| **Session** | Per-phase (for conflict resolution) |
| **Checkpointing** | On completion only |
| **Max Iterations** | 10 (lower - mostly git ops) |
| **Best For** | Branch sync and merge preparation |

---

## Execution Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         EXECUTOR                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────┐    ┌─────────────┐    ┌─────────────┐    │
│  │ BuildExecution  │───►│   Claude    │───►│   Output    │    │
│  │    Context()    │    │   Executor  │    │   Parser    │    │
│  └─────────────────┘    └─────────────┘    └─────────────┘    │
│        ▲                                          │            │
│        │                                          ▼            │
│        │                                  ┌─────────────┐      │
│        │                                  │ Completion  │      │
│        │                                  │  Detector   │      │
│        │                                  └──────┬──────┘      │
│        │                                         │             │
│        │             ┌───────────────────────────┤             │
│        │             │                           │             │
│        │             ▼                           ▼             │
│  ┌─────────────────────┐               ┌─────────────┐        │
│  │  NOT COMPLETE       │               │  COMPLETE   │        │
│  │  (loop continues)   │               │ (checkpoint)│        │
│  └─────────────────────┘               └─────────────┘        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

All executors share unified context building and Claude invocation:
- BuildExecutionContext() handles template rendering, spec loading, context injection
- NewClaudeExecutorFromContext() creates TurnExecutor with consistent CLI options
- TurnExecutor.ExecuteTurn() sends prompts with --json-schema for completion detection
```

---

## Phase Execution Loop

```go
func RunPhase(task *Task, phase *Phase) error {
    for i := 0; i < phase.MaxIterations; i++ {
        // 1. Construct prompt with current context
        prompt := ConstructPrompt(task, phase)
        
        // 2. Run Claude Code subprocess
        result := RunClaudeSession(prompt)
        
        // 3. Save transcript
        SaveTranscript(task, phase, i, result)
        
        // 4. Check completion criteria
        if MeetsCriteria(result, phase.CompletionCriteria) {
            Checkpoint(task, phase, "completed")
            return nil
        }
        
        // 5. Periodic checkpoint (every N iterations, starting from 0)
        // Example: frequency=3 checkpoints at iterations 0, 3, 6, 9...
        if i % phase.CheckpointFrequency == 0 {
            Checkpoint(task, phase, fmt.Sprintf("iteration-%d", i))
        }
        
        // 6. Stuck detection
        if IsStuck(task, result, 3) {
            return ErrStuck
        }
    }
    
    return ErrMaxIterations
}
```

---

## Completion Detection

### JSON Completion Pattern

Claude outputs completion signals as JSON:

```markdown
I've completed the implementation. All tests pass.

{"status": "complete", "summary": "Implemented feature X with tests"}
```

### Parsing Logic

```go
type PhaseResponse struct {
    Status  string `json:"status"`  // complete, blocked, continue
    Reason  string `json:"reason,omitempty"`
    Summary string `json:"summary,omitempty"`
}

func CheckPhaseCompletionJSON(content string) (PhaseCompletionStatus, string) {
    resp, err := ParsePhaseResponse(content)
    if err != nil {
        return PhaseStatusContinue, ""
    }
    switch resp.Status {
    case "complete":
        return PhaseStatusComplete, resp.Summary
    case "blocked":
        return PhaseStatusBlocked, resp.Reason
    default:
        return PhaseStatusContinue, resp.Reason
    }
}
```

**Note:** `--json-schema` only works with `--print` mode. For session-based output,
use `ExtractPhaseResponse()` which falls back to Haiku LLM extraction.

### Additional Criteria

| Criterion | Check Method |
|-----------|--------------|
| `all_tests_pass` | Run `go test ./...`, check exit code |
| `no_lint_errors` | Run linter, check exit code |
| `files_exist` | Check filesystem |
| `coverage_above: N` | Parse coverage report, verify >= N% |
| `claude_confirms` | Claude outputs `{"status": "complete"}` |
| `spec_complete` | Spec artifact exists and passes AI validation |
| `review_approved` | Review phase completed with no major findings |
| `design_approved` | Design document exists and approved |
| `custom: <cmd>` | Run custom command, check exit code 0 |

### Playwright MCP Validation Criteria (for validate phase)

| Criterion | Check Method |
|-----------|--------------|
| `playwright_e2e_pass` | All Playwright MCP tests complete without errors |
| `all_components_covered` | Every UI component tested via browser_snapshot |
| `no_console_errors` | browser_console_messages returns no errors |
| `no_failed_network_requests` | browser_network_requests shows no failures |
| `accessibility_validated` | browser_snapshot captures valid accessibility tree |
| `visual_regression_pass` | Screenshots match baselines (if configured) |

---

## Claude Code Invocation

```go
func RunClaudeSession(prompt string) (*Result, error) {
    // Create temp file with prompt
    promptFile := writeTempFile(prompt)
    defer os.Remove(promptFile)
    
    // Build command
    cmd := exec.Command("claude", 
        "--print", promptFile,
        "--output-format", "json",
        "--max-tokens", "100000",
    )
    
    // Capture output
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    // Run with timeout
    ctx, cancel := context.WithTimeout(context.Background(), phase.Timeout)
    defer cancel()
    
    err := cmd.Run()
    
    return &Result{
        Output:   stdout.String(),
        Errors:   stderr.String(),
        ExitCode: cmd.ProcessState.ExitCode(),
    }, err
}
```

---

## Stuck Detection

Detect when Claude is spinning on the same issue.

### Error Signature Extraction

Error signatures are extracted by:
1. Finding error patterns in output (lines starting with `error:`, `Error:`, `FAILED`, etc.)
2. Normalizing file paths (remove timestamps, line numbers)
3. Taking first 200 characters of normalized error
4. Hashing for comparison

```go
func extractErrorSignature(result *Result) string {
    // Extract error lines from output
    errorLines := extractErrorLines(result.Errors + result.Output)
    if len(errorLines) == 0 {
        return "" // No errors detected
    }

    // Normalize: remove timestamps, paths, line numbers
    normalized := normalizeErrors(errorLines)

    // Truncate and hash for comparison
    if len(normalized) > 200 {
        normalized = normalized[:200]
    }
    return sha256(normalized)[:16]
}
```

### Stuck Detector

```go
type StuckDetector struct {
    errorHistory []string
    threshold    int  // Default: 3 consecutive identical errors
}

func (s *StuckDetector) IsStuck(result *Result) bool {
    errorSig := extractErrorSignature(result)
    if errorSig == "" {
        return false // No errors = not stuck
    }

    // Count consecutive same errors
    count := 0
    for i := len(s.errorHistory) - 1; i >= 0; i-- {
        if s.errorHistory[i] == errorSig {
            count++
        } else {
            break
        }
    }

    s.errorHistory = append(s.errorHistory, errorSig)
    return count >= s.threshold
}
```

### When Stuck

1. **Create `.stuck.md`** with analysis:
```markdown
<!-- .orc/tasks/TASK-001/.stuck.md -->
# Stuck Analysis

**Phase**: implement
**Iteration**: 7
**Consecutive identical errors**: 3

## Error Pattern
```
cannot find package "github.com/example/missing"
```

## Context
- Last 3 iterations produced identical error
- Error suggests missing dependency

## Suggested Actions
1. Check if package exists in go.mod
2. Run `go mod tidy`
3. Verify package URL is correct

## To Resume
```bash
orc run TASK-001 --continue
```
```

2. **Set task status** to `stuck`
3. **Notify human** via configured channels
4. **Skip to next phase** if `skip_on_stuck: true` configured

---

## Cross-Phase Retry

When a later phase fails (e.g., tests fail), orc can automatically retry from an earlier phase with context about what went wrong.

### Retry Flow

```
implement → test (FAIL) → implement (retry with failure context) → test (PASS)
```

### Configuration

```yaml
# .orc/config.yaml
retry:
  enabled: true
  retry_map:
    test: implement      # If test fails, retry from implement
    validate: implement  # If validate fails, retry from implement

executor:
  max_retries: 5         # Max retry attempts per phase (default: 5)
```

The `executor.max_retries` setting controls how many times orc will retry before giving up. The default is 5 attempts.

**Environment variable**: `ORC_EXECUTOR_MAX_RETRIES`

### Retry Context

When retrying, the phase receives a `{{RETRY_CONTEXT}}` template variable containing:
- Which phase failed and why
- The failure output (test errors, validation messages)
- Attempt number

This helps Claude understand what needs fixing.

### Retry Limits

| Setting | Location | Default | Description |
|---------|----------|---------|-------------|
| `executor.max_retries` | config.yaml | 5 | Primary setting for retry limit |
| `retry.max_retries` | config.yaml | 5 | Deprecated, use `executor.max_retries` |

---

## Finalize Executor

The Finalize Executor is a specialized executor for the `finalize` phase, handling branch synchronization, conflict resolution, test verification, and risk assessment before merge.

### Execution Steps

```
1. Fetch target branch    → Get latest changes from remote
2. Check divergence       → Count commits ahead/behind
3. Sync with target       → Merge or rebase (per config)
4. Resolve conflicts      → AI-assisted if conflicts detected
5. Run tests              → Verify tests pass after sync
6. Fix tests (if needed)  → AI attempts to fix failures
7. Risk assessment        → Classify merge risk level
8. Create finalize commit → Document finalization
```

### Sync Strategies

| Strategy | Behavior | Use Case |
|----------|----------|----------|
| `merge` (default) | Merge target into task branch | Preserves commit history |
| `rebase` | Rebase task branch onto target | Linear history, cleaner |

### Conflict Resolution

When conflicts are detected, Claude is invoked with specific rules:

| Rule | Description |
|------|-------------|
| **Never remove features** | Both task and upstream changes must be preserved |
| **Merge intentions** | Understand what each side was trying to accomplish |
| **Prefer additive** | When in doubt, keep both implementations |
| **Test per file** | Verify after resolving each conflicted file |

**Prohibited resolutions:**
- Taking "ours" or "theirs" without understanding
- Removing upstream features to fix conflicts
- Commenting out conflicting code

### Risk Assessment

After sync, changes are assessed for merge risk:

| Metric | Low | Medium | High | Critical |
|--------|-----|--------|------|----------|
| Files changed | 1-5 | 6-15 | 16-30 | >30 |
| Lines changed | <100 | 100-500 | 500-1000 | >1000 |
| Conflicts | 0 | 1-3 | 4-10 | >10 |

Risk level determines whether re-review is triggered (configurable via `re_review_threshold`).

### Escalation to Implement

Finalize can escalate back to the implement phase when:
- More than 10 conflicts couldn't be resolved
- More than 5 tests fail after multiple fix attempts

The implement phase receives `{{RETRY_CONTEXT}}` with:
- List of unresolved conflicts
- Test failure details
- Instructions to fix and retry finalize

### Auto-Trigger on PR Approval

When automation profile is `auto` and a PR is approved:
1. PR status poller detects approval (polls every 60s)
2. `TriggerFinalizeOnApproval` is called automatically
3. Finalize runs in background (async)
4. WebSocket events broadcast progress

**Conditions for auto-trigger:**
- `completion.finalize.enabled` is `true`
- `completion.finalize.auto_trigger_on_approval` is `true`
- Automation profile is `auto` (fully automated)
- Task has weight that supports finalize (not `trivial`)
- Task status is `completed` (has an approved PR)
- Finalize hasn't already completed

### Configuration

```yaml
# .orc/config.yaml
completion:
  finalize:
    enabled: true                  # Enable finalize phase
    auto_trigger: true             # Run after validate phase
    auto_trigger_on_approval: true # Run when PR is approved (auto profile only)
    sync:
      strategy: merge              # merge | rebase
    conflict_resolution:
      enabled: true                # AI-assisted conflict resolution
      instructions: ""             # Additional resolution instructions
    risk_assessment:
      enabled: true                # Enable risk classification
      re_review_threshold: high    # low | medium | high | critical
    gates:
      pre_merge: auto              # auto | ai | human | none
```

See `docs/specs/CONFIG_HIERARCHY.md` for full configuration options.

---

## Auto-Approve PRs

In `auto` and `fast` automation profiles, PRs are automatically approved after creation. This makes the automation truly end-to-end without requiring human intervention for approval.

### Approval Flow

After PR creation, the auto-approve process:

1. **Get PR diff** - Retrieve the changes via `gh pr diff`
2. **Check CI status** - Verify tests/checks via `gh pr checks`
3. **Review and approve** - If CI passes, approve via `gh pr review --approve`

### CI Status Evaluation

| Status | Action |
|--------|--------|
| All checks passed | Approve PR |
| Checks pending | Approve PR (CI still running) |
| Checks failed | Skip approval with warning |
| No checks configured | Approve PR |

### Approval Comment

The approval includes a summary comment:

```
Auto-approved by orc orchestrator.

**Review Summary:**
- Task: <task title>
- CI Status: All checks passed
- Implementation: Completed via AI-assisted development
- Tests: Passed during test phase
- Validation: Completed during validate phase
```

### Profile Behavior

| Profile | Auto-Approve | Rationale |
|---------|--------------|-----------|
| `auto` | Yes | Fully automated pipeline |
| `fast` | Yes | Speed prioritized |
| `safe` | No | Human approval required |
| `strict` | No | Human approval required |

### Configuration

```yaml
# .orc/config.yaml
completion:
  pr:
    auto_approve: true    # Enable AI-assisted PR approval (default: true for auto/fast)
```

When `auto_approve` is enabled and the profile is `auto` or `fast`:
- PRs are approved immediately after creation if CI passes
- Approval failures are logged but don't fail the task
- The PR is still created even if approval fails

---

## CI Wait and Auto-Merge

After the finalize phase completes, orc can automatically wait for CI checks to pass and then merge the PR directly via the GitHub REST API. This bypasses GitHub's auto-merge feature (which requires branch protection) and avoids issues with worktrees.

### Why REST API Instead of CLI?

The `gh pr merge` CLI command has limitations:
- Tries to fast-forward the local target branch after merge
- Fails when target branch is checked out in another worktree (common case)
- Error: `fatal: 'main' is already used by worktree at '/path/to/repo'`

Orc's REST API merge flow:
- Merges server-side only (no local git operations)
- Works regardless of which branch is checked out locally
- Polls CI status and merges when all checks pass
- Gets merge commit SHA directly from API response
- Deletes branch via API after merge (no local branch cleanup issues)

### Auto-Merge Flow

After finalize phase completes successfully:

```
1. Push finalize changes     → Sync commits, conflict resolutions
2. Poll CI checks            → Wait for all checks to pass (or timeout)
3. Merge PR via API          → PUT /repos/{owner}/{repo}/pulls/{number}/merge
4. Delete branch via API     → DELETE /repos/{owner}/{repo}/git/refs/heads/{branch}
5. Update task state         → Record merge commit SHA, set status to finished
```

### CI Status Evaluation

| Status | Action |
|--------|--------|
| All checks passed | Proceed to merge |
| Checks pending | Continue polling |
| Checks failed | Abort with error, PR remains open |
| No checks configured | Treat as passed, proceed to merge |
| Timeout reached | Abort, PR remains open for manual merge |

### Polling Behavior

The CI merger polls `gh pr checks` at regular intervals:

| Setting | Default | Purpose |
|---------|---------|---------|
| `poll_interval` | 30s | Time between CI status checks |
| `ci_timeout` | 10m | Max time to wait for CI |

During polling, WebSocket events broadcast progress:
- "Waiting for CI checks to pass..."
- "CI: 3/5 passed, 2 pending"
- "CI checks passed. Merging PR..."
- "PR merged successfully!"

### Merge Methods

| Method | API Value | Behavior |
|--------|-----------|----------|
| `squash` (default) | `merge_method=squash` | Combines all commits into one |
| `merge` | `merge_method=merge` | Creates merge commit |
| `rebase` | `merge_method=rebase` | Rebases commits onto target |

### Profile Behavior

| Profile | Wait for CI | Auto-Merge | Rationale |
|---------|-------------|------------|-----------|
| `auto` | Yes | Yes | Fully automated pipeline |
| `fast` | Yes | Yes | Speed prioritized, but CI gate maintained |
| `safe` | No | No | Human must review and merge |
| `strict` | No | No | Human gates on all merge decisions |

### Configuration

```yaml
# .orc/config.yaml
completion:
  ci:
    wait_for_ci: true         # Wait for CI checks before merge (default: true)
    ci_timeout: 10m           # Max time to wait for CI (default: 10m)
    poll_interval: 30s        # CI status polling interval (default: 30s)
    merge_on_ci_pass: true    # Auto-merge when CI passes (default: true)
    merge_method: squash      # squash | merge | rebase (default: squash)
  delete_branch: true         # Delete branch after merge (default: true)
```

### Merge Retry Logic

When parallel tasks target the same branch, one task may merge first, causing subsequent merges to receive HTTP 405 "Base branch was modified" from GitHub. Orc handles this with automatic retry:

| Attempt | Backoff | Action |
|---------|---------|--------|
| 1 | 0s | Initial merge attempt |
| 2 | 2s | Rebase onto target, retry merge |
| 3 | 4s | Rebase onto target, retry merge |
| 4 | 8s | Final rebase and retry |

**Retry flow:**
1. Attempt merge via GitHub REST API
2. If 405 "Base branch was modified" received:
   - Wait with exponential backoff (2^attempt seconds, max 8s)
   - Fetch latest from origin
   - Rebase task branch onto target (e.g., `origin/main`)
   - Push rebased branch with `--force-with-lease`
   - Retry merge
3. If rebase conflicts or max retries exceeded, return `ErrMergeFailed`

**Error classification:**

| Error | Retryable | Handling |
|-------|-----------|----------|
| HTTP 405 + "Base branch was modified" | Yes | Retry with rebase |
| HTTP 422 (validation failed) | No | Block immediately |
| Rebase conflicts | No | Block with conflict details |
| Other errors | No | Block with error message |

### Error Handling

| Error | Behavior |
|-------|----------|
| CI timeout | Log warning, PR remains open for manual merge |
| CI failed | Log error with failed check names, PR remains open |
| Merge conflict (405 retryable) | Auto-retry with rebase up to 3 times |
| Merge conflict (after retries) | Task blocked with `blocked_reason=merge_failed` |
| Rebase conflicts | Task blocked, requires manual resolution |
| gh CLI error | Log error, PR remains open |

Merge failures now properly block task completion instead of incorrectly marking the task as completed. Users can resolve issues and run `orc resume TASK-XXX` to retry.

### WebSocket Events

During the CI wait and merge flow, progress is broadcast via WebSocket `transcript` events:

```json
{
  "type": "transcript",
  "task_id": "TASK-001",
  "phase": "ci_merge",
  "iteration": 0,
  "status": "progress",
  "content": "Waiting for CI... 3/5 passed, 2 pending"
}
```

---

## Activity Tracking and Progress Indication

Long-running Claude API calls now include activity tracking and progress indication to keep users informed.

### Activity States

| State | Description | Display |
|-------|-------------|---------|
| `idle` | No activity | - |
| `waiting_api` | Waiting for Claude API response | "Waiting for Claude API..." |
| `streaming` | Receiving streaming response | Progress dots |
| `running_tool` | Claude is executing a tool | "Running tool..." |
| `processing` | Processing response | - |

### Progress Indicators

During long API calls, orc provides visual feedback:

1. **Activity announcements**: State changes shown on new lines
2. **Heartbeat dots**: Periodic dots (default: every 30s) during API waits
3. **Elapsed time**: After 2 minutes, dots include elapsed time
4. **Idle warnings**: Alert if no activity for configured duration

Example output:
```
⏳ Waiting for Claude API...
.... (2m30s)
⚠️  No activity for 2m - API may be slow or stuck
```

### Timeouts

| Timeout | Default | Purpose |
|---------|---------|---------|
| `turn_max` | 10m | Max time for single API turn; cancels gracefully if exceeded |
| `idle_timeout` | 2m | Warn if no streaming activity |
| `phase_max` | 30m | Max time for entire phase |

When turn timeout is reached:
1. Request is cancelled gracefully
2. User sees "Turn timeout after X - cancelling request"
3. Task can be resumed with `orc resume`

### Configuration

```yaml
# config.yaml
timeouts:
  phase_max: 30m           # Max time per phase (0 = unlimited)
  turn_max: 10m            # Max time per API turn (0 = unlimited)
  idle_warning: 5m         # Warn if no tool calls
  heartbeat_interval: 30s  # Progress dots (0 = disable)
  idle_timeout: 2m         # Warn if no streaming activity
```

Environment variables:
- `ORC_PHASE_MAX_TIMEOUT`
- `ORC_TURN_MAX_TIMEOUT`
- `ORC_IDLE_WARNING`
- `ORC_HEARTBEAT_INTERVAL`
- `ORC_IDLE_TIMEOUT`

---

## Transcript Storage

```
.orc/tasks/TASK-001/transcripts/
├── 01-classify-001.md
├── 02-spec-001.md
├── 02-spec-002.md
├── 03-implement-001.md
├── 03-implement-002.md
├── 03-implement-003.md
└── 04-review-001.md
```

### Naming Format

```
PP-phasename-III.md
```

| Component | Format | Example |
|-----------|--------|---------|
| `PP` | Two-digit phase sequence number (01-99) | `03` |
| `phasename` | Phase name, lowercase | `implement` |
| `III` | Three-digit iteration number (001-999) | `007` |

Phase sequence numbers are assigned in execution order:
- 01: classify (if present)
- 02: research (if present)
- 03: spec (if present)
- etc.

### Transcript Contents

Each transcript contains:

| Section | Description |
|---------|-------------|
| Header | Timestamp, duration, tokens, status |
| Prompt | Full prompt sent to Claude |
| Response | Complete Claude response |
| Completion | Tests passing, lint status, phase complete flag |
| Files Changed | Table of modified files with line counts |
