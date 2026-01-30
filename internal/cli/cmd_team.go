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
	"golang.org/x/term"
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
	Version int `yaml:"version"`
	TaskID  struct {
		Mode         string `yaml:"mode"`
		PrefixSource string `yaml:"prefix_source"`
	} `yaml:"task_id"`
	Defaults struct {
		Profile string `yaml:"profile"`
	} `yaml:"defaults"`
	Gates struct {
		DefaultType    string            `yaml:"default_type,omitempty"`
		PhaseOverrides map[string]string `yaml:"phase_overrides,omitempty"`
	} `yaml:"gates,omitempty"`
	Cost struct {
		WarnPerTask float64 `yaml:"warn_per_task,omitempty"`
	} `yaml:"cost,omitempty"`
}

// findProjectRoot returns the project root, using ResolveProjectPath()
// which has worktree awareness and proper validation.
func findProjectRoot() (string, error) {
	return ResolveProjectPath()
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
	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}

	sharedDir := filepath.Join(projectRoot, config.OrcDir, "shared")

	// Check if already exists
	if _, err := os.Stat(sharedDir); err == nil && !force {
		return fmt.Errorf("%s already exists (use --force to reinitialize)", sharedDir)
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
	sharedCfg.Gates.DefaultType = "auto"

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

	_, _ = fmt.Println("Initialized P2P team structure:")
	_, _ = fmt.Printf("  %s/config.yaml\n", sharedDir)
	_, _ = fmt.Printf("  %s/prompts/\n", sharedDir)
	_, _ = fmt.Printf("  %s/skills/\n", sharedDir)
	_, _ = fmt.Printf("  %s/templates/\n", sharedDir)
	_, _ = fmt.Printf("  %s/team.yaml\n", sharedDir)
	_, _ = fmt.Println()
	_, _ = fmt.Println("Next steps:")
	_, _ = fmt.Println("  1. Run 'orc team join' to register yourself")
	_, _ = fmt.Println("  2. Commit .orc/shared/ to git")
	_, _ = fmt.Println("  3. Have team members pull and run 'orc team join'")

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
	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}

	sharedDir := filepath.Join(projectRoot, config.OrcDir, "shared")
	teamPath := filepath.Join(sharedDir, "team.yaml")

	// Check if shared directory exists
	if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist; run 'orc team init' first", sharedDir)
	}

	// Load existing registry
	registry, err := loadTeamRegistry(teamPath)
	if err != nil {
		return fmt.Errorf("load team registry: %w", err)
	}

	// Check TTY for interactive prompts
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))

	// Interactive prompts if flags not provided
	reader := bufio.NewReader(os.Stdin)

	if initials == "" {
		if !isTTY {
			return fmt.Errorf("--initials flag required in non-interactive mode")
		}
		_, _ = fmt.Print("Enter your initials (e.g., AM): ")
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
		isLetter := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		if !isLetter && !isDigit {
			return fmt.Errorf("initials must be alphanumeric")
		}
	}

	// Check if initials already taken (use uppercase for comparison)
	for _, m := range registry.Members {
		if strings.ToUpper(m.Initials) == initials {
			return fmt.Errorf("initials '%s' already registered to %s", initials, m.Name)
		}
	}

	// Check if prefix already reserved (use uppercase for comparison)
	for _, p := range registry.ReservedPrefixes {
		if strings.ToUpper(p) == initials {
			return fmt.Errorf("prefix '%s' is already reserved", initials)
		}
	}

	if name == "" {
		if !isTTY {
			return fmt.Errorf("--name flag required in non-interactive mode")
		}
		_, _ = fmt.Print("Enter your display name: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read name: %w", err)
		}
		name = strings.TrimSpace(input)
	}

	if name == "" {
		return fmt.Errorf("display name is required")
	}

	if email == "" && isTTY {
		_, _ = fmt.Print("Enter your email (optional, press Enter to skip): ")
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

	// Save to user config for task ID generation (hard error if this fails)
	if err := saveUserIdentity(initials, name); err != nil {
		return fmt.Errorf("save user identity to ~/.orc/config.yaml: %w", err)
	}

	_, _ = fmt.Printf("\nRegistered as team member:\n")
	_, _ = fmt.Printf("  Initials: %s\n", initials)
	_, _ = fmt.Printf("  Name:     %s\n", name)
	if email != "" {
		_, _ = fmt.Printf("  Email:    %s\n", email)
	}
	_, _ = fmt.Printf("\nYour task IDs will be: TASK-%s-001, TASK-%s-002, ...\n", initials, initials)
	_, _ = fmt.Printf("\nRemember to commit the updated %s to share with your team.\n", teamPath)

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
	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}

	sharedDir := filepath.Join(projectRoot, config.OrcDir, "shared")
	teamPath := filepath.Join(sharedDir, "team.yaml")

	// Check if shared directory exists
	if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist; run 'orc team init' first", sharedDir)
	}

	registry, err := loadTeamRegistry(teamPath)
	if err != nil {
		return fmt.Errorf("load team registry: %w", err)
	}

	if len(registry.Members) == 0 {
		_, _ = fmt.Println("No team members registered. Run 'orc team join' to register.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "INITIALS\tNAME\tEMAIL")
	_, _ = fmt.Fprintln(w, "--------\t----\t-----")

	for _, m := range registry.Members {
		email := m.Email
		if email == "" {
			email = "-"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", m.Initials, m.Name, email)
	}
	_ = w.Flush()

	// Always show reserved prefixes section
	_, _ = fmt.Printf("\nReserved prefixes: %s\n", strings.Join(registry.ReservedPrefixes, ", "))

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
	projectRoot, err := findProjectRoot()
	if err != nil {
		return err
	}

	sharedDir := filepath.Join(projectRoot, config.OrcDir, "shared")

	// Check if shared directory exists
	if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist; run 'orc team init' first or pull from remote", sharedDir)
	}

	// Check for uncommitted changes in shared directory
	statusCmd := exec.Command("git", "-C", projectRoot, "status", "--porcelain", filepath.Join(config.OrcDir, "shared"))
	output, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("check git status: %w", err)
	}
	if len(output) > 0 {
		return fmt.Errorf("uncommitted changes in %s; commit or stash first", sharedDir)
	}

	_, _ = fmt.Println("Pulling latest shared resources...")

	// Run git pull from project root
	gitCmd := exec.Command("git", "-C", projectRoot, "pull", "--rebase")
	gitCmd.Stdout = os.Stdout
	gitCmd.Stderr = os.Stderr

	if err := gitCmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	_, _ = fmt.Println("\nShared resources synced successfully.")

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
// Preserves existing config values, only updates the identity section.
func saveUserIdentity(initials, displayName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	userOrcDir := filepath.Join(home, ".orc")
	if err := os.MkdirAll(userOrcDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", userOrcDir, err)
	}

	userCfgPath := filepath.Join(userOrcDir, "config.yaml")

	// Load existing config or create new
	var userCfg map[string]interface{}

	data, err := os.ReadFile(userCfgPath)
	if err == nil {
		if err := yaml.Unmarshal(data, &userCfg); err != nil {
			return fmt.Errorf("parse existing %s: %w", userCfgPath, err)
		}
	} else if os.IsNotExist(err) {
		userCfg = make(map[string]interface{})
	} else {
		return fmt.Errorf("read %s: %w", userCfgPath, err)
	}

	if userCfg == nil {
		userCfg = make(map[string]interface{})
	}

	// Set identity (preserves other config values)
	identity := map[string]string{
		"initials":     initials,
		"display_name": displayName,
	}
	userCfg["identity"] = identity

	// Save
	newData, err := yaml.Marshal(userCfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(userCfgPath, newData, 0644); err != nil {
		return fmt.Errorf("write %s: %w", userCfgPath, err)
	}

	return nil
}
