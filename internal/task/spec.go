// Package task provides task management for orc.
// Note: File I/O functions have been removed. Use storage.Backend for persistence.
package task

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// Note: time is still used by SpecMetadata

// SpecMetadata holds metadata about a task specification.
type SpecMetadata struct {
	// Hash is the SHA-256 hash of the spec content
	Hash string `yaml:"hash"`
	// LastSyncedAt is when the spec was last synced
	LastSyncedAt time.Time `yaml:"last_synced_at"`
	// Source indicates where the spec came from (file, db, generated)
	Source string `yaml:"source,omitempty"`
}

// Spec represents a task specification with content and metadata.
type Spec struct {
	// Content is the markdown content of the spec
	Content string `yaml:"-"`
	// Metadata holds sync metadata
	Metadata SpecMetadata `yaml:"metadata"`
}

// HashContent computes SHA-256 hash of content.
func HashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// SpecValidation contains the outcome of spec validation.
type SpecValidation struct {
	// Valid is true if the spec meets all requirements.
	Valid bool

	// HasIntent is true if the Intent section exists and has content.
	HasIntent bool

	// HasSuccessCriteria is true if Success Criteria section exists with items.
	HasSuccessCriteria bool

	// HasTesting is true if Testing section exists with content.
	HasTesting bool

	// Issues contains specific validation issues found.
	Issues []string
}

// ValidateSpec validates a spec against requirements based on task weight.
// Trivial tasks skip validation entirely.
func ValidateSpec(content string, weight orcv1.TaskWeight) *SpecValidation {
	result := &SpecValidation{
		Valid:  true,
		Issues: []string{},
	}

	// Trivial tasks skip validation entirely
	if weight == orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL {
		result.HasIntent = true
		result.HasSuccessCriteria = true
		result.HasTesting = true
		return result
	}

	// Check for Intent section
	if hasSection(content, "intent") {
		intentContent := extractSection(content, "intent")
		if hasMeaningfulContent(intentContent) {
			result.HasIntent = true
		} else {
			result.Issues = append(result.Issues, "Intent section exists but is empty")
			result.Valid = false
		}
	} else {
		result.Issues = append(result.Issues, "Missing Intent section")
		result.Valid = false
	}

	// Check for Success Criteria section
	if hasSection(content, "success criteria") {
		criteriaContent := extractSection(content, "success criteria")
		if hasMeaningfulContent(criteriaContent) {
			result.HasSuccessCriteria = true
		} else {
			result.Issues = append(result.Issues, "Success Criteria section exists but has no items")
			result.Valid = false
		}
	} else {
		result.Issues = append(result.Issues, "Missing Success Criteria section")
		result.Valid = false
	}

	// Check for Testing section
	if hasSection(content, "testing") {
		testingContent := extractSection(content, "testing")
		if hasMeaningfulContent(testingContent) {
			result.HasTesting = true
		} else {
			result.Issues = append(result.Issues, "Testing section exists but is empty")
			result.Valid = false
		}
	} else {
		result.Issues = append(result.Issues, "Missing Testing section")
		result.Valid = false
	}

	return result
}

// hasSection checks if a markdown section with the given name exists.
func hasSection(content, sectionName string) bool {
	pattern := regexp.MustCompile(`(?im)^##?\s*` + regexp.QuoteMeta(sectionName))
	return pattern.MatchString(content)
}

// extractSection extracts content from a markdown section.
// Returns content between the section header and the next header (or end of file).
func extractSection(content, sectionName string) string {
	// Find the section header
	pattern := regexp.MustCompile(`(?im)^(##?\s*` + regexp.QuoteMeta(sectionName) + `[^\n]*)\n`)
	loc := pattern.FindStringIndex(content)
	if loc == nil {
		return ""
	}

	// Find the start of content (after header)
	startIdx := loc[1]
	remaining := content[startIdx:]

	// Find the next section header (## or #)
	nextHeader := regexp.MustCompile(`(?m)^##?\s+\w`)
	nextLoc := nextHeader.FindStringIndex(remaining)

	if nextLoc != nil {
		return strings.TrimSpace(remaining[:nextLoc[0]])
	}

	return strings.TrimSpace(remaining)
}

// hasMeaningfulContent checks if a section has meaningful content (not just whitespace).
func hasMeaningfulContent(s string) bool {
	trimmed := strings.TrimSpace(s)
	// Must have at least some non-trivial content
	return len(trimmed) > 10
}
