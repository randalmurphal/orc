package workflow

import (
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/templates"
	"gopkg.in/yaml.v3"
)

// AgentFrontmatter represents the YAML frontmatter in agent markdown files.
type AgentFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Model       string   `yaml:"model"`
	Tools       []string `yaml:"tools"`
}

// ParseAgentMarkdown parses an agent markdown file into frontmatter and prompt body.
func ParseAgentMarkdown(content []byte) (*AgentFrontmatter, string, error) {
	str := string(content)

	// Check for frontmatter delimiter
	if !strings.HasPrefix(str, "---\n") {
		return nil, "", fmt.Errorf("missing frontmatter delimiter")
	}

	// Find end of frontmatter
	endIdx := strings.Index(str[4:], "\n---")
	if endIdx == -1 {
		return nil, "", fmt.Errorf("missing closing frontmatter delimiter")
	}

	frontmatterStr := str[4 : 4+endIdx]
	bodyStr := strings.TrimPrefix(str[4+endIdx+4:], "\n")

	var fm AgentFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterStr), &fm); err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}

	return &fm, bodyStr, nil
}

// builtinAgentFiles lists the embedded agent markdown files.
var builtinAgentFiles = []string{
	"agents/code-reviewer.md",
	"agents/code-simplifier.md",
	"agents/comment-analyzer.md",
	"agents/dependency-validator.md",
	"agents/pr-test-analyzer.md",
	"agents/silent-failure-hunter.md",
	"agents/type-design-analyzer.md",
}

// builtinPhaseAgents defines which agents run for which phases.
// Same sequence = parallel execution.
var builtinPhaseAgents = []db.PhaseAgent{
	// Review phase - parallel group 0 (all run concurrently)
	{PhaseTemplateID: "review", AgentID: "code-reviewer", Sequence: 0, Role: "guidelines", WeightFilter: []string{"small", "medium", "large"}, IsBuiltin: true},
	{PhaseTemplateID: "review", AgentID: "silent-failure-hunter", Sequence: 0, Role: "error-handling", WeightFilter: []string{"small", "medium", "large"}, IsBuiltin: true},
	{PhaseTemplateID: "review", AgentID: "pr-test-analyzer", Sequence: 0, Role: "test-coverage", WeightFilter: []string{"medium", "large"}, IsBuiltin: true},
	{PhaseTemplateID: "review", AgentID: "comment-analyzer", Sequence: 0, Role: "documentation", WeightFilter: []string{"medium", "large"}, IsBuiltin: true},
	{PhaseTemplateID: "review", AgentID: "type-design-analyzer", Sequence: 0, Role: "type-design", WeightFilter: []string{"large"}, IsBuiltin: true},

	// Implement phase - code-simplifier runs AFTER main implementation (sequence 1)
	{PhaseTemplateID: "implement", AgentID: "code-simplifier", Sequence: 1, Role: "simplifier", WeightFilter: []string{"medium", "large"}, IsBuiltin: true},

}

// SeedAgents populates the database with built-in agent definitions and phase associations.
// Reads agent markdown files from embedded templates and creates database records.
// Returns the number of items seeded (agents + phase associations).
func SeedAgents(gdb *db.GlobalDB) (int, error) {
	seeded := 0

	// Seed agent definitions from embedded files
	for _, file := range builtinAgentFiles {
		content, err := templates.Agents.ReadFile(file)
		if err != nil {
			return seeded, fmt.Errorf("read agent file %s: %w", file, err)
		}

		fm, prompt, err := ParseAgentMarkdown(content)
		if err != nil {
			return seeded, fmt.Errorf("parse agent file %s: %w", file, err)
		}

		// Check if already exists
		existing, err := gdb.GetAgent(fm.Name)
		if err != nil {
			return seeded, fmt.Errorf("check agent %s: %w", fm.Name, err)
		}
		if existing != nil {
			continue // Already seeded
		}

		agent := &db.Agent{
			ID:          fm.Name,
			Name:        fm.Name,
			Description: fm.Description,
			Prompt:      prompt,
			Tools:       fm.Tools,
			Model:       fm.Model,
			IsBuiltin:   true,
		}

		if err := gdb.SaveAgent(agent); err != nil {
			return seeded, fmt.Errorf("save agent %s: %w", fm.Name, err)
		}
		seeded++
	}

	// Seed phase-agent associations
	for _, pa := range builtinPhaseAgents {
		// Check if phase template exists (foreign key)
		pt, err := gdb.GetPhaseTemplate(pa.PhaseTemplateID)
		if err != nil {
			return seeded, fmt.Errorf("check phase template %s: %w", pa.PhaseTemplateID, err)
		}
		if pt == nil {
			// Phase template doesn't exist yet - skip for now
			// Will be created when SeedBuiltins is called
			continue
		}

		// Check if agent exists
		agent, err := gdb.GetAgent(pa.AgentID)
		if err != nil {
			return seeded, fmt.Errorf("check agent %s: %w", pa.AgentID, err)
		}
		if agent == nil {
			// Agent doesn't exist - should have been created above
			continue
		}

		// Check if association already exists
		existing, err := gdb.GetPhaseAgents(pa.PhaseTemplateID)
		if err != nil {
			return seeded, fmt.Errorf("check phase agents for %s: %w", pa.PhaseTemplateID, err)
		}

		alreadyExists := false
		for _, e := range existing {
			if e.AgentID == pa.AgentID {
				alreadyExists = true
				break
			}
		}

		if alreadyExists {
			continue
		}

		if err := gdb.SavePhaseAgent(&pa); err != nil {
			return seeded, fmt.Errorf("save phase agent %s/%s: %w", pa.PhaseTemplateID, pa.AgentID, err)
		}
		seeded++
	}

	return seeded, nil
}

// ListBuiltinAgentIDs returns all built-in agent IDs.
func ListBuiltinAgentIDs() []string {
	return []string{
		"code-reviewer",
		"code-simplifier",
		"comment-analyzer",
		"dependency-validator",
		"pr-test-analyzer",
		"silent-failure-hunter",
		"type-design-analyzer",
	}
}
