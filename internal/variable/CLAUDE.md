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

| Category | Variables |
|----------|-----------|
| Task | `TASK_ID`, `TASK_TITLE`, `TASK_DESCRIPTION`, `TASK_CATEGORY`, `WEIGHT` |
| Workflow | `RUN_ID`, `WORKFLOW_ID`, `PROMPT`, `INSTRUCTIONS` |
| Phase | `PHASE`, `ITERATION`, `RETRY_CONTEXT` |
| Git | `WORKTREE_PATH`, `PROJECT_ROOT`, `TASK_BRANCH`, `TARGET_BRANCH` |
| Constitution | `CONSTITUTION_CONTENT` |
| Initiative | `INITIATIVE_ID`, `INITIATIVE_TITLE`, `INITIATIVE_VISION`, `INITIATIVE_DECISIONS`, `INITIATIVE_CONTEXT`, `INITIATIVE_TASKS` |
| Review | `REVIEW_ROUND`, `REVIEW_FINDINGS` |
| Project Detection | `LANGUAGE`, `HAS_FRONTEND`, `HAS_TESTS`, `TEST_COMMAND`, `LINT_COMMAND`, `BUILD_COMMAND`, `FRAMEWORKS` |
| Testing | `COVERAGE_THRESHOLD`, `REQUIRES_UI_TESTING`, `SCREENSHOT_DIR`, `TEST_RESULTS`, `TDD_TEST_PLAN` |
| Automation | `RECENT_COMPLETED_TASKS`, `RECENT_CHANGED_FILES`, `CHANGELOG_CONTENT`, `CLAUDEMD_CONTENT` |
| Prior Outputs | `SPEC_CONTENT`, `RESEARCH_CONTENT`, `DESIGN_CONTENT`, `TDD_TESTS_CONTENT`, `BREAKDOWN_CONTENT`, `IMPLEMENT_CONTENT`, `IMPLEMENTATION_SUMMARY`, `OUTPUT_{PHASE}` |

**Context enrichment:** The executor calls `enrichContextForPhase()` before each phase to load phase-specific data (review findings, test results, etc.) into the resolution context.

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
- `internal/executor/workflow_execution.go` - THE executor, uses `Resolver.ResolveAll()` and `RenderTemplate()`
- `internal/workflow/` - Defines workflow variables stored in database

All template rendering goes through this package. The executor populates `ResolutionContext` via `buildResolutionContext()` and `enrichContextForPhase()`.
