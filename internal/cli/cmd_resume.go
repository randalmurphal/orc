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
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

func newResumeCmd() *cobra.Command {
	var forceResume bool

	cmd := &cobra.Command{
		Use:   "resume <task-id>",
		Short: "Resume a paused, blocked, interrupted, or orphaned task",
		Long: `Resume a task that was paused, blocked, interrupted, or became orphaned.

For tasks marked as "running" but whose executor process has died (orphaned),
this command will automatically mark them as interrupted and resume execution.

Use --force to resume a task even if it appears to still be running.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			s, err := state.Load(id)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}

			// Handle different task statuses
			switch t.Status {
			case task.StatusPaused, task.StatusBlocked:
				// These are always resumable
			case task.StatusRunning:
				// Check if it's orphaned
				isOrphaned, reason := s.CheckOrphaned()
				if isOrphaned {
					fmt.Printf("Task %s appears orphaned (%s)\n", id, reason)
					fmt.Println("Marking as interrupted and resuming...")
					if err := state.MarkOrphanedAsInterrupted("", id); err != nil {
						return fmt.Errorf("mark orphaned task as interrupted: %w", err)
					}
					// Reload task after marking
					t, err = task.Load(id)
					if err != nil {
						return fmt.Errorf("reload task: %w", err)
					}
					s, err = state.Load(id)
					if err != nil {
						return fmt.Errorf("reload state: %w", err)
					}
				} else if forceResume {
					fmt.Printf("Warning: Task %s may still be running (PID %d)\n", id, s.GetExecutorPID())
					fmt.Println("Force-resuming as requested...")
					// Clear execution info to allow resume
					s.ClearExecution()
					s.InterruptPhase(s.CurrentPhase)
					if err := s.Save(); err != nil {
						return fmt.Errorf("save state: %w", err)
					}
					t.Status = task.StatusBlocked
					if err := t.Save(); err != nil {
						return fmt.Errorf("save task: %w", err)
					}
				} else {
					return fmt.Errorf("task is currently running (PID %d). Use --force to resume anyway", s.GetExecutorPID())
				}
			case task.StatusFailed:
				// Allow resuming failed tasks
				fmt.Printf("Task %s failed previously, resuming from last phase...\n", id)
			default:
				return fmt.Errorf("task cannot be resumed (status: %s)", t.Status)
			}

			p, err := plan.Load(id)
			if err != nil {
				return fmt.Errorf("load plan: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Set up signal handling
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\n⚠️  Interrupt received, saving state...")
				cancel()
			}()

			disp := progress.New(id, quiet)
			disp.Info(fmt.Sprintf("Resuming task %s", id))

			// Show session ID if available (useful for manual Claude resume)
			if sessionID := s.GetSessionID(); sessionID != "" {
				disp.Info(fmt.Sprintf("Session ID: %s (use 'claude --resume %s' for direct Claude access)", sessionID, sessionID))
			}

			exec := executor.NewWithConfig(executor.ConfigFromOrc(cfg), cfg)

			// Set up streaming publisher if verbose or --stream flag is set
			stream, _ := cmd.Flags().GetBool("stream")
			if verbose || stream {
				publisher := events.NewCLIPublisher(os.Stdout, events.WithStreamMode(true))
				exec.SetPublisher(publisher)
				defer publisher.Close()
			}

			// Find resume phase
			resumePhase := s.GetResumePhase()
			if resumePhase == "" {
				resumePhase = s.CurrentPhase
			}
			if resumePhase == "" {
				return fmt.Errorf("no phase to resume from")
			}

			err = exec.ResumeFromPhase(ctx, t, p, s, resumePhase)
			if err != nil {
				if ctx.Err() != nil {
					disp.TaskInterrupted()
					return nil
				}
				disp.TaskFailed(err)
				return err
			}

			disp.TaskComplete(s.Tokens.TotalTokens, time.Since(s.StartedAt))
			return nil
		},
	}
	cmd.Flags().Bool("stream", false, "stream Claude transcript to stdout")
	cmd.Flags().BoolVarP(&forceResume, "force", "f", false, "force resume even if task appears to be running")
	return cmd
}
