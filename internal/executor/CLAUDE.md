# Executor Package

Phase execution engine implementing Ralph-style iteration loops with multiple executor strategies.

## File Structure

### Core Files

| File | Purpose | Lines |
|------|---------|-------|
| `executor.go` | Main Executor orchestrator, task lifecycle | ~320 |
| `task_execution.go` | ExecuteTask, ResumeFromPhase, gate evaluation | ~400 |
| `phase.go` | ExecutePhase, session/flowgraph dispatch | ~150 |

### Executor Types

| File | Strategy | Use Case |
|------|----------|----------|
| `trivial.go` | Fire-and-forget, no session | Quick single-prompt tasks |
| `standard.go` | Session per phase, iteration loop | Small/medium tasks |
| `full.go` | Persistent session, per-iteration checkpointing | Large/greenfield tasks |

### Support Modules

| File | Responsibility |
|------|----------------|
| `publish.go` | EventPublisher with nil-safety |
| `template.go` | Prompt template rendering |
| `retry.go` | Cross-phase retry context management |
| `worktree.go` | Git worktree setup/cleanup |
| `flowgraph_nodes.go` | Flowgraph node builders (uses worktree git for commits) |
| `session_adapter.go` | LLM session abstraction |
| `completion.go` | Phase completion detection |
| `config.go` | ExecutorConfig struct and defaults |
| `context.go` | Execution context management |
| `permissions.go` | Tool permission management |
| `recovery.go` | Error recovery strategies |
| `phase_executor.go` | PhaseExecutor interface |
| `pr.go` | PR creation and auto-merge |
| `test_parser.go` | Test output parsing (Go, Jest, Pytest) |
| `review.go` | Multi-round review session parsing |
| `qa.go` | QA session result parsing |
| `knowledge.go` | Post-phase knowledge extraction (fallback) |
| `activity.go` | Activity tracking for long-running API calls |

## Architecture

```
Executor
├── ExecuteTask()           # Main entry point
│   ├── setupWorktree()     # Isolate in git worktree
│   ├── loadPlan()          # Get phase sequence
│   └── for each phase:
│       ├── evaluateGate()  # Check gate conditions
│       ├── ExecutePhase()  # Run phase
│       │   ├── TrivialExecutor  # Weight: trivial
│       │   ├── StandardExecutor # Weight: small/medium
│       │   └── FullExecutor     # Weight: large/greenfield
│       └── checkpoint()    # Git commit
└── cleanup()               # Remove worktree if configured
```

## Executor Strategies

### TrivialExecutor
- **Session**: None (stateless completions)
- **Checkpointing**: None
- **Max iterations**: 5
- **Best for**: Single-prompt tasks, quick fixes

### StandardExecutor
- **Session**: Per-phase (maintains context within phase)
- **Checkpointing**: On phase completion
- **Max iterations**: 20
- **Best for**: Small/medium tasks

### FullExecutor
- **Session**: Persistent, resumable
- **Checkpointing**: Every iteration
- **Max iterations**: 30-50
- **Best for**: Large/greenfield, crash recovery needed

## Key Components

### EventPublisher (publish.go)

Nil-safe event publishing:
```go
publisher := NewEventPublisher(nil)  // Safe with nil
publisher.PhaseStart(taskID, phaseID)
publisher.PhaseComplete(taskID, phaseID, result)
publisher.Transcript(taskID, phaseID, iteration, "response", content)
```

### Template Rendering (template.go)

Variable substitution in prompts:
```go
vars := BuildTemplateVars(task, phase, iteration, retryContext)
vars = vars.WithUITestingContext(uiCtx)  // Add UI testing context if needed
rendered := RenderTemplate(template, vars)
```

**Standard Variables:** `{{TASK_ID}}`, `{{TASK_TITLE}}`, `{{TASK_DESCRIPTION}}`, `{{PHASE}}`, `{{WEIGHT}}`, `{{ITERATION}}`, `{{RETRY_CONTEXT}}`

**UI Testing Variables (when `requires_ui_testing: true`):**
- `{{REQUIRES_UI_TESTING}}` - Boolean flag indicating UI testing is needed
- `{{SCREENSHOT_DIR}}` - Path to save screenshots (`.orc/tasks/{id}/test-results/screenshots/`)
- `{{TEST_RESULTS}}` - Previous test results (for validate phase)

**UITestingContext:**
```go
type UITestingContext struct {
    RequiresUITesting bool
    ScreenshotDir     string
    TestResults       string
}
```

### Retry Context (retry.go)

Cross-phase retry when tests fail:
```go
ctx := buildRetryContext(failedPhase, output, attempt)
saveRetryContextFile(taskDir, ctx)
// On retry, phase receives {{RETRY_CONTEXT}} with failure info
```

### Fresh Session Retry

Retries use fresh Claude sessions with comprehensive context injection:

```go
type RetryContext struct {
    FailedPhase     string          // Which phase failed
    FailureReason   string          // Why it failed
    FailureOutput   string          // Last 1000 chars of output
    ReviewComments  []ReviewComment // Comments from code review UI
    PRComments      []PRComment     // Comments from GitHub PR
    Instructions    string          // User-provided guidance
    PreviousContext string          // Summary from previous session
}

func BuildRetryContextForFreshSession(opts RetryOptions) string
```

Context injection includes:
- **Failure output**: Last 1500 chars of what went wrong
- **Review comments**: Grouped by file with line numbers and severity
- **PR comments**: GitHub PR review feedback
- **User instructions**: Additional guidance from retry UI

### Worktree Isolation (worktree.go)

Tasks run in isolated git worktrees:
```go
worktreePath, cleanup, err := setupWorktree(gitSvc, taskID, config)
defer cleanup()
```

### Setup Failure Handling (task_execution.go)

When setup fails (e.g., worktree creation), `failSetup()` ensures proper error handling:
- Sets task status to `failed`
- Stores error in `state.yaml`
- Publishes error event (phase: "setup")
- Displays error to user (always shown, even in quiet mode)

This ensures setup errors are never silently swallowed.

## Completion Detection

Phases signal completion via XML tags:
```xml
<phase_complete>true</phase_complete>
```

Or blocking:
```xml
<phase_blocked>reason: missing dependencies</phase_blocked>
```

Parsed by `CheckPhaseCompletion()` in `completion.go`.

## Test Output Parsing (test_parser.go)

Auto-detects and parses test output from multiple frameworks:

```go
result, err := ParseTestOutput(output)  // Auto-detect framework
result, err := ParseGoTestOutput(output) // Go-specific
result, err := ParseJestOutput(output)   // Jest-specific
result, err := ParsePytestOutput(output) // Pytest-specific
```

Returns `ParsedTestResult`:
- `Passed`, `Failed`, `Skipped` counts
- `Coverage` percentage
- `Failures` with file:line references
- `Duration` of test run
- `Framework` detected

Coverage validation:
```go
valid := ValidateTestResults(result, thresholdPercent, required)
pass, reason := CheckCoverageThreshold(75.5, 80)
```

Retry context generation:
```go
context := BuildTestRetryContext("test", result)
context := BuildCoverageRetryContext(65.0, 80, result)
```

## Review Session (review.go)

Multi-round code review with structured output parsing.

**Types:**
- `ReviewFindings` - Round 1 exploratory findings (issues, questions, positives)
- `ReviewDecision` - Round 2 validation decision (pass/fail/needs_user_input)
- `ReviewResult` - Complete multi-round review result

**Parsing:**
```go
findings, err := ParseReviewFindings(response)  // Round 1
decision, err := ParseReviewDecision(response)  // Round 2

// Check for high-severity issues
if findings.HasHighSeverityIssues() { ... }

// Count by severity
counts := findings.CountBySeverity()  // map[string]int

// Format for Round 2 injection
formatted := FormatFindingsForRound2(findings)
```

**Configuration helpers:**
```go
shouldRun := ShouldRunReview(cfg, weight)
rounds := GetReviewRounds(cfg)
```

## QA Session (qa.go)

QA session result parsing for tests, coverage, and documentation.

**Types:**
- `QAResult` - Complete QA session result
- `QATest` - Test written (file, description, type)
- `QATestRun` - Test execution counts (total, passed, failed, skipped)
- `QACoverage` - Coverage percentage and uncovered areas
- `QADoc` - Documentation created (file, type)
- `QAIssue` - Issue found (severity, description, reproduction)

**Parsing:**
```go
result, err := ParseQAResult(response)

// Check results
if result.HasHighSeverityIssues() { ... }
if result.AllTestsPassed() { ... }

// Format for display
summary := FormatQAResultSummary(result)
```

**Configuration helpers:**
```go
shouldRun := ShouldRunQA(cfg, weight)
```

## Session Adapter (session_adapter.go)

LLM session abstraction with token tracking.

**Types:**
- `SessionAdapter` - Wraps Claude session for headless execution
- `TurnResult` - Single turn result with content and completion flags
- `TokenUsage` - Token counts from Claude response

**Token Usage:**

Claude reports tokens in multiple fields that must be combined to get the actual context size:

```go
type TokenUsage struct {
    InputTokens              int  // Raw input tokens (uncached portion)
    OutputTokens             int  // Output tokens generated
    CacheCreationInputTokens int  // Tokens written to cache
    CacheReadInputTokens     int  // Tokens read from cache
}

// EffectiveInputTokens returns actual context size (input + cached)
func (u TokenUsage) EffectiveInputTokens() int {
    return u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

// EffectiveTotalTokens returns total tokens including cached inputs
func (u TokenUsage) EffectiveTotalTokens() int {
    return u.EffectiveInputTokens() + u.OutputTokens
}
```

**Important:** Raw `InputTokens` can appear misleadingly low (e.g., 56 tokens) when most of the prompt is served from cache. Always use `EffectiveInputTokens()` when displaying or tracking context size.

**UI Display:** The web UI shows cached tokens in multiple locations:
- Dashboard stats: Total with cached count in parentheses
- Task detail: Input/Output/Cached/Total breakdown
- Transcript: Per-iteration with tooltip breakdown

See `web/CLAUDE.md` for component details.

**Turn Execution:**
```go
adapter := NewSessionAdapter(client, opts)
result, err := adapter.ExecuteTurn(ctx, prompt)

// Use effective tokens for accurate tracking
effectiveInput := result.Usage.EffectiveInputTokens()
totalTokens := result.Usage.EffectiveTotalTokens()
```

## Activity Tracking (activity.go)

Tracks execution state and provides progress indication for long-running API calls.

**Activity States:**
- `ActivityIdle` - No activity
- `ActivityWaitingAPI` - Waiting for Claude API response
- `ActivityStreaming` - Receiving streaming response
- `ActivityRunningTool` - Claude is running a tool
- `ActivityProcessing` - Processing response

**ActivityTracker:**
```go
tracker := NewActivityTracker(
    WithHeartbeatInterval(30 * time.Second),
    WithIdleTimeout(2 * time.Minute),
    WithTurnTimeout(10 * time.Minute),
    WithStateChangeCallback(func(state ActivityState) { ... }),
    WithHeartbeatCallback(func() { fmt.Print(".") }),
    WithIdleWarningCallback(func(d time.Duration) { ... }),
    WithTurnTimeoutCallback(func() { ... }),
)

tracker.Start(ctx)
defer tracker.Stop()

// During execution
tracker.SetState(ActivityWaitingAPI)
tracker.RecordChunk()  // On streaming chunk
tracker.SetIteration(5)

// Query state
duration := tracker.TurnDuration()
idle := tracker.IdleDuration()
chunks := tracker.ChunksReceived()
```

**Used by:** StandardExecutor, FullExecutor for progress indication during API calls.

---

## Testing

```bash
# Run all executor tests
go test ./internal/executor/... -v

# Run specific module tests
go test ./internal/executor/... -run TestPublish -v
go test ./internal/executor/... -run TestTemplate -v
go test ./internal/executor/... -run TestRetry -v
go test ./internal/executor/... -run TestWorktree -v
```

Test coverage for each module:
- `publish_test.go` - Nil safety, event types
- `template_test.go` - Variable substitution
- `retry_test.go` - Context file I/O
- `worktree_test.go` - Setup/cleanup
- `flowgraph_nodes_test.go` - Node builders
- `executor_test.go` - Integration tests
- `test_parser_test.go` - Framework detection, parsing, coverage validation
- `review_test.go` - Review findings/decision parsing, edge cases
- `qa_test.go` - QA result parsing, status validation
- `session_adapter_test.go` - Token usage calculations, effective token methods
- `activity_test.go` - Activity state transitions, heartbeat timing, timeout callbacks
