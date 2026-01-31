<context>
# Breakdown Phase

<task>
ID: {{TASK_ID}}
Title: {{TASK_TITLE}}
Weight: {{WEIGHT}}
Category: {{TASK_CATEGORY}}
</task>

<worktree_safety>
Path: {{WORKTREE_PATH}}
Branch: {{TASK_BRANCH}}
Target: {{TARGET_BRANCH}}
</worktree_safety>

<specification>
{{SPEC_CONTENT}}
</specification>

<tdd_tests>
{{TDD_TESTS_CONTENT}}
</tdd_tests>
</context>

<instructions>
Generate a checkboxed task list from the spec, design, and TDD tests. This breakdown guides the implement phase.

## Task Structure

Group tasks by user story (from spec's Prioritized User Stories):

```markdown
# Implementation Breakdown

## User Story 1: [Title from P1]
- [ ] T001 [P] Create database schema
  - Files: internal/db/schema/xxx.sql
  - Depends: none
  - Makes pass: TestSchemaCreation
- [ ] T002 Add API handler
  - Files: internal/api/handlers.go
  - Depends: T001
  - Makes pass: TestAPIHandler

## User Story 2: [Title from P2]
- [ ] T003 [P] Create frontend component
  - Files: web/src/components/Feature.tsx
  - Depends: none
  - Makes pass: Feature.test.tsx
```

## Markers

| Marker | Meaning |
|--------|---------|
| `[P]` | **Parallelizable** - No dependency on incomplete task in same story |
| `[ ]` | Pending task |
| `[x]` | Completed task |

## Task Properties

Each task MUST include:
- **Files**: Which files will be created/modified
- **Depends**: Task ID dependencies (or "none")
- **Makes pass**: Which TDD test this task should make pass

## Rules

1. **Order by dependency** - Tasks that others depend on come first
2. **Mark parallelizable** - Tasks with no incomplete dependencies get `[P]`
3. **Link to tests** - Every task should make at least one test pass
4. **Atomic tasks** - Each task should be completable in one focused session
5. **No orphans** - Every task must be reachable from a user story
</instructions>

<output_format>
Output a JSON object with the breakdown:

```json
{
  "status": "complete",
  "summary": "Generated N tasks across M user stories",
  "content": "# Implementation Breakdown\n\n## User Story 1: [Title]\n- [ ] T001 [P] Create...\n..."
}
```

If blocked:
```json
{
  "status": "blocked",
  "reason": "[What's unclear about the breakdown]"
}
```
</output_format>
