package executor

import (
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

func TestQualityCheckResult_AsContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   *QualityCheckResult
		wantEmpty bool
		contains []string
	}{
		{
			name: "all passed returns empty",
			result: &QualityCheckResult{
				AllPassed: true,
				Checks: []CheckResult{
					{Name: "tests", Passed: true},
				},
			},
			wantEmpty: true,
		},
		{
			name: "failed test includes output",
			result: &QualityCheckResult{
				AllPassed: false,
				Checks: []CheckResult{
					{Name: "tests", Passed: false, Output: "FAIL: TestFoo\n    expected 1, got 2", OnFailure: "block"},
				},
			},
			wantEmpty: false,
			contains: []string{
				"Quality Check Failures",
				"Tests",
				"FAIL: TestFoo",
				"expected 1, got 2",
			},
		},
		{
			name: "skipped checks not included",
			result: &QualityCheckResult{
				AllPassed: false,
				Checks: []CheckResult{
					{Name: "tests", Passed: false, Output: "test failure", OnFailure: "block"},
					{Name: "lint", Skipped: true},
				},
			},
			wantEmpty: false,
			contains:  []string{"Tests", "test failure"},
		},
		{
			name: "warning checks included with severity",
			result: &QualityCheckResult{
				AllPassed: false,
				Checks: []CheckResult{
					{Name: "lint", Passed: false, Output: "warning: unused var", OnFailure: "warn"},
				},
			},
			wantEmpty: false,
			contains:  []string{"(Warning)", "Lint"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.AsContext()
			if tt.wantEmpty && got != "" {
				t.Errorf("AsContext() = %q, want empty", got)
			}
			if !tt.wantEmpty && got == "" {
				t.Error("AsContext() = empty, want non-empty")
			}
			for _, want := range tt.contains {
				if !contains(got, want) {
					t.Errorf("AsContext() missing %q", want)
				}
			}
		})
	}
}

func TestQualityCheckResult_FailureSummary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		result *QualityCheckResult
		want   string
	}{
		{
			name: "all passed",
			result: &QualityCheckResult{
				AllPassed: true,
				Checks: []CheckResult{
					{Name: "tests", Passed: true},
				},
			},
			want: "all checks passed",
		},
		{
			name: "single failure",
			result: &QualityCheckResult{
				AllPassed: false,
				Checks: []CheckResult{
					{Name: "tests", Passed: false},
				},
			},
			want: "tests failed",
		},
		{
			name: "multiple failures",
			result: &QualityCheckResult{
				AllPassed: false,
				Checks: []CheckResult{
					{Name: "tests", Passed: false},
					{Name: "lint", Passed: false},
				},
			},
			want: "tests, lint failed",
		},
		{
			name: "some passed some failed",
			result: &QualityCheckResult{
				AllPassed: false,
				Checks: []CheckResult{
					{Name: "tests", Passed: true},
					{Name: "lint", Passed: false},
					{Name: "build", Passed: true},
				},
			},
			want: "lint failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.FailureSummary()
			if got != tt.want {
				t.Errorf("FailureSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadQualityChecksForPhase(t *testing.T) {
	t.Parallel()

	codeChecks := `[{"type":"code","name":"tests","enabled":true,"on_failure":"block"},{"type":"code","name":"lint","enabled":true,"on_failure":"warn"}]`
	overrideChecks := `[{"type":"code","name":"tests","enabled":false}]`

	tests := []struct {
		name          string
		template      *db.PhaseTemplate
		workflowPhase *db.WorkflowPhase
		wantNames     []string
		wantErr       bool
	}{
		{
			name: "loads from template",
			template: &db.PhaseTemplate{
				ID:            "implement",
				QualityChecks: codeChecks,
			},
			workflowPhase: nil,
			wantNames:     []string{"tests", "lint"},
		},
		{
			name: "nil template returns empty",
			template: nil,
			workflowPhase: nil,
			wantNames:     []string{},
		},
		{
			name: "empty quality_checks returns empty",
			template: &db.PhaseTemplate{
				ID:            "implement",
				QualityChecks: "",
			},
			wantNames: []string{},
		},
		{
			name: "workflow override takes precedence",
			template: &db.PhaseTemplate{
				ID:            "implement",
				QualityChecks: codeChecks,
			},
			workflowPhase: &db.WorkflowPhase{
				WorkflowID:            "test",
				PhaseTemplateID:       "implement",
				QualityChecksOverride: overrideChecks,
			},
			wantNames: []string{"tests"},
		},
		{
			name: "invalid JSON returns error",
			template: &db.PhaseTemplate{
				ID:            "implement",
				QualityChecks: "not json",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadQualityChecksForPhase(tt.template, tt.workflowPhase)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadQualityChecksForPhase() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(got) != len(tt.wantNames) {
				t.Errorf("LoadQualityChecksForPhase() returned %d checks, want %d", len(got), len(tt.wantNames))
				return
			}

			for i, name := range tt.wantNames {
				if got[i].Name != name {
					t.Errorf("check[%d].Name = %q, want %q", i, got[i].Name, name)
				}
			}
		})
	}
}

func TestFormatQualityChecksForPrompt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		result   *QualityCheckResult
		contains []string
	}{
		{
			name:     "nil result returns empty",
			result:   nil,
			contains: []string{},
		},
		{
			name: "all passed returns empty",
			result: &QualityCheckResult{
				AllPassed: true,
			},
			contains: []string{},
		},
		{
			name: "failures formatted for prompt",
			result: &QualityCheckResult{
				AllPassed: false,
				Checks: []CheckResult{
					{Name: "tests", Passed: false, Output: "test failure output", OnFailure: "block"},
				},
				Duration: 5 * time.Second,
			},
			contains: []string{
				"Quality Check Failures",
				"Tests",
				"test failure output",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatQualityChecksForPrompt(tt.result)
			for _, want := range tt.contains {
				if !contains(got, want) {
					t.Errorf("FormatQualityChecksForPrompt() missing %q", want)
				}
			}
		})
	}
}

func TestNewQualityCheckRunner(t *testing.T) {
	t.Parallel()

	checks := []db.QualityCheck{
		{Type: "code", Name: "tests", Enabled: true, OnFailure: "block"},
	}
	commands := map[string]*db.ProjectCommand{
		"tests": {Name: "tests", Command: "go test ./..."},
	}

	runner := NewQualityCheckRunner("/tmp", checks, commands, nil)
	if runner == nil {
		t.Fatal("NewQualityCheckRunner() returned nil")
	}
	if runner.workDir != "/tmp" {
		t.Errorf("workDir = %q, want /tmp", runner.workDir)
	}
	if len(runner.checks) != 1 {
		t.Errorf("checks count = %d, want 1", len(runner.checks))
	}
	if runner.shell == "" {
		t.Error("shell not detected")
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
