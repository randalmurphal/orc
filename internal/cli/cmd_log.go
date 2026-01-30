// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
)

// ANSI color codes for transcript display
const (
	ansiDim     = "\033[2m"
	ansiBold    = "\033[1m"
	ansiCyan    = "\033[36m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiReset   = "\033[0m"
	ansiMagenta = "\033[35m"
)

// newLogCmd creates the log command
func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <task-id>",
		Short: "Show task transcripts (use --follow for real-time streaming)",
		Long: `Show Claude transcripts from task execution.

Transcripts capture the full conversation between orc and Claude during each
phase. Use this to understand what Claude did, debug issues, or learn from
the AI's approach.

Viewing modes:
  Default     Shows the most recent transcript entries
  --all       Shows all transcripts for all phases
  --phase     Filter to a specific phase (implement, test, etc.)
  --follow    Real-time streaming during execution (polls database)

Content filtering:
  --response-only   Show only Claude's responses (assistant messages)
  --prompt-only     Show only the prompts (user messages)
  --no-color        Disable color output
  --raw             Show raw JSON content (unformatted)

Quality tips:
  * When debugging a failed task, start with the latest transcript
  * Use --phase to find specific work (e.g., --phase test for test phase)
  * Use --follow during execution to watch Claude work in real-time
  * Transcripts are stored directly in the database during execution

Examples:
  orc log TASK-001              # Latest transcript entries
  orc log TASK-001 --all        # All transcripts, all phases
  orc log TASK-001 --phase test # Just the test phase transcript
  orc log TASK-001 --tail 50    # Last 50 entries only
  orc log TASK-001 --tail 0     # All entries (no limit)
  orc log TASK-001 --follow     # Stream new messages in real-time
  orc log TASK-001 -r           # Show only Claude's responses
  orc log TASK-001 --prompt-only # Show only the prompts sent to Claude
  orc log TASK-001 --raw        # Show raw JSON content

See also:
  orc show TASK-001 --session   # View session stats (tokens, timing)
  orc diff TASK-001             # View code changes made by task`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]
			phase, _ := cmd.Flags().GetString("phase")
			all, _ := cmd.Flags().GetBool("all")
			tail, _ := cmd.Flags().GetInt("tail")
			follow, _ := cmd.Flags().GetBool("follow")
			responseOnly, _ := cmd.Flags().GetBool("response-only")
			promptOnly, _ := cmd.Flags().GetBool("prompt-only")
			noColor, _ := cmd.Flags().GetBool("no-color")
			raw, _ := cmd.Flags().GetBool("raw")

			// Validate mutually exclusive flags
			if responseOnly && promptOnly {
				return fmt.Errorf("--response-only and --prompt-only are mutually exclusive")
			}

			// Build display options
			opts := transcriptDisplayOptions{
				responseOnly: responseOnly,
				promptOnly:   promptOnly,
				useColor:     !noColor && isatty.IsTerminal(os.Stdout.Fd()),
				raw:          raw,
				phase:        phase,
			}

			// Follow mode - poll database for new transcripts
			if follow {
				return followTranscripts(id, opts)
			}

			// Create storage backend to query database
			backend, err := getBackend()
			if err != nil {
				return err
			}
			defer func() { _ = backend.Close() }()

			// Get transcripts from database
			transcripts, err := backend.GetTranscripts(id)
			if err != nil {
				return fmt.Errorf("get transcripts: %w", err)
			}

			if len(transcripts) == 0 {
				fmt.Printf("No transcripts found for task %s\n", id)
				fmt.Println("\nThe task may not have run yet.")
				fmt.Printf("Try: orc run %s\n", id)
				return nil
			}

			// Filter by phase if specified
			if phase != "" {
				var filtered []storage.Transcript
				for _, t := range transcripts {
					if strings.EqualFold(t.Phase, phase) {
						filtered = append(filtered, t)
					}
				}
				if len(filtered) == 0 {
					fmt.Printf("No transcripts found for phase '%s'\n", phase)
					fmt.Println("\nAvailable phases:")
					phases := collectPhases(transcripts)
					for _, p := range phases {
						fmt.Printf("  %s\n", p)
					}
					return nil
				}
				transcripts = filtered
			}

			// Filter by type (user/assistant)
			if opts.responseOnly {
				var filtered []storage.Transcript
				for _, t := range transcripts {
					if t.Type == "assistant" {
						filtered = append(filtered, t)
					}
				}
				transcripts = filtered
			} else if opts.promptOnly {
				var filtered []storage.Transcript
				for _, t := range transcripts {
					if t.Type == "user" {
						filtered = append(filtered, t)
					}
				}
				transcripts = filtered
			}

			// Apply tail limit (default 100, 0 for all)
			if !all && tail > 0 && len(transcripts) > tail {
				transcripts = transcripts[len(transcripts)-tail:]
			}

			// Display transcripts
			displayTranscripts(transcripts, opts)

			return nil
		},
	}

	cmd.Flags().StringP("phase", "p", "", "filter to specific phase (e.g., 'implement', 'test')")
	cmd.Flags().BoolP("all", "a", false, "show all transcripts (not just latest)")
	cmd.Flags().IntP("tail", "n", 100, "number of entries to show (0 for all)")
	cmd.Flags().BoolP("follow", "f", false, "stream new messages as they are written")
	cmd.Flags().BoolP("response-only", "r", false, "show only Claude's responses (assistant messages)")
	cmd.Flags().Bool("prompt-only", false, "show only the prompts (user messages)")
	cmd.Flags().Bool("no-color", false, "disable color output")
	cmd.Flags().Bool("raw", false, "show raw JSON content (unformatted)")

	return cmd
}

// transcriptDisplayOptions configures transcript display behavior
type transcriptDisplayOptions struct {
	responseOnly bool   // Show only assistant messages
	promptOnly   bool   // Show only user messages
	useColor     bool   // Enable color output
	raw          bool   // Show raw JSON content
	phase        string // Filter by phase
}

// displayTranscripts renders transcripts to stdout
func displayTranscripts(transcripts []storage.Transcript, opts transcriptDisplayOptions) {
	var currentPhase string

	for _, t := range transcripts {
		// Show phase header when phase changes
		if t.Phase != currentPhase {
			currentPhase = t.Phase
			if opts.useColor {
				fmt.Printf("\n%s─── %s ───%s\n\n", ansiBold, currentPhase, ansiReset)
			} else {
				fmt.Printf("\n─── %s ───\n\n", currentPhase)
			}
		}

		displaySingleTranscript(t, opts)
	}
}

// displaySingleTranscript renders a single transcript entry
func displaySingleTranscript(t storage.Transcript, opts transcriptDisplayOptions) {
	// Format timestamp
	ts := time.UnixMilli(t.Timestamp)
	timeStr := ts.Format("15:04:05")

	// Type indicator
	typeIndicator := "?"
	typeColor := ""
	switch t.Type {
	case "user":
		typeIndicator = "USER"
		typeColor = ansiCyan
	case "assistant":
		typeIndicator = "ASSISTANT"
		typeColor = ansiGreen
	}

	// Header line
	if opts.useColor {
		fmt.Printf("%s[%s]%s %s%s%s", ansiDim, timeStr, ansiReset, typeColor, typeIndicator, ansiReset)
	} else {
		fmt.Printf("[%s] %s", timeStr, typeIndicator)
	}

	// Show model for assistant messages
	if t.Model != "" && t.Type == "assistant" {
		if opts.useColor {
			fmt.Printf(" %s(%s)%s", ansiMagenta, t.Model, ansiReset)
		} else {
			fmt.Printf(" (%s)", t.Model)
		}
	}

	// Show token usage for assistant messages
	if t.Type == "assistant" && (t.InputTokens > 0 || t.OutputTokens > 0) {
		if opts.useColor {
			fmt.Printf(" %s[in:%d out:%d]%s", ansiDim, t.InputTokens, t.OutputTokens, ansiReset)
		} else {
			fmt.Printf(" [in:%d out:%d]", t.InputTokens, t.OutputTokens)
		}
	}

	fmt.Println()

	// Content
	if opts.raw {
		fmt.Println(t.Content)
	} else {
		displayFormattedContent(t.Content, opts)
	}

	fmt.Println()
}

// displayFormattedContent renders the content JSON in a readable format
func displayFormattedContent(content string, opts transcriptDisplayOptions) {
	// Content is JSON array of content blocks
	var blocks []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &blocks); err != nil {
		// Not JSON, display as-is
		fmt.Println(content)
		return
	}

	for _, block := range blocks {
		blockType, _ := block["type"].(string)

		switch blockType {
		case "text":
			text, _ := block["text"].(string)
			fmt.Println(text)

		case "tool_use":
			name, _ := block["name"].(string)
			if opts.useColor {
				fmt.Printf("%s[Tool: %s]%s\n", ansiYellow, name, ansiReset)
			} else {
				fmt.Printf("[Tool: %s]\n", name)
			}
			// Optionally show tool input (can be verbose)
			if input, ok := block["input"]; ok {
				inputJSON, _ := json.MarshalIndent(input, "  ", "  ")
				if opts.useColor {
					fmt.Printf("%s  %s%s\n", ansiDim, string(inputJSON), ansiReset)
				} else {
					fmt.Printf("  %s\n", string(inputJSON))
				}
			}

		case "tool_result":
			if opts.useColor {
				fmt.Printf("%s[Tool Result]%s\n", ansiYellow, ansiReset)
			} else {
				fmt.Println("[Tool Result]")
			}

		case "thinking":
			// Extended thinking block - show the thinking content
			thinking, _ := block["thinking"].(string)
			if thinking != "" {
				if opts.useColor {
					fmt.Printf("%s[Thinking]%s\n", ansiMagenta, ansiReset)
					fmt.Printf("%s%s%s\n", ansiDim, thinking, ansiReset)
				} else {
					fmt.Println("[Thinking]")
					fmt.Println(thinking)
				}
			}

		default:
			// Unknown block type, show as JSON
			blockJSON, _ := json.MarshalIndent(block, "", "  ")
			fmt.Println(string(blockJSON))
		}
	}
}

// collectPhases returns unique phase names from transcripts
func collectPhases(transcripts []storage.Transcript) []string {
	seen := make(map[string]bool)
	var phases []string
	for _, t := range transcripts {
		if !seen[t.Phase] {
			seen[t.Phase] = true
			phases = append(phases, t.Phase)
		}
	}
	return phases
}

// followTranscripts polls the database for new transcripts during task execution.
// This provides real-time streaming without relying on filesystem-based JSONL files.
func followTranscripts(taskID string, opts transcriptDisplayOptions) error {
	backend, err := getBackend()
	if err != nil {
		return err
	}
	defer func() { _ = backend.Close() }()

	// Verify task exists
	if exists, err := backend.TaskExists(taskID); err != nil {
		return fmt.Errorf("check task: %w", err)
	} else if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nStopping...")
		cancel()
	}()

	fmt.Printf("Following transcripts for %s (Ctrl+C to stop)...\n\n", taskID)

	// Track last seen transcript ID
	var lastSeenID int64
	var currentPhase string
	pollInterval := 500 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(pollInterval):
			// Get all transcripts and filter to new ones
			transcripts, err := backend.GetTranscripts(taskID)
			if err != nil {
				// Log but continue polling
				fmt.Fprintf(os.Stderr, "Warning: failed to get transcripts: %v\n", err)
				continue
			}

			// Filter to only new transcripts
			for _, t := range transcripts {
				if t.ID <= lastSeenID {
					continue
				}

				// Apply type filter
				if opts.responseOnly && t.Type != "assistant" {
					continue
				}
				if opts.promptOnly && t.Type != "user" {
					continue
				}

				// Apply phase filter
				if opts.phase != "" && !strings.EqualFold(t.Phase, opts.phase) {
					continue
				}

				// Show phase header when phase changes
				if t.Phase != currentPhase {
					currentPhase = t.Phase
					if opts.useColor {
						fmt.Printf("\n%s─── %s ───%s\n\n", ansiBold, currentPhase, ansiReset)
					} else {
						fmt.Printf("\n─── %s ───\n\n", currentPhase)
					}
				}

				// Display the transcript
				displaySingleTranscript(t, opts)

				lastSeenID = t.ID
			}

			// Check if task is still running (task.Status is single source of truth)
			t, err := backend.LoadTask(taskID)
			if err == nil && t != nil {
				// If task completed/failed/paused, show message and exit
				switch t.Status {
				case orcv1.TaskStatus_TASK_STATUS_COMPLETED,
					orcv1.TaskStatus_TASK_STATUS_RESOLVED,
					orcv1.TaskStatus_TASK_STATUS_FAILED,
					orcv1.TaskStatus_TASK_STATUS_PAUSED,
					orcv1.TaskStatus_TASK_STATUS_BLOCKED:
					statusStr := t.Status.String()
					if opts.useColor {
						fmt.Printf("\n%sTask %s (status: %s)%s\n", ansiDim, taskID, statusStr, ansiReset)
					} else {
						fmt.Printf("\nTask %s (status: %s)\n", taskID, statusStr)
					}
					return nil
				}
			}
		}
	}
}
