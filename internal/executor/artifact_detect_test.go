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

func TestArtifactDetector_DetectSpecArtifacts(t *testing.T) {
	t.Parallel()
	// Create temp task directory
	tmpDir := t.TempDir()
	taskID := "TEST-001"

	tests := []struct {
		name         string
		setup        func(taskDir string)
		weight       task.Weight
		wantArtifact bool
		wantAutoSkip bool
		wantDescSub  string
	}{
		{
			name:         "no spec file",
			setup:        func(taskDir string) {},
			weight:       task.WeightMedium,
			wantArtifact: false,
			wantDescSub:  "no spec found",
		},
		{
			name: "valid spec file",
			setup: func(taskDir string) {
				content := `# Task Specification

## Intent
This task implements a new feature for artifact detection.

## Success Criteria
- Detect existing artifacts
- Prompt user to skip phases

## Testing
- Unit tests for artifact detection
- Integration tests for CLI
`
				_ = os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte(content), 0644)
			},
			weight:       task.WeightMedium,
			wantArtifact: true,
			wantAutoSkip: true,
			wantDescSub:  "valid content",
		},
		{
			name: "empty spec file",
			setup: func(taskDir string) {
				_ = os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte(""), 0644)
			},
			weight:       task.WeightMedium,
			wantArtifact: false,
			wantDescSub:  "empty",
		},
		{
			name: "minimal spec file - too short",
			setup: func(taskDir string) {
				_ = os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte("# Title"), 0644)
			},
			weight:       task.WeightMedium,
			wantArtifact: false,
			wantDescSub:  "empty or minimal",
		},
		{
			name: "spec file missing required sections",
			setup: func(taskDir string) {
				content := `# Task Specification

## Intent
This task does something.

But it's missing Success Criteria and Testing sections.
`
				_ = os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte(content), 0644)
			},
			weight:       task.WeightMedium,
			wantArtifact: true,
			wantAutoSkip: false, // Should not auto-skip invalid specs
			wantDescSub:  "incomplete",
		},
		{
			name: "trivial weight - skip validation",
			setup: func(taskDir string) {
				// Trivial tasks skip validation, so content doesn't need full spec structure
				// but needs to be more than 50 chars to pass the basic content check
				content := `# Simple fix

This is a trivial task to fix a small typo in the documentation.
The fix is straightforward and doesn't need detailed specification.
`
				_ = os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte(content), 0644)
			},
			weight:       task.WeightTrivial,
			wantArtifact: true,
			wantAutoSkip: true, // Trivial tasks skip validation
			wantDescSub:  "valid content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh task dir for each test
			taskDir := filepath.Join(tmpDir, tt.name)
			_ = os.MkdirAll(taskDir, 0755)

			// Run setup
			tt.setup(taskDir)

			// Create detector
			detector := NewArtifactDetectorWithDir(taskDir, taskID, tt.weight)
			status := detector.DetectPhaseArtifacts("spec")

			if status.HasArtifacts != tt.wantArtifact {
				t.Errorf("HasArtifacts = %v, want %v", status.HasArtifacts, tt.wantArtifact)
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

func TestArtifactDetector_DetectResearchArtifacts(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskID := "TEST-002"

	tests := []struct {
		name         string
		setup        func(taskDir string)
		wantArtifact bool
		wantAutoSkip bool
	}{
		{
			name:         "no research artifacts",
			setup:        func(taskDir string) {},
			wantArtifact: false,
		},
		{
			name: "research.md in artifacts dir",
			setup: func(taskDir string) {
				artifactDir := filepath.Join(taskDir, "artifacts")
				_ = os.MkdirAll(artifactDir, 0755)
				content := `# Research Findings

This is the research content with sufficient detail to be meaningful.
`
				_ = os.WriteFile(filepath.Join(artifactDir, "research.md"), []byte(content), 0644)
			},
			wantArtifact: true,
			wantAutoSkip: true,
		},
		{
			name: "research section in spec.md",
			setup: func(taskDir string) {
				content := `# Specification

## Intent
Do something.

## Research
Detailed research findings go here with lots of information.
`
				_ = os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte(content), 0644)
			},
			wantArtifact: true,
			wantAutoSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskDir := filepath.Join(tmpDir, tt.name)
			_ = os.MkdirAll(taskDir, 0755)
			tt.setup(taskDir)

			detector := NewArtifactDetectorWithDir(taskDir, taskID, task.WeightMedium)
			status := detector.DetectPhaseArtifacts("research")

			if status.HasArtifacts != tt.wantArtifact {
				t.Errorf("HasArtifacts = %v, want %v", status.HasArtifacts, tt.wantArtifact)
			}

			if status.CanAutoSkip != tt.wantAutoSkip {
				t.Errorf("CanAutoSkip = %v, want %v", status.CanAutoSkip, tt.wantAutoSkip)
			}
		})
	}
}

func TestArtifactDetector_ImplementTestValidateNotAutoSkippable(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskID := "TEST-003"
	taskDir := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(taskDir, 0755)

	detector := NewArtifactDetectorWithDir(taskDir, taskID, task.WeightMedium)

	// These phases should never be auto-skippable
	phases := []string{"implement", "test", "validate"}
	for _, phaseID := range phases {
		status := detector.DetectPhaseArtifacts(phaseID)
		if status.CanAutoSkip {
			t.Errorf("Phase %s should not be auto-skippable, but CanAutoSkip = true", phaseID)
		}
	}
}

func TestArtifactDetector_DetectDocsArtifacts(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskID := "TEST-004"

	tests := []struct {
		name         string
		setup        func(taskDir string)
		wantArtifact bool
		wantAutoSkip bool
	}{
		{
			name:         "no docs artifacts",
			setup:        func(taskDir string) {},
			wantArtifact: false,
		},
		{
			name: "docs.md in artifacts dir",
			setup: func(taskDir string) {
				artifactDir := filepath.Join(taskDir, "artifacts")
				_ = os.MkdirAll(artifactDir, 0755)
				_ = os.WriteFile(filepath.Join(artifactDir, "docs.md"), []byte("# Documentation"), 0644)
			},
			wantArtifact: true,
			wantAutoSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskDir := filepath.Join(tmpDir, tt.name)
			_ = os.MkdirAll(taskDir, 0755)
			tt.setup(taskDir)

			detector := NewArtifactDetectorWithDir(taskDir, taskID, task.WeightMedium)
			status := detector.DetectPhaseArtifacts("docs")

			if status.HasArtifacts != tt.wantArtifact {
				t.Errorf("HasArtifacts = %v, want %v", status.HasArtifacts, tt.wantArtifact)
			}

			if status.CanAutoSkip != tt.wantAutoSkip {
				t.Errorf("CanAutoSkip = %v, want %v", status.CanAutoSkip, tt.wantAutoSkip)
			}
		})
	}
}

func TestArtifactDetector_SuggestSkippablePhases(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskID := "TEST-005"
	taskDir := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(taskDir, 0755)

	// Create a valid spec
	specContent := `# Specification

## Intent
Implement artifact detection.

## Success Criteria
- Works correctly.

## Testing
- Unit tests pass.
`
	_ = os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte(specContent), 0644)

	// Create research artifact (needs to be >50 chars)
	artifactDir := filepath.Join(taskDir, "artifacts")
	_ = os.MkdirAll(artifactDir, 0755)
	researchContent := `# Research Findings

This research document contains detailed analysis of the codebase
and architectural decisions that will inform the implementation.
`
	_ = os.WriteFile(filepath.Join(artifactDir, "research.md"), []byte(researchContent), 0644)

	detector := NewArtifactDetectorWithDir(taskDir, taskID, task.WeightMedium)

	// Test with phases that have artifacts
	phases := []string{"spec", "research", "implement", "test", "docs"}
	skippable := detector.SuggestSkippablePhases(phases)

	// Should suggest spec and research (both have artifacts and are auto-skippable)
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

func TestArtifactDetector_UnknownPhase(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(taskDir, 0755)

	detector := NewArtifactDetectorWithDir(taskDir, "TEST-006", task.WeightMedium)
	status := detector.DetectPhaseArtifacts("unknown_phase")

	if status.HasArtifacts {
		t.Error("Unknown phase should not have artifacts")
	}
	if status.CanAutoSkip {
		t.Error("Unknown phase should not be auto-skippable")
	}
	if !strings.Contains(strings.ToLower(status.Description), "unknown") {
		t.Errorf("Expected description to mention unknown, got: %s", status.Description)
	}
}

// TestArtifactDetector_DetectSpecFromDatabase verifies that spec detection
// prioritizes database over file-based storage.
func TestArtifactDetector_DetectSpecFromDatabase(t *testing.T) {
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
- No file artifact needed

## Testing
- Unit test verifies database loading
`
	if err := backend.SaveSpec(taskID, specContent, "test"); err != nil {
		t.Fatalf("save spec: %v", err)
	}

	// Create detector with backend
	detector := NewArtifactDetectorWithBackend(taskDir, taskID, task.WeightMedium, backend)
	status := detector.DetectPhaseArtifacts("spec")

	// Should detect spec from database
	if !status.HasArtifacts {
		t.Error("HasArtifacts should be true when spec exists in database")
	}
	if !status.CanAutoSkip {
		t.Error("CanAutoSkip should be true for valid spec in database")
	}
	if !strings.Contains(status.Description, "database") {
		t.Errorf("Description should mention database, got: %s", status.Description)
	}
	if len(status.Artifacts) != 1 || status.Artifacts[0] != "database:spec" {
		t.Errorf("Artifacts should be ['database:spec'], got: %v", status.Artifacts)
	}

	// Verify no spec.md file exists (spec should only be in DB)
	specPath := filepath.Join(taskDir, "spec.md")
	if _, err := os.Stat(specPath); err == nil {
		t.Error("spec.md file should not exist - spec is in database only")
	}
}

// TestArtifactDetector_PrefersDatabaseOverFile verifies that when both
// database and file spec exist, database is preferred.
func TestArtifactDetector_PrefersDatabaseOverFile(t *testing.T) {
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
	if err := backend.SaveSpec(taskID, dbSpecContent, "test"); err != nil {
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
	detector := NewArtifactDetectorWithBackend(taskDir, taskID, task.WeightMedium, backend)
	status := detector.DetectPhaseArtifacts("spec")

	// Should detect spec from database (not file)
	if !status.HasArtifacts {
		t.Error("HasArtifacts should be true")
	}
	if !strings.Contains(status.Description, "database") {
		t.Errorf("Description should mention database (not legacy file), got: %s", status.Description)
	}
	if len(status.Artifacts) != 1 || status.Artifacts[0] != "database:spec" {
		t.Errorf("Artifacts should be ['database:spec'], got: %v", status.Artifacts)
	}
}

// TestArtifactDetector_FallsBackToFile verifies that when no database backend
// is available, the detector falls back to file-based detection.
func TestArtifactDetector_FallsBackToFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task")
	_ = os.MkdirAll(taskDir, 0755)

	taskID := "TEST-FILE-001"

	// Create legacy file-based spec (no database)
	fileSpecContent := `# File Spec

## Intent
This is a legacy file-based spec.

## Success Criteria
- Used when no database

## Testing
- Verify file fallback works
`
	if err := os.WriteFile(filepath.Join(taskDir, "spec.md"), []byte(fileSpecContent), 0644); err != nil {
		t.Fatalf("write spec file: %v", err)
	}

	// Create detector WITHOUT backend (nil)
	detector := NewArtifactDetectorWithDir(taskDir, taskID, task.WeightMedium)
	status := detector.DetectPhaseArtifacts("spec")

	// Should detect spec from file (fallback)
	if !status.HasArtifacts {
		t.Error("HasArtifacts should be true when spec file exists")
	}
	if !strings.Contains(status.Description, "legacy") {
		t.Errorf("Description should mention legacy file, got: %s", status.Description)
	}
	if len(status.Artifacts) != 1 || status.Artifacts[0] != "spec.md" {
		t.Errorf("Artifacts should be ['spec.md'], got: %v", status.Artifacts)
	}
}
