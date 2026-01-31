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
| `cmd_export.go` | Export with tar.gz/zip/dir formats |
| `cmd_import.go` | Import from archives (tar.gz/zip/dir/YAML) |
| `cmd_import_jira.go` | Import from Jira Cloud via API |
| `cmd_initiative_plan.go` | Bulk task creation from YAML manifest (`orc initiative plan`) |
| `cmd_migrate.go` | Plan migration commands (`orc migrate plans`) |
| `cmd_run.go` | Task execution with auto-migration |
| `cmd_phases.go` | Phase template CRUD (`orc phase new/show/config`) |
| `cmd_workflows.go` | Workflow management (`orc workflow add-phase`) |
| `cmd_show.go` | Task display with workflow-aware phase listing |
| `cmd_gates.go` | Gate inspection (`orc gates list/show`) |
| `cmd_gates_test.go` | Gate command tests |
| `cmd_show_gates_test.go` | Gate display in `orc show` tests |
| `cmd_run_skipgates_test.go` | `--skip-gates` flag tests |
| `git_helpers.go` | `NewGitOpsFromConfig()` — ONE way to create `git.Git` |

## Git Operations

**ONE way to create `git.Git` in CLI commands:**

```go
gitOps, err := NewGitOpsFromConfig(projectRoot, orcConfig)
```

Defined in `git_helpers.go`. Resolves worktree dir via `config.ResolveWorktreeDir`, sets branch/commit prefix and executor prefix from orc config.

| Do | Don't |
|----|-------|
| `NewGitOpsFromConfig(root, cfg)` | `git.New(root, git.DefaultConfig())` |
| Use the helper for every command | Inline `git.Config{}` construction |
| Pass `projectRoot` from `ResolveProjectPath()` | Use `os.Getwd()` for path resolution |

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

Creates task with AI weight classification. Flags: `--weight`, `--category`, `--template`, `--blocked-by`, `--initiative`, `--branch`, `--target-branch`, `--pr-draft`, `--pr-labels`, `--pr-reviewers`

Branch control flags (`cmd_new.go:627-636`):

| Flag | Type | Purpose |
|------|------|---------|
| `--branch` | `string` | Custom branch name (default: auto-generated `orc/TASK-XXX`) |
| `--target-branch` | `string` | PR target branch (default: repo default branch) |
| `--pr-draft` | `bool` | Create PR as draft |
| `--pr-labels` | `[]string` | Labels to apply to PR |
| `--pr-reviewers` | `[]string` | Reviewers to request on PR |

Both `--branch` and `--target-branch` are validated via `git.ValidateBranchName()` before task creation.

### `orc run TASK-ID`

Executes phases: setup worktree -> load plan -> execute phases -> create PR/merge. Auto-migrates stale plans before execution. Flags: `--force`, `--profile`, `--auto-skip`, `--skip-gates`

`--skip-gates` bypasses all gate evaluations (auto-approves every phase). Useful for dev/testing iterations.

### `orc resume TASK-ID`

Resumes paused/blocked/failed/orphaned tasks from last incomplete phase.

### `orc status`

Priority display: Orphaned -> Attention -> Running -> Blocked -> Ready -> Paused -> Recent

Dependency-aware: BLOCKED (waiting on deps), READY (deps complete)

**Phase display:** Reads `task.CurrentPhase` directly from the task record (set by executor before each phase starts). Only shows "starting" when CurrentPhase is genuinely empty (task just created). No enrichment from `workflow_runs`.

**Worktree path:** Displayed for running tasks. Resolved via `config.ResolveWorktreeDir()` → absolute path at `~/.orc/worktrees/<project-id>/orc-TASK-XXX/`.

### `orc deps [TASK-ID]`

Views: default (single task), `--tree` (recursive), `--graph` (ASCII)

### `orc log TASK-ID --follow`

Real-time streaming via fsnotify with polling fallback.

### `orc export`

Export tasks/initiatives to tar.gz archive. Defaults include state and transcripts. Flags: `--all-tasks`, `--initiatives`, `--minimal`, `--format`. See [COMMANDS.md](COMMANDS.md) for full flag reference.

### `orc import [path]`

Import from archive, directory, or YAML file. Auto-detects format. Newer `updated_at` wins, running tasks become interrupted. Flags: `--force`, `--skip-existing`, `--dry-run`

### `orc import jira`

Import Jira Cloud issues as orc tasks. Epics → initiatives (default, disable with `--no-epics`). Idempotent: existing tasks updated, not duplicated. Auth: flags > env vars > config. Flags: `--url`, `--email`, `--token`, `--project`, `--jql`, `--no-epics`, `--dry-run`, `--weight`, `--queue`

### `orc initiative plan`

Creates multiple tasks from a YAML manifest file. Auto-assigns `workflow_id` based on task weight via `workflow.WeightToWorkflowID()`. Supports `--create-initiative` to create a new initiative and link all tasks. Tasks are created in topological order (respecting `blocked_by`).

### `orc migrate plans`

Migrates stale task plans to current templates. Staleness detected via version mismatch, phase sequence change, or inline prompts. Preserves completed/skipped phase statuses. Flags: `--all`, `--dry-run`

### `orc show TASK-ID`

Displays task details including phases from actual workflow (not weight-derived). Falls back to weight-based display if no workflow found. Checks `task.WorkflowId` first, then `workflow_runs` table.

`--gates` flag shows gate decision history (type, approved/rejected, reason) per phase. `--full` includes gates alongside session, cost, and review info.

**Worktree path:** Displayed for running/in-progress tasks when worktree exists on disk.

### Gate Commands

| Command | Purpose |
|---------|---------|
| `orc gates list` | Table of gate config per workflow phase (`--json` for machine output) |
| `orc gates show <phase>` | Detailed gate config: type, source, retry target, agent |

Gate inspection uses `gate.Resolver` to show effective gate types after resolution hierarchy (task override > workflow phase > phase template > config > profile default).

### Phase Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `orc phase new ID` | Create phase template | `--agent`, `--max-iterations`, `--gate` |
| `orc phase show ID` | View phase template details | - |
| `orc phase config ID` | Update phase template | `--agent`, `--max-iterations`, `--thinking` |

**`--agent` flag:** Sets the executor agent for the phase. Validates agent exists before saving.

### Workflow Phase Commands

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `orc workflow add-phase WF PHASE` | Add phase to workflow | `--agent` (override executor) |

**Agent override:** When `--agent` is specified on `workflow add-phase`, it overrides the phase template's default agent for this workflow only.

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
