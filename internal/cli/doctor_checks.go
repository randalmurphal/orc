package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/hosting"
)

type doctorCheck struct {
	Name   string
	OK     bool
	Detail string
}

func runWorkflowDoctorChecks(cfg *config.Config, gdb *db.GlobalDB, workflowID string) ([]doctorCheck, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if gdb == nil {
		return nil, fmt.Errorf("global database is required")
	}
	if strings.TrimSpace(workflowID) == "" {
		return nil, fmt.Errorf("workflow ID is required")
	}

	var checks []doctorCheck

	requiredProviders, err := requiredWorkflowProviders(cfg, gdb, workflowID)
	if err != nil {
		return nil, err
	}

	for provider, binary := range requiredProviders {
		_, lookErr := exec.LookPath(binary)
		checks = append(checks, doctorCheck{
			Name:   fmt.Sprintf("%s provider", provider),
			OK:     lookErr == nil,
			Detail: providerBinaryMessage(provider, binary, lookErr),
		})
	}

	commandChecks := []struct {
		name string
		cmd  string
	}{
		{name: "unit test command", cmd: cfg.Testing.Commands.Unit},
		{name: "integration test command", cmd: cfg.Testing.Commands.Integration},
		{name: "e2e test command", cmd: cfg.Testing.Commands.E2E},
	}
	for _, item := range commandChecks {
		checks = append(checks, commandPresenceCheck(item.name, item.cmd))
	}

	if workflowUsesPhase(gdb, workflowID, "qa_e2e_test") {
		checks = append(checks, commandPresenceCheck("playwright runtime", "npx"))
	}

	completionCheck, err := completionAuthCheck(cfg, gdb, workflowID)
	if err != nil {
		return nil, err
	}
	if completionCheck != nil {
		checks = append(checks, *completionCheck)
	}

	return checks, nil
}

func requiredWorkflowProviders(cfg *config.Config, gdb *db.GlobalDB, workflowID string) (map[string]string, error) {
	phases, err := gdb.GetWorkflowPhases(workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow phases for %s: %w", workflowID, err)
	}

	required := make(map[string]string)
	for _, phase := range phases {
		tmpl, err := gdb.GetPhaseTemplate(phase.PhaseTemplateID)
		if err != nil {
			return nil, fmt.Errorf("load phase template %s: %w", phase.PhaseTemplateID, err)
		}
		provider := resolvePhaseProviderName(cfg, phase.ProviderOverride, tmpl.Provider)
		switch provider {
		case "claude":
			required["claude"] = resolveClaudeBinary(cfg)
		case "codex":
			required["codex"] = resolveCodexBinary(cfg)
		}
	}

	return required, nil
}

func workflowUsesPhase(gdb *db.GlobalDB, workflowID string, phaseID string) bool {
	phases, err := gdb.GetWorkflowPhases(workflowID)
	if err != nil {
		return false
	}
	for _, phase := range phases {
		if phase.PhaseTemplateID == phaseID {
			return true
		}
	}
	return false
}

func resolvePhaseProviderName(cfg *config.Config, override string, templateProvider string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	if strings.TrimSpace(templateProvider) != "" {
		return templateProvider
	}
	if strings.TrimSpace(cfg.Provider) != "" {
		return cfg.Provider
	}
	return "claude"
}

func resolveClaudeBinary(cfg *config.Config) string {
	if cfg != nil && strings.TrimSpace(cfg.ClaudePath) != "" {
		return cfg.ClaudePath
	}
	return "claude"
}

func resolveCodexBinary(cfg *config.Config) string {
	if cfg != nil {
		if strings.TrimSpace(cfg.CodexPath) != "" {
			return cfg.CodexPath
		}
		if strings.TrimSpace(cfg.Providers.Codex.Path) != "" {
			return cfg.Providers.Codex.Path
		}
	}
	return "codex"
}

func providerBinaryMessage(provider string, binary string, err error) string {
	if err != nil {
		return fmt.Sprintf("%s binary %q not found in PATH", provider, binary)
	}
	return fmt.Sprintf("%s binary %q available", provider, binary)
}

func commandPresenceCheck(name string, command string) doctorCheck {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return doctorCheck{
			Name:   name,
			OK:     false,
			Detail: "command is not configured",
		}
	}

	binary := fields[0]
	if _, err := exec.LookPath(binary); err != nil {
		return doctorCheck{
			Name:   name,
			OK:     false,
			Detail: fmt.Sprintf("binary %q not found for command %q", binary, command),
		}
	}

	return doctorCheck{
		Name:   name,
		OK:     true,
		Detail: fmt.Sprintf("command %q is runnable", command),
	}
}

func failWorkflowDoctorChecks(checks []doctorCheck, failClosed bool) error {
	if !failClosed {
		return nil
	}
	for _, check := range checks {
		if !check.OK {
			return fmt.Errorf("preflight failed: %s - %s", check.Name, check.Detail)
		}
	}
	return nil
}

func completionAuthCheck(cfg *config.Config, gdb *db.GlobalDB, workflowID string) (*doctorCheck, error) {
	action, err := resolveWorkflowCompletionAction(cfg, gdb, workflowID)
	if err != nil {
		return nil, err
	}
	if action == "" || action == "none" || action == "commit" {
		return nil, nil
	}

	workDir, err := ResolveProjectPath()
	if err != nil {
		return &doctorCheck{
			Name:   "completion hosting auth",
			OK:     false,
			Detail: fmt.Sprintf("resolve project path: %v", err),
		}, nil
	}

	provider, err := resolveHostingProviderForDoctor(workDir, cfg)
	if err != nil {
		return &doctorCheck{
			Name:   "completion hosting auth",
			OK:     false,
			Detail: err.Error(),
		}, nil
	}

	tokenEnvVar := resolveHostingTokenEnvVar(cfg, provider)
	if strings.TrimSpace(os.Getenv(tokenEnvVar)) == "" {
		return &doctorCheck{
			Name:   "completion hosting auth",
			OK:     false,
			Detail: fmt.Sprintf("%s is not set for %s completion action %q", tokenEnvVar, provider, action),
		}, nil
	}

	return &doctorCheck{
		Name:   "completion hosting auth",
		OK:     true,
		Detail: fmt.Sprintf("%s is set for %s completion action %q", tokenEnvVar, provider, action),
	}, nil
}

func resolveWorkflowCompletionAction(cfg *config.Config, gdb *db.GlobalDB, workflowID string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}

	action := cfg.ResolveCompletionAction(workflowID)
	workflowDef, err := gdb.GetWorkflow(workflowID)
	if err != nil {
		return "", fmt.Errorf("load workflow %s: %w", workflowID, err)
	}
	if strings.TrimSpace(workflowDef.CompletionAction) != "" {
		action = workflowDef.CompletionAction
	}
	return action, nil
}

func resolveHostingProviderForDoctor(workDir string, cfg *config.Config) (string, error) {
	if cfg != nil {
		explicit := strings.TrimSpace(cfg.Hosting.Provider)
		if explicit != "" && explicit != "auto" {
			return explicit, nil
		}
	}

	remoteURL, err := getOriginRemoteURL(workDir)
	if err != nil {
		return "", err
	}

	provider := hosting.DetectProvider(remoteURL)
	if provider == hosting.ProviderUnknown {
		return "", fmt.Errorf("cannot detect hosting provider from remote URL %q", remoteURL)
	}
	return string(provider), nil
}

func resolveHostingTokenEnvVar(cfg *config.Config, provider string) string {
	if cfg != nil && strings.TrimSpace(cfg.Hosting.TokenEnvVar) != "" {
		return cfg.Hosting.TokenEnvVar
	}
	if provider == string(hosting.ProviderGitLab) {
		return "GITLAB_TOKEN"
	}
	return "GITHUB_TOKEN"
}

func getOriginRemoteURL(workDir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get origin remote URL: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
