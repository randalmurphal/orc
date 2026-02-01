# UX Simplification Brainstorm

**Status:** In Progress
**Depends on:** INIT-036 (Workflow System Redesign)
**Goal:** Make orc intuitive - users should be able to use its power without thinking about the underlying complexity.

---

## Core Problem

> "I built it and I don't understand it."

Creating a workflow requires jumping between 5+ pages. Nothing guides users through configuration. Features exist but are scattered and disconnected.

---

## Agreed Principles

| Principle | Meaning |
|-----------|---------|
| **Workflow-centric** | Workflows are the primary concept users interact with |
| **Never leave the editor** | Everything needed to configure a workflow is accessible from the workflow editor |
| **Progressive disclosure** | Show common configs, collapse advanced ones |
| **Guided for novices, fast for veterans** | Wizard flow with skip option |
| **One modal, full context** | Modals are self-contained, don't navigate away |
| **Smart defaults** | Pre-fill sensibly, user only changes what they want |

---

## Agreed Navigation Changes

### Current
```
Board | Initiatives | Timeline | Stats | Workflows | Agents | Environ | Settings | Help
```

### Proposed
```
Board | Initiatives | Timeline | Stats | Workflows | Settings | Help
```

| Change | Rationale |
|--------|-----------|
| Kill **Agents** from nav | Executor agents are implementation details. Agent config moves to phase inspector. Standalone agents move to Settings > Agents. |
| Kill **Environ** from nav | Merge into Settings > Environment |
| Kill **Phase Templates** section | Phases accessed only through editor palette |

---

## Agreed Workflows Page Redesign

### Current Structure
```
Workflows Page
├── Built-in Workflows (cards)
├── Custom Workflows (cards)
└── Phase Templates (separate section, confusing)
```

### Proposed Structure
```
Workflows Page
├── Your Workflows (custom, editable)
└── Built-in (clone to customize)

[+ New Workflow] → Opens guided creation flow
```

Phase Templates removed from this page entirely.

---

## Agreed Workflow Editor Enhancements

### Current 3-Panel Layout
```
┌──────────────┬─────────────────────┬──────────────────┐
│ Palette      │ Canvas              │ Inspector        │
│ (phases)     │ (React Flow)        │ (Settings tab)   │
└──────────────┴─────────────────────┴──────────────────┘
```

### Proposed Enhancements

**Left Palette:**
- Phases (draggable, as today)
- **[+ New Phase]** button → opens Create Phase modal inline
- **Agents** section (collapsible) - drag to assign?
- **Workflow Settings** section (collapsible) - default model, thinking, completion action

**Right Inspector (when phase selected):**
- **Executor** dropdown (NEW - missing today!)
- Model dropdown
- Gate dropdown
- Max iterations
- **▸ Sub-Agents** (collapsible, add/remove)
- **▸ Prompt** (collapsible, view/edit)
- **▸ Advanced** (collapsible - thinking, hooks, skills, MCP, tools)
- **▸ Triggers** (collapsible - before-phase triggers)
- **▸ Gate Config** (collapsible - input context, output actions)
- **▸ Loop Config** (collapsible - if phase has loop)
- **▸ Condition** (collapsible - if phase is conditional)

---

## Agreed Guided Creation Flow

When clicking **[+ New Workflow]**:

### Step 1: Intent
```
What kind of workflow?

[Build] [Review] [Test] [Document] [Custom]

                              [Skip to Editor →]
```

### Step 2: Name
```
Name your workflow

┌─────────────────────────────────────┐
│ Code Review with Security           │
└─────────────────────────────────────┘

ID: code-review-security (auto-generated)

▸ Description (optional)
▸ Default model (optional)
```

### Step 3: Phases
```
Choose your phases

Recommended for "Review":
☑ Code Review
☐ Security Scan
☐ Docs Check

Or drag to reorder...
```

### Step 4: Opens Editor
With phases pre-configured, user can refine.

---

---

## Agreed: Gates as Transitions (Edge-Based Model)

### Mental Model Shift

**Old model:** Gates are a property OF phases (gate_type on PhaseTemplate)
**New model:** Gates ARE the transitions BETWEEN phases (edges in the graph)

```
Phases = Nodes (the work)
Gates = Edges (the transitions between work)
```

### Visual Representation

Lines flow from left canvas edge → through phases → to right canvas edge.
Gate symbols (◆) sit on the edges. No explicit Start/End nodes.

```
══════◆══════╗              ╔══════◆══════╗              ╔══════◆════════
              ║              ║              ║              ║
         ┌────╨────┐    ┌────╨────┐    ┌────╨────┐
         │  Spec   │    │Implement│    │ Review  │
         └─────────┘    └─────────┘    └─────────┘
              ↑              ↑              ↑
           Gate 1         Gate 2         Gate 3
```

### Gate Symbol States

| Visual | Color | Meaning |
|--------|-------|---------|
| `─────` | Gray | Passthrough (no gate) |
| `──○──` | Blue | Auto (system checks) |
| `──◆──` | Yellow | Human approval required |
| `──◆──` | Purple | AI evaluates |
| `──◆──` | Red | Blocked/failed |
| `──◆──` | Green | Passed |

### Interaction

- **Click gate symbol** → Inspector shows gate config
- **Hover** → Tooltip with config summary
- **Drag edge** → Gate travels with the connection

### Single-Phase Workflow

Same visual, just one phase with entry and exit gates:

```
══════◆══════╗              ╔══════◆════════
              ║              ║
         ┌────╨────┐
         │  Phase  │
         └─────────┘
```

### Data Model Implication

Gates move from PhaseTemplate to WorkflowPhase edges. Each edge has:
- `condition`: When can we traverse? (auto checks, human, AI, always)
- `on_fail`: What if we can't? (retry, retry_from, fail, pause)
- `reviewer`: Who evaluates? (agent_id for AI gates)
- `context`: What info flows to reviewer? (phase outputs, task details)

### Loops Are Just Backward Edges

```
══◆══╗        ╔══◆══╗        ╔══◆══
     ║        ║     ║        ║
┌────╨───┐ ┌──╨────┐ ┌──────╨┐
│  Spec  │ │Implmnt│ │Review │
└────────┘ └───────┘ └───────┘
               ↑          │
               └────◆─────┘
                 Loop gate
```

No separate "loop config" - it's just a gate on a backward edge with its own condition.

### Triggers Fit Inside Gates

Triggers become "run before" / "run after" actions in the gate's Advanced section:
- Run before: validation scripts, pre-checks
- Run after: notifications, cleanup

---

## Agreed: Complete Configuration Model

### Three Levels

```
Workflow (container)
├── Workflow Settings (defaults, git, completion)
├── Phases[] (the work)
│   └── Phase Settings (executor, model, prompt, output)
└── Gates[] (transitions)
    └── Gate Settings (approval, failure handling)
```

### Workflow-Level Settings

| Category | Setting | Options | Default |
|----------|---------|---------|---------|
| **Identity** | name | string | required |
| | description | string | optional |
| **Defaults** | default_model | opus / sonnet / haiku | sonnet |
| | default_thinking | bool | false |
| | default_max_iterations | number | 10 |
| **Git Mode** | git_mode | `worktree` / `branch` / `none` | worktree |
| | source_branch | string | current branch |
| | target_branch | string | main |
| **Completion** | completion_action | `pr` / `merge` / `commit` / `none` | pr |
| | pr_draft | bool | false |
| | pr_labels | string[] | [] |
| | pr_reviewers | string[] | [] |
| | auto_merge | bool | false |
| | wait_for_ci | bool | false |

**Git Mode:**
- `worktree`: Create isolated worktree + branch (standard task execution)
- `branch`: Work on branch in main repo (quick changes)
- `none`: No git operations (research, specs, non-code tasks)

**Completion Action:**
- `pr`: Create pull request (standard)
- `merge`: Merge directly (dangerous, auto-merge scenarios)
- `commit`: Commit but no PR (local-only)
- `none`: No git actions (just produce output)

### Phase-Level Settings

| Category | Setting | Options | Default |
|----------|---------|---------|---------|
| **Identity** | template_id | phase template | required |
| | name_override | string | from template |
| **Execution** | executor | agent_id | from template |
| | model_override | opus / sonnet / haiku / inherit | inherit |
| | thinking_override | bool / inherit | inherit |
| | max_iterations | number | from template |
| **Prompt** | prompt_source | `template` / `custom` / `file` | template |
| | prompt_content | string | from template |
| | prompt_file | path | - |
| **Data Flow** | input_vars | string[] | from template |
| | output_var | string | from template |
| | produces_artifact | bool | from template |
| | artifact_type | spec / tests / docs / etc | from template |
| **Environment** | working_dir | `worktree` / `main` / `custom` | worktree |
| | env_vars | key-value | {} |
| | tools | tool list | from agent |
| | mcp_servers | server list | [] |
| | skills | skill list | [] |
| | hooks | hook list | [] |

### Gate-Level Settings

| Category | Setting | Options | Default |
|----------|---------|---------|---------|
| **Approval** | approval_type | `auto` / `human` / `ai` / `none` | auto |
| **Auto Config** | criteria | has_output, no_errors, completion_marker, custom | [has_output, no_errors] |
| | custom_check | string pattern | - |
| **Human Config** | review_prompt | string | - |
| **AI Config** | reviewer | agent_id | - |
| | context_sources | phase_outputs, task_details, vars | [phase_outputs] |
| **Failure** | on_fail | `retry` / `retry_from` / `fail` / `pause` | retry |
| | retry_from | phase_id | current phase |
| | max_retries | number | 3 |
| **Advanced** | before_script | path | - |
| | after_script | path | - |
| | store_result_as | variable name | - |

### Built-in Workflows Using This Model

| Workflow | Git Mode | Completion | Phases | Notable Gates |
|----------|----------|------------|--------|---------------|
| implement-large | worktree | pr | spec→tdd→breakdown→implement→review→docs | AI after review |
| implement-small | worktree | pr | tiny_spec→implement→review | Auto gates |
| implement-trivial | worktree | pr | implement | Auto gate |
| review | none | none | review | Human gate |
| spec | none | none | spec | Auto gate |
| qa-e2e | worktree | pr | test↔fix (loop) | Loop gate |
| docs | worktree | pr | docs | Auto gate |

---

---

## Agreed: Task Creation Flow (Workflow-First)

### Kill Weight Entirely

Weight (trivial/small/medium/large) is dead. Workflows are the only thing that matters for determining execution.

| Old Model | New Model |
|-----------|-----------|
| Weight determines workflow | Workflow chosen directly |
| `--weight medium` | `--workflow implement-medium` |
| Weight as required field | Weight removed from model |

### UI: Two-Step Flow

**Step 1: Workflow Picker**
```
┌─────────────────────────────────────────────────────────────────┐
│ New Task                                                        │
├─────────────────────────────────────────────────────────────────┤
│ Choose a workflow                                               │
│                                                                 │
│ ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│ │ ★ Implement     │  │   Implement     │  │   Implement     │  │
│ │   (Small)       │  │   (Medium)      │  │   (Large)       │  │
│ │ 3 phases        │  │ 5 phases        │  │ 6 phases        │  │
│ └────────●────────┘  └─────────────────┘  └─────────────────┘  │
│      Default                                                    │
│                                                                 │
│ ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│ │   Review        │  │   QA E2E        │  │   Spec Only     │  │
│ └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│                                                                 │
│                                              [Cancel] [Next →]  │
└─────────────────────────────────────────────────────────────────┘
```

**Step 2: Task Details**
```
┌─────────────────────────────────────────────────────────────────┐
│ New Task                                                        │
├─────────────────────────────────────────────────────────────────┤
│ Workflow: Implement (Small)                      [← Change]     │
│                                                                 │
│ Title                                                           │
│ ┌─────────────────────────────────────────────────────────────┐│
│ │ Fix authentication bug in login flow                        ││
│ └─────────────────────────────────────────────────────────────┘│
│                                                                 │
│ Description (optional)                                          │
│ ┌─────────────────────────────────────────────────────────────┐│
│ │                                                             ││
│ └─────────────────────────────────────────────────────────────┘│
│                                                                 │
│ ▸ Advanced (category, priority, initiative, branch)            │
│                                                                 │
│                              [← Back] [Create] [Create & Run]   │
└─────────────────────────────────────────────────────────────────┘
```

### CLI: Interactive + Explicit

**Interactive (default):**
```bash
$ orc new "Fix auth bug"

? Select workflow:
  ★ implement-small (default)
    implement-medium
    implement-large
    review
    qa-e2e

Created TASK-042: Fix auth bug
```

**Explicit (skip picker):**
```bash
$ orc new "Fix auth bug" --workflow implement-small
$ orc new "Fix auth bug" -w small  # Short alias
```

**List workflows:**
```bash
$ orc workflows
ID               NAME                  PHASES  DEFAULT
implement-small  Implement (Small)     3       ★
implement-medium Implement (Medium)    5
...
```

### Project Defaults

```yaml
# .orc/config.yaml
defaults:
  workflow: implement-small           # Default for "orc new"

  category_workflows:                 # Override by category
    bug: implement-small
    feature: implement-medium
    docs: docs
```

---

---

## Agreed: Two-Level View Hierarchy

### Board (Dashboard) - Attention Management

Shows what needs focus. Compact, sorted by priority.

```
┌─────────────────────────────────────────────────────────────────────────┐
│  RUNNING                                                                │
│  ┌──────────────────────────────────────┐  ┌──────────────────────────┐│
│  │ TASK-042 Fix auth bug                │  │ TASK-045 Add caching    ││
│  │ ● Implement (1:23)                   │  │ ✓ Review → ◆ Waiting    ││
│  │ ████████░░░░░░░░                     │  │ ████████████████░░      ││
│  └──────────────────────────────────────┘  └──────────────────────────┘│
│                                                                         │
│  NEEDS ATTENTION                                                        │
│  ┌──────────────────────────────────────┐                              │
│  │ ◆ TASK-045 - Gate waiting            │  [Approve] [View]           │
│  └──────────────────────────────────────┘                              │
│                                                                         │
│  QUEUE                                                                  │
│  TASK-046 Refactor database layer        implement-medium   Ready      │
│  TASK-047 Update API docs                docs                Ready      │
└─────────────────────────────────────────────────────────────────────────┘
```

Click task → Task Detail Page

### Task Detail Page - Deep Work

Everything needed to understand and collaborate on a task.

```
┌─────────────────────────────────────────────────────────────────────────┐
│  ← Back to Board                                                        │
│                                                                         │
│  TASK-042: Fix authentication bug in login flow                         │
│  Workflow: implement-small  •  Branch: orc/TASK-042  •  ⏱ 3:42         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  WORKFLOW PROGRESS                                                      │
│  ═══◆═══╗        ╔═══●═══╗        ╔═══○═══                             │
│         ║        ║       ║        ║                                     │
│    ┌────╨───┐ ┌──╨──┐ ┌──╨────┐                                        │
│    │ ✓ Spec │ │●Impl│ │○Review│                                        │
│    └────────┘ └─────┘ └───────┘                                        │
│                                                                         │
├───────────────────────────────┬─────────────────────────────────────────┤
│  LIVE OUTPUT                  │  CHANGES                                │
│                               │                                         │
│  Reading login.go...          │  internal/auth/login.go         +32 -8 │
│  Found token validation issue │  internal/auth/validate.go      +12 -0 │
│  Editing login.go             │  internal/auth/login_test.go    +18 -2 │
│                               │                                         │
│  Running tests...             │  ─────────────────────────────────────  │
│  ✓ TestLogin                  │   func Login(ctx context.Context) {    │
│  ✓ TestLoginInvalid           │  +    if err := validateToken(); err { │
│  (12 more...)                 │  +        return fmt.Errorf("...")     │
│                               │                                         │
│  [Expand Output]              │  [View Full Diff] [Open in GitHub]     │
│                               │                                         │
├───────────────────────────────┴─────────────────────────────────────────┤
│  Tokens: 45.2K  •  Cost: $1.96  •  Iterations: 2/10                    │
│  [Pause] [Cancel] [Retry from Spec]                                    │
└─────────────────────────────────────────────────────────────────────────┘
```

Split pane (lazygit-inspired):
- Left: Live output (truncated smartly)
- Right: Changes with inline diff preview

### Diff View (lazygit-Inspired)

Full diff review with file list + diff panel:

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Changes for TASK-042                                    [Close]        │
├─────────────────────┬───────────────────────────────────────────────────┤
│  FILES              │  DIFF                                             │
│                     │                                                   │
│  ● login.go    +32  │  internal/auth/login.go                          │
│    validate.go +12  │  ─────────────────────────────────────────────── │
│    login_test  +18  │                                                   │
│                     │  @@ -45,6 +45,14 @@ func Login(ctx ...           │
│                     │   func Login(ctx context.Context, creds ...       │
│                     │  +    if err := validateToken(token); err != nil {│
│                     │  +        return fmt.Errorf("token: %w", err)     │
│                     │       user, err := db.GetUser(ctx, creds.Email)   │
│                     │                                                   │
├─────────────────────┴───────────────────────────────────────────────────┤
│  [Approve Changes] [Request Changes] [Add Comment] [Back]              │
└─────────────────────────────────────────────────────────────────────────┘
```

External links: View on GitHub, Open PR, Compare with main

---

## Agreed: Real-Time Agent Collaboration

### The Vision

You're not just running tasks - you're **pair programming with the agent**. Watch, comment, steer.

### Inline Comments on Code

Click any line in the diff to add feedback:

```
│  +    if err := validateToken(token); err != nil {                 │
│  +        return fmt.Errorf("token: %w", err)                      │
│                                                                    │
│  💬 ┌────────────────────────────────────────────────────────────┐│
│     │ Use validateSession() instead - it already handles both   ││
│     │ token AND session validation. See auth/session.go:84      ││
│     └────────────────────────────────────────────────────────────┘│
```

### Feedback Timing Options

| Option | Behavior | Use When |
|--------|----------|----------|
| **Send Now** | Pause agent, inject feedback, resume | Urgent: "Stop! Wrong approach" |
| **Send When Done** | Queue, inject after phase (before gate) | Non-urgent: "Also consider X" |
| **Save for Later** | Keep in queue, manual send | Collecting thoughts |

### Feedback Panel

```
┌─────────────────────────────────────────────────────────────────────────┐
│  FEEDBACK TO AGENT                                          [Collapse] │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Pending feedback (2):                                                  │
│  • 📍 login.go:47 - "Use validateSession() instead"                    │
│  • 📝 General - "Also add a test for expired tokens"                   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ Add a note...                                                   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  [Send Now - Pause Agent]  [Send When Phase Done]  [Save for Later]    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Feedback Types

| Type | UI | What Agent Sees |
|------|-----|-----------------|
| **Inline comment** | Click line → add comment | "User commented on file.go:47: ..." |
| **General note** | Text box | "User note: ..." |
| **Approval hint** | "Looks good" button | "User approved current direction" |
| **Direction change** | "Try different approach" | "User wants different approach: ..." |

### Backend Requirements

```go
type Feedback struct {
    ID       string
    Type     string    // "inline", "general", "approval", "direction"
    File     string    // For inline comments
    Line     int       // For inline comments
    Text     string
    Timing   string    // "now", "when_done", "manual"
    SentAt   time.Time
    Received bool
}
```

API:
- `POST /tasks/{id}/feedback` - Add feedback
- `POST /tasks/{id}/feedback/send` - Send queued feedback (triggers pause if needed)
- Feedback injected into agent context on next prompt

---

## Agreed: CLI is Agent-First

No interactive prompts by default. Clear output. Excellent help text.

```bash
# Uses project default - no prompts
orc new "Fix auth bug"

# Explicit workflow
orc new "Fix auth bug" --workflow implement-small
orc new "Fix auth bug" -w small

# Error if no default and no flag (not a prompt)
$ orc new "Fix bug"
Error: No default workflow. Use --workflow <id> or set defaults.workflow

# Machine-readable output
orc status --json
orc workflows --json
```

TUI (bubbletea) will be the interactive human mode - designed later based on web UI.

---

---

## Agreed: Variable System UX

### Clean Canvas, Details on Demand

- **Canvas**: No variable clutter on nodes
- **Hover**: Tooltip shows inputs/outputs
- **Inspector**: Full variable details when phase selected

```
Hover tooltip:
┌─────────────────────────┐
│ Inputs: SPEC_CONTENT    │
│         BREAKDOWN       │
│ Output: IMPLEMENTATION  │
└─────────────────────────┘
```

Inspector shows full details with ability to configure variable mappings.

---

## Agreed: Agent Management

### Two Locations

| Location | Purpose |
|----------|---------|
| **Settings > Agents** | Create, edit, delete agents (power user) |
| **Workflow Editor palette** | Quick-assign to phases, browse available |

### Agent Types

| Type | Use |
|------|-----|
| **Built-in** | Curated list (code-reviewer, qa-functional, etc.) |
| **Custom** | Users can define with prompts, tools, model |

Most users pick from built-ins. Power users create custom.

### No More Auto-Generated Executors

Kill the "{Phase}-executor" pattern. Phases just reference agents by ID.

---

## Agreed: Error States & Recovery

### Board (Compact Error)

```
┌──────────────────────────────────────┐
│ TASK-042 Fix auth bug                │
│ ✗ Implement (failed)                 │
│ ████████████░░░░░░░░ Error           │
│ [View] [Retry] [Abort]               │
└──────────────────────────────────────┘
```

### Task Detail (Full Context)

```
┌─────────────────────────────────────────────────────────────────────────┐
│  ═══✓═══╗        ╔═══✗═══╗        ╔═══○═══                             │
│         ║        ║       ║        ║                                     │
│    ┌────╨───┐ ┌──╨──┐ ┌──╨────┐                                        │
│    │ ✓ Spec │ │✗Impl│ │○Review│                                        │
│    └────────┘ └─────┘ └───────┘                                        │
│                                                                         │
├─────────────────────────────────────────────────────────────────────────┤
│  ✗ PHASE FAILED: Implement                                              │
│                                                                         │
│  Error: Tests failed after 3 attempts                                   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ === FAILED: TestLoginInvalidToken ===                           │   │
│  │ Expected: error "invalid token"                                 │   │
│  │ Got: nil                                                        │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  [Retry Implement]  [Retry from Spec]  [Fix Manually]  [Abort Task]    │
│                                                                         │
│  💬 Add guidance for retry: [_______________________________]          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Recovery Actions

| Action | Behavior |
|--------|----------|
| **Retry this phase** | Re-run with error context injected |
| **Retry from earlier** | Go back to spec/tdd/etc |
| **Fix manually** | Mark for manual intervention, user fixes, then resume |
| **Abort task** | Stop entirely, clean up |

### Guidance on Retry

User can add a note injected into retry context:

```
💬 "The validateToken function needs to return an error for empty strings"
```

Agent sees this guidance when retrying.

---

## Summary: The New Orc UX

### Core Philosophy

| Principle | Implementation |
|-----------|----------------|
| **Workflow-centric** | Everything revolves around workflows. Kill weight. |
| **Gates are edges** | Transitions between phases, not properties of phases |
| **Never leave the editor** | All config accessible from workflow editor |
| **Progressive disclosure** | Common stuff visible, advanced collapsed |
| **Agent-first CLI** | No interactive prompts, clear errors, JSON output |
| **Collaborative execution** | Real-time feedback to agent while running |

### Navigation (Simplified)

```
Board | Initiatives | Timeline | Stats | Workflows | Settings | Help
```

Killed: Agents (moved to Settings), Environ (moved to Settings)

### Key Screens

| Screen | Purpose |
|--------|---------|
| **Board** | Attention management - what needs focus |
| **Workflows Page** | List workflows, create new (guided) |
| **Workflow Editor** | 3-panel: palette, canvas, inspector |
| **Task Detail Page** | Deep work - live output, diff, feedback |
| **Settings > Agents** | Agent management (power user) |

### Data Model Changes

| Change | Old | New |
|--------|-----|-----|
| Weight | Required, determines workflow | **Killed** |
| Gates | Property of phase template | **Edge between phases** |
| Gate types | auto/human/ai/skip | Approval: Auto/Human/AI/None |
| Executors | Auto-generated "{Phase}-executor" | **Direct agent reference** |
| Triggers | Separate concept | **Inside gate advanced config** |
| Loops | Separate loop_config | **Backward edge with gate** |

### New Capabilities

| Feature | Description |
|---------|-------------|
| **Guided workflow creation** | Step-by-step slideshow for new workflows |
| **Real-time agent feedback** | Comment on code, steer agent mid-execution |
| **Inline diff review** | lazygit-inspired split-pane diff view |
| **External links** | Direct links to GitHub/GitLab PR, branch, diff |
| **Feedback timing** | Send now (pause) / Send when done / Save for later |

### What's Next

1. **Wait for INIT-036 to complete** - Backend infrastructure
2. **Create new initiative** - UX Simplification based on this brainstorm
3. **Break into tasks** - Following the agreed design
4. **Implement iteratively** - Navigation → Editor → Task views → Feedback system

---

## Open Questions (For Implementation)

1. Edge rendering - How to visually distinguish forward edges vs loop edges vs conditional edges?
2. Agent assignment - Dropdown in inspector vs drag from palette vs both?
3. Workflow testing - "Run this workflow" button in editor for quick testing?
4. Mobile/narrow screens - Responsive behavior for the 3-panel editor?
5. Keyboard shortcuts - Power user navigation in diff view, editor?
6. Offline/disconnected - What happens when WebSocket drops during task execution?

---

## Appendix: ASCII Visual Reference

### Workflow Canvas
```
══════◆══════╗              ╔══════◆══════╗              ╔══════◆════════
              ║              ║              ║              ║
         ┌────╨────┐    ┌────╨────┐    ┌────╨────┐
         │  Spec   │    │Implement│    │ Review  │
         └─────────┘    └─────────┘    └─────────┘
```

### Gate Symbols
```
─────  Gray    = Passthrough (no gate)
──○──  Blue    = Auto (system checks)
──◆──  Yellow  = Human approval
──◆──  Purple  = AI evaluates
──◆──  Red     = Failed/blocked
──◆──  Green   = Passed
```

### Task Progress
```
✓ = Completed
● = Active/running
○ = Pending
✗ = Failed
◆ = Gate waiting
```

## Reference: Current Data Model

```
Task (ProjectDB)
  └── workflow_id ──→ Workflow (GlobalDB)
                          └── phases[] ──→ WorkflowPhase
                                              ├── phase_template_id ──→ PhaseTemplate
                                              │                           ├── agent_id (executor)
                                              │                           ├── sub_agent_ids
                                              │                           ├── gate_type
                                              │                           ├── gate_agent_id
                                              │                           ├── gate_input_config
                                              │                           └── gate_output_config
                                              ├── model_override
                                              ├── gate_type_override
                                              ├── condition
                                              ├── loop_config
                                              └── before_triggers[]
```

---

## Reference: Gate System

| Type | Behavior | Config |
|------|----------|--------|
| `auto` | Deterministic checks (has_output, no_errors, completion_marker) | criteria[] |
| `human` | User approval (CLI interactive or API headless) | - |
| `ai` | LLM evaluates with schema | gate_agent_id, gate_input_config |
| `skip` | No gate, always continues | - |

**Gate Input Config:** What context flows to evaluator (phase outputs, task details, extra vars)
**Gate Output Config:** What happens after (on_approved, on_rejected, retry_from, script, variable_name)

---

## Reference: Trigger System

**Before-Phase Triggers:** Run before a phase executes (validation, pre-checks)
**Lifecycle Triggers:** Run on task events (created, completed, failed)

Both use same config structure as gates (agent, input config, output config).
