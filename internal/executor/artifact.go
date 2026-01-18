// Package executor provides task phase execution for orc.
package executor

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// SavePhaseArtifact extracts and saves artifact content from phase output.
// Returns the path to the saved artifact file, or empty string if no artifact found.
//
// NOTE: For the "spec" phase, this function returns early without writing to the
// filesystem. Spec content is saved exclusively to the database via SaveSpecToDatabase
// to avoid merge conflicts in worktrees. Use SaveSpecToDatabase for spec phase output.
func SavePhaseArtifact(taskID, phaseID, output string) (string, error) {
	// Skip file writing for spec phase - use database only via SaveSpecToDatabase
	if phaseID == "spec" {
		return "", nil
	}

	artifact := extractArtifact(output)
	if artifact == "" {
		return "", nil // No artifact to save
	}

	taskDir := task.TaskDir(taskID)
	artifactDir := filepath.Join(taskDir, "artifacts")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return "", err
	}

	path := filepath.Join(artifactDir, phaseID+".md")
	if err := os.WriteFile(path, []byte(artifact), 0644); err != nil {
		return "", err
	}

	return path, nil
}

// ExtractArtifactContent extracts artifact content from raw output without saving.
// This is useful for getting the artifact content for template variables.
func ExtractArtifactContent(output string) string {
	return extractArtifact(output)
}

// SaveSpecToDatabase saves spec content to the database for the spec phase.
// This is the SOLE mechanism for saving spec content - no file artifacts are created
// for the spec phase to avoid merge conflicts in worktrees. The database is the source
// of truth for spec content, which is loaded via backend.LoadSpec() to populate
// {{SPEC_CONTENT}} in subsequent phase templates.
//
// The worktreePath parameter is optional - if provided, we check for spec.md files
// that agents may have written instead of using artifact tags.
//
// This should be called after a successful spec phase completion.
// Returns true if the spec was saved, false if the phase is not "spec" or no content found.
func SaveSpecToDatabase(backend storage.Backend, taskID, phaseID, output string, worktreePath ...string) (bool, error) {
	// Only save for spec phase
	if phaseID != "spec" {
		return false, nil
	}

	if backend == nil {
		return false, nil
	}

	// Extract the spec content from the output using artifact tags or structured markers
	specContent := extractArtifact(output)

	// If no artifact tags found, check for spec file in task directory
	// Agents sometimes write spec.md files instead of using artifact tags
	if specContent == "" && len(worktreePath) > 0 && worktreePath[0] != "" {
		specPath := task.SpecPathIn(worktreePath[0], taskID)
		if content, err := os.ReadFile(specPath); err == nil && len(content) > 0 {
			specContent = strings.TrimSpace(string(content))
		}
	}

	if specContent == "" {
		// No structured spec content found - don't save raw output as it may contain
		// completion markers or other noise that isn't a valid spec
		return false, nil
	}

	// Validate that the spec content looks like a valid spec
	// A valid spec should have meaningful content and not just completion markers
	if !isValidSpecContent(specContent) {
		return false, nil
	}

	// Save to database with source indicating it came from execution
	if err := backend.SaveSpec(taskID, specContent, "executor"); err != nil {
		return false, err
	}

	return true, nil
}

// isValidSpecContent validates that spec content is meaningful and not just noise.
// A valid spec should:
// - Have a minimum length (50 chars)
// - Not consist primarily of completion markers
// - Ideally have at least one spec-like section (Intent, Success Criteria, etc.)
func isValidSpecContent(content string) bool {
	trimmed := strings.TrimSpace(content)

	// Minimum length check - a real spec should have at least some content
	if len(trimmed) < 50 {
		return false
	}

	lowerContent := strings.ToLower(trimmed)

	// Reject content that is primarily completion markers or noise
	noisePatterns := []string{
		"<phase_complete>",
		"<phase_blocked>",
		"the working tree is clean",
		"the spec was created as output in this conversation",
		"spec is in conversation output",
		"n/a (spec is in conversation",
	}

	for _, noise := range noisePatterns {
		if strings.Contains(lowerContent, noise) {
			// If noise pattern is found, check if there's meaningful content before it
			noiseIdx := strings.Index(lowerContent, noise)
			beforeNoise := strings.TrimSpace(trimmed[:noiseIdx])
			// Need at least 50 meaningful chars before the noise
			if len(beforeNoise) < 50 {
				return false
			}
		}
	}

	// Check for at least one spec-like section header (case insensitive)
	specSections := []string{
		"intent",
		"success criteria",
		"testing",
		"scope",
		"requirements",
		"approach",
		"technical",
		"acceptance",
		"specification",
		"overview",
		"background",
	}

	for _, section := range specSections {
		if strings.Contains(lowerContent, section) {
			return true
		}
	}

	// If no recognized spec sections, require longer content (200 chars)
	// to avoid accepting random garbage
	return len(trimmed) >= 200
}
