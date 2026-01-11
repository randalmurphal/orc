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

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// newRunCmd creates the run command
func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <task-id>",
		Short: "Execute a task",
		Long: `Execute a task through its phases.

The task will be executed according to its plan (based on weight).
Each phase creates a git checkpoint for rewindability.

Automation profiles control gate behavior:
  auto   - Fully automated, no human intervention (default)
  fast   - Minimal gates, speed over safety
  safe   - AI reviews, human approval only for merge
  strict - Human gates on spec/review/merge

Example:
  orc run TASK-001
  orc run TASK-001 --profile safe
  orc run TASK-001 --phase implement  # run specific phase`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]
			profile, _ := cmd.Flags().GetString("profile")

			// Load task
			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Check if task can run
			if !t.CanRun() && t.Status != task.StatusRunning {
				return fmt.Errorf("task cannot be run (status: %s)", t.Status)
			}

			// Load plan
			p, err := plan.Load(id)
			if err != nil {
				return fmt.Errorf("load plan: %w", err)
			}

			// Load or create state
			s, err := state.Load(id)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
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

			// Set up signal handling for graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\n⚠️  Interrupt received, saving state...")
				cancel()
			}()

			// Create progress display
			disp := progress.New(id, quiet)
			disp.Info(fmt.Sprintf("Starting task %s (%s) [profile: %s]", id, t.Weight, cfg.Profile))

			// Create executor with config
			exec := executor.NewWithConfig(executor.ConfigFromOrc(cfg), cfg)

			// Execute task
			err = exec.ExecuteTask(ctx, t, p, s)
			if err != nil {
				if ctx.Err() != nil {
					// Update task and state status for clean interrupt
					s.InterruptPhase(s.CurrentPhase)
					s.Save()
					t.Status = task.StatusBlocked
					t.Save()
					disp.TaskInterrupted()
					return nil // Clean interrupt
				}
				disp.TaskFailed(err)
				return err
			}

			disp.TaskComplete(s.Tokens.TotalTokens, time.Since(s.StartedAt))
			return nil
		},
	}
	cmd.Flags().String("phase", "", "run specific phase only")
	cmd.Flags().StringP("profile", "p", "", "automation profile (auto, fast, safe, strict)")
	cmd.Flags().Bool("continue", false, "continue from last checkpoint")
	return cmd
}
