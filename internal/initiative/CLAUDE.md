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

## Directory Structure

```
# Solo mode
.orc/initiatives/INIT-001/
├── initiative.yaml
├── research.md      # Context file
├── spec.md          # Context file
└── architecture.md  # Context file

# P2P/Team mode
.orc/shared/initiatives/INIT-001/
├── initiative.yaml
└── ...
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

## Testing

```bash
go test ./internal/initiative/... -v
```
