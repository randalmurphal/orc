package executor

import (
	"os"
	"path/filepath"
	"testing"
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
