package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// maxAgentsMDBytes is the Codex CLI limit for AGENTS.md file size.
const maxAgentsMDBytes = 32 * 1024

// AgentsMDContent is the structured input for AGENTS.md generation.
// Fields are ordered by truncation priority: Constitution and PhaseContext
// are never truncated; ExtraInstructions are truncated first, then AgentPrompts.
type AgentsMDContent struct {
	Constitution      string
	PhaseContext      string
	AgentPrompts      []string
	ExtraInstructions string
}

// BuildAgentsMDContent creates an AgentsMDContent from individual fields.
func BuildAgentsMDContent(constitution, phaseContext string, agentPrompts []string, extraInstructions string) AgentsMDContent {
	return AgentsMDContent{
		Constitution:      constitution,
		PhaseContext:      phaseContext,
		AgentPrompts:      agentPrompts,
		ExtraInstructions: extraInstructions,
	}
}

// WriteAgentsMD writes an AGENTS.md file to the given directory.
// Codex CLI reads AGENTS.md from the worktree root for context (equivalent
// of CLAUDE.md + system prompt for Claude Code).
//
// If total content exceeds 32KB, sections are truncated in priority order:
//  1. ExtraInstructions truncated first
//  2. AgentPrompts truncated second
//  3. Constitution and PhaseContext are never truncated
func WriteAgentsMD(dir string, content AgentsMDContent) error {
	if dir == "" {
		return fmt.Errorf("write agents md: directory path is required")
	}

	// Build each section independently so we can measure and truncate.
	constitutionSection := buildSection("Constitution", content.Constitution)
	phaseSection := buildSection("Phase Context", content.PhaseContext)
	agentsSection := buildAgentsSection(content.AgentPrompts)
	extraSection := buildSection("Additional Instructions", content.ExtraInstructions)

	// Protected sections (never truncated)
	protectedSize := len(constitutionSection) + len(phaseSection)

	// Check if truncation is needed
	totalSize := protectedSize + len(agentsSection) + len(extraSection)
	if totalSize > maxAgentsMDBytes {
		budget := maxAgentsMDBytes - protectedSize
		if budget < 0 {
			// Protected content alone exceeds limit — write it anyway (best effort).
			budget = 0
		}

		// Truncate extra instructions first
		if len(extraSection)+len(agentsSection) > budget {
			extraBudget := budget - len(agentsSection)
			if extraBudget <= 0 {
				extraSection = ""
			} else {
				extraSection = extraSection[:extraBudget]
			}
		}

		// If still over, truncate agent prompts
		if len(agentsSection)+len(extraSection) > budget {
			agentsBudget := budget - len(extraSection)
			if agentsBudget <= 0 {
				agentsSection = ""
			} else {
				agentsSection = agentsSection[:agentsBudget]
			}
		}
	}

	text := constitutionSection + phaseSection + agentsSection + extraSection

	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		return fmt.Errorf("write agents md: %w", err)
	}
	return nil
}

// buildSection renders a titled markdown section. Returns empty string if
// the content is blank.
func buildSection(title, content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	return "# " + title + "\n\n" + trimmed + "\n\n"
}

// buildAgentsSection renders the agent prompts as numbered sub-sections.
func buildAgentsSection(prompts []string) string {
	if len(prompts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# Agent Prompts\n\n")
	for i, prompt := range prompts {
		trimmed := strings.TrimSpace(prompt)
		if trimmed == "" {
			continue
		}
		fmt.Fprintf(&b, "## Agent %d\n\n%s\n\n", i+1, trimmed)
	}
	return b.String()
}
