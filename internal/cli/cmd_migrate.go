// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// newMigrateCmd creates the migrate command
func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate data between storage formats",
		Long: `Migrate data between YAML files and database storage.

Commands:
  yaml-to-db    Migrate existing YAML files to database
  db-to-yaml    Export database to YAML files for backup`,
	}

	cmd.AddCommand(newMigrateYAMLToDBCmd())
	cmd.AddCommand(newMigrateDBToYAMLCmd())
	cmd.AddCommand(newMigratePlansCmd())

	return cmd
}

// newMigrateYAMLToDBCmd creates the yaml-to-db subcommand
func newMigrateYAMLToDBCmd() *cobra.Command {
	var dryRun bool
	var deleteAfter bool

	cmd := &cobra.Command{
		Use:   "yaml-to-db",
		Short: "Migrate YAML files to database",
		Long: `Migrate existing YAML files (.orc/tasks/, .orc/initiatives/) to database storage.

This is a one-time migration for transitioning to pure database storage.
Existing tasks, plans, states, specs, and initiatives are imported.

Examples:
  orc migrate yaml-to-db              # Migrate and keep YAML files
  orc migrate yaml-to-db --dry-run    # Preview what would be migrated
  orc migrate yaml-to-db --delete     # Migrate and delete YAML files after`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrateYAMLToDB(dryRun, deleteAfter)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview migration without making changes")
	cmd.Flags().BoolVar(&deleteAfter, "delete", false, "delete YAML files after successful migration")

	return cmd
}

// newMigrateDBToYAMLCmd creates the db-to-yaml subcommand
func newMigrateDBToYAMLCmd() *cobra.Command {
	var outputDir string

	cmd := &cobra.Command{
		Use:   "db-to-yaml",
		Short: "Export database to YAML files",
		Long: `Export all data from database to YAML files for backup or inspection.

Examples:
  orc migrate db-to-yaml                      # Export to ./backup/
  orc migrate db-to-yaml -o /path/to/backup   # Export to specific directory`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMigrateDBToYAML(outputDir)
		},
	}

	cmd.Flags().StringVarP(&outputDir, "output", "o", "./backup", "output directory for YAML files")

	return cmd
}

func runMigrateYAMLToDB(dryRun, deleteAfter bool) error {
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("not in an orc project: %w", err)
	}

	// Check if .orc exists (FindProjectRoot already validates this, but keep for clarity)
	orcDir := filepath.Join(projectRoot, ".orc")
	if _, err := os.Stat(orcDir); os.IsNotExist(err) {
		return fmt.Errorf("no .orc directory found - run 'orc init' first")
	}

	// Create backend (this initializes the database)
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var backend storage.Backend
	if !dryRun {
		backend, err = storage.NewBackend(projectRoot, &cfg.Storage)
		if err != nil {
			return fmt.Errorf("create backend: %w", err)
		}
		defer func() { _ = backend.Close() }()
	}

	// Track migration stats
	var stats struct {
		Tasks       int
		Plans       int
		States      int
		Specs       int
		Initiatives int
		Attachments int
	}

	// Migrate tasks
	tasksDir := filepath.Join(orcDir, "tasks")
	if entries, err := os.ReadDir(tasksDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			taskID := entry.Name()
			taskDir := filepath.Join(tasksDir, taskID)

			// Load task.yaml
			taskPath := filepath.Join(taskDir, "task.yaml")
			if data, err := os.ReadFile(taskPath); err == nil {
				var t task.Task
				if err := yaml.Unmarshal(data, &t); err == nil {
					if dryRun {
						fmt.Printf("  Would migrate task: %s - %s\n", t.ID, t.Title)
					} else {
						if err := backend.SaveTask(&t); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to migrate task %s: %v\n", taskID, err)
							continue
						}
					}
					stats.Tasks++
				}
			}

			// Load plan.yaml
			planPath := filepath.Join(taskDir, "plan.yaml")
			if data, err := os.ReadFile(planPath); err == nil {
				var p plan.Plan
				if err := yaml.Unmarshal(data, &p); err == nil {
					if !dryRun {
						if err := backend.SavePlan(&p, taskID); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to migrate plan for %s: %v\n", taskID, err)
						} else {
							stats.Plans++
						}
					} else {
						stats.Plans++
					}
				}
			}

			// Load state.yaml
			statePath := filepath.Join(taskDir, "state.yaml")
			if data, err := os.ReadFile(statePath); err == nil {
				var s state.State
				if err := yaml.Unmarshal(data, &s); err == nil {
					if !dryRun {
						if err := backend.SaveState(&s); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to migrate state for %s: %v\n", taskID, err)
						} else {
							stats.States++
						}
					} else {
						stats.States++
					}
				}
			}

			// Load spec.md
			specPath := filepath.Join(taskDir, "spec.md")
			if data, err := os.ReadFile(specPath); err == nil {
				if !dryRun {
					if err := backend.SaveSpec(taskID, string(data), "migrated"); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to migrate spec for %s: %v\n", taskID, err)
					} else {
						stats.Specs++
					}
				} else {
					stats.Specs++
				}
			}

			// Migrate attachments
			attachmentsDir := filepath.Join(taskDir, "attachments")
			if files, err := os.ReadDir(attachmentsDir); err == nil {
				for _, file := range files {
					if file.IsDir() {
						continue
					}
					filePath := filepath.Join(attachmentsDir, file.Name())
					if data, err := os.ReadFile(filePath); err == nil {
						if !dryRun {
							contentType := task.DetectContentType(file.Name())
							if _, err := backend.SaveAttachment(taskID, file.Name(), contentType, data); err != nil {
								fmt.Fprintf(os.Stderr, "Warning: failed to migrate attachment %s/%s: %v\n", taskID, file.Name(), err)
							} else {
								stats.Attachments++
							}
						} else {
							stats.Attachments++
						}
					}
				}
			}
		}
	}

	// Migrate initiatives
	initiativesDir := filepath.Join(orcDir, "initiatives")
	if entries, err := os.ReadDir(initiativesDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			initID := entry.Name()
			initPath := filepath.Join(initiativesDir, initID, "initiative.yaml")
			if data, err := os.ReadFile(initPath); err == nil {
				var init initiative.Initiative
				if err := yaml.Unmarshal(data, &init); err == nil {
					if dryRun {
						fmt.Printf("  Would migrate initiative: %s - %s\n", init.ID, init.Title)
					} else {
						if err := backend.SaveInitiative(&init); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: failed to migrate initiative %s: %v\n", initID, err)
							continue
						}
					}
					stats.Initiatives++
				}
			}
		}
	}

	// Print summary
	if dryRun {
		fmt.Println("\nDry run complete. Would migrate:")
	} else {
		fmt.Println("\nMigration complete:")
	}
	fmt.Printf("  Tasks: %d\n", stats.Tasks)
	fmt.Printf("  Plans: %d\n", stats.Plans)
	fmt.Printf("  States: %d\n", stats.States)
	fmt.Printf("  Specs: %d\n", stats.Specs)
	fmt.Printf("  Initiatives: %d\n", stats.Initiatives)
	fmt.Printf("  Attachments: %d\n", stats.Attachments)

	// Delete YAML files if requested
	if deleteAfter && !dryRun && stats.Tasks > 0 {
		fmt.Println("\nDeleting YAML files...")
		if err := os.RemoveAll(tasksDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete tasks directory: %v\n", err)
		}
		if err := os.RemoveAll(initiativesDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to delete initiatives directory: %v\n", err)
		}
		fmt.Println("YAML files deleted.")
	}

	return nil
}

func runMigrateDBToYAML(outputDir string) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	// Create output directories
	tasksOutputDir := filepath.Join(outputDir, "tasks")
	initiativesOutputDir := filepath.Join(outputDir, "initiatives")

	if err := os.MkdirAll(tasksOutputDir, 0700); err != nil {
		return fmt.Errorf("create tasks output dir: %w", err)
	}
	if err := os.MkdirAll(initiativesOutputDir, 0700); err != nil {
		return fmt.Errorf("create initiatives output dir: %w", err)
	}

	var stats struct {
		Tasks       int
		Plans       int
		States      int
		Specs       int
		Initiatives int
	}

	// Export tasks
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	for _, t := range tasks {
		taskDir := filepath.Join(tasksOutputDir, t.ID)
		if err := os.MkdirAll(taskDir, 0700); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to create task dir for %s: %v\n", t.ID, err)
			continue
		}

		// Save task.yaml
		taskData, _ := yaml.Marshal(t)
		if err := os.WriteFile(filepath.Join(taskDir, "task.yaml"), taskData, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write task.yaml for %s: %v\n", t.ID, err)
			continue
		}
		stats.Tasks++

		// Save plan.yaml if exists
		if p, err := backend.LoadPlan(t.ID); err == nil {
			planData, _ := yaml.Marshal(p)
			if err := os.WriteFile(filepath.Join(taskDir, "plan.yaml"), planData, 0600); err == nil {
				stats.Plans++
			}
		}

		// Save state.yaml if exists
		if s, err := backend.LoadState(t.ID); err == nil {
			stateData, _ := yaml.Marshal(s)
			if err := os.WriteFile(filepath.Join(taskDir, "state.yaml"), stateData, 0600); err == nil {
				stats.States++
			}
		}

		// Save spec.md if exists
		if specContent, err := backend.LoadSpec(t.ID); err == nil && specContent != "" {
			if err := os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte(specContent), 0600); err == nil {
				stats.Specs++
			}
		}
	}

	// Export initiatives
	initiatives, err := backend.LoadAllInitiatives()
	if err == nil {
		for _, init := range initiatives {
			initDir := filepath.Join(initiativesOutputDir, init.ID)
			if err := os.MkdirAll(initDir, 0700); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to create initiative dir for %s: %v\n", init.ID, err)
				continue
			}

			initData, _ := yaml.Marshal(init)
			if err := os.WriteFile(filepath.Join(initDir, "initiative.yaml"), initData, 0600); err == nil {
				stats.Initiatives++
			}
		}
	}

	fmt.Println("Export complete:")
	fmt.Printf("  Tasks: %d\n", stats.Tasks)
	fmt.Printf("  Plans: %d\n", stats.Plans)
	fmt.Printf("  States: %d\n", stats.States)
	fmt.Printf("  Specs: %d\n", stats.Specs)
	fmt.Printf("  Initiatives: %d\n", stats.Initiatives)
	fmt.Printf("\nFiles written to: %s\n", outputDir)

	return nil
}

// newMigratePlansCmd creates the plans migration subcommand
func newMigratePlansCmd() *cobra.Command {
	var allFlag bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "plans [task-id...]",
		Short: "Migrate stale task plans to current templates",
		Long: `Migrate stale task plans to use current phase templates.

Plans become stale when:
- Template version changes
- Phase sequence differs from current weight template
- Plan contains inline prompts (legacy format)

Migration preserves completed and skipped phase statuses.

Examples:
  orc migrate plans TASK-001              # Migrate single task
  orc migrate plans TASK-001 TASK-002    # Migrate specific tasks
  orc migrate plans --all                 # Migrate all stale plans
  orc migrate plans --all --dry-run       # Preview migrations without changing`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Must provide task IDs or --all flag
			if len(args) == 0 && !allFlag {
				return fmt.Errorf("must provide task IDs or use --all flag")
			}
			if len(args) > 0 && allFlag {
				return fmt.Errorf("cannot specify both task IDs and --all flag")
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			var taskIDs []string

			// Get task IDs to process
			if allFlag {
				// Load all tasks
				tasks, err := backend.LoadAllTasks()
				if err != nil {
					return fmt.Errorf("load tasks: %w", err)
				}
				for _, t := range tasks {
					taskIDs = append(taskIDs, t.ID)
				}
			} else {
				taskIDs = args
			}

			// Get output writers from cobra
			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()

			// Migrate each task
			var migrated, skipped, failed int
			for _, taskID := range taskIDs {
				// Load task
				t, err := backend.LoadTask(taskID)
				if err != nil {
					// If explicitly specified (not --all), error out
					if !allFlag {
						return fmt.Errorf("load task %s: %w", taskID, err)
					}
					fmt.Fprintf(errOut, "⚠️  Skip %s: load task failed: %v\n", taskID, err)
					failed++
					continue
				}

				// Load plan
				p, err := backend.LoadPlan(taskID)
				if err != nil {
					// No plan - skip
					if err == plan.ErrNotFound {
						skipped++
						continue
					}
					fmt.Fprintf(errOut, "⚠️  Skip %s: load plan failed: %v\n", taskID, err)
					failed++
					continue
				}

				// Check if stale
				stale, reason := plan.IsPlanStale(p, t)
				if !stale {
					skipped++
					continue
				}

				// Migrate
				result, err := plan.MigratePlan(t, p)
				if err != nil {
					fmt.Fprintf(errOut, "⚠️  Skip %s: migration failed: %v\n", taskID, err)
					failed++
					continue
				}

				// Save if not dry-run
				if !dryRun {
					if err := backend.SavePlan(result.NewPlan, taskID); err != nil {
						fmt.Fprintf(errOut, "⚠️  Skip %s: save failed: %v\n", taskID, err)
						failed++
						continue
					}
				}

				// Report migration
				if dryRun {
					fmt.Fprintf(out, "🔍 Would migrate %s: %s\n", taskID, reason)
				} else {
					fmt.Fprintf(out, "✅ Migrated %s: %s\n", taskID, reason)
				}
				fmt.Fprintf(out, "   Old: %s\n", result.OldPhases)
				fmt.Fprintf(out, "   New: %s\n", result.NewPhases)
				fmt.Fprintf(out, "   Preserved: %d, Reset: %d\n", result.PreservedCount, result.ResetCount)

				migrated++
			}

			// Summary
			fmt.Fprintf(out, "\n")
			if dryRun {
				fmt.Fprintf(out, "Dry run summary:\n")
			} else {
				fmt.Fprintf(out, "Migration summary:\n")
			}
			fmt.Fprintf(out, "  Total tasks: %d\n", len(taskIDs))
			fmt.Fprintf(out, "  Migrated: %d\n", migrated)
			fmt.Fprintf(out, "  Skipped (current): %d\n", skipped)
			if failed > 0 {
				fmt.Fprintf(out, "  Failed: %d\n", failed)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&allFlag, "all", false, "Migrate all tasks with stale plans")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview migrations without saving")

	return cmd
}
