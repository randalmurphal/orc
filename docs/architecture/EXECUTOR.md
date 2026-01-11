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
| **Trivial Executor** | `trivial.go` | Stateless, fire-and-forget execution |
| **Standard Executor** | `standard.go` | Session per phase, iteration loop |
| **Full Executor** | `full.go` | Persistent sessions, per-iteration checkpointing |
| **Publishing** | `publish.go` | Nil-safe EventPublisher |
| **Templates** | `template.go` | Prompt variable substitution |
| **Retry** | `retry.go` | Cross-phase retry context |
| **Worktree** | `worktree.go` | Git worktree isolation |
| **Flowgraph** | `flowgraph_nodes.go` | Flowgraph node builders |
| **Completion** | `completion.go` | Phase completion detection |

---

## Executor Strategies

Three executor types scale to task weight:

| Executor | Session | Checkpointing | Max Iterations | Best For |
|----------|---------|---------------|----------------|----------|
| **Trivial** | None | None | 5 | Quick single-prompt tasks |
| **Standard** | Per-phase | On completion | 20 | Small/medium tasks |
| **Full** | Persistent | Every iteration | 30-50 | Large/greenfield |

---

## Execution Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         EXECUTOR                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐        │
│  │   Template  │───►│   Claude    │───►│   Output    │        │
│  │  Rendering  │    │   Session   │    │   Parser    │        │
│  └─────────────┘    └─────────────┘    └─────────────┘        │
│        ▲                                      │                │
│        │                                      ▼                │
│        │                              ┌─────────────┐          │
│        │                              │ Completion  │          │
│        │                              │  Detector   │          │
│        │                              └──────┬──────┘          │
│        │                                     │                 │
│        │         ┌───────────────────────────┤                 │
│        │         │                           │                 │
│        │         ▼                           ▼                 │
│  ┌─────────────────────┐           ┌─────────────┐            │
│  │  NOT COMPLETE       │           │  COMPLETE   │            │
│  │  (loop continues)   │           │ (checkpoint)│            │
│  └─────────────────────┘           └─────────────┘            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
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

### XML Tag Pattern

Claude outputs completion signals as XML tags:

```markdown
I've completed the implementation. All tests pass.

<phase_complete>true</phase_complete>
```

### Parsing Logic

```go
var completionPattern = regexp.MustCompile(`<phase_complete>(\w+)</phase_complete>`)

func DetectCompletion(output string) bool {
    matches := completionPattern.FindStringSubmatch(output)
    if len(matches) > 1 {
        return matches[1] == "true"
    }
    return false
}
```

### Additional Criteria

| Criterion | Check Method |
|-----------|--------------|
| `all_tests_pass` | Run `go test ./...`, check exit code |
| `no_lint_errors` | Run linter, check exit code |
| `files_exist` | Check filesystem |
| `coverage_above: N` | Parse coverage report, verify >= N% |
| `claude_confirms` | Claude outputs `<phase_complete>true</phase_complete>` |
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
