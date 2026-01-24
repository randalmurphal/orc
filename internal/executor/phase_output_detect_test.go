// Package executor provides task phase execution for orc.
package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func TestPhaseOutputDetector_DetectSpecOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		specContent   string // Content to save to database (empty = no spec)
		weight        task.Weight
		wantHasOutput bool
		wantAutoSkip  bool
		wantDescSub   string
	}{
		{
			name:          "no spec in database",
			specContent:   "",
			weight:        task.WeightMedium,
			wantHasOutput: false,
			wantDescSub:   "no spec found",
		},
		{
			name: "valid spec in database",
			specContent: `# Task Specification

## Intent
This task implements a new feature for output detection.

## Success Criteria
- Detect existing outputs
- Prompt user to skip phases

## Testing
- Unit tests for output detection
- Integration tests for CLI
`,
			weight:        task.WeightMedium,
			wantHasOutput: true,
			wantAutoSkip:  true,
			wantDescSub:   "valid content",
		},
		{
			name:          "empty spec in database",
			specContent:   "",
			weight:        task.WeightMedium,
			wantHasOutput: false,
			wantDescSub:   "no spec found",
		},
		{
			name:          "minimal spec - too short",
			specContent:   "# Title",
			weight:        task.WeightMedium,
			wantHasOutput: false,
			wantDescSub:   "empty or minimal",
		},
		{
			name: "spec missing required sections",
			specContent: `# Task Specification

## Intent
This task does something.

But it's missing Success Criteria and Testing sections.
`,
			weight:        task.WeightMedium,
			wantHasOutput: true,
			wantAutoSkip:  false, // Should not auto-skip invalid specs
			wantDescSub:   "incomplete",
		},
		{
			name: "trivial weight - skip validation",
			specContent: `# Simple fix

This is a trivial task to fix a small typo in the documentation.
The fix is straightforward and doesn't need detailed specification.
`,
			weight:        task.WeightTrivial,
			wantHasOutput: true,
			wantAutoSkip:  true, // Trivial tasks skip validation
			wantDescSub:   "valid content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			taskDir := filepath.Join(tmpDir, "task")
			_ = os.MkdirAll(taskDir, 0755)
			taskID := "TEST-001"

			// Create database backend
			backend, err := storage.NewDatabaseBackend(tmpDir, nil)
			if err != nil {
				t.Fatalf("create backend: %v", err)
			}
			defer func() { _ = backend.Close() }()

			// Create task first (required for spec save)
			testTask := &task.Task{
				ID:     taskID,
				Title:  "Test task",
				Status: task.StatusCreated,
				Weight: tt.weight,
			}
			if err := backend.SaveTask(testTask); err != nil {
				t.Fatalf("save task: %v", err)
			}

			// Save spec to database if provided
			if tt.specContent != "" {
				if err := backend.SaveSpecForTask(taskID, tt.specContent, "test"); err != nil {
					t.Fatalf("save spec: %v", err)
				}
			}

			// Create detector with backend
			detector := NewPhaseOutputDetectorWithBackend(taskDir, taskID, tt.weight, backend)
			status := detector.DetectPhaseOutput("spec")

			if status.HasOutput != tt.wantHasOutput {
				t.Errorf("HasOutput = %v, want %v", status.HasOutput, tt.wantHasOutput)
			}

			if status.CanAutoSkip != tt.wantAutoSkip {
				t.Errorf("CanAutoSkip = %v, want %v", status.CanAutoSkip, tt.wantAutoSkip)
			}

			if tt.wantDescSub != "" && !strings.Contains(strings.ToLower(status.Description), strings.ToLower(tt.wantDescSub)) {
				t.Errorf("Description = %q, want substring %q", status.Description, tt.wantDescSub)
			}
		})
	}
}

func TestPhaseOutputDetector_DetectResearchOutput(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskID := "TEST-002"

	tests := []struct {
		name          string
		setup         func(taskDir string)
		wantHasOutput bool
		wantAutoSkip  bool
	}{
		{
			name:          "no research output",
			setup:         func(taskDir string) {},
			wantHasOutput: false,
		},
		{
			name: "research.md in outputs dir",
			setup: func(taskDir string) {
				outputDir := filepath.Join(taskDir, "outputs")
				_ = os.MkdirAll(outputDir, 0755)
				content := `# Research Findings

This is the research content with sufficient detail to be meaningful.
`
				_ = os.WriteFile(filepath.Join(outputDir, "research.md"), []byte(content), 0644)
			},
			wantHasOutput: true,
			wantAutoSkip:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskDir := filepath.Join(tmpDir, tt.name)
			_ = os.MkdirAll(taskDir, 0755)
			tt.setup(taskDir)

			detector := NewPhaseOutputDetectorWithDir(taskDir, taskID, task.WeightMedium)
			status := detector.DetectPhaseOutput("research")

			if status.HasOutput != tt.wantHasOutput {
				t.Errorf("HasOutput = %v, want %v", status.HasOutput, tt.wantHasOutput)
			}

			if status.CanAutoSkip != tt.wantAutoSkip {
				t.Errorf("CanAutoSkip = %v, want %v", status.CanAutoSkip, tt.wantAutoSkip)
			}
		})
	}
}

func TestPhaseOutputDetector_ImplementTestNotAutoSkippable(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskID := "TEST-003"
	taskDir := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(taskDir, 0755)

	detector := NewPhaseOutputDetectorWithDir(taskDir, taskID, task.WeightMedium)

	// These phases should never be auto-skippable
	phases := []string{"implement", "test"}
	for _, phaseID := range phases {
		status := detector.DetectPhaseOutput(phaseID)
		if status.CanAutoSkip {
			t.Errorf("Phase %s should not be auto-skippable, but CanAutoSkip = true", phaseID)
		}
	}
}

func TestPhaseOutputDetector_DetectDocsOutput(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskID := "TEST-004"

	tests := []struct {
		name          string
		setup         func(taskDir string)
		wantHasOutput bool
		wantAutoSkip  bool
	}{
		{
			name:          "no docs output",
			setup:         func(taskDir string) {},
			wantHasOutput: false,
		},
		{
			name: "docs.md in outputs dir",
			setup: func(taskDir string) {
				outputDir := filepath.Join(taskDir, "outputs")
				_ = os.MkdirAll(outputDir, 0755)
				_ = os.WriteFile(filepath.Join(outputDir, "docs.md"), []byte("# Documentation"), 0644)
			},
			wantHasOutput: true,
			wantAutoSkip:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskDir := filepath.Join(tmpDir, tt.name)
			_ = os.MkdirAll(taskDir, 0755)
			tt.setup(taskDir)

			detector := NewPhaseOutputDetectorWithDir(taskDir, taskID, task.WeightMedium)
			status := detector.DetectPhaseOutput("docs")

			if status.HasOutput != tt.wantHasOutput {
				t.Errorf("HasOutput = %v, want %v", status.HasOutput, tt.wantHasOutput)
			}

			if status.CanAutoSkip != tt.wantAutoSkip {
				t.Errorf("CanAutoSkip = %v, want %v", status.CanAutoSkip, tt.wantAutoSkip)
			}
		})
	}
}

func TestPhaseOutputDetector_SuggestSkippablePhases(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskID := "TEST-005"
	taskDir := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(taskDir, 0755)

	// Create database backend
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	// Create task first
	testTask := &task.Task{
		ID:     taskID,
		Title:  "Test task",
		Status: task.StatusCreated,
		Weight: task.WeightMedium,
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create a valid spec in database
	specContent := `# Specification

## Intent
Implement output detection.

## Success Criteria
- Works correctly.

## Testing
- Unit tests pass.
`
	if err := backend.SaveSpecForTask(taskID, specContent, "test"); err != nil {
		t.Fatalf("save spec: %v", err)
	}

	// Create research output file
	outputDir := filepath.Join(taskDir, "outputs")
	_ = os.MkdirAll(outputDir, 0755)
	researchContent := `# Research Findings

This research document contains detailed analysis of the codebase
and architectural decisions that will inform the implementation.
`
	_ = os.WriteFile(filepath.Join(outputDir, "research.md"), []byte(researchContent), 0644)

	detector := NewPhaseOutputDetectorWithBackend(taskDir, taskID, task.WeightMedium, backend)

	// Test with phases that have outputs
	phases := []string{"spec", "research", "implement", "test", "docs"}
	skippable := detector.SuggestSkippablePhases(phases)

	// Should suggest spec and research (both have outputs and are auto-skippable)
	if len(skippable) != 2 {
		t.Errorf("Expected 2 skippable phases, got %d: %v", len(skippable), skippable)
	}

	expectedSkippable := map[string]bool{"spec": true, "research": true}
	for _, p := range skippable {
		if !expectedSkippable[p] {
			t.Errorf("Unexpected skippable phase: %s", p)
		}
	}
}

func TestPhaseOutputDetector_UnknownPhase(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(taskDir, 0755)

	detector := NewPhaseOutputDetectorWithDir(taskDir, "TEST-006", task.WeightMedium)
	status := detector.DetectPhaseOutput("unknown_phase")

	if status.HasOutput {
		t.Error("Unknown phase should not have output")
	}
	if status.CanAutoSkip {
		t.Error("Unknown phase should not be auto-skippable")
	}
	if !strings.Contains(strings.ToLower(status.Description), "unknown") {
		t.Errorf("Expected description to mention unknown, got: %s", status.Description)
	}
}

// TestPhaseOutputDetector_DetectSpecFromDatabase verifies that spec detection
// prioritizes database over file-based storage.
func TestPhaseOutputDetector_DetectSpecFromDatabase(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(taskDir, 0755)

	// Create database backend
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	taskID := "TEST-DB-001"

	// Create task in database
	testTask := &task.Task{
		ID:     taskID,
		Title:  "Test task",
		Status: task.StatusCreated,
		Weight: task.WeightMedium,
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Save spec to database
	specContent := `# Specification

## Intent
Test spec stored in database, not as file.

## Success Criteria
- Spec loaded from database
- No file needed

## Testing
- Unit test verifies database loading
`
	if err := backend.SaveSpecForTask(taskID, specContent, "test"); err != nil {
		t.Fatalf("save spec: %v", err)
	}

	// Create detector with backend
	detector := NewPhaseOutputDetectorWithBackend(taskDir, taskID, task.WeightMedium, backend)
	status := detector.DetectPhaseOutput("spec")

	// Should detect spec from database
	if !status.HasOutput {
		t.Error("HasOutput should be true when spec exists in database")
	}
	if !status.CanAutoSkip {
		t.Error("CanAutoSkip should be true for valid spec in database")
	}
	if !strings.Contains(status.Description, "database") {
		t.Errorf("Description should mention database, got: %s", status.Description)
	}
	if len(status.Outputs) != 1 || status.Outputs[0] != "database:spec" {
		t.Errorf("Outputs should be ['database:spec'], got: %v", status.Outputs)
	}

	// Verify no spec.md file exists (spec should only be in DB)
	specPath := filepath.Join(taskDir, "spec.md")
	if _, err := os.Stat(specPath); err == nil {
		t.Error("spec.md file should not exist - spec is in database only")
	}
}

// TestPhaseOutputDetector_PrefersDatabaseOverFile verifies that when both
// database and file spec exist, database is preferred.
func TestPhaseOutputDetector_PrefersDatabaseOverFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(taskDir, 0755)

	// Create database backend
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	taskID := "TEST-DB-002"

	// Create task in database
	testTask := &task.Task{
		ID:     taskID,
		Title:  "Test task",
		Status: task.StatusCreated,
		Weight: task.WeightMedium,
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Save spec to database
	dbSpecContent := `# Database Spec

## Intent
This is the spec from the database.

## Success Criteria
- Database takes precedence

## Testing
- Verify database spec is used
`
	if err := backend.SaveSpecForTask(taskID, dbSpecContent, "test"); err != nil {
		t.Fatalf("save spec: %v", err)
	}

	// Also create a legacy file-based spec
	fileSpecContent := `# File Spec

## Intent
This is the legacy file-based spec.

## Success Criteria
- Should NOT be used

## Testing
- This should be ignored
`
	if err := os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte(fileSpecContent), 0644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Create detector with backend
	detector := NewPhaseOutputDetectorWithBackend(taskDir, taskID, task.WeightMedium, backend)
	status := detector.DetectPhaseOutput("spec")

	// Should detect spec from database (not file)
	if !status.HasOutput {
		t.Error("HasOutput should be true")
	}
	if !strings.Contains(status.Description, "database") {
		t.Errorf("Description should mention database (not legacy file), got: %s", status.Description)
	}
	if len(status.Outputs) != 1 || status.Outputs[0] != "database:spec" {
		t.Errorf("Outputs should be ['database:spec'], got: %v", status.Outputs)
	}
}
