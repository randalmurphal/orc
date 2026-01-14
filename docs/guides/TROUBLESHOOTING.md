# Troubleshooting Guide

**Purpose**: Diagnose and resolve common orc issues.

---

## Task Stuck on Same Error

**Symptoms**:
- Task status shows `stuck`
- `.stuck.md` file created in task directory
- Same error appears in multiple consecutive transcripts

**Diagnosis**:
```bash
# View stuck analysis
cat .orc/tasks/TASK-XXX/.stuck.md

# Check recent transcripts
orc log TASK-XXX --tail 100

# View error history
orc show TASK-XXX --errors
```

**Solutions**:

| Cause | Fix |
|-------|-----|
| Missing dependency | `go mod tidy` or `npm install`, then `orc run --continue` |
| Wrong file path | Fix path in spec, rewind to spec phase |
| API/external failure | Check network, retry later |
| Misunderstood requirement | Add clarification to task description, rewind |

**To resume after fixing**:
```bash
orc run TASK-XXX --continue
```

**To start completely fresh**:
```bash
orc reset TASK-XXX   # Clear all progress and retry from beginning
orc run TASK-XXX
```

**If you fixed the issue manually or it's no longer relevant**:
```bash
orc resolve TASK-XXX -m "Fixed manually"  # Mark failed task as resolved without re-running
```

---

## Phase Never Completes (Infinite Loop)

**Symptoms**:
- Iteration count keeps increasing
- No `<phase_complete>true</phase_complete>` in output
- Approaching max_iterations limit

**Diagnosis**:
```bash
# Check what Claude is outputting
orc log TASK-XXX -f  # Follow live

# Look for completion signals
grep "phase_complete" .orc/tasks/TASK-XXX/transcripts/*.md
```

**Common Causes**:

1. **Vague completion criteria**
   - Fix: Add objective, testable criteria (tests pass, file exists)
   - Example: Change "implement auth" to "all tests in auth_test.go pass"

2. **Unachievable goal**
   - Fix: Break task into smaller pieces or adjust scope
   - Use `orc skip --phase` if phase is optional

3. **Missing context**
   - Fix: Add more detail to task description or spec

**To stop and preserve state**:
```bash
orc pause TASK-XXX --reason "Investigating infinite loop"
```

---

## Orphaned Tasks (Stuck in "Running")

**Symptoms**:
- Task shows as "running" but no executor process is active
- `orc status` shows task in "ORPHANED" section
- Task was running when machine crashed, session closed, or process was killed

**Diagnosis**:
```bash
# Check task status
orc status

# View execution info
cat .orc/tasks/TASK-XXX/state.yaml | grep -A5 "execution:"
```

**How Orphan Detection Works**:

Orc tracks executor process information in `state.yaml`:
- **PID**: Process ID of the executor
- **Hostname**: Machine running the executor
- **Heartbeat**: Last time executor updated state

A task is considered orphaned when:
1. Status is "running" but no execution info exists (legacy state)
2. Status is "running" but executor PID is no longer alive
3. Status is "running" but heartbeat is stale (>5 minutes)

**Solutions**:

| Method | Command | Notes |
|--------|---------|-------|
| Auto-resume | `orc resume TASK-XXX` | Detects orphan, marks as interrupted, resumes |
| Force resume | `orc resume TASK-XXX --force` | For tasks that appear running but you know are not |
| Reset | `orc reset TASK-XXX --force` | Start completely fresh (clears all progress) |
| Resolve | `orc resolve TASK-XXX -m "reason"` | Mark as resolved if fix was applied manually |
| Check in Web UI | `orc serve` then view Dashboard | Orphaned tasks highlighted with warning |

**The resume command automatically**:
1. Checks if task is orphaned (executor dead or heartbeat stale)
2. Marks the task as interrupted
3. Clears stale execution info
4. Resumes from the last active phase

**Manual Recovery** (if auto-detection fails):
```bash
# Mark task as blocked (so it can be resumed)
# Edit .orc/tasks/TASK-XXX/task.yaml: change status to "blocked"
# Edit .orc/tasks/TASK-XXX/state.yaml: change status to "interrupted", remove execution block
orc resume TASK-XXX
```

---

## Gate Rejection

**Symptoms**:
- Status shows `waiting` at gate
- AI or human rejected phase output

**Diagnosis**:
```bash
# View gate decision
orc show TASK-XXX --gates

# Read rejection rationale
grep -A10 "decision: rejected" .orc/tasks/TASK-XXX/state.yaml
```

**Solutions**:

| Gate Type | Action |
|-----------|--------|
| AI rejection | Review AI's feedback, fix issues, re-run phase |
| Human rejection | Address reviewer feedback, re-run with `orc run --phase` |
| NEEDS_CLARIFICATION | Provide clarification with `orc approve --clarify` |

**To retry phase**:
```bash
orc rewind TASK-XXX --to <phase>
orc run TASK-XXX
```

---

## Task Classification Wrong

**Symptoms**:
- Task feels too light or too heavy
- Missing important phases (no research, no design)
- Too much ceremony for simple fix

**Solutions**:

```bash
# Override classification
orc run TASK-XXX --weight large  # Upgrade

# Or if already running, rewind and reclassify
orc rewind TASK-XXX --to classify
orc run TASK-XXX --weight medium
```

**Classification signals to consider**:
- Number of files: 10+ usually means large/greenfield
- "breaking change", "database migration": +1 weight level
- "typo", "config tweak": probably trivial

---

## Git State Issues

### Dirty Working Tree

**Symptoms**:
```
error: cannot create task - working tree not clean
```

**Solutions**:
```bash
# Stash changes
git stash

# Or commit them
git add -A && git commit -m "WIP"

# Then create task
orc new "My task"
```

### Branch Already Exists

**Symptoms**:
```
error: branch orc/TASK-XXX already exists
```

**Solutions**:
```bash
# If old task, clean up
orc cleanup --all

# Or delete manually
git branch -D orc/TASK-XXX
```

### Worktree Conflict

**Symptoms**:
```
fatal: '.orc/worktrees/TASK-XXX' already exists
```

**Note**: As of TASK-042, orc automatically handles stale worktree registrations. If a worktree directory was deleted without proper cleanup (e.g., via `rm -rf`), orc will automatically prune stale entries and retry when creating a new worktree for a task.

**Manual Solutions** (if auto-recovery fails):
```bash
# Remove stale worktree
git worktree remove .orc/worktrees/TASK-XXX --force
git worktree prune
```

### Stale Worktree Registration

**Symptoms**:
```
fatal: 'path/to/worktree' is already a worktree
```
(Even though the directory doesn't exist)

**Cause**: A worktree directory was deleted without using `git worktree remove`. Git still has a stale registration.

**Solutions**:
- **Automatic**: Orc handles this automatically - just run your task and it will prune stale entries
- **Manual**: `git worktree prune` removes stale registrations

---

## Claude Code Errors

### Claude Not Found

**Symptoms**:
```
error: claude: command not found
```

**How orc finds Claude**:
1. `claude_path` config (if set explicitly)
2. PATH lookup (`which claude`)
3. Common install locations:
   - `~/.local/bin/claude`
   - `~/.claude/local/claude`
   - `/usr/local/bin/claude`
   - `/opt/homebrew/bin/claude`
   - `/snap/bin/claude`

**Solutions**:
```bash
# Check Claude is installed
which claude

# If Claude is installed but not in PATH, add it:
export PATH="$HOME/.local/bin:$PATH"  # Add to ~/.bashrc or ~/.zshrc

# Or set explicit path in config (not recommended - prefer PATH)
orc config claude_path /path/to/claude
```

### Rate Limited

**Symptoms**:
- Errors mentioning "rate limit" or "429"
- Task stalls without progress

**Solutions**:
- Wait and retry: `orc run TASK-XXX --continue`
- Reduce concurrency (run fewer parallel tasks)
- Check API quotas in Anthropic console

### Timeout

**Symptoms**:
```
error: phase timed out after 600s
```
or
```
⏰ Turn timeout after 10m0s - cancelling request
```

**Solutions**:
```bash
# Increase turn timeout (per API call)
ORC_TURN_MAX_TIMEOUT=20m orc run TASK-XXX

# Increase phase timeout (total phase time)
ORC_PHASE_MAX_TIMEOUT=1h orc run TASK-XXX

# Or configure in config.yaml
timeouts:
  turn_max: 20m      # Per API turn
  phase_max: 1h      # Per phase total
```

### Slow API with No Progress

**Symptoms**:
- No output for extended periods
- Warning: "No activity for 2m - API may be slow or stuck"
- Progress dots appearing but no response completing

**Diagnosis**:
The activity tracker monitors Claude API calls and provides visual feedback:
- Progress dots appear every 30s during API waits
- Idle warnings appear after 2m of no streaming activity
- Turn timeouts cancel requests after 10m (configurable)

**Solutions**:

| Issue | Fix |
|-------|-----|
| API consistently slow | Increase `timeouts.turn_max` |
| Want faster feedback | Decrease `timeouts.heartbeat_interval` |
| Too many warnings | Increase `timeouts.idle_timeout` |
| Disable heartbeats | Set `timeouts.heartbeat_interval: 0` |

**Configuration**:
```yaml
# config.yaml
timeouts:
  phase_max: 30m           # Max time per phase
  turn_max: 10m            # Max time per API turn
  idle_warning: 5m         # Warn if no tool calls
  heartbeat_interval: 30s  # Progress dots (0 = disable)
  idle_timeout: 2m         # Warn if no streaming activity
```

**Environment variables**:
- `ORC_TURN_MAX_TIMEOUT` - Override turn timeout
- `ORC_HEARTBEAT_INTERVAL` - Override heartbeat interval
- `ORC_IDLE_TIMEOUT` - Override idle timeout

---

## Checkpoint Issues

### Can't Rewind

**Symptoms**:
```
error: commit abc123 not found
```

**Cause**: Checkpoint commits may have been garbage collected or branch deleted.

**Solutions**:
```bash
# Check what checkpoints exist
orc show TASK-XXX --checkpoints

# Use git reflog as fallback
git reflog | grep "orc/TASK-XXX"
```

### Rewind Lost Work

**Symptoms**:
- Rewound too far and lost desired changes

**Recovery**:
```bash
# Find commit before rewind
git reflog

# Create branch at that point
git branch recovery-TASK-XXX <commit-sha>

# Cherry-pick or merge as needed
```

---

## Performance Issues

### Slow Execution

**Diagnosis**:
```bash
# Check token usage
orc show TASK-XXX

# View iteration durations
grep "Duration:" .orc/tasks/TASK-XXX/transcripts/*.md
```

**Solutions**:
- Use Sonnet for implementation phases (faster, cheaper)
- Break large tasks into smaller weights
- Reduce checkpoint frequency for fast phases

### High Token Usage

**Symptoms**:
- Token counts growing rapidly
- Expensive bills

**Solutions**:
- Check for verbose prompts in custom templates
- Use `max_tokens` limit in config
- Break tasks into smaller scope

---

## PR Creation Issues

### Missing Labels Warning

**Symptoms**:
```
WARN PR labels not found on repository, creating PR without labels
```

**Cause**: The configured `completion.pr.labels` reference labels that don't exist on the GitHub repository.

**Behavior**: Orc gracefully handles this by:
1. Detecting the label error from GitHub CLI
2. Logging a warning
3. Retrying PR creation without labels
4. PR is created successfully without the missing labels

**Solutions**:

| Approach | Action |
|----------|--------|
| Create missing labels | Go to GitHub repo → Issues → Labels → New label |
| Remove from config | Edit `completion.pr.labels` in `.orc/config.yaml` |
| Ignore warning | No action needed - PR creation succeeds without labels |

**Note**: This is informational only. The PR will be created successfully; labels are simply omitted when they don't exist on the repository.

### GitHub CLI Not Authenticated

**Symptoms**:
```
GitHub CLI not authenticated: exit status 1: ...
```

**Cause**: The `gh` CLI is not logged in to GitHub. This happens when:
- Fresh machine setup without `gh auth login`
- Auth token expired
- Wrong account authenticated for this repository

**Error Message Includes Fix**:

When PR creation fails due to auth, orc shows:
```
GitHub CLI not authenticated: ...

  To fix this, run:
    gh auth login

  Then retry with:
    orc resume TASK-XXX
```

**Solutions**:

| Method | Command | Notes |
|--------|---------|-------|
| Interactive login | `gh auth login` | Follow prompts for browser/token auth |
| Token-based | `gh auth login --with-token < token.txt` | For CI/automation |
| Check status | `gh auth status` | Verify which account is authenticated |
| Switch account | `gh auth switch` | If multiple accounts configured |

**After Authenticating**:
```bash
orc resume TASK-XXX    # Continues from where it left off
```

**Note**: Auto-merge enablement also requires authentication. If auto-merge fails with auth errors, orc logs a warning but doesn't fail the task - the PR is created successfully, just without auto-merge enabled.

---

## Web UI Issues

### No Tasks Displayed / "Select Project" Message

**Symptoms**:
- Web UI shows "No project selected" instead of tasks
- Task operations fail with "Please select a project first"

**Cause**: The server can run from any directory, but task operations require an explicit project selection. This is by design to prevent confusion when the server's working directory doesn't match the intended project.

**Solutions**:

| Approach | Action |
|----------|--------|
| Select a project | Click "Select Project" button or use `Cmd+P` |
| Set default project | Run `orc serve` from the project directory, or set via API |

**How It Works**:

The Web UI uses a 3-tier fallback for project selection:
1. **localStorage** - User's last selection persists in browser
2. **Server default** - Global default from `~/.orc/projects.yaml`
3. **First project** - Falls back to first registered project

If no projects are registered, the UI prompts to select a project. Register projects with:
```bash
orc init  # In a project directory
```

---

## Log Locations

| File | Purpose |
|------|---------|
| `.orc/tasks/TASK-XXX/state.yaml` | Current task state |
| `.orc/tasks/TASK-XXX/transcripts/` | All Claude I/O |
| `.orc/tasks/TASK-XXX/.stuck.md` | Stuck analysis (if stuck) |
| `orc.yaml` | Project configuration |

---

## Getting Help

1. **View task details**: `orc show TASK-XXX`
2. **Read transcripts**: `orc log TASK-XXX --phase implement`
3. **Check git state**: `git status`, `git log --oneline orc/TASK-XXX`
4. **Verbose mode**: `orc -vv run TASK-XXX`
