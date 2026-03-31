package cli

import (
	"fmt"
	"os"
	"sync"

	"github.com/randalmurphal/orc/internal/task"
	"github.com/spf13/cobra"
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
				projectRoot, err := ResolveProjectPath()
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
