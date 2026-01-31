package executor

import (
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToInlineAgentDef(t *testing.T) {
	agent := &db.Agent{
		ID:          "test-agent",
		Name:        "Test Agent",
		Description: "A test agent",
		Prompt:      "You are a test agent.",
		Tools:       []string{"Read", "Grep"},
		Model:       "opus",
	}

	def := ToInlineAgentDef(agent, nil)

	assert.Equal(t, agent.Description, def.Description)
	assert.Equal(t, agent.Prompt, def.Prompt)
	assert.Equal(t, agent.Tools, def.Tools)
	assert.Equal(t, agent.Model, def.Model)
}

func TestToInlineAgentDef_RendersTemplateVars(t *testing.T) {
	agent := &db.Agent{
		ID:          "test-agent",
		Description: "Test agent",
		Prompt:      "Language: {{LANGUAGE}}\nFrameworks: {{FRAMEWORKS}}",
		Tools:       []string{"Read", "Grep"},
		Model:       "sonnet",
	}
	vars := map[string]string{
		"LANGUAGE":   "go",
		"FRAMEWORKS": "cobra, grpc",
	}
	def := ToInlineAgentDef(agent, vars)
	assert.Equal(t, "Language: go\nFrameworks: cobra, grpc", def.Prompt)
	assert.Equal(t, "Test agent", def.Description)
	assert.Equal(t, "sonnet", def.Model)
}

func TestToInlineAgentDef_NilVars(t *testing.T) {
	agent := &db.Agent{
		ID:     "test-agent",
		Prompt: "No vars {{LANGUAGE}} here",
	}
	def := ToInlineAgentDef(agent, nil)
	assert.Equal(t, "No vars {{LANGUAGE}} here", def.Prompt)
}

func TestToInlineAgentDef_EmptyVars(t *testing.T) {
	agent := &db.Agent{
		ID:     "test-agent",
		Prompt: "No vars {{LANGUAGE}} here",
	}
	def := ToInlineAgentDef(agent, map[string]string{})
	assert.Equal(t, "No vars {{LANGUAGE}} here", def.Prompt)
}

func TestToInlineAgentDef_PartialVars(t *testing.T) {
	agent := &db.Agent{
		ID:     "test-agent",
		Prompt: "Lang: {{LANGUAGE}}, Framework: {{FRAMEWORKS}}",
	}
	vars := map[string]string{
		"LANGUAGE": "python",
	}
	def := ToInlineAgentDef(agent, vars)
	// FRAMEWORKS not in vars, so it becomes empty per RenderTemplate behavior
	assert.Equal(t, "Lang: python, Framework: ", def.Prompt)
}

func TestLoadPhaseAgents(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	defer func() { _ = gdb.Close() }()

	// Seed built-in phase templates first
	_, err = workflow.SeedBuiltins(gdb)
	require.NoError(t, err)

	// Seed built-in agents
	_, err = workflow.SeedAgents(gdb)
	require.NoError(t, err)

	// Test loading agents for review phase with small weight
	agents, err := LoadPhaseAgents(gdb, "review", "small", nil)
	require.NoError(t, err)
	require.NotNil(t, agents)

	// Should have at least code-reviewer and silent-failure-hunter for small weight
	assert.Contains(t, agents, "code-reviewer")
	assert.Contains(t, agents, "silent-failure-hunter")

	// Verify agent structure
	reviewer := agents["code-reviewer"]
	assert.NotEmpty(t, reviewer.Description)
	assert.NotEmpty(t, reviewer.Prompt)
	assert.NotEmpty(t, reviewer.Tools)
	assert.Equal(t, "opus", reviewer.Model)

	// Test loading agents for review phase with large weight (should have more agents)
	largeAgents, err := LoadPhaseAgents(gdb, "review", "large", nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(largeAgents), len(agents), "large weight should have >= agents than small")

	// Test loading agents for phase with no agents
	noAgents, err := LoadPhaseAgents(gdb, "docs", "small", nil)
	require.NoError(t, err)
	assert.Empty(t, noAgents) // docs phase doesn't have agents
}

func TestLoadPhaseAgents_NonExistentPhase(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	defer func() { _ = gdb.Close() }()

	// Load agents for non-existent phase - should return empty, not error
	agents, err := LoadPhaseAgents(gdb, "nonexistent-phase", "medium", nil)
	require.NoError(t, err)
	assert.Empty(t, agents)
}

func TestLoadPhaseAgents_WithVars(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	defer func() { _ = gdb.Close() }()

	// Seed built-in data
	_, err = workflow.SeedBuiltins(gdb)
	require.NoError(t, err)
	_, err = workflow.SeedAgents(gdb)
	require.NoError(t, err)

	vars := map[string]string{
		"LANGUAGE":   "go",
		"FRAMEWORKS": "cobra",
	}

	// Load with vars - should not error and agents should be returned
	agents, err := LoadPhaseAgents(gdb, "review", "small", vars)
	require.NoError(t, err)
	require.NotNil(t, agents)

	// Agents should still be present
	assert.Contains(t, agents, "code-reviewer")
}
