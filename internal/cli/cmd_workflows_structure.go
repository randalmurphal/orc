// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/db"
)

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
			return fmt.Errorf("cannot modify built-in workflow '%s' - use 'orc workflow new <name> --from %s' to create a custom copy",
				workflowID, workflowID)
		}

		pt, err := gdb.GetPhaseTemplate(phaseTemplateID)
		if err != nil {
			return fmt.Errorf("get phase template: %w", err)
		}
		if pt == nil {
			return fmt.Errorf("phase template not found: %s", phaseTemplateID)
		}

		phases, err := gdb.GetWorkflowPhases(workflowID)
		if err != nil {
			return fmt.Errorf("get phases: %w", err)
		}

		seq, _ := cmd.Flags().GetInt("sequence")
		if seq <= 0 {
			seq = len(phases)
		} else {
			for _, p := range phases {
				if p.Sequence >= seq {
					p.Sequence++
					if err := gdb.SaveWorkflowPhase(p); err != nil {
						return fmt.Errorf("update phase sequence: %w", err)
					}
				}
			}
		}

		newPhase := &db.WorkflowPhase{
			WorkflowID:      workflowID,
			PhaseTemplateID: phaseTemplateID,
			Sequence:        seq,
		}

		if cmd.Flags().Changed("model") {
			newPhase.ModelOverride, _ = cmd.Flags().GetString("model")
		}
		if cmd.Flags().Changed("gate-type") {
			newPhase.GateTypeOverride, _ = cmd.Flags().GetString("gate-type")
		}
		if cmd.Flags().Changed("agent") {
			agentID, _ := cmd.Flags().GetString("agent")
			if agentID != "" {
				agent, err := gdb.GetAgent(agentID)
				if err != nil {
					return fmt.Errorf("get agent: %w", err)
				}
				if agent == nil {
					return fmt.Errorf("agent not found: %s", agentID)
				}
			}
			newPhase.AgentOverride = agentID
		}

		if err := gdb.SaveWorkflowPhase(newPhase); err != nil {
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
			return fmt.Errorf("cannot modify built-in workflow '%s' - use 'orc workflow new <name> --from %s' to create a custom copy",
				workflowID, workflowID)
		}

		phases, err := gdb.GetWorkflowPhases(workflowID)
		if err != nil {
			return fmt.Errorf("get phases: %w", err)
		}

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

		if err := gdb.DeleteWorkflowPhase(workflowID, phaseTemplateID); err != nil {
			return fmt.Errorf("delete phase: %w", err)
		}

		for _, p := range phases {
			if p.Sequence > removedSeq {
				p.Sequence--
				if err := gdb.SaveWorkflowPhase(p); err != nil {
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
			return fmt.Errorf("cannot modify built-in workflow '%s' - use 'orc workflow new <name> --from %s' to create a custom copy",
				workflowID, workflowID)
		}

		sourceType, _ := cmd.Flags().GetString("source-type")
		value, _ := cmd.Flags().GetString("value")
		desc, _ := cmd.Flags().GetString("description")
		required, _ := cmd.Flags().GetBool("required")

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

		if err := gdb.SaveWorkflowVariable(newVar); err != nil {
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
			return fmt.Errorf("cannot modify built-in workflow '%s' - use 'orc workflow new <name> --from %s' to create a custom copy",
				workflowID, workflowID)
		}

		if err := gdb.DeleteWorkflowVariable(workflowID, varName); err != nil {
			return fmt.Errorf("delete variable: %w", err)
		}

		fmt.Printf("Removed variable '%s' from workflow '%s'\n", varName, workflowID)
		return nil
	},
}
