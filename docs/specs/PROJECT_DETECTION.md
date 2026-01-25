# Project Detection

**Status**: Implemented
**Priority**: P1
**Last Updated**: 2026-01-12

---

## Problem Statement

Current `orc init`:
- Creates generic configuration
- No project-aware defaults
- No automatic skill/plugin installation
- Prompts aren't optimized for project type

---

## Solution: Smart Project Detection

Automatically detect project type and configure orc accordingly:
- Language and framework detection
- Suggest and install relevant skills
- Configure test/lint commands
- Set up project-specific prompts

---

## Detection Hierarchy

### 1. Language Detection

| File | Language | Confidence |
|------|----------|------------|
| `go.mod` | Go | High |
| `package.json` | JavaScript/TypeScript | High |
| `pyproject.toml` | Python | High |
| `Cargo.toml` | Rust | High |
| `pom.xml` | Java (Maven) | High |
| `build.gradle` | Java (Gradle) | High |
| `*.csproj` | C# | High |
| `mix.exs` | Elixir | High |
| `Gemfile` | Ruby | High |
| `composer.json` | PHP | High |

### 2. Framework Detection

#### Go
| Indicator | Framework |
|-----------|-----------|
| `gin-gonic/gin` in go.mod | Gin HTTP |
| `gorilla/mux` in go.mod | Gorilla Mux |
| `spf13/cobra` in go.mod | Cobra CLI |
| `labstack/echo` in go.mod | Echo |
| `gofiber/fiber` in go.mod | Fiber |
| `ent` in go.mod | Ent ORM |
| `gorm` in go.mod | GORM ORM |

#### JavaScript/TypeScript
| Indicator | Framework |
|-----------|-----------|
| `next` in package.json | Next.js |
| `react` in package.json | React |
| `vue` in package.json | Vue.js |
| `svelte` in package.json | Svelte |
| `express` in package.json | Express |
| `nestjs` in package.json | NestJS |
| `vite` in package.json | Vite |
| `typescript` in devDeps | TypeScript |

#### Python
| Indicator | Framework |
|-----------|-----------|
| `fastapi` in pyproject.toml | FastAPI |
| `django` in requirements | Django |
| `flask` in requirements | Flask |
| `sqlalchemy` in requirements | SQLAlchemy |
| `pytest` in dev-dependencies | pytest |

### 3. Frontend Detection

Orc auto-detects frontend projects to enable Playwright testing recommendations.

| Signal | Detection Method |
|--------|------------------|
| Frontend frameworks | React, Vue, Svelte, Angular, Next.js in dependencies |
| Frontend directories | `web/`, `frontend/`, `client/`, `src/components/`, `src/pages/` |
| Frontend files | `*.tsx`, `*.jsx`, `*.vue`, `*.svelte` in detected directories |

The `has_frontend` field in `Detection` is set to `true` if any of these signals are detected.

**Effect on `orc init`:**

When `has_frontend: true`, the init output recommends installing the Playwright plugin:

```
Claude Code plugins (run once in Claude Code):
  /plugin marketplace add randalmurphal/orc-claude-plugin
  /plugin install orc@orc
  /plugin install playwright@claude-plugins-official  # Frontend detected
```

**Effect on task creation:**

When a task is created with UI-related keywords and the project `has_frontend: true`:
- `requires_ui_testing` is set to `true`
- `testing_requirements.e2e` is set to `true`

---

### 4. Tool Detection

| Indicator | Tool |
|-----------|------|
| `.github/workflows/` exists | GitHub Actions |
| `.gitlab-ci.yml` exists | GitLab CI |
| `Dockerfile` exists | Docker |
| `.devcontainer/` exists | Dev Containers |
| `Makefile` exists | Make |
| `.pre-commit-config.yaml` exists | pre-commit |

---

## Detection Output

```go
type ProjectInfo struct {
    // Language
    Language     string   `yaml:"language"`      // "go", "typescript", "python"
    LanguageVer  string   `yaml:"language_ver"`  // "1.22", "3.12"

    // Frameworks
    Frameworks   []string `yaml:"frameworks"`    // ["gin", "cobra"]

    // Build/Test
    TestCommand  string   `yaml:"test_command"`  // "go test ./..."
    LintCommand  string   `yaml:"lint_command"`  // "golangci-lint run"
    BuildCommand string   `yaml:"build_command"` // "go build ./..."

    // Tools
    HasDocker    bool     `yaml:"has_docker"`
    HasCI        bool     `yaml:"has_ci"`
    CIProvider   string   `yaml:"ci_provider"`   // "github", "gitlab"
    HasTests     bool     `yaml:"has_tests"`
    HasFrontend  bool     `yaml:"has_frontend"`  // Frontend project detected

    // Monorepo
    IsMonorepo   bool     `yaml:"is_monorepo"`
    Workspaces   []string `yaml:"workspaces"`    // ["packages/*"]

    // Confidence
    Confidence   float64  `yaml:"confidence"`    // 0.0-1.0

    // Suggested skills
    SuggestedSkills []string `yaml:"suggested_skills,omitempty"`
}
```

---

## Configuration Effects

### Commands

| Language | Test | Lint | Build |
|----------|------|------|-------|
| Go | `go test ./... -v -race` | `golangci-lint run` | `go build ./...` |
| TypeScript | `bun test` | `bun run lint` | `bun run build` |
| Python | `pytest` | `ruff check .` | N/A |
| Rust | `cargo test` | `cargo clippy` | `cargo build` |

### Prompts

Project type affects default prompts:

```markdown
<!-- templates/prompts/implement.md for Go -->
## Implementation Guidelines

- Use Go idioms and patterns
- Run tests: `go test ./... -v -race`
- Check for race conditions
- Lint with: `golangci-lint run`
- Document exported functions
```

```markdown
<!-- templates/prompts/implement.md for TypeScript -->
## Implementation Guidelines

- Use TypeScript strict mode patterns
- Run tests: `bun test`
- Ensure type safety (no `any` without justification)
- Lint with: `bun run lint`
- Follow ESLint rules
```

---

## Skill Recommendations

### By Language

| Language | Recommended Skills |
|----------|-------------------|
| Go | `go-testing`, `go-errors`, `go-concurrency` |
| TypeScript | `ts-types`, `ts-react` (if React), `ts-testing` |
| Python | `python-typing`, `python-testing`, `python-async` |
| Rust | `rust-ownership`, `rust-async`, `rust-testing` |

### By Framework

| Framework | Additional Skills |
|-----------|------------------|
| Gin | `gin-middleware`, `gin-testing` |
| Next.js | `nextjs-routing`, `nextjs-data` |
| FastAPI | `fastapi-deps`, `fastapi-testing` |
| React | `react-hooks`, `react-testing` |

---

## Plugin/MCP Server Recommendations

Based on project type, suggest relevant MCP servers:

| Scenario | MCP Server |
|----------|------------|
| Has Playwright | `playwright-mcp` for E2E testing |
| Has database | `sqlite-mcp` or relevant DB server |
| Has API | `http-mcp` for API testing |
| Frontend project | `playwright-mcp` for browser testing |

---

## Implementation

### Detection Function

```go
func DetectProject(path string) (*ProjectInfo, error) {
    info := &ProjectInfo{
        Confidence: 0.0,
    }

    // Language detection
    if exists(path, "go.mod") {
        info.Language = "go"
        info.LanguageVer = parseGoVersion(path)
        info.TestCommand = "go test ./... -v -race"
        info.LintCommand = "golangci-lint run"
        info.BuildCommand = "go build ./..."
        info.Confidence = 0.9

        // Framework detection
        goMod := readFile(path, "go.mod")
        info.Frameworks = detectGoFrameworks(goMod)
    } else if exists(path, "package.json") {
        info.Language = "typescript" // or javascript
        pkg := parsePackageJSON(path)
        info.Frameworks = detectJSFrameworks(pkg)
        info.TestCommand = getScript(pkg, "test", "bun test")
        info.LintCommand = getScript(pkg, "lint", "bun run lint")
        info.Confidence = 0.9
    }
    // ... other languages

    // Tool detection
    info.HasDocker = exists(path, "Dockerfile")
    info.HasCI = exists(path, ".github/workflows") || exists(path, ".gitlab-ci.yml")
    if exists(path, ".github/workflows") {
        info.CIProvider = "github"
    }

    // Monorepo detection
    if exists(path, "pnpm-workspace.yaml") || exists(path, "lerna.json") {
        info.IsMonorepo = true
        info.Workspaces = detectWorkspaces(path)
    }

    return info, nil
}
```

### Skill Installation

```go
func InstallRecommendedSkills(info *ProjectInfo) error {
    skills := getRecommendedSkills(info.Language, info.Frameworks)

    for _, skill := range skills {
        // Check if already installed
        if skillExists(skill.Name) {
            continue
        }

        // Fetch skill content
        content, err := fetchSkill(skill.Source)
        if err != nil {
            log.Warn("Failed to fetch skill", "skill", skill.Name, "error", err)
            continue
        }

        // Install to .claude/skills/
        if err := installSkill(skill.Name, content); err != nil {
            return fmt.Errorf("install skill %s: %w", skill.Name, err)
        }

        log.Info("Installed skill", "skill", skill.Name)
    }

    return nil
}
```

---

## User Experience

### During Init

```bash
$ orc init

Detecting project type...

┌─ Project Detection ─────────────────────────────────────────┐
│                                                             │
│  Language:   Go 1.22                                        │
│  Frameworks: Gin, Cobra                                     │
│  Tools:      Docker, GitHub Actions                         │
│                                                             │
│  Detected commands:                                         │
│    Test:  go test ./... -v -race                            │
│    Lint:  golangci-lint run                                 │
│    Build: go build ./...                                    │
│                                                             │
│  Recommended skills:                                        │
│    ✓ go-testing    Go test patterns                         │
│    ✓ go-errors     Error handling patterns                  │
│    ✓ gin-testing   Gin HTTP testing                         │
│                                                             │
│  ? Install recommended skills? [Y/n]                        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Override Detection

```bash
# If detection is wrong
$ orc init --language python --framework fastapi

# Skip detection entirely
$ orc init --no-detect
```

---

## Config Storage

Detection results stored in config:

```yaml
# .orc/config.yaml
version: 1

project:
  language: go
  language_version: "1.22"
  frameworks:
    - gin
    - cobra
  test_command: go test ./... -v -race
  lint_command: golangci-lint run
  build_command: go build ./...
  detected_at: 2026-01-10T14:30:00Z
  confidence: 0.9

# User can override
project_overrides:
  test_command: make test  # Override detected command
```

---

## Skill Sources

### Built-in Skills

Bundled with orc binary, always available.

### Anthropic Skills Library

Official skills from Anthropic:
- `https://github.com/anthropics/claude-code-skills`
- Fetched on-demand during init

### Custom Skill Repositories

User can configure additional sources:

```yaml
# ~/.orc/config.yaml
skill_sources:
  - name: company-skills
    url: https://github.com/company/claude-skills
  - name: local
    path: ~/my-skills/
```

---

## API Endpoints

### Detect Project

```
POST /api/projects/:id/detect

Response:
{
  "language": "go",
  "language_version": "1.22",
  "frameworks": ["gin", "cobra"],
  "test_command": "go test ./... -v -race",
  "lint_command": "golangci-lint run",
  "confidence": 0.9,
  "recommended_skills": [
    {
      "name": "go-testing",
      "description": "Go test patterns and table-driven tests",
      "installed": false
    }
  ]
}
```

### Install Skills

```
POST /api/skills/install
{
  "skills": ["go-testing", "go-errors"]
}

Response:
{
  "installed": ["go-testing", "go-errors"],
  "failed": []
}
```

---

## Testing Requirements

### Coverage Target
- 80%+ line coverage for detection code
- 100% coverage for language/framework detection

### Unit Tests

| Test | Description |
|------|-------------|
| `TestDetectLanguage_Go` | go.mod present |
| `TestDetectLanguage_TypeScript` | package.json with TypeScript |
| `TestDetectLanguage_Python` | pyproject.toml present |
| `TestDetectLanguage_Rust` | Cargo.toml present |
| `TestDetectLanguage_NoMatch` | Empty directory handling |
| `TestDetectGoFramework_Gin` | gin in go.mod |
| `TestDetectGoFramework_Cobra` | cobra in go.mod |
| `TestDetectJSFramework_React` | react in package.json |
| `TestDetectJSFramework_Next` | next in package.json |
| `TestDetectJSFramework_Svelte` | svelte in package.json |
| `TestDetectPythonFramework_FastAPI` | fastapi in pyproject.toml |
| `TestDetectPythonFramework_Django` | django in requirements |
| `TestDetectTool_Docker` | Dockerfile exists |
| `TestDetectTool_GitHubActions` | .github/workflows/ exists |
| `TestDetectTool_GitLabCI` | .gitlab-ci.yml exists |
| `TestDetectMonorepo_PNPM` | pnpm-workspace.yaml |
| `TestDetectMonorepo_Lerna` | lerna.json |
| `TestParseGoVersion` | Extracts version from go.mod |
| `TestConfidenceScore` | High when multiple signals align |

### Integration Tests

| Test | Description |
|------|-------------|
| `TestCLIInitDetection` | `orc init` outputs detection |
| `TestCLIInitOverride` | `orc init --language python` |
| `TestCLIInitNoDetect` | `orc init --no-detect` |
| `TestDetectionSavedToConfig` | .orc/config.yaml has project section |
| `TestSkillRecommendations` | Suggests relevant skills |
| `TestSkillInstallation` | Skills installed when requested |
| `TestAPIDetectProject` | `POST /api/projects/:id/detect` |
| `TestAPIInstallSkills` | `POST /api/skills/install` |

### E2E Tests (Playwright MCP)

| Test | Tools | Description |
|------|-------|-------------|
| `test_project_info_in_settings` | `browser_navigate`, `browser_snapshot` | Settings shows project info |
| `test_detected_commands_shown` | `browser_snapshot` | Commands shown in config |
| `test_skill_recommendations` | `browser_snapshot` | Recommended skills visible |
| `test_override_detected_language` | `browser_click`, `browser_type` | Can change detected language |
| `test_redetect_button` | `browser_click` | Re-detect updates info |

### Test Fixtures
- Sample go.mod with various frameworks
- Sample package.json with various frameworks
- Sample pyproject.toml with various frameworks
- Sample Cargo.toml
- Monorepo structures

---

## Success Criteria

- [ ] Detect Go, TypeScript, Python, Rust accurately
- [ ] Framework detection works for major frameworks
- [ ] Commands are correctly inferred
- [ ] Skills are recommended based on detection
- [ ] User can override detection
- [ ] Detection runs in <1 second
- [ ] Results stored in config for reference
- [ ] Works with monorepos
- [ ] 80%+ test coverage on detection code
- [ ] All E2E tests pass
