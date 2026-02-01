---
name: brainstorm-to-tasks
description: Use when a brainstorm or design is validated and needs to be converted into orc initiatives, tasks, and dependencies. Produces manifest.yaml for orc initiative plan or individual orc CLI commands.
---

# Brainstorm to Orc Tasks

## Overview

Convert a validated design into executable orc work items - initiatives, tasks, dependencies, and decisions.

**Core principle:** Every task must have enough context and specificity for Claude to execute it in isolation, because orc runs each task in a separate worktree with no shared session context.

**Announce at start:** "I'm using the brainstorm-to-tasks skill to convert this design into orc tasks."

## When to Use

- After brainstorming skill produces a validated design
- When converting any spec/plan into orc work items
- When decomposing a large feature into tracked tasks

**Not for:** Single tasks with obvious scope (just `orc new` directly).

## The Process

### Step 1: Extract from the Design

From the brainstorm output, identify:

| Element | Maps To | Required? |
|---------|---------|-----------|
| Overall goal / vision | Initiative vision | Yes, if 2+ tasks |
| Key decisions made | Initiative decisions | Yes |
| Components / work units | Tasks | Yes |
| Order constraints | `depends_on` / `blocked_by` | If any |
| Success criteria | Task descriptions | Yes |
| Testing strategy | Task descriptions + weight selection | Yes |

### Step 2: Decide Structure

```
Single task needed?
  └── Just run: orc new "title" -w <weight> -d "description"

Multiple related tasks?
  └── Create initiative + manifest.yaml
      └── orc initiative plan manifest.yaml --create-initiative
```

**Use an initiative when:** 2+ tasks share context, decisions, or ordering. The initiative's vision and decisions flow into every linked task's prompts.

### Step 3: Weight Each Task

Weight determines which phases run. **When in doubt, go ONE heavier.**

| Weight | When | Phases |
|--------|------|--------|
| `trivial` | One-liner fixes, typos | implement only |
| `small` | Bug fixes, isolated changes | tiny_spec → implement → review |
| `medium` | Features needing design thought | spec → tdd_write → implement → review → docs |
| `large` | Complex multi-file features | spec → tdd_write → breakdown → implement → review → docs |

**Common mistake:** Under-weighting. A "medium" task run as "small" skips the spec phase, so Claude guesses requirements instead of generating success criteria.

### Step 4: Write Task Descriptions

Each description flows into EVERY phase prompt. It's how you communicate with the executing Claude instance. Be specific:

**Good description:**
```
Prevent abuse by limiting API requests to 100/min per user.
Must return 429 with Retry-After header when exceeded.
Admin users (role=admin) are exempt from limits.
Rate state stored in Redis, not in-memory.
Must not add latency >5ms to normal requests.
```

**Bad description:**
```
Add rate limiting to the API.
```

**Include in every description:**
- What problem exists (the pain point)
- What success looks like (acceptance criteria hints)
- Constraints (performance, compatibility, etc.)
- Context Claude needs (related systems, edge cases)

### Step 5: Produce the Manifest

For multi-task work, create a `manifest.yaml`:

```yaml
version: 1
create_initiative:
  title: "Feature Name"
  vision: "What we're building and why - from the brainstorm goal"
tasks:
  - id: 1
    title: "First component"
    weight: medium
    category: feature
    priority: normal
    description: |
      Detailed description with context, success criteria hints,
      constraints, and edge cases. This flows into every phase prompt.
    spec: |
      # Optional inline spec (skips spec phase if provided)
      ## Intent
      Why this matters.
      ## Success Criteria
      - [ ] Testable condition 1
      - [ ] Testable condition 2
      ## Testing
      How to verify.

  - id: 2
    title: "Second component"
    weight: small
    category: feature
    priority: normal
    depends_on: [1]
    description: |
      Depends on task 1 being complete.
      Details about what this task does...
```

Then execute:
```bash
orc initiative plan manifest.yaml --create-initiative --yes
```

### Step 6: Record Decisions

After initiative creation, record key decisions from the brainstorm:

```bash
orc initiative decide INIT-XXX "Use JWT for auth tokens" \
  --rationale "Industry standard, stateless, works with our API gateway"

orc initiative decide INIT-XXX "Store rate limits in Redis" \
  --rationale "Shared state across instances, sub-ms lookups"
```

Decisions flow into all linked task prompts, keeping Claude aligned across tasks.

### Step 7: Verify and Activate

```bash
orc initiative show INIT-XXX        # Review tasks, deps, decisions
orc initiative activate INIT-XXX    # Mark ready to run
orc initiative run INIT-XXX         # Preview execution order
orc initiative run INIT-XXX --execute  # Actually run all tasks
```

## Task Decomposition Rules

### Granularity

Each task should be **one logical unit of work** that can execute independently in its own worktree:

- **Too big:** "Implement the entire auth system" → break down
- **Too small:** "Add import statement" → merge into parent task
- **Right size:** "Implement JWT token generation and validation"

### Independence

Tasks run in isolated worktrees. A task CANNOT:
- Depend on uncommitted work from another task
- Assume another task's branch is merged
- Share runtime state with other tasks

If task B needs code from task A, use `depends_on: [A]` - orc will run them in order and task B's worktree will have task A's changes merged.

### Dependency Rules

- Use `depends_on` only when task B literally cannot start without task A's code
- Don't over-constrain - parallel tasks finish faster
- No cycles allowed
- When in doubt, tasks are probably independent (they each get the full codebase)

## Quick Reference

| Action | Command |
|--------|---------|
| Create initiative | `orc initiative new "Title" --vision "..."` |
| Create single task | `orc new "Title" -w medium -d "..." -i INIT-XXX` |
| Create from manifest | `orc initiative plan manifest.yaml --create-initiative` |
| Record decision | `orc initiative decide INIT-XXX "Decision" --rationale "Why"` |
| Add dependency | `orc new "Title" --blocked-by TASK-XXX` |
| Link existing task | `orc initiative link INIT-XXX TASK-XXX` |
| Preview run order | `orc initiative run INIT-XXX` |
| Execute all | `orc initiative run INIT-XXX --execute` |
| Check status | `orc initiative show INIT-XXX` |

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Vague task descriptions | Include problem, success criteria, constraints, context |
| Under-weighting tasks | When in doubt, go one weight heavier |
| Over-constraining dependencies | Only depend when code from prior task is literally required |
| Skipping initiative for multi-task work | Initiative vision + decisions keep Claude aligned |
| No decisions recorded | Record every architectural choice with rationale |
| Giant monolith tasks | Break into independent units that fit one worktree |
| Tiny trivial tasks | Merge into logical parent task |
| Inline spec when spec phase would be better | Only use inline spec for well-understood work |

## Red Flags

**Never:**
- Create tasks with descriptions like "implement the feature" (too vague)
- Skip the initiative when tasks share context
- Forget to record decisions (Claude loses architectural context)
- Create circular dependencies
- Weight everything as "small" to go faster (skips critical phases)

**Always:**
- Include success criteria hints in descriptions
- Record decisions with rationale
- Verify dependency graph before activating
- Use `orc initiative show` to review before running
