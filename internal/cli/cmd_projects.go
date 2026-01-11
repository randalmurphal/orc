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
		Short: "List registered orc projects",
		Long: `List all orc projects registered on this machine.

Projects are automatically registered when running 'orc init'.
The registry is stored at ~/.orc/projects.yaml.

Example:
  orc projects             # List all projects
  orc projects --json      # Output as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			projects, err := project.ListProjects()
			if err != nil {
				return fmt.Errorf("list projects: %w", err)
			}

			if len(projects) == 0 {
				fmt.Println("No projects registered. Run 'orc init' in a project directory.")
				return nil
			}

			if jsonOut {
				// JSON output handled separately
				fmt.Println("[")
				for i, p := range projects {
					comma := ","
					if i == len(projects)-1 {
						comma = ""
					}
					fmt.Printf(`  {"id": "%s", "name": "%s", "path": "%s"}%s`+"\n",
						p.ID, p.Name, p.Path, comma)
				}
				fmt.Println("]")
				return nil
			}

			// Table output
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tPATH")
			for _, p := range projects {
				fmt.Fprintf(w, "%s\t%s\t%s\n", p.ID, p.Name, p.Path)
			}
			w.Flush()

			return nil
		},
	}
	return cmd
}
