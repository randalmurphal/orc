# Setup

The setup package owns the interactive setup flow after bootstrap.

## Owns

- setup prompt generation
- spawning the interactive setup session
- validating setup results

## Rules

- Keep setup distinct from bootstrap:
  - bootstrap creates minimum project state
  - setup helps tailor the project configuration
- Validation should be explicit and user-visible when it fails.

## Verification

```bash
go test ./internal/setup/...
```
