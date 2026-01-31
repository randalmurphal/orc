# Configuration Hierarchy Specification

> Simplified 4-level configuration: defaults → shared → personal → runtime.

## Design Principles

1. **Four levels only** - Simple mental model, easy to debug
2. **Individual wins** - Personal settings always override team defaults
3. **Explicit sources** - Users can see where each setting comes from
4. **No forced sync** - Personal settings never synced without consent
5. **Git-friendly** - Shared configs are git-trackable

---

## Resolution Order (Simplified)

The previous 8-level hierarchy was reduced to 4 conceptual levels:

```
Highest Priority
       ↓
┌─────────────────────────────────────────┐
│ 1. RUNTIME                              │  env vars, CLI flags
│    (temporary, not persisted)           │  (ORC_*, --model, --profile)
├─────────────────────────────────────────┤
│ 2. PERSONAL                             │  ~/.orc/config.yaml
│    (user's machine-wide defaults)       │  ~/.orc/projects/<id>/config.yaml
├─────────────────────────────────────────┤
│ 3. PROJECT                              │  .orc/config.yaml
│    (project defaults, git-tracked)      │
├─────────────────────────────────────────┤
│ 4. DEFAULTS                             │  Built-in code defaults
│    (fallback values)                    │
└─────────────────────────────────────────┘
       ↓
Lowest Priority
```

### Why 4 Levels?

| Old Level | New Level | Rationale |
|-----------|-----------|-----------|
| env vars | Runtime | Temporary overrides, same purpose |
| CLI flags | Runtime | Temporary overrides, same purpose |
| user global | Personal | User's preferences (`~/.orc/config.yaml`) |
| project local | Personal | User's project-specific preferences (`~/.orc/projects/<id>/config.yaml`) |
| project shared | Removed | `.orc/shared/` eliminated — use project config |
| project root | Project | Project defaults (`.orc/config.yaml`, git-tracked) |
| system (/etc/) | Removed | Rarely used, admins can use env vars |
| built-in | Defaults | Fallback |

### Level Details

**Runtime** (Level 1)
- Environment variables (`ORC_*`)
- CLI flags (`--model`, `--profile`)
- Not persisted, applies only to current command
- Highest priority - always wins
- **Within-level order**: CLI flags override env vars

**Personal** (Level 2)
- `~/.orc/config.yaml` - User's global defaults
- `~/.orc/projects/<id>/config.yaml` - User's project-specific preferences
- Persisted, applies to all commands
- Second priority - wins over project defaults
- **Within-level order**: project-specific overrides global (project-specific > global)

**Project** (Level 3)
- `.orc/config.yaml` - Project defaults (git-tracked)
- Third priority - project baseline
- **Note**: The old `.orc/shared/` and `.orc/local/` directories have been removed. Personal config moved to `~/.orc/projects/<id>/`.

**Defaults** (Level 4)
- Built-in values in code
- Lowest priority - fallback only

### Within-Level Order (Important!)

Each level may have multiple sources. **Later sources override earlier sources** within the same level:

```
LEVEL           | ORDER (earlier → later)                    | WHY
----------------|-------------------------------------------- |----------------------------------
Runtime         | env vars → CLI flags                       | CLI is more intentional
Personal        | ~/.orc/ → .orc/local/                      | Project-specific > global
Shared          | .orc/config.yaml → .orc/shared/config.yaml | Team defaults > project defaults
```

**Visual Example:**

```
$ orc config resolution model

Level 1 - Runtime:
  env (ORC_MODEL):     claude-opus-4        # Set in shell
  flags (--model):     (not set)
  → Runtime value: claude-opus-4

Level 2 - Personal:
  ~/.orc/config.yaml:       claude-sonnet-4  # User's global
  .orc/local/config.yaml:   (not set)
  → Personal value: claude-sonnet-4

Level 3 - Shared:
  .orc/config.yaml:         claude-sonnet-4  # Project default
  .orc/shared/config.yaml:  claude-haiku     # Team default (wins)
  → Shared value: claude-haiku

Level 4 - Defaults:
  builtin: claude-opus-4-5-20251101

FINAL: claude-opus-4 (from Runtime env ORC_MODEL)
```

**Common Scenarios:**

| Scenario | Which File Wins | Why |
|----------|-----------------|-----|
| User sets personal preference | `~/.orc/config.yaml` | No project-local override |
| User overrides for one project | `.orc/local/config.yaml` | Project-specific > global |
| Team sets defaults | `.orc/shared/config.yaml` | Team > project defaults |
| Quick one-off override | `ORC_MODEL=x orc run` | Runtime > everything |

---

## Directory Structure

```
~/.orc/                                # User global
├── config.yaml                        # Personal defaults
├── ui.yaml                            # UI preferences
├── prompts/                           # Personal prompt overrides
│   └── implement.md
└── skills/                            # Personal skills
    └── my-style/

~/project/.orc/                        # Project
├── config.yaml                        # Project defaults
├── orc.db                             # Local database
│
├── shared/                            # Git-tracked team resources
│   ├── config.yaml                    # Team defaults
│   ├── prompts/                       # Team prompts
│   ├── skills/                        # Team skills
│   └── templates/                     # Team templates
│
└── local/                             # Gitignored personal
    ├── config.yaml                    # Personal overrides
    ├── prompts/                       # Personal prompt overrides
    └── notes/                         # Personal task notes
```

---

## Configuration Schema

### Full Config Structure

```yaml
# config.yaml
version: 1

# Automation profile (auto, fast, safe, strict)
profile: auto

# AI execution settings (individual control)
model: claude-sonnet-4-20250514
max_iterations: 30
timeout: 10m

# Gate configuration
gates:
  default_type: auto                   # auto | ai | human
  auto_approve_on_success: true
  retry_on_failure: true
  max_retries: 5
  phase_overrides:                     # Override per phase
    review: ai
    merge: human
  weight_overrides:                    # Override per weight class
    large:
      spec: human
      design: human

# Cross-phase retry
retry:
  enabled: true
  max_retries: 5                       # Deprecated: use executor.max_retries
  retry_map:
    test: implement
    validate: implement

# Execution settings
executor:
  use_session_execution: false         # Use session-based vs flowgraph execution
  session_persistence: true
  checkpoint_interval: 0               # 0 = phase-complete only
  max_retries: 5                       # Max retry attempts when phase fails (default: 5)

# Artifact skip detection
artifact_skip:
  enabled: true                        # Check for existing artifacts (default: true)
  auto_skip: false                     # Skip without prompting (default: false, --auto-skip flag overrides)
  phases:                              # Phases to check for artifacts
    - spec                             # spec.md with valid content
    - research                         # artifacts/research.md or research in spec
    - docs                             # artifacts/docs.md

# Worktree isolation
worktree:
  enabled: true
  dir: .orc/worktrees
  cleanup_on_complete: true
  cleanup_on_fail: false

# Task completion actions
completion:
  action: pr                           # pr | merge | none
  target_branch: main
  delete_branch: true
  pr:
    title: '[orc] {{TASK_TITLE}}'
    body_template: templates/pr-body.md
    labels: [automated]                # Labels applied to PR (gracefully skipped if missing)
    draft: false
    auto_merge: false                  # Auto-merge after finalize (default: false)
    auto_approve: false                # AI-assisted PR approval (default: false)
    team_reviewers: []                 # GitHub team slugs to request review from
    assignees: []                      # GitHub usernames to assign to the PR
    maintainer_can_modify: true        # Allow maintainers to push to the PR branch (default: true)
  ci:
    wait_for_ci: false                 # Wait for CI checks before merge (default: false)
    ci_timeout: 10m                    # Max time to wait for CI checks (default: 10m)
    poll_interval: 30s                 # CI status polling interval (default: 30s)
    merge_on_ci_pass: false            # Auto-merge when CI passes (default: false)
    merge_method: squash               # Merge method: squash | merge | rebase (default: squash)
    # Commit message templates - available variables: {{TASK_ID}}, {{TASK_TITLE}}, {{TASK_BRANCH}}
    merge_commit_template: ""          # Custom merge commit message (empty = provider default)
    squash_commit_template: ""         # Custom squash commit message (empty = provider default)
    verify_sha_on_merge: true          # Verify HEAD SHA before merge to prevent races (default: true)
  sync:
    strategy: completion               # none | phase | completion | detect
    sync_on_start: true                # Sync before execution starts (default: true, catches stale worktrees)
    fail_on_conflict: true             # Abort on conflicts vs continue with warning
    max_conflict_files: 0              # Max files with conflicts before aborting (0 = unlimited)
    skip_for_weights: [trivial]        # Skip sync for these task weights
  finalize:
    enabled: true                      # Enable finalize phase (default: true)
    auto_trigger: true                 # Auto-run after validate phase (default: true)
    auto_trigger_on_approval: true     # Auto-run when PR is approved (default: true for "auto" profile)
    sync:
      strategy: merge                  # merge | rebase (default: merge)
    conflict_resolution:
      enabled: true                    # AI-assisted conflict resolution (default: true)
      instructions: ""                 # Additional resolution instructions
    risk_assessment:
      enabled: true                    # Enable risk classification (default: true)
      re_review_threshold: high        # low | medium | high | critical (default: high)
    gates:
      pre_merge: auto                  # auto | ai | human | none (default: auto)

# Git settings
git:
  branch_prefix: orc/
  commit_prefix: '[orc]'

# Claude CLI settings
claude:
  path: claude                            # Auto-detects: PATH lookup → common install locations
  dangerously_skip_permissions: true

# Token pool (personal only)
pool:
  enabled: true
  use_team_pool: false                 # Opt-in to team tokens

# Cost tracking
cost:
  warn_per_task: 2.00
  warn_daily: 20.00
  show_running_cost: true

# Timeouts and progress indication
timeouts:
  phase_max: 60m                       # Max time per phase (0 = unlimited, default: 60m)
  turn_max: 10m                        # Max time per API turn (0 = unlimited)
  idle_warning: 5m                     # Warn if no tool calls for this duration
  heartbeat_interval: 30s              # Progress dots during API calls (0 = disable)
  idle_timeout: 2m                     # Warn if no streaming activity

# Task settings
tasks:
  disable_auto_commit: false           # Disable auto-commit for .orc/ file mutations (default: false)

# Diagnostics configuration
diagnostics:
  resource_tracking:
    enabled: true                      # Enable process/memory tracking (default: true)
    memory_threshold_mb: 500           # Warn if memory grows by > threshold (default: 500)
    filter_system_processes: true      # Only flag orc-related processes as orphans (default: true)

# Task ID configuration (team mode)
task_id:
  mode: solo                           # solo | p2p | team
  prefix_source: none                  # none | initials | username

# Server configuration (team mode)
server:
  host: 127.0.0.1
  port: 8080
  auth:
    enabled: false
    type: token                        # token | oidc

# Team mode
team:
  enabled: false
  server_url: ""
  sync_tasks: false

# Jira Cloud import
jira:
  url: "https://acme.atlassian.net"        # Jira Cloud instance URL
  email: "user@acme.com"                    # Email for basic auth
  token_env_var: ORC_JIRA_TOKEN             # Env var name for API token (default)
  epic_to_initiative: true                  # Map epics → orc initiatives (default: true)
  default_weight: ""                        # Default weight for imported tasks (trivial|small|medium|large)
  default_queue: ""                         # Default queue for imported tasks (active|backlog)
  default_projects: []                      # Project keys imported by default (override with --project)
  custom_fields:                            # Jira custom field ID → metadata key name
    # customfield_10020: jira_sprint
    # customfield_10028: jira_story_points
  status_overrides: {}                      # Jira status name → orc queue ("active", "backlog")
  category_overrides: {}                    # Jira issue type → orc category ("bug", "feature", etc.)
  priority_overrides: {}                    # Jira priority name → orc priority ("critical", "high", etc.)
```

### Config Categories

| Category | Individual Control? | Shareable? |
|----------|--------------------|-----------|
| `model` | Always | Can suggest default |
| `max_iterations` | Always | Can suggest default |
| `timeout` | Always | Can suggest default |
| `profile` | Always | Can suggest default |
| `gates` | Always | Can set defaults |
| `retry` | Always | Can set defaults |
| `artifact_skip` | Always | Can set defaults |
| `worktree` | Always | Can set defaults |
| `timeouts` | Always | Can set defaults |
| `tasks` | Always | Can set defaults |
| `diagnostics` | Always | Can set defaults |
| `completion` | Project | Project level |
| `git` | Project | Project level |
| `claude` | Always | No |
| `pool` | Always | No |
| `cost` | Always | No |
| `task_id` | Project | Project level |
| `server` | Machine | No |
| `team` | Project | Project level |
| `jira` | Project | Project level |

---

## Implementation

### Config Loader (Simplified)

```go
// internal/config/loader.go

// ConfigLevel represents one of the 4 conceptual levels
type ConfigLevel int

const (
    LevelDefaults ConfigLevel = iota  // Built-in defaults
    LevelShared                       // Team + project config
    LevelPersonal                     // User global + project local
    LevelRuntime                      // Env vars + CLI flags
)

type Loader struct {
    projectDir string
    userDir    string
    flags      *pflag.FlagSet
}

func NewLoader(projectDir string) *Loader {
    return &Loader{
        projectDir: projectDir,
        userDir:    filepath.Join(os.Getenv("HOME"), ".orc"),
    }
}

func (l *Loader) Load() (*Config, error) {
    result := &TrackedConfig{
        Config:  DefaultConfig(),
        Sources: make(map[string]string),
    }

    // Level 4: Defaults (already set)

    // Level 3: Shared (team + project)
    l.loadLevel(result, LevelShared, []string{
        filepath.Join(l.projectDir, ".orc", "config.yaml"),
        filepath.Join(l.projectDir, ".orc", "shared", "config.yaml"),
    })

    // Level 2: Personal (user global + project local)
    l.loadLevel(result, LevelPersonal, []string{
        filepath.Join(l.userDir, "config.yaml"),
        filepath.Join(l.projectDir, ".orc", "local", "config.yaml"),
    })

    // Level 1: Runtime (env + flags)
    l.loadEnv(result)
    l.loadFlags(result)

    return result.Config, nil
}

func (l *Loader) loadLevel(result *TrackedConfig, level ConfigLevel, paths []string) {
    levelName := levelNames[level]
    for _, path := range paths {
        cfg, err := loadYAML(path)
        if err != nil {
            continue // Skip missing/invalid files
        }
        mergeWithTracking(result, cfg, levelName+": "+path)
    }
}

var levelNames = map[ConfigLevel]string{
    LevelDefaults: "default",
    LevelShared:   "shared",
    LevelPersonal: "personal",
    LevelRuntime:  "runtime",
}
```

### Config Merging

```go
// internal/config/merge.go
func mergeConfig(base, overlay *Config) *Config {
    result := *base

    // Simple fields: overlay wins if set
    if overlay.Model != "" {
        result.Model = overlay.Model
    }
    if overlay.MaxIterations > 0 {
        result.MaxIterations = overlay.MaxIterations
    }
    if overlay.Timeout > 0 {
        result.Timeout = overlay.Timeout
    }
    if overlay.Profile != "" {
        result.Profile = overlay.Profile
    }

    // Nested structs: merge recursively
    result.Gates = mergeGates(base.Gates, overlay.Gates)
    result.Retry = mergeRetry(base.Retry, overlay.Retry)
    result.Worktree = mergeWorktree(base.Worktree, overlay.Worktree)
    result.Completion = mergeCompletion(base.Completion, overlay.Completion)

    // Maps: overlay wins per-key
    result.Gates.PhaseOverrides = mergeMaps(
        base.Gates.PhaseOverrides,
        overlay.Gates.PhaseOverrides,
    )

    return &result
}

func mergeMaps[K comparable, V any](base, overlay map[K]V) map[K]V {
    result := make(map[K]V)
    for k, v := range base {
        result[k] = v
    }
    for k, v := range overlay {
        result[k] = v
    }
    return result
}
```

### Source Tracking

```go
// internal/config/tracked.go
type TrackedConfig struct {
    Config  *Config
    Sources map[string]string  // field path → source name
}

func (l *Loader) recordSources(cfg *Config) {
    tracked := &TrackedConfig{
        Config:  cfg,
        Sources: make(map[string]string),
    }

    // For each field, record which source set it
    for i := len(l.sources) - 1; i >= 0; i-- {
        source := l.sources[i]
        if source.Config == nil {
            continue
        }

        // Check each field
        if source.Config.Model != "" {
            tracked.Sources["model"] = source.Name
        }
        // ... etc for all fields
    }

    cfg.tracked = tracked
}

// CLI: orc config show --source
func (c *TrackedConfig) PrintWithSources() {
    fmt.Printf("model = %s (from %s)\n", c.Config.Model, c.Sources["model"])
    fmt.Printf("max_iterations = %d (from %s)\n", c.Config.MaxIterations, c.Sources["max_iterations"])
    // ...
}
```

### Environment Variables

```go
// internal/config/env.go
var envMapping = map[string]string{
    "ORC_PROFILE":           "profile",
    "ORC_MODEL":             "model",
    "ORC_MAX_ITERATIONS":    "max_iterations",
    "ORC_TIMEOUT":           "timeout",
    "ORC_CLAUDE_PATH":       "claude.path",
    "ORC_RETRY_ENABLED":       "retry.enabled",
    "ORC_RETRY_MAX_RETRIES":   "retry.max_retries",
    "ORC_EXECUTOR_MAX_RETRIES": "executor.max_retries",
    "ORC_GATES_DEFAULT":       "gates.default_type",
    "ORC_GATES_MAX_RETRIES":   "gates.max_retries",
    "ORC_WORKTREE_ENABLED":  "worktree.enabled",
    "ORC_WORKTREE_DIR":      "worktree.dir",
    "ORC_COMPLETION_ACTION":          "completion.action",
    "ORC_SYNC_STRATEGY":              "completion.sync.strategy",
    "ORC_SYNC_ON_START":              "completion.sync.sync_on_start",
    "ORC_SYNC_FAIL_ON_CONFLICT":      "completion.sync.fail_on_conflict",
    // CI and merge settings
    "ORC_CI_WAIT":                    "completion.ci.wait_for_ci",
    "ORC_CI_TIMEOUT":                 "completion.ci.ci_timeout",
    "ORC_CI_POLL_INTERVAL":           "completion.ci.poll_interval",
    "ORC_CI_MERGE_ON_PASS":           "completion.ci.merge_on_ci_pass",
    "ORC_CI_MERGE_METHOD":            "completion.ci.merge_method",
    "ORC_BRANCH_PREFIX":              "git.branch_prefix",
    "ORC_COMMIT_PREFIX":     "git.commit_prefix",
    "ORC_POOL_ENABLED":      "pool.enabled",
    "ORC_HOST":              "server.host",
    "ORC_PORT":              "server.port",
    "ORC_AUTH_ENABLED":      "server.auth.enabled",
    "ORC_AUTH_TYPE":         "server.auth.type",
    "ORC_AUTH_TOKEN":        "server.auth.token",  // Never logged!
    "ORC_TEAM_ENABLED":      "team.enabled",
    "ORC_TEAM_SERVER":       "team.server_url",
    // Timeouts
    "ORC_PHASE_MAX_TIMEOUT":  "timeouts.phase_max",
    "ORC_TURN_MAX_TIMEOUT":   "timeouts.turn_max",
    "ORC_IDLE_WARNING":       "timeouts.idle_warning",
    "ORC_HEARTBEAT_INTERVAL": "timeouts.heartbeat_interval",
    "ORC_IDLE_TIMEOUT":       "timeouts.idle_timeout",
}

func (l *Loader) loadFromEnv() *Config {
    cfg := &Config{}

    for env, path := range envMapping {
        if value := os.Getenv(env); value != "" {
            setField(cfg, path, value)
        }
    }

    return cfg
}
```

---

## Prompt Resolution

### Resolution Chain

```go
// internal/prompt/resolver.go
type Resolver struct {
    personalDir string  // ~/.orc/prompts/
    localDir    string  // .orc/local/prompts/
    sharedDir   string  // .orc/shared/prompts/
    projectDir  string  // .orc/prompts/
    embedded    embed.FS
}

func (r *Resolver) Resolve(phase string) (content string, source Source, err error) {
    filename := phase + ".md"

    // 1. Personal global
    if content, err := r.readFile(r.personalDir, filename); err == nil {
        return content, SourcePersonalGlobal, nil
    }

    // 2. Project local (personal override)
    if content, err := r.readFile(r.localDir, filename); err == nil {
        return content, SourceProjectLocal, nil
    }

    // 3. Project shared (team default)
    if content, err := r.readFile(r.sharedDir, filename); err == nil {
        return content, SourceProjectShared, nil
    }

    // 4. Project root (legacy location)
    if content, err := r.readFile(r.projectDir, filename); err == nil {
        return content, SourceProject, nil
    }

    // 5. Embedded default
    content, err = r.readEmbedded(filename)
    return content, SourceEmbedded, err
}

type Source string

const (
    SourcePersonalGlobal Source = "personal_global" // ~/.orc/prompts/
    SourceProjectLocal   Source = "project_local"   // .orc/local/prompts/
    SourceProjectShared  Source = "project_shared"  // .orc/shared/prompts/
    SourceProject        Source = "project"         // .orc/prompts/
    SourceEmbedded       Source = "embedded"        // Built-in
)
```

### Prompt Inheritance

```yaml
# ~/.orc/prompts/implement.md
---
extends: embedded              # Start with built-in prompt
prepend: |
  PERSONAL PREFERENCES:
  - Always use TypeScript strict mode
  - Prefer functional patterns
  - Include comprehensive error handling
append: |
  REMINDERS:
  - Run tests before marking complete
---
```

```go
// internal/prompt/inheritance.go
type PromptMeta struct {
    Extends string `yaml:"extends"`
    Prepend string `yaml:"prepend"`
    Append  string `yaml:"append"`
}

func (r *Resolver) resolveWithInheritance(phase string, source Source) (string, error) {
    content, _ := r.readFromSource(phase, source)

    // Parse frontmatter
    meta, body := parseFrontmatter(content)

    if meta.Extends == "" {
        return body, nil
    }

    // Resolve parent
    var parentSource Source
    switch meta.Extends {
    case "embedded":
        parentSource = SourceEmbedded
    case "shared":
        parentSource = SourceProjectShared
    case "project":
        parentSource = SourceProject
    default:
        return "", fmt.Errorf("unknown extends: %s", meta.Extends)
    }

    parent, err := r.resolveWithInheritance(phase, parentSource)
    if err != nil {
        return "", err
    }

    // Combine
    var result strings.Builder
    if meta.Prepend != "" {
        result.WriteString(meta.Prepend)
        result.WriteString("\n\n")
    }
    result.WriteString(parent)
    if meta.Append != "" {
        result.WriteString("\n\n")
        result.WriteString(meta.Append)
    }

    return result.String(), nil
}
```

---

## CLI Commands

### Show Config

```bash
$ orc config show
profile: safe
model: claude-sonnet-4-20250514
max_iterations: 30
timeout: 10m
gates:
  default_type: auto
  phase_overrides:
    review: ai
    merge: human
...
```

### Show Config with Sources

```bash
$ orc config show --source
profile = safe (personal: ~/.orc/config.yaml)
model = claude-sonnet-4-20250514 (personal: ~/.orc/config.yaml)
max_iterations = 30 (default)
timeout = 10m (default)
gates.default_type = auto (shared: .orc/shared/config.yaml)
gates.phase_overrides.review = ai (shared: .orc/shared/config.yaml)
gates.phase_overrides.merge = human (shared: .orc/shared/config.yaml)
```

### Get Specific Value

```bash
$ orc config get model
claude-sonnet-4-20250514

$ orc config get model --source
claude-sonnet-4-20250514 (from user: ~/.orc/config.yaml)
```

### Set Value

```bash
# Set in user config
$ orc config set model claude-opus-4
Set model = claude-opus-4 in ~/.orc/config.yaml

# Set in project config
$ orc config set --project gates.default_type ai
Set gates.default_type = ai in .orc/config.yaml

# Set in shared config
$ orc config set --shared gates.phase_overrides.merge human
Set gates.phase_overrides.merge = human in .orc/shared/config.yaml
```

### Edit Config

```bash
# Open user config in editor
$ orc config edit

# Open project config
$ orc config edit --project

# Open shared config
$ orc config edit --shared
```

### Show Resolution Chain

```bash
$ orc config resolution model
Resolution chain for 'model':
  RUNTIME (highest priority):
    env (ORC_MODEL): not set
    flags (--model): not set
  PERSONAL:
    ~/.orc/config.yaml: claude-sonnet-4-20250514 ← WINNER
    .orc/local/config.yaml: not set
  SHARED:
    .orc/shared/config.yaml: claude-sonnet-4
    .orc/config.yaml: not set
  DEFAULTS:
    builtin: claude-opus-4-5-20251101

Final value: claude-sonnet-4-20250514 (from personal)
```

---

## UI Integration

### Settings Page

```svelte
<script lang="ts">
    let config = $state<Config>(null);
    let sources = $state<Record<string, string>>({});

    async function loadConfig() {
        const response = await api.getConfigWithSources();
        config = response.config;
        sources = response.sources;
    }

    function getSourceBadge(field: string): string {
        const source = sources[field];
        switch (source) {
            case 'user_global': return 'Personal';
            case 'project_local': return 'Personal (Project)';
            case 'project_shared': return 'Team';
            case 'project': return 'Project';
            case 'env': return 'Environment';
            default: return 'Default';
        }
    }
</script>

<div class="settings-page">
    <section>
        <h2>AI Settings</h2>
        <p class="description">These settings control how Claude executes tasks.
           Your personal settings always override team defaults.</p>

        <div class="setting">
            <label>Model</label>
            <select bind:value={config.model}>
                <option value="claude-opus-4-5-20251101">Claude Opus 4.5</option>
                <option value="claude-sonnet-4-20250514">Claude Sonnet 4</option>
            </select>
            <span class="source-badge">{getSourceBadge('model')}</span>
        </div>

        <div class="setting">
            <label>Max Iterations</label>
            <input type="number" bind:value={config.max_iterations} />
            <span class="source-badge">{getSourceBadge('max_iterations')}</span>
        </div>
    </section>

    <section>
        <h2>Team Defaults</h2>
        <p class="description">These are your team's recommended settings.
           You can override them in the Personal section above.</p>

        {#if teamConfig}
            <div class="team-config readonly">
                <div>Model: {teamConfig.model}</div>
                <div>Profile: {teamConfig.profile}</div>
            </div>
        {/if}
    </section>
</div>
```

### Config Hierarchy Visualization

```svelte
<!-- Simplified 4-level visualization -->
<div class="config-hierarchy">
    <div class="level runtime" class:active={hasRuntimeOverrides}>
        <span class="number">1</span>
        <span class="label">Runtime</span>
        <span class="sources">env vars, CLI flags</span>
        <span class="status">{hasRuntimeOverrides ? 'Active' : 'None'}</span>
    </div>
    <div class="arrow">↓</div>
    <div class="level personal" class:active={hasPersonalConfig}>
        <span class="number">2</span>
        <span class="label">Personal</span>
        <span class="sources">~/.orc/, .orc/local/</span>
        <span class="status">{hasPersonalConfig ? 'Active' : 'Empty'}</span>
    </div>
    <div class="arrow">↓</div>
    <div class="level shared" class:active={hasSharedConfig}>
        <span class="number">3</span>
        <span class="label">Shared</span>
        <span class="sources">.orc/shared/, .orc/</span>
        <span class="status">{hasSharedConfig ? 'Active' : 'Empty'}</span>
    </div>
    <div class="arrow">↓</div>
    <div class="level defaults active">
        <span class="number">4</span>
        <span class="label">Defaults</span>
        <span class="sources">built-in</span>
        <span class="status">Always</span>
    </div>
</div>

<style>
    .level { display: flex; align-items: center; gap: 1rem; padding: 0.5rem; }
    .level.active { background: var(--surface-active); }
    .number { font-weight: bold; color: var(--accent); }
    .sources { color: var(--text-muted); font-size: 0.875rem; }
</style>
```

---

## Testing

### Config Loading Tests

```go
func TestConfigResolution(t *testing.T) {
    // Setup test directories
    tmpDir := t.TempDir()
    userDir := filepath.Join(tmpDir, ".orc")
    projectDir := filepath.Join(tmpDir, "project", ".orc")
    sharedDir := filepath.Join(projectDir, "shared")

    os.MkdirAll(userDir, 0755)
    os.MkdirAll(sharedDir, 0755)

    // Write user config
    writeYAML(filepath.Join(userDir, "config.yaml"), map[string]any{
        "model": "user-model",
    })

    // Write shared config
    writeYAML(filepath.Join(sharedDir, "config.yaml"), map[string]any{
        "model":          "shared-model",
        "max_iterations": 50,
    })

    // Load config
    loader := NewLoader()
    loader.SetUserDir(userDir)
    loader.SetProjectDir(projectDir)

    cfg, err := loader.Load()
    require.NoError(t, err)

    // User wins over shared
    assert.Equal(t, "user-model", cfg.Model)

    // Shared provides max_iterations
    assert.Equal(t, 50, cfg.MaxIterations)
}

func TestEnvOverride(t *testing.T) {
    t.Setenv("ORC_MODEL", "env-model")

    loader := NewLoader()
    cfg, err := loader.Load()
    require.NoError(t, err)

    // Env wins over everything
    assert.Equal(t, "env-model", cfg.Model)
}
```

### Prompt Resolution Tests

```go
func TestPromptResolution(t *testing.T) {
    tmpDir := t.TempDir()

    // Create hierarchy
    os.MkdirAll(filepath.Join(tmpDir, ".orc", "prompts"), 0755)
    os.MkdirAll(filepath.Join(tmpDir, "project", ".orc", "shared", "prompts"), 0755)
    os.MkdirAll(filepath.Join(tmpDir, "project", ".orc", "local", "prompts"), 0755)

    // Write prompts at different levels
    os.WriteFile(
        filepath.Join(tmpDir, "project", ".orc", "shared", "prompts", "implement.md"),
        []byte("shared prompt"),
        0644,
    )
    os.WriteFile(
        filepath.Join(tmpDir, "project", ".orc", "local", "prompts", "implement.md"),
        []byte("local prompt"),
        0644,
    )

    resolver := NewResolver(
        filepath.Join(tmpDir, ".orc", "prompts"),
        filepath.Join(tmpDir, "project", ".orc", "local", "prompts"),
        filepath.Join(tmpDir, "project", ".orc", "shared", "prompts"),
    )

    content, source, err := resolver.Resolve("implement")
    require.NoError(t, err)

    // Local wins over shared
    assert.Equal(t, "local prompt", content)
    assert.Equal(t, SourceProjectLocal, source)
}
```
