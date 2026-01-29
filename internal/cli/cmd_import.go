package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
)

// deferredInitiativeDeps tracks initiative dependencies that couldn't be added
// during import because the referenced initiative hadn't been imported yet.
// After all initiatives are imported, we retry adding these dependencies.
var (
	deferredInitiativeDeps   = make(map[string][]string) // initiativeID -> []dependsOnIDs
	deferredInitiativeDepsMu sync.Mutex
)

// registerDeferredInitiativeDeps records dependencies that failed to import
// so they can be retried after all initiatives are imported.
func registerDeferredInitiativeDeps(initiativeID string, deps []string) {
	deferredInitiativeDepsMu.Lock()
	defer deferredInitiativeDepsMu.Unlock()
	deferredInitiativeDeps[initiativeID] = deps
}

// processDeferredInitiativeDeps retries adding deferred dependencies after
// all initiatives have been imported.
func processDeferredInitiativeDeps() {
	deferredInitiativeDepsMu.Lock()
	defer deferredInitiativeDepsMu.Unlock()

	if len(deferredInitiativeDeps) == 0 {
		return
	}

	backend, err := getBackend()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not process deferred dependencies: %v\n", err)
		return
	}
	defer func() { _ = backend.Close() }()

	var resolved, failed int
	for initID, deps := range deferredInitiativeDeps {
		// Load the initiative and set dependencies
		init, err := backend.LoadInitiative(initID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not load initiative %s for deferred deps: %v\n", initID, err)
			failed++
			continue
		}

		// Check which dependencies now exist
		var validDeps []string
		var missingDeps []string
		for _, depID := range deps {
			exists, _ := backend.InitiativeExists(depID)
			if exists {
				validDeps = append(validDeps, depID)
			} else {
				missingDeps = append(missingDeps, depID)
			}
		}

		if len(missingDeps) > 0 {
			fmt.Fprintf(os.Stderr, "Warning: %s: blocked_by references non-existent initiative(s): %v\n", initID, missingDeps)
		}

		if len(validDeps) > 0 {
			init.BlockedBy = validDeps
			if err := backend.SaveInitiative(init); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not add dependencies to %s: %v\n", initID, err)
				failed++
			} else {
				resolved++
			}
		}
	}

	if resolved > 0 || failed > 0 {
		fmt.Printf("Resolved %d deferred initiative dependencies", resolved)
		if failed > 0 {
			fmt.Printf(", %d failed", failed)
		}
		fmt.Println()
	}

	// Clear the deferred deps map
	deferredInitiativeDeps = make(map[string][]string)
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
				projectRoot, err := config.FindProjectRoot()
				if err != nil {
					return fmt.Errorf("not in an orc project: %w", err)
				}
				exportDir := task.ExportPath(projectRoot)
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

	// Subcommands for external source imports
	cmd.AddCommand(newImportJiraCmd())

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

// importData imports task, initiative, or workflow data with smart merge logic.
func importData(data []byte, sourceName string, force, skipExisting bool) error {
	// First, try to detect the data type
	var typeCheck struct {
		Type string `yaml:"type"`
	}
	if err := yaml.Unmarshal(data, &typeCheck); err == nil {
		switch typeCheck.Type {
		case "initiative":
			return importInitiativeData(data, sourceName, force, skipExisting)
		case "phase_template":
			return importPhaseTemplateData(data, sourceName, force, skipExisting)
		case "workflow":
			return importWorkflowData(data, sourceName, force, skipExisting)
		case "workflow_run":
			return importWorkflowRunData(data, sourceName, force, skipExisting)
		case "project_commands":
			return importProjectCommandsData(data, sourceName, force, skipExisting)
		}
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
	existing, _ := backend.LoadTask(export.Task.Id)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("task %s skipped (--skip-existing)", export.Task.Id)
		}

		if !force {
			// Smart merge: compare updated_at timestamps
			// Local wins on ties (equal timestamps)
			exportTime := time.Time{}
			existingTime := time.Time{}
			if export.Task.UpdatedAt != nil {
				exportTime = export.Task.UpdatedAt.AsTime()
			}
			if existing.UpdatedAt != nil {
				existingTime = existing.UpdatedAt.AsTime()
			}
			if !exportTime.After(existingTime) {
				return fmt.Errorf("task %s skipped (local version is newer or same)", export.Task.Id)
			}
			// Incoming is newer, proceed with import
		}
	}

	// Handle "running" tasks from another machine - they can't actually be running here
	// Set to paused/interrupted so user can resume with 'orc resume'
	wasRunning := false
	if export.Task.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		wasRunning = true
		export.Task.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
		// Clear executor info - it's invalid on this machine
		export.Task.ExecutorPid = 0
		export.Task.ExecutorHostname = nil
		// Update timestamp to reflect this change
		export.Task.UpdatedAt = timestamppb.Now()
		// Note: task.Status is the single source of truth - no state.Status update needed
	}

	// Save task (includes execution state in Task.Execution)
	if err := backend.SaveTask(export.Task); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	// Import transcripts if present (with deduplication by MessageUUID)
	if len(export.Transcripts) > 0 {
		// Get existing transcripts to deduplicate
		existingTranscripts, _ := backend.GetTranscripts(export.Task.Id)
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
			if _, err := backend.SaveAttachment(export.Task.Id, a.Filename, a.ContentType, a.Data); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not import attachment %s: %v\n", a.Filename, err)
			}
		}
	}

	// Import spec if present
	if export.Spec != "" {
		if err := backend.SaveSpecForTask(export.Task.Id, export.Spec, "imported"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not import spec: %v\n", err)
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s task %s from %s", action, export.Task.Id, sourceName)
	if wasRunning {
		fmt.Printf(" (was running, now interrupted - use 'orc resume %s' to continue)", export.Task.Id)
	}
	fmt.Println()
	return nil
}

// importInitiativeData imports an initiative with smart merge logic.
// Dependencies (blocked_by) are deferred and added after the base initiative is saved,
// to handle cases where dependencies are imported in arbitrary order.
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

	// Defer dependencies - save without them first to avoid foreign key issues
	// when initiatives are imported in arbitrary order
	deferredDeps := export.Initiative.BlockedBy
	export.Initiative.BlockedBy = nil

	// Save initiative without dependencies
	if err := backend.SaveInitiative(export.Initiative); err != nil {
		return fmt.Errorf("save initiative: %w", err)
	}

	// Now try to add dependencies - they may fail if referenced initiatives
	// haven't been imported yet; we'll collect these for a second pass
	if len(deferredDeps) > 0 {
		// Restore dependencies and save again to add them
		export.Initiative.BlockedBy = deferredDeps
		if err := backend.SaveInitiative(export.Initiative); err != nil {
			// Dependencies failed - record for deferred processing
			registerDeferredInitiativeDeps(export.Initiative.ID, deferredDeps)
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s initiative %s from %s\n", action, export.Initiative.ID, sourceName)
	return nil
}

// importPhaseTemplateData imports a phase template with smart merge logic.
func importPhaseTemplateData(data []byte, sourceName string, force, skipExisting bool) error {
	var export PhaseTemplateExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if export.PhaseTemplate == nil {
		return fmt.Errorf("no phase template found in %s", sourceName)
	}

	// Skip built-in templates - they exist in every installation
	if export.PhaseTemplate.IsBuiltin {
		return fmt.Errorf("phase template %s skipped (built-in)", export.PhaseTemplate.ID)
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	// Check if phase template exists
	existing, _ := backend.GetPhaseTemplate(export.PhaseTemplate.ID)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("phase template %s skipped (--skip-existing)", export.PhaseTemplate.ID)
		}

		if !force {
			// Smart merge: compare updated_at timestamps
			if !export.PhaseTemplate.UpdatedAt.After(existing.UpdatedAt) {
				return fmt.Errorf("phase template %s skipped (local version is newer or same)", export.PhaseTemplate.ID)
			}
		}
	}

	// Save phase template
	if err := backend.SavePhaseTemplate(export.PhaseTemplate); err != nil {
		return fmt.Errorf("save phase template: %w", err)
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s phase template %s from %s\n", action, export.PhaseTemplate.ID, sourceName)
	return nil
}

// importWorkflowData imports a workflow with its phases and variables.
func importWorkflowData(data []byte, sourceName string, force, skipExisting bool) error {
	var export WorkflowExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if export.Workflow == nil {
		return fmt.Errorf("no workflow found in %s", sourceName)
	}

	// Skip built-in workflows - they exist in every installation
	if export.Workflow.IsBuiltin {
		return fmt.Errorf("workflow %s skipped (built-in)", export.Workflow.ID)
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	// Check if workflow exists
	existing, _ := backend.GetWorkflow(export.Workflow.ID)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("workflow %s skipped (--skip-existing)", export.Workflow.ID)
		}

		if !force {
			// Smart merge: compare updated_at timestamps
			if !export.Workflow.UpdatedAt.After(existing.UpdatedAt) {
				return fmt.Errorf("workflow %s skipped (local version is newer or same)", export.Workflow.ID)
			}
		}
	}

	// Save workflow
	if err := backend.SaveWorkflow(export.Workflow); err != nil {
		return fmt.Errorf("save workflow: %w", err)
	}

	// Save workflow phases
	for _, phase := range export.Phases {
		if err := backend.SaveWorkflowPhase(phase); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save workflow phase %s: %v\n", phase.PhaseTemplateID, err)
		}
	}

	// Save workflow variables
	for _, variable := range export.Variables {
		if err := backend.SaveWorkflowVariable(variable); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save workflow variable %s: %v\n", variable.Name, err)
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s workflow %s from %s\n", action, export.Workflow.ID, sourceName)
	return nil
}

// importWorkflowRunData imports a workflow run with its phases.
func importWorkflowRunData(data []byte, sourceName string, force, skipExisting bool) error {
	var export WorkflowRunExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if export.WorkflowRun == nil {
		return fmt.Errorf("no workflow run found in %s", sourceName)
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	// Check if workflow run exists
	existing, _ := backend.GetWorkflowRun(export.WorkflowRun.ID)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("workflow run %s skipped (--skip-existing)", export.WorkflowRun.ID)
		}

		if !force {
			// Smart merge: compare updated_at timestamps
			if !export.WorkflowRun.UpdatedAt.After(existing.UpdatedAt) {
				return fmt.Errorf("workflow run %s skipped (local version is newer or same)", export.WorkflowRun.ID)
			}
		}
	}

	// Handle "running" workflow runs from another machine
	wasRunning := false
	if export.WorkflowRun.Status == "running" {
		wasRunning = true
		export.WorkflowRun.Status = "paused"
	}

	// Save workflow run
	if err := backend.SaveWorkflowRun(export.WorkflowRun); err != nil {
		return fmt.Errorf("save workflow run: %w", err)
	}

	// Save workflow run phases
	for _, phase := range export.Phases {
		if err := backend.SaveWorkflowRunPhase(phase); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save workflow run phase %s: %v\n", phase.PhaseTemplateID, err)
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s workflow run %s from %s", action, export.WorkflowRun.ID, sourceName)
	if wasRunning {
		fmt.Printf(" (was running, now paused)")
	}
	fmt.Println()
	return nil
}

// importProjectCommandsData imports project commands (quality check commands).
func importProjectCommandsData(data []byte, sourceName string, force, skipExisting bool) error {
	var export ProjectCommandsExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if len(export.Commands) == 0 {
		return fmt.Errorf("no commands found in %s", sourceName)
	}

	// Get project database directly for project commands
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}

	pdb, err := db.OpenProject(projectRoot)
	if err != nil {
		return fmt.Errorf("open project database: %w", err)
	}
	defer func() { _ = pdb.Close() }()

	var imported, skipped int
	for _, cmd := range export.Commands {
		// Check if command exists
		existing, _ := pdb.GetProjectCommand(cmd.Name)
		if existing != nil {
			if skipExisting {
				skipped++
				continue
			}

			if !force {
				// Smart merge: compare updated_at timestamps
				if !cmd.UpdatedAt.After(existing.UpdatedAt) {
					skipped++
					continue
				}
			}
		}

		// Save command
		if err := pdb.SaveProjectCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save command %s: %v\n", cmd.Name, err)
			continue
		}
		imported++
	}

	if imported > 0 || skipped > 0 {
		fmt.Printf("Imported %d project command(s) from %s", imported, sourceName)
		if skipped > 0 {
			fmt.Printf(", skipped %d", skipped)
		}
		fmt.Println()
	}

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

	// Process any deferred initiative dependencies now that all items are imported
	processDeferredInitiativeDeps()

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
		// Detect type - just check if task key exists, actual parsing uses ExportData
		var typeCheck struct {
			Type string `yaml:"type"`
			Task any    `yaml:"task"`
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
			existing, _ := backend.LoadTask(export.Task.Id)
			statusNote := ""
			if export.Task.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
				statusNote = " (running->interrupted)"
			}
			if existing == nil {
				fmt.Printf("  %-20s  [WOULD IMPORT] task %s%s\n", filepath.Base(f.name), export.Task.Id, statusNote)
				wouldImport++
			} else {
				exportTime := time.Time{}
				existingTime := time.Time{}
				if export.Task.UpdatedAt != nil {
					exportTime = export.Task.UpdatedAt.AsTime()
				}
				if existing.UpdatedAt != nil {
					existingTime = existing.UpdatedAt.AsTime()
				}
				if exportTime.After(existingTime) {
					fmt.Printf("  %-20s  [WOULD UPDATE] task %s (incoming newer)%s\n", filepath.Base(f.name), export.Task.Id, statusNote)
					wouldUpdate++
				} else {
					fmt.Printf("  %-20s  [WOULD SKIP]   task %s (local newer or same)\n", filepath.Base(f.name), export.Task.Id)
					wouldSkip++
				}
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

	// Process any deferred initiative dependencies now that all items are imported
	processDeferredInitiativeDeps()

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

	// Process any deferred initiative dependencies now that all items are imported
	processDeferredInitiativeDeps()

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
