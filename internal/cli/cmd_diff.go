// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// newDiffCmd creates the diff command
func newDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <task-id>",
		Short: "Show changes made by a task",
		Long: `Show git diff of changes made by a task.

Displays the cumulative diff from when the task branch was created to its current state.

Example:
  orc diff TASK-001           # Show full diff
  orc diff TASK-001 --stat    # Show summary stats only
  orc diff TASK-001 --name-only  # Show changed files only`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			taskID := args[0]
			stat, _ := cmd.Flags().GetBool("stat")
			nameOnly, _ := cmd.Flags().GetBool("name-only")

			// Load task
			t, err := task.Load(taskID)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Get base branch (main or master)
			baseBranch := getBaseBranch()

			// Build git command
			var gitArgs []string
			if stat {
				gitArgs = []string{"diff", "--stat", baseBranch + "..." + t.Branch}
			} else if nameOnly {
				gitArgs = []string{"diff", "--name-only", baseBranch + "..." + t.Branch}
			} else {
				gitArgs = []string{"diff", baseBranch + "..." + t.Branch}
			}

			// Execute git diff
			gitCmd := exec.Command("git", gitArgs...)
			output, err := gitCmd.CombinedOutput()
			if err != nil {
				// Check if it's because no branch exists yet
				if strings.Contains(string(output), "unknown revision") {
					return fmt.Errorf("task branch %s not found - task may not have started yet", t.Branch)
				}
				return fmt.Errorf("git diff: %w\n%s", err, string(output))
			}

			if len(output) == 0 {
				fmt.Println("No changes detected")
				return nil
			}

			fmt.Print(string(output))
			return nil
		},
	}
	cmd.Flags().Bool("stat", false, "show summary stats only")
	cmd.Flags().Bool("name-only", false, "show changed files only")
	return cmd
}

// getBaseBranch returns the base branch name (main or master)
func getBaseBranch() string {
	// Check for main first
	cmd := exec.Command("git", "rev-parse", "--verify", "main")
	if err := cmd.Run(); err == nil {
		return "main"
	}

	// Fall back to master
	cmd = exec.Command("git", "rev-parse", "--verify", "master")
	if err := cmd.Run(); err == nil {
		return "master"
	}

	// Default to main
	return "main"
}
