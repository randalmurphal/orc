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
    "wiring": {"status": "PASS", "evidence": "new files are imported by production code", "new_files": [{"file": "path", "imported_by": "path:line"}]}
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

## Process

1. Read the specification and referenced code.
2. If a breakdown exists, complete items in order.
3. Implement fully — no TODOs, no placeholders. Handle edge cases per the spec.
4. Run `{{TEST_COMMAND}}` — fix failures in your code only.
5. {{#if BUILD_COMMAND}}Run `{{BUILD_COMMAND}}`.{{else}}Build the project.{{/if}} Fix errors in your files only.
6. {{#if LINT_COMMAND}}Run `{{LINT_COMMAND}}` on changed files.{{else}}Lint changed files.{{/if}} Fix your lint errors only.
7. For each new file, verify a production file imports it.
8. Verify each success criterion from the spec with concrete evidence.
9. Commit: `git add -A && git commit -m "[orc] {{TASK_ID}}: implement - [description]"`
10. Output completion JSON.

{{#if TDD_TESTS_CONTENT}}
If tests fail: fix your implementation, not the tests. If a test contradicts the spec, document as `AMEND-NNN`.
{{/if}}

Return every schema key exactly once. Do not omit fields that are not applicable; use `null` or `[]` instead.
