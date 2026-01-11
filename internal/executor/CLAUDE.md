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
| `flowgraph_nodes.go` | Flowgraph node builders |
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
rendered := RenderTemplate(template, vars)
```

Variables: `{{TASK_ID}}`, `{{TASK_TITLE}}`, `{{TASK_DESCRIPTION}}`, `{{PHASE}}`, `{{WEIGHT}}`, `{{ITERATION}}`, `{{RETRY_CONTEXT}}`

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
