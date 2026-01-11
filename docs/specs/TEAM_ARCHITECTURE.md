# Orc Team Architecture

> Evolving orc from solo-dev tool to team-capable system while maintaining individual developer autonomy.

## Design Principles

| Principle | Description |
|-----------|-------------|
| **Individual First** | Solo dev experience is unchanged and first-class |
| **Additive Layers** | Team features layer on top, never replace |
| **Local Autonomy** | Users control their own Claude usage, models, settings |
| **No Forced Sync** | Nothing requires a shared server to function |
| **Opt-in Complexity** | Team features activate only when configured |
| **Predictable Costs** | Users always see their token usage, never surprised |

---

## Architecture Tiers

### Tier 1: Solo Developer (Current)

```
┌─────────────────────────────────────────┐
│  Developer Machine                       │
│  ┌─────────────────────────────────────┐│
│  │ ~/.orc/                              ││
│  │ ├── orc.db          (global SQLite) ││
│  │ ├── config.yaml     (user config)   ││
│  │ ├── projects.yaml   (registry)      ││
│  │ └── token-pool/     (OAuth tokens)  ││
│  └─────────────────────────────────────┘│
│  ┌─────────────────────────────────────┐│
│  │ ~/project/.orc/                      ││
│  │ ├── orc.db          (project SQLite)││
│  │ ├── config.yaml     (project config)││
│  │ ├── prompts/        (overrides)     ││
│  │ └── tasks/          (task state)    ││
│  └─────────────────────────────────────┘│
│  ┌─────────────────────────────────────┐│
│  │ orc serve (localhost:8080)           ││
│  │ └── Web UI + WebSocket               ││
│  └─────────────────────────────────────┘│
└─────────────────────────────────────────┘
```

**Unchanged from current implementation.** This is the baseline.

---

### Tier 2: Peer-to-Peer Collaboration (No Shared Server)

Multiple developers working on the same project, each with their own orc instance, coordinating via git.

```
┌──────────────────┐    ┌──────────────────┐
│  Developer A      │    │  Developer B      │
│  ┌──────────────┐│    │┌──────────────┐  │
│  │ orc (local)  ││    ││ orc (local)  │  │
│  │ SQLite       ││    ││ SQLite       │  │
│  │ TASK-001     ││    ││ TASK-002     │  │
│  └──────────────┘│    │└──────────────┘  │
│         │        │    │       │          │
└─────────┼────────┘    └───────┼──────────┘
          │                     │
          └──────────┬──────────┘
                     │
              ┌──────┴──────┐
              │   Git Repo   │
              │ (GitHub/Lab) │
              │              │
              │ .orc/shared/ │
              │ ├── prompts/ │
              │ ├── skills/  │
              │ └── config/  │
              └─────────────┘
```

**Key Features:**
- Task IDs are globally unique (include machine/user identifier)
- Shared resources in `.orc/shared/` (git-tracked)
- Personal overrides in `.orc/local/` (gitignored)
- No conflicts - each dev works on different tasks
- Git branches provide natural isolation

---

### Tier 3: Team Server (Shared Infrastructure)

Centralized server for visibility, resource sharing, and coordination.

```
┌───────────────────────────────────────────────────────────────┐
│  Team Server                                                   │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │ orc-server                                               │  │
│  │ ├── PostgreSQL (shared state)                            │  │
│  │ │   ├── organizations                                    │  │
│  │ │   ├── members                                          │  │
│  │ │   ├── shared_resources                                 │  │
│  │ │   ├── task_visibility (read-only mirror)               │  │
│  │ │   └── cost_tracking                                    │  │
│  │ ├── API Server (:8080)                                   │  │
│  │ ├── WebSocket Hub (real-time)                            │  │
│  │ └── Optional: Token Pool (shared accounts)               │  │
│  └─────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────┘
              │
    ┌─────────┴─────────┬─────────────────┐
    │                   │                 │
┌───┴───┐          ┌────┴────┐       ┌────┴────┐
│ Dev A │          │  Dev B  │       │  Dev C  │
│ Local │          │  Local  │       │  Local  │
│ orc   │          │  orc    │       │  orc    │
└───────┘          └─────────┘       └─────────┘
```

**Key Features:**
- Server provides visibility, NOT execution
- All task execution happens locally
- Server syncs read-only task metadata
- Shared prompts/skills downloaded to local
- Individual settings always override shared

---

## Data Model

### Current Entities (Preserved)

```go
// Task - unchanged, local execution
type Task struct {
    ID           string            // TASK-001
    Title        string
    Description  string
    Weight       Weight
    Status       Status
    CurrentPhase string
    Branch       string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    Metadata     map[string]string
}

// State - unchanged, local persistence
type State struct {
    TaskID           string
    CurrentPhase     string
    CurrentIteration int
    Status           Status
    Phases           map[string]*PhaseState
    Tokens           TokenUsage
    Cost             CostTracking
}
```

### New Entities (Team Features)

```go
// Organization - billing and auth boundary (Tier 3 only)
type Organization struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    Slug      string    `json:"slug"`      // URL-safe identifier
    Plan      Plan      `json:"plan"`      // free, team, enterprise
    CreatedAt time.Time `json:"created_at"`
}

// Member - user in an org (Tier 3 only)
type Member struct {
    ID       string    `json:"id"`
    OrgID    string    `json:"org_id"`
    UserID   string    `json:"user_id"`
    Email    string    `json:"email"`
    Role     Role      `json:"role"`
    JoinedAt time.Time `json:"joined_at"`
}

type Role string
const (
    RoleOwner  Role = "owner"   // Full control, billing, delete org
    RoleAdmin  Role = "admin"   // Manage members, settings
    RoleMember Role = "member"  // Create/run tasks, use resources
    RoleViewer Role = "viewer"  // Read-only access
)

// UserPreferences - individual overrides (all tiers)
type UserPreferences struct {
    UserID           string            `json:"user_id"`
    DefaultModel     string            `json:"default_model,omitempty"`
    MaxIterations    int               `json:"max_iterations,omitempty"`
    Timeout          time.Duration     `json:"timeout,omitempty"`
    NotificationMode string            `json:"notification_mode,omitempty"`
    UIPreferences    map[string]any    `json:"ui_preferences,omitempty"`
    PromptOverrides  map[string]string `json:"prompt_overrides,omitempty"` // phase -> content
}

// SharedResource - team-level resources (Tier 2+)
type SharedResource struct {
    ID        string    `json:"id"`
    OrgID     string    `json:"org_id,omitempty"` // nil for git-based sharing
    Type      string    `json:"type"`             // prompt, skill, template, config
    Name      string    `json:"name"`
    Content   string    `json:"content"`
    Version   int       `json:"version"`
    UpdatedBy string    `json:"updated_by"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

---

## Configuration Hierarchy

### Resolution Order (Highest to Lowest Priority)

```
1. Environment Variables    (ORC_*)
2. CLI Flags               (--model, --profile)
3. User Local Overrides    (~/.orc/config.yaml)
4. Project Local Overrides (.orc/local/config.yaml)  [NEW - gitignored]
5. Project Shared Config   (.orc/shared/config.yaml) [NEW - git-tracked]
6. Organization Defaults   (server, if connected)    [NEW - Tier 3]
7. Built-in Defaults       (code)
```

### Individual Settings (Never Synced)

These settings are ALWAYS local and user-controlled:

| Setting | Location | Description |
|---------|----------|-------------|
| `default_model` | `~/.orc/config.yaml` | User's preferred Claude model |
| `max_iterations` | `~/.orc/config.yaml` | Max iterations before pause |
| `timeout` | `~/.orc/config.yaml` | Execution timeout |
| `notification_mode` | `~/.orc/config.yaml` | focus/balanced/everything |
| `token_pool` | `~/.orc/token-pool/` | User's OAuth tokens |
| `ui_preferences` | `~/.orc/ui.yaml` | Theme, layout, shortcuts |
| `prompt_overrides` | `~/.orc/prompts/` | Personal prompt customizations |

### Shared Settings (Team Level)

These can be shared but individually overridden:

| Setting | Shared Location | Override Location |
|---------|-----------------|-------------------|
| Prompts | `.orc/shared/prompts/` | `.orc/local/prompts/`, `~/.orc/prompts/` |
| Skills | `.orc/shared/skills/` | `.orc/local/skills/`, `~/.orc/skills/` |
| Templates | `.orc/shared/templates/` | `.orc/local/templates/` |
| Profile | `.orc/shared/config.yaml` | `.orc/local/config.yaml` |
| Gate configs | `.orc/shared/config.yaml` | Individual override |

### Directory Structure Evolution

```
~/.orc/                              # User global (unchanged)
├── orc.db                           # Global SQLite
├── config.yaml                      # User preferences
├── projects.yaml                    # Project registry
├── token-pool/                      # Personal OAuth tokens
├── prompts/                         # Personal prompt overrides [NEW]
├── skills/                          # Personal skills [NEW]
└── ui.yaml                          # UI preferences [NEW]

~/project/.orc/                      # Project (evolved)
├── orc.db                           # Project SQLite
├── config.yaml                      # → now imports from shared/
├── tasks/                           # Task state (unchanged)
├── worktrees/                       # Worktrees (gitignored)
│
├── shared/                          # [NEW] Git-tracked team resources
│   ├── config.yaml                  # Shared configuration
│   ├── prompts/                     # Team prompts
│   ├── skills/                      # Team skills
│   └── templates/                   # Team task templates
│
└── local/                           # [NEW] Gitignored personal overrides
    ├── config.yaml                  # Personal project config
    ├── prompts/                     # Personal prompt overrides
    └── notes/                       # Personal task notes
```

---

## Task ID Strategy

### Problem
Multiple developers creating tasks simultaneously could conflict on IDs.

### Solution: Composite IDs

```
TASK-<prefix>-<sequence>

Prefix options:
- Solo: None (TASK-001, TASK-002)
- P2P:  User initials or machine hash (TASK-RM-001, TASK-abc-001)
- Team: User ID prefix (TASK-u7x-001)
```

### Implementation

```go
type TaskIDGenerator struct {
    Mode     IDMode   // solo, p2p, team
    Prefix   string   // user/machine identifier
    Sequence int      // auto-increment per prefix
}

func (g *TaskIDGenerator) Next() string {
    g.Sequence++
    if g.Prefix == "" {
        return fmt.Sprintf("TASK-%03d", g.Sequence)
    }
    return fmt.Sprintf("TASK-%s-%03d", g.Prefix, g.Sequence)
}
```

### Configuration

```yaml
# .orc/shared/config.yaml
task_id:
  mode: p2p              # solo | p2p | team
  prefix_source: initials # none | initials | machine | user_id
```

---

## Execution Model

### Core Principle: All Execution is Local

Regardless of tier, task execution ALWAYS happens on the developer's machine:

```
┌─────────────────────────────────────────────────────────────┐
│  Developer Machine                                           │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ orc run TASK-001                                         ││
│  │   ↓                                                      ││
│  │ 1. Load task from .orc/tasks/TASK-001/                   ││
│  │ 2. Load prompts (personal → project → shared → builtin)  ││
│  │ 3. Load user's model preference                          ││
│  │ 4. Execute via user's Claude CLI (their OAuth token)     ││
│  │ 5. Save state locally                                    ││
│  │ 6. Optionally sync metadata to team server               ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### Why?
- User controls their Claude costs
- User can work offline
- No server-side secrets needed
- No rate limit sharing conflicts
- Debugging happens locally

---

## Real-Time Collaboration

### Tier 2 (P2P): Git-Based Coordination

No real-time sync needed. Developers see each other's work via git:

```bash
# Developer A
git pull
orc status  # Shows local tasks + any task branches from others

# Developer B's TASK-B-001 appears as a remote branch
git branch -r | grep orc/
# origin/orc/TASK-A-001
# origin/orc/TASK-B-002
```

### Tier 3 (Team Server): WebSocket Hub

```
┌─────────────────────────────────────────────────────────────┐
│  Team Server - WebSocket Hub                                 │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Connection Registry                                      ││
│  │ ┌─────────┬────────────┬───────────────────────────────┐││
│  │ │ User    │ Status     │ Subscriptions                 │││
│  │ ├─────────┼────────────┼───────────────────────────────┤││
│  │ │ alice   │ online     │ [TASK-A-001, project:acme]    │││
│  │ │ bob     │ online     │ [project:acme]                │││
│  │ │ charlie │ away (5m)  │ []                            │││
│  │ └─────────┴────────────┴───────────────────────────────┘││
│  └─────────────────────────────────────────────────────────┘│
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Event Types                                              ││
│  │ - task.started    (task_id, user, project)               ││
│  │ - task.phase      (task_id, phase, status)               ││
│  │ - task.completed  (task_id, result)                      ││
│  │ - task.blocked    (task_id, reason)                      ││
│  │ - presence.update (user, status)                         ││
│  │ - resource.updated(type, name, version)                  ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

### Event Flow

```
Developer A starts task:
  Local orc → task.started event → Team Server
                                        ↓
                              Broadcast to subscribers
                                        ↓
                              Developer B's UI updates
```

### Presence Tracking

```go
type Presence struct {
    UserID    string    `json:"user_id"`
    Status    string    `json:"status"`    // online, away, offline
    ActiveOn  string    `json:"active_on"` // task_id or "idle"
    LastSeen  time.Time `json:"last_seen"`
}
```

---

## Task Locking

### Problem
Two developers shouldn't run the same task simultaneously.

### Solution: Advisory Locks

```go
type TaskLock struct {
    TaskID    string    `json:"task_id"`
    Owner     string    `json:"owner"`     // user_id
    Machine   string    `json:"machine"`   // hostname
    Acquired  time.Time `json:"acquired"`
    Heartbeat time.Time `json:"heartbeat"`
    TTL       Duration  `json:"ttl"`       // 60s default
}
```

### Lock Flow

```
1. orc run TASK-001
2. Check .orc/tasks/TASK-001/lock.yaml (or server)
3. If locked by another:
   - Show: "Task locked by alice@laptop (running for 5m)"
   - Options: [Wait] [Force unlock] [Cancel]
4. If unlocked or stale (heartbeat > TTL):
   - Acquire lock
   - Start heartbeat goroutine
   - Execute task
5. On completion/exit:
   - Release lock
```

### Tier-Specific Implementation

| Tier | Lock Storage | Coordination |
|------|--------------|--------------|
| Solo | `.orc/tasks/<id>/lock.yaml` | Local file |
| P2P | `.orc/tasks/<id>/lock.yaml` + git push | Git-based |
| Team | Server-side lock table | WebSocket + TTL |

---

## Resource Sharing

### Prompt Resolution Chain

```
Request: Get prompt for "implement" phase

1. Check personal override:    ~/.orc/prompts/implement.md
2. Check project local:        .orc/local/prompts/implement.md
3. Check project shared:       .orc/shared/prompts/implement.md
4. Check org resources:        (server, if Tier 3)
5. Use builtin:               templates/prompts/implement.md
```

### Skill Resolution Chain

```
Request: Get skill "python-style"

1. Check personal:             ~/.orc/skills/python-style/
2. Check project local:        .orc/local/skills/python-style/
3. Check project shared:       .orc/shared/skills/python-style/
4. Check Claude built-in:      ~/.claude/skills/python-style/
```

### Version Tracking for Shared Resources

```yaml
# .orc/shared/prompts/implement.md
---
version: 3
updated_by: alice
updated_at: 2026-01-10T12:00:00Z
---
# Implementation Phase

[prompt content]
```

---

## Cost Tracking

### Individual Cost Visibility (All Tiers)

Every user ALWAYS sees their own costs:

```
$ orc cost
Today:     $2.45 (12,340 tokens)
This week: $18.72 (94,230 tokens)
By task:
  TASK-001  $1.23  implement (completed)
  TASK-002  $1.22  test (running)
```

### Team Cost Aggregation (Tier 3)

Server aggregates for visibility, not billing:

```
Team costs (this week):
  alice    $42.30  (15 tasks)
  bob      $31.20  (12 tasks)
  charlie  $28.50  (10 tasks)
  ─────────────────────────
  Total:   $102.00 (37 tasks)
```

### Budget Caps (Optional)

```yaml
# .orc/shared/config.yaml
cost:
  daily_warning: 10.00    # Warn at $10/day
  daily_limit: 50.00      # Block at $50/day (soft - user can override)
  monthly_limit: 500.00   # Hard limit (Tier 3 only)
```

### Implementation

```go
type CostTracker struct {
    db     *sql.DB
    userID string
}

func (t *CostTracker) RecordUsage(taskID string, tokens TokenUsage, model string) {
    cost := calculateCost(tokens, model)
    t.db.Exec(`INSERT INTO cost_log (user_id, task_id, tokens, cost, model, timestamp)
               VALUES (?, ?, ?, ?, ?, ?)`, t.userID, taskID, tokens.Total, cost, model, time.Now())
}

func (t *CostTracker) CheckBudget() (Budget, error) {
    // Check personal daily/weekly limits
    // Return warning or error if exceeded
}
```

---

## Authentication

### Tier 1 (Solo): None Required

```
orc serve → localhost:8080 (no auth)
```

### Tier 2 (P2P): Optional Bearer Token

```yaml
# ~/.orc/config.yaml
server:
  auth:
    type: token
    token: ${ORC_AUTH_TOKEN}  # env var
```

```bash
curl -H "Authorization: Bearer $ORC_AUTH_TOKEN" http://localhost:8080/api/tasks
```

### Tier 3 (Team Server): OIDC/SAML

```yaml
# Server config
auth:
  provider: oidc
  issuer: https://accounts.google.com
  client_id: ${OIDC_CLIENT_ID}
  client_secret: ${OIDC_CLIENT_SECRET}
  allowed_domains: ["company.com"]
```

---

## Security Model

### Secrets Handling

| Secret Type | Storage | Access |
|-------------|---------|--------|
| OAuth tokens | `~/.orc/token-pool/` (0600) | Local only |
| API tokens | Environment variable | Never in config files |
| OIDC secrets | Server env vars | Never logged |

### Audit Logging (Tier 3)

```go
type AuditEvent struct {
    Timestamp time.Time `json:"timestamp"`
    UserID    string    `json:"user_id"`
    Action    string    `json:"action"`
    Resource  string    `json:"resource"`
    Details   any       `json:"details"`
    IP        string    `json:"ip"`
}

// Actions: task.create, task.run, task.delete, member.invite, config.update
```

### Rate Limiting

```go
type RateLimiter struct {
    limits map[string]RateLimit
}

var DefaultLimits = map[string]RateLimit{
    "task.create": {Requests: 100, Window: time.Hour},
    "task.run":    {Requests: 50, Window: time.Hour},
    "api.read":    {Requests: 1000, Window: time.Minute},
}
```

---

## Migration Path

### Phase 1: Solo → P2P Ready

1. Add `.orc/shared/` directory structure
2. Add `.orc/local/` for gitignored overrides
3. Implement resource resolution chain
4. Add task ID prefixing option
5. Add file-based lock mechanism

### Phase 2: P2P → Team Server Ready

1. Add database abstraction layer (SQLite/Postgres)
2. Add user/org models
3. Add API authentication middleware
4. Add WebSocket presence/events
5. Add cost aggregation endpoints

### Phase 3: Team Server

1. Server deployment configuration
2. OIDC integration
3. Audit logging
4. Admin dashboard
5. Resource versioning and sync

---

## File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/auth/` | Authentication middleware |
| `internal/user/` | User preferences, identity |
| `internal/org/` | Organization management |
| `internal/sharing/` | Resource sharing logic |
| `internal/lock/` | Task locking |
| `internal/presence/` | Presence tracking |
| `internal/cost/` | Cost tracking/budgets |

### Modified Files

| File | Changes |
|------|---------|
| `internal/db/` | Add Postgres support, abstraction layer |
| `internal/config/` | Add resolution chain, user prefs |
| `internal/api/` | Add auth middleware, new endpoints |
| `internal/prompt/` | Add resolution chain (personal → shared) |
| `internal/task/` | Add ID prefixing, locking |

### New Directories

```
.orc/shared/        # Git-tracked team resources
.orc/local/         # Gitignored personal overrides
~/.orc/prompts/     # Personal prompt overrides
~/.orc/skills/      # Personal skills
~/.orc/ui.yaml      # UI preferences
```

---

## Success Criteria

### Solo Developer (Must Not Regress)

- [ ] `orc init && orc new "task" && orc run TASK-001` works unchanged
- [ ] No new required configuration
- [ ] No server dependency
- [ ] SQLite remains default
- [ ] Performance unchanged

### P2P Collaboration

- [ ] Multiple devs can work on same project
- [ ] Task IDs don't conflict
- [ ] Shared prompts via git
- [ ] Personal overrides work
- [ ] Lock prevents concurrent execution

### Team Server

- [ ] Single container deployment
- [ ] OIDC authentication
- [ ] Team visibility dashboard
- [ ] Cost tracking aggregation
- [ ] Resource sharing/versioning
