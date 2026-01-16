// Package executor provides task phase execution for orc.
package executor

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

const (
	// maxArtifactFileSize is the maximum size we'll read for artifact detection.
	// This prevents memory exhaustion from maliciously large files.
	maxArtifactFileSize = 5 * 1024 * 1024 // 5 MB

	// minMeaningfulContent is the minimum content length to consider an artifact valid.
	minMeaningfulContent = 50
)

// ArtifactDetector checks for existing phase artifacts.
type ArtifactDetector struct {
	taskDir string
	taskID  string
	weight  task.Weight
	backend storage.Backend // Optional: used for spec detection from database
}

// NewArtifactDetector creates a detector for a task.
func NewArtifactDetector(taskID string, weight task.Weight) *ArtifactDetector {
	return &ArtifactDetector{
		taskDir: task.TaskDir(taskID),
		taskID:  taskID,
		weight:  weight,
	}
}

// NewArtifactDetectorWithDir creates a detector for a task in a specific directory.
func NewArtifactDetectorWithDir(taskDir, taskID string, weight task.Weight) *ArtifactDetector {
	return &ArtifactDetector{
		taskDir: taskDir,
		taskID:  taskID,
		weight:  weight,
	}
}

// NewArtifactDetectorWithBackend creates a detector with database backend for spec detection.
// This is the preferred constructor as it enables spec detection from the database
// (specs are not stored as file artifacts to avoid merge conflicts).
func NewArtifactDetectorWithBackend(taskDir, taskID string, weight task.Weight, backend storage.Backend) *ArtifactDetector {
	return &ArtifactDetector{
		taskDir: taskDir,
		taskID:  taskID,
		weight:  weight,
		backend: backend,
	}
}

// readFileLimited reads a file up to maxArtifactFileSize bytes.
// Returns the content or an error if the file is too large or unreadable.
func readFileLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Check file size first
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > maxArtifactFileSize {
		return nil, io.ErrUnexpectedEOF // File too large
	}

	return io.ReadAll(io.LimitReader(f, maxArtifactFileSize))
}

// ArtifactStatus represents what artifacts exist for a phase.
type ArtifactStatus struct {
	// PhaseID is the phase being checked.
	PhaseID string

	// HasArtifacts is true if relevant artifacts exist.
	HasArtifacts bool

	// Artifacts lists the detected artifact paths (relative to task dir).
	Artifacts []string

	// Description is a human-readable description of what was found.
	Description string

	// CanAutoSkip is true if this phase can be safely auto-skipped.
	// Some phases produce artifacts but shouldn't be auto-skipped.
	CanAutoSkip bool
}

// DetectPhaseArtifacts checks if artifacts exist for a phase.
func (d *ArtifactDetector) DetectPhaseArtifacts(phaseID string) *ArtifactStatus {
	switch phaseID {
	case "spec":
		return d.detectSpecArtifacts()
	case "research":
		return d.detectResearchArtifacts()
	case "implement":
		return d.detectImplementArtifacts()
	case "test":
		return d.detectTestArtifacts()
	case "docs":
		return d.detectDocsArtifacts()
	case "validate":
		return d.detectValidateArtifacts()
	default:
		return &ArtifactStatus{
			PhaseID:      phaseID,
			HasArtifacts: false,
			Description:  "unknown phase",
		}
	}
}

// detectSpecArtifacts checks if spec exists in the database.
// Spec content is stored exclusively in the database (not as file artifacts)
// to avoid merge conflicts in worktrees.
func (d *ArtifactDetector) detectSpecArtifacts() *ArtifactStatus {
	status := &ArtifactStatus{
		PhaseID: "spec",
	}

	// Try to load spec from database (preferred source)
	if d.backend != nil && d.taskID != "" {
		specContent, err := d.backend.LoadSpec(d.taskID)
		if err == nil && specContent != "" {
			// Check if spec has meaningful content
			if len(strings.TrimSpace(specContent)) < minMeaningfulContent {
				status.Description = "spec exists in database but appears empty or minimal"
				return status
			}

			// Validate spec content based on weight
			validation := task.ValidateSpec(specContent, d.weight)
			if !validation.Valid {
				status.HasArtifacts = true
				status.Artifacts = []string{"database:spec"}
				status.Description = "spec exists in database but incomplete: " + strings.Join(validation.Issues, ", ")
				status.CanAutoSkip = false // Don't auto-skip invalid specs
				return status
			}

			status.HasArtifacts = true
			status.Artifacts = []string{"database:spec"}
			status.Description = "spec exists in database with valid content"
			status.CanAutoSkip = true
			return status
		}
	}

	// Fallback: check for legacy spec.md file (for backward compatibility)
	specPath := filepath.Join(d.taskDir, "spec.md")
	content, err := readFileLimited(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			status.Description = "no spec found (checked database and spec.md)"
		} else {
			status.Description = "spec.md exists but unreadable"
		}
		return status
	}

	// Check if spec has meaningful content
	contentStr := string(content)
	if len(strings.TrimSpace(contentStr)) < minMeaningfulContent {
		status.Description = "spec.md exists but appears empty or minimal"
		return status
	}

	// Validate spec content based on weight
	validation := task.ValidateSpec(contentStr, d.weight)
	if !validation.Valid {
		status.HasArtifacts = true
		status.Artifacts = []string{"spec.md"}
		status.Description = "spec.md exists but incomplete: " + strings.Join(validation.Issues, ", ")
		status.CanAutoSkip = false // Don't auto-skip invalid specs
		return status
	}

	status.HasArtifacts = true
	status.Artifacts = []string{"spec.md"}
	status.Description = "spec.md exists with valid content (legacy file)"
	status.CanAutoSkip = true
	return status
}

// detectResearchArtifacts checks if research artifacts exist.
func (d *ArtifactDetector) detectResearchArtifacts() *ArtifactStatus {
	status := &ArtifactStatus{
		PhaseID: "research",
	}

	// Check for research artifact file
	researchPath := filepath.Join(d.taskDir, "artifacts", "research.md")
	if content, err := readFileLimited(researchPath); err == nil {
		if len(strings.TrimSpace(string(content))) > minMeaningfulContent {
			status.HasArtifacts = true
			status.Artifacts = []string{"artifacts/research.md"}
			status.Description = "research.md artifact exists"
			status.CanAutoSkip = true
			return status
		}
	}

	// Also check for research content in spec.md (sometimes embedded)
	specPath := filepath.Join(d.taskDir, "spec.md")
	if content, err := readFileLimited(specPath); err == nil {
		contentStr := strings.ToLower(string(content))
		if strings.Contains(contentStr, "## research") || strings.Contains(contentStr, "# research") {
			status.HasArtifacts = true
			status.Artifacts = []string{"spec.md (contains research section)"}
			status.Description = "research content found in spec.md"
			status.CanAutoSkip = true
			return status
		}
	}

	status.Description = "no research artifacts found"
	return status
}

// detectImplementArtifacts checks if implementation appears complete.
// This is the hardest phase to auto-detect since "implementation" varies widely.
func (d *ArtifactDetector) detectImplementArtifacts() *ArtifactStatus {
	status := &ArtifactStatus{
		PhaseID:     "implement",
		Description: "implement phase cannot be reliably auto-detected",
		// Never auto-skip implement phase - too risky
		CanAutoSkip: false,
	}

	// Could potentially check for:
	// - Git commits since last phase
	// - Code changes matching spec requirements
	// But this is too complex and error-prone to auto-skip

	return status
}

// detectTestArtifacts checks if test artifacts exist.
func (d *ArtifactDetector) detectTestArtifacts() *ArtifactStatus {
	status := &ArtifactStatus{
		PhaseID: "test",
	}

	// Check for test results in task directory
	testResultsPath := filepath.Join(d.taskDir, "test-results")
	if info, err := os.Stat(testResultsPath); err == nil && info.IsDir() {
		// Check for report.json or similar
		reportPath := filepath.Join(testResultsPath, "report.json")
		if _, err := os.Stat(reportPath); err == nil {
			status.HasArtifacts = true
			status.Artifacts = []string{"test-results/report.json"}
			status.Description = "test results exist"
			// Don't auto-skip tests - they should be re-run to validate current code
			status.CanAutoSkip = false
			return status
		}
	}

	// Check for artifacts/test.md from previous run
	testArtifactPath := filepath.Join(d.taskDir, "artifacts", "test.md")
	if _, err := os.Stat(testArtifactPath); err == nil {
		status.HasArtifacts = true
		status.Artifacts = []string{"artifacts/test.md"}
		status.Description = "test phase artifact exists from previous run"
		// Still don't auto-skip - tests should validate current code state
		status.CanAutoSkip = false
		return status
	}

	status.Description = "no test artifacts found"
	return status
}

// detectDocsArtifacts checks if documentation artifacts exist.
func (d *ArtifactDetector) detectDocsArtifacts() *ArtifactStatus {
	status := &ArtifactStatus{
		PhaseID: "docs",
	}

	// Check for docs artifact
	docsArtifactPath := filepath.Join(d.taskDir, "artifacts", "docs.md")
	if _, err := os.Stat(docsArtifactPath); err == nil {
		status.HasArtifacts = true
		status.Artifacts = []string{"artifacts/docs.md"}
		status.Description = "docs phase artifact exists"
		status.CanAutoSkip = true
		return status
	}

	status.Description = "no docs artifacts found"
	return status
}

// detectValidateArtifacts checks if validation artifacts exist.
func (d *ArtifactDetector) detectValidateArtifacts() *ArtifactStatus {
	status := &ArtifactStatus{
		PhaseID: "validate",
	}

	// Check for validate artifact
	validateArtifactPath := filepath.Join(d.taskDir, "artifacts", "validate.md")
	if _, err := os.Stat(validateArtifactPath); err == nil {
		status.HasArtifacts = true
		status.Artifacts = []string{"artifacts/validate.md"}
		status.Description = "validate phase artifact exists"
		// Don't auto-skip validation - it should re-validate current state
		status.CanAutoSkip = false
		return status
	}

	status.Description = "no validation artifacts found"
	return status
}

// DetectAllPhaseArtifacts checks all phases in a plan.
func (d *ArtifactDetector) DetectAllPhaseArtifacts(phaseIDs []string) map[string]*ArtifactStatus {
	results := make(map[string]*ArtifactStatus)
	for _, phaseID := range phaseIDs {
		results[phaseID] = d.DetectPhaseArtifacts(phaseID)
	}
	return results
}

// SuggestSkippablePhases returns phases that have artifacts and can be safely skipped.
func (d *ArtifactDetector) SuggestSkippablePhases(phaseIDs []string) []string {
	var skippable []string
	for _, phaseID := range phaseIDs {
		status := d.DetectPhaseArtifacts(phaseID)
		if status.HasArtifacts && status.CanAutoSkip {
			skippable = append(skippable, phaseID)
		}
	}
	return skippable
}
