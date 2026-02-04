package api

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
)

// SC-8: dbPhaseTemplateToProto populates InputVariables from DB JSON field.
// The current implementation skips InputVariables entirely — these tests will
// fail until the conversion function is updated to parse the JSON column.
func TestDbPhaseTemplateToProto_InputVariables(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		inputVariables string
		wantVars       []string
	}{
		{
			name:           "valid JSON array with multiple variables",
			inputVariables: `["SPEC_CONTENT","TASK_DESCRIPTION"]`,
			wantVars:       []string{"SPEC_CONTENT", "TASK_DESCRIPTION"},
		},
		{
			name:           "single variable",
			inputVariables: `["WORKTREE_PATH"]`,
			wantVars:       []string{"WORKTREE_PATH"},
		},
		{
			name:           "empty JSON array returns empty slice not nil",
			inputVariables: `[]`,
			wantVars:       []string{},
		},
		{
			name:           "empty string returns empty slice",
			inputVariables: "",
			wantVars:       nil,
		},
		{
			name:           "malformed JSON returns empty slice",
			inputVariables: "not-json-at-all",
			wantVars:       nil,
		},
		{
			name:           "all four built-in variables",
			inputVariables: `["SPEC_CONTENT","PROJECT_ROOT","TASK_DESCRIPTION","WORKTREE_PATH"]`,
			wantVars:       []string{"SPEC_CONTENT", "PROJECT_ROOT", "TASK_DESCRIPTION", "WORKTREE_PATH"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpl := &db.PhaseTemplate{
				ID:             "test-phase",
				Name:           "Test Phase",
				InputVariables: tt.inputVariables,
				PromptSource:   "db",
			}

			proto := dbPhaseTemplateToProto(tmpl)

			if tt.wantVars == nil {
				if len(proto.InputVariables) != 0 {
					t.Errorf("expected no input variables, got %v", proto.InputVariables)
				}
				return
			}

			if len(proto.InputVariables) != len(tt.wantVars) {
				t.Fatalf("expected %d input variables, got %d: %v",
					len(tt.wantVars), len(proto.InputVariables), proto.InputVariables)
			}
			for i, want := range tt.wantVars {
				if proto.InputVariables[i] != want {
					t.Errorf("InputVariables[%d]: expected %q, got %q",
						i, want, proto.InputVariables[i])
				}
			}
		})
	}
}

// Verify existing fields are still populated when InputVariables is set.
// Guards against regressions where adding InputVariables handling breaks
// other field mappings.
func TestDbPhaseTemplateToProto_InputVariablesWithOtherFields(t *testing.T) {
	t.Parallel()

	tmpl := &db.PhaseTemplate{
		ID:             "analysis",
		Name:           "Analysis Phase",
		Description:    "Analyze the codebase",
		InputVariables: `["SPEC_CONTENT","TASK_DESCRIPTION"]`,
		OutputVarName:  "ANALYSIS_REPORT",
		PromptSource:   "file",
		PromptPath:     "analysis.md",
		GateType:       "human",
		AgentID:        "claude-agent",
	}

	proto := dbPhaseTemplateToProto(tmpl)

	// InputVariables should be populated
	if len(proto.InputVariables) != 2 {
		t.Fatalf("expected 2 input variables, got %d: %v",
			len(proto.InputVariables), proto.InputVariables)
	}
	if proto.InputVariables[0] != "SPEC_CONTENT" {
		t.Errorf("InputVariables[0]: expected SPEC_CONTENT, got %q", proto.InputVariables[0])
	}
	if proto.InputVariables[1] != "TASK_DESCRIPTION" {
		t.Errorf("InputVariables[1]: expected TASK_DESCRIPTION, got %q", proto.InputVariables[1])
	}

	// Other fields should still be populated correctly
	if proto.Id != "analysis" {
		t.Errorf("expected Id=analysis, got %q", proto.Id)
	}
	if proto.Name != "Analysis Phase" {
		t.Errorf("expected Name=Analysis Phase, got %q", proto.Name)
	}
	if proto.Description == nil || *proto.Description != "Analyze the codebase" {
		t.Errorf("expected Description=Analyze the codebase, got %v", proto.Description)
	}
	if proto.OutputVarName == nil || *proto.OutputVarName != "ANALYSIS_REPORT" {
		t.Errorf("expected OutputVarName=ANALYSIS_REPORT, got %v", proto.OutputVarName)
	}
	if proto.PromptPath == nil || *proto.PromptPath != "analysis.md" {
		t.Errorf("expected PromptPath=analysis.md, got %v", proto.PromptPath)
	}
	if proto.AgentId == nil || *proto.AgentId != "claude-agent" {
		t.Errorf("expected AgentId=claude-agent, got %v", proto.AgentId)
	}
}
