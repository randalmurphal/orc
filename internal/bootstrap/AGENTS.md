# Bootstrap

Bootstrap owns `orc init`: creating the minimum project and global state needed to start using orc.

## Owns

- project `.orc/` config scaffolding
- global project registration
- initial database creation/migration
- initial repo-side helpers installed during bootstrap

## Rules

- Bootstrap must stay fast, idempotent, and safe to rerun.
- Keep the split clear:
  - project `.orc/` is git-tracked config
  - `~/.orc/projects/<id>/...` is runtime state
- Do not move interactive setup work here; that belongs in `setup`.

## Verification

```bash
go test ./internal/bootstrap/...
```
