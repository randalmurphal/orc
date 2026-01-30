package workflow

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/templates"
)

// --- SC-1: Built-in agent dependency-validator exists with correct properties ---

func TestDependencyValidatorAgent_ExistsInBuiltinAgentFiles(t *testing.T) {
	t.Parallel()

	found := false
	for _, file := range builtinAgentFiles {
		if file == "agents/dependency-validator.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("agents/dependency-validator.md not found in builtinAgentFiles")
	}
}

func TestDependencyValidatorAgent_EmbeddedFileParses(t *testing.T) {
	t.Parallel()

	content, err := templates.Agents.ReadFile("agents/dependency-validator.md")
	if err != nil {
		t.Fatalf("failed to read embedded agent file: %v", err)
	}

	fm, prompt, err := ParseAgentMarkdown(content)
	if err != nil {
		t.Fatalf("failed to parse agent markdown: %v", err)
	}

	// SC-1: name must be "dependency-validator"
	if fm.Name != "dependency-validator" {
		t.Errorf("Name = %q, want %q", fm.Name, "dependency-validator")
	}

	// SC-1: model must be "haiku"
	if fm.Model != "haiku" {
		t.Errorf("Model = %q, want %q", fm.Model, "haiku")
	}

	// SC-1: must have a description
	if fm.Description == "" {
		t.Error("Description is empty")
	}

	// SC-1: must have tools
	if len(fm.Tools) == 0 {
		t.Error("Tools list is empty")
	}

	// SC-2: prompt must not be empty
	if prompt == "" {
		t.Error("prompt body is empty")
	}
}

// --- SC-2: Agent markdown covers all 4 analysis categories ---

func TestDependencyValidatorAgent_PromptCoversAnalysisCategories(t *testing.T) {
	t.Parallel()

	content, err := templates.Agents.ReadFile("agents/dependency-validator.md")
	if err != nil {
		t.Fatalf("failed to read embedded agent file: %v", err)
	}

	_, prompt, err := ParseAgentMarkdown(content)
	if err != nil {
		t.Fatalf("failed to parse agent markdown: %v", err)
	}

	promptLower := strings.ToLower(prompt)

	// SC-2: Prompt must cover all 4 analysis categories
	categories := []struct {
		name    string
		markers []string // At least one marker must appear in prompt
	}{
		{
			name:    "function/type definitions",
			markers: []string{"function", "type", "definition", "import"},
		},
		{
			name:    "file creates/modifies",
			markers: []string{"file", "create", "modif"},
		},
		{
			name:    "API endpoints",
			markers: []string{"api", "endpoint"},
		},
		{
			name:    "shared state/config",
			markers: []string{"shared", "state", "config"},
		},
	}

	for _, cat := range categories {
		found := false
		for _, marker := range cat.markers {
			if strings.Contains(promptLower, marker) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("prompt does not cover analysis category %q (none of %v found)",
				cat.name, cat.markers)
		}
	}
}

// --- SC-3: Agent prompt instructs filtering existing dependencies ---

func TestDependencyValidatorAgent_PromptFiltersExistingDeps(t *testing.T) {
	t.Parallel()

	content, err := templates.Agents.ReadFile("agents/dependency-validator.md")
	if err != nil {
		t.Fatalf("failed to read embedded agent file: %v", err)
	}

	_, prompt, err := ParseAgentMarkdown(content)
	if err != nil {
		t.Fatalf("failed to parse agent markdown: %v", err)
	}

	promptLower := strings.ToLower(prompt)

	// SC-3: prompt must mention filtering/excluding existing/declared dependencies
	hasFilter := strings.Contains(promptLower, "existing") ||
		strings.Contains(promptLower, "already declared") ||
		strings.Contains(promptLower, "already in blocked_by") ||
		strings.Contains(promptLower, "blocked_by")

	if !hasFilter {
		t.Error("prompt does not instruct filtering out existing/declared dependencies")
	}
}

// --- SC-4: Agent output must conform to GateAgentResponse schema ---
// (The schema conformance is enforced by ExecuteWithSchema[GateAgentResponse]
// in the agent executor. We verify the prompt instructs correct output format.)

func TestDependencyValidatorAgent_PromptSpecifiesOutputSchema(t *testing.T) {
	t.Parallel()

	content, err := templates.Agents.ReadFile("agents/dependency-validator.md")
	if err != nil {
		t.Fatalf("failed to read embedded agent file: %v", err)
	}

	_, prompt, err := ParseAgentMarkdown(content)
	if err != nil {
		t.Fatalf("failed to parse agent markdown: %v", err)
	}

	promptLower := strings.ToLower(prompt)

	// SC-4: prompt must reference the expected output structure
	if !strings.Contains(promptLower, "missing_deps") && !strings.Contains(promptLower, "missing deps") {
		t.Error("prompt does not mention missing_deps output field")
	}
	if !strings.Contains(promptLower, "confidence") {
		t.Error("prompt does not mention confidence output field")
	}
	if !strings.Contains(promptLower, "approved") || !strings.Contains(promptLower, "rejected") {
		t.Error("prompt does not mention approved/rejected status values")
	}
}

// --- SC-10: Agent registered in ListBuiltinAgentIDs ---

func TestDependencyValidatorAgent_InListBuiltinAgentIDs(t *testing.T) {
	t.Parallel()

	ids := ListBuiltinAgentIDs()

	found := false
	for _, id := range ids {
		if id == "dependency-validator" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ListBuiltinAgentIDs() does not include %q; got %v",
			"dependency-validator", ids)
	}
}

func TestListBuiltinAgentIDs_CountIncludesDependencyValidator(t *testing.T) {
	t.Parallel()

	ids := ListBuiltinAgentIDs()
	// Was 6, now should be 7 with dependency-validator
	if len(ids) != 7 {
		t.Errorf("ListBuiltinAgentIDs() returned %d, want 7; got %v", len(ids), ids)
	}
}

// --- SC-1 (integration): SeedAgents creates dependency-validator in GlobalDB ---

func TestSeedAgents_CreatesDependencyValidator(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	if err != nil {
		t.Fatalf("OpenGlobalAt failed: %v", err)
	}
	defer func() { _ = gdb.Close() }()

	// Seed phase templates first (foreign key requirement)
	_, err = SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	// Seed agents
	_, err = SeedAgents(gdb)
	if err != nil {
		t.Fatalf("SeedAgents failed: %v", err)
	}

	// Verify dependency-validator was created
	agent, err := gdb.GetAgent("dependency-validator")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if agent == nil {
		t.Fatal("dependency-validator agent not found in GlobalDB after SeedAgents")
	}

	// SC-1: model must be haiku
	if agent.Model != "haiku" {
		t.Errorf("agent model = %q, want %q", agent.Model, "haiku")
	}

	// SC-1: must be builtin
	if !agent.IsBuiltin {
		t.Error("agent IsBuiltin = false, want true")
	}

	// SC-1: must have tools
	if len(agent.Tools) == 0 {
		t.Error("agent has no tools")
	}

	// Must have a prompt
	if agent.Prompt == "" {
		t.Error("agent has empty prompt")
	}
}

func TestSeedAgents_TotalAgentCount(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	if err != nil {
		t.Fatalf("OpenGlobalAt failed: %v", err)
	}
	defer func() { _ = gdb.Close() }()

	_, err = SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	_, err = SeedAgents(gdb)
	if err != nil {
		t.Fatalf("SeedAgents failed: %v", err)
	}

	agents, err := gdb.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	// Was 6, now should be at least 7
	if len(agents) < 7 {
		t.Errorf("ListAgents returned %d agents, want at least 7", len(agents))
	}
}

// --- Preservation: Existing agents still seed correctly ---

func TestSeedAgents_ExistingAgentsPreserved(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	if err != nil {
		t.Fatalf("OpenGlobalAt failed: %v", err)
	}
	defer func() { _ = gdb.Close() }()

	_, err = SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	_, err = SeedAgents(gdb)
	if err != nil {
		t.Fatalf("SeedAgents failed: %v", err)
	}

	// All original 6 agents must still exist
	existingAgents := []string{
		"code-reviewer",
		"code-simplifier",
		"comment-analyzer",
		"pr-test-analyzer",
		"silent-failure-hunter",
		"type-design-analyzer",
	}

	for _, id := range existingAgents {
		agent, err := gdb.GetAgent(id)
		if err != nil {
			t.Errorf("GetAgent(%s) failed: %v", id, err)
			continue
		}
		if agent == nil {
			t.Errorf("existing agent %s not found after adding dependency-validator", id)
		}
	}
}
