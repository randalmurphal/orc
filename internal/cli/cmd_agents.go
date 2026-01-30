// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/db"
)

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentListCmd)
	agentsCmd.AddCommand(agentShowCmd)
	agentsCmd.AddCommand(agentNewCmd)
	agentsCmd.AddCommand(agentEditCmd)
	agentsCmd.AddCommand(agentDeleteCmd)

	// List flags
	agentsCmd.Flags().Bool("custom", false, "Show only custom agents")
	agentsCmd.Flags().Bool("builtin", false, "Show only built-in agents")

	// New flags
	agentNewCmd.Flags().String("description", "", "Agent description (required)")
	agentNewCmd.Flags().String("prompt", "", "Sub-agent role prompt (used when agent is a sub-agent)")
	agentNewCmd.Flags().String("system-prompt", "", "System prompt for executor role (used when agent runs a phase)")
	agentNewCmd.Flags().String("config", "", "Claude config JSON (for executor role)")
	agentNewCmd.Flags().String("model", "", "Model to use (opus, sonnet, haiku)")
	agentNewCmd.Flags().StringSlice("tools", nil, "Allowed tools (Read, Grep, Glob, Edit, Bash, etc.)")
	_ = agentNewCmd.MarkFlagRequired("description")

	// Edit flags
	agentEditCmd.Flags().String("description", "", "New description")
	agentEditCmd.Flags().String("prompt", "", "New sub-agent role prompt")
	agentEditCmd.Flags().String("system-prompt", "", "New system prompt for executor role")
	agentEditCmd.Flags().String("config", "", "New claude config JSON")
	agentEditCmd.Flags().String("model", "", "New model")
	agentEditCmd.Flags().StringSlice("tools", nil, "New tool list (replaces existing)")
}

var agentsCmd = &cobra.Command{
	Use:     "agents",
	Aliases: []string{"agent"},
	Short:   "Manage agents",
	Long: `List, create, and manage agent definitions.

Agents are a unified concept that can serve two roles:
1. EXECUTOR: The main agent that runs a phase (uses system-prompt, model, config)
2. SUB-AGENT: Delegated to by the executor (uses prompt for role context)

Built-in agents are provided by orc for common tasks like code review.
Custom agents can be created for project-specific needs.

Examples:
  orc agents                   # List all agents
  orc agents --custom          # List only custom agents
  orc agents --builtin         # List only built-in agents`,
	RunE: runAgentList,
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents",
	Long: `List all available agents.

Examples:
  orc agent list               # List all agents
  orc agent list --custom      # List only custom agents`,
	RunE: runAgentList,
}

func runAgentList(cmd *cobra.Command, args []string) error {
	projectRoot, err := ResolveProjectPath()
	if err != nil {
		return err
	}

	pdb, err := db.OpenProject(projectRoot)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = pdb.Close() }()

	agents, err := pdb.ListAgents()
	if err != nil {
		return fmt.Errorf("list agents: %w", err)
	}

	customOnly, _ := cmd.Flags().GetBool("custom")
	builtinOnly, _ := cmd.Flags().GetBool("builtin")

	// Filter agents
	var filtered []*db.Agent
	for _, a := range agents {
		if customOnly && a.IsBuiltin {
			continue
		}
		if builtinOnly && !a.IsBuiltin {
			continue
		}
		filtered = append(filtered, a)
	}

	if len(filtered) == 0 {
		fmt.Println("No agents found.")
		return nil
	}

	// Display as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tMODEL\tTOOLS\tBUILT-IN\tDESCRIPTION")
	for _, a := range filtered {
		builtinStr := ""
		if a.IsBuiltin {
			builtinStr = "yes"
		}
		model := a.Model
		if model == "" {
			model = "-"
		}
		tools := "-"
		if len(a.Tools) > 0 {
			if len(a.Tools) <= 3 {
				tools = strings.Join(a.Tools, ", ")
			} else {
				tools = fmt.Sprintf("%s, +%d more", strings.Join(a.Tools[:3], ", "), len(a.Tools)-3)
			}
		}
		desc := a.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			a.Name, model, tools, builtinStr, desc)
	}
	_ = w.Flush()

	return nil
}

var agentShowCmd = &cobra.Command{
	Use:   "show <agent-name>",
	Short: "Show agent details",
	Long: `Display detailed information about an agent including its prompts,
tools, and configuration.

Examples:
  orc agent show code-reviewer
  orc agent show my-custom-agent`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]

		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		agent, err := pdb.GetAgent(agentName)
		if err != nil {
			return fmt.Errorf("get agent: %w", err)
		}
		if agent == nil {
			return fmt.Errorf("agent not found: %s", agentName)
		}

		// Display agent info
		fmt.Printf("Agent: %s\n", agent.Name)
		if agent.IsBuiltin {
			fmt.Println("Type: Built-in")
		} else {
			fmt.Println("Type: Custom")
		}
		fmt.Printf("Description: %s\n", agent.Description)
		if agent.Model != "" {
			fmt.Printf("Model: %s\n", agent.Model)
		}
		if len(agent.Tools) > 0 {
			fmt.Printf("Tools: %s\n", strings.Join(agent.Tools, ", "))
		}

		// Determine role capabilities
		roles := []string{}
		if agent.SystemPrompt != "" {
			roles = append(roles, "executor")
		}
		if agent.Prompt != "" {
			roles = append(roles, "sub-agent")
		}
		if len(roles) > 0 {
			fmt.Printf("Role: Can be used as %s\n", strings.Join(roles, " or "))
		}

		// Show system prompt (executor role)
		if agent.SystemPrompt != "" {
			fmt.Println("\nSystem Prompt (executor role):")
			fmt.Println("---")
			fmt.Println(agent.SystemPrompt)
			fmt.Println("---")
		}

		// Show prompt (sub-agent role)
		if agent.Prompt != "" {
			fmt.Println("\nPrompt (sub-agent role):")
			fmt.Println("---")
			fmt.Println(agent.Prompt)
			fmt.Println("---")
		}

		// Show claude config if present
		if agent.ClaudeConfig != "" {
			fmt.Println("\nClaude Config:")
			fmt.Println(agent.ClaudeConfig)
		}

		return nil
	},
}

var agentNewCmd = &cobra.Command{
	Use:   "new <agent-name>",
	Short: "Create a new custom agent",
	Long: `Create a new custom agent with specified configuration.

An agent requires at minimum a name and description. Agents can serve two roles:

EXECUTOR ROLE (main phase runner):
  --system-prompt  System prompt used when agent executes a phase
  --config         Claude config JSON (additional settings)
  --model          Model to use (opus, sonnet, haiku)

SUB-AGENT ROLE (delegated work):
  --prompt         Role context used when another agent delegates to this one

Examples:
  # Executor agent for implementation phases
  orc agent new impl-executor --description "Implementation executor" \
    --system-prompt "You are a careful implementer..." --model opus

  # Sub-agent for security review
  orc agent new security-checker --description "Review code for security issues" \
    --prompt "You are a security expert. Review the code for vulnerabilities." --model sonnet

  # Agent that can be both executor and sub-agent
  orc agent new test-writer --description "Write and run tests" \
    --system-prompt "You write comprehensive tests" --prompt "Write tests for the given code" \
    --tools Read,Grep,Edit,Bash`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]

		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		// Check if agent already exists
		existing, err := pdb.GetAgent(agentName)
		if err != nil {
			return fmt.Errorf("check existing: %w", err)
		}
		if existing != nil {
			return fmt.Errorf("agent already exists: %s", agentName)
		}

		desc, _ := cmd.Flags().GetString("description")
		prompt, _ := cmd.Flags().GetString("prompt")
		systemPrompt, _ := cmd.Flags().GetString("system-prompt")
		claudeConfig, _ := cmd.Flags().GetString("config")
		model, _ := cmd.Flags().GetString("model")
		tools, _ := cmd.Flags().GetStringSlice("tools")

		agent := &db.Agent{
			ID:           agentName,
			Name:         agentName,
			Description:  desc,
			Prompt:       prompt,
			SystemPrompt: systemPrompt,
			ClaudeConfig: claudeConfig,
			Model:        model,
			Tools:        tools,
			IsBuiltin:    false,
		}

		if err := pdb.SaveAgent(agent); err != nil {
			return fmt.Errorf("save agent: %w", err)
		}

		fmt.Printf("Created agent '%s'\n", agentName)
		if model != "" {
			fmt.Printf("  Model: %s\n", model)
		}
		if len(tools) > 0 {
			fmt.Printf("  Tools: %s\n", strings.Join(tools, ", "))
		}
		if systemPrompt != "" {
			fmt.Println("  Role: Can be used as executor")
		}
		if prompt != "" {
			fmt.Println("  Role: Can be used as sub-agent")
		}

		return nil
	},
}

var agentEditCmd = &cobra.Command{
	Use:   "edit <agent-name>",
	Short: "Edit a custom agent",
	Long: `Edit an existing custom agent's configuration.

Built-in agents cannot be edited. Create a custom agent with the same
capabilities if you need to modify behavior.

Examples:
  orc agent edit my-agent --description "Updated description"
  orc agent edit my-agent --model sonnet
  orc agent edit my-agent --system-prompt "You are a careful coder"
  orc agent edit my-agent --tools Read,Grep,Edit  # Replace tool list`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]

		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		agent, err := pdb.GetAgent(agentName)
		if err != nil {
			return fmt.Errorf("get agent: %w", err)
		}
		if agent == nil {
			return fmt.Errorf("agent not found: %s", agentName)
		}
		if agent.IsBuiltin {
			return fmt.Errorf("cannot edit built-in agent: %s", agentName)
		}

		// Update fields if flags provided
		updated := false
		if cmd.Flags().Changed("description") {
			agent.Description, _ = cmd.Flags().GetString("description")
			updated = true
		}
		if cmd.Flags().Changed("prompt") {
			agent.Prompt, _ = cmd.Flags().GetString("prompt")
			updated = true
		}
		if cmd.Flags().Changed("system-prompt") {
			agent.SystemPrompt, _ = cmd.Flags().GetString("system-prompt")
			updated = true
		}
		if cmd.Flags().Changed("config") {
			agent.ClaudeConfig, _ = cmd.Flags().GetString("config")
			updated = true
		}
		if cmd.Flags().Changed("model") {
			agent.Model, _ = cmd.Flags().GetString("model")
			updated = true
		}
		if cmd.Flags().Changed("tools") {
			agent.Tools, _ = cmd.Flags().GetStringSlice("tools")
			updated = true
		}

		if !updated {
			return fmt.Errorf("no changes specified. Use --description, --prompt, --system-prompt, --config, --model, or --tools")
		}

		if err := pdb.SaveAgent(agent); err != nil {
			return fmt.Errorf("save agent: %w", err)
		}

		fmt.Printf("Updated agent '%s'\n", agentName)
		return nil
	},
}

var agentDeleteCmd = &cobra.Command{
	Use:   "delete <agent-name>",
	Short: "Delete a custom agent",
	Long: `Delete a custom agent.

Built-in agents cannot be deleted.

Examples:
  orc agent delete my-custom-agent`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]

		projectRoot, err := ResolveProjectPath()
		if err != nil {
			return err
		}

		pdb, err := db.OpenProject(projectRoot)
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer func() { _ = pdb.Close() }()

		agent, err := pdb.GetAgent(agentName)
		if err != nil {
			return fmt.Errorf("get agent: %w", err)
		}
		if agent == nil {
			return fmt.Errorf("agent not found: %s", agentName)
		}
		if agent.IsBuiltin {
			return fmt.Errorf("cannot delete built-in agent: %s", agentName)
		}

		if err := pdb.DeleteAgent(agentName); err != nil {
			return fmt.Errorf("delete agent: %w", err)
		}

		fmt.Printf("Deleted agent '%s'\n", agentName)
		return nil
	},
}
