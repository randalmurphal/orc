# Automation

Automation turns project signals into follow-up work.

## Owns

- trigger definitions and persistence
- evaluation of trigger conditions
- creation of automation tasks and notifications

## Rules

- Automation should decide when to act, not reimplement task execution.
- Keep trigger evaluation deterministic where possible.
- If automation creates tasks, those tasks must be valid normal orc tasks, not a side channel.

## Verification

```bash
go test ./internal/automation/...
```
