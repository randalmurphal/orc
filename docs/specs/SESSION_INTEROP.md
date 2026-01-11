# Session Interoperability

**Status**: Planning
**Priority**: P0
**Last Updated**: 2026-01-10

---

## Problem Statement

Users need to seamlessly switch between:
- Web UI for monitoring and quick actions
- CLI for hands-on work
- Claude Code sessions for interactive debugging

Currently, starting a task in one interface locks you into that interface.

---

## Solution: Session-Aware Orchestration

Store Claude Code session IDs with task state, enabling:
1. Start in UI → Pause → Resume in CLI with full context
2. Start in CLI → Monitor in UI
3. Pause anywhere → Resume anywhere

---

## Claude Code Session Fundamentals

### Session ID

- Format: UUID v4 (e.g., `550e8400-e29b-41d4-a716-446655440000`)
- Generated when Claude Code starts a conversation
- Captured from init message in SDK output
- Stored locally per git repository

### Resume Mechanisms

```bash
# Resume by ID
claude --resume 550e8400-e29b-41d4-a716-446655440000

# Resume by name (if renamed)
claude --resume auth-fix-task

# Continue most recent
claude --continue
```

### What's Preserved on Resume

| Preserved | Not Preserved |
|-----------|---------------|
| Full message history | Unsaved file changes |
| File read context | Bash command effects (already executed) |
| Tool execution results | External state changes |
| Conversation understanding | |

---

## Task State Schema

```yaml
# .orc/tasks/TASK-001/state.yaml
task_id: TASK-001
current_phase: implement
status: running

# Session tracking
session:
  id: 550e8400-e29b-41d4-a716-446655440000
  started_at: 2026-01-10T14:30:00Z
  last_activity: 2026-01-10T14:45:00Z
  paused_at: null  # Set when paused
  iterations: 3

# Phase-specific sessions (if phases use different sessions)
phase_sessions:
  spec: abc12345-...
  implement: 550e8400-...

# For resume context
resume_context:
  last_prompt: "Continue implementing the auth timeout fix..."
  last_response_summary: "Modified authClient.go, running tests..."
  pending_action: null
```

---

## User Flows

### Flow 1: UI Start → CLI Resume

```
┌─────────────────────────────────────────────────────────────┐
│ 1. User creates task in Web UI                              │
│    POST /api/tasks { title: "Fix auth timeout" }            │
│                                                             │
│ 2. User clicks "Run" in UI                                  │
│    POST /api/tasks/TASK-001/run                             │
│    → Executor starts Claude Code                            │
│    → Captures session_id: 550e8400-...                      │
│    → Stores in state.yaml                                   │
│                                                             │
│ 3. Task executes, user monitors in UI                       │
│    → WebSocket streams transcript                           │
│    → Phase progress updates                                 │
│                                                             │
│ 4. User clicks "Pause" in UI                                │
│    POST /api/tasks/TASK-001/pause                           │
│    → Graceful stop of Claude Code                           │
│    → state.yaml updated with paused_at                      │
│    → Session preserved                                      │
│                                                             │
│ 5. UI shows resume command:                                 │
│    ┌───────────────────────────────────────────────────┐   │
│    │ Task paused. Resume options:                      │   │
│    │                                                   │   │
│    │ • Click "Resume" to continue in UI                │   │
│    │ • Or from terminal:                               │   │
│    │   orc resume TASK-001                             │   │
│    │   claude --resume 550e8400-e29b-41d4-a716-...     │   │
│    └───────────────────────────────────────────────────┘   │
│                                                             │
│ 6. User runs in terminal:                                   │
│    $ claude --resume 550e8400-e29b-41d4-a716-...            │
│    → Full context restored                                  │
│    → User interacts directly                                │
└─────────────────────────────────────────────────────────────┘
```

### Flow 2: CLI Start → UI Monitor

```bash
$ orc run TASK-001

Starting task TASK-001...
Session ID: 550e8400-e29b-41d4-a716-446655440000

Monitor in browser: http://localhost:8080/tasks/TASK-001
Press Ctrl+C to pause (session preserved)

⏳ implement [iteration 3/30]
> Modifying authClient.go...
```

User opens browser → sees live progress via WebSocket.

### Flow 3: Resume from Any Interface

```bash
# Check task status
$ orc show TASK-001
TASK-001 - Fix auth timeout bug
────────────────────────────────
Status:     paused
Phase:      implement (iteration 3)
Session:    550e8400-e29b-41d4-a716-...
Paused at:  2026-01-10 14:45:00

Resume options:
  orc resume TASK-001                    # Continue in orc
  claude --resume 550e8400-...           # Direct Claude session
  http://localhost:8080/tasks/TASK-001   # Resume in UI
```

---

## CLI Commands

### orc resume

```bash
# Resume task in orc (managed execution)
$ orc resume TASK-001

Resuming TASK-001 from implement phase...
Session: 550e8400-e29b-41d4-a716-...

⏳ implement [iteration 4/30]
> Continuing from previous context...
```

### orc session

```bash
# Show session info
$ orc session TASK-001
Task:     TASK-001
Session:  550e8400-e29b-41d4-a716-446655440000
Phase:    implement
Status:   paused

Resume with:
  claude --resume 550e8400-e29b-41d4-a716-446655440000

# Copy session ID to clipboard
$ orc session TASK-001 --copy
Session ID copied to clipboard.

# Open Claude Code directly
$ orc session TASK-001 --open
Opening Claude Code with session...
```

### orc attach

```bash
# Attach to running task's Claude session (for intervention)
$ orc attach TASK-001

⚠️  Attaching to running session. Orc execution will pause.

# User is now in interactive Claude session
# On exit, orc can optionally resume managed execution
```

---

## API Endpoints

### Get Session Info

```
GET /api/tasks/:id/session

Response:
{
  "task_id": "TASK-001",
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "paused",
  "phase": "implement",
  "paused_at": "2026-01-10T14:45:00Z",
  "resume_commands": {
    "orc": "orc resume TASK-001",
    "claude": "claude --resume 550e8400-e29b-41d4-a716-446655440000"
  }
}
```

### Pause with Session Preservation

```
POST /api/tasks/:id/pause

Response:
{
  "status": "paused",
  "session_id": "550e8400-...",
  "paused_at": "2026-01-10T14:45:00Z",
  "can_resume": true
}
```

---

## Web UI Components

### Paused Task Card

```
┌─────────────────────────────────────────────────────────────┐
│ ⏸ TASK-001                                          [small] │
│   Fix auth timeout bug                                      │
│                                                             │
│   ○ spec ─── ● implement ─── ○ test                         │
│              ↑ paused                                       │
│                                                             │
│   Paused 5 minutes ago                                      │
│                                                             │
│   [▶ Resume]  [📋 Copy Session ID]  [🔗 Open in Claude]     │
└─────────────────────────────────────────────────────────────┘
```

### Resume Modal

```
┌─ Resume Task ───────────────────────────────────────────────┐
│                                                             │
│ TASK-001 - Fix auth timeout bug                             │
│ Paused at: implement phase, iteration 3                     │
│                                                             │
│ Resume options:                                             │
│                                                             │
│ [▶ Resume in UI]                                            │
│   Continue managed execution with live monitoring           │
│                                                             │
│ [📋 Copy CLI Command]                                       │
│   orc resume TASK-001                                       │
│                                                             │
│ [🔗 Copy Claude Session ID]                                 │
│   550e8400-e29b-41d4-a716-446655440000                      │
│   For: claude --resume <id>                                 │
│                                                             │
│                                             [Cancel]        │
└─────────────────────────────────────────────────────────────┘
```

---

## Executor Integration

### Capturing Session ID

```go
func (e *Executor) executePhase(ctx context.Context, task *Task, phase *Phase) error {
    // Start Claude Code and capture session ID
    sessionID := ""

    for msg := range e.claudeSession.Run(ctx, prompt) {
        // Capture session ID from init message
        if msg.Type == "system" && msg.Subtype == "init" {
            sessionID = msg.SessionID

            // Save immediately
            e.state.Session.ID = sessionID
            e.state.Session.StartedAt = time.Now()
            e.state.Save()
        }

        // Process other messages...
    }

    return nil
}
```

### Graceful Pause

```go
func (e *Executor) pauseTask(taskID string) error {
    // Signal Claude Code to stop gracefully
    e.cancel()

    // Wait for current tool call to complete
    <-e.done

    // Update state
    e.state.Session.PausedAt = time.Now()
    e.state.Status = "paused"

    // Preserve resume context
    e.state.ResumeContext = &ResumeContext{
        LastPrompt:          e.lastPrompt,
        LastResponseSummary: summarize(e.lastResponse),
        Iteration:           e.iteration,
    }

    return e.state.Save()
}
```

### Resuming

```go
func (e *Executor) resumeTask(ctx context.Context, taskID string) error {
    state := e.loadState(taskID)

    // Build continuation prompt
    resumePrompt := fmt.Sprintf(`
Resuming task from where we paused.

Previous context:
%s

Continue from iteration %d of the %s phase.
`, state.ResumeContext.LastResponseSummary,
   state.ResumeContext.Iteration,
   state.CurrentPhase)

    // Resume with session ID
    for msg := range e.claudeSession.Resume(ctx, state.Session.ID, resumePrompt) {
        // Process messages...
    }

    return nil
}
```

---

## Edge Cases

### Session Expired or Lost

```bash
$ orc resume TASK-001

⚠️  Session 550e8400-... not found or expired.

Options:
  1. Start fresh from current phase
  2. Rewind to previous checkpoint
  3. Cancel

Choice [1]: 1

Starting fresh from implement phase...
```

### Concurrent Access

```
User A runs task in UI
User B tries to resume in CLI

$ orc resume TASK-001
❌ Task TASK-001 is currently running.

Options:
  • Wait for it to pause or complete
  • Force takeover: orc resume TASK-001 --force
    (This will interrupt the current execution)
```

### Worktree Sessions

Each worktree has its own session storage:

```
main repo/
├── .git/
└── .orc/tasks/TASK-001/state.yaml  # session_id: aaa...

.orc/worktrees/orc-task-001/
├── .git/  # Separate git dir
└── ...    # Session bbb... scoped to this worktree
```

---

## Implementation Checklist

- [ ] Capture session ID on Claude Code start
- [ ] Store session ID in state.yaml
- [ ] Graceful pause preserves session
- [ ] Resume constructs continuation prompt
- [ ] CLI commands: `orc session`, `orc attach`
- [ ] API endpoint: GET /api/tasks/:id/session
- [ ] Web UI shows session ID and resume options
- [ ] Handle expired/lost sessions gracefully
- [ ] Concurrent access detection
- [ ] Worktree session isolation

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for session-related code
- 100% coverage for session capture and state persistence

### Unit Tests

| Test | Description |
|------|-------------|
| `TestSessionIDCapture` | Verify session ID extracted from Claude init message |
| `TestGracefulPause` | state.yaml updated with paused_at on pause |
| `TestResumeContextSerialization` | ResumeContext marshals/unmarshals correctly |
| `TestConcurrentAccessDetection` | Detect when task already running |
| `TestWorktreeSessionIsolation` | Sessions scoped to worktree correctly |
| `TestSessionExpiry` | Handle expired/lost sessions gracefully |
| `TestContinuationPromptConstruction` | Resume prompt includes context correctly |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestPauseAPIPreservesSession` | `POST /api/tasks/:id/pause` saves session to state.yaml |
| `TestResumeAPIConstructsPrompt` | Resume builds correct continuation prompt |
| `TestGetSessionEndpoint` | `GET /api/tasks/:id/session` returns session info |
| `TestStateYAMLRoundTrip` | Session data survives save/load cycle |
| `TestWebSocketSessionEvents` | WebSocket publishes events on pause/resume |
| `TestCLISessionCommand` | `orc session TASK-ID` outputs correct info |
| `TestCLIResumeCommand` | `orc resume TASK-ID` starts with continuation |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_pause_shows_session_id` | `browser_click`, `browser_snapshot` | Pause task, verify session ID displayed |
| `test_copy_session_id` | `browser_click`, `browser_evaluate` | Copy to clipboard, verify content |
| `test_resume_continues_execution` | `browser_click`, `browser_wait_for` | Resume button continues task |
| `test_concurrent_access_warning` | `browser_snapshot` | Warning modal for running task |
| `test_session_info_in_paused_card` | `browser_navigate`, `browser_snapshot` | Paused task card shows session options |

### Test Fixtures
- Mock Claude init message with session ID
- Sample state.yaml with session data
- Mock WebSocket events for session state changes

---

## Success Criteria

- [ ] Can start task in UI, pause, resume in CLI with full context
- [ ] Can start task in CLI, monitor progress in UI
- [ ] Session ID is always available for paused tasks
- [ ] Resume in Claude Code directly works with orc session ID
- [ ] Graceful handling of lost sessions
- [ ] No context loss on interface switch
- [ ] 80%+ test coverage on session code
- [ ] All E2E tests pass
