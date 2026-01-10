# Orc Project - Ground-Up Build with flowgraph + llmkit

## Goal

Build the **entire** Orc orchestrator from scratch using:
- **flowgraph** (`~/repos/flowgraph`) - Graph-based execution engine
- **llmkit** (`~/repos/llmkit`) - Claude CLI wrapper, tokens, templates, parsing

Every component must be fully implemented, tested, and validated with Playwright MCP for E2E.

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                              ORC CLI                                 │
│  orc init | new | run | pause | stop | rewind | approve | ...       │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         TASK MANAGER                                 │
│  Load/Save tasks, plans, state to .orc/tasks/TASK-ID/*.yaml         │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    PHASE EXECUTOR (flowgraph)                        │
│  Each phase = CompiledGraph[PhaseState]                             │
│  Gates = Conditional edges with RouterFunc                          │
│  Checkpointing = flowgraph checkpoint.Store + git commits           │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      LLM LAYER (llmkit)                              │
│  claude.Client | model.CostTracker | template.Engine                │
│  strings.Contains for <phase_complete> detection                    │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        GIT LAYER (keep existing)                     │
│  Branches, checkpoints, worktrees                                   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## What to Keep

### Keep As-Is
- `internal/git/checkpoint.go` - Git operations (210 lines, good quality)
- `templates/plans/*.yaml` - Phase templates (5 files)
- `templates/prompts/*.md` - Prompt templates (7 files)
- `docs/` - All documentation

### Delete / Replace
- `internal/executor/` - Replace with flowgraph integration
- `internal/claude/` - Delete, use llmkit
- `internal/cli/commands.go` - Rewrite with real implementations

### Keep Types, Add Persistence
- `internal/task/task.go` - Keep Weight, Status, Task types; add Load/Save
- `internal/plan/plan.go` - Keep Phase, Plan types; add Load/Save

---

## Implementation Plan

### Phase 1: Dependencies and Structure

**Option A: Native Development**
```bash
cd /home/randy/repos/orc
make setup    # Configures go.mod with local dependencies (already done)
make build    # Builds bin/orc
make test     # Runs tests
```

**Option B: Container Development**
```bash
cd /home/randy/repos/orc
make dev      # Interactive shell with all tools
# Inside container:
go build -o bin/orc ./cmd/orc
go test ./...
```

**go.mod is pre-configured with:**
```go
require (
    github.com/randalmurphal/flowgraph v0.0.0
    github.com/randalmurphal/llmkit v0.0.0
)

replace github.com/randalmurphal/llmkit => ../llmkit
replace github.com/randalmurphal/flowgraph => ../flowgraph
```

**Required Environment (Native):**
- Go 1.23+
- Claude CLI installed and authenticated (`claude --version`)
- Git 2.30+

**Required Environment (Container):**
- Docker/nerdctl/podman with compose support
- Claude CLI on host (for actual task execution)

Create directory structure:
```
internal/
├── cli/           # Cobra commands (rewrite)
├── task/          # Task types + YAML persistence
├── plan/          # Plan types + YAML persistence
├── state/         # Execution state tracking
├── executor/      # flowgraph-based phase execution (NEW)
├── gate/          # Gate evaluation (auto/ai/human) (NEW)
├── git/           # Git operations (KEEP)
├── prompt/        # Prompt template loading (NEW)
├── progress/      # Progress display for user feedback (NEW)
└── transcript/    # Transcript saving (NEW)
```

### Phase 2: Core Types with Persistence

#### `internal/task/task.go`
```go
package task

import (
    "os"
    "path/filepath"
    "gopkg.in/yaml.v3"
)

// Keep existing types: Weight, Status, Task

// Add persistence
func Load(id string) (*Task, error) {
    path := filepath.Join(".orc", "tasks", id, "task.yaml")
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var t Task
    return &t, yaml.Unmarshal(data, &t)
}

func (t *Task) Save() error {
    dir := filepath.Join(".orc", "tasks", t.ID)
    os.MkdirAll(dir, 0755)
    data, _ := yaml.Marshal(t)
    return os.WriteFile(filepath.Join(dir, "task.yaml"), data, 0644)
}

func LoadAll() ([]*Task, error) // Scan .orc/tasks/*/task.yaml
func NextID() (string, error)   // TASK-001, TASK-002, ...
```

#### `internal/plan/plan.go`
```go
package plan

// Keep existing types: Phase, Plan, Gate, GateType

// Add persistence
func Load(taskID string) (*Plan, error)
func (p *Plan) Save(taskID string) error

// Add template loading from disk
func LoadTemplate(weight task.Weight) (*Plan, error) {
    path := fmt.Sprintf("templates/plans/%s.yaml", weight)
    // Parse and return
}
```

#### `internal/state/state.go` (NEW)
```go
package state

type State struct {
    TaskID           string                 `yaml:"task_id"`
    CurrentPhase     string                 `yaml:"current_phase"`
    CurrentIteration int                    `yaml:"current_iteration"`
    Status           string                 `yaml:"status"`
    StartedAt        time.Time              `yaml:"started_at"`
    Phases           map[string]*PhaseState `yaml:"phases"`
    Gates            []GateDecision         `yaml:"gates"`
    Tokens           TokenUsage             `yaml:"tokens"`
}

type PhaseState struct {
    Status      string    `yaml:"status"`
    StartedAt   time.Time `yaml:"started_at"`
    CompletedAt time.Time `yaml:"completed_at,omitempty"`
    Iterations  int       `yaml:"iterations"`
    CommitSHA   string    `yaml:"commit_sha,omitempty"`
    Artifacts   []string  `yaml:"artifacts,omitempty"`
}

func Load(taskID string) (*State, error)
func (s *State) Save() error
```

### Phase 3: Executor with flowgraph + llmkit

#### `internal/executor/executor.go` (NEW - flowgraph based)
```go
package executor

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/randalmurphal/flowgraph/pkg/flowgraph"
    "github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
    "github.com/randalmurphal/llmkit/claude"
    "github.com/randalmurphal/llmkit/model"
    "github.com/randalmurphal/llmkit/template"
)

// PhaseState holds state during phase execution
type PhaseState struct {
    // Task context
    TaskID    string
    TaskTitle string
    Phase     string
    Weight    string

    // Execution state
    Iteration  int
    Prompt     string   // Rendered prompt sent to Claude
    Response   string   // Claude's response
    Complete   bool     // Phase completion detected
    Blocked    bool     // Phase blocked (needs clarification)
    Error      error    // Any error during execution
    Artifacts  []string // Files created/modified
    CommitSHA  string   // Git commit after phase completion
    TokensUsed int      // Total tokens used in this phase

    // Prior phase content (for template rendering)
    ResearchContent string
    SpecContent     string
    DesignContent   string
}

// Executor runs phases using flowgraph
type Executor struct {
    client          claude.Client
    selector        *model.Selector
    templates       *template.Engine
    costTracker     *model.CostTracker
    gitOps          *git.Git
    checkpointStore checkpoint.Store
    logger          *slog.Logger
    transcriptDir   string  // .orc/tasks/{id}/transcripts
}

func New(cfg *Config) (*Executor, error) {
    // Initialize llmkit components
    client := claude.NewClient(claude.WithModel("claude-opus-4-5-20251101"))
    selector := model.NewSelector(
        model.WithThinkingModel(model.ModelOpus),
        model.WithDefaultModel(model.ModelOpus),  // User wants Opus everywhere
        model.WithFastModel(model.ModelOpus),
    )
    // ...
}

// ExecutePhase runs a single phase as a flowgraph
func (e *Executor) ExecutePhase(ctx context.Context, t *task.Task, p *plan.Phase) error {
    // Build phase graph
    graph := flowgraph.NewGraph[PhaseState]()

    // Nodes
    graph.AddNode("prompt", e.buildPromptNode(p))
    graph.AddNode("execute", e.executeClaudeNode())
    graph.AddNode("check", e.checkCompletionNode(p))
    graph.AddNode("commit", e.commitCheckpointNode())

    // Edges - Ralph-style loop
    graph.SetEntry("prompt")  // Entry point (no START constant)
    graph.AddEdge("prompt", "execute")
    graph.AddEdge("execute", "check")
    graph.AddConditionalEdge("check", func(ctx flowgraph.Context, s PhaseState) string {
        if s.Complete {
            return "commit"
        }
        if s.Iteration >= p.MaxIterations {
            return flowgraph.END  // Max iterations reached
        }
        return "prompt"  // Loop back for another iteration
    })
    graph.AddEdge("commit", flowgraph.END)

    // Compile and run with checkpointing
    compiled, err := graph.Compile()
    if err != nil {
        return fmt.Errorf("compile phase graph: %w", err)
    }

    // Create flowgraph context with LLM client
    fgCtx := flowgraph.NewContext(ctx,
        flowgraph.WithLLM(e.client),
        flowgraph.WithLogger(e.logger),
        flowgraph.WithContextRunID(fmt.Sprintf("%s-%s", t.ID, p.Name)),
    )

    initialState := PhaseState{
        TaskID:    t.ID,
        TaskTitle: t.Title,
        Phase:     p.Name,
        Iteration: 0,
    }

    _, err = compiled.Run(fgCtx, initialState,
        flowgraph.WithCheckpointing(e.checkpointStore),
        flowgraph.WithRunID(fmt.Sprintf("%s-%s", t.ID, p.Name)),
        flowgraph.WithMaxIterations(p.MaxIterations + 10), // Buffer for nodes per iteration
    )

    return err
}

func (e *Executor) buildPromptNode(p *plan.Phase) flowgraph.NodeFunc[PhaseState] {
    return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
        // Load template from templates/prompts/{phase}.md
        templatePath := filepath.Join("templates", "prompts", p.Name+".md")
        tmplContent, err := os.ReadFile(templatePath)
        if err != nil {
            return s, fmt.Errorf("read template %s: %w", templatePath, err)
        }

        // Render with task context using template.Engine
        rendered, err := e.templates.Render(string(tmplContent), map[string]any{
            "TASK_ID":    s.TaskID,
            "TASK_TITLE": s.TaskTitle,
            "PHASE":      s.Phase,
            "WEIGHT":     s.Weight,
            // Additional context loaded from artifacts
            "RESEARCH_CONTENT": s.ResearchContent,
            "SPEC_CONTENT":     s.SpecContent,
        })
        if err != nil {
            return s, fmt.Errorf("render template: %w", err)
        }

        s.Prompt = rendered
        s.Iteration++
        return s, nil
    }
}

func (e *Executor) executeClaudeNode() flowgraph.NodeFunc[PhaseState] {
    return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
        // Use LLM client from context (set in fgCtx), or fallback to executor's client
        client := ctx.LLM()
        if client == nil {
            client = e.client
        }

        resp, err := client.Complete(ctx, claude.CompletionRequest{
            Messages: []claude.Message{{Role: claude.RoleUser, Content: s.Prompt}},
            Model:    "claude-opus-4-5-20251101",
        })
        if err != nil {
            s.Error = err
            return s, fmt.Errorf("claude completion: %w", err)
        }

        s.Response = resp.Content
        s.TokensUsed += resp.Usage.TotalTokens

        // Track costs - need to map string model to ModelName
        modelName := model.ModelOpus // We know we're using Opus
        e.costTracker.Record(modelName, resp.Usage.InputTokens, resp.Usage.OutputTokens)

        return s, nil
    }
}

func (e *Executor) checkCompletionNode(p *plan.Phase) flowgraph.NodeFunc[PhaseState] {
    return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
        // Detect completion marker in response
        s.Complete = strings.Contains(s.Response, "<phase_complete>true</phase_complete>")

        // Also check for blocked state
        if strings.Contains(s.Response, "<phase_blocked>") {
            s.Blocked = true
            // Extract blocking reason (could use llmkit parser for this)
        }

        // Save transcript for this iteration
        if err := e.saveTranscript(s); err != nil {
            ctx.Logger().Warn("failed to save transcript", "error", err)
        }

        return s, nil
    }
}

func (e *Executor) commitCheckpointNode() flowgraph.NodeFunc[PhaseState] {
    return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
        // Git commit checkpoint
        msg := fmt.Sprintf("[orc] %s: %s - completed\n\nPhase: %s\nStatus: completed",
            s.TaskID, s.Phase, s.Phase)
        sha, err := e.gitOps.CreateCheckpoint(s.TaskID, s.Phase, msg)
        s.CommitSHA = sha
        return s, err
    }
}
```

### Phase 4: Gate Evaluation

#### `internal/gate/gate.go` (NEW)
```go
package gate

type Evaluator struct {
    client claude.Client
}

// Evaluate determines if a gate passes
func (e *Evaluator) Evaluate(ctx context.Context, gate *plan.Gate, phaseOutput string) (Decision, error) {
    switch gate.Type {
    case plan.GateAuto:
        return e.evaluateAuto(gate, phaseOutput)
    case plan.GateAI:
        return e.evaluateAI(ctx, gate, phaseOutput)
    case plan.GateHuman:
        return e.requestHumanApproval(gate)
    }
}

type Decision struct {
    Approved  bool
    Reason    string
    Questions []string  // For NEEDS_CLARIFICATION
}
```

### Phase 5: CLI Commands

#### `internal/cli/commands.go` (REWRITE)
```go
// Each command fully implemented using:
// - task.Load/Save for persistence
// - executor.ExecutePhase for running
// - gate.Evaluate for approvals
// - git.CreateCheckpoint for commits

func newInitCmd() *cobra.Command {
    return &cobra.Command{
        Use: "init",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Create .orc/ directory
            os.MkdirAll(".orc/tasks", 0755)

            // Write default orc.yaml
            cfg := config.Default()
            return cfg.Save("orc.yaml")
        },
    }
}

func newNewCmd() *cobra.Command {
    return &cobra.Command{
        Use: "new <title>",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Generate ID
            id, _ := task.NextID()

            // Create task
            t := task.New(id, args[0])

            // Classify weight (or use --weight flag)
            weight, _ := cmd.Flags().GetString("weight")
            if weight == "" {
                // Run classify phase
                weight = classifier.Classify(t)
            }
            t.Weight = task.Weight(weight)

            // Generate plan from template
            p, _ := plan.LoadTemplate(t.Weight)

            // Create git branch
            gitOps.CreateBranch(t.ID)

            // Save
            t.Save()
            p.Save(t.ID)

            fmt.Printf("Created %s: %s (weight: %s)\n", t.ID, t.Title, t.Weight)
            return nil
        },
    }
}

func newRunCmd() *cobra.Command {
    return &cobra.Command{
        Use: "run <task-id>",
        RunE: func(cmd *cobra.Command, args []string) error {
            t, _ := task.Load(args[0])
            p, _ := plan.Load(t.ID)
            s, _ := state.Load(t.ID)

            exec := executor.New(config.Load())

            for _, phase := range p.Phases {
                if s.Phases[phase.Name].Status == "completed" {
                    continue  // Skip completed phases
                }

                // Execute phase
                err := exec.ExecutePhase(ctx, t, &phase)
                if err != nil {
                    return err
                }

                // Evaluate gate
                decision, _ := gate.Evaluate(ctx, &phase.Gate, /* output */)
                if !decision.Approved {
                    // Handle rejection
                }

                // Update state
                s.Phases[phase.Name].Status = "completed"
                s.Save()
            }

            return nil
        },
    }
}
// ... remaining commands
```

### Phase 6: Documentation Phase

The docs phase runs AFTER implementation and review, when the agent has full context of what changed:

```go
// internal/executor/docs_phase.go

// DocsPhase handles documentation creation/update
func (e *Executor) executeDocsPhase(ctx context.Context, t *task.Task, p *plan.Phase) error {
    // Docs phase prompt receives:
    // - Full implementation summary
    // - Files changed list
    // - Current documentation status

    vars := map[string]any{
        "TASK_ID":                t.ID,
        "TASK_TITLE":             t.Title,
        "WEIGHT":                 t.Weight,
        "PROJECT_TYPE":           detectProjectType(),
        "IMPLEMENTATION_SUMMARY": getImplementationSummary(t.ID),
        "FILES_CHANGED":          getFilesChanged(t.ID),
        "DOC_STATUS":             auditCurrentDocs(),
    }

    return e.ExecutePhase(ctx, t, p, vars)
}

// detectProjectType determines doc requirements
func detectProjectType() string {
    // Check for indicators:
    // - go.work or multiple go.mod → "monorepo"
    // - No existing docs → "undocumented"
    // - Single README → "single_package"
    // - Empty .orc/tasks → "greenfield"
}
```

**Documentation Phase Workflow:**
1. **Audit** - Check what docs exist vs what's needed
2. **Create** - Create missing required docs (CLAUDE.md, README.md)
3. **Update** - Update existing docs affected by changes
4. **Validate** - Ensure all code blocks work, links resolve

**CLAUDE.md Requirements:**
- Under 200 lines (concise for AI agents)
- Required sections: Quick Start, Structure, Commands, Patterns
- Use tables over prose
- Actual working commands (not placeholders)

See `docs/specs/DOCUMENTATION.md` for full specification.

### Phase 7: Prompt Template Loading

#### `internal/prompt/loader.go` (NEW)
```go
package prompt

import (
    "github.com/randalmurphal/llmkit/template"
)

type Loader struct {
    engine *template.Engine
    dir    string  // templates/prompts/
}

func (l *Loader) Load(phase string, vars map[string]any) (string, error) {
    path := filepath.Join(l.dir, phase+".md")
    tmpl, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    return l.engine.Render(string(tmpl), vars)
}
```

### Phase 8: Transcript Saving

#### `internal/transcript/transcript.go` (NEW)
```go
package transcript

type Transcript struct {
    TaskID    string
    Phase     string
    Iteration int
    Timestamp time.Time
    Duration  time.Duration
    Tokens    int
    Prompt    string
    Response  string
    Status    string
}

func (t *Transcript) Save() error {
    // Save to .orc/tasks/{id}/transcripts/PP-phase-III.md
    filename := fmt.Sprintf("%02d-%s-%03d.md", t.phaseNum, t.Phase, t.Iteration)
    path := filepath.Join(".orc", "tasks", t.TaskID, "transcripts", filename)
    // Write markdown format
}
```

### Phase 9: Tests

Create test files for every package:
- `internal/task/task_test.go` - Test Load/Save/NextID
- `internal/plan/plan_test.go` - Test Load/Save/LoadTemplate
- `internal/state/state_test.go` - Test Load/Save
- `internal/executor/executor_test.go` - Test phase execution with flowgraph
- `internal/executor/recovery_test.go` - Test retry logic and error classification
- `internal/gate/gate_test.go` - Test gate evaluation
- `internal/prompt/loader_test.go` - Test template loading
- `internal/progress/display_test.go` - Test progress output
- `internal/transcript/transcript_test.go` - Test saving
- `internal/cli/signals_test.go` - Test interrupt handling

Use llmkit's `claude.MockClient` for testing without real API calls.
Use `flowgraph/checkpoint.NewMemoryStore()` for checkpoint testing.

### Phase 10: Error Recovery & Resilience

#### `internal/executor/recovery.go` (NEW)
```go
package executor

import (
    "context"
    "errors"
    "time"
)

// Sentinel errors for recovery decisions
var (
    ErrRateLimited    = errors.New("rate limited by API")
    ErrNetworkFailure = errors.New("network failure")
    ErrTimeout        = errors.New("execution timeout")
    ErrMaxRetries     = errors.New("max retries exceeded")
)

// RetryConfig controls retry behavior
type RetryConfig struct {
    MaxRetries     int           // Max attempts before giving up
    InitialBackoff time.Duration // Starting backoff duration
    MaxBackoff     time.Duration // Maximum backoff duration
    BackoffFactor  float64       // Multiplier for each retry
}

// DefaultRetryConfig returns sensible defaults
func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxRetries:     3,
        InitialBackoff: 2 * time.Second,
        MaxBackoff:     60 * time.Second,
        BackoffFactor:  2.0,
    }
}

// ExecuteWithRetry wraps phase execution with retry logic
func (e *Executor) ExecuteWithRetry(ctx context.Context, t *task.Task, p *plan.Phase) error {
    cfg := DefaultRetryConfig()
    backoff := cfg.InitialBackoff

    for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
        err := e.ExecutePhase(ctx, t, p)
        if err == nil {
            return nil
        }

        // Check if error is retryable
        if !isRetryable(err) {
            return err
        }

        e.logger.Warn("phase execution failed, retrying",
            "phase", p.Name,
            "attempt", attempt,
            "max_attempts", cfg.MaxRetries,
            "backoff", backoff,
            "error", err,
        )

        // Save state before retry (in case of crash during backoff)
        if saveErr := e.saveRecoveryState(t, p, attempt, err); saveErr != nil {
            e.logger.Error("failed to save recovery state", "error", saveErr)
        }

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(backoff):
        }

        // Exponential backoff
        backoff = time.Duration(float64(backoff) * cfg.BackoffFactor)
        if backoff > cfg.MaxBackoff {
            backoff = cfg.MaxBackoff
        }
    }

    return ErrMaxRetries
}

func isRetryable(err error) bool {
    // Network errors, rate limits, and timeouts are retryable
    return errors.Is(err, ErrRateLimited) ||
           errors.Is(err, ErrNetworkFailure) ||
           errors.Is(err, context.DeadlineExceeded) ||
           strings.Contains(err.Error(), "connection refused") ||
           strings.Contains(err.Error(), "rate limit") ||
           strings.Contains(err.Error(), "timeout")
}
```

### Phase 11: Interrupt Handling

#### `internal/cli/signals.go` (NEW)
```go
package cli

import (
    "context"
    "os"
    "os/signal"
    "syscall"
)

// SetupSignalHandler returns a context that is cancelled on SIGINT/SIGTERM
func SetupSignalHandler() (context.Context, context.CancelFunc) {
    ctx, cancel := context.WithCancel(context.Background())

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        sig := <-sigChan
        fmt.Printf("\n⚠️  Received %s, saving state and exiting gracefully...\n", sig)
        cancel()

        // Second signal forces immediate exit
        sig = <-sigChan
        fmt.Printf("\n🛑 Received %s again, forcing exit\n", sig)
        os.Exit(1)
    }()

    return ctx, cancel
}

// GracefulShutdown saves current state before exit
func GracefulShutdown(t *task.Task, s *state.State, phase string) error {
    // Mark phase as interrupted (not failed - can be resumed)
    s.Phases[phase].Status = "interrupted"
    s.Phases[phase].InterruptedAt = time.Now()

    if err := s.Save(); err != nil {
        return fmt.Errorf("save state on interrupt: %w", err)
    }

    fmt.Printf("✅ State saved. Resume with: orc resume %s\n", t.ID)
    return nil
}
```

**Interrupt Behavior:**
- First Ctrl+C: Save state, commit checkpoint, exit gracefully
- Second Ctrl+C: Force exit immediately (may lose in-progress iteration)
- State marked as "interrupted" (distinct from "failed")
- Clear message showing how to resume

### Phase 12: Resume Command

#### `internal/cli/resume.go` (NEW)
```go
func newResumeCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "resume <task-id>",
        Short: "Resume an interrupted or failed task",
        RunE: func(cmd *cobra.Command, args []string) error {
            if len(args) == 0 {
                return fmt.Errorf("task ID required")
            }

            t, err := task.Load(args[0])
            if err != nil {
                return fmt.Errorf("load task: %w", err)
            }

            s, err := state.Load(t.ID)
            if err != nil {
                return fmt.Errorf("load state: %w", err)
            }

            // Find the phase to resume from
            var resumePhase string
            for _, phase := range plan.Phases {
                phaseState := s.Phases[phase.Name]
                if phaseState.Status == "interrupted" || phaseState.Status == "running" {
                    resumePhase = phase.Name
                    break
                }
            }

            if resumePhase == "" {
                return fmt.Errorf("no interrupted phase found to resume")
            }

            fmt.Printf("📂 Resuming %s from phase: %s (iteration %d)\n",
                t.ID, resumePhase, s.Phases[resumePhase].Iterations)

            // Switch to task branch
            if err := git.Checkout(t.ID); err != nil {
                return fmt.Errorf("checkout branch: %w", err)
            }

            exec := executor.New(config.Load())
            return exec.ResumeFromPhase(ctx, t, resumePhase)
        },
    }
}
```

**Resume Workflow:**
1. Load task and state from `.orc/tasks/{id}/`
2. Find first phase with status "interrupted" or "running"
3. Switch to task's git branch
4. Resume execution from that phase (using flowgraph checkpoint if available)
5. Continue normally through remaining phases

### Phase 13: Progress Indication

#### `internal/progress/display.go` (NEW)
```go
package progress

import (
    "fmt"
    "time"
)

// Display shows progress to user
type Display struct {
    taskID    string
    phase     string
    iteration int
    maxIter   int
    startTime time.Time
    tokens    int
}

func (d *Display) Update(iteration int, tokens int) {
    d.iteration = iteration
    d.tokens = tokens

    elapsed := time.Since(d.startTime)

    // Clear line and print status
    fmt.Printf("\r⏳ %s | Phase: %s | Iteration: %d/%d | Tokens: %d | Elapsed: %s",
        d.taskID,
        d.phase,
        iteration,
        d.maxIter,
        tokens,
        elapsed.Round(time.Second),
    )
}

func (d *Display) PhaseStart(phase string, maxIter int) {
    d.phase = phase
    d.maxIter = maxIter
    d.iteration = 0
    d.startTime = time.Now()

    fmt.Printf("\n🚀 Starting phase: %s (max %d iterations)\n", phase, maxIter)
}

func (d *Display) PhaseComplete(phase string, commit string) {
    fmt.Printf("\n✅ Phase %s complete (commit: %s)\n", phase, commit[:7])
}

func (d *Display) PhaseFailed(phase string, err error) {
    fmt.Printf("\n❌ Phase %s failed: %s\n", phase, err)
}

func (d *Display) GatePending(gate string) {
    fmt.Printf("\n⏸️  Waiting for gate: %s\n", gate)
}
```

**Progress Output Example:**
```
🚀 Starting phase: implement (max 10 iterations)
⏳ TASK-001 | Phase: implement | Iteration: 3/10 | Tokens: 12,450 | Elapsed: 2m34s
✅ Phase implement complete (commit: abc1234)
⏸️  Waiting for gate: ai-review
```

---

## Completion Criteria (ALL must be TRUE)

### Build & Lint
- [ ] `go build ./...` succeeds with zero errors
- [ ] `go vet ./...` reports no issues
- [ ] No import errors for llmkit or flowgraph
- [ ] `cd web && npm install && npm run check` passes

### Test Coverage Requirements

**Run: `go test ./... -v -race -cover`**

Each package MUST have tests covering ALL exported functions and edge cases:

#### `internal/task/` (>90% coverage)
| Function | Test Cases Required |
|----------|---------------------|
| `New()` | Valid input, empty title, special characters |
| `Load()` | Exists, not found, corrupted YAML, permissions |
| `Save()` | New task, update existing, directory creation |
| `LoadAll()` | Empty dir, multiple tasks, partial failures |
| `NextID()` | First task, sequential IDs, gaps in sequence |
| `TaskDir()` | Valid ID, special characters |
| `IsTerminal()` | All status values (pending, running, completed, failed, paused) |
| `CanRun()` | All status transitions |

#### `internal/plan/` (>90% coverage)
| Function | Test Cases Required |
|----------|---------------------|
| `Load()` | Exists, not found, invalid YAML |
| `Save()` | New plan, update existing |
| `LoadTemplate()` | All weights (trivial, small, medium, large, greenfield) |
| `Generator.Generate()` | Each weight type, custom overrides |
| Phase dependency resolution | Linear, parallel, complex DAG |

#### `internal/state/` (>90% coverage)
| Function | Test Cases Required |
|----------|---------------------|
| `New()` | Valid task ID |
| `Load()` | Exists, not found, corrupted |
| `Save()` | New state, update existing |
| `StartPhase()` | First phase, subsequent phases |
| `CompletePhase()` | Normal completion, with artifacts |
| `FailPhase()` | With error message |
| `IncrementIteration()` | Normal increment |
| `AddTokens()` | Input/output tokens |
| `RecordGateDecision()` | Approved, rejected, all gate types |
| `GetResumePhase()` | Interrupted, running, completed states |
| `IsPhaseCompleted()` | All phase states |
| `SetRetryContext()` | With/without failure output |
| `ClearRetryContext()` | After retry success |

#### `internal/executor/` (>80% coverage)
| Function | Test Cases Required |
|----------|---------------------|
| `New()` | Valid config, nil config |
| `ExecutePhase()` | Successful completion, max iterations, blocked |
| `ExecuteWithRetry()` | Success first try, retry success, max retries exceeded |
| `buildPromptNode()` | Template rendering, variable substitution |
| `executeClaudeNode()` | Success, API error, timeout |
| `checkCompletionNode()` | Complete detected, blocked detected, neither |
| `commitCheckpointNode()` | Successful commit |
| `isRetryable()` | Rate limit, network, timeout, non-retryable |
| `classifyError()` | All error types |

#### `internal/gate/` (>90% coverage)
| Function | Test Cases Required |
|----------|---------------------|
| `NewEvaluator()` | Valid config |
| `Evaluate()` | Auto gate pass/fail, AI gate approve/reject, human gate pending |
| `evaluateAuto()` | Success output, failure output |
| `evaluateAI()` | Approved, rejected, needs clarification |
| `requestHumanApproval()` | Returns pending status |

#### `internal/config/` (>90% coverage)
| Function | Test Cases Required |
|----------|---------------------|
| `Default()` | Returns valid defaults |
| `Load()` | Exists, not found, invalid YAML |
| `Save()` | New config, update existing |
| `Init()` | Fresh directory, already initialized |
| `IsInitialized()` | True, false cases |
| `RequireInit()` | Initialized, not initialized |

#### `internal/git/` (>80% coverage)
| Function | Test Cases Required |
|----------|---------------------|
| `CreateBranch()` | New branch, already exists |
| `CreateCheckpoint()` | With changes, no changes |
| `SwitchBranch()` | Exists, not exists |
| `GetCurrentBranch()` | On branch, detached HEAD |
| `ListTaskBranches()` | Multiple branches, none |

#### `internal/progress/` (>80% coverage)
| Function | Test Cases Required |
|----------|---------------------|
| `New()` | Valid params |
| `PhaseStart()` | Normal phase |
| `Update()` | Progress update |
| `PhaseComplete()` | With commit SHA |
| `PhaseFailed()` | With error |
| `GatePending()` | Human gate |
| `TaskComplete()` | Normal completion |
| `FormatDuration()` | Seconds, minutes, hours |

#### `internal/api/` (>80% coverage)
| Endpoint | Test Cases Required |
|----------|---------------------|
| `GET /api/health` | Returns 200 |
| `GET /api/tasks` | Empty list, multiple tasks, with filters |
| `POST /api/tasks` | Valid task, missing title, invalid weight |
| `GET /api/tasks/{id}` | Exists, not found |
| `DELETE /api/tasks/{id}` | Exists, not found |
| `GET /api/tasks/{id}/state` | Exists, not found |
| `GET /api/tasks/{id}/plan` | Exists, not found |
| `POST /api/tasks/{id}/run` | Can run, already running, completed |
| `POST /api/tasks/{id}/pause` | Running, not running |
| `POST /api/tasks/{id}/resume` | Paused, not paused |
| `GET /api/tasks/{id}/stream` | SSE connection, events sent |
| CORS | Preflight requests, headers present |

#### `internal/cli/` (>70% coverage)
| Command | Test Cases Required |
|---------|---------------------|
| Signal handling | SIGINT once, SIGINT twice, SIGTERM |
| Graceful shutdown | State saved, message shown |

### CLI Command Verification (ALL commands, ALL flags)

**Every command must work with all its flags:**

#### `orc init`
```bash
./orc init                    # Creates .orc/ and orc.yaml
./orc init --force            # Overwrites existing config
./orc init --help             # Shows usage
```

#### `orc new`
```bash
./orc new "Task title"                           # Auto-classify weight
./orc new "Task title" --weight trivial          # Explicit weight
./orc new "Task title" --weight small
./orc new "Task title" --weight medium
./orc new "Task title" --weight large
./orc new "Task title" --weight greenfield
./orc new "Task title" -w medium                 # Short flag
./orc new "Task title" --description "Details"   # With description
./orc new "Task title" -d "Details"              # Short flag
./orc new "Task title" --branch "custom-branch"  # Custom branch name
./orc new "Task title" -b "custom-branch"        # Short flag
./orc new --help                                 # Shows usage
```

#### `orc list`
```bash
./orc list                           # List active tasks
./orc list --all                     # Include completed
./orc list -a                        # Short flag
./orc list --status running          # Filter by status
./orc list --status paused
./orc list --status completed
./orc list --status failed
./orc list -s running                # Short flag
./orc list --weight medium           # Filter by weight
./orc list -w medium                 # Short flag
./orc list --json                    # JSON output
./orc list -j                        # Short flag
./orc list --help                    # Shows usage
```

#### `orc show`
```bash
./orc show TASK-001                  # Show task details
./orc show TASK-001 --checkpoints    # Include checkpoint history
./orc show TASK-001 --json           # JSON output
./orc show TASK-999                  # Error: not found
./orc show --help                    # Shows usage
```

#### `orc run`
```bash
./orc run TASK-001                        # Run with default profile
./orc run TASK-001 --profile auto         # Explicit profile
./orc run TASK-001 --profile fast
./orc run TASK-001 --profile safe
./orc run TASK-001 --profile strict
./orc run TASK-001 -P auto                # Short flag
./orc run TASK-001 --phase implement      # Start from specific phase
./orc run TASK-001 -p implement           # Short flag
./orc run TASK-001 --continue             # Resume from last position
./orc run TASK-001 -C                     # Short flag
./orc run TASK-001 --dry-run              # Show plan only
./orc run --help                          # Shows usage
```

#### `orc pause`
```bash
./orc pause TASK-001                      # Pause running task
./orc pause TASK-001 --reason "Need info" # With reason
./orc pause TASK-999                      # Error: not found
./orc pause --help                        # Shows usage
```

#### `orc resume`
```bash
./orc resume TASK-001                     # Resume paused/interrupted
./orc resume TASK-999                     # Error: not found
./orc resume --help                       # Shows usage
```

#### `orc stop`
```bash
./orc stop TASK-001                       # Stop task
./orc stop TASK-001 --force               # Force stop
./orc stop --help                         # Shows usage
```

#### `orc rewind`
```bash
./orc rewind TASK-001 --to implement      # Rewind to phase
./orc rewind TASK-001 -t implement        # Short flag
./orc rewind TASK-001 --to spec --hard    # Discard later checkpoints
./orc rewind --help                       # Shows usage
```

#### `orc skip`
```bash
./orc skip TASK-001 --phase design                    # Skip phase
./orc skip TASK-001 -p design                         # Short flag
./orc skip TASK-001 --phase design --reason "Done"    # With reason
./orc skip TASK-001 -p design -r "Done"               # Short flags
./orc skip --help                                     # Shows usage
```

#### `orc approve`
```bash
./orc approve TASK-001                    # Approve pending gate
./orc approve TASK-001 --comment "LGTM"   # With comment
./orc approve --help                      # Shows usage
```

#### `orc reject`
```bash
./orc reject TASK-001 --reason "Tests fail"  # Reject gate
./orc reject TASK-001 -r "Tests fail"        # Short flag
./orc reject --help                          # Shows usage
```

#### `orc log`
```bash
./orc log TASK-001                        # Show transcript
./orc log TASK-001 --phase implement      # Specific phase
./orc log TASK-001 -p implement           # Short flag
./orc log TASK-001 --tail 50              # Last N lines
./orc log TASK-001 -n 50                  # Short flag
./orc log TASK-001 --follow               # Live output
./orc log TASK-001 -f                     # Short flag
./orc log --help                          # Shows usage
```

#### `orc diff`
```bash
./orc diff TASK-001                       # Show changes
./orc diff TASK-001 --phase implement     # Specific phase
./orc diff TASK-001 --stat                # Summary only
./orc diff --help                         # Shows usage
```

#### `orc status`
```bash
./orc status                              # Overall status
./orc status --json                       # JSON output
./orc status --help                       # Shows usage
```

#### `orc cleanup`
```bash
./orc cleanup                             # Clean completed tasks
./orc cleanup --all                       # All task branches
./orc cleanup -a                          # Short flag
./orc cleanup --older-than 7d             # Age filter
./orc cleanup --dry-run                   # Preview only
./orc cleanup --help                      # Shows usage
```

#### `orc serve`
```bash
./orc serve                               # Start on :8080
./orc serve --port 3000                   # Custom port
./orc serve -p 3000                       # Short flag
./orc serve --help                        # Shows usage
```

#### `orc config`
```bash
./orc config                              # Show all config
./orc config --list                       # List keys
./orc config --edit                       # Open editor
./orc config key                          # Get value
./orc config key value                    # Set value
./orc config --help                       # Shows usage
```

#### Global Flags (work with all commands)
```bash
./orc --verbose <command>                 # Verbose output
./orc -v <command>                        # Short flag
./orc -vv <command>                       # Extra verbose
./orc --quiet <command>                   # Suppress output
./orc -q <command>                        # Short flag
./orc --json <command>                    # JSON output
./orc -j <command>                        # Short flag
./orc --config path/to/config <command>   # Custom config
./orc -c path/to/config <command>         # Short flag
./orc --help                              # Help
./orc -h                                  # Short flag
./orc --version                           # Version
./orc -V                                  # Short flag
```

### Frontend Feature Verification

**ALL frontend features must work:**

#### Task List Page (`/`)
- [ ] Page loads without errors
- [ ] Shows loading state while fetching
- [ ] Displays task cards with: ID, title, weight badge, status badge
- [ ] Empty state shown when no tasks
- [ ] "New Task" button visible and clickable
- [ ] Task cards are clickable (navigate to detail)
- [ ] Status badges have correct colors (pending=gray, running=blue, completed=green, failed=red, paused=yellow)
- [ ] Weight badges displayed correctly
- [ ] Responsive layout (mobile, tablet, desktop)

#### Task Creation
- [ ] "New Task" button opens creation form/modal
- [ ] Title input field works
- [ ] Weight selector works (all 5 options)
- [ ] Description field works (optional)
- [ ] Submit creates task via API
- [ ] Success: redirects to task detail or shows in list
- [ ] Error: shows error message, preserves form state
- [ ] Cancel closes form without creating
- [ ] Validation: empty title shows error

#### Task Detail Page (`/tasks/{id}`)
- [ ] Page loads without errors
- [ ] Shows task title, ID, weight, status
- [ ] Shows phase timeline with all phases
- [ ] Current phase highlighted
- [ ] Completed phases show checkmark
- [ ] Failed phases show X
- [ ] Shows transcript container
- [ ] Transcript displays prompt and response
- [ ] Shows token usage
- [ ] Shows elapsed time
- [ ] Back button returns to list

#### Execution Controls
- [ ] "Run" button visible when task can run
- [ ] "Run" button starts execution via API
- [ ] "Pause" button visible when running
- [ ] "Pause" button pauses execution
- [ ] "Resume" button visible when paused
- [ ] "Resume" button resumes execution
- [ ] "Stop" button visible when running
- [ ] "Stop" button stops execution
- [ ] Buttons disabled when action not available
- [ ] Loading states shown during API calls

#### Timeline Component
- [ ] Shows all phases in order
- [ ] Phase names displayed
- [ ] Phase status indicators (pending, running, completed, failed, skipped)
- [ ] Current phase visually highlighted
- [ ] Iteration count shown for running phase
- [ ] Duration shown for completed phases
- [ ] Clickable to view phase details (if implemented)

#### Transcript Component
- [ ] Container scrolls when content overflows
- [ ] Shows prompt sent to Claude
- [ ] Shows response from Claude
- [ ] Tool calls displayed (if applicable)
- [ ] Errors displayed with red styling
- [ ] Auto-scroll to bottom on new content
- [ ] Timestamps shown
- [ ] Iteration markers shown

#### Real-time Updates (SSE)
- [ ] SSE connection established on task detail page
- [ ] Transcript updates in real-time during execution
- [ ] Timeline updates when phase changes
- [ ] Status badge updates on state change
- [ ] Token count updates
- [ ] Connection reconnects on disconnect
- [ ] Connection closed when leaving page

#### Error Handling
- [ ] 404 page for non-existent task
- [ ] Network error shown gracefully
- [ ] API error messages displayed
- [ ] No unhandled exceptions in console
- [ ] Retry option for failed requests

#### Styling & UX
- [ ] Dark theme applied consistently
- [ ] Fonts readable
- [ ] Buttons have hover states
- [ ] Focus states visible (keyboard nav)
- [ ] Loading spinners/skeletons shown
- [ ] Transitions smooth
- [ ] No layout shift on load

### Error Scenario Tests

```bash
# Test: Interrupt handling (Ctrl+C)
./orc run TASK-001  # Start running
# Press Ctrl+C
# Should see: ⚠️ Received interrupt, saving state...
# Should see: ✅ State saved. Resume with: orc resume TASK-001

# Test: Resume after interrupt
./orc resume TASK-001
# Should continue from interrupted phase

# Test: Network failure recovery (simulate with timeout)
# The executor should automatically retry with backoff

# Test: Invalid task ID
./orc show TASK-999
# Should show clear error: "task TASK-999 not found"

# Test: Run on non-existent directory
cd /tmp && /path/to/orc status
# Should show: "not an orc project (no .orc directory)"
```

### Integration Verification
- [ ] llmkit imports work: `claude.Client`, `template.Engine`, `model.*`
- [ ] flowgraph imports work: `NewGraph`, `CompiledGraph`, `checkpoint.Store`, `Context`
- [ ] Phase execution creates git commits with `[orc]` prefix
- [ ] Transcripts saved to `.orc/tasks/TASK-ID/transcripts/`
- [ ] Checkpoints saved and loadable for resume

### UX Verification
- [ ] Progress indication visible during execution
- [ ] Interrupt (Ctrl+C) saves state and shows resume command
- [ ] Error messages include: what failed, what to do, what was expected
- [ ] Resume works after interrupt
- [ ] All CLI commands have --help with examples

### File Structure After Init + New
```
.orc/
├── orc.yaml
└── tasks/
    └── TASK-001/
        ├── task.yaml
        ├── plan.yaml
        ├── state.yaml
        └── transcripts/
```

---

## Phase Commit Requirement

**CRITICAL**: Every phase completion MUST commit before marking complete.

Each phase prompt template includes commit instructions. The executor's `commitCheckpointNode` enforces this. Commit format:

```
[orc] TASK-ID: phase - completed

Phase: phase-name
Status: completed
```

---

## Model Selection

Use **Claude Opus 4.5** (`claude-opus-4-5-20251101`) for ALL phases:

```go
selector := model.NewSelector(
    model.WithThinkingModel(model.ModelOpus),
    model.WithDefaultModel(model.ModelOpus),
    model.WithFastModel(model.ModelOpus),
)
```

---

## Self-Correction Rules

### If import fails
```bash
# Verify go.mod has replace directives (should already be configured)
grep "replace.*llmkit" go.mod
grep "replace.*flowgraph" go.mod

# Ensure sibling repos exist and are up to date
ls ../llmkit ../flowgraph   # Should exist
cd ../llmkit && git pull
cd ../flowgraph && git pull
cd ../orc && go mod tidy

# Or use container (has deps mounted)
make dev
```

### If build fails
1. Read exact error message
2. Fix the specific file/line
3. Re-run `go build ./...`

### If tests fail
1. Run failing test: `go test -v -run TestName ./path/`
2. Fix code or test expectation
3. Re-run until green

### If stuck 3+ iterations on same error
1. Write analysis to `.stuck.md`
2. Try alternative approach
3. If truly blocked, document and continue to next component

---

## Do NOT
- Keep the old executor.go homebrew loop - DELETE IT, use flowgraph
- Implement token counting manually - use llmkit tokens package
- Implement prompt templates manually - use llmkit template package
- Implement Claude CLI wrapper - use llmkit claude package
- Skip git commits after phases - REQUIRED for rollback
- Output `<promise>COMPLETE</promise>` until ALL criteria verified

---

## E2E Testing (MANDATORY - Playwright MCP)

**You MUST run E2E tests using Playwright MCP tools before outputting `<promise>COMPLETE</promise>`.**

### Setup

```bash
# Terminal 1: Start API server
make serve  # Runs on :8080

# Terminal 2: Start frontend
make web-dev  # Runs on :5173
```

### E2E Test Protocol

Use these Playwright MCP tools to verify the system works end-to-end:

| Tool | Purpose |
|------|---------|
| `mcp__playwright__browser_navigate` | Navigate to pages |
| `mcp__playwright__browser_snapshot` | Capture accessibility state (preferred over screenshot) |
| `mcp__playwright__browser_click` | Click buttons, links |
| `mcp__playwright__browser_type` | Type into inputs |
| `mcp__playwright__browser_fill_form` | Fill multiple form fields |
| `mcp__playwright__browser_wait_for` | Wait for text/conditions |
| `mcp__playwright__browser_network_requests` | Verify API calls |

### Required E2E Test Scenarios

**MUST pass ALL of these before completion:**

#### 1. Task Creation Flow
```
1. mcp__playwright__browser_navigate to http://localhost:5173
2. mcp__playwright__browser_snapshot - verify task list loads
3. mcp__playwright__browser_click "New Task" button
4. mcp__playwright__browser_type task title
5. mcp__playwright__browser_click "Create" button
6. mcp__playwright__browser_wait_for task to appear in list
7. mcp__playwright__browser_snapshot - verify task shows with correct status
```

#### 2. Task Detail View
```
1. mcp__playwright__browser_click on a task card
2. mcp__playwright__browser_snapshot - verify detail page loads
3. Verify: task title, weight, status, phase timeline visible
4. Verify: transcript container present
```

#### 3. Task Execution Flow
```
1. Navigate to task detail page
2. mcp__playwright__browser_click "Run" button
3. mcp__playwright__browser_wait_for execution to start
4. mcp__playwright__browser_snapshot - verify:
   - Status changes to "running"
   - Timeline shows current phase highlighted
   - Transcript shows streaming output (if SSE working)
5. mcp__playwright__browser_network_requests - verify API calls to /api/tasks/{id}/run
```

#### 4. Pause/Resume Flow
```
1. During execution, mcp__playwright__browser_click "Pause" button
2. mcp__playwright__browser_snapshot - verify status is "paused"
3. mcp__playwright__browser_click "Resume" button
4. mcp__playwright__browser_snapshot - verify status returns to "running"
```

#### 5. Error Handling
```
1. Navigate to non-existent task: http://localhost:5173/tasks/TASK-999
2. mcp__playwright__browser_snapshot - verify error message displayed
3. Verify no crash, graceful error state
```

#### 6. API Verification
```
1. mcp__playwright__browser_network_requests after each action
2. Verify: GET /api/tasks returns 200
3. Verify: POST /api/tasks creates task
4. Verify: GET /api/tasks/{id} returns task details
5. Verify: SSE connection to /api/tasks/{id}/stream works
```

### E2E Completion Checklist

- [ ] Task list page loads and displays tasks
- [ ] Task creation works end-to-end
- [ ] Task detail page shows all information
- [ ] Run button starts execution
- [ ] Pause/Resume buttons work
- [ ] Timeline updates during execution
- [ ] Transcript shows output (real-time if SSE working)
- [ ] Error states handled gracefully
- [ ] API calls return correct status codes
- [ ] No console errors in browser

---

## When Complete

After verifying ALL completion criteria INCLUDING E2E TESTS:

```
All completion criteria verified:
✓ Build passes with llmkit + flowgraph (local module replace)
✓ Tests pass (>80% coverage)
✓ CLI commands functional:
  - init, new, list, show, status
  - run, pause, resume, rewind
  - approve, skip, cleanup
✓ Phase execution uses flowgraph (NewGraph, SetEntry, AddConditionalEdge)
✓ LLM calls use llmkit (claude.Client with Opus 4.5)
✓ Git commits on phase completion ([orc] prefix)
✓ File structure correct (.orc/tasks/TASK-ID/)
✓ Error recovery: retry with backoff on transient failures
✓ Interrupt handling: Ctrl+C saves state, shows resume command
✓ Progress indication: real-time iteration/token/elapsed display
✓ Resume works: continues from interrupted phase

E2E TESTS (Playwright MCP):
✓ Task list page loads correctly
✓ Task creation flow works
✓ Task detail page displays all info
✓ Run/Pause/Resume execution works
✓ Timeline updates during execution
✓ Transcript displays output
✓ Error handling works
✓ All API calls return correct responses

<promise>COMPLETE</promise>
```
