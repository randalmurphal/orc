// Package cli implements the orc command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/gate"
)

// newGatesCmd creates the gates command for inspecting gate configurations.
func newGatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gates",
		Short: "Inspect gate configurations",
	}
	cmd.AddCommand(newGatesListCmd())
	cmd.AddCommand(newGatesShowCmd())
	return cmd
}

// gatePhaseInfo holds resolved gate information for a workflow phase.
type gatePhaseInfo struct {
	PhaseID        string `json:"phase"`
	GateType       string `json:"gate_type"`
	Source         string `json:"source"`
	RetryFromPhase string `json:"retry_from_phase,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
}

// gatesContext holds all data needed by gates subcommands.
type gatesContext struct {
	cfg       *config.Config
	templates []*db.PhaseTemplate
	resolver  *gate.Resolver
}

// loadGatesContext loads the config, project DB, phase templates, and builds a gate resolver.
func loadGatesContext() (*gatesContext, error) {
	cfgPath := filepath.Join(".orc", "config.yaml")
	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	if cfg.Workflow == "" {
		return nil, fmt.Errorf("no active workflow; run `orc init` first")
	}

	pdb, err := db.OpenProject(".")
	if err != nil {
		return nil, fmt.Errorf("open project database: %w", err)
	}
	defer func() { _ = pdb.Close() }()

	phases, err := pdb.GetWorkflowPhases(cfg.Workflow)
	if err != nil {
		return nil, fmt.Errorf("load workflow phases: %w", err)
	}
	if len(phases) == 0 {
		return nil, fmt.Errorf("no active workflow; run `orc init` first")
	}

	templates := make([]*db.PhaseTemplate, 0, len(phases))
	phaseGates := make(map[string]*db.PhaseGate, len(phases))
	for _, ph := range phases {
		tmpl, err := pdb.GetPhaseTemplate(ph.PhaseTemplateID)
		if err != nil {
			return nil, fmt.Errorf("load phase template %s: %w", ph.PhaseTemplateID, err)
		}
		templates = append(templates, tmpl)

		if tmpl.GateType != "" {
			phaseGates[tmpl.ID] = &db.PhaseGate{
				PhaseID:  tmpl.ID,
				GateType: tmpl.GateType,
				Enabled:  true,
			}
		}
	}

	resolver := gate.NewResolver(cfg, gate.WithPhaseGates(phaseGates))

	return &gatesContext{
		cfg:       cfg,
		templates: templates,
		resolver:  resolver,
	}, nil
}

// newGatesListCmd creates the gates list subcommand.
func newGatesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List gate configurations for all phases",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := loadGatesContext()
			if err != nil {
				return err
			}

			infos := make([]gatePhaseInfo, 0, len(ctx.templates))
			for _, tmpl := range ctx.templates {
				result := ctx.resolver.Resolve(tmpl.ID, "")
				infos = append(infos, gatePhaseInfo{
					PhaseID:  tmpl.ID,
					GateType: string(result.GateType),
					Source:   result.Source,
				})
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(infos)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "PHASE\tGATE TYPE\tSOURCE")
			for _, info := range infos {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", info.PhaseID, info.GateType, info.Source)
			}
			return w.Flush()
		},
	}
}

// newGatesShowCmd creates the gates show subcommand.
func newGatesShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <phase>",
		Short: "Show detailed gate configuration for a phase",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			phaseID := args[0]

			ctx, err := loadGatesContext()
			if err != nil {
				return err
			}

			// Find the requested phase template.
			var tmpl *db.PhaseTemplate
			for _, t := range ctx.templates {
				if t.ID == phaseID {
					tmpl = t
					break
				}
			}
			if tmpl == nil {
				return fmt.Errorf("phase '%s' not found in current workflow", phaseID)
			}

			result := ctx.resolver.Resolve(tmpl.ID, "")

			info := gatePhaseInfo{
				PhaseID:        tmpl.ID,
				GateType:       string(result.GateType),
				Source:         result.Source,
				RetryFromPhase: tmpl.RetryFromPhase,
				AgentID:        tmpl.AgentID,
			}

			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}

			fmt.Printf("Phase:           %s\n", info.PhaseID)
			fmt.Printf("Gate Type:       %s\n", info.GateType)
			fmt.Printf("Source:          %s\n", info.Source)
			if info.RetryFromPhase != "" {
				fmt.Printf("Retry From:      %s\n", info.RetryFromPhase)
			}
			if info.AgentID != "" {
				fmt.Printf("Agent:           %s\n", info.AgentID)
			}
			return nil
		},
	}
}
