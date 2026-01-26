// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newApproveCmd creates the approve command
func newApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <task-id>",
		Short: "Approve a blocked gate and allow task execution to continue",
		Long: `Approve a blocked task gate, changing its status from BLOCKED to PLANNED.

Gates are checkpoints in the orc workflow where execution pauses for review.
When a task hits a gate requiring approval (e.g., before creating a PR, before
merging), its status changes to BLOCKED. Use this command to approve and
allow execution to continue.

When to use:
  • After reviewing code changes and deciding they're ready to proceed
  • When a task is waiting at a manual review gate
  • To unblock a task that passed automated checks but needs human sign-off

After approval:
  The task status changes to PLANNED. Run 'orc run <task-id>' to continue
  execution from where it left off.

Examples:
  orc approve TASK-001              # Approve gate, then run: orc run TASK-001
  orc status                        # See which tasks are blocked at gates

See also:
  orc reject   - Reject a gate and mark task as failed
  orc resume   - Resume a paused or failed task
  orc status   - See task statuses including blocked tasks`,
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

			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
				return fmt.Errorf("task is not blocked (status: %s)", task.StatusFromProto(t.Status))
			}

			t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("✅ Task %s approved\n", id)
			fmt.Printf("   Run: orc run %s to continue\n", id)
			return nil
		},
	}
}

// newRejectCmd creates the reject command
func newRejectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject <task-id>",
		Short: "Reject a gate and mark task as failed",
		Long: `Reject a blocked task gate, marking the task as FAILED.

Use this when reviewing a task at a gate checkpoint and deciding the work
should not proceed. The task will be marked as failed with your rejection
reason recorded in the execution history.

When to use:
  • Code quality doesn't meet standards after review
  • Implementation doesn't match requirements
  • Security or architectural concerns found during review
  • Deciding to abandon the current approach

After rejection:
  The task status changes to FAILED. To retry with a different approach,
  use 'orc reset <task-id>' to clear state and start fresh.

Examples:
  orc reject TASK-001                           # Reject with default message
  orc reject TASK-001 --reason "Needs tests"    # Reject with specific reason
  orc reject TASK-001 -r "Wrong approach"       # Short flag version

See also:
  orc approve  - Approve a gate and allow execution to continue
  orc reset    - Clear task state to retry from scratch
  orc resolve  - Mark failed task as resolved without re-running`,
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
			reason, _ := cmd.Flags().GetString("reason")

			t, err := backend.LoadTask(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if reason == "" {
				reason = "rejected by user"
			}

			// Record gate decision in task execution state
			task.EnsureExecutionProto(t)
			t.Execution.Gates = append(t.Execution.Gates, &orcv1.GateDecision{
				Phase:     task.GetCurrentPhaseProto(t),
				GateType:  "human",
				Approved:  false,
				Reason:    &reason,
				Timestamp: timestamppb.Now(),
			})

			t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
			if err := backend.SaveTask(t); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("❌ Task %s rejected: %s\n", id, reason)
			return nil
		},
	}
	cmd.Flags().String("reason", "", "rejection reason")
	return cmd
}
