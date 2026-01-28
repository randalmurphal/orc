package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/internal/detect"
	"github.com/randalmurphal/orc/internal/wizard"
)

// InitWizardState holds the collected configuration from the wizard.
type InitWizardState struct {
	// Global settings (first-time only)
	DefaultModel   string
	DefaultProfile string

	// Project detection
	Languages       []detect.LanguageInfo
	ConfirmedLangs  []string // Language values user confirmed
	HasFrontend     bool
	PrimaryLanguage string

	// Configuration
	Profile      string
	TargetBranch string

	// MCP setup
	EnablePlaywright bool
	PlaywrightConfig PlaywrightMCPConfig

	// Hooks & integration
	InstallHooks     bool
	UpdateGitignore  bool
	SetConstitution  bool
	ConstitutionPath string
}

// PlaywrightMCPConfig holds Playwright MCP settings.
type PlaywrightMCPConfig struct {
	Headless   bool
	Browser    string // chromium, firefox, webkit
	DisableGPU bool
}

// buildInitWizard creates the wizard with all init steps.
func buildInitWizard(projectPath string) (*wizard.Wizard, *InitWizardState) {
	state := &InitWizardState{
		PlaywrightConfig: PlaywrightMCPConfig{
			Headless:   true,
			Browser:    "chromium",
			DisableGPU: true,
		},
	}

	// Run detection first
	multiDetection, _ := detect.DetectMulti(projectPath)
	if multiDetection != nil {
		state.Languages = multiDetection.Languages
		state.HasFrontend = multiDetection.HasFrontend
		if primary := multiDetection.GetPrimaryLanguage(); primary != nil {
			state.PrimaryLanguage = string(primary.Language)
		}
	}

	// Check for constitution files
	constitutionPaths := []string{
		filepath.Join(projectPath, "INVARIANTS.md"),
		filepath.Join(projectPath, "docs", "INVARIANTS.md"),
	}
	for _, p := range constitutionPaths {
		if _, err := os.Stat(p); err == nil {
			state.ConstitutionPath = p
			break
		}
	}

	// Build wizard steps
	steps := []wizard.Step{}

	// Step 1: Global setup (if ~/.orc doesn't exist)
	steps = append(steps, buildGlobalSetupStep())

	// Step 2: Detection confirmation
	steps = append(steps, buildDetectionStep(state))

	// Step 3: Profile selection
	steps = append(steps, buildProfileStep())

	// Step 4: Target branch
	steps = append(steps, buildTargetBranchStep(projectPath))

	// Step 5: MCP setup (if frontend detected)
	steps = append(steps, buildMCPStep(state))

	// Step 6: Hooks installation
	steps = append(steps, buildHooksStep())

	// Step 7: Constitution (if found)
	steps = append(steps, buildConstitutionStep(state))

	// Step 8: Summary
	steps = append(steps, buildSummaryStep(state))

	w := wizard.New(steps...).WithState(wizard.State{
		"init_state": state,
	})

	return w, state
}

// Step 1: Global setup
func buildGlobalSetupStep() wizard.Step {
	return wizard.NewSelectStep("global_setup", "Default AI Model", []wizard.SelectOption{
		{Value: "opus", Label: "Opus", Description: "Most capable, higher cost"},
		{Value: "sonnet", Label: "Sonnet (Recommended)", Description: "Good balance of capability and cost"},
		{Value: "haiku", Label: "Haiku", Description: "Fastest and cheapest"},
	}).
		WithDescription("Choose the default model for task execution").
		WithSkipFunc(func(s wizard.State) bool {
			// Skip if global config already exists
			home, _ := os.UserHomeDir()
			_, err := os.Stat(filepath.Join(home, ".orc", "config.yaml"))
			return err == nil
		})
}

// Step 2: Detection confirmation
func buildDetectionStep(state *InitWizardState) wizard.Step {
	// Build options from detected languages
	var options []wizard.SelectOption
	var defaults []string

	for _, lang := range state.Languages {
		label := string(lang.Language)
		if lang.RootPath != "" {
			label += " (" + lang.RootPath + ")"
		}

		desc := ""
		if len(lang.Frameworks) > 0 {
			fws := make([]string, len(lang.Frameworks))
			for i, fw := range lang.Frameworks {
				fws[i] = string(fw)
			}
			desc = "Frameworks: " + strings.Join(fws, ", ")
		}

		value := lang.GetScope()
		options = append(options, wizard.SelectOption{
			Value:       value,
			Label:       label,
			Description: desc,
		})
		defaults = append(defaults, value)
	}

	// Add option for manual addition
	options = append(options, wizard.SelectOption{
		Value:       "_add_more",
		Label:       "Add another language...",
		Description: "Manually specify additional language",
	})

	return wizard.NewMultiSelectStep("languages", "Detected Languages", options).
		WithDescription("Confirm the detected languages for this project").
		WithDefaults(defaults).
		WithSkipFunc(func(s wizard.State) bool {
			return len(state.Languages) == 0
		})
}

// Step 3: Profile selection
func buildProfileStep() wizard.Step {
	return wizard.NewSelectStep("profile", "Automation Profile", []wizard.SelectOption{
		{Value: "auto", Label: "Auto (Recommended)", Description: "Fully automated - AI handles everything"},
		{Value: "fast", Label: "Fast", Description: "Speed over safety, minimal retries"},
		{Value: "safe", Label: "Safe", Description: "AI reviews, human approves merges"},
		{Value: "strict", Label: "Strict", Description: "Human gates on spec, review, and merge"},
	}).WithDescription("How much automation do you want?")
}

// Step 4: Target branch
func buildTargetBranchStep(projectPath string) wizard.Step {
	// Try to detect default branch from git HEAD
	detectedBranch := "main"
	gitHeadPath := filepath.Join(projectPath, ".git", "HEAD")
	if data, err := os.ReadFile(gitHeadPath); err == nil {
		content := string(data)
		if strings.Contains(content, "refs/heads/master") {
			detectedBranch = "master"
		} else if strings.Contains(content, "refs/heads/develop") {
			detectedBranch = "develop"
		}
	}

	// Build options with detected branch first
	branches := []string{"main", "master", "develop"}
	var options []wizard.SelectOption
	// Add detected branch first
	options = append(options, wizard.SelectOption{
		Value: detectedBranch,
		Label: detectedBranch + " (detected)",
	})
	// Add others
	for _, b := range branches {
		if b != detectedBranch {
			options = append(options, wizard.SelectOption{Value: b, Label: b})
		}
	}

	return wizard.NewSelectStep("target_branch", "Target Branch", options).
		WithDescription("Where should completed tasks merge to?").
		WithStateKey("target_branch")
}

// Step 5: MCP setup
func buildMCPStep(state *InitWizardState) wizard.Step {
	return wizard.NewConfirmStep("enable_playwright", "Enable Playwright MCP?").
		WithDescription("Playwright MCP enables browser automation for frontend testing").
		WithDefault(true).
		WithSkipFunc(func(s wizard.State) bool {
			return !state.HasFrontend
		})
}

// Step 6: Hooks installation
func buildHooksStep() wizard.Step {
	return wizard.NewConfirmStep("install_hooks", "Install Claude Code Hooks?").
		WithDescription("Hooks enable TDD enforcement and graceful task stopping").
		WithDefault(true)
}

// Step 7: Constitution
func buildConstitutionStep(state *InitWizardState) wizard.Step {
	return wizard.NewConfirmStep("set_constitution", "Set as Project Constitution?").
		WithDescription(fmt.Sprintf("Found: %s", state.ConstitutionPath)).
		WithDefault(true).
		WithSkipFunc(func(s wizard.State) bool {
			return state.ConstitutionPath == ""
		})
}

// Step 8: Summary
func buildSummaryStep(state *InitWizardState) wizard.Step {
	return wizard.NewDisplayStep("summary", "Configuration Summary", func(s wizard.State) string {
		var b strings.Builder

		b.WriteString("The following will be configured:\n\n")

		// Profile
		if profile, ok := s["profile"].(string); ok {
			b.WriteString(fmt.Sprintf("  Profile: %s\n", profile))
		}

		// Target branch
		if branch, ok := s["target_branch"].(string); ok {
			b.WriteString(fmt.Sprintf("  Target Branch: %s\n", branch))
		}

		// Languages
		if langs, ok := s["languages"].([]string); ok && len(langs) > 0 {
			b.WriteString(fmt.Sprintf("  Languages: %s\n", strings.Join(langs, ", ")))
		}

		// MCP
		if enable, ok := s["enable_playwright"].(bool); ok && enable {
			b.WriteString("  Playwright MCP: enabled\n")
		}

		// Hooks
		if install, ok := s["install_hooks"].(bool); ok && install {
			b.WriteString("  Claude Code Hooks: will be installed\n")
		}

		// Constitution
		if set, ok := s["set_constitution"].(bool); ok && set {
			b.WriteString(fmt.Sprintf("  Constitution: %s\n", state.ConstitutionPath))
		}

		b.WriteString("\nPress enter to proceed with initialization.")

		return b.String()
	})
}

// extractWizardResults extracts the wizard state into the InitWizardState struct.
func extractWizardResults(wizardState wizard.State, state *InitWizardState) {
	if v, ok := wizardState["global_setup"].(string); ok {
		state.DefaultModel = v
	}
	if v, ok := wizardState["profile"].(string); ok {
		state.Profile = v
	}
	if v, ok := wizardState["target_branch"].(string); ok {
		state.TargetBranch = v
	}
	if v, ok := wizardState["languages"].([]string); ok {
		state.ConfirmedLangs = v
	}
	if v, ok := wizardState["enable_playwright"].(bool); ok {
		state.EnablePlaywright = v
	}
	if v, ok := wizardState["install_hooks"].(bool); ok {
		state.InstallHooks = v
	}
	if v, ok := wizardState["set_constitution"].(bool); ok {
		state.SetConstitution = v
	}

	// Always update gitignore
	state.UpdateGitignore = true
}
