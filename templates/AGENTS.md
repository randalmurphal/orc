# Templates

The templates directory contains embedded product behavior: prompts, workflow YAML, phase YAML, agents, and helper assets.

## Owns

- built-in prompt content
- built-in workflow definitions
- built-in phase template definitions
- embedded helper scripts and agent definitions

## Rules

- Treat prompt and workflow changes like behavior changes, not cosmetic text edits.
- Keep variable usage aligned with the runtime variable system.
- If a built-in workflow or phase field changes, verify parser, resolver, seed, and tests still agree.
- Avoid duplicating the same instruction in many templates when one shared prompt fragment or workflow rule will do.

## Verification

```bash
go test ./templates/... ./internal/workflow/...
```
