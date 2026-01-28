package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectMulti_GoOnlyProject(t *testing.T) {
	// Create a temp directory with only Go
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.21"), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)

	d, err := DetectMulti(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Languages) != 1 {
		t.Errorf("expected 1 language, got %d", len(d.Languages))
	}

	if d.Languages[0].Language != ProjectTypeGo {
		t.Errorf("expected Go, got %s", d.Languages[0].Language)
	}

	if d.Languages[0].RootPath != "" {
		t.Errorf("expected empty root path, got %s", d.Languages[0].RootPath)
	}

	if d.Languages[0].TestCommand != "go test ./..." {
		t.Errorf("expected 'go test ./...', got %s", d.Languages[0].TestCommand)
	}
}

func TestDetectMulti_TypeScriptOnlyProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"react":"^18"}}`), 0644)
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0644)

	d, err := DetectMulti(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Languages) != 1 {
		t.Errorf("expected 1 language, got %d", len(d.Languages))
	}

	if d.Languages[0].Language != ProjectTypeTypeScript {
		t.Errorf("expected TypeScript, got %s", d.Languages[0].Language)
	}

	if !containsFramework(d.Languages[0].Frameworks, FrameworkReact) {
		t.Error("expected React framework to be detected")
	}

	if !d.HasFrontend {
		t.Error("expected HasFrontend to be true")
	}
}

func TestDetectMulti_GoWithTypeScriptFrontend(t *testing.T) {
	// This is similar to the orc project structure
	dir := t.TempDir()

	// Go at root
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.21\nrequire github.com/spf13/cobra v1.8.0"), 0644)

	// TypeScript in web/
	webDir := filepath.Join(dir, "web")
	os.MkdirAll(webDir, 0755)
	os.WriteFile(filepath.Join(webDir, "package.json"), []byte(`{"dependencies":{"react":"^18"}}`), 0644)
	os.WriteFile(filepath.Join(webDir, "tsconfig.json"), []byte(`{}`), 0644)

	d, err := DetectMulti(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Languages) != 2 {
		t.Errorf("expected 2 languages, got %d: %+v", len(d.Languages), d.Languages)
	}

	// Check Go
	goLang := findLanguage(d.Languages, ProjectTypeGo)
	if goLang == nil {
		t.Fatal("expected Go to be detected")
	}
	if goLang.RootPath != "" {
		t.Errorf("expected Go at root, got path %s", goLang.RootPath)
	}
	if !containsFramework(goLang.Frameworks, FrameworkCobra) {
		t.Error("expected Cobra framework to be detected")
	}

	// Check TypeScript
	tsLang := findLanguage(d.Languages, ProjectTypeTypeScript)
	if tsLang == nil {
		t.Fatal("expected TypeScript to be detected")
	}
	if tsLang.RootPath != "web" {
		t.Errorf("expected TypeScript at web/, got path %s", tsLang.RootPath)
	}
	if !containsFramework(tsLang.Frameworks, FrameworkReact) {
		t.Error("expected React framework to be detected")
	}

	// Check frontend detection
	if !d.HasFrontend {
		t.Error("expected HasFrontend to be true")
	}

	// Check scopes
	if goLang.GetScope() != "go" {
		t.Errorf("expected Go scope to be 'go', got %s", goLang.GetScope())
	}
	if tsLang.GetScope() != "frontend" {
		t.Errorf("expected TypeScript scope to be 'frontend', got %s", tsLang.GetScope())
	}
}

func TestDetectMulti_CommandsWithRelativePaths(t *testing.T) {
	dir := t.TempDir()

	// TypeScript in web/
	webDir := filepath.Join(dir, "web")
	os.MkdirAll(webDir, 0755)
	os.WriteFile(filepath.Join(webDir, "package.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(webDir, "tsconfig.json"), []byte(`{}`), 0644)

	d, err := DetectMulti(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Languages) != 1 {
		t.Errorf("expected 1 language, got %d", len(d.Languages))
	}

	lang := d.Languages[0]
	if lang.TestCommand != "cd web && npm test" {
		t.Errorf("expected 'cd web && npm test', got %s", lang.TestCommand)
	}
	if lang.LintCommand != "cd web && npm run lint" {
		t.Errorf("expected 'cd web && npm run lint', got %s", lang.LintCommand)
	}
}

func TestDetectMulti_ToLegacyDetection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.21"), 0644)

	d, err := DetectMulti(dir)
	if err != nil {
		t.Fatal(err)
	}

	legacy := d.ToLegacyDetection()
	if legacy.Language != ProjectTypeGo {
		t.Errorf("expected Go, got %s", legacy.Language)
	}
}

func TestDetectMulti_BunPackageManager(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte{}, 0644)

	d, err := DetectMulti(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Languages) != 1 {
		t.Errorf("expected 1 language, got %d", len(d.Languages))
	}

	lang := d.Languages[0]
	if lang.BuildTool != BuildToolBun {
		t.Errorf("expected Bun, got %s", lang.BuildTool)
	}
	if lang.TestCommand != "bun test" {
		t.Errorf("expected 'bun test', got %s", lang.TestCommand)
	}
}

func TestDetectMulti_PythonProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(`[project]\nname = "test"\n[tool.poetry.dependencies]\nfastapi = "^0.100"`), 0644)

	d, err := DetectMulti(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(d.Languages) != 1 {
		t.Errorf("expected 1 language, got %d", len(d.Languages))
	}

	lang := d.Languages[0]
	if lang.Language != ProjectTypePython {
		t.Errorf("expected Python, got %s", lang.Language)
	}
	if lang.TestCommand != "pytest" {
		t.Errorf("expected 'pytest', got %s", lang.TestCommand)
	}
}

// Helper functions

func findLanguage(langs []LanguageInfo, lang ProjectType) *LanguageInfo {
	for i := range langs {
		if langs[i].Language == lang {
			return &langs[i]
		}
	}
	return nil
}

func containsFramework(frameworks []Framework, fw Framework) bool {
	for _, f := range frameworks {
		if f == fw {
			return true
		}
	}
	return false
}
