// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/randalmurphal/llmkit/claude/jsonl"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/state"
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

	// If JSONLPath is empty, try to construct it as a fallback
	if jsonlPath == "" {
		// Try to construct path from session ID and worktree
		constructedPath, constructErr := constructJSONLPathFallback(taskID, st)
		if constructErr == nil && constructedPath != "" {
			jsonlPath = constructedPath
		} else {
			// Provide accurate error message based on task status
			return formatFollowError(taskID, st, constructErr)
		}
	}

	// Check file exists
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		// File doesn't exist yet - check if task is still starting up
		if st.Status == state.StatusRunning {
			return fmt.Errorf("JSONL file not yet created at %s (session may still be starting)", jsonlPath)
		}
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

// constructJSONLPathFallback attempts to construct the JSONL path from task state
// when the path wasn't persisted to state. This uses the same path format as llmkit:
// ~/.claude/projects/{normalized-workdir}/{sessionId}.jsonl
func constructJSONLPathFallback(taskID string, st *state.State) (string, error) {
	// Check for session ID in state
	sessionID := st.GetSessionID()

	// If no explicit session ID, try constructing from current phase
	// Session IDs for orc tasks are typically: {taskID}-{phaseID}
	if sessionID == "" && st.CurrentPhase != "" {
		sessionID = fmt.Sprintf("%s-%s", taskID, st.CurrentPhase)
	}

	if sessionID == "" {
		return "", fmt.Errorf("no session ID available")
	}

	// Get home directory for ~/.claude location
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}

	// Try to find the worktree path
	// First, check common worktree location relative to current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	// Look for worktree pattern: .orc/worktrees/orc-{TASK-ID}
	worktreePath := filepath.Join(cwd, ".orc", "worktrees", "orc-"+taskID)
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists, construct JSONL path using worktree as workdir
		normalizedPath := normalizeProjectPath(worktreePath)
		return fmt.Sprintf("%s/.claude/projects/%s/%s.jsonl", homeDir, normalizedPath, sessionID), nil
	}

	// Fallback: use current directory as workdir (non-worktree execution)
	normalizedPath := normalizeProjectPath(cwd)
	jsonlPath := fmt.Sprintf("%s/.claude/projects/%s/%s.jsonl", homeDir, normalizedPath, sessionID)

	// Verify the constructed path exists before returning
	if _, err := os.Stat(jsonlPath); err != nil {
		return "", fmt.Errorf("constructed JSONL path does not exist")
	}

	return jsonlPath, nil
}

// normalizeProjectPath converts an absolute path to Claude Code's normalized format.
// Example: /home/user/repos/project -> -home-user-repos-project
func normalizeProjectPath(path string) string {
	// Remove leading slash and replace remaining slashes with dashes
	normalized := strings.TrimPrefix(path, "/")
	normalized = strings.ReplaceAll(normalized, "/", "-")
	// Prepend dash to match Claude Code's format
	return "-" + normalized
}

// formatFollowError returns an appropriate error message based on task status
// when --follow cannot find a JSONL file.
func formatFollowError(taskID string, st *state.State, constructErr error) error {
	switch st.Status {
	case state.StatusPending:
		return fmt.Errorf("task %s has not started yet (status: pending)", taskID)
	case state.StatusCompleted:
		return fmt.Errorf("task %s has already completed - use 'orc log %s' without --follow to view transcripts", taskID, taskID)
	case state.StatusFailed:
		return fmt.Errorf("task %s has failed - use 'orc log %s' without --follow to view transcripts", taskID, taskID)
	case state.StatusPaused:
		return fmt.Errorf("task %s is paused (not actively running) - use 'orc resume %s' to continue", taskID, taskID)
	case state.StatusInterrupted:
		return fmt.Errorf("task %s was interrupted - use 'orc resume %s' to continue", taskID, taskID)
	case state.StatusRunning:
		// Task claims to be running but no JSONL - might be starting up or executor died
		if constructErr != nil {
			return fmt.Errorf("task %s is running but JSONL file not yet available (session may still be starting): %w", taskID, constructErr)
		}
		return fmt.Errorf("task %s is running but JSONL file not yet available (session may still be starting)", taskID)
	default:
		return fmt.Errorf("no JSONL file available for task %s (task status: %s)", taskID, st.Status)
	}
}
