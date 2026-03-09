package bench

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureBenchGitignore_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()

	// Write a .gitignore with some existing entries
	existing := "*.log\nvenv/\nbuild/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	ensureBenchGitignore(dir)

	got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)

	// Should still have original entries
	if !containsLine(content, "*.log") {
		t.Error("lost original *.log entry")
	}
	if !containsLine(content, "venv/") {
		t.Error("lost original venv/ entry")
	}

	// Should have added .venv/ and node_modules/
	if !containsLine(content, ".venv/") {
		t.Error("missing .venv/ entry")
	}
	if !containsLine(content, "node_modules/") {
		t.Error("missing node_modules/ entry")
	}
}

func TestEnsureBenchGitignore_CreatesNew(t *testing.T) {
	dir := t.TempDir()

	// No .gitignore exists
	ensureBenchGitignore(dir)

	got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)

	for _, entry := range benchGitignoreEntries {
		if !containsLine(content, entry) {
			t.Errorf("missing entry: %s", entry)
		}
	}
}

func TestEnsureBenchGitignore_Idempotent(t *testing.T) {
	dir := t.TempDir()

	ensureBenchGitignore(dir)
	first, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))

	ensureBenchGitignore(dir)
	second, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))

	if string(first) != string(second) {
		t.Error("ensureBenchGitignore is not idempotent")
	}
}

func TestEnsureBenchGitignore_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()

	// Existing file without trailing newline
	existing := "*.log"
	os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0644)

	ensureBenchGitignore(dir)

	got, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	content := string(got)

	// Should have added a newline before the bench entries
	if !containsLine(content, "*.log") {
		t.Error("lost original *.log entry")
	}
	if !containsLine(content, ".venv/") {
		t.Error("missing .venv/ entry")
	}
}

func TestContainsLine(t *testing.T) {
	content := "*.log\n.venv/\nnode_modules/\n"

	if !containsLine(content, ".venv/") {
		t.Error("should find .venv/")
	}
	if containsLine(content, ".venv") {
		t.Error("should not match without trailing slash")
	}
	if containsLine(content, "missing/") {
		t.Error("should not find missing/")
	}
}
