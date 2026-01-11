package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/randalmurphal/orc/internal/planner"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Generate tasks from specification documents",
	Long: `Reads specification documents and uses Claude to generate a task breakdown.

The plan command:
1. Loads spec files from the specified directory (default: .spec/)
2. Generates a planning prompt with all spec content
3. Runs Claude to analyze specs and propose tasks
4. Displays the proposed tasks for approval
5. Creates tasks with dependencies

Examples:
  # Plan from default .spec/ directory
  orc plan

  # Plan from custom directory
  orc plan --from docs/specs/

  # Create tasks without confirmation
  orc plan --yes

  # Link tasks to an existing initiative
  orc plan --initiative INIT-001

  # Create a new initiative for the tasks
  orc plan --create-initiative

  # Show prompt without running Claude
  orc plan --dry-run`,
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
)

func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.Flags().StringVarP(&planFrom, "from", "f", ".spec", "Directory containing spec documents")
	planCmd.Flags().BoolVarP(&planYes, "yes", "y", false, "Create tasks without confirmation")
	planCmd.Flags().BoolVar(&planDryRun, "dry-run", false, "Show prompt without running Claude")
	planCmd.Flags().StringVarP(&planInitiative, "initiative", "i", "", "Link tasks to existing initiative")
	planCmd.Flags().BoolVarP(&planCreateInitiative, "create-initiative", "I", false, "Create new initiative for tasks")
	planCmd.Flags().StringVarP(&planModel, "model", "m", "", "Claude model to use")
	planCmd.Flags().StringSliceVar(&planInclude, "include", []string{"*.md"}, "File patterns to include")
}

func runPlan(cmd *cobra.Command, args []string) error {
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
