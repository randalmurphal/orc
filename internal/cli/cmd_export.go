// Package cli implements the orc command-line interface.
package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ExportFormatVersion is the current version of the export format.
// Increment when making breaking changes to ExportData.
const ExportFormatVersion = 2

// ExportData contains all data for a task export.
type ExportData struct {
	// Metadata for format versioning
	Version    int       `yaml:"version"`
	ExportedAt time.Time `yaml:"exported_at"`

	// Core task data
	Task  *task.Task   `yaml:"task"`
	Plan  *plan.Plan   `yaml:"plan,omitempty"`
	Spec  string       `yaml:"spec,omitempty"`
	State *state.State `yaml:"state,omitempty"`

	// Execution history
	Transcripts   []storage.Transcript   `yaml:"transcripts,omitempty"`
	GateDecisions []storage.GateDecision `yaml:"gate_decisions,omitempty"`

	// Collaboration data
	TaskComments   []storage.TaskComment   `yaml:"task_comments,omitempty"`
	ReviewComments []storage.ReviewComment `yaml:"review_comments,omitempty"`

	// Attachments (binary data base64 encoded in YAML)
	Attachments []AttachmentExport `yaml:"attachments,omitempty"`
}

// AttachmentExport represents an attachment for export.
type AttachmentExport struct {
	Filename    string `yaml:"filename"`
	ContentType string `yaml:"content_type"`
	SizeBytes   int64  `yaml:"size_bytes"`
	IsImage     bool   `yaml:"is_image"`
	Data        []byte `yaml:"data"` // base64 encoded in YAML
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
	var withInitiatives bool

	cmd := &cobra.Command{
		Use:   "export [task-id|init-id]",
		Short: "Export task(s) and initiative(s) for cross-machine portability",
		Long: `Export task(s) and initiative(s) with all related data.

The default export location is .orc/exports/ - this is where import looks by default.

Export modes:

1. Single Task YAML:
   orc export TASK-001                    # YAML to stdout
   orc export TASK-001 -o task.yaml       # YAML to file

2. All Tasks (full backup):
   orc export --all-tasks                 # All tasks to .orc/exports/
   orc export --all-tasks -o backup.zip   # All tasks to zip archive

3. With Initiatives:
   orc export --all-tasks --initiatives   # Tasks + initiatives to .orc/exports/

Data options:
   --state        Include execution state and gate decisions
   --transcripts  Include full transcript content
   --all          Include all data (state + transcripts + context)
   --initiatives  Include all initiatives with decisions and task links

What gets exported:
  - Task definition, plan, spec, state
  - Transcripts (execution history)
  - Comments (task and review)
  - Attachments (binary files)
  - Gate decisions
  - Initiative vision and decisions (with --initiatives)

Examples:
  orc export --all-tasks --all                # Full backup to .orc/exports/
  orc export --all-tasks --all -o backup.zip  # Full backup to zip
  orc export TASK-001 --all -o task.yaml      # Single task with all data`,
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
				// Default to .orc/exports/ if no output specified
				if outputFile == "" {
					wd, err := os.Getwd()
					if err != nil {
						return fmt.Errorf("get working directory: %w", err)
					}
					outputFile = task.ExportPath(wd)
				}
				if err := exportAllTasks(outputFile, withState, withTranscripts); err != nil {
					return err
				}
				// Also export initiatives if requested
				if withInitiatives {
					return exportAllInitiatives(outputFile, withState)
				}
				return nil
			}

			// Single task export requires task ID
			if len(args) == 0 {
				return fmt.Errorf("task ID required (or use --all-tasks to export all)")
			}
			taskID := args[0]

			// Check if it's an initiative ID
			if strings.HasPrefix(taskID, "INIT-") {
				return exportInitiative(taskID, outputFile, withState)
			}

			// Branch export mode
			if toBranch {
				return exportToBranchDir(taskID, withState, withTranscripts, withContext)
			}

			// YAML export mode
			return exportToYAML(taskID, outputFile, withState, withTranscripts)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "output path (default: .orc/exports/ for --all-tasks, stdout for single)")
	cmd.Flags().BoolVar(&withTranscripts, "transcripts", false, "include transcript content")
	cmd.Flags().BoolVar(&withState, "state", false, "include execution state")
	cmd.Flags().BoolVar(&withContext, "context", false, "include context.md summary")
	cmd.Flags().BoolVar(&toBranch, "to-branch", false, "export to .orc/exports/ directory")
	cmd.Flags().BoolVar(&allData, "all", false, "export all available data")
	cmd.Flags().BoolVar(&allTasks, "all-tasks", false, "export all tasks")
	cmd.Flags().BoolVar(&withInitiatives, "initiatives", false, "include initiatives (with --all-tasks)")

	return cmd
}

// exportToYAML performs the YAML export for backup/migration.
func exportToYAML(taskID, outputFile string, withState, withTranscripts bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	// Load task
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return fmt.Errorf("load task: %w", err)
	}

	// Build full export data
	export := buildExportDataWithBackend(backend, t, withState, withTranscripts)

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
	defer func() { _ = backend.Close() }()

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
		Use:   "import [path]",
		Short: "Import task(s) and initiative(s) from YAML, directory, or zip archive",
		Long: `Import task(s) and initiative(s) from YAML file, directory, or zip archive.

Default location: .orc/exports/ (where export places files by default)

Smart merge behavior (default):
  • New items are imported
  • Existing items: import only if incoming has newer updated_at
  • Use --force to always overwrite
  • Use --skip-existing to never overwrite

Supports:
  - Task YAML files (detected by task field)
  - Initiative YAML files (detected by type: initiative)
  - Directories containing both
  - Zip archives

Examples:
  orc import                        # Import from default .orc/exports/
  orc import task.yaml              # Import single task (smart merge)
  orc import ./backup/              # Import all YAML from directory
  orc import tasks.zip              # Import all from zip archive
  orc import tasks.zip --force      # Always overwrite existing
  orc import tasks.zip --skip-existing  # Never overwrite existing`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var path string
			if len(args) == 0 {
				// Default to .orc/exports/
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("get working directory: %w", err)
				}
				path = task.ExportPath(wd)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					return fmt.Errorf("default export directory not found: %s\nUse 'orc export --all-tasks' first or specify a path", path)
				}
			} else {
				path = args[0]
			}

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

	cmd.Flags().BoolVar(&force, "force", false, "always overwrite existing items")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "never overwrite existing items")

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

// importData imports task or initiative data with smart merge logic.
func importData(data []byte, sourceName string, force, skipExisting bool) error {
	// First, try to detect if this is an initiative export
	var typeCheck struct {
		Type string `yaml:"type"`
	}
	if err := yaml.Unmarshal(data, &typeCheck); err == nil && typeCheck.Type == "initiative" {
		return importInitiativeData(data, sourceName, force, skipExisting)
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

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
		for i := range export.Transcripts {
			if err := backend.AddTranscript(&export.Transcripts[i]); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not import transcript: %v\n", err)
			}
		}
	}

	// Import gate decisions if present
	if len(export.GateDecisions) > 0 {
		for i := range export.GateDecisions {
			if err := backend.SaveGateDecision(&export.GateDecisions[i]); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not import gate decision: %v\n", err)
			}
		}
	}

	// Import task comments if present
	if len(export.TaskComments) > 0 {
		for i := range export.TaskComments {
			if err := backend.SaveTaskComment(&export.TaskComments[i]); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not import task comment: %v\n", err)
			}
		}
	}

	// Import review comments if present
	if len(export.ReviewComments) > 0 {
		for i := range export.ReviewComments {
			if err := backend.SaveReviewComment(&export.ReviewComments[i]); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not import review comment: %v\n", err)
			}
		}
	}

	// Import attachments if present
	if len(export.Attachments) > 0 {
		for _, a := range export.Attachments {
			if _, err := backend.SaveAttachment(export.Task.ID, a.Filename, a.ContentType, a.Data); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not import attachment %s: %v\n", a.Filename, err)
			}
		}
	}

	// Import spec if present
	if export.Spec != "" {
		if err := backend.SaveSpec(export.Task.ID, export.Spec, "imported"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not import spec: %v\n", err)
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s task %s from %s\n", action, export.Task.ID, sourceName)
	return nil
}

// importInitiativeData imports an initiative with smart merge logic.
func importInitiativeData(data []byte, sourceName string, force, skipExisting bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	var export InitiativeExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if export.Initiative == nil {
		return fmt.Errorf("no initiative found in %s", sourceName)
	}

	// Check if initiative exists
	existing, _ := backend.LoadInitiative(export.Initiative.ID)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("initiative %s skipped (--skip-existing)", export.Initiative.ID)
		}

		if !force {
			// Smart merge: compare updated_at timestamps
			if !export.Initiative.UpdatedAt.After(existing.UpdatedAt) {
				return fmt.Errorf("initiative %s skipped (local version is newer or same)", export.Initiative.ID)
			}
			// Incoming is newer, proceed with import
		}
	}

	// Save initiative
	if err := backend.SaveInitiative(export.Initiative); err != nil {
		return fmt.Errorf("save initiative: %w", err)
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s initiative %s from %s\n", action, export.Initiative.ID, sourceName)
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
		_ = rc.Close()
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
	var tasksImported, tasksSkipped int
	var initiativesImported, initiativesSkipped int

	// Import YAML files in the root directory (tasks)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Check for initiatives subdirectory
			if entry.Name() == "initiatives" {
				initDir := filepath.Join(dir, "initiatives")
				imported, skipped := importInitiativesFromDir(initDir, force, skipExisting)
				initiativesImported += imported
				initiativesSkipped += skipped
			}
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := importFileWithMerge(path, force, skipExisting); err != nil {
			if strings.Contains(err.Error(), "skipped") {
				tasksSkipped++
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", path, err)
			continue
		}
		tasksImported++
	}

	// Print summary
	if tasksImported == 0 && tasksSkipped == 0 && initiativesImported == 0 && initiativesSkipped == 0 {
		fmt.Println("No YAML files found to import")
	} else {
		if tasksImported > 0 || tasksSkipped > 0 {
			fmt.Printf("Imported %d task(s)", tasksImported)
			if tasksSkipped > 0 {
				fmt.Printf(", skipped %d (newer local version)", tasksSkipped)
			}
			fmt.Println()
		}
		if initiativesImported > 0 || initiativesSkipped > 0 {
			fmt.Printf("Imported %d initiative(s)", initiativesImported)
			if initiativesSkipped > 0 {
				fmt.Printf(", skipped %d (newer local version)", initiativesSkipped)
			}
			fmt.Println()
		}
	}

	return nil
}

// importInitiativesFromDir imports all initiatives from a directory.
func importInitiativesFromDir(dir string, force, skipExisting bool) (imported, skipped int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read initiatives directory: %v\n", err)
		return 0, 0
	}

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

	return imported, skipped
}

// exportAllTasks exports all tasks to a directory or zip file.
func exportAllTasks(outputPath string, withState, withTranscripts bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

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
	defer func() { _ = zipFile.Close() }()

	zipWriter := zip.NewWriter(zipFile)
	defer func() { _ = zipWriter.Close() }()

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
// When withAll is true, includes all data (state, transcripts, comments, attachments, etc.)
func buildExportDataWithBackend(backend storage.Backend, t *task.Task, withState, withTranscripts bool) *ExportData {
	export := &ExportData{
		Version:    ExportFormatVersion,
		ExportedAt: time.Now(),
		Task:       t,
	}

	// Always load plan
	if p, err := backend.LoadPlan(t.ID); err == nil {
		export.Plan = p
	}

	// Always load spec
	if spec, err := backend.LoadSpec(t.ID); err == nil {
		export.Spec = spec
	}

	// Load state if requested
	if withState {
		if s, err := backend.LoadState(t.ID); err == nil {
			export.State = s
		}

		// Also load gate decisions when exporting state
		if decisions, err := backend.ListGateDecisions(t.ID); err == nil {
			export.GateDecisions = decisions
		}
	}

	// Load transcripts if requested
	if withTranscripts {
		if transcripts, err := backend.GetTranscripts(t.ID); err == nil {
			export.Transcripts = transcripts
		}
	}

	// Always load collaboration data (small, important for context)
	if comments, err := backend.ListTaskComments(t.ID); err == nil {
		export.TaskComments = comments
	}
	if reviews, err := backend.ListReviewComments(t.ID); err == nil {
		export.ReviewComments = reviews
	}

	// Always load attachments (with data)
	if attachments, err := backend.ListAttachments(t.ID); err == nil {
		export.Attachments = make([]AttachmentExport, 0, len(attachments))
		for _, a := range attachments {
			// Get attachment data
			_, data, err := backend.GetAttachment(t.ID, a.Filename)
			if err != nil {
				continue // Skip attachments we can't read
			}
			// Check if it's an image by content type
			isImage := strings.HasPrefix(a.ContentType, "image/")
			export.Attachments = append(export.Attachments, AttachmentExport{
				Filename:    a.Filename,
				ContentType: a.ContentType,
				SizeBytes:   a.Size,
				IsImage:     isImage,
				Data:        data,
			})
		}
	}

	return export
}

// InitiativeExportData contains all data for an initiative export.
type InitiativeExportData struct {
	Version    int       `yaml:"version"`
	ExportedAt time.Time `yaml:"exported_at"`
	Type       string    `yaml:"type"` // "initiative" to distinguish from task exports

	Initiative *initiative.Initiative `yaml:"initiative"`
}

// exportInitiative exports a single initiative to YAML.
func exportInitiative(initID, outputFile string, withState bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	init, err := backend.LoadInitiative(initID)
	if err != nil {
		return fmt.Errorf("load initiative: %w", err)
	}

	export := &InitiativeExportData{
		Version:    ExportFormatVersion,
		ExportedAt: time.Now(),
		Type:       "initiative",
		Initiative: init,
	}

	data, err := yaml.Marshal(export)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, data, 0644); err != nil {
			return fmt.Errorf("write file: %w", err)
		}
		fmt.Printf("Exported initiative %s to %s\n", initID, outputFile)
	} else {
		fmt.Print(string(data))
	}

	return nil
}

// exportAllInitiatives exports all initiatives to a directory or zip.
func exportAllInitiatives(outputPath string, withState bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	initiatives, err := backend.LoadAllInitiatives()
	if err != nil {
		return fmt.Errorf("load initiatives: %w", err)
	}

	if len(initiatives) == 0 {
		fmt.Println("No initiatives to export")
		return nil
	}

	// Check if output is a zip file
	isZip := strings.HasSuffix(strings.ToLower(outputPath), ".zip")

	var exported int
	if isZip {
		// For zip, we need to append to the existing archive or create new
		// For simplicity, export initiatives to a subdirectory
		fmt.Printf("Note: Initiative export to zip not yet implemented, exporting to directory instead\n")
	}

	// Export to directory
	initDir := filepath.Join(outputPath, "initiatives")
	if err := os.MkdirAll(initDir, 0755); err != nil {
		return fmt.Errorf("create initiatives directory: %w", err)
	}

	for _, init := range initiatives {
		export := &InitiativeExportData{
			Version:    ExportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "initiative",
			Initiative: init,
		}

		data, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", init.ID, err)
			continue
		}

		filename := filepath.Join(initDir, init.ID+".yaml")
		if err := os.WriteFile(filename, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", init.ID, err)
			continue
		}
		exported++
	}

	fmt.Printf("Exported %d initiative(s) to %s\n", exported, initDir)
	return nil
}
