package executor

import (
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

	def := ToInlineAgentDef(agent)

	assert.Equal(t, agent.Description, def.Description)
	assert.Equal(t, agent.Prompt, def.Prompt)
	assert.Equal(t, agent.Tools, def.Tools)
	assert.Equal(t, agent.Model, def.Model)
}

func TestLoadPhaseAgents(t *testing.T) {
	pdb, err := db.OpenProjectInMemory()
	require.NoError(t, err)
	defer pdb.Close()

	// Seed built-in phase templates first
	_, err = workflow.SeedBuiltins(pdb)
	require.NoError(t, err)

	// Seed built-in agents
	_, err = workflow.SeedAgents(pdb)
	require.NoError(t, err)

	// Test loading agents for review phase with small weight
	agents, err := LoadPhaseAgents(pdb, "review", "small")
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
	largeAgents, err := LoadPhaseAgents(pdb, "review", "large")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(largeAgents), len(agents), "large weight should have >= agents than small")

	// Test loading agents for phase with no agents
	noAgents, err := LoadPhaseAgents(pdb, "docs", "small")
	require.NoError(t, err)
	assert.Empty(t, noAgents) // docs phase doesn't have agents
}

func TestLoadPhaseAgents_NonExistentPhase(t *testing.T) {
	pdb, err := db.OpenProjectInMemory()
	require.NoError(t, err)
	defer pdb.Close()

	// Load agents for non-existent phase - should return empty, not error
	agents, err := LoadPhaseAgents(pdb, "nonexistent-phase", "medium")
	require.NoError(t, err)
	assert.Empty(t, agents)
}
