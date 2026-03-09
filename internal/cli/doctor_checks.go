package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
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
