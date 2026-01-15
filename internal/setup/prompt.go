package setup

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/randalmurphal/orc/internal/db"
)

//go:embed builtin/setup.yaml
var builtinPromptTemplate string

// PromptData contains the data used to generate the setup prompt.
type PromptData struct {
	// Project detection info
	Language    string
	Frameworks  []string
	BuildTools  []string
	HasTests    bool
	TestCommand string
	LintCommand string

	// Existing CLAUDE.md content (if any)
	ExistingClaudeMD string

	// Project info
	ProjectName string
	ProjectPath string

	// Project size estimation
	ProjectSize string // "small", "medium", "large", "monorepo"
}

// GeneratePrompt creates the setup prompt from detection results.
func GeneratePrompt(workDir string, detection *db.Detection) (string, error) {
	data := PromptData{
		ProjectName: filepath.Base(workDir),
		ProjectPath: workDir,
	}

	if detection != nil {
		data.Language = detection.Language
		data.Frameworks = detection.Frameworks
		data.BuildTools = detection.BuildTools
		data.HasTests = detection.HasTests
		data.TestCommand = detection.TestCommand
		data.LintCommand = detection.LintCommand
	}

	// Read existing CLAUDE.md if present
	claudeMDPath := filepath.Join(workDir, ".claude", "CLAUDE.md")
	if content, err := os.ReadFile(claudeMDPath); err == nil {
		data.ExistingClaudeMD = string(content)
	}

	// Estimate project size
	data.ProjectSize = estimateProjectSize(workDir)

	// Parse and execute template
	tmpl, err := template.New("setup").Parse(builtinPromptTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// estimateProjectSize estimates the project size based on file count.
func estimateProjectSize(workDir string) string {
	count := 0
	maxCount := 5000 // Stop counting after this

	_ = filepath.WalkDir(workDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if count >= maxCount {
			return filepath.SkipAll
		}

		// Skip common non-source directories
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == "vendor" ||
				name == "__pycache__" || name == ".orc" || name == "dist" ||
				name == "build" || name == "target" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only count source files
		ext := filepath.Ext(path)
		if isSourceFile(ext) {
			count++
		}
		return nil
	})

	switch {
	case count < 50:
		return "small"
	case count < 500:
		return "medium"
	case count < 2000:
		return "large"
	default:
		return "monorepo"
	}
}

// isSourceFile returns true for common source file extensions.
func isSourceFile(ext string) bool {
	sourceExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true,
		".jsx": true, ".rs": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true, ".rb": true, ".php": true, ".swift": true,
		".kt": true, ".scala": true, ".vue": true, ".svelte": true,
	}
	return sourceExts[ext]
}
