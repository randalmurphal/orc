package executor

import (
	"fmt"
	"strings"

	llmkit "github.com/randalmurphal/llmkit/v2"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

const (
	ProviderClaude = "claude"
	ProviderCodex  = "codex"
)

var orcSupportedProviders = map[string]struct{}{
	ProviderClaude: {},
	ProviderCodex:  {},
}

func validatedProvider(provider string) (string, error) {
	provider = normalizeProvider(provider)
	if provider == "" {
		provider = ProviderClaude
	}
	if err := validateProvider(provider); err != nil {
		return "", err
	}
	return provider, nil
}

func normalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

func isCodexFamilyProvider(provider string) bool {
	switch normalizeProvider(provider) {
	case ProviderCodex:
		return true
	default:
		return false
	}
}

func explicitProviderFromModelTuple(model string) (string, bool) {
	model = strings.TrimSpace(model)
	idx := strings.Index(model, ":")
	if idx <= 0 {
		return "", false
	}
	p := normalizeProvider(model[:idx])
	if p == "" {
		return "", false
	}
	return p, true
}

func ParseProviderModel(s string) (provider, model string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}

	idx := strings.Index(s, ":")
	if idx <= 0 {
		return ProviderClaude, s
	}

	provider = normalizeProvider(s[:idx])
	model = strings.TrimSpace(s[idx+1:])
	if provider == "" {
		provider = ProviderClaude
	}
	return provider, model
}

func FormatProviderModel(provider, model string) string {
	if provider == "" || provider == ProviderClaude {
		return model
	}
	return provider + ":" + model
}

func validateProvider(provider string) error {
	_, err := getOrcProviderDefinition(provider)
	return err
}

func getOrcProviderDefinition(provider string) (llmkit.ProviderDefinition, error) {
	provider = normalizeProvider(provider)
	if _, ok := orcSupportedProviders[provider]; !ok {
		return llmkit.ProviderDefinition{}, fmt.Errorf("unsupported provider %q (supported: claude, codex)", provider)
	}

	def, ok := llmkit.GetProviderDefinition(provider)
	if !ok {
		return llmkit.ProviderDefinition{}, fmt.Errorf("provider definition missing for %q", provider)
	}
	if !def.Supported {
		return llmkit.ProviderDefinition{}, fmt.Errorf("provider %q is not supported", provider)
	}
	return def, nil
}

func providerDefaultModel(provider string) string {
	switch normalizeProvider(provider) {
	case ProviderClaude:
		return "opus"
	case ProviderCodex:
		return "gpt-5.3-codex"
	default:
		return ""
	}
}

func (we *WorkflowExecutor) resolveExecutorAgent(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) (*db.Agent, error) {
	if we.projectDB == nil {
		if phase != nil && phase.AgentOverride != "" {
			return nil, fmt.Errorf("phase %s agent override %q requires project database access", tmpl.ID, phase.AgentOverride)
		}
		if tmpl != nil && tmpl.AgentID != "" {
			return nil, fmt.Errorf("phase %s agent %q requires project database access", tmpl.ID, tmpl.AgentID)
		}
		return nil, nil
	}

	if phase != nil && phase.AgentOverride != "" {
		agent, err := we.projectDB.GetAgent(phase.AgentOverride)
		if err != nil {
			return nil, fmt.Errorf("load agent override %q for phase %s: %w", phase.AgentOverride, tmpl.ID, err)
		}
		if agent == nil {
			return nil, fmt.Errorf("agent override %q for phase %s not found", phase.AgentOverride, tmpl.ID)
		}
		return agent, nil
	}

	if tmpl != nil && tmpl.AgentID != "" {
		agent, err := we.projectDB.GetAgent(tmpl.AgentID)
		if err != nil {
			return nil, fmt.Errorf("load executor agent %q for phase %s: %w", tmpl.AgentID, tmpl.ID, err)
		}
		if agent == nil {
			return nil, fmt.Errorf("executor agent %q for phase %s not found", tmpl.AgentID, tmpl.ID)
		}
		return agent, nil
	}

	return nil, nil
}

func (we *WorkflowExecutor) resolvePhaseProvider(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) (string, error) {
	if we.runProvider != "" {
		return validatedProvider(we.runProvider)
	}

	if phase != nil && phase.ProviderOverride != "" {
		return validatedProvider(phase.ProviderOverride)
	}

	if we.wf != nil && we.wf.DefaultProvider != "" {
		return validatedProvider(we.wf.DefaultProvider)
	}

	if tmpl != nil && tmpl.Provider != "" {
		return validatedProvider(tmpl.Provider)
	}

	agent, err := we.resolveExecutorAgent(tmpl, phase)
	if err != nil {
		return "", err
	}
	if agent != nil && agent.Provider != "" {
		return validatedProvider(agent.Provider)
	}

	if we.orcConfig != nil && we.orcConfig.Provider != "" {
		return validatedProvider(we.orcConfig.Provider)
	}

	if phase != nil {
		if p, ok := explicitProviderFromModelTuple(phase.ModelOverride); ok {
			return validatedProvider(p)
		}
	}
	if we.wf != nil {
		if p, ok := explicitProviderFromModelTuple(we.wf.DefaultModel); ok {
			return validatedProvider(p)
		}
	}
	if agent != nil {
		if p, ok := explicitProviderFromModelTuple(agent.Model); ok {
			return validatedProvider(p)
		}
	}
	if we.orcConfig != nil {
		if p, ok := explicitProviderFromModelTuple(we.orcConfig.Model); ok {
			return validatedProvider(p)
		}
	}

	return validatedProvider(ProviderClaude)
}

func (we *WorkflowExecutor) resolvePhaseModel(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) (string, error) {
	if phase != nil && phase.ModelOverride != "" {
		if _, m := ParseProviderModel(phase.ModelOverride); m != "" {
			return m, nil
		}
		return phase.ModelOverride, nil
	}

	if we.wf != nil && we.wf.DefaultModel != "" {
		if _, m := ParseProviderModel(we.wf.DefaultModel); m != "" {
			return m, nil
		}
		return we.wf.DefaultModel, nil
	}

	agent, err := we.resolveExecutorAgent(tmpl, phase)
	if err != nil {
		return "", err
	}
	if agent != nil && agent.Model != "" {
		if _, m := ParseProviderModel(agent.Model); m != "" {
			return m, nil
		}
		return agent.Model, nil
	}

	provider, err := we.resolvePhaseProvider(tmpl, phase)
	if err != nil {
		return "", err
	}
	if isCodexFamilyProvider(provider) {
		if m := providerDefaultModel(provider); m != "" {
			return m, nil
		}
	}

	if we.orcConfig != nil && we.orcConfig.Model != "" {
		if _, m := ParseProviderModel(we.orcConfig.Model); m != "" {
			return m, nil
		}
		return we.orcConfig.Model, nil
	}

	if m := providerDefaultModel(provider); m != "" {
		return m, nil
	}
	return "opus", nil
}

func (we *WorkflowExecutor) getEffectivePhaseRuntimeConfig(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) (*PhaseRuntimeConfig, error) {
	var cfg *PhaseRuntimeConfig

	agent, err := we.resolveExecutorAgent(tmpl, phase)
	if err != nil {
		return nil, err
	}

	if agent != nil && agent.RuntimeConfig != "" {
		base, err := ParsePhaseRuntimeConfig(agent.RuntimeConfig)
		if err != nil {
			return nil, fmt.Errorf("parse agent runtime_config for %s: %w", agent.ID, err)
		}
		cfg = base
	}

	if tmpl != nil && tmpl.RuntimeConfig != "" {
		tmplCfg, err := ParsePhaseRuntimeConfig(tmpl.RuntimeConfig)
		if err != nil {
			return nil, fmt.Errorf("parse template runtime_config for %s: %w", tmpl.ID, err)
		}
		if cfg == nil {
			cfg = tmplCfg
		} else {
			cfg = cfg.Merge(tmplCfg)
		}
	}

	if phase != nil && phase.RuntimeConfigOverride != "" {
		override, err := ParsePhaseRuntimeConfig(phase.RuntimeConfigOverride)
		if err != nil {
			return nil, fmt.Errorf("parse workflow phase runtime_config_override for %s: %w", phase.PhaseTemplateID, err)
		}
		if cfg == nil {
			cfg = override
		} else {
			cfg = cfg.Merge(override)
		}
	}

	if cfg == nil {
		cfg = &PhaseRuntimeConfig{}
	}

	if agent != nil && agent.SystemPrompt != "" {
		cfg.Shared.SystemPrompt = agent.SystemPrompt
	}

	if cfg.IsEmpty() {
		return nil, nil
	}
	return cfg, nil
}

func setTaskSessionMetadata(t *orcv1.Task, phaseID, provider, model string) {
	if t == nil {
		return
	}
	if t.Metadata == nil {
		t.Metadata = make(map[string]string)
	}
	t.Metadata["phase:"+phaseID+":provider"] = provider
	t.Metadata["phase:"+phaseID+":model"] = model
}
