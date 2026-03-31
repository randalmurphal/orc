// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/db"
)

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

		fmt.Printf("Workflow: %s\n", wf.ID)
		fmt.Printf("Name: %s\n", wf.Name)
		if wf.Description != "" {
			fmt.Printf("Description: %s\n", wf.Description)
		}
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

		phases, err := gdb.GetWorkflowPhases(workflowID)
		if err != nil {
			return fmt.Errorf("get phases: %w", err)
		}

		if len(phases) > 0 {
			fmt.Println("\nPhases:")
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "  SEQ\tPHASE\tMODEL\tGATE")
			for _, p := range phases {
				model := "-"
				if p.ModelOverride != "" {
					model = p.ModelOverride
				}
				gate := "-"
				if p.GateTypeOverride != "" {
					gate = p.GateTypeOverride
				}
				_, _ = fmt.Fprintf(w, "  %d\t%s\t%s\t%s\n",
					p.Sequence, p.PhaseTemplateID, model, gate)
			}
			_ = w.Flush()
		}

		vars, err := gdb.GetWorkflowVariables(workflowID)
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
