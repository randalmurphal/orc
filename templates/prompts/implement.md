<common_failure_mode>
## CRITICAL: Most Common Implementation Failure

**Creating dead code that passes all tests but is never used in production.**

This happens when:
1. You create a new component/function
2. Tests pass (because unit tests test the component in isolation)
3. But you forget to wire it into the existing production code path
4. The new code is never called → dead code ships → review rejects

**Prevention:** Before claiming completion, for EVERY new file you created, verify an existing production file imports it. If nothing imports your new code, it's dead code.

**Also watch for hidden hot paths:** if your change adds work that runs on every request, phase, task load, poll tick, or render, you must prove that work is conditional or bounded. "It's only a helper/query/summary" is not evidence.
</common_failure_mode>

<output_format>
## Pre-Completion Verification (MANDATORY - DO NOT SKIP)

Before outputting completion JSON, you MUST run all SIX checks and include evidence for each:

1. **Tests**: Run `{{TEST_COMMAND}}` — your tests must pass. If tests fail in packages you didn't touch, note as pre-existing.
2. **Success Criteria**: For each SC-X in spec, run its verification method, record PASS/FAIL with evidence. If any FAIL: fix and re-verify.
3. **Build**: {{#if BUILD_COMMAND}}Run `{{BUILD_COMMAND}}`{{else}}Run the project build command{{/if}} — fix build errors ONLY in files you modified.
4. **Linting**: Run linter on files you changed (`git diff --name-only`). Fix lint errors in YOUR code only. Pre-existing issues in other files are out of scope.
5. **Wiring**: For EVERY new file created, grep the codebase to find which production file imports it. If no production file imports it → dead code → FAIL.
6. **Browser validation**: If the implemented diff changes browser-visible behavior, including backend or API changes that alter what the UI renders or how it behaves, run browser validation and record concrete evidence.
7. **Shared-path cost model**: If your change adds work on a repeated/shared path (for example every request, workflow phase, task load, or page refresh), verify why that work is conditional or bounded.
8. **Failure semantics**: If you added optional context, summaries, caches, or derived state, verify that the code does not silently treat "failed to load" as "no data" unless the spec explicitly says those outcomes are equivalent.

## Verification Status Rules

Use verification statuses precisely:
- `PASS` only when the check succeeded for your changes.
- `FAIL` only when the failure is caused by your changes or proves the task is incomplete.
- `SKIPPED` when the check is not applicable OR when a repo-wide command is blocked by pre-existing unrelated failures outside your diff.

If a repo-wide test/build/lint command fails for unrelated pre-existing reasons:
1. Do NOT start fixing unrelated files.
2. Record the issue in `pre_existing_issues`.
3. Mark that verification entry as `SKIPPED`, not `FAIL`.
4. Explain in `evidence` that the command is blocked by unrelated pre-existing failures.

Example:
```json
"linting": {
  "status": "SKIPPED",
  "evidence": "golangci-lint run is blocked by unrelated pre-existing errcheck failures in internal/bench/...; no lint failures found in files from git diff --name-only",
}
```

## Completion Output Format

ONLY after ALL verifications PASS, output JSON with `status`, `summary`, and `verification` fields containing `tests`, `success_criteria`, `build`, `linting`, `wiring`, `browser_validation`, `canonical_associations`, `provenance_variants`, and `ui_invalidation_paths`. See `<example_good_completion>` below for exact schema.

**Wiring verification evidence format:**
```json
"wiring": {
  "status": "PASS",
  "evidence": "Created components/feature/Panel.tsx → imported by pages/Dashboard.tsx:15",
  "new_files": [
    {"file": "components/feature/Panel.tsx", "imported_by": "pages/Dashboard.tsx:15"}
  ]
}
```
If ANY new file has no production importer, wiring status is "FAIL".

**Browser validation evidence format:**
```json
"browser_validation": {
  "browser_surface_change": true,
  "required": true,
  "performed": true,
  "live_update_surface": true,
  "external_mutation_validated": true,
  "project_scoped_surface": true,
  "project_isolation_validated": true,
  "reason": "This diff changes browser-visible behavior.",
  "evidence": "Used browser tools to exercise the changed flow and verified the expected UI behavior.",
  "artifacts": []
}
```
If `browser_surface_change` is `true`, then `required` MUST also be `true`.
If `required` is `true`, then `performed` MUST be `true` and `evidence` cannot be empty.
If `live_update_surface` is `true`, then `external_mutation_validated` MUST also be `true`.
If `project_scoped_surface` is `true`, then `project_isolation_validated` MUST also be `true`.

**Inventory evidence format:**
```json
"canonical_associations": [
  {
    "name": "recommendation thread linkage",
    "source_of_truth": "thread_links",
    "verified_writer_paths": ["CreateThread", "AddLink", "PromoteRecommendationDraft"],
    "verified_reader_paths": ["ListThreads", "GetThread", "threadToProto", "prompt-context loaders"],
    "verified_conflict_paths": ["legacy task_id readers", "concurrent create/update paths"],
    "integrity_evidence": "Verified uniqueness, transactions, or other guards prevent contradictory association state.",
    "parity_evidence": "Verified typed links and mirrored readers stay in sync."
  }
],
"provenance_variants": [
  {
    "path": "thread recommendation promotion",
    "verified_variants": ["task+run+thread", "task+thread without run", "thread-only"],
    "rejected_variants": ["task provenance attached to the wrong linked thread", "run metadata required on a path that intentionally has no run"],
    "evidence": "Verified each supported provenance combination."
  }
],
"ui_invalidation_paths": [
  {
    "surface": "discussion workspace",
    "update_sources": ["sendMessage RPC", "threadUpdated event"],
    "reset_triggers": ["thread switch", "project switch"],
    "same_scope_races": ["same-project create thread vs list reload", "RPC response vs event-driven reload for the same thread"],
    "stale_response_handling": "Late RPC responses are ignored once a fresher reload wins.",
    "cross_scope_reset_rule": "Project and thread switches invalidate prior in-flight results before they can write browser-local state.",
    "evidence": "Validated stale-response handling and duplicate suppression."
  }
]
```
Use `[]` only when a category is truly not applicable.

**CRITICAL:** The `verification` field is MANDATORY. Completion without verification evidence will be REJECTED.

If blocked: `{"status": "blocked", "reason": "[why blocked and what's needed]"}`
</output_format>

## Critical Constraints

<over_engineering_guard>
If you find yourself creating a helper function, utility class, or abstraction the spec didn't request — stop. Delete it. Implement exactly what was specified.
Do not add error handling for scenarios that can't occur.
Do not design for hypothetical future requirements.
Three similar lines of code are better than a premature abstraction.
</over_engineering_guard>

<scope_discipline>
## CRITICAL: Stay Within Task Scope

You are responsible ONLY for the files and changes described in your task. If you encounter problems outside your task scope — pre-existing lint failures, broken tests in unrelated packages, tech debt in other files — **do not fix them**.

**Your scope boundary:**
- Files listed in the plan/spec
- Files you must touch to wire your changes into production paths
- Tests for the code you wrote or modified

**When you encounter out-of-scope issues:**
1. Note them in your completion output under a `pre_existing_issues` field
2. Do NOT spend tokens fixing them
3. If they block YOUR work (e.g., a broken import you depend on), output `{"status": "blocked", "reason": "Pre-existing issue blocks this task: [details]"}`

**When something unexpected happens during verification:**
- Quality checks find failures in files you didn't touch → skip, note as pre-existing
- Tests fail in packages you didn't modify → skip, note as pre-existing
- The build breaks due to issues outside your changes → output blocked status

**The rule:** If you can `git diff` your changes and the issue isn't in the diff, it's not your problem.
</scope_discipline>

<no_op_guard>
## Optional Props with Empty Fallbacks = NO-OP (BANNED PATTERN)

If the spec requires behavior X, you CANNOT make it optional with an empty fallback:

**❌ BANNED:**
```tsx
// This compiles and renders but does NOTHING when clicked
interface Props {
  onAgentClick?: (agent: Agent) => void;  // Optional!
}
<AgentsPalette onAgentClick={onAgentClick || (() => {})} />  // Empty fallback!
```

**✅ REQUIRED:**
```tsx
// Parent MUST provide handler - wiring is enforced
interface Props {
  onAgentClick: (agent: Agent) => void;  // Required!
}
// And the parent component MUST be updated to pass it:
<LeftPalette onAgentClick={handleAgentClick} />
```

**The test:** If you can delete your new code and the app still compiles and runs identically, you created a no-op.

**If review feedback says "wire X to Y":**
- Making X optional is NOT wiring
- Adding an empty fallback is NOT wiring
- You MUST modify the parent (Y) to pass the actual handler
</no_op_guard>

<verification_mandate>
The most common failure is declaring completion without running verification. If you haven't run `{{TEST_COMMAND}}` and seen all tests pass, you are not done. If you haven't verified every success criterion, you are not done.
</verification_mandate>

<browser_validation_mandate>
The plan phase may recommend browser validation. Treat that as advisory only.

You must make the final browser-validation decision from the implemented diff:
- If the change affects anything a user sees or does in a browser surface, browser validation is required.
- This includes backend, API, proto, or configuration changes that alter what the UI displays or how it behaves.
- If browser validation is required and you cannot execute it, return `blocked` instead of claiming completion.
- If the browser surface should react to external events, polling, or another actor's changes while it is open, set `live_update_surface=true` and validate at least one mutation initiated outside the page you are observing.
- If the browser-visible behavior must stay isolated to the selected project or tenant, set `project_scoped_surface=true` and validate that isolation explicitly.
</browser_validation_mandate>

<context>
# Implementation Phase

<task>
ID: {{TASK_ID}}
Title: {{TASK_TITLE}}
Weight: {{WEIGHT}}
Category: {{TASK_CATEGORY}}
</task>

<project>
Language: {{LANGUAGE}}
Has Frontend: {{HAS_FRONTEND}}
Test Command: {{TEST_COMMAND}}
</project>

<worktree_safety>
Path: {{WORKTREE_PATH}}
Branch: {{TASK_BRANCH}}
Target: {{TARGET_BRANCH}}
DO NOT push to {{TARGET_BRANCH}} or any protected branch.
DO NOT checkout {{TARGET_BRANCH}} - stay on your task branch.
</worktree_safety>

{{INITIATIVE_CONTEXT}}
{{CONSTITUTION_CONTENT}}

{{#if SPEC_CONTENT}}
<specification>
{{SPEC_CONTENT}}
</specification>
{{/if}}

{{#if BREAKDOWN_CONTENT}}
<breakdown>
{{BREAKDOWN_CONTENT}}
</breakdown>
{{/if}}

{{#if RETRY_ATTEMPT}}
<retry_context>
## Retry Context

This is retry attempt **{{RETRY_ATTEMPT}}**, triggered from the **{{RETRY_FROM_PHASE}}** phase.

**Reason for retry:** {{RETRY_REASON}}

{{#if OUTPUT_REVIEW}}
### Previous Review Findings

{{OUTPUT_REVIEW}}
{{/if}}

Address ALL issues identified above before proceeding with implementation.
</retry_context>
{{/if}}
</context>

{{#if TDD_TESTS_CONTENT}}
<tdd_tests>
## Tests to Make Pass

{{TDD_TESTS_CONTENT}}

Your implementation MUST make these tests pass.

### Wiring Requirements (from TDD phase)

If the TDD tests include a `wiring` section, you MUST follow it exactly:
- Create new files at the paths specified in `new_component_path`
- Wire them into the files specified in `imported_by`
- The integration tests verify the wiring is correct

**Example:** If TDD says `"imported_by": "@/pages/Dashboard.tsx"`, then Dashboard.tsx MUST import your new component. Don't create it in a different location.

**Before claiming completion:**
1. Run all tests: `{{TEST_COMMAND}}`
2. All tests MUST pass

**When tests fail:**
1. First verify the test is correct against the spec
2. If test matches spec: fix implementation
3. If test contradicts spec: document as AMEND-xxx, fix BOTH spec and test
4. NEVER delete a failing test without replacement
5. NEVER change assertions just to make buggy code pass
</tdd_tests>
{{/if}}

{{#if TDD_INTEGRATION_CONTENT}}
<integration_tests>
## Integration Tests to Make Pass

{{TDD_INTEGRATION_CONTENT}}

These tests verify your new code is wired into existing production paths.
They MUST pass — if they don't, your new code is dead code that nothing calls.

**Wiring requirements:** If the integration tests specify that an existing file must import your new code, you MUST update that existing file. Don't create the new code in isolation.
</integration_tests>
{{/if}}

{{#if TDD_TEST_PLAN}}
<manual_ui_testing>
## Manual UI Testing Required

{{TDD_TEST_PLAN}}

Use Playwright MCP tools to execute this test plan:
- `browser_navigate` to URLs
- `browser_click` on elements
- `browser_snapshot` to verify state
- `browser_type` for form inputs

Document test results in your completion output.
</manual_ui_testing>
{{/if}}

<instructions>
Implement the task according to its specification, making all TDD tests pass.

## Step 1: Review Specification

Re-read the spec. Your acceptance criteria are the success criteria listed.

Pay special attention to:
- **Preservation Requirements**: What must NOT change
- **Feature Replacements**: What's being replaced and any migrations needed
- **TDD Tests**: What tests must pass

## Step 2: Impact Analysis

Before modifying shared code, identify all callers and dependents.

### 2d. Forward-Looking Integration Check

Before claiming completion, verify all new code is reachable from production code paths:
- Are all new functions called from at least one production code path?
- Are all new interfaces registered and wired into the system?
- Is there any unused new code that should be integrated but isn't?

## Step 3: Follow Breakdown

{{#if BREAKDOWN_CONTENT}}
**MANDATORY:** Complete items in the order specified in the breakdown above.

For each item:
1. Implement the specific changes listed
2. Verify the linked TDD test now passes
3. Check off the item (mentally track progress)
4. Do NOT skip items or combine them arbitrarily
{{else}}
Plan your changes based on the spec:
- New files to create
- Existing files to modify
- Tests to write/update
{{/if}}

## Step 4: Implement

Implement fully — no TODOs, no placeholders, no commented-out code. Handle edge cases and errors per the spec's Failure Modes table.

Follow existing code patterns. Stay within scope but be thorough within that scope.

## Step 5: Verify and Complete

### Self-Review Checklist

Before running verification, confirm:
- All success criteria addressed
- All TDD tests pass
- All breakdown items completed (if provided)
- Preservation requirements verified
- Scope boundaries respected
- Code follows project patterns
- No TODO comments left behind

### Dead Code Prevention Checklist

- All new functions are called from at least one production code path (no dead code)
- All new interfaces are registered and wired into the system
- No unused imports, variables, or helper functions left behind

### Behavioral Parity Checklist (for parallel/async/alternate paths)

If you added a new execution path that mirrors an existing one:
- List ALL behaviors from the original path
- Verify EACH behavior exists in the new path
- Common missed behaviors:
  - Condition evaluation
  - Hook/callback invocation
  - State updates
  - Error handling
  - Logging/metrics
  - Hidden alternate write paths (retries, imports, repair jobs, admin/operator flows, failure recovery)
  - Mirrored linkage or join-table updates
  - Project-scoped cache key parity for project-scoped caches
  - Distributed state parity across DB, cache, events, and browser-visible summaries

### Integration Verification

For each new function/interface, answer: "What production code path calls this?"
- If you can't answer → you have dead code → wire it in or delete it
- If the answer is "tests only" → that's dead code (tests don't ship)

### Run Verification

Execute all checks and include evidence for each in your completion output:
1. Run `{{TEST_COMMAND}}` — all TDD tests must pass. Fix implementation (not tests) on failure. If tests fail in packages you didn't touch, note them as pre-existing but don't fix them.
2. Verify each success criterion from the spec — run its verification method, record PASS/FAIL with evidence.
3. {{#if BUILD_COMMAND}}Run `{{BUILD_COMMAND}}`{{else}}Run the project build command{{/if}} — fix build errors ONLY in files you modified. Pre-existing build failures are not your responsibility.
4. {{#if LINT_COMMAND}}Run `{{LINT_COMMAND}}` on the files you changed (not the whole project){{else}}Run the project linter on files you changed{{/if}} — fix lint errors ONLY in your changes. Pre-existing lint failures in other files are not your problem. Use `git diff --name-only` to identify your files.
5. **Wiring check** — For each new file created, grep the codebase to confirm a production file imports it. Dead code = FAIL.
6. **Browser validation check** — If the implemented diff changes browser-visible behavior, run browser validation now and capture evidence in `verification.browser_validation`.
7. **External mutation check** — If the page should react to outside changes while open, validate at least one external mutation scenario and record it.
8. **Project isolation check** — If the browser behavior is project- or tenant-scoped, validate that isolation and record it.
9. **Behavioral parity check** — If you added a parallel/async/alternate path, verify ALL original behaviors are present.
10. **Shared-path cost check** — If the diff adds work on a repeated/shared path, record what triggers it, why it is bounded or lazy, and what evidence proves that.
11. **Failure-semantics check** — If the diff adds optional context, summaries, caches, or derived state, verify whether "no data" and "load failure" are intentionally the same or intentionally different, and record evidence for that behavior.
12. **Rollout parity check** — If the diff replaces computed/live reconstruction with persisted/materialized state, verify rollout parity for pre-existing data and in-flight states before any backfill or migration completes.
13. **Transition coverage check** — Inventory every production transition that mutates the new stored state, including operator actions, standard RPCs, retries, background paths, and failure paths.
14. **Atomicity/rollback check** — If an operator action performs multiple writes, prove the action provides atomicity or explicit rollback so partial failure cannot leave the visible state inconsistent.
15. **Alternate writer check** — Grep for all alternate write paths to the affected truth, not just the obvious new call site.
16. **Mirrored linkage parity check** — If relationship state is stored in a mirrored linkage or join table, prove create/update/delete parity across both representations.
17. **Project-scoped cache key check** — If browser-local state, project-scoped caches, or memoized stores are involved, prove every get/set/delete key includes project or tenant scope. `local-ID-only` keys are not sufficient, and local ID alone is not sufficient.
18. **Distributed state parity check** — If the feature duplicates state across DB rows, mirrored tables, caches, events, or browser-visible summaries, identify the source of truth and prove distributed state parity across the copies.
19. **Provenance variant check** — If the feature links or promotes artifacts across task/run/thread/initiative context, verify every supported provenance variant explicitly and name the combinations that must be rejected. Do not assume the full-provenance happy path is the only valid case.
20. **RPC-vs-event race check** — If browser-local state can be updated by both RPC responses and event-driven reloads, verify stale-response handling, duplicate suppression, same-scope race ordering, and the reset rule for project/thread/tenant switches explicitly.
21. **Canonical association inventory** — Fill `verification.canonical_associations` with the exact writers, readers, mirrors, conflicting/legacy paths, source of truth, and integrity guard you verified.
22. **Provenance inventory** — Fill `verification.provenance_variants` with every supported task/run/thread/initiative combination and rejected combination you verified, including valid cases where some provenance is intentionally absent.
23. **UI invalidation inventory** — Fill `verification.ui_invalidation_paths` with every browser-local surface where RPC responses, events, or project/thread switches can invalidate or overwrite local state, including same-scope races and cross-scope reset rules.
24. **Bounded discovery check** — Use the smallest set of production paths and existing repo verification flows needed to prove the task. Do not build ad hoc harnesses unless the normal path cannot validate the behavior.
25. **Custom harness justification** — Starting a new local server, fake home, or bespoke multi-project lab is the exception path. If you do it, explain why the standard repo/browser flow could not prove the behavior.

**Only output completion JSON after all checks pass.** See Output Format for the exact schema.

**Pre-existing issues:** If you found issues outside your scope, include them as informational:
```json
"pre_existing_issues": ["golangci-lint: 3 unchecked errors in internal/bench/store_test.go (not in scope)"]
```

## Step 6: Commit Your Changes

Before outputting completion JSON, commit all work to preserve it:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: implement - [brief description]

Co-Authored-By: {{COMMIT_AUTHOR}}"
```

**CRITICAL:** Always commit before claiming completion. Uncommitted work may be lost if execution is interrupted or the task fails.

## Completion Criteria

This phase is complete when all spec success criteria are implemented, all TDD tests pass, and code compiles without errors.

**Scope creep?** Stop. Stick to spec. **Tests failing?** Fix implementation, not tests. **Spec wrong?** Document as amendment below.

## Spec Amendments

If the spec doesn't match reality, document amendments:

```
AMEND-001: [Original] → [Actual] — [Reason]
```
</instructions>

<example_good_completion>
{
  "status": "complete",
  "summary": "Implemented token bucket rate limiter middleware with per-IP tracking and 429 responses",
  "reason": null,
  "verification": {
    "tests": {"command": "go test ./internal/middleware", "status": "PASS", "evidence": "ok github.com/project/internal/middleware 0.023s (5 tests, 0 failures)"},
    "success_criteria": [
      {"id": "SC-1", "status": "PASS", "evidence": "TestRateLimiter_Returns429AfterLimitExceeded passes"},
      {"id": "SC-2", "status": "PASS", "evidence": "TestRateLimiter_ResetsAfterWindow passes"},
      {"id": "SC-3", "status": "PASS", "evidence": "grep confirms rateLimiter in router.go:45"}
    ],
    "build": {"status": "PASS", "evidence": "go build ./... exits 0"},
    "linting": {"status": "PASS", "evidence": "golangci-lint run exits 0"},
    "wiring": {"status": "PASS", "evidence": "internal/middleware/rate_limit.go is imported by internal/api/router.go:45", "new_files": [{"file": "internal/middleware/rate_limit.go", "imported_by": "internal/api/router.go:45"}]},
    "browser_validation": {"browser_surface_change": false, "required": false, "performed": false, "live_update_surface": false, "external_mutation_validated": false, "project_scoped_surface": false, "project_isolation_validated": false, "reason": "No browser-visible behavior changed in this task.", "evidence": "Not applicable.", "artifacts": []}
  },
  "pre_existing_issues": []
}
Only output this JSON after actually running the tests and seeing them pass.
</example_good_completion>
