package executor

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func TestExtractPhaseContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "extracts content from JSON",
			output: `{"status": "complete", "content": "The content text"}`,
			want:   "The content text",
		},
		{
			name:   "returns empty when no content field",
			output: `{"status": "complete", "summary": "Done"}`,
			want:   "",
		},
		{
			name:   "returns empty for invalid JSON",
			output: "not json at all",
			want:   "",
		},
		{
			name:   "handles content with newlines",
			output: `{"status": "complete", "content": "Line 1\nLine 2\nLine 3"}`,
			want:   "Line 1\nLine 2\nLine 3",
		},
		{
			name:   "handles content with escaped characters",
			output: `{"status": "complete", "content": "Code: \"function() {}\""}`,
			want:   `Code: "function() {}"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractPhaseContent(tt.output)
			if got != tt.want {
				t.Errorf("ExtractPhaseContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

// createPhaseContentTestTask creates a task in the backend for testing spec operations.
func createPhaseContentTestTask(t *testing.T, backend storage.Backend, taskID string) {
	t.Helper()
	testTask := task.NewProtoTask(taskID, "Test task")
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("create test task: %v", err)
	}
}

// createPhaseContentTestWorkflowRun creates a workflow and workflow run for testing phase output operations.
// The phase_outputs table has a foreign key constraint to workflow_runs, which needs the run to exist.
func createPhaseContentTestWorkflowRun(t *testing.T, backend storage.Backend, runID, taskID string) {
	t.Helper()
	// Create a minimal test workflow first (in-memory backend doesn't seed workflows)
	workflow := &db.Workflow{
		ID:          "test-workflow",
		Name:        "Test Workflow",
		Description: "Workflow for testing",
	}
	if err := backend.SaveWorkflow(workflow); err != nil {
		// Ignore duplicate key error - workflow may already exist
		if !strings.Contains(err.Error(), "UNIQUE constraint") {
			t.Fatalf("create test workflow: %v", err)
		}
	}

	// Create workflow run
	run := &db.WorkflowRun{
		ID:          runID,
		WorkflowID:  "test-workflow",
		TaskID:      &taskID,
		ContextType: "task",
		ContextData: "{}",
		Prompt:      "Test prompt",
		Status:      "running",
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("create test workflow run: %v", err)
	}
}

func TestSaveSpecToDatabase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		phaseID string
		output  string
		wantErr bool
	}{
		{
			name:    "skips non-spec phase",
			phaseID: "implement",
			output:  `{"status": "complete", "content": "some content"}`,
			wantErr: false, // Non-spec phases return (false, nil)
		},
		{
			name:    "returns error when no content in JSON",
			phaseID: "spec",
			output:  `{"status": "complete", "summary": "Done"}`,
			wantErr: true,
		},
		{
			name:    "returns error for invalid JSON",
			phaseID: "spec",
			output:  "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := newTestBackend(t)
			taskID := "TASK-SPEC-001"
			createPhaseContentTestTask(t, backend, taskID)

			saved, err := SaveSpecToDatabase(backend, "RUN-001", taskID, tt.phaseID, tt.output)

			if tt.wantErr {
				if err == nil {
					t.Error("SaveSpecToDatabase() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("SaveSpecToDatabase() error = %v", err)
				}
			}

			if tt.phaseID != "spec" && saved {
				t.Error("non-spec phase should not save")
			}
		})
	}
}

func TestSaveSpecToDatabase_NilBackend(t *testing.T) {
	t.Parallel()
	saved, err := SaveSpecToDatabase(nil, "RUN-001", "TASK-001", "spec", `{"status": "complete", "content": "content"}`)
	if err == nil {
		t.Fatal("SaveSpecToDatabase() with nil backend should return error")
	}
	if !strings.Contains(err.Error(), "backend is nil") {
		t.Errorf("error should mention nil backend, got: %v", err)
	}
	if saved {
		t.Error("SaveSpecToDatabase() with nil backend should return false")
	}
}

// TestSaveSpecToDatabase_ExtractsFromJSON verifies that SaveSpecToDatabase extracts
// spec content from JSON content field.
func TestSaveSpecToDatabase_ExtractsFromJSON(t *testing.T) {
	t.Parallel()
	backend := newTestBackend(t)
	taskID := "TASK-JSON-001"
	runID := "RUN-JSON-001"

	createPhaseContentTestTask(t, backend, taskID)
	createPhaseContentTestWorkflowRun(t, backend, runID, taskID)

	// Output with spec in content field
	specContent := `# Specification: Test Feature

## Problem Statement
This tests the JSON extraction mechanism.

## Success Criteria
- [ ] Agent outputs spec in content field
- [ ] System extracts from JSON
`
	output := `{"status": "complete", "summary": "Spec completed", "content": ` + escapeJSONString(specContent) + `}`

	saved, err := SaveSpecToDatabase(backend, runID, taskID, "spec", output)
	if err != nil {
		t.Fatalf("SaveSpecToDatabase() error = %v", err)
	}
	if !saved {
		t.Error("SaveSpecToDatabase() should have saved spec from JSON")
	}

	// Verify content was saved from content field
	loadedSpec, err := backend.GetSpecForTask(taskID)
	if err != nil {
		t.Fatalf("GetSpecForTask() error = %v", err)
	}

	if !strings.Contains(loadedSpec, "JSON extraction mechanism") {
		t.Errorf("spec content should be from JSON content field, got: %s", loadedSpec)
	}
}

// escapeJSONString properly escapes a string for JSON
func escapeJSONString(s string) string {
	// Simple escaping for test purposes
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return `"` + s + `"`
}

func TestIsValidSpecContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "valid spec with intent section",
			content: "# Specification\n\n## Intent\nImplement user authentication with proper session management.",
			want:    true,
		},
		{
			name:    "valid spec with success criteria",
			content: "# Spec\n\n## Success Criteria\n- Users can log in\n- Sessions persist across requests",
			want:    true,
		},
		{
			name:    "valid spec with testing section",
			content: "# Technical Spec\n\n## Testing\n- Unit tests for auth module\n- Integration tests for login flow",
			want:    true,
		},
		{
			name:    "rejects empty content",
			content: "",
			want:    false,
		},
		{
			name:    "rejects very short content",
			content: "Short",
			want:    false,
		},
		{
			name:    "rejects JSON completion status only",
			content: `{"status": "complete", "summary": "Done"}`,
			want:    false,
		},
		{
			name:    "rejects garbage with JSON completion",
			content: "The working tree is clean - the spec was created.\n" + `{"status": "complete", "summary": "Done"}`,
			want:    false,
		},
		{
			name:    "rejects common garbage pattern",
			content: "The spec was created as output in this conversation rather than a file.\n**Commit**: N/A",
			want:    false,
		},
		{
			name:    "accepts long content without sections",
			content: "This is a very detailed specification that describes the implementation requirements in great detail. It covers all the necessary aspects of the feature including edge cases, error handling, and performance considerations. The implementation should follow best practices.",
			want:    true, // 200+ chars without noise
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSpecContent(tt.content)
			if got != tt.want {
				t.Errorf("isValidSpecContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSpecExtractionError_Diagnostics verifies that the error message includes
// comprehensive diagnostic information for debugging spec failures.
func TestSpecExtractionError_Diagnostics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		err            *SpecExtractionError
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "includes output length and preview",
			err: &SpecExtractionError{
				Reason:        "no content field in JSON output",
				OutputLen:     500,
				OutputPreview: "Some output without content field...",
			},
			wantContains: []string{
				"no content field in JSON output",
				"output_length: 500 bytes",
				"output_preview: \"Some output without content field...\"",
			},
		},
		{
			name: "includes validation failure reason",
			err: &SpecExtractionError{
				Reason:            "spec content failed validation",
				OutputLen:         500,
				OutputPreview:     "short",
				ValidationFailure: "content too short (5 chars, need at least 50)",
			},
			wantContains: []string{
				"validation_failure: content too short (5 chars, need at least 50)",
			},
		},
		{
			name: "omits empty fields",
			err: &SpecExtractionError{
				Reason:    "no content field",
				OutputLen: 0,
			},
			wantNotContain: []string{
				"output_preview",
				"validation_failure",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()

			for _, want := range tt.wantContains {
				if !strings.Contains(errMsg, want) {
					t.Errorf("error message should contain %q\ngot: %s", want, errMsg)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(errMsg, notWant) {
					t.Errorf("error message should NOT contain %q\ngot: %s", notWant, errMsg)
				}
			}
		})
	}
}

// TestValidateSpecContent_ReturnsReason verifies the new validateSpecContent
// function returns descriptive failure reasons.
func TestValidateSpecContent_ReturnsReason(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		content     string
		wantReason  string // empty means valid
		wantContain string // substring that should be in the reason
	}{
		{
			name:       "valid spec returns empty",
			content:    "# Spec\n\n## Intent\nBuild feature X with robust error handling.",
			wantReason: "",
		},
		{
			name:        "too short returns length info",
			content:     "Short",
			wantContain: "content too short (5 chars, need at least 50)",
		},
		{
			name: "noise pattern detected",
			// 50 chars minimum to pass first check, but less than 50 before the noise marker
			content:     `Short preamble. {"status": "complete", "summary": "Done"} More text to reach 50 chars minimum length`,
			wantContain: "noise pattern detected",
		},
		{
			name:        "missing sections and short",
			content:     "This is some content without any spec sections at all.",
			wantContain: "no recognized spec sections",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := validateSpecContent(tt.content)

			if tt.wantReason != "" && reason != tt.wantReason {
				t.Errorf("validateSpecContent() = %q, want %q", reason, tt.wantReason)
			}

			if tt.wantContain != "" && !strings.Contains(reason, tt.wantContain) {
				t.Errorf("validateSpecContent() should contain %q, got %q", tt.wantContain, reason)
			}

			if tt.wantReason == "" && tt.wantContain == "" && reason != "" {
				t.Errorf("validateSpecContent() should be valid (empty reason), got %q", reason)
			}
		})
	}
}

// TestSaveSpecToDatabase_PopulatesDiagnostics verifies that SaveSpecToDatabase
// populates all diagnostic fields in SpecExtractionError.
func TestSaveSpecToDatabase_PopulatesDiagnostics(t *testing.T) {
	t.Parallel()
	backend := newTestBackend(t)
	taskID := "TASK-DIAG-001"
	runID := "RUN-DIAG-001"
	createPhaseContentTestTask(t, backend, taskID)
	createPhaseContentTestWorkflowRun(t, backend, runID, taskID)

	t.Run("no content includes output preview", func(t *testing.T) {
		output := `{"status": "complete", "summary": "Done but no content"}`
		_, err := SaveSpecToDatabase(backend, runID, taskID, "spec", output)

		specErr, ok := err.(*SpecExtractionError)
		if !ok {
			t.Fatalf("expected SpecExtractionError, got %T", err)
		}

		if specErr.OutputLen != len(output) {
			t.Errorf("OutputLen = %d, want %d", specErr.OutputLen, len(output))
		}
		if specErr.OutputPreview == "" {
			t.Error("OutputPreview should not be empty")
		}
	})

	t.Run("content extraction success", func(t *testing.T) {
		specContent := "# Specification\n\n## Intent\nBuild a feature with proper error handling and tests."
		output := `{"status": "complete", "content": ` + escapeJSONString(specContent) + `}`
		saved, err := SaveSpecToDatabase(backend, runID, taskID, "spec", output)

		if err != nil {
			t.Fatalf("SaveSpecToDatabase() unexpected error: %v", err)
		}
		if !saved {
			t.Error("SaveSpecToDatabase() should have saved from content field")
		}
	})

	t.Run("content too short returns validation failure", func(t *testing.T) {
		output := `{"status": "complete", "content": "short"}`
		_, err := SaveSpecToDatabase(backend, runID, taskID, "spec", output)

		specErr, ok := err.(*SpecExtractionError)
		if !ok {
			t.Fatalf("expected SpecExtractionError, got %T", err)
		}

		if specErr.ValidationFailure == "" {
			t.Error("ValidationFailure should be populated for short content")
		}
		if !strings.Contains(specErr.ValidationFailure, "content too short") {
			t.Errorf("ValidationFailure should mention 'too short', got %q", specErr.ValidationFailure)
		}
	})
}

// TestContentProducingPhases verifies the phase content mapping
func TestContentProducingPhases(t *testing.T) {
	t.Parallel()

	// Includes TDD phases: tiny_spec (combined spec+TDD), tdd_write, breakdown
	contentPhases := []string{"spec", "tiny_spec", "research", "tdd_write", "breakdown", "docs"}
	nonContentPhases := []string{"implement", "test", "review", "finalize"}

	for _, phase := range contentPhases {
		if !contentProducingPhases[phase] {
			t.Errorf("contentProducingPhases[%q] should be true", phase)
		}
	}

	for _, phase := range nonContentPhases {
		if contentProducingPhases[phase] {
			t.Errorf("contentProducingPhases[%q] should be false", phase)
		}
	}
}

// TestGetSchemaForPhase verifies schema selection by phase
func TestGetSchemaForPhase(t *testing.T) {
	t.Parallel()

	t.Run("content phases get content schema", func(t *testing.T) {
		for _, phase := range []string{"spec", "tiny_spec", "research", "tdd_write", "breakdown", "docs"} {
			schema := GetSchemaForPhase(phase)
			if !strings.Contains(schema, `"content"`) {
				t.Errorf("GetSchemaForPhase(%q) should return schema with content field", phase)
			}
		}
	})

	t.Run("standard phases get basic schema", func(t *testing.T) {
		for _, phase := range []string{"implement", "test", "finalize"} {
			schema := GetSchemaForPhase(phase)
			// Should not have content field
			if strings.Contains(schema, `"content"`) {
				t.Errorf("GetSchemaForPhase(%q) should return schema WITHOUT content field", phase)
			}
		}
	})

	t.Run("review phase gets specialized schema", func(t *testing.T) {
		// Round 1 (default) gets ReviewFindingsSchema
		schema := GetSchemaForPhaseWithRound("review", 1)
		if !strings.Contains(schema, `"issues"`) {
			t.Error("review round 1 should return ReviewFindingsSchema with issues field")
		}

		// Round 2 gets ReviewDecisionSchema
		schema = GetSchemaForPhaseWithRound("review", 2)
		if !strings.Contains(schema, `"gaps_addressed"`) {
			t.Error("review round 2 should return ReviewDecisionSchema with gaps_addressed field")
		}
	})

	t.Run("qa phase gets specialized schema", func(t *testing.T) {
		schema := GetSchemaForPhase("qa")
		if !strings.Contains(schema, `"tests_written"`) {
			t.Error("qa phase should return QAResultSchema with tests_written field")
		}
	})
}
