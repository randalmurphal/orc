// Package cli implements the orc command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

// newInitiativeNotesCmd creates the 'initiative notes' command for listing notes.
func newInitiativeNotesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes <initiative-id>",
		Short: "List notes for an initiative",
		Long: `List all notes for an initiative, grouped by type.

Notes capture shared knowledge across tasks within an initiative:
  pattern   Reusable patterns or approaches that worked well
  warning   Pitfalls or issues to avoid
  learning  Key insights or discoveries
  handoff   Information for downstream tasks

Examples:
  orc initiative notes INIT-001              # List all notes
  orc initiative notes INIT-001 --type pattern  # Filter by type
  orc initiative notes INIT-001 --json       # JSON output`,
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

			initID := args[0]
			noteType, _ := cmd.Flags().GetString("type")

			// Verify initiative exists
			exists, err := backend.InitiativeExists(initID)
			if err != nil {
				return fmt.Errorf("check initiative: %w", err)
			}
			if !exists {
				return fmt.Errorf("initiative %s not found", initID)
			}

			// Validate note type if specified
			if noteType != "" {
				validTypes := []string{db.NoteTypePattern, db.NoteTypeWarning, db.NoteTypeLearning, db.NoteTypeHandoff}
				valid := false
				for _, vt := range validTypes {
					if noteType == vt {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("invalid note type %q: must be one of %s", noteType, strings.Join(validTypes, ", "))
				}
			}

			// Get notes
			var notes []db.InitiativeNote
			if noteType != "" {
				notes, err = backend.GetInitiativeNotesByType(initID, noteType)
			} else {
				notes, err = backend.GetInitiativeNotes(initID)
			}
			if err != nil {
				return fmt.Errorf("get notes: %w", err)
			}

			// JSON output
			if jsonOut {
				data, _ := json.MarshalIndent(notes, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(notes) == 0 {
				if noteType != "" {
					fmt.Printf("No %s notes found for %s.\n", noteType, initID)
				} else {
					fmt.Printf("No notes found for %s.\n", initID)
				}
				fmt.Println("\nAdd notes with: orc initiative note <initiative-id> --type <type> \"content\"")
				return nil
			}

			// Group notes by type for display
			byType := make(map[string][]db.InitiativeNote)
			typeOrder := []string{db.NoteTypePattern, db.NoteTypeWarning, db.NoteTypeLearning, db.NoteTypeHandoff}
			for _, n := range notes {
				byType[n.NoteType] = append(byType[n.NoteType], n)
			}

			fmt.Printf("Notes for %s (%d total):\n\n", initID, len(notes))

			for _, nt := range typeOrder {
				typeNotes, ok := byType[nt]
				if !ok || len(typeNotes) == 0 {
					continue
				}

				icon := noteTypeIcon(nt)
				fmt.Printf("%s %s (%d)\n", icon, strings.ToUpper(nt), len(typeNotes))
				fmt.Println(strings.Repeat("─", 40))

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				for _, n := range typeNotes {
					source := n.Author
					if n.AuthorType == db.NoteAuthorAgent && n.SourceTask != "" {
						source = n.SourceTask
					}
					created := n.CreatedAt.Format("2006-01-02")
					content := truncate(n.Content, 60)
					_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", n.ID, content, source, created)
				}
				_ = w.Flush()
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringP("type", "t", "", "filter by note type (pattern, warning, learning, handoff)")

	return cmd
}

// newInitiativeNoteCmd creates the 'initiative note' command for adding/deleting notes.
func newInitiativeNoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Add or manage initiative notes",
		Long: `Add or manage notes for an initiative.

Notes are shared knowledge that flows to future tasks via {{INITIATIVE_CONTEXT}}:
  pattern   Reusable patterns or approaches that worked well
  warning   Pitfalls or issues to avoid
  learning  Key insights or discoveries
  handoff   Information for downstream tasks

Examples:
  orc initiative note INIT-001 --type pattern "Use the factory pattern for handlers"
  orc initiative note INIT-001 --type warning "Config validation fails silently"
  orc initiative note delete NOTE-001
  orc initiative note delete NOTE-001 --force`,
	}

	cmd.AddCommand(newInitiativeNoteAddCmd())
	cmd.AddCommand(newInitiativeNoteDeleteCmd())

	return cmd
}

// newInitiativeNoteAddCmd creates the subcommand for adding a note.
func newInitiativeNoteAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <initiative-id> <content>",
		Short: "Add a note to an initiative",
		Long: `Add a human-authored note to an initiative.

The note will be shared with future tasks in the initiative.

Examples:
  orc initiative note add INIT-001 --type pattern "Always validate config before use"
  orc initiative note add INIT-001 --type warning "The legacy API returns 200 on errors"`,
		Args: cobra.ExactArgs(2),
		RunE: runInitiativeNoteAdd,
	}

	cmd.Flags().StringP("type", "t", "", "note type (required): pattern, warning, learning, handoff")
	_ = cmd.MarkFlagRequired("type")

	return cmd
}

// For backward compatibility, also make the parent 'note' command runnable
// when called with args directly (without 'add' subcommand).
func init() {
	// This is handled by the command structure - 'note add' is the explicit form
}

func runInitiativeNoteAdd(cmd *cobra.Command, args []string) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	initID := args[0]
	content := args[1]
	noteType, _ := cmd.Flags().GetString("type")

	// Validate note type
	validTypes := []string{db.NoteTypePattern, db.NoteTypeWarning, db.NoteTypeLearning, db.NoteTypeHandoff}
	valid := false
	for _, vt := range validTypes {
		if noteType == vt {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid note type %q: must be one of %s", noteType, strings.Join(validTypes, ", "))
	}

	// Verify initiative exists
	exists, err := backend.InitiativeExists(initID)
	if err != nil {
		return fmt.Errorf("check initiative: %w", err)
	}
	if !exists {
		return fmt.Errorf("initiative %s not found", initID)
	}

	// Generate note ID
	noteID, err := backend.GetNextNoteID()
	if err != nil {
		return fmt.Errorf("generate note ID: %w", err)
	}

	// Create note
	note := &db.InitiativeNote{
		ID:           noteID,
		InitiativeID: initID,
		Author:       "human",
		AuthorType:   db.NoteAuthorHuman,
		NoteType:     noteType,
		Content:      content,
		CreatedAt:    time.Now(),
	}

	if err := backend.SaveInitiativeNote(note); err != nil {
		return fmt.Errorf("save note: %w", err)
	}

	if !quiet {
		icon := noteTypeIcon(noteType)
		fmt.Printf("Note added: %s\n", noteID)
		fmt.Printf("  %s Type: %s\n", icon, noteType)
		fmt.Printf("  Content: %s\n", truncate(content, 60))
		fmt.Printf("  Initiative: %s\n", initID)
	}

	return nil
}

// newInitiativeNoteDeleteCmd creates the subcommand for deleting a note.
func newInitiativeNoteDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <note-id>",
		Short: "Delete a note by ID",
		Long: `Delete an initiative note by its ID.

Examples:
  orc initiative note delete NOTE-001
  orc initiative note delete NOTE-001 --force   # Skip confirmation`,
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

			noteID := args[0]
			force, _ := cmd.Flags().GetBool("force")

			// Check if note exists
			note, err := backend.GetInitiativeNote(noteID)
			if err != nil {
				return fmt.Errorf("get note: %w", err)
			}
			if note == nil {
				return fmt.Errorf("note %s not found", noteID)
			}

			if !force {
				fmt.Printf("Delete note %s? [y/N]: ", noteID)
				fmt.Printf("  Type: %s\n", note.NoteType)
				fmt.Printf("  Content: %s\n", truncate(note.Content, 60))
				var response string
				_, _ = fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Cancelled")
					return nil
				}
			}

			if err := backend.DeleteInitiativeNote(noteID); err != nil {
				return fmt.Errorf("delete note: %w", err)
			}

			fmt.Printf("Deleted note %s\n", noteID)
			return nil
		},
	}

	cmd.Flags().BoolP("force", "f", false, "skip confirmation")

	return cmd
}

// noteTypeIcon returns an icon for the note type.
func noteTypeIcon(noteType string) string {
	switch noteType {
	case db.NoteTypePattern:
		return "🔄"
	case db.NoteTypeWarning:
		return "⚠️"
	case db.NoteTypeLearning:
		return "💡"
	case db.NoteTypeHandoff:
		return "📋"
	default:
		return "📝"
	}
}
