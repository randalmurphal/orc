// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

func init() {
	rootCmd.AddCommand(workflowsCmd)
	workflowsCmd.AddCommand(workflowShowCmd)
	workflowsCmd.AddCommand(workflowNewCmd)
	workflowsCmd.AddCommand(workflowEditCmd)
	workflowsCmd.AddCommand(workflowDeleteCmd)

	// List flags
	workflowsCmd.Flags().Bool("custom", false, "Show only custom workflows")
	workflowsCmd.Flags().Bool("builtin", false, "Show only built-in workflows")

	// New flags
	workflowNewCmd.Flags().String("from", "", "Clone from existing workflow")
	workflowNewCmd.Flags().String("description", "", "Workflow description")
	workflowNewCmd.Flags().String("type", "task", "Workflow type (task, branch, standalone)")
}

var workflowsCmd = &cobra.Command{
	Use:     "workflows",
	Aliases: []string{"wf", "workflow"},
	Short:   "List available workflows",
	Long: `List all workflows available for use with 'orc run'.

Workflows define the sequence of phases to execute. Built-in workflows
(trivial, small, medium, large) are provided by orc. You can create
custom workflows that compose phases differently.

Examples:
  orc workflows                 # List all workflows
  orc workflows --custom        # List only custom workflows
  orc workflows --builtin       # List only built-in workflows`,
	RunE: func(cmd *cobra.Command, args []string) error {
		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer pdb.Close()

		workflows, err := pdb.ListWorkflows()
		if err != nil {
			return fmt.Errorf("list workflows: %w", err)
		}

		customOnly, _ := cmd.Flags().GetBool("custom")
		builtinOnly, _ := cmd.Flags().GetBool("builtin")

		// Filter workflows
		var filtered []*db.Workflow
		for _, wf := range workflows {
			if customOnly && wf.IsBuiltin {
				continue
			}
			if builtinOnly && !wf.IsBuiltin {
				continue
			}
			filtered = append(filtered, wf)
		}

		if len(filtered) == 0 {
			fmt.Println("No workflows found.")
			return nil
		}

		// Display as table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tTYPE\tPHASES\tBUILT-IN")
		for _, wf := range filtered {
			phases, _ := pdb.GetWorkflowPhases(wf.ID)
			builtinStr := ""
			if wf.IsBuiltin {
				builtinStr = "yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
				wf.ID, wf.Name, wf.WorkflowType, len(phases), builtinStr)
		}
		w.Flush()

		return nil
	},
}

var workflowShowCmd = &cobra.Command{
	Use:   "show <workflow-id>",
	Short: "Show workflow details",
	Long: `Display detailed information about a workflow including its phases,
variables, and configuration.

Examples:
  orc workflow show medium        # Show the medium workflow
  orc workflow show my-review     # Show a custom workflow`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer pdb.Close()

		wf, err := pdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("get workflow: %w", err)
		}
		if wf == nil {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}

		// Display workflow info
		fmt.Printf("Workflow: %s\n", wf.ID)
		fmt.Printf("Name: %s\n", wf.Name)
		if wf.Description != "" {
			fmt.Printf("Description: %s\n", wf.Description)
		}
		fmt.Printf("Type: %s\n", wf.WorkflowType)
		if wf.DefaultModel != "" {
			fmt.Printf("Default Model: %s\n", wf.DefaultModel)
		}
		if wf.DefaultThinking {
			fmt.Println("Extended Thinking: enabled")
		}
		if wf.IsBuiltin {
			fmt.Println("Built-in: yes")
		}
		if wf.BasedOn != "" {
			fmt.Printf("Based on: %s\n", wf.BasedOn)
		}

		// Display phases
		phases, err := pdb.GetWorkflowPhases(workflowID)
		if err != nil {
			return fmt.Errorf("get phases: %w", err)
		}

		if len(phases) > 0 {
			fmt.Println("\nPhases:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  SEQ\tPHASE\tMAX ITER\tMODEL\tGATE")
			for _, p := range phases {
				maxIter := "-"
				if p.MaxIterationsOverride != nil {
					maxIter = fmt.Sprintf("%d", *p.MaxIterationsOverride)
				}
				model := "-"
				if p.ModelOverride != "" {
					model = p.ModelOverride
				}
				gate := "-"
				if p.GateTypeOverride != "" {
					gate = p.GateTypeOverride
				}
				fmt.Fprintf(w, "  %d\t%s\t%s\t%s\t%s\n",
					p.Sequence, p.PhaseTemplateID, maxIter, model, gate)
			}
			w.Flush()
		}

		// Display variables
		vars, err := pdb.GetWorkflowVariables(workflowID)
		if err != nil {
			return fmt.Errorf("get variables: %w", err)
		}

		if len(vars) > 0 {
			fmt.Println("\nVariables:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "  NAME\tSOURCE\tREQUIRED\tDESCRIPTION")
			for _, v := range vars {
				required := ""
				if v.Required {
					required = "yes"
				}
				desc := v.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
				fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
					v.Name, v.SourceType, required, desc)
			}
			w.Flush()
		}

		return nil
	},
}

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

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer pdb.Close()

		// Check if workflow already exists
		existing, err := pdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("check existing: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("workflow already exists: %s", workflowID)
		}

		fromID, _ := cmd.Flags().GetString("from")
		desc, _ := cmd.Flags().GetString("description")
		wfType, _ := cmd.Flags().GetString("type")

		if fromID != "" {
			// Clone existing workflow
			source, err := pdb.GetWorkflow(fromID)
			if err != nil {
				return fmt.Errorf("load source workflow: %w", err)
			}
			if source == nil {
				return fmt.Errorf("source workflow not found: %s", fromID)
			}

			// Create new workflow based on source
			newWf := &db.Workflow{
				ID:              workflowID,
				Name:            workflowID,
				Description:     desc,
				WorkflowType:    source.WorkflowType,
				DefaultModel:    source.DefaultModel,
				DefaultThinking: source.DefaultThinking,
				IsBuiltin:       false,
				BasedOn:         fromID,
			}
			if desc == "" {
				newWf.Description = source.Description
			}

			if err := pdb.SaveWorkflow(newWf); err != nil {
				return fmt.Errorf("save workflow: %w", err)
			}

			// Copy phases
			phases, err := pdb.GetWorkflowPhases(fromID)
			if err != nil {
				return fmt.Errorf("get source phases: %w", err)
			}
			for _, p := range phases {
				newPhase := &db.WorkflowPhase{
					WorkflowID:            workflowID,
					PhaseTemplateID:       p.PhaseTemplateID,
					Sequence:              p.Sequence,
					DependsOn:             p.DependsOn,
					MaxIterationsOverride: p.MaxIterationsOverride,
					ModelOverride:         p.ModelOverride,
					ThinkingOverride:      p.ThinkingOverride,
					GateTypeOverride:      p.GateTypeOverride,
					Condition:             p.Condition,
				}
				if err := pdb.SaveWorkflowPhase(newPhase); err != nil {
					return fmt.Errorf("save phase: %w", err)
				}
			}

			// Copy variables
			vars, err := pdb.GetWorkflowVariables(fromID)
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
				if err := pdb.SaveWorkflowVariable(newVar); err != nil {
					return fmt.Errorf("save variable: %w", err)
				}
			}

			fmt.Printf("Created workflow '%s' from '%s' with %d phases and %d variables\n",
				workflowID, fromID, len(phases), len(vars))
		} else {
			// Create empty workflow
			newWf := &db.Workflow{
				ID:           workflowID,
				Name:         workflowID,
				Description:  desc,
				WorkflowType: wfType,
				IsBuiltin:    false,
			}

			if err := pdb.SaveWorkflow(newWf); err != nil {
				return fmt.Errorf("save workflow: %w", err)
			}

			fmt.Printf("Created empty workflow '%s'\n", workflowID)
			fmt.Println("Add phases with 'orc workflow edit' or via the UI")
		}

		return nil
	},
}

var workflowEditCmd = &cobra.Command{
	Use:   "edit <workflow-id>",
	Short: "Edit a workflow",
	Long: `Edit a workflow's configuration.

Use the web UI for full workflow editing, or modify individual properties
using the subcommands.

Note: Built-in workflows cannot be edited directly. Use --from to
create a custom copy first.

Examples:
  orc workflow edit my-review --description "Updated description"
  orc workflow edit my-review --model sonnet`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer pdb.Close()

		wf, err := pdb.GetWorkflow(workflowID)
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

		// TODO: Add edit flags and implementation
		// For now, direct users to the web UI
		fmt.Println("Workflow editing via CLI is coming soon.")
		fmt.Println("Use the web UI to edit workflows: orc serve")
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

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer pdb.Close()

		wf, err := pdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("get workflow: %w", err)
		}
		if wf == nil {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}

		if wf.IsBuiltin {
			return fmt.Errorf("cannot delete built-in workflow: %s", workflowID)
		}

		if err := pdb.DeleteWorkflow(workflowID); err != nil {
			return fmt.Errorf("delete workflow: %w", err)
		}

		fmt.Printf("Deleted workflow '%s'\n", workflowID)
		return nil
	},
}
