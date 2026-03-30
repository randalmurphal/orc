# CLI

The CLI package is a thin command layer over the rest of the system.

## Owns

- Cobra command registration
- flag parsing
- terminal presentation
- command-to-service wiring

## Rules

- Commands should delegate to domain packages instead of reimplementing business logic.
- Shared construction helpers belong in one place and should be reused by commands.
- Help text matters. Update it when behavior changes.
- Prefer non-interactive flows unless a command is explicitly interactive.

## When Changing Commands

- Check existing command patterns before introducing a new helper.
- Keep output stable where users may script against it.
- If a command touches project resolution, verify default project and explicit `--project` behavior.

## Verification

```bash
go test ./internal/cli/...
```
