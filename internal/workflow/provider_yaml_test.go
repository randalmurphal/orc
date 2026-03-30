package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestProviderYAML_WorkflowDefaultProvider(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
id: test-provider
name: Provider Test
default_provider: codex
phases:
  - template: implement
    sequence: 1
`)

	wf, err := parseWorkflowYAML(yamlData)
	require.NoError(t, err)

	assert.Equal(t, "test-provider", wf.ID)
	assert.Equal(t, "codex", wf.DefaultProvider)
}

func TestProviderYAML_PhaseProviderOverride(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
id: test-phase-provider
name: Phase Provider Test
phases:
  - template: implement
    sequence: 1
    provider_override: codex
  - template: review
    sequence: 2
`)

	wf, err := parseWorkflowYAML(yamlData)
	require.NoError(t, err)

	require.Len(t, wf.Phases, 2)
	assert.Equal(t, "codex", wf.Phases[0].ProviderOverride)
	assert.Equal(t, "", wf.Phases[1].ProviderOverride)
}

func TestProviderYAML_WorkflowAndPhaseProviders(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
id: mixed-provider
name: Mixed Provider Test
default_provider: codex
phases:
  - template: spec
    sequence: 1
  - template: implement
    sequence: 2
    provider_override: codex
  - template: review
    sequence: 3
    provider_override: claude
`)

	wf, err := parseWorkflowYAML(yamlData)
	require.NoError(t, err)

	assert.Equal(t, "codex", wf.DefaultProvider)
	require.Len(t, wf.Phases, 3)
	assert.Equal(t, "", wf.Phases[0].ProviderOverride)
	assert.Equal(t, "codex", wf.Phases[1].ProviderOverride)
	assert.Equal(t, "claude", wf.Phases[2].ProviderOverride)
}

func TestProviderYAML_EmptyProviderOmittedFromOutput(t *testing.T) {
	t.Parallel()

	wf := &Workflow{
		ID:   "test-wf",
		Name: "Test Workflow",
		Phases: []WorkflowPhase{
			{
				PhaseTemplateID: "implement",
				Sequence:        1,
			},
		},
	}

	data, err := marshalWorkflowYAML(wf)
	require.NoError(t, err)

	// Unmarshal to map to check raw keys
	var raw map[string]any
	require.NoError(t, yaml.Unmarshal(data, &raw))
	_, hasDefaultProvider := raw["default_provider"]
	assert.False(t, hasDefaultProvider, "empty default_provider should be omitted from YAML output")

	// Check phases don't have provider_override key when empty
	phases, ok := raw["phases"].([]any)
	require.True(t, ok)
	for i, p := range phases {
		phaseMap, ok := p.(map[string]any)
		require.True(t, ok)
		_, hasProviderOverride := phaseMap["provider_override"]
		assert.False(t, hasProviderOverride, "phase %d: empty provider_override should be omitted from YAML output", i)
	}
}

func TestProviderYAML_RoundTrip_Workflow(t *testing.T) {
	t.Parallel()

	original := &Workflow{
		ID:              "round-trip",
		Name:            "Round Trip",
		DefaultModel:    "opus",
		DefaultProvider: "codex",
		Phases: []WorkflowPhase{
			{
				PhaseTemplateID:  "spec",
				Sequence:         1,
				ProviderOverride: "codex",
			},
			{
				PhaseTemplateID: "implement",
				Sequence:        2,
			},
			{
				PhaseTemplateID:  "review",
				Sequence:         3,
				ProviderOverride: "claude",
			},
		},
	}

	// Write to YAML
	data, err := marshalWorkflowYAML(original)
	require.NoError(t, err)

	// Read back
	parsed, err := parseWorkflowYAML(data)
	require.NoError(t, err)

	assert.Equal(t, original.DefaultProvider, parsed.DefaultProvider)
	require.Len(t, parsed.Phases, 3)
	assert.Equal(t, "codex", parsed.Phases[0].ProviderOverride)
	assert.Equal(t, "", parsed.Phases[1].ProviderOverride)
	assert.Equal(t, "claude", parsed.Phases[2].ProviderOverride)
}

func TestProviderYAML_RoundTrip_Phase(t *testing.T) {
	t.Parallel()

	original := &PhaseTemplate{
		ID:           "test-phase",
		Name:         "Test Phase",
		PromptSource: PromptSourceEmbedded,
		GateType:     GateAuto,
		Provider:     "codex",
	}

	// Write to YAML
	data, err := marshalPhaseYAML(original)
	require.NoError(t, err)

	// Read back
	parsed, err := parsePhaseYAML(data)
	require.NoError(t, err)

	assert.Equal(t, "codex", parsed.Provider)
}

func TestProviderYAML_PhaseTemplate_EmptyProviderOmitted(t *testing.T) {
	t.Parallel()

	phase := &PhaseTemplate{
		ID:           "no-provider-phase",
		Name:         "No Provider Phase",
		PromptSource: PromptSourceEmbedded,
		GateType:     GateAuto,
	}

	data, err := marshalPhaseYAML(phase)
	require.NoError(t, err)

	// Unmarshal to a map to check raw keys
	var raw map[string]any
	require.NoError(t, yaml.Unmarshal(data, &raw))
	_, hasProvider := raw["provider"]
	assert.False(t, hasProvider, "empty provider should be omitted from YAML output")
}

func TestProviderYAML_DefaultProviderOmittedWhenEmpty(t *testing.T) {
	t.Parallel()

	wf := &Workflow{
		ID:   "no-default-provider",
		Name: "No Default Provider",
		Phases: []WorkflowPhase{
			{
				PhaseTemplateID:  "implement",
				Sequence:         1,
				ProviderOverride: "codex",
			},
		},
	}

	data, err := marshalWorkflowYAML(wf)
	require.NoError(t, err)

	// Unmarshal to a map to check raw keys
	var raw map[string]any
	require.NoError(t, yaml.Unmarshal(data, &raw))
	_, hasDefault := raw["default_provider"]
	assert.False(t, hasDefault, "empty default_provider should be omitted from YAML output")
}

func TestProviderYAML_PhaseModelAndProviderOverrides(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
id: test-phase-overrides
name: Phase Override Test
phases:
  - template: implement_codex
    sequence: 1
    provider_override: codex
    model_override: gpt-5.4
`)

	wf, err := parseWorkflowYAML(yamlData)
	require.NoError(t, err)
	require.Len(t, wf.Phases, 1)
	assert.Equal(t, "codex", wf.Phases[0].ProviderOverride)
	assert.Equal(t, "gpt-5.4", wf.Phases[0].ModelOverride)
}

func TestProviderYAML_RoundTrip_PhaseOverridesUseOverrideKeys(t *testing.T) {
	t.Parallel()

	original := &Workflow{
		ID:   "round-trip-overrides",
		Name: "Round Trip Overrides",
		Phases: []WorkflowPhase{
			{
				PhaseTemplateID:  "implement_codex",
				Sequence:         1,
				ModelOverride:    "gpt-5.4",
				ProviderOverride: "codex",
			},
		},
	}

	data, err := marshalWorkflowYAML(original)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, yaml.Unmarshal(data, &raw))
	phases, ok := raw["phases"].([]any)
	require.True(t, ok)
	require.Len(t, phases, 1)
	phaseMap, ok := phases[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "gpt-5.4", phaseMap["model_override"])
	assert.Equal(t, "codex", phaseMap["provider_override"])
	_, hasLegacyModel := phaseMap["model"]
	assert.False(t, hasLegacyModel)
	_, hasLegacyProvider := phaseMap["provider"]
	assert.False(t, hasLegacyProvider)

	parsed, err := parseWorkflowYAML(data)
	require.NoError(t, err)
	require.Len(t, parsed.Phases, 1)
	assert.Equal(t, "gpt-5.4", parsed.Phases[0].ModelOverride)
	assert.Equal(t, "codex", parsed.Phases[0].ProviderOverride)
}

func TestProviderYAML_NoProviderFieldsDefault(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
id: legacy-workflow
name: Legacy Workflow
phases:
  - template: implement
    sequence: 1
`)

	wf, err := parseWorkflowYAML(yamlData)
	require.NoError(t, err)

	assert.Equal(t, "", wf.DefaultProvider)
	require.Len(t, wf.Phases, 1)
	assert.Equal(t, "", wf.Phases[0].ProviderOverride)
}

func TestProviderYAML_RuntimeConfigOverride(t *testing.T) {
	t.Parallel()

	yamlData := []byte(`
id: test-claude-config-override
name: Runtime Config Override Test
phases:
  - template: review_cross
    sequence: 1
    provider_override: codex
    model_override: gpt-5.4
    runtime_config_override: '{"codex":{"reasoning_effort":"xhigh"}}'
`)

	wf, err := parseWorkflowYAML(yamlData)
	require.NoError(t, err)
	require.Len(t, wf.Phases, 1)
	assert.Equal(t, `{"codex":{"reasoning_effort":"xhigh"}}`, wf.Phases[0].RuntimeConfigOverride)
}
