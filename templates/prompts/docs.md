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

---

## AI Documentation Standards

**Key Insight**: Documentation = MAP to codebase, not replacement. AI agents can read code - provide structure and location references, not exhaustive explanations.

### Core Principles

| Principle | Rule |
|-----------|------|
| Concise over comprehensive | Tables, bullets, code snippets >>> Paragraphs |
| Location references | Include `file:line` for implementation details |
| Structure over prose | AI parses structured content faster |
| No duplication | Define once at appropriate level, reference elsewhere |

### CLAUDE.md Line Count Rules

| Level | Target Lines | Focus |
|-------|--------------|-------|
| Project root | 150-180 | Project standards, commands, structure |
| Package/subsystem | 100-150 | Architecture overview, key patterns |
| Simple tool | 200-250 | Tool-specific logic, gotchas |
| Complex tool | 300-400 max | Critical business logic, architecture |

**Rule**: Context-loaded files (CLAUDE.md) must be concise. Reference docs in `docs/` can be longer.

### Anti-Patterns

- **Don't write tutorials** - AI needs REFERENCE, not teaching
- **Don't explain obvious code** - One-line summary + file:line + WHY
- **Don't duplicate across hierarchy** - Testing in project, core principles in global
- **Don't mix abstraction levels** - OVERVIEW stays high-level

---

## Instructions

### Step 1: Staleness Audit (MANDATORY)

**Before writing ANY new docs, search for stale references:**

```bash
# Storage model consistency (all locations)
grep -rn "source of truth" docs/ internal/**/CLAUDE.md web/CLAUDE.md
grep -rn "YAML.*source\|hybrid.*storage\|HybridBackend" docs/ internal/ web/
grep -rn "\.yaml.*task\|task.*\.yaml" docs/ CLAUDE.md  # Except FILE_FORMATS.md

# File watcher (deprecated - now uses database events)
grep -rn "file.*watcher\|watcher\s*monitor\|watcher/" docs/ internal/**/CLAUDE.md web/CLAUDE.md

# Deprecated tech/framework references
grep -rn "matching Svelte\|mirror.*Svelte\|Svelte.*store" web/ docs/

# Package-specific staleness
grep -rn "initiative.*YAML\|YAML.*initiative" internal/initiative/ docs/

# ADR status check
grep -l "Status.*Accepted" docs/decisions/ | while read f; do
  echo "Check if superseded: $f"
done

# Knowledge table duplicate TASK IDs
grep -oE 'TASK-[0-9]+' CLAUDE.md | sort | uniq -d
```

**For EVERY stale reference found:**
1. Update to match current implementation
2. Mark superseded ADRs as "Superseded" with date
3. Remove deprecated code examples
4. Update package descriptions in CLAUDE.md files

**Report format:**
```
Staleness audit:
- [file:line] - [what was stale] → [what it was changed to]
```

### Step 1.5: Constitution Update (CRITICAL for bug fixes)

**This creates a feedback loop: bugs → invariants → prevention.**

If this task fixed a bug caused by violating an implicit rule, capture the invariant:

1. **Check if constitution exists:**
   ```bash
   cat .orc/CONSTITUTION.md 2>/dev/null || orc constitution show
   ```

2. **If bug was caused by pattern violation, ADD the invariant:**

   Edit `.orc/CONSTITUTION.md` directly (it's git-tracked) and add a row to the Invariants table:
   ```markdown
   | INV-XX | [Rule that was violated] | [How to verify] | [Why it matters] |
   ```
   Include reference: `(from {{TASK_ID}})`

   **Examples of bugs → invariants:**
   - Silent error swallowing → `INV: All errors must be logged or returned`
   - Race condition → `INV: Shared state requires mutex protection`
   - Missing validation → `INV: All user input validated at boundary`

3. **If feature established new pattern, consider if it should be a default:**
   - Does this pattern apply broadly across the codebase?
   - Would violating it cause bugs or confusion?
   - Add to Defaults table if yes

**If testing commands or patterns changed:**
- Update the test command in CLAUDE.md/README.md
- Note any new test fixtures or mocking patterns in knowledge section

### Step 2: Audit Missing Documentation

Check what exists and what's missing:

1. **Root level**: README.md, CLAUDE.md, CONTRIBUTING.md
2. **Changed directories**: README.md, CLAUDE.md in affected paths
3. **Architecture docs**: docs/architecture/ if structural changes
4. **API docs**: If public interfaces changed

### Step 3: Create Missing Required Docs

**For all projects:**
- `CLAUDE.md` at root (AI-readable project overview)
- `README.md` at root (human-readable overview)

**For monorepos:**
- `README.md` in each package/module directory
- Optional `CLAUDE.md` for complex packages

**For significant changes:**
- Architecture docs if new components added
- ADR if architectural decisions were made

### Step 4: Update Existing Docs

For each existing doc in the blast radius:

1. Check if content is still accurate
2. Update sections affected by changes
3. Add documentation for new features/APIs
4. Update examples if behavior changed
5. Update command references if CLI changed

### Step 5: CLAUDE.md Quality Check

Ensure CLAUDE.md files are:
- Under target line count (see table above)
- Use tables over prose
- Include Quick Start section
- Have accurate Structure table
- List actual commands that work
- No duplicate content from parent CLAUDE.md

**Verify line counts (excluding knowledge tables):**

```bash
# Root CLAUDE.md (target: 150-180, max 400)
head -n $(grep -n "orc:knowledge:begin" CLAUDE.md | cut -d: -f1) CLAUDE.md | wc -l

# Package CLAUDE.md files (target: 100-150)
find internal -name "CLAUDE.md" -exec wc -l {} \; | sort -rn

# Web CLAUDE.md (target: 200-250 for complex tools)
wc -l web/CLAUDE.md
```

If any file exceeds limits:
- Extract detailed content to `docs/*.md` or `QUICKREF.md`
- Keep CLAUDE.md as concise reference with "See X for details"

### Step 5.1: File Layout Accuracy Check

If CLAUDE.md contains a File Layout or Directory Structure section:

```bash
# Verify documented structure matches reality
ls -la ~/.orc/ 2>/dev/null || echo "No global ~/.orc/"
ls -la .orc/ 2>/dev/null || echo "No project .orc/"
ls .orc/*.db 2>/dev/null || echo "No database files"
```

**Common issues:**
- Showing YAML files that no longer exist (task.yaml, state.yaml, plan.yaml)
- Showing directories that were removed (watcher/)
- Missing actual structure (orc.db)

Update File Layout to match actual current structure.

### Step 6: Validate Documentation

- All code blocks have correct syntax highlighting
- All internal links resolve to existing files
- All examples are runnable
- No TODO/FIXME placeholders in final docs
- No references to deprecated/removed features

### Step 7: Update Project Knowledge

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

**Before adding entries, check for duplicates:**

```bash
# Extract all TASK-XXX references from knowledge section
sed -n '/orc:knowledge:begin/,/orc:knowledge:end/p' CLAUDE.md | grep -oE 'TASK-[0-9]+' | sort | uniq -d
```

If duplicates found, review each case:
- **Same insight in multiple tables**: Consolidate to most appropriate table (Gotcha > Pattern for bug fixes)
- **Different insights from same task**: Valid - one task can produce multiple learnings
- **Example**: TASK-016 can have two Gotchas (embed issue + go.work issue) if they're different problems

---

## Pre-Completion Validation (REQUIRED)

Before marking phase complete, run the doc-lint check:

```bash
./scripts/doc-lint.sh
```

If any file exceeds threshold + tolerance (BLOCK status):
1. Extract detailed content to reference docs (SCHEMA.md, ENDPOINTS.md, etc.)
2. Add pointer in CLAUDE.md: `See [file.md](file.md) for details`
3. Re-run lint until passing

**Do NOT signal completion until lint passes.**

---

## Validation Checklist (Run Before Completing)

### Doc Lint
- [ ] `./scripts/doc-lint.sh` passes (no BLOCK status)

### Staleness
- [ ] No references to deprecated storage patterns (YAML source of truth, hybrid storage)
- [ ] No references to removed packages/frameworks
- [ ] All ADRs have correct status (Accepted/Superseded)
- [ ] Code examples match current implementation

### Constitution (for bug fixes)
- [ ] If bug was caused by pattern violation, invariant added to `.orc/CONSTITUTION.md`
- [ ] If new pattern established, considered for Defaults table

### Line Counts
- [ ] Root CLAUDE.md ≤ 180 lines (excluding knowledge tables)
- [ ] Package CLAUDE.md files ≤ 150 lines
- [ ] No CLAUDE.md > 400 lines

### No Duplication
- [ ] Testing standards not repeated (project level only)
- [ ] Core principles not repeated (global level only)
- [ ] Package patterns not repeated (parent level only)

### Structure
- [ ] Tables over prose for business logic
- [ ] Bullet points over paragraphs
- [ ] File references include path (and line when relevant)

### Knowledge Tables
- [ ] No same-insight duplicates across Patterns/Gotchas/Decisions (different insights OK)
- [ ] Each entry has Pattern/Issue, Description, and Source columns
- [ ] Source column uses TASK-XXX format

### File Layout
- [ ] File Layout section matches actual directory structure
- [ ] No references to non-existent files (task.yaml, plan.yaml, state.yaml)
- [ ] Database file (orc.db) shown if applicable

---

## Output Format

**CRITICAL**: Your final output MUST be a JSON object with the documentation summary in the `content` field.

Create a documentation summary following this structure:

```markdown
## Documentation Summary

**Task**: {{TASK_TITLE}}

### Staleness Audit Results
- Files searched: [count]
- Stale references found: [count]
- Fixes applied:
  - [file:line]: [old] → [new]

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
[created/updated/verified] - [line count]
```

## Phase Completion

Output a JSON object with the summary in the `content` field:

```json
{
  "status": "complete",
  "summary": "Documentation updated, lint passed",
  "content": "## Documentation Summary\n\n**Task**: Feature X\n..."
}
```

If blocked (e.g., unclear what to document):
```json
{
  "status": "blocked",
  "reason": "[what's blocking documentation and what clarification is needed]"
}
```
