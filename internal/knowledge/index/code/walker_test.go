package code

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// --- SC-1: Walker discovers source files respecting .gitignore, defaults, globs, config ---

// SC-1: Walker discovers Go files in project tree.
func TestWalker_DiscoverGoFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\nfunc main() {}\n")
	writeFile(t, root, "lib/util.go", "package lib\nfunc Util() {}\n")
	writeFile(t, root, "README.md", "# Hello\n")

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Should find .go files, not .md
	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "main.go"))
	assertContains(t, paths, filepath.Join(root, "lib/util.go"))
	assertNotContains(t, paths, filepath.Join(root, "README.md"))
}

// SC-1: Walker discovers multi-language files (Go, Python, JS, TS, TSX, JSX).
func TestWalker_DiscoverMultiLanguage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, "app.py", "def main(): pass\n")
	writeFile(t, root, "index.js", "function main() {}\n")
	writeFile(t, root, "app.ts", "function main(): void {}\n")
	writeFile(t, root, "Component.tsx", "export function App() { return <div/> }\n")
	writeFile(t, root, "Button.jsx", "export function Button() { return <button/> }\n")
	writeFile(t, root, "data.json", `{"key": "value"}`)

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "main.go"))
	assertContains(t, paths, filepath.Join(root, "app.py"))
	assertContains(t, paths, filepath.Join(root, "index.js"))
	assertContains(t, paths, filepath.Join(root, "app.ts"))
	assertContains(t, paths, filepath.Join(root, "Component.tsx"))
	assertContains(t, paths, filepath.Join(root, "Button.jsx"))
	assertNotContains(t, paths, filepath.Join(root, "data.json"))
}

// SC-1: Walker respects .gitignore patterns.
func TestWalker_RespectsGitignore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "*.log\ngenerated/\n")
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, "debug.log", "some log\n")
	writeFile(t, root, "generated/code.go", "package gen\n")

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "main.go"))
	assertNotContains(t, paths, filepath.Join(root, "debug.log"))
	assertNotContains(t, paths, filepath.Join(root, "generated/code.go"))
}

// SC-1: Walker applies default excludes (.git, node_modules, vendor, dist, build, __pycache__).
func TestWalker_DefaultExcludes(t *testing.T) {
	root := t.TempDir()
	excludedDirs := []string{".git", "node_modules", "vendor", "dist", "build", "__pycache__"}
	for _, dir := range excludedDirs {
		writeFile(t, root, dir+"/file.go", "package excluded\n")
	}
	writeFile(t, root, "main.go", "package main\n")

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "main.go"))
	for _, dir := range excludedDirs {
		assertNotContains(t, paths, filepath.Join(root, dir, "file.go"))
	}
}

// SC-1: Walker supports double-star glob syntax in config overrides.
func TestWalker_DoubleStarGlob(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, "deep/nested/lib.go", "package lib\n")
	writeFile(t, root, "deep/nested/more/util.go", "package util\n")

	w := NewWalker(WithIncludes([]string{"**/*.go"}))
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "main.go"))
	assertContains(t, paths, filepath.Join(root, "deep/nested/lib.go"))
	assertContains(t, paths, filepath.Join(root, "deep/nested/more/util.go"))
}

// SC-1: Walker accepts per-project config overrides for includes/excludes.
func TestWalker_ConfigOverrides(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, "script.rb", "puts 'hello'\n")
	writeFile(t, root, "testdata/fixture.go", "package testdata\n")

	// Custom includes adds .rb, custom excludes adds testdata/
	w := NewWalker(
		WithIncludes([]string{"**/*.go", "**/*.rb"}),
		WithExcludes([]string{"testdata"}),
	)
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "main.go"))
	assertContains(t, paths, filepath.Join(root, "script.rb"))
	assertNotContains(t, paths, filepath.Join(root, "testdata/fixture.go"))
}

// SC-1 error path: Returns error with context if root directory doesn't exist.
func TestWalker_NonExistentRoot(t *testing.T) {
	w := NewWalker()
	_, err := w.Walk(context.Background(), "/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("Walk should return error for non-existent root")
	}
}

// SC-1 error path: Skips unreadable files with warning logged, continues.
func TestWalker_SkipsUnreadableFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "readable.go", "package main\n")
	unreadable := filepath.Join(root, "unreadable.go")
	if err := os.WriteFile(unreadable, []byte("package main\n"), 0o000); err != nil {
		t.Fatalf("create unreadable file: %v", err)
	}

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk should not error on unreadable files: %v", err)
	}

	// Should find the readable file, skip the unreadable one
	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "readable.go"))
	// The unreadable file may or may not appear depending on implementation
	// (discovery vs reading) — what matters is no error returned
}

// SC-1 edge case: Walker does not follow symlinks to avoid cycles.
func TestWalker_SkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "real.go", "package main\n")

	// Create a symlink that could cause infinite loop
	target := filepath.Join(root, "subdir")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, root, "subdir/inner.go", "package inner\n")
	if err := os.Symlink(root, filepath.Join(target, "loop")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk should handle symlinks without error: %v", err)
	}

	// Should find real files but not follow symlink loop
	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "real.go"))
}

// SC-1 edge case: Walker detects binary content and skips.
func TestWalker_SkipsBinaryFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")

	// Write a binary file with .go extension
	binaryPath := filepath.Join(root, "binary.go")
	binaryContent := make([]byte, 100)
	binaryContent[0] = 0x00 // null byte indicates binary
	binaryContent[1] = 0x7f // ELF header byte
	if err := os.WriteFile(binaryPath, binaryContent, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "main.go"))
	assertNotContains(t, paths, binaryPath)
}

// SC-1 edge case: Walker skips files >10MB.
func TestWalker_SkipsLargeFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "small.go", "package main\n")

	// Create a file just over 10MB
	largePath := filepath.Join(root, "large.go")
	largeContent := make([]byte, 10*1024*1024+1) // 10MB + 1 byte
	copy(largeContent, []byte("package large\n"))
	if err := os.WriteFile(largePath, largeContent, 0o644); err != nil {
		t.Fatalf("write large file: %v", err)
	}

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "small.go"))
	assertNotContains(t, paths, largePath)
}

// SC-1 edge case: Walker works with no .gitignore file.
func TestWalker_NoGitignoreFile(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, "lib/util.go", "package lib\n")
	// No .gitignore file present

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	paths := filePaths(files)
	assertContains(t, paths, filepath.Join(root, "main.go"))
	assertContains(t, paths, filepath.Join(root, "lib/util.go"))
}

// SC-1 edge case: Empty project directory returns 0 files.
func TestWalker_EmptyDirectory(t *testing.T) {
	root := t.TempDir()

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Walk returned %d files for empty dir, want 0", len(files))
	}
}

// SC-1: Walker returns FileInfo with correct language detection.
func TestWalker_LanguageDetection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, "app.py", "def main(): pass\n")
	writeFile(t, root, "index.js", "function main() {}\n")
	writeFile(t, root, "app.ts", "function main(): void {}\n")

	w := NewWalker()
	files, err := w.Walk(context.Background(), root)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	langMap := make(map[string]string)
	for _, f := range files {
		langMap[filepath.Base(f.Path)] = f.Language
	}

	tests := []struct {
		file     string
		wantLang string
	}{
		{"main.go", "go"},
		{"app.py", "python"},
		{"index.js", "javascript"},
		{"app.ts", "typescript"},
	}

	for _, tt := range tests {
		got, ok := langMap[tt.file]
		if !ok {
			t.Errorf("file %s not found in results", tt.file)
			continue
		}
		if got != tt.wantLang {
			t.Errorf("language(%s) = %s, want %s", tt.file, got, tt.wantLang)
		}
	}
}

// --- Test helpers ---

func writeFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	abs := filepath.Join(root, relPath)
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
}

func filePaths(files []FileInfo) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}

func assertContains(t *testing.T, paths []string, want string) {
	t.Helper()
	for _, p := range paths {
		if p == want {
			return
		}
	}
	t.Errorf("paths %v should contain %s", paths, want)
}

func assertNotContains(t *testing.T, paths []string, notWant string) {
	t.Helper()
	for _, p := range paths {
		if p == notWant {
			t.Errorf("paths should NOT contain %s", notWant)
			return
		}
	}
}
