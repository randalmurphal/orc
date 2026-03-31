// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
)

var workflowNewCmd = &cobra.Command{
	Use:   "new <workflow-id>",
	Short: "Create a new custom workflow",
	Long: `Create a new custom workflow from scratch or by cloning an existing one.

When using --from, the workflow and all its phases are copied. You can then
modify the new workflow using 'orc workflow edit'.

Examples:
  orc workflow new my-review                        # Create empty workflow
  orc workflow new my-review --from review          # Clone from review workflow
  orc workflow new quick-impl --from small --description "Fast implementation"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]

		gdb, err := db.OpenGlobal()
		if err != nil {
			return fmt.Errorf("open global database: %w", err)
		}
		defer func() { _ = gdb.Close() }()

		existing, err := gdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("check existing: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("workflow already exists: %s", workflowID)
		}

		fromID, _ := cmd.Flags().GetString("from")
		desc, _ := cmd.Flags().GetString("description")

		if fromID != "" {
			source, err := gdb.GetWorkflow(fromID)
			if err != nil {
				return fmt.Errorf("load source workflow: %w", err)
			}
			if source == nil {
				return fmt.Errorf("source workflow not found: %s", fromID)
			}

			newWf := &db.Workflow{
				ID:              workflowID,
				Name:            workflowID,
				Description:     desc,
				DefaultModel:    source.DefaultModel,
				DefaultThinking: source.DefaultThinking,
				IsBuiltin:       false,
				BasedOn:         fromID,
			}
			if desc == "" {
				newWf.Description = source.Description
			}

			if err := gdb.SaveWorkflow(newWf); err != nil {
				return fmt.Errorf("save workflow: %w", err)
			}

			phases, err := gdb.GetWorkflowPhases(fromID)
			if err != nil {
				return fmt.Errorf("get source phases: %w", err)
			}
			for _, p := range phases {
				newPhase := &db.WorkflowPhase{
					WorkflowID:       workflowID,
					PhaseTemplateID:  p.PhaseTemplateID,
					Sequence:         p.Sequence,
					DependsOn:        p.DependsOn,
					ModelOverride:    p.ModelOverride,
					ThinkingOverride: p.ThinkingOverride,
					GateTypeOverride: p.GateTypeOverride,
					Condition:        p.Condition,
				}
				if err := gdb.SaveWorkflowPhase(newPhase); err != nil {
					return fmt.Errorf("save phase: %w", err)
				}
			}

			vars, err := gdb.GetWorkflowVariables(fromID)
			if err != nil {
				return fmt.Errorf("get source variables: %w", err)
			}
			for _, v := range vars {
				newVar := &db.WorkflowVariable{
					WorkflowID:      workflowID,
					Name:            v.Name,
					Description:     v.Description,
					SourceType:      v.SourceType,
					SourceConfig:    v.SourceConfig,
					Required:        v.Required,
					DefaultValue:    v.DefaultValue,
					CacheTTLSeconds: v.CacheTTLSeconds,
				}
				if err := gdb.SaveWorkflowVariable(newVar); err != nil {
					return fmt.Errorf("save variable: %w", err)
				}
			}

			fmt.Printf("Created workflow '%s' from '%s' with %d phases and %d variables\n",
				workflowID, fromID, len(phases), len(vars))
			return nil
		}

		newWf := &db.Workflow{
			ID:          workflowID,
			Name:        workflowID,
			Description: desc,
			IsBuiltin:   false,
		}

		if err := gdb.SaveWorkflow(newWf); err != nil {
			return fmt.Errorf("save workflow: %w", err)
		}

		fmt.Printf("Created empty workflow '%s'\n", workflowID)
		fmt.Println("Add phases with 'orc workflow edit' or via the UI")
		return nil
	},
}

var workflowEditCmd = &cobra.Command{
	Use:   "edit <workflow-id>",
	Short: "Edit a workflow's properties",
	Long: `Edit a custom workflow's name, description, or defaults.

For phase management, use 'add-phase' and 'remove-phase' subcommands.
For variable management, use 'add-variable' and 'remove-variable' subcommands.

Built-in workflows cannot be edited directly.

Examples:
  orc workflow edit my-review --description "Updated description"
  orc workflow edit my-review --model sonnet
  orc workflow edit my-review --name "My Review Workflow"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]

		gdb, err := db.OpenGlobal()
		if err != nil {
			return fmt.Errorf("open global database: %w", err)
		}
		defer func() { _ = gdb.Close() }()

		wf, err := gdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("get workflow: %w", err)
		}
		if wf == nil {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}
		if wf.IsBuiltin {
			return fmt.Errorf("cannot edit built-in workflow '%s' - use 'orc workflow new <name> --from %s' to create a custom copy",
				workflowID, workflowID)
		}

		updated := false
		if cmd.Flags().Changed("name") {
			wf.Name, _ = cmd.Flags().GetString("name")
			updated = true
		}
		if cmd.Flags().Changed("description") {
			wf.Description, _ = cmd.Flags().GetString("description")
			updated = true
		}
		if cmd.Flags().Changed("model") {
			wf.DefaultModel, _ = cmd.Flags().GetString("model")
			updated = true
		}
		if cmd.Flags().Changed("thinking") {
			wf.DefaultThinking, _ = cmd.Flags().GetBool("thinking")
			updated = true
		}
		if !updated {
			return fmt.Errorf("no changes specified. Use --name, --description, --model, or --thinking")
		}

		if err := gdb.SaveWorkflow(wf); err != nil {
			return fmt.Errorf("save workflow: %w", err)
		}

		fmt.Printf("Updated workflow '%s'\n", workflowID)
		return nil
	},
}

var workflowDeleteCmd = &cobra.Command{
	Use:   "delete <workflow-id>",
	Short: "Delete a custom workflow",
	Long: `Delete a custom workflow.

Built-in workflows cannot be deleted.

Examples:
  orc workflow delete my-review`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]

		gdb, err := db.OpenGlobal()
		if err != nil {
			return fmt.Errorf("open global database: %w", err)
		}
		defer func() { _ = gdb.Close() }()

		wf, err := gdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("get workflow: %w", err)
		}
		if wf == nil {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}
		if wf.IsBuiltin {
			return fmt.Errorf("cannot delete built-in workflow: %s", workflowID)
		}

		if err := gdb.DeleteWorkflow(workflowID); err != nil {
			return fmt.Errorf("delete workflow: %w", err)
		}

		fmt.Printf("Deleted workflow '%s'\n", workflowID)
		return nil
	},
}

var workflowCloneCmd = &cobra.Command{
	Use:   "clone <source-id> <dest-id>",
	Short: "Clone a workflow to a new file",
	Long: `Clone a workflow to a YAML file for customization.

This creates a standalone copy that can be edited. The cloned workflow
becomes file-based and can be customized without affecting the original.

Levels:
  personal - ~/.orc/workflows/ (user machine-wide)
  local    - .orc/local/workflows/ (personal project-specific, gitignored)
  shared   - .orc/shared/workflows/ (team defaults, git-tracked)
  project  - .orc/workflows/ (project defaults)

Examples:
  orc workflow clone implement-medium my-medium           # Clone to project level
  orc workflow clone implement-medium my-medium -l local  # Clone to local level
  orc workflow clone implement-medium my-medium -f        # Overwrite if exists`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceID := args[0]
		destID := args[1]

		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		orcDir := filepath.Join(projectRoot, ".orc")
		cloner := workflow.NewClonerFromOrcDir(orcDir)

		levelStr, _ := cmd.Flags().GetString("level")
		level, err := workflow.ParseWriteLevel(levelStr)
		if err != nil {
			return err
		}

		force, _ := cmd.Flags().GetBool("force")

		result, err := cloner.CloneWorkflow(sourceID, destID, level, force)
		if err != nil {
			return err
		}

		fmt.Printf("Cloned workflow '%s' to '%s'\n", sourceID, destID)
		fmt.Printf("File: %s\n", result.DestPath)
		fmt.Printf("Source: %s\n", workflow.SourceDisplayName(result.SourceLoc))
		fmt.Printf("Level: %s\n", result.DestLevel)

		if result.WasOverwrite {
			fmt.Println("(overwrote existing file)")
		}

		return nil
	},
}

var workflowSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync workflow files to database cache",
	Long: `Synchronize workflow YAML files to the database cache.

This scans all workflow directories (personal, local, shared, project, embedded)
and updates the database to match. The database acts as a runtime cache.

Use this when:
  - You've manually edited workflow YAML files
  - You want to force refresh embedded workflows after a binary update

Examples:
  orc workflow sync             # Sync all workflows
  orc workflow sync --force     # Force update all (including embedded)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		orcDir := filepath.Join(projectRoot, ".orc")
		gdb, err := db.OpenGlobal()
		if err != nil {
			return fmt.Errorf("open global database: %w", err)
		}
		defer func() { _ = gdb.Close() }()

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open project database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		globalCache := workflow.NewCacheServiceFromOrcDir(orcDir, gdb)
		projectCache := workflow.NewCacheServiceFromOrcDir(orcDir, pdb)

		force, _ := cmd.Flags().GetBool("force")

		var globalResult *workflow.SyncResult
		if force {
			globalResult, err = globalCache.ForceSync()
		} else {
			globalResult, err = globalCache.SyncAll()
		}
		if err != nil {
			return fmt.Errorf("sync global cache: %w", err)
		}

		var projectResult *workflow.SyncResult
		if force {
			projectResult, err = projectCache.ForceSync()
		} else {
			projectResult, err = projectCache.SyncAll()
		}
		if err != nil {
			return fmt.Errorf("sync project cache: %w", err)
		}

		fmt.Printf("Sync complete:\n")
		fmt.Printf("  Global workflows: %d added, %d updated\n", globalResult.WorkflowsAdded, globalResult.WorkflowsUpdated)
		fmt.Printf("  Global phases: %d added, %d updated\n", globalResult.PhasesAdded, globalResult.PhasesUpdated)
		fmt.Printf("  Project workflows: %d added, %d updated\n", projectResult.WorkflowsAdded, projectResult.WorkflowsUpdated)
		fmt.Printf("  Project phases: %d added, %d updated\n", projectResult.PhasesAdded, projectResult.PhasesUpdated)

		totalErrors := len(globalResult.Errors) + len(projectResult.Errors)
		if totalErrors > 0 {
			fmt.Printf("\nWarnings (%d):\n", totalErrors)
			for _, syncErr := range globalResult.Errors {
				fmt.Printf("  - global: %s\n", syncErr)
			}
			for _, syncErr := range projectResult.Errors {
				fmt.Printf("  - project: %s\n", syncErr)
			}
		}

		return nil
	},
}

func init() {
	workflowSyncCmd.Flags().Bool("force", false, "Force update all workflows including embedded")
}
