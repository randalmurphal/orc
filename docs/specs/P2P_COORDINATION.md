# Peer-to-Peer Coordination Specification

> Multiple developers collaborating on the same project WITHOUT a shared server.

## Overview

P2P mode enables team collaboration using only:
1. **Git repository** (GitHub, GitLab, Bitbucket)
2. **Local orc instances** (one per developer)
3. **Shared directory structure** (`.orc/shared/`)

No central server, no database sync, no WebSocket infrastructure.

---

## Mode Detection and Guards

### Three Modes

| Mode | When | Features Active |
|------|------|-----------------|
| `solo` | Default, single user | No identity config needed, simple task IDs |
| `p2p` | `.orc/shared/` exists | Executor tags on branches, git-based visibility |
| `team` | `team.server_url` configured | Real-time visibility, server notifications |

### Solo Mode Guarantees

When `mode: solo` (the default), these are **guaranteed**:

- **No identity config required** - `identity.initials` not read or validated
- **No `.orc/shared/` checks** - directory not scanned
- **No team.yaml validation** - file not read
- **Simple task IDs** - `TASK-001`, `TASK-002` (no prefix)
- **Simple branch names** - `orc/TASK-001` (no executor tag)
- **No git sync prompts** - no team visibility features
- **Zero overhead** - same performance as pre-P2P orc

### Auto-Detection

```go
// internal/config/mode.go
func DetectMode(projectPath string) Mode {
    cfg := loadConfig(projectPath)

    // Check for team server first
    if cfg.Team.ServerURL != "" {
        return ModeTeam
    }

    // Check for shared directory
    sharedDir := filepath.Join(projectPath, ".orc", "shared")
    if _, err := os.Stat(sharedDir); err == nil {
        return ModeP2P
    }

    // Default: solo - no further config checks
    return ModeSolo
}
```

### Mode Guard in Executor

```go
// internal/executor/executor.go
func (e *Executor) Run(taskID string) error {
    mode := DetectMode(e.projectDir)

    switch mode {
    case ModeSolo:
        // GUARANTEED: No identity config read, no team features
        return e.runInWorktree(taskID, "", "orc/"+taskID)

    case ModeP2P, ModeTeam:
        // Only P2P/Team modes require identity
        tag := e.config.Identity.Initials
        if tag == "" {
            return fmt.Errorf("identity.initials required for %s mode\n"+
                "Run: orc config set identity.initials YOUR_INITIALS", mode)
        }

        // Check for existing work before creating redundant branch
        if warning := e.checkExistingBranches(taskID, tag); warning != "" {
            if !e.promptContinue(warning) {
                return nil // User cancelled
            }
        }

        branchName := BranchName(taskID, tag)

        if mode == ModeTeam {
            e.notifyServerStart(taskID) // Non-blocking, failures logged
            defer e.notifyServerStop(taskID)
        }

        return e.runInWorktree(taskID, tag, branchName)

    default:
        return fmt.Errorf("unknown mode: %s", mode)
    }
}

func (e *Executor) checkExistingBranches(taskID, myTag string) string {
    branches := e.git.FindBranches("orc/" + taskID + "-*")

    for _, b := range branches {
        if b.IsMerged {
            return fmt.Sprintf("Branch '%s' was merged to main %s ago.\n"+
                "You may be duplicating completed work.", b.Name, b.MergedAgo)
        }
        if b.Tag != myTag {
            return fmt.Sprintf("Branch '%s' exists (last commit: %s by %s).\n"+
                "Someone else is working on this task.", b.Name, b.LastCommit, b.Author)
        }
    }
    return ""
}

func (e *Executor) runInWorktree(taskID, tag, branchName string) error {
    worktreePath := WorktreePath(taskID, tag)

    // Handle existing worktree (crash recovery)
    if exists(worktreePath) {
        return e.handleExistingWorktree(taskID, worktreePath)
    }

    // Create new worktree
    if err := e.git.CreateWorktree(worktreePath, branchName); err != nil {
        return err
    }

    // PID guard: prevent same user running twice
    guard := &PIDGuard{worktreePath: worktreePath}
    guard.Acquire()
    defer guard.Release()

    return e.executeInDir(worktreePath, taskID)
}

func (e *Executor) handleExistingWorktree(taskID, worktreePath string) error {
    guard := &PIDGuard{worktreePath: worktreePath}

    if err := guard.Check(); err != nil {
        // Process is running - show detailed error
        return e.formatRunningError(taskID, worktreePath, guard.PID())
    }

    // Worktree exists but no process - show recovery options
    state := e.inspectWorktreeState(worktreePath)

    fmt.Printf("Worktree exists: %s\n", worktreePath)
    fmt.Printf("No active process (likely crashed).\n\n")
    fmt.Printf("Last checkpoint:\n")
    fmt.Printf("  Phase: %s (iteration %d/%d)\n", state.Phase, state.Iteration, state.MaxIterations)
    fmt.Printf("  Time: %s ago\n", state.LastActivity)
    fmt.Printf("  Changes: %d files modified, %d uncommitted\n", state.ModifiedFiles, state.UncommittedFiles)
    fmt.Println()
    fmt.Println("Options:")
    fmt.Println("  [1] Resume from checkpoint")
    fmt.Println("  [2] Inspect worktree (opens shell)")
    fmt.Println("  [3] Clean up and restart")
    fmt.Println("  [4] Cancel")

    switch promptChoice([]string{"1", "2", "3", "4"}) {
    case "1":
        guard.Acquire()
        defer guard.Release()
        return e.executeInDir(worktreePath, taskID)
    case "2":
        return e.openShellIn(worktreePath)
    case "3":
        e.cleanupWorktree(worktreePath)
        return e.Run(taskID) // Recurse to create fresh
    default:
        return nil
    }
}

func (e *Executor) formatRunningError(taskID, worktreePath string, pid int) error {
    state := e.inspectWorktreeState(worktreePath)

    return fmt.Errorf("Task %s is already running.\n\n"+
        "  Process: PID %d\n"+
        "  Started: %s ago\n"+
        "  Phase: %s (iteration %d/%d)\n\n"+
        "Options:\n"+
        "  orc status %s      # View progress\n"+
        "  orc logs %s -f     # Follow transcript\n"+
        "  orc pause %s       # Stop execution\n\n"+
        "If process is hung: orc run %s --force-clean",
        taskID, pid, state.StartedAgo, state.Phase, state.Iteration, state.MaxIterations,
        taskID, taskID, taskID, taskID)
}
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Git Repository                            │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ .orc/shared/                      (git-tracked)            │  │
│  │ ├── config.yaml                   Team defaults            │  │
│  │ ├── prompts/                      Shared prompts           │  │
│  │ │   ├── implement.md                                       │  │
│  │ │   └── test.md                                            │  │
│  │ ├── skills/                       Shared skills            │  │
│  │ │   └── company-style/                                     │  │
│  │ ├── templates/                    Shared task templates    │  │
│  │ │   └── feature.yaml                                       │  │
│  │ └── team.yaml                     Team member registry     │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ .orc/tasks/                       (git-tracked per task)   │  │
│  │ ├── TASK-AM-001/                  Alice's tasks            │  │
│  │ │   ├── task.yaml                                          │  │
│  │ │   ├── plan.yaml                                          │  │
│  │ │   └── state.yaml                                         │  │
│  │ └── TASK-BJ-001/                  Bob's tasks              │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  .orc/local/                         (gitignored)               │
│  .orc/orc.db                         (gitignored)               │
│  .orc/worktrees/                     (gitignored)               │
└─────────────────────────────────────────────────────────────────┘

     ▲                    ▲                    ▲
     │ git pull/push      │ git pull/push      │ git pull/push
     │                    │                    │
┌────┴────┐          ┌────┴────┐          ┌────┴────┐
│ Alice   │          │  Bob    │          │ Charlie │
│ Laptop  │          │ Desktop │          │ Server  │
│ orc     │          │ orc     │          │ orc     │
└─────────┘          └─────────┘          └─────────┘
```

---

## Task ID Namespacing

### Problem

Without coordination, developers could create conflicting task IDs:
- Alice creates TASK-001
- Bob creates TASK-001
- Git conflict!

### Solution: Prefixed Task IDs

```yaml
# .orc/shared/config.yaml
task_id:
  mode: p2p
  prefix_source: initials  # or: username, email_hash, machine
```

| Prefix Source | Example ID | Notes |
|---------------|------------|-------|
| `initials` | TASK-AM-001 | User's initials (configured) |
| `username` | TASK-alice-001 | System username |
| `email_hash` | TASK-a1b2-001 | First 4 chars of email hash |
| `machine` | TASK-laptop-001 | Machine hostname |

### User Configuration

```yaml
# ~/.orc/config.yaml
identity:
  initials: AM              # For prefix_source: initials
  display_name: Alice M     # For team visibility
```

### ID Generation

```go
type TaskIDGenerator struct {
    Mode     string    // p2p
    Prefix   string    // AM
    store    TaskStore
}

func (g *TaskIDGenerator) Next() (string, error) {
    // Get next sequence number for this prefix
    seq := g.store.NextSequence(g.Prefix)
    return fmt.Sprintf("TASK-%s-%03d", g.Prefix, seq), nil
}
```

### Sequence Storage

```yaml
# .orc/local/sequences.yaml (gitignored)
prefixes:
  AM: 15       # Alice's next number: 16
  BJ: 8        # Bob's next number: 9
```

---

## Shared Resources

### Directory Structure

```
.orc/shared/
├── config.yaml          # Team defaults
├── prompts/             # Shared prompts
│   ├── implement.md
│   ├── test.md
│   └── review.md
├── skills/              # Shared skills
│   ├── company-style/
│   │   └── SKILL.md
│   └── code-review/
│       └── SKILL.md
├── templates/           # Task templates
│   ├── feature.yaml
│   ├── bugfix.yaml
│   └── spike.yaml
└── team.yaml            # Team registry
```

### Shared Config (`config.yaml`)

```yaml
# .orc/shared/config.yaml
version: 1

# Team defaults (overridable by individuals)
defaults:
  profile: safe              # Suggested profile
  model: claude-sonnet-4     # Suggested model

# Task ID configuration
task_id:
  mode: p2p
  prefix_source: initials

# Shared gate defaults
gates:
  default_type: auto
  phase_overrides:
    review: ai               # AI reviews for all team members

# Cost warnings (informational only)
cost:
  warn_per_task: 2.00        # Suggest warning at $2
```

### Team Registry (`team.yaml`)

```yaml
# .orc/shared/team.yaml
version: 1
members:
  - initials: AM
    name: Alice Martinez
    email: alice@company.com   # Optional

  - initials: BJ
    name: Bob Johnson
    email: bob@company.com

# Prefix reservation (prevents conflicts)
reserved_prefixes:
  - AM
  - BJ
  - CC    # Reserved for Charlie
```

---

## Resource Resolution

### Prompt Resolution Chain

```
1. ~/.orc/prompts/implement.md           # Personal global
2. .orc/local/prompts/implement.md       # Personal project (gitignored)
3. .orc/shared/prompts/implement.md      # Team shared
4. templates/prompts/implement.md        # Builtin
```

### Implementation

```go
func (s *PromptService) Resolve(phase string) (content string, source Source, err error) {
    // 1. Personal global
    if content, err := s.readFile(s.userPromptsDir, phase+".md"); err == nil {
        return content, SourcePersonalGlobal, nil
    }

    // 2. Personal project (local)
    if content, err := s.readFile(s.projectLocalDir, "prompts", phase+".md"); err == nil {
        return content, SourcePersonalProject, nil
    }

    // 3. Team shared
    if content, err := s.readFile(s.projectSharedDir, "prompts", phase+".md"); err == nil {
        return content, SourceTeamShared, nil
    }

    // 4. Builtin
    return s.getEmbedded(phase)
}
```

---

## Concurrent Execution Model

### Design Philosophy

**No cross-user blocking.** Anyone with access can run any task. Each execution is isolated:

| Who runs | Branch | Worktree |
|----------|--------|----------|
| Alice runs TASK-AM-001 | `orc/TASK-AM-001-am` | `.orc/worktrees/TASK-AM-001-am/` |
| Bob runs TASK-AM-001 | `orc/TASK-AM-001-bj` | `.orc/worktrees/TASK-AM-001-bj/` |
| Alice runs again | Same branch, resumes | Same worktree |
| Solo user runs TASK-001 | `orc/TASK-001` | `.orc/worktrees/TASK-001/` |

### Ownership Model

**Creator prefix** vs **Executor tag** - two distinct concepts:

| Concept | Example | Meaning |
|---------|---------|---------|
| **Creator prefix** | `TASK-AM-001` | Alice (AM) created this task |
| **Executor tag** | `orc/TASK-AM-001-bj` | Bob (BJ) is running this execution |

**Key principles:**
- Creator prefix is **immutable** - part of the task ID forever
- Creator prefix indicates who made the task, nothing more
- **Anyone can run any task** - no access control
- Executor tag shows who's running this particular execution
- Multiple people can run the same task (separate branches)
- Ownership ≠ exclusive access

### Redundant Work Prevention

Before creating a new branch, orc checks for existing work:

```
$ orc run TASK-AM-001
Note: Branch 'orc/TASK-AM-001-am' was merged to main 2 hours ago.
      You may be duplicating completed work.

Options:
  [1] View what was done (git diff main..orc/TASK-AM-001-am)
  [2] Continue anyway (create orc/TASK-AM-001-bj)
  [3] Cancel

>
```

```
$ orc run TASK-AM-001
Note: Branch 'orc/TASK-AM-001-am' exists (last commit: 30 min ago by alice)
      Someone else is working on this task.

Options:
  [1] Join Alice's branch (checkout orc/TASK-AM-001-am)
  [2] Fork your own (create orc/TASK-AM-001-bj)
  [3] Cancel

>
```

This is a **warning, not a block**. Users can always proceed.

### Reconciling Parallel Executions

When multiple people work on the same task:

1. **First to merge wins** - their branch becomes the canonical implementation
2. **Others can:**
   - Cherry-pick specific commits from their branch
   - Rebase onto main and continue
   - Abandon their branch if work is redundant
3. **Normal git workflow applies** - no special orc handling
4. **PRs show the diff** - reviewers see all approaches

### Same-User Protection (PID Guard)

The only hard block is preventing the same user from running twice:

```go
// internal/executor/pid_guard.go
type PIDGuard struct {
    worktreePath string
}

func (g *PIDGuard) Check() error {
    pidFile := filepath.Join(g.worktreePath, ".orc.pid")

    data, err := os.ReadFile(pidFile)
    if err != nil {
        return nil // No PID file, good to go
    }

    pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
    if processExists(pid) {
        return fmt.Errorf("task already running (pid %d)", pid)
    }

    // Stale PID, clean it up
    os.Remove(pidFile)
    return nil
}

func (g *PIDGuard) Acquire() error {
    pidFile := filepath.Join(g.worktreePath, ".orc.pid")
    return os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func (g *PIDGuard) Release() {
    os.Remove(filepath.Join(g.worktreePath, ".orc.pid"))
}
```

### Branch/Worktree Naming

Includes executor tag to prevent conflicts between users:

```go
// internal/git/naming.go

// BranchName creates the git branch name for a task execution.
// Solo mode: orc/TASK-001
// P2P/Team:  orc/TASK-AM-001-bj (task ID + lowercase executor tag)
func BranchName(taskID, executorTag string) string {
    if executorTag == "" {
        return "orc/" + taskID  // Solo mode
    }
    return fmt.Sprintf("orc/%s-%s", taskID, strings.ToLower(executorTag))
}

// WorktreePath creates the worktree directory path.
// Solo mode: .orc/worktrees/TASK-001/
// P2P/Team:  .orc/worktrees/TASK-AM-001-bj/
func WorktreePath(taskID, executorTag string) string {
    name := taskID
    if executorTag != "" {
        name = fmt.Sprintf("%s-%s", taskID, strings.ToLower(executorTag))
    }
    return filepath.Join(".orc", "worktrees", name)
}
```

### Casing Rules

| Context | Format | Example |
|---------|--------|---------|
| Task ID | UPPERCASE prefix | `TASK-AM-001` |
| Branch name | lowercase tag | `orc/TASK-AM-001-bj` |
| Worktree path | lowercase tag | `.orc/worktrees/TASK-AM-001-bj/` |

### Execution Flow

```
┌─────────────────────┐
│ orc run TASK-AM-001 │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Get executor prefix │  (from ~/.orc/config.yaml identity.initials)
│ e.g., "bj" for Bob  │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│ Check worktree      │  .orc/worktrees/TASK-AM-001-bj/
│ exists?             │
└──────────┬──────────┘
           │
     ┌─────┴─────┐
     │           │
    Yes          No
     │           │
     ▼           ▼
┌─────────┐  ┌─────────────┐
│ Check   │  │ Create      │
│ PID     │  │ worktree    │
│ guard   │  │ and branch  │
└────┬────┘  └──────┬──────┘
     │              │
     ▼              ▼
┌─────────────────────┐
│ Execute task        │
│ (write PID file)    │
└─────────────────────┘
```

---

## Task Visibility

### Mode-Aware Task List

**In P2P/Team mode**, `orc list` shows both local and team tasks by default:

```bash
$ orc list
Fetching remote branches... done

TASK-AM-001  running   large   implement  Add authentication    (you)
TASK-AM-002  planned   small   -          Fix login bug         (you)
TASK-BJ-001  running   medium  test       Refactor API          (bob) ← remote
TASK-BJ-002  completed small   -          Update README         (bob) ← remote
```

**In Solo mode**, shows local tasks only (no remote fetch):

```bash
$ orc list
TASK-001  running   large   implement  Add authentication
TASK-002  planned   small   -          Fix login bug
```

### Filter Options

```bash
# P2P mode - local tasks only (skip remote fetch)
$ orc list --local
TASK-AM-001  running   large   implement  Add authentication
TASK-AM-002  planned   small   -          Fix login bug

# Show only remote tasks
$ orc list --remote
TASK-BJ-001  running   medium  test       Refactor API          (bob)
TASK-BJ-002  completed small   -          Update README         (bob)

# Filter by creator
$ orc list --creator bj
TASK-BJ-001  running   medium  test       Refactor API          (bob)
TASK-BJ-002  completed small   -          Update README         (bob)

# Filter by status
$ orc list --status running
TASK-AM-001  running   large   implement  Add authentication    (you)
TASK-BJ-001  running   medium  test       Refactor API          (bob)
```

### Visual Indicators

| Indicator | Meaning |
|-----------|---------|
| `(you)` | Your task (created by you) |
| `(alice)` | Task creator's name |
| `← remote` | Task from git remote, not local |
| `← local` | Task exists locally but not pushed |
| `⚡ active` | Task is currently running (has PID) |

### Implementation

```go
// internal/task/list.go
func ListTasks(mode Mode, filter ListFilter) ([]TaskSummary, error) {
    var tasks []TaskSummary

    // Always include local tasks
    localTasks, err := listLocalTasks()
    if err != nil {
        return nil, err
    }
    tasks = append(tasks, localTasks...)

    // P2P/Team: also fetch remote tasks (unless --local flag)
    if mode != ModeSolo && !filter.LocalOnly {
        remoteTasks, err := listRemoteTasks()
        if err != nil {
            // Non-fatal: log warning but continue
            log.Warn("Failed to fetch remote tasks: %v", err)
        } else {
            tasks = mergeTaskLists(tasks, remoteTasks)
        }
    }

    return applyFilters(tasks, filter), nil
}

func listRemoteTasks() ([]TaskSummary, error) {
    // Fetch with timeout to not block if network is slow
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := git.FetchWithContext(ctx, "origin"); err != nil {
        return nil, err
    }

    // List all orc/* branches
    branches, err := git.ListRemoteBranches("origin", "orc/TASK-*")
    if err != nil {
        return nil, err
    }

    var tasks []TaskSummary
    for _, branch := range branches {
        taskID := extractTaskID(branch)

        // Read task.yaml from that branch
        content, err := git.ShowFile("origin/"+branch, ".orc/tasks/"+taskID+"/task.yaml")
        if err != nil {
            continue
        }

        var task Task
        yaml.Unmarshal(content, &task)
        tasks = append(tasks, TaskSummary{
            ID:       task.ID,
            Title:    task.Title,
            Status:   task.Status,
            Creator:  extractCreator(taskID),
            IsRemote: true,
        })
    }
    return tasks, nil
}

func mergeTaskLists(local, remote []TaskSummary) []TaskSummary {
    seen := make(map[string]bool)
    var result []TaskSummary

    // Local tasks first
    for _, t := range local {
        seen[t.ID] = true
        result = append(result, t)
    }

    // Add remote tasks not in local
    for _, t := range remote {
        if !seen[t.ID] {
            result = append(result, t)
        }
    }

    return result
}
```

---

## Conflict Avoidance

### Task Files: No Conflicts

Each developer works on their own tasks (prefixed IDs), so task files don't conflict.

### Shared Resources: Git Handles It

Prompts, skills, templates in `.orc/shared/` are normal git-tracked files:
- Edit conflicts handled by git merge
- Use pull requests for shared resource changes
- Review process for team defaults

### Config Conflicts: Merge Strategy

```yaml
# .orc/shared/config.yaml
# Use YAML-aware merge driver for config files

# .gitattributes
.orc/shared/config.yaml merge=union
```

---

## Workflow Examples

### Example 1: Alice Starts a Task

```bash
# Alice
cd project
git pull                           # Get latest
orc new "Add user dashboard"       # Creates TASK-AM-005
orc run TASK-AM-005                # Executes locally
git add .orc/tasks/TASK-AM-005/
git commit -m "[orc] TASK-AM-005: planned"
git push
```

### Example 2: Bob Sees Alice's Task

```bash
# Bob
cd project
git pull                           # Gets Alice's task
orc list                           # Shows TASK-AM-005 (alice) - team tasks by default
```

### Example 3: Parallel Work

```bash
# Alice working on TASK-AM-005 (implement phase)
# Bob working on TASK-BJ-003 (test phase)
# No conflicts - different task directories
```

### Example 4: Shared Prompt Update

```bash
# Alice wants to improve implement prompt
cd project
vim .orc/shared/prompts/implement.md

git add .orc/shared/prompts/implement.md
git commit -m "Improve implement prompt with better test guidance"
git push

# Bob pulls and gets the new prompt automatically
```

### Example 5: Multiple Users Same Task

```bash
# Alice runs TASK-AM-005
orc run TASK-AM-005
# Creates branch: orc/TASK-AM-005-am
# Creates worktree: .orc/worktrees/TASK-AM-005-am/

# Bob also runs TASK-AM-005 (different execution)
orc run TASK-AM-005
# Creates branch: orc/TASK-AM-005-bj
# Creates worktree: .orc/worktrees/TASK-AM-005-bj/

# Both run independently, no conflicts
# Each has their own branch and worktree
```

---

## Gitignore Configuration

```gitignore
# .gitignore

# Local state (never share)
.orc/orc.db
.orc/local/
.orc/worktrees/
.orc/tasks/*/transcripts/    # Optional: transcripts can be large

# Keep these tracked
!.orc/shared/
!.orc/tasks/*/task.yaml
!.orc/tasks/*/plan.yaml
!.orc/tasks/*/state.yaml
```

---

## Sync Strategy

### What to Commit

| Path | Commit? | Notes |
|------|---------|-------|
| `.orc/shared/*` | Yes | Team resources |
| `.orc/tasks/*/task.yaml` | Yes | Task definition |
| `.orc/tasks/*/plan.yaml` | Yes | Phase sequence |
| `.orc/tasks/*/state.yaml` | Optional | Execution state |
| `.orc/tasks/*/transcripts/` | Optional | Can be large |
| `.orc/local/*` | No | Personal overrides |
| `.orc/worktrees/` | No | Local worktrees |
| `.orc/orc.db` | No | Local database |

---

## Team Setup

### Zero-Friction Onboarding

Team setup should take **< 30 seconds**. No YAML editing required.

### First Team Member (Creates P2P Structure)

```bash
$ orc init --p2p
Creating P2P structure...
Enter your initials: AM
Enter your name (optional): Alice Martinez

✓ Created .orc/shared/
✓ Created .orc/shared/config.yaml
✓ Added you to team.yaml
✓ Configured identity.initials = AM

Next steps:
  git add .orc/shared/
  git commit -m "Initialize orc P2P structure"
  git push
```

### Teammates Join (Auto-Detected)

When a teammate clones or pulls:

```bash
$ orc init
P2P mode detected (.orc/shared/ exists)
Enter your initials: BJ
Enter your name (optional): Bob Johnson

✓ Added you to team.yaml
✓ Configured identity.initials = BJ

Ready to create tasks!
```

### Registration is Optional

By default, **no registration required**. Anyone can use any initials:

```yaml
# .orc/shared/config.yaml (default)
task_id:
  require_registration: false  # First use claims initials
```

For larger teams that want prefix reservation:

```yaml
# .orc/shared/config.yaml
task_id:
  require_registration: true  # Must be in team.yaml
```

### Team Registry (Auto-Populated)

```yaml
# .orc/shared/team.yaml - auto-populated by orc init
members:
  AM:
    name: Alice Martinez
    joined: 2025-01-10T10:00:00Z
  BJ:
    name: Bob Johnson
    joined: 2025-01-11T09:00:00Z
```

No manual editing needed. `orc init` handles it.

### Duplicate Initials Handling

```bash
$ orc init
P2P mode detected.
Enter your initials: AM
Error: Initials 'AM' already registered to Alice Martinez.
       Pick different initials or contact Alice.

Enter your initials: AX
✓ Configured identity.initials = AX
```

### Legacy: Adding Team Member Manually

For users who prefer manual setup:

```bash
# 1. Configure identity
cat >> ~/.orc/config.yaml << 'EOF'
identity:
  initials: AM
  display_name: Alice Martinez
EOF

# 2. Register in team.yaml (via PR)
# Add to .orc/shared/team.yaml:
#   - initials: AM
#     name: Alice Martinez

# 3. Reserve prefix
# Add to reserved_prefixes: [AM]
```

### Validation

```go
func validateTaskID(id string, team *Team) error {
    prefix := extractPrefix(id)  // TASK-AM-001 -> AM

    // Check if prefix is registered
    if !team.HasMember(prefix) && !team.HasReservedPrefix(prefix) {
        return fmt.Errorf("unknown prefix %s: register in team.yaml first", prefix)
    }

    return nil
}
```

---

## Edge Cases

### Developer Leaves Team

1. Complete or reassign their in-progress tasks
2. Remove from `team.yaml`
3. Their completed tasks remain in history
4. Their prefix can be reserved or reassigned

### Merge Conflicts in Shared Resources

```bash
# Standard git conflict resolution
git pull
# CONFLICT: .orc/shared/prompts/implement.md

# Edit to resolve
vim .orc/shared/prompts/implement.md
git add .orc/shared/prompts/implement.md
git commit
```

---

## CLI Commands

### Mode-Specific Behavior

| Command | Solo Mode | P2P/Team Mode |
|---------|-----------|---------------|
| `orc init` | Standard init | `orc init --p2p` for first user, auto-detects for others |
| `orc new` | Simple ID (`TASK-001`) | Prefixed ID (`TASK-AM-001`) |
| `orc run` | Single branch | Branch per executor (`orc/TASK-AM-001-bj`) |
| `orc list` | Local tasks only | Team tasks by default (with remote indicator) |

### P2P Commands

| Command | Description |
|---------|-------------|
| `orc init --p2p` | Initialize P2P structure (creates `.orc/shared/`) |
| `orc list` | Show all tasks (local + team in P2P mode) |
| `orc list --local` | Show only local tasks (filter out team) |
| `orc team members` | List registered team members |

### Worktree Management

| Command | Description |
|---------|-------------|
| `orc status` | Show running task status (PID, phase, iterations) |
| `orc gc` | Clean up orphaned worktrees (no active process) |
| `orc gc --dry-run` | Show what would be cleaned without removing |
| `orc gc --force` | Clean all worktrees (even with active processes) |

### Garbage Collection (`orc gc`)

Cleans up orphaned worktrees from crashed or completed executions:

```bash
$ orc gc
Scanning .orc/worktrees/...

Found 3 worktrees:
  TASK-AM-001-am   orphaned (no PID, last modified 2 days ago)
  TASK-AM-002-am   active (PID 12345)
  TASK-BJ-003-bj   orphaned (stale PID 99999)

Clean up 2 orphaned worktrees? [y/N] y
Removed .orc/worktrees/TASK-AM-001-am/
Removed .orc/worktrees/TASK-BJ-003-bj/
Done. 2 worktrees cleaned, 23MB freed.
```

### Implementation

```go
// internal/cli/cmd_gc.go
func runGC(dryRun, force bool) error {
    worktrees, err := scanWorktrees(".orc/worktrees/")
    if err != nil {
        return err
    }

    var orphaned []WorktreeInfo
    for _, wt := range worktrees {
        if !wt.HasActivePID() {
            orphaned = append(orphaned, wt)
        }
    }

    if len(orphaned) == 0 {
        fmt.Println("No orphaned worktrees found.")
        return nil
    }

    // Show what we found
    fmt.Printf("Found %d orphaned worktrees:\n", len(orphaned))
    for _, wt := range orphaned {
        fmt.Printf("  %s (last modified %s)\n", wt.Name, wt.ModifiedAgo)
    }

    if dryRun {
        return nil
    }

    if !force && !promptConfirm("Clean up?") {
        return nil
    }

    var freed int64
    for _, wt := range orphaned {
        freed += wt.Size
        if err := os.RemoveAll(wt.Path); err != nil {
            fmt.Printf("Warning: failed to remove %s: %v\n", wt.Path, err)
            continue
        }
        // Also remove git worktree reference
        exec.Command("git", "worktree", "remove", wt.Path).Run()
    }

    fmt.Printf("Done. %d worktrees cleaned, %s freed.\n",
        len(orphaned), humanize.Bytes(uint64(freed)))
    return nil
}
```

---

## Comparison: P2P vs Server Mode

| Feature | P2P Mode | Server Mode |
|---------|----------|-------------|
| Infrastructure | Git only | Dedicated server |
| Real-time updates | No (git pull) | Yes (WebSocket) |
| Presence | No | Yes |
| Centralized dashboard | No | Yes |
| Cost aggregation | No | Yes |
| Authentication | Git-based | OIDC/SAML |
| Offline work | Full | Partial |
| Setup complexity | Low | Medium |

---

## Migration: P2P → Server Mode

P2P and Server modes are compatible. To add server:

```yaml
# .orc/shared/config.yaml
team:
  mode: p2p                          # Keep P2P for git coordination
  server: https://orc.company.com    # Add server for dashboard
  sync:
    tasks: true                      # Sync task metadata to server
    resources: false                 # Keep resources in git
```

Server becomes optional dashboard/visibility layer, not execution coordinator.

---

## Testing P2P Mode

### Test: Mode Detection

```go
func TestModeDetection(t *testing.T) {
    tests := []struct {
        name     string
        setup    func(dir string)
        expected Mode
    }{
        {
            name:     "default is solo",
            setup:    func(dir string) {},
            expected: ModeSolo,
        },
        {
            name: "p2p when shared exists",
            setup: func(dir string) {
                os.MkdirAll(filepath.Join(dir, ".orc", "shared"), 0755)
            },
            expected: ModeP2P,
        },
        {
            name: "team when server configured",
            setup: func(dir string) {
                cfg := `team:
  server_url: https://orc.example.com`
                os.MkdirAll(filepath.Join(dir, ".orc"), 0755)
                os.WriteFile(filepath.Join(dir, ".orc", "config.yaml"), []byte(cfg), 0644)
            },
            expected: ModeTeam,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            dir := t.TempDir()
            tt.setup(dir)
            assert.Equal(t, tt.expected, DetectMode(dir))
        })
    }
}
```

### Test: Prefix Generation

```go
func TestPrefixedTaskID(t *testing.T) {
    gen := NewTaskIDGenerator("p2p", "AM")

    id1 := gen.Next()
    assert.Equal(t, "TASK-AM-001", id1)

    id2 := gen.Next()
    assert.Equal(t, "TASK-AM-002", id2)
}

func TestSoloTaskID(t *testing.T) {
    gen := NewTaskIDGenerator("solo", "")

    id1 := gen.Next()
    assert.Equal(t, "TASK-001", id1)  // No prefix in solo mode
}
```

### Test: Branch/Worktree Naming

```go
func TestBranchNaming(t *testing.T) {
    tests := []struct {
        taskID      string
        executorTag string
        expected    string
    }{
        {"TASK-001", "", "orc/TASK-001"},                // Solo
        {"TASK-AM-001", "bj", "orc/TASK-AM-001-bj"},    // P2P
        {"TASK-AM-001", "BJ", "orc/TASK-AM-001-bj"},    // Lowercase enforcement
    }

    for _, tt := range tests {
        result := BranchName(tt.taskID, tt.executorTag)
        assert.Equal(t, tt.expected, result)
    }
}

func TestWorktreePath(t *testing.T) {
    tests := []struct {
        taskID      string
        executorTag string
        expected    string
    }{
        {"TASK-001", "", ".orc/worktrees/TASK-001"},
        {"TASK-AM-001", "bj", ".orc/worktrees/TASK-AM-001-bj"},
    }

    for _, tt := range tests {
        result := WorktreePath(tt.taskID, tt.executorTag)
        assert.Equal(t, tt.expected, result)
    }
}
```

### Test: PID Guard

```go
func TestPIDGuard(t *testing.T) {
    dir := t.TempDir()
    guard := &PIDGuard{worktreePath: dir}

    // No PID file - check passes
    assert.NoError(t, guard.Check())

    // Acquire creates PID file
    assert.NoError(t, guard.Acquire())
    pidFile := filepath.Join(dir, ".orc.pid")
    assert.FileExists(t, pidFile)

    // Current process - check fails
    assert.Error(t, guard.Check())

    // Release removes PID file
    guard.Release()
    assert.NoFileExists(t, pidFile)
}

func TestPIDGuardStalePID(t *testing.T) {
    dir := t.TempDir()
    pidFile := filepath.Join(dir, ".orc.pid")

    // Write stale PID (non-existent process)
    os.WriteFile(pidFile, []byte("999999"), 0644)

    guard := &PIDGuard{worktreePath: dir}
    // Stale PID should pass (auto-cleaned)
    assert.NoError(t, guard.Check())
    assert.NoFileExists(t, pidFile)  // Stale PID removed
}
```

### Test: Existing Branch Warning

```go
func TestExistingBranchWarning(t *testing.T) {
    // Setup mock git with existing branch
    git := &MockGit{
        branches: []BranchInfo{
            {Name: "orc/TASK-AM-001-am", IsMerged: true, MergedAgo: "2h"},
        },
    }

    executor := &Executor{git: git}
    warning := executor.checkExistingBranches("TASK-AM-001", "bj")

    assert.Contains(t, warning, "merged to main")
    assert.Contains(t, warning, "duplicating completed work")
}

func TestNoWarningForOwnBranch(t *testing.T) {
    git := &MockGit{
        branches: []BranchInfo{
            {Name: "orc/TASK-AM-001-am", IsMerged: false, Tag: "am"},
        },
    }

    executor := &Executor{git: git}
    warning := executor.checkExistingBranches("TASK-AM-001", "am")

    assert.Empty(t, warning)  // Own branch, no warning
}
```

### Test: Resource Resolution

```go
func TestPromptResolution(t *testing.T) {
    // Setup
    // - ~/.orc/prompts/implement.md = "personal"
    // - .orc/shared/prompts/implement.md = "team"

    service := NewPromptService(...)
    content, source, _ := service.Resolve("implement")

    assert.Equal(t, "personal", content)
    assert.Equal(t, SourcePersonalGlobal, source)
}
```

### Test: Garbage Collection

```go
func TestGarbageCollection(t *testing.T) {
    dir := t.TempDir()
    worktreesDir := filepath.Join(dir, ".orc", "worktrees")
    os.MkdirAll(worktreesDir, 0755)

    // Create orphaned worktree (no PID)
    orphaned := filepath.Join(worktreesDir, "TASK-001")
    os.MkdirAll(orphaned, 0755)

    // Create active worktree (with current PID)
    active := filepath.Join(worktreesDir, "TASK-002")
    os.MkdirAll(active, 0755)
    os.WriteFile(filepath.Join(active, ".orc.pid"),
        []byte(strconv.Itoa(os.Getpid())), 0644)

    // Run GC
    gc := &GarbageCollector{baseDir: dir}
    cleaned, err := gc.Run(false)  // not dry-run

    assert.NoError(t, err)
    assert.Equal(t, 1, len(cleaned))
    assert.DirExists(t, active)
    assert.NoDirExists(t, orphaned)
}
```

### Test: Solo Mode Guarantees

```go
func TestSoloModeNoIdentityRequired(t *testing.T) {
    dir := t.TempDir()
    setupSoloProject(dir)  // No .orc/shared/

    cfg := &Config{}  // Empty identity
    executor := NewExecutor(dir, cfg)

    // Should succeed without identity.initials
    err := executor.Run("TASK-001")
    assert.NoError(t, err)
}

func TestP2PModeRequiresIdentity(t *testing.T) {
    dir := t.TempDir()
    setupP2PProject(dir)  // Creates .orc/shared/

    cfg := &Config{}  // Empty identity
    executor := NewExecutor(dir, cfg)

    // Should fail without identity.initials
    err := executor.Run("TASK-001")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "identity.initials required")
}
```
