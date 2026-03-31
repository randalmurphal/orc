// Package cli implements the orc command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/workflow"
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
	workflowsCmd.AddCommand(workflowCloneCmd)
	workflowsCmd.AddCommand(workflowSyncCmd)

	// List flags
	workflowsCmd.Flags().Bool("custom", false, "Show only custom workflows")
	workflowsCmd.Flags().Bool("builtin", false, "Show only built-in workflows")
	workflowsCmd.Flags().Bool("sources", false, "Show source locations for each workflow")

	// Clone flags
	workflowCloneCmd.Flags().StringP("level", "l", "project", "Target level: personal, local, shared, project")
	workflowCloneCmd.Flags().BoolP("force", "f", false, "Overwrite if exists")

	// New flags
	workflowNewCmd.Flags().String("from", "", "Clone from existing workflow")
	workflowNewCmd.Flags().String("description", "", "Workflow description")
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
	workflowAddPhaseCmd.Flags().String("agent", "", "Override executor agent (uses this agent instead of phase template's default)")

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
custom workflows by cloning and modifying them.

Sources (--sources flag):
  personal  - ~/.orc/workflows/ (user machine-wide)
  local     - .orc/local/workflows/ (personal project-specific)
  shared    - .orc/shared/workflows/ (team defaults)
  project   - .orc/workflows/ (project defaults)
  embedded  - Built into the binary

Examples:
  orc workflows                 # List all workflows
  orc workflows --sources       # Show where each workflow comes from
  orc workflows --custom        # List only custom workflows
  orc workflows --builtin       # List only built-in workflows`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Silence Cobra's error output when JSON mode is enabled
		if jsonOut {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
		}

		projectRoot, err := ResolveProjectPath()
		if err != nil {
			if jsonOut {
				outputJSONError(cmd, err)
			}
			return err
		}

		orcDir := filepath.Join(projectRoot, ".orc")
		resolver := workflow.NewResolverFromOrcDir(orcDir)

		showSources, _ := cmd.Flags().GetBool("sources")
		customOnly, _ := cmd.Flags().GetBool("custom")
		builtinOnly, _ := cmd.Flags().GetBool("builtin")

		// Load config to get default workflow
		var defaultWorkflowID string
		if cfg, cfgErr := config.LoadFrom(projectRoot); cfgErr == nil {
			defaultWorkflowID = cfg.Workflow
		}

		workflows, err := resolver.ListWorkflows()
		if err != nil {
			wfErr := fmt.Errorf("list workflows: %w", err)
			if jsonOut {
				outputJSONError(cmd, wfErr)
			}
			return wfErr
		}

		// Filter workflows
		var filtered []workflow.ResolvedWorkflow
		for _, rw := range workflows {
			isBuiltin := rw.Source == workflow.SourceEmbedded
			if customOnly && isBuiltin {
				continue
			}
			if builtinOnly && !isBuiltin {
				continue
			}
			filtered = append(filtered, rw)
		}

		if len(filtered) == 0 {
			if jsonOut {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(map[string]interface{}{"workflows": []interface{}{}})
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No workflows found.")
			return nil
		}

		// JSON output mode
		if jsonOut {
			return outputWorkflowsJSON(cmd, filtered, defaultWorkflowID)
		}

		// Display as table
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		if showSources {
			_, _ = fmt.Fprintln(w, "ID\tNAME\tPHASES\tDEFAULT\tSOURCE")
		} else {
			_, _ = fmt.Fprintln(w, "ID\tNAME\tPHASES\tDEFAULT")
		}
		for _, rw := range filtered {
			phaseCount := len(rw.Workflow.Phases)
			defaultStr := ""
			if rw.Workflow.ID == defaultWorkflowID {
				defaultStr = "★"
			}
			if showSources {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
					rw.Workflow.ID, rw.Workflow.Name,
					phaseCount, defaultStr, workflow.SourceDisplayName(rw.Source))
			} else {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
					rw.Workflow.ID, rw.Workflow.Name,
					phaseCount, defaultStr)
			}
		}
		_ = w.Flush()

		return nil
	},
}

// workflowJSON represents a workflow in JSON output
type workflowJSON struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	PhaseCount  int    `json:"phase_count"`
	IsDefault   bool   `json:"is_default"`
	Source      string `json:"source,omitempty"`
}

// workflowsOutputJSON represents the JSON output structure
type workflowsOutputJSON struct {
	Workflows []workflowJSON `json:"workflows"`
}

// outputWorkflowsJSON outputs workflows as JSON
func outputWorkflowsJSON(cmd *cobra.Command, workflows []workflow.ResolvedWorkflow, defaultWorkflowID string) error {
	var jsonWorkflows []workflowJSON
	for _, rw := range workflows {
		jsonWorkflows = append(jsonWorkflows, workflowJSON{
			ID:          rw.Workflow.ID,
			Name:        rw.Workflow.Name,
			Description: rw.Workflow.Description,
			PhaseCount:  len(rw.Workflow.Phases),
			IsDefault:   rw.Workflow.ID == defaultWorkflowID,
			Source:      workflow.SourceDisplayName(rw.Source),
		})
	}

	output := workflowsOutputJSON{
		Workflows: jsonWorkflows,
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
