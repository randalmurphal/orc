// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newApproveCmd creates the approve command
func newApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <task-id>",
		Short: "Approve a gate",
		Args:  cobra.ExactArgs(1),
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

			if t.Status != task.StatusBlocked {
				return fmt.Errorf("task is not blocked (status: %s)", t.Status)
			}

			t.Status = task.StatusPlanned
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
		Short: "Reject a gate",
		Args:  cobra.ExactArgs(1),
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

			s, err := backend.LoadState(id)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}

			if reason == "" {
				reason = "rejected by user"
			}

			s.RecordGateDecision(s.CurrentPhase, "human", false, reason)
			if err := backend.SaveState(s); err != nil {
				return fmt.Errorf("save state: %w", err)
			}

			t.Status = task.StatusFailed
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
