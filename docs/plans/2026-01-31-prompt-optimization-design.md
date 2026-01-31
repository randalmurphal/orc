# Prompt Optimization & Agent Quality Overhaul

**Date:** 2026-01-31
**Status:** Design
**Scope:** All phase prompts, system prompts, sub-agent definitions, agent loader, init wizard, config, frontend

## Problem

Orc's ~4,857 lines of prompt content across 50+ files have accumulated without systematic optimization. Research against Anthropic's latest prompting best practices reveals:

1. **Top 3 failure modes** (user-reported): vague specs, incomplete implementations, over-engineering
2. **Zero multishot examples** — Anthropic's #2 most effective technique, completely unused
3. **Information ordering is backwards** — output format buried at bottom, context at top
4. **Generic engineering advice wastes tokens** — prompts teach Claude what Claude already knows
5. **Sub-agent prompts can't access template variables** — they're passed as raw text, never rendered
6. **Agents have hardcoded Anthropic-internal patterns** — `constants/errorIds.ts`, Sentry, Statsig, ES modules
7. **No dedicated agents for top failure modes** — nothing catches vague specs or over-engineering

## Goals

- Maximize output quality from every phase (spec, TDD, implement, review)
- Make agents language-agnostic and project-adaptive (orc orchestrates ANY project)
- Add multishot examples to every phase prompt
- Restructure prompts following Anthropic's recommended information ordering
- Add two new agents targeting the #1 and #3 failure modes
- Extend init wizard to capture language-specific error patterns (auto-defaulted, user-editable)

## Non-Goals

- Changing the two-tier system prompt / phase prompt architecture (stays as-is)
- Changing the output JSON schemas (downstream parsers depend on them)
- Rewriting the workflow engine or phase execution pipeline
- Changing the agent seeding or database-first architecture

---

## Part 1: Enable Template Variables in Sub-Agent Prompts

### Problem

Sub-agent prompts go from DB → raw JSON → Claude CLI `--agents` flag. No `{{VARIABLE}}` substitution happens. This is why agents have hardcoded language-specific patterns — there was never a mechanism to inject project context.

### Changes

**File: `internal/executor/agent_loader.go`**

Current (line 19-26):
```go
func ToInlineAgentDef(a *db.Agent) InlineAgentDef {
    return InlineAgentDef{
        Description: a.Description,
        Prompt:      a.Prompt,      // Raw, unrendered
        Tools:       a.Tools,
        Model:       a.Model,
    }
}
```

Change to:
```go
func ToInlineAgentDef(a *db.Agent, vars map[string]string) InlineAgentDef {
    return InlineAgentDef{
        Description: a.Description,
        Prompt:      variable.RenderTemplate(a.Prompt, vars),
        Tools:       a.Tools,
        Model:       a.Model,
    }
}
```

**File: `internal/executor/agent_loader.go` — `LoadPhaseAgents()` (line 30-48)**

Add `vars map[string]string` parameter. Pass through to `ToInlineAgentDef()`.

**File: `internal/executor/workflow_phase.go` (line 131-146)**

Pass the already-built `vars` map to `LoadPhaseAgents()`:
```go
phaseAgents, err := LoadPhaseAgents(we.globalDB, tmpl.ID, rctx.TaskWeight, vars)
```

The `vars` map already exists at this point — it's built by `buildResolutionContext()` and used for phase prompt rendering on line 117.

### Variables Now Available to Agents

| Variable | Example Value | Use Case |
|----------|--------------|----------|
| `{{LANGUAGE}}` | `go` | Language-specific review patterns |
| `{{FRAMEWORKS}}` | `cobra, grpc` | Framework-aware analysis |
| `{{TEST_COMMAND}}` | `make test` | Test execution in agents |
| `{{CONSTITUTION_CONTENT}}` | Project principles | Project-specific conventions |
| `{{ERROR_PATTERNS}}` | Language error idioms | Error handling review (new, see Part 4) |
| `{{SPEC_CONTENT}}` | The spec | Spec compliance checking |

---

## Part 2: Phase Prompt Restructuring

### New Information Ordering (all prompts)

Anthropic's recommended priority ordering applied to orc's phase prompts:

```
1. OUTPUT FORMAT         — What you're producing (JSON schema, artifact structure)
2. CRITICAL CONSTRAINTS  — Top 2-3 failure modes for this phase
3. INJECTED ARTIFACTS    — Spec, TDD tests, breakdown, initiative context
4. CONTEXT/METADATA      — Task ID, project detection, worktree safety
5. INSTRUCTIONS          — How to approach the work (streamlined)
6. EXAMPLE               — One condensed example of good output
```

### Spec Phase (`templates/prompts/spec.md` — currently 513 lines)

**Target: ~300 lines** (remove generic advice, add example)

| Section | Current | Change |
|---------|---------|--------|
| Output format | Lines 393-513 (bottom) | Move to top |
| Quality checklist | Lines 345-360 (Step 11 of 12) | Move to position 2, right after output format |
| Initiative alignment | Lines 38-53 | Keep, move to critical constraints |
| Referenced files study | Lines 55-66 (40 lines, preemptive) | Condense to 3 lines: "Read every referenced file. Extract behavioral requirements. Cross-reference against your success criteria." |
| Steps 1-2 (analyze requirements, project context) | Lines 77-118 | Condense — Claude knows how to analyze requirements |
| Steps 3-4 (user stories, success criteria) | Lines 133-188 | Keep success criteria rules, trim user story scaffolding |
| Step 4b (behavioral specs) | Lines 191-215 | Keep but make optional/shorter |
| Steps 5-7 (testing, scope, technical approach) | Lines 217-258 | Condense to constraint-only: "Include testing requirements, explicit scope, and technical approach" |
| Step 8 (category-specific) | Lines 262-319 | Keep bug analysis (Pattern Prevalence is valuable), trim feature/refactor sections |
| Steps 9-10 (failure modes, edge cases) | Lines 322-343 | Condense — tables are good, explanatory text is generic |
| Steps 11-12 (checklists) | Lines 345-390 | Quality checklist → top. Review checklist → cut (reviewer handles this) |
| **NEW: Example** | N/A | Add ~40 line condensed example of a sharp spec |

**Key content cuts:**
- Verification method type descriptions (lines 160-166) — Claude knows what `go test` is
- Feature Replacement Policy (lines 122-131) — generic advice, 10 lines
- Integration Requirements explanatory text (lines 244-258) — keep the table template, cut the "Rules" and "Mandatory Questions"
- Entire Step 12 Review Checklist (lines 362-390) — this duplicates what the reviewer does

**Key content additions:**
- Condensed example spec (~40 lines) showing sharp success criteria with concrete verification methods
- Failure mode callout: "The most common failure is success criteria that verify existence ('file exists') instead of behavior ('file does X when given Y'). Every SC must describe observable behavior."

### TDD Phase (`templates/prompts/tdd_write.md` — currently 303 lines)

**Target: ~200 lines**

| Section | Current | Change |
|---------|---------|--------|
| Output format | Lines 249-303 (bottom) | Move to top |
| Pre-output verification | Lines 224-247 (bottom) | Move to position 2 |
| Test classification tables | Lines 86-148 | Cut basic definitions (solitary/sociable/integration descriptions), keep wiring verification pattern and embedded code testing |
| Critical mindset DO/DON'T | Lines 38-56 | Good, keep |
| Test isolation section | Lines 58-70 | Generic mocking advice — condense to 2 lines |
| Error path testing | Lines 72-84 | Generic — condense to "Test error paths per spec's Failure Modes table" |
| Steps 1-3 | Lines 150-221 | Keep Step 1 (analyze SC), condense Step 2 (Claude knows how to write tests), keep Step 3 (verify tests fail) |
| **NEW: Example** | N/A | Add ~30 line example: one SC → one test → coverage mapping |

### Implement Phase (`templates/prompts/implement.md` — currently 344 lines)

**Target: ~200 lines**

| Section | Current | Change |
|---------|---------|--------|
| Output format + verification | Lines 280-343 (bottom) | Move to top |
| Pre-completion checklist | Lines 280-303 | Move to position 2: "Before claiming done, all of these must be true" |
| Impact analysis with grep examples | Lines 86-141 (~55 lines) | Replace with 2 lines: "Before modifying shared code, identify all callers and dependents. Before claiming completion, verify all new code is reachable from production code paths." |
| Steps 4-6 (implement, edge cases, error handling) | Lines 125-198 | Condense to: "Implement fully — no TODOs, no placeholders. Handle edge cases and errors per the spec." |
| Over-engineering guard | Lines 222-232 | Strengthen: "If you find yourself creating a helper, utility, or abstraction the spec didn't request — stop. Delete it. Implement exactly what was specified." |
| **NEW: Failure mode callout** | N/A | "The most common failure is declaring completion without running verification. If you haven't run `{{TEST_COMMAND}}` and seen all tests pass, you are not done." |
| **NEW: Example** | N/A | Add ~20 line example of a completion JSON with real verification evidence |

### Review Phase (`templates/prompts/review.md` — currently 212 lines)

**Target: ~180 lines** (lightest touch — already well-structured)

| Section | Change |
|---------|--------|
| Bash command examples (lines 47-52, 63-67, 129-135) | Cut — review agent knows how to check builds |
| Spec compliance section (lines 86-102) | Strengthen: "If success criteria are vague or untestable, this is a blocking finding — the spec phase failed." |
| **NEW: Over-engineering check** | Add to "What to Check": "Did the implementation add functionality, abstractions, or error handling beyond what the spec requested?" |
| Decision tree (lines 199-211) | Keep as-is — it's excellent |

### Review Round 1 (`templates/prompts/review_round1.md` — 111 lines)

Minimal changes. Move output format closer to top. The multi-agent parallel review structure is good.

### Review Round 2 (`templates/prompts/review_round2.md` — 123 lines)

Minimal changes. Same restructuring pattern.

---

## Part 3: System Prompt Updates

System prompts are short (20-32 lines each) and define WHO the agent is. Light touch.

### `system_prompts/implement.md` (30 lines)

**Add anti-over-engineering strengthening:**

Current `<avoid_over_engineering>` (lines 11-13):
```
Only make changes directly requested or clearly necessary. Keep solutions simple.
Don't add features, refactoring, or "improvements" beyond the spec.
```

Replace with:
```
Only make changes directly requested or clearly necessary.
If you find yourself creating a helper function, utility class, or abstraction
that the spec didn't ask for — stop and delete it.
Do not add error handling for scenarios that can't occur.
Do not design for hypothetical future requirements.
The right complexity is the minimum needed for the current task.
```

### `system_prompts/review.md` (32 lines)

**Add over-engineering detection to review focus:**

Add to `<review_focus>` (line 7-12):
```
- Over-engineering: does the implementation exceed the spec's requested scope?
```

### All other system prompts

No changes needed. They're well-scoped and language-agnostic already.

---

## Part 4: Sub-Agent Rewrites (Language-Agnostic)

All agents get a new `<project_context>` block that uses template variables:

```markdown
<project_context>
Language: {{LANGUAGE}}
Frameworks: {{FRAMEWORKS}}
Test Command: {{TEST_COMMAND}}

{{CONSTITUTION_CONTENT}}

Adapt your analysis to the project's language, conventions, and standards above.
Read the project's CLAUDE.md files for additional project-specific coding standards.
</project_context>
```

### `code-reviewer.md` (currently 48 lines)

| Change | Detail |
|--------|--------|
| Add `<project_context>` block | Inject language/constitution variables |
| Remove hardcoded "CLAUDE.md" references | Replace with "project conventions from the context above" |
| Add over-engineering check | "Flag unrequested abstractions, helpers, or error handling beyond the spec" |
| Keep confidence scoring | 0-100 scale, report ≥80 only — this is good |

### `code-simplifier.md` (currently 54 lines)

| Change | Detail |
|--------|--------|
| Strip all JS/TS-specific patterns | Remove: ES modules, `function` keyword preference, React Props types, arrow functions |
| Add `<project_context>` block | Language-adaptive simplification |
| Rewrite "Apply Project Standards" section | From hardcoded JS rules to: "Follow the project's established coding standards from the context above" |
| Keep balance principle | "Avoid over-simplification... explicit code is often better than overly compact code" |

### `silent-failure-hunter.md` (currently 131 lines in orc, 130 in Anthropic)

| Change | Detail |
|--------|--------|
| Strip Anthropic-internal references | Remove: `logForDebugging`, `logError (Sentry)`, `logEvent (Statsig)`, `constants/errorIds.ts` |
| Add `<project_context>` block | Inject language/constitution/error patterns |
| Make language examples polymorphic | Current: "try-catch blocks (or try-except in Python, Result types in Rust, etc.)" — keep this, it's already good |
| Replace "Special Considerations" section (lines 121-128) | From hardcoded Anthropic patterns to: "Review the project conventions above for project-specific error handling rules, logging conventions, and error tracking requirements." |
| Add `{{ERROR_PATTERNS}}` injection | New variable with language-specific error idioms |

### `pr-test-analyzer.md` (currently from Anthropic's toolkit)

Already mostly language-agnostic. Add `<project_context>` block. No other changes needed.

### `comment-analyzer.md`

Already language-agnostic. Add `<project_context>` block for project conventions awareness.

### `type-design-analyzer.md`

Already language-agnostic. Add `<project_context>` block. Consider adding language-specific type system notes: "For Go: check interface satisfaction and struct embedding. For TypeScript: check generic constraints and discriminated unions."

---

## Part 5: New Agents

### `spec-quality-auditor`

**Purpose:** Catches vague specs before they cascade to TDD/implement/review. Addresses failure mode #1.

**Phase association:** Spec phase, sequence 1 (runs after main spec generation).

```markdown
---
name: spec-quality-auditor
description: Reviews specification quality by checking that all success criteria are behavioral, testable, and concrete. Use after spec generation to catch vague or existential-only criteria before they cascade downstream.
model: inherit
tools: ["Read", "Grep", "Glob"]
---

You are a specification quality auditor. You review specs AFTER they're written,
checking for the specific failure modes that cause bad implementations.

<project_context>
Language: {{LANGUAGE}}
Test Command: {{TEST_COMMAND}}
</project_context>

## What You Check

For each success criterion (SC-X):

1. **Behavioral vs existential?**
   - FAIL: "File exists on disk", "Record created in DB", "Function is defined"
   - PASS: "File blocks first stop attempt (exit 2)", "API returns 200 with user ID", "Function returns sorted list"

2. **Verification produces binary pass/fail?**
   - FAIL: "Manual review", "Check that it works", "Verify correctness"
   - PASS: "Run `{{TEST_COMMAND}}`, expect 0 failures", "curl endpoint, expect HTTP 200"

3. **Expected result is concrete?**
   - FAIL: "Works correctly", "Handles errors properly", "Is performant"
   - PASS: "Returns HTTP 200 with JSON body containing 'id' field", "Completes in <500ms"

4. **Integration is scoped?**
   - For any new code: is wiring into existing code paths explicitly in scope?
   - For any new function: is there a caller identified?

## Output

For each SC-X, rate: SHARP / VAGUE / EXISTENTIAL-ONLY

```json
{
  "status": "complete",
  "summary": "Reviewed N criteria: X sharp, Y vague, Z existential-only",
  "findings": [
    {"criterion": "SC-1", "rating": "SHARP", "reason": "Concrete verification with expected output"},
    {"criterion": "SC-2", "rating": "VAGUE", "reason": "Says 'handles errors properly' without defining what that means", "suggestion": "Specify: returns HTTP 400 with error message when input is empty"}
  ],
  "recommendation": "pass" | "block"
}
```

Block if any SC is EXISTENTIAL-ONLY or if >1 SC is VAGUE.
```

### `over-engineering-detector`

**Purpose:** Catches scope creep and unnecessary abstractions. Addresses failure mode #3.

**Phase association:** Implement phase, sequence 1 (runs after implementation, before review).

```markdown
---
name: over-engineering-detector
description: Detects code that exceeds specification scope — unrequested abstractions, unnecessary error handling, future-proofing, and file proliferation. Use after implementation to catch over-engineering before review.
model: inherit
tools: ["Read", "Grep", "Glob"]
---

You detect implementations that exceed what the specification requested.

<project_context>
Language: {{LANGUAGE}}
</project_context>

## What You Check

Review the git diff against the spec's success criteria and scope sections.

1. **Unrequested abstractions**
   - Helper functions or utility classes not in the spec
   - Interfaces with only one implementation
   - Generic solutions where a specific one was requested
   - Ask: "Did the spec ask for this? Would removing it break any SC?"

2. **Unnecessary error handling**
   - Try/catch for scenarios that can't occur given the calling context
   - Validation of internal values that are already validated upstream
   - Defensive nil checks on values guaranteed non-nil by construction
   - Ask: "What realistic scenario triggers this error path?"

3. **Future-proofing**
   - Configurability that wasn't requested ("just in case")
   - Extension points, plugin architectures, or strategy patterns for one case
   - Parameters that are always passed the same value
   - Ask: "Is there a second use case for this flexibility today?"

4. **File proliferation**
   - New files that could have been additions to existing files
   - Constants files for a single constant
   - Types files for a single type
   - Ask: "Could this live in an existing file without harming clarity?"

5. **Scope creep**
   - Changes to files or functions not mentioned in the spec
   - Refactoring of existing code that wasn't broken
   - "While I'm here" improvements

## Output

```json
{
  "status": "complete",
  "summary": "Found N over-engineering concerns (X high, Y medium, Z low)",
  "findings": [
    {
      "file": "path/to/file.go",
      "line": 42,
      "severity": "HIGH",
      "type": "unrequested_abstraction",
      "description": "Created ConfigManager interface with only one implementation",
      "spec_reference": "Not in any SC or scope section",
      "suggestion": "Remove interface, use concrete type directly"
    }
  ],
  "recommendation": "pass" | "flag"
}
```

Severity: HIGH (new abstraction/file/interface), MEDIUM (extra error handling/validation), LOW (minor extras).
Flag (don't block) if findings are HIGH. Implementation can proceed but findings feed into review.
```

---

## Part 6: Language Error Patterns in Config + Init

### Config Schema Change

**File: `internal/config/config_types.go`**

Add to an appropriate config struct (likely a new `ProjectDetectionConfig` or extend `DeveloperConfig`):

```go
// LanguagePatternsConfig holds language-specific patterns for agent context injection.
type LanguagePatternsConfig struct {
    // ErrorPatterns describes language-specific error handling idioms.
    // Auto-detected during init, user-editable.
    // Injected into agents as {{ERROR_PATTERNS}}.
    ErrorPatterns string `yaml:"error_patterns,omitempty"`
}
```

### Init Wizard Change

**File: `internal/cli/init_wizard.go`**

Add a step to the wizard after language detection (line 58-65 area) that:
1. Auto-generates error patterns based on detected language using a lookup table
2. Shows the generated patterns to the user as a default
3. Allows the user to edit/override in a textarea step

Lookup table (embedded in init or in a separate file):

| Language | Default Error Patterns |
|----------|----------------------|
| `go` | "Always check error returns with `if err != nil`. Wrap errors with context: `fmt.Errorf(\"context: %w\", err)`. Never discard errors with `_` in production. Use `errors.Is`/`errors.As` for comparison." |
| `python` | "Use specific exception types, never bare `except`. Log with `logger.exception()` for stack traces. Use `contextlib.suppress` only for documented expected cases." |
| `typescript` | "Avoid broad `catch(e)` — catch specific error types. Never swallow errors in empty catch blocks. Use typed error responses at API boundaries." |
| `rust` | "Use `?` operator for propagation. Use `thiserror` for library errors, `anyhow` for application errors. Never `.unwrap()` in production code." |
| `java` | "Catch specific exceptions, never bare `Exception`. Always log with context. Use try-with-resources for closeable resources." |

### Variable Registration

**File: `internal/variable/resolver.go` — `addBuiltinVariables()` (line 288-320)**

Add:
```go
if rctx.ErrorPatterns != "" {
    vars["ERROR_PATTERNS"] = rctx.ErrorPatterns
}
```

**File: `internal/executor/workflow_context.go` — `buildResolutionContext()`**

Load error patterns from config:
```go
if cfg.LanguagePatterns.ErrorPatterns != "" {
    rctx.ErrorPatterns = cfg.LanguagePatterns.ErrorPatterns
}
```

---

## Part 7: Frontend Changes

### 7a: Error Patterns Editor

**File: `web/src/pages/environment/Config.tsx`**

Add a new accordion section "Language Patterns" after the existing "Claude" section:

```tsx
<Accordion.Item value="language_patterns">
    <Accordion.Header>
        <Accordion.Trigger>Language Patterns</Accordion.Trigger>
    </Accordion.Header>
    <Accordion.Content>
        <div className="config-field">
            <label>Error Handling Patterns</label>
            <p className="settings-section-description">
                Language-specific error handling idioms injected into review agents.
                Auto-detected during init. Edit to match your project's conventions.
            </p>
            <textarea
                className="settings-textarea"
                value={formData.languagePatterns?.errorPatterns || ''}
                onChange={(e) => handleChange('languagePatterns', 'errorPatterns', e.target.value)}
                rows={6}
            />
        </div>
    </Accordion.Content>
</Accordion.Item>
```

Requires adding `languagePatterns` to the config protobuf schema and the form state.

### 7b: Agent Prompt Preview (Optional Enhancement)

**File: `web/src/components/agents/AgentsView.tsx`**

Add a "View Prompt" button to each agent card that opens a modal showing the rendered prompt (with template variables resolved for the current project). This helps users understand what their agents will actually say.

This is a nice-to-have, not blocking.

---

## Part 8: Multishot Examples

Each phase prompt gets one condensed example. These are the highest-leverage additions.

### Spec Phase Example (~40 lines)

```markdown
<example_good_spec>
# Specification: Add rate limiting to API endpoints

## Problem Statement
API endpoints have no rate limiting, allowing abuse and resource exhaustion.

## User Stories
| Priority | Story | Success Criteria |
|----------|-------|------------------|
| P1 (MVP) | As an API consumer, I want rate limits so the service stays available | SC-1, SC-2, SC-3 |

## Success Criteria
| ID | Criterion | Verification | Expected Result | Error Path |
|----|-----------|-------------|-----------------|------------|
| SC-1 | Rate limiter returns 429 after limit exceeded | `curl -X GET /api/tasks -H "X-Test-Rate: burst" && echo $?` | HTTP 429 with Retry-After header | Client receives clear "rate limit exceeded" message |
| SC-2 | Rate limit resets after window expires | `sleep 61 && curl -X GET /api/tasks` | HTTP 200 (limit reset) | N/A |
| SC-3 | Rate limiter middleware is wired into router | `grep -r "rateLimiter" internal/api/` | Found in router setup | Build fails if middleware not imported |

## Scope
### In Scope
- Token bucket rate limiter middleware
- Per-IP rate limiting with configurable limits
- 429 response with Retry-After header

### Out of Scope
- Per-user rate limiting (requires auth, separate task)
- Rate limit dashboard or monitoring
- Distributed rate limiting (single-instance only)
</example_good_spec>
```

### TDD Phase Example (~25 lines)

```markdown
<example_good_tdd>
SC-1 from spec: "Rate limiter returns 429 after limit exceeded"

Test (solitary):
```go
func TestRateLimiter_Returns429AfterLimitExceeded(t *testing.T) {
    limiter := NewRateLimiter(Config{MaxRequests: 5, Window: time.Minute})
    handler := limiter.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(200)
    }))

    // First 5 requests succeed
    for i := 0; i < 5; i++ {
        rec := httptest.NewRecorder()
        handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
        assert.Equal(t, 200, rec.Code)
    }

    // 6th request is rate limited
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
    assert.Equal(t, 429, rec.Code)
    assert.NotEmpty(t, rec.Header().Get("Retry-After"))
}
```

Coverage: SC-1 → TestRateLimiter_Returns429AfterLimitExceeded (solitary)
</example_good_tdd>
```

### Implement Phase Example (~15 lines)

```markdown
<example_good_completion>
```json
{
  "status": "complete",
  "summary": "Implemented token bucket rate limiter middleware with per-IP tracking and 429 responses",
  "verification": {
    "tests_passed": true,
    "test_output": "ok  github.com/project/internal/middleware  0.023s (5 tests, 0 failures)",
    "build_passed": true,
    "lint_passed": true
  }
}
```
Only output this after actually running the tests and seeing them pass.
</example_good_completion>
```

### Review Phase

No example needed — the three-outcome decision tree and severity guide are already effective.

---

## Implementation Order

| Step | Scope | Files Changed | Depends On |
|------|-------|--------------|------------|
| 1 | Template rendering for sub-agents | `agent_loader.go`, `workflow_phase.go` | Nothing |
| 2 | Rewrite agents as language-agnostic | 7 agent `.md` files | Step 1 |
| 3 | Add new agents (spec-quality-auditor, over-engineering-detector) | 2 new agent `.md` files, `seed_agents.go` | Step 1 |
| 4 | Restructure spec prompt | `templates/prompts/spec.md` | Nothing |
| 5 | Restructure TDD prompt | `templates/prompts/tdd_write.md` | Nothing |
| 6 | Restructure implement prompt | `templates/prompts/implement.md` | Nothing |
| 7 | Restructure review prompt | `templates/prompts/review.md` | Nothing |
| 8 | Update system prompts | 2 system prompt `.md` files | Nothing |
| 9 | Add error_patterns to config + init wizard | `config_types.go`, `init_wizard.go`, `resolver.go`, `workflow_context.go` | Nothing |
| 10 | Frontend: error patterns editor | `Config.tsx`, protobuf schema | Step 9 |

Steps 1-3 are sequential (each depends on prior). Steps 4-9 are independent of each other and can be parallelized. Step 10 depends on 9.

## Task Breakdown for Orc

| Task | Weight | Description |
|------|--------|-------------|
| Enable sub-agent template rendering | small | Code change in agent_loader.go + workflow_phase.go |
| Rewrite built-in agents as language-agnostic | medium | 7 agent files, remove hardcoded patterns, add project_context blocks |
| Add spec-quality-auditor agent | small | New agent file + seed_agents.go association |
| Add over-engineering-detector agent | small | New agent file + seed_agents.go association |
| Restructure spec phase prompt | medium | Reorder, trim, add example. Largest prompt rewrite |
| Restructure TDD phase prompt | small | Reorder, trim, add example |
| Restructure implement phase prompt | small | Reorder, trim, add example, strengthen verification |
| Restructure review phase prompt | small | Light touch — add over-engineering check, trim bash examples |
| Update system prompts | trivial | 2 files, minor wording changes |
| Add error_patterns to config + init | medium | Config schema, init wizard step, variable registration |
| Frontend: error patterns editor | small | New accordion section in Config.tsx |

## Validation

After implementation, verify with a test task run through the full pipeline:

1. Create a medium-weight task with a spec that has intentionally vague criteria
2. Verify spec-quality-auditor catches the vague criteria and blocks
3. Fix the spec, re-run → verify TDD produces tests that map to every SC
4. Verify implementation doesn't over-engineer (over-engineering-detector should pass clean)
5. Verify review catches any remaining issues
6. Verify agents reference project-specific conventions (not JS/TS patterns) for a Go project
7. Repeat for a TypeScript project to confirm language-agnosticism
