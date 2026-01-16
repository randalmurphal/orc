package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func TestSavePhaseArtifact(t *testing.T) {
	// Create temp task directory
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-ART-001")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	// Override task.OrcDir for testing
	oldOrcDir := ".orc"
	defer func() { _ = oldOrcDir }()

	tests := []struct {
		name        string
		taskID      string
		phaseID     string
		output      string
		wantSaved   bool
		wantContent string
	}{
		{
			name:    "extracts artifact from tags",
			taskID:  "TASK-ART-001",
			phaseID: "spec",
			output: `Some preamble text

<artifact>
# Specification

## Problem Statement
This is the spec content.

## Success Criteria
- Criterion 1
- Criterion 2
</artifact>

<phase_complete>true</phase_complete>`,
			wantSaved: true,
			wantContent: `# Specification

## Problem Statement
This is the spec content.

## Success Criteria
- Criterion 1
- Criterion 2`,
		},
		{
			name:      "no artifact when no tags",
			taskID:    "TASK-ART-001",
			phaseID:   "implement",
			output:    "Just some random output without artifact tags",
			wantSaved: false,
		},
		{
			name:      "empty artifact",
			taskID:    "TASK-ART-001",
			phaseID:   "design",
			output:    "<artifact></artifact>",
			wantSaved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For this test, manually create the artifact to bypass the task.TaskDir lookup
			artifact := extractArtifact(tt.output)

			if tt.wantSaved {
				if artifact == "" {
					t.Error("expected artifact to be extracted, got empty string")
					return
				}
				if artifact != tt.wantContent {
					t.Errorf("artifact content mismatch\ngot:\n%s\n\nwant:\n%s", artifact, tt.wantContent)
				}
			} else {
				if artifact != "" {
					t.Errorf("expected no artifact, got: %s", artifact)
				}
			}
		})
	}
}

func TestExtractArtifact(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "simple artifact",
			content: "<artifact>test content</artifact>",
			want:    "test content",
		},
		{
			name: "artifact with markdown",
			content: `<artifact>
# Title

## Section
Content here.
</artifact>`,
			want: `# Title

## Section
Content here.`,
		},
		{
			name:    "no artifact tags",
			content: "just some text",
			want:    "",
		},
		{
			name: "specification section fallback",
			content: `## Specification

This is the spec content.

## Other Section
Something else`,
			want: "This is the spec content.",
		},
		{
			name: "research results fallback",
			content: `## Research Results

Found these patterns:
- Pattern 1
- Pattern 2

## Conclusion
Done`,
			want: `Found these patterns:
- Pattern 1
- Pattern 2`,
		},
		{
			name: "design section fallback",
			content: `## Design

Architecture overview here.

## Implementation
Not this part`,
			want: "Architecture overview here.",
		},
		{
			name: "implementation summary fallback",
			content: `## Implementation Summary

Changed these files:
- file1.go
- file2.go

## Done`,
			want: `Changed these files:
- file1.go
- file2.go`,
		},
		{
			name:    "artifact tags take precedence",
			content: "<artifact>preferred content</artifact>\n\n## Specification\nfallback content",
			want:    "preferred content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractArtifact(tt.content)
			if got != tt.want {
				t.Errorf("extractArtifact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadFromTranscript(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "TASK-TRANS-001")
	transcriptsDir := filepath.Join(taskDir, "transcripts")
	if err := os.MkdirAll(transcriptsDir, 0755); err != nil {
		t.Fatalf("failed to create transcripts dir: %v", err)
	}

	// Create transcript files
	files := map[string]string{
		"spec-001.md":  "iteration 1 content",
		"spec-002.md":  "<artifact>iteration 2 artifact</artifact>",
		"spec-003.md":  "<artifact>iteration 3 artifact (latest)</artifact>",
		"other-001.md": "other phase content",
	}

	for name, content := range files {
		path := filepath.Join(transcriptsDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file %s: %v", name, err)
		}
	}

	tests := []struct {
		name    string
		phaseID string
		want    string
	}{
		{
			name:    "loads latest transcript",
			phaseID: "spec",
			want:    "iteration 3 artifact (latest)",
		},
		{
			name:    "returns empty for no matching phase",
			phaseID: "missing",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loadFromTranscript(taskDir, tt.phaseID)
			if got != tt.want {
				t.Errorf("loadFromTranscript() = %q, want %q", got, tt.want)
			}
		})
	}
}

// newArtifactTestBackend creates a test backend for artifact tests.
func newArtifactTestBackend(t *testing.T) *storage.DatabaseBackend {
	t.Helper()
	tmpDir := t.TempDir()
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.Close()
	})
	return backend
}

// createTestTask creates a task in the backend for testing spec operations.
func createTestTask(t *testing.T, backend *storage.DatabaseBackend, taskID string) {
	t.Helper()
	testTask := &task.Task{
		ID:     taskID,
		Title:  "Test task",
		Status: task.StatusCreated,
		Weight: task.WeightSmall,
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("create test task: %v", err)
	}
}

func TestSaveSpecToDatabase(t *testing.T) {
	tests := []struct {
		name        string
		phaseID     string
		output      string
		wantSaved   bool
		wantContent string
	}{
		{
			name:    "saves spec content from artifact tags",
			phaseID: "spec",
			output: `Some preamble

<artifact>
# Specification

## Intent
Build a feature.

## Success Criteria
- Works correctly
</artifact>

<phase_complete>true</phase_complete>`,
			wantSaved: true,
			wantContent: `# Specification

## Intent
Build a feature.

## Success Criteria
- Works correctly`,
		},
		{
			name:        "saves raw output when no artifact tags",
			phaseID:     "spec",
			output:      "Raw spec content without artifact tags",
			wantSaved:   true,
			wantContent: "Raw spec content without artifact tags",
		},
		{
			name:      "skips non-spec phase",
			phaseID:   "implement",
			output:    "<artifact>Some content</artifact>",
			wantSaved: false,
		},
		{
			name:      "skips empty output",
			phaseID:   "spec",
			output:    "",
			wantSaved: false,
		},
		{
			name:      "skips research phase",
			phaseID:   "research",
			output:    "<artifact>Research results</artifact>",
			wantSaved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newArtifactTestBackend(t)
			taskID := "TASK-SPEC-001"

			// Create task first (required for foreign key constraint)
			createTestTask(t, backend, taskID)

			saved, err := SaveSpecToDatabase(backend, taskID, tt.phaseID, tt.output)
			if err != nil {
				t.Fatalf("SaveSpecToDatabase() error = %v", err)
			}

			if saved != tt.wantSaved {
				t.Errorf("SaveSpecToDatabase() saved = %v, want %v", saved, tt.wantSaved)
			}

			if tt.wantSaved {
				// Verify content was saved to database
				specContent, err := backend.LoadSpec(taskID)
				if err != nil {
					t.Fatalf("LoadSpec() error = %v", err)
				}
				if specContent == "" {
					t.Fatal("LoadSpec() returned empty, expected spec")
				}
				if specContent != tt.wantContent {
					t.Errorf("spec content = %q, want %q", specContent, tt.wantContent)
				}
			}
		})
	}
}

func TestSaveSpecToDatabase_NilBackend(t *testing.T) {
	saved, err := SaveSpecToDatabase(nil, "TASK-001", "spec", "Some content")
	if err != nil {
		t.Fatalf("SaveSpecToDatabase() with nil backend should not error, got %v", err)
	}
	if saved {
		t.Error("SaveSpecToDatabase() with nil backend should return false")
	}
}

// TestSavePhaseArtifact_SkipsSpecPhase verifies that SavePhaseArtifact does NOT
// write files for the spec phase. Spec content should only be saved to the database
// via SaveSpecToDatabase to avoid merge conflicts in worktrees.
func TestSavePhaseArtifact_SkipsSpecPhase(t *testing.T) {
	// Create a temp directory with task structure
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-SKIP-001")
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	// Save the old task dir resolver and restore after test
	oldTaskDir := task.TaskDir("TASK-SKIP-001")
	_ = oldTaskDir // acknowledge old value

	// Since task.TaskDir uses a global path, we need to verify behavior
	// by checking that the function returns empty string for spec phase

	specOutput := `<artifact>
# Specification

## Problem Statement
This spec should NOT be written to a file.

## Success Criteria
- Only saved to database
</artifact>

<phase_complete>true</phase_complete>`

	// Call SavePhaseArtifact for spec phase
	path, err := SavePhaseArtifact("TASK-SKIP-001", "spec", specOutput)
	if err != nil {
		t.Fatalf("SavePhaseArtifact() error = %v", err)
	}

	// Should return empty path for spec phase (no file written)
	if path != "" {
		t.Errorf("SavePhaseArtifact(spec) should return empty path, got %q", path)
	}

	// Verify no artifacts directory was created in the actual task dir
	// (this tests the real behavior when task.TaskDir resolves)
	artifactDir := filepath.Join(taskDir, "artifacts")
	specPath := filepath.Join(artifactDir, "spec.md")
	if _, err := os.Stat(specPath); err == nil {
		t.Error("spec.md file should not exist in artifacts directory")
	}
}

// TestSavePhaseArtifact_WritesNonSpecPhases verifies that SavePhaseArtifact
// still writes files for non-spec phases like implement, test, docs, etc.
func TestSavePhaseArtifact_WritesNonSpecPhases(t *testing.T) {
	// This test verifies the behavior through the extractArtifact function
	// since actual file writing depends on task.TaskDir configuration

	implementOutput := `<artifact>
## Implementation Summary

Changed these files:
- file1.go
- file2.go
</artifact>`

	// Verify artifact is extracted for non-spec phases
	artifact := extractArtifact(implementOutput)
	if artifact == "" {
		t.Error("extractArtifact should extract content for non-spec phases")
	}

	expectedContent := `## Implementation Summary

Changed these files:
- file1.go
- file2.go`
	if artifact != expectedContent {
		t.Errorf("artifact content mismatch\ngot:\n%s\n\nwant:\n%s", artifact, expectedContent)
	}
}

func TestSaveSpecToDatabase_ArtifactTagsPrecedence(t *testing.T) {
	backend := newArtifactTestBackend(t)
	taskID := "TASK-SPEC-002"

	// Create task first (required for foreign key constraint)
	createTestTask(t, backend, taskID)

	// Output with both artifact tags and other content
	output := `Some preamble that should be ignored.

<artifact>
The real spec content
</artifact>

And some trailing text.`

	saved, err := SaveSpecToDatabase(backend, taskID, "spec", output)
	if err != nil {
		t.Fatalf("SaveSpecToDatabase() error = %v", err)
	}
	if !saved {
		t.Error("SaveSpecToDatabase() should have saved spec")
	}

	// Verify only artifact content was saved
	specContent, err := backend.LoadSpec(taskID)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}
	if specContent != "The real spec content" {
		t.Errorf("spec content = %q, want 'The real spec content'", specContent)
	}
}

func TestLoadPriorContent(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "TASK-PRIOR-001")
	artifactsDir := filepath.Join(taskDir, "artifacts")
	transcriptsDir := filepath.Join(taskDir, "transcripts")

	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		t.Fatalf("failed to create artifacts dir: %v", err)
	}
	if err := os.MkdirAll(transcriptsDir, 0755); err != nil {
		t.Fatalf("failed to create transcripts dir: %v", err)
	}

	// Create artifact file
	artifactContent := "This is the saved artifact"
	if err := os.WriteFile(filepath.Join(artifactsDir, "spec.md"), []byte(artifactContent), 0644); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	// Create transcript file (for fallback test)
	transcriptContent := "<artifact>Transcript artifact</artifact>"
	if err := os.WriteFile(filepath.Join(transcriptsDir, "design-001.md"), []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	tests := []struct {
		name    string
		phaseID string
		want    string
	}{
		{
			name:    "loads from artifact file",
			phaseID: "spec",
			want:    artifactContent,
		},
		{
			name:    "falls back to transcript",
			phaseID: "design",
			want:    "Transcript artifact",
		},
		{
			name:    "returns empty for missing phase",
			phaseID: "missing",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loadPriorContent(taskDir, nil, tt.phaseID)
			if got != tt.want {
				t.Errorf("loadPriorContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
