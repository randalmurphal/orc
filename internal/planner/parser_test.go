package planner

import (
	"testing"

	"github.com/randalmurphal/orc/internal/task"
)

func TestParseTaskBreakdown_Valid(t *testing.T) {
	response := `Here is my analysis of the spec.

<task_breakdown>
<task id="1">
<title>Create User model</title>
<description>Define the User model with email, password_hash, created_at fields.</description>
<weight>small</weight>
<depends_on></depends_on>
</task>
<task id="2">
<title>Add password hashing</title>
<description>Implement bcrypt-based password hashing.</description>
<weight>trivial</weight>
<depends_on>1</depends_on>
</task>
<task id="3">
<title>Create registration endpoint</title>
<description>POST /api/auth/register with validation.</description>
<weight>medium</weight>
<depends_on>1,2</depends_on>
</task>
</task_breakdown>`

	breakdown, err := ParseTaskBreakdown(response)
	if err != nil {
		t.Fatalf("ParseTaskBreakdown failed: %v", err)
	}

	if len(breakdown.Tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(breakdown.Tasks))
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

func TestParseTaskBreakdown_NoTasks(t *testing.T) {
	response := `<task_breakdown>
</task_breakdown>`

	_, err := ParseTaskBreakdown(response)
	if err == nil {
		t.Error("Expected error for empty task breakdown")
	}
}

func TestParseTaskBreakdown_NoBreakdown(t *testing.T) {
	response := "Just some text without any task breakdown."

	_, err := ParseTaskBreakdown(response)
	if err == nil {
		t.Error("Expected error when no task breakdown found")
	}
}

func TestParseDependencies(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"", nil},
		{"1", []int{1}},
		{"1,2", []int{1, 2}},
		{"1, 2, 3", []int{1, 2, 3}},
		{"  1  ,  2  ", []int{1, 2}},
		{"invalid", nil},
		{"1,invalid,2", []int{1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDependencies(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("parseDependencies(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseDependencies(%q) = %v, want %v", tt.input, got, tt.want)
					return
				}
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
