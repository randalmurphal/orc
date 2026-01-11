// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// ExportData contains all data for a task export.
type ExportData struct {
	Task        *task.Task            `yaml:"task"`
	Plan        *plan.Plan            `yaml:"plan,omitempty"`
	State       *state.State          `yaml:"state,omitempty"`
	Transcripts []db.Transcript       `yaml:"transcripts,omitempty"`
}

// newExportCmd creates the export command
func newExportCmd() *cobra.Command {
	var outputFile string
	var withTranscripts bool
	var withState bool

	cmd := &cobra.Command{
		Use:   "export <task-id>",
		Short: "Export task to YAML",
		Long: `Export a task and its related data to YAML format.

The exported YAML includes:
  • Task definition (ID, title, weight, status)
  • Plan (phases and configuration)
  • State (optional, with --state)
  • Transcripts (optional, with --transcripts)

Use this for:
  • Backing up task data
  • Moving tasks between projects
  • Version controlling task definitions
  • Creating templates from existing tasks

Example:
  orc export TASK-001                    # Output to stdout
  orc export TASK-001 -o task.yaml       # Output to file
  orc export TASK-001 --transcripts      # Include transcripts`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			// Load task
			t, err := task.Load(taskID)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			export := &ExportData{Task: t}

			// Load plan
			p, err := plan.Load(taskID)
			if err == nil {
				export.Plan = p
			}

			// Load state if requested
			if withState {
				s, err := state.Load(taskID)
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
					transcripts, err := pdb.GetTranscripts(taskID)
					if err == nil {
						export.Transcripts = transcripts
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
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file path (default: stdout)")
	cmd.Flags().BoolVar(&withTranscripts, "transcripts", false, "include transcript content")
	cmd.Flags().BoolVar(&withState, "state", false, "include execution state")

	return cmd
}

// newImportCmd creates the import command
func newImportCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import task from YAML",
		Long: `Import a task from a YAML file.

The YAML file should contain task definition exported with 'orc export'.
This creates a new task in the current project.

Use this for:
  • Restoring from backup
  • Copying tasks between projects
  • Creating tasks from templates

Example:
  orc import task.yaml           # Import task from file
  orc import task.yaml --force   # Overwrite if exists
  orc import ./exports/          # Import all YAML files in directory`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			// Check if directory or file
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("stat %s: %w", path, err)
			}

			if info.IsDir() {
				// Import all YAML files in directory
				return importDirectory(path, force)
			}

			return importFile(path, force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing tasks")

	return cmd
}

func importFile(path string, force bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var export ExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if export.Task == nil {
		return fmt.Errorf("no task found in %s", path)
	}

	// Check if task exists
	existing, _ := task.Load(export.Task.ID)
	if existing != nil && !force {
		return fmt.Errorf("task %s already exists (use --force to overwrite)", export.Task.ID)
	}

	// Save task
	if err := export.Task.Save(); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	// Save plan if present
	if export.Plan != nil {
		if err := export.Plan.Save(export.Task.ID); err != nil {
			// Non-fatal
			fmt.Fprintf(os.Stderr, "Warning: could not save plan: %v\n", err)
		}
	}

	// Save state if present
	if export.State != nil {
		if err := export.State.Save(); err != nil {
			// Non-fatal
			fmt.Fprintf(os.Stderr, "Warning: could not save state: %v\n", err)
		}
	}

	// Import transcripts if present
	if len(export.Transcripts) > 0 {
		wd, _ := os.Getwd()
		pdb, err := db.OpenProject(wd)
		if err == nil {
			defer pdb.Close()
			for i := range export.Transcripts {
				pdb.AddTranscript(&export.Transcripts[i])
			}
		}
	}

	fmt.Printf("Imported task %s from %s\n", export.Task.ID, path)
	return nil
}

func importDirectory(dir string, force bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	var imported int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := importFile(path, force); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", path, err)
			continue
		}
		imported++
	}

	if imported == 0 {
		fmt.Println("No YAML files found to import")
	} else {
		fmt.Printf("Imported %d task(s)\n", imported)
	}

	return nil
}
