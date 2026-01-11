package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLanguage_Go(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.22\n"), 0644)

	lang := detectLanguage(dir)
	if lang != ProjectTypeGo {
		t.Errorf("expected Go, got %s", lang)
	}
}

func TestDetectLanguage_Python(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.poetry]\nname = \"test\"\n"), 0644)

	lang := detectLanguage(dir)
	if lang != ProjectTypePython {
		t.Errorf("expected Python, got %s", lang)
	}
}

func TestDetectLanguage_TypeScript(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0644)

	lang := detectLanguage(dir)
	if lang != ProjectTypeTypeScript {
		t.Errorf("expected TypeScript, got %s", lang)
	}
}

func TestDetectLanguage_JavaScript(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)

	lang := detectLanguage(dir)
	if lang != ProjectTypeJavaScript {
		t.Errorf("expected JavaScript, got %s", lang)
	}
}

func TestDetectLanguage_Rust(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname = \"test\"\n"), 0644)

	lang := detectLanguage(dir)
	if lang != ProjectTypeRust {
		t.Errorf("expected Rust, got %s", lang)
	}
}

func TestDetectGoFramework_Gin(t *testing.T) {
	dir := t.TempDir()
	gomod := `module test

go 1.22

require github.com/gin-gonic/gin v1.9.0
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	frameworks := detectFrameworks(dir, ProjectTypeGo)
	if len(frameworks) != 1 || frameworks[0] != FrameworkGin {
		t.Errorf("expected [gin], got %v", frameworks)
	}
}

func TestDetectGoFramework_Cobra(t *testing.T) {
	dir := t.TempDir()
	gomod := `module test

go 1.22

require github.com/spf13/cobra v1.8.0
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	frameworks := detectFrameworks(dir, ProjectTypeGo)
	if len(frameworks) != 1 || frameworks[0] != FrameworkCobra {
		t.Errorf("expected [cobra], got %v", frameworks)
	}
}

func TestDetectJSFramework_React(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies": {"react": "^18.0.0"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)

	frameworks := detectFrameworks(dir, ProjectTypeJavaScript)
	if len(frameworks) != 1 || frameworks[0] != FrameworkReact {
		t.Errorf("expected [react], got %v", frameworks)
	}
}

func TestDetectJSFramework_NextJS(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies": {"next": "^14.0.0", "react": "^18.0.0"}}`
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0644)

	frameworks := detectFrameworks(dir, ProjectTypeTypeScript)
	if len(frameworks) != 2 {
		t.Errorf("expected [react, nextjs], got %v", frameworks)
	}
}

func TestDetectBuildTools_Bun(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "bun.lockb"), []byte(""), 0644)

	tools := detectBuildTools(dir)
	found := false
	for _, tool := range tools {
		if tool == BuildToolBun {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected bun in %v", tools)
	}
}

func TestDetectBuildTools_NPM(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{}"), 0644)

	tools := detectBuildTools(dir)
	found := false
	for _, tool := range tools {
		if tool == BuildToolNPM {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected npm in %v", tools)
	}
}

func TestInferTestCommand(t *testing.T) {
	tests := []struct {
		name     string
		d        *Detection
		expected string
	}{
		{
			name:     "go project",
			d:        &Detection{Language: ProjectTypeGo},
			expected: "go test ./...",
		},
		{
			name:     "bun project",
			d:        &Detection{Language: ProjectTypeTypeScript, BuildTools: []BuildTool{BuildToolBun}},
			expected: "bun test",
		},
		{
			name:     "npm project",
			d:        &Detection{Language: ProjectTypeJavaScript, BuildTools: []BuildTool{BuildToolNPM}},
			expected: "npm test",
		},
		{
			name:     "python project",
			d:        &Detection{Language: ProjectTypePython},
			expected: "pytest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferTestCommand(tt.d)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestDescribeProject(t *testing.T) {
	tests := []struct {
		name     string
		d        *Detection
		expected string
	}{
		{
			name:     "unknown",
			d:        &Detection{Language: ProjectTypeUnknown},
			expected: "Unknown project type",
		},
		{
			name:     "go basic",
			d:        &Detection{Language: ProjectTypeGo},
			expected: "go project",
		},
		{
			name:     "go with gin",
			d:        &Detection{Language: ProjectTypeGo, Frameworks: []Framework{FrameworkGin}},
			expected: "go project with gin",
		},
		{
			name:     "ts with react and next",
			d:        &Detection{Language: ProjectTypeTypeScript, Frameworks: []Framework{FrameworkReact, FrameworkNextJS}},
			expected: "typescript project with react, nextjs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DescribeProject(tt.d)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestSuggestSkills(t *testing.T) {
	d := &Detection{
		Language:   ProjectTypeGo,
		Frameworks: []Framework{FrameworkGin},
		HasTests:   true,
		HasDocker:  true,
	}

	skills := suggestSkills(d)
	expected := map[string]bool{"go-style": true, "testing-standards": true, "docker-patterns": true}

	for _, skill := range skills {
		if !expected[skill] {
			t.Errorf("unexpected skill: %s", skill)
		}
		delete(expected, skill)
	}

	if len(expected) > 0 {
		t.Errorf("missing skills: %v", expected)
	}
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	gomod := `module test

go 1.22

require (
	github.com/gin-gonic/gin v1.9.0
	github.com/spf13/cobra v1.8.0
)
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)
	os.WriteFile(filepath.Join(dir, "Makefile"), []byte("all:\n\tgo build\n"), 0644)
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM golang:1.22\n"), 0644)

	d, err := Detect(dir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if d.Language != ProjectTypeGo {
		t.Errorf("expected Go, got %s", d.Language)
	}
	if len(d.Frameworks) != 2 {
		t.Errorf("expected 2 frameworks, got %v", d.Frameworks)
	}
	if !d.HasDocker {
		t.Error("expected HasDocker=true")
	}
}
