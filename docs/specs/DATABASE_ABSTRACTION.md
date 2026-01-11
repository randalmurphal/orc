# Database Abstraction Specification

> SQLite for solo developers, PostgreSQL for teams. Same code, same models.

## Design Goals

1. **Zero-config solo** - SQLite works out of box, no setup
2. **Single switch to Postgres** - Just `DATABASE_URL` env var
3. **Same Go code** - No dialect-specific application logic
4. **Full-text search** - Works on both (FTS5 vs tsvector)
5. **Clean-slate migrations** - No incremental, version check on startup

---

## Storage Roles: YAML vs Database

### Key Principle

**YAML files are the source of truth for task data. Database is derived/cached data for performance.**

```
┌─────────────────────────────────────────────────────────────────┐
│                        Storage Roles                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  YAML Files (git-tracked, human-editable, SOURCE OF TRUTH)      │
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
│  Postgres (team server only, aggregation + visibility)          │
│  ├── organizations                    Org management            │
│  ├── members                          User membership           │
│  ├── task_visibility                  Read-only task mirror     │
│  ├── cost_aggregation                 Team cost rollups         │
│  └── audit_log                        Security audit trail      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Sync Pattern

```go
// internal/task/repo.go

// Save writes to YAML first (source of truth), then updates DB index
func (r *Repo) Save(task *Task) error {
    // 1. Write YAML (source of truth)
    if err := r.writeYAML(task); err != nil {
        return fmt.Errorf("write task yaml: %w", err)
    }

    // 2. Update DB index (for search, can be rebuilt)
    if err := r.updateIndex(task); err != nil {
        log.Warn("failed to update db index", "task", task.ID, "error", err)
        // Don't fail - YAML is saved
    }

    return nil
}

// Get reads from YAML (source of truth)
func (r *Repo) Get(id string) (*Task, error) {
    return r.readYAML(id)
}

// List uses DB index for fast listing
func (r *Repo) List() ([]*Task, error) {
    return r.db.ListTasks()
}

// RebuildIndex regenerates DB from YAML files
func (r *Repo) RebuildIndex() error {
    tasks, err := r.scanYAMLFiles()
    if err != nil {
        return err
    }
    return r.db.ReplaceIndex(tasks)
}
```

### Why This Pattern?

| Concern | YAML | Database |
|---------|------|----------|
| Git tracking | Yes | No |
| Human readable | Yes | No |
| Survives corruption | Yes (text) | Rebuild from YAML |
| Fast search | No | Yes (FTS) |
| Fast listing | Slow (filesystem) | Yes |
| Aggregation | No | Yes |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Application                              │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │                        DB Interface                          ││
│  │  Tasks, Projects, Transcripts, Cost, Users, Orgs            ││
│  └─────────────────────────────────────────────────────────────┘│
└───────────────────────────────────┬─────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Bun ORM Layer                            │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  dialect.SQLite  ◀──────────────▶  dialect.PG               ││
│  └─────────────────────────────────────────────────────────────┘│
└───────────────────────────────────┬─────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
                    ▼               ▼               ▼
             ┌──────────┐   ┌──────────┐   ┌──────────────┐
             │ SQLite   │   │ Postgres │   │ Postgres     │
             │ (local)  │   │ (local)  │   │ (container)  │
             └──────────┘   └──────────┘   └──────────────┘
```

---

## ORM Choice: Bun

### Why Bun?

| Criteria | Bun | GORM | sqlx |
|----------|-----|------|------|
| Multi-dialect | Yes | Yes | Manual |
| Type-safe queries | Yes | Partial | No |
| Performance | Fast | Slower | Fastest |
| Migration support | Yes | Yes | No |
| Raw SQL when needed | Yes | Yes | Yes |
| Learning curve | Medium | Low | Low |

Bun provides the best balance of type safety, multi-dialect support, and raw SQL escape hatches.

### Dependencies

```go
// go.mod additions
require (
    github.com/uptrace/bun v1.2.0
    github.com/uptrace/bun/dialect/sqlitedialect v1.2.0
    github.com/uptrace/bun/dialect/pgdialect v1.2.0
    github.com/uptrace/bun/driver/sqliteshim v1.2.0  // Pure Go SQLite
    github.com/uptrace/bun/driver/pgdriver v1.2.0    // Postgres driver
)
```

---

## Connection Management

### Configuration

```go
// internal/db/config.go
type Config struct {
    Driver   string        // "sqlite" or "postgres"
    DSN      string        // Connection string or file path
    MaxConns int           // Max open connections
    Timeout  time.Duration // Query timeout
}

func ConfigFromEnv() Config {
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        // Default: SQLite in .orc directory
        return Config{
            Driver:   "sqlite",
            DSN:      filepath.Join(orcDir(), "orc.db"),
            MaxConns: 1,  // SQLite single-writer
            Timeout:  30 * time.Second,
        }
    }

    // Parse DATABASE_URL
    if strings.HasPrefix(dsn, "postgres://") {
        return Config{
            Driver:   "postgres",
            DSN:      dsn,
            MaxConns: 10,
            Timeout:  30 * time.Second,
        }
    }

    // Assume SQLite file path
    return Config{
        Driver:   "sqlite",
        DSN:      dsn,
        MaxConns: 1,
        Timeout:  30 * time.Second,
    }
}
```

### Opening Connection

```go
// internal/db/db.go
func Open(cfg Config) (*bun.DB, error) {
    var sqldb *sql.DB
    var dialect schema.Dialect

    switch cfg.Driver {
    case "sqlite":
        sqldb, err := sql.Open("sqlite", cfg.DSN+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
        if err != nil {
            return nil, fmt.Errorf("open sqlite: %w", err)
        }
        sqldb.SetMaxOpenConns(1)  // SQLite is single-writer
        dialect = sqlitedialect.New()

    case "postgres":
        sqldb = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(cfg.DSN)))
        sqldb.SetMaxOpenConns(cfg.MaxConns)
        dialect = pgdialect.New()

    default:
        return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
    }

    db := bun.NewDB(sqldb, dialect)

    // Test connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("ping database: %w", err)
    }

    return db, nil
}
```

---

## Schema Management

### Schema Versioning

```go
// internal/db/migrations.go
const SchemaVersion = 1

type SchemaMeta struct {
    Version   int       `bun:"version,pk"`
    AppliedAt time.Time `bun:"applied_at"`
}

func CheckSchema(db *bun.DB) error {
    var meta SchemaMeta
    err := db.NewSelect().Model(&meta).Limit(1).Scan(context.Background())

    if err == sql.ErrNoRows {
        // Fresh database, apply schema
        return ApplySchema(db, SchemaVersion)
    }

    if meta.Version != SchemaVersion {
        return fmt.Errorf("schema version mismatch: have %d, need %d (clean slate required)", meta.Version, SchemaVersion)
    }

    return nil
}
```

### Clean-Slate Migrations

No incremental migrations. On version mismatch:

```go
func ApplySchema(db *bun.DB, version int) error {
    ctx := context.Background()

    // Use transaction
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Create tables
    models := []interface{}{
        (*Task)(nil),
        (*Phase)(nil),
        (*Transcript)(nil),
        (*Project)(nil),
        (*CostLog)(nil),
        (*User)(nil),
        (*Organization)(nil),
        (*Member)(nil),
    }

    for _, model := range models {
        _, err := tx.NewCreateTable().
            Model(model).
            IfNotExists().
            Exec(ctx)
        if err != nil {
            return fmt.Errorf("create table %T: %w", model, err)
        }
    }

    // Create indexes
    if err := createIndexes(ctx, tx); err != nil {
        return err
    }

    // Setup FTS (dialect-specific)
    if err := setupFullTextSearch(ctx, tx, db.Dialect().Name()); err != nil {
        return err
    }

    // Record version
    meta := &SchemaMeta{Version: version, AppliedAt: time.Now()}
    _, err = tx.NewInsert().Model(meta).Exec(ctx)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

---

## Models

### Task

```go
// internal/db/models/task.go
type Task struct {
    bun.BaseModel `bun:"table:tasks"`

    ID           string            `bun:"id,pk"`
    ProjectID    string            `bun:"project_id,notnull"`
    Title        string            `bun:"title,notnull"`
    Description  string            `bun:"description"`
    Weight       string            `bun:"weight,notnull"`
    Status       string            `bun:"status,notnull"`
    CurrentPhase string            `bun:"current_phase"`
    Branch       string            `bun:"branch"`
    CreatedBy    string            `bun:"created_by"`
    CreatedAt    time.Time         `bun:"created_at,notnull,default:current_timestamp"`
    UpdatedAt    time.Time         `bun:"updated_at,notnull,default:current_timestamp"`
    StartedAt    bun.NullTime      `bun:"started_at"`
    CompletedAt  bun.NullTime      `bun:"completed_at"`
    Metadata     map[string]string `bun:"metadata,type:jsonb"`

    // Relations
    Phases      []*Phase      `bun:"rel:has-many,join:id=task_id"`
    Transcripts []*Transcript `bun:"rel:has-many,join:id=task_id"`
}
```

### Phase

```go
// internal/db/models/phase.go
type Phase struct {
    bun.BaseModel `bun:"table:phases"`

    ID          int64         `bun:"id,pk,autoincrement"`
    TaskID      string        `bun:"task_id,notnull"`
    PhaseID     string        `bun:"phase_id,notnull"`    // "implement", "test"
    Status      string        `bun:"status,notnull"`
    StartedAt   bun.NullTime  `bun:"started_at"`
    CompletedAt bun.NullTime  `bun:"completed_at"`
    Iterations  int           `bun:"iterations,default:0"`
    CommitSHA   string        `bun:"commit_sha"`
    InputTokens  int          `bun:"input_tokens,default:0"`
    OutputTokens int          `bun:"output_tokens,default:0"`
    Error       string        `bun:"error"`

    // Relations
    Task *Task `bun:"rel:belongs-to,join:task_id=id"`
}

// Composite unique constraint
func init() {
    // CREATE UNIQUE INDEX idx_phases_task_phase ON phases(task_id, phase_id)
}
```

### Transcript

```go
// internal/db/models/transcript.go
type Transcript struct {
    bun.BaseModel `bun:"table:transcripts"`

    ID        int64     `bun:"id,pk,autoincrement"`
    TaskID    string    `bun:"task_id,notnull"`
    PhaseID   string    `bun:"phase_id,notnull"`
    Iteration int       `bun:"iteration,notnull"`
    Role      string    `bun:"role,notnull"`      // "user", "assistant", "tool"
    Content   string    `bun:"content,notnull"`
    Timestamp time.Time `bun:"timestamp,notnull"`

    // FTS support (see Full-Text Search section)
}
```

### Project

```go
// internal/db/models/project.go
type Project struct {
    bun.BaseModel `bun:"table:projects"`

    ID        string    `bun:"id,pk"`
    OrgID     string    `bun:"org_id"`          // NULL for personal
    Name      string    `bun:"name,notnull"`
    Path      string    `bun:"path,notnull"`
    CreatedBy string    `bun:"created_by"`
    CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`

    // Relations
    Tasks []*Task `bun:"rel:has-many,join:id=project_id"`
}
```

### Cost Log

```go
// internal/db/models/cost.go
type CostLog struct {
    bun.BaseModel `bun:"table:cost_logs"`

    ID           int64     `bun:"id,pk,autoincrement"`
    TaskID       string    `bun:"task_id,notnull"`
    PhaseID      string    `bun:"phase_id"`
    UserID       string    `bun:"user_id"`
    Model        string    `bun:"model,notnull"`
    InputTokens  int       `bun:"input_tokens,notnull"`
    OutputTokens int       `bun:"output_tokens,notnull"`
    Cost         float64   `bun:"cost,notnull"`
    Timestamp    time.Time `bun:"timestamp,notnull"`
}
```

### Organization (Team Feature)

```go
// internal/db/models/org.go
type Organization struct {
    bun.BaseModel `bun:"table:organizations"`

    ID        string    `bun:"id,pk"`
    Name      string    `bun:"name,notnull"`
    Slug      string    `bun:"slug,notnull,unique"`
    Plan      string    `bun:"plan,notnull,default:'free'"`
    CreatedAt time.Time `bun:"created_at,notnull,default:current_timestamp"`

    // Relations
    Members  []*Member  `bun:"rel:has-many,join:id=org_id"`
    Projects []*Project `bun:"rel:has-many,join:id=org_id"`
}

type Member struct {
    bun.BaseModel `bun:"table:members"`

    ID       int64     `bun:"id,pk,autoincrement"`
    OrgID    string    `bun:"org_id,notnull"`
    UserID   string    `bun:"user_id,notnull"`
    Email    string    `bun:"email,notnull"`
    Role     string    `bun:"role,notnull,default:'member'"`
    JoinedAt time.Time `bun:"joined_at,notnull,default:current_timestamp"`

    // Relations
    Org *Organization `bun:"rel:belongs-to,join:org_id=id"`
}
```

### User

```go
// internal/db/models/user.go
type User struct {
    bun.BaseModel `bun:"table:users"`

    ID          string    `bun:"id,pk"`
    Email       string    `bun:"email,notnull,unique"`
    DisplayName string    `bun:"display_name"`
    AvatarURL   string    `bun:"avatar_url"`
    Preferences string    `bun:"preferences,type:jsonb"`  // JSON blob
    CreatedAt   time.Time `bun:"created_at,notnull,default:current_timestamp"`
    LastSeenAt  time.Time `bun:"last_seen_at"`
}
```

---

## Full-Text Search

### SQLite: FTS5

```go
func setupFTSSQLite(ctx context.Context, tx bun.Tx) error {
    // Create FTS virtual table
    _, err := tx.ExecContext(ctx, `
        CREATE VIRTUAL TABLE IF NOT EXISTS transcripts_fts USING fts5(
            content,
            task_id UNINDEXED,
            phase_id UNINDEXED,
            content='transcripts',
            content_rowid='id'
        )
    `)
    if err != nil {
        return err
    }

    // Triggers to keep FTS in sync
    _, err = tx.ExecContext(ctx, `
        CREATE TRIGGER IF NOT EXISTS transcripts_ai AFTER INSERT ON transcripts BEGIN
            INSERT INTO transcripts_fts(rowid, content, task_id, phase_id)
            VALUES (new.id, new.content, new.task_id, new.phase_id);
        END
    `)
    if err != nil {
        return err
    }

    _, err = tx.ExecContext(ctx, `
        CREATE TRIGGER IF NOT EXISTS transcripts_ad AFTER DELETE ON transcripts BEGIN
            INSERT INTO transcripts_fts(transcripts_fts, rowid, content, task_id, phase_id)
            VALUES ('delete', old.id, old.content, old.task_id, old.phase_id);
        END
    `)
    return err
}

func searchTranscriptsSQLite(db *bun.DB, query string) ([]Transcript, error) {
    var transcripts []Transcript
    err := db.NewRaw(`
        SELECT t.* FROM transcripts t
        JOIN transcripts_fts fts ON t.id = fts.rowid
        WHERE transcripts_fts MATCH ?
        ORDER BY rank
    `, query).Scan(context.Background(), &transcripts)
    return transcripts, err
}
```

### PostgreSQL: tsvector

```go
func setupFTSPostgres(ctx context.Context, tx bun.Tx) error {
    // Add tsvector column
    _, err := tx.ExecContext(ctx, `
        ALTER TABLE transcripts
        ADD COLUMN IF NOT EXISTS search_vector tsvector
        GENERATED ALWAYS AS (to_tsvector('english', content)) STORED
    `)
    if err != nil {
        return err
    }

    // Create GIN index
    _, err = tx.ExecContext(ctx, `
        CREATE INDEX IF NOT EXISTS idx_transcripts_search
        ON transcripts USING GIN(search_vector)
    `)
    return err
}

func searchTranscriptsPostgres(db *bun.DB, query string) ([]Transcript, error) {
    var transcripts []Transcript
    err := db.NewSelect().
        Model(&transcripts).
        Where("search_vector @@ plainto_tsquery('english', ?)", query).
        OrderExpr("ts_rank(search_vector, plainto_tsquery('english', ?)) DESC", query).
        Scan(context.Background())
    return transcripts, err
}
```

### Unified Search Interface

```go
// internal/db/search.go
type SearchService struct {
    db      *bun.DB
    dialect string
}

func (s *SearchService) SearchTranscripts(ctx context.Context, query string, opts SearchOpts) ([]Transcript, error) {
    switch s.dialect {
    case "sqlite":
        return s.searchSQLite(ctx, query, opts)
    case "postgres":
        return s.searchPostgres(ctx, query, opts)
    default:
        return nil, fmt.Errorf("unsupported dialect: %s", s.dialect)
    }
}
```

---

## Repository Pattern

### Interface

```go
// internal/db/repository.go
type Repository interface {
    // Tasks
    CreateTask(ctx context.Context, task *Task) error
    GetTask(ctx context.Context, id string) (*Task, error)
    UpdateTask(ctx context.Context, task *Task) error
    DeleteTask(ctx context.Context, id string) error
    ListTasks(ctx context.Context, filter TaskFilter) ([]*Task, error)

    // Phases
    CreatePhase(ctx context.Context, phase *Phase) error
    GetPhase(ctx context.Context, taskID, phaseID string) (*Phase, error)
    UpdatePhase(ctx context.Context, phase *Phase) error
    ListPhases(ctx context.Context, taskID string) ([]*Phase, error)

    // Transcripts
    AppendTranscript(ctx context.Context, transcript *Transcript) error
    GetTranscripts(ctx context.Context, taskID string) ([]*Transcript, error)
    SearchTranscripts(ctx context.Context, query string) ([]*Transcript, error)

    // Projects
    CreateProject(ctx context.Context, project *Project) error
    GetProject(ctx context.Context, id string) (*Project, error)
    ListProjects(ctx context.Context, userID string) ([]*Project, error)

    // Cost
    LogCost(ctx context.Context, entry *CostLog) error
    GetCostSummary(ctx context.Context, filter CostFilter) (*CostSummary, error)

    // Users (Team feature)
    CreateUser(ctx context.Context, user *User) error
    GetUser(ctx context.Context, id string) (*User, error)
    GetUserByEmail(ctx context.Context, email string) (*User, error)

    // Organizations (Team feature)
    CreateOrg(ctx context.Context, org *Organization) error
    GetOrg(ctx context.Context, id string) (*Organization, error)
    AddMember(ctx context.Context, member *Member) error
    GetMembers(ctx context.Context, orgID string) ([]*Member, error)
}
```

### Implementation

```go
// internal/db/repository_bun.go
type BunRepository struct {
    db *bun.DB
}

func NewRepository(db *bun.DB) Repository {
    return &BunRepository{db: db}
}

func (r *BunRepository) CreateTask(ctx context.Context, task *Task) error {
    _, err := r.db.NewInsert().Model(task).Exec(ctx)
    return err
}

func (r *BunRepository) GetTask(ctx context.Context, id string) (*Task, error) {
    task := new(Task)
    err := r.db.NewSelect().
        Model(task).
        Where("id = ?", id).
        Scan(ctx)
    if err == sql.ErrNoRows {
        return nil, ErrNotFound
    }
    return task, err
}

func (r *BunRepository) ListTasks(ctx context.Context, filter TaskFilter) ([]*Task, error) {
    var tasks []*Task
    query := r.db.NewSelect().Model(&tasks)

    if filter.ProjectID != "" {
        query = query.Where("project_id = ?", filter.ProjectID)
    }
    if filter.Status != "" {
        query = query.Where("status = ?", filter.Status)
    }
    if filter.Weight != "" {
        query = query.Where("weight = ?", filter.Weight)
    }

    query = query.Order("created_at DESC")

    if filter.Limit > 0 {
        query = query.Limit(filter.Limit)
    }
    if filter.Offset > 0 {
        query = query.Offset(filter.Offset)
    }

    err := query.Scan(ctx)
    return tasks, err
}
```

---

## Transaction Support

```go
// internal/db/tx.go
type TxFunc func(ctx context.Context, repo Repository) error

func (r *BunRepository) InTx(ctx context.Context, fn TxFunc) error {
    return r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        txRepo := &BunRepository{db: tx}
        return fn(ctx, txRepo)
    })
}

// Usage
func (s *TaskService) CompleteTask(ctx context.Context, taskID string) error {
    return s.repo.InTx(ctx, func(ctx context.Context, repo Repository) error {
        task, err := repo.GetTask(ctx, taskID)
        if err != nil {
            return err
        }

        task.Status = "completed"
        task.CompletedAt = bun.NullTime{Time: time.Now()}

        if err := repo.UpdateTask(ctx, task); err != nil {
            return err
        }

        // Log final cost
        return repo.LogCost(ctx, &CostLog{
            TaskID:    taskID,
            Timestamp: time.Now(),
            // ...
        })
    })
}
```

---

## Dual Database Support

### Global vs Project Database

Solo mode uses two SQLite databases:
- `~/.orc/orc.db` - Global (projects, cost, user prefs)
- `.orc/orc.db` - Project (tasks, phases, transcripts)

Team mode uses one Postgres:
- All tables in one database, partitioned by project_id

```go
// internal/db/dual.go
type DualDB struct {
    Global  Repository  // ~/.orc/orc.db or shared postgres
    Project Repository  // .orc/orc.db or shared postgres
}

func NewDualDB(cfg Config) (*DualDB, error) {
    if cfg.Driver == "postgres" {
        // Single postgres for everything
        db, err := Open(cfg)
        if err != nil {
            return nil, err
        }
        repo := NewRepository(db)
        return &DualDB{Global: repo, Project: repo}, nil
    }

    // SQLite: separate databases
    globalCfg := Config{Driver: "sqlite", DSN: globalDBPath()}
    globalDB, err := Open(globalCfg)
    if err != nil {
        return nil, err
    }

    projectCfg := Config{Driver: "sqlite", DSN: projectDBPath()}
    projectDB, err := Open(projectCfg)
    if err != nil {
        return nil, err
    }

    return &DualDB{
        Global:  NewRepository(globalDB),
        Project: NewRepository(projectDB),
    }, nil
}
```

---

## Testing

### SQLite Tests (Default)

```go
func TestTaskCRUD(t *testing.T) {
    db := setupTestDB(t)  // Uses SQLite in-memory
    repo := NewRepository(db)

    // Create
    task := &Task{ID: "TASK-001", Title: "Test", Weight: "small", Status: "pending"}
    err := repo.CreateTask(context.Background(), task)
    require.NoError(t, err)

    // Read
    got, err := repo.GetTask(context.Background(), "TASK-001")
    require.NoError(t, err)
    assert.Equal(t, "Test", got.Title)

    // Update
    got.Status = "running"
    err = repo.UpdateTask(context.Background(), got)
    require.NoError(t, err)

    // Delete
    err = repo.DeleteTask(context.Background(), "TASK-001")
    require.NoError(t, err)
}

func setupTestDB(t *testing.T) *bun.DB {
    db, err := Open(Config{Driver: "sqlite", DSN: ":memory:"})
    require.NoError(t, err)
    require.NoError(t, ApplySchema(db, SchemaVersion))
    t.Cleanup(func() { db.Close() })
    return db
}
```

### Postgres Tests (Optional)

```go
func TestTaskCRUD_Postgres(t *testing.T) {
    if os.Getenv("TEST_POSTGRES_URL") == "" {
        t.Skip("TEST_POSTGRES_URL not set")
    }

    db := setupPostgresTestDB(t)
    repo := NewRepository(db)

    // Same tests as SQLite
    // ...
}

func setupPostgresTestDB(t *testing.T) *bun.DB {
    // Use test container or shared test database
    db, err := Open(Config{Driver: "postgres", DSN: os.Getenv("TEST_POSTGRES_URL")})
    require.NoError(t, err)

    // Cleanup: drop all tables
    t.Cleanup(func() {
        db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
        db.Close()
    })

    require.NoError(t, ApplySchema(db, SchemaVersion))
    return db
}
```

---

## Migration from Current Schema

### Current: Split YAML + SQLite

```
.orc/
├── orc.db                    # SQLite with detection, tasks, phases, transcripts
└── tasks/TASK-001/
    ├── task.yaml             # Task definition
    ├── plan.yaml             # Phase sequence
    └── state.yaml            # Execution state
```

### New: Unified Database

All state in database, YAML files for backup/export only.

### Migration Strategy

```go
func MigrateFromYAML(db *bun.DB, orcDir string) error {
    // Find all task directories
    taskDirs, _ := filepath.Glob(filepath.Join(orcDir, "tasks", "TASK-*"))

    for _, dir := range taskDirs {
        // Read YAML files
        taskYAML, _ := os.ReadFile(filepath.Join(dir, "task.yaml"))
        var task Task
        yaml.Unmarshal(taskYAML, &task)

        // Insert into database
        db.NewInsert().Model(&task).Exec(context.Background())

        // Similar for plan, state, transcripts
    }

    return nil
}
```

---

## Performance Considerations

### SQLite Optimizations

```go
func optimizeSQLite(db *sql.DB) {
    // WAL mode for concurrent reads
    db.Exec("PRAGMA journal_mode = WAL")

    // Reasonable busy timeout
    db.Exec("PRAGMA busy_timeout = 5000")

    // Foreign keys
    db.Exec("PRAGMA foreign_keys = ON")

    // Synchronous mode (normal is safe with WAL)
    db.Exec("PRAGMA synchronous = NORMAL")
}
```

### Postgres Optimizations

```go
func optimizePostgres(db *sql.DB, cfg Config) {
    // Connection pool settings
    db.SetMaxOpenConns(cfg.MaxConns)
    db.SetMaxIdleConns(cfg.MaxConns / 2)
    db.SetConnMaxLifetime(time.Hour)
}
```

### Query Optimization

```go
// Preload relations to avoid N+1
func (r *BunRepository) GetTaskWithPhases(ctx context.Context, id string) (*Task, error) {
    task := new(Task)
    err := r.db.NewSelect().
        Model(task).
        Relation("Phases").
        Where("task.id = ?", id).
        Scan(ctx)
    return task, err
}
```

---

## Error Handling

```go
// internal/db/errors.go
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrConstraint    = errors.New("constraint violation")
)

func mapError(err error) error {
    if err == nil {
        return nil
    }
    if err == sql.ErrNoRows {
        return ErrNotFound
    }

    // SQLite constraint error
    if strings.Contains(err.Error(), "UNIQUE constraint") {
        return ErrAlreadyExists
    }

    // Postgres constraint error
    var pgErr *pgdriver.Error
    if errors.As(err, &pgErr) {
        if pgErr.Field('C') == "23505" {  // unique_violation
            return ErrAlreadyExists
        }
    }

    return err
}
```

---

## Configuration Examples

### Solo Developer (SQLite)

```bash
# No configuration needed - uses ~/.orc/orc.db and .orc/orc.db by default
orc serve
```

### Local Postgres

```bash
# Start postgres
docker run -d --name orc-db -e POSTGRES_PASSWORD=orc -p 5432:5432 postgres:16

# Configure orc
export DATABASE_URL="postgres://postgres:orc@localhost:5432/orc?sslmode=disable"
orc serve
```

### Team Server

```yaml
# docker-compose.yaml
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: orc
      POSTGRES_PASSWORD: ${DB_PASSWORD}
      POSTGRES_DB: orc
    volumes:
      - postgres_data:/var/lib/postgresql/data

  orc:
    image: orc:latest
    environment:
      DATABASE_URL: postgres://orc:${DB_PASSWORD}@db:5432/orc?sslmode=disable
    depends_on:
      - db
```
