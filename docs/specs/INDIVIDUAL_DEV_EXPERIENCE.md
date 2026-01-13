# Individual Developer Experience Specification

> The individual developer experience is SACRED. Team features must NEVER degrade it.

## Core Guarantees

### 1. Zero-Config Solo Experience

A developer should be able to use orc with ZERO additional configuration:

```bash
cd my-project
orc init              # <500ms, no prompts
orc new "fix login"   # Instant task creation
orc run TASK-001      # Executes with sensible defaults
```

**Guarantees:**
- No server required
- No auth required
- No team setup required
- No organization required
- SQLite by default (embedded)
- All prompts work out of box

### 2. Individual Control Over AI Usage

**PRINCIPLE:** A team admin can NEVER force a developer to use a specific model, iteration limit, or Claude configuration.

| Setting | Can Team Set Default? | Can User Override? | Override Location |
|---------|----------------------|-------------------|-------------------|
| Model | Yes | **Always** | `~/.orc/config.yaml` |
| Max iterations | Yes | **Always** | `~/.orc/config.yaml` |
| Timeout | Yes | **Always** | `~/.orc/config.yaml` |
| Profile (auto/safe/strict) | Yes | **Always** | `~/.orc/config.yaml` |
| Gate types | Yes | **Always** | `~/.orc/config.yaml` |
| Token pool | No | N/A | Personal only |
| Claude path | No | N/A | Personal only |

**Why?**
- Developers pay for their own API usage (BYOK model)
- Different tasks may need different models
- Developers know their task complexity best
- Forcing settings creates friction and resentment

### 3. Personal OAuth Tokens

OAuth tokens are NEVER shared:

```
~/.orc/token-pool/
├── pool.yaml          # Personal accounts only
└── state.yaml         # Personal usage state
```

**Team token pool** (Tier 3) is OPTIONAL and OPT-IN:

```yaml
# ~/.orc/config.yaml
pool:
  use_team_pool: false    # Default: false
  fallback_to_personal: true
```

### 4. Personal Prompt Overrides

Developers can ALWAYS override team prompts:

```
Resolution order (highest priority first):
1. ~/.orc/prompts/{phase}.md          # Personal global
2. .orc/local/prompts/{phase}.md      # Personal project (gitignored)
3. .orc/shared/prompts/{phase}.md     # Team shared (git-tracked)
4. templates/prompts/{phase}.md       # Builtin
```

**Use cases:**
- Developer prefers different coding style instructions
- Developer wants more/less verbose output
- Developer working on specific tech stack needs tailored prompts

### 5. Personal UI Preferences

```yaml
# ~/.orc/ui.yaml
theme: dark              # User's choice
sidebar_pinned: true     # User's choice
shortcuts:               # User's custom shortcuts
  custom:
    "ctrl+shift+r": "run-selected"
notification_mode: focus # focus | balanced | everything
```

**Never synced, never overridden by team.**

---

## Individual Settings Schema

### User Config (`~/.orc/config.yaml`)

```yaml
# Personal preferences - ALWAYS take priority
version: 1

# AI execution settings
model: claude-sonnet-4-20250514     # User's default model
max_iterations: 25                   # User's iteration preference
timeout: 15m                         # User's timeout preference
profile: safe                        # User's automation preference

# Gate preferences
gates:
  prefer_ai_review: true             # User wants AI to review their code
  auto_approve_trivial: true         # Auto-approve trivial tasks

# Cost preferences
cost:
  daily_warning: 20.00               # Personal daily warning threshold
  show_running_cost: true            # Show cost in real-time

# Notification preferences
notifications:
  mode: focus                        # focus | balanced | everything
  sound: false                       # No notification sounds
  desktop: false                     # No desktop notifications

# Token pool
pool:
  enabled: true
  use_team_pool: false               # Don't use shared team tokens
  accounts:                          # Personal accounts
    - personal_max
    - personal_pro

# Claude CLI settings (claude_path auto-detects from PATH and common locations)
# claude_path: /custom/path/to/claude   # Only needed if not in PATH
dangerously_skip_permissions: true   # User's choice
```

### Personal Prompts (`~/.orc/prompts/`)

```markdown
# ~/.orc/prompts/implement.md
---
extends: builtin                     # Extend, don't replace
prepend: |
  PERSONAL PREFERENCES:
  - Use explicit type annotations
  - Prefer composition over inheritance
  - Write tests alongside code, not after
---
```

### Personal Skills (`~/.orc/skills/`)

```
~/.orc/skills/
├── my-coding-style/
│   └── SKILL.md
└── my-company-patterns/
    └── SKILL.md
```

---

## Team Features: Opt-In Only

### What "Opt-In" Means

Team features are:
1. **Disabled by default** - Solo users never see them
2. **Explicitly activated** - Requires user action
3. **Reversible** - Can always go back to solo mode
4. **Non-blocking** - Never prevent local work

### Team Feature Activation

```yaml
# .orc/config.yaml (project level)
team:
  enabled: false                     # Default: disabled

# To enable:
team:
  enabled: true
  server: https://orc.company.com    # Optional: team server
  org_id: acme-corp                  # Optional: organization
```

### What Changes When Team Features Are Enabled

| Feature | Solo Mode | Team Mode (Opt-In) |
|---------|-----------|-------------------|
| Task IDs | TASK-001 | TASK-rm-001 (prefixed) |
| Prompts | builtin only | shared → builtin |
| Skills | personal only | shared → personal |
| Config | project only | shared → project |
| Visibility | local only | optional server sync |
| Presence | N/A | optional WebSocket |

### What NEVER Changes

| Feature | Behavior |
|---------|----------|
| Execution | Always local |
| OAuth tokens | Always personal |
| Model choice | Always user-controlled |
| Iteration limits | Always user-controlled |
| Cost tracking | Always personal visibility |
| Prompt overrides | Always possible |

---

## Offline Mode

### Guarantee: Full Functionality Offline

```bash
# No network, no problem
orc new "offline task"
orc run TASK-001
orc status
orc log TASK-001
```

**Works offline:**
- Task creation
- Task execution
- All CLI commands
- Web UI (local)
- Prompt resolution (cached)
- Skill resolution (cached)

**Requires network:**
- Team server sync (graceful degradation)
- Shared resource updates (uses cached)
- Presence updates (ignored)

### Graceful Degradation

```go
func syncToTeamServer(task *Task) error {
    if config.Team.Mode == "local" {
        return nil  // Local mode, nothing to sync
    }

    err := server.SyncTask(task)
    if err != nil {
        logger.Warn("team sync failed, will retry", "error", err)
        queueForRetry(task)  // Background retry
        return nil           // Don't block user
    }
    return nil
}
```

---

## Cost Transparency

### Real-Time Cost Display

```
$ orc run TASK-001
Starting task: Add user authentication
Phase: implement
  Iteration 1: 1,234 tokens ($0.02)
  Iteration 2: 2,456 tokens ($0.04)
  ...
Phase complete: $0.15

Phase: test
  Iteration 1: 890 tokens ($0.01)
  ...

Task completed
Total: 8,234 tokens ($0.18)
```

### Cost Controls

```yaml
# ~/.orc/config.yaml
cost:
  # Warnings (non-blocking)
  warn_per_task: 1.00        # Warn if task exceeds $1
  warn_per_phase: 0.50       # Warn if phase exceeds $0.50
  warn_daily: 10.00          # Warn at $10/day

  # Limits (blocking - user can override)
  limit_per_task: 5.00       # Pause task at $5
  limit_daily: 50.00         # Pause all tasks at $50/day

  # Display
  show_running_cost: true    # Show cost during execution
  show_token_count: true     # Show token counts
```

### Cost Overrides

When a limit is hit:

```
⚠️  Task TASK-001 has exceeded $5.00 cost limit ($5.23)

Options:
  [1] Continue (increase limit to $10)
  [2] Pause task (resume later)
  [3] Cancel task

Your choice: _
```

**Users can ALWAYS override cost limits for their own tasks.**

---

## Personal Workspace

### Local Task Notes

```
.orc/local/notes/
├── TASK-001.md          # Personal notes for task
└── TASK-002.md
```

These are:
- Gitignored (never shared)
- Searchable via `orc search --notes`
- Visible in task detail view

### Personal Task Templates

```
~/.orc/templates/
├── my-bugfix.yaml       # Personal template
└── my-feature.yaml
```

```bash
orc new "fix login" --template my-bugfix
```

### Personal Shortcuts

```yaml
# ~/.orc/ui.yaml
shortcuts:
  global:
    "ctrl+shift+n": "new-task"
    "ctrl+shift+r": "run-selected"
  custom:
    "ctrl+shift+1": "orc run --phase implement"
    "ctrl+shift+2": "orc run --phase test"
```

---

## Multi-Machine Sync (Personal)

### Problem

Developer uses orc on laptop AND desktop.

### Solution: Personal Sync (Not Team Sync)

```yaml
# ~/.orc/config.yaml
sync:
  enabled: true
  provider: git                      # or: dropbox, syncthing
  path: ~/sync/orc-personal/
  items:
    - prompts
    - skills
    - templates
    - ui.yaml
    - config.yaml
```

This syncs PERSONAL preferences across machines, independent of any team.

---

## Privacy

### What's Never Shared (Even with Team)

| Data | Privacy Level |
|------|---------------|
| OAuth tokens | Personal only, never leaves machine |
| API keys | Personal only, never logged |
| Local task notes | Personal only |
| UI preferences | Personal only |
| Notification settings | Personal only |
| Cost details | Personal (aggregates may be shared) |
| Personal prompt overrides | Personal only |

### What Can Be Shared (With Team)

| Data | Sharing Level | User Control |
|------|---------------|--------------|
| Task existence | Team visible | Can mark private |
| Task status | Team visible | Can hide |
| Task title | Team visible | Can hide |
| Token usage (aggregate) | Team visible | Can opt-out |

### Private Tasks

```bash
orc new "secret experiment" --private
```

Private tasks:
- Not synced to team server
- Not visible to team members
- Local only
- Normal ID (TASK-xxx) without prefix

---

## Migration: Solo → Team

### Step 1: Add Shared Directory

```bash
mkdir -p .orc/shared
git add .orc/shared
```

### Step 2: Move Shareable Resources

```bash
# Copy prompts you want to share
cp .orc/prompts/*.md .orc/shared/prompts/

# Keep personal overrides
mkdir -p .orc/local/prompts
mv .orc/prompts/implement.md .orc/local/prompts/  # Personal version
```

### Step 3: Enable Team Mode (Optional)

```yaml
# .orc/config.yaml
team:
  enabled: true
```

### Step 4: Personal Settings Remain

All settings in `~/.orc/` continue to work unchanged.

---

## CLI Behavior

### Commands That Work Identically (Solo & Team)

| Command | Behavior |
|---------|----------|
| `orc init` | Creates .orc/, registers project |
| `orc new` | Creates task locally |
| `orc run` | Executes locally with user's Claude |
| `orc pause` | Pauses local execution |
| `orc resume` | Resumes local execution |
| `orc status` | Shows local status |
| `orc log` | Shows local transcripts |
| `orc config` | Shows effective config |

### Commands With Team Extensions

| Command | Solo | Team Addition |
|---------|------|---------------|
| `orc status` | Local tasks | + Team activity (if enabled) |
| `orc list` | Local tasks | + `--team` flag for team tasks |
| `orc sync` | N/A | Sync with team server |

### New Team-Only Commands

| Command | Purpose |
|---------|---------|
| `orc team status` | Show team activity |
| `orc team sync` | Force sync to server |
| `orc team members` | List team members |

These commands are NO-OP in solo mode:

```bash
$ orc team status
Team mode not enabled. Use `orc init --team` to enable.
```

---

## Testing Individual Experience

### Acceptance Criteria

1. **Solo Init Speed**
   ```bash
   time orc init  # Must complete in <500ms
   ```

2. **No Network for Basic Operations**
   ```bash
   # Disconnect network
   orc new "test task"       # Must succeed
   orc run TASK-001          # Must succeed (with Claude access)
   orc status                # Must succeed
   ```

3. **Personal Overrides Always Win**
   ```bash
   # Set up team prompt
   echo "team prompt" > .orc/shared/prompts/implement.md
   # Set up personal override
   echo "my prompt" > ~/.orc/prompts/implement.md

   orc run TASK-001
   # Must use "my prompt", not "team prompt"
   ```

4. **Model Choice Always Respected**
   ```yaml
   # ~/.orc/config.yaml
   model: claude-haiku-3-20240307
   ```
   ```bash
   orc run TASK-001
   # Must use haiku, regardless of team default
   ```

5. **Cost Visibility Always Present**
   ```bash
   orc run TASK-001
   # Must show token usage and cost estimates

   orc cost
   # Must show personal cost breakdown
   ```

---

## Error Messages

### Good: Respects Individual Context

```
Error: Rate limit exceeded on your personal Claude account.

Options:
  - Wait 60 seconds and retry
  - Switch to another account: orc pool switch
  - Use team token pool: orc config set pool.use_team_pool true
```

### Bad: Assumes Team Context

```
Error: Rate limit exceeded. Contact your team administrator.
```

### Good: Explains Override

```
Using personal prompt override for 'implement' phase.
  Source: ~/.orc/prompts/implement.md
  Team default: .orc/shared/prompts/implement.md

To use team default, remove your override or run with --no-personal-overrides
```

---

## Summary

The individual developer experience is protected by:

1. **Technical guarantees** - Personal settings always override
2. **Opt-in team features** - Never forced
3. **Offline capability** - Never dependent on network
4. **Cost transparency** - Always visible
5. **Privacy defaults** - Personal data stays personal
6. **Clear escalation** - Team features are additive, not replacing
