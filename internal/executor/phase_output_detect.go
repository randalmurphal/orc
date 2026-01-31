// Package executor provides task phase execution for orc.
package executor

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

const (
	// maxPhaseOutputFileSize is the maximum size we'll read for phase output detection.
	// This prevents memory exhaustion from maliciously large files.
	maxPhaseOutputFileSize = 5 * 1024 * 1024 // 5 MB

	// minMeaningfulContent is the minimum content length to consider an output valid.
	minMeaningfulContent = 50
)

// PhaseOutputDetector checks for existing phase outputs.
type PhaseOutputDetector struct {
	taskDir string
	taskID  string
	weight  orcv1.TaskWeight
	backend storage.Backend // Optional: used for spec detection from database
}

// NewPhaseOutputDetectorWithDir creates a detector for a task in a specific directory.
func NewPhaseOutputDetectorWithDir(taskDir, taskID string, weight orcv1.TaskWeight) *PhaseOutputDetector {
	return &PhaseOutputDetector{
		taskDir: taskDir,
		taskID:  taskID,
		weight:  weight,
	}
}

// NewPhaseOutputDetectorWithBackend creates a detector with database backend for spec detection.
// This is the preferred constructor as it enables spec detection from the database.
func NewPhaseOutputDetectorWithBackend(taskDir, taskID string, weight orcv1.TaskWeight, backend storage.Backend) *PhaseOutputDetector {
	return &PhaseOutputDetector{
		taskDir: taskDir,
		taskID:  taskID,
		weight:  weight,
		backend: backend,
	}
}

// readFileLimited reads a file up to maxPhaseOutputFileSize bytes.
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
	if info.Size() > maxPhaseOutputFileSize {
		return nil, io.ErrUnexpectedEOF // File too large
	}

	return io.ReadAll(io.LimitReader(f, maxPhaseOutputFileSize))
}

// PhaseOutputStatus represents what outputs exist for a phase.
type PhaseOutputStatus struct {
	// PhaseID is the phase being checked.
	PhaseID string

	// HasOutput is true if relevant phase output exists.
	HasOutput bool

	// Outputs lists the detected output sources (database, files).
	Outputs []string

	// Description is a human-readable description of what was found.
	Description string

	// CanAutoSkip is true if this phase can be safely auto-skipped.
	// Some phases produce outputs but shouldn't be auto-skipped.
	CanAutoSkip bool
}

// DetectPhaseOutput checks if output exists for a phase.
func (d *PhaseOutputDetector) DetectPhaseOutput(phaseID string) *PhaseOutputStatus {
	switch phaseID {
	case "spec":
		return d.detectSpecOutput()
	case "research":
		return d.detectResearchOutput()
	case "implement":
		return d.detectImplementOutput()
	case "test":
		return d.detectTestOutput()
	case "docs":
		return d.detectDocsOutput()
	default:
		return &PhaseOutputStatus{
			PhaseID:     phaseID,
			HasOutput:   false,
			Description: "unknown phase",
		}
	}
}

// detectSpecOutput checks if spec exists in the database.
// Spec content is stored exclusively in the database to avoid merge conflicts in worktrees.
func (d *PhaseOutputDetector) detectSpecOutput() *PhaseOutputStatus {
	status := &PhaseOutputStatus{
		PhaseID: "spec",
	}

	// Try to load spec from database (preferred source)
	if d.backend != nil && d.taskID != "" {
		specContent, err := d.backend.GetSpecForTask(d.taskID)
		if err == nil && specContent != "" {
			// Check if spec has meaningful content
			if len(strings.TrimSpace(specContent)) < minMeaningfulContent {
				status.Description = "spec exists in database but appears empty or minimal"
				return status
			}

			// Validate spec content based on weight
			validation := task.ValidateSpec(specContent, d.weight)
			if !validation.Valid {
				status.HasOutput = true
				status.Outputs = []string{"database:spec"}
				status.Description = "spec exists in database but incomplete: " + strings.Join(validation.Issues, ", ")
				status.CanAutoSkip = false // Don't auto-skip invalid specs
				return status
			}

			status.HasOutput = true
			status.Outputs = []string{"database:spec"}
			status.Description = "spec exists in database with valid content"
			status.CanAutoSkip = true
			return status
		}
	}

	// No spec found - specs must be in database
	status.Description = "no spec found in database"
	return status
}

// detectResearchOutput checks if research output exists.
func (d *PhaseOutputDetector) detectResearchOutput() *PhaseOutputStatus {
	status := &PhaseOutputStatus{
		PhaseID: "research",
	}

	// Check for research output file
	researchPath := filepath.Join(d.taskDir, "outputs", "research.md")
	if content, err := readFileLimited(researchPath); err == nil {
		if len(strings.TrimSpace(string(content))) > minMeaningfulContent {
			status.HasOutput = true
			status.Outputs = []string{"outputs/research.md"}
			status.Description = "research.md output exists"
			status.CanAutoSkip = true
			return status
		}
	}

	status.Description = "no research output found"
	return status
}

// detectImplementOutput checks if implementation appears complete.
// This is the hardest phase to auto-detect since "implementation" varies widely.
func (d *PhaseOutputDetector) detectImplementOutput() *PhaseOutputStatus {
	status := &PhaseOutputStatus{
		PhaseID:     "implement",
		Description: "implement phase cannot be reliably auto-detected",
		// Never auto-skip implement phase - too risky
		CanAutoSkip: false,
	}

	return status
}

// detectTestOutput checks if test output exists.
func (d *PhaseOutputDetector) detectTestOutput() *PhaseOutputStatus {
	status := &PhaseOutputStatus{
		PhaseID: "test",
	}

	// Check for test results in task directory
	testResultsPath := filepath.Join(d.taskDir, "test-results")
	if info, err := os.Stat(testResultsPath); err == nil && info.IsDir() {
		// Check for report.json or similar
		reportPath := filepath.Join(testResultsPath, "report.json")
		if _, err := os.Stat(reportPath); err == nil {
			status.HasOutput = true
			status.Outputs = []string{"test-results/report.json"}
			status.Description = "test results exist"
			// Don't auto-skip tests - they should be re-run to validate current code
			status.CanAutoSkip = false
			return status
		}
	}

	// Check for outputs/test.md from previous run
	testOutputPath := filepath.Join(d.taskDir, "outputs", "test.md")
	if _, err := os.Stat(testOutputPath); err == nil {
		status.HasOutput = true
		status.Outputs = []string{"outputs/test.md"}
		status.Description = "test phase output exists from previous run"
		// Still don't auto-skip - tests should validate current code state
		status.CanAutoSkip = false
		return status
	}

	status.Description = "no test output found"
	return status
}

// detectDocsOutput checks if documentation output exists.
func (d *PhaseOutputDetector) detectDocsOutput() *PhaseOutputStatus {
	status := &PhaseOutputStatus{
		PhaseID: "docs",
	}

	// Check for docs output
	docsOutputPath := filepath.Join(d.taskDir, "outputs", "docs.md")
	if _, err := os.Stat(docsOutputPath); err == nil {
		status.HasOutput = true
		status.Outputs = []string{"outputs/docs.md"}
		status.Description = "docs phase output exists"
		status.CanAutoSkip = true
		return status
	}

	status.Description = "no docs output found"
	return status
}

// DetectAllPhaseOutputs checks all phases in a plan.
func (d *PhaseOutputDetector) DetectAllPhaseOutputs(phaseIDs []string) map[string]*PhaseOutputStatus {
	results := make(map[string]*PhaseOutputStatus)
	for _, phaseID := range phaseIDs {
		results[phaseID] = d.DetectPhaseOutput(phaseID)
	}
	return results
}

// SuggestSkippablePhases returns phases that have outputs and can be safely skipped.
func (d *PhaseOutputDetector) SuggestSkippablePhases(phaseIDs []string) []string {
	var skippable []string
	for _, phaseID := range phaseIDs {
		status := d.DetectPhaseOutput(phaseID)
		if status.HasOutput && status.CanAutoSkip {
			skippable = append(skippable, phaseID)
		}
	}
	return skippable
}
