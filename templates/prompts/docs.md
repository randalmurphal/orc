# Documentation Phase

You are updating documentation after implementation is complete for task {{TASK_ID}}.

<output_format>
**CRITICAL**: Your final output MUST be a JSON object with the documentation summary in the `content` field.

```json
{
  "status": "complete",
  "summary": "Documentation updated, hierarchy verified, templates followed",
  "content": "## Documentation Summary\n\n**Task**: {{TASK_TITLE}}\n\n### Staleness Audit\n- Files searched: N\n- Stale references fixed: N\n  - file:line: old -> new\n\n### Hierarchy Check\n- Parent docs referenced: [list]\n- Duplicate content removed: yes/no/none found\n\n### Docs Created\n| Path | Type | Lines | Purpose |\n|------|------|-------|---------|\n\n### Docs Updated\n| Path | Changes |\n|------|---------|\n\n### Line Counts\n- Root CLAUDE.md: N lines\n- [other files]: N lines\n\n### Navigation\n- llms.txt: created/updated/not needed"{{#if INITIATIVE_ID}},
  "initiative_notes": [
    {"type": "pattern|warning|learning|handoff", "content": "Concise but self-contained note", "relevant_files": ["optional/file/paths"]}
  ],
  "notes_rationale": "Brief explanation of why these notes were/weren't extracted"{{/if}}
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
## Failure Mode Priming

The most common failure is creating documentation that duplicates content from parent CLAUDE.md files or exceeds line limits for its level. Every child doc MUST contain only unique information.

Write maps to code, not tutorials. Tables over prose. File:line references over explanations. Claude reads code — provide structure and location references, not exhaustive walkthroughs.

## Hierarchical Inheritance (Non-Negotiable)

Claude loads CLAUDE.md from root to current directory. Everything becomes context, so verbosity = wasted tokens.

| Level | Max Lines | Include | Exclude |
|-------|-----------|---------|---------|
| Project root | 150-180 | Project standards, commands, structure | Subsystem details, implementation specifics |
| Package/subsystem | 100-150 | Architecture overview, unique patterns | Testing (project level), code style (project level) |
| Complex tool | 300-400 | Critical business logic, tool-specific gotchas | Patterns from parent, project standards |
| Reference docs (`docs/*.md`) | 500-1000+ | Comprehensive reference (loaded on-demand) | N/A |

**Never duplicate across levels:**
- Testing patterns -> project CLAUDE.md only
- Code style -> project CLAUDE.md only
- Core principles -> root CLAUDE.md only
- Subsystem patterns -> subsystem CLAUDE.md only

Child files MUST reference parent: "See project CLAUDE.md for testing standards"

## When to Extract to QUICKREF.md or docs/

Triggers (any one = extract):
1. CLAUDE.md exceeds 400 lines
2. More than 5 code examples with before/after patterns
3. Detailed walkthroughs > 50 lines per pattern
4. Comprehensive testing strategies with mock examples

| Destination | Content |
|-------------|---------|
| QUICKREF.md | Full code examples, detailed patterns, debugging strategies |
| docs/*.md | Domain-specific reference (API_REFERENCE.md, SCHEMA.md, etc.) |
| CLAUDE.md | Condensed summary + "See X for details" |
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

**Git State**: Previous phases (spec, tdd_write, implement, review) have already committed their work. The worktree is clean. Use `git log --oneline -10` to see recent commits.

DO NOT push to {{TARGET_BRANCH}} or any protected branch.
DO NOT checkout {{TARGET_BRANCH}} - stay on your task branch.
</worktree_safety>

{{INITIATIVE_CONTEXT}}
{{CONSTITUTION_CONTENT}}
</context>

<document_type_templates>
## OVERVIEW.md (100-200 lines) — Primary Pattern

**Purpose**: Give AI complete mental model in <5 minutes

```markdown
# [Component] Overview

**Purpose**: [One sentence]
**Performance**: [Key metrics if relevant]

## What It Does
- [Core responsibility 1]
- [Core responsibility 2]

## Key Components
| Component | Purpose | Location |
|-----------|---------|----------|
| [Name] | [What it does] | path/to/file.py |

## Data Flow
1. Input -> Process A -> Output
2. [Each step one line with file references]

## Critical Decisions
| Decision | Rationale | Trade-off |
|----------|-----------|-----------|

## Common Gotchas
- [Issue] - [Why] - [How to avoid]

## Related Docs
- [Links to detailed docs]
```

## Other Document Types — Summary

Use the OVERVIEW.md structure as the base pattern. Adapt sections per type:

| Type | Lines | Purpose | Key Sections |
|------|-------|---------|-------------|
| ARCHITECTURE.md | 200-400 | Patterns and coordination | Design Principles, Processing Pipeline (phase/purpose/input/output/location), Key Patterns (purpose/when/example/used-by), Integration Points, Performance |
| BUSINESS_RULES.md | 300-800 | Single source of truth for logic | Rules Index, Rule table (#/Rule/Condition/Behavior/WHY/Location), Detailed rules with Implementation + Test Coverage + Edge Cases |
| API_REFERENCE.md | 200-600 | Function catalog | Public Functions table, Per-function: signature, purpose, params, returns, raises, example, location |
| TROUBLESHOOTING.md | 150-300 | Fast issue resolution | Quick Diagnosis table (Symptom/Cause/Fix), Detailed issues with Cause/Fix/Validation, Debug Workflows |
| llms.txt | 50-150 | Navigation index (>10 doc files) | Quick Start links with line counts, By Domain sections, For Specific Tasks pointers |
</document_type_templates>

<instructions>
## Step 1: Staleness Audit (MANDATORY)

Before writing ANY new docs, search for stale references.

**Find and check:**
- All CLAUDE.md and AGENTS.md files in the repo
- Deprecated/TODO/FIXME/obsolete markers in docs/ and CLAUDE.md files
- Stale file path references in documentation
- ADR status (check if accepted ADRs have been superseded)

**For EVERY stale reference found:**
1. Update to match current implementation
2. Mark superseded ADRs as "Superseded" with date
3. Remove deprecated code examples
4. Update package descriptions

**Report format:**
```
Staleness audit:
- [file:line] - [what was stale] -> [what it was changed to]
```

## Step 1.5: Constitution Update (CRITICAL for bug fixes)

This creates a feedback loop: bugs -> invariants -> prevention.

If this task fixed a bug caused by violating an implicit rule:

1. **Check if constitution exists** at `.orc/CONSTITUTION.md`
2. **If bug was caused by pattern violation, ADD the invariant:**
   ```markdown
   | INV-XX | [Rule violated] | [How to verify] | [Why it matters] |
   ```
   Include reference: `(from {{TASK_ID}})`
3. **If feature established new pattern**, consider if it should be a default

## Step 2: Determine Documentation Scope

| Scope | Affected Area | Documentation Target |
|-------|---------------|---------------------|
| **project_local** | Single package/tool | Package CLAUDE.md, package docs/ |
| **parent_scope** | Shared subsystem | Parent CLAUDE.md, shared docs/ |
| **repo_scope** | Cross-cutting pattern | Root CLAUDE.md, root docs/ |

**Decision tree:**
1. Affects only one package? -> project_local
2. Affects a shared subsystem? -> parent_scope
3. Cross-cutting pattern/decision? -> repo_scope

## Step 3: Audit Missing Documentation

Check what exists vs what's needed in root level, changed directories, and architecture docs.

**Required for all projects:**
- Root `CLAUDE.md` or `AGENTS.md` (AI-readable)
- Root `README.md` (human-readable)

**Required for significant changes:**
- `docs/decisions/ADR-XXX.md` if architectural decision was made
- Package `CLAUDE.md` if complex package was added/changed
- `llms.txt` if >10 doc files exist

## Step 4: Select Document Type

| Content Type | Document | Template |
|--------------|----------|----------|
| System overview, mental model | `docs/OVERVIEW.md` | OVERVIEW.md pattern above |
| Design patterns, pipeline | `docs/ARCHITECTURE.md` | See summary table |
| Business logic, rules | `docs/BUSINESS_RULES.md` | See summary table |
| Function signatures | `docs/API_REFERENCE.md` | See summary table |
| Issue resolution | `docs/TROUBLESHOOTING.md` | See summary table |
| Quick reference, navigation | `CLAUDE.md` | Keep concise, link to docs/ |

## Step 5: Update Existing Docs

For each doc in the blast radius:

1. **Check accuracy** — does content match implementation?
2. **Update affected sections** — don't rewrite entire doc
3. **Add new features/APIs** — with file:line references
4. **Update examples** — if behavior changed
5. **Verify file:line references** — are they still accurate?
6. **Check line counts** — extract if exceeding limits

## Step 6: CLAUDE.md Quality Check

Verify hierarchical integrity: check line counts of all CLAUDE.md files and check for duplicate section headers across hierarchy levels.

**Quality criteria:**
- [ ] Under target line count for its level
- [ ] Tables over prose for structured content
- [ ] File:line references for implementation details
- [ ] No duplicate content from parent CLAUDE.md
- [ ] Accurate file layout section (if present)

## Step 7: Document Insights (If Applicable)

If this task established patterns, decisions, or gotchas — place them by scope:

| Scope | Insight Type | Location |
|-------|--------------|----------|
| project_local | Package gotcha | Package CLAUDE.md (Gotchas section) |
| project_local | Package pattern | Package docs/ (ARCHITECTURE.md) |
| parent_scope | Subsystem pattern | Parent docs/ (ARCHITECTURE.md) |
| repo_scope | Architecture decision | `docs/decisions/ADR-XXX.md` |
| repo_scope | Cross-cutting pattern | Root `docs/PATTERNS.md` |
| repo_scope | Business rule | `docs/BUSINESS_RULES.md` |
| repo_scope | Common issue | `docs/TROUBLESHOOTING.md` |

**Guidelines:** Use tables and structured formats. Include file:line references. Capture the WHY, not just the WHAT. One insight = one location.

**Skip if:** Task was routine with no novel insights, knowledge already documented, or insight is too task-specific.

## Step 8: Validate Documentation

**Run validation:**
- Doc lint script (if available)
- Check for broken internal markdown links
- Verify file:line references point to existing files
- Spot-check a sample of references for line accuracy

**Final checks:**
- [ ] All code blocks have correct syntax highlighting
- [ ] All internal links resolve to existing files
- [ ] All file:line references are accurate
- [ ] All examples are runnable
- [ ] No TODO/FIXME placeholders in final docs
- [ ] No references to deprecated/removed features
</instructions>

<validation_checklist>
## Pre-Completion Checklist

### Hierarchy
- [ ] Child CLAUDE.md files don't duplicate parent content
- [ ] Each level contains only unique information
- [ ] Cross-references use "See X for details" pattern

### Line Counts
- [ ] Root CLAUDE.md <= 180 lines
- [ ] Package CLAUDE.md files <= 150 lines
- [ ] No CLAUDE.md > 400 lines (extract to QUICKREF.md or docs/)

### Content Quality
- [ ] Tables over prose for structured content
- [ ] File:line references for implementation details
- [ ] No duplicate content across files
- [ ] Document templates followed for each doc type

### Accuracy
- [ ] File layout matches actual directory structure
- [ ] Code examples match current implementation
- [ ] All file:line references are valid
- [ ] All ADRs have correct status

### Navigation (large repos)
- [ ] llms.txt exists if >10 doc files
- [ ] Related docs linked from each document

### Constitution (bug fixes)
- [ ] If bug was caused by pattern violation, invariant added
- [ ] If new pattern established, considered for defaults
</validation_checklist>

{{#if INITIATIVE_ID}}
{{#if SUPPORTS_SUBAGENTS}}
<knowledge_curator>
## Initiative Knowledge Extraction (MANDATORY)

This task is part of Initiative {{INITIATIVE_ID}}. You MUST spawn a knowledge curator sub-agent to extract learnings for future tasks.

**Spawn using the Task tool:**
```
Task tool with subagent_type="general-purpose", model="sonnet", prompt="Extract initiative knowledge from task {{TASK_ID}}: {{TASK_TITLE}}

You are a knowledge curator extracting learnings from a completed task for future tasks in the same initiative.

## Task Context
- Task ID: {{TASK_ID}}
- Title: {{TASK_TITLE}}
- Initiative: {{INITIATIVE_ID}}
- Category: {{CATEGORY}}

## Existing Initiative Notes (for deduplication)
{{INITIATIVE_NOTES}}

## Note Types
| Type | When to Extract | Example |
|------|-----------------|---------|
| pattern | Reusable approach that worked well | 'Use repository pattern for all data access - see internal/repo/' |
| warning | Pitfall or gotcha future tasks should avoid | 'Don't modify legacy_handler.go directly - it has implicit state dependencies' |
| learning | Non-obvious discovery about the codebase | 'The config system merges files in reverse priority order' |
| handoff | Incomplete work or planned follow-up | 'Pagination not implemented - use TASK-XXX pattern when adding' |

## Quality Guidelines
1. **Be selective** - Only extract notes that will help future tasks. Most tasks don't produce notes.
2. **Be concise but self-contained** - Note should make sense without reading the full task context.
3. **Include file references** - Add relevant_files for patterns/warnings tied to specific code.
4. **Avoid duplicates** - Check existing notes above; don't repeat what's already captured.
5. **Think forward** - What would have helped YOU if you had this note before starting?

## Output Format
Return a JSON object:
{
  'initiative_notes': [
    {'type': 'pattern|warning|learning|handoff', 'content': 'Concise note text', 'relevant_files': ['optional/paths']}
  ],
  'rationale': 'Brief explanation of why these notes were/were not extracted'
}

If no notes are worth extracting, return empty array with rationale explaining why (e.g., 'routine implementation with no novel patterns').
"
```

**Wait for the sub-agent to complete.** Include its extracted notes in your final JSON output.

**If the sub-agent finds no notes worth extracting**, that's valid - include the empty array and rationale in your output.
</knowledge_curator>
{{/if}}
{{/if}}
