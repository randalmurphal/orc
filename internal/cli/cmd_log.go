// Package cli implements the orc command-line interface.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
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
  Default     Shows last 100 lines of the most recent transcript
  --all       Shows all transcripts for all phases
  --phase     Filter to a specific phase (implement, test, etc.)
  --list      List available transcript files without showing content
  --follow    Real-time streaming as Claude writes (like tail -f)

Quality tips:
  • When debugging a failed task, start with the latest transcript
  • Use --phase to find specific work (e.g., --phase test for test phase)
  • Use --follow during execution to watch Claude work in real-time
  • Transcripts are stored in .orc/tasks/TASK-XXX/transcripts/

Examples:
  orc log TASK-001              # Latest transcript (last 100 lines)
  orc log TASK-001 --all        # All transcripts, all phases
  orc log TASK-001 --phase test # Just the test phase transcript
  orc log TASK-001 --list       # List available transcripts
  orc log TASK-001 --tail 50    # Last 50 lines only
  orc log TASK-001 --tail 0     # Full transcript (no limit)
  orc log TASK-001 --follow     # Stream new lines in real-time

See also:
  orc show TASK-001 --session   # View session stats (tokens, timing)
  orc diff TASK-001             # View code changes made by task`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]
			listOnly, _ := cmd.Flags().GetBool("list")
			phase, _ := cmd.Flags().GetString("phase")
			all, _ := cmd.Flags().GetBool("all")
			tail, _ := cmd.Flags().GetInt("tail")
			follow, _ := cmd.Flags().GetBool("follow")

			transcriptsDir := fmt.Sprintf(".orc/tasks/%s/transcripts", id)
			entries, err := os.ReadDir(transcriptsDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("No transcripts found for task %s\n", id)
					fmt.Println("\nThe task may not have run yet, or transcripts were not saved.")
					fmt.Printf("Try: orc run %s\n", id)
					return nil
				}
				return fmt.Errorf("read transcripts: %w", err)
			}

			if len(entries) == 0 {
				fmt.Printf("No transcripts found for task %s\n", id)
				return nil
			}

			// Sort by modification time (newest last)
			sort.Slice(entries, func(i, j int) bool {
				iInfo, _ := entries[i].Info()
				jInfo, _ := entries[j].Info()
				if iInfo == nil || jInfo == nil {
					return entries[i].Name() < entries[j].Name()
				}
				return iInfo.ModTime().Before(jInfo.ModTime())
			})

			// List mode - just show files
			if listOnly {
				fmt.Printf("Transcripts for %s:\n\n", id)
				for _, entry := range entries {
					info, _ := entry.Info()
					size := "?"
					if info != nil {
						size = formatSize(info.Size())
					}
					fmt.Printf("  %s  (%s)\n", entry.Name(), size)
				}
				fmt.Printf("\nUse: orc log %s --phase <name> to view content\n", id)
				return nil
			}

			// Determine which files to show
			var filesToShow []string
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()

				// Filter by phase if specified
				if phase != "" {
					if !strings.Contains(strings.ToLower(name), strings.ToLower(phase)) {
						continue
					}
				}

				filesToShow = append(filesToShow, filepath.Join(transcriptsDir, name))
			}

			if len(filesToShow) == 0 {
				if phase != "" {
					fmt.Printf("No transcripts found for phase '%s'\n", phase)
					fmt.Println("\nAvailable transcripts:")
					for _, entry := range entries {
						fmt.Printf("  %s\n", entry.Name())
					}
				}
				return nil
			}

			// If not --all, only show the latest
			if !all && len(filesToShow) > 1 {
				filesToShow = filesToShow[len(filesToShow)-1:]
			}

			// Follow mode - stream new lines
			if follow {
				if len(filesToShow) > 1 {
					fmt.Println("Follow mode only works with a single file. Using latest.")
					filesToShow = filesToShow[len(filesToShow)-1:]
				}
				return followFile(filesToShow[0])
			}

			// Show content
			for i, filePath := range filesToShow {
				if len(filesToShow) > 1 {
					fmt.Printf("─── %s ───\n", filepath.Base(filePath))
				}

				if err := showFileContent(filePath, tail); err != nil {
					fmt.Printf("Error reading %s: %v\n", filePath, err)
					continue
				}

				if i < len(filesToShow)-1 {
					fmt.Println()
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolP("list", "l", false, "list transcript files only (don't show content)")
	cmd.Flags().StringP("phase", "p", "", "filter to specific phase (e.g., 'implement', 'test')")
	cmd.Flags().BoolP("all", "a", false, "show all transcripts (not just latest)")
	cmd.Flags().IntP("tail", "n", 100, "number of lines to show (0 for all)")
	cmd.Flags().BoolP("follow", "f", false, "stream new lines as they are written")

	return cmd
}

// showFileContent displays the content of a transcript file
func showFileContent(filePath string, tailLines int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	if tailLines == 0 {
		// Show all lines
		scanner := bufio.NewScanner(file)
		// Increase buffer size for long lines
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
		return scanner.Err()
	}

	// Tail mode - read last N lines
	lines := make([]string, 0, tailLines)
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > tailLines {
			lines = lines[1:]
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	for _, line := range lines {
		fmt.Println(line)
	}

	return nil
}

// followFile streams new lines from a file using fsnotify for real-time updates.
// Falls back to polling with proper delays if fsnotify fails.
func followFile(filePath string) error {
	// Set up context with signal handling for clean shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nStopping...")
		cancel()
	}()

	// Try fsnotify first, fall back to polling
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return followFilePolling(ctx, filePath)
	}
	defer func() { _ = watcher.Close() }()

	// Watch the directory (more reliable than watching file directly)
	dir := filepath.Dir(filePath)
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return followFilePolling(ctx, filePath)
	}

	return followFileWithWatcher(ctx, filePath, watcher)
}

// followFileWithWatcher uses fsnotify for efficient real-time streaming.
func followFileWithWatcher(ctx context.Context, filePath string, watcher *fsnotify.Watcher) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Seek to end to only show new content
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seek to end: %w", err)
	}

	fmt.Printf("Following %s (Ctrl+C to stop)...\n\n", filepath.Base(filePath))

	baseName := filepath.Base(filePath)
	reader := bufio.NewReader(file)
	var partialLine strings.Builder

	for {
		select {
		case <-ctx.Done():
			// Print any remaining partial line before exit
			if partialLine.Len() > 0 {
				fmt.Println(partialLine.String())
			}
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only care about writes to our file
			if filepath.Base(event.Name) != baseName {
				continue
			}
			if !event.Has(fsnotify.Write) {
				continue
			}

			// Check if file was truncated (offset beyond current size)
			info, err := file.Stat()
			if err != nil {
				continue
			}
			if info.Size() < offset {
				// File was truncated, reset to beginning
				_, _ = file.Seek(0, io.SeekStart)
				offset = 0
				reader.Reset(file)
				partialLine.Reset()
				fmt.Println("[file truncated, reading from start]")
			}

			// Read new content
			offset = readNewContent(reader, &partialLine, offset)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			// Log error but continue - fsnotify errors are usually recoverable
			fmt.Fprintf(os.Stderr, "[watcher error: %v]\n", err)
		}
	}
}

// followFilePolling is a fallback that uses polling with proper delays.
func followFilePolling(ctx context.Context, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Seek to end
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seek to end: %w", err)
	}

	fmt.Printf("Following %s (polling mode, Ctrl+C to stop)...\n\n", filepath.Base(filePath))

	reader := bufio.NewReader(file)
	var partialLine strings.Builder

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if partialLine.Len() > 0 {
				fmt.Println(partialLine.String())
			}
			return nil

		case <-ticker.C:
			// Check for truncation
			info, err := file.Stat()
			if err != nil {
				continue
			}
			if info.Size() < offset {
				_, _ = file.Seek(0, io.SeekStart)
				offset = 0
				reader.Reset(file)
				partialLine.Reset()
				fmt.Println("[file truncated, reading from start]")
			}

			// Read new content
			offset = readNewContent(reader, &partialLine, offset)
		}
	}
}

// readNewContent reads available content from the file and prints complete lines.
// Returns the new offset position.
func readNewContent(reader *bufio.Reader, partialLine *strings.Builder, offset int64) int64 {
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			offset += int64(len(line))
			if strings.HasSuffix(line, "\n") {
				// Complete line - print with any partial content
				if partialLine.Len() > 0 {
					fmt.Print(partialLine.String())
					partialLine.Reset()
				}
				fmt.Print(line)
			} else {
				// Partial line - buffer it
				partialLine.WriteString(line)
			}
		}
		if err != nil {
			// EOF or error - stop reading for now
			break
		}
	}
	return offset
}

// formatSize returns a human-readable file size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
