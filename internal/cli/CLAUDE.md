# CLI Package

Command-line interface using Cobra. Each command is in its own file.

## File Structure

| File | Purpose |
|------|---------|
| `root.go` | Root command, global flags |
| `commands.go` | Command registration, helpers |
| `serve.go` | API server command |
| `session.go` | Interactive session handling |
| `signals.go` | Signal handling (SIGINT, SIGTERM) |
| `cmd_export.go` | Export/import with tar.gz/zip/dir formats |

## Command Pattern

```go
// cmd_example.go
func init() {
    rootCmd.AddCommand(exampleCmd)
}

var exampleCmd = &cobra.Command{
    Use:   "example [args]",
    Short: "Brief description",
    RunE: func(cmd *cobra.Command, args []string) error {
        return nil
    },
}
```

## Key Commands

### `orc new "title"`

Creates task with AI weight classification. Flags: `--weight`, `--category`, `--template`, `--blocked-by`, `--initiative`

### `orc run TASK-ID`

Executes phases: setup worktree -> load plan -> execute phases -> create PR/merge. Flags: `--force`, `--profile`, `--auto-skip`

### `orc resume TASK-ID`

Resumes paused/blocked/failed/orphaned tasks from last incomplete phase.

### `orc status`

Priority display: Orphaned -> Attention -> Running -> Blocked -> Ready -> Paused -> Recent

Dependency-aware: BLOCKED (waiting on deps), READY (deps complete)

### `orc deps [TASK-ID]`

Views: default (single task), `--tree` (recursive), `--graph` (ASCII)

### `orc log TASK-ID --follow`

Real-time streaming via fsnotify with polling fallback.

### `orc export/import`

Data portability with tar.gz archives. Export defaults include state and transcripts. Import auto-detects format and handles running→interrupted transformation. See [COMMANDS.md](COMMANDS.md) for full flag reference.

## Global Flags

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Verbose output |
| `--quiet, -q` | Suppress output |
| `--profile, -p` | auto/fast/safe/strict |
| `--plain` | Disable emoji |

## Aliases

`ls` -> `list`, `st` -> `status`, `rm` -> `delete`

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Task not found |
| 4 | Gate blocked |
| 5 | Execution failed |

## Testing

```bash
go test ./internal/cli/... -v
go test ./internal/cli/... -run TestInitCmd -v
```

## Adding Commands

1. Create `cmd_mycommand.go`
2. Define `&cobra.Command{}`
3. Register in `init()` with `rootCmd.AddCommand()`
4. Add tests in `cmd_mycommand_test.go`

## Reference

See [COMMANDS.md](COMMANDS.md) for full command reference.
