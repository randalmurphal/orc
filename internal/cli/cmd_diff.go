// Package cli implements the orc command-line interface.
package cli

// TODO: Implement diff command to show task changes via git diff.
// The command should show all commits/changes made by a task.
//
// Planned usage:
//   orc diff <task-id>        # Show all changes made by task
//   orc diff <task-id> --stat # Show summary stats only
//
// Implementation needs:
//   - Get task branch from task.yaml
//   - Find base commit (before task started)
//   - Run git diff base..HEAD or git log base..HEAD
