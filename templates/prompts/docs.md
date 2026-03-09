# Documentation Phase

You are updating documentation after implementation is complete for task {{TASK_ID}}.

<output_format>
Your final output MUST be a JSON object.

When `status` is `complete`, there are only two valid outcomes:
- `updated`: you changed a small set of operational docs that were genuinely affected
- `no-op`: you verified no meaningful docs updates were needed

Do not create placeholder docs, broad rewrites, or speculative documentation.

Example `updated` response:

```json
{
  "status": "complete",
  "summary": "Updated README and docs/API_REFERENCE.md for the new CLI verification flow",
  "content": "## Documentation Outcome\n\nResult: updated\n\n## Files Updated\n| Path | Why |\n|------|-----|\n| README.md | Added the new strict default workflow and doctor preflight usage |\n| docs/API_REFERENCE.md | Documented the new CLI command and expected behavior |\n\n## Accuracy Checks\n- Verified commands and file paths against current implementation\n- Confirmed no stale references remain in touched sections\n\n## Deferred Docs\n- None"{{#if INITIATIVE_ID}},
  "initiative_notes": [
    {"type": "pattern", "content": "Strict default workflows should document preflight requirements in operator-facing docs", "relevant_files": ["README.md", "docs/API_REFERENCE.md"]}
  ],
  "notes_rationale": "The task changed durable operational behavior that future work should preserve"{{/if}}
}
```

Example `no-op` response:

```json
{
  "status": "complete",
  "summary": "No meaningful documentation updates required",
  "content": "## Documentation Outcome\n\nResult: no-op\n\n## Reason\n- Existing README, AGENTS.md, and docs already describe the changed behavior accurately enough for operators and future implementers.\n- No user-facing commands, config, API contracts, or architectural guidance became stale.\n\n## Files Reviewed\n- README.md\n- AGENTS.md\n- docs/API_REFERENCE.md\n\n## Deferred Docs\n- None"{{#if INITIATIVE_ID}},
  "initiative_notes": [],
  "notes_rationale": "Routine task or no durable project-level learning to capture"{{/if}}
}
```

If blocked:
```json
{
  "status": "blocked",
  "reason": "[what's blocking and what clarification is needed]"
}
```
</output_format>

<critical_constraints>
Only update docs that are operationally affected by the implementation. Prioritize:
- `README.md` when commands, setup, or operator-facing behavior changed
- `AGENTS.md` or package docs when execution rules or subsystem behavior changed
- `docs/API_REFERENCE.md`, config docs, architecture docs, or troubleshooting docs when those contracts changed

Do not perform repo-wide doc churn. Do not rewrite hierarchy guidance unless the task actually changed it. Prefer narrow edits over new files.

The default answer should be `no-op` unless a specific doc became inaccurate, incomplete, or operationally insufficient.
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

**Git State**: Previous phases have already committed their work. Use `git log --oneline -10` and `git diff {{TARGET_BRANCH}}..HEAD` to see what changed.

DO NOT push to {{TARGET_BRANCH}} or any protected branch.
DO NOT checkout {{TARGET_BRANCH}} - stay on your task branch.
</worktree_safety>

{{INITIATIVE_CONTEXT}}
{{CONSTITUTION_CONTENT}}

{{#if SPEC_CONTENT}}
<plan_artifact>
{{SPEC_CONTENT}}
</plan_artifact>
{{/if}}
</context>

<instructions>
## Step 1: Inspect the blast radius

Read the task diff and identify whether any of these changed materially:
- CLI commands, flags, setup, or operational flow
- Config keys or default behavior
- Public API or documented contract
- Architecture, troubleshooting steps, or operator expectations
- Demo/UI flow that README or demo docs explicitly describe

## Step 2: Choose the smallest necessary docs surface

Prefer updating an existing document over creating a new one.

Typical mapping:
- Command/config/setup change -> `README.md`
- AI/operator workflow or subsystem rule change -> `AGENTS.md`
- Public API or config contract change -> `docs/API_REFERENCE.md` or config docs
- Operational failure handling or recovery change -> troubleshooting or architecture docs

If none of those became stale, return the explicit `no-op` result.

## Step 3: Make targeted edits only

For each doc you touch:
- Update only the sections made inaccurate by the task
- Keep wording concrete and operator-focused
- Remove or fix stale commands, paths, flags, and behavior claims
- Do not add tutorials, speculative guidance, or generic restatements of code

## Step 4: Validate the docs decision

Before returning:
- Confirm every mentioned command, file path, and workflow name matches the current repo
- Confirm any changed docs are sufficient for someone operating or extending the affected surface
- If you chose `no-op`, make sure no command, config, API, or operational behavior changed in a way that leaves docs stale

## Initiative notes

Only emit `initiative_notes` when this task produced a durable project-level pattern, warning, learning, or handoff worth carrying forward. Otherwise emit `[]` and explain why in `notes_rationale`.
</instructions>
