# Real-Time Collaboration Specification

> WebSocket-based coordination for team visibility without blocking local execution.

## Design Principles

1. **Visibility, not control** - Server shows what's happening, doesn't execute tasks
2. **Local execution** - All task execution happens on developer machines
3. **Graceful degradation** - Works offline, syncs when connected
4. **Advisory locking** - Prevent conflicts, not enforce them
5. **Presence is optional** - Team can work without seeing each other

---

## Lock Mechanisms by Mode

The lock mechanism differs by mode - this is intentional design.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Lock Mechanisms by Mode                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  SOLO MODE: NO LOCKING                                          │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ • Single user, single machine                            │    │
│  │ • No coordination needed                                 │    │
│  │ • Zero overhead from lock checking                       │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  P2P MODE: FILE-BASED LOCKING                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ • Lock file: .orc/tasks/TASK-001/lock.yaml (gitignored) │    │
│  │ • TTL-based with heartbeat (60s TTL, 10s heartbeat)     │    │
│  │ • Stale lock detection (heartbeat > TTL)                │    │
│  │ • No server dependency                                   │    │
│  │ • Eventual consistency via git                           │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
│  TEAM MODE: SERVER-SIDE LOCKING                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ • WebSocket-based real-time locks                        │    │
│  │ • Server manages TTL and heartbeat                       │    │
│  │ • Immediate conflict detection                           │    │
│  │ • Falls back to file-based if server unavailable        │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Lock Interface

```go
// internal/lock/lock.go
type Locker interface {
    Acquire(taskID string) error
    Release(taskID string) error
    Heartbeat(taskID string) error
    IsLocked(taskID string) (bool, *LockInfo, error)
}

// Factory creates appropriate locker for mode
func NewLocker(mode string, config *Config) Locker {
    switch mode {
    case "solo":
        return &NoOpLocker{} // No locking, zero overhead
    case "p2p":
        return &FileLocker{dir: ".orc/tasks"}
    case "team":
        return &CompositeLocker{
            primary:  NewWebSocketLocker(config.Team.ServerURL),
            fallback: &FileLocker{dir: ".orc/tasks"},
        }
    default:
        return &NoOpLocker{}
    }
}
```

### Fallback Behavior

Team mode gracefully degrades to file-based locking:

```go
// internal/lock/composite.go
type CompositeLocker struct {
    primary  Locker
    fallback Locker
}

func (c *CompositeLocker) Acquire(taskID string) error {
    err := c.primary.Acquire(taskID)
    if err != nil {
        if isConnectionError(err) {
            log.Warn("server unavailable, using file lock", "task", taskID)
            return c.fallback.Acquire(taskID)
        }
        return err
    }
    return nil
}
```

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  Team Server                                                     │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                    WebSocket Hub                             ││
│  │  ┌─────────────────────────────────────────────────────────┐││
│  │  │ Connections: [alice, bob, charlie]                       │││
│  │  │ Subscriptions:                                           │││
│  │  │   alice → [TASK-AM-001, project:acme]                    │││
│  │  │   bob   → [project:acme]                                 │││
│  │  │   charlie → []                                           │││
│  │  └─────────────────────────────────────────────────────────┘││
│  │  ┌─────────────────────────────────────────────────────────┐││
│  │  │ Presence:                                                │││
│  │  │   alice   → online, active on TASK-AM-001               │││
│  │  │   bob     → online, idle                                 │││
│  │  │   charlie → away (5m ago)                                │││
│  │  └─────────────────────────────────────────────────────────┘││
│  │  ┌─────────────────────────────────────────────────────────┐││
│  │  │ Locks:                                                   │││
│  │  │   TASK-AM-001 → alice (TTL: 55s)                         │││
│  │  └─────────────────────────────────────────────────────────┘││
│  └─────────────────────────────────────────────────────────────┘│
└───────────────────────────────────────────────────────────────┬─┘
                                                                │
        ┌───────────────────────────┬───────────────────────────┤
        │                           │                           │
   ┌────▼────┐                 ┌────▼────┐                 ┌────▼────┐
   │  Alice  │                 │   Bob   │                 │ Charlie │
   │  orc    │                 │   orc   │                 │   orc   │
   │ (local) │                 │ (local) │                 │ (local) │
   └─────────┘                 └─────────┘                 └─────────┘
```

---

## WebSocket Protocol

### Connection

```
wss://orc.company.com/api/ws?token=<jwt>
```

### Message Format

```typescript
interface WSMessage {
    type: string;
    id?: string;        // For request-response correlation
    data?: any;
}

// Client → Server
interface ClientMessage extends WSMessage {
    type: 'subscribe' | 'unsubscribe' | 'presence' | 'lock' | 'unlock' | 'ping';
}

// Server → Client
interface ServerMessage extends WSMessage {
    type: 'subscribed' | 'event' | 'presence' | 'lock_result' | 'error' | 'pong';
}
```

### Client → Server Messages

#### Subscribe to Task

```json
{
    "type": "subscribe",
    "id": "req-1",
    "data": {
        "task_id": "TASK-AM-001"
    }
}
```

#### Subscribe to Project

```json
{
    "type": "subscribe",
    "id": "req-2",
    "data": {
        "project_id": "acme"
    }
}
```

#### Unsubscribe

```json
{
    "type": "unsubscribe",
    "id": "req-3",
    "data": {
        "task_id": "TASK-AM-001"
    }
}
```

#### Update Presence

```json
{
    "type": "presence",
    "data": {
        "status": "online",
        "active_task": "TASK-AM-001"
    }
}
```

#### Acquire Lock

```json
{
    "type": "lock",
    "id": "req-4",
    "data": {
        "task_id": "TASK-AM-001",
        "ttl": 60
    }
}
```

#### Release Lock

```json
{
    "type": "unlock",
    "data": {
        "task_id": "TASK-AM-001"
    }
}
```

#### Ping

```json
{
    "type": "ping"
}
```

### Server → Client Messages

#### Subscription Confirmed

```json
{
    "type": "subscribed",
    "id": "req-1",
    "data": {
        "task_id": "TASK-AM-001",
        "current_state": {
            "status": "running",
            "phase": "implement",
            "iteration": 3
        }
    }
}
```

#### Task Event

```json
{
    "type": "event",
    "data": {
        "event_type": "state",
        "task_id": "TASK-AM-001",
        "timestamp": "2026-01-10T12:00:00Z",
        "payload": {
            "status": "running",
            "phase": "test",
            "iteration": 1
        }
    }
}
```

#### Presence Update

```json
{
    "type": "presence",
    "data": {
        "user_id": "bob",
        "display_name": "Bob Johnson",
        "status": "online",
        "active_task": null
    }
}
```

#### Lock Result

```json
{
    "type": "lock_result",
    "id": "req-4",
    "data": {
        "acquired": true,
        "task_id": "TASK-AM-001",
        "expires_at": "2026-01-10T12:01:00Z"
    }
}
```

```json
{
    "type": "lock_result",
    "id": "req-4",
    "data": {
        "acquired": false,
        "task_id": "TASK-AM-001",
        "owner": "bob",
        "owner_name": "Bob Johnson",
        "expires_at": "2026-01-10T12:01:00Z"
    }
}
```

#### Error

```json
{
    "type": "error",
    "id": "req-5",
    "data": {
        "code": "NOT_FOUND",
        "message": "Task TASK-XX-001 not found"
    }
}
```

#### Pong

```json
{
    "type": "pong"
}
```

---

## Event Types

### Task Events

| Event | Trigger | Payload |
|-------|---------|---------|
| `task.created` | New task created | Task metadata |
| `task.started` | Execution started | Task ID, user |
| `task.phase` | Phase transition | Phase ID, status |
| `task.iteration` | New iteration | Phase, iteration number |
| `task.transcript` | New transcript line | Role, content (truncated) |
| `task.completed` | Task finished | Result, duration |
| `task.failed` | Task failed | Error message |
| `task.blocked` | Task blocked | Reason |
| `task.paused` | Task paused | User who paused |
| `task.resumed` | Task resumed | User who resumed |

### Presence Events

| Event | Trigger | Payload |
|-------|---------|---------|
| `presence.online` | User connects | User info |
| `presence.away` | User idle >5min | User ID |
| `presence.offline` | User disconnects | User ID |
| `presence.active` | User starts task | User ID, task ID |
| `presence.idle` | User stops task | User ID |

### Lock Events

| Event | Trigger | Payload |
|-------|---------|---------|
| `lock.acquired` | Lock obtained | Task ID, user, TTL |
| `lock.released` | Lock released | Task ID, user |
| `lock.expired` | TTL elapsed | Task ID, previous owner |
| `lock.stolen` | Force unlock | Task ID, old owner, new owner |

---

## Server Implementation

### WebSocket Hub

```go
// internal/api/ws/hub.go
type Hub struct {
    mu          sync.RWMutex
    connections map[string]*Connection
    subscriptions map[string]map[string]bool  // taskID -> connectionID -> subscribed
    projectSubs   map[string]map[string]bool  // projectID -> connectionID -> subscribed
    presence    map[string]*Presence
    locks       *LockManager
    broadcast   chan Event
}

func NewHub() *Hub {
    h := &Hub{
        connections:   make(map[string]*Connection),
        subscriptions: make(map[string]map[string]bool),
        projectSubs:   make(map[string]map[string]bool),
        presence:      make(map[string]*Presence),
        locks:         NewLockManager(),
        broadcast:     make(chan Event, 100),
    }
    go h.run()
    return h
}

func (h *Hub) run() {
    for event := range h.broadcast {
        h.mu.RLock()

        // Send to task subscribers
        if subs, ok := h.subscriptions[event.TaskID]; ok {
            for connID := range subs {
                if conn, exists := h.connections[connID]; exists {
                    conn.Send(event)
                }
            }
        }

        // Send to project subscribers
        if event.ProjectID != "" {
            if subs, ok := h.projectSubs[event.ProjectID]; ok {
                for connID := range subs {
                    if conn, exists := h.connections[connID]; exists {
                        conn.Send(event)
                    }
                }
            }
        }

        h.mu.RUnlock()
    }
}
```

### Connection Handler

```go
// internal/api/ws/connection.go
type Connection struct {
    ID     string
    UserID string
    User   *User
    conn   *websocket.Conn
    hub    *Hub
    send   chan []byte
}

func (c *Connection) ReadPump() {
    defer func() {
        c.hub.Unregister(c)
        c.conn.Close()
    }()

    c.conn.SetReadLimit(maxMessageSize)
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })

    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            break
        }

        var msg ClientMessage
        if err := json.Unmarshal(message, &msg); err != nil {
            c.sendError("", "INVALID_JSON", "Invalid message format")
            continue
        }

        c.handleMessage(msg)
    }
}

func (c *Connection) handleMessage(msg ClientMessage) {
    switch msg.Type {
    case "subscribe":
        c.handleSubscribe(msg)
    case "unsubscribe":
        c.handleUnsubscribe(msg)
    case "presence":
        c.handlePresence(msg)
    case "lock":
        c.handleLock(msg)
    case "unlock":
        c.handleUnlock(msg)
    case "ping":
        c.send <- []byte(`{"type":"pong"}`)
    }
}
```

### Lock Manager

```go
// internal/api/ws/locks.go
type LockManager struct {
    mu    sync.RWMutex
    locks map[string]*Lock
}

type Lock struct {
    TaskID    string
    OwnerID   string
    OwnerName string
    ConnID    string
    AcquiredAt time.Time
    ExpiresAt time.Time
}

func (m *LockManager) TryAcquire(taskID, userID, userName, connID string, ttl time.Duration) (*Lock, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Check existing lock
    if existing, ok := m.locks[taskID]; ok {
        if time.Now().Before(existing.ExpiresAt) && existing.OwnerID != userID {
            return existing, ErrLockHeld
        }
        // Lock expired or same owner - can acquire
    }

    lock := &Lock{
        TaskID:     taskID,
        OwnerID:    userID,
        OwnerName:  userName,
        ConnID:     connID,
        AcquiredAt: time.Now(),
        ExpiresAt:  time.Now().Add(ttl),
    }
    m.locks[taskID] = lock
    return lock, nil
}

func (m *LockManager) Release(taskID, userID string) bool {
    m.mu.Lock()
    defer m.mu.Unlock()

    if lock, ok := m.locks[taskID]; ok {
        if lock.OwnerID == userID {
            delete(m.locks, taskID)
            return true
        }
    }
    return false
}

func (m *LockManager) Heartbeat(taskID, connID string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if lock, ok := m.locks[taskID]; ok {
        if lock.ConnID == connID {
            lock.ExpiresAt = time.Now().Add(60 * time.Second)
            return nil
        }
        return ErrNotLockOwner
    }
    return ErrLockNotFound
}

// Background cleanup
func (m *LockManager) CleanupExpired() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        m.mu.Lock()
        now := time.Now()
        for taskID, lock := range m.locks {
            if now.After(lock.ExpiresAt) {
                delete(m.locks, taskID)
                // Broadcast lock expiry event
            }
        }
        m.mu.Unlock()
    }
}
```

---

## Client Implementation

### WebSocket Client

```typescript
// web/src/lib/websocket.ts
export class OrcWebSocket {
    private ws: WebSocket | null = null;
    private reconnectAttempts = 0;
    private maxReconnectAttempts = 5;
    private reconnectDelay = 1000;
    private pingInterval: number | null = null;
    private handlers: Map<string, Set<(event: ServerMessage) => void>> = new Map();
    private pendingRequests: Map<string, { resolve: Function; reject: Function }> = new Map();

    constructor(private url: string, private token: string) {}

    connect(): Promise<void> {
        return new Promise((resolve, reject) => {
            this.ws = new WebSocket(`${this.url}?token=${this.token}`);

            this.ws.onopen = () => {
                this.reconnectAttempts = 0;
                this.startPing();
                resolve();
            };

            this.ws.onmessage = (event) => {
                const msg: ServerMessage = JSON.parse(event.data);
                this.handleMessage(msg);
            };

            this.ws.onclose = () => {
                this.stopPing();
                this.reconnect();
            };

            this.ws.onerror = (error) => {
                reject(error);
            };
        });
    }

    private handleMessage(msg: ServerMessage) {
        // Handle request-response correlation
        if (msg.id && this.pendingRequests.has(msg.id)) {
            const { resolve, reject } = this.pendingRequests.get(msg.id)!;
            this.pendingRequests.delete(msg.id);

            if (msg.type === 'error') {
                reject(new Error(msg.data.message));
            } else {
                resolve(msg.data);
            }
            return;
        }

        // Broadcast to handlers
        const eventType = msg.type === 'event' ? msg.data.event_type : msg.type;
        const handlers = this.handlers.get(eventType) || new Set();
        handlers.forEach(handler => handler(msg));

        // Also broadcast to 'all' handlers
        const allHandlers = this.handlers.get('all') || new Set();
        allHandlers.forEach(handler => handler(msg));
    }

    async subscribe(taskId: string): Promise<any> {
        return this.request('subscribe', { task_id: taskId });
    }

    async subscribeProject(projectId: string): Promise<any> {
        return this.request('subscribe', { project_id: projectId });
    }

    async unsubscribe(taskId: string): Promise<void> {
        return this.request('unsubscribe', { task_id: taskId });
    }

    async acquireLock(taskId: string, ttl = 60): Promise<LockResult> {
        return this.request('lock', { task_id: taskId, ttl });
    }

    async releaseLock(taskId: string): Promise<void> {
        return this.request('unlock', { task_id: taskId });
    }

    updatePresence(status: string, activeTask?: string) {
        this.send({
            type: 'presence',
            data: { status, active_task: activeTask }
        });
    }

    on(eventType: string, handler: (event: ServerMessage) => void) {
        if (!this.handlers.has(eventType)) {
            this.handlers.set(eventType, new Set());
        }
        this.handlers.get(eventType)!.add(handler);
        return () => this.handlers.get(eventType)!.delete(handler);
    }

    private request(type: string, data: any): Promise<any> {
        return new Promise((resolve, reject) => {
            const id = `req-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
            this.pendingRequests.set(id, { resolve, reject });

            this.send({ type, id, data });

            // Timeout after 30 seconds
            setTimeout(() => {
                if (this.pendingRequests.has(id)) {
                    this.pendingRequests.delete(id);
                    reject(new Error('Request timeout'));
                }
            }, 30000);
        });
    }

    private send(msg: any) {
        if (this.ws?.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(msg));
        }
    }

    private startPing() {
        this.pingInterval = window.setInterval(() => {
            this.send({ type: 'ping' });
        }, 30000);
    }

    private stopPing() {
        if (this.pingInterval) {
            clearInterval(this.pingInterval);
            this.pingInterval = null;
        }
    }

    private reconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('Max reconnect attempts reached');
            return;
        }

        this.reconnectAttempts++;
        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);

        setTimeout(() => {
            console.log(`Reconnecting (attempt ${this.reconnectAttempts})...`);
            this.connect();
        }, delay);
    }
}
```

---

## Local Executor Integration

### Event Publishing to Server

```go
// internal/executor/publish_server.go
type ServerPublisher struct {
    serverURL string
    token     string
    client    *http.Client
    ws        *websocket.Conn
    taskID    string
}

func (p *ServerPublisher) Start(taskID string) error {
    // Connect to server WebSocket
    u, _ := url.Parse(p.serverURL)
    u.Scheme = "wss"
    u.Path = "/api/ws"
    u.RawQuery = "token=" + p.token

    conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        return fmt.Errorf("connect to server: %w", err)
    }

    p.ws = conn
    p.taskID = taskID

    // Acquire lock
    resp, err := p.sendRequest("lock", map[string]any{
        "task_id": taskID,
        "ttl":     60,
    })
    if err != nil {
        conn.Close()
        return fmt.Errorf("acquire lock: %w", err)
    }

    if !resp["acquired"].(bool) {
        conn.Close()
        return fmt.Errorf("task locked by %s", resp["owner_name"])
    }

    // Start heartbeat
    go p.heartbeatLoop()

    return nil
}

func (p *ServerPublisher) heartbeatLoop() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        p.send(map[string]any{
            "type": "lock",
            "data": map[string]any{
                "task_id": p.taskID,
                "ttl":     60,
            },
        })
    }
}

func (p *ServerPublisher) PublishState(state *State) {
    p.send(map[string]any{
        "type": "event",
        "data": map[string]any{
            "event_type": "task.state",
            "task_id":    p.taskID,
            "payload": map[string]any{
                "status":    state.Status,
                "phase":     state.CurrentPhase,
                "iteration": state.CurrentIteration,
            },
        },
    })
}

func (p *ServerPublisher) PublishTranscript(role, content string) {
    // Truncate for real-time display
    if len(content) > 500 {
        content = content[:500] + "..."
    }

    p.send(map[string]any{
        "type": "event",
        "data": map[string]any{
            "event_type": "task.transcript",
            "task_id":    p.taskID,
            "payload": map[string]any{
                "role":    role,
                "content": content,
            },
        },
    })
}

func (p *ServerPublisher) Stop() {
    if p.ws != nil {
        // Release lock
        p.send(map[string]any{
            "type": "unlock",
            "data": map[string]any{"task_id": p.taskID},
        })
        p.ws.Close()
    }
}
```

### Graceful Degradation

```go
// internal/executor/executor.go
func (e *Executor) setupPublishing(taskID string) {
    // Always use local publisher
    e.localPublisher = events.NewMemoryPublisher()

    // Optionally add server publisher
    if e.config.Team.ServerURL != "" {
        serverPub := NewServerPublisher(e.config.Team.ServerURL, e.config.Team.Token)
        if err := serverPub.Start(taskID); err != nil {
            e.logger.Warn("server sync disabled", "error", err)
            // Continue without server - local execution still works
        } else {
            e.serverPublisher = serverPub
        }
    }
}

func (e *Executor) publishEvent(event Event) {
    // Always publish locally
    e.localPublisher.Publish(event)

    // Optionally publish to server (non-blocking)
    if e.serverPublisher != nil {
        go func() {
            if err := e.serverPublisher.Publish(event); err != nil {
                e.logger.Debug("server publish failed", "error", err)
                // Don't block local execution
            }
        }()
    }
}
```

---

## Presence System

### Presence States

| State | Description | Timeout |
|-------|-------------|---------|
| `online` | User connected, responsive | - |
| `active` | User running a task | - |
| `away` | User idle | After 5min no activity |
| `offline` | User disconnected | On disconnect |

### Presence Tracking

```go
// internal/api/ws/presence.go
type PresenceManager struct {
    mu       sync.RWMutex
    presence map[string]*Presence
    hub      *Hub
}

type Presence struct {
    UserID      string    `json:"user_id"`
    DisplayName string    `json:"display_name"`
    Status      string    `json:"status"`
    ActiveTask  string    `json:"active_task,omitempty"`
    LastSeen    time.Time `json:"last_seen"`
    ConnID      string    `json:"-"`
}

func (m *PresenceManager) Update(userID string, status string, activeTask string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    p, ok := m.presence[userID]
    if !ok {
        return
    }

    changed := p.Status != status || p.ActiveTask != activeTask
    p.Status = status
    p.ActiveTask = activeTask
    p.LastSeen = time.Now()

    if changed {
        m.hub.broadcast <- Event{
            Type: "presence",
            Data: p,
        }
    }
}

func (m *PresenceManager) CheckIdle() {
    ticker := time.NewTicker(time.Minute)
    for range ticker.C {
        m.mu.Lock()
        now := time.Now()
        for _, p := range m.presence {
            if p.Status == "online" && now.Sub(p.LastSeen) > 5*time.Minute {
                p.Status = "away"
                m.hub.broadcast <- Event{
                    Type: "presence",
                    Data: p,
                }
            }
        }
        m.mu.Unlock()
    }
}
```

---

## Conflict Resolution

### Lock Conflicts

When attempting to run a locked task:

```
CLI:
$ orc run TASK-AM-001
Error: Task TASK-AM-001 is currently locked

  Locked by: Bob Johnson (bob@company.com)
  Started: 5 minutes ago
  Last activity: 10 seconds ago

Options:
  1. Wait and retry automatically when available
  2. Force unlock (may corrupt task state)
  3. Cancel

Your choice [1/2/3]: _
```

### Config Conflicts

When editing shared resources:

```go
// Version-based optimistic locking
type SharedResource struct {
    ID      string
    Version int
    Content string
}

func (s *ResourceService) Update(ctx context.Context, id string, content string, expectedVersion int) error {
    resource, err := s.Get(ctx, id)
    if err != nil {
        return err
    }

    if resource.Version != expectedVersion {
        return &ConflictError{
            Current:  resource.Version,
            Expected: expectedVersion,
            Message:  "Resource was modified by another user",
        }
    }

    resource.Content = content
    resource.Version++
    return s.Save(ctx, resource)
}
```

UI handles conflict:

```
Dialog:
┌─────────────────────────────────────────────────┐
│ Conflict Detected                                │
│                                                  │
│ This prompt was modified by Bob 2 minutes ago.  │
│                                                  │
│ [View Diff] [Use Theirs] [Use Mine] [Merge]     │
└─────────────────────────────────────────────────┘
```

---

## Dashboard Components

### Team Activity Feed

```svelte
<script lang="ts">
    import { onMount } from 'svelte';
    import { ws } from '$lib/websocket';

    let activities = $state<Activity[]>([]);

    onMount(() => {
        const unsubscribe = ws.on('all', (event) => {
            activities = [
                { ...event.data, timestamp: new Date() },
                ...activities.slice(0, 49)  // Keep last 50
            ];
        });
        return unsubscribe;
    });
</script>

<div class="activity-feed">
    {#each activities as activity}
        <div class="activity-item">
            <span class="user">{activity.user_name}</span>
            <span class="action">{formatAction(activity)}</span>
            <span class="time">{formatTime(activity.timestamp)}</span>
        </div>
    {/each}
</div>
```

### Presence Indicators

```svelte
<script lang="ts">
    import { ws } from '$lib/websocket';

    let teamMembers = $state<Presence[]>([]);

    onMount(() => {
        ws.on('presence', (event) => {
            const idx = teamMembers.findIndex(m => m.user_id === event.data.user_id);
            if (idx >= 0) {
                teamMembers[idx] = event.data;
            } else {
                teamMembers = [...teamMembers, event.data];
            }
        });
    });
</script>

<div class="team-presence">
    {#each teamMembers as member}
        <div class="member" class:active={member.status === 'active'}>
            <span class="indicator {member.status}"></span>
            <span class="name">{member.display_name}</span>
            {#if member.active_task}
                <span class="task">on {member.active_task}</span>
            {/if}
        </div>
    {/each}
</div>

<style>
    .indicator.online { background: var(--status-success); }
    .indicator.active { background: var(--accent-primary); animation: pulse 2s infinite; }
    .indicator.away { background: var(--status-warning); }
    .indicator.offline { background: var(--text-muted); }
</style>
```

---

## Testing

### WebSocket Tests

```go
func TestHubSubscribe(t *testing.T) {
    hub := NewHub()
    conn := newMockConnection("user-1")

    hub.Register(conn)
    hub.Subscribe(conn.ID, "TASK-001")

    // Publish event
    hub.broadcast <- Event{
        Type:   "task.state",
        TaskID: "TASK-001",
        Data:   map[string]any{"status": "running"},
    }

    // Verify connection received event
    select {
    case msg := <-conn.sent:
        assert.Contains(t, string(msg), "running")
    case <-time.After(time.Second):
        t.Fatal("timeout waiting for message")
    }
}

func TestLockManager(t *testing.T) {
    lm := NewLockManager()

    // Alice acquires
    lock, err := lm.TryAcquire("TASK-001", "alice", "Alice", "conn-1", 60*time.Second)
    assert.NoError(t, err)
    assert.Equal(t, "alice", lock.OwnerID)

    // Bob cannot acquire
    _, err = lm.TryAcquire("TASK-001", "bob", "Bob", "conn-2", 60*time.Second)
    assert.Equal(t, ErrLockHeld, err)

    // Alice releases
    released := lm.Release("TASK-001", "alice")
    assert.True(t, released)

    // Bob can now acquire
    lock, err = lm.TryAcquire("TASK-001", "bob", "Bob", "conn-2", 60*time.Second)
    assert.NoError(t, err)
    assert.Equal(t, "bob", lock.OwnerID)
}
```
