# Templates

Embedded templates for plans and prompts. Used by the executor to generate phase sequences and construct prompts.

## Directory Structure

```
templates/
├── embed.go          # Go embed directives
├── plans/            # Weight-based plan templates
│   ├── trivial.yaml
│   ├── small.yaml
│   ├── medium.yaml
│   ├── large.yaml
│   └── greenfield.yaml
├── prompts/          # Phase prompt templates
│   ├── classify.md
│   ├── research.md
│   ├── spec.md
│   ├── design.md
│   ├── implement.md
│   ├── review.md
│   ├── test.md
│   ├── docs.md
│   ├── validate.md
│   └── finalize.md
├── scripts/          # Script templates
└── pr-body.md        # PR description template
```

## Plan Templates

Plans define phase sequences based on task weight:

| Weight | Phases |
|--------|--------|
| `trivial` | implement |
| `small` | implement → test |
| `medium` | spec → implement → **review** → test → docs |
| `large` | spec → design → implement → **review** → test → docs → validate → finalize |
| `greenfield` | research → spec → design → implement → **review** → test → docs → validate → finalize |

**Design phase** (large/greenfield only): Architecture decisions, component relationships, key patterns. Output becomes `{{DESIGN_CONTENT}}` for implement phase.

**Review phase** (medium/large/greenfield): Multi-agent code review using 5 specialized reviewers. Catches bugs, security issues, performance problems, and merge conflicts before test phase.

### Plan Format (YAML)

Plan templates use inline prompts that include task context:

```yaml
id: medium
weight: medium
phases:
  - id: implement
    name: Implementation
    prompt: |
      **Task**: {{TASK_TITLE}}
      **Description**: {{TASK_DESCRIPTION}}

      [Phase instructions...]

      <phase_complete>true</phase_complete>
    gate: auto
    max_iterations: 20
```

The `{{TASK_DESCRIPTION}}` variable includes the full description provided when creating a task with `orc new "title" -d "description"`.

## Prompt Templates

### Template Variables

| Variable | Description |
|----------|-------------|
| `{{TASK_ID}}` | Task identifier (e.g., TASK-001) |
| `{{TASK_TITLE}}` | Task title |
| `{{TASK_DESCRIPTION}}` | Full task description |
| `{{PHASE}}` | Current phase ID |
| `{{WEIGHT}}` | Task weight (trivial/small/medium/large/greenfield) |
| `{{ITERATION}}` | Current iteration number |
| `{{SPEC_CONTENT}}` | Specification from spec phase |
| `{{DESIGN_CONTENT}}` | Design artifact (large/greenfield only) |
| `{{RETRY_CONTEXT}}` | Retry information (if retrying after failure) |
| `{{WORKTREE_PATH}}` | Absolute path to isolated worktree (if worktree enabled) |
| `{{TASK_BRANCH}}` | Git branch for this task (e.g., orc/TASK-001) |
| `{{TARGET_BRANCH}}` | Branch to merge into (from config, defaults to main) |
| `{{REQUIRES_UI_TESTING}}` | Boolean flag if task requires UI testing |
| `{{SCREENSHOT_DIR}}` | Path to save screenshots (`.orc/tasks/{id}/test-results/screenshots/`) |
| `{{TEST_RESULTS}}` | Previous test results (for validate phase) |
| `{{COVERAGE_THRESHOLD}}` | Minimum test coverage percentage (default: 85%) |
| `{{INITIATIVE_CONTEXT}}` | Initiative details if task is linked to one |
| `{{VERIFICATION_RESULTS}}` | Verification results from implement phase |
| `{{REVIEW_ROUND}}` | Current review round (1 or 2) |
| `{{REVIEW_FINDINGS}}` | Previous round's findings (for Round 2) |

### Worktree Safety Variables

When worktree isolation is enabled, prompts receive additional context for safety:

```markdown
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
- Merging happens via PR after all phases complete
```

Protected branches (main, master, develop, release) are enforced at multiple levels:
1. **Prompt instructions** - AI is told not to push to protected branches
2. **Code-level validation** - `git.Push()` blocks protected branch pushes
3. **Git hooks** - Pre-push hooks in worktree block protected pushes

### Prompt Structure

Each prompt template follows this structure:

```markdown
# Phase Name

## Context
- Task: {{TASK_TITLE}}
- Phase: {{PHASE}}
- Weight: {{WEIGHT}}

{{RETRY_CONTEXT}}

## Instructions
[Phase-specific instructions]

## Completion
When complete, output:
<phase_complete>true</phase_complete>

If blocked, output:
<phase_blocked>reason: [explanation]</phase_blocked>
```

### Phase Prompts

| Phase | Purpose |
|-------|---------|
| `classify.md` | Classify task weight based on description |
| `research.md` | Research existing patterns, dependencies |
| `spec.md` | Create technical specification with verification criteria |
| `design.md` | Design architecture/approach (large/greenfield) |
| `implement.md` | Write the implementation, verify all criteria |
| `review.md` | Multi-agent code review (medium/large/greenfield) |
| `test.md` | Write and run tests (includes Playwright E2E for UI tasks) |
| `docs.md` | Update documentation |
| `validate.md` | E2E validation with Playwright MCP |
| `finalize.md` | Sync with target branch, resolve conflicts, risk assessment (large/greenfield only) |

### Design Phase (large/greenfield)

The `design.md` prompt is used for large and greenfield tasks after spec:

1. **Architecture decisions** - Component relationships, data flow
2. **Key patterns** - Design patterns to use, rationale
3. **Risk areas** - Potential issues, mitigation strategies
4. **File structure** - What to create/modify

Output becomes `{{DESIGN_CONTENT}}` for implement phase.

### Verification Criteria

Spec and implement phases use a structured verification format:

**Spec phase** defines success criteria with verification methods:
```markdown
| ID | Criterion | Verification Method | Expected Result |
|----|-----------|---------------------|-----------------|
| SC-1 | User can log out | `npm test -- logout.spec.ts` | Tests pass |
| SC-2 | Session invalidated | `curl -X POST /logout` | Returns 200 |
```

**Implement phase** must verify all criteria before completion:
```markdown
| ID | Criterion | Method | Result | Notes |
|----|-----------|--------|--------|-------|
| SC-1 | User can log out | `npm test` | ✅ PASS | 3 tests passed |
| SC-2 | Session invalidated | `curl` | ✅ PASS | 200, cookie cleared |
```

Completion is blocked until all criteria pass.

### UI Testing in Prompts

When `{{REQUIRES_UI_TESTING}}` is true, the `test.md` and `validate.md` prompts include:

1. **Playwright MCP tool reference** - Lists available browser tools
2. **E2E test workflow** - Step-by-step guide for UI testing
3. **Screenshot naming conventions** - Consistent naming patterns
4. **Validation workflow** - How to verify UI components

The `{{SCREENSHOT_DIR}}` variable provides the path where screenshots should be saved for automatic attachment to the task.

### Finalize Phase

The `finalize.md` prompt is used for large and greenfield tasks to prepare the branch for merge:

1. **Sync with target branch** - Merge main (or target) into task branch
2. **Conflict resolution** - Resolve conflicts following strict rules:
   - **NEVER remove features** from either side
   - **Merge intentions**, not just text
   - **Prefer additive** resolution when in doubt
   - **Test per file** after resolving each conflict
3. **Test verification** - Re-run full test suite after conflict resolution
4. **Risk assessment** - Classify merge risk based on diff size and conflicts
5. **Merge decision output** - Produce finalization report with recommendation

The finalize phase uses these template variables:

| Variable | Description |
|----------|-------------|
| `{{TASK_BRANCH}}` | Current task branch (e.g., `orc/TASK-001`) |
| `{{TARGET_BRANCH}}` | Branch to merge into (from config, defaults to `main`) |
| `{{IMPLEMENTATION_SUMMARY}}` | Summary of changes made during implementation |
| `{{SPEC_CONTENT}}` | Full specification content for context |

## Embedding

Templates are embedded at compile time via `embed.go`:

```go
//go:embed prompts/*.md
var Prompts embed.FS

//go:embed plans/*.yaml
var Plans embed.FS
```

Access in code:
```go
content, err := templates.Prompts.ReadFile("prompts/implement.md")
```

## Project Overrides

Projects can override prompts in `.orc/prompts/`:
```
.orc/
└── prompts/
    └── implement.md  # Overrides default implement.md
```

The prompt service checks project overrides first, falls back to embedded templates.

## PR Body Template

`pr-body.md` is used for auto-generated PRs:

```markdown
## Summary
{{TASK_TITLE}}

## Changes
{{TASK_DESCRIPTION}}

## Phases Completed
{{PHASE_SUMMARY}}

---
Generated by [orc](https://github.com/randalmurphal/orc)
```
