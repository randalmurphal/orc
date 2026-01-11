# TUI Watch Mode

**Status**: Planning
**Priority**: P2
**Last Updated**: 2026-01-10

---

## Problem Statement

Terminal users need rich task monitoring without leaving the CLI:
- Current CLI output is linear (logs stream by)
- No interactive control
- Can't see multiple tasks at once
- No way to navigate task history

---

## Solution: Lazygit-Style TUI

Build an interactive terminal UI using bubbletea:
- Real-time task monitoring
- Vim-style navigation
- Multiple panels/views
- Quick actions via keybindings

---

## Design Inspiration

### Lazygit Patterns

| Pattern | Application to Orc |
|---------|-------------------|
| Panel switching | Tab between tasks, transcript, details |
| Vim navigation | j/k to move, Enter to select |
| Context actions | Keybindings change per panel |
| Status line | Show current task state |
| Popup modals | Confirm dangerous actions |

---

## Screen Layouts

### Main View (Task List)

```
┌─ orc watch ─────────────────────────────────────── [?] help ┐
│                                                             │
│ Tasks                                             [n] new   │
│ ─────────────────────────────────────────────────────────── │
│ ⏳ TASK-007 Implement caching layer         [large] 3/5    │
│ ⏳ TASK-008 Fix login redirect bug          [small] 1/2    │
│ 🚫 TASK-006 Add dark mode toggle           [medium] blocked│
│ ✅ TASK-005 Update API documentation        [small] done   │
│ ✅ TASK-004 Refactor auth middleware        [medium] done  │
│ ❌ TASK-003 Add rate limiting               [large] failed │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ TASK-007 - Implement caching layer                          │
│ Phase: implement (iteration 3/5)                            │
│ Tokens: 45.2K | Cost: $1.05 | Duration: 15m                 │
│ ─────────────────────────────────────────────────────────── │
│ ○ spec ─── ● implement ─── ○ test ─── ○ validate            │
│                                                             │
│ Recent output:                                              │
│ > Implementing Redis cache wrapper...                       │
│ > Added cache invalidation logic                            │
│ > Running tests to verify...                                │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ [r]un [p]ause [c]ancel [v]iew [t]ranscript    [q]uit        │
└─────────────────────────────────────────────────────────────┘
```

### Transcript View

```
┌─ TASK-007 Transcript ────────────────────────── [Esc] back ─┐
│                                                             │
│ Phase: implement | Iteration: 3                             │
│ ─────────────────────────────────────────────────────────── │
│                                                             │
│ ▶ PROMPT (14:30:05)                                         │
│ │ Continue implementing the caching layer. Focus on:        │
│ │ 1. Redis connection pooling                               │
│ │ 2. Cache key generation                                   │
│ │ ...                                                       │
│                                                             │
│ ◀ RESPONSE (14:30:45)                                       │
│ │ I'll implement the Redis connection pooling first.        │
│ │ Let me check the existing connection code...              │
│ │                                                           │
│ │ Reading internal/cache/redis.go...                        │
│ │ ...                                                       │
│                                                             │
│ ⚡ TOOL: Read (14:30:46)                                     │
│ │ File: internal/cache/redis.go                             │
│ │ Lines: 1-50                                               │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ j/k scroll  [p]hase select  [f]ollow  [/]search    [Esc]back│
└─────────────────────────────────────────────────────────────┘
```

### Task Detail View

```
┌─ TASK-007 Detail ────────────────────────────── [Esc] back ─┐
│                                                             │
│ Title:   Implement caching layer                            │
│ Weight:  large                                              │
│ Status:  running                                            │
│ Branch:  orc/TASK-007                                       │
│                                                             │
│ Created:  2026-01-10 14:15:00                               │
│ Started:  2026-01-10 14:15:05                               │
│ Duration: 15m 23s                                           │
│                                                             │
│ Tokens:  45,234 input / 12,456 output                       │
│ Cost:    $1.05 estimated                                    │
│                                                             │
│ ─────────────────────────────────────────────────────────── │
│ Timeline                                                    │
│ ─────────────────────────────────────────────────────────── │
│ ● spec      │ 2 iterations │ 12.5K tokens │ $0.30 │ 4m     │
│ ● implement │ 3 iterations │ 32.7K tokens │ $0.75 │ 11m    │
│ ○ test      │ pending      │              │       │        │
│ ○ validate  │ pending      │              │       │        │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ [r]un [p]ause [w]rewind [t]ranscript           [Esc]back    │
└─────────────────────────────────────────────────────────────┘
```

### Help View

```
┌─ Keyboard Shortcuts ─────────────────────────── [Esc] close ┐
│                                                             │
│ Navigation                                                  │
│ ─────────────────────────────────────────────────────────── │
│ j / ↓       Move down                                       │
│ k / ↑       Move up                                         │
│ Enter       Select / Open                                   │
│ Esc         Back / Close                                    │
│ Tab         Switch panel                                    │
│                                                             │
│ Actions                                                     │
│ ─────────────────────────────────────────────────────────── │
│ n           New task                                        │
│ r           Run task                                        │
│ p           Pause task                                      │
│ c           Cancel task                                     │
│ d           Delete task                                     │
│                                                             │
│ Views                                                       │
│ ─────────────────────────────────────────────────────────── │
│ v           Task detail view                                │
│ t           Transcript view                                 │
│ l           Log view                                        │
│                                                             │
│ Other                                                       │
│ ─────────────────────────────────────────────────────────── │
│ ?           Show this help                                  │
│ q           Quit                                            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Implementation

### Tech Stack

- **bubbletea** - Terminal UI framework
- **lipgloss** - Styling
- **bubbles** - Reusable components (list, viewport, textinput)

### Main Model

```go
type Model struct {
    // Data
    tasks       []task.Task
    selectedIdx int
    transcript  []TranscriptLine

    // Views
    currentView View // list, detail, transcript

    // Components
    taskList    list.Model
    viewport    viewport.Model

    // State
    width       int
    height      int
    quitting    bool

    // Realtime
    eventCh     chan events.Event
}

type View int

const (
    ViewList View = iota
    ViewDetail
    ViewTranscript
    ViewHelp
)
```

### Message Types

```go
type (
    // Data updates
    TasksUpdatedMsg     []task.Task
    TranscriptLineMsg   TranscriptLine
    TaskStateChangedMsg task.Task

    // Actions
    RunTaskMsg    string // task ID
    PauseTaskMsg  string
    CancelTaskMsg string

    // Navigation
    SwitchViewMsg View

    // System
    TickMsg       time.Time
    WindowSizeMsg tea.WindowSizeMsg
)
```

### Update Function

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch m.currentView {
        case ViewList:
            return m.updateList(msg)
        case ViewDetail:
            return m.updateDetail(msg)
        case ViewTranscript:
            return m.updateTranscript(msg)
        case ViewHelp:
            if msg.String() == "esc" || msg.String() == "?" {
                m.currentView = ViewList
            }
        }

    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.taskList.SetSize(msg.Width, msg.Height-10)

    case TasksUpdatedMsg:
        m.tasks = msg
        m.updateTaskList()

    case TranscriptLineMsg:
        m.transcript = append(m.transcript, msg)
        m.viewport.SetContent(m.renderTranscript())
        if m.followMode {
            m.viewport.GotoBottom()
        }
    }

    return m, nil
}
```

### List Navigation

```go
func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "j", "down":
        if m.selectedIdx < len(m.tasks)-1 {
            m.selectedIdx++
        }

    case "k", "up":
        if m.selectedIdx > 0 {
            m.selectedIdx--
        }

    case "enter", "v":
        m.currentView = ViewDetail

    case "t":
        m.currentView = ViewTranscript
        return m, m.loadTranscript(m.selectedTask().ID)

    case "r":
        return m, m.runTask(m.selectedTask().ID)

    case "p":
        return m, m.pauseTask(m.selectedTask().ID)

    case "n":
        // Open new task input
        m.showNewTaskInput = true

    case "?":
        m.currentView = ViewHelp

    case "q":
        m.quitting = true
        return m, tea.Quit
    }

    return m, nil
}
```

### Styling

```go
var (
    titleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("12"))

    selectedStyle = lipgloss.NewStyle().
        Background(lipgloss.Color("8")).
        Foreground(lipgloss.Color("15"))

    statusRunning = lipgloss.NewStyle().
        Foreground(lipgloss.Color("12"))

    statusComplete = lipgloss.NewStyle().
        Foreground(lipgloss.Color("10"))

    statusFailed = lipgloss.NewStyle().
        Foreground(lipgloss.Color("9"))

    helpStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("8"))
)
```

---

## Real-time Updates

### WebSocket Connection

```go
func (m Model) connectWebSocket() tea.Cmd {
    return func() tea.Msg {
        conn, err := websocket.Dial(m.wsURL)
        if err != nil {
            return ErrorMsg{err}
        }

        go func() {
            for {
                var event events.Event
                if err := conn.ReadJSON(&event); err != nil {
                    return
                }
                m.eventCh <- event
            }
        }()

        return WSConnectedMsg{}
    }
}

func (m Model) listenForEvents() tea.Cmd {
    return func() tea.Msg {
        event := <-m.eventCh
        switch event.Type {
        case events.EventTranscript:
            return TranscriptLineMsg(event.Data.(TranscriptLine))
        case events.EventState:
            return TaskStateChangedMsg(event.Data.(task.Task))
        }
        return nil
    }
}
```

---

## CLI Command

```bash
# Start TUI
orc watch

# Watch specific task
orc watch TASK-001

# Start TUI with new task
orc watch --new "Fix the bug"
```

### Flags

| Flag | Description |
|------|-------------|
| `--task, -t` | Focus specific task |
| `--new, -n` | Create and watch new task |
| `--follow, -f` | Auto-follow transcript |

---

## Features

### Multi-Task View

Split screen showing multiple running tasks:

```
┌─ Running Tasks ─────────────────────────────────────────────┐
│                                                             │
│ ┌─ TASK-007 ──────────────────────────────────────────────┐│
│ │ implement 3/5                                           ││
│ │ > Implementing Redis wrapper...                         ││
│ │ > Added connection pooling...                           ││
│ └─────────────────────────────────────────────────────────┘│
│                                                             │
│ ┌─ TASK-008 ──────────────────────────────────────────────┐│
│ │ test 1/2                                                ││
│ │ > Running test suite...                                 ││
│ │ > 15/20 tests passed                                    ││
│ └─────────────────────────────────────────────────────────┘│
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Tab to switch focus  [1-9] focus task  [q] quit            │
└─────────────────────────────────────────────────────────────┘
```

### Search/Filter

Press `/` to open search:

```
┌─ orc watch ─────────────────────────────────────────────────┐
│                                                             │
│ Filter: auth_                                               │
│ ─────────────────────────────────────────────────────────── │
│ ⏳ TASK-007 Fix auth timeout                               │
│ ✅ TASK-004 Refactor auth middleware                       │
│                                                             │
│ 2 tasks matching "auth"                                     │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Enter to select  Esc to clear  Tab to cycle                 │
└─────────────────────────────────────────────────────────────┘
```

### Progress Indicators

```
Phase progress:  ████████░░░░░░░░░░░░ 40%
Token usage:     ████████████████░░░░ 80% of typical

Iteration 3 of ~5 (estimated)
```

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for TUI code
- 100% coverage for navigation logic

### Unit Tests

| Test | Description |
|------|-------------|
| `TestModelUpdate_KeyNavigation` | j/k moves selection, bounds checking |
| `TestModelUpdate_ViewSwitching` | Enter/v/t/Esc switch views correctly |
| `TestModelUpdate_WindowResize` | Components resize proportionally |
| `TestTaskListFiltering` | Search filters task list correctly |
| `TestTranscriptRendering` | Lines render with correct formatting |
| `TestProgressBarCalculation` | Phase progress percentages correct |
| `TestStatusIconMapping` | Task states map to correct icons |
| `TestStyleApplication` | lipgloss styles apply correctly |
| `TestWebSocketMessageParsing` | Event messages deserialize correctly |
| `TestFollowModeScrolling` | Auto-scroll when follow enabled |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestTUIWithMockAPI` | TUI loads tasks from mock server |
| `TestWebSocketReconnection` | TUI reconnects after disconnect |
| `TestTaskActionCommands` | Run/pause/cancel sends correct API calls |
| `TestRealTimeUpdates` | Transcript lines appear as events arrive |
| `TestMultiTaskView` | Multiple running tasks display correctly |

### CLI E2E Tests

| Test | Description |
|------|-------------|
| `test_orc_watch_launches` | `orc watch` launches without error |
| `test_navigation_keys` | j/k/Enter/Esc work in terminal |
| `test_run_task_flow` | Select task, press r, verify task starts |
| `test_pause_resume_flow` | Pause running task, resume works |
| `test_transcript_scroll` | Scroll transcript, verify content |
| `test_help_modal` | ? shows help, Esc closes |
| `test_terminal_resize` | Resize terminal, UI adapts |
| `test_graceful_quit` | q exits cleanly |
| `test_watch_specific_task` | `orc watch TASK-001` focuses task |

### Performance Tests

| Test | Description |
|------|-------------|
| `test_100_tasks_render` | Handles 100+ tasks without lag |
| `test_rapid_updates` | High-frequency updates don't crash |
| `test_large_transcript` | Long transcript scrolls smoothly |

### Accessibility Tests

| Test | Description |
|------|-------------|
| `test_color_independence` | Status visible without color |
| `test_focus_indicators` | Focus ring visible |

### Test Fixtures
- Mock task data for various states
- Mock WebSocket events
- Terminal size scenarios

---

## Success Criteria

- [ ] TUI launches with `orc watch`
- [ ] Vim-style j/k navigation works
- [ ] Task list shows all tasks with status
- [ ] Detail view shows task info
- [ ] Transcript view with scrolling
- [ ] Real-time updates via WebSocket
- [ ] Run/pause/cancel actions work
- [ ] Help modal shows all shortcuts
- [ ] Responsive to terminal resize
- [ ] Graceful exit with q
- [ ] 80%+ test coverage on TUI code
- [ ] All tests pass
