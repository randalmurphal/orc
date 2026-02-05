package cli

import (
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/wizard"
)

// HostingStepResult contains the result of the hosting step execution.
type HostingStepResult struct {
	Error error
}

// TokenCheckResult contains the result of checking if a token exists.
type TokenCheckResult struct {
	TokenExists bool
	TokenEnvVar string
	Warning     string
}

// HostingStep is a specialized wizard step for hosting detection.
type HostingStep struct {
	state *InitWizardState
}

// buildHostingStep creates a hosting detection wizard step.
func buildHostingStep(state *InitWizardState) *HostingStep {
	return &HostingStep{state: state}
}

// ID returns the step identifier.
func (s *HostingStep) ID() string { return "hosting" }

// Title returns the step title.
func (s *HostingStep) Title() string { return "Hosting Provider Detection" }

// Description returns the step description.
func (s *HostingStep) Description() string { return "Detecting git hosting provider from remote URL" }

// Skip returns whether this step should be skipped.
func (s *HostingStep) Skip(state wizard.State) bool {
	return false
}

// Init creates the initial model for this step.
func (s *HostingStep) Init(state wizard.State) tea.Model {
	return &hostingModel{hostingStep: s, wizardState: state}
}

// Result extracts the result from the model and stores it in state.
func (s *HostingStep) Result(model tea.Model, state wizard.State) {
	state["hosting_provider"] = string(s.state.DetectedProvider)
	state["hosting_base_url"] = s.state.DetectedBaseURL
	state["hosting_confirmed"] = true
}

// Execute runs the hosting detection step directly (for testing).
func (s *HostingStep) Execute(ws wizard.State) HostingStepResult {
	projectPath, ok := ws["project_path"].(string)
	if !ok || projectPath == "" {
		return HostingStepResult{} // No error, just skip
	}

	// Get remote URL from git
	remoteURL, err := getGitRemoteURL(projectPath)
	if err != nil || remoteURL == "" {
		// No remote configured - mark as skipped
		s.state.HostingSkipped = true
		return HostingStepResult{}
	}

	// Detect provider from remote URL
	provider := hosting.DetectProvider(remoteURL)
	s.state.DetectedProvider = provider

	if provider == hosting.ProviderUnknown {
		// Unknown provider - require manual selection
		s.state.RequiresManualSelection = true
		return HostingStepResult{}
	}

	// Check if self-hosted
	baseURL, isSelfHosted := hosting.ExtractBaseURL(remoteURL, provider)
	s.state.IsSelfHosted = isSelfHosted
	s.state.DetectedBaseURL = baseURL

	return HostingStepResult{}
}

// hostingModel is the bubbletea model for the hosting step.
type hostingModel struct {
	hostingStep *HostingStep
	wizardState wizard.State
	detected    bool
}

func (m *hostingModel) Init() tea.Cmd {
	// Run detection on init
	result := m.hostingStep.Execute(m.wizardState)
	m.detected = true
	if result.Error != nil {
		return nil
	}
	return nil
}

func (m *hostingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", " ":
			return m, wizard.CompleteStep()
		}
	}
	return m, nil
}

func (m *hostingModel) View() string {
	state := m.hostingStep.state
	var b strings.Builder

	if state.HostingSkipped {
		b.WriteString("No git remote configured - hosting step skipped\n")
	} else if state.RequiresManualSelection {
		b.WriteString("Unknown hosting provider - manual configuration required\n")
	} else {
		b.WriteString("Detected: " + string(state.DetectedProvider) + "\n")
		if state.IsSelfHosted {
			b.WriteString("Self-hosted at: " + state.DetectedBaseURL + "\n")
		}
	}

	b.WriteString("\nPress enter to continue")
	return b.String()
}

// checkTokenExists checks if the hosting token environment variable is set.
func checkTokenExists(state *InitWizardState) *TokenCheckResult {
	result := &TokenCheckResult{}

	// Get expected token env var based on provider
	tokenEnvVar := hosting.GetTokenEnvVar(state.DetectedProvider, hosting.Config{})
	result.TokenEnvVar = tokenEnvVar

	if tokenEnvVar == "" {
		return result
	}

	// Check if token is set
	token := os.Getenv(tokenEnvVar)
	result.TokenExists = token != ""

	if !result.TokenExists {
		result.Warning = tokenEnvVar + " not set - required for PR creation"
	}

	return result
}

// getGitRemoteURL gets the origin remote URL for the repo at the given path.
func getGitRemoteURL(path string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
