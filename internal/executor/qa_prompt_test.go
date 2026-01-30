package executor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// projectRoot finds the project root by walking up from the test file to find go.mod.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	dir := filepath.Dir(filename)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

func TestQAE2ETestPromptReferencesOutputDir(t *testing.T) {
	t.Parallel()

	root := projectRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "templates", "prompts", "qa_e2e_test.md"))
	if err != nil {
		t.Fatalf("failed to read qa_e2e_test.md: %v", err)
	}

	text := string(content)

	if !strings.Contains(text, "{{QA_OUTPUT_DIR}}") {
		t.Error("qa_e2e_test.md must reference {{QA_OUTPUT_DIR}} variable")
	}

	// The template must instruct the agent to use QA_OUTPUT_DIR for artifacts
	if !strings.Contains(text, "QA_OUTPUT_DIR") {
		t.Error("qa_e2e_test.md must contain instructions about QA_OUTPUT_DIR")
	}
}

func TestQAE2EFixPromptReferencesOutputDir(t *testing.T) {
	t.Parallel()

	root := projectRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "templates", "prompts", "qa_e2e_fix.md"))
	if err != nil {
		t.Fatalf("failed to read qa_e2e_fix.md: %v", err)
	}

	text := string(content)

	if !strings.Contains(text, "{{QA_OUTPUT_DIR}}") {
		t.Error("qa_e2e_fix.md must reference {{QA_OUTPUT_DIR}} variable")
	}
}

func TestGitignoreContainsQAArtifactPatterns(t *testing.T) {
	t.Parallel()

	root := projectRoot(t)
	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	text := string(content)

	requiredPatterns := []string{
		"QA-REPORT",
		"QA-FINDINGS",
	}

	for _, pattern := range requiredPatterns {
		if !strings.Contains(text, pattern) {
			t.Errorf(".gitignore must contain pattern for %q artifacts", pattern)
		}
	}
}
