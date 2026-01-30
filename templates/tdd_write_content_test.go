package templates

import (
	"strings"
	"testing"
)

// loadTDDWriteTemplate reads the embedded tdd_write.md template content.
func loadTDDWriteTemplate(t *testing.T) string {
	t.Helper()
	content, err := Prompts.ReadFile("prompts/tdd_write.md")
	if err != nil {
		t.Fatalf("read tdd_write.md: %v", err)
	}
	return string(content)
}

// TestTDDWrite_TestClassificationSection verifies SC-1: template contains a test
// classification section defining solitary, sociable, and integration test types
// with clear descriptions of when to use each.
func TestTDDWrite_TestClassificationSection(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	// Each term must appear at least twice: once in definition, once in usage context
	terms := []string{"solitary", "sociable", "integration"}
	for _, term := range terms {
		count := strings.Count(strings.ToLower(content), term)
		if count < 2 {
			t.Errorf("term %q appears %d times, want at least 2 (definition + usage context)", term, count)
		}
	}

	// Total count of all three terms must be at least 6
	total := 0
	for _, term := range terms {
		total += strings.Count(strings.ToLower(content), term)
	}
	if total < 6 {
		t.Errorf("combined count of solitary/sociable/integration = %d, want >= 6", total)
	}
}

// TestTDDWrite_ClassificationUsesXMLTag verifies the classification section
// follows the existing XML tag pattern used by other sections in the template.
func TestTDDWrite_ClassificationUsesXMLTag(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	if !strings.Contains(content, "<test_classification>") {
		t.Error("missing <test_classification> opening tag")
	}
	if !strings.Contains(content, "</test_classification>") {
		t.Error("missing </test_classification> closing tag")
	}
}

// TestTDDWrite_ClassificationBeforeStepTwo verifies SC-2: test classification
// section is placed before Step 2 (Write Tests) so Claude reads it before
// writing any tests.
func TestTDDWrite_ClassificationBeforeStepTwo(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	classificationIdx := strings.Index(content, "<test_classification>")
	stepTwoIdx := strings.Index(content, "## Step 2: Write Tests")

	if classificationIdx == -1 {
		t.Fatal("missing <test_classification> section")
	}
	if stepTwoIdx == -1 {
		t.Fatal("missing '## Step 2: Write Tests' heading")
	}
	if classificationIdx >= stepTwoIdx {
		t.Errorf("<test_classification> at position %d is NOT before Step 2 at position %d",
			classificationIdx, stepTwoIdx)
	}
}

// TestTDDWrite_ClassificationAfterErrorPathTesting verifies the classification
// section is inserted after <error_path_testing> per the spec's technical approach.
func TestTDDWrite_ClassificationAfterErrorPathTesting(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	errorPathEnd := strings.Index(content, "</error_path_testing>")
	classificationStart := strings.Index(content, "<test_classification>")

	if errorPathEnd == -1 {
		t.Fatal("missing </error_path_testing> closing tag")
	}
	if classificationStart == -1 {
		t.Fatal("missing <test_classification> section")
	}
	if classificationStart <= errorPathEnd {
		t.Errorf("<test_classification> at position %d should come after </error_path_testing> at position %d",
			classificationStart, errorPathEnd)
	}
}

// TestTDDWrite_IntegrationTestRequirement verifies SC-3: template contains
// explicit instruction requiring integration tests when the task creates new
// functions that should be called from existing code paths.
func TestTDDWrite_IntegrationTestRequirement(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	// "integration test" must appear at least 3 times
	count := strings.Count(strings.ToLower(content), "integration test")
	if count < 3 {
		t.Errorf("'integration test' appears %d times, want >= 3", count)
	}

	// Must contain mandatory language (MUST, not "should" or "consider")
	lowerContent := strings.ToLower(content)
	hasMandatory := strings.Contains(lowerContent, "must write integration test") ||
		strings.Contains(lowerContent, "must include integration test") ||
		strings.Contains(lowerContent, "must write an integration test")
	if !hasMandatory {
		t.Error("integration test requirement must use mandatory language (MUST), not optional")
	}

	// Must reference creating new functions/interfaces as the trigger condition
	hasNewFunctionTrigger := strings.Contains(lowerContent, "new function") ||
		strings.Contains(lowerContent, "new interface")
	if !hasNewFunctionTrigger {
		t.Error("integration test requirement must reference 'new function' or 'new interface' as trigger condition")
	}
}

// TestTDDWrite_WiringVerificationInstruction verifies SC-4: template instructs
// Claude to verify new interfaces/implementations are registered or wired into
// the system.
func TestTDDWrite_WiringVerificationInstruction(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)
	lowerContent := strings.ToLower(content)

	// Must mention wiring verification
	if !strings.Contains(lowerContent, "wir") {
		t.Error("template must contain wiring-related instruction (wired/wiring)")
	}

	// Must mention interfaces or implementations being registered/wired
	hasInterfaceWiring := strings.Contains(lowerContent, "interface") &&
		(strings.Contains(lowerContent, "registered") || strings.Contains(lowerContent, "wired"))
	if !hasInterfaceWiring {
		t.Error("template must instruct verification that interfaces/implementations are registered or wired")
	}
}

// TestTDDWrite_WiringVerificationPattern verifies SC-5: template includes a
// concrete wiring verification test pattern showing mock-based verification
// that new code is called from expected code paths.
func TestTDDWrite_WiringVerificationPattern(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	// Must contain a Go code example (code block with func Test)
	if !strings.Contains(content, "func Test") {
		t.Error("template must contain a concrete Go test example (func Test...)")
	}

	// The pattern must show: (1) mock setup, (2) running code path, (3) assertion
	// Check for mock/called pattern - the core of wiring verification
	hasCalledFlag := strings.Contains(content, "called") &&
		(strings.Contains(content, "false") || strings.Contains(content, "true"))
	if !hasCalledFlag {
		t.Error("wiring pattern must show a 'called' flag tracking mechanism")
	}

	// Must show assertion that the mock was called
	hasFatalOrError := strings.Contains(content, "t.Fatal") || strings.Contains(content, "t.Error")
	hasNeverCalled := strings.Contains(content, "never called") || strings.Contains(content, "was not called")
	if !hasFatalOrError || !hasNeverCalled {
		t.Error("wiring pattern must assert mock was called with t.Fatal/t.Error and descriptive message")
	}
}

// TestTDDWrite_PreservesExistingSections verifies that existing template sections
// are preserved after the modification (preservation requirements from spec).
func TestTDDWrite_PreservesExistingSections(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	requiredSections := []struct {
		name    string
		marker  string
	}{
		{"critical_mindset opening", "<critical_mindset>"},
		{"critical_mindset closing", "</critical_mindset>"},
		{"test_isolation opening", "<test_isolation>"},
		{"test_isolation closing", "</test_isolation>"},
		{"error_path_testing opening", "<error_path_testing>"},
		{"error_path_testing closing", "</error_path_testing>"},
		{"pre_output_verification opening", "<pre_output_verification>"},
		{"pre_output_verification closing", "</pre_output_verification>"},
		{"output_format opening", "<output_format>"},
		{"output_format closing", "</output_format>"},
		{"Step 1", "## Step 1: Analyze Success Criteria"},
		{"Step 2", "## Step 2: Write Tests"},
		{"Step 3", "## Step 3: Verify Tests Fail"},
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section.marker) {
			t.Errorf("preserved section %q missing: expected %q", section.name, section.marker)
		}
	}
}

// TestTDDWrite_StepOrderPreserved verifies Steps 1, 2, 3 remain in order
// after the template modification.
func TestTDDWrite_StepOrderPreserved(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	step1 := strings.Index(content, "## Step 1: Analyze Success Criteria")
	step2 := strings.Index(content, "## Step 2: Write Tests")
	step3 := strings.Index(content, "## Step 3: Verify Tests Fail")

	if step1 == -1 || step2 == -1 || step3 == -1 {
		t.Fatal("one or more steps not found in template")
	}

	if step1 >= step2 {
		t.Errorf("Step 1 (pos %d) must come before Step 2 (pos %d)", step1, step2)
	}
	if step2 >= step3 {
		t.Errorf("Step 2 (pos %d) must come before Step 3 (pos %d)", step2, step3)
	}
}

// TestTDDWrite_OutputFormatUnchanged verifies the output_format section still
// contains the required tests[] and coverage structure needed by downstream phases.
func TestTDDWrite_OutputFormatUnchanged(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	// Extract output_format section
	start := strings.Index(content, "<output_format>")
	end := strings.Index(content, "</output_format>")
	if start == -1 || end == -1 {
		t.Fatal("output_format section not found")
	}
	outputSection := content[start:end]

	requiredFields := []string{
		`"tests"`,
		`"covers"`,
		`"coverage"`,
		`"covered"`,
		`"manual_verification"`,
	}

	for _, field := range requiredFields {
		if !strings.Contains(outputSection, field) {
			t.Errorf("output_format section missing required field %s", field)
		}
	}
}

// TestTDDWrite_ClassificationDefinesAllThreeTypes verifies each test type
// (solitary, sociable, integration) has a distinct description, not just a mention.
func TestTDDWrite_ClassificationDefinesAllThreeTypes(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)

	// Extract the test_classification section
	start := strings.Index(content, "<test_classification>")
	end := strings.Index(content, "</test_classification>")
	if start == -1 || end == -1 {
		t.Fatal("test_classification section not found")
	}
	classSection := content[start:end]

	types := []string{"solitary", "sociable", "integration"}
	for _, typ := range types {
		if !strings.Contains(strings.ToLower(classSection), typ) {
			t.Errorf("test_classification section must define %q test type", typ)
		}
	}
}

// TestTDDWrite_ConditionalLanguageForGreenfield verifies the template handles
// the edge case where new code has no existing callers (greenfield scenario).
// The requirement should be conditional: "if there are existing code paths".
func TestTDDWrite_ConditionalLanguageForGreenfield(t *testing.T) {
	t.Parallel()
	content := loadTDDWriteTemplate(t)
	lowerContent := strings.ToLower(content)

	// The integration test requirement must be conditional, not absolute
	hasConditional := strings.Contains(lowerContent, "if ") &&
		(strings.Contains(lowerContent, "existing code") ||
			strings.Contains(lowerContent, "existing path") ||
			strings.Contains(lowerContent, "called from"))
	if !hasConditional {
		t.Error("integration test requirement must use conditional language (e.g., 'if your task creates new functions that should be called from existing code')")
	}
}
