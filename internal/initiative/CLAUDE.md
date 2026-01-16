# Initiative Package

Provides initiative/feature grouping for related tasks with shared context.

## Overview

Initiatives group multiple related tasks under a shared vision, decisions, and context. They enable:
- Shared context across related tasks
- Decision tracking
- Task dependency management
- P2P/team collaboration via shared directories

## Key Types

| Type | Purpose |
|------|---------|
| `Initiative` | Main struct for initiative data |
| `Status` | Initiative status (draft, active, completed, archived) |
| `Decision` | Recorded decision with rationale |
| `TaskRef` | Reference to a task within the initiative |
| `Identity` | Owner information (initials, name, email) |
| `Store` | Database storage manager |
| `CommitConfig` | Git commit configuration |
| `Manifest` | Bulk task creation from YAML file |
| `ManifestTask` | Task definition within a manifest |

## Database Storage

Initiatives are stored in SQLite (source of truth):

```
┌─────────────────────────────────────────────────────────────┐
│ SQLite Database (.orc/orc.db)                               │
│ ├── initiatives              Initiative definitions         │
│ ├── initiative_tasks         Task-to-initiative links       │
│ ├── initiative_decisions     Decisions with rationale       │
│ └── initiative_dependencies  Blocked-by relationships       │
└─────────────────────────────────────────────────────────────┘
```

### Store Usage

```go
// Create store
store, err := initiative.NewStore(initiative.StoreConfig{
    ProjectRoot: projectRoot,
})
defer store.Close()

// Save (writes to database)
err = store.Save(init)

// Load
init, err := store.Load(id)

// List all
initiatives, err := store.List()
```

## Usage

```go
// Create new initiative
init := initiative.New("INIT-001", "User Authentication")
init.Owner = initiative.Identity{Initials: "RM"}
init.Vision = "Secure authentication using JWT tokens"
init.Save()

// Load existing
init, err := initiative.Load("INIT-001")

// Add tasks
init.AddTask("TASK-001", "Auth models", nil)
init.AddTask("TASK-002", "Login endpoints", []string{"TASK-001"})

// Record decision
init.AddDecision("Using bcrypt for passwords", "Industry standard", "RM")

// Get ready tasks (all deps satisfied)
ready := init.GetReadyTasks()

// Update task status
init.UpdateTaskStatus("TASK-001", "completed")

// List initiatives
all, err := initiative.List(false)  // false = local, true = shared
active, err := initiative.ListByStatus(initiative.StatusActive, false)
```

## Status Lifecycle

```
draft -> active -> completed
                -> archived (abandoned)
```

## P2P Integration

Use `shared=true` for team collaboration:
```go
init, err := initiative.LoadShared("INIT-001")
init.SaveShared()
initiatives, err := initiative.List(true) // shared
```

## Bulk Task Creation (Manifest)

Create multiple tasks from a YAML manifest file with inline specs:

```go
// Parse manifest
manifest, err := initiative.ParseManifest("tasks.yaml")

// Validate
if errs := initiative.ValidateManifest(manifest); len(errs) > 0 {
    // Handle validation errors
}

// Get topological order for creation
order, err := initiative.TopologicalSort(manifest.Tasks)
```

See `docs/specs/FILE_FORMATS.md` for the manifest YAML format.

### Key Functions

| Function | Purpose |
|----------|---------|
| `ParseManifest(path)` | Read and parse manifest YAML file |
| `ParseManifestBytes(data)` | Parse manifest from byte slice |
| `ValidateManifest(m)` | Validate manifest, returns all errors |
| `TopologicalSort(tasks)` | Sort tasks by dependency order |

## Testing

```bash
go test ./internal/initiative/... -v
```
