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

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <task-id>",
		Short: "Resume a paused or interrupted task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status != task.StatusPaused && t.Status != task.StatusBlocked {
				return fmt.Errorf("task cannot be resumed (status: %s)", t.Status)
			}

			p, err := plan.Load(id)
			if err != nil {
				return fmt.Errorf("load plan: %w", err)
			}

			s, err := state.Load(id)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
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

			exec := executor.New(executor.ConfigFromOrc(cfg))

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
}
