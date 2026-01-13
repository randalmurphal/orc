# Bootstrap Package

Instant project initialization (<500ms, zero prompts).

## Overview

This package handles the `orc init` command - creating the `.orc/` directory structure and initializing SQLite databases. It's designed to be fast (git-style) with no interactive prompts.

## Key Functions

| Function | Purpose |
|----------|---------|
| `Run(opts Options) (*Result, error)` | Main initialization entry point |
| `UpdateGitignore(projectDir string) error` | Add orc entries to .gitignore |
| `InjectOrcSection(projectDir string) error` | Add orc section to CLAUDE.md |
| `InjectKnowledgeSection(projectDir string) error` | Add knowledge section to CLAUDE.md |
| `HasKnowledgeSection(projectDir string) bool` | Check if knowledge section exists |
| `ShouldSuggestSplit(projectDir string) (bool, int, error)` | Check if CLAUDE.md exceeds 200 lines |

## Initialization Steps

1. Create `.orc/` directory structure
2. Write minimal `config.yaml` (profile: auto, version: 1)
3. Create and migrate SQLite database (`.orc/orc.db`)
4. Run project detection, store in SQLite
5. Register project in global registry (`~/.orc/projects.yaml`)
6. Update `.gitignore` with orc patterns
7. Install hooks (orc-stop.sh)
8. Install plugins (slash commands)
9. Inject orc section into CLAUDE.md
10. Inject knowledge section into CLAUDE.md

## Options

```go
type Options struct {
    WorkDir string  // Project directory (default: cwd)
    Force   bool    // Overwrite existing .orc/
}
```

## Result

```go
type Result struct {
    ProjectDir string        // Initialized directory
    Duration   time.Duration // Time taken (<500ms target)
    Detection  *detect.Detection // Project detection results
}
```

## Usage

```go
result, err := bootstrap.Run(bootstrap.Options{
    WorkDir: "/path/to/project",
    Force:   false,
})
if err != nil {
    return err
}
fmt.Printf("Initialized in %v\n", result.Duration)
```

## .gitignore Entries

Added automatically:
```
# orc - Claude Code Task Orchestrator
.orc/tasks/
.orc/worktrees/
.orc/orc.db
.orc/orc.db-journal
.orc/orc.db-wal
.orc/orc.db-shm
.mcp.json
```

**Why .orc/tasks/ is ignored:** Task runtime state should not be in git (same pattern as Terraform state files). Use `orc export/import` for sharing tasks between machines or team members.

## Performance

Target: <500ms with no prompts. Actual: ~20-30ms for typical projects.

## Testing

```bash
go test ./internal/bootstrap/... -v
```

Tests verify:
- Directory structure creation
- Config file contents
- SQLite database initialization
- .gitignore updates
- Performance (<500ms)
