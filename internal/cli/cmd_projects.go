// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/project"
)

// newProjectsCmd creates the projects command for listing registered projects
func newProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage registered orc projects",
		Long: `Manage orc projects registered on this machine.

Projects are automatically registered when running 'orc init'.
The registry is stored at ~/.orc/projects.json.

Commands:
  projects          List all registered projects
  projects add      Register a project directory
  projects remove   Unregister a project
  projects default  Set or show the default project

Example:
  orc projects                  # List all projects
  orc projects add .            # Register current directory
  orc projects remove abc123    # Unregister project by ID
  orc projects default abc123   # Set default project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := project.ListProjects()
			if err != nil {
				return fmt.Errorf("list projects: %w", err)
			}

			if len(projects) == 0 {
				fmt.Println("No projects registered. Run 'orc init' in a project directory.")
				return nil
			}

			// Get default project
			defaultID, _ := project.GetDefaultProject()

			if jsonOut {
				// JSON output handled separately
				fmt.Println("[")
				for i, p := range projects {
					comma := ","
					if i == len(projects)-1 {
						comma = ""
					}
					isDefault := p.ID == defaultID
					fmt.Printf(`  {"id": "%s", "name": "%s", "path": "%s", "default": %v}%s`+"\n",
						p.ID, p.Name, p.Path, isDefault, comma)
				}
				fmt.Println("]")
				return nil
			}

			// Table output
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tNAME\tPATH\tDEFAULT")
			for _, p := range projects {
				isDefault := ""
				if p.ID == defaultID {
					isDefault = "*"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.ID, p.Name, p.Path, isDefault)
			}
			_ = w.Flush()

			return nil
		},
	}

	// Add subcommands
	cmd.AddCommand(newProjectsAddCmd())
	cmd.AddCommand(newProjectsRemoveCmd())
	cmd.AddCommand(newProjectsDefaultCmd())

	return cmd
}

// newProjectsAddCmd creates the projects add command
func newProjectsAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add [path]",
		Short: "Register a project directory",
		Long: `Register a project directory with orc.

If no path is provided, the current directory is used.
Projects can be referenced by ID, name, or path when using --project flag.

Examples:
  orc projects add               # Register current directory
  orc projects add .             # Same as above
  orc projects add ~/repos/myapp # Register a specific directory`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) > 0 {
				path = args[0]
			}

			proj, err := project.RegisterProject(path)
			if err != nil {
				return fmt.Errorf("register project: %w", err)
			}

			fmt.Printf("Registered project:\n")
			fmt.Printf("  ID:   %s\n", proj.ID)
			fmt.Printf("  Name: %s\n", proj.Name)
			fmt.Printf("  Path: %s\n", proj.Path)
			fmt.Println()
			fmt.Printf("Use with: orc --project %s <command>\n", proj.ID)
			fmt.Printf("      or: orc -P %s <command>\n", proj.ID)

			return nil
		},
	}
}

// newProjectsRemoveCmd creates the projects remove command
func newProjectsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id-or-path>",
		Short: "Unregister a project",
		Long: `Unregister a project from orc.

You can specify the project by ID or path.
This does not delete any files, only removes the registration.

Examples:
  orc projects remove abc123           # Remove by ID
  orc projects remove ~/repos/myapp    # Remove by path`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			idOrPath := args[0]

			reg, err := project.LoadRegistry()
			if err != nil {
				return fmt.Errorf("load registry: %w", err)
			}

			// Get project info before removing (for display)
			proj, _ := reg.Get(idOrPath)

			if err := reg.Unregister(idOrPath); err != nil {
				return fmt.Errorf("unregister project: %w", err)
			}

			if err := reg.Save(); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			if proj != nil {
				fmt.Printf("Unregistered project: %s (%s)\n", proj.Name, proj.ID)
			} else {
				fmt.Printf("Unregistered project: %s\n", idOrPath)
			}

			return nil
		},
	}
}

// newProjectsDefaultCmd creates the projects default command
func newProjectsDefaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "default [id]",
		Short: "Set or show the default project",
		Long: `Set or show the default project.

When a default project is set, it will be used automatically when no
--project flag is provided and you're not in a project directory.

Examples:
  orc projects default           # Show current default
  orc projects default abc123    # Set default project
  orc projects default --clear   # Clear default project`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clear, _ := cmd.Flags().GetBool("clear")

			if clear {
				if err := project.SetDefaultProject(""); err != nil {
					return fmt.Errorf("clear default: %w", err)
				}
				fmt.Println("Default project cleared.")
				return nil
			}

			if len(args) == 0 {
				// Show current default
				defaultID, err := project.GetDefaultProject()
				if err != nil {
					return fmt.Errorf("get default: %w", err)
				}
				if defaultID == "" {
					fmt.Println("No default project set.")
					fmt.Println("Set one with: orc projects default <project-id>")
					return nil
				}

				reg, err := project.LoadRegistry()
				if err != nil {
					return fmt.Errorf("load registry: %w", err)
				}

				proj, err := reg.Get(defaultID)
				if err != nil {
					fmt.Printf("Default project: %s (warning: not found in registry)\n", defaultID)
					return nil
				}

				fmt.Printf("Default project: %s\n", proj.Name)
				fmt.Printf("  ID:   %s\n", proj.ID)
				fmt.Printf("  Path: %s\n", proj.Path)
				return nil
			}

			// Set default
			projectID := args[0]
			if err := project.SetDefaultProject(projectID); err != nil {
				return fmt.Errorf("set default: %w", err)
			}

			fmt.Printf("Default project set to: %s\n", projectID)
			return nil
		},
	}

	cmd.Flags().Bool("clear", false, "Clear the default project")
	return cmd
}
