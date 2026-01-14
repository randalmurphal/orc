# Documentation Phase

You are creating and updating documentation after implementation is complete.

## Context

**Task ID**: {{TASK_ID}}
**Task**: {{TASK_TITLE}}
**Weight**: {{WEIGHT}}

{{INITIATIVE_CONTEXT}}

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

## Implementation Summary

{{IMPLEMENT_CONTENT}}

## Instructions

### Step 1: Audit Existing Documentation

Check what documentation exists and what's missing:

1. **Root level**: README.md, CLAUDE.md, CONTRIBUTING.md
2. **Changed directories**: README.md, CLAUDE.md in affected paths
3. **Architecture docs**: docs/architecture/ if structural changes
4. **API docs**: If public interfaces changed

### Step 2: Create Missing Required Docs

Based on project type, create any missing required documentation:

**For all projects:**
- `CLAUDE.md` at root (AI-readable project overview)
- `README.md` at root (human-readable overview)

**For monorepos:**
- `README.md` in each package/module directory
- Optional `CLAUDE.md` for complex packages

**For significant changes:**
- Architecture docs if new components added
- ADR if architectural decisions were made

### Step 3: Update Existing Docs

For each existing doc in the blast radius:

1. Check if content is still accurate
2. Update sections affected by changes
3. Add documentation for new features/APIs
4. Update examples if behavior changed
5. Update command references if CLI changed

### Step 4: CLAUDE.md Quality Check

Ensure CLAUDE.md files are:
- Under 200 lines
- Use tables over prose
- Include Quick Start section
- Have accurate Structure table
- List actual commands that work

### Step 5: Validate Documentation

- All code blocks have correct syntax highlighting
- All internal links resolve to existing files
- All examples are runnable
- No TODO/FIXME placeholders in final docs

### Step 6: Update Project Knowledge

Review what you learned during this task and update CLAUDE.md's knowledge section.

Look for the section between `<!-- orc:knowledge:begin -->` and `<!-- orc:knowledge:end -->`:

1. **Patterns Learned**: If you established a reusable code pattern, add a row to the table
   - Pattern name, brief description, task ID as source
   - Example: `| Functional options | Config via With* functions | TASK-003 |`

2. **Known Gotchas**: If something didn't work as expected, document the resolution
   - What the issue was, how to resolve it, task ID
   - Example: `| SQLite locks | Use WAL mode | TASK-002 |`

3. **Decisions**: If you made an architectural or design decision, capture the rationale
   - What was decided, why, task ID
   - Example: `| PostgreSQL over SQLite | Need concurrent writes | TASK-005 |`

**Guidelines:**
- Keep entries concise (one line per item)
- Only add genuinely reusable knowledge
- Skip if nothing new was learned (empty tables are fine)
- Edit the tables directly - no special markup needed

## Output Format

Create a documentation summary and wrap it in artifact tags for automatic persistence:

<artifact>
## Documentation Summary

**Task**: {{TASK_TITLE}}

### Auto-Updated Sections
Look for sections marked with `<!-- orc:auto:* -->` and regenerate them:
- `<!-- orc:auto:api-endpoints -->` - API endpoints table
- `<!-- orc:auto:commands -->` - CLI commands table
- `<!-- orc:auto:config -->` - Configuration options

### Docs Created
- [path/to/doc.md]: [purpose]

### Docs Updated
- [path/to/doc.md]: [what changed]

### CLAUDE.md Status
[created/updated/verified]
</artifact>

## Phase Completion

After completing documentation updates, commit your changes:

```bash
git add -A
git commit -m "[orc] {{TASK_ID}}: docs - completed"
```

Then output:

```
**Commit**: [commit SHA]

<phase_complete>true</phase_complete>
```

If blocked (e.g., unclear what to document):
```
<phase_blocked>
reason: [what's blocking documentation]
needs: [what clarification is needed]
</phase_blocked>
```
