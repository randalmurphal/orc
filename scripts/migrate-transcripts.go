// Script to migrate file-based transcripts to SQLite database.
// Run with: go run scripts/migrate-transcripts.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
)

func main() {
	// Find .orc directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting working directory: %v\n", err)
		os.Exit(1)
	}

	orcDir := filepath.Join(wd, ".orc")
	if _, err := os.Stat(orcDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "No .orc directory found in %s\n", wd)
		os.Exit(1)
	}

	// Open database (OpenProject takes project path, not db path)
	projectDB, err := db.OpenProject(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = projectDB.Close() }()

	// Find all transcript files
	tasksDir := filepath.Join(orcDir, "tasks")
	pattern := filepath.Join(tasksDir, "TASK-*", "transcripts", "*.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding transcript files: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No transcript files found to migrate")
		return
	}

	fmt.Printf("Found %d transcript files to migrate\n", len(files))

	// Regex to parse filename: phase-iteration.md
	filenameRe := regexp.MustCompile(`^([a-z]+)-(\d+)\.md$`)

	var migrated, skipped, errors int

	for _, file := range files {
		// Extract task ID from path
		// Path format: .orc/tasks/TASK-XXX/transcripts/phase-NNN.md
		parts := strings.Split(file, string(os.PathSeparator))
		var taskID string
		for i, p := range parts {
			if strings.HasPrefix(p, "TASK-") && i+1 < len(parts) && parts[i+1] == "transcripts" {
				taskID = p
				break
			}
		}

		if taskID == "" {
			fmt.Fprintf(os.Stderr, "Warning: could not extract task ID from %s\n", file)
			errors++
			continue
		}

		// Parse filename
		filename := filepath.Base(file)
		matches := filenameRe.FindStringSubmatch(filename)
		if matches == nil {
			fmt.Fprintf(os.Stderr, "Warning: could not parse filename %s\n", filename)
			errors++
			continue
		}

		phase := matches[1]
		iteration, _ := strconv.Atoi(matches[2])

		// Read file content
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", file, err)
			errors++
			continue
		}

		// Get file modification time for timestamp
		info, err := os.Stat(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not stat %s: %v\n", file, err)
			errors++
			continue
		}

		// Check if transcript already exists in DB
		existing, err := projectDB.GetTranscripts(taskID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not check existing transcripts for %s: %v\n", taskID, err)
		}

		// Check if this specific phase/iteration already exists
		alreadyExists := false
		for _, t := range existing {
			if t.Phase == phase && t.Iteration == iteration {
				alreadyExists = true
				break
			}
		}

		if alreadyExists {
			skipped++
			continue
		}

		// Insert transcript
		transcript := &db.Transcript{
			TaskID:    taskID,
			Phase:     phase,
			Iteration: iteration,
			Role:      "combined",
			Content:   string(content),
			Timestamp: info.ModTime(),
		}

		if err := projectDB.AddTranscript(transcript); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not insert transcript for %s %s-%d: %v\n", taskID, phase, iteration, err)
			errors++
			continue
		}

		migrated++
	}

	fmt.Printf("\nMigration complete:\n")
	fmt.Printf("  Migrated: %d\n", migrated)
	fmt.Printf("  Skipped (already in DB): %d\n", skipped)
	fmt.Printf("  Errors: %d\n", errors)
}
