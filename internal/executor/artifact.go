// Package executor provides task phase execution for orc.
package executor

import (
	"fmt"
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
// SpecExtractionError provides details about why spec extraction failed
type SpecExtractionError struct {
	Reason            string
	OutputLen         int
	OutputPreview     string // First 200 chars of output for debugging
	SpecPath          string
	FileExists        bool
	FileSize          int64 // Size of spec.md if it exists
	FileReadErr       error
	ValidationFailure string // Specific reason why isValidSpecContent failed
}

func (e *SpecExtractionError) Error() string {
	var b strings.Builder
	b.WriteString(e.Reason)

	// Add diagnostic details
	fmt.Fprintf(&b, "\n  output_length: %d bytes", e.OutputLen)
	if e.OutputPreview != "" {
		fmt.Fprintf(&b, "\n  output_preview: %q", e.OutputPreview)
	}
	if e.SpecPath != "" {
		fmt.Fprintf(&b, "\n  spec_path: %s", e.SpecPath)
		fmt.Fprintf(&b, "\n  file_exists: %v", e.FileExists)
		if e.FileExists && e.FileSize > 0 {
			fmt.Fprintf(&b, "\n  file_size: %d bytes", e.FileSize)
		}
		if e.FileReadErr != nil {
			fmt.Fprintf(&b, "\n  file_read_error: %v", e.FileReadErr)
		}
	}
	if e.ValidationFailure != "" {
		fmt.Fprintf(&b, "\n  validation_failure: %s", e.ValidationFailure)
	}

	return b.String()
}

func SaveSpecToDatabase(backend storage.Backend, taskID, phaseID, output string, worktreePath ...string) (bool, error) {
	// Only save for spec phase
	if phaseID != "spec" {
		return false, nil
	}

	if backend == nil {
		// This shouldn't happen in production - return error for visibility
		return false, fmt.Errorf("backend is nil - cannot save spec")
	}

	// Helper to get first N chars of output for preview
	outputPreview := func(s string, maxLen int) string {
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen] + "..."
	}

	// Extract the spec content from the output using artifact tags or structured markers
	specContent := extractArtifact(output)
	var specPath string
	var fileExists bool
	var fileSize int64
	var fileReadErr error

	// If no artifact tags found, check for spec file in multiple locations
	// Agents sometimes write spec files instead of using artifact tags
	if specContent == "" && len(worktreePath) > 0 && worktreePath[0] != "" {
		// Try legacy location first: .orc/tasks/TASK-XXX/spec.md
		specPath = task.SpecPathIn(worktreePath[0], taskID)
		if info, err := os.Stat(specPath); err == nil {
			fileExists = true
			fileSize = info.Size()
			if content, err := os.ReadFile(specPath); err == nil && len(content) > 0 {
				specContent = strings.TrimSpace(string(content))
			} else if err != nil {
				fileReadErr = err
			}
		} else if !os.IsNotExist(err) {
			// Stat failed but not because file doesn't exist
			fileReadErr = err
		}

		// If still not found, try .orc/specs/TASK-XXX.md location
		if specContent == "" {
			altSpecPath := filepath.Join(worktreePath[0], ".orc", "specs", taskID+".md")
			if info, err := os.Stat(altSpecPath); err == nil {
				specPath = altSpecPath // Update for error reporting
				fileExists = true
				fileSize = info.Size()
				if content, err := os.ReadFile(altSpecPath); err == nil && len(content) > 0 {
					specContent = strings.TrimSpace(string(content))
				} else if err != nil {
					fileReadErr = err
				}
			}
		}
	}

	if specContent == "" {
		// No structured spec content found - return detailed error for diagnostics
		return false, &SpecExtractionError{
			Reason:        "no spec content found in output or file",
			OutputLen:     len(output),
			OutputPreview: outputPreview(output, 200),
			SpecPath:      specPath,
			FileExists:    fileExists,
			FileSize:      fileSize,
			FileReadErr:   fileReadErr,
		}
	}

	// Validate that the spec content looks like a valid spec
	// A valid spec should have meaningful content and not just completion markers
	if validationFailure := validateSpecContent(specContent); validationFailure != "" {
		return false, &SpecExtractionError{
			Reason:            "spec content failed validation",
			OutputLen:         len(output),
			OutputPreview:     outputPreview(output, 200),
			SpecPath:          specPath,
			FileExists:        fileExists,
			FileSize:          fileSize,
			ValidationFailure: validationFailure,
		}
	}

	// Save to database with source indicating it came from execution
	if err := backend.SaveSpec(taskID, specContent, "executor"); err != nil {
		return false, fmt.Errorf("database save failed: %w", err)
	}

	return true, nil
}

// isValidSpecContent validates that spec content is meaningful and not just noise.
// A valid spec should:
// - Have a minimum length (50 chars)
// - Not consist primarily of completion markers
// - Ideally have at least one spec-like section (Intent, Success Criteria, etc.)
func isValidSpecContent(content string) bool {
	reason := validateSpecContent(content)
	return reason == ""
}

// validateSpecContent checks if spec content is valid and returns an empty string
// if valid, or a description of why validation failed.
func validateSpecContent(content string) string {
	trimmed := strings.TrimSpace(content)

	// Minimum length check - a real spec should have at least some content
	if len(trimmed) < 50 {
		return fmt.Sprintf("content too short (%d chars, need at least 50)", len(trimmed))
	}

	lowerContent := strings.ToLower(trimmed)

	// Reject content that is primarily completion markers or noise
	noisePatterns := []string{
		`"status": "complete"`,
		`"status": "blocked"`,
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
				return fmt.Sprintf("noise pattern detected (%q) with only %d chars of content before it (need 50)", noise, len(beforeNoise))
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
			return "" // Valid: has spec section
		}
	}

	// If no recognized spec sections, require longer content (200 chars)
	// to avoid accepting random garbage
	if len(trimmed) >= 200 {
		return "" // Valid: long enough without sections
	}

	return fmt.Sprintf("no recognized spec sections (intent, success criteria, etc.) and content too short (%d chars, need 200 without sections)", len(trimmed))
}
