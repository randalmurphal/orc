# Variable Package

Unified variable resolution system for orc workflows. Replaces scattered variable building with a single, extensible resolver.

## Overview

| File | Purpose |
|------|---------|
| `types.go` | Core types: Definition, SourceType, configs |
| `resolver.go` | Main Resolver with all source type handlers |
| `script.go` | Sandboxed script execution |
| `cache.go` | TTL-based caching |

## Source Types

| Type | Config Struct | Description |
|------|---------------|-------------|
| `static` | `StaticConfig` | Literal fixed value |
| `env` | `EnvConfig` | Environment variable |
| `script` | `ScriptConfig` | Script stdout (sandboxed) |
| `api` | `APIConfig` | HTTP GET response |
| `phase_output` | `PhaseOutputConfig` | Prior phase artifact |
| `prompt_fragment` | `PromptFragmentConfig` | Reusable prompt snippet |

## Usage

```go
import "github.com/randalmurphal/orc/internal/variable"

// Create resolver for project
resolver := variable.NewResolver(projectRoot)

// Define custom variables
defs := []variable.Definition{
    {
        Name:       "JIRA_CONTEXT",
        SourceType: variable.SourceScript,
        SourceConfig: json.RawMessage(`{"path": "fetch-jira.sh", "timeout_ms": 5000}`),
        CacheTTL:   5 * time.Minute,
    },
}

// Create resolution context
ctx := &variable.ResolutionContext{
    TaskID:      "TASK-001",
    Phase:       "implement",
    WorkingDir:  "/path/to/worktree",
    PriorOutputs: map[string]string{
        "spec": specContent,
    },
}

// Resolve all variables
vars, err := resolver.ResolveAll(context.Background(), defs, ctx)

// Render template
prompt := variable.RenderTemplate(template, vars)
```

## Built-in Variables

These are automatically populated from `ResolutionContext`:

| Variable | Source |
|----------|--------|
| `TASK_ID`, `TASK_TITLE`, `TASK_DESCRIPTION`, `TASK_CATEGORY` | Task |
| `RUN_ID`, `WORKFLOW_ID`, `PROMPT`, `INSTRUCTIONS` | Workflow run |
| `PHASE`, `ITERATION` | Current phase |
| `WORKTREE_PATH`, `TASK_BRANCH`, `TARGET_BRANCH` | Git context |
| `SPEC_CONTENT`, `RESEARCH_CONTENT`, `TDD_TESTS_CONTENT`, etc. | Prior outputs |
| `OUTPUT_{PHASE}` | Any prior phase output |

## Script Security

Scripts are sandboxed:
- **Allowed paths**: Only `.orc/scripts/` directory
- **Timeout**: Default 5 seconds, configurable
- **Max output**: 1MB limit
- **Environment**: Inherits env + `ORC_PROJECT_ROOT`

```bash
# Scripts go in .orc/scripts/
.orc/scripts/fetch-jira.sh
.orc/scripts/get-context.sh
```

## Caching

Variables with `CacheTTL > 0` are cached:

```go
variable.Definition{
    Name:       "EXPENSIVE_API",
    SourceType: variable.SourceAPI,
    SourceConfig: json.RawMessage(`{"url": "https://api.example.com/data"}`),
    CacheTTL:   10 * time.Minute,  // Cache for 10 minutes
}
```

Cache keys include context (task ID for phase outputs) to prevent cross-task contamination.

## Integration Points

This package is used by:
- `internal/executor/` - Replaces `BuildTemplateVars()` and `RenderTemplate()`
- `internal/workflow/` - Resolves workflow-defined variables before phase execution

## Migration from Existing Code

| Old Pattern | New Pattern |
|-------------|-------------|
| `BuildTemplateVars(t, p, s, iter, retry)` | `resolver.ResolveAll(ctx, defs, rctx)` |
| `vars.WithSpecFromDatabase(...)` | Prior outputs in `ResolutionContext.PriorOutputs` |
| `RenderTemplate(tmpl, vars)` | `variable.RenderTemplate(tmpl, vars)` |
| Scattered `With*` methods | Single `ResolutionContext` struct |
