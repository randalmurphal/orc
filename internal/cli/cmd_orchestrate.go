package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/orchestrator"
	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/spf13/cobra"
)

var orchestrateCmd = &cobra.Command{
	Use:   "orchestrate [task-ids...]",
	Short: "Run multiple tasks in parallel with dependency awareness",
	Long: `Orchestrates execution of multiple tasks using parallel Claude sessions.

The orchestrator:
1. Creates git worktrees for each task
2. Runs Claude sessions in parallel (up to max-concurrent)
3. Respects task dependencies
4. Uses ralph-style iteration loops per task
5. Publishes events for real-time monitoring

Examples:
  # Run all pending tasks
  orc orchestrate

  # Run specific tasks
  orc orchestrate TASK-001 TASK-002 TASK-003

  # Run tasks from an initiative
  orc orchestrate --initiative INIT-001

  # Limit concurrency
  orc orchestrate --max-concurrent 2

  # Show status of running orchestrator
  orc orchestrate --status`,
	RunE: runOrchestrate,
}

var (
	orchestrateMaxConcurrent int
	orchestrateInitiative    string
	orchestrateStatus        bool
)

func init() {
	orchestrateCmd.GroupID = groupPlanning
	rootCmd.AddCommand(orchestrateCmd)

	orchestrateCmd.Flags().IntVar(&orchestrateMaxConcurrent, "max-concurrent", 4, "Maximum parallel tasks")
	orchestrateCmd.Flags().StringVarP(&orchestrateInitiative, "initiative", "i", "", "Run tasks from initiative")
	orchestrateCmd.Flags().BoolVar(&orchestrateStatus, "status", false, "Show orchestrator status")
}

func runOrchestrate(cmd *cobra.Command, args []string) error {
	// Check if .orc directory exists
	if _, err := os.Stat(".orc"); os.IsNotExist(err) {
		return fmt.Errorf("orc not initialized in this directory (run 'orc init' first)")
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

	// Create git operations
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	gitOps, err := NewGitOpsFromConfig(cwd, cfg)
	if err != nil {
		return fmt.Errorf("init git: %w", err)
	}

	// Create prompt service
	promptSvc := prompt.NewService(".orc")

	// Create event publisher (simple stdout for now)
	publisher := &stdoutPublisher{}

	// Create orchestrator
	orcCfg := &orchestrator.Config{
		MaxConcurrent: orchestrateMaxConcurrent,
		PollInterval:  2 * time.Second,
	}
	orc := orchestrator.New(orcCfg, cfg, publisher, gitOps, promptSvc, backend, nil)

	// Add tasks based on arguments
	if orchestrateInitiative != "" {
		// Load initiative and add its tasks
		init, err := backend.LoadInitiative(orchestrateInitiative)
		if err != nil {
			return fmt.Errorf("load initiative: %w", err)
		}

		if err := orc.AddTasksFromInitiative(init); err != nil {
			return fmt.Errorf("add tasks from initiative: %w", err)
		}

		fmt.Printf("Added %d tasks from initiative %s\n", len(init.Tasks), init.ID)
	} else if len(args) > 0 {
		// Add specific tasks
		for _, taskID := range args {
			orc.AddTask(taskID, "", nil, orchestrator.PriorityDefault)
		}
		fmt.Printf("Added %d tasks to orchestrator\n", len(args))
	} else {
		// Add all pending tasks
		if err := orc.AddPendingTasks(); err != nil {
			return fmt.Errorf("add pending tasks: %w", err)
		}
	}

	// Check status
	status := orc.Status()
	if status.QueueLength == 0 {
		fmt.Println("No tasks to run.")
		return nil
	}

	fmt.Printf("\nStarting orchestrator...\n")
	fmt.Printf("  Max concurrent: %d\n", orchestrateMaxConcurrent)
	fmt.Printf("  Tasks queued: %d\n", status.QueueLength)
	fmt.Println()

	// Setup context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt, stopping orchestrator...")
		cancel()
	}()

	// Start orchestrator
	if err := orc.Start(ctx); err != nil {
		return fmt.Errorf("start orchestrator: %w", err)
	}

	// Print status periodically
	go printOrchestratorStatus(ctx, orc)

	// Wait for completion or interrupt
	orc.Wait()

	// Stop orchestrator
	if err := orc.Stop(); err != nil {
		return fmt.Errorf("stop orchestrator: %w", err)
	}

	// Print final status
	finalStatus := orc.Status()
	fmt.Println()
	fmt.Println("=== Orchestration Complete ===")
	fmt.Printf("  Completed: %d\n", finalStatus.CompletedCount)
	fmt.Printf("  Failed: %d\n", finalStatus.FailedCount)

	if finalStatus.FailedCount > 0 {
		return fmt.Errorf("%d tasks failed", finalStatus.FailedCount)
	}

	return nil
}

// printOrchestratorStatus prints periodic status updates.
func printOrchestratorStatus(ctx context.Context, orc *orchestrator.Orchestrator) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := orc.Status()
			if status.Status == orchestrator.StatusRunning {
				fmt.Printf("\n[Status] Active: %d/%d | Queue: %d | Completed: %d | Failed: %d\n",
					status.ActiveCount, status.MaxConcurrent,
					status.QueueLength, status.CompletedCount, status.FailedCount)
				if len(status.RunningTasks) > 0 {
					fmt.Printf("  Running: %v\n", status.RunningTasks)
				}
			}
		}
	}
}

// stdoutPublisher is a simple event publisher that prints to stdout.
type stdoutPublisher struct{}

func (p *stdoutPublisher) Publish(event events.Event) {
	switch event.Type {
	case events.EventPhase:
		if data, ok := event.Data.(map[string]any); ok {
			phase := data["phase"]
			status := data["status"]
			fmt.Printf("[%s] Phase %s: %s\n", event.TaskID, phase, status)
		}
	case events.EventComplete:
		fmt.Printf("[%s] Task completed\n", event.TaskID)
	case events.EventError:
		if data, ok := event.Data.(map[string]any); ok {
			fmt.Printf("[%s] Error: %v\n", event.TaskID, data["error"])
		}
	}
}

func (p *stdoutPublisher) Subscribe(taskID string) <-chan events.Event {
	return nil
}

func (p *stdoutPublisher) Unsubscribe(taskID string, ch <-chan events.Event) {}

func (p *stdoutPublisher) Close() {}
