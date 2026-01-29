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

**Key Insight**: Documentation = MAP to codebase, not replacement. AI agents can read code - provide structure and location references (`file:line`), not exhaustive explanations.

### Core Principles

| Principle | Rule |
|-----------|------|
| Concise over comprehensive | Tables, bullets, code snippets >>> Paragraphs |
| Location references | Include `file:line` for implementation details |
| Structure over prose | AI parses structured content faster |
| Hierarchical inheritance | Child docs ONLY contain unique info, reference parent for shared |

### Hierarchical Inheritance (CRITICAL)

**Claude loads CLAUDE.md from root → current directory.** Everything becomes context, so verbosity = wasted tokens.

| Level | Lines | What to Include | What to Exclude |
|-------|-------|-----------------|-----------------|
| Project root | 150-180 | Project standards, commands, structure | Subsystem details, implementation specifics |
| Package/subsystem | 100-150 | Architecture overview, unique patterns | Testing (project level), code style (project level) |
| Complex tool | 300-400 max | Critical business logic, tool-specific gotchas | Patterns from parent, project standards |
| Reference docs (`docs/*.md`) | 500-1000+ | Comprehensive reference, loaded on-demand | N/A - not auto-loaded |

**Never duplicate across levels:**
- Testing patterns → project CLAUDE.md only
- Code style → project CLAUDE.md only
- Core principles → root CLAUDE.md only
- Subsystem patterns → subsystem CLAUDE.md only

**Child files must reference parent:** "See project CLAUDE.md for testing standards"

### When to Extract to QUICKREF.md or docs/

**Triggers (any one = extract):**
1. CLAUDE.md exceeds 400 lines
2. More than 5 code examples with before/after patterns
3. Detailed implementation walkthroughs (>50 lines per pattern)
4. Comprehensive testing strategies with mock examples

**What goes where:**
- **QUICKREF.md**: Full code examples, detailed patterns, debugging strategies
- **docs/*.md**: Domain-specific reference (API_REFERENCE.md, SCHEMA.md, etc.)
- **CLAUDE.md**: Condensed summary + "See X for details"

---

## Document Type Templates

When creating documentation, use the appropriate template structure:

### OVERVIEW.md (100-200 lines)
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
1. Input → Process A → Output
2. [Each step one line with file references]

## Critical Decisions
| Decision | Rationale | Trade-off |
|----------|-----------|-----------|

## Common Gotchas
- [Issue] - [Why] - [How to avoid]

## Related Docs
- [Links to detailed docs]
```

### ARCHITECTURE.md (200-400 lines)
**Purpose**: Explain patterns and coordination

```markdown
# [Component] Architecture

## Design Principles
- [Principle]: [Why this matters]

## Processing Pipeline

### Phase 1: [Name]
**Purpose**: [What]
**Input/Output**: [Types]
**Location**: `module/file.py:function()`

## Key Patterns
### Pattern: [Name]
**Purpose**: [Why]
**When to use**: [Conditions]
**Example**: [Code snippet]
**Used by**: [Components]

## Integration Points
- Depends on: [A, B]
- Used by: [C, D]

## Performance Characteristics
- Time complexity: [Big O]
- Bottlenecks: [Known constraints]
```

### BUSINESS_RULES.md (300-800 lines)
**Purpose**: Single source of truth for business logic

```markdown
# [Component] Business Rules

## Rules Index
[Quick navigation]

## Rule Definitions
| # | Rule | Condition | Behavior | WHY | Location |
|---|------|-----------|----------|-----|----------|
| 1 | [Name] | When [X] | Do [Y] | [Reason] | file.py:L123 |

### Rule 1: [Name] (Detailed)
**Condition**: [When]
**Behavior**: [What]
**WHY**: [Business justification]
**Implementation**: `function()` @ file.py:123
**Test Coverage**: test_file.py::test_rule_1()
**Edge Cases**: [List]
```

### API_REFERENCE.md (200-600 lines)
**Purpose**: Function catalog for quick lookup

```markdown
# [Module] API Reference

## Public Functions
| Function | Purpose | Input | Output | Location |
|----------|---------|-------|--------|----------|

### `function_name(param1: Type) -> ReturnType`
**Purpose**: [One sentence]
**Parameters**: [List with descriptions]
**Returns**: [What]
**Raises**: [Exceptions]
**Example**: [Code snippet]
**Location**: `module/file.py:123`
```

### TROUBLESHOOTING.md (150-300 lines)
**Purpose**: Fast issue resolution

```markdown
# [Component] Troubleshooting

## Quick Diagnosis
| Symptom | Likely Cause | Fix |
|---------|--------------|-----|

## Common Issues
### Issue: [Symptom]
**Cause**: [Why]
**Fix**: [Steps with file references]
**Validation**: [How to confirm fixed]

## Debug Workflows
### When [Scenario]
1. Check [Location] for [What]
2. Run [Command]
3. If [Condition], then [Action]
```

### llms.txt (For Large Repos)
**Purpose**: Navigation index for AI agents
**When to create**: >10 doc files, multiple subsystems

```markdown
# llms.txt - Documentation Index

## Quick Start
- docs/OVERVIEW.md (150 lines) - System architecture
- docs/QUICK_REF.md (300 lines) - Common tasks

## By Domain
- docs/architecture/ - System design docs
- docs/business_rules/ - Logic references

## For Specific Tasks
**Adding new component**: Read docs/PATTERNS.md first
**Debugging**: Check docs/TROUBLESHOOTING.md
```

---

## Anti-Patterns (DO NOT DO)

### ❌ Don't Write Tutorials

**Bad**:
```markdown
# How to Add a New Processor
In this guide, we'll walk through adding a new processor step by step...
[500 lines of tutorial]
```

**Good**:
```markdown
# Adding New Processor - Checklist
**Base class**: `processors/base.py:BaseProcessor`
**Reference**: See `example_processor.py`
**Steps**:
1. Create file extending BaseProcessor
2. Implement required methods (see API_REFERENCE.md)
3. Register in main.py
4. Add tests
```

### ❌ Don't Explain Obvious Code

**Bad**: "The function first checks if the record is None. If it is None, it returns False..."

**Good**: "**Skip insert**: severity in ['0', 'Pass'] - Location: `processor.py:234` - WHY: Pass = compliant"

### ❌ Don't Mix Abstraction Levels

**Bad** (OVERVIEW.md with implementation details):
```markdown
# System Overview
The AsyncClient uses sessionless API key authentication to download data in parallel chunks of 5000 records with asyncio.gather()...
```

**Good**:
```markdown
# System Overview
## Pipeline
1. **Download**: Async parallel chunking (100x faster)
2. **Process**: Parallel workers with caching
**For implementation**: See architecture/DOWNLOAD.md
```

### ❌ Don't Duplicate Across Hierarchy

**Bad**:
```markdown
# Global CLAUDE.md
## Testing: 95% coverage required

# Project CLAUDE.md
## Testing: 95% coverage required  ← DUPLICATE!

# Tool CLAUDE.md
## Testing: 95% coverage required  ← DUPLICATE AGAIN!
```

**Good**:
```markdown
# Global CLAUDE.md
## Testing: 95% coverage required

# Project CLAUDE.md
## Testing Structure
- See global CLAUDE.md for coverage requirements

# Tool CLAUDE.md
## Testing
- See project CLAUDE.md for structure
```

---

## Instructions

### Step 1: Staleness Audit (MANDATORY)

**Before writing ANY new docs, search for stale references:**

```bash
# Find all CLAUDE.md files
find . -name "CLAUDE.md" -o -name "AGENTS.md" | head -20

# Check for deprecated patterns
grep -rn "deprecated\|TODO\|FIXME\|obsolete" docs/ **/CLAUDE.md 2>/dev/null

# Check for stale file references
grep -roh '\bpath/to/\|\.yaml\b' docs/ CLAUDE.md 2>/dev/null | sort -u

# ADR status check
grep -l "Status.*Accepted" docs/decisions/ 2>/dev/null | while read f; do
  echo "Check if superseded: $f"
done
```

**For EVERY stale reference found:**
1. Update to match current implementation
2. Mark superseded ADRs as "Superseded" with date
3. Remove deprecated code examples
4. Update package descriptions

**Report format:**
```
Staleness audit:
- [file:line] - [what was stale] → [what it was changed to]
```

### Step 1.5: Constitution Update (CRITICAL for bug fixes)

**This creates a feedback loop: bugs → invariants → prevention.**

If this task fixed a bug caused by violating an implicit rule:

1. **Check if constitution exists:**
   ```bash
   cat .orc/CONSTITUTION.md 2>/dev/null || echo "No constitution"
   ```

2. **If bug was caused by pattern violation, ADD the invariant:**
   ```markdown
   | INV-XX | [Rule violated] | [How to verify] | [Why it matters] |
   ```
   Include reference: `(from {{TASK_ID}})`

3. **If feature established new pattern, consider if it should be a default**

### Step 2: Determine Documentation Scope

Identify what level of documentation is affected:

| Scope | Affected Area | Documentation Target |
|-------|---------------|---------------------|
| **project_local** | Single package/tool | Package CLAUDE.md, package docs/ |
| **parent_scope** | Shared subsystem | Parent CLAUDE.md, shared docs/ |
| **repo_scope** | Cross-cutting pattern | Root CLAUDE.md, root docs/ |

**Decision tree:**
1. Does this change affect only one package? → project_local
2. Does it affect a shared subsystem? → parent_scope
3. Is it a cross-cutting pattern/decision? → repo_scope

### Step 3: Audit Missing Documentation

Check what exists vs what's needed:

```bash
# Root level
ls -la README.md CLAUDE.md AGENTS.md 2>/dev/null

# Changed directories
for dir in [affected_paths]; do
  ls -la $dir/CLAUDE.md $dir/README.md 2>/dev/null
done

# Architecture docs
ls -la docs/architecture/ docs/decisions/ 2>/dev/null

# Check if llms.txt needed (>10 doc files)
find docs -name "*.md" 2>/dev/null | wc -l
```

**Required for all projects:**
- Root `CLAUDE.md` or `AGENTS.md` (AI-readable)
- Root `README.md` (human-readable)

**Required for significant changes:**
- `docs/decisions/ADR-XXX.md` if architectural decision was made
- Package `CLAUDE.md` if complex package was added/changed
- `llms.txt` if >10 doc files exist

### Step 4: Select Document Type

Based on what you're documenting, use the appropriate template:

| Content Type | Document | Template Section |
|--------------|----------|------------------|
| System overview, mental model | `docs/OVERVIEW.md` | OVERVIEW.md template |
| Design patterns, pipeline | `docs/ARCHITECTURE.md` | ARCHITECTURE.md template |
| Business logic, rules | `docs/BUSINESS_RULES.md` | BUSINESS_RULES.md template |
| Function signatures | `docs/API_REFERENCE.md` | API_REFERENCE.md template |
| Issue resolution | `docs/TROUBLESHOOTING.md` | TROUBLESHOOTING.md template |
| Quick reference, navigation | `CLAUDE.md` | Keep concise, link to docs/ |

### Step 5: Update Existing Docs

For each doc in the blast radius:

1. **Check accuracy** - Does content match implementation?
2. **Update affected sections** - Don't rewrite entire doc
3. **Add new features/APIs** - With file:line references
4. **Update examples** - If behavior changed
5. **Verify file:line references** - Are they still accurate?
6. **Check line counts** - Extract if exceeding limits

### Step 6: CLAUDE.md Quality Check

**Verify hierarchical integrity:**
```bash
# Check line counts
wc -l CLAUDE.md
find . -name "CLAUDE.md" -exec wc -l {} \; 2>/dev/null | sort -rn

# Check for duplication (should find minimal matches)
grep -h "^## " CLAUDE.md */CLAUDE.md 2>/dev/null | sort | uniq -d
```

**Quality criteria:**
- [ ] Under target line count for its level
- [ ] Tables over prose for structured content
- [ ] File:line references for implementation details
- [ ] No duplicate content from parent CLAUDE.md
- [ ] Accurate file layout section (if present)

**If exceeds limits, apply optimization strategies:**

| Strategy | When to Use | How |
|----------|-------------|-----|
| **Table-ify** | Prose explaining logic | Convert to What/When/Why/Where columns |
| **Condense trees** | Full directory listing | Show key files, use `[N more files]` |
| **Extract-Reference** | Duplicate content | Single source + "See X for details" |
| **Bullets over paragraphs** | Explanatory text | One concept per line |

### Step 7: Document Insights (If Applicable)

If this task established patterns, decisions, or gotchas:

**Placement by scope:**

| Scope | Insight Type | Location | Template |
|-------|--------------|----------|----------|
| project_local | Package gotcha | Package CLAUDE.md | Gotchas section |
| project_local | Package pattern | Package docs/ | ARCHITECTURE.md |
| parent_scope | Subsystem pattern | Parent docs/ | ARCHITECTURE.md |
| repo_scope | Architecture decision | `docs/decisions/ADR-XXX.md` | ADR format |
| repo_scope | Cross-cutting pattern | Root `docs/PATTERNS.md` | ARCHITECTURE.md |
| repo_scope | Business rule | `docs/BUSINESS_RULES.md` | BUSINESS_RULES.md |
| repo_scope | Common issue | `docs/TROUBLESHOOTING.md` | TROUBLESHOOTING.md |

**Guidelines:**
- Use tables and structured formats (not prose)
- Include file:line references for implementation details
- Capture the WHY (rationale), not just the WHAT
- One insight = one location (no duplicating)

**Skip if:**
- Task was routine with no novel insights
- Knowledge already documented elsewhere
- Insight is too task-specific to help future work

### Step 8: Validate Documentation

**Run validation:**

```bash
# Doc lint (if available)
./scripts/doc-lint.sh 2>/dev/null || echo "No doc-lint script"

# Check for broken internal links
grep -roh '\[.*\](.*\.md)' docs/ CLAUDE.md 2>/dev/null | \
  sed 's/.*(\(.*\))/\1/' | while read f; do
    [ ! -f "$f" ] && echo "Broken link: $f"
  done

# Verify file:line references exist
grep -roh '[a-zA-Z_/]*\.[a-z]*:[0-9]*' docs/ CLAUDE.md 2>/dev/null | \
  head -10 | while read ref; do
    file=$(echo $ref | cut -d: -f1)
    line=$(echo $ref | cut -d: -f2)
    [ -f "$file" ] || echo "Missing file: $file"
  done
```

**Final checks:**
- [ ] All code blocks have correct syntax highlighting
- [ ] All internal links resolve to existing files
- [ ] All file:line references are accurate
- [ ] All examples are runnable
- [ ] No TODO/FIXME placeholders in final docs
- [ ] No references to deprecated/removed features

---

## Validation Checklist

### Hierarchy
- [ ] Child CLAUDE.md files don't duplicate parent content
- [ ] Each level contains only unique information
- [ ] Cross-references use "See X for details" pattern
- [ ] No testing/code style repeated outside project level

### Line Counts
- [ ] Root CLAUDE.md ≤ 180 lines
- [ ] Package CLAUDE.md files ≤ 150 lines
- [ ] No CLAUDE.md > 400 lines (extract to QUICKREF.md or docs/)
- [ ] Reference docs (docs/*.md) can exceed 500 lines

### Content Quality
- [ ] Tables over prose for structured content
- [ ] Bullet points over paragraphs
- [ ] File:line references for implementation details
- [ ] No duplicate content across files
- [ ] Document templates followed for each doc type

### Accuracy
- [ ] File layout matches actual directory structure
- [ ] Code examples match current implementation
- [ ] All file:line references are valid
- [ ] All ADRs have correct status (Accepted/Superseded)

### Navigation (for large repos)
- [ ] llms.txt exists if >10 doc files
- [ ] llms.txt has task-specific navigation sections
- [ ] Related docs linked from each document

### Constitution (for bug fixes)
- [ ] If bug was caused by pattern violation, invariant added
- [ ] If new pattern established, considered for defaults

---

## Output Format

**CRITICAL**: Your final output MUST be a JSON object with the documentation summary in the `content` field.

```markdown
## Documentation Summary

**Task**: {{TASK_TITLE}}

### Staleness Audit
- Files searched: [count]
- Stale references fixed: [count]
  - [file:line]: [old] → [new]

### Hierarchy Check
- Parent docs referenced: [list]
- Duplicate content removed: [yes/no/none found]

### Docs Created
| Path | Type | Lines | Purpose |
|------|------|-------|---------|
| [path] | [OVERVIEW/ARCHITECTURE/etc] | [count] | [purpose] |

### Docs Updated
| Path | Changes |
|------|---------|
| [path] | [what changed] |

### Line Counts
- Root CLAUDE.md: [count] lines
- [other files]: [count] lines

### Navigation
- llms.txt: [created/updated/not needed]
```

## Phase Completion

Output a JSON object:

```json
{
  "status": "complete",
  "summary": "Documentation updated, hierarchy verified, templates followed",
  "content": "## Documentation Summary\n\n**Task**: Feature X\n..."
}
```

If blocked:
```json
{
  "status": "blocked",
  "reason": "[what's blocking and what clarification is needed]"
}
```
