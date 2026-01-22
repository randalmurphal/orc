// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bootstrap"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// newGoCmd creates the go command - main entry point for orc workflows
func newGoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go [description]",
		Short: "Run complete orc workflow (main entry point)",
		Long: `Quick way to create and execute a task in one command.

This is the fastest path from idea to implementation, but for best results
you should provide enough context for Claude to succeed.

═══════════════════════════════════════════════════════════════════════════════
WHAT MAKES 'orc go' SUCCEED
═══════════════════════════════════════════════════════════════════════════════

The description you provide becomes the foundation for the entire task.
Claude uses it to:
  • Generate the specification (success criteria, testing requirements)
  • Guide implementation decisions
  • Know when the work is done

GOOD (specific, provides context):
  orc go "Add rate limiting to /api/users - max 100 req/min per user,
  return 429 when exceeded, exclude admin users"

BAD (vague, Claude will guess):
  orc go "fix api"

═══════════════════════════════════════════════════════════════════════════════
WEIGHT SELECTION (Critical for quality)
═══════════════════════════════════════════════════════════════════════════════

Weight determines which phases run. Default is 'medium'.

  trivial    → tiny_spec → implement
             Use for: typos, config tweaks, one-liners

  small      → tiny_spec → implement → review
             Use for: bug fixes, isolated changes

  medium     → spec → tdd_write → implement → review → docs (DEFAULT)
             Use for: features needing thought

  large      → spec → tdd_write → breakdown → implement → review → docs → validate
             Use for: complex multi-file features, new systems

The 'spec/tiny_spec' phase generates Success Criteria. The 'tdd_write' phase writes
failing tests BEFORE implementation. The 'review' phase runs 5 specialized code reviewers.

⚠️  Under-weighting is the #1 cause of poor results.
    A "medium" task run as "small" skips spec → Claude guesses → poor results.

═══════════════════════════════════════════════════════════════════════════════
EXECUTION MODES
═══════════════════════════════════════════════════════════════════════════════

  Interactive (default)   Run with no args - shows status, guides next steps
  Quick (description)     Provide description - creates and executes immediately
  Headless (--headless)   Runs all ready tasks with no user interaction

═══════════════════════════════════════════════════════════════════════════════
AUTOMATION PROFILES
═══════════════════════════════════════════════════════════════════════════════

  auto (default)  Fully automated, AI handles all gates
  fast            Speed optimized, minimal validation
  safe            AI reviews, requires human approval for merge
  strict          Human gates on all major decisions

Use --profile=safe for important work where you want final approval.

═══════════════════════════════════════════════════════════════════════════════
EXAMPLES
═══════════════════════════════════════════════════════════════════════════════

# Quick fixes (trivial weight)
orc go "Fix typo: 'recieve' → 'receive' in error messages" -w trivial

# Bug fixes (small weight)
orc go "Fix login failing silently on auth timeout - show error message" -w small

# Features (medium weight - default)
orc go "Add pagination to user list API - limit/offset, max 100 per page"

# Complex features (large weight)
orc go "Implement Redis caching layer for API responses" -w large

# Watch Claude work
orc go "Add dark mode toggle" --stream

# Safe mode for production changes
orc go "Update payment processing logic" --profile safe

See also:
  orc new      - Create task with full control (description, initiative, deps)
  orc run      - Execute an existing task
  orc status   - View what needs attention`,
		RunE: func(cmd *cobra.Command, args []string) error {
			headless, _ := cmd.Flags().GetBool("headless")
			quick, _ := cmd.Flags().GetBool("quick")
			profile, _ := cmd.Flags().GetString("profile")
			weight, _ := cmd.Flags().GetString("weight")
			skipReview, _ := cmd.Flags().GetBool("skip-review")
			skipQA, _ := cmd.Flags().GetBool("skip-qa")

			// Determine mode
			var description string
			if len(args) > 0 {
				description = args[0]
				quick = true // Description implies quick mode
			}

			// Step 1: Ensure orc is initialized
			if err := ensureInit(); err != nil {
				return err
			}

			// Get backend
			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Apply profile if specified
			if profile != "" {
				cfg.ApplyProfile(config.AutomationProfile(profile))
			}

			// Apply skip flags
			if skipReview {
				cfg.Review.Enabled = false
			}
			if skipQA {
				cfg.QA.Enabled = false
			}

			// Set up context and signal handling
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\nInterrupt received, saving state...")
				cancel()
			}()

			stream, _ := cmd.Flags().GetBool("stream")

			if quick && description != "" {
				return runQuickMode(ctx, backend, cfg, description, weight, stream)
			}

			if headless {
				return runHeadlessMode(ctx, backend, cfg)
			}

			return runInteractiveMode(ctx, backend, cfg)
		},
	}

	cmd.Flags().Bool("headless", false, "run in headless mode (no user interaction)")
	cmd.Flags().Bool("quick", false, "quick mode: skip spec, create single task")
	cmd.Flags().StringP("profile", "p", "", "automation profile (auto, fast, safe, strict)")
	cmd.Flags().StringP("weight", "w", "medium", "task weight (trivial, small, medium, large, greenfield)")
	cmd.Flags().Bool("skip-review", false, "skip review phase")
	cmd.Flags().Bool("skip-qa", false, "skip QA phase")
	cmd.Flags().Bool("stream", false, "stream Claude transcript to stdout")

	return cmd
}

// ensureInit checks if orc is initialized, and initializes if not
func ensureInit() error {
	orcDir := ".orc"
	if _, err := os.Stat(orcDir); os.IsNotExist(err) {
		fmt.Println("Orc not initialized. Running orc init...")
		result, err := bootstrap.Run(bootstrap.Options{})
		if err != nil {
			return fmt.Errorf("initialize orc: %w", err)
		}
		bootstrap.PrintResult(result)
		fmt.Println()
	}
	return nil
}

// runQuickMode creates a single task and executes it immediately
func runQuickMode(ctx context.Context, backend storage.Backend, cfg *config.Config, description, weight string, stream bool) error {
	fmt.Printf("Quick mode: %s\n\n", description)

	// Create task
	id, err := backend.GetNextTaskID()
	if err != nil {
		return fmt.Errorf("generate task id: %w", err)
	}

	t := task.New(id, description)
	t.Description = description

	// Set weight (flag has default "medium", so always set)
	t.Weight = task.Weight(weight)

	// Save task - status will be set to planned
	t.Status = task.StatusPlanned
	if err := backend.SaveTask(t); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	// Create plan dynamically from task weight
	p := createGoPlanForWeight(id, t.Weight)

	// Create state
	s := state.New(id)
	if err := backend.SaveState(s); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	fmt.Printf("Created task %s\n", id)
	fmt.Printf("  Weight: %s\n", t.Weight)
	fmt.Printf("  Phases: %d\n\n", len(p.Phases))

	// Execute task
	return executeTaskWithBackend(ctx, backend, cfg, t, p, s, stream)
}

// runHeadlessMode executes existing tasks or parses spec in automated mode
func runHeadlessMode(ctx context.Context, backend storage.Backend, cfg *config.Config) error {
	fmt.Println("Headless mode: Looking for tasks to execute...")
	fmt.Println()

	// Find tasks to run
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	// Filter to runnable tasks
	var runnable []*task.Task
	for _, t := range tasks {
		if t.CanRun() {
			runnable = append(runnable, t)
		}
	}

	if len(runnable) == 0 {
		fmt.Println("No runnable tasks found.")
		fmt.Println("Create a task with: orc new \"task description\"")
		fmt.Println("Or run: orc go \"quick task description\"")
		return nil
	}

	fmt.Printf("Found %d runnable task(s)\n\n", len(runnable))

	// Execute tasks in order
	for _, t := range runnable {
		// Create plan dynamically from task weight
		p := createGoPlanForWeight(t.ID, t.Weight)

		s, err := backend.LoadState(t.ID)
		if err != nil {
			s = state.New(t.ID)
		}

		fmt.Printf("Running %s: %s\n", t.ID, t.Title)
		if err := executeTaskWithBackend(ctx, backend, cfg, t, p, s, false); err != nil {
			if ctx.Err() != nil {
				return nil // Clean interrupt
			}
			fmt.Printf("Task %s failed: %v\n\n", t.ID, err)
			// Continue to next task
		}
		fmt.Println()
	}

	return nil
}

// runInteractiveMode starts an interactive Claude session for spec creation
func runInteractiveMode(ctx context.Context, backend storage.Backend, cfg *config.Config) error {
	fmt.Println("Orc Interactive Mode")
	fmt.Println()

	// Check for existing runnable tasks
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	var runnable []*task.Task
	for _, t := range tasks {
		if t.CanRun() {
			runnable = append(runnable, t)
		}
	}

	if len(runnable) > 0 {
		fmt.Printf("Found %d existing runnable task(s):\n", len(runnable))
		for _, t := range runnable {
			fmt.Printf("  %s %s: %s (%s)\n", statusIcon(t.Status), t.ID, t.Title, t.Status)
		}
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  orc orchestrate       - Run all ready tasks")
		fmt.Println("  orc run TASK-ID       - Run specific task")
		fmt.Println("  orc new \"description\" - Create new task")
		fmt.Println()
		return nil
	}

	// No tasks - guide user to create one
	fmt.Println("No tasks found. To get started:")
	fmt.Println()
	fmt.Println("  orc new \"task description\"   Create a task")
	fmt.Println("  orc go \"task description\"    Quick execute")
	fmt.Println("  orc setup                     Interactive setup")
	fmt.Println()
	fmt.Println("Or use slash commands in Claude Code:")
	fmt.Println("  /orc:init     Initialize or create spec")
	fmt.Println("  /orc:status   Check task status")
	fmt.Println()

	return nil
}

// executeTaskWithBackend runs a single task through all phases
func executeTaskWithBackend(ctx context.Context, backend storage.Backend, cfg *config.Config, t *task.Task, p *executor.Plan, s *state.State, stream bool) error {
	// Create progress display
	disp := progress.New(t.ID, quiet)
	disp.Info(fmt.Sprintf("Executing %s (%s)", t.ID, t.Weight))

	// Create executor
	exec := executor.NewWithConfig(executor.ConfigFromOrc(cfg), cfg)
	exec.SetBackend(backend)

	// Set up streaming if verbose or --stream flag is set
	if verbose || stream {
		publisher := events.NewCLIPublisher(os.Stdout, events.WithStreamMode(true))
		exec.SetPublisher(publisher)
		defer publisher.Close()
	}

	// Execute task
	err := exec.ExecuteTask(ctx, t, p, s)
	if err != nil {
		if ctx.Err() != nil {
			s.InterruptPhase(s.CurrentPhase)
			if saveErr := backend.SaveState(s); saveErr != nil {
				disp.Warning(fmt.Sprintf("failed to save state on interrupt: %v", saveErr))
			}
			t.Status = task.StatusBlocked
			if saveErr := backend.SaveTask(t); saveErr != nil {
				disp.Warning(fmt.Sprintf("failed to save task on interrupt: %v", saveErr))
			}
			disp.TaskInterrupted()
			return nil
		}

		// Check if task is blocked (phases succeeded but completion failed)
		if errors.Is(err, executor.ErrTaskBlocked) {
			// Reload task to get updated metadata with conflict info
			t, _ = backend.LoadTask(t.ID)
			blockedCtx := buildBlockedContext(t, cfg)
			disp.TaskBlockedWithContext(s.Tokens.TotalTokens, s.Elapsed(), "sync conflict", blockedCtx)
			return nil // Not a fatal error - task execution succeeded
		}

		disp.TaskFailed(err)
		return err
	}

	// Compute file change stats for completion summary
	var fileStats *progress.FileChangeStats
	if t.Branch != "" {
		fileStats = getGoFileChangeStats(ctx, t.Branch, cfg)
	}

	disp.TaskComplete(s.Tokens.TotalTokens, s.Elapsed(), fileStats)
	return nil
}

// getGoFileChangeStats computes diff statistics for the task branch vs target branch.
// Returns nil if stats cannot be computed (not an error - just no stats to display).
func getGoFileChangeStats(ctx context.Context, taskBranch string, cfg *config.Config) *progress.FileChangeStats {
	// Get project root for diff computation
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		return nil
	}

	// Determine target branch from config
	targetBranch := "main"
	if cfg != nil && cfg.Completion.TargetBranch != "" {
		targetBranch = cfg.Completion.TargetBranch
	}

	// Create diff service to compute stats
	diffSvc := diff.NewService(projectRoot, nil)

	// Resolve target branch (handles origin/main fallback)
	resolvedBase := diffSvc.ResolveRef(ctx, targetBranch)

	// Get diff stats between target branch and task branch
	stats, err := diffSvc.GetStats(ctx, resolvedBase, taskBranch)
	if err != nil {
		// Diff stat computation is best-effort - don't fail task completion
		return nil
	}

	return &progress.FileChangeStats{
		FilesChanged: stats.FilesChanged,
		Additions:    stats.Additions,
		Deletions:    stats.Deletions,
	}
}

// createGoPlanForWeight creates an execution plan based on task weight.
// Plans are created dynamically for execution, not stored.
func createGoPlanForWeight(taskID string, weight task.Weight) *executor.Plan {
	var phases []executor.Phase

	switch weight {
	case task.WeightTrivial:
		phases = []executor.Phase{
			{ID: "tiny_spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case task.WeightSmall:
		phases = []executor.Phase{
			{ID: "tiny_spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case task.WeightMedium:
		phases = []executor.Phase{
			{ID: "spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	case task.WeightLarge:
		phases = []executor.Phase{
			{ID: "spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "tdd_write", Name: "TDD Tests", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "breakdown", Name: "Breakdown", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "docs", Name: "Documentation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "validate", Name: "Validation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	default:
		phases = []executor.Phase{
			{ID: "spec", Name: "Specification", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "implement", Name: "Implementation", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
			{ID: "review", Name: "Review", Status: executor.PhasePending, Gate: gate.Gate{Type: gate.GateAuto}},
		}
	}

	return &executor.Plan{
		TaskID: taskID,
		Phases: phases,
	}
}
