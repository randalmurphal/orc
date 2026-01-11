// Package wizard provides interactive project initialization.
package wizard

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/detect"
	"github.com/randalmurphal/orc/internal/project"
)

// Options configures the initialization wizard.
type Options struct {
	// Quick mode skips interactive prompts
	Quick bool

	// Force overwrites existing configuration
	Force bool

	// Advanced shows additional configuration options
	Advanced bool

	// Profile sets the automation profile
	Profile string

	// WorkDir is the directory to initialize (defaults to cwd)
	WorkDir string
}

// Result contains the initialization results.
type Result struct {
	ConfigPath      string
	Detection       *detect.Detection
	Profile         string
	InstalledSkills []string
	ProjectID       string
}

// Run executes the initialization wizard.
func Run(opts Options) (*Result, error) {
	workDir := opts.WorkDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
	}

	// Check if already initialized
	orcDir := filepath.Join(workDir, ".orc")
	if _, err := os.Stat(orcDir); err == nil && !opts.Force {
		return nil, fmt.Errorf("orc is already initialized (use --force to reinitialize)")
	}

	// Detect project type
	detection, err := detect.Detect(workDir)
	if err != nil {
		return nil, fmt.Errorf("detect project: %w", err)
	}

	result := &Result{
		Detection: detection,
	}

	// Select profile
	if opts.Quick {
		result.Profile = "auto"
		if opts.Profile != "" {
			result.Profile = opts.Profile
		}
	} else {
		result.Profile, err = selectProfile(detection)
		if err != nil {
			return nil, err
		}
	}

	// Initialize orc
	if err := config.Init(opts.Force); err != nil {
		return nil, fmt.Errorf("initialize orc: %w", err)
	}
	result.ConfigPath = filepath.Join(orcDir, "config.yaml")

	// Update config with detected settings
	if err := applyDetectedSettings(result.ConfigPath, detection, result.Profile); err != nil {
		return nil, fmt.Errorf("apply settings: %w", err)
	}

	// Register project
	proj, err := project.RegisterProject(workDir)
	if err != nil {
		// Non-fatal warning
		fmt.Printf("⚠️  Failed to register project: %v\n", err)
	} else {
		result.ProjectID = proj.ID
	}

	// Install suggested skills (if not quick mode)
	if !opts.Quick && len(detection.SuggestedSkills) > 0 {
		installed, err := installSkills(workDir, detection.SuggestedSkills)
		if err != nil {
			// Non-fatal warning
			fmt.Printf("⚠️  Failed to install some skills: %v\n", err)
		}
		result.InstalledSkills = installed
	}

	// Update CLAUDE.md with orc section
	if err := updateClaudeMD(workDir, detection); err != nil {
		// Non-fatal warning
		fmt.Printf("⚠️  Failed to update CLAUDE.md: %v\n", err)
	}

	return result, nil
}

// selectProfile prompts the user to select an automation profile.
func selectProfile(detection *detect.Detection) (string, error) {
	profiles := []struct {
		name        string
		description string
	}{
		{"auto", "Fully automated, no human intervention (recommended for solo projects)"},
		{"fast", "Minimal gates, speed over safety"},
		{"safe", "AI reviews, human gates for risky operations"},
		{"strict", "Human approval required at major checkpoints"},
	}

	fmt.Println("\n📋 Select automation profile:")
	for i, p := range profiles {
		marker := "  "
		if i == 0 {
			marker = "→ "
		}
		fmt.Printf("%s%d) %s\n", marker, i+1, p.name)
		fmt.Printf("      %s\n", p.description)
	}

	fmt.Print("\nEnter selection [1]: ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "auto", nil // Default on error
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return "auto", nil
	}

	switch input {
	case "1":
		return "auto", nil
	case "2":
		return "fast", nil
	case "3":
		return "safe", nil
	case "4":
		return "strict", nil
	default:
		return "auto", nil
	}
}

// applyDetectedSettings updates the config with detected project settings.
func applyDetectedSettings(configPath string, detection *detect.Detection, profile string) error {
	// Read current config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	// Parse as string and update fields
	content := string(data)

	// Set profile
	content = setConfigValue(content, "profile", profile)

	// Set detected commands
	if detection.TestCommand != "" {
		content = setConfigValue(content, "test_command", detection.TestCommand)
	}
	if detection.LintCommand != "" {
		content = setConfigValue(content, "lint_command", detection.LintCommand)
	}
	if detection.BuildCommand != "" {
		content = setConfigValue(content, "build_command", detection.BuildCommand)
	}

	return os.WriteFile(configPath, []byte(content), 0644)
}

// setConfigValue updates or adds a config value.
func setConfigValue(content, key, value string) string {
	lines := strings.Split(content, "\n")
	found := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			lines[i] = key + ": " + value
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, key+": "+value)
	}

	return strings.Join(lines, "\n")
}

// installSkills installs suggested skills to .claude/skills/.
func installSkills(workDir string, skills []string) ([]string, error) {
	skillsDir := filepath.Join(workDir, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, err
	}

	var installed []string
	for _, skill := range skills {
		// Create a placeholder skill file
		skillPath := filepath.Join(skillsDir, skill, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(skillPath), 0755); err != nil {
			continue
		}

		content := fmt.Sprintf(`---
name: %s
description: Auto-generated skill placeholder
---

# %s

This skill was suggested during project initialization.
Customize this file with domain-specific instructions.
`, skill, skill)

		if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
			continue
		}
		installed = append(installed, skill)
	}

	return installed, nil
}

// updateClaudeMD adds an orc section to CLAUDE.md if not present.
func updateClaudeMD(workDir string, detection *detect.Detection) error {
	claudeMDPath := filepath.Join(workDir, "CLAUDE.md")

	// Check if file exists
	existingContent := ""
	if data, err := os.ReadFile(claudeMDPath); err == nil {
		existingContent = string(data)
		// Check if orc section already exists
		if strings.Contains(existingContent, "## Orc Orchestrator") ||
			strings.Contains(existingContent, "## orc") {
			return nil // Already has orc section
		}
	}

	// Generate orc section
	section := generateOrcSection(detection)

	// Append or create
	var newContent string
	if existingContent != "" {
		newContent = existingContent + "\n\n" + section
	} else {
		newContent = "# Project Instructions\n\n" + section
	}

	return os.WriteFile(claudeMDPath, []byte(newContent), 0644)
}

// generateOrcSection creates the orc documentation section.
func generateOrcSection(detection *detect.Detection) string {
	var sb strings.Builder

	sb.WriteString("## Orc Orchestrator\n\n")
	sb.WriteString("This project uses [orc](https://github.com/randalmurphal/orc) for task orchestration.\n\n")

	sb.WriteString("### Detected Configuration\n\n")
	sb.WriteString(fmt.Sprintf("| Setting | Value |\n"))
	sb.WriteString(fmt.Sprintf("|---------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Language | %s |\n", detection.Language))

	if len(detection.Frameworks) > 0 {
		fws := make([]string, len(detection.Frameworks))
		for i, fw := range detection.Frameworks {
			fws[i] = string(fw)
		}
		sb.WriteString(fmt.Sprintf("| Frameworks | %s |\n", strings.Join(fws, ", ")))
	}

	if detection.TestCommand != "" {
		sb.WriteString(fmt.Sprintf("| Test | `%s` |\n", detection.TestCommand))
	}
	if detection.LintCommand != "" {
		sb.WriteString(fmt.Sprintf("| Lint | `%s` |\n", detection.LintCommand))
	}
	if detection.BuildCommand != "" {
		sb.WriteString(fmt.Sprintf("| Build | `%s` |\n", detection.BuildCommand))
	}

	sb.WriteString("\n### Commands\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("orc new \"task description\"  # Create task\n")
	sb.WriteString("orc run TASK-001            # Execute task\n")
	sb.WriteString("orc status                  # Show status\n")
	sb.WriteString("```\n")

	return sb.String()
}

// PrintResult displays the initialization result.
func PrintResult(result *Result) {
	fmt.Println()
	fmt.Println("✅ orc initialized successfully")
	fmt.Println()
	fmt.Println("📁 Configuration")
	fmt.Printf("   Config:  %s\n", result.ConfigPath)
	fmt.Printf("   Profile: %s\n", result.Profile)

	if result.ProjectID != "" {
		fmt.Printf("   Project: %s\n", result.ProjectID)
	}

	fmt.Println()
	fmt.Println("🔍 Detected")
	fmt.Printf("   Type:    %s\n", detect.DescribeProject(result.Detection))

	if result.Detection.TestCommand != "" {
		fmt.Printf("   Test:    %s\n", result.Detection.TestCommand)
	}
	if result.Detection.LintCommand != "" {
		fmt.Printf("   Lint:    %s\n", result.Detection.LintCommand)
	}

	if len(result.InstalledSkills) > 0 {
		fmt.Println()
		fmt.Println("📚 Installed Skills")
		for _, skill := range result.InstalledSkills {
			fmt.Printf("   • %s\n", skill)
		}
	}

	fmt.Println()
	fmt.Println("📝 Next steps:")
	fmt.Println("   orc new \"Your first task\"  - Create a task")
	fmt.Println("   orc serve                  - Start web UI")
}
