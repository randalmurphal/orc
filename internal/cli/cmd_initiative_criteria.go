// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
)

func newInitiativeCriteriaCmd() *cobra.Command {
	var statusFlag string
	var evidenceFlag string

	cmd := &cobra.Command{
		Use:   "criteria <initiative-id> [subcommand] [args...]",
		Short: "Manage initiative acceptance criteria",
		Long: `Manage acceptance criteria for an initiative.

Without a subcommand, lists all criteria with their status.

Subcommands:
  add       Add a new acceptance criterion
  map       Map a criterion to a task
  verify    Verify a criterion with status and evidence
  coverage  Show coverage summary report

Examples:
  orc initiative criteria INIT-001                              # List criteria
  orc initiative criteria INIT-001 add "User can log in"        # Add criterion
  orc initiative criteria INIT-001 map AC-001 TASK-001          # Map to task
  orc initiative criteria INIT-001 verify AC-001 --status satisfied --evidence "Tests pass"
  orc initiative criteria INIT-001 coverage                     # Coverage report`,
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagParsing:    false,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			initID := args[0]

			// No subcommand → list
			if len(args) == 1 {
				return runCriteriaList(initID)
			}

			subcommand := args[1]
			subArgs := args[2:]

			switch subcommand {
			case "add":
				return runCriteriaAdd(initID, subArgs)
			case "map":
				return runCriteriaMap(initID, subArgs)
			case "verify":
				return runCriteriaVerify(initID, subArgs, statusFlag, evidenceFlag)
			case "coverage":
				return runCriteriaCoverage(initID)
			default:
				return fmt.Errorf("unknown criteria subcommand: %s", subcommand)
			}
		},
	}

	cmd.Flags().StringVar(&statusFlag, "status", "", "verification status (satisfied, regressed, covered, uncovered)")
	cmd.Flags().StringVar(&evidenceFlag, "evidence", "", "evidence supporting the verification")

	return cmd
}

func runCriteriaList(initID string) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	init, err := backend.LoadInitiative(initID)
	if err != nil {
		return fmt.Errorf("load initiative: %w", err)
	}

	if len(init.Criteria) == 0 {
		fmt.Println("No criteria defined for this initiative.")
		fmt.Printf("\nAdd criteria with: orc initiative criteria %s add \"description\"\n", initID)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tDESCRIPTION\tSTATUS\tTASKS")
	_, _ = fmt.Fprintln(w, "--\t-----------\t------\t-----")

	for _, c := range init.Criteria {
		taskCount := fmt.Sprintf("%d", len(c.TaskIDs))
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			c.ID, truncate(c.Description, 40), c.Status, taskCount)
	}
	_ = w.Flush()

	return nil
}

func runCriteriaAdd(initID string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: criteria <initiative-id> add <description>")
	}
	description := args[0]
	if description == "" {
		return fmt.Errorf("criterion description cannot be empty")
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	init, err := backend.LoadInitiative(initID)
	if err != nil {
		return fmt.Errorf("load initiative: %w", err)
	}

	init.AddCriterion(description)

	if err := backend.SaveInitiative(init); err != nil {
		return fmt.Errorf("save initiative: %w", err)
	}

	newCriterion := init.Criteria[len(init.Criteria)-1]
	fmt.Printf("Added criterion %s: %s\n", newCriterion.ID, description)
	return nil
}

func runCriteriaMap(initID string, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: criteria <initiative-id> map <criterion-id> <task-id>")
	}
	criterionID := args[0]
	taskID := args[1]

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	init, err := backend.LoadInitiative(initID)
	if err != nil {
		return fmt.Errorf("load initiative: %w", err)
	}

	if err := init.MapCriterionToTask(criterionID, taskID); err != nil {
		return err
	}

	if err := backend.SaveInitiative(init); err != nil {
		return fmt.Errorf("save initiative: %w", err)
	}

	fmt.Printf("Mapped %s to %s\n", criterionID, taskID)
	return nil
}

func runCriteriaVerify(initID string, args []string, status, evidence string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: criteria <initiative-id> verify <criterion-id> --status <status> --evidence <evidence>")
	}
	criterionID := args[0]

	if status == "" {
		return fmt.Errorf("--status flag is required")
	}
	if evidence == "" {
		return fmt.Errorf("--evidence flag is required")
	}

	// Validate status first
	if err := initiative.ValidateCriterionStatus(status); err != nil {
		return err
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	init, err := backend.LoadInitiative(initID)
	if err != nil {
		return fmt.Errorf("load initiative: %w", err)
	}

	if err := init.VerifyCriterion(criterionID, status, evidence); err != nil {
		return err
	}

	if err := backend.SaveInitiative(init); err != nil {
		return fmt.Errorf("save initiative: %w", err)
	}

	fmt.Printf("Verified %s: status=%s\n", criterionID, status)
	return nil
}

func runCriteriaCoverage(initID string) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	init, err := backend.LoadInitiative(initID)
	if err != nil {
		return fmt.Errorf("load initiative: %w", err)
	}

	report := init.GetCoverageReport()

	fmt.Printf("Coverage Report for %s\n", initID)
	fmt.Printf("  Total:     %d\n", report.Total)
	fmt.Printf("  Uncovered: %d\n", report.Uncovered)
	fmt.Printf("  Covered:   %d\n", report.Covered)
	fmt.Printf("  Satisfied: %d\n", report.Satisfied)
	fmt.Printf("  Regressed: %d\n", report.Regressed)

	if len(report.Criteria) > 0 {
		fmt.Println()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tDESCRIPTION\tSTATUS\tTASKS")
		_, _ = fmt.Fprintln(w, "--\t-----------\t------\t-----")
		for _, c := range report.Criteria {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
				c.ID, truncate(c.Description, 40), c.Status, len(c.TaskIDs))
		}
		_ = w.Flush()
	}

	return nil
}
