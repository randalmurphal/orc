package code

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var defaultIncludes = []string{
	"**/*.go", "**/*.py", "**/*.js", "**/*.ts", "**/*.tsx", "**/*.jsx",
}

var defaultExcludes = []string{
	".git", "node_modules", "vendor", "dist", "build", "__pycache__",
}

const maxFileSize = 10 * 1024 * 1024 // 10MB

var extToLang = map[string]string{
	".go":  "go",
	".py":  "python",
	".js":  "javascript",
	".ts":  "typescript",
	".tsx": "typescript",
	".jsx": "javascript",
}

// Walker discovers source files in a project tree.
type Walker struct {
	includes []string
	excludes []string
}

// WalkerOption configures a Walker.
type WalkerOption func(*Walker)

// NewWalker creates a new file walker.
func NewWalker(opts ...WalkerOption) *Walker {
	w := &Walker{
		includes: defaultIncludes,
		excludes: defaultExcludes,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// WithIncludes sets custom include glob patterns.
func WithIncludes(patterns []string) WalkerOption {
	return func(w *Walker) {
		w.includes = patterns
	}
}

// WithExcludes sets custom exclude patterns (directory names).
func WithExcludes(patterns []string) WalkerOption {
	return func(w *Walker) {
		w.excludes = append(w.excludes, patterns...)
	}
}

// Walk discovers source files under root, respecting .gitignore and config.
func (w *Walker) Walk(_ context.Context, root string) ([]FileInfo, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("walk files in %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("walk files in %s: not a directory", root)
	}

	gitignorePatterns := w.loadGitignore(root)

	var files []FileInfo
	err = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}

		relPath, _ := filepath.Rel(root, path)
		if relPath == "." {
			return nil
		}

		// Check symlinks — skip them
		linfo, lerr := os.Lstat(path)
		if lerr == nil && linfo.Mode()&os.ModeSymlink != 0 {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check directory excludes
		if fi.IsDir() {
			baseName := filepath.Base(path)
			for _, exc := range w.excludes {
				if baseName == exc {
					return filepath.SkipDir
				}
			}
			// Check gitignore patterns against directories
			if w.matchesGitignore(relPath+"/", gitignorePatterns) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check gitignore for files
		if w.matchesGitignore(relPath, gitignorePatterns) {
			return nil
		}

		// Check file size
		if fi.Size() > maxFileSize {
			return nil
		}

		// Check include patterns
		if !w.matchesIncludes(relPath) {
			return nil
		}

		// Check binary content
		if isBinary(path) {
			return nil
		}

		lang := detectLanguage(path)
		files = append(files, FileInfo{
			Path:     path,
			Language: lang,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk files in %s: %w", root, err)
	}

	return files, nil
}

func (w *Walker) loadGitignore(root string) []string {
	path := filepath.Join(root, ".gitignore")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns
}

func (w *Walker) matchesGitignore(relPath string, patterns []string) bool {
	for _, pat := range patterns {
		// Directory pattern
		if strings.HasSuffix(pat, "/") {
			dirPat := strings.TrimSuffix(pat, "/")
			parts := strings.Split(relPath, string(filepath.Separator))
			for _, part := range parts {
				if part == dirPat {
					return true
				}
			}
			continue
		}
		// Glob pattern (e.g. *.log)
		matched, _ := doublestar.Match(pat, relPath)
		if matched {
			return true
		}
		// Also match against just the filename
		matched, _ = doublestar.Match(pat, filepath.Base(relPath))
		if matched {
			return true
		}
	}
	return false
}

func (w *Walker) matchesIncludes(relPath string) bool {
	for _, pat := range w.includes {
		matched, _ := doublestar.Match(pat, relPath)
		if matched {
			return true
		}
	}
	return false
}

func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false
	}

	for _, b := range buf[:n] {
		if b == 0 {
			return true
		}
	}
	return false
}

func detectLanguage(path string) string {
	ext := filepath.Ext(path)
	if lang, ok := extToLang[ext]; ok {
		return lang
	}
	return ""
}
