# Specification: Flowgraph executor renderTemplate missing template variables

## Problem Statement

The flowgraph executor's `renderTemplate()` method in `flowgraph_nodes.go` is missing 12 template variables that exist in `template.go:RenderTemplate()`. This causes prompts sent to Claude to contain literal `{{TASK_CATEGORY}}`, `{{INITIATIVE_CONTEXT}}`, etc. instead of substituted values, degrading task execution quality.

## Success Criteria

- [ ] All template variables from `RenderTemplate()` in `template.go` are also handled in `Executor.renderTemplate()` in `flowgraph_nodes.go`
- [ ] `PhaseState` struct includes fields for all template variables (task category, initiative context, UI testing context, coverage threshold, verification results)
- [ ] `PhaseState` initialization in `phase.go` populates all new fields from `templateVars`
- [ ] Templates using `{{TASK_CATEGORY}}` receive the actual category value (bug, feature, etc.)
- [ ] Templates using `{{INITIATIVE_CONTEXT}}` receive formatted initiative context when task belongs to an initiative
- [ ] Tests pass: `make test` succeeds with no new failures
- [ ] Existing template rendering behavior is preserved (no regressions)

## Testing Requirements

- [ ] Unit test: `TestRenderTemplateWithCategory` - verify `{{TASK_CATEGORY}}` substitution
- [ ] Unit test: `TestRenderTemplateWithInitiativeContext` - verify all 5 initiative variables
- [ ] Unit test: `TestRenderTemplateWithUITestingContext` - verify `{{REQUIRES_UI_TESTING}}`, `{{SCREENSHOT_DIR}}`, `{{TEST_RESULTS}}`
- [ ] Unit test: `TestRenderTemplateWithCoverageThreshold` - verify `{{COVERAGE_THRESHOLD}}`
- [ ] Unit test: `TestRenderTemplateWithVerificationResults` - verify `{{VERIFICATION_RESULTS}}`
- [ ] Regression test: Existing `TestRenderTemplate*` tests continue to pass

## Scope

### In Scope

- Add missing fields to `PhaseState` struct to match `TemplateVars`
- Sync `Executor.renderTemplate()` variable map with `RenderTemplate()`
- Update `PhaseState` initialization in `phase.go` to populate new fields
- Add unit tests for new template variables
- Include helper function for `{{INITIATIVE_CONTEXT}}` (the formatted section)

### Out of Scope

- Refactoring to use a single shared implementation (future improvement)
- Automation context variables (`{{RECENT_COMPLETED_TASKS}}`, `{{RECENT_CHANGED_FILES}}`, `{{CHANGELOG_CONTENT}}`, `{{CLAUDEMD_CONTENT}}`) - only used by automation tasks which use different executor path
- Changes to `template.go` or how standard executors work
- Changes to prompt templates themselves

## Technical Approach

### Strategy: Sync the Two Implementations

Rather than refactoring to a single implementation (higher risk, larger change), sync the missing variables to `renderTemplate()`. This is the minimal fix for the immediate problem.

### Missing Variables Analysis

| Variable | In template.go | In flowgraph_nodes.go | Action |
|----------|---------------|----------------------|--------|
| `{{TASK_CATEGORY}}` | Yes | **No** | Add |
| `{{INITIATIVE_ID}}` | Yes | **No** | Add |
| `{{INITIATIVE_TITLE}}` | Yes | **No** | Add |
| `{{INITIATIVE_VISION}}` | Yes | **No** | Add |
| `{{INITIATIVE_DECISIONS}}` | Yes | **No** | Add |
| `{{INITIATIVE_CONTEXT}}` | Yes | **No** | Add (uses helper) |
| `{{COVERAGE_THRESHOLD}}` | Yes | **No** | Add |
| `{{REQUIRES_UI_TESTING}}` | Yes | **No** | Add |
| `{{SCREENSHOT_DIR}}` | Yes | **No** | Add |
| `{{TEST_RESULTS}}` | Yes | **No** | Add |
| `{{VERIFICATION_RESULTS}}` | Yes | **No** | Add |

### Files to Modify

1. **`internal/executor/executor.go`** (lines 28-66):
   - Add new fields to `PhaseState` struct:
     - `TaskCategory string`
     - `InitiativeID string`
     - `InitiativeTitle string`
     - `InitiativeVision string`
     - `InitiativeDecisions string`
     - `CoverageThreshold int`
     - `RequiresUITesting bool`
     - `ScreenshotDir string`
     - `TestResults string`
     - `VerificationResults string`

2. **`internal/executor/flowgraph_nodes.go`** (lines 46-79):
   - Add all missing variables to the `replacements` map in `renderTemplate()`
   - Add local helper function or import for `{{INITIATIVE_CONTEXT}}` formatting (reuse logic from `formatInitiativeContextSection`)

3. **`internal/executor/phase.go`** (lines 112-127):
   - Update `PhaseState` initialization to populate new fields from `templateVars`:
     - `TaskCategory: string(t.Category)`
     - `InitiativeID: templateVars.InitiativeID`
     - etc.
   - Load initiative context if task has `InitiativeID`
   - Load UI testing context if task has `RequiresUITesting`

4. **`internal/executor/executor_test.go`**:
   - Add new test functions for category, initiative context, UI testing, coverage threshold, and verification results

## Bug Analysis

### Reproduction Steps
1. Create a task with category `bug`: `orc new "Fix template issue" -c bug`
2. Add the task to an initiative: `orc edit TASK-XXX --initiative INIT-001`
3. Run the task: `orc run TASK-XXX`
4. Examine transcript in `.orc/tasks/TASK-XXX/transcripts/`
5. Observe literal `{{TASK_CATEGORY}}` and `{{INITIATIVE_CONTEXT}}` in prompts

### Current Behavior

Prompts contain unsubstituted placeholders:
```
**Category**: {{TASK_CATEGORY}}

{{INITIATIVE_CONTEXT}}
```

### Expected Behavior

Prompts contain substituted values:
```
**Category**: bug

## Initiative Context

This task is part of **React Migration** (INIT-001).

### Vision
Migrate frontend from Svelte to React 19...
```

### Root Cause

Two parallel implementations:
- `template.go:RenderTemplate()` - complete, used by session-based executors
- `flowgraph_nodes.go:renderTemplate()` - incomplete, used by flowgraph executor

The flowgraph executor was created earlier and never updated when new variables were added to the template system.

### Verification

After fix:
1. Run a task with category set
2. Run a task belonging to an initiative
3. Verify transcripts show substituted values, not literal placeholders
