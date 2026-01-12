// Package cli implements the orc command-line interface.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
)

// newLogCmd creates the log command
func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log <task-id>",
		Short: "Show task transcripts",
		Long: `Show task transcripts with content.

By default, shows the latest transcript. Use flags to customize output.

Examples:
  orc log TASK-001              # Show latest transcript (last 100 lines)
  orc log TASK-001 --all        # Show all transcripts
  orc log TASK-001 --phase test # Show specific phase transcript
  orc log TASK-001 --list       # List transcript files only
  orc log TASK-001 --tail 50    # Show last 50 lines
  orc log TASK-001 --follow     # Stream new lines (like tail -f)`,
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
	defer file.Close()

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

// followFile streams new lines from a file (like tail -f)
func followFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Seek to end
	file.Seek(0, 2)

	fmt.Printf("Following %s (Ctrl+C to stop)...\n\n", filepath.Base(filePath))

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// No new data, wait and retry
			continue
		}
		fmt.Print(line)
	}
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
