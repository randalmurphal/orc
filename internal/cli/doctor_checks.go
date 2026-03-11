package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/workflow"
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

	resolvedWorkflow, resolver, err := loadWorkflowForDoctor(workflowID)
	if err != nil {
		return nil, err
	}

	var checks []doctorCheck

	requiredProviders, err := requiredWorkflowProviders(cfg, resolvedWorkflow, resolver)
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

	if workflowUsesPhase(resolvedWorkflow, "qa_e2e_test") {
		checks = append(checks, commandPresenceCheck("playwright runtime", "npx"))
	}

	completionChecks, err := completionHostingChecks(cfg, workflowID)
	if err != nil {
		return nil, err
	}
	checks = append(checks, completionChecks...)

	return checks, nil
}

func loadWorkflowForDoctor(workflowID string) (*workflow.Workflow, *workflow.Resolver, error) {
	projectRoot, err := ResolveProjectPath()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve project path: %w", err)
	}

	resolver := workflow.NewResolverFromOrcDir(filepath.Join(projectRoot, ".orc"))
	resolved, err := resolver.ResolveWorkflow(workflowID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve workflow %s: %w", workflowID, err)
	}
	return resolved.Workflow, resolver, nil
}

func requiredWorkflowProviders(
	cfg *config.Config,
	wf *workflow.Workflow,
	resolver *workflow.Resolver,
) (map[string]string, error) {
	if wf == nil {
		return nil, fmt.Errorf("workflow is required")
	}
	if resolver == nil {
		return nil, fmt.Errorf("workflow resolver is required")
	}

	required := make(map[string]string)
	for _, phase := range wf.Phases {
		tmpl, err := resolver.ResolvePhase(phase.PhaseTemplateID)
		if err != nil {
			return nil, fmt.Errorf("resolve phase template %s: %w", phase.PhaseTemplateID, err)
		}
		provider := resolvePhaseProviderName(cfg, phase.ProviderOverride, tmpl.Phase.Provider)
		switch provider {
		case "claude":
			required["claude"] = resolveClaudeBinary(cfg)
		case "codex":
			required["codex"] = resolveCodexBinary(cfg)
		}
	}

	return required, nil
}

func workflowUsesPhase(wf *workflow.Workflow, phaseID string) bool {
	if wf == nil {
		return false
	}
	for _, phase := range wf.Phases {
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

func completionHostingChecks(cfg *config.Config, workflowID string) ([]doctorCheck, error) {
	action, err := resolveWorkflowCompletionAction(cfg, workflowID)
	if err != nil {
		return nil, err
	}
	if action == "" || action == "none" || action == "commit" {
		return nil, nil
	}

	workDir, err := ResolveProjectPath()
	if err != nil {
		return []doctorCheck{{
			Name:   "hosting account",
			OK:     false,
			Detail: fmt.Sprintf("resolve project path: %v", err),
		}}, nil
	}

	resolved, err := hosting.ResolveConfig(workDir, cfg)
	if err != nil {
		return []doctorCheck{{
			Name:   "hosting account",
			OK:     false,
			Detail: err.Error(),
		}}, nil
	}

	accountDetail := fmt.Sprintf("provider %s via %s", resolved.ProviderType, resolved.TokenEnvVar)
	if resolved.AccountName != "" {
		accountDetail = fmt.Sprintf("account %s (%s via %s)", resolved.AccountName, resolved.ProviderType, resolved.TokenEnvVar)
	}

	checks := []doctorCheck{{
		Name:   "hosting account",
		OK:     true,
		Detail: accountDetail,
	}}
	if strings.TrimSpace(os.Getenv(resolved.TokenEnvVar)) == "" {
		checks = append(checks, doctorCheck{
			Name:   "completion hosting auth",
			OK:     false,
			Detail: fmt.Sprintf("%s is not set for %s completion action %q", resolved.TokenEnvVar, resolved.ProviderType, action),
		})
		return checks, nil
	}
	checks = append(checks, doctorCheck{
		Name:   "completion hosting auth",
		OK:     true,
		Detail: fmt.Sprintf("%s is set for %s completion action %q", resolved.TokenEnvVar, resolved.ProviderType, action),
	})
	return checks, nil
}

func resolveWorkflowCompletionAction(cfg *config.Config, workflowID string) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("config is required")
	}

	action := cfg.ResolveCompletionAction(workflowID)
	workflowDef, _, err := loadWorkflowForDoctor(workflowID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(workflowDef.CompletionAction) != "" {
		action = workflowDef.CompletionAction
	}
	return action, nil
}
