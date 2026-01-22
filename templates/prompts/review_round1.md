# Review Round 1: Exploratory Review

You are a senior engineer performing an exploratory code review.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}

## Worktree Safety

You are working in an **isolated git worktree**.

| Property | Value |
|----------|-------|
| Worktree Path | `{{WORKTREE_PATH}}` |
| Task Branch | `{{TASK_BRANCH}}` |
| Target Branch | `{{TARGET_BRANCH}}` |

**CRITICAL SAFETY RULES:**
- All commits go to branch `{{TASK_BRANCH}}`
- **DO NOT** push to `{{TARGET_BRANCH}}` or any protected branch
- **DO NOT** checkout other branches - stay on `{{TASK_BRANCH}}`
- Merging happens via PR after all phases complete
- Git hooks are active to prevent accidental protected branch modifications

## Specification

{{SPEC_CONTENT}}

{{#if CONSTITUTION_CONTENT}}
## Constitution & Invariants

The following rules govern this project. **Invariants CANNOT be ignored or overridden.**

<constitution>
{{CONSTITUTION_CONTENT}}
</constitution>

### Constitution Compliance Check

For each issue you find, determine if it violates the constitution:
- `constitution_violation: "invariant"` - **BLOCKER** - Must fix before completion
- `constitution_violation: "default"` - Warning - Document justification if intentional deviation
- Omit field if not a constitution violation

**Any issue with `constitution_violation: "invariant"` automatically fails the review.** These are absolute rules that cannot be waived.
{{/if}}

## Instructions

As a senior engineer, examine the implemented code thoroughly:

### Step 1: Read the Implementation

Use the available tools to:
1. List all modified files with `git diff --name-only HEAD~5` (adjust based on commit count)
2. Read each modified file to understand the changes
3. Compare against the specification

### Step 2: Identify Gaps and Issues

**CRITICAL CHECKS (do these first):**

- **Completeness**: Were all dependents from impact analysis updated?
  - Check implementation artifact's "Impact Analysis Results"
  - Verify no broken imports/references: `go build ./...` or `npm run typecheck`

- **Preservation**: Was anything removed that shouldn't be?
  - Cross-reference spec's "Preservation Requirements" table
  - Check for large deletions: `git diff --stat`
  - Verify preserved behaviors still work

**Standard checks:**
- **Architecture alignment**: Does the implementation match the spec's design?
- **Edge cases**: Are all edge cases handled properly?
- **Error handling**: Are errors handled gracefully with clear messages?
- **Security**: Any potential vulnerabilities (injection, XSS, auth issues)?
- **Performance**: Any obvious performance issues (N+1 queries, memory leaks)?
- **Maintainability**: Is the code clear and well-organized?
- **Integration**: Does it integrate properly with existing code?

### Step 3: Document Findings

For each issue found, categorize by severity:
- **critical**: Incomplete updates (missed dependents), removed preserved functionality
- **high**: Bugs, security issues, incorrect behavior
- **medium**: Missing edge cases, unclear code, potential issues
- **low**: Style issues, minor improvements, suggestions

## Output Format

Output JSON matching the review findings schema:

```json
{
  "round": 1,
  "summary": "Brief overview of review findings",
  "issues": [
    {"severity": "high", "file": "path/to/file.go", "line": 42, "description": "Issue description", "suggestion": "How to fix", "constitution_violation": "invariant"},
    {"severity": "medium", "file": "path/to/another.go", "line": 100, "description": "Missing error handling", "suggestion": "Add retry logic"}
  ],
  "questions": ["Question requiring clarification"],
  "positives": ["Good thing noticed in the implementation"]
}
```

**Note:** Include `constitution_violation` field only if the issue violates a constitution rule. Value is `"invariant"` (blocker) or `"default"` (warning).

## If User Input Required

If questions require user decisions, use the review decision schema:

```json
{
  "status": "needs_user_input",
  "summary": "Review requires user input",
  "user_questions": ["Architecture question needing decision"],
  "recommendation": "Await user decision before proceeding"
}
```

## Phase Completion

After documenting all findings, output ONLY this JSON:

```json
{"status": "complete", "summary": "Review round 1: [count] critical, [count] warnings, [count] suggestions"}
```
