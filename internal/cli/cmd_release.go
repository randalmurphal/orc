// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os/user"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

// newReleaseCmd creates the release command
func newReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release <task-id>",
		Short: "Release your claim on a task",
		Long: `Release your claim on a task.

This clears the claimed_by and claimed_at fields on the task and records
the release in the claim history. The task becomes available for others
to claim.

You can only release tasks you have claimed. Attempting to release a task
claimed by someone else will fail with an error.

Examples:
  orc release TASK-001      # Release your claim on TASK-001`,
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

			taskID := args[0]

			// Get current system user
			currentUser, err := user.Current()
			if err != nil {
				return fmt.Errorf("get current user: %w", err)
			}

			// Open global DB to resolve user ID
			globalDB, err := db.OpenGlobal()
			if err != nil {
				return fmt.Errorf("open global database: %w", err)
			}
			defer func() { _ = globalDB.Close() }()

			// Get or create user ID for current user
			currentUserID, err := globalDB.GetOrCreateUser(currentUser.Username)
			if err != nil {
				return fmt.Errorf("resolve user: %w", err)
			}

			// Load task to check claim status
			dbTask, err := backend.DB().GetTask(taskID)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}
			if dbTask == nil {
				return fmt.Errorf("task %s not found", taskID)
			}

			// Check if task is claimed
			if dbTask.ClaimedBy == "" {
				return fmt.Errorf("%s is not claimed", taskID)
			}

			// Check if claimed by current user
			if dbTask.ClaimedBy != currentUserID {
				// Resolve claimer's name for error message
				claimer, err := globalDB.GetUser(dbTask.ClaimedBy)
				var claimerName string
				if err == nil && claimer != nil {
					claimerName = claimer.Name
				} else {
					claimerName = dbTask.ClaimedBy // Fallback to ID if name lookup fails
				}
				return fmt.Errorf("%s is not claimed by you (claimed by %s)", taskID, claimerName)
			}

			// Release the claim
			released, err := backend.ReleaseUserClaim(taskID, currentUserID)
			if err != nil {
				return fmt.Errorf("release claim: %w", err)
			}

			if !released {
				// Should not happen given our checks above, but handle gracefully
				return fmt.Errorf("failed to release claim on %s", taskID)
			}

			fmt.Printf("Released claim on %s\n", taskID)
			return nil
		},
	}

	return cmd
}
