package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashContent(t *testing.T) {
	content := "Hello, World!"
	hash1 := HashContent(content)
	hash2 := HashContent(content)

	if hash1 != hash2 {
		t.Errorf("HashContent() not deterministic: %s != %s", hash1, hash2)
	}

	differentHash := HashContent("Different content")
	if hash1 == differentHash {
		t.Error("HashContent() returned same hash for different content")
	}

	if len(hash1) != 64 {
		t.Errorf("HashContent() returned unexpected length: %d", len(hash1))
	}
}

func TestSaveSpecTo(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task-001")

	specContent := "# Test Spec\n\nThis is a test specification."

	// Save spec
	err := SaveSpecTo(taskDir, specContent, "test")
	if err != nil {
		t.Fatalf("SaveSpecTo() error: %v", err)
	}

	// Verify files exist
	specPath := filepath.Join(taskDir, "spec.md")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		t.Error("Spec file was not created")
	}

	metaPath := filepath.Join(taskDir, "spec_meta.yaml")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("Spec metadata file was not created")
	}

	// Verify content
	data, _ := os.ReadFile(specPath)
	if string(data) != specContent {
		t.Errorf("Spec content mismatch: got %q, want %q", string(data), specContent)
	}
}

func TestLoadSpecFrom(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "task-001")

	specContent := "# Load Test Spec"
	SaveSpecTo(taskDir, specContent, "test-source")

	spec, err := LoadSpecFrom(taskDir)
	if err != nil {
		t.Fatalf("LoadSpecFrom() error: %v", err)
	}
	if spec == nil {
		t.Fatal("LoadSpecFrom() returned nil")
	}

	if spec.Content != specContent {
		t.Errorf("Spec content = %q, want %q", spec.Content, specContent)
	}

	if spec.Metadata.Source != "test-source" {
		t.Errorf("Spec source = %q, want %q", spec.Metadata.Source, "test-source")
	}

	expectedHash := HashContent(specContent)
	if spec.Metadata.Hash != expectedHash {
		t.Errorf("Spec hash = %q, want %q", spec.Metadata.Hash, expectedHash)
	}
}

func TestLoadSpecFromNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "nonexistent")

	spec, err := LoadSpecFrom(taskDir)
	if err != nil {
		t.Errorf("LoadSpecFrom() for non-existent should return nil, not error: %v", err)
	}
	if spec != nil {
		t.Error("LoadSpecFrom() for non-existent should return nil")
	}
}

func TestSpecChangedLogic(t *testing.T) {
	// Test the hash comparison logic directly
	content1 := "Original content"
	content2 := "Different content"

	hash1 := HashContent(content1)
	hash2 := HashContent(content2)

	if hash1 == hash2 {
		t.Error("Different content should have different hashes")
	}

	// Same content should have same hash
	hash1Again := HashContent(content1)
	if hash1 != hash1Again {
		t.Error("Same content should have same hash")
	}
}

func TestSaveAndLoadSpecRoundtrip(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, "roundtrip-task")

	specContent := `# Feature Spec

## Overview
This is a comprehensive test spec.

## Requirements
- Requirement 1
- Requirement 2

## Technical Details
Some technical details here.
`

	// Save
	err := SaveSpecTo(taskDir, specContent, "generated")
	if err != nil {
		t.Fatalf("SaveSpecTo() error: %v", err)
	}

	// Load
	spec, err := LoadSpecFrom(taskDir)
	if err != nil {
		t.Fatalf("LoadSpecFrom() error: %v", err)
	}

	// Verify roundtrip
	if spec.Content != specContent {
		t.Errorf("Content changed during roundtrip")
	}
	if spec.Metadata.Source != "generated" {
		t.Errorf("Source = %q, want 'generated'", spec.Metadata.Source)
	}
	if spec.Metadata.LastSyncedAt.IsZero() {
		t.Error("LastSyncedAt should be set")
	}
}

func TestValidateSpec_Valid(t *testing.T) {
	content := `# Task Spec

## Intent

This is a well-defined intent section explaining the purpose of the task.

## Success Criteria

- Criterion 1: Feature works correctly
- Criterion 2: All edge cases handled
- Criterion 3: Performance meets requirements

## Testing

Unit tests cover all public functions. Integration tests verify end-to-end behavior.
`

	tests := []struct {
		name   string
		weight Weight
	}{
		{"small weight", WeightSmall},
		{"medium weight", WeightMedium},
		{"large weight", WeightLarge},
		{"greenfield weight", WeightGreenfield},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSpec(content, tt.weight)

			if !result.Valid {
				t.Errorf("ValidateSpec() Valid = false, want true; issues: %v", result.Issues)
			}
			if !result.HasIntent {
				t.Error("ValidateSpec() HasIntent = false, want true")
			}
			if !result.HasSuccessCriteria {
				t.Error("ValidateSpec() HasSuccessCriteria = false, want true")
			}
			if !result.HasTesting {
				t.Error("ValidateSpec() HasTesting = false, want true")
			}
			if len(result.Issues) != 0 {
				t.Errorf("ValidateSpec() Issues = %v, want empty", result.Issues)
			}
		})
	}
}

func TestValidateSpec_MissingIntent(t *testing.T) {
	content := `# Task Spec

## Success Criteria

- Criterion 1: Feature works correctly
- Criterion 2: All edge cases handled

## Testing

Unit tests cover all public functions.
`

	result := ValidateSpec(content, WeightSmall)

	if result.Valid {
		t.Error("ValidateSpec() Valid = true, want false for missing Intent")
	}
	if result.HasIntent {
		t.Error("ValidateSpec() HasIntent = true, want false")
	}
	if !result.HasSuccessCriteria {
		t.Error("ValidateSpec() HasSuccessCriteria = false, want true")
	}
	if !result.HasTesting {
		t.Error("ValidateSpec() HasTesting = false, want true")
	}

	foundIssue := false
	for _, issue := range result.Issues {
		if issue == "Missing Intent section" {
			foundIssue = true
			break
		}
	}
	if !foundIssue {
		t.Errorf("ValidateSpec() Issues should contain 'Missing Intent section', got %v", result.Issues)
	}
}

func TestValidateSpec_MissingSuccessCriteria(t *testing.T) {
	content := `# Task Spec

## Intent

This is a well-defined intent section explaining the purpose of the task.

## Testing

Unit tests cover all public functions and edge cases.
`

	result := ValidateSpec(content, WeightMedium)

	if result.Valid {
		t.Error("ValidateSpec() Valid = true, want false for missing Success Criteria")
	}
	if !result.HasIntent {
		t.Error("ValidateSpec() HasIntent = false, want true")
	}
	if result.HasSuccessCriteria {
		t.Error("ValidateSpec() HasSuccessCriteria = true, want false")
	}
	if !result.HasTesting {
		t.Error("ValidateSpec() HasTesting = false, want true")
	}

	foundIssue := false
	for _, issue := range result.Issues {
		if issue == "Missing Success Criteria section" {
			foundIssue = true
			break
		}
	}
	if !foundIssue {
		t.Errorf("ValidateSpec() Issues should contain 'Missing Success Criteria section', got %v", result.Issues)
	}
}

func TestValidateSpec_MissingTesting(t *testing.T) {
	content := `# Task Spec

## Intent

This is a well-defined intent section explaining the purpose of the task.

## Success Criteria

- Criterion 1: Feature works correctly
- Criterion 2: All edge cases handled
`

	result := ValidateSpec(content, WeightLarge)

	if result.Valid {
		t.Error("ValidateSpec() Valid = true, want false for missing Testing")
	}
	if !result.HasIntent {
		t.Error("ValidateSpec() HasIntent = false, want true")
	}
	if !result.HasSuccessCriteria {
		t.Error("ValidateSpec() HasSuccessCriteria = false, want true")
	}
	if result.HasTesting {
		t.Error("ValidateSpec() HasTesting = true, want false")
	}

	foundIssue := false
	for _, issue := range result.Issues {
		if issue == "Missing Testing section" {
			foundIssue = true
			break
		}
	}
	if !foundIssue {
		t.Errorf("ValidateSpec() Issues should contain 'Missing Testing section', got %v", result.Issues)
	}
}

func TestValidateSpec_EmptySections(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectIntent  bool
		expectCrit    bool
		expectTesting bool
		expectIssue   string
	}{
		{
			name: "empty intent",
			content: `# Task Spec

## Intent

## Success Criteria

- Criterion 1: Feature works correctly

## Testing

Unit tests cover all public functions and edge cases.
`,
			expectIntent:  false,
			expectCrit:    true,
			expectTesting: true,
			expectIssue:   "Intent section exists but is empty",
		},
		{
			name: "empty success criteria",
			content: `# Task Spec

## Intent

This is a well-defined intent section explaining the purpose.

## Success Criteria

## Testing

Unit tests cover all public functions and edge cases.
`,
			expectIntent:  true,
			expectCrit:    false,
			expectTesting: true,
			expectIssue:   "Success Criteria section exists but has no items",
		},
		{
			name: "empty testing",
			content: `# Task Spec

## Intent

This is a well-defined intent section explaining the purpose.

## Success Criteria

- Criterion 1: Feature works correctly

## Testing

`,
			expectIntent:  true,
			expectCrit:    true,
			expectTesting: false,
			expectIssue:   "Testing section exists but is empty",
		},
		{
			name: "too short content (whitespace only)",
			content: `# Task Spec

## Intent



## Success Criteria

- OK content here for testing

## Testing

Valid testing content here for the test.
`,
			expectIntent:  false,
			expectCrit:    true,
			expectTesting: true,
			expectIssue:   "Intent section exists but is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSpec(tt.content, WeightSmall)

			if result.Valid {
				t.Error("ValidateSpec() Valid = true, want false for empty section")
			}
			if result.HasIntent != tt.expectIntent {
				t.Errorf("ValidateSpec() HasIntent = %v, want %v", result.HasIntent, tt.expectIntent)
			}
			if result.HasSuccessCriteria != tt.expectCrit {
				t.Errorf("ValidateSpec() HasSuccessCriteria = %v, want %v", result.HasSuccessCriteria, tt.expectCrit)
			}
			if result.HasTesting != tt.expectTesting {
				t.Errorf("ValidateSpec() HasTesting = %v, want %v", result.HasTesting, tt.expectTesting)
			}

			foundIssue := false
			for _, issue := range result.Issues {
				if issue == tt.expectIssue {
					foundIssue = true
					break
				}
			}
			if !foundIssue {
				t.Errorf("ValidateSpec() Issues should contain %q, got %v", tt.expectIssue, result.Issues)
			}
		})
	}
}

func TestValidateSpec_TrivialWeight(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{
			name:    "empty content",
			content: "",
		},
		{
			name:    "minimal content",
			content: "# Just a title",
		},
		{
			name:    "missing all sections",
			content: "# Task Spec\n\nSome random text without proper sections.",
		},
		{
			name: "partial sections",
			content: `# Task Spec

## Intent

Short.
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSpec(tt.content, WeightTrivial)

			if !result.Valid {
				t.Errorf("ValidateSpec() Valid = false, want true for trivial weight; issues: %v", result.Issues)
			}
			if !result.HasIntent {
				t.Error("ValidateSpec() HasIntent = false, want true for trivial weight")
			}
			if !result.HasSuccessCriteria {
				t.Error("ValidateSpec() HasSuccessCriteria = false, want true for trivial weight")
			}
			if !result.HasTesting {
				t.Error("ValidateSpec() HasTesting = false, want true for trivial weight")
			}
			if len(result.Issues) != 0 {
				t.Errorf("ValidateSpec() Issues = %v, want empty for trivial weight", result.Issues)
			}
		})
	}
}

func TestHasValidSpec(t *testing.T) {
	tests := []struct {
		name        string
		specContent string
		weight      Weight
		wantValid   bool
	}{
		{
			name: "valid spec",
			specContent: `# Task Spec

## Intent

This is a well-defined intent section explaining the purpose of the task.

## Success Criteria

- Criterion 1: Feature works correctly
- Criterion 2: All edge cases handled

## Testing

Unit tests cover all public functions and edge cases.
`,
			weight:    WeightSmall,
			wantValid: true,
		},
		{
			name: "invalid spec missing sections",
			specContent: `# Task Spec

Just some text without proper sections.
`,
			weight:    WeightMedium,
			wantValid: false,
		},
		{
			name: "invalid spec becomes valid with trivial weight",
			specContent: `# Task Spec

Minimal content.
`,
			weight:    WeightTrivial,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TEST-001")

			// Save spec to task directory
			err := SaveSpecTo(taskDir, tt.specContent, "test")
			if err != nil {
				t.Fatalf("SaveSpecTo() error: %v", err)
			}

			// Test HasValidSpec using LoadSpecFrom and ValidateSpec directly
			// since HasValidSpec uses OrcDir which is process-wide
			spec, err := LoadSpecFrom(taskDir)
			if err != nil {
				t.Fatalf("LoadSpecFrom() error: %v", err)
			}
			if spec == nil {
				t.Fatal("LoadSpecFrom() returned nil")
			}

			result := ValidateSpec(spec.Content, tt.weight)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateSpec() Valid = %v, want %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestHasValidSpec_NoSpecFile(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TEST-001")

	// Create task directory but no spec file
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	spec, err := LoadSpecFrom(taskDir)
	if err != nil {
		t.Errorf("LoadSpecFrom() should return nil, not error: %v", err)
	}
	if spec != nil {
		t.Error("LoadSpecFrom() should return nil when spec doesn't exist")
	}
}

func TestHasSection(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		sectionName string
		want        bool
	}{
		{
			name:        "h2 header exact match",
			content:     "# Title\n\n## Intent\n\nSome content",
			sectionName: "intent",
			want:        true,
		},
		{
			name:        "h1 header exact match",
			content:     "# Intent\n\nSome content",
			sectionName: "intent",
			want:        true,
		},
		{
			name:        "case insensitive match lowercase",
			content:     "# Title\n\n## intent\n\nSome content",
			sectionName: "Intent",
			want:        true,
		},
		{
			name:        "case insensitive match uppercase",
			content:     "# Title\n\n## INTENT\n\nSome content",
			sectionName: "intent",
			want:        true,
		},
		{
			name:        "case insensitive mixed case",
			content:     "# Title\n\n## InTenT\n\nSome content",
			sectionName: "intent",
			want:        true,
		},
		{
			name:        "multi-word section",
			content:     "# Title\n\n## Success Criteria\n\nSome content",
			sectionName: "success criteria",
			want:        true,
		},
		{
			name:        "section not found",
			content:     "# Title\n\n## Overview\n\nSome content",
			sectionName: "intent",
			want:        false,
		},
		{
			name:        "partial match does match (prefix)",
			content:     "# Title\n\n## Intentional\n\nSome content",
			sectionName: "intent",
			want:        true, // hasSection uses prefix matching, not exact
		},
		{
			name:        "empty content",
			content:     "",
			sectionName: "intent",
			want:        false,
		},
		{
			name:        "header with extra spaces",
			content:     "# Title\n\n##  Intent\n\nSome content",
			sectionName: "intent",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSection(tt.content, tt.sectionName)
			if got != tt.want {
				t.Errorf("hasSection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractSection(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		sectionName string
		want        string
	}{
		{
			name: "extract section between headers",
			content: `# Title

## Intent

This is the intent content.

## Success Criteria

Criteria here.
`,
			sectionName: "intent",
			want:        "This is the intent content.",
		},
		{
			name: "extract section to end of file",
			content: `# Title

## Intent

This is content.

## Testing

This is the testing content.
It spans multiple lines.
`,
			sectionName: "testing",
			want:        "This is the testing content.\nIt spans multiple lines.",
		},
		{
			name:        "section not found returns empty",
			content:     "# Title\n\n## Overview\n\nSome content",
			sectionName: "intent",
			want:        "",
		},
		{
			name: "extract with list content",
			content: `# Spec

## Success Criteria

- Item 1
- Item 2
- Item 3

## Testing

Test content.
`,
			sectionName: "success criteria",
			want:        "- Item 1\n- Item 2\n- Item 3",
		},
		{
			name: "extract h1 section",
			content: `# Intent

The main intent of this task.

# Other Section

Other content.
`,
			sectionName: "intent",
			want:        "The main intent of this task.",
		},
		{
			name:        "empty content",
			content:     "",
			sectionName: "intent",
			want:        "",
		},
		{
			name: "section with only whitespace",
			content: `# Title

## Intent



## Next
`,
			sectionName: "intent",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSection(tt.content, tt.sectionName)
			if got != tt.want {
				t.Errorf("extractSection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasMeaningfulContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "empty string",
			content: "",
			want:    false,
		},
		{
			name:    "whitespace only",
			content: "   \n\t\n   ",
			want:    false,
		},
		{
			name:    "short content (10 chars exactly)",
			content: "1234567890",
			want:    false,
		},
		{
			name:    "short content (under 10 chars)",
			content: "short",
			want:    false,
		},
		{
			name:    "valid content (11 chars)",
			content: "12345678901",
			want:    true,
		},
		{
			name:    "valid longer content",
			content: "This is meaningful content that describes something important.",
			want:    true,
		},
		{
			name:    "content with leading/trailing whitespace",
			content: "   valid content here   ",
			want:    true,
		},
		{
			name:    "multiline content",
			content: "Line 1\nLine 2\nLine 3",
			want:    true,
		},
		{
			name:    "list items",
			content: "- Item 1\n- Item 2",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMeaningfulContent(tt.content)
			if got != tt.want {
				t.Errorf("hasMeaningfulContent(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}
