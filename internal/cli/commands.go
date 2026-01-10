// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newInitCmd creates the init command
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize orc in current project",
		Long: `Initialize orc configuration in the current directory.

Creates .orc/ directory with:
  • config.yaml - project configuration
  • prompts/ - phase prompt templates (if not using defaults)

Example:
  orc init
  orc init --force  # overwrite existing config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement init
			fmt.Println("Initializing orc...")
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "overwrite existing configuration")
	return cmd
}

// newNewCmd creates the new task command
func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new task",
		Long: `Create a new task to be orchestrated by orc.

The task will be classified by weight (trivial, small, medium, large, greenfield)
either automatically by AI or manually via --weight flag.

Example:
  orc new "Fix authentication timeout bug"
  orc new "Implement user dashboard" --weight large
  orc new "Create new microservice" --weight greenfield`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement new task
			fmt.Printf("Creating task: %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringP("weight", "w", "", "task weight (trivial, small, medium, large, greenfield)")
	cmd.Flags().StringP("description", "d", "", "task description")
	return cmd
}

// newListCmd creates the list command
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tasks",
		Long: `List all tasks in the current project.

Example:
  orc list
  orc list --status running
  orc list --weight large`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement list
			fmt.Println("Listing tasks...")
			return nil
		},
	}
}

// newShowCmd creates the show command
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement show
			fmt.Printf("Showing task: %s\n", args[0])
			return nil
		},
	}
}

// newRunCmd creates the run command
func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <task-id>",
		Short: "Execute a task",
		Long: `Execute a task through its phases.

The task will be executed according to its plan (based on weight).
Each phase creates a git checkpoint for rewindability.

Example:
  orc run TASK-001
  orc run TASK-001 --phase implement  # run specific phase
  orc run TASK-001 --continue         # resume from last checkpoint`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement run
			fmt.Printf("Running task: %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().String("phase", "", "run specific phase only")
	cmd.Flags().Bool("continue", false, "continue from last checkpoint")
	return cmd
}

// newPauseCmd creates the pause command
func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <task-id>",
		Short: "Pause task execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement pause
			fmt.Printf("Pausing task: %s\n", args[0])
			return nil
		},
	}
}

// newStopCmd creates the stop command
func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <task-id>",
		Short: "Stop task execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement stop
			fmt.Printf("Stopping task: %s\n", args[0])
			return nil
		},
	}
}

// newRewindCmd creates the rewind command
func newRewindCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rewind <task-id> --to <phase>",
		Short: "Rewind task to a checkpoint",
		Long: `Rewind a task to a previous checkpoint.

This uses git reset to restore the codebase state at that checkpoint.
All changes after that checkpoint will be lost.

Example:
  orc rewind TASK-001 --to spec
  orc rewind TASK-001 --to implement`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement rewind
			fmt.Printf("Rewinding task: %s\n", args[0])
			return nil
		},
	}
}

// newStatusCmd creates the status command
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show orc status",
		Long: `Show current orc status including:
  • Active tasks and their phases
  • Pending approvals
  • Recent completions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement status
			fmt.Println("orc status...")
			return nil
		},
	}
}

// newLogCmd creates the log command
func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log <task-id>",
		Short: "Show task transcripts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement log
			fmt.Printf("Showing log for task: %s\n", args[0])
			return nil
		},
	}
}

// newDiffCmd creates the diff command
func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <task-id>",
		Short: "Show task changes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement diff
			fmt.Printf("Showing diff for task: %s\n", args[0])
			return nil
		},
	}
}

// newApproveCmd creates the approve command
func newApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <task-id>",
		Short: "Approve a gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement approve
			fmt.Printf("Approving task: %s\n", args[0])
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
			// TODO: Implement reject
			fmt.Printf("Rejecting task: %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().String("reason", "", "rejection reason")
	return cmd
}

// newExportCmd creates the export command
func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <task-id>",
		Short: "Export task context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement export
			fmt.Printf("Exporting task: %s\n", args[0])
			return nil
		},
	}
}

// newImportCmd creates the import command
func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Import context into task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement import
			fmt.Printf("Importing from: %s\n", args[0])
			return nil
		},
	}
}

// newConfigCmd creates the config command
func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config [key] [value]",
		Short: "Get or set configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement config
			fmt.Println("Configuration...")
			return nil
		},
	}
}

// newVersionCmd creates the version command
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show orc version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("orc version 0.1.0-dev")
		},
	}
}
