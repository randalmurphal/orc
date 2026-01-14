// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bootstrap"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// newGoCmd creates the go command - main entry point for orc workflows
func newGoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "go [description]",
		Short: "Run complete orc workflow",
		Long: `Run the complete orc workflow from start to finish.

This is the main entry point for orc. It handles:
1. Project initialization (if not already initialized)
2. Spec creation (interactive) or task creation (quick mode)
3. Task planning and execution
4. Review and QA sessions (configurable)

Modes:
  Interactive (default): Shows status and guides next steps
  Headless (--headless): Automated execution, no user interaction
  Quick (description):   Skip spec, create single task, execute immediately

Examples:
  orc go                          # Interactive guidance
  orc go --headless               # Automated mode
  orc go "Add user authentication" # Quick single task
  orc go --headless --quick "Fix login bug"`,
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
				return runQuickMode(ctx, cfg, description, weight, stream)
			}

			if headless {
				return runHeadlessMode(ctx, cfg)
			}

			return runInteractiveMode(ctx, cfg)
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
func runQuickMode(ctx context.Context, cfg *config.Config, description, weight string, stream bool) error {
	fmt.Printf("Quick mode: %s\n\n", description)

	// Create task
	id, err := task.NextID()
	if err != nil {
		return fmt.Errorf("generate task id: %w", err)
	}

	t := task.New(id, description)
	t.Description = description

	// Set weight (flag has default "medium", so always set)
	t.Weight = task.Weight(weight)

	// Save task
	if err := t.Save(); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	// Generate plan from weight template
	p, err := plan.CreateFromTemplate(t)
	if err != nil {
		// If template not found, use default plan
		fmt.Printf("Warning: No template for weight %s, using default plan\n", t.Weight)
		p = &plan.Plan{
			Version:     1,
			TaskID:      id,
			Weight:      t.Weight,
			Description: "Default plan",
			Phases: []plan.Phase{
				{ID: "implement", Name: "implement", Gate: plan.Gate{Type: plan.GateAuto}, Status: plan.PhasePending},
			},
		}
	}

	if err := p.Save(id); err != nil {
		return fmt.Errorf("save plan: %w", err)
	}

	// Update task status
	t.Status = task.StatusPlanned
	if err := t.Save(); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	// Create state
	s := state.New(id)
	if err := s.Save(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	fmt.Printf("Created task %s\n", id)
	fmt.Printf("  Weight: %s\n", t.Weight)
	fmt.Printf("  Phases: %d\n\n", len(p.Phases))

	// Execute task
	return executeTask(ctx, cfg, t, p, s, stream)
}

// runHeadlessMode executes existing tasks or parses spec in automated mode
func runHeadlessMode(ctx context.Context, cfg *config.Config) error {
	fmt.Println("Headless mode: Looking for tasks to execute...")
	fmt.Println()

	// Find tasks to run
	tasks, err := task.LoadAll()
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
		p, err := plan.Load(t.ID)
		if err != nil {
			fmt.Printf("Warning: Skipping %s: could not load plan: %v\n", t.ID, err)
			continue
		}

		s, err := state.Load(t.ID)
		if err != nil {
			s = state.New(t.ID)
		}

		fmt.Printf("Running %s: %s\n", t.ID, t.Title)
		if err := executeTask(ctx, cfg, t, p, s, false); err != nil {
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
func runInteractiveMode(ctx context.Context, cfg *config.Config) error {
	fmt.Println("Orc Interactive Mode")
	fmt.Println()

	// Check for existing runnable tasks
	tasks, err := task.LoadAll()
	if err != nil && !os.IsNotExist(err) {
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

// executeTask runs a single task through all phases
func executeTask(ctx context.Context, cfg *config.Config, t *task.Task, p *plan.Plan, s *state.State, stream bool) error {
	// Create progress display
	disp := progress.New(t.ID, quiet)
	disp.Info(fmt.Sprintf("Executing %s (%s)", t.ID, t.Weight))

	// Create executor
	exec := executor.NewWithConfig(executor.ConfigFromOrc(cfg), cfg)

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
			s.Save()
			t.Status = task.StatusBlocked
			t.Save()
			disp.TaskInterrupted()
			return nil
		}
		disp.TaskFailed(err)
		return err
	}

	// Compute file change stats for completion summary
	var fileStats *progress.FileChangeStats
	if t.Branch != "" {
		fileStats = getGoFileChangeStats(ctx, t.Branch, cfg)
	}

	disp.TaskComplete(s.Tokens.TotalTokens, time.Since(s.StartedAt), fileStats)
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
