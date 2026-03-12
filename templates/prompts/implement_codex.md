# Implementation Phase

Task: {{TASK_ID}} — {{TASK_TITLE}}
Category: {{TASK_CATEGORY}}
Worktree: {{WORKTREE_PATH}}
Branch: {{TASK_BRANCH}}
Target: {{TARGET_BRANCH}}

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

{{#if TDD_TESTS_CONTENT}}
<tests_to_pass>
{{TDD_TESTS_CONTENT}}
</tests_to_pass>
{{/if}}

{{#if TDD_INTEGRATION_CONTENT}}
<integration_tests>
{{TDD_INTEGRATION_CONTENT}}
</integration_tests>
{{/if}}

{{#if RETRY_ATTEMPT}}
<retry>
Attempt {{RETRY_ATTEMPT}}, triggered from {{RETRY_FROM_PHASE}}.
Reason: {{RETRY_REASON}}
{{#if OUTPUT_REVIEW}}
Previous review findings:
{{OUTPUT_REVIEW}}
{{/if}}
Fix ALL issues above before proceeding.
</retry>
{{/if}}

## Output Contract

When complete, output JSON:

```json
{
  "status": "complete",
  "summary": "What was implemented",
  "reason": null,
  "verification": {
    "tests": {"command": "{{TEST_COMMAND}}", "status": "PASS", "evidence": "command output"},
    "success_criteria": [{"id": "SC-1", "status": "PASS", "evidence": "proof"}],
    "build": {"status": "PASS", "evidence": "command output"},
    "linting": {"status": "PASS", "evidence": "command output"},
    "wiring": {"status": "PASS", "evidence": "new files are imported by production code", "new_files": [{"file": "path", "imported_by": "path:line"}]},
    "browser_validation": {
      "browser_surface_change": true,
      "required": true,
      "performed": true,
      "live_update_surface": true,
      "external_mutation_validated": true,
      "project_scoped_surface": true,
      "project_isolation_validated": true,
      "reason": "This task changes browser-visible behavior, including rendered UI state and interactions.",
      "evidence": "Used Playwright/browser tools to exercise the changed flow and verify the expected UI behavior.",
      "artifacts": []
    }
  },
  "pre_existing_issues": []
}
```

If blocked, still return the same top-level keys. Use `null` or `[]` for fields that do not apply:

```json
{
  "status": "blocked",
  "summary": null,
  "reason": "what and why",
  "verification": null,
  "pre_existing_issues": []
}
```

## Rules

1. Implement exactly what the specification describes. No extras, no abstractions the spec didn't request.
2. Every new file must be imported by an existing production file. If nothing imports it, it's dead code.
3. Run verification only on files you changed (`git diff --name-only`). Pre-existing failures in other files are not your scope — list them in `pre_existing_issues`.
4. Commit before outputting completion JSON.
5. DO NOT push to {{TARGET_BRANCH}} or checkout other branches.
6. Treat the plan phase's `risk_assessment.requires_browser_qa` and `verification_plan.e2e` as advisory only, not final authority. Decide from the implemented diff whether browser-visible behavior changed.
7. If the implemented change affects browser-visible behavior in any way, including backend or API changes that alter what the UI renders or how it behaves, `verification.browser_validation.required` must be `true` and you must perform browser validation yourself before claiming completion.
8. If browser validation is required but the browser app cannot be run or validated, return `blocked` instead of claiming completion.
9. If the browser surface is expected to react to external events, polling, or another actor's changes while it is open, set `live_update_surface=true` and validate it with a mutation initiated outside the page you are observing.
10. If the browser-visible behavior must stay isolated to the selected project or tenant, set `project_scoped_surface=true` and validate that the behavior stays scoped correctly.
11. If the diff adds work on a repeated/shared path (every request, workflow phase, task load, page refresh, poll tick), verify that the work is conditional or bounded rather than silently scaling with the whole project or dataset.
12. If the diff adds optional context, summaries, caches, or derived state, verify that the code does not silently treat "failed to load" as "no data" unless the specification explicitly allows those outcomes to be equivalent.
13. If the diff replaces computed/live reconstruction with persisted/materialized state, verify rollout parity for pre-existing data and in-flight states before claiming completion.
14. For that same pattern, inventory every production transition that mutates the new stored state, including normal RPCs, retries, background paths, and failure paths; do not assume the code you just added is the only writer.
15. If an operator action writes multiple records or state transitions, verify atomicity or explicit rollback. A partial failure that leaves the visible state lying is a failure, not a follow-up.
16. Grep for alternate write paths to the affected truth, not just the call sites of the new function. Include retries, imports, repair jobs, admin/operator flows, mirrored write helpers, and failure recovery paths.
17. If relationship state is mirrored in a mirrored linkage or join table, verify create/update/delete parity across both representations, including delete-path parity.
18. If project-scoped caches, browser-local state, or UI memoization are involved, verify every get/set/delete key includes project or tenant scope plus a stable identifier. Local-ID-only keys are a correctness bug.
19. If the feature duplicates state across source rows, mirrored tables, caches, events, or browser-visible summaries, verify distributed state parity and name the source of truth in your evidence.
20. If the feature links or promotes artifacts across task/run/thread/initiative context, verify every supported provenance variant explicitly. Do not assume the full-provenance happy path is the only valid case.
21. If browser-local state can be updated by both RPC responses and event-driven reloads, verify stale-response handling and duplicate suppression explicitly.
22. Prefer the smallest set of production paths needed to prove the task. After you inventory the relevant writers and readers, start editing.
23. Prefer existing repo verification commands, fixtures, and browser flows over ad hoc temp harnesses. Only build a custom harness when the normal path cannot prove the behavior, and explain why in the evidence.

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

## Browser Validation Rules

Populate `verification.browser_validation` on every completion:
- `browser_surface_change`: `true` when the implemented behavior changes anything a user sees or does in a browser surface, even if the code change is mostly backend, API, or proto wiring.
- `required`: `true` when browser validation was needed for the implemented diff.
- `performed`: `true` only if you actually executed browser validation.
- `live_update_surface`: `true` when the page should react to an update initiated outside the page itself.
- `external_mutation_validated`: `true` only if you proved the page reacted correctly to a change initiated elsewhere (another tab, another request, seeded data, background event, etc.).
- `project_scoped_surface`: `true` when the browser-visible behavior must stay isolated to the selected project or tenant.
- `project_isolation_validated`: `true` only if you proved the browser-visible behavior stayed scoped to the correct project or tenant.
- `reason`: explain why browser validation was or was not required.
- `evidence`: describe the exact browser flow you validated and what you observed.
- `artifacts`: include screenshot, trace, or log paths when you produced them; otherwise use `[]`.

If `browser_surface_change=true`, then `required` must also be `true`.
If `required=true`, then `performed` must be `true` and `evidence` must be concrete.
If `live_update_surface=true`, then `external_mutation_validated` must also be `true`.
If `project_scoped_surface=true`, then `project_isolation_validated` must also be `true`.

When browser validation is required, use the browser tools available in this environment to exercise the changed flow. Validate the real user-visible behavior, not just unit tests.
Do not stop at same-page happy paths when the surface depends on external updates or project scoping.

## Process

1. Read the specification and referenced code.
2. If a breakdown exists, complete items in order.
3. Implement fully — no TODOs, no placeholders. Handle edge cases per the spec.
4. Run `{{TEST_COMMAND}}` — fix failures in your code only.
5. {{#if BUILD_COMMAND}}Run `{{BUILD_COMMAND}}`.{{else}}Build the project.{{/if}} Fix errors in your files only.
6. {{#if LINT_COMMAND}}Run `{{LINT_COMMAND}}` on changed files.{{else}}Lint changed files.{{/if}} Fix your lint errors only.
7. For each new file, verify a production file imports it.
8. Decide whether the implemented diff changed browser-visible behavior. If it did, run browser validation now and capture evidence in `verification.browser_validation`.
9. For event-driven or live-updating browser surfaces, include at least one external mutation scenario in your validation.
10. For multi-project or tenant-scoped browser surfaces, include an isolation scenario in your validation.
11. Verify each success criterion from the spec with concrete evidence.
12. If the diff adds work on a repeated/shared path, record what triggers it, why it is bounded or lazy, and what verification proves that.
13. If the diff adds optional context, summaries, caches, or derived state, verify whether "no data" and "load failure" are intentionally the same or intentionally different, and record evidence for that behavior.
14. If the diff replaces computed/live behavior with persisted/materialized state, verify rollout parity with pre-existing rows or states and record the evidence.
15. Verify every production transition that must keep the new stored state synchronized, including task-control paths, operator actions, retries, and failure paths.
16. If any operator action performs multiple writes, verify atomicity or explicit rollback and record what proves partial failure cannot leave user-visible state inconsistent.
17. Grep for alternate writers to the affected state and record which ones you verified.
18. If relationship state is mirrored in a mirrored linkage or join table, verify create/update/delete parity, including delete paths and cleanup paths.
19. If project-scoped caches or browser-local state are involved, verify cache get/set/delete keys include project or tenant scope; do not accept local-ID-only keys.
20. If state is duplicated across DB rows, mirrored tables, caches, events, and browser-visible summaries, verify distributed state parity and record the source of truth.
21. If the feature links or promotes artifacts across task/run/thread/initiative context, verify every supported provenance variant and record which ones are valid, including cases where run provenance is intentionally absent.
22. If browser-local state can be updated by both RPC responses and event-driven reloads, verify stale-response handling and duplicate suppression with a real race scenario.
23. Prefer existing repo/browser validation flows over ad hoc temp environments. Build a custom harness only if the normal path cannot prove the behavior, and say why.
24. Commit: `git add -A && git commit -m "[orc] {{TASK_ID}}: implement - [description]"`
25. Output completion JSON.

{{#if TDD_TESTS_CONTENT}}
If tests fail: fix your implementation, not the tests. If a test contradicts the spec, document as `AMEND-NNN`.
{{/if}}

Return every schema key exactly once. Do not omit fields that are not applicable; use `null` or `[]` instead.
