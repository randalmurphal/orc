# Database Package

SQLite-based persistence layer with embedded schema migrations and FTS5 full-text search.

## Overview

This package provides two database types:
- **GlobalDB** (`~/.orc/orc.db`) - Cross-project data: project registry, cost logs, templates
- **ProjectDB** (`.orc/orc.db`) - Per-project data: tasks, phases, transcripts with FTS

## Key Types

| Type | Purpose |
|------|---------|
| `DB` | Core database wrapper with Open/Close/Migrate |
| `GlobalDB` | Global operations (extends DB) |
| `ProjectDB` | Project operations with FTS (extends DB) |
| `Transcript` | Transcript record with task/phase/content |
| `TranscriptMatch` | FTS search result with snippets |
| `CostSummary` | Aggregated cost statistics |

## Schema Files

Embedded via `//go:embed schema/*.sql`:

| File | Purpose |
|------|---------|
| `schema/global_001.sql` | Projects, cost_log, templates tables |
| `schema/project_001.sql` | Detection, tasks, phases, transcripts, FTS |

## Usage

```go
// Global DB (cross-project)
gdb, err := db.OpenGlobal()
defer gdb.Close()
gdb.SyncProject(project)
gdb.RecordCost(projectID, taskID, phase, 0.05, 1000, 500)
summary, _ := gdb.GetCostSummary(projectID, since)

// Project DB (per-project)
pdb, err := db.OpenProject("/path/to/project")
defer pdb.Close()
pdb.StoreDetection(detection)
pdb.SaveTask(task)
matches, _ := pdb.SearchTranscripts("error handling")
```

## Key Features

### Automatic Migrations
```go
func (d *DB) Migrate(schemaType string) error
// Runs all schema/*.sql files matching pattern
// Safe to call multiple times (IF NOT EXISTS)
```

### Full-Text Search
```go
func (p *ProjectDB) SearchTranscripts(query string) ([]TranscriptMatch, error)
// Uses FTS5 for fast content search
// Returns snippets with highlighted matches
```

### Cost Tracking
```go
func (g *GlobalDB) RecordCost(projectID, taskID, phase string, costUSD float64, inputTokens, outputTokens int) error
func (g *GlobalDB) GetCostSummary(projectID string, since time.Time) (*CostSummary, error)
```

## Schema Design

### Global Tables
- `projects` - Registered projects (id, name, path, language, created_at)
- `cost_log` - Token usage per phase (no FK for orphan entries)
- `templates` - Shared task templates (JSON phases)

### Project Tables
- `detection` - Project detection results (language, frameworks, build tools)
- `tasks` - Task records (id, title, weight, status, cost)
- `phases` - Phase execution state (iterations, tokens)
- `transcripts` - Claude conversation logs
- `transcripts_fts` - FTS5 virtual table with triggers
- `initiatives` - Initiative groupings (id, title, status, vision, owner)
- `initiative_decisions` - Decisions within initiatives
- `initiative_tasks` - Task-to-initiative mappings with sequence
- `task_dependencies` - Task dependency relationships

## Testing

```bash
go test ./internal/db/... -v
```

Tests use `t.TempDir()` for isolated databases.
