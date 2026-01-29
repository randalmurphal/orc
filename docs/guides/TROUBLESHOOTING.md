# Troubleshooting Guide

**Purpose**: Diagnose and resolve common orc issues.

---

## Task is Blocked by Dependencies

**Symptoms**:
- `orc run` shows warning about incomplete blockers
- API returns 409 Conflict with `task_blocked` error
- Web UI shows blocking warning when attempting to run task

**Example CLI output**:
```
⚠️  This task is blocked by incomplete tasks:
    - TASK-060: Add initiative_id field... (planned)
    - TASK-061: Add Initiatives section... (running)

Run anyway? [y/N]:
```

**Diagnosis**:
```bash
# View task dependencies
orc show TASK-XXX

# Check which tasks are blocking
orc show TASK-XXX | grep -A5 "blocked_by:"

# See dependency graph via API
curl http://localhost:8080/api/tasks/TASK-XXX/dependencies
```

**Solutions**:

| Approach | Command | When to Use |
|----------|---------|-------------|
| Complete blockers first | `orc run TASK-060` | Best practice - respects dependency order |
| Force run anyway | `orc run TASK-XXX --force` | When you know the dependency is soft |
| Remove blocker | `orc edit TASK-XXX --remove-blocker TASK-060` | If dependency was added by mistake |

**Understanding Dependencies**:

- **blocked_by**: Hard dependencies - tasks that should complete first
- **related_to**: Soft relationships - informational only, doesn't block execution

**For API users**: Add `?force=true` query parameter to bypass the blocking check:
```bash
curl -X POST http://localhost:8080/api/tasks/TASK-XXX/run?force=true
```

**When to use --force**:
- The blocking task is being worked on in parallel and doesn't affect this task
- The dependency was added for tracking purposes but isn't truly required
- You're prototyping and want to test independently

**When NOT to use --force**:
- The blocking task produces output this task depends on
- The tasks share resources and concurrent execution could cause conflicts
- You're unsure why the dependency exists

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
| Missing dependency | `go mod tidy` or `bun install`, then `orc run --continue` |
| Wrong file path | Fix path in spec, rewind to spec phase |
| API/external failure | Check network, retry later |
| Misunderstood requirement | Add clarification to task description, rewind |

**To resume a failed task** (recommended):
```bash
orc resume TASK-XXX   # Resume from last incomplete phase
```

The `resume` command works with failed tasks, allowing you to retry from where the task left off without losing progress. Fix the underlying issue first (missing dependency, config error, etc.), then resume.

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
- No `{"status": "complete"}` JSON in output
- Approaching max_iterations limit

**Diagnosis**:
```bash
# Check what Claude is outputting
orc log TASK-XXX -f  # Follow live

# Look for completion signals (JSON format)
grep '"status"' .orc/tasks/TASK-XXX/transcripts/*.md
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
orc show TASK-XXX --state
```

**How Orphan Detection Works**:

Orc tracks executor process information in the database:
- **PID**: Process ID of the executor
- **Hostname**: Machine running the executor
- **Heartbeat**: Last time executor updated state (updated every 2 minutes during execution)

A task is considered orphaned when:
1. Status is "running" but no execution info exists (legacy state)
2. Status is "running" but executor PID is no longer alive

**Note**: A live PID always indicates a healthy task, regardless of heartbeat age. Heartbeat staleness is only used as additional context when the PID check indicates the executor is dead. This prevents false positives during long-running phases (which can take hours).

**Solutions**:

| Method | Command | Notes |
|--------|---------|-------|
| Auto-resume | `orc resume TASK-XXX` | Detects orphan, marks as interrupted, resumes |
| Force resume | `orc resume TASK-XXX --force` | For tasks that appear running but you know are not |
| Reset | `orc reset TASK-XXX --force` | Start completely fresh (clears all progress) |
| Resolve | `orc resolve TASK-XXX --force` | Mark as resolved if PR was already merged |
| Check in Web UI | `orc serve` then view Dashboard | Orphaned tasks highlighted with warning |

**The resume command automatically**:
1. Checks if task is orphaned (executor dead or heartbeat stale)
2. Marks the task as interrupted
3. Clears stale execution info
4. Resumes from the last active phase

**Manual Recovery** (if auto-detection fails):
```bash
# Force reset the task to allow resuming
orc reset TASK-XXX --force
orc resume TASK-XXX
```

---

## Stuck Running Tasks with Merged PRs

**Symptoms**:
- Task shows as "running" in `orc status`
- PR was already created and merged in GitHub
- Executor crashed or lost connection after PR merge but before marking task complete
- `orc resolve TASK-XXX` fails with "task is running, not failed"

**Diagnosis**:
```bash
# Check task status
orc show TASK-XXX

# Check if PR was merged (look for PR info in output)
orc show TASK-XXX | grep -A5 "PR:"

# Or check GitHub directly
gh pr view <PR-NUMBER> --json state,merged
```

**Cause**: The executor creates a PR, waits for merge (or auto-merges), then marks the task complete. If the executor dies between merge and status update, the task is stuck in "running" with a merged PR.

**Solution**:

Use `orc resolve --force` to mark the task as completed:

```bash
# Will auto-detect merged PR and report it
orc resolve TASK-XXX --force
# Output: PR merged (PR #123)
# Output: Task TASK-XXX marked as resolved (was: running)

# With a message explaining the resolution
orc resolve TASK-XXX --force -m "Executor crashed after PR merge"
```

**What happens**:
1. Command checks the task's PR field for merge status
2. If PR is merged, reports: `PR merged (PR #123)`
3. If PR is not merged or missing, shows a warning
4. Task is marked as completed with metadata:
   - `force_resolved: true`
   - `original_status: running`
   - `pr_was_merged: true` (if applicable)

**When NOT to use --force**:
- If the PR was not merged (use `orc resume` instead to continue execution)
- If you're unsure whether the work was completed (check PR status first)
- If you want to retry the task from scratch (use `orc reset` instead)

**Alternative approaches**:

| Scenario | Command |
|----------|---------|
| PR merged, mark complete | `orc resolve TASK-XXX --force` |
| PR not merged, continue | `orc resume TASK-XXX` |
| Start over completely | `orc reset TASK-XXX --force` |

---

## Failed Tasks

**Symptoms**:
- Task status shows `failed`
- Phase execution stopped due to an error
- Task cannot proceed without intervention

**Diagnosis**:
```bash
# Check task status
orc show TASK-XXX

# View what went wrong
orc log TASK-XXX --phase <failing-phase>
```

**Solutions**:

| Method | Command | When to Use |
|--------|---------|-------------|
| Resume | `orc resume TASK-XXX` | After fixing the underlying issue |
| Reset | `orc reset TASK-XXX` | To start fresh from the beginning |
| Resolve | `orc resolve TASK-XXX -m "reason"` | If fixed manually outside orc |

**Resuming Failed Tasks**:

The `resume` command supports failed tasks directly—no need to reset or manually edit files:

```bash
# Fix the underlying issue (install dependency, fix config, etc.)
# Then resume from where it left off
orc resume TASK-XXX
```

This preserves completed phases and continues from the last incomplete phase. Useful when:
- A dependency was missing and is now installed
- An external service was down and is now available
- A configuration issue was fixed
- A transient error occurred

**Note**: If the same error recurs, consider using `orc reset` to start fresh or `orc resolve` to mark it as handled.

### Worktree State Issues During Resolve

**Symptoms**:
- `orc resolve` shows warnings about dirty worktree or in-progress git operations
- Worktree has uncommitted changes from interrupted task
- Worktree has rebase-in-progress or merge-in-progress state

**Example output**:
```
📁 Worktree: .orc/worktrees/orc-TASK-001
   ⚠️  Rebase in progress - worktree is in an incomplete state
   ⚠️  3 uncommitted file(s)

⚠️  Resolve task TASK-001 as completed?
```

**Solutions**:

| Flag | Behavior | When to Use |
|------|----------|-------------|
| `--cleanup` | Abort in-progress git ops, discard uncommitted changes | Worktree state is garbage from a crash |
| `--force` | Skip checks, resolve anyway (worktree unchanged) | You want to preserve worktree state |
| (default) | Show warnings, prompt for confirmation | Review the state before deciding |

**Using --cleanup**:
```bash
orc resolve TASK-XXX --cleanup   # Clean worktree state, then resolve
```

This aborts any in-progress rebase/merge and discards uncommitted changes, leaving the worktree clean. The worktree itself is preserved (not deleted).

**Using --force**:
```bash
orc resolve TASK-XXX --force     # Skip checks, keep worktree as-is
```

This resolves the task without touching the worktree at all. Useful if you want to preserve the worktree state for manual inspection.

**Worktree Metadata**: When resolving, orc records worktree state in task metadata:
- `worktree_was_dirty: true` - Had uncommitted changes
- `worktree_had_conflicts: true` - Had unresolved merge conflicts
- `worktree_had_incomplete_operation: true` - Had rebase/merge in progress

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
orc show TASK-XXX --state | grep -A10 "decision: rejected"
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

## Stray spec.md Files

### spec.md at Repo Root or in Worktrees

**Symptoms**:
- `spec.md` file appears at repository root
- `artifacts/spec.md` or similar paths contain spec content
- Merge conflicts in worktrees involving spec.md files
- `git status` shows untracked spec.md

**Cause**: In older versions of orc, Claude Code sometimes wrote spec.md files directly to the filesystem. Specs are now captured via JSON output and stored in the database.

**Current design**: Specs are output in JSON `artifact` field using `--json-schema` constrained output, extracted by orc, and saved to database via `SaveSpecToDatabase()`.

**Solutions**:

| Situation | Action |
|-----------|--------|
| Untracked spec.md | `rm spec.md` (safe to delete, spec is in database) |
| Committed spec.md | Remove and commit: `git rm spec.md && git commit -m "Remove stray spec.md"` |
| In .gitignore | Already handled - file is ignored by git |
| Merge conflict | Delete the file, resolve conflict, spec content is in database |

**Verification**:

Check that the spec is in the database:
```bash
orc show TASK-XXX --spec  # Displays spec from database
```

If the spec displays correctly, any filesystem spec.md is redundant and can be deleted.

**Prevention**:

- `spec.md` and `artifacts/spec.md` are in `.gitignore`
- Spec prompts use JSON schema for structured output

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

## Orphaned Processes / System Freezes

**Symptoms**:
- System becomes sluggish after running multiple orc tasks
- `WARN orphaned processes detected` appears in logs
- `WARN memory growth exceeded threshold` appears in logs
- Many `chromium`, `playwright`, or browser processes in `ps aux`

**Cause**: Claude CLI sessions spawn MCP servers (Playwright, browsers) that don't get cleaned up when sessions end. The parent process dies but the child processes survive, becoming orphaned.

**Diagnosis**:
```bash
# Check for orphaned browser/MCP processes
ps aux | grep -E 'playwright|chromium|chrome|firefox|webkit'

# Check orc logs for orphan warnings
orc log TASK-XXX | grep -i orphan

# Check memory growth warnings
orc log TASK-XXX | grep -i "memory growth"
```

**Solutions**:

| Approach | Command | When to Use |
|----------|---------|-------------|
| Manual cleanup | `pkill -f playwright && pkill -f chromium` | Immediate relief when system is slow |
| Disable UI testing | `orc update TASK-XXX --requires-ui-testing=false` | If not actually doing UI tests |
| Monitor logs | Watch for orphan warnings after tasks | Identify which tasks cause issues |

**Understanding the Logs**:

```
INFO resource snapshot taken (before) processes=145 memory_mb=2456.3
INFO resource snapshot taken (after) processes=148 memory_mb=2892.1
WARN orphaned processes detected count=3 processes="chromium (PID=12345) [MCP]..."
WARN memory growth exceeded threshold delta_mb=435.8 threshold_mb=100
```

- **before/after snapshots**: Shows process count and total memory at task start/end
- **orphaned processes**: New processes that survived task completion (reparented to init)
- **[MCP] tag**: Indicates MCP-related process (Playwright, browser)
- **memory growth**: Delta between before/after (warning if > threshold)

**Understanding the Tags**:

```
WARN orphaned processes detected count=2 processes="chromium (PID=12345) [MCP], node (PID=12346) [orc]..."
```

- **[MCP]**: Browser/Playwright processes (chromium, chrome, firefox, webkit, puppeteer, selenium)
- **[orc]**: Other orc-related processes (claude, node, npx, bun, npm, mcp-server)

System processes (systemd, snapper, etc.) are filtered out by default and don't appear in orphan warnings.

**Configuration**:

```yaml
# config.yaml
diagnostics:
  resource_tracking:
    enabled: true                 # Enable tracking (default: true)
    memory_threshold_mb: 500      # Warn threshold (default: 500)
    filter_system_processes: true # Filter out system processes (default: true)
```

| Option | Default | Purpose |
|--------|---------|---------|
| `enabled` | `true` | Enable/disable resource tracking entirely |
| `memory_threshold_mb` | `500` | Warn if memory grows by more than this |
| `filter_system_processes` | `true` | Only flag orc-related processes as orphans |

**Filter System Processes (New in TASK-279)**:

When `filter_system_processes: true` (default), only processes matching orc-related patterns are flagged as potential orphans:
- Browser automation: playwright, chromium, chrome, firefox, webkit, puppeteer, selenium
- Claude Code and Node.js: claude, node, npx, bun, npm
- MCP servers: mcp-server, mcp

System processes that happen to start during task execution (like `systemd-timedated`, `snapper`, `updatedb`, etc.) are ignored. This eliminates false positives where unrelated system activity was incorrectly flagged as orphaned.

Set to `false` to use the original behavior where all new orphaned processes are flagged (prone to false positives on active systems).

**Disabling Resource Tracking**:

If tracking itself causes issues (unlikely), disable it:
```yaml
diagnostics:
  resource_tracking:
    enabled: false
```

**Process Group Cleanup (Orchestrator Workers)**:

For orchestrator workers (`orc run --orchestrate`), process group handling is implemented:
- Workers create commands with `Setpgid: true` to put child processes in their own process group
- `Worker.Stop()` kills the entire process group via `syscall.Kill(-pid, SIGKILL)`
- This ensures MCP servers spawned by Claude are properly terminated

**Limitations**:
- Only applies to orchestrator workers (parallel execution mode)
- Single-task CLI runs (`orc run TASK-XXX`) still use llmkit's default behavior
- Windows does not support POSIX process groups (stub implementation)

**Note**: For single-task execution, the resource tracker still provides visibility. A future llmkit update may add process group support for all Claude invocations.

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

### Missing Labels (Silent Fallback)

**Behavior**: When configured `completion.pr.labels` reference labels that don't exist on the repository, orc gracefully handles this by:
1. Detecting the label error from the hosting provider API
2. Logging at DEBUG level (silent in normal operation)
3. Retrying PR creation without labels
4. PR is created successfully without the missing labels

**To see this in logs**, use verbose mode: `orc -vv run TASK-XXX`

**Solutions** (if you want labels to appear on PRs):

| Approach | Action |
|----------|--------|
| Create missing labels | Go to your repository settings to create the label (GitHub: Issues → Labels → New label; GitLab: Project → Labels) |
| Remove from config | Edit `completion.pr.labels` in `.orc/config.yaml` |
| Leave as-is | No action needed - PR creation succeeds without labels |

**Note**: This is normal behavior for repos without pre-configured labels. The PR will be created successfully; labels are simply omitted.

### Hosting Provider Not Authenticated

**Symptoms**:
```
hosting provider not configured: ...
failed to create PR: authentication failed
```

**Cause**: The hosting provider (GitHub or GitLab) is not configured. This happens when:
- No `ORC_GITHUB_TOKEN` or `ORC_GITLAB_TOKEN` environment variable is set
- Token is expired or revoked
- Token doesn't have required scopes (repo for GitHub, api for GitLab)

**Solutions**:

| Method | Provider | Command / Action |
|--------|----------|-----------------|
| GitHub token | GitHub | Set `ORC_GITHUB_TOKEN` env var with a PAT (repo scope) |
| GitLab token | GitLab | Set `ORC_GITLAB_TOKEN` env var with a PAT (api scope) |
| GitHub App | GitHub | Configure app installation in `.orc/config.yaml` |
| Check status | Both | `orc status` shows hosting provider status |

**After Configuring**:
```bash
orc resume TASK-XXX    # Continues from where it left off
```

**Note**: Auto-merge requires provider support:
- **GitHub**: Requires GraphQL API (not currently supported by orc - returns ErrAutoMergeNotSupported)
- **GitLab**: Uses MergeWhenPipelineSucceeds API (fully supported)

In both cases, the PR is created successfully; only auto-merge may be skipped.

---

## PR Merge Failures (Race Condition)

### Task Blocked After Merge Failure

**Symptoms**:
- Task shows status `blocked` with `blocked_reason=merge_failed`
- Error message mentions "Base branch was modified"
- Multiple tasks ran in parallel and one merged first

**Example output**:
```
⚠️  Task TASK-042 blocked: merge failed
   PR was created but merge failed after 3 retries.

   The most likely cause is another PR merged first, modifying the
   target branch. This can happen when running parallel tasks.

   To resolve:
     orc resume TASK-042
```

**Cause**: When parallel tasks both complete and attempt to merge:
1. TASK-A and TASK-B both create PRs targeting `main`
2. TASK-A's PR merges first, advancing `main`
3. TASK-B's merge attempt gets HTTP 405 "Base branch was modified"
4. Orc automatically retries with rebase (up to 3 times)
5. If retries exhausted or rebase conflicts, task is blocked

**What Orc Does Automatically**:

| Step | Action |
|------|--------|
| 1. Detect 405 | Recognize "Base branch was modified" as retryable |
| 2. Backoff | Wait with exponential backoff (2s, 4s, 8s) |
| 3. Rebase | Fetch and rebase branch onto latest target |
| 4. Push | Force-push rebased branch with `--force-with-lease` |
| 5. Retry | Attempt merge again (up to 3 total attempts) |
| 6. Block | If all retries fail, block task for manual resolution |

**Solutions**:

| Scenario | Command | Notes |
|----------|---------|-------|
| Retries failed (conflicts) | Resolve conflicts manually, then `orc resume TASK-XXX` | Most common |
| Transient failure | `orc resume TASK-XXX` | Retry may succeed |
| Give up | `orc resolve TASK-XXX --force` | If PR is no longer needed |

**Manual Resolution Steps**:
```bash
# Navigate to worktree
cd .orc/worktrees/orc-TASK-042

# Fetch and rebase onto target
git fetch origin
git rebase origin/main

# Resolve any conflicts, then:
git add <resolved-files>
git rebase --continue

# Force push the rebased branch
git push --force-with-lease origin orc/TASK-042

# Resume the task (will retry merge)
orc resume TASK-042
```

**Non-Retryable Merge Errors**:

Some merge errors indicate permanent failures that won't be fixed by retry:

| Error Code | Meaning | Action |
|------------|---------|--------|
| HTTP 405 + "Base branch was modified" | Retryable | Automatic retry with rebase |
| HTTP 422 | Validation failed (conflicts, required checks) | Manual resolution required |
| Other errors | Various failures | Check error message for details |

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
| Select a project | Click "Select Project" button or use `Shift+Alt+P` |
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

## Parallel Task Conflicts

### Merge Conflicts from Stale Worktree

**Symptoms**:
- Task fails during completion sync with merge conflicts
- Error mentions files that another task recently modified
- Multiple tasks ran in parallel and modified similar files

**Cause**: When tasks run in parallel:
1. TASK-A and TASK-B both start worktrees from same `main` commit
2. TASK-A completes and merges first → `main` moves forward
3. TASK-B's worktree is now stale, unaware of TASK-A's changes
4. When TASK-B tries to sync at completion, conflicts occur

**Prevention**:

The `sync_on_start` setting (enabled by default) prevents this by syncing the task branch before execution begins:

```yaml
# .orc/config.yaml (default: true)
completion:
  sync:
    sync_on_start: true
```

With this enabled, TASK-B would rebase onto latest `main` (including TASK-A's changes) before its implement phase runs. The AI then sees the updated code and can incorporate it.

**If You See This Error**:

```
sync conflict: task branch has 3 files in conflict with target
  Conflicting files: [CLAUDE.md src/api/handler.go ...]
  Resolution options:
    1. Run with sync_on_start: false and resolve conflicts during finalize
    2. Manually rebase the task branch and retry
    3. Set completion.sync.fail_on_conflict: false to proceed anyway
```

**Solutions**:

| Approach | Command | When to Use |
|----------|---------|-------------|
| Manual rebase | `cd worktree && git rebase origin/main` | Resolve conflicts yourself |
| Let finalize handle it | Resume with `fail_on_conflict: false` | AI-assisted resolution |
| Force without sync | `ORC_SYNC_ON_START=false orc resume TASK-XXX` | Intentional isolation |

**After resolving**:
```bash
orc resume TASK-XXX
```

### Task Blocked After Sync Conflict

**Symptoms**:

When a task is blocked by sync conflicts, orc now provides detailed guidance:

```
⚠️  Task TASK-042 blocked: sync conflict
   All phases completed, but sync with target branch failed.

   Worktree: .orc/worktrees/orc-TASK-042
   Conflicted files:
     - internal/api/handler.go
     - CLAUDE.md

   To resolve manually:
   ────────────────────────────────────────────
   cd .orc/worktrees/orc-TASK-042
   git fetch origin
   git rebase origin/main

   # For each conflicted file:
   #   1. Edit the file to resolve conflict markers
   #   2. git add <file>

   git rebase --continue
   ────────────────────────────────────────────

   Verify resolution:
     git diff --name-only --diff-filter=U  # Should show no files

   Then resume:
     orc resume TASK-042

   Total tokens: 45,231
   Total time: 12m34s
```

**Note**: The commands shown are contextual—if your project uses merge strategy instead of rebase, the instructions will show `git merge` commands instead.

**Cause**: All task phases completed successfully, but the final sync with the target branch failed due to merge conflicts. This happens when:
- Another task merged to the target branch while this task was running
- Manual commits were pushed to the target branch
- The worktree's branch diverged from the remote

**Why This Message (Not "Completed!")**:

Previously, this scenario would display a celebration message ("completed!") which was misleading since the task couldn't actually be finalized. Now orc correctly shows that the task is blocked and needs attention, with actionable copy-paste commands.

**Solutions**:

| Approach | Command | When to Use |
|----------|---------|-------------|
| Resolve manually | Follow the displayed instructions | Standard approach |
| Force resume | `orc resume TASK-XXX --force` | Skip conflict check |

**Viewing Blocked Tasks with `orc status`**:

The `orc status` command now shows additional detail for tasks blocked by sync conflicts:

```
⚠️  ATTENTION NEEDED

  TASK-042  Add user authentication  (sync conflict)
      Worktree: .orc/worktrees/orc-TASK-042
      → orc resume TASK-042 (after resolving conflicts)
```

This makes it easy to identify which worktree needs attention and provides the exact resume command.

**Task State**: The task remains in `running` status with `sync_conflict` state. All completed phases are preserved - only the sync/finalize step needs to be retried.

---

### Disabling Sync on Start

If you want task isolation (to work on an older branch state without other changes):

```bash
# One-time via environment
ORC_SYNC_ON_START=false orc run TASK-XXX

# Permanent via config
orc config completion.sync.sync_on_start false
```

**Warning**: Disabling sync increases the chance of conflicts at completion time.

### Local Repos Without Remotes

**Behavior**: For git repositories without a remote (e.g., `git init` only, E2E test sandboxes), orc automatically skips sync operations:

- `syncOnTaskStart` and `syncWithTarget` check `git.HasRemote("origin")` first
- If no remote exists, sync is silently skipped (DEBUG log, not WARN)
- No fetch, rebase, or push operations are attempted

This is by design for:
- **E2E test sandboxes**: Created in `/tmp` with no push target
- **Local-only projects**: Git for version control without remote collaboration
- **Offline development**: Working without network connectivity

No configuration needed - orc detects the absence of remotes automatically.

---

## CLAUDE.md Merge Conflicts

### Auto-Resolved Successfully

**Symptoms**:
```
INFO CLAUDE.md auto-merge successful tables_merged=1
```

**Meaning**: Orc detected a conflict in CLAUDE.md's knowledge section and automatically resolved it by combining rows from both sides. No action needed.

### Auto-Resolution Failed

**Symptoms**:
```
WARN CLAUDE.md conflict cannot be auto-resolved: Table 'Patterns Learned': conflict is not purely additive
```

**Cause**: The conflict involves more than just adding new rows - either:
- Same row was edited differently on both sides
- Conflict is outside the knowledge section markers
- Table structure is malformed

**Solutions**:

| Scenario | Resolution |
|----------|------------|
| Overlapping edits | Manually resolve in editor, keeping both changes |
| Outside markers | Ensure `<!-- orc:knowledge:begin -->` and `<!-- orc:knowledge:end -->` markers exist |
| Malformed table | Fix table syntax (proper `|` separators, header row) |

**To resolve manually**:
```bash
# View the conflict
cat CLAUDE.md | grep -A20 "<<<<<<<"

# Edit the file to resolve
$EDITOR CLAUDE.md

# Mark as resolved
git add CLAUDE.md
git commit -m "Resolve CLAUDE.md conflict"
```

### Preventing CLAUDE.md Conflicts

**Best practices for parallel task execution**:

1. **Use different tables for different task types**: Patterns vs Gotchas vs Decisions
2. **Include unique source IDs**: Every entry should have `TASK-XXX` identifier
3. **Keep entries atomic**: One pattern/gotcha/decision per row
4. **Run finalize soon after completion**: Reduces divergence time

**If conflicts are frequent**:

Orc's auto-merge handles most cases automatically. If you're seeing manual resolution required frequently, check:
- Are the knowledge section markers present?
- Are multiple tasks editing the same row (not just adding)?
- Is the table structure valid markdown?

---

## Log Locations

| File | Purpose |
|------|---------|
| `.orc/orc.db` | SQLite database (source of truth) |
| `.orc/tasks/TASK-XXX/transcripts/` | Claude session logs (markdown exports) |
| `orc.yaml` | Project configuration |

---

## Getting Help

1. **View task details**: `orc show TASK-XXX`
2. **Read transcripts**: `orc log TASK-XXX --phase implement`
3. **Check git state**: `git status`, `git log --oneline orc/TASK-XXX`
4. **Verbose mode**: `orc -vv run TASK-XXX`
