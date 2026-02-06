// Package cli implements the orc command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

func init() {
	rootCmd.AddCommand(phasesCmd)
	phasesCmd.AddCommand(phaseShowCmd)
	phasesCmd.AddCommand(phaseNewCmd)
	phasesCmd.AddCommand(phaseConfigCmd)
	phasesCmd.AddCommand(phaseCloneCmd)

	// List flags
	phasesCmd.Flags().Bool("custom", false, "Show only custom phase templates")
	phasesCmd.Flags().Bool("builtin", false, "Show only built-in phase templates")
	phasesCmd.Flags().Bool("sources", false, "Show source locations for each phase")

	// Clone flags
	phaseCloneCmd.Flags().StringP("level", "l", "project", "Target level: personal, local, shared, project")
	phaseCloneCmd.Flags().BoolP("force", "f", false, "Overwrite if exists")

	// New flags
	phaseNewCmd.Flags().String("prompt", "", "Inline prompt content")
	phaseNewCmd.Flags().String("prompt-file", "", "Load prompt from file")
	phaseNewCmd.Flags().Int("max-iterations", 20, "Maximum iterations")
	phaseNewCmd.Flags().String("gate", "auto", "Gate type (auto, human, none)")
	phaseNewCmd.Flags().Bool("artifact", false, "Phase produces an artifact")
	phaseNewCmd.Flags().String("agent", "", "Executor agent ID (the agent that runs this phase)")

	// Config flags
	phaseConfigCmd.Flags().String("agent", "", "Executor agent ID")
	phaseConfigCmd.Flags().Int("max-iterations", 0, "Max iterations override")
	phaseConfigCmd.Flags().String("gate", "", "Gate type override")
	phaseConfigCmd.Flags().Bool("thinking", false, "Enable extended thinking")
}

var phasesCmd = &cobra.Command{
	Use:     "phases [TASK-ID]",
	Aliases: []string{"phase"},
	Short:   "List phase templates or show phase execution history",
	Long: `List all phase templates available for use in workflows, or show
phase execution history for a specific task.

When called with a TASK-ID argument, displays phase execution history
including timing, cost, and iteration counts for each phase.

When called without arguments, lists available phase templates.

Sources (--sources flag):
  personal  - ~/.orc/phases/ (user machine-wide)
  local     - .orc/local/phases/ (personal project-specific)
  shared    - .orc/shared/phases/ (team defaults)
  project   - .orc/phases/ (project defaults)
  embedded  - Built into the binary

Examples:
  orc phases TASK-001            # Show phase execution history
  orc phases TASK-001 --json     # Phase history as JSON
  orc phases                     # List all phase templates
  orc phases --sources           # Show where each phase comes from
  orc phases --custom            # List only custom templates
  orc phases --builtin           # List only built-in templates`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If a task ID argument is provided, show phase execution history
		if len(args) > 0 {
			return runPhaseHistory(args[0])
		}

		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		orcDir := filepath.Join(projectRoot, ".orc")
		resolver := workflow.NewResolverFromOrcDir(orcDir)

		showSources, _ := cmd.Flags().GetBool("sources")
		customOnly, _ := cmd.Flags().GetBool("custom")
		builtinOnly, _ := cmd.Flags().GetBool("builtin")

		phases, err := resolver.ListPhases()
		if err != nil {
			return fmt.Errorf("list phases: %w", err)
		}

		// Filter phases
		var filtered []workflow.ResolvedPhase
		for _, rp := range phases {
			isBuiltin := rp.Source == workflow.SourceEmbedded
			if customOnly && isBuiltin {
				continue
			}
			if builtinOnly && !isBuiltin {
				continue
			}
			filtered = append(filtered, rp)
		}

		if len(filtered) == 0 {
			fmt.Println("No phase templates found.")
			return nil
		}

		// Display as table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if showSources {
			_, _ = fmt.Fprintln(w, "ID\tNAME\tGATE\tSOURCE")
		} else {
			_, _ = fmt.Fprintln(w, "ID\tNAME\tGATE\tARTIFACT\tBUILT-IN")
		}
		for _, rp := range filtered {
			p := rp.Phase
			if showSources {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					p.ID, p.Name, p.GateType,
					workflow.SourceDisplayName(rp.Source))
			} else {
				artifact := ""
				if p.ProducesArtifact {
					artifact = "yes"
				}
				builtin := ""
				if rp.Source == workflow.SourceEmbedded {
					builtin = "yes"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					p.ID, p.Name, p.GateType, artifact, builtin)
			}
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

		projectRoot, err := ResolveProjectPath()
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
		if t.AgentID != "" {
			fmt.Printf("Executor Agent: %s\n", t.AgentID)
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

		projectRoot, err := ResolveProjectPath()
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
		gate, _ := cmd.Flags().GetString("gate")
		artifact, _ := cmd.Flags().GetBool("artifact")
		agentID, _ := cmd.Flags().GetString("agent")

		// Validate agent exists if specified
		if agentID != "" {
			agent, err := pdb.GetAgent(agentID)
			if err != nil {
				return fmt.Errorf("get agent: %w", err)
			}
			if agent == nil {
				return fmt.Errorf("agent not found: %s", agentID)
			}
		}

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
			AgentID:          agentID,
			PromptSource:     promptSource,
			PromptContent:    promptContent,
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

		projectRoot, err := ResolveProjectPath()
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

		if cmd.Flags().Changed("agent") {
			agentID, _ := cmd.Flags().GetString("agent")
			// Validate agent exists if specified
			if agentID != "" {
				agent, err := pdb.GetAgent(agentID)
				if err != nil {
					return fmt.Errorf("get agent: %w", err)
				}
				if agent == nil {
					return fmt.Errorf("agent not found: %s", agentID)
				}
			}
			tmpl.AgentID = agentID
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

// runPhaseHistory displays phase execution history for a task.
func runPhaseHistory(taskID string) error {
	backend, err := getBackend()
	if err != nil {
		return err
	}
	defer func() { _ = backend.Close() }()

	// Load task to verify it exists and get workflow ID
	tk, err := backend.LoadTask(taskID)
	if err != nil {
		return fmt.Errorf("load task %s: %w", taskID, err)
	}
	if tk == nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	// Get phase execution records
	phases, err := backend.DB().GetPhases(taskID)
	if err != nil {
		return fmt.Errorf("get phases for %s: %w", taskID, err)
	}

	// Build phase map for lookup
	phaseMap := make(map[string]*db.Phase)
	for i := range phases {
		phaseMap[phases[i].PhaseID] = &phases[i]
	}

	// Determine display order from workflow sequence
	var orderedPhaseIDs []string
	if tk.WorkflowId != nil && *tk.WorkflowId != "" {
		wfPhases, wfErr := backend.GetWorkflowPhases(*tk.WorkflowId)
		if wfErr == nil && len(wfPhases) > 0 {
			seen := make(map[string]bool)
			for _, wp := range wfPhases {
				orderedPhaseIDs = append(orderedPhaseIDs, wp.PhaseTemplateID)
				seen[wp.PhaseTemplateID] = true
			}
			// Append any executed phases not in the workflow definition
			for _, ph := range phases {
				if !seen[ph.PhaseID] {
					orderedPhaseIDs = append(orderedPhaseIDs, ph.PhaseID)
				}
			}
		}
	}

	// Fallback: use phases from DB in their stored order
	if len(orderedPhaseIDs) == 0 {
		for _, ph := range phases {
			orderedPhaseIDs = append(orderedPhaseIDs, ph.PhaseID)
		}
	}

	if jsonOut {
		return printPhaseHistoryJSON(taskID, orderedPhaseIDs, phaseMap)
	}
	return printPhaseHistoryTable(orderedPhaseIDs, phaseMap)
}

// printPhaseHistoryTable renders phase execution history as a table.
func printPhaseHistoryTable(orderedPhaseIDs []string, phaseMap map[string]*db.Phase) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "PHASE\tSTATUS\tSTARTED\tCOMPLETED\tDURATION\tITERATIONS\tCOST")

	var totalCost float64
	var totalIterations int

	for _, phaseID := range orderedPhaseIDs {
		ph, ok := phaseMap[phaseID]
		if !ok {
			continue
		}

		started := "-"
		completed := "-"
		duration := "-"
		iterations := fmt.Sprintf("%d", ph.Iterations)
		cost := formatCost(ph.CostUSD)

		if ph.StartedAt != nil {
			started = ph.StartedAt.Format("2006-01-02 15:04")
		}
		if ph.CompletedAt != nil {
			completed = ph.CompletedAt.Format("2006-01-02 15:04")
		}
		if ph.StartedAt != nil && ph.CompletedAt != nil {
			dur := ph.CompletedAt.Sub(*ph.StartedAt)
			duration = task.FormatDuration(dur)
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			phaseID, ph.Status, started, completed, duration, iterations, cost)

		totalCost += ph.CostUSD
		totalIterations += ph.Iterations
	}

	_, _ = fmt.Fprintf(w, "TOTAL\t\t\t\t\t%d\t%s\n",
		totalIterations, formatCost(totalCost))

	_ = w.Flush()
	return nil
}

// printPhaseHistoryJSON renders phase execution history as JSON.
func printPhaseHistoryJSON(taskID string, orderedPhaseIDs []string, phaseMap map[string]*db.Phase) error {
	type phaseEntry struct {
		Phase           string  `json:"phase"`
		Status          string  `json:"status"`
		StartedAt       string  `json:"started_at"`
		CompletedAt     string  `json:"completed_at"`
		DurationSeconds float64 `json:"duration_seconds"`
		Iterations      int     `json:"iterations"`
		CostUSD         float64 `json:"cost_usd"`
		InputTokens     int     `json:"input_tokens"`
		OutputTokens    int     `json:"output_tokens"`
	}

	entries := make([]phaseEntry, 0)
	var totalCost float64
	var totalIterations int

	for _, phaseID := range orderedPhaseIDs {
		ph, ok := phaseMap[phaseID]
		if !ok {
			continue
		}

		entry := phaseEntry{
			Phase:        phaseID,
			Status:       ph.Status,
			Iterations:   ph.Iterations,
			CostUSD:      ph.CostUSD,
			InputTokens:  ph.InputTokens,
			OutputTokens: ph.OutputTokens,
		}

		if ph.StartedAt != nil {
			entry.StartedAt = ph.StartedAt.Format(time.RFC3339)
		}
		if ph.CompletedAt != nil {
			entry.CompletedAt = ph.CompletedAt.Format(time.RFC3339)
		}
		if ph.StartedAt != nil && ph.CompletedAt != nil {
			dur := ph.CompletedAt.Sub(*ph.StartedAt)
			entry.DurationSeconds = dur.Seconds()
		}

		entries = append(entries, entry)
		totalCost += ph.CostUSD
		totalIterations += ph.Iterations
	}

	result := map[string]any{
		"task_id": taskID,
		"phases":  entries,
		"totals": map[string]any{
			"cost_usd":   totalCost,
			"iterations": totalIterations,
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

var phaseCloneCmd = &cobra.Command{
	Use:   "clone <source-id> <dest-id>",
	Short: "Clone a phase template to a new file",
	Long: `Clone a phase template (built-in or custom) to create a new customizable copy.

The cloned phase is written to a YAML file and can be edited directly.
Use --level to control where the clone is created:

Levels:
  personal  - ~/.orc/phases/ (user machine-wide, not shared)
  local     - .orc/local/phases/ (personal project-specific, gitignored)
  shared    - .orc/shared/phases/ (team defaults, git-tracked)
  project   - .orc/phases/ (project defaults, git-tracked) [default]

Examples:
  orc phase clone implement my-implement           # Clone to .orc/phases/
  orc phase clone implement my-implement -l local  # Clone to .orc/local/phases/
  orc phase clone spec my-spec --force             # Overwrite if exists`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sourceID := args[0]
		destID := args[1]

		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		levelStr, _ := cmd.Flags().GetString("level")
		force, _ := cmd.Flags().GetBool("force")

		level, err := workflow.ParseWriteLevel(levelStr)
		if err != nil {
			return err
		}

		orcDir := filepath.Join(projectRoot, ".orc")
		cloner := workflow.NewClonerFromOrcDir(orcDir)

		result, err := cloner.ClonePhase(sourceID, destID, level, force)
		if err != nil {
			return fmt.Errorf("clone phase: %w", err)
		}

		fmt.Printf("Cloned phase '%s' to '%s'\n", sourceID, destID)
		fmt.Printf("File: %s\n", result.DestPath)
		fmt.Printf("Source: %s\n", workflow.SourceDisplayName(result.SourceLoc))
		fmt.Printf("Level: %s\n", result.DestLevel)

		return nil
	},
}
