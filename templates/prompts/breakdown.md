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

<critical_constraints>
The most common failure is creating breakdown tasks that aren't linked to specific TDD tests, making it impossible to verify progress during implementation.

1. **Order by dependency** - Tasks that others depend on come first
2. **Mark parallelizable** - Tasks with no incomplete dependencies get `[P]`
3. **Link to tests** - Every task should make at least one test pass
4. **Atomic tasks** - Each task should be completable in one focused session
5. **No orphans** - Every task must be reachable from a user story
6. **Identify integration points** - Tasks that wire new code into existing paths MUST be explicit
7. **Flag concurrent/parallel code** - Any task involving goroutines, parallel execution, or concurrent state needs explicit call-out
</critical_constraints>

<parallel_code_requirements>
## Parallel/Concurrent Code Handling

If the spec involves ANY concurrent or parallel execution:

1. **Explicit synchronization tasks** - Create separate tasks for:
   - Setting up synchronization primitives (mutexes, channels, atomics)
   - Implementing thread-safe wrappers if needed
   - Adding cancellation/context propagation

2. **All code paths must be covered** - Parallel code often has multiple execution paths:
   - Happy path (all succeed)
   - Partial failure (some succeed, some fail)
   - Full failure (all fail)
   - Cancellation mid-execution
   - Timeout scenarios

3. **Mark with `[CONC]`** - Any task involving concurrent code gets this marker:
   ```markdown
   - [ ] T005 [P][CONC] Implement parallel phase execution
     - Files: internal/executor/parallel.go
     - Depends: T003
     - Makes pass: TestParallel_HappyPath, TestParallel_PartialFailure
     - Concurrency: Uses errgroup, must handle context cancellation
   ```

4. **Integration tasks are separate** - Don't combine "implement the feature" with "wire it into the system":
   ```markdown
   - [ ] T006 [P] Implement parallel executor
   - [ ] T007 Wire parallel executor into workflow engine
     - Depends: T006
     - Makes pass: TestWorkflow_UsesParallelExecution (integration)
   ```
</parallel_code_requirements>

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

{{#if SPEC_CONTENT}}
<specification>
{{SPEC_CONTENT}}
</specification>
{{/if}}

{{#if TDD_TESTS_CONTENT}}
<tdd_tests>
{{TDD_TESTS_CONTENT}}
</tdd_tests>
{{/if}}
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
</instructions>
