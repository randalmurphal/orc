# Review Round 1: Exploratory Review

<output_format>
Output your structured response matching the review findings schema:

- `round`: 1
- `status`: "complete" | "needs_user_input"
- `summary`: Brief overview with issue counts by severity (critical/high/medium/low)
- `issues`: List of findings, each with `severity`, `file`, `line`, `description`, `suggestion`, and optionally `constitution_violation` ("invariant" = BLOCKER, "default" = warning)
- `positives`: Notable good patterns in the implementation
- `questions`: Clarification questions (if any)

If user decisions are needed:
- `status`: "needs_user_input"
- `summary`: Explain what requires user input
- `questions`: List of questions requiring user decision
- `recommendation`: "Await user decision before proceeding"
</output_format>

<critical_constraints>
The most common failure is missing integration completeness issues — dead code, unwired interfaces, and functions that exist but are never called from production paths.

## Severity Definitions

| Severity | Criteria |
|----------|----------|
| **critical** | Incomplete updates (missed dependents), removed preserved functionality |
| **high** | Bugs, security issues, incorrect behavior, dead code, missing integration, obvious performance regressions on important paths, alternate writers, conflicting association paths, mirrored linkage drift, project-scoped cache bugs, distributed state parity gaps, rejected provenance gaps, same-scope or cross-scope UI race bugs |
| **medium** | Missing edge cases, unclear code, unnecessary complexity, weak or incomplete tests |
| **low** | Style issues, minor improvements, suggestions |
</critical_constraints>

<context>
<task>
ID: {{TASK_ID}}
Title: {{TASK_TITLE}}
Weight: {{WEIGHT}}
</task>

<worktree_safety>
Path: {{WORKTREE_PATH}}
Branch: {{TASK_BRANCH}}
Target: {{TARGET_BRANCH}}

**Git State**: Previous phases have committed their work. Worktree is clean. Use `git diff {{TARGET_BRANCH}}..HEAD` to see changes.

DO NOT push to {{TARGET_BRANCH}} or checkout other branches. Stay on {{TASK_BRANCH}}.
</worktree_safety>
</context>

{{#if SPEC_CONTENT}}
<specification>
{{SPEC_CONTENT}}
</specification>
{{/if}}

{{#if CONSTITUTION_CONTENT}}
<constitution>
{{CONSTITUTION_CONTENT}}
</constitution>

For each issue, determine if it violates the constitution:
- `constitution_violation: "invariant"` — **BLOCKER**: must fix before completion. Any invariant violation automatically fails the review.
- `constitution_violation: "default"` — Warning: document justification if intentional deviation.
- Omit the field if not a constitution violation.
{{/if}}

<instructions>
## Step 1: Read the Implementation

1. List all modified files: `git diff --name-only HEAD~5` (adjust based on commit count)
2. Read each modified file to understand the changes
3. Compare against the specification

## Step 2: Identify Gaps and Issues

**CRITICAL CHECKS (do these first):**

- **Completeness**: Were all dependents from impact analysis updated?
  - Check implementation artifact's "Impact Analysis Results"
  - Verify no broken imports/references: `go build ./...` or `bun run typecheck`

- **Preservation**: Was anything removed that shouldn't be?
  - Cross-reference spec's "Preservation Requirements" table
  - Check for large deletions: `git diff --stat`
  - Verify preserved behaviors still work

**Standard checks:**
- **Architecture alignment**: Does the implementation match the spec's design?
- **Edge cases**: Are all edge cases handled properly?
- **Error handling**: Are errors handled gracefully with clear messages?
- **Security**: Any potential vulnerabilities (injection, XSS, auth issues, unsafe state transitions, race conditions)?
- **Performance**: Any obvious performance issues (N+1 queries, unbounded work, leaks, missing limits/timeouts) on paths that matter?
- **Over-engineering**: Unrequested abstractions, scope creep, speculative configurability
- **Maintainability**: Is the code simple, clear, and aligned with existing patterns?
- **Testing**: Do tests prove the real behavior, edge cases, and failure paths, not just the happy path?
- **Integration**: Does it integrate properly with existing code?

**Integration Completeness** (the most commonly missed category):
- [ ] All new functions are called from at least one production code path
- [ ] No defined-but-never-called functions exist (dead code)
- [ ] New interfaces have implementations wired into the system
- [ ] If the task adds hooks/callbacks/triggers, they are registered
- [ ] Hidden alternate write paths are covered, not just the obvious new RPC or helper
- [ ] Conflicting association paths and legacy readers/writers are covered, not just the new canonical path
- [ ] Mirrored linkage or join tables stay in create/update/delete parity with the source of truth
- [ ] Project-scoped caches and browser-local state key by project or tenant scope, not local ID alone
- [ ] Distributed state parity holds across DB rows, mirrored tables, caches, events, and browser-visible summaries
- [ ] Invalid provenance combinations are rejected instead of silently written
- [ ] Same-scope races and cross-scope reset behavior are safe for browser-local state
- [ ] The implementation inventories are concrete about conflicting paths, integrity guards, rejected variants, and reset rules

## Step 3: Document Findings

For each issue found, categorize by severity using the definitions above. Include file, line number, description, and a specific fix suggestion.

When deciding severity, bias toward blocking if the issue affects:
- security or integrity of data/state
- performance on a hot or scalable path
- code simplicity or maintainability enough to obscure correctness
- test coverage for critical behavior or failure modes
- alternate writers, conflicting association paths, mirrored linkage state, project-scoped cache keys, distributed state parity, rejected provenance handling, or same-scope/cross-scope UI races
</instructions>
