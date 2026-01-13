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
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/playwright"
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
				// Provide helpful error message based on status
				switch t.Status {
				case task.StatusPaused:
					fmt.Printf("Task %s is paused.\n\n", id)
					fmt.Printf("To resume:  orc resume %s\n", id)
					fmt.Printf("To restart: orc rewind %s --to <phase>\n", id)
					return nil
				case task.StatusBlocked:
					fmt.Printf("Task %s is blocked and needs user input.\n\n", id)
					fmt.Println("Check the task for pending questions or approvals.")
					fmt.Printf("To view:    orc show %s\n", id)
					return nil
				case task.StatusCompleted:
					fmt.Printf("Task %s is already completed.\n\n", id)
					fmt.Printf("To rerun:   orc rewind %s --to <phase>\n", id)
					fmt.Printf("To view:    orc show %s\n", id)
					return nil
				case task.StatusFailed:
					fmt.Printf("Task %s has failed.\n\n", id)
					fmt.Printf("To retry:   orc rewind %s --to <phase>\n", id)
					fmt.Printf("To view:    orc log %s\n", id)
					return nil
				default:
					return fmt.Errorf("task cannot be run (status: %s)", t.Status)
				}
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

			// Ensure Playwright MCP is configured if task requires UI testing
			if t.RequiresUITesting {
				screenshotDir := playwright.GetScreenshotDir(".", id)
				mcpConfig := &playwright.Config{
					Enabled:       true,
					ScreenshotDir: screenshotDir,
					Headless:      true,
					Browser:       "chromium",
				}

				if _, err := playwright.EnsureMCPServer(".", mcpConfig); err != nil {
					// Log warning but don't fail - MCP may already be configured
					if !quiet {
						fmt.Printf("⚠️  Warning: Could not configure Playwright MCP: %v\n", err)
						fmt.Println("   You may need to configure it manually or use existing MCP tools.")
					}
				} else if !quiet {
					fmt.Println("🎭 Playwright MCP configured for UI testing")
				}
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

			// Set up streaming publisher if verbose or --stream flag is set
			stream, _ := cmd.Flags().GetBool("stream")
			if verbose || stream {
				publisher := events.NewCLIPublisher(os.Stdout, events.WithStreamMode(true))
				exec.SetPublisher(publisher)
				defer publisher.Close()
			}

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
	cmd.Flags().Bool("stream", false, "stream Claude transcript to stdout")
	return cmd
}
