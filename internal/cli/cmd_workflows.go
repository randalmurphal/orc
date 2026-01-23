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
	workflowsCmd.AddCommand(workflowAddPhaseCmd)
	workflowsCmd.AddCommand(workflowRemovePhaseCmd)
	workflowsCmd.AddCommand(workflowAddVariableCmd)
	workflowsCmd.AddCommand(workflowRemoveVariableCmd)

	// List flags
	workflowsCmd.Flags().Bool("custom", false, "Show only custom workflows")
	workflowsCmd.Flags().Bool("builtin", false, "Show only built-in workflows")

	// New flags
	workflowNewCmd.Flags().String("from", "", "Clone from existing workflow")
	workflowNewCmd.Flags().String("description", "", "Workflow description")
	workflowNewCmd.Flags().String("type", "task", "Workflow type (task, branch, standalone)")

	// Edit flags
	workflowEditCmd.Flags().String("name", "", "New workflow name")
	workflowEditCmd.Flags().String("description", "", "New description")
	workflowEditCmd.Flags().String("model", "", "Default model")
	workflowEditCmd.Flags().Bool("thinking", false, "Enable extended thinking")

	// Add-phase flags
	workflowAddPhaseCmd.Flags().Int("sequence", 0, "Position in workflow (0 = append at end)")
	workflowAddPhaseCmd.Flags().Int("max-iterations", 0, "Override max iterations")
	workflowAddPhaseCmd.Flags().String("model", "", "Override model")
	workflowAddPhaseCmd.Flags().String("gate-type", "", "Override gate type (auto, human)")

	// Add-variable flags
	workflowAddVariableCmd.Flags().String("source-type", "static", "Variable source (static, env, script, api)")
	workflowAddVariableCmd.Flags().String("value", "", "Value for static variables")
	workflowAddVariableCmd.Flags().String("description", "", "Variable description")
	workflowAddVariableCmd.Flags().Bool("required", false, "Whether variable is required")
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
		defer func() { _ = pdb.Close() }()

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
		_, _ = fmt.Fprintln(w, "ID\tNAME\tTYPE\tPHASES\tBUILT-IN")
		for _, wf := range filtered {
			phases, _ := pdb.GetWorkflowPhases(wf.ID)
			builtinStr := ""
			if wf.IsBuiltin {
				builtinStr = "yes"
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
				wf.ID, wf.Name, wf.WorkflowType, len(phases), builtinStr)
		}
		_ = w.Flush()

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
		defer func() { _ = pdb.Close() }()

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
			_, _ = fmt.Fprintln(w, "  SEQ\tPHASE\tMAX ITER\tMODEL\tGATE")
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
				_, _ = fmt.Fprintf(w, "  %d\t%s\t%s\t%s\t%s\n",
					p.Sequence, p.PhaseTemplateID, maxIter, model, gate)
			}
			_ = w.Flush()
		}

		// Display variables
		vars, err := pdb.GetWorkflowVariables(workflowID)
		if err != nil {
			return fmt.Errorf("get variables: %w", err)
		}

		if len(vars) > 0 {
			fmt.Println("\nVariables:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "  NAME\tSOURCE\tREQUIRED\tDESCRIPTION")
			for _, v := range vars {
				required := ""
				if v.Required {
					required = "yes"
				}
				desc := v.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
				_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n",
					v.Name, v.SourceType, required, desc)
			}
			_ = w.Flush()
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
		defer func() { _ = pdb.Close() }()

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

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

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

		// Update fields if flags provided
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

		if err := pdb.SaveWorkflow(wf); err != nil {
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

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

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

var workflowAddPhaseCmd = &cobra.Command{
	Use:   "add-phase <workflow-id> <phase-template-id>",
	Short: "Add a phase to a custom workflow",
	Long: `Add a phase template to a custom workflow.

The phase is appended at the end by default. Use --sequence to insert at a
specific position.

Built-in workflows cannot be modified. Create a custom copy first with
'orc workflow new <name> --from <builtin>'.

Examples:
  orc workflow add-phase my-review docs                    # Append docs phase
  orc workflow add-phase my-impl implement --sequence 2   # Insert at position 2
  orc workflow add-phase my-impl review --model opus      # Override model`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]
		phaseTemplateID := args[1]

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		// Check workflow exists and is not builtin
		wf, err := pdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("get workflow: %w", err)
		}
		if wf == nil {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}
		if wf.IsBuiltin {
			return fmt.Errorf("cannot modify built-in workflow '%s' - use 'orc workflow new <name> --from %s' to create a custom copy",
				workflowID, workflowID)
		}

		// Check phase template exists
		pt, err := pdb.GetPhaseTemplate(phaseTemplateID)
		if err != nil {
			return fmt.Errorf("get phase template: %w", err)
		}
		if pt == nil {
			return fmt.Errorf("phase template not found: %s", phaseTemplateID)
		}

		// Get existing phases to determine sequence
		phases, err := pdb.GetWorkflowPhases(workflowID)
		if err != nil {
			return fmt.Errorf("get phases: %w", err)
		}

		seq, _ := cmd.Flags().GetInt("sequence")
		if seq <= 0 {
			// Append at end
			seq = len(phases)
		} else {
			// Adjust sequences for existing phases that need to move
			for _, p := range phases {
				if p.Sequence >= seq {
					p.Sequence++
					if err := pdb.SaveWorkflowPhase(p); err != nil {
						return fmt.Errorf("update phase sequence: %w", err)
					}
				}
			}
		}

		// Create new phase
		newPhase := &db.WorkflowPhase{
			WorkflowID:      workflowID,
			PhaseTemplateID: phaseTemplateID,
			Sequence:        seq,
		}

		// Apply overrides from flags
		if cmd.Flags().Changed("max-iterations") {
			maxIter, _ := cmd.Flags().GetInt("max-iterations")
			newPhase.MaxIterationsOverride = &maxIter
		}
		if cmd.Flags().Changed("model") {
			newPhase.ModelOverride, _ = cmd.Flags().GetString("model")
		}
		if cmd.Flags().Changed("gate-type") {
			newPhase.GateTypeOverride, _ = cmd.Flags().GetString("gate-type")
		}

		if err := pdb.SaveWorkflowPhase(newPhase); err != nil {
			return fmt.Errorf("save phase: %w", err)
		}

		fmt.Printf("Added phase '%s' to workflow '%s' at sequence %d\n",
			phaseTemplateID, workflowID, seq)
		return nil
	},
}

var workflowRemovePhaseCmd = &cobra.Command{
	Use:   "remove-phase <workflow-id> <phase-template-id>",
	Short: "Remove a phase from a custom workflow",
	Long: `Remove a phase from a custom workflow.

If the phase appears multiple times, the first occurrence is removed.
Built-in workflows cannot be modified.

Examples:
  orc workflow remove-phase my-review docs
  orc workflow remove-phase my-impl validate`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]
		phaseTemplateID := args[1]

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		// Check workflow exists and is not builtin
		wf, err := pdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("get workflow: %w", err)
		}
		if wf == nil {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}
		if wf.IsBuiltin {
			return fmt.Errorf("cannot modify built-in workflow '%s' - use 'orc workflow new <name> --from %s' to create a custom copy",
				workflowID, workflowID)
		}

		// Get existing phases
		phases, err := pdb.GetWorkflowPhases(workflowID)
		if err != nil {
			return fmt.Errorf("get phases: %w", err)
		}

		// Find the phase to get its sequence
		removedSeq := -1
		for _, p := range phases {
			if p.PhaseTemplateID == phaseTemplateID {
				removedSeq = p.Sequence
				break
			}
		}

		if removedSeq == -1 {
			return fmt.Errorf("phase '%s' not found in workflow '%s'", phaseTemplateID, workflowID)
		}

		// Delete the phase
		if err := pdb.DeleteWorkflowPhase(workflowID, phaseTemplateID); err != nil {
			return fmt.Errorf("delete phase: %w", err)
		}

		// Re-sequence remaining phases
		for _, p := range phases {
			if p.Sequence > removedSeq {
				p.Sequence--
				if err := pdb.SaveWorkflowPhase(p); err != nil {
					return fmt.Errorf("update phase sequence: %w", err)
				}
			}
		}

		fmt.Printf("Removed phase '%s' from workflow '%s'\n", phaseTemplateID, workflowID)
		return nil
	},
}

var workflowAddVariableCmd = &cobra.Command{
	Use:   "add-variable <workflow-id> <variable-name>",
	Short: "Add a variable to a custom workflow",
	Long: `Add a workflow variable with a specified source type.

Variable sources:
  static   - Fixed value (use --value)
  env      - Environment variable
  script   - Script output
  api      - HTTP API response

Examples:
  orc workflow add-variable my-wf API_KEY --source-type env --required
  orc workflow add-variable my-wf VERSION --source-type static --value "1.0.0"
  orc workflow add-variable my-wf CONTEXT --description "Extra context"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]
		varName := args[1]

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		// Check workflow exists and is not builtin
		wf, err := pdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("get workflow: %w", err)
		}
		if wf == nil {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}
		if wf.IsBuiltin {
			return fmt.Errorf("cannot modify built-in workflow '%s' - use 'orc workflow new <name> --from %s' to create a custom copy",
				workflowID, workflowID)
		}

		sourceType, _ := cmd.Flags().GetString("source-type")
		value, _ := cmd.Flags().GetString("value")
		desc, _ := cmd.Flags().GetString("description")
		required, _ := cmd.Flags().GetBool("required")

		// Build source config based on type
		var sourceConfig string
		switch sourceType {
		case "static":
			if value == "" {
				return fmt.Errorf("--value is required for static source type")
			}
			sourceConfig = fmt.Sprintf(`{"value": %q}`, value)
		case "env":
			sourceConfig = fmt.Sprintf(`{"var": %q}`, varName)
		default:
			sourceConfig = "{}"
		}

		newVar := &db.WorkflowVariable{
			WorkflowID:   workflowID,
			Name:         varName,
			Description:  desc,
			SourceType:   sourceType,
			SourceConfig: sourceConfig,
			Required:     required,
		}

		if err := pdb.SaveWorkflowVariable(newVar); err != nil {
			return fmt.Errorf("save variable: %w", err)
		}

		fmt.Printf("Added variable '%s' to workflow '%s' (source: %s)\n",
			varName, workflowID, sourceType)
		return nil
	},
}

var workflowRemoveVariableCmd = &cobra.Command{
	Use:   "remove-variable <workflow-id> <variable-name>",
	Short: "Remove a variable from a custom workflow",
	Long: `Remove a workflow variable.

Built-in workflows cannot be modified.

Examples:
  orc workflow remove-variable my-wf API_KEY
  orc workflow remove-variable my-impl EXTRA_CONTEXT`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		workflowID := args[0]
		varName := args[1]

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		// Check workflow exists and is not builtin
		wf, err := pdb.GetWorkflow(workflowID)
		if err != nil {
			return fmt.Errorf("get workflow: %w", err)
		}
		if wf == nil {
			return fmt.Errorf("workflow not found: %s", workflowID)
		}
		if wf.IsBuiltin {
			return fmt.Errorf("cannot modify built-in workflow '%s' - use 'orc workflow new <name> --from %s' to create a custom copy",
				workflowID, workflowID)
		}

		if err := pdb.DeleteWorkflowVariable(workflowID, varName); err != nil {
			return fmt.Errorf("delete variable: %w", err)
		}

		fmt.Printf("Removed variable '%s' from workflow '%s'\n", varName, workflowID)
		return nil
	},
}
