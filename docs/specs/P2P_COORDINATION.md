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
| `solo` | Default, single user | No locks, no prefixes, no sync |
| `p2p` | `.orc/shared/` exists | File locks, prefixed IDs, git sync |
| `team` | `team.server_url` configured | Server locks, real-time, dashboard |

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

    // Default: solo
    return ModeSolo
}
```

### Mode Guard in Executor

```go
// internal/executor/executor.go
func (e *Executor) Run(taskID string) error {
    mode := e.config.TaskID.Mode
    prefix := e.config.Identity.Initials

    // All modes use worktree isolation with executor prefix
    worktreePath := WorktreePath(taskID, prefix)
    branchName := BranchName(taskID, prefix)

    switch mode {
    case "solo":
        // No prefix in naming, simple worktree
        return e.runInWorktree(taskID, "", taskID)

    case "p2p":
        // Worktree with executor prefix
        return e.runInWorktree(taskID, prefix, branchName)

    case "team":
        // Worktree + server sync for visibility
        if err := e.notifyServerStart(taskID); err != nil {
            log.Warn("server notification failed", "error", err)
            // Continue anyway - execution is local
        }
        defer e.notifyServerStop(taskID)
        return e.runInWorktree(taskID, prefix, branchName)

    default:
        return fmt.Errorf("unknown mode: %s", mode)
    }
}

func (e *Executor) runInWorktree(taskID, prefix, branchName string) error {
    worktreePath := WorktreePath(taskID, prefix)

    // PID guard: prevent same user running same task twice
    guard := &PIDGuard{worktreePath: worktreePath}
    if err := guard.Check(); err != nil {
        return err
    }

    // Create or reuse worktree
    if !exists(worktreePath) {
        if err := e.git.CreateWorktree(worktreePath, branchName); err != nil {
            return err
        }
    }

    guard.Acquire()
    defer guard.Release()

    return e.executeInDir(worktreePath, taskID)
}
```

### Solo Mode Guarantees

When `mode: solo`, these features are simplified:

- No prefix in branch/worktree naming (just task ID)
- No team member registry validation
- No server sync attempts

```go
// Solo mode: simple naming without prefix
if mode == ModeSolo {
    return e.runInWorktree(taskID, "", taskID)
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
│  │ ├── TASK-alice-001/               Alice's tasks            │  │
│  │ │   ├── task.yaml                                          │  │
│  │ │   ├── plan.yaml                                          │  │
│  │ │   └── lock.yaml                 Execution lock           │  │
│  │ └── TASK-bob-001/                 Bob's tasks              │  │
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

**No cross-user locking.** Anyone with access can run any task. Each execution is independent:

| Who runs | Branch | Worktree |
|----------|--------|----------|
| Alice runs TASK-AM-001 | `orc/TASK-AM-001-am` | `.orc/worktrees/TASK-AM-001-am/` |
| Bob runs TASK-AM-001 | `orc/TASK-AM-001-bj` | `.orc/worktrees/TASK-AM-001-bj/` |
| Alice runs again | Same branch, resumes | Same worktree |

**Key principles:**
- Task prefix (AM, BJ) indicates creator, not exclusive access
- Multiple people can work on same task simultaneously
- Each execution gets its own branch/worktree (no conflicts)
- Ownership is metadata, not access control

### Same-User Protection (PID Guard)

The only protection is preventing the same user from accidentally running the same task twice:

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

Includes executor identity to prevent conflicts:

```go
// internal/git/naming.go
func BranchName(taskID, executorPrefix string) string {
    return fmt.Sprintf("orc/%s-%s", taskID, strings.ToLower(executorPrefix))
}

func WorktreePath(taskID, executorPrefix string) string {
    return filepath.Join(".orc", "worktrees",
        fmt.Sprintf("%s-%s", taskID, strings.ToLower(executorPrefix)))
}
```

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

### Local Task List

```bash
$ orc list
TASK-AM-001  running   large   implement  Add authentication
TASK-AM-002  planned   small   -          Fix login bug
```

### Team Task List (from Git)

```bash
$ orc list --team
Fetching remote branches...

TASK-AM-001  running   large   implement  Add authentication    (alice)
TASK-AM-002  planned   small   -          Fix login bug         (alice)
TASK-BJ-001  running   medium  test       Refactor API          (bob)
TASK-BJ-002  completed small   -          Update README         (bob)
```

### Implementation

```go
func listTeamTasks() ([]TaskSummary, error) {
    // List all orc/* branches
    branches, err := git.ListRemoteBranches("origin", "orc/TASK-*")
    if err != nil {
        return nil, err
    }

    var tasks []TaskSummary
    for _, branch := range branches {
        // Extract task ID from branch name
        taskID := strings.TrimPrefix(branch, "orc/")

        // Read task.yaml from that branch
        content, err := git.ShowFile("origin/"+branch, ".orc/tasks/"+taskID+"/task.yaml")
        if err != nil {
            continue
        }

        var task Task
        yaml.Unmarshal(content, &task)
        tasks = append(tasks, TaskSummary{
            ID:     task.ID,
            Title:  task.Title,
            Status: task.Status,
            Owner:  extractOwner(taskID),  // From prefix
        })
    }
    return tasks, nil
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
orc list --team                    # Shows TASK-AM-005 (alice)
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

### Initial Setup

```bash
# Team lead creates shared structure
mkdir -p .orc/shared/{prompts,skills,templates}

# Create team config
cat > .orc/shared/config.yaml << 'EOF'
version: 1
task_id:
  mode: p2p
  prefix_source: initials
defaults:
  profile: safe
EOF

# Create team registry
cat > .orc/shared/team.yaml << 'EOF'
version: 1
members: []
reserved_prefixes: []
EOF

git add .orc/shared/
git commit -m "Initialize orc team structure"
```

### Adding Team Member

```bash
# Alice joins the team
# 1. Configure her identity
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

### Stale Lock Detection

```go
func (l *FileLock) IsStale() bool {
    lock, err := l.readLock()
    if err != nil {
        return true  // No lock file
    }
    return time.Since(lock.Heartbeat) > lock.TTL
}
```

### Machine Crash During Execution

Lock has TTL of 60 seconds. After crash:
1. No heartbeat updates
2. Lock becomes stale after TTL
3. Next execution claims lock automatically
4. Warning logged about stale lock

---

## CLI Commands

### P2P-Specific Commands

| Command | Description |
|---------|-------------|
| `orc list --team` | Show tasks from all team members |
| `orc team join` | Register in team.yaml |
| `orc team members` | List team members |
| `orc team sync` | Pull latest shared resources |

### Standard Commands (P2P-Aware)

| Command | P2P Behavior |
|---------|--------------|
| `orc new` | Uses prefixed ID |
| `orc run` | Checks/acquires lock |
| `orc status` | Shows lock status |

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

### Test: Prefix Generation

```go
func TestPrefixedTaskID(t *testing.T) {
    gen := NewTaskIDGenerator("p2p", "AM")

    id1 := gen.Next()
    assert.Equal(t, "TASK-AM-001", id1)

    id2 := gen.Next()
    assert.Equal(t, "TASK-AM-002", id2)
}
```

### Test: Lock Acquisition

```go
func TestFileLock(t *testing.T) {
    tmpDir := t.TempDir()
    lock1 := NewFileLock(tmpDir, "alice@laptop")
    lock2 := NewFileLock(tmpDir, "bob@desktop")

    // Alice acquires
    acquired, err := lock1.TryAcquire()
    assert.True(t, acquired)
    assert.NoError(t, err)

    // Bob cannot acquire
    acquired, err = lock2.TryAcquire()
    assert.False(t, acquired)
    assert.NoError(t, err)

    // Alice releases
    lock1.Release()

    // Bob can now acquire
    acquired, err = lock2.TryAcquire()
    assert.True(t, acquired)
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
