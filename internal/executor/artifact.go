// Package executor provides task phase execution for orc.
package executor

import (
	"os"
	"path/filepath"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// SavePhaseArtifact extracts and saves artifact content from phase output.
// Returns the path to the saved artifact file, or empty string if no artifact found.
func SavePhaseArtifact(taskID, phaseID, output string) (string, error) {
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
// This implements the dual-write pattern: spec content is saved to both the file artifact
// (via SavePhaseArtifact) AND the database (via this function). The database copy is
// required for template variable substitution - implement phase loads spec via LoadSpec()
// to populate {{SPEC_CONTENT}}.
//
// This should be called after a successful spec phase completion.
// Returns true if the spec was saved, false if the phase is not "spec" or no content found.
func SaveSpecToDatabase(backend storage.Backend, taskID, phaseID, output string) (bool, error) {
	// Only save for spec phase
	if phaseID != "spec" {
		return false, nil
	}

	if backend == nil {
		return false, nil
	}

	// Extract the spec content from the output
	specContent := extractArtifact(output)
	if specContent == "" {
		// If no artifact tags, try using the raw output (trimmed)
		// Some spec outputs may not use artifact tags
		specContent = output
	}

	if specContent == "" {
		return false, nil
	}

	// Save to database with source indicating it came from execution
	if err := backend.SaveSpec(taskID, specContent, "executor"); err != nil {
		return false, err
	}

	return true, nil
}
