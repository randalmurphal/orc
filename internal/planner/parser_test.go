package planner

import (
	"testing"

	"github.com/randalmurphal/orc/internal/task"
)

func TestParseTaskBreakdown_Valid(t *testing.T) {
	response := `Here is my analysis of the spec.

` + "```json" + `
{
  "summary": "User authentication feature implementation",
  "tasks": [
    {
      "id": 1,
      "title": "Create User model",
      "description": "Define the User model with email, password_hash, created_at fields.",
      "weight": "small",
      "depends_on": []
    },
    {
      "id": 2,
      "title": "Add password hashing",
      "description": "Implement bcrypt-based password hashing.",
      "weight": "trivial",
      "depends_on": [1]
    },
    {
      "id": 3,
      "title": "Create registration endpoint",
      "description": "POST /api/auth/register with validation.",
      "weight": "medium",
      "depends_on": [1, 2]
    }
  ]
}
` + "```"

	breakdown, err := ParseTaskBreakdown(response)
	if err != nil {
		t.Fatalf("ParseTaskBreakdown failed: %v", err)
	}

	if len(breakdown.Tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(breakdown.Tasks))
	}

	// Check summary
	if breakdown.Summary != "User authentication feature implementation" {
		t.Errorf("Summary = %q, want %q", breakdown.Summary, "User authentication feature implementation")
	}

	// Check first task
	if breakdown.Tasks[0].Title != "Create User model" {
		t.Errorf("Task 1 title = %q, want %q", breakdown.Tasks[0].Title, "Create User model")
	}
	if breakdown.Tasks[0].Weight != task.WeightSmall {
		t.Errorf("Task 1 weight = %q, want %q", breakdown.Tasks[0].Weight, task.WeightSmall)
	}
	if len(breakdown.Tasks[0].DependsOn) != 0 {
		t.Errorf("Task 1 should have no dependencies, got %v", breakdown.Tasks[0].DependsOn)
	}

	// Check task with dependencies
	if len(breakdown.Tasks[2].DependsOn) != 2 {
		t.Errorf("Task 3 should have 2 dependencies, got %v", breakdown.Tasks[2].DependsOn)
	}
	if breakdown.Tasks[2].DependsOn[0] != 1 || breakdown.Tasks[2].DependsOn[1] != 2 {
		t.Errorf("Task 3 dependencies = %v, want [1, 2]", breakdown.Tasks[2].DependsOn)
	}
}

func TestParseTaskBreakdown_RawJSON(t *testing.T) {
	// Test parsing raw JSON without markdown code blocks
	response := `{
  "summary": "Simple task list",
  "tasks": [
    {
      "id": 1,
      "title": "Single task",
      "description": "Just one task",
      "weight": "small",
      "depends_on": []
    }
  ]
}`

	breakdown, err := ParseTaskBreakdown(response)
	if err != nil {
		t.Fatalf("ParseTaskBreakdown failed: %v", err)
	}

	if len(breakdown.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(breakdown.Tasks))
	}
}

func TestParseTaskBreakdown_NoTasks(t *testing.T) {
	response := `{
  "summary": "Empty breakdown",
  "tasks": []
}`

	_, err := ParseTaskBreakdown(response)
	if err == nil {
		t.Error("Expected error for empty task breakdown")
	}
}

func TestParseTaskBreakdown_NoJSON(t *testing.T) {
	response := "Just some text without any task breakdown."

	_, err := ParseTaskBreakdown(response)
	if err == nil {
		t.Error("Expected error when no task breakdown found")
	}
}

func TestParseTaskBreakdown_InvalidJSON(t *testing.T) {
	response := `{
  "summary": "Broken JSON",
  "tasks": [
    {invalid json}
  ]
}`

	_, err := ParseTaskBreakdown(response)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "json code block",
			content: "Some text\n```json\n{\"key\": \"value\"}\n```\nMore text",
			want:    `{"key": "value"}`,
		},
		{
			name:    "generic code block",
			content: "Some text\n```\n{\"key\": \"value\"}\n```\nMore text",
			want:    `{"key": "value"}`,
		},
		{
			name:    "raw json",
			content: `Some text {"key": "value"} more text`,
			want:    `{"key": "value"}`,
		},
		{
			name:    "nested json",
			content: `{"outer": {"inner": "value"}}`,
			want:    `{"outer": {"inner": "value"}}`,
		},
		{
			name:    "no json",
			content: "No JSON here",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.content)
			if got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeWeight(t *testing.T) {
	tests := []struct {
		input string
		want  task.Weight
	}{
		{"trivial", task.WeightTrivial},
		{"TRIVIAL", task.WeightTrivial},
		{"small", task.WeightSmall},
		{"medium", task.WeightMedium},
		{"large", task.WeightLarge},
		{"greenfield", task.WeightGreenfield},
		{"unknown", task.WeightMedium}, // default
		{"", task.WeightMedium},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeWeight(tt.input)
			if got != tt.want {
				t.Errorf("normalizeWeight(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateDependencies_Valid(t *testing.T) {
	breakdown := &TaskBreakdown{
		Tasks: []*ProposedTask{
			{Index: 1, DependsOn: nil},
			{Index: 2, DependsOn: []int{1}},
			{Index: 3, DependsOn: []int{1, 2}},
		},
	}

	if err := ValidateDependencies(breakdown); err != nil {
		t.Errorf("ValidateDependencies should pass: %v", err)
	}
}

func TestValidateDependencies_NonExistent(t *testing.T) {
	breakdown := &TaskBreakdown{
		Tasks: []*ProposedTask{
			{Index: 1, DependsOn: nil},
			{Index: 2, DependsOn: []int{99}}, // 99 doesn't exist
		},
	}

	if err := ValidateDependencies(breakdown); err == nil {
		t.Error("ValidateDependencies should fail for non-existent dependency")
	}
}

func TestValidateDependencies_ForwardReference(t *testing.T) {
	breakdown := &TaskBreakdown{
		Tasks: []*ProposedTask{
			{Index: 1, DependsOn: []int{2}}, // Forward reference
			{Index: 2, DependsOn: nil},
		},
	}

	if err := ValidateDependencies(breakdown); err == nil {
		t.Error("ValidateDependencies should fail for forward reference")
	}
}

func TestValidateDependencies_Circular(t *testing.T) {
	breakdown := &TaskBreakdown{
		Tasks: []*ProposedTask{
			{Index: 1, DependsOn: []int{3}},
			{Index: 2, DependsOn: []int{1}},
			{Index: 3, DependsOn: []int{2}},
		},
	}

	if err := ValidateDependencies(breakdown); err == nil {
		t.Error("ValidateDependencies should fail for circular dependency")
	}
}
