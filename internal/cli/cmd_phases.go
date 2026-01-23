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
	rootCmd.AddCommand(phasesCmd)
	phasesCmd.AddCommand(phaseShowCmd)
	phasesCmd.AddCommand(phaseNewCmd)
	phasesCmd.AddCommand(phaseConfigCmd)

	// List flags
	phasesCmd.Flags().Bool("custom", false, "Show only custom phase templates")
	phasesCmd.Flags().Bool("builtin", false, "Show only built-in phase templates")

	// New flags
	phaseNewCmd.Flags().String("prompt", "", "Inline prompt content")
	phaseNewCmd.Flags().String("prompt-file", "", "Load prompt from file")
	phaseNewCmd.Flags().Int("max-iterations", 20, "Maximum iterations")
	phaseNewCmd.Flags().String("gate", "auto", "Gate type (auto, human, none)")
	phaseNewCmd.Flags().Bool("artifact", false, "Phase produces an artifact")

	// Config flags
	phaseConfigCmd.Flags().String("model", "", "Model override")
	phaseConfigCmd.Flags().Int("max-iterations", 0, "Max iterations override")
	phaseConfigCmd.Flags().String("gate", "", "Gate type override")
	phaseConfigCmd.Flags().Bool("thinking", false, "Enable extended thinking")
}

var phasesCmd = &cobra.Command{
	Use:     "phases",
	Aliases: []string{"phase"},
	Short:   "List available phase templates",
	Long: `List all phase templates available for use in workflows.

Phase templates define reusable execution units with prompts, configuration,
and input/output contracts. Built-in templates provide standard phases like
'spec', 'implement', 'review'. You can create custom templates for specialized
workflows.

Examples:
  orc phases                   # List all phase templates
  orc phases --custom          # List only custom templates
  orc phases --builtin         # List only built-in templates`,
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

		templates, err := pdb.ListPhaseTemplates()
		if err != nil {
			return fmt.Errorf("list phase templates: %w", err)
		}

		customOnly, _ := cmd.Flags().GetBool("custom")
		builtinOnly, _ := cmd.Flags().GetBool("builtin")

		// Filter templates
		var filtered []*db.PhaseTemplate
		for _, t := range templates {
			if customOnly && t.IsBuiltin {
				continue
			}
			if builtinOnly && !t.IsBuiltin {
				continue
			}
			filtered = append(filtered, t)
		}

		if len(filtered) == 0 {
			fmt.Println("No phase templates found.")
			return nil
		}

		// Display as table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tNAME\tSOURCE\tMAX ITER\tGATE\tARTIFACT\tBUILT-IN")
		for _, t := range filtered {
			artifact := ""
			if t.ProducesArtifact {
				artifact = "yes"
			}
			builtin := ""
			if t.IsBuiltin {
				builtin = "yes"
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
				t.ID, t.Name, t.PromptSource, t.MaxIterations, t.GateType, artifact, builtin)
		}
		_ = w.Flush()

		return nil
	},
}

var phaseShowCmd = &cobra.Command{
	Use:   "show <phase-id>",
	Short: "Show phase template details",
	Long: `Display detailed information about a phase template including
its prompt content, configuration, and input/output contracts.

Examples:
  orc phase show spec          # Show the spec phase template
  orc phase show my-security   # Show a custom phase template`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phaseID := args[0]

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		t, err := pdb.GetPhaseTemplate(phaseID)
		if err != nil {
			return fmt.Errorf("get phase template: %w", err)
		}
		if t == nil {
			return fmt.Errorf("phase template not found: %s", phaseID)
		}

		// Display template info
		fmt.Printf("Phase Template: %s\n", t.ID)
		fmt.Printf("Name: %s\n", t.Name)
		if t.Description != "" {
			fmt.Printf("Description: %s\n", t.Description)
		}
		fmt.Printf("Prompt Source: %s\n", t.PromptSource)
		if t.PromptPath != "" {
			fmt.Printf("Prompt Path: %s\n", t.PromptPath)
		}
		fmt.Printf("Max Iterations: %d\n", t.MaxIterations)
		if t.ModelOverride != "" {
			fmt.Printf("Model Override: %s\n", t.ModelOverride)
		}
		if t.ThinkingEnabled != nil && *t.ThinkingEnabled {
			fmt.Println("Extended Thinking: enabled")
		}
		fmt.Printf("Gate Type: %s\n", t.GateType)
		if t.ProducesArtifact {
			fmt.Println("Produces Artifact: yes")
			if t.ArtifactType != "" {
				fmt.Printf("Artifact Type: %s\n", t.ArtifactType)
			}
		}
		if t.Checkpoint {
			fmt.Println("Checkpoint: yes")
		}
		if t.IsBuiltin {
			fmt.Println("Built-in: yes")
		}

		// Show input variables if defined
		if t.InputVariables != "" && t.InputVariables != "[]" {
			fmt.Printf("\nInput Variables: %s\n", t.InputVariables)
		}

		// Show prompt content if inline
		if t.PromptSource == "db" && t.PromptContent != "" {
			fmt.Println("\nPrompt Content:")
			fmt.Println("---")
			if len(t.PromptContent) > 500 {
				fmt.Printf("%s...\n(truncated, use --full to see complete prompt)\n", t.PromptContent[:500])
			} else {
				fmt.Println(t.PromptContent)
			}
			fmt.Println("---")
		}

		return nil
	},
}

var phaseNewCmd = &cobra.Command{
	Use:   "new <phase-id>",
	Short: "Create a new phase template",
	Long: `Create a new phase template for use in workflows.

A phase template defines a reusable execution unit with a prompt,
configuration, and input/output contracts.

Examples:
  orc phase new my-security --prompt "Review code for security vulnerabilities..."
  orc phase new my-lint --prompt-file prompts/lint.md --max-iterations 5
  orc phase new my-docs --gate human --artifact`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phaseID := args[0]

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		// Check if template already exists
		existing, err := pdb.GetPhaseTemplate(phaseID)
		if err != nil {
			return fmt.Errorf("check existing: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("phase template already exists: %s", phaseID)
		}

		prompt, _ := cmd.Flags().GetString("prompt")
		promptFile, _ := cmd.Flags().GetString("prompt-file")
		maxIter, _ := cmd.Flags().GetInt("max-iterations")
		gate, _ := cmd.Flags().GetString("gate")
		artifact, _ := cmd.Flags().GetBool("artifact")

		// Get prompt content
		var promptContent string
		var promptSource string
		if promptFile != "" {
			content, err := os.ReadFile(promptFile)
			if err != nil {
				return fmt.Errorf("read prompt file: %w", err)
			}
			promptContent = string(content)
			promptSource = "db" // Store in database
		} else if prompt != "" {
			promptContent = prompt
			promptSource = "db"
		} else {
			return fmt.Errorf("either --prompt or --prompt-file is required")
		}

		tmpl := &db.PhaseTemplate{
			ID:               phaseID,
			Name:             phaseID,
			PromptSource:     promptSource,
			PromptContent:    promptContent,
			MaxIterations:    maxIter,
			GateType:         gate,
			ProducesArtifact: artifact,
			Checkpoint:       true,
			IsBuiltin:        false,
		}

		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			return fmt.Errorf("save phase template: %w", err)
		}

		fmt.Printf("Created phase template '%s'\n", phaseID)
		return nil
	},
}

var phaseConfigCmd = &cobra.Command{
	Use:   "config <phase-id>",
	Short: "Configure a phase template",
	Long: `Update configuration options for a phase template.

Note: Built-in phase templates cannot be modified. Create a custom
template to customize behavior.

Examples:
  orc phase config my-security --model opus --thinking
  orc phase config my-lint --max-iterations 10
  orc phase config my-docs --gate human`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phaseID := args[0]

		projectRoot, err := config.FindProjectRoot()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		tmpl, err := pdb.GetPhaseTemplate(phaseID)
		if err != nil {
			return fmt.Errorf("get phase template: %w", err)
		}
		if tmpl == nil {
			return fmt.Errorf("phase template not found: %s", phaseID)
		}

		if tmpl.IsBuiltin {
			return fmt.Errorf("cannot modify built-in phase template: %s", phaseID)
		}

		// Apply updates
		changed := false

		if cmd.Flags().Changed("model") {
			model, _ := cmd.Flags().GetString("model")
			tmpl.ModelOverride = model
			changed = true
		}

		if cmd.Flags().Changed("max-iterations") {
			maxIter, _ := cmd.Flags().GetInt("max-iterations")
			tmpl.MaxIterations = maxIter
			changed = true
		}

		if cmd.Flags().Changed("gate") {
			gate, _ := cmd.Flags().GetString("gate")
			tmpl.GateType = gate
			changed = true
		}

		if cmd.Flags().Changed("thinking") {
			thinking, _ := cmd.Flags().GetBool("thinking")
			tmpl.ThinkingEnabled = &thinking
			changed = true
		}

		if !changed {
			fmt.Println("No configuration changes specified.")
			return nil
		}

		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			return fmt.Errorf("save phase template: %w", err)
		}

		fmt.Printf("Updated phase template '%s'\n", phaseID)
		return nil
	},
}
