// Package detect provides project type and technology detection.
package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ProjectType represents the detected project type.
type ProjectType string

const (
	ProjectTypeGo         ProjectType = "go"
	ProjectTypePython     ProjectType = "python"
	ProjectTypeTypeScript ProjectType = "typescript"
	ProjectTypeJavaScript ProjectType = "javascript"
	ProjectTypeRust       ProjectType = "rust"
	ProjectTypeUnknown    ProjectType = "unknown"
)

// Framework represents a detected framework.
type Framework string

const (
	// Go frameworks
	FrameworkGin   Framework = "gin"
	FrameworkCobra Framework = "cobra"
	FrameworkEcho  Framework = "echo"
	FrameworkFiber Framework = "fiber"

	// JS/TS frameworks
	FrameworkReact   Framework = "react"
	FrameworkNextJS  Framework = "nextjs"
	FrameworkVue     Framework = "vue"
	FrameworkSvelte  Framework = "svelte"
	FrameworkAngular Framework = "angular"
	FrameworkExpress Framework = "express"
	FrameworkNestJS  Framework = "nestjs"

	// Python frameworks
	FrameworkFastAPI Framework = "fastapi"
	FrameworkFlask   Framework = "flask"
	FrameworkDjango  Framework = "django"
)

// BuildTool represents a detected build/package tool.
type BuildTool string

const (
	BuildToolMake   BuildTool = "make"
	BuildToolNPM    BuildTool = "npm"
	BuildToolYarn   BuildTool = "yarn"
	BuildToolPnpm   BuildTool = "pnpm"
	BuildToolBun    BuildTool = "bun"
	BuildToolPoetry BuildTool = "poetry"
	BuildToolPip    BuildTool = "pip"
	BuildToolCargo  BuildTool = "cargo"
)

// Detection contains the results of project detection.
type Detection struct {
	Language    ProjectType `yaml:"language" json:"language"`
	Frameworks  []Framework `yaml:"frameworks,omitempty" json:"frameworks,omitempty"`
	BuildTools  []BuildTool `yaml:"build_tools,omitempty" json:"build_tools,omitempty"`
	HasDocker   bool        `yaml:"has_docker" json:"has_docker"`
	HasCI       bool        `yaml:"has_ci" json:"has_ci"`
	HasTests    bool        `yaml:"has_tests" json:"has_tests"`
	HasFrontend bool        `yaml:"has_frontend" json:"has_frontend"`

	// Inferred commands
	TestCommand  string `yaml:"test_command,omitempty" json:"test_command,omitempty"`
	LintCommand  string `yaml:"lint_command,omitempty" json:"lint_command,omitempty"`
	BuildCommand string `yaml:"build_command,omitempty" json:"build_command,omitempty"`

	// Suggested skills
	SuggestedSkills []string `yaml:"suggested_skills,omitempty" json:"suggested_skills,omitempty"`
}

// Detect analyzes the project at the given path.
func Detect(path string) (*Detection, error) {
	d := &Detection{
		Language: ProjectTypeUnknown,
	}

	// Detect language
	d.Language = detectLanguage(path)

	// Detect frameworks
	d.Frameworks = detectFrameworks(path, d.Language)

	// Detect build tools
	d.BuildTools = detectBuildTools(path)

	// Detect infrastructure
	d.HasDocker = fileExists(filepath.Join(path, "Dockerfile")) ||
		fileExists(filepath.Join(path, "docker-compose.yml")) ||
		fileExists(filepath.Join(path, "docker-compose.yaml"))

	d.HasCI = fileExists(filepath.Join(path, ".github/workflows")) ||
		fileExists(filepath.Join(path, ".gitlab-ci.yml")) ||
		fileExists(filepath.Join(path, ".circleci"))

	// Detect tests
	d.HasTests = detectTests(path, d.Language)

	// Detect frontend
	d.HasFrontend = detectFrontend(path, d.Frameworks)

	// Infer commands
	d.TestCommand = inferTestCommand(d)
	d.LintCommand = inferLintCommand(d)
	d.BuildCommand = inferBuildCommand(d)

	// Suggest skills
	d.SuggestedSkills = suggestSkills(d)

	return d, nil
}

func detectLanguage(path string) ProjectType {
	// Check for Go
	if fileExists(filepath.Join(path, "go.mod")) {
		return ProjectTypeGo
	}

	// Check for Python
	if fileExists(filepath.Join(path, "pyproject.toml")) ||
		fileExists(filepath.Join(path, "setup.py")) ||
		fileExists(filepath.Join(path, "requirements.txt")) {
		return ProjectTypePython
	}

	// Check for TypeScript
	if fileExists(filepath.Join(path, "tsconfig.json")) {
		return ProjectTypeTypeScript
	}

	// Check for JavaScript (package.json but no tsconfig)
	if fileExists(filepath.Join(path, "package.json")) {
		return ProjectTypeJavaScript
	}

	// Check for Rust
	if fileExists(filepath.Join(path, "Cargo.toml")) {
		return ProjectTypeRust
	}

	return ProjectTypeUnknown
}

func detectFrameworks(path string, lang ProjectType) []Framework {
	var frameworks []Framework

	switch lang {
	case ProjectTypeGo:
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

	case ProjectTypeTypeScript, ProjectTypeJavaScript:
		pkg := readPackageJSON(path)
		if pkg != nil {
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
		}

	case ProjectTypePython:
		// Check pyproject.toml or requirements.txt
		if pyprojectContains(path, "fastapi") || requirementsContains(path, "fastapi") {
			frameworks = append(frameworks, FrameworkFastAPI)
		}
		if pyprojectContains(path, "flask") || requirementsContains(path, "flask") {
			frameworks = append(frameworks, FrameworkFlask)
		}
		if pyprojectContains(path, "django") || requirementsContains(path, "django") {
			frameworks = append(frameworks, FrameworkDjango)
		}
	}

	return frameworks
}

func detectBuildTools(path string) []BuildTool {
	var tools []BuildTool

	if fileExists(filepath.Join(path, "Makefile")) {
		tools = append(tools, BuildToolMake)
	}

	if fileExists(filepath.Join(path, "package.json")) {
		// Check for lock files to determine package manager
		if fileExists(filepath.Join(path, "bun.lockb")) || fileExists(filepath.Join(path, "bun.lock")) {
			tools = append(tools, BuildToolBun)
		} else if fileExists(filepath.Join(path, "pnpm-lock.yaml")) {
			tools = append(tools, BuildToolPnpm)
		} else if fileExists(filepath.Join(path, "yarn.lock")) {
			tools = append(tools, BuildToolYarn)
		} else {
			tools = append(tools, BuildToolNPM)
		}
	}

	if fileExists(filepath.Join(path, "poetry.lock")) {
		tools = append(tools, BuildToolPoetry)
	} else if fileExists(filepath.Join(path, "requirements.txt")) {
		tools = append(tools, BuildToolPip)
	}

	if fileExists(filepath.Join(path, "Cargo.toml")) {
		tools = append(tools, BuildToolCargo)
	}

	return tools
}

func detectTests(path string, lang ProjectType) bool {
	switch lang {
	case ProjectTypeGo:
		// Look for *_test.go files
		matches, _ := filepath.Glob(filepath.Join(path, "**/*_test.go"))
		if len(matches) > 0 {
			return true
		}
		// Also check root
		matches, _ = filepath.Glob(filepath.Join(path, "*_test.go"))
		return len(matches) > 0

	case ProjectTypeTypeScript, ProjectTypeJavaScript:
		return fileExists(filepath.Join(path, "jest.config.js")) ||
			fileExists(filepath.Join(path, "jest.config.ts")) ||
			fileExists(filepath.Join(path, "vitest.config.ts")) ||
			fileExists(filepath.Join(path, "playwright.config.ts"))

	case ProjectTypePython:
		return fileExists(filepath.Join(path, "pytest.ini")) ||
			fileExists(filepath.Join(path, "conftest.py")) ||
			dirExists(filepath.Join(path, "tests"))
	}

	return false
}

// detectFrontend checks if the project has frontend components.
// Returns true if any of the following are detected:
// - Frontend frameworks (React, Vue, Svelte, Next.js, Angular)
// - Frontend directories (web/, frontend/, src/components/)
// - Package.json with frontend dependencies
func detectFrontend(path string, frameworks []Framework) bool {
	// Check for frontend frameworks
	frontendFrameworks := map[Framework]bool{
		FrameworkReact:   true,
		FrameworkNextJS:  true,
		FrameworkVue:     true,
		FrameworkSvelte:  true,
		FrameworkAngular: true,
	}
	for _, f := range frameworks {
		if frontendFrameworks[f] {
			return true
		}
	}

	// Check for common frontend directories
	frontendDirs := []string{
		"web",
		"frontend",
		"client",
		"src/components",
		"src/pages",
		"src/views",
		"app",   // Next.js app router
		"pages", // Next.js pages router
		"components",
	}
	for _, dir := range frontendDirs {
		if dirExists(filepath.Join(path, dir)) {
			// Additional validation: check if it looks like a frontend directory
			// (not just any random "app" directory)
			if isFrontendDir(filepath.Join(path, dir)) {
				return true
			}
		}
	}

	return false
}

// isFrontendDir checks if a directory appears to contain frontend code.
func isFrontendDir(dir string) bool {
	// Look for common frontend file patterns
	frontendPatterns := []string{
		"*.tsx",
		"*.jsx",
		"*.vue",
		"*.svelte",
		"*.html",
	}

	for _, pattern := range frontendPatterns {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		if len(matches) > 0 {
			return true
		}
	}

	// Check for index files commonly found in frontend dirs
	indexFiles := []string{
		"index.tsx",
		"index.jsx",
		"index.ts",
		"index.js",
		"App.tsx",
		"App.jsx",
		"App.vue",
		"App.svelte",
	}
	for _, f := range indexFiles {
		if fileExists(filepath.Join(dir, f)) {
			return true
		}
	}

	return false
}

func inferTestCommand(d *Detection) string {
	switch d.Language {
	case ProjectTypeGo:
		return "go test ./..."
	case ProjectTypeTypeScript, ProjectTypeJavaScript:
		for _, tool := range d.BuildTools {
			switch tool {
			case BuildToolBun:
				return "bun test"
			case BuildToolPnpm:
				return "pnpm test"
			case BuildToolYarn:
				return "yarn test"
			default:
				return "npm test"
			}
		}
	case ProjectTypePython:
		return "pytest"
	case ProjectTypeRust:
		return "cargo test"
	}
	return ""
}

func inferLintCommand(d *Detection) string {
	switch d.Language {
	case ProjectTypeGo:
		return "golangci-lint run"
	case ProjectTypeTypeScript, ProjectTypeJavaScript:
		for _, tool := range d.BuildTools {
			switch tool {
			case BuildToolBun:
				return "bun run lint"
			case BuildToolPnpm:
				return "pnpm lint"
			case BuildToolYarn:
				return "yarn lint"
			default:
				return "npm run lint"
			}
		}
	case ProjectTypePython:
		return "ruff check ."
	case ProjectTypeRust:
		return "cargo clippy"
	}
	return ""
}

func inferBuildCommand(d *Detection) string {
	switch d.Language {
	case ProjectTypeGo:
		return "go build ./..."
	case ProjectTypeTypeScript, ProjectTypeJavaScript:
		for _, tool := range d.BuildTools {
			switch tool {
			case BuildToolBun:
				return "bun run build"
			case BuildToolPnpm:
				return "pnpm build"
			case BuildToolYarn:
				return "yarn build"
			default:
				return "npm run build"
			}
		}
	case ProjectTypePython:
		return "" // Python typically doesn't need a build step
	case ProjectTypeRust:
		return "cargo build"
	}
	return ""
}

func suggestSkills(d *Detection) []string {
	var skills []string

	switch d.Language {
	case ProjectTypeGo:
		skills = append(skills, "go-style")
	case ProjectTypePython:
		skills = append(skills, "python-style")
	case ProjectTypeTypeScript:
		skills = append(skills, "typescript-style")
	}

	// Framework-specific skills
	for _, fw := range d.Frameworks {
		switch fw {
		case FrameworkReact:
			skills = append(skills, "react-patterns")
		case FrameworkNextJS:
			skills = append(skills, "nextjs-patterns")
		case FrameworkFastAPI:
			skills = append(skills, "fastapi-patterns")
		}
	}

	// General skills
	if d.HasTests {
		skills = append(skills, "testing-standards")
	}
	if d.HasDocker {
		skills = append(skills, "docker-patterns")
	}

	return skills
}

// Helper functions

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func goModContains(path string, pkg string) bool {
	data, err := os.ReadFile(filepath.Join(path, "go.mod"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), pkg)
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func readPackageJSON(path string) *packageJSON {
	data, err := os.ReadFile(filepath.Join(path, "package.json"))
	if err != nil {
		return nil
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return &pkg
}

func mergeMaps(m1, m2 map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range m1 {
		result[k] = v
	}
	for k, v := range m2 {
		result[k] = v
	}
	return result
}

func pyprojectContains(path string, pkg string) bool {
	data, err := os.ReadFile(filepath.Join(path, "pyproject.toml"))
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), pkg)
}

func requirementsContains(path string, pkg string) bool {
	data, err := os.ReadFile(filepath.Join(path, "requirements.txt"))
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), pkg)
}

// DescribeProject generates a human-readable project description.
func DescribeProject(d *Detection) string {
	if d.Language == ProjectTypeUnknown {
		return "Unknown project type"
	}

	var parts []string
	parts = append(parts, string(d.Language)+" project")

	if len(d.Frameworks) > 0 {
		fws := make([]string, len(d.Frameworks))
		for i, fw := range d.Frameworks {
			fws[i] = string(fw)
		}
		parts = append(parts, "with "+strings.Join(fws, ", "))
	}

	return strings.Join(parts, " ")
}

