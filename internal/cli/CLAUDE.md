# CLI Package

Command-line interface using Cobra. Each command is in its own file.

## File Structure

### Core Files

| File | Purpose |
|------|---------|
| `root.go` | Root command, global flags, initialization |
| `commands.go` | Command registration, helper utilities |
| `serve.go` | API server command |
| `session.go` | Interactive session handling |
| `signals.go` | Signal handling (SIGINT, SIGTERM) |
| `template.go` | CLI output templates |
| `errors.go` | CLI error types and formatting |

### Command Files (21 total)

| File | Command | Description |
|------|---------|-------------|
| `cmd_init.go` | `orc init` | Initialize .orc/ in project (instant, <500ms) |
| `cmd_setup.go` | `orc setup` | Claude-powered interactive setup |
| `cmd_new.go` | `orc new "title"` | Create new task |
| `cmd_list.go` | `orc list` | List all tasks |
| `cmd_show.go` | `orc show TASK-ID` | Show task details |
| `cmd_run.go` | `orc run TASK-ID` | Execute task phases |
| `cmd_pause.go` | `orc pause TASK-ID` | Pause running task |
| `cmd_resume.go` | `orc resume TASK-ID` | Resume paused task |
| `cmd_rewind.go` | `orc rewind TASK-ID --to PHASE` | Reset to before phase |
| `cmd_status.go` | `orc status` | Show running tasks |
| `cmd_log.go` | `orc log TASK-ID` | Show task transcripts |
| `cmd_diff.go` | `orc diff TASK-ID` | Show task changes |
| `cmd_delete.go` | `orc delete TASK-ID` | Delete task and files |
| `cmd_approve.go` | `orc approve TASK-ID` | Approve pending gate |
| `cmd_config.go` | `orc config [key] [value]` | Get/set configuration |
| `cmd_pool.go` | `orc pool [subcommand]` | Manage OAuth token pool |
| `cmd_skip.go` | `orc skip TASK-ID` | Skip current phase |
| `cmd_cleanup.go` | `orc cleanup` | Clean up stale worktrees |
| `cmd_version.go` | `orc version` | Show version info |
| `cmd_projects.go` | `orc projects` | List registered projects |
| `cmd_export.go` | `orc export TASK-ID` | Export task to YAML (with plan, state, transcripts) |
| `cmd_export.go` | `orc import <file>` | Import task from YAML file |
| `cmd_initiative.go` | `orc initiative [subcommand]` | Manage initiatives (grouped tasks) |

## Command Structure

Each command file follows this pattern:

```go
// cmd_example.go
package cli

import (
    "github.com/spf13/cobra"
)

func init() {
    rootCmd.AddCommand(exampleCmd)
}

var exampleCmd = &cobra.Command{
    Use:   "example [args]",
    Short: "Brief description",
    Long:  `Detailed description with examples.`,
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        // Implementation
        return nil
    },
}
```

## Key Commands

### `orc new`
Creates task with AI-powered weight classification:
1. Prompts for task description
2. Calls Claude to classify weight (trivial/small/medium/large/greenfield)
3. Generates plan based on weight template
4. Creates task files in `.orc/tasks/TASK-XXX/`

### `orc run`
Executes task phases:
1. Sets up git worktree (if enabled)
2. Loads task and plan
3. For each phase:
   - Evaluates gate conditions
   - Executes phase (via executor)
   - Creates checkpoint commit
4. Creates PR or merges (based on config)

### `orc resume`
Resumes from checkpoint:
1. Loads task state
2. Identifies last completed phase
3. Continues from next phase

## Global Flags

| Flag | Description |
|------|-------------|
| `--verbose, -v` | Enable verbose output |
| `--quiet, -q` | Suppress non-essential output |
| `--profile, -p` | Automation profile (auto/fast/safe/strict) |

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
# Run CLI tests
go test ./internal/cli/... -v

# Test specific command
go test ./internal/cli/... -run TestInitCmd -v
```

## Adding a New Command

1. Create `cmd_mycommand.go`
2. Define command with `&cobra.Command{}`
3. Register in `init()` with `rootCmd.AddCommand()`
4. Add tests in `cmd_mycommand_test.go`
