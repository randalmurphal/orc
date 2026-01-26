package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

func TestInjectOrcSection(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("new CLAUDE.md", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "new-project")
		_ = os.MkdirAll(projectDir, 0755)

		err := InjectOrcSection(projectDir)
		if err != nil {
			t.Fatalf("InjectOrcSection() error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(projectDir, "CLAUDE.md"))
		if err != nil {
			t.Fatalf("Failed to read CLAUDE.md: %v", err)
		}

		if !strings.Contains(string(content), "<!-- orc:begin -->") {
			t.Error("Missing orc section start marker")
		}
		if !strings.Contains(string(content), "<!-- orc:end -->") {
			t.Error("Missing orc section end marker")
		}
		if !strings.Contains(string(content), "/orc:init") {
			t.Error("Missing slash command documentation")
		}
	})

	t.Run("existing CLAUDE.md without orc section", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "existing-project")
		_ = os.MkdirAll(projectDir, 0755)

		existingContent := "# My Project\n\nExisting content here.\n"
		_ = os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte(existingContent), 0644)

		err := InjectOrcSection(projectDir)
		if err != nil {
			t.Fatalf("InjectOrcSection() error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(projectDir, "CLAUDE.md"))
		if err != nil {
			t.Fatalf("Failed to read CLAUDE.md: %v", err)
		}

		if !strings.Contains(string(content), "# My Project") {
			t.Error("Existing content was lost")
		}
		if !strings.Contains(string(content), "<!-- orc:begin -->") {
			t.Error("Missing orc section")
		}
	})

	t.Run("existing CLAUDE.md with orc section", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "update-project")
		_ = os.MkdirAll(projectDir, 0755)

		oldContent := `# My Project

<!-- orc:begin -->
Old orc content
<!-- orc:end -->

Other content
`
		_ = os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte(oldContent), 0644)

		err := InjectOrcSection(projectDir)
		if err != nil {
			t.Fatalf("InjectOrcSection() error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(projectDir, "CLAUDE.md"))
		if err != nil {
			t.Fatalf("Failed to read CLAUDE.md: %v", err)
		}

		if strings.Contains(string(content), "Old orc content") {
			t.Error("Old orc content was not replaced")
		}
		if !strings.Contains(string(content), "/orc:init") {
			t.Error("New orc content was not injected")
		}
		if !strings.Contains(string(content), "Other content") {
			t.Error("Content after orc section was lost")
		}
	})
}

func TestRemoveOrcSection(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("remove orc section", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "remove-project")
		_ = os.MkdirAll(projectDir, 0755)

		content := `# My Project

<!-- orc:begin -->
Orc content
<!-- orc:end -->

Other content
`
		_ = os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte(content), 0644)

		err := RemoveOrcSection(projectDir)
		if err != nil {
			t.Fatalf("RemoveOrcSection() error: %v", err)
		}

		result, _ := os.ReadFile(filepath.Join(projectDir, "CLAUDE.md"))
		resultStr := string(result)

		if strings.Contains(resultStr, "orc:begin") {
			t.Error("Orc section was not removed")
		}
		if !strings.Contains(resultStr, "# My Project") {
			t.Error("Header was lost")
		}
		if !strings.Contains(resultStr, "Other content") {
			t.Error("Other content was lost")
		}
	})

	t.Run("no CLAUDE.md exists", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "no-claude-md")
		_ = os.MkdirAll(projectDir, 0755)

		err := RemoveOrcSection(projectDir)
		if err != nil {
			t.Errorf("RemoveOrcSection() should not error when file doesn't exist: %v", err)
		}
	})
}

func TestHasOrcSection(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("has orc section", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "has-orc")
		_ = os.MkdirAll(projectDir, 0755)
		_ = os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte("<!-- orc:begin -->\ncontent\n<!-- orc:end -->"), 0644)

		if !HasOrcSection(projectDir) {
			t.Error("HasOrcSection() returned false for file with orc section")
		}
	})

	t.Run("no orc section", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "no-orc")
		_ = os.MkdirAll(projectDir, 0755)
		_ = os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte("# No orc here"), 0644)

		if HasOrcSection(projectDir) {
			t.Error("HasOrcSection() returned true for file without orc section")
		}
	})

	t.Run("no file", func(t *testing.T) {
		projectDir := filepath.Join(tmpDir, "no-file")
		_ = os.MkdirAll(projectDir, 0755)

		if HasOrcSection(projectDir) {
			t.Error("HasOrcSection() returned true for non-existent file")
		}
	})
}

func TestUpdateTaskContext(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "context-test")
	_ = os.MkdirAll(projectDir, 0755)

	// Create CLAUDE.md with orc section
	_ = InjectOrcSection(projectDir)

	currentPhase := "implement"
	activeTask := &orcv1.Task{
		Id:           "TASK-001",
		Title:        "Test Task",
		CurrentPhase: &currentPhase,
		Status:       orcv1.TaskStatus_TASK_STATUS_RUNNING,
	}

	err := UpdateTaskContext(projectDir, activeTask)
	if err != nil {
		t.Fatalf("UpdateTaskContext() error: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(projectDir, "CLAUDE.md"))
	contentStr := string(content)

	if !strings.Contains(contentStr, "TASK-001") {
		t.Error("Task ID not found in context")
	}
	if !strings.Contains(contentStr, "Test Task") {
		t.Error("Task title not found in context")
	}
	if !strings.Contains(contentStr, "implement") {
		t.Error("Phase not found in context")
	}
}

func TestClearTaskContext(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "clear-context-test")
	_ = os.MkdirAll(projectDir, 0755)

	// Create CLAUDE.md with orc section and context
	_ = InjectOrcSection(projectDir)
	testPhase := "test"
	_ = UpdateTaskContext(projectDir, &orcv1.Task{
		Id:           "TASK-001",
		Title:        "Test",
		CurrentPhase: &testPhase,
		Status:       orcv1.TaskStatus_TASK_STATUS_RUNNING,
	})

	err := ClearTaskContext(projectDir)
	if err != nil {
		t.Fatalf("ClearTaskContext() error: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(projectDir, "CLAUDE.md"))
	contentStr := string(content)

	if strings.Contains(contentStr, "orc:context:begin") {
		t.Error("Context section was not removed")
	}
	// Main orc section should still exist
	if !strings.Contains(contentStr, "orc:begin") {
		t.Error("Main orc section was accidentally removed")
	}
}
