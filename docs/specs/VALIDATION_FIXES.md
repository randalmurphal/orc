# Validation Fixes

> Addressing critical issues identified during spec validation.

## Executive Summary

Five validation agents reviewed the specs from different perspectives (Developer, DevOps, Security, UX, Architecture). This document consolidates their findings and provides specific fixes.

### Critical Issues

| Issue | Severity | Spec | Fix |
|-------|----------|------|-----|
| OAuth tokens in plain YAML | CRITICAL | AUTH_PERMISSIONS | Use OS keychain |
| TLS not enforced | HIGH | AUTH_PERMISSIONS | Require TLS for non-localhost |
| 8-level config hierarchy | HIGH | CONFIG_HIERARCHY | Reduce to 4 levels |
| Missing CSRF protection | HIGH | AUTH_PERMISSIONS | Add CSRF tokens |
| Solo mode lock overhead | MEDIUM | P2P_COORDINATION | Add mode guard |
| Conflicting storage specs | MEDIUM | Multiple | Clarify roles |

---

## Fix 1: Secure Token Storage (CRITICAL)

### Problem

OAuth tokens stored in plain YAML at `~/.orc/token-pool/pool.yaml` is a security vulnerability.

### Solution

Use OS keychain for token storage with encrypted file fallback.

```go
// internal/tokenpool/storage.go

// Storage interface for token persistence
type Storage interface {
    Save(accountID string, token *Token) error
    Load(accountID string) (*Token, error)
    Delete(accountID string) error
    List() ([]string, error)
}

// KeychainStorage uses OS-native secure storage
type KeychainStorage struct {
    service string // "orc-token-pool"
}

func (k *KeychainStorage) Save(accountID string, token *Token) error {
    data, _ := json.Marshal(token)
    return keyring.Set(k.service, accountID, string(data))
}

func (k *KeychainStorage) Load(accountID string) (*Token, error) {
    data, err := keyring.Get(k.service, accountID)
    if err != nil {
        return nil, err
    }
    var token Token
    return &token, json.Unmarshal([]byte(data), &token)
}

// EncryptedFileStorage fallback when keychain unavailable
type EncryptedFileStorage struct {
    path   string
    key    []byte // derived from user passphrase
}

func (e *EncryptedFileStorage) Save(accountID string, token *Token) error {
    data, _ := json.Marshal(token)
    encrypted := encrypt(data, e.key) // AES-256-GCM
    return os.WriteFile(e.tokenPath(accountID), encrypted, 0600)
}
```

### Pool Config Changes

```yaml
# ~/.orc/token-pool/pool.yaml (NEW format - no tokens in file)
version: 2
strategy: round-robin
accounts:
  - id: personal
    name: "Personal Max"
    enabled: true
    # Tokens stored in OS keychain, not here
  - id: work
    name: "Work Account"
    enabled: true
```

### Migration

```bash
# One-time migration for existing users
orc pool migrate

# Output:
# Migrating token pool to secure storage...
# Found 2 accounts in pool.yaml
# Migrated 'personal' to system keychain
# Migrated 'work' to system keychain
# Removed tokens from pool.yaml
# Done. Old tokens backed up to ~/.orc/token-pool/pool.yaml.bak
```

---

## Fix 2: Simplified Config Hierarchy

### Problem

8 levels creates cognitive overload and debugging nightmares.

### Solution

Reduce to 4 conceptual levels with clear purposes.

```
BEFORE (8 levels):
env → flags → user → local → shared → project → system → builtin

AFTER (4 levels):
┌─────────────────────────────────────┐
│ 1. Runtime Overrides                │  env vars, CLI flags
│    (temporary, not persisted)       │
├─────────────────────────────────────┤
│ 2. Personal                         │  ~/.orc/config.yaml
│    (user's machine-wide defaults)   │  .orc/local/config.yaml
├─────────────────────────────────────┤
│ 3. Shared                           │  .orc/shared/config.yaml
│    (team defaults, git-tracked)     │  .orc/config.yaml
├─────────────────────────────────────┤
│ 4. Defaults                         │  Built-in code defaults
│    (fallback values)                │
└─────────────────────────────────────┘
```

### Implementation

```go
// internal/config/loader.go
type ConfigLevel int

const (
    LevelDefaults ConfigLevel = iota
    LevelShared   // .orc/shared/ + .orc/config.yaml
    LevelPersonal // ~/.orc/ + .orc/local/
    LevelRuntime  // env + flags
)

type Loader struct{}

func (l *Loader) Load() (*Config, error) {
    // 1. Start with defaults
    cfg := DefaultConfig()

    // 2. Layer shared configs (team + project)
    cfg = merge(cfg, l.loadShared())

    // 3. Layer personal configs (user global + project local)
    cfg = merge(cfg, l.loadPersonal())

    // 4. Apply runtime overrides
    cfg = merge(cfg, l.loadRuntime())

    return cfg, nil
}

func (l *Loader) loadShared() *Config {
    cfg := &Config{}
    // .orc/config.yaml (project defaults)
    merge(cfg, loadYAML(".orc/config.yaml"))
    // .orc/shared/config.yaml (team defaults - wins over project)
    merge(cfg, loadYAML(".orc/shared/config.yaml"))
    return cfg
}

func (l *Loader) loadPersonal() *Config {
    cfg := &Config{}
    // ~/.orc/config.yaml (user global)
    merge(cfg, loadYAML(userConfigPath()))
    // .orc/local/config.yaml (user project-specific - wins)
    merge(cfg, loadYAML(".orc/local/config.yaml"))
    return cfg
}
```

### Removed Levels

| Removed | Reason | Migration |
|---------|--------|-----------|
| `/etc/orc/config.yaml` | Rarely used, admin can use env vars | Use ORC_* env vars |
| Separate `project` vs `shared` | Confusing distinction | Both are "shared" level |

### CLI Changes

```bash
# Show simplified view
$ orc config show --source
profile = safe (personal: ~/.orc/config.yaml)
model = claude-sonnet (personal: ~/.orc/config.yaml)
gates.default = auto (shared: .orc/shared/config.yaml)
timeout = 10m (default)

# Show full resolution (debugging)
$ orc config show --verbose
profile:
  default: auto
  shared (.orc/shared/config.yaml): safe
  personal (~/.orc/config.yaml): safe  ← WINNER
```

---

## Fix 3: TLS Enforcement

### Problem

No requirement for TLS on non-localhost connections.

### Solution

Require TLS for any non-localhost binding.

```go
// internal/api/server.go
func (s *Server) Start() error {
    cfg := s.config.Server

    // Localhost binding: TLS optional
    if cfg.Host == "127.0.0.1" || cfg.Host == "localhost" {
        return s.startHTTP()
    }

    // Network binding: TLS required
    if cfg.TLS.CertFile == "" || cfg.TLS.KeyFile == "" {
        return fmt.Errorf(
            "TLS required for non-localhost binding (%s)\n"+
            "Set server.tls.cert_file and server.tls.key_file in config, "+
            "or use a reverse proxy like Caddy for auto-TLS",
            cfg.Host,
        )
    }

    return s.startHTTPS()
}
```

### Configuration

```yaml
# .orc/config.yaml
server:
  host: 0.0.0.0
  port: 8443
  tls:
    cert_file: /path/to/cert.pem
    key_file: /path/to/key.pem
    # Or use auto-TLS with ACME (team server)
    acme:
      enabled: true
      domain: orc.company.com
      email: admin@company.com
```

### Recommendation

Use Caddy as reverse proxy for production - handles TLS automatically:

```
# Caddyfile
orc.company.com {
    reverse_proxy localhost:8080
}
```

---

## Fix 4: CSRF Protection

### Problem

No CSRF protection for state-changing requests.

### Solution

Add CSRF tokens for web UI forms.

```go
// internal/api/middleware/csrf.go
func CSRFMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Skip for API endpoints with Authorization header
        if r.Header.Get("Authorization") != "" {
            next.ServeHTTP(w, r)
            return
        }

        // Skip safe methods
        if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
            // Generate and set token for response
            token := generateCSRFToken()
            http.SetCookie(w, &http.Cookie{
                Name:     "csrf_token",
                Value:    token,
                HttpOnly: true,
                Secure:   true,
                SameSite: http.SameSiteStrictMode,
                Path:     "/",
            })
            next.ServeHTTP(w, r)
            return
        }

        // Validate token for state-changing requests
        cookie, err := r.Cookie("csrf_token")
        if err != nil {
            http.Error(w, "CSRF token required", http.StatusForbidden)
            return
        }

        header := r.Header.Get("X-CSRF-Token")
        if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
            http.Error(w, "CSRF token invalid", http.StatusForbidden)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

### Client Integration

```typescript
// web/src/lib/api.ts
async function apiRequest(method: string, path: string, body?: any) {
    const headers: Record<string, string> = {
        'Content-Type': 'application/json',
    };

    // Add CSRF token for mutations
    if (method !== 'GET') {
        const csrfToken = getCookie('csrf_token');
        if (csrfToken) {
            headers['X-CSRF-Token'] = csrfToken;
        }
    }

    return fetch(`/api${path}`, { method, headers, body: JSON.stringify(body) });
}
```

---

## Fix 5: Mode Guard for Solo

### Problem

Lock files checked even in solo mode, adding unnecessary overhead.

### Solution

Skip team features when `mode: solo`.

```go
// internal/executor/executor.go
func (e *Executor) Run(taskID string) error {
    mode := e.config.TaskID.Mode

    // Solo mode: skip all team coordination
    if mode == "solo" {
        return e.runLocal(taskID)
    }

    // P2P mode: file-based locking
    if mode == "p2p" {
        if err := e.acquireFileLock(taskID); err != nil {
            return err
        }
        defer e.releaseFileLock(taskID)
        return e.runLocal(taskID)
    }

    // Team mode: server-based locking + sync
    if mode == "team" {
        if err := e.acquireServerLock(taskID); err != nil {
            return err
        }
        defer e.releaseServerLock(taskID)
        return e.runWithSync(taskID)
    }

    return fmt.Errorf("unknown mode: %s", mode)
}
```

### Default Mode Detection

```go
// internal/config/mode.go
func DetectMode(projectPath string) string {
    // Check for team server config
    if cfg := loadConfig(projectPath); cfg.Team.ServerURL != "" {
        return "team"
    }

    // Check for shared directory
    if exists(filepath.Join(projectPath, ".orc", "shared")) {
        return "p2p"
    }

    // Default: solo
    return "solo"
}
```

---

## Fix 6: Resolve Storage Conflicts

### Problem

Specs are unclear about database vs YAML file roles.

### Solution

Clarify distinct purposes.

```
┌─────────────────────────────────────────────────────────────────┐
│                        Storage Roles                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  YAML Files (git-tracked, human-editable)                        │
│  ├── .orc/tasks/TASK-001/task.yaml    Task definition           │
│  ├── .orc/tasks/TASK-001/plan.yaml    Phase sequence            │
│  ├── .orc/tasks/TASK-001/state.yaml   Execution state           │
│  ├── .orc/shared/config.yaml          Team configuration        │
│  └── .orc/shared/prompts/*.md         Shared prompts            │
│                                                                  │
│  SQLite Database (local index, NOT source of truth)             │
│  ├── tasks table                      Index for search/list     │
│  ├── cost_log table                   Token usage tracking      │
│  ├── transcripts_fts                  Full-text search          │
│  └── projects table                   Project registry          │
│                                                                  │
│  Postgres (team server, aggregation)                            │
│  ├── organizations                    Org management            │
│  ├── members                          User membership           │
│  ├── task_visibility                  Read-only task mirror     │
│  ├── cost_aggregation                 Team cost rollups         │
│  └── audit_log                        Security audit trail      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘

Key Principle: YAML files are the source of truth for task data.
Database is derived/cached data for performance.
```

### Sync Pattern

```go
// internal/task/repo.go

// Save writes to YAML and updates DB index
func (r *Repo) Save(task *Task) error {
    // 1. Write YAML (source of truth)
    if err := r.writeYAML(task); err != nil {
        return err
    }

    // 2. Update DB index (for search)
    return r.updateIndex(task)
}

// Load reads from YAML, falls back to DB for list
func (r *Repo) Get(id string) (*Task, error) {
    return r.readYAML(id)
}

func (r *Repo) List() ([]*Task, error) {
    // Use DB index for fast listing
    return r.db.ListTasks()
}

// Rebuild regenerates DB index from YAML files
func (r *Repo) RebuildIndex() error {
    tasks, err := r.scanYAMLFiles()
    if err != nil {
        return err
    }
    return r.db.ReplaceIndex(tasks)
}
```

---

## Fix 7: Simplified Execution Model (No Cross-User Locking)

### Problem

Original design had complex locking (TTL, heartbeat, file-based, server-based). This was over-engineered.

### Solution

**Remove cross-user locking entirely.** Each execution is independent:

- Anyone can run any task they have access to
- Multiple users running same task = separate branches/worktrees
- Only protection: PID guard prevents same user running twice

```
┌─────────────────────────────────────────────────────────────────┐
│                    Execution Model (Simplified)                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ALL MODES: WORKTREE ISOLATION                                  │
│  - Each execution: own worktree + branch                         │
│  - Branch naming: orc/{taskID}-{executorPrefix}                  │
│  - No blocking between users                                     │
│                                                                  │
│  SAME-USER PROTECTION ONLY                                      │
│  - PID file in worktree                                          │
│  - Prevents accidental double-run                                │
│  - Stale PID auto-cleaned                                        │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation

```go
// internal/executor/pid_guard.go
type PIDGuard struct {
    worktreePath string
}

func (g *PIDGuard) Check() error {
    pidFile := filepath.Join(g.worktreePath, ".orc.pid")
    data, err := os.ReadFile(pidFile)
    if err != nil {
        return nil // No PID, good to go
    }

    pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
    if processExists(pid) {
        return fmt.Errorf("already running (pid %d)", pid)
    }
    os.Remove(pidFile) // Stale
    return nil
}

func (g *PIDGuard) Acquire() error {
    return os.WriteFile(
        filepath.Join(g.worktreePath, ".orc.pid"),
        []byte(strconv.Itoa(os.Getpid())),
        0644,
    )
}
```

### Branch/Worktree Naming

```go
// Includes executor identity to prevent conflicts
func BranchName(taskID, prefix string) string {
    if prefix == "" {
        return "orc/" + taskID  // Solo mode
    }
    return fmt.Sprintf("orc/%s-%s", taskID, strings.ToLower(prefix))
}
```

---

## Fix 8: WebSocket Hub Bounds

### Problem

WebSocket hub has no connection limits, could be DoS'd.

### Solution

Add connection limits and rate limiting.

```go
// internal/api/ws/hub.go
type Hub struct {
    mu          sync.RWMutex
    connections map[string]*Connection

    // Limits
    maxConnections    int           // 1000 default
    maxPerUser        int           // 5 default
    maxSubscriptions  int           // 100 per connection
    messageRateLimit  rate.Limiter  // 10 msg/sec per connection
}

func (h *Hub) Register(conn *Connection) error {
    h.mu.Lock()
    defer h.mu.Unlock()

    // Check global limit
    if len(h.connections) >= h.maxConnections {
        return ErrTooManyConnections
    }

    // Check per-user limit
    userConns := 0
    for _, c := range h.connections {
        if c.UserID == conn.UserID {
            userConns++
        }
    }
    if userConns >= h.maxPerUser {
        return ErrTooManyUserConnections
    }

    h.connections[conn.ID] = conn
    return nil
}

func (c *Connection) handleMessage(msg ClientMessage) {
    // Rate limit
    if !c.rateLimiter.Allow() {
        c.sendError(msg.ID, "RATE_LIMITED", "Too many messages")
        return
    }

    // Subscription limit
    if msg.Type == "subscribe" && len(c.subscriptions) >= c.hub.maxSubscriptions {
        c.sendError(msg.ID, "SUB_LIMIT", "Too many subscriptions")
        return
    }

    // ... handle message
}
```

---

## Fix 9: Orphaned Worktree Cleanup

### Problem

If execution crashes, worktree is left behind with stale PID.

### Solution

Detect stale PID and prompt for cleanup/resume.

```go
// internal/executor/executor.go
func (e *Executor) Run(taskID string) error {
    worktreePath := e.worktreePath(taskID)

    if exists(worktreePath) {
        guard := &PIDGuard{worktreePath: worktreePath}
        if err := guard.Check(); err != nil {
            return err // Actually running
        }

        // Worktree exists but no active process
        fmt.Printf("Worktree exists at %s\n", worktreePath)
        fmt.Println("No active process found (likely crashed).\n")
        fmt.Println("Options:")
        fmt.Println("  [1] Resume from last checkpoint")
        fmt.Println("  [2] Clean up and restart")
        fmt.Println("  [3] Cancel")

        choice := promptChoice([]string{"1", "2", "3"})
        switch choice {
        case "1":
            return e.resumeInWorktree(taskID, worktreePath)
        case "2":
            e.cleanupWorktree(worktreePath)
            // Fall through to create new
        case "3":
            return nil
        }
    }

    return e.createAndRun(taskID)
}
```

### CLI UX

```bash
$ orc run TASK-AM-001
Worktree exists at .orc/worktrees/TASK-AM-001-am/
No active process found (likely crashed).

Options:
  [1] Resume from last checkpoint
  [2] Clean up and restart
  [3] Cancel

Choice: _
```

---

## Fix 10: Event Type Simplification

### Problem

18 event types is overwhelming.

### Solution

Group into categories with filtering.

```typescript
// Event categories for UI filtering
const eventCategories = {
    task: ['task.created', 'task.started', 'task.completed', 'task.failed'],
    phase: ['task.phase', 'task.iteration'],
    presence: ['presence.online', 'presence.away', 'presence.offline'],
    lock: ['lock.acquired', 'lock.released'],
} as const;

// UI filter presets
const filterPresets = {
    essential: ['task.completed', 'task.failed', 'task.blocked'],
    default: [...eventCategories.task, 'task.phase'],
    verbose: Object.values(eventCategories).flat(),
};
```

### Dashboard UI

```svelte
<script>
    let filter = $state('default');
</script>

<div class="activity-filter">
    <button onclick={() => filter = 'essential'}>Essential</button>
    <button onclick={() => filter = 'default'}>Default</button>
    <button onclick={() => filter = 'verbose'}>All</button>
</div>

<ActivityFeed events={filteredEvents(filter)} />
```

---

## Implementation Priority

### Phase 1: Security (Do First)

1. **Token storage migration** - Critical security fix
2. **TLS enforcement** - Required for network access
3. **CSRF protection** - Web security baseline

### Phase 2: Simplification

4. **Config hierarchy simplification** - Reduces cognitive load
5. **Mode guard for solo** - Removes unnecessary overhead
6. **Event filtering** - Improves UX

### Phase 3: Clarification

7. **Storage role documentation** - Prevents confusion
8. **Lock mechanism documentation** - Clarifies intentional design
9. **WebSocket limits** - Prevents DoS
10. **Force unlock safety** - Prevents accidents

---

## Testing Requirements

Each fix must include:

1. **Unit tests** for new functionality
2. **Integration tests** for cross-component behavior
3. **Migration tests** for existing users
4. **E2E tests** for critical paths (token storage, CSRF)

```go
// Example: Token storage migration test
func TestTokenPoolMigration(t *testing.T) {
    // Setup: Create old-format pool.yaml with tokens
    oldFormat := `
version: 1
accounts:
  - id: personal
    access_token: sk-ant-oat01-xxx
    refresh_token: sk-ant-ort01-xxx
`
    writeFile("~/.orc/token-pool/pool.yaml", oldFormat)

    // Run migration
    err := MigrateTokenPool()
    require.NoError(t, err)

    // Verify: Tokens moved to keychain
    token, err := keyring.Get("orc-token-pool", "personal")
    assert.Contains(t, token, "sk-ant-oat01")

    // Verify: pool.yaml no longer contains tokens
    newFormat := readFile("~/.orc/token-pool/pool.yaml")
    assert.NotContains(t, newFormat, "sk-ant-oat01")
    assert.Contains(t, newFormat, "version: 2")
}
```
