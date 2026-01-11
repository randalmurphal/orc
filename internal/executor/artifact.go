// Package executor provides task phase execution for orc.
package executor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

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

// extractPhaseArtifact is a more comprehensive extraction that looks for phase-specific markers.
func extractPhaseArtifact(output, phaseID string) string {
	// First try the generic artifact extraction
	if artifact := extractArtifact(output); artifact != "" {
		return artifact
	}

	// Try phase-specific markers
	switch phaseID {
	case "spec":
		return extractSpecArtifact(output)
	case "research":
		return extractResearchArtifact(output)
	case "design":
		return extractDesignArtifact(output)
	case "implement":
		return extractImplementArtifact(output)
	}

	return ""
}

// extractSpecArtifact extracts specification content.
func extractSpecArtifact(output string) string {
	patterns := []string{
		`(?s)## Specification\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Spec\s*\n(.*?)(?:\n##|$)`,
		`(?s)### Success Criteria\s*\n(.*?)(?:\n###|$)`,
		`(?s)spec_complete:\s*\n(.*?)(?:\n\n|$)`,
	}
	return tryPatterns(output, patterns)
}

// extractResearchArtifact extracts research findings.
func extractResearchArtifact(output string) string {
	patterns := []string{
		`(?s)## Research Results\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Research Findings\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Analysis\s*\n(.*?)(?:\n##|$)`,
	}
	return tryPatterns(output, patterns)
}

// extractDesignArtifact extracts design documentation.
func extractDesignArtifact(output string) string {
	patterns := []string{
		`(?s)## Design\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Architecture\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Technical Design\s*\n(.*?)(?:\n##|$)`,
	}
	return tryPatterns(output, patterns)
}

// extractImplementArtifact extracts implementation summary.
func extractImplementArtifact(output string) string {
	patterns := []string{
		`(?s)## Implementation Summary\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Changes Made\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Files Modified\s*\n(.*?)(?:\n##|$)`,
	}
	return tryPatterns(output, patterns)
}

// tryPatterns tries each pattern and returns the first match.
func tryPatterns(content string, patterns []string) string {
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(content); len(m) >= 2 {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}
