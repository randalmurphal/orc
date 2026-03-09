package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [workflow-id]",
		Short: "Validate local prerequisites for the default or specified workflow",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := ResolveProjectPath()
			if err != nil {
				return err
			}

			cfg, err := config.LoadFrom(projectRoot)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			gdb, err := db.OpenGlobal()
			if err != nil {
				return fmt.Errorf("open global database: %w", err)
			}
			defer func() { _ = gdb.Close() }()

			if _, err := workflow.SeedBuiltins(gdb); err != nil {
				return fmt.Errorf("seed workflows: %w", err)
			}

			workflowID := cfg.WorkflowDefaults.GetDefaultWorkflow("feature")
			if len(args) == 1 && args[0] != "" {
				workflowID = args[0]
			}
			if workflowID == "" {
				workflowID = "crossmodel-standard"
			}

			checks, err := runWorkflowDoctorChecks(cfg, gdb, workflowID)
			if err != nil {
				return fmt.Errorf("doctor: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Project: %s\n", projectRoot)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Workflow: %s\n", workflowID)
			for _, check := range checks {
				status := "PASS"
				if !check.OK {
					status = "FAIL"
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s  %s: %s\n", status, check.Name, check.Detail)
			}

			return failWorkflowDoctorChecks(checks, cfg.QualityPolicy.FailClosedOnMissingProvider)
		},
	}
}
