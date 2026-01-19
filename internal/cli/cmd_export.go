// Package cli implements the orc command-line interface.
package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
// Version 3: state and transcripts included by default, tar.gz support
const ExportFormatVersion = 3

// maxImportFileSize is the maximum size of a single file to import (100MB).
// This prevents zip/tar bomb attacks that could exhaust memory.
const maxImportFileSize = 100 * 1024 * 1024

// ExportManifest contains metadata about an export archive.
type ExportManifest struct {
	Version          int       `yaml:"version"`
	ExportedAt       time.Time `yaml:"exported_at"`
	SourceHostname   string    `yaml:"source_hostname"`
	SourceProject    string    `yaml:"source_project,omitempty"`
	OrcVersion       string    `yaml:"orc_version,omitempty"`
	TaskCount        int       `yaml:"task_count"`
	InitiativeCount  int       `yaml:"initiative_count"`
	IncludesState    bool      `yaml:"includes_state"`
	IncludesTranscripts bool   `yaml:"includes_transcripts"`
}

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
	var minimal bool
	var noState bool
	var format string

	cmd := &cobra.Command{
		Use:   "export [task-id|init-id]",
		Short: "Export task(s) and initiative(s) for cross-machine portability",
		Long: `Export task(s) and initiative(s) with all related data.

By default, exports include ALL data (state, transcripts, comments, attachments).
This ensures in-progress tasks can be resumed on another machine.

Export modes:

1. All Tasks (recommended for backup/migration):
   orc export --all-tasks                    # tar.gz archive to .orc/exports/
   orc export --all-tasks --format=zip       # zip archive
   orc export --all-tasks --format=dir       # directory of YAML files

2. Single Task:
   orc export TASK-001                       # YAML to stdout (full data)
   orc export TASK-001 -o task.yaml          # YAML to file

3. With Initiatives:
   orc export --all-tasks --initiatives      # Tasks + initiatives

Data options:
   --minimal      Lightweight export: exclude transcripts and large attachments
   --no-state     Exclude execution state (not recommended for active tasks)
   --all          Include context.md summary (in addition to defaults)
   --initiatives  Include all initiatives with decisions and task links

What gets exported (by default):
  - Task definition, plan, spec
  - Execution state (phases, progress, gates)
  - Transcripts (conversation history)
  - Comments (task and review)
  - Attachments (binary files)
  - Initiative vision and decisions (with --initiatives)

Examples:
  orc export --all-tasks                      # Full backup (recommended)
  orc export --all-tasks --minimal            # Smaller backup without transcripts
  orc export --all-tasks --initiatives        # Include initiatives
  orc export TASK-001 -o task.yaml            # Single task export`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default: export everything (state + transcripts)
			// This ensures cross-machine portability preserves execution state
			withState = true
			withTranscripts = true

			// --minimal: lightweight export (no transcripts/attachments)
			if minimal {
				withTranscripts = false
			}

			// --no-state: explicitly exclude execution state
			if noState {
				withState = false
			}

			// --all: all data including context
			if allData {
				withState = true
				withTranscripts = true
				withContext = true
			}

			// Export all tasks
			if allTasks {
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("get working directory: %w", err)
				}

				// Generate default output path based on format
				if outputFile == "" {
					timestamp := time.Now().Format("20060102-150405")
					switch format {
					case "tar.gz", "tgz":
						outputFile = filepath.Join(task.ExportPath(wd), fmt.Sprintf("orc-export-%s.tar.gz", timestamp))
					case "zip":
						outputFile = filepath.Join(task.ExportPath(wd), fmt.Sprintf("orc-export-%s.zip", timestamp))
					case "dir":
						outputFile = task.ExportPath(wd)
					default:
						return fmt.Errorf("unknown format %q: use tar.gz, zip, or dir", format)
					}
				}

				if err := exportAllTasks(outputFile, format, withState, withTranscripts, withInitiatives); err != nil {
					return err
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

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "output path (default: tar.gz to .orc/exports/ for --all-tasks, stdout for single)")
	cmd.Flags().StringVar(&format, "format", "tar.gz", "output format: tar.gz (default), zip, dir")
	cmd.Flags().BoolVar(&minimal, "minimal", false, "lightweight export: exclude transcripts and large attachments")
	cmd.Flags().BoolVar(&noState, "no-state", false, "exclude execution state (not recommended for cross-machine)")
	cmd.Flags().BoolVar(&withContext, "context", false, "include context.md summary")
	cmd.Flags().BoolVar(&toBranch, "to-branch", false, "export to .orc/exports/ directory structure")
	cmd.Flags().BoolVar(&allData, "all", false, "export all available data including context")
	cmd.Flags().BoolVar(&allTasks, "all-tasks", false, "export all tasks")
	cmd.Flags().BoolVar(&withInitiatives, "initiatives", false, "include initiatives (with --all-tasks)")
	// Keep old flags as hidden for backwards compat
	cmd.Flags().BoolVar(&withTranscripts, "transcripts", false, "include transcripts (default: true)")
	cmd.Flags().BoolVar(&withState, "state", false, "include state (default: true)")
	_ = cmd.Flags().MarkHidden("transcripts")
	_ = cmd.Flags().MarkHidden("state")

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
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "import [path]",
		Short: "Import task(s) and initiative(s) from archive, directory, or YAML file",
		Long: `Import task(s) and initiative(s) from tar.gz, zip, directory, or YAML file.

Default location: .orc/exports/ (where export places files by default)

Smart merge behavior (default):
  • New items are imported
  • Existing items: import only if incoming has newer updated_at
  • Local wins on ties (equal timestamps)
  • "Running" tasks from another machine are imported as "interrupted"
  • Use --force to always overwrite
  • Use --skip-existing to never overwrite

Supports (auto-detected):
  - tar.gz archives (recommended, default export format)
  - Zip archives
  - Directories containing YAML files
  - Single YAML files (task or initiative)

Examples:
  orc import                            # Import from default .orc/exports/
  orc import backup.tar.gz              # Import from tar.gz archive
  orc import backup.zip                 # Import from zip archive
  orc import ./backup/                  # Import from directory
  orc import task.yaml                  # Import single task
  orc import backup.tar.gz --dry-run    # Preview what would be imported
  orc import backup.tar.gz --force      # Always overwrite existing
  orc import --skip-existing            # Never overwrite existing`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var path string
			if len(args) == 0 {
				// Default to .orc/exports/ - find most recent archive or use directory
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("get working directory: %w", err)
				}
				exportDir := task.ExportPath(wd)
				path, err = findLatestExport(exportDir)
				if err != nil {
					return err
				}
			} else {
				path = args[0]
			}

			// Auto-detect format by extension and magic bytes
			format, err := detectImportFormat(path)
			if err != nil {
				return fmt.Errorf("detect format: %w", err)
			}

			if dryRun {
				return importDryRun(path, format)
			}

			switch format {
			case "tar.gz":
				return importTarGz(path, force, skipExisting)
			case "zip":
				return importZip(path, force, skipExisting)
			case "dir":
				return importDirectory(path, force, skipExisting)
			case "yaml":
				return importFileWithMerge(path, force, skipExisting)
			default:
				return fmt.Errorf("unsupported import format: %s", format)
			}
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "always overwrite existing items")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "never overwrite existing items")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be imported without making changes")

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
			// Local wins on ties (equal timestamps)
			if !export.Task.UpdatedAt.After(existing.UpdatedAt) {
				return fmt.Errorf("task %s skipped (local version is newer or same)", export.Task.ID)
			}
			// Incoming is newer, proceed with import
		}
	}

	// Handle "running" tasks from another machine - they can't actually be running here
	// Set to paused/interrupted so user can resume with 'orc resume'
	wasRunning := false
	if export.Task.Status == task.StatusRunning {
		wasRunning = true
		export.Task.Status = task.StatusPaused
		// Update timestamp to reflect this change
		export.Task.UpdatedAt = time.Now()

		// Also update state if present
		if export.State != nil {
			export.State.Status = state.StatusInterrupted
			// Clear execution info - it's invalid on this machine
			export.State.Execution = nil
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
			// State is critical for active tasks - fail if we can't save it
			// Note: wasRunning tracks if task was originally running (now paused)
			if wasRunning || export.Task.Status == task.StatusPaused {
				return fmt.Errorf("save state for active task: %w", err)
			}
			// Non-fatal for completed/failed tasks
			fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
		}
	}

	// Import transcripts if present (with deduplication by MessageUUID)
	if len(export.Transcripts) > 0 {
		// Get existing transcripts to deduplicate
		existingTranscripts, _ := backend.GetTranscripts(export.Task.ID)
		transcriptKeys := make(map[string]bool)
		for _, t := range existingTranscripts {
			// Use MessageUUID for deduplication (unique per message in JSONL)
			if t.MessageUUID != "" {
				transcriptKeys[t.MessageUUID] = true
			}
		}

		var imported, skipped int
		for i := range export.Transcripts {
			t := &export.Transcripts[i]
			// Skip if we already have this message
			if t.MessageUUID != "" && transcriptKeys[t.MessageUUID] {
				skipped++
				continue // Skip duplicate
			}
			if err := backend.AddTranscript(t); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not import transcript: %v\n", err)
			} else {
				imported++
				if t.MessageUUID != "" {
					transcriptKeys[t.MessageUUID] = true
				}
			}
		}
		if skipped > 0 {
			fmt.Fprintf(os.Stderr, "Info: skipped %d duplicate transcript(s)\n", skipped)
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
	fmt.Printf("%s task %s from %s", action, export.Task.ID, sourceName)
	if wasRunning {
		fmt.Printf(" (was running, now interrupted - use 'orc resume %s' to continue)", export.Task.ID)
	}
	fmt.Println()
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

// findLatestExport finds the most recent export file or falls back to directory.
func findLatestExport(exportDir string) (string, error) {
	// Check if export directory exists
	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		return "", fmt.Errorf("export directory not found: %s\nUse 'orc export --all-tasks' first or specify a path", exportDir)
	}

	// Look for tar.gz and zip files, find the most recent
	entries, err := os.ReadDir(exportDir)
	if err != nil {
		return "", fmt.Errorf("read export directory: %w", err)
	}

	var latestArchive string
	var latestTime time.Time

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".zip") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if latestArchive == "" || info.ModTime().After(latestTime) {
				latestArchive = filepath.Join(exportDir, entry.Name())
				latestTime = info.ModTime()
			}
		}
	}

	if latestArchive != "" {
		return latestArchive, nil
	}

	// No archives found, use directory itself
	return exportDir, nil
}

// detectImportFormat detects the import format from file extension and magic bytes.
func detectImportFormat(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}

	if info.IsDir() {
		return "dir", nil
	}

	// Check extension first
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return "tar.gz", nil
	}
	if strings.HasSuffix(lower, ".zip") {
		return "zip", nil
	}
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		return "yaml", nil
	}

	// Check magic bytes
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		return "", fmt.Errorf("read magic bytes: %w", err)
	}

	// gzip magic: 1f 8b
	if magic[0] == 0x1f && magic[1] == 0x8b {
		return "tar.gz", nil
	}
	// zip magic: 50 4b (PK)
	if magic[0] == 0x50 && magic[1] == 0x4b {
		return "zip", nil
	}

	// Assume YAML if it starts with common YAML patterns
	return "yaml", nil
}

// importTarGz imports all tasks and initiatives from a tar.gz archive.
func importTarGz(archivePath string, force, skipExisting bool) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(gzipReader)

	var imported, skipped int
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		// Skip directories and non-YAML files
		if header.Typeflag == tar.TypeDir {
			continue
		}
		ext := filepath.Ext(header.Name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		// Skip manifest.yaml - it's metadata only
		if filepath.Base(header.Name) == "manifest.yaml" {
			continue
		}

		// Read file content (with size limit to prevent tar bombs)
		data, err := io.ReadAll(io.LimitReader(tarReader, maxImportFileSize))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: read error: %v\n", header.Name, err)
			continue
		}

		// Import the data
		if err := importData(data, header.Name, force, skipExisting); err != nil {
			if strings.Contains(err.Error(), "skipped") {
				skipped++
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", header.Name, err)
			continue
		}
		imported++
	}

	if imported == 0 && skipped == 0 {
		fmt.Println("No YAML files found in archive")
	} else {
		fmt.Printf("Imported %d item(s) from %s", imported, archivePath)
		if skipped > 0 {
			fmt.Printf(", skipped %d", skipped)
		}
		fmt.Println()
	}

	return nil
}

// importDryRun previews what would be imported without making changes.
func importDryRun(path, format string) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	fmt.Printf("Dry run - previewing import from %s (format: %s)\n\n", path, format)

	var files []struct {
		name string
		data []byte
	}

	// Collect files based on format
	switch format {
	case "tar.gz":
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open archive: %w", err)
		}
		defer func() { _ = file.Close() }()

		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("create gzip reader: %w", err)
		}
		defer func() { _ = gzipReader.Close() }()

		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("read tar: %w", err)
			}
			if header.Typeflag == tar.TypeDir {
				continue
			}
			ext := filepath.Ext(header.Name)
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			if filepath.Base(header.Name) == "manifest.yaml" {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(tarReader, maxImportFileSize))
			if err != nil {
				continue
			}
			files = append(files, struct {
				name string
				data []byte
			}{header.Name, data})
		}

	case "zip":
		r, err := zip.OpenReader(path)
		if err != nil {
			return fmt.Errorf("open zip: %w", err)
		}
		defer func() { _ = r.Close() }()

		for _, f := range r.File {
			if f.FileInfo().IsDir() {
				continue
			}
			ext := filepath.Ext(f.Name)
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			if filepath.Base(f.Name) == "manifest.yaml" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(rc, maxImportFileSize))
			_ = rc.Close()
			if err != nil {
				continue
			}
			files = append(files, struct {
				name string
				data []byte
			}{f.Name, data})
		}

	case "yaml":
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		files = append(files, struct {
			name string
			data []byte
		}{path, data})

	case "dir":
		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := filepath.Ext(p)
			if ext != ".yaml" && ext != ".yml" {
				return nil
			}
			if filepath.Base(p) == "manifest.yaml" {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			files = append(files, struct {
				name string
				data []byte
			}{p, data})
			return nil
		})
		if err != nil {
			return fmt.Errorf("walk directory: %w", err)
		}
	}

	// Analyze each file
	var wouldImport, wouldUpdate, wouldSkip int
	for _, f := range files {
		// Detect type
		var typeCheck struct {
			Type string     `yaml:"type"`
			Task *task.Task `yaml:"task"`
		}
		if err := yaml.Unmarshal(f.data, &typeCheck); err != nil {
			fmt.Printf("  %-20s  [ERROR: %v]\n", filepath.Base(f.name), err)
			continue
		}

		if typeCheck.Type == "initiative" {
			var export InitiativeExportData
			if err := yaml.Unmarshal(f.data, &export); err != nil {
				fmt.Printf("  %-20s  [ERROR: %v]\n", filepath.Base(f.name), err)
				continue
			}
			existing, _ := backend.LoadInitiative(export.Initiative.ID)
			if existing == nil {
				fmt.Printf("  %-20s  [WOULD IMPORT] initiative %s\n", filepath.Base(f.name), export.Initiative.ID)
				wouldImport++
			} else if export.Initiative.UpdatedAt.After(existing.UpdatedAt) {
				fmt.Printf("  %-20s  [WOULD UPDATE] initiative %s (incoming newer)\n", filepath.Base(f.name), export.Initiative.ID)
				wouldUpdate++
			} else {
				fmt.Printf("  %-20s  [WOULD SKIP]   initiative %s (local newer or same)\n", filepath.Base(f.name), export.Initiative.ID)
				wouldSkip++
			}
		} else if typeCheck.Task != nil {
			var export ExportData
			if err := yaml.Unmarshal(f.data, &export); err != nil {
				fmt.Printf("  %-20s  [ERROR: %v]\n", filepath.Base(f.name), err)
				continue
			}
			existing, _ := backend.LoadTask(export.Task.ID)
			statusNote := ""
			if export.Task.Status == task.StatusRunning {
				statusNote = " (running→interrupted)"
			}
			if existing == nil {
				fmt.Printf("  %-20s  [WOULD IMPORT] task %s%s\n", filepath.Base(f.name), export.Task.ID, statusNote)
				wouldImport++
			} else if export.Task.UpdatedAt.After(existing.UpdatedAt) {
				fmt.Printf("  %-20s  [WOULD UPDATE] task %s (incoming newer)%s\n", filepath.Base(f.name), export.Task.ID, statusNote)
				wouldUpdate++
			} else {
				fmt.Printf("  %-20s  [WOULD SKIP]   task %s (local newer or same)\n", filepath.Base(f.name), export.Task.ID)
				wouldSkip++
			}
		}
	}

	fmt.Printf("\nSummary: %d would import, %d would update, %d would skip\n", wouldImport, wouldUpdate, wouldSkip)
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
		// Skip manifest.yaml - it's metadata only
		if filepath.Base(f.Name) == "manifest.yaml" {
			continue
		}

		// Read file from zip (with size limit to prevent zip bombs)
		rc, err := f.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: open error: %v\n", f.Name, err)
			continue
		}

		data, err := io.ReadAll(io.LimitReader(rc, maxImportFileSize))
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

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	// Check for subdirectory structure (tasks/, initiatives/)
	hasTasksDir := false
	hasInitiativesDir := false
	for _, entry := range entries {
		if entry.IsDir() {
			switch entry.Name() {
			case "tasks":
				hasTasksDir = true
			case "initiatives":
				hasInitiativesDir = true
			}
		}
	}

	// Import from tasks/ subdirectory if it exists (v3 format)
	if hasTasksDir {
		tasksDir := filepath.Join(dir, "tasks")
		imported, skipped := importTasksFromDir(tasksDir, force, skipExisting)
		tasksImported += imported
		tasksSkipped += skipped
	}

	// Import from initiatives/ subdirectory if it exists
	if hasInitiativesDir {
		initDir := filepath.Join(dir, "initiatives")
		imported, skipped := importInitiativesFromDir(initDir, force, skipExisting)
		initiativesImported += imported
		initiativesSkipped += skipped
	}

	// Also import YAML files in root directory (v2 format or single files)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		// Skip manifest
		if entry.Name() == "manifest.yaml" {
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

// importTasksFromDir imports all tasks from a directory.
func importTasksFromDir(dir string, force, skipExisting bool) (imported, skipped int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read tasks directory: %v\n", err)
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

// exportAllTasks exports all tasks to a directory, zip, or tar.gz archive.
func exportAllTasks(outputPath, format string, withState, withTranscripts, withInitiatives bool) error {
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

	// Load initiatives if requested
	var initiatives []*initiative.Initiative
	if withInitiatives {
		initiatives, err = backend.LoadAllInitiatives()
		if err != nil {
			return fmt.Errorf("load initiatives: %w", err)
		}
	}

	if len(tasks) == 0 && len(initiatives) == 0 {
		fmt.Println("No tasks or initiatives to export")
		return nil
	}

	// Detect format from filename if not using explicit format
	if format == "" {
		lower := strings.ToLower(outputPath)
		switch {
		case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
			format = "tar.gz"
		case strings.HasSuffix(lower, ".zip"):
			format = "zip"
		default:
			format = "dir"
		}
	}

	switch format {
	case "tar.gz", "tgz":
		return exportAllToTarGz(backend, tasks, initiatives, outputPath, withState, withTranscripts)
	case "zip":
		return exportAllToZip(backend, tasks, initiatives, outputPath, withState, withTranscripts)
	case "dir":
		return exportAllToDir(backend, tasks, initiatives, outputPath, withState, withTranscripts)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// buildManifest creates an export manifest with metadata.
func buildManifest(taskCount, initCount int, withState, withTranscripts bool) *ExportManifest {
	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()

	return &ExportManifest{
		Version:             ExportFormatVersion,
		ExportedAt:          time.Now(),
		SourceHostname:      hostname,
		SourceProject:       cwd,
		OrcVersion:          runtime.Version(), // Go version as proxy for now
		TaskCount:           taskCount,
		InitiativeCount:     initCount,
		IncludesState:       withState,
		IncludesTranscripts: withTranscripts,
	}
}

// exportAllToTarGz exports all tasks and initiatives to a tar.gz archive.
func exportAllToTarGz(backend storage.Backend, tasks []*task.Task, initiatives []*initiative.Initiative, archivePath string, withState, withTranscripts bool) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Create the tar.gz file
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	gzipWriter := gzip.NewWriter(file)
	defer func() { _ = gzipWriter.Close() }()

	tarWriter := tar.NewWriter(gzipWriter)
	defer func() { _ = tarWriter.Close() }()

	// Write manifest first
	manifest := buildManifest(len(tasks), len(initiatives), withState, withTranscripts)
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := writeTarFile(tarWriter, "manifest.yaml", manifestData); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Export tasks
	var exported int
	for _, t := range tasks {
		export := buildExportDataWithBackend(backend, t, withState, withTranscripts)
		data, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", t.ID, err)
			continue
		}
		if err := writeTarFile(tarWriter, filepath.Join("tasks", t.ID+".yaml"), data); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", t.ID, err)
			continue
		}
		exported++
	}

	// Export initiatives
	var initExported int
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
		if err := writeTarFile(tarWriter, filepath.Join("initiatives", init.ID+".yaml"), data); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", init.ID, err)
			continue
		}
		initExported++
	}

	fmt.Printf("Exported %d task(s)", exported)
	if initExported > 0 {
		fmt.Printf(" and %d initiative(s)", initExported)
	}
	fmt.Printf(" to %s\n", archivePath)
	return nil
}

// writeTarFile writes a single file to a tar archive.
func writeTarFile(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Mode:    0644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

// exportAllToZip exports all tasks and initiatives to a zip archive.
func exportAllToZip(backend storage.Backend, tasks []*task.Task, initiatives []*initiative.Initiative, zipPath string, withState, withTranscripts bool) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer func() { _ = zipFile.Close() }()

	zipWriter := zip.NewWriter(zipFile)
	defer func() { _ = zipWriter.Close() }()

	// Write manifest
	manifest := buildManifest(len(tasks), len(initiatives), withState, withTranscripts)
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := writeZipFile(zipWriter, "manifest.yaml", manifestData); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Export tasks
	var exported int
	for _, t := range tasks {
		export := buildExportDataWithBackend(backend, t, withState, withTranscripts)
		data, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", t.ID, err)
			continue
		}
		if err := writeZipFile(zipWriter, filepath.Join("tasks", t.ID+".yaml"), data); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", t.ID, err)
			continue
		}
		exported++
	}

	// Export initiatives
	var initExported int
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
		if err := writeZipFile(zipWriter, filepath.Join("initiatives", init.ID+".yaml"), data); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", init.ID, err)
			continue
		}
		initExported++
	}

	fmt.Printf("Exported %d task(s)", exported)
	if initExported > 0 {
		fmt.Printf(" and %d initiative(s)", initExported)
	}
	fmt.Printf(" to %s\n", zipPath)
	return nil
}

// writeZipFile writes a single file to a zip archive.
func writeZipFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// exportAllToDir exports all tasks and initiatives to a directory.
func exportAllToDir(backend storage.Backend, tasks []*task.Task, initiatives []*initiative.Initiative, dir string, withState, withTranscripts bool) error {
	// Create output directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Write manifest
	manifest := buildManifest(len(tasks), len(initiatives), withState, withTranscripts)
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), manifestData, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Create tasks subdirectory
	tasksDir := filepath.Join(dir, "tasks")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return fmt.Errorf("create tasks directory: %w", err)
	}

	var exported int
	for _, t := range tasks {
		export := buildExportDataWithBackend(backend, t, withState, withTranscripts)
		data, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", t.ID, err)
			continue
		}
		if err := os.WriteFile(filepath.Join(tasksDir, t.ID+".yaml"), data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", t.ID, err)
			continue
		}
		exported++
	}

	// Export initiatives if any
	var initExported int
	if len(initiatives) > 0 {
		initDir := filepath.Join(dir, "initiatives")
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
			if err := os.WriteFile(filepath.Join(initDir, init.ID+".yaml"), data, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", init.ID, err)
				continue
			}
			initExported++
		}
	}

	fmt.Printf("Exported %d task(s)", exported)
	if initExported > 0 {
		fmt.Printf(" and %d initiative(s)", initExported)
	}
	fmt.Printf(" to %s\n", dir)
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

