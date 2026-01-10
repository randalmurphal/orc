# Documentation Phase

You are creating and updating documentation after implementation is complete.

## Context

**Task ID**: ${TASK_ID}
**Task**: ${TASK_TITLE}
**Weight**: ${WEIGHT}
**Project Type**: ${PROJECT_TYPE}

## Implementation Summary

${IMPLEMENTATION_SUMMARY}

## Files Changed

${FILES_CHANGED}

## Current Documentation Status

${DOC_STATUS}

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

## Output Format

### Documentation Changes

For each file created or updated:

```markdown
### [Created/Updated]: path/to/doc.md

**Reason**: [Why this doc needed creation/update]

**Content Summary**: [Brief description of content]
```

## Phase Completion

### Commit Documentation

```bash
git add -A
git commit -m "[orc] ${TASK_ID}: docs - completed

Phase: docs
Status: completed
Docs created: [count]
Docs updated: [count]
"
```

### Output Completion

```
### Documentation Summary

**Project Type**: ${PROJECT_TYPE}
**Docs Created**: [list]
**Docs Updated**: [list]
**CLAUDE.md Status**: [created/updated/verified]
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
