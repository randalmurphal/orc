// Package executor provides phase execution for workflows.
package executor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// SkillLoader loads skill content for injection into phase prompts.
type SkillLoader struct {
	claudeDir string
}

// NewSkillLoader creates a skill loader for the given .claude directory.
func NewSkillLoader(claudeDir string) *SkillLoader {
	return &SkillLoader{claudeDir: claudeDir}
}

// LoadSkillsContent loads the specified skills and returns their combined content.
// Skills are loaded from the .claude/skills/ directory.
// Returns empty string if no skills found or all skills fail to load.
func (l *SkillLoader) LoadSkillsContent(skillRefs []string) (string, error) {
	if len(skillRefs) == 0 {
		return "", nil
	}

	var content strings.Builder
	var errors []string

	for _, ref := range skillRefs {
		skillPath := filepath.Join(l.claudeDir, "skills", ref)
		skill, err := claudeconfig.ParseSkillMD(skillPath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("skill %q: %v", ref, err))
			continue
		}

		// Format skill content for injection
		content.WriteString("\n\n## Skill: ")
		content.WriteString(skill.Name)
		content.WriteString("\n")
		if skill.Description != "" {
			content.WriteString(skill.Description)
			content.WriteString("\n\n")
		}
		content.WriteString(skill.Content)
	}

	// Return combined content even if some skills failed
	// Log errors but don't fail the entire operation
	if len(errors) > 0 && content.Len() == 0 {
		return "", fmt.Errorf("failed to load any skills: %s", strings.Join(errors, "; "))
	}

	return content.String(), nil
}

// LoadSkillAllowedTools returns the combined allowed tools from the specified skills.
// This is used to merge skill tool restrictions into the phase config.
func (l *SkillLoader) LoadSkillAllowedTools(skillRefs []string) ([]string, error) {
	if len(skillRefs) == 0 {
		return nil, nil
	}

	seen := make(map[string]bool)
	var tools []string

	for _, ref := range skillRefs {
		skillPath := filepath.Join(l.claudeDir, "skills", ref)
		skill, err := claudeconfig.ParseSkillMD(skillPath)
		if err != nil {
			continue // Skip skills that fail to load
		}

		for _, tool := range skill.AllowedTools {
			if !seen[tool] {
				seen[tool] = true
				tools = append(tools, tool)
			}
		}
	}

	return tools, nil
}

// LoadSkillsForConfig loads skills specified in the config and modifies it in-place.
// - Appends skill content to AppendSystemPrompt
// - Merges skill AllowedTools into the config's AllowedTools
func (l *SkillLoader) LoadSkillsForConfig(cfg *PhaseClaudeConfig) error {
	if cfg == nil || len(cfg.SkillRefs) == 0 {
		return nil
	}

	// Load skill content
	content, err := l.LoadSkillsContent(cfg.SkillRefs)
	if err != nil {
		return fmt.Errorf("load skills content: %w", err)
	}

	// Append to system prompt
	if content != "" {
		if cfg.AppendSystemPrompt != "" {
			cfg.AppendSystemPrompt += content
		} else {
			cfg.AppendSystemPrompt = content
		}
	}

	// Load and merge allowed tools
	allowedTools, err := l.LoadSkillAllowedTools(cfg.SkillRefs)
	if err != nil {
		return fmt.Errorf("load skill allowed tools: %w", err)
	}

	if len(allowedTools) > 0 {
		// Merge with existing allowed tools
		seen := make(map[string]bool)
		for _, t := range cfg.AllowedTools {
			seen[t] = true
		}
		for _, t := range allowedTools {
			if !seen[t] {
				cfg.AllowedTools = append(cfg.AllowedTools, t)
				seen[t] = true
			}
		}
	}

	return nil
}

// LoadSkillsContentSimple is a convenience function for loading skills without a SkillLoader instance.
// claudeDir should be the path to the .claude directory.
func LoadSkillsContentSimple(claudeDir string, skillRefs []string) (string, error) {
	loader := NewSkillLoader(claudeDir)
	return loader.LoadSkillsContent(skillRefs)
}
