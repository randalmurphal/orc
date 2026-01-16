package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan_session"
	"github.com/randalmurphal/orc/internal/planner"
)

var planCmd = &cobra.Command{
	Use:   "plan [TARGET]",
	Short: "Start interactive planning session or generate tasks from specs",
	Long: `Plan command supports two modes:

INTERACTIVE MODE (default):
  Plan with Claude Code to create specifications interactively.

  TARGET can be:
    - A task ID (e.g., TASK-001) to plan/refine an existing task
    - A feature title (e.g., "Add user auth") to create a new feature spec

  Examples:
    orc plan TASK-001                     # Plan existing task
    orc plan "Add user authentication"    # Create feature spec
    orc plan "Refactor API" --create-tasks  # Create spec and generate tasks
    orc plan TASK-001 --initiative INIT-001 # Link to initiative

BATCH MODE (--from):
  Read existing spec files and generate tasks from them.

  Examples:
    orc plan --from .spec/                # Read specs from directory
    orc plan --from docs/specs/ --yes     # Create tasks without confirmation`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPlan,
}

var (
	planFrom             string
	planYes              bool
	planDryRun           bool
	planInitiative       string
	planCreateInitiative bool
	planModel            string
	planInclude          []string
	planWeight           string
	planCreateTasks      bool
	planShared           bool
)

func init() {
	planCmd.GroupID = groupPlanning
	rootCmd.AddCommand(planCmd)

	// Interactive mode flags
	planCmd.Flags().StringVarP(&planInitiative, "initiative", "i", "", "Link to existing initiative")
	planCmd.Flags().StringVarP(&planModel, "model", "m", "", "Claude model to use")
	planCmd.Flags().StringVarP(&planWeight, "weight", "w", "", "Pre-set task weight (skip asking)")
	planCmd.Flags().BoolVarP(&planCreateTasks, "create-tasks", "t", false, "Create tasks from spec output (feature mode)")
	planCmd.Flags().Bool("skip-validation", false, "Skip spec validation")
	planCmd.Flags().BoolVar(&planShared, "shared", false, "Use shared initiative")

	// Batch mode flags
	planCmd.Flags().StringVarP(&planFrom, "from", "f", "", "Directory containing spec documents (batch mode)")
	planCmd.Flags().BoolVarP(&planYes, "yes", "y", false, "Create tasks without confirmation (batch mode)")
	planCmd.Flags().BoolVarP(&planCreateInitiative, "create-initiative", "I", false, "Create new initiative for tasks (batch mode)")
	planCmd.Flags().StringSliceVar(&planInclude, "include", []string{"*.md"}, "File patterns to include (batch mode)")

	// Shared flags
	planCmd.Flags().BoolVar(&planDryRun, "dry-run", false, "Show prompt without running")
}

func runPlan(cmd *cobra.Command, args []string) error {
	// Check if we're in batch mode (--from specified)
	if planFrom != "" {
		return runPlanBatch(cmd, args)
	}

	// Interactive mode
	return runPlanInteractive(cmd, args)
}

// runPlanInteractive handles the interactive Claude Code planning session.
func runPlanInteractive(cmd *cobra.Command, args []string) error {
	if err := config.RequireInit(); err != nil {
		return err
	}

	target := ""
	if len(args) > 0 {
		target = args[0]
	}

	skipValidation, _ := cmd.Flags().GetBool("skip-validation")

	ctx := context.Background()

	result, err := plan_session.Run(ctx, target, plan_session.Options{
		WorkDir:        ".",
		Model:          planModel,
		InitiativeID:   planInitiative,
		Weight:         planWeight,
		CreateTasks:    planCreateTasks,
		DryRun:         planDryRun,
		SkipValidation: skipValidation,
		Shared:         planShared,
	})
	if err != nil {
		return fmt.Errorf("planning session failed: %w", err)
	}

	if planDryRun {
		return nil
	}

	// Show results
	if result.SpecPath != "" {
		fmt.Printf("\nSpec created: %s\n", result.SpecPath)
	}

	if result.TaskID != "" {
		fmt.Printf("Task: %s\n", result.TaskID)
	}

	if len(result.TaskIDs) > 0 {
		fmt.Printf("Tasks created: %v\n", result.TaskIDs)
	}

	// Show validation results
	if result.ValidationResult != nil {
		if result.ValidationResult.Valid {
			if !plain {
				fmt.Println("✓ Spec validation passed")
			} else {
				fmt.Println("[OK] Spec validation passed")
			}
		} else {
			if !plain {
				fmt.Println("⚠ Spec validation issues:")
			} else {
				fmt.Println("[WARN] Spec validation issues:")
			}
			for _, issue := range result.ValidationResult.Issues {
				fmt.Printf("  - %s\n", issue)
			}
			fmt.Println("\nRun with --skip-validation to bypass, or edit the spec to add missing sections.")
		}
	}

	return nil
}

// runPlanBatch handles the batch mode - reading existing specs and generating tasks.
func runPlanBatch(_ *cobra.Command, _ []string) error {
	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	// Check if .orc directory exists
	if _, err := os.Stat(".orc"); os.IsNotExist(err) {
		return fmt.Errorf("orc not initialized in this directory (run 'orc init' first)")
	}

	// Create planner
	opts := planner.Options{
		SpecDir:          planFrom,
		Include:          planInclude,
		WorkDir:          wd,
		Model:            planModel,
		InitiativeID:     planInitiative,
		CreateInitiative: planCreateInitiative,
		DryRun:           planDryRun,
		BatchMode:        planYes,
	}
	p := planner.New(opts)

	// Load spec files
	fmt.Printf("Loading specifications from %s...\n", planFrom)
	files, err := p.LoadSpecs()
	if err != nil {
		return fmt.Errorf("load specs: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no specification files found in %s", planFrom)
	}

	fmt.Printf("Found %d spec file(s):\n", len(files))
	for _, f := range files {
		fmt.Printf("  - %s (%d words)\n", f.Path, f.Words)
	}
	fmt.Println()

	// Generate prompt
	prompt, err := p.GeneratePrompt(files)
	if err != nil {
		return fmt.Errorf("generate prompt: %w", err)
	}

	// Dry run - just show the prompt
	if planDryRun {
		fmt.Println("=== Planning Prompt ===")
		fmt.Println(prompt)
		fmt.Println("=== End Prompt ===")
		return nil
	}

	// Run Claude
	fmt.Println("Running Claude analysis...")
	ctx := context.Background()
	response, err := p.RunClaude(ctx, prompt)
	if err != nil {
		return fmt.Errorf("run claude: %w", err)
	}

	// Parse response
	breakdown, err := p.ParseResponse(response)
	if err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	// Display proposed tasks
	fmt.Println()
	fmt.Println("=== Proposed Task Breakdown ===")
	if breakdown.Summary != "" {
		fmt.Println()
		fmt.Println("Summary:")
		// Truncate summary if too long
		summary := breakdown.Summary
		if len(summary) > 500 {
			summary = summary[:500] + "..."
		}
		fmt.Printf("  %s\n", strings.ReplaceAll(summary, "\n", "\n  "))
	}

	fmt.Printf("\nTasks (%d):\n\n", len(breakdown.Tasks))
	for _, t := range breakdown.Tasks {
		deps := ""
		if len(t.DependsOn) > 0 {
			depStrs := make([]string, len(t.DependsOn))
			for i, d := range t.DependsOn {
				depStrs[i] = fmt.Sprintf("%d", d)
			}
			deps = fmt.Sprintf(" (depends on: %s)", strings.Join(depStrs, ", "))
		}
		fmt.Printf("  %d. %s [%s]%s\n", t.Index, t.Title, t.Weight, deps)
		// Show first 100 chars of description
		desc := t.Description
		if len(desc) > 100 {
			desc = desc[:100] + "..."
		}
		fmt.Printf("     %s\n\n", strings.ReplaceAll(desc, "\n", "\n     "))
	}

	// Confirm unless --yes
	if !planYes {
		fmt.Printf("Create these %d tasks? [y/N]: ", len(breakdown.Tasks))
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Create tasks
	fmt.Println()
	fmt.Println("Creating tasks...")
	results, err := p.CreateTasks(breakdown)
	if err != nil {
		return fmt.Errorf("create tasks: %w", err)
	}

	fmt.Println()
	fmt.Println("Tasks created:")
	for _, r := range results {
		deps := ""
		if len(r.DependsOn) > 0 {
			deps = fmt.Sprintf(" (depends on: %s)", strings.Join(r.DependsOn, ", "))
		}
		fmt.Printf("  %s: %s [%s]%s\n", r.TaskID, r.Title, r.Weight, deps)
	}

	fmt.Println()
	fmt.Println("Next steps:")
	if len(results) > 0 {
		fmt.Printf("  orc list                     # View all tasks\n")
		fmt.Printf("  orc run %s              # Start first task\n", results[0].TaskID)
		fmt.Printf("  orc orchestrate              # Run all tasks with orchestrator\n")
	}

	return nil
}
