// Package detect provides project type and technology detection.
// This file contains the multi-language detection system for polyglot projects.
package detect

import (
	"path/filepath"
)

// LanguageInfo contains detection results for a single language at a specific path.
type LanguageInfo struct {
	Language     ProjectType `json:"language"`
	RootPath     string      `json:"root_path"`     // Relative path: "" = project root, "web/" = subdir
	Frameworks   []Framework `json:"frameworks"`    // Detected frameworks for this language
	BuildTool    BuildTool   `json:"build_tool"`    // Package manager / build tool
	TestCommand  string      `json:"test_command"`  // Inferred test command
	LintCommand  string      `json:"lint_command"`  // Inferred lint command
	BuildCommand string      `json:"build_command"` // Inferred build command
}

// MultiDetection contains the results of multi-language project detection.
type MultiDetection struct {
	Languages   []LanguageInfo `json:"languages"`
	HasDocker   bool           `json:"has_docker"`
	HasCI       bool           `json:"has_ci"`
	HasFrontend bool           `json:"has_frontend"`

	// Suggested skills aggregated from all languages
	SuggestedSkills []string `json:"suggested_skills,omitempty"`
}

// DetectMulti analyzes the project at the given path for all languages.
// Unlike Detect(), this returns information for ALL detected languages.
func DetectMulti(path string) (*MultiDetection, error) {
	d := &MultiDetection{}

	// Detect all languages at project root
	rootLangs := detectLanguagesAtPath(path, "")
	d.Languages = append(d.Languages, rootLangs...)

	// Check common subdirectories for additional languages
	subDirs := []string{"web", "frontend", "client", "ui", "app", "server", "backend", "api", "packages"}
	for _, subDir := range subDirs {
		subPath := filepath.Join(path, subDir)
		if dirExists(subPath) {
			subLangs := detectLanguagesAtPath(subPath, subDir)
			// Only add if we found languages AND they're different from root
			for _, lang := range subLangs {
				if !hasLanguageAtPath(d.Languages, lang.Language, lang.RootPath) {
					d.Languages = append(d.Languages, lang)
				}
			}
		}
	}

	// Detect infrastructure (same as before)
	d.HasDocker = fileExists(filepath.Join(path, "Dockerfile")) ||
		fileExists(filepath.Join(path, "docker-compose.yml")) ||
		fileExists(filepath.Join(path, "docker-compose.yaml"))

	d.HasCI = fileExists(filepath.Join(path, ".github/workflows")) ||
		fileExists(filepath.Join(path, ".gitlab-ci.yml")) ||
		fileExists(filepath.Join(path, ".circleci"))

	// Check for frontend from any detected language
	d.HasFrontend = hasFrontendLanguage(d.Languages)

	// Aggregate skills from all languages
	d.SuggestedSkills = aggregateSkills(d)

	return d, nil
}

// detectLanguagesAtPath detects all languages present at a specific path.
func detectLanguagesAtPath(fullPath, relativePath string) []LanguageInfo {
	var langs []LanguageInfo

	// Check for Go
	if fileExists(filepath.Join(fullPath, "go.mod")) {
		lang := LanguageInfo{
			Language:   ProjectTypeGo,
			RootPath:   relativePath,
			Frameworks: detectGoFrameworks(fullPath),
			BuildTool:  detectGoBuildTool(fullPath),
		}
		lang.TestCommand = inferGoTestCommand(fullPath, relativePath)
		lang.LintCommand = inferGoLintCommand(fullPath, relativePath)
		lang.BuildCommand = inferGoBuildCommand(fullPath, relativePath)
		langs = append(langs, lang)
	}

	// Check for TypeScript (before JavaScript, as it's more specific)
	if fileExists(filepath.Join(fullPath, "tsconfig.json")) {
		lang := LanguageInfo{
			Language:   ProjectTypeTypeScript,
			RootPath:   relativePath,
			Frameworks: detectJSFrameworks(fullPath),
			BuildTool:  detectJSBuildTool(fullPath),
		}
		lang.TestCommand = inferJSTestCommand(fullPath, relativePath, lang.BuildTool)
		lang.LintCommand = inferJSLintCommand(fullPath, relativePath, lang.BuildTool)
		lang.BuildCommand = inferJSBuildCommand(fullPath, relativePath, lang.BuildTool)
		langs = append(langs, lang)
	} else if fileExists(filepath.Join(fullPath, "package.json")) {
		// JavaScript (package.json but no tsconfig)
		lang := LanguageInfo{
			Language:   ProjectTypeJavaScript,
			RootPath:   relativePath,
			Frameworks: detectJSFrameworks(fullPath),
			BuildTool:  detectJSBuildTool(fullPath),
		}
		lang.TestCommand = inferJSTestCommand(fullPath, relativePath, lang.BuildTool)
		lang.LintCommand = inferJSLintCommand(fullPath, relativePath, lang.BuildTool)
		lang.BuildCommand = inferJSBuildCommand(fullPath, relativePath, lang.BuildTool)
		langs = append(langs, lang)
	}

	// Check for Python
	if fileExists(filepath.Join(fullPath, "pyproject.toml")) ||
		fileExists(filepath.Join(fullPath, "setup.py")) ||
		fileExists(filepath.Join(fullPath, "requirements.txt")) {
		lang := LanguageInfo{
			Language:   ProjectTypePython,
			RootPath:   relativePath,
			Frameworks: detectPythonFrameworks(fullPath),
			BuildTool:  detectPythonBuildTool(fullPath),
		}
		lang.TestCommand = inferPythonTestCommand(fullPath, relativePath)
		lang.LintCommand = inferPythonLintCommand(fullPath, relativePath)
		lang.BuildCommand = "" // Python typically doesn't need build
		langs = append(langs, lang)
	}

	// Check for Rust
	if fileExists(filepath.Join(fullPath, "Cargo.toml")) {
		lang := LanguageInfo{
			Language:    ProjectTypeRust,
			RootPath:    relativePath,
			Frameworks:  nil, // Rust framework detection not implemented yet
			BuildTool:   BuildToolCargo,
			TestCommand: inferRustTestCommand(fullPath, relativePath),
			LintCommand: inferRustLintCommand(fullPath, relativePath),
		}
		lang.BuildCommand = inferRustBuildCommand(fullPath, relativePath)
		langs = append(langs, lang)
	}

	return langs
}

// Per-language framework detection

func detectGoFrameworks(path string) []Framework {
	var frameworks []Framework
	if goModContains(path, "github.com/gin-gonic/gin") {
		frameworks = append(frameworks, FrameworkGin)
	}
	if goModContains(path, "github.com/spf13/cobra") {
		frameworks = append(frameworks, FrameworkCobra)
	}
	if goModContains(path, "github.com/labstack/echo") {
		frameworks = append(frameworks, FrameworkEcho)
	}
	if goModContains(path, "github.com/gofiber/fiber") {
		frameworks = append(frameworks, FrameworkFiber)
	}
	return frameworks
}

func detectJSFrameworks(path string) []Framework {
	var frameworks []Framework
	pkg := readPackageJSON(path)
	if pkg == nil {
		return frameworks
	}

	deps := mergeMaps(pkg.Dependencies, pkg.DevDependencies)
	if _, ok := deps["react"]; ok {
		frameworks = append(frameworks, FrameworkReact)
	}
	if _, ok := deps["next"]; ok {
		frameworks = append(frameworks, FrameworkNextJS)
	}
	if _, ok := deps["vue"]; ok {
		frameworks = append(frameworks, FrameworkVue)
	}
	if _, ok := deps["svelte"]; ok {
		frameworks = append(frameworks, FrameworkSvelte)
	}
	if _, ok := deps["@angular/core"]; ok {
		frameworks = append(frameworks, FrameworkAngular)
	}
	if _, ok := deps["express"]; ok {
		frameworks = append(frameworks, FrameworkExpress)
	}
	if _, ok := deps["@nestjs/core"]; ok {
		frameworks = append(frameworks, FrameworkNestJS)
	}

	return frameworks
}

func detectPythonFrameworks(path string) []Framework {
	var frameworks []Framework
	if pyprojectContains(path, "fastapi") || requirementsContains(path, "fastapi") {
		frameworks = append(frameworks, FrameworkFastAPI)
	}
	if pyprojectContains(path, "flask") || requirementsContains(path, "flask") {
		frameworks = append(frameworks, FrameworkFlask)
	}
	if pyprojectContains(path, "django") || requirementsContains(path, "django") {
		frameworks = append(frameworks, FrameworkDjango)
	}
	return frameworks
}

// Per-language build tool detection

func detectGoBuildTool(path string) BuildTool {
	if fileExists(filepath.Join(path, "Makefile")) {
		return BuildToolMake
	}
	return "" // Go doesn't really have a build tool beyond `go`
}

func detectJSBuildTool(path string) BuildTool {
	if fileExists(filepath.Join(path, "bun.lockb")) || fileExists(filepath.Join(path, "bun.lock")) {
		return BuildToolBun
	}
	if fileExists(filepath.Join(path, "pnpm-lock.yaml")) {
		return BuildToolPnpm
	}
	if fileExists(filepath.Join(path, "yarn.lock")) {
		return BuildToolYarn
	}
	return BuildToolNPM
}

func detectPythonBuildTool(path string) BuildTool {
	if fileExists(filepath.Join(path, "poetry.lock")) {
		return BuildToolPoetry
	}
	return BuildToolPip
}

// Per-language command inference with relative path support

func inferGoTestCommand(fullPath, relativePath string) string {
	if relativePath != "" {
		return "cd " + relativePath + " && go test ./..."
	}
	return "go test ./..."
}

func inferGoLintCommand(fullPath, relativePath string) string {
	if relativePath != "" {
		return "cd " + relativePath + " && golangci-lint run"
	}
	return "golangci-lint run"
}

func inferGoBuildCommand(fullPath, relativePath string) string {
	if relativePath != "" {
		return "cd " + relativePath + " && go build ./..."
	}
	return "go build ./..."
}

func inferJSTestCommand(fullPath, relativePath string, tool BuildTool) string {
	var cmd string
	switch tool {
	case BuildToolBun:
		cmd = "bun test"
	case BuildToolPnpm:
		cmd = "pnpm test"
	case BuildToolYarn:
		cmd = "yarn test"
	default:
		cmd = "npm test"
	}
	if relativePath != "" {
		return "cd " + relativePath + " && " + cmd
	}
	return cmd
}

func inferJSLintCommand(fullPath, relativePath string, tool BuildTool) string {
	var cmd string
	switch tool {
	case BuildToolBun:
		cmd = "bun run lint"
	case BuildToolPnpm:
		cmd = "pnpm lint"
	case BuildToolYarn:
		cmd = "yarn lint"
	default:
		cmd = "npm run lint"
	}
	if relativePath != "" {
		return "cd " + relativePath + " && " + cmd
	}
	return cmd
}

func inferJSBuildCommand(fullPath, relativePath string, tool BuildTool) string {
	var cmd string
	switch tool {
	case BuildToolBun:
		cmd = "bun run build"
	case BuildToolPnpm:
		cmd = "pnpm build"
	case BuildToolYarn:
		cmd = "yarn build"
	default:
		cmd = "npm run build"
	}
	if relativePath != "" {
		return "cd " + relativePath + " && " + cmd
	}
	return cmd
}

func inferPythonTestCommand(fullPath, relativePath string) string {
	if relativePath != "" {
		return "cd " + relativePath + " && pytest"
	}
	return "pytest"
}

func inferPythonLintCommand(fullPath, relativePath string) string {
	if relativePath != "" {
		return "cd " + relativePath + " && ruff check ."
	}
	return "ruff check ."
}

func inferRustTestCommand(fullPath, relativePath string) string {
	if relativePath != "" {
		return "cd " + relativePath + " && cargo test"
	}
	return "cargo test"
}

func inferRustLintCommand(fullPath, relativePath string) string {
	if relativePath != "" {
		return "cd " + relativePath + " && cargo clippy"
	}
	return "cargo clippy"
}

func inferRustBuildCommand(fullPath, relativePath string) string {
	if relativePath != "" {
		return "cd " + relativePath + " && cargo build"
	}
	return "cargo build"
}

// Helper functions

func hasLanguageAtPath(langs []LanguageInfo, language ProjectType, rootPath string) bool {
	for _, l := range langs {
		if l.Language == language && l.RootPath == rootPath {
			return true
		}
	}
	return false
}

func hasFrontendLanguage(langs []LanguageInfo) bool {
	frontendFrameworks := map[Framework]bool{
		FrameworkReact:   true,
		FrameworkNextJS:  true,
		FrameworkVue:     true,
		FrameworkSvelte:  true,
		FrameworkAngular: true,
	}

	for _, lang := range langs {
		// Check if language itself is frontend-ish
		if lang.Language == ProjectTypeTypeScript || lang.Language == ProjectTypeJavaScript {
			for _, fw := range lang.Frameworks {
				if frontendFrameworks[fw] {
					return true
				}
			}
		}
		// Also check the path
		if lang.RootPath == "web" || lang.RootPath == "frontend" || lang.RootPath == "client" || lang.RootPath == "ui" {
			return true
		}
	}
	return false
}

func aggregateSkills(d *MultiDetection) []string {
	skillSet := make(map[string]bool)

	for _, lang := range d.Languages {
		// Language-specific skills
		switch lang.Language {
		case ProjectTypeGo:
			skillSet["go-style"] = true
		case ProjectTypePython:
			skillSet["python-style"] = true
		case ProjectTypeTypeScript:
			skillSet["typescript-style"] = true
		}

		// Framework-specific skills
		for _, fw := range lang.Frameworks {
			switch fw {
			case FrameworkReact:
				skillSet["react-patterns"] = true
			case FrameworkNextJS:
				skillSet["nextjs-patterns"] = true
			case FrameworkFastAPI:
				skillSet["fastapi-patterns"] = true
			}
		}
	}

	// Infrastructure skills
	if d.HasDocker {
		skillSet["docker-patterns"] = true
	}

	// Convert to slice
	var skills []string
	for skill := range skillSet {
		skills = append(skills, skill)
	}
	return skills
}

// GetScope returns the scope identifier for a language.
// This is used for scoped commands (e.g., "tests:go", "tests:frontend").
func (l *LanguageInfo) GetScope() string {
	// If it's in a known frontend directory, use "frontend" as scope
	if l.RootPath == "web" || l.RootPath == "frontend" || l.RootPath == "client" || l.RootPath == "ui" {
		return "frontend"
	}

	// Otherwise use the language name
	return string(l.Language)
}

// GetPrimaryLanguage returns the first detected language, or nil if none.
func (d *MultiDetection) GetPrimaryLanguage() *LanguageInfo {
	if len(d.Languages) == 0 {
		return nil
	}

	// Prefer root-level languages
	for i := range d.Languages {
		if d.Languages[i].RootPath == "" {
			return &d.Languages[i]
		}
	}

	// Fall back to first detected
	return &d.Languages[0]
}

// ToLegacyDetection converts MultiDetection to the legacy Detection format.
// This maintains backward compatibility with existing code.
func (d *MultiDetection) ToLegacyDetection() *Detection {
	primary := d.GetPrimaryLanguage()
	if primary == nil {
		return &Detection{Language: ProjectTypeUnknown}
	}

	// Collect all frameworks and build tools
	var frameworks []Framework
	var buildTools []BuildTool
	hasTests := false

	for _, lang := range d.Languages {
		frameworks = append(frameworks, lang.Frameworks...)
		if lang.BuildTool != "" {
			buildTools = append(buildTools, lang.BuildTool)
		}
	}

	return &Detection{
		Language:        primary.Language,
		Frameworks:      frameworks,
		BuildTools:      buildTools,
		HasDocker:       d.HasDocker,
		HasCI:           d.HasCI,
		HasTests:        hasTests,
		HasFrontend:     d.HasFrontend,
		TestCommand:     primary.TestCommand,
		LintCommand:     primary.LintCommand,
		BuildCommand:    primary.BuildCommand,
		SuggestedSkills: d.SuggestedSkills,
	}
}
