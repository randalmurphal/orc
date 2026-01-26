// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// getBackend is imported from commands.go

// newPauseCmd creates the pause command
func newPauseCmd() *cobra.Command {
	var force bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "pause <task-id>",
		Short: "Pause task execution (can resume later)",
		Long: `Pause a running task, saving its current state.

The task can be resumed later with 'orc resume'. All progress is preserved,
including uncommitted work which will be committed and pushed.

Use 'orc stop' instead if you want to abort the task permanently.

Examples:
  orc pause TASK-001
  orc pause TASK-001 --timeout 60s  # Wait up to 60s for graceful pause
  orc resume TASK-001               # Continue later`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]

			t, err := backend.LoadTaskProto(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
				return fmt.Errorf("task is not running (status: %s)", t.Status)
			}

			// Check if executor process is alive and signal it
			if t.ExecutorPid > 0 {
				if task.IsPIDAlive(int(t.ExecutorPid)) {
					fmt.Printf("⏸️  Signaling executor (PID %d) to pause...\n", t.ExecutorPid)

					proc, procErr := os.FindProcess(int(t.ExecutorPid))
					if procErr == nil {
						// Send SIGUSR1 for graceful pause
						if sigErr := proc.Signal(syscall.SIGUSR1); sigErr != nil {
							fmt.Printf("Warning: Could not signal executor: %v\n", sigErr)
						} else {
							// Wait for executor to save state
							if waitErr := waitForTaskStatusProto(backend, id, orcv1.TaskStatus_TASK_STATUS_PAUSED, timeout); waitErr != nil {
								if !force {
									return fmt.Errorf("executor did not pause in time: %w (use --force to override)", waitErr)
								}
								fmt.Println("Warning: Executor did not respond, forcing pause...")
							} else {
								fmt.Printf("✅ Task %s paused successfully\n", id)
								fmt.Printf("   Resume with: orc resume %s\n", id)
								return nil
							}
						}
					}
				}
			}

			// Fallback: Update status directly (executor not running or signal failed)
			t.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
			if err := backend.SaveTaskProto(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("⏸️  Task %s paused\n", id)
			fmt.Printf("   Resume with: orc resume %s\n", id)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force pause even if executor doesn't respond")
	cmd.Flags().DurationVarP(&timeout, "timeout", "t", 30*time.Second, "Timeout waiting for executor to pause")
	return cmd
}

// waitForTaskStatusProto polls until proto task reaches expected status or timeout
func waitForTaskStatusProto(backend storage.Backend, taskID string, expected orcv1.TaskStatus, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		t, err := backend.LoadTaskProto(taskID)
		if err == nil && t.Status == expected {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for status %s", expected)
}

// newStopCmd creates the stop command
func newStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <task-id>",
		Short: "Stop task execution permanently (marks as failed)",
		Long: `Stop a task and mark it as failed. This is permanent.

Unlike 'pause', a stopped task cannot be resumed. Use this when you want
to abandon a task entirely.

Use 'orc pause' instead if you want to continue the task later.

Examples:
  orc stop TASK-001           # Prompts for confirmation
  orc stop TASK-001 --force   # Skip confirmation`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			id := args[0]
			force, _ := cmd.Flags().GetBool("force")

			t, err := backend.LoadTaskProto(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
				return fmt.Errorf("task is already completed")
			}

			if t.Status == orcv1.TaskStatus_TASK_STATUS_FAILED {
				fmt.Printf("Task %s is already stopped/failed\n", id)
				return nil
			}

			if !force && !quiet {
				fmt.Printf("⚠️  Stop task %s?\n", id)
				fmt.Println("   This marks the task as failed and cannot be resumed.")
				fmt.Println("   Use 'orc pause' instead to preserve progress.")
				fmt.Print("   Continue? [y/N]: ")

				var input string
				_, _ = fmt.Scanln(&input)
				if input != "y" && input != "Y" {
					fmt.Println("Aborted. Task still running.")
					fmt.Printf("To pause instead: orc pause %s\n", id)
					return nil
				}
			}

			t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
			if err := backend.SaveTaskProto(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("🛑 Task %s stopped (marked as failed)\n", id)
			fmt.Println("\nTo start fresh: orc rewind " + id + " --to <phase>")
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "skip confirmation prompt")
	return cmd
}
