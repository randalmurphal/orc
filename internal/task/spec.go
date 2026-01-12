package task

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

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

// SpecPath returns the path to the spec file for a task.
func SpecPath(taskID string) string {
	return filepath.Join(OrcDir, TasksDir, taskID, "spec.md")
}

// SpecPathIn returns the path to the spec file for a task in a specific directory.
func SpecPathIn(workDir, taskID string) string {
	return filepath.Join(workDir, OrcDir, TasksDir, taskID, "spec.md")
}

// SpecMetadataPath returns the path to the spec metadata file.
func SpecMetadataPath(taskID string) string {
	return filepath.Join(OrcDir, TasksDir, taskID, "spec_meta.yaml")
}

// LoadSpec loads a spec from the task directory.
func LoadSpec(taskID string) (*Spec, error) {
	specPath := SpecPath(taskID)
	content, err := os.ReadFile(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No spec yet
		}
		return nil, fmt.Errorf("read spec: %w", err)
	}

	spec := &Spec{
		Content: string(content),
	}

	// Load metadata if it exists
	metaPath := SpecMetadataPath(taskID)
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		yaml.Unmarshal(metaData, &spec.Metadata)
	}

	return spec, nil
}

// SaveSpec saves a spec to the task directory.
func SaveSpec(taskID, content, source string) error {
	taskDir := filepath.Join(OrcDir, TasksDir, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	// Write spec content
	specPath := SpecPath(taskID)
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}

	// Write metadata
	meta := SpecMetadata{
		Hash:         HashContent(content),
		LastSyncedAt: time.Now(),
		Source:       source,
	}
	metaPath := SpecMetadataPath(taskID)
	metaData, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal spec metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("write spec metadata: %w", err)
	}

	return nil
}

// SpecExists returns true if a spec exists for the task.
func SpecExists(taskID string) bool {
	_, err := os.Stat(SpecPath(taskID))
	return err == nil
}

// HashContent computes SHA-256 hash of content.
func HashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// SpecChanged checks if spec content has changed since last sync.
func SpecChanged(taskID, newContent string) bool {
	spec, err := LoadSpec(taskID)
	if err != nil || spec == nil {
		return true // No existing spec, so it's "changed"
	}

	newHash := HashContent(newContent)
	return spec.Metadata.Hash != newHash
}

// LoadSpecFrom loads a spec from a specific directory.
func LoadSpecFrom(taskDir string) (*Spec, error) {
	specPath := filepath.Join(taskDir, "spec.md")
	content, err := os.ReadFile(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read spec: %w", err)
	}

	spec := &Spec{
		Content: string(content),
	}

	metaPath := filepath.Join(taskDir, "spec_meta.yaml")
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		yaml.Unmarshal(metaData, &spec.Metadata)
	}

	return spec, nil
}

// SaveSpecTo saves a spec to a specific directory.
func SaveSpecTo(taskDir, content, source string) error {
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	specPath := filepath.Join(taskDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}

	meta := SpecMetadata{
		Hash:         HashContent(content),
		LastSyncedAt: time.Now(),
		Source:       source,
	}
	metaPath := filepath.Join(taskDir, "spec_meta.yaml")
	metaData, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal spec metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("write spec metadata: %w", err)
	}

	return nil
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
func ValidateSpec(content string, weight Weight) *SpecValidation {
	result := &SpecValidation{
		Valid:  true,
		Issues: []string{},
	}

	// Trivial tasks skip validation entirely
	if weight == WeightTrivial {
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

// HasValidSpec checks if a task has a valid spec file.
func HasValidSpec(taskID string, weight Weight) bool {
	spec, err := LoadSpec(taskID)
	if err != nil || spec == nil {
		return false
	}

	result := ValidateSpec(spec.Content, weight)
	return result.Valid
}

// HasValidSpecIn checks if a task has a valid spec file in a specific directory.
func HasValidSpecIn(workDir, taskID string, weight Weight) bool {
	taskDir := filepath.Join(workDir, OrcDir, TasksDir, taskID)
	spec, err := LoadSpecFrom(taskDir)
	if err != nil || spec == nil {
		return false
	}

	result := ValidateSpec(spec.Content, weight)
	return result.Valid
}

// LoadSpecIn loads a spec from a task in a specific working directory.
func LoadSpecIn(workDir, taskID string) (*Spec, error) {
	taskDir := filepath.Join(workDir, OrcDir, TasksDir, taskID)
	return LoadSpecFrom(taskDir)
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
