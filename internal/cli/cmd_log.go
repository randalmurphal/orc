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

	"github.com/randalmurphal/llmkit/claude/jsonl"
	"github.com/randalmurphal/llmkit/claude/session"
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
  --follow    Real-time streaming during execution (tails JSONL file)

Content filtering:
  --response-only   Show only Claude's responses (assistant messages)
  --prompt-only     Show only the prompts (user messages)
  --no-color        Disable color output
  --raw             Show raw JSON content (unformatted)

Quality tips:
  * When debugging a failed task, start with the latest transcript
  * Use --phase to find specific work (e.g., --phase test for test phase)
  * Use --follow during execution to watch Claude work in real-time
  * Transcripts are stored in the database and synced from Claude's JSONL files

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

			// Follow mode - stream from live JSONL file
			if follow {
				return followLiveJSONL(id, opts)
			}

			// Create storage backend to query database
			backend, err := storage.NewDatabaseBackend(".", nil)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = backend.Close() }()

			// Get transcripts from database
			transcripts, err := backend.GetTranscripts(id)
			if err != nil {
				return fmt.Errorf("get transcripts: %w", err)
			}

			if len(transcripts) == 0 {
				fmt.Printf("No transcripts found for task %s\n", id)
				fmt.Println("\nThe task may not have run yet, or transcripts were not synced.")
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
	cmd.Flags().BoolP("prompt-only", "P", false, "show only the prompts (user messages)")
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

// followLiveJSONL streams messages from the live JSONL file during task execution
func followLiveJSONL(taskID string, opts transcriptDisplayOptions) error {
	// Create storage backend to load state
	backend, err := storage.NewDatabaseBackend(".", nil)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = backend.Close() }()

	// Load task state to get JSONL path
	st, err := backend.LoadState(taskID)
	if err != nil {
		return fmt.Errorf("load task state: %w", err)
	}
	if st == nil {
		return fmt.Errorf("no state found for task %s", taskID)
	}

	// Get JSONL path from state (if available)
	jsonlPath := st.JSONLPath
	if jsonlPath == "" {
		return fmt.Errorf("no active JSONL file for task %s (task may not be running)", taskID)
	}

	// Check file exists
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		return fmt.Errorf("JSONL file not found: %s", jsonlPath)
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

	fmt.Printf("Following %s (Ctrl+C to stop)...\n\n", jsonlPath)

	// Create JSONL reader and tail
	reader, err := jsonl.NewReader(jsonlPath)
	if err != nil {
		return fmt.Errorf("create JSONL reader: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Stream new messages
	msgCh := reader.Tail(ctx)
	for msg := range msgCh {
		// Filter by type if needed
		if opts.responseOnly && msg.Type != "assistant" {
			continue
		}
		if opts.promptOnly && msg.Type != "user" {
			continue
		}

		// Filter by phase if specified
		if opts.phase != "" {
			// JSONL messages don't have phase - skip filtering in follow mode
			// or we could match by session ID patterns
		}

		// Display the message
		displayJSONLMessage(msg, opts)
	}

	return nil
}

// displayJSONLMessage renders a JSONL message during live streaming
func displayJSONLMessage(msg session.JSONLMessage, opts transcriptDisplayOptions) {
	// Parse timestamp
	ts, err := time.Parse(time.RFC3339Nano, msg.Timestamp)
	if err != nil {
		ts = time.Now()
	}
	timeStr := ts.Format("15:04:05")

	// Type indicator
	typeIndicator := strings.ToUpper(msg.Type)
	typeColor := ""
	switch msg.Type {
	case "user":
		typeColor = ansiCyan
	case "assistant":
		typeColor = ansiGreen
	}

	// Header
	if opts.useColor {
		fmt.Printf("%s[%s]%s %s%s%s", ansiDim, timeStr, ansiReset, typeColor, typeIndicator, ansiReset)
	} else {
		fmt.Printf("[%s] %s", timeStr, typeIndicator)
	}

	// Model and tokens for assistant
	if msg.Message != nil {
		if msg.Message.Model != "" {
			if opts.useColor {
				fmt.Printf(" %s(%s)%s", ansiMagenta, msg.Message.Model, ansiReset)
			} else {
				fmt.Printf(" (%s)", msg.Message.Model)
			}
		}
		if msg.Message.Usage != nil {
			if opts.useColor {
				fmt.Printf(" %s[in:%d out:%d]%s", ansiDim, msg.Message.Usage.InputTokens, msg.Message.Usage.OutputTokens, ansiReset)
			} else {
				fmt.Printf(" [in:%d out:%d]", msg.Message.Usage.InputTokens, msg.Message.Usage.OutputTokens)
			}
		}
	}

	fmt.Println()

	// Content
	if msg.Message != nil && len(msg.Message.Content) > 0 {
		if opts.raw {
			fmt.Println(string(msg.Message.Content))
		} else {
			displayFormattedContent(string(msg.Message.Content), opts)
		}
	}

	fmt.Println()
}
