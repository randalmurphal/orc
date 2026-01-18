# Orc System Audit Synthesis

Generated: 2026-01-18
Audits Completed: TASK-340, TASK-361, Executor System, Prompt Templates

---

## Executive Summary

Four deep audits revealed **systemic issues** in orc's execution pipeline:

| Category | Critical | High | Medium | Low |
|----------|----------|------|--------|-----|
| Code Architecture | 2 | 1 | 3 | 2 |
| Template System | 1 | 1 | 4 | 2 |
| Execution Logic | 1 | 2 | 2 | 1 |
| Task Quality | 1 | 0 | 2 | 2 |
| **Total** | **5** | **4** | **11** | **7** |

---

## CRITICAL ISSUES (Must Fix)

### C1: Template Variable Duplication (Two Renderers)
**Files:** `internal/executor/template.go` + `internal/executor/flowgraph_nodes.go`

**Problem:** Two separate implementations of template variable substitution must stay in sync. New variables added to one are silently lost in the other.

**Evidence:**
- `template.go:RenderTemplate()` - session-based executors
- `flowgraph_nodes.go:renderTemplate()` - legacy flowgraph
- No tests verify both produce identical output

**Impact:** Variables work in session mode but fail silently in flowgraph mode.

**Fix Required:**
1. Consolidate to single `RenderTemplate()` function
2. Delete `flowgraph_nodes.go:renderTemplate()`
3. Add test comparing both paths produce identical output
4. Document which executor path is used for each weight

**Test Coverage Needed:**
- Unit test: given identical TemplateVars, both renderers produce identical output
- Integration test: flowgraph execution uses consolidated renderer

---

### C2: Automation Context Lost in Flowgraph Path
**File:** `internal/executor/phase.go:104-116`

**Problem:** `LoadAutomationContext()` is never called in flowgraph execution path, but IS called in session executors.

**Evidence:**
```go
// standard.go:228-241 - DOES this:
if t.IsAutomation {
    if autoCtx := LoadAutomationContext(...); autoCtx != nil {
        vars = vars.WithAutomationContext(*autoCtx)
    }
}

// phase.go - DOES NOT do this
// Automation context is never injected
```

**Impact:** AUTO-XXX tasks lose recent task and changed file context when using flowgraph.

**Fix Required:**
1. Add `WithAutomationContext()` call to `phase.go:executePhaseWithFlowgraph()`
2. Mirror the logic from `standard.go:228-241`

**Test Coverage Needed:**
- Integration test: automation task in flowgraph mode receives automation context
- Unit test: `LoadAutomationContext()` returns expected data for AUTO-XXX tasks

---

### C3: Completion Detection Inconsistency
**Files:** `internal/executor/completion.go` vs `internal/executor/flowgraph_nodes.go:156`

**Problem:** Session executors use llmkit parser; flowgraph uses naive string matching.

**Evidence:**
```go
// Session (parser-based):
status, _ := CheckPhaseCompletion(response)

// Flowgraph (string match):
s.Complete = strings.Contains(s.Response, "<phase_complete>true</phase_complete>")
```

**Impact:** Malformed markers (whitespace variations) detected by parser, missed by string match.

**Fix Required:**
1. Replace flowgraph string matching with parser calls:
   ```go
   s.Complete = parser.IsPhaseComplete(s.Response)
   s.Blocked = parser.IsPhaseBlocked(s.Response)
   ```

**Test Coverage Needed:**
- Unit test: both paths detect `<phase_complete>true</phase_complete>`
- Unit test: both paths handle whitespace variations identically
- Unit test: both paths detect `<phase_blocked>` with content

---

### C4: Redundant Task Execution (No Deduplication)
**Evidence:** TASK-340 re-implemented code already merged in PR #208

**Problem:** When a task's implementation files already exist and pass tests, Claude:
1. Skips actual implementation
2. Runs verification
3. Claims completion
4. Creates empty commits

**Impact:** Wasted tokens, confusing git history, false completion markers.

**Fix Required:**
1. Add pre-execution check in implement phase:
   - If all "FILES TO CREATE" already exist
   - AND tests pass
   - THEN mark as "verification-only" or skip phase
2. Warn if phase commits have zero file changes
3. Add task state to track "already-implemented-elsewhere"

**Test Coverage Needed:**
- Integration test: task with pre-existing implementation is handled correctly
- Unit test: empty commit detection warns/fails

---

### C5: State/Plan Save Not Atomic (Race Condition)
**File:** `internal/executor/task_execution.go:194-199`

**Problem:** If `SaveState()` succeeds but `SavePlan()` fails, task status becomes inconsistent.

**Evidence:**
```go
if err := e.backend.SaveState(s); err != nil {
    return fmt.Errorf("save state: %w", err)
}
if err := e.backend.SavePlan(p, t.ID); err != nil {
    return fmt.Errorf("save plan: %w", err)  // State already saved!
}
```

**Impact:** State shows phase completed, plan shows phase running.

**Fix Required:**
1. Use database transaction for both saves
2. Or implement rollback on second save failure
3. Or use atomic multi-key update

**Test Coverage Needed:**
- Unit test: simulate SavePlan failure after SaveState success
- Verify state is rolled back or both fail together

---

## HIGH PRIORITY ISSUES

### H1: Conditional Syntax Undocumented
**Files:** `templates/review.md`, `templates/CLAUDE.md`

**Problem:** `{{#if REVIEW_ROUND_1}}` syntax used but never documented. No list of which variables support conditionals.

**Fix Required:**
1. Document conditional syntax in `templates/CLAUDE.md`
2. List all conditional variables
3. Show examples
4. Link to `processReviewConditionals()` implementation

**Test Coverage Needed:**
- Unit test: `processReviewConditionals()` handles Round 1 and Round 2
- Unit test: nested conditionals (if supported) work correctly

---

### H2: Transcript Extraction Returns Empty on Parse Failure
**File:** `internal/executor/template.go:543-560`

**Problem:** If structured patterns don't match, returns empty string instead of raw content.

**Evidence:**
```go
// Current behavior:
if no structured patterns match:
    return ""  // Lost entire transcript content

// Should be:
if no structured patterns match:
    return strings.TrimSpace(content)  // Return raw output
```

**Impact:** Prior phase content lost if format doesn't match expected patterns.

**Fix Required:**
1. Add raw content fallback after all structured patterns fail
2. Log warning when falling back to raw content

**Test Coverage Needed:**
- Unit test: malformed transcript returns raw content
- Unit test: well-formed transcript extracts structured content

---

### H3: Review Round Conditionals Not Tested
**File:** `internal/executor/template.go:269-291`

**Problem:** `processReviewConditionals()` uses regex that could break on nested blocks.

**Evidence:**
```go
round1Pattern := regexp.MustCompile(`(?s)\{\{#if REVIEW_ROUND_1\}\}(.*?)\{\{/if\}\}`)
// Nested blocks would break:
// {{#if REVIEW_ROUND_1}}
//   Content with {{#if REVIEW_ROUND_2}}nested{{/if}} blocks
// {{/if}}
```

**Fix Required:**
1. Add tests for nested conditional handling
2. Either document "no nesting" or implement proper parser

**Test Coverage Needed:**
- Unit test: simple conditionals work
- Unit test: nested conditionals either work or fail gracefully with clear error

---

### H4: No Validation of Backend at Startup
**File:** `internal/executor/executor.go`

**Problem:** If backend is nil, operations silently do nothing.

**Evidence:**
```go
func (v TemplateVars) WithSpecFromDatabase(backend, taskID string) TemplateVars {
    if backend == nil {
        return v  // Silent no-op
    }
    // ...
}
```

**Fix Required:**
1. Add validation in `executor.go:New()`
2. Fail fast if backend required but nil
3. Log warning if optional features disabled due to nil backend

**Test Coverage Needed:**
- Unit test: executor creation fails with helpful error when backend nil but required

---

## MEDIUM PRIORITY ISSUES

### M1: Worktree Safety Rules Duplicated
**Files:** 15+ templates

**Problem:** ~150 lines of identical safety rules copied across all templates.

**Fix Required:**
1. Extract to shared preamble or include mechanism
2. Single source of truth for safety rules

---

### M2: Artifact Storage Inconsistency Not Documented
**Problem:** Spec → database only, other phases → files. Not explained anywhere.

**Fix Required:**
1. Document in `executor/CLAUDE.md` with table showing where each phase outputs
2. Explain WHY spec is database-only (worktree merge conflicts)

---

### M3: `phase_blocked` Format Inconsistent
**Problem:** Different XML structures across templates.

**Fix Required:**
1. Define strict XML schema for all phase markers
2. Add validation logic

---

### M4: Review.md Over-Complex (745 lines)
**Problem:** 5 agent prompts embedded inline, hard to maintain.

**Fix Required:**
1. Extract agent prompts to separate files in `templates/review/`
2. Makes agents independently maintainable

---

### M5: Retry Context Doesn't Escape Markdown
**File:** `internal/executor/retry.go:84-112`

**Problem:** If reason contains backticks or special chars, markdown breaks.

**Fix Required:**
1. Escape markdown special characters in reason/output
2. Or use code block for entire context

---

### M6: Coverage Threshold Variable May Not Exist
**Files:** `templates/test.md`, `templates/validate.md`

**Problem:** If `{{COVERAGE_THRESHOLD}}` not set, templates have broken output.

**Fix Required:**
1. Add default value (85) in `BuildTemplateVars()`
2. Or document as required variable

---

### M7: Placeholder Implementations in statsStore
**File:** `web/src/stores/statsStore.ts`

**Problem:** `topInitiatives` and `topFiles` hardcoded empty (documented).

**Fix Required:**
1. Create backend API endpoints
2. Wire up to store when ready
3. Track as tech debt if not immediate priority

---

### M8: Token Tracking Misleading for Cache
**Problem:** `effectiveInput = InputTokens + CacheCreationTokens + CacheReadTokens` overestimates.

**Fix Required:**
1. Calculate actual cost: `InputTokens + (CacheReadTokens * 0.1)`
2. Or document current behavior

---

## LOW PRIORITY ISSUES

### L1: Session ID Generation Not Cryptographically Secure
- Acceptable for session tracking, document if security concerns arise

### L2: localStorage Key Prefix Minimal (`orc-`)
- Consider more unique prefix if conflicts possible

### L3: No API Error Differentiation in statsStore
- Could distinguish network/404/500 errors for better debugging

### L4: Phase Naming Inconsistency
- `qa.md` called "QA Session" not "QA Phase"

### L5: updateMetrics Logic Could Be Clearer
- Partial update recalculation behavior could be documented

### L6: Success Rate Rounding Precision
- Document rounding logic in comments

### L7: No Test for WebSocket Integration
- sessionStore WebSocket integration only documented, not tested

---

## Architecture Improvements

### A1: Consolidate Execution Paths
**Current:** Two paths (session-based + flowgraph) with duplicated logic.

**Recommended:**
1. Deprecate flowgraph path OR
2. Abstract shared logic into common functions
3. Single source of truth for template rendering, completion detection, context injection

### A2: Structured Phase Output Schema
**Current:** Each phase outputs different formats, parsed with regex.

**Recommended:**
1. Define JSON/XML schema for phase outputs
2. Structured parsing instead of regex extraction
3. Validation of output format before saving

### A3: Task Deduplication System
**Current:** No check for already-implemented work.

**Recommended:**
1. Pre-execution check: do implementation files exist?
2. Pre-execution check: do tests pass?
3. If both true, mark as "verification-only" or skip

### A4: Atomic State Management
**Current:** State and plan saved separately.

**Recommended:**
1. Transaction-based saves
2. Consistent state across crash/restart

---

## Test Coverage Gaps

| Area | Current | Needed |
|------|---------|--------|
| Template rendering paths | None | Compare session vs flowgraph output |
| Completion detection | Partial | Both paths, whitespace variations |
| Automation context injection | None | Flowgraph path receives context |
| Conditional processing | None | Round 1, Round 2, nested blocks |
| Transcript extraction fallback | None | Malformed content handling |
| State/plan atomicity | None | Failure scenarios |
| Backend validation | None | Nil backend handling |
| Empty commit detection | None | Warn on zero-change commits |

---

## Implementation Order

### Phase 1: Critical Fixes (Block other work)
1. C1: Consolidate template renderers
2. C2: Add automation context to flowgraph
3. C3: Unify completion detection
4. C5: Atomic state/plan saves

### Phase 2: High Priority (Quality gates)
1. H1: Document conditionals
2. H2: Transcript extraction fallback
3. H3: Test review conditionals
4. H4: Backend validation

### Phase 3: Medium Priority (Tech debt)
1. M1: Extract worktree safety rules
2. M2: Document artifact storage
3. M3-M8: Various fixes

### Phase 4: Architecture (Long-term)
1. A1: Consolidate execution paths
2. A2: Structured phase output schema
3. A3: Task deduplication system
4. A4: Atomic state management

---

## Files to Modify

| File | Changes |
|------|---------|
| `internal/executor/flowgraph_nodes.go` | Remove `renderTemplate()`, use parser for completion |
| `internal/executor/phase.go` | Add `WithAutomationContext()` |
| `internal/executor/template.go` | Add fallback for transcript extraction |
| `internal/executor/task_execution.go` | Make saves atomic |
| `internal/executor/executor.go` | Validate backend at startup |
| `internal/executor/retry.go` | Escape markdown in context |
| `templates/CLAUDE.md` | Document conditionals, artifact storage |
| `templates/*.md` | Extract common safety rules |

---

## Retrospective Questions

1. **Why didn't existing tests catch template duplication?**
   - No test compares both rendering paths

2. **Why did TASK-340 re-implement existing code?**
   - No deduplication check before execution

3. **Why is automation context missing in flowgraph?**
   - Feature added to session executors, flowgraph not updated

4. **Why are conditionals undocumented?**
   - Implemented for review phase, docs not updated

5. **Why can state become inconsistent?**
   - Sequential saves without transaction
