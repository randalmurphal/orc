package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/template"
)

// Note: getBackend is imported from commands.go

// newTemplateCmd creates the template command group.
func newTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage task templates",
		Long: `Manage reusable task templates.

Templates capture successful task patterns including:
  • Weight classification
  • Phase sequence
  • Custom prompts
  • Variable substitutions

Built-in templates: bugfix, feature, refactor, migration, spike`,
	}

	cmd.AddCommand(newTemplateListCmd())
	cmd.AddCommand(newTemplateShowCmd())
	cmd.AddCommand(newTemplateSaveCmd())
	cmd.AddCommand(newTemplateDeleteCmd())

	return cmd
}

// newTemplateListCmd creates the template list command.
func newTemplateListCmd() *cobra.Command {
	var (
		showGlobal  bool
		showLocal   bool
		showBuiltin bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List available templates",
		Long: `List all available templates.

By default, shows all templates (project, global, and built-in).
Use filters to show only specific types.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			templates, err := template.List()
			if err != nil {
				return fmt.Errorf("list templates: %w", err)
			}

			// Filter by scope if requested
			if showGlobal || showLocal || showBuiltin {
				var filtered []template.TemplateInfo
				for _, t := range templates {
					if (showLocal && t.Scope == template.ScopeProject) ||
						(showGlobal && t.Scope == template.ScopeGlobal) ||
						(showBuiltin && t.Scope == template.ScopeBuiltin) {
						filtered = append(filtered, t)
					}
				}
				templates = filtered
			}

			if jsonOut {
				data, _ := json.MarshalIndent(templates, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if len(templates) == 0 {
				fmt.Println("No templates found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tWEIGHT\tPHASES\tSCOPE\tDESCRIPTION")
			_, _ = fmt.Fprintln(w, "────\t──────\t──────\t─────\t───────────")

			for _, t := range templates {
				phases := strings.Join(t.Phases, ",")
				if len(phases) > 20 {
					phases = phases[:17] + "..."
				}
				desc := t.Description
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					t.Name, t.Weight, phases, t.Scope, desc)
			}
			_ = w.Flush()

			return nil
		},
	}

	cmd.Flags().BoolVarP(&showGlobal, "global", "g", false, "Show only global templates")
	cmd.Flags().BoolVarP(&showLocal, "local", "l", false, "Show only local templates")
	cmd.Flags().BoolVarP(&showBuiltin, "builtin", "b", false, "Show only built-in templates")

	return cmd
}

// newTemplateShowCmd creates the template show command.
func newTemplateShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show template details",
		Long:  "Display detailed information about a template.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			t, err := template.Load(name)
			if err != nil {
				return fmt.Errorf("template %q not found", name)
			}

			if jsonOut {
				data, _ := json.MarshalIndent(t, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Template: %s\n", t.Name)
			fmt.Println("────────────────")
			fmt.Println()
			fmt.Printf("Weight: %s\n", t.Weight)
			fmt.Printf("Phases: %s\n", strings.Join(t.Phases, " → "))
			fmt.Printf("Scope:  %s\n", t.Scope)

			if t.Description != "" {
				fmt.Println()
				fmt.Printf("Description:\n  %s\n", t.Description)
			}

			if len(t.Variables) > 0 {
				fmt.Println()
				fmt.Println("Variables:")
				for _, v := range t.Variables {
					required := ""
					if v.Required {
						required = " (required)"
					}
					fmt.Printf("  {{%s}}%s\n", v.Name, required)
					if v.Description != "" {
						fmt.Printf("    %s\n", v.Description)
					}
				}
			}

			if len(t.Prompts) > 0 {
				fmt.Println()
				fmt.Println("Custom Prompts:")
				for phase, file := range t.Prompts {
					fmt.Printf("  %s: %s\n", phase, file)
				}
			}

			if t.CreatedFrom != "" {
				fmt.Println()
				fmt.Printf("Created from: %s\n", t.CreatedFrom)
			}

			fmt.Println()
			fmt.Printf("Usage:\n  orc new --template %s \"your task title\"\n", t.Name)

			return nil
		},
	}

	return cmd
}

// newTemplateSaveCmd creates the template save command.
func newTemplateSaveCmd() *cobra.Command {
	var (
		name        string
		description string
		global      bool
	)

	cmd := &cobra.Command{
		Use:   "save <task-id>",
		Short: "Save a task as a template",
		Long: `Save a completed task as a reusable template.

This captures:
  • Task weight
  • Phase sequence
  • Custom prompts (if any)

Example:
  orc template save TASK-001 --name bugfix
  orc template save TASK-042 --name migration --global`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return wrapNotInitialized()
			}

			taskID := args[0]

			if name == "" {
				return fmt.Errorf("--name is required")
			}

			if err := template.ValidateName(name); err != nil {
				return err
			}

			// Check if template already exists
			if template.Exists(name) {
				return fmt.Errorf("template %q already exists", name)
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			t, err := template.SaveFromTask(taskID, name, description, global, backend)
			if err != nil {
				return fmt.Errorf("save template: %w", err)
			}

			if jsonOut {
				data, _ := json.MarshalIndent(t, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			fmt.Printf("Template saved: %s\n", t.Name)
			fmt.Println()
			fmt.Println("Captured:")
			fmt.Printf("  • Weight: %s\n", t.Weight)
			fmt.Printf("  • Phases: %s\n", strings.Join(t.Phases, " → "))
			if len(t.Prompts) > 0 {
				fmt.Printf("  • Custom prompts: %s\n", strings.Join(promptKeys(t.Prompts), ", "))
			}
			fmt.Println()
			fmt.Printf("Use with:\n  orc new --template %s \"task title\"\n", t.Name)

			return nil
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "Template name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Template description")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "Save to global templates (~/.orc/templates/)")
	_ = cmd.MarkFlagRequired("name")

	return cmd
}

// newTemplateDeleteCmd creates the template delete command.
func newTemplateDeleteCmd() *cobra.Command {
	var global bool

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a template",
		Long:  "Delete a template from project or global templates.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			t, err := template.Load(name)
			if err != nil {
				return fmt.Errorf("template %q not found", name)
			}

			if t.Scope == template.ScopeBuiltin {
				return fmt.Errorf("cannot delete built-in template %q", name)
			}

			if err := t.Delete(); err != nil {
				return fmt.Errorf("delete template: %w", err)
			}

			if !quiet {
				fmt.Printf("Template %q deleted.\n", name)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&global, "global", "g", false, "Delete from global templates")

	return cmd
}

// promptKeys returns the keys of a prompts map.
func promptKeys(prompts map[string]string) []string {
	var keys []string
	for k := range prompts {
		keys = append(keys, k)
	}
	return keys
}
