package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// Note: task import is used for task.ExportPath() utility function.

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
	var withWorkflows bool
	var noRuns bool
	var minimal bool
	var noState bool
	var format string

	cmd := &cobra.Command{
		Use:   "export [task-id|init-id]",
		Short: "Export task(s), initiative(s), and workflows for cross-machine portability",
		Long: `Export task(s), initiative(s), and workflows with all related data.

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

3. With Initiatives and Workflows:
   orc export --all-tasks --initiatives      # Tasks + initiatives
   orc export --all-tasks --workflows        # Tasks + custom workflows + phase templates

Data options:
   --minimal      Lightweight export: exclude transcripts and large attachments
   --no-state     Exclude execution state (not recommended for active tasks)
   --all          Include context.md summary (in addition to defaults)
   --initiatives  Include all initiatives with decisions and task links
   --workflows    Include custom workflows, phase templates, and workflow runs
   --no-runs      Exclude workflow runs (useful for smaller exports)

What gets exported (by default):
  - Task definition, plan, spec
  - Execution state (phases, progress, gates)
  - Transcripts (conversation history)
  - Comments (task and review)
  - Attachments (binary files)
  - Initiative vision and decisions (with --initiatives)
  - Custom workflows and phase templates (with --workflows)

Examples:
  orc export --all-tasks                      # Full backup (recommended)
  orc export --all-tasks --minimal            # Smaller backup without transcripts
  orc export --all-tasks --initiatives        # Include initiatives
  orc export --all-tasks --workflows          # Include custom workflows
  orc export --all-tasks --workflows --no-runs  # Workflows without run history
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
				wd, err := ResolveProjectPath()
				if err != nil {
					return fmt.Errorf("not in an orc project: %w", err)
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

				opts := exportAllOptions{
					withState:       withState,
					withTranscripts: withTranscripts,
					withInitiatives: withInitiatives,
					withWorkflows:   withWorkflows,
					withRuns:        !noRuns, // --no-runs inverts this
				}
				if err := exportAllTasks(outputFile, format, opts); err != nil {
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
	cmd.Flags().BoolVar(&withWorkflows, "workflows", false, "include custom workflows and phase templates (with --all-tasks)")
	cmd.Flags().BoolVar(&noRuns, "no-runs", false, "exclude workflow runs (with --workflows)")
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

	// Get project path (worktree-aware)
	projectRoot, err := ResolveProjectPath()
	if err != nil {
		return fmt.Errorf("not in an orc project: %w", err)
	}

	// Create storage backend
	backend, err := storage.NewBackend(projectRoot, &cfg.Storage)
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
