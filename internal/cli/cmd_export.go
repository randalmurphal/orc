// Package cli implements the orc command-line interface.
package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ExportData contains all data for a task export.
type ExportData struct {
	Task        *task.Task      `yaml:"task"`
	Plan        *plan.Plan      `yaml:"plan,omitempty"`
	State       *state.State    `yaml:"state,omitempty"`
	Transcripts []db.Transcript `yaml:"transcripts,omitempty"`
}

// newExportCmd creates the export command
func newExportCmd() *cobra.Command {
	var outputFile string
	var withTranscripts bool
	var withState bool
	var withContext bool
	var toBranch bool
	var allData bool
	var allTasks bool

	cmd := &cobra.Command{
		Use:   "export [task-id]",
		Short: "Export task(s) to YAML, directory, or zip archive",
		Long: `Export task(s) and related data.

Export modes:

1. Single Task YAML (default):
   orc export TASK-001                    # YAML to stdout
   orc export TASK-001 -o task.yaml       # YAML to file

2. All Tasks:
   orc export --all-tasks -o ./backup/    # All tasks to directory
   orc export --all-tasks -o tasks.zip    # All tasks to zip archive

3. Branch Export (--to-branch):
   orc export TASK-001 --to-branch        # Export to .orc/exports/

Data options:
   --state        Include execution state
   --transcripts  Include transcript content
   --all          Include all data (state + transcripts + context)

Examples:
  orc export TASK-001                        # Single task to stdout
  orc export TASK-001 -o task.yaml           # Single task to file
  orc export --all-tasks -o ./backup/        # All tasks to directory
  orc export --all-tasks -o tasks.zip        # All tasks to zip archive
  orc export --all-tasks --all -o backup.zip # All tasks with all data`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --all is set, enable all export data flags
			if allData {
				withState = true
				withTranscripts = true
				withContext = true
			}

			// Export all tasks
			if allTasks {
				if outputFile == "" {
					return fmt.Errorf("--all-tasks requires -o/--output (directory or .zip file)")
				}
				return exportAllTasks(outputFile, withState, withTranscripts)
			}

			// Single task export requires task ID
			if len(args) == 0 {
				return fmt.Errorf("task ID required (or use --all-tasks to export all)")
			}
			taskID := args[0]

			// Branch export mode
			if toBranch {
				return exportToBranchDir(taskID, withState, withTranscripts, withContext)
			}

			// Legacy YAML export mode
			return exportToYAML(taskID, outputFile, withState, withTranscripts)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file/directory path (default: stdout)")
	cmd.Flags().BoolVar(&withTranscripts, "transcripts", false, "include transcript content")
	cmd.Flags().BoolVar(&withState, "state", false, "include execution state")
	cmd.Flags().BoolVar(&withContext, "context", false, "include context.md summary")
	cmd.Flags().BoolVar(&toBranch, "to-branch", false, "export to .orc/exports/ directory")
	cmd.Flags().BoolVar(&allData, "all", false, "export all available data for task(s)")
	cmd.Flags().BoolVar(&allTasks, "all-tasks", false, "export all tasks")

	return cmd
}

// exportToYAML performs the legacy YAML export for backup/migration.
func exportToYAML(taskID, outputFile string, withState, withTranscripts bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer backend.Close()

	// Load task
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return fmt.Errorf("load task: %w", err)
	}

	export := &ExportData{Task: t}

	// Load plan
	p, err := backend.LoadPlan(taskID)
	if err == nil {
		export.Plan = p
	} else {
		fmt.Fprintf(os.Stderr, "Warning: could not load plan: %v\n", err)
	}

	// Load state if requested
	if withState {
		s, err := backend.LoadState(taskID)
		if err == nil {
			export.State = s
		} else {
			fmt.Fprintf(os.Stderr, "Warning: could not load state: %v\n", err)
		}
	}

	// Load transcripts if requested
	if withTranscripts {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not get working directory for transcripts: %v\n", err)
		} else {
			pdb, err := db.OpenProject(wd)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not open database for transcripts: %v\n", err)
			} else {
				defer pdb.Close()
				transcripts, err := pdb.GetTranscripts(taskID)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not load transcripts: %v\n", err)
				} else {
					export.Transcripts = transcripts
				}
			}
		}
	}

	// Marshal to YAML
	data, err := yaml.Marshal(export)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	// Output
	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Exported task %s to %s\n", taskID, outputFile)
	} else {
		fmt.Print(string(data))
	}

	return nil
}

// exportToBranchDir exports task artifacts to .orc/exports/ using the storage package.
func exportToBranchDir(taskID string, withState, withTranscripts, withContext bool) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Get project path
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Create storage backend
	backend, err := storage.NewBackend(cwd, &cfg.Storage)
	if err != nil {
		return fmt.Errorf("create storage backend: %w", err)
	}
	defer backend.Close()

	// Create export service
	exportSvc := storage.NewExportService(backend, &cfg.Storage)

	// Build export options
	opts := &storage.ExportOptions{
		TaskDefinition: true, // Always include task definition
		FinalState:     withState,
		Transcripts:    withTranscripts,
		ContextSummary: withContext,
	}

	// Perform export
	if err := exportSvc.Export(taskID, opts); err != nil {
		return fmt.Errorf("export: %w", err)
	}

	fmt.Printf("Exported task %s to .orc/exports/%s/\n", taskID, taskID)
	return nil
}

// newImportCmd creates the import command
func newImportCmd() *cobra.Command {
	var force bool
	var skipExisting bool

	cmd := &cobra.Command{
		Use:   "import <file|directory|archive>",
		Short: "Import task(s) from YAML, directory, or zip archive",
		Long: `Import task(s) from YAML file, directory, or zip archive.

Smart merge behavior (default):
  • New tasks are imported
  • Existing tasks: import only if incoming has newer updated_at
  • Use --force to always overwrite
  • Use --skip-existing to never overwrite

Examples:
  orc import task.yaml              # Import single task (smart merge)
  orc import ./backup/              # Import all YAML from directory
  orc import tasks.zip              # Import all from zip archive
  orc import tasks.zip --force      # Always overwrite existing
  orc import tasks.zip --skip-existing  # Never overwrite existing`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			// Check if zip file
			if strings.HasSuffix(strings.ToLower(path), ".zip") {
				return importZip(path, force, skipExisting)
			}

			// Check if directory or file
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("stat %s: %w", path, err)
			}

			if info.IsDir() {
				return importDirectory(path, force, skipExisting)
			}

			return importFileWithMerge(path, force, skipExisting)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "always overwrite existing tasks")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "never overwrite existing tasks")

	return cmd
}

// importFileWithMerge imports a task with smart merge logic.
// - force: always overwrite existing
// - skipExisting: never overwrite existing
// - default (both false): overwrite only if incoming is newer
func importFileWithMerge(path string, force, skipExisting bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	return importData(data, path, force, skipExisting)
}

// importData imports task data with smart merge logic.
func importData(data []byte, sourceName string, force, skipExisting bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer backend.Close()

	var export ExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if export.Task == nil {
		return fmt.Errorf("no task found in %s", sourceName)
	}

	// Check if task exists
	existing, _ := backend.LoadTask(export.Task.ID)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("task %s skipped (--skip-existing)", export.Task.ID)
		}

		if !force {
			// Smart merge: compare updated_at timestamps
			if !export.Task.UpdatedAt.After(existing.UpdatedAt) {
				return fmt.Errorf("task %s skipped (local version is newer or same)", export.Task.ID)
			}
			// Incoming is newer, proceed with import
		}
	}

	// Save task
	if err := backend.SaveTask(export.Task); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	// Save plan if present
	if export.Plan != nil {
		if err := backend.SavePlan(export.Plan, export.Task.ID); err != nil {
			// Non-fatal
			fmt.Fprintf(os.Stderr, "Warning: could not save plan: %v\n", err)
		}
	}

	// Save state if present
	if export.State != nil {
		if err := backend.SaveState(export.State); err != nil {
			// Non-fatal
			fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
		}
	}

	// Import transcripts if present
	if len(export.Transcripts) > 0 {
		wd, _ := os.Getwd()
		pdb, err := db.OpenProject(wd)
		if err == nil {
			defer func() { _ = pdb.Close() }()
			for i := range export.Transcripts {
				_ = pdb.AddTranscript(&export.Transcripts[i])
			}
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s task %s from %s\n", action, export.Task.ID, sourceName)
	return nil
}

// importZip imports all tasks from a zip archive.
func importZip(zipPath string, force, skipExisting bool) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	var imported, skipped int
	for _, f := range r.File {
		// Skip directories and non-YAML files
		if f.FileInfo().IsDir() {
			continue
		}
		ext := filepath.Ext(f.Name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		// Read file from zip
		rc, err := f.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: open error: %v\n", f.Name, err)
			continue
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: read error: %v\n", f.Name, err)
			continue
		}

		// Import the data
		if err := importData(data, f.Name, force, skipExisting); err != nil {
			if strings.Contains(err.Error(), "skipped") {
				skipped++
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", f.Name, err)
			continue
		}
		imported++
	}

	if imported == 0 && skipped == 0 {
		fmt.Println("No YAML files found in archive")
	} else {
		fmt.Printf("Imported %d task(s) from %s", imported, zipPath)
		if skipped > 0 {
			fmt.Printf(", skipped %d", skipped)
		}
		fmt.Println()
	}

	return nil
}

func importDirectory(dir string, force, skipExisting bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	var imported, skipped int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := importFileWithMerge(path, force, skipExisting); err != nil {
			if strings.Contains(err.Error(), "skipped") {
				skipped++
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", path, err)
			continue
		}
		imported++
	}

	if imported == 0 && skipped == 0 {
		fmt.Println("No YAML files found to import")
	} else {
		fmt.Printf("Imported %d task(s)", imported)
		if skipped > 0 {
			fmt.Printf(", skipped %d (newer local version)", skipped)
		}
		fmt.Println()
	}

	return nil
}

// exportAllTasks exports all tasks to a directory or zip file.
func exportAllTasks(outputPath string, withState, withTranscripts bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer backend.Close()

	// Load all tasks
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks to export")
		return nil
	}

	// Check if output is a zip file
	isZip := strings.HasSuffix(strings.ToLower(outputPath), ".zip")

	if isZip {
		return exportAllTasksToZipWithBackend(backend, tasks, outputPath, withState, withTranscripts)
	}
	return exportAllTasksToDirWithBackend(backend, tasks, outputPath, withState, withTranscripts)
}

// exportAllTasksToDirWithBackend exports all tasks to a directory.
func exportAllTasksToDirWithBackend(backend storage.Backend, tasks []*task.Task, dir string, withState, withTranscripts bool) error {
	// Create output directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	var exported int
	for _, t := range tasks {
		export := buildExportDataWithBackend(backend, t, withState, withTranscripts)

		data, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", t.ID, err)
			continue
		}

		filename := filepath.Join(dir, t.ID+".yaml")
		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", t.ID, err)
			continue
		}
		exported++
	}

	fmt.Printf("Exported %d task(s) to %s\n", exported, dir)
	return nil
}

// exportAllTasksToZipWithBackend exports all tasks to a zip archive.
func exportAllTasksToZipWithBackend(backend storage.Backend, tasks []*task.Task, zipPath string, withState, withTranscripts bool) error {
	// Create zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	var exported int
	for _, t := range tasks {
		export := buildExportDataWithBackend(backend, t, withState, withTranscripts)

		data, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", t.ID, err)
			continue
		}

		// Add file to zip
		w, err := zipWriter.Create(t.ID + ".yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: zip error: %v\n", t.ID, err)
			continue
		}

		if _, err := w.Write(data); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", t.ID, err)
			continue
		}
		exported++
	}

	fmt.Printf("Exported %d task(s) to %s\n", exported, zipPath)
	return nil
}

// buildExportDataWithBackend creates ExportData for a task using the backend.
func buildExportDataWithBackend(backend storage.Backend, t *task.Task, withState, withTranscripts bool) *ExportData {
	export := &ExportData{Task: t}

	// Load plan
	p, err := backend.LoadPlan(t.ID)
	if err == nil {
		export.Plan = p
	}

	// Load state if requested
	if withState {
		s, err := backend.LoadState(t.ID)
		if err == nil {
			export.State = s
		}
	}

	// Load transcripts if requested
	if withTranscripts {
		wd, _ := os.Getwd()
		pdb, err := db.OpenProject(wd)
		if err == nil {
			defer pdb.Close()
			transcripts, err := pdb.GetTranscripts(t.ID)
			if err == nil {
				export.Transcripts = transcripts
			}
		}
	}

	return export
}
