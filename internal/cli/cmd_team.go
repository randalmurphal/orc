// Package cli implements the orc command-line interface.
package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/config"
)

// TeamMember represents a team member in team.yaml.
type TeamMember struct {
	Initials string `yaml:"initials"`
	Name     string `yaml:"name"`
	Email    string `yaml:"email,omitempty"`
}

// TeamRegistry represents the team.yaml file structure.
type TeamRegistry struct {
	Version          int          `yaml:"version"`
	Members          []TeamMember `yaml:"members"`
	ReservedPrefixes []string     `yaml:"reserved_prefixes"`
}

// SharedConfig represents the .orc/shared/config.yaml file structure.
type SharedConfig struct {
	Version  int `yaml:"version"`
	TaskID   struct {
		Mode         string `yaml:"mode"`
		PrefixSource string `yaml:"prefix_source"`
	} `yaml:"task_id"`
	Defaults struct {
		Profile string `yaml:"profile"`
	} `yaml:"defaults"`
}

func newTeamCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage P2P team coordination",
		Long: `Manage peer-to-peer team coordination for multi-developer workflows.

Commands:
  init     Initialize .orc/shared/ directory structure for P2P mode
  join     Register yourself in the team registry
  members  List all team members
  sync     Pull latest shared resources from git`,
	}

	cmd.AddCommand(newTeamInitCmd())
	cmd.AddCommand(newTeamJoinCmd())
	cmd.AddCommand(newTeamMembersCmd())
	cmd.AddCommand(newTeamSyncCmd())

	return cmd
}

func newTeamInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize .orc/shared/ directory structure",
		Long: `Initialize the shared directory structure for P2P team coordination.

This creates:
  .orc/shared/
  ├── config.yaml    Team defaults
  ├── prompts/       Shared prompt templates
  ├── skills/        Shared Claude skills
  ├── templates/     Shared task templates
  └── team.yaml      Team member registry

After running this command, commit the .orc/shared/ directory to git
so team members can access it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamInit(force)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing shared directory")

	return cmd
}

func runTeamInit(force bool) error {
	sharedDir := filepath.Join(config.OrcDir, "shared")

	// Check if already exists
	if _, err := os.Stat(sharedDir); err == nil && !force {
		return fmt.Errorf(".orc/shared/ already exists (use --force to reinitialize)")
	}

	// Create directory structure
	dirs := []string{
		sharedDir,
		filepath.Join(sharedDir, "prompts"),
		filepath.Join(sharedDir, "skills"),
		filepath.Join(sharedDir, "templates"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Create shared config
	sharedCfg := SharedConfig{Version: 1}
	sharedCfg.TaskID.Mode = "p2p"
	sharedCfg.TaskID.PrefixSource = "initials"
	sharedCfg.Defaults.Profile = "safe"

	cfgPath := filepath.Join(sharedDir, "config.yaml")
	cfgData, err := yaml.Marshal(sharedCfg)
	if err != nil {
		return fmt.Errorf("marshal shared config: %w", err)
	}
	if err := os.WriteFile(cfgPath, cfgData, 0644); err != nil {
		return fmt.Errorf("write shared config: %w", err)
	}

	// Create team registry
	registry := TeamRegistry{
		Version:          1,
		Members:          []TeamMember{},
		ReservedPrefixes: []string{},
	}

	teamPath := filepath.Join(sharedDir, "team.yaml")
	teamData, err := yaml.Marshal(registry)
	if err != nil {
		return fmt.Errorf("marshal team registry: %w", err)
	}
	if err := os.WriteFile(teamPath, teamData, 0644); err != nil {
		return fmt.Errorf("write team registry: %w", err)
	}

	fmt.Println("Initialized P2P team structure:")
	fmt.Println("  .orc/shared/config.yaml")
	fmt.Println("  .orc/shared/prompts/")
	fmt.Println("  .orc/shared/skills/")
	fmt.Println("  .orc/shared/templates/")
	fmt.Println("  .orc/shared/team.yaml")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Run 'orc team join' to register yourself")
	fmt.Println("  2. Commit .orc/shared/ to git")
	fmt.Println("  3. Have team members pull and run 'orc team join'")

	return nil
}

func newTeamJoinCmd() *cobra.Command {
	var initials string
	var name string
	var email string

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Register yourself in the team registry",
		Long: `Register yourself as a team member in the team registry.

This adds your entry to .orc/shared/team.yaml and reserves your prefix
for task ID generation.

You can provide your details via flags or be prompted interactively.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamJoin(initials, name, email)
		},
	}

	cmd.Flags().StringVarP(&initials, "initials", "i", "", "your initials for task ID prefix (e.g., AM)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "your display name")
	cmd.Flags().StringVarP(&email, "email", "e", "", "your email (optional)")

	return cmd
}

func runTeamJoin(initials, name, email string) error {
	sharedDir := filepath.Join(config.OrcDir, "shared")
	teamPath := filepath.Join(sharedDir, "team.yaml")

	// Check if shared directory exists
	if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
		return fmt.Errorf(".orc/shared/ does not exist. Run 'orc team init' first")
	}

	// Load existing registry
	registry, err := loadTeamRegistry(teamPath)
	if err != nil {
		return fmt.Errorf("load team registry: %w", err)
	}

	// Interactive prompts if flags not provided
	reader := bufio.NewReader(os.Stdin)

	if initials == "" {
		fmt.Print("Enter your initials (e.g., AM): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read initials: %w", err)
		}
		initials = strings.TrimSpace(input)
	}

	if initials == "" {
		return fmt.Errorf("initials are required")
	}

	// Normalize initials to uppercase
	initials = strings.ToUpper(initials)

	// Validate initials format (2-4 alphanumeric characters)
	if len(initials) < 2 || len(initials) > 4 {
		return fmt.Errorf("initials must be 2-4 characters")
	}
	for _, c := range initials {
		if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return fmt.Errorf("initials must be alphanumeric")
		}
	}

	// Check if initials already taken
	for _, m := range registry.Members {
		if strings.EqualFold(m.Initials, initials) {
			return fmt.Errorf("initials '%s' already registered to %s", initials, m.Name)
		}
	}

	// Check if prefix already reserved
	for _, p := range registry.ReservedPrefixes {
		if strings.EqualFold(p, initials) {
			return fmt.Errorf("prefix '%s' is already reserved", initials)
		}
	}

	if name == "" {
		fmt.Print("Enter your display name: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read name: %w", err)
		}
		name = strings.TrimSpace(input)
	}

	if name == "" {
		return fmt.Errorf("display name is required")
	}

	if email == "" {
		fmt.Print("Enter your email (optional, press Enter to skip): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read email: %w", err)
		}
		email = strings.TrimSpace(input)
	}

	// Add member and reserve prefix
	member := TeamMember{
		Initials: initials,
		Name:     name,
		Email:    email,
	}
	registry.Members = append(registry.Members, member)
	registry.ReservedPrefixes = append(registry.ReservedPrefixes, initials)

	// Save registry
	if err := saveTeamRegistry(teamPath, registry); err != nil {
		return fmt.Errorf("save team registry: %w", err)
	}

	// Also save to user config for task ID generation
	if err := saveUserIdentity(initials, name); err != nil {
		fmt.Printf("Warning: could not save to user config: %v\n", err)
	}

	fmt.Printf("\nRegistered as team member:\n")
	fmt.Printf("  Initials: %s\n", initials)
	fmt.Printf("  Name:     %s\n", name)
	if email != "" {
		fmt.Printf("  Email:    %s\n", email)
	}
	fmt.Printf("\nYour task IDs will be: TASK-%s-001, TASK-%s-002, ...\n", initials, initials)
	fmt.Println("\nRemember to commit the updated team.yaml to share with your team.")

	return nil
}

func newTeamMembersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "members",
		Short: "List all team members",
		Long:  `Display all registered team members from the team registry.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamMembers()
		},
	}
}

func runTeamMembers() error {
	sharedDir := filepath.Join(config.OrcDir, "shared")
	teamPath := filepath.Join(sharedDir, "team.yaml")

	// Check if shared directory exists
	if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
		return fmt.Errorf(".orc/shared/ does not exist. Run 'orc team init' first")
	}

	registry, err := loadTeamRegistry(teamPath)
	if err != nil {
		return fmt.Errorf("load team registry: %w", err)
	}

	if len(registry.Members) == 0 {
		fmt.Println("No team members registered. Run 'orc team join' to register.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "INITIALS\tNAME\tEMAIL")
	fmt.Fprintln(w, "--------\t----\t-----")

	for _, m := range registry.Members {
		email := m.Email
		if email == "" {
			email = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", m.Initials, m.Name, email)
	}
	w.Flush()

	if len(registry.ReservedPrefixes) > len(registry.Members) {
		fmt.Printf("\nReserved prefixes: %s\n", strings.Join(registry.ReservedPrefixes, ", "))
	}

	return nil
}

func newTeamSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Pull latest shared resources from git",
		Long: `Pull the latest shared resources from the remote git repository.

This runs 'git pull' to fetch the latest changes to:
  - .orc/shared/config.yaml
  - .orc/shared/prompts/
  - .orc/shared/skills/
  - .orc/shared/templates/
  - .orc/shared/team.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTeamSync()
		},
	}
}

func runTeamSync() error {
	sharedDir := filepath.Join(config.OrcDir, "shared")

	// Check if shared directory exists
	if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
		return fmt.Errorf(".orc/shared/ does not exist. Run 'orc team init' first or pull from remote")
	}

	fmt.Println("Pulling latest shared resources...")

	// Run git pull
	gitCmd := exec.Command("git", "pull", "--rebase")
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr

	if err := gitCmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	fmt.Println("\nShared resources synced successfully.")

	return nil
}

// loadTeamRegistry loads the team registry from disk.
func loadTeamRegistry(path string) (*TeamRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TeamRegistry{
				Version:          1,
				Members:          []TeamMember{},
				ReservedPrefixes: []string{},
			}, nil
		}
		return nil, err
	}

	var registry TeamRegistry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return nil, err
	}

	if registry.Members == nil {
		registry.Members = []TeamMember{}
	}
	if registry.ReservedPrefixes == nil {
		registry.ReservedPrefixes = []string{}
	}

	return &registry, nil
}

// saveTeamRegistry saves the team registry to disk.
func saveTeamRegistry(path string, registry *TeamRegistry) error {
	data, err := yaml.Marshal(registry)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// saveUserIdentity saves the user's identity to ~/.orc/config.yaml.
func saveUserIdentity(initials, displayName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	userOrcDir := filepath.Join(home, ".orc")
	if err := os.MkdirAll(userOrcDir, 0755); err != nil {
		return err
	}

	userCfgPath := filepath.Join(userOrcDir, "config.yaml")

	// Load existing config or create new
	var userCfg map[string]interface{}

	data, err := os.ReadFile(userCfgPath)
	if err == nil {
		if err := yaml.Unmarshal(data, &userCfg); err != nil {
			userCfg = make(map[string]interface{})
		}
	} else {
		userCfg = make(map[string]interface{})
	}

	// Set identity
	identity := map[string]string{
		"initials":     initials,
		"display_name": displayName,
	}
	userCfg["identity"] = identity

	// Save
	newData, err := yaml.Marshal(userCfg)
	if err != nil {
		return err
	}

	return os.WriteFile(userCfgPath, newData, 0644)
}
