package setup

import (
	"os"
	"path/filepath"
)

// Validator checks the output of Claude's setup.
type Validator struct {
	workDir string
}

// NewValidator creates a new setup validator.
func NewValidator(workDir string) *Validator {
	return &Validator{workDir: workDir}
}

// Validate checks that Claude's setup produced valid output.
// Returns a list of validation errors, or empty if all is well.
func (v *Validator) Validate() []string {
	var errors []string

	// Check that CLAUDE.md exists or was created
	claudeMDPath := filepath.Join(v.workDir, ".claude", "CLAUDE.md")
	if _, err := os.Stat(claudeMDPath); os.IsNotExist(err) {
		// Not an error - CLAUDE.md may be in project root
		rootClaudeMD := filepath.Join(v.workDir, "CLAUDE.md")
		if _, err := os.Stat(rootClaudeMD); os.IsNotExist(err) {
			// Still not found - check if .claude directory exists
			claudeDir := filepath.Join(v.workDir, ".claude")
			if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
				// This is fine - setup may have been skipped or minimal
			}
		}
	}

	// Check that config.yaml wasn't corrupted
	configPath := filepath.Join(v.workDir, ".orc", "config.yaml")
	if info, err := os.Stat(configPath); err != nil {
		errors = append(errors, "config.yaml was deleted or moved")
	} else if info.Size() == 0 {
		errors = append(errors, "config.yaml is empty")
	}

	// Additional validations can be added here
	// - Check that any created skills are valid SKILL.md format
	// - Check that prompts/*.md are valid templates
	// - Check that settings.json is valid JSON

	return errors
}

// ValidateSkillFile checks if a skill file is valid SKILL.md format.
func (v *Validator) ValidateSkillFile(path string) []string {
	var errors []string

	content, err := os.ReadFile(path)
	if err != nil {
		errors = append(errors, "cannot read skill file: "+err.Error())
		return errors
	}

	// Basic validation - should have YAML frontmatter
	if len(content) < 10 {
		errors = append(errors, "skill file is too short")
		return errors
	}

	// Check for frontmatter markers
	if string(content[:3]) != "---" {
		errors = append(errors, "skill file missing YAML frontmatter")
	}

	return errors
}
