package initiative

import (
	"strings"
	"testing"
)

// ============================================================================
// SC-6: Manifest support for acceptance_criteria
// Adds optional acceptance_criteria list to manifest's create_initiative section.
// Each entry is a description string, auto-generates ID, starts as uncovered.
// ============================================================================

func TestParseManifestBytes_WithAcceptanceCriteria(t *testing.T) {
	t.Parallel()

	yaml := `
version: 1
create_initiative:
  title: "User Authentication"
  vision: "JWT-based auth with refresh tokens"
  acceptance_criteria:
    - "User can log in with JWT"
    - "User can refresh expired token"
    - "Invalid tokens are rejected with 401"
tasks:
  - id: 1
    title: "Login endpoint"
`
	manifest, err := ParseManifestBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifestBytes() error = %v", err)
	}

	if manifest.CreateInitiative == nil {
		t.Fatal("CreateInitiative is nil")
	}

	if len(manifest.CreateInitiative.AcceptanceCriteria) != 3 {
		t.Fatalf("AcceptanceCriteria len = %d, want 3", len(manifest.CreateInitiative.AcceptanceCriteria))
	}

	expected := []string{
		"User can log in with JWT",
		"User can refresh expired token",
		"Invalid tokens are rejected with 401",
	}
	for i, want := range expected {
		got := manifest.CreateInitiative.AcceptanceCriteria[i]
		if got != want {
			t.Errorf("AcceptanceCriteria[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestParseManifestBytes_NoAcceptanceCriteria(t *testing.T) {
	t.Parallel()

	yaml := `
version: 1
create_initiative:
  title: "Simple Initiative"
  vision: "Just do it"
tasks:
  - id: 1
    title: "Only task"
`
	manifest, err := ParseManifestBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifestBytes() error = %v", err)
	}

	if manifest.CreateInitiative == nil {
		t.Fatal("CreateInitiative is nil")
	}

	// Should be nil or empty
	if len(manifest.CreateInitiative.AcceptanceCriteria) != 0 {
		t.Errorf("AcceptanceCriteria len = %d, want 0", len(manifest.CreateInitiative.AcceptanceCriteria))
	}
}

func TestParseManifestBytes_EmptyAcceptanceCriteria(t *testing.T) {
	t.Parallel()

	yaml := `
version: 1
create_initiative:
  title: "Empty Criteria Initiative"
  acceptance_criteria: []
tasks:
  - id: 1
    title: "Only task"
`
	manifest, err := ParseManifestBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifestBytes() error = %v", err)
	}

	if manifest.CreateInitiative == nil {
		t.Fatal("CreateInitiative is nil")
	}

	if len(manifest.CreateInitiative.AcceptanceCriteria) != 0 {
		t.Errorf("AcceptanceCriteria len = %d, want 0", len(manifest.CreateInitiative.AcceptanceCriteria))
	}
}

func TestParseManifestBytes_AcceptanceCriteriaNotOnExistingInitiative(t *testing.T) {
	t.Parallel()

	// When referencing an existing initiative (not create_initiative),
	// acceptance_criteria is on CreateInitiative which is nil - should not parse.
	yaml := `
version: 1
initiative: INIT-001
tasks:
  - id: 1
    title: "Only task"
`
	manifest, err := ParseManifestBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifestBytes() error = %v", err)
	}

	if manifest.CreateInitiative != nil {
		t.Error("CreateInitiative should be nil when using existing initiative")
	}
}

func TestValidateManifest_AcceptanceCriteriaEmptyString(t *testing.T) {
	t.Parallel()

	manifest := &Manifest{
		Version: 1,
		CreateInitiative: &CreateInitiative{
			Title: "Test Initiative",
			AcceptanceCriteria: []string{
				"Valid criterion",
				"",                // Empty string - should be validation error
				"Another valid",
			},
		},
		Tasks: []ManifestTask{
			{ID: 1, Title: "Task 1"},
		},
	}

	errs := ValidateManifest(manifest)

	// Should have at least one error about empty criterion
	hasEmptyErr := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "acceptance_criteria") && strings.Contains(e.Error(), "empty") {
			hasEmptyErr = true
			break
		}
	}
	if !hasEmptyErr {
		t.Error("ValidateManifest should report error for empty acceptance criterion")
	}
}

func TestManifestTaskReferenceCriteria(t *testing.T) {
	t.Parallel()

	// Tasks in manifest can reference criteria by index
	yaml := `
version: 1
create_initiative:
  title: "Auth Feature"
  acceptance_criteria:
    - "User can log in"
    - "User can register"
tasks:
  - id: 1
    title: "Login endpoint"
    criteria: [0]
  - id: 2
    title: "Registration endpoint"
    criteria: [1]
  - id: 3
    title: "Auth middleware"
    criteria: [0, 1]
`
	manifest, err := ParseManifestBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseManifestBytes() error = %v", err)
	}

	// Verify tasks have criteria references
	if len(manifest.Tasks) != 3 {
		t.Fatalf("Tasks len = %d, want 3", len(manifest.Tasks))
	}

	// Task 1 should reference criterion at index 0
	if len(manifest.Tasks[0].Criteria) != 1 || manifest.Tasks[0].Criteria[0] != 0 {
		t.Errorf("Task 1 Criteria = %v, want [0]", manifest.Tasks[0].Criteria)
	}

	// Task 2 should reference criterion at index 1
	if len(manifest.Tasks[1].Criteria) != 1 || manifest.Tasks[1].Criteria[0] != 1 {
		t.Errorf("Task 2 Criteria = %v, want [1]", manifest.Tasks[1].Criteria)
	}

	// Task 3 should reference both
	if len(manifest.Tasks[2].Criteria) != 2 {
		t.Errorf("Task 3 Criteria len = %d, want 2", len(manifest.Tasks[2].Criteria))
	}
}

func TestValidateManifest_CriteriaIndexOutOfBounds(t *testing.T) {
	t.Parallel()

	manifest := &Manifest{
		Version: 1,
		CreateInitiative: &CreateInitiative{
			Title: "Test",
			AcceptanceCriteria: []string{
				"Only one criterion",
			},
		},
		Tasks: []ManifestTask{
			{ID: 1, Title: "Task 1", Criteria: []int{0}},    // Valid
			{ID: 2, Title: "Task 2", Criteria: []int{5}},    // Out of bounds
		},
	}

	errs := ValidateManifest(manifest)

	hasOOBErr := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "criteria") {
			hasOOBErr = true
			break
		}
	}
	if !hasOOBErr {
		t.Error("ValidateManifest should report error for out-of-bounds criteria index")
	}
}

func TestValidateManifest_CriteriaOnExistingInitiative(t *testing.T) {
	t.Parallel()

	// When using existing initiative, task criteria indices can't be validated
	// against acceptance_criteria (they don't exist in manifest).
	// This should either be ignored or error.
	manifest := &Manifest{
		Version:    1,
		Initiative: "INIT-001",
		Tasks: []ManifestTask{
			{ID: 1, Title: "Task 1", Criteria: []int{0}},
		},
	}

	errs := ValidateManifest(manifest)

	// Should have an error - can't reference criteria when using existing initiative
	// (acceptance_criteria is only on create_initiative)
	hasCriteriaErr := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "criteria") {
			hasCriteriaErr = true
			break
		}
	}
	if !hasCriteriaErr {
		t.Error("ValidateManifest should report error when tasks reference criteria without create_initiative")
	}
}
