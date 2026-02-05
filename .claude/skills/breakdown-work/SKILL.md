---
name: breakdown-work
description: Use when a brainstorm or design is validated and ready to be decomposed into orc initiatives, tasks, and dependencies. The step after brainstorming — breaks down a plan into executable orc work items with proper weights, categories, and execution structure.
---

# Work Breakdown

## Overview

Convert a validated design into executable orc work — initiatives, tasks, dependencies, and decisions.

**Core principle:** Each task executes in an isolated worktree with no shared session context. Every task must be self-contained enough for Claude to execute it alone, guided only by its description, the spec phase output, initiative context, and project constitution.

## When to Use

- After the brainstorming skill produces a validated design
- When converting any spec/plan into orc work items
- When decomposing a large feature into tracked, executable tasks

**Not for:** Single tasks with obvious scope (just `orc new` directly).

## How Orc Executes Tasks

Understanding the execution pipeline is critical for creating good tasks.

### Weight → Phases → Quality

Weight is the highest-leverage decision. It determines which phases run:

| Weight | Phases | When to Use |
|--------|--------|-------------|
| `trivial` | implement | One-liner fixes, typos, config tweaks |
| `small` | tiny_spec → implement → review | Bug fixes, isolated changes (max 3 success criteria) |
| `medium` | spec → tdd_write → implement → review → docs | Features needing design thought |
| `large` | spec → tdd_write → breakdown → implement → review → docs | Complex multi-file features, intertwined work |

**Under-weighting is the #1 mistake.** A medium task run as small skips the spec phase entirely — Claude guesses requirements instead of generating behavioral success criteria. **When in doubt, go one heavier.**

### What Each Phase Does

| Phase | Input | Output | Why It Matters |
|-------|-------|--------|----------------|
| **spec** | Task description + initiative context | Behavioral success criteria, failure modes, edge cases, wiring checklist | Foundation for everything — bad spec = bad implementation |
| **tiny_spec** | Task description | Combined spec + TDD (max 3 criteria) | Lighter spec for small tasks |
| **tdd_write** | Spec output | Failing tests mapped to each SC-X criterion | Tests exist BEFORE implementation, ensuring correctness |
| **breakdown** | Spec + TDD tests | Ordered checklist of implementation steps | Decomposes large tasks into atomic units within ONE worktree |
| **implement** | Spec + TDD tests + breakdown | Working code that passes all tests | Must make TDD tests pass, verify all success criteria |
| **review** | All prior outputs | Multi-agent review (6 specialized reviewers) | No-op detection, spec compliance, code quality |

### What Flows Into Prompts

Every phase prompt receives:

| Variable | Source | Impact |
|----------|--------|--------|
| `{{TASK_DESCRIPTION}}` | Your description text | **Primary guidance** — be specific |
| `{{TASK_CATEGORY}}` | category field | Changes spec analysis entirely (bug → root cause, refactor → before/after) |
| `{{INITIATIVE_CONTEXT}}` | Vision + all decisions | Keeps Claude aligned across tasks |
| `{{SPEC_CONTENT}}` | Spec phase output | Drives TDD, implement, and review phases |
| `{{CONSTITUTION_CONTENT}}` | `.orc/CONSTITUTION.md` | Project-wide coding principles |

## The Process

### Step 1: Assess Task Boundaries

Before creating anything, decide how to decompose the work. This is the critical step.

**The Merge-Up Rule:** If two pieces of work are deeply intertwined — they modify the same files, share new interfaces, or one can't be meaningfully tested without the other — **combine them into one larger task.** Don't split tightly coupled work into separate tasks with dependencies.

Why: Each orc task runs in its own worktree with its own Claude session. Splitting coupled work means:
- Task B's worktree won't have Task A's uncommitted experiments
- Claude in Task B has zero context from Task A's session
- Dependencies add sequential execution overhead
- Merge conflicts between coupled branches are painful

The `large` weight exists for exactly this purpose — the **breakdown phase** decomposes complex work into ordered implementation steps that execute within a single worktree and session.

```
Can Task B be implemented and tested
without ANY code from Task A?
  ├── Yes, completely independent → Separate tasks (parallel execution)
  ├── B needs A's code merged first, → Separate tasks with depends_on
  │   but they touch different areas    (sequential, but clean boundaries)
  └── They share interfaces, modify  → ONE larger task (use large weight,
      the same files, or are deeply     breakdown phase handles ordering)
      coupled
```

### Step 2: Set Categories Correctly

Category changes how the spec phase analyzes the work:

| Category | Spec Phase Behavior |
|----------|-------------------|
| `feature` | User stories, success criteria, integration wiring checklist |
| `bug` | Root cause analysis, reproduction steps, **pattern prevalence check** (finds same bug in other code paths) |
| `refactor` | Before/after pattern analysis, risk assessment, caller impact |
| `chore` | Minimal spec, focused on verification |
| `docs` | Documentation-focused criteria |
| `test` | Test coverage focused |

**Don't default everything to `feature`.** A bug categorized as feature misses root cause analysis. A refactor categorized as feature generates unnecessary user stories.

### Step 2.5: Choose the Right Workflow

**Weight suggests a default workflow, but you can override it.** Different workflows exist for different types of work:

| Workflow | Use For | Phases |
|----------|---------|--------|
| `implement-trivial` | One-liner fixes, typos | implement |
| `implement-small` | Bug fixes, isolated changes | tiny_spec → implement → review → docs |
| `implement-medium` | Features needing design | spec → tdd → implement → review → docs |
| `implement-large` | Complex multi-file features | spec → tdd → breakdown → implement → review → docs |
| `qa-e2e` | E2E testing, QA verification | test → fix loop |
| `docs` | Documentation only | docs |
| `review` | Code review only | review |
| `spec` | Specification only | spec |

**Match workflow to work type, not just complexity:**

| Task Type | Wrong | Right |
|-----------|-------|-------|
| QA/testing task | implement-medium | qa-e2e |
| Documentation task | implement-small | docs |
| Pure code review | implement-medium | review |
| Spec/design only | implement-medium | spec |

In manifests, explicitly set workflow when the default (based on weight) isn't appropriate:

```yaml
tasks:
  - id: 1
    title: "QA: Verify pagination works end-to-end"
    weight: medium
    workflow: qa-e2e  # Override: this is testing, not implementing
    category: test
    description: |
      E2E tests for pagination feature...
```

### Step 3: Write Task Descriptions

The description flows into EVERY phase prompt via `{{TASK_DESCRIPTION}}`. It's your primary communication channel with the executing Claude instance.

**What makes a good description:**

```yaml
description: |
  The /api/users endpoint returns all users with no pagination, causing
  timeouts on large datasets (>10k users).

  Add limit/offset pagination with:
  - Default limit=20, max=100
  - Total count in X-Total-Count response header
  - Invalid limit/offset returns 400 with descriptive error
  - Existing callers (web UI user list, admin export) must not break

  The web UI user list component at web/src/pages/UsersPage.tsx currently
  fetches all users. It will need updating to use pagination, but that's
  a separate task.
```

**What it includes:**
- The actual problem (why this matters)
- Specific requirements (what success looks like)
- Constraints and edge cases
- What NOT to touch (scope boundaries)
- References to existing code (Claude will read these files)

**What makes a bad description:**
```yaml
description: Add pagination to the API.
```

The spec phase amplifies good descriptions and struggles with vague ones. **Garbage in, garbage out.**

### Step 4: Decide on Inline Specs vs Generated Specs

The spec phase is excellent — it generates behavioral success criteria with quality checklists, failure modes, edge cases, and integration wiring verification. **Let it do its job in most cases.**

| Situation | Use Inline Spec? | Why |
|-----------|------------------|-----|
| You've already done detailed analysis | Yes | Avoid redundant work |
| Well-understood, narrowly-scoped change | Maybe | If you can write behavioral SC-X criteria |
| New feature, complex change | **No** | Let spec phase analyze the codebase |
| Bug fix | **No** | Spec phase does root cause + pattern prevalence |
| Refactor | **No** | Spec phase does before/after + blast radius |

**If you use inline specs, they must match spec phase quality:**
- Behavioral success criteria (SC-X format with verification methods)
- Not "file exists" — "file does X when given Y"
- Failure modes and edge cases
- In/out scope explicitly listed

### Step 5: Create the Initiative

Use an initiative when 2+ tasks share context. The initiative's vision and decisions flow into every linked task's prompts.

**Create via manifest** (preferred for multi-task work):

```yaml
version: 1
create_initiative:
  title: "User List Pagination"
  vision: |
    Add pagination across the user management system. API endpoints
    return paginated results with total counts. Web UI components
    use infinite scroll or page controls. Admin export still gets
    all records via a separate bulk endpoint.
tasks:
  - id: 1
    title: "Add pagination to /api/users endpoint"
    weight: medium
    # workflow: implement-medium  # Optional: override if weight default isn't right
    category: feature
    priority: high
    description: |
      The /api/users endpoint returns all users with no pagination.
      Add limit/offset pagination with default limit=20, max=100.
      Return total count in X-Total-Count header.
      Invalid params return 400 with descriptive error.
      Must not break existing callers.

  - id: 2
    title: "Add bulk export endpoint for admin"
    weight: small
    category: feature
    priority: normal
    description: |
      Admin users need to export all users for compliance reporting.
      Add GET /api/users/export that returns all records as CSV.
      Requires admin role. Rate limited to 1 request per minute.

  - id: 3
    title: "Update web UI user list to use pagination"
    weight: medium
    category: feature
    priority: normal
    depends_on: [1]
    description: |
      web/src/pages/UsersPage.tsx currently fetches all users.
      Update to use the paginated /api/users endpoint.
      Add page controls (prev/next, page size selector).
      Show total count. Handle loading and empty states.

  - id: 4
    title: "QA: Verify pagination end-to-end"
    weight: medium
    workflow: qa-e2e  # IMPORTANT: Testing tasks use qa-e2e, not implement-*
    category: test
    priority: normal
    depends_on: [1, 3]
    description: |
      E2E tests verifying pagination works correctly:
      - API returns correct page sizes
      - UI displays page controls
      - Navigation between pages works
      - Edge cases (empty, single page, last page)
```

Then execute:
```bash
orc initiative plan manifest.yaml --create-initiative
```

This creates the initiative, all tasks with proper dependencies, and auto-assigns workflows based on weight. Tasks are topologically sorted.

**Or create individually** (for adding tasks to existing initiatives):
```bash
orc new "Add pagination to /api/users" -w medium -c feature -i INIT-001 \
  -d "The /api/users endpoint returns all users..."
```

### Step 6: Record Decisions

Decisions appear in every linked task's prompts as:
```
### Decisions
The following decisions have been made for this initiative:
- Use bcrypt for passwords: Industry standard, battle-tested
- Stateless JWT design: Horizontal scaling requirement
```

Record every architectural choice:
```bash
orc initiative decide INIT-XXX "Use limit/offset pagination (not cursor)" \
  --rationale "Simpler to implement, adequate for our dataset size (<100k records)"

orc initiative decide INIT-XXX "Return total count in header, not body" \
  --rationale "Keeps response body as pure array, consistent with REST conventions"
```

**Good decisions to record:**
- Technology choices with rationale
- Architectural patterns chosen (and why)
- Scope boundaries ("X is out of scope because Y")
- Constraints discovered during brainstorm

### Step 7: Verify and Execute

```bash
orc initiative show INIT-XXX        # Review everything
orc initiative activate INIT-XXX    # Mark ready
orc initiative run INIT-XXX         # Preview execution order
orc initiative run INIT-XXX --execute  # Run all tasks
```

## Task Decomposition Rules

### The Right Granularity

| Signal | Action |
|--------|--------|
| Task touches 1-3 files in one area | Probably right-sized |
| Task touches 5+ files across areas | Consider splitting IF areas are independent |
| Two tasks modify the same files | Merge into one larger task |
| Task introduces interface + all consumers | One task (intertwined) |
| Task B can't compile without Task A's new types | Merge or use depends_on |
| Both tasks are medium but deeply coupled | One large task (breakdown handles ordering) |

### When to Use `depends_on`

`depends_on` means "Task B's worktree needs Task A's code merged into the target branch first."

**Use when:**
- Task B imports a package Task A creates
- Task B calls an API endpoint Task A implements
- Task B's tests need Task A's database migrations
- The tasks are in clearly different areas but have a directional dependency

**Don't use when:**
- Tasks modify the same files (merge them)
- Tasks share new interfaces being designed (merge them)
- You want ordering but there's no actual code dependency (unnecessary constraint)
- "It feels like they should be ordered" (that's not a dependency)

### Independence Test

For each task, ask: **"Can Claude implement and test this in a fresh worktree with only the target branch code (plus completed dependency branches), the task description, and the initiative context?"**

If yes → task is properly scoped.
If no → either merge with the task it depends on, or add the missing context to the description/decisions.

## Quick Reference

| Action | Command |
|--------|---------|
| Create from manifest | `orc initiative plan manifest.yaml --create-initiative` |
| Preview without creating | `orc initiative plan manifest.yaml --dry-run` |
| Create initiative only | `orc initiative new "Title" --vision "..."` |
| Create single task | `orc new "Title" -w medium -c feature -d "..." -i INIT-XXX` |
| Record decision | `orc initiative decide INIT-XXX "Decision" --rationale "Why"` |
| Link existing task | `orc initiative link INIT-XXX TASK-XXX` |
| Add blocker after creation | Tasks: `--blocked-by TASK-XXX` flag on `orc new` |
| Review before running | `orc initiative show INIT-XXX` |
| Preview run order | `orc initiative run INIT-XXX` |
| Execute all tasks | `orc initiative run INIT-XXX --execute` |

## Common Mistakes

| Mistake | Why It Fails | Fix |
|---------|-------------|-----|
| Vague descriptions | Spec phase amplifies vagueness | Include problem, requirements, constraints, scope |
| Under-weighting | Skips spec and/or TDD phases | When in doubt, one weight heavier |
| Splitting coupled work | Merge conflicts, lost context between sessions | Merge into one large task, let breakdown decompose |
| Over-constraining deps | Forces sequential execution unnecessarily | Only depend when code is literally required |
| Skipping initiative | Claude loses cross-task architectural context | Always use initiative for 2+ related tasks |
| Wrong category | Bug as feature misses root cause analysis | Set category accurately — it changes spec analysis |
| Always using inline specs | Skips the excellent spec phase analysis | Let spec phase generate unless you've done equivalent analysis |
| No decisions recorded | Each task's Claude reinvents architectural choices | Record every decision with rationale |
| Everything is medium | Some work needs large (breakdown) or small (light spec) | Match weight to actual complexity |
| Wrong workflow for task type | QA task runs implement phases, wastes effort | Use `qa-e2e` for testing, `docs` for documentation, etc. |
| **Deprecation hedging** | Creates legacy cruft, fallback paths, tech debt | See "Code Removal" section below — DELETE or KEEP, no hedging |

## Code Removal vs Legacy

When a task replaces existing code, be explicit about what happens to the old code.

### The Rule

**DELETE or KEEP. No middle ground.**

| Action | When to Use | Task Description Says |
|--------|-------------|----------------------|
| **DELETE** | New code fully replaces old | "DELETE `old_file.go` entirely" |
| **KEEP** | Old code needed for compatibility window | "KEEP for 2 releases, remove in v3.0" (with ticket) |

### Forbidden Language

Never use these in task descriptions:

| ❌ Don't Write | ✅ Write Instead |
|----------------|------------------|
| "deprecate the old approach" | "DELETE the old approach" |
| "either migrate or keep alongside" | Pick one. State which. |
| "mark as legacy" | "DELETE — git has history" |
| "add fallback to old behavior" | Only if explicitly requested |
| "keep for backward compatibility" | Only with explicit timeline and removal ticket |

### Why This Matters

Each orc task runs in isolation. Hedging language like "deprecate or migrate" gives Claude permission to:
- Keep both implementations "just in case"
- Add fallback paths that complicate the codebase
- Create tech debt that never gets cleaned up

The executing Claude doesn't know what you actually wanted — it sees "or" and picks the safer (messier) option.

### Example

**Bad:**
```yaml
description: |
  Refactor the claim system. The existing task_claims table in team.go
  uses a different schema. Either migrate it or create the new one
  alongside and deprecate the old approach.
```

**Good:**
```yaml
description: |
  Refactor the claim system to use atomic operations.

  **DELETE the old approach:**
  - DELETE internal/db/team.go entirely
  - DELETE TeamMember, TaskClaim types
  - The new users table + atomic claim replaces ALL of this

  **Do NOT:**
  - Keep team.go "for compatibility"
  - Add fallback logic
  - Deprecate — just delete
```
