// Package planner provides spec-to-task planning functionality.
package planner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SpecFile represents a loaded specification file.
type SpecFile struct {
	Path    string // Relative path from spec directory
	Name    string // Filename
	Content string // File contents
	Words   int    // Approximate word count
}

// SpecLoader loads specification files from a directory.
type SpecLoader struct {
	specDir string
	include []string // Glob patterns to include (default: *.md)
}

// NewSpecLoader creates a new spec loader.
func NewSpecLoader(specDir string, include []string) *SpecLoader {
	if len(include) == 0 {
		include = []string{"*.md"}
	}
	return &SpecLoader{
		specDir: specDir,
		include: include,
	}
}

// Load loads all specification files matching the include patterns.
func (l *SpecLoader) Load() ([]*SpecFile, error) {
	// Verify directory exists
	info, err := os.Stat(l.specDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("spec directory does not exist: %s", l.specDir)
		}
		return nil, fmt.Errorf("access spec directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", l.specDir)
	}

	var files []*SpecFile

	// Walk directory and collect matching files
	err = filepath.WalkDir(l.specDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check if file matches any include pattern
		name := d.Name()
		matched := false
		for _, pattern := range l.include {
			if m, _ := filepath.Match(pattern, name); m {
				matched = true
				break
			}
		}
		if !matched {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		// Calculate relative path
		relPath, err := filepath.Rel(l.specDir, path)
		if err != nil {
			relPath = name
		}

		files = append(files, &SpecFile{
			Path:    relPath,
			Name:    name,
			Content: string(content),
			Words:   countWords(string(content)),
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk spec directory: %w", err)
	}

	// Sort files by path for consistent ordering
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files, nil
}

// countWords returns an approximate word count.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// AggregateContent combines all spec files into a single formatted string.
func AggregateContent(files []*SpecFile) string {
	if len(files) == 0 {
		return ""
	}

	var sb strings.Builder

	for i, f := range files {
		if i > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("## File: %s\n\n", f.Path))
		sb.WriteString(f.Content)
	}

	return sb.String()
}

// DescribeFiles returns a formatted list of spec files.
func DescribeFiles(files []*SpecFile) string {
	var sb strings.Builder

	for _, f := range files {
		sb.WriteString(fmt.Sprintf("  - %s (%d words)\n", f.Path, f.Words))
	}

	return sb.String()
}
