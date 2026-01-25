# Interactive Init Wizard

**Status**: Planning
**Priority**: P0
**Last Updated**: 2026-01-10

---

## Problem Statement

Current `orc init`:
- Creates `.orc/` directory with default config
- No customization opportunity
- No project type detection
- No guidance for new users

---

## Solution: Interactive Wizard with Smart Defaults

A step-by-step initialization that:
1. Detects project type automatically
2. Offers sensible defaults with easy overrides
3. Optionally spawns Claude session for advanced setup
4. Installs relevant skills/plugins

---

## User Experience

### Default Mode (Interactive)

```bash
$ orc init

┌─ orc init ──────────────────────────────────────────────────┐
│                                                             │
│  Welcome to orc! Let's set up your project.                 │
│                                                             │
│  Detected: Go project (go.mod found)                        │
│  Framework: Cobra CLI, Gin HTTP                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘

? Automation profile
  > auto    - Fully automated, no human intervention
    safe    - AI reviews code, humans approve merges
    strict  - Human gates on spec, design, and merge
    custom  - Configure gates manually

? When tasks complete
  > Create PR    - Open pull request for review
    Merge        - Merge directly to target branch
    None         - Just commit, no PR or merge

? Default model
  > claude-opus-4-5  - Best quality (recommended)
    claude-sonnet    - Faster, good for simple tasks

? Install recommended skills for Go projects?
  > Yes - Install go-testing, go-linting skills
    No  - Skip skill installation

? Add orc section to CLAUDE.md?
  > Yes - Add project context for Claude
    No  - Skip

Initializing...
  ✓ Created .orc/config.yaml
  ✓ Created .orc/tasks/
  ✓ Registered project: my-app (ID: 9709f4b3)
  ✓ Installed skills: go-testing, go-linting
  ✓ Updated CLAUDE.md

✅ orc initialized successfully!

Next steps:
  orc new "Your first task"   Create a task
  orc serve                   Start web UI
  orc --help                  See all commands
```

### Quick Mode (Non-Interactive)

```bash
$ orc init --quick

✓ Detected: Go project
✓ Using defaults: auto profile, PR on complete
✓ Created .orc/config.yaml
✓ Registered project: my-app

Done! Run: orc new "Your task"
```

### Advanced Mode (Claude Session)

```bash
$ orc init --advanced

Starting advanced setup with Claude...

# Opens interactive Claude session with setup prompt
# Claude can:
# - Analyze project structure deeply
# - Suggest custom prompts based on codebase
# - Configure complex automation rules
# - Set up project-specific scripts
# - Install and configure MCP servers

Session complete. Configuration saved.
```

---

## Project Detection

### Language Detection

| Indicator | Project Type | Config Implications |
|-----------|--------------|---------------------|
| `go.mod` | Go | Test: `go test ./...`, Lint: ruff-equivalent |
| `package.json` | Node.js | Test: `bun test`, Lint: eslint |
| `pyproject.toml` | Python | Test: `pytest`, Lint: ruff |
| `Cargo.toml` | Rust | Test: `cargo test`, Lint: clippy |
| `pom.xml` | Java/Maven | Test: `mvn test` |
| `build.gradle` | Java/Gradle | Test: `gradle test` |

### Framework Detection

| Indicator | Framework | Additional Config |
|-----------|-----------|-------------------|
| `gin-gonic/gin` in go.mod | Gin | HTTP testing templates |
| `next.config.js` | Next.js | E2E with Playwright |
| `vite.config.ts` | Vite | Vitest for testing |
| `fastapi` in pyproject.toml | FastAPI | pytest-asyncio |
| `django` in requirements | Django | Django test runner |

### Detection Output

```go
type ProjectInfo struct {
    Language    string   // "go", "python", "typescript"
    Frameworks  []string // ["gin", "cobra"]
    TestCommand string   // "go test ./..."
    LintCommand string   // "golangci-lint run"
    HasTests    bool     // true if test files exist
    HasCI       bool     // true if .github/workflows exists
}
```

---

## Wizard Steps

### Step 1: Project Detection (Automatic)

```go
func detectProject(path string) (*ProjectInfo, error) {
    info := &ProjectInfo{}

    // Check for language markers
    if fileExists("go.mod") {
        info.Language = "go"
        info.TestCommand = "go test ./... -v -race"
        info.LintCommand = "golangci-lint run"

        // Check for frameworks
        goMod := readFile("go.mod")
        if strings.Contains(goMod, "gin-gonic/gin") {
            info.Frameworks = append(info.Frameworks, "gin")
        }
        if strings.Contains(goMod, "spf13/cobra") {
            info.Frameworks = append(info.Frameworks, "cobra")
        }
    }
    // ... similar for other languages

    return info, nil
}
```

### Step 2: Profile Selection

```
? Automation profile

  > auto
    Fully automated execution. Claude handles everything.
    Best for: Routine tasks, trusted environments.
    Gates: All phases auto-approve on success.

    safe
    AI reviews code, humans approve merges.
    Best for: Production codebases, team environments.
    Gates: review=ai, merge=human.

    strict
    Human approval on key decisions.
    Best for: Critical systems, compliance requirements.
    Gates: spec=human, design=human, merge=human.

    custom
    Configure each gate manually.

  [↑↓ to move, Enter to select]
```

### Step 3: Completion Action

```
? When tasks complete

  > Create PR
    Opens a pull request for team review.
    Good for: Team workflows, code review requirements.

    Merge directly
    Merges to target branch immediately.
    Good for: Solo projects, auto-approved changes.

    None
    Just commits to the task branch.
    Good for: Manual workflow, CI/CD handles merging.

  [↑↓ to move, Enter to select]
```

### Step 4: Model Selection

```
? Default model

  > claude-opus-4-5 (recommended)
    Best reasoning, highest quality output.
    ~$15/1M input, ~$75/1M output tokens.

    claude-sonnet
    Fast and capable for most tasks.
    ~$3/1M input, ~$15/1M output tokens.

  [↑↓ to move, Enter to select]
```

### Step 5: Skills Installation

```
? Install recommended skills for Go projects?

  Recommended skills:
  • go-testing    - Go test patterns and table-driven tests
  • go-linting    - golangci-lint integration
  • go-modules    - Dependency management patterns

  > Yes, install recommended skills
    No, skip skill installation
    Custom, choose which to install

  [↑↓ to move, Enter to select]
```

### Step 6: CLAUDE.md Integration

```
? Add orc section to CLAUDE.md?

  This adds:
  • Orc commands reference
  • Project-specific scripts
  • Testing conventions
  • Coding standards

  The section is clearly marked and can be regenerated.

  > Yes, add orc section
    No, skip

  [↑↓ to move, Enter to select]
```

---

## CLAUDE.md Section

Added to project CLAUDE.md:

```markdown
<!-- orc:start - Auto-generated, do not edit manually -->
## Orc Task Orchestrator

This project uses [orc](https://github.com/randalmurphal/orc) for task orchestration.

### Commands

| Command | Description |
|---------|-------------|
| `orc new "title"` | Create a new task |
| `orc run TASK-ID` | Execute a task |
| `orc status` | Show running tasks |
| `orc pause TASK-ID` | Pause execution |
| `orc resume TASK-ID` | Resume execution |

### Project Configuration

| Setting | Value |
|---------|-------|
| Language | Go |
| Test Command | `go test ./... -v -race` |
| Lint Command | `golangci-lint run` |
| Profile | auto |

### Available Scripts

| Script | Description |
|--------|-------------|
| `.claude/scripts/python-code-quality` | Run linters and type checks |

### Task Guidelines

When working on orc tasks:
- Run tests after implementation
- Ensure no lint errors
- Update documentation if adding new features
<!-- orc:end -->
```

---

## Skill Installation

### Skill Sources

1. **Built-in skills** (bundled with orc)
2. **Anthropic skills library** (from anthropic-skills repo)
3. **Custom skills** (local `.claude/skills/`)

### Installation Process

```go
func installSkills(projectInfo *ProjectInfo) error {
    recommended := getRecommendedSkills(projectInfo.Language)

    for _, skill := range recommended {
        // Download from source
        content := fetchSkill(skill.Source, skill.Name)

        // Save to project
        skillPath := fmt.Sprintf(".claude/skills/%s/SKILL.md", skill.Name)
        writeFile(skillPath, content)
    }

    return nil
}
```

### Recommended Skills by Language

| Language | Skills |
|----------|--------|
| Go | go-testing, go-modules, go-errors |
| Python | python-testing, python-typing, python-packaging |
| TypeScript | ts-testing, ts-react (if React), ts-node (if Node) |
| Rust | rust-testing, rust-errors, rust-async |

---

## Advanced Mode (Claude Session)

When `--advanced` is used:

```markdown
# Advanced Project Setup

You are helping set up orc for a new project.

## Project Information
- Path: {{PROJECT_PATH}}
- Language: {{LANGUAGE}}
- Frameworks: {{FRAMEWORKS}}

## Your Tasks

1. **Analyze the project structure**
   - Understand the codebase organization
   - Identify testing patterns
   - Find existing CI/CD configuration

2. **Suggest custom prompts**
   Based on the codebase, suggest modifications to default prompts:
   - Implement phase: Any project-specific patterns?
   - Test phase: Specific testing requirements?
   - Review phase: Code style preferences?

3. **Configure automation**
   - Which phases need human review?
   - Any custom completion criteria?
   - Integration with existing tools?

4. **Set up scripts**
   - Create any helpful analysis scripts
   - Register existing scripts with orc

5. **Generate CLAUDE.md section**
   Create a detailed orc section for CLAUDE.md with:
   - Project-specific conventions
   - Testing requirements
   - Coding standards

When setup is complete, output:
<setup_complete>true</setup_complete>

Include the configuration you recommend.
```

---

## CLI Flags

```bash
orc init [flags]

Flags:
  --quick, -q         Non-interactive, use all defaults
  --advanced, -a      Open Claude session for advanced setup
  --force, -f         Overwrite existing configuration
  --profile <name>    Set profile (auto, safe, strict)
  --no-skills         Skip skill installation
  --no-claudemd       Don't modify CLAUDE.md
```

---

## Configuration Output

### .orc/config.yaml

```yaml
version: 1
profile: auto

# Auto-detected
project:
  language: go
  frameworks: [gin, cobra]
  test_command: go test ./... -v -race
  lint_command: golangci-lint run

# User selections
completion:
  action: pr
  target_branch: main
  pr:
    title: '[orc] {{TASK_TITLE}}'
    auto_merge: true

model: claude-opus-4-5-20251101

# Defaults
gates:
  default_type: auto
retry:
  enabled: true
  max_retries: 2
worktree:
  enabled: true
```

---

## Implementation Checklist

- [ ] Project detection (language, frameworks)
- [ ] Interactive prompts with arrow key navigation
- [ ] Quick mode with sensible defaults
- [ ] Advanced mode with Claude session
- [ ] Skill recommendation and installation
- [ ] CLAUDE.md section generation (idempotent)
- [ ] Config file generation
- [ ] Global registry registration
- [ ] Post-init guidance

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for wizard and detection code
- 100% coverage for project detection logic

### Unit Tests

| Test | Description |
|------|-------------|
| `TestDetectGoProject` | go.mod parsing, version extraction |
| `TestDetectPythonProject` | pyproject.toml parsing |
| `TestDetectNodeProject` | package.json parsing, TypeScript detection |
| `TestDetectRustProject` | Cargo.toml parsing |
| `TestFrameworkDetectionGo` | Gin, Cobra, Echo detection from go.mod |
| `TestFrameworkDetectionJS` | React, Next.js, Svelte from package.json |
| `TestFrameworkDetectionPython` | FastAPI, Django from requirements |
| `TestCLAUDEMDSectionIdempotent` | Regeneration doesn't duplicate markers |
| `TestSkillRecommendationByLanguage` | Correct skills per language |
| `TestConfigYAMLGeneration` | Valid YAML output with all fields |
| `TestProfileValidation` | Only auto/safe/strict/custom allowed |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestOrcInitCreatesFiles` | `orc init` creates .orc/config.yaml, .orc/tasks/ |
| `TestOrcInitQuickMode` | `--quick` completes without prompts |
| `TestOrcInitForceMode` | `--force` overwrites existing config |
| `TestSkillInstallation` | Skills written to `.claude/skills/` |
| `TestGlobalRegistryUpdate` | Project registered in ~/.orc/projects.yaml |
| `TestCLAUDEMDModification` | CLAUDE.md updated with orc section |
| `TestAdvancedModeSpawnsSession` | `--advanced` starts Claude session |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_project_appears_in_dropdown` | `browser_navigate`, `browser_snapshot` | After init, project in header dropdown |
| `test_config_page_shows_detection` | `browser_navigate`, `browser_snapshot` | Settings shows detected project info |
| `test_reinit_warns_existing` | `browser_snapshot` | Re-init shows warning |

### CLI E2E Tests (subprocess)

| Test | Description |
|------|-------------|
| `test_orc_init_interactive` | Run with simulated input, verify prompts |
| `test_orc_init_quick` | Verify fast completion, correct defaults |
| `test_orc_init_advanced` | Verify Claude session starts |
| `test_exit_code_success` | Exit 0 on success |
| `test_exit_code_failure` | Non-zero on failure |

### Test Fixtures
- Sample go.mod, package.json, pyproject.toml, Cargo.toml
- Mock project directories for detection
- Sample CLAUDE.md for modification testing

---

## Success Criteria

- [ ] New users can run `orc init` and get working config
- [ ] Project type is correctly detected for Go, Python, Node, Rust
- [ ] Recommended skills are installed
- [ ] CLAUDE.md section can be regenerated without conflicts
- [ ] Advanced mode produces high-quality custom configuration
- [ ] Quick mode takes <1 second
- [ ] All selections have clear descriptions
- [ ] 80%+ test coverage on wizard/detection code
- [ ] All E2E tests pass
