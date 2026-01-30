// Package cli implements the orc command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

func newCommentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Manage task comments and notes",
		Long: `Manage comments and notes on tasks.

Comments allow humans and agents to leave notes, feedback, and context
on tasks throughout their lifecycle.

Commands:
  add     Add a comment to a task
  list    List comments for a task
  delete  Delete a comment`,
	}

	cmd.AddCommand(newCommentAddCmd())
	cmd.AddCommand(newCommentListCmd())
	cmd.AddCommand(newCommentDeleteCmd())

	return cmd
}

func newCommentAddCmd() *cobra.Command {
	var author string
	var authorType string
	var phase string

	cmd := &cobra.Command{
		Use:   "add <task-id> <content>",
		Short: "Add a comment to a task",
		Long: `Add a comment or note to a task.

Examples:
  orc comment add TASK-001 "This approach won't work with the existing auth flow"
  orc comment add TASK-001 "Note: uses deprecated API" --author "claude" --type agent
  orc comment add TASK-001 "Review feedback addressed" --phase implement`,
		Args: cobra.MinimumNArgs(2),
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
			content := strings.Join(args[1:], " ")

			// Validate task exists
			exists, err := backend.TaskExists(taskID)
			if err != nil {
				return fmt.Errorf("check task: %w", err)
			}
			if !exists {
				return fmt.Errorf("task %s not found", taskID)
			}

			// Validate author type
			at := db.AuthorType(authorType)
			if at != "" && at != db.AuthorTypeHuman && at != db.AuthorTypeAgent && at != db.AuthorTypeSystem {
				return fmt.Errorf("invalid author type: must be human, agent, or system")
			}
			if at == "" {
				at = db.AuthorTypeHuman
			}

			wd, err := ResolveProjectPath()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			pdb, err := db.OpenProject(wd)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = pdb.Close() }()

			comment := &db.TaskComment{
				TaskID:     taskID,
				Author:     author,
				AuthorType: at,
				Content:    content,
				Phase:      phase,
			}

			if err := pdb.CreateTaskComment(comment); err != nil {
				return fmt.Errorf("create comment: %w", err)
			}

			fmt.Printf("Added comment %s to %s\n", comment.ID, taskID)
			return nil
		},
	}

	cmd.Flags().StringVarP(&author, "author", "a", "", "Author name (default: anonymous)")
	cmd.Flags().StringVarP(&authorType, "type", "t", "human", "Author type: human, agent, or system")
	cmd.Flags().StringVarP(&phase, "phase", "p", "", "Phase this comment relates to")

	return cmd
}

func newCommentListCmd() *cobra.Command {
	var authorType string
	var phase string

	cmd := &cobra.Command{
		Use:   "list [task-id]",
		Short: "List comments for a task",
		Long: `List comments and notes for a task.

Examples:
  orc comment list TASK-001
  orc comment list TASK-001 --type agent
  orc comment list TASK-001 --phase implement`,
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

			// Validate task exists
			exists, err := backend.TaskExists(taskID)
			if err != nil {
				return fmt.Errorf("check task: %w", err)
			}
			if !exists {
				return fmt.Errorf("task %s not found", taskID)
			}

			wd, err := ResolveProjectPath()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			pdb, err := db.OpenProject(wd)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = pdb.Close() }()

			var comments []db.TaskComment

			if authorType != "" {
				comments, err = pdb.ListTaskCommentsByAuthorType(taskID, db.AuthorType(authorType))
			} else if phase != "" {
				comments, err = pdb.ListTaskCommentsByPhase(taskID, phase)
			} else {
				comments, err = pdb.ListTaskComments(taskID)
			}

			if err != nil {
				return fmt.Errorf("list comments: %w", err)
			}

			if len(comments) == 0 {
				fmt.Printf("No comments found for %s\n", taskID)
				return nil
			}

			if jsonOut {
				data, _ := json.MarshalIndent(comments, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tAUTHOR\tTYPE\tPHASE\tCREATED\tCONTENT")
			_, _ = fmt.Fprintln(w, "--\t------\t----\t-----\t-------\t-------")

			for _, c := range comments {
				author := c.Author
				if author == "" {
					author = string(c.AuthorType)
				}

				phase := c.Phase
				if phase == "" {
					phase = "-"
				}

				content := c.Content
				if len(content) > 50 {
					content = content[:47] + "..."
				}
				// Remove newlines for table display
				content = strings.ReplaceAll(content, "\n", " ")

				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					c.ID,
					truncate(author, 15),
					c.AuthorType,
					phase,
					formatTimeAgo(c.CreatedAt),
					content,
				)
			}
			_ = w.Flush()

			return nil
		},
	}

	cmd.Flags().StringVarP(&authorType, "type", "t", "", "Filter by author type: human, agent, or system")
	cmd.Flags().StringVarP(&phase, "phase", "p", "", "Filter by phase")

	return cmd
}

func newCommentDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <comment-id>",
		Short: "Delete a comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			commentID := args[0]
			wd, err := ResolveProjectPath()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			pdb, err := db.OpenProject(wd)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = pdb.Close() }()

			// Check if comment exists
			comment, err := pdb.GetTaskComment(commentID)
			if err != nil {
				return fmt.Errorf("get comment: %w", err)
			}
			if comment == nil {
				return fmt.Errorf("comment %s not found", commentID)
			}

			if err := pdb.DeleteTaskComment(commentID); err != nil {
				return fmt.Errorf("delete comment: %w", err)
			}

			fmt.Printf("Deleted comment %s\n", commentID)
			return nil
		},
	}
}
