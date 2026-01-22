package bootstrap

import (
	"path/filepath"
	"regexp"
)

// IsTestFile returns true if the given file path is a test file or test infrastructure.
// This function mirrors the logic in tdd-discipline.sh for testing and reuse.
//
// Test file patterns supported:
//   - Go: *_test.go
//   - Python: test_*.py, *_test.py
//   - JavaScript/TypeScript: *.test.ts, *.spec.ts, *.test.js, *.spec.js
//   - Ruby: *_spec.rb
//   - Rust: *_test.rs
//
// Test directories:
//   - /tests/, /test/, /__tests__/, /spec/, /e2e/, /integration/
//
// Test infrastructure:
//   - conftest.py, pytest.ini
//   - jest.config.*, vitest.config.*, playwright.config.*, cypress.config.*
//   - setupTests.ts, setup.ts
//
// Test data and fixtures:
//   - /fixtures/, /testdata/, /mocks/, /stubs/, /fakes/
//   - *.mock.ts, *.stub.go, *.fake.py
func IsTestFile(filePath string) bool {
	// Normalize path separators
	filePath = filepath.ToSlash(filePath)
	baseName := filepath.Base(filePath)

	// Test files by naming convention
	testFilePatterns := []*regexp.Regexp{
		// Go, Python, TS, JS, Rust, Ruby: *_test.*
		regexp.MustCompile(`_test\.(go|py|ts|js|tsx|jsx|rs|rb)$`),
		// JavaScript/TypeScript: *.test.*
		regexp.MustCompile(`\.test\.(ts|js|tsx|jsx|mjs|cjs)$`),
		// JavaScript/TypeScript/Ruby: *.spec.*
		regexp.MustCompile(`\.spec\.(ts|js|tsx|jsx|mjs|cjs|rb)$`),
		// Ruby: *_spec.rb
		regexp.MustCompile(`_spec\.rb$`),
		// Python: test_*.py
		regexp.MustCompile(`^test_.*\.py$`),
	}

	for _, pattern := range testFilePatterns {
		if pattern.MatchString(baseName) {
			return true
		}
	}

	// Test directories (match both /tests/ in path and tests/ at start)
	testDirPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(^|/)tests?/`),
		regexp.MustCompile(`(^|/)__tests__/`),
		regexp.MustCompile(`(^|/)spec/`),
		regexp.MustCompile(`(^|/)e2e/`),
		regexp.MustCompile(`(^|/)integration/`),
	}

	for _, pattern := range testDirPatterns {
		if pattern.MatchString(filePath) {
			return true
		}
	}

	// Test infrastructure and configuration
	infraPatterns := []*regexp.Regexp{
		// Python test config
		regexp.MustCompile(`^conftest\.py$`),
		regexp.MustCompile(`^pytest\.ini$`),
		// JavaScript test configs
		regexp.MustCompile(`^(jest|vitest|playwright|cypress)\.config\.`),
		// Setup files
		regexp.MustCompile(`^setupTests?\.(ts|js|tsx|jsx)$`),
	}

	for _, pattern := range infraPatterns {
		if pattern.MatchString(baseName) {
			return true
		}
	}

	// Test data and fixtures directories (match both /fixtures/ in path and fixtures/ at start)
	fixturePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(^|/)fixtures?/`),
		regexp.MustCompile(`(^|/)testdata/`),
		regexp.MustCompile(`(^|/)mocks?/`),
		regexp.MustCompile(`(^|/)stubs?/`),
		regexp.MustCompile(`(^|/)fakes?/`),
	}

	for _, pattern := range fixturePatterns {
		if pattern.MatchString(filePath) {
			return true
		}
	}

	// Mock/stub/fake files by convention
	mockPattern := regexp.MustCompile(`\.(mock|stub|fake)\.(ts|js|go|py|tsx|jsx)$`)
	if mockPattern.MatchString(baseName) {
		return true
	}

	return false
}
