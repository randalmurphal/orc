# Error Message Standards

**Status**: Planning
**Priority**: P0
**Last Updated**: 2026-01-10

---

## Problem Statement

Current error messages are inconsistent:
- Some errors just say "failed" without context
- No guidance on how to fix issues
- Technical errors exposed to users without explanation
- No distinction between user errors and system errors

---

## Solution: Structured Error Messages

Every error message must include:
1. **What** - Clear description of what went wrong
2. **Why** - Context about why it happened
3. **Fix** - Actionable steps to resolve

---

## Error Message Format

### CLI Errors

```
❌ {what}

{why}

{fix}
```

Example:
```
❌ Failed to create task: no orc configuration found

This directory hasn't been initialized with orc yet.

To fix:
  orc init                  # Initialize orc in this directory
  cd /path/to/project       # Or switch to an orc project
```

### API Errors

```json
{
  "error": {
    "code": "NOT_INITIALIZED",
    "message": "No orc configuration found",
    "context": "This directory hasn't been initialized with orc yet",
    "fix": [
      "Run 'orc init' to initialize",
      "Or switch to a directory with .orc/"
    ],
    "docs": "https://orc.dev/docs/getting-started"
  }
}
```

### Web UI Errors

```
┌─ Error ─────────────────────────────────────────────────────┐
│                                                             │
│  ❌ Task execution failed                                   │
│                                                             │
│  The implement phase failed after 3 retry attempts.        │
│  Tests are still failing with authentication errors.       │
│                                                             │
│  Suggested actions:                                         │
│  • View the transcript for details                          │
│  • Rewind to spec phase and clarify requirements            │
│  • Resume with additional context                           │
│                                                             │
│  [View Transcript]  [Rewind]  [Resume with Context]         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Error Categories

### User Errors (Fixable by User)

| Category | Example | Fix Guidance |
|----------|---------|--------------|
| Not initialized | No .orc/ found | Run `orc init` |
| Task not found | TASK-999 doesn't exist | Check ID with `orc list` |
| Invalid state | Can't run completed task | Use `orc rewind` first |
| Invalid input | Weight "huge" not valid | Show valid options |
| Permission | Can't write to .orc/ | Check file permissions |

### System Errors (Orc Bug or Environment)

| Category | Example | Fix Guidance |
|----------|---------|--------------|
| Claude unavailable | Can't connect to Claude | Check claude CLI, network |
| Timeout | Claude took too long | Retry or increase timeout |
| Parse error | Can't parse Claude output | Report bug |
| State corruption | Invalid state.yaml | Show recovery options |

### External Errors (Third-party Issues)

| Category | Example | Fix Guidance |
|----------|---------|--------------|
| Git error | Failed to create branch | Check git status |
| Test failure | Tests didn't pass | Fix code or adjust criteria |
| Build error | Compilation failed | Check build output |

---

## Error Codes

Each error has a unique code for documentation and debugging:

```go
const (
    // Initialization
    ErrNotInitialized     = "NOT_INITIALIZED"
    ErrAlreadyInitialized = "ALREADY_INITIALIZED"

    // Tasks
    ErrTaskNotFound       = "TASK_NOT_FOUND"
    ErrTaskInvalidState   = "TASK_INVALID_STATE"
    ErrTaskRunning        = "TASK_RUNNING"

    // Execution
    ErrClaudeUnavailable  = "CLAUDE_UNAVAILABLE"
    ErrClaudeTimeout      = "CLAUDE_TIMEOUT"
    ErrPhaseStuck         = "PHASE_STUCK"
    ErrMaxRetries         = "MAX_RETRIES"

    // Configuration
    ErrConfigInvalid      = "CONFIG_INVALID"
    ErrConfigMissing      = "CONFIG_MISSING"

    // Git
    ErrGitDirty           = "GIT_DIRTY"
    ErrGitBranchExists    = "GIT_BRANCH_EXISTS"
)
```

---

## Implementation

### Error Type

```go
type OrcError struct {
    Code    string   // Machine-readable code
    What    string   // What went wrong
    Why     string   // Context/explanation
    Fix     []string // Actionable steps
    DocsURL string   // Link to documentation
    Cause   error    // Underlying error
}

func (e *OrcError) Error() string {
    return e.What
}

func (e *OrcError) UserMessage() string {
    var sb strings.Builder

    sb.WriteString(fmt.Sprintf("❌ %s\n\n", e.What))

    if e.Why != "" {
        sb.WriteString(fmt.Sprintf("%s\n\n", e.Why))
    }

    if len(e.Fix) > 0 {
        sb.WriteString("To fix:\n")
        for _, fix := range e.Fix {
            sb.WriteString(fmt.Sprintf("  %s\n", fix))
        }
    }

    return sb.String()
}
```

### Error Constructors

```go
func ErrNotInitializedError() *OrcError {
    return &OrcError{
        Code: ErrNotInitialized,
        What: "No orc configuration found",
        Why:  "This directory hasn't been initialized with orc yet.",
        Fix: []string{
            "orc init                  # Initialize orc in this directory",
            "cd /path/to/project       # Or switch to an orc project",
        },
        DocsURL: "https://orc.dev/docs/getting-started",
    }
}

func ErrTaskNotFoundError(taskID string) *OrcError {
    return &OrcError{
        Code: ErrTaskNotFound,
        What: fmt.Sprintf("Task %s not found", taskID),
        Why:  "No task with this ID exists in the current project.",
        Fix: []string{
            "orc list                  # See all tasks",
            "orc new \"title\"          # Create a new task",
        },
    }
}

func ErrClaudeTimeoutError(phase string, duration time.Duration) *OrcError {
    return &OrcError{
        Code: ErrClaudeTimeout,
        What: fmt.Sprintf("Claude timed out during %s phase", phase),
        Why:  fmt.Sprintf("No response received within %s.", duration),
        Fix: []string{
            "orc resume TASK-ID         # Retry from where it stopped",
            "orc config timeout 20m     # Increase timeout limit",
            "Check Claude service status if the issue persists",
        },
    }
}
```

### CLI Error Printing

```go
func printError(err error) {
    if orcErr, ok := err.(*OrcError); ok {
        fmt.Fprintln(os.Stderr, orcErr.UserMessage())

        // Show docs link if available
        if orcErr.DocsURL != "" {
            fmt.Fprintf(os.Stderr, "\nDocumentation: %s\n", orcErr.DocsURL)
        }
    } else {
        // Unknown error - wrap it
        fmt.Fprintf(os.Stderr, "❌ %s\n\nPlease report this issue.\n", err)
    }
}
```

---

## Common Error Messages

### Initialization

```
❌ No orc configuration found

This directory hasn't been initialized with orc yet.

To fix:
  orc init                  # Initialize orc in this directory
  cd /path/to/project       # Or switch to an orc project
```

```
❌ orc is already initialized

A .orc/ directory already exists in this project.

To fix:
  orc init --force          # Overwrite existing configuration
  rm -rf .orc/              # Remove and start fresh
```

### Task Operations

```
❌ Task TASK-005 not found

No task with this ID exists in the current project.

To fix:
  orc list                  # See all tasks
  orc new "title"           # Create a new task
```

```
❌ Cannot run task TASK-003: task is already running

This task is currently being executed.

To fix:
  orc status                # Check what's running
  orc pause TASK-003        # Pause to make changes
  Wait for completion       # Let it finish
```

```
❌ Cannot resume task TASK-002: task is not paused

Only paused or blocked tasks can be resumed.
Current status: completed

To fix:
  orc rewind TASK-002 --to implement    # Go back to a phase
  orc new "new task"                    # Create a new task
```

### Execution Errors

```
❌ Claude timed out during implement phase

No response received within 10 minutes.

To fix:
  orc resume TASK-001       # Retry from where it stopped
  orc config timeout 20m    # Increase timeout limit
  Check Claude service status if the issue persists
```

```
❌ Task stuck: same error 3 times in a row

The implement phase is failing with the same error repeatedly.
Error: cannot find package "github.com/example/missing"

To fix:
  orc log TASK-001          # View full transcript
  orc rewind TASK-001 --to spec    # Go back and clarify
  Fix the issue manually and resume
```

```
❌ Maximum retries exceeded for test phase

Tests failed 3 times after retrying from implement.

To fix:
  orc log TASK-001 --phase test     # See what's failing
  orc attach TASK-001               # Debug interactively
  orc rewind TASK-001 --to spec     # Reconsider approach
```

### Git Errors

```
❌ Cannot create task: uncommitted changes

Git has uncommitted changes that could conflict with task execution.

To fix:
  git stash                 # Stash changes temporarily
  git commit -am "WIP"      # Commit current work
  git status                # See what's pending
```

```
❌ Branch orc/TASK-001 already exists

A branch for this task already exists.

To fix:
  git branch -D orc/TASK-001    # Delete the branch
  orc cleanup                   # Clean up old task branches
```

### Configuration Errors

```
❌ Invalid configuration: unknown profile "turbo"

The profile "turbo" is not recognized.

Valid profiles:
  auto    - Fully automated
  fast    - Speed over safety
  safe    - AI reviews, human merges
  strict  - Human gates on key decisions

To fix:
  orc config profile auto   # Set a valid profile
  Edit .orc/config.yaml     # Fix manually
```

---

## Validation Rules

### Implementing New Errors

Before adding a new error message, verify:

- [ ] **What** is specific (not "failed" or "error occurred")
- [ ] **Why** explains the context a user needs
- [ ] **Fix** contains at least one actionable command
- [ ] No jargon without explanation
- [ ] Error code is unique and documented

### Error Message Review Checklist

- [ ] Can a new user understand this?
- [ ] Is at least one fix actionable right now?
- [ ] Does it avoid blame ("you did X wrong")?
- [ ] Is technical info (stack traces) hidden unless --debug?
- [ ] Does it link to docs for complex issues?

---

## Logging Levels

| Level | When to Use | User Visibility |
|-------|-------------|-----------------|
| ERROR | User-facing failures | Always shown |
| WARN | Recoverable issues | Shown with --verbose |
| INFO | Progress updates | Shown normally |
| DEBUG | Internal details | Only with --debug |

```go
// ERROR - Always visible, needs action
log.Error("Task execution failed", "task", taskID, "phase", phase, "error", err)

// WARN - Something's off but we're continuing
log.Warn("Retry limit approaching", "task", taskID, "attempt", 2, "max", 3)

// INFO - Progress updates
log.Info("Phase completed", "task", taskID, "phase", "implement")

// DEBUG - Internals for troubleshooting
log.Debug("Claude response received", "tokens", 1234, "duration", "5.2s")
```

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for `internal/errors/`
- 100% coverage for error constructor functions

### Unit Tests

| Test | Description |
|------|-------------|
| `TestOrcErrorFormat` | Verify `UserMessage()` output format matches spec |
| `TestOrcErrorJSON` | API JSON serialization includes all fields |
| `TestErrNotInitializedError` | Constructor produces correct code/what/why/fix |
| `TestErrTaskNotFoundError` | ID interpolation works correctly |
| `TestErrClaudeTimeoutError` | Duration formatting correct |
| `TestErrorCodeUniqueness` | No duplicate error codes in constants |
| `TestErrorMessageValidation` | All errors have What/Why/Fix populated |
| `TestErrorUnwrap` | Cause field properly unwraps |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestCLIPrintsUserFriendlyError` | CLI uses `printError()` for each error code |
| `TestAPIReturnsCorrectHTTPStatus` | 404 for NotFound, 400 for Invalid, 500 for System |
| `TestDebugFlagShowsStackTrace` | `--debug` includes stack trace in output |
| `TestVerboseFlagShowsWarnings` | `--verbose` includes warning-level messages |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_error_card_appears` | `browser_navigate`, `browser_snapshot` | Trigger error in UI, verify error card appears |
| `test_error_card_buttons` | `browser_click` | Click "View Transcript", "Rewind" buttons work |
| `test_error_toast_transient` | `browser_wait_for` | Toast error disappears after timeout |
| `test_error_modal_blocking` | `browser_snapshot` | Modal error blocks interaction |

### Test Fixtures
- Mock `OrcError` instances for each error code
- Test cases with nil Cause, empty Fix arrays

### Compliance Tests
- Every `Err*` constant has a constructor function
- Every constructor produces valid What/Why/Fix
- No error message contains only "failed" without context

---

## Success Criteria

- [ ] Every error has code, what, why, and fix
- [ ] No error just says "failed" without context
- [ ] At least one fix is an actionable command
- [ ] Stack traces hidden unless --debug
- [ ] API errors follow JSON schema
- [ ] Web UI shows friendly error cards with actions
- [ ] Error codes are documented
- [ ] New errors pass review checklist
- [ ] 80%+ test coverage on error package
- [ ] All E2E tests pass
