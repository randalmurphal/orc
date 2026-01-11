# Authentication & Permissions Specification

> Progressive auth: none for solo, token for remote, OIDC for teams.

## Design Principles

1. **Zero auth by default** - Localhost binding requires no auth
2. **Progressive complexity** - Add auth only when needed
3. **User brings own tokens** - No shared Claude credentials by default
4. **Permissions don't restrict AI usage** - Users control their own Claude
5. **Simple RBAC** - Four roles, no complex hierarchies

---

## Authentication Tiers

### Tier 1: No Auth (Solo on Localhost)

```
┌─────────────────────────────────────────────┐
│  localhost:8080                              │
│  ┌─────────────────────────────────────────┐│
│  │ Server binds to 127.0.0.1 only          ││
│  │ No auth required                         ││
│  │ Full access to all endpoints             ││
│  └─────────────────────────────────────────┘│
└─────────────────────────────────────────────┘
```

**Config:**
```yaml
# .orc/config.yaml (default)
server:
  host: 127.0.0.1     # Localhost only
  port: 8080
  auth:
    enabled: false    # Default: no auth
```

### Tier 2: Bearer Token (Remote Access)

```
┌─────────────────────────────────────────────┐
│  0.0.0.0:8080 (network accessible)           │
│  ┌─────────────────────────────────────────┐│
│  │ Authorization: Bearer <token>            ││
│  │ Token stored in env var                  ││
│  │ Full access with valid token             ││
│  └─────────────────────────────────────────┘│
└─────────────────────────────────────────────┘
```

**Config:**
```yaml
# .orc/config.yaml
server:
  host: 0.0.0.0       # Network accessible
  port: 8080
  auth:
    enabled: true
    type: token
    # Token from env var (never in config file)
```

**Usage:**
```bash
# Set token
export ORC_AUTH_TOKEN="your-secret-token"
orc serve

# API requests
curl -H "Authorization: Bearer $ORC_AUTH_TOKEN" http://server:8080/api/tasks
```

### Tier 3: OIDC (Team Server)

```
┌─────────────────────────────────────────────┐
│  Team Server                                 │
│  ┌─────────────────────────────────────────┐│
│  │ OIDC Provider: Google, GitHub, Okta     ││
│  │ Session cookies for web UI              ││
│  │ JWT tokens for API                       ││
│  │ Role-based access control                ││
│  └─────────────────────────────────────────┘│
└─────────────────────────────────────────────┘
```

**Config:**
```yaml
# Server config
server:
  auth:
    enabled: true
    type: oidc
    oidc:
      issuer: https://accounts.google.com
      client_id: ${OIDC_CLIENT_ID}
      client_secret: ${OIDC_CLIENT_SECRET}
      redirect_url: https://orc.company.com/auth/callback
      scopes: [openid, email, profile]
      allowed_domains: [company.com]   # Optional: restrict to domain
```

---

## Authentication Implementation

### Middleware Chain

```go
// internal/api/middleware/auth.go
func AuthMiddleware(cfg AuthConfig) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Skip auth for health check
            if r.URL.Path == "/api/health" {
                next.ServeHTTP(w, r)
                return
            }

            // Localhost bypass (Tier 1)
            if cfg.IsLocalhostOnly() && isLocalRequest(r) {
                ctx := context.WithValue(r.Context(), userKey, anonymousUser)
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            }

            // Auth required
            if !cfg.Enabled {
                http.Error(w, "auth required for non-localhost", http.StatusUnauthorized)
                return
            }

            user, err := authenticate(r, cfg)
            if err != nil {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }

            ctx := context.WithValue(r.Context(), userKey, user)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func authenticate(r *http.Request, cfg AuthConfig) (*User, error) {
    switch cfg.Type {
    case "token":
        return authenticateToken(r, cfg)
    case "oidc":
        return authenticateOIDC(r, cfg)
    default:
        return nil, fmt.Errorf("unknown auth type: %s", cfg.Type)
    }
}
```

### Token Authentication

```go
// internal/api/auth/token.go
func authenticateToken(r *http.Request, cfg AuthConfig) (*User, error) {
    // Check Authorization header
    auth := r.Header.Get("Authorization")
    if auth == "" {
        return nil, ErrNoToken
    }

    if !strings.HasPrefix(auth, "Bearer ") {
        return nil, ErrInvalidToken
    }

    token := strings.TrimPrefix(auth, "Bearer ")

    // Compare with configured token (constant-time)
    expected := os.Getenv("ORC_AUTH_TOKEN")
    if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
        return nil, ErrInvalidToken
    }

    // Token auth returns a generic authenticated user
    return &User{
        ID:    "token-user",
        Role:  RoleAdmin,  // Token has full access
    }, nil
}
```

### OIDC Authentication

```go
// internal/api/auth/oidc.go
type OIDCAuthenticator struct {
    provider *oidc.Provider
    verifier *oidc.IDTokenVerifier
    oauth    *oauth2.Config
    sessions *SessionStore
}

func NewOIDCAuthenticator(cfg OIDCConfig) (*OIDCAuthenticator, error) {
    ctx := context.Background()

    provider, err := oidc.NewProvider(ctx, cfg.Issuer)
    if err != nil {
        return nil, fmt.Errorf("create oidc provider: %w", err)
    }

    return &OIDCAuthenticator{
        provider: provider,
        verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
        oauth: &oauth2.Config{
            ClientID:     cfg.ClientID,
            ClientSecret: cfg.ClientSecret,
            RedirectURL:  cfg.RedirectURL,
            Endpoint:     provider.Endpoint(),
            Scopes:       cfg.Scopes,
        },
        sessions: NewSessionStore(),
    }, nil
}

func (a *OIDCAuthenticator) Authenticate(r *http.Request) (*User, error) {
    // Check session cookie first
    cookie, err := r.Cookie("orc_session")
    if err == nil {
        if user, ok := a.sessions.Get(cookie.Value); ok {
            return user, nil
        }
    }

    // Check JWT in Authorization header (for API calls)
    auth := r.Header.Get("Authorization")
    if strings.HasPrefix(auth, "Bearer ") {
        token := strings.TrimPrefix(auth, "Bearer ")
        return a.verifyJWT(r.Context(), token)
    }

    return nil, ErrNotAuthenticated
}

func (a *OIDCAuthenticator) verifyJWT(ctx context.Context, rawToken string) (*User, error) {
    idToken, err := a.verifier.Verify(ctx, rawToken)
    if err != nil {
        return nil, fmt.Errorf("verify token: %w", err)
    }

    var claims struct {
        Email string `json:"email"`
        Name  string `json:"name"`
    }
    if err := idToken.Claims(&claims); err != nil {
        return nil, err
    }

    // Look up user in database
    return a.userStore.GetByEmail(ctx, claims.Email)
}
```

### Login Flow (OIDC)

```go
// internal/api/handlers/auth.go
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
    state := generateState()
    h.stateStore.Set(state, time.Now().Add(10*time.Minute))

    url := h.oidc.AuthCodeURL(state)
    http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func (h *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
    // Verify state
    state := r.URL.Query().Get("state")
    if !h.stateStore.Valid(state) {
        http.Error(w, "invalid state", http.StatusBadRequest)
        return
    }

    // Exchange code for token
    code := r.URL.Query().Get("code")
    oauth2Token, err := h.oidc.Exchange(r.Context(), code)
    if err != nil {
        http.Error(w, "exchange failed", http.StatusInternalServerError)
        return
    }

    // Extract ID token
    rawIDToken, ok := oauth2Token.Extra("id_token").(string)
    if !ok {
        http.Error(w, "no id_token", http.StatusInternalServerError)
        return
    }

    idToken, err := h.oidc.Verify(r.Context(), rawIDToken)
    if err != nil {
        http.Error(w, "verify failed", http.StatusUnauthorized)
        return
    }

    // Get or create user
    var claims struct {
        Email string `json:"email"`
        Name  string `json:"name"`
    }
    idToken.Claims(&claims)

    user, err := h.userStore.GetOrCreate(r.Context(), claims.Email, claims.Name)
    if err != nil {
        http.Error(w, "user creation failed", http.StatusInternalServerError)
        return
    }

    // Create session
    sessionID := h.sessions.Create(user)
    http.SetCookie(w, &http.Cookie{
        Name:     "orc_session",
        Value:    sessionID,
        Path:     "/",
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteLaxMode,
        MaxAge:   86400, // 24 hours
    })

    // Redirect to app
    http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}
```

---

## Authorization (RBAC)

### Roles

| Role | Description | Capabilities |
|------|-------------|--------------|
| `owner` | Organization owner | All + billing, delete org |
| `admin` | Administrator | Manage members, settings, all tasks |
| `member` | Regular member | Own tasks, use shared resources |
| `viewer` | Read-only | View tasks, no execution |

### Permissions Matrix

| Permission | Owner | Admin | Member | Viewer |
|------------|-------|-------|--------|--------|
| View all tasks | ✓ | ✓ | Own only | ✓ |
| Create task | ✓ | ✓ | ✓ | ✗ |
| Run own task | ✓ | ✓ | ✓ | ✗ |
| Run any task | ✓ | ✓ | ✗ | ✗ |
| Delete own task | ✓ | ✓ | ✓ | ✗ |
| Delete any task | ✓ | ✓ | ✗ | ✗ |
| Edit shared prompts | ✓ | ✓ | ✗ | ✗ |
| Edit shared skills | ✓ | ✓ | ✗ | ✗ |
| Manage members | ✓ | ✓ | ✗ | ✗ |
| View cost (own) | ✓ | ✓ | ✓ | ✓ |
| View cost (team) | ✓ | ✓ | ✗ | ✗ |
| Edit org settings | ✓ | ✓ | ✗ | ✗ |
| Delete org | ✓ | ✗ | ✗ | ✗ |
| Billing | ✓ | ✗ | ✗ | ✗ |

### Implementation

```go
// internal/auth/rbac.go
type Permission string

const (
    PermViewTasks       Permission = "tasks:view"
    PermCreateTask      Permission = "tasks:create"
    PermRunOwnTask      Permission = "tasks:run:own"
    PermRunAnyTask      Permission = "tasks:run:any"
    PermDeleteOwnTask   Permission = "tasks:delete:own"
    PermDeleteAnyTask   Permission = "tasks:delete:any"
    PermEditPrompts     Permission = "prompts:edit"
    PermEditSkills      Permission = "skills:edit"
    PermManageMembers   Permission = "members:manage"
    PermViewOwnCost     Permission = "cost:view:own"
    PermViewTeamCost    Permission = "cost:view:team"
    PermEditOrgSettings Permission = "org:settings"
    PermDeleteOrg       Permission = "org:delete"
    PermBilling         Permission = "billing"
)

var rolePermissions = map[Role][]Permission{
    RoleOwner: {
        PermViewTasks, PermCreateTask, PermRunOwnTask, PermRunAnyTask,
        PermDeleteOwnTask, PermDeleteAnyTask, PermEditPrompts, PermEditSkills,
        PermManageMembers, PermViewOwnCost, PermViewTeamCost,
        PermEditOrgSettings, PermDeleteOrg, PermBilling,
    },
    RoleAdmin: {
        PermViewTasks, PermCreateTask, PermRunOwnTask, PermRunAnyTask,
        PermDeleteOwnTask, PermDeleteAnyTask, PermEditPrompts, PermEditSkills,
        PermManageMembers, PermViewOwnCost, PermViewTeamCost,
        PermEditOrgSettings,
    },
    RoleMember: {
        PermViewTasks, PermCreateTask, PermRunOwnTask,
        PermDeleteOwnTask, PermViewOwnCost,
    },
    RoleViewer: {
        PermViewTasks, PermViewOwnCost,
    },
}

func HasPermission(user *User, perm Permission) bool {
    perms, ok := rolePermissions[user.Role]
    if !ok {
        return false
    }
    for _, p := range perms {
        if p == perm {
            return true
        }
    }
    return false
}
```

### Authorization Middleware

```go
// internal/api/middleware/authz.go
func RequirePermission(perm Permission) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            user := UserFromContext(r.Context())
            if user == nil {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }

            if !HasPermission(user, perm) {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// Usage in routes
mux.Handle("POST /api/tasks", RequirePermission(PermCreateTask)(createTaskHandler))
mux.Handle("DELETE /api/tasks/{id}", RequirePermission(PermDeleteOwnTask)(deleteTaskHandler))
```

### Resource-Level Authorization

```go
// internal/api/handlers/tasks.go
func (h *TaskHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
    user := UserFromContext(r.Context())
    taskID := r.PathValue("id")

    task, err := h.repo.GetTask(r.Context(), taskID)
    if err != nil {
        http.Error(w, "task not found", http.StatusNotFound)
        return
    }

    // Check ownership or elevated permission
    isOwner := task.CreatedBy == user.ID
    canDeleteAny := HasPermission(user, PermDeleteAnyTask)

    if !isOwner && !canDeleteAny {
        http.Error(w, "forbidden: not task owner", http.StatusForbidden)
        return
    }

    // Proceed with deletion
    // ...
}
```

---

## What Permissions DON'T Control

### Individual AI Settings (Always User-Controlled)

| Setting | Controlled By |
|---------|---------------|
| Claude model | Individual user |
| Max iterations | Individual user |
| Timeout | Individual user |
| Personal OAuth tokens | Individual user |
| Personal prompt overrides | Individual user |
| Cost limits (personal) | Individual user |

**A team admin CANNOT:**
- Force a user to use a specific model
- Force a user to use team OAuth tokens
- Prevent a user from using their own prompts
- Set cost limits on a user's personal API usage

---

## API Token Management

### Personal Access Tokens (PAT)

```go
// internal/auth/pat.go
type PersonalAccessToken struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    Name      string    `json:"name"`
    Token     string    `json:"-"`         // Never returned in API
    TokenHash string    `json:"token_hash"` // SHA256 hash
    Scopes    []string  `json:"scopes"`
    ExpiresAt time.Time `json:"expires_at"`
    CreatedAt time.Time `json:"created_at"`
    LastUsed  time.Time `json:"last_used"`
}

func CreatePAT(userID, name string, scopes []string, ttl time.Duration) (*PersonalAccessToken, string, error) {
    // Generate secure token
    tokenBytes := make([]byte, 32)
    rand.Read(tokenBytes)
    token := base64.URLEncoding.EncodeToString(tokenBytes)

    // Prefix for identification
    fullToken := "orc_pat_" + token

    pat := &PersonalAccessToken{
        ID:        uuid.New().String(),
        UserID:    userID,
        Name:      name,
        TokenHash: sha256Sum(fullToken),
        Scopes:    scopes,
        ExpiresAt: time.Now().Add(ttl),
        CreatedAt: time.Now(),
    }

    return pat, fullToken, nil  // Return token only once
}
```

### Token Scopes

```go
const (
    ScopeTasksRead   = "tasks:read"
    ScopeTasksWrite  = "tasks:write"
    ScopeTasksRun    = "tasks:run"
    ScopeConfigRead  = "config:read"
    ScopeConfigWrite = "config:write"
)

// Default scopes for CLI usage
var DefaultCLIScopes = []string{
    ScopeTasksRead,
    ScopeTasksWrite,
    ScopeTasksRun,
    ScopeConfigRead,
}
```

### Token Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/tokens` | Create new PAT |
| GET | `/api/tokens` | List user's PATs (no secrets) |
| DELETE | `/api/tokens/{id}` | Revoke PAT |

---

## Session Management

### Session Store

```go
// internal/auth/session.go
type SessionStore struct {
    mu       sync.RWMutex
    sessions map[string]*Session
}

type Session struct {
    ID        string
    UserID    string
    User      *User
    CreatedAt time.Time
    ExpiresAt time.Time
    LastSeen  time.Time
}

func (s *SessionStore) Create(user *User) string {
    sessionID := generateSessionID()

    s.mu.Lock()
    defer s.mu.Unlock()

    s.sessions[sessionID] = &Session{
        ID:        sessionID,
        UserID:    user.ID,
        User:      user,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(24 * time.Hour),
        LastSeen:  time.Now(),
    }

    return sessionID
}

func (s *SessionStore) Get(sessionID string) (*User, bool) {
    s.mu.RLock()
    session, ok := s.sessions[sessionID]
    s.mu.RUnlock()

    if !ok || time.Now().After(session.ExpiresAt) {
        return nil, false
    }

    // Update last seen
    s.mu.Lock()
    session.LastSeen = time.Now()
    s.mu.Unlock()

    return session.User, true
}
```

### Session Persistence (Team Mode)

For team servers with multiple instances, use database-backed sessions:

```go
type DBSessionStore struct {
    db *bun.DB
}

func (s *DBSessionStore) Create(user *User) (string, error) {
    session := &DBSession{
        ID:        generateSessionID(),
        UserID:    user.ID,
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }
    _, err := s.db.NewInsert().Model(session).Exec(context.Background())
    return session.ID, err
}
```

---

## Security Considerations

### Token Storage (CRITICAL)

**Never store tokens in plain text files.** Use OS keychain or encrypted storage.

```go
// internal/tokenpool/storage.go

// Storage interface for token persistence
type Storage interface {
    Save(accountID string, token *Token) error
    Load(accountID string) (*Token, error)
    Delete(accountID string) error
    List() ([]string, error)
}

// KeychainStorage uses OS-native secure storage (preferred)
// - macOS: Keychain
// - Linux: libsecret/GNOME Keyring
// - Windows: Credential Manager
type KeychainStorage struct {
    service string // "orc-token-pool"
}

func NewKeychainStorage() *KeychainStorage {
    return &KeychainStorage{service: "orc-token-pool"}
}

func (k *KeychainStorage) Save(accountID string, token *Token) error {
    data, err := json.Marshal(token)
    if err != nil {
        return err
    }
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
    path string
    key  []byte // derived from passphrase via Argon2
}

func (e *EncryptedFileStorage) Save(accountID string, token *Token) error {
    data, _ := json.Marshal(token)

    // Encrypt with AES-256-GCM
    block, _ := aes.NewCipher(e.key)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)
    encrypted := gcm.Seal(nonce, nonce, data, nil)

    return os.WriteFile(e.tokenPath(accountID), encrypted, 0600)
}
```

### Pool Config Format (No Tokens in Config)

```yaml
# ~/.orc/token-pool/pool.yaml (v2 - secure)
version: 2
strategy: round-robin
accounts:
  - id: personal
    name: "Personal Max"
    enabled: true
    # Tokens stored in OS keychain, NOT here
  - id: work
    name: "Work Account"
    enabled: true
```

### PAT Token Hashing

```go
// NEVER store raw PAT tokens
// Always store hashes
func hashToken(token string) string {
    h := sha256.Sum256([]byte(token))
    return hex.EncodeToString(h[:])
}

// Constant-time comparison
func verifyToken(provided, stored string) bool {
    providedHash := hashToken(provided)
    return subtle.ConstantTimeCompare([]byte(providedHash), []byte(stored)) == 1
}
```

### TLS Enforcement

```go
// internal/api/server.go
func (s *Server) Start() error {
    cfg := s.config.Server

    // Localhost binding: TLS optional
    if cfg.Host == "127.0.0.1" || cfg.Host == "localhost" {
        log.Info("starting HTTP server (localhost only)")
        return s.startHTTP()
    }

    // Network binding: TLS REQUIRED
    if cfg.TLS.CertFile == "" || cfg.TLS.KeyFile == "" {
        return fmt.Errorf(
            "TLS required for non-localhost binding (%s)\n"+
            "Options:\n"+
            "  1. Set server.tls.cert_file and server.tls.key_file\n"+
            "  2. Use a reverse proxy like Caddy for auto-TLS\n"+
            "  3. Bind to localhost only (127.0.0.1)",
            cfg.Host,
        )
    }

    log.Info("starting HTTPS server", "host", cfg.Host, "port", cfg.Port)
    return s.startHTTPS()
}
```

### CSRF Protection

```go
// internal/api/middleware/csrf.go
func CSRFMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Skip for API requests with Authorization header (PAT, JWT)
        if r.Header.Get("Authorization") != "" {
            next.ServeHTTP(w, r)
            return
        }

        // Skip for safe methods (GET, HEAD, OPTIONS)
        if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
            // Generate token for forms
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

func generateCSRFToken() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}
```

### Headers

```go
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

        if r.TLS != nil {
            w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        }

        next.ServeHTTP(w, r)
    })
}
```

### Rate Limiting

```go
func rateLimitMiddleware(limiter *rate.Limiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## Audit Logging

### Audit Events

```go
// internal/audit/audit.go
type AuditEvent struct {
    ID        string         `json:"id"`
    Timestamp time.Time      `json:"timestamp"`
    UserID    string         `json:"user_id"`
    Action    string         `json:"action"`
    Resource  string         `json:"resource"`
    ResourceID string        `json:"resource_id"`
    Details   map[string]any `json:"details"`
    IP        string         `json:"ip"`
    UserAgent string         `json:"user_agent"`
}

// Actions
const (
    AuditLogin         = "auth.login"
    AuditLogout        = "auth.logout"
    AuditTaskCreate    = "task.create"
    AuditTaskRun       = "task.run"
    AuditTaskDelete    = "task.delete"
    AuditMemberInvite  = "member.invite"
    AuditMemberRemove  = "member.remove"
    AuditConfigUpdate  = "config.update"
    AuditPromptUpdate  = "prompt.update"
)
```

### Audit Logger

```go
type AuditLogger struct {
    db *bun.DB
}

func (l *AuditLogger) Log(ctx context.Context, event AuditEvent) error {
    _, err := l.db.NewInsert().Model(&event).Exec(ctx)
    return err
}

// Usage
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
    user := UserFromContext(r.Context())
    // ... create task ...

    h.audit.Log(r.Context(), AuditEvent{
        ID:         uuid.New().String(),
        Timestamp:  time.Now(),
        UserID:     user.ID,
        Action:     AuditTaskCreate,
        Resource:   "task",
        ResourceID: task.ID,
        Details:    map[string]any{"title": task.Title, "weight": task.Weight},
        IP:         r.RemoteAddr,
        UserAgent:  r.UserAgent(),
    })
}
```

---

## Configuration Reference

### Solo (No Auth)

```yaml
server:
  host: 127.0.0.1
  port: 8080
  auth:
    enabled: false
```

### Remote Access (Token)

```yaml
server:
  host: 0.0.0.0
  port: 8080
  auth:
    enabled: true
    type: token
```

```bash
export ORC_AUTH_TOKEN="your-secret-token"
```

### Team Server (OIDC)

```yaml
server:
  host: 0.0.0.0
  port: 8080
  auth:
    enabled: true
    type: oidc
    oidc:
      issuer: https://accounts.google.com
      client_id: ${OIDC_CLIENT_ID}
      client_secret: ${OIDC_CLIENT_SECRET}
      redirect_url: https://orc.company.com/auth/callback
      scopes: [openid, email, profile]
      allowed_domains: [company.com]
    session:
      cookie_name: orc_session
      cookie_secure: true
      max_age: 86400
```

---

## Testing

### Auth Bypass for Tests

```go
func TestHandler(t *testing.T) {
    // Create server with auth disabled
    cfg := Config{Auth: AuthConfig{Enabled: false}}
    srv := NewServer(cfg)

    // Or inject test user
    ctx := context.WithValue(context.Background(), userKey, &User{
        ID:   "test-user",
        Role: RoleAdmin,
    })
    req := httptest.NewRequest("GET", "/api/tasks", nil).WithContext(ctx)
}
```

### OIDC Mock

```go
func TestOIDCFlow(t *testing.T) {
    // Use mock OIDC provider
    mockProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Return mock OIDC discovery, tokens, etc.
    }))
    defer mockProvider.Close()

    cfg := OIDCConfig{Issuer: mockProvider.URL}
    // ... test flow
}
```
