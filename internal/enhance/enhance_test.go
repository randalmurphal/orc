package enhance

import (
	"testing"
)

func TestQuickEnhance(t *testing.T) {
	result, err := quickEnhance("Fix login bug", "small")
	if err != nil {
		t.Fatalf("quickEnhance failed: %v", err)
	}

	if result.Weight != "small" {
		t.Errorf("expected weight 'small', got %q", result.Weight)
	}
	if result.OriginalTitle != "Fix login bug" {
		t.Errorf("original title mismatch")
	}
	if result.EnhancedTitle != "Fix login bug" {
		t.Errorf("enhanced title should match original for quick mode")
	}
}

func TestQuickEnhance_DefaultWeight(t *testing.T) {
	result, err := quickEnhance("Some task", "")
	if err != nil {
		t.Fatalf("quickEnhance failed: %v", err)
	}

	if result.Weight != "medium" {
		t.Errorf("expected default weight 'medium', got %q", result.Weight)
	}
}

func TestQuickEnhance_InvalidWeight(t *testing.T) {
	_, err := quickEnhance("Some task", "invalid")
	if err == nil {
		t.Error("expected error for invalid weight")
	}
}

func TestQuickEnhance_AllWeights(t *testing.T) {
	weights := []string{"trivial", "small", "medium", "large", "greenfield"}

	for _, weight := range weights {
		t.Run(weight, func(t *testing.T) {
			result, err := quickEnhance("Task", weight)
			if err != nil {
				t.Fatalf("unexpected error for weight %q: %v", weight, err)
			}
			if result.Weight != weight {
				t.Errorf("expected weight %q, got %q", weight, result.Weight)
			}
		})
	}
}

func TestExtractTag(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		tag      string
		expected string
	}{
		{
			name:     "simple tag",
			content:  "<title>Hello World</title>",
			tag:      "title",
			expected: "Hello World",
		},
		{
			name: "multiline tag",
			content: `<description>
Line 1
Line 2
</description>`,
			tag:      "description",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "tag with whitespace",
			content:  "<weight>  medium  </weight>",
			tag:      "weight",
			expected: "medium",
		},
		{
			name:     "missing tag",
			content:  "<other>value</other>",
			tag:      "missing",
			expected: "",
		},
		{
			name:     "nested content",
			content:  "<analysis><scope>Local scope</scope></analysis>",
			tag:      "scope",
			expected: "Local scope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTag(tt.content, tt.tag)
			if got != tt.expected {
				t.Errorf("extractTag(%q, %q) = %q, want %q", tt.content, tt.tag, got, tt.expected)
			}
		})
	}
}

func TestSplitList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a, b, c", []string{"a", "b", "c"}},
		{"single", []string{"single"}},
		{"", nil},
		{"none", nil},
		{"unknown", nil},
		{"  spaced  ,  items  ", []string{"spaced", "items"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitList(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("splitList(%q) = %v, want %v", tt.input, got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("splitList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestParseEnhanceResponse(t *testing.T) {
	response := `Here's the analysis:

<enhanced_title>Fix authentication timeout on login page</enhanced_title>

<description>
The login page times out after 30 seconds, which is too short for slow connections.
- Increase timeout to 60 seconds
- Add retry logic
- Show loading indicator
</description>

<weight>small</weight>

<analysis>
<scope>Single component change</scope>
<affected_files>src/auth/login.ts, src/auth/config.ts</affected_files>
<risks>none</risks>
<dependencies>none</dependencies>
<test_strategy>Unit test timeout handling, E2E test login flow</test_strategy>
</analysis>`

	result, err := parseEnhanceResponse("Fix login bug", response)
	if err != nil {
		t.Fatalf("parseEnhanceResponse failed: %v", err)
	}

	if result.OriginalTitle != "Fix login bug" {
		t.Errorf("original title mismatch")
	}

	if result.EnhancedTitle != "Fix authentication timeout on login page" {
		t.Errorf("enhanced title = %q", result.EnhancedTitle)
	}

	if result.Weight != "small" {
		t.Errorf("weight = %q, want 'small'", result.Weight)
	}

	if result.Analysis == nil {
		t.Fatal("analysis is nil")
	}

	if result.Analysis.Scope != "Single component change" {
		t.Errorf("scope = %q", result.Analysis.Scope)
	}

	if len(result.Analysis.AffectedFiles) != 2 {
		t.Errorf("affected files = %v", result.Analysis.AffectedFiles)
	}
}

func TestParseEnhanceResponse_InvalidWeight(t *testing.T) {
	response := `<enhanced_title>Task</enhanced_title>
<description>Description</description>
<weight>invalid</weight>`

	result, err := parseEnhanceResponse("Task", response)
	if err != nil {
		t.Fatalf("parseEnhanceResponse failed: %v", err)
	}

	// Should default to medium
	if result.Weight != "medium" {
		t.Errorf("expected default weight 'medium', got %q", result.Weight)
	}
}

func TestParseEnhanceResponse_EmptyResponse(t *testing.T) {
	result, err := parseEnhanceResponse("Original Task", "")
	if err != nil {
		t.Fatalf("parseEnhanceResponse failed: %v", err)
	}

	// Should fallback to original
	if result.EnhancedTitle != "Original Task" {
		t.Errorf("enhanced title should fallback to original")
	}
	if result.Weight != "medium" {
		t.Errorf("weight should default to medium")
	}
}

func TestBuildEnhancePrompt(t *testing.T) {
	prompt := buildEnhancePrompt("Add dark mode")

	if prompt == "" {
		t.Error("prompt should not be empty")
	}

	// Should contain the task
	if !contains(prompt, "Add dark mode") {
		t.Error("prompt should contain the task title")
	}

	// Should contain weight guidelines
	if !contains(prompt, "trivial") || !contains(prompt, "greenfield") {
		t.Error("prompt should contain weight guidelines")
	}

	// Should contain expected XML tags
	if !contains(prompt, "<enhanced_title>") || !contains(prompt, "<weight>") {
		t.Error("prompt should contain expected XML tags")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()))
}
