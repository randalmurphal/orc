package db

import (
	"testing"
)

// openTestProjectDB opens an in-memory database for testing.
func openTestProjectDB(t *testing.T) *ProjectDB {
	t.Helper()
	pdb, err := OpenProjectInMemory()
	if err != nil {
		t.Fatalf("OpenProjectInMemory failed: %v", err)
	}
	t.Cleanup(func() { pdb.Close() })
	return pdb
}

func TestAgentCRUD(t *testing.T) {
	pdb := openTestProjectDB(t)

	// Create agent
	agent := &Agent{
		ID:          "test-agent",
		Name:        "Test Agent",
		Description: "A test agent for unit testing",
		Prompt:      "You are a test agent.",
		Tools:       []string{"Read", "Grep"},
		Model:       "sonnet",
		IsBuiltin:   false,
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("SaveAgent failed: %v", err)
	}

	// Get agent
	got, err := pdb.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetAgent returned nil")
	}

	if got.ID != agent.ID {
		t.Errorf("ID = %s, want %s", got.ID, agent.ID)
	}
	if got.Name != agent.Name {
		t.Errorf("Name = %s, want %s", got.Name, agent.Name)
	}
	if got.Description != agent.Description {
		t.Errorf("Description = %s, want %s", got.Description, agent.Description)
	}
	if got.Prompt != agent.Prompt {
		t.Errorf("Prompt = %s, want %s", got.Prompt, agent.Prompt)
	}
	if len(got.Tools) != len(agent.Tools) {
		t.Errorf("Tools = %v, want %v", got.Tools, agent.Tools)
	}
	if got.Model != agent.Model {
		t.Errorf("Model = %s, want %s", got.Model, agent.Model)
	}

	// List agents
	agents, err := pdb.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("ListAgents returned %d agents, want 1", len(agents))
	}

	// Count agents
	count, err := pdb.CountAgents()
	if err != nil {
		t.Fatalf("CountAgents failed: %v", err)
	}
	if count != 1 {
		t.Errorf("CountAgents = %d, want 1", count)
	}

	// Delete agent
	if err := pdb.DeleteAgent("test-agent"); err != nil {
		t.Fatalf("DeleteAgent failed: %v", err)
	}

	// Verify deleted
	got, err = pdb.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent after delete failed: %v", err)
	}
	if got != nil {
		t.Error("GetAgent returned agent after delete")
	}
}

func TestDeleteBuiltinAgent(t *testing.T) {
	pdb := openTestProjectDB(t)

	// Create builtin agent
	agent := &Agent{
		ID:          "builtin-agent",
		Name:        "Builtin Agent",
		Description: "A builtin agent",
		Prompt:      "You are a builtin agent.",
		IsBuiltin:   true,
	}

	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("SaveAgent failed: %v", err)
	}

	// Try to delete - should fail
	err := pdb.DeleteAgent("builtin-agent")
	if err == nil {
		t.Error("DeleteAgent should fail for builtin agent")
	}
}

func TestPhaseAgentCRUD(t *testing.T) {
	pdb := openTestProjectDB(t)

	// First create the phase template (required for foreign key)
	pt := &PhaseTemplate{
		ID:           "review",
		Name:         "Review",
		PromptSource: "embedded",
		PromptPath:   "prompts/review.md",
	}
	if err := pdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	// Create an agent
	agent := &Agent{
		ID:          "review-agent",
		Name:        "Review Agent",
		Description: "Code review agent",
		Prompt:      "You are a code reviewer.",
		Tools:       []string{"Read", "Grep"},
	}
	if err := pdb.SaveAgent(agent); err != nil {
		t.Fatalf("SaveAgent failed: %v", err)
	}

	// Create phase agent association
	pa := &PhaseAgent{
		PhaseTemplateID: "review",
		AgentID:         "review-agent",
		Sequence:        0,
		Role:            "correctness",
		WeightFilter:    []string{"medium", "large"},
		IsBuiltin:       false,
	}

	if err := pdb.SavePhaseAgent(pa); err != nil {
		t.Fatalf("SavePhaseAgent failed: %v", err)
	}

	// Get phase agents
	agents, err := pdb.GetPhaseAgents("review")
	if err != nil {
		t.Fatalf("GetPhaseAgents failed: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("GetPhaseAgents returned %d agents, want 1", len(agents))
	}

	got := agents[0]
	if got.AgentID != pa.AgentID {
		t.Errorf("AgentID = %s, want %s", got.AgentID, pa.AgentID)
	}
	if got.Role != pa.Role {
		t.Errorf("Role = %s, want %s", got.Role, pa.Role)
	}
	if len(got.WeightFilter) != 2 {
		t.Errorf("WeightFilter = %v, want %v", got.WeightFilter, pa.WeightFilter)
	}

	// Test weight filtering
	mediumAgents, err := pdb.GetPhaseAgentsForWeight("review", "medium")
	if err != nil {
		t.Fatalf("GetPhaseAgentsForWeight failed: %v", err)
	}
	if len(mediumAgents) != 1 {
		t.Errorf("GetPhaseAgentsForWeight(medium) returned %d, want 1", len(mediumAgents))
	}

	smallAgents, err := pdb.GetPhaseAgentsForWeight("review", "small")
	if err != nil {
		t.Fatalf("GetPhaseAgentsForWeight failed: %v", err)
	}
	if len(smallAgents) != 0 {
		t.Errorf("GetPhaseAgentsForWeight(small) returned %d, want 0", len(smallAgents))
	}

	// Delete phase agent
	if err := pdb.DeletePhaseAgent("review", "review-agent"); err != nil {
		t.Fatalf("DeletePhaseAgent failed: %v", err)
	}

	// Verify deleted
	agents, err = pdb.GetPhaseAgents("review")
	if err != nil {
		t.Fatalf("GetPhaseAgents after delete failed: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("GetPhaseAgents after delete returned %d, want 0", len(agents))
	}
}

func TestGetPhaseAgentsWithDefinitions(t *testing.T) {
	pdb := openTestProjectDB(t)

	// First create the phase template (required for foreign key)
	pt := &PhaseTemplate{
		ID:           "implement",
		Name:         "Implement",
		PromptSource: "embedded",
		PromptPath:   "prompts/implement.md",
	}
	if err := pdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	// Create two agents
	agent1 := &Agent{
		ID:          "agent1",
		Name:        "Agent 1",
		Description: "First agent",
		Prompt:      "You are agent 1.",
		Tools:       []string{"Read"},
	}
	agent2 := &Agent{
		ID:          "agent2",
		Name:        "Agent 2",
		Description: "Second agent",
		Prompt:      "You are agent 2.",
		Tools:       []string{"Read", "Grep"},
	}
	if err := pdb.SaveAgent(agent1); err != nil {
		t.Fatalf("SaveAgent failed: %v", err)
	}
	if err := pdb.SaveAgent(agent2); err != nil {
		t.Fatalf("SaveAgent failed: %v", err)
	}

	// Create phase associations (same sequence = parallel)
	pa1 := &PhaseAgent{
		PhaseTemplateID: "implement",
		AgentID:         "agent1",
		Sequence:        0,
		Role:            "role1",
	}
	pa2 := &PhaseAgent{
		PhaseTemplateID: "implement",
		AgentID:         "agent2",
		Sequence:        0, // Same sequence = parallel
		Role:            "role2",
	}
	if err := pdb.SavePhaseAgent(pa1); err != nil {
		t.Fatalf("SavePhaseAgent failed: %v", err)
	}
	if err := pdb.SavePhaseAgent(pa2); err != nil {
		t.Fatalf("SavePhaseAgent failed: %v", err)
	}

	// Get with definitions
	agents, err := pdb.GetPhaseAgentsWithDefinitions("implement", "medium")
	if err != nil {
		t.Fatalf("GetPhaseAgentsWithDefinitions failed: %v", err)
	}
	if len(agents) != 2 {
		t.Errorf("GetPhaseAgentsWithDefinitions returned %d, want 2", len(agents))
	}

	// Test grouping
	groups := GroupBySequence(agents)
	if len(groups) != 1 {
		t.Errorf("GroupBySequence returned %d groups, want 1 (parallel)", len(groups))
	}
	if len(groups[0]) != 2 {
		t.Errorf("GroupBySequence[0] has %d agents, want 2", len(groups[0]))
	}
}

func TestGroupBySequence(t *testing.T) {
	// Create test agents with different sequences
	agents := []*AgentWithAssignment{
		{
			Agent:      &Agent{ID: "a1"},
			PhaseAgent: &PhaseAgent{Sequence: 0},
		},
		{
			Agent:      &Agent{ID: "a2"},
			PhaseAgent: &PhaseAgent{Sequence: 0},
		},
		{
			Agent:      &Agent{ID: "a3"},
			PhaseAgent: &PhaseAgent{Sequence: 1},
		},
	}

	groups := GroupBySequence(agents)
	if len(groups) != 2 {
		t.Errorf("GroupBySequence returned %d groups, want 2", len(groups))
	}
	if len(groups[0]) != 2 {
		t.Errorf("groups[0] has %d agents, want 2 (sequence 0)", len(groups[0]))
	}
	if len(groups[1]) != 1 {
		t.Errorf("groups[1] has %d agents, want 1 (sequence 1)", len(groups[1]))
	}
}
