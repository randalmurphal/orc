# orc

Intelligent Claude Code orchestrator that scales rigor to task complexity.

## Features

- **Weight-based rigor**: Trivial tasks stay trivial, complex tasks get full lifecycle
- **Git-native checkpointing**: Branches, commits, and worktrees for isolation
- **Multi-project support**: Global registry to manage tasks across multiple repositories
- **Quality gates**: Auto, AI, or human approval between phases
- **Auto-completion**: Create PRs or direct merge after task completion
- **Full visibility**: Live transcripts, timeline view, rewindable history
- **Ralph-style execution**: Persistent loops within structured phases

## Quick Start

```bash
# Initialize in your project
orc init

# Create a task
orc new "Add user authentication"

# Run the task
orc run TASK-001

# Check status
orc status
```

## Task Weights

| Weight | What It Means | Phases |
|--------|---------------|--------|
| **trivial** | <10 lines, obvious fix | implement |
| **small** | Single component | implement → test |
| **medium** | Multiple files | spec → implement → review → test |
| **large** | Cross-cutting changes | research → spec → design → impl → review → test |
| **greenfield** | New system from scratch | Full lifecycle + architecture + docs |

AI classifies tasks automatically. Override with `--weight`:

```bash
orc new "Refactor auth system" --weight large
```

## Commands

```bash
orc new <title>         # Create task
orc run <task-id>       # Execute/resume task
orc pause <task-id>     # Pause with checkpoint
orc rewind <task-id> --to <phase>  # Rewind to checkpoint
orc approve <task-id>   # Approve human gate
orc log <task-id>       # View transcripts
orc diff <task-id>      # View changes
orc status              # Overall status
```

## Configuration

```yaml
# .orc/config.yaml
gates:
  default_type: auto    # auto | ai | human
  phase_overrides:
    merge: human        # Human approves merge

worktree:
  enabled: true         # Enable worktree isolation (default: true)
  cleanup_on_complete: true

completion:
  action: pr            # pr | merge | none
  target_branch: main
  pr:
    title: "[orc] {{TASK_TITLE}}"
    auto_merge: true
```

## How It Works

1. **Classify**: AI determines task weight (user can override)
2. **Plan**: Generate phase sequence from weight template
3. **Execute**: Ralph-style loop within each phase until completion (in isolated worktree)
4. **Checkpoint**: Git commit after each phase
5. **Gate**: Auto/AI/human approval before next phase
6. **Complete**: Create PR or direct merge (configurable)

## Documentation

- [Architecture Overview](docs/architecture/OVERVIEW.md)
- [CLI Specification](docs/specs/CLI.md)
- [File Formats](docs/specs/FILE_FORMATS.md)
- [Design Decisions](docs/decisions/)

## Development

**Native:**
```bash
make setup    # First-time setup
make build    # Build to bin/orc
make test     # Run tests
make lint     # Run linters
./bin/orc --help
```

**Container:**
```bash
make dev           # Interactive development shell
make docker-test   # Run tests in container
make docker-build  # Build all images
```

**Dependencies:**
- Requires sibling repos: `../llmkit` and `../flowgraph`
- Uses `go.mod` replace directives for local development

## License

MIT
