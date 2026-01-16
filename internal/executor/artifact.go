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
