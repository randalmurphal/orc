package bootstrap

import (
	"testing"
)

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		// Go test files
		{"go test file", "foo_test.go", true},
		{"go test file with path", "internal/db/task_test.go", true},
		{"go source file", "foo.go", false},
		{"go source file with path", "internal/db/task.go", false},

		// Python test files
		{"python test_prefix", "test_foo.py", true},
		{"python test_prefix with path", "tests/test_foo.py", true},
		{"python _test suffix", "foo_test.py", true},
		{"python conftest", "conftest.py", true},
		{"python conftest in path", "tests/conftest.py", true},
		{"python pytest.ini", "pytest.ini", true},
		{"python source file", "foo.py", false},
		{"python file starting with test but not test_", "testing.py", false},

		// TypeScript/JavaScript test files
		{"ts test file", "foo.test.ts", true},
		{"ts spec file", "foo.spec.ts", true},
		{"tsx test file", "Component.test.tsx", true},
		{"tsx spec file", "Component.spec.tsx", true},
		{"js test file", "foo.test.js", true},
		{"js spec file", "foo.spec.js", true},
		{"jsx test file", "Component.test.jsx", true},
		{"mjs test file", "foo.test.mjs", true},
		{"cjs test file", "foo.test.cjs", true},
		{"ts source file", "foo.ts", false},
		{"tsx source file", "Component.tsx", false},

		// JavaScript test configs
		{"jest config ts", "jest.config.ts", true},
		{"jest config js", "jest.config.js", true},
		{"vitest config ts", "vitest.config.ts", true},
		{"vitest config mts", "vitest.config.mts", true},
		{"playwright config ts", "playwright.config.ts", true},
		{"cypress config js", "cypress.config.js", true},
		{"setup tests ts", "setupTests.ts", true},
		{"setup tests tsx", "setupTests.tsx", true},
		{"setup test js", "setupTest.js", true},

		// Ruby test files
		{"ruby spec file", "foo_spec.rb", true},
		{"ruby rspec", "user_spec.rb", true},
		{"ruby source file", "foo.rb", false},

		// Rust test files
		{"rust test file", "foo_test.rs", true},
		{"rust source file", "foo.rs", false},

		// Test directories
		{"file in tests dir", "tests/helper.go", true},
		{"file in test dir", "test/helper.py", true},
		{"file in __tests__ dir", "__tests__/helper.ts", true},
		{"file in nested __tests__", "src/__tests__/Component.tsx", true},
		{"file in spec dir", "spec/helper.rb", true},
		{"file in e2e dir", "e2e/login.spec.ts", true},
		{"file in integration dir", "integration/api.test.ts", true},

		// Fixtures and test data
		{"file in fixtures dir", "fixtures/data.json", true},
		{"file in fixture dir", "fixture/mock.json", true},
		{"file in testdata dir", "testdata/input.txt", true},
		{"file in mocks dir", "mocks/api.ts", true},
		{"file in mock dir", "mock/service.go", true},
		{"file in stubs dir", "stubs/database.py", true},
		{"file in fakes dir", "fakes/repository.ts", true},

		// Mock/stub/fake files by naming convention
		{"ts mock file", "api.mock.ts", true},
		{"js mock file", "service.mock.js", true},
		{"go stub file", "database.stub.go", true},
		{"py fake file", "repository.fake.py", true},
		{"tsx mock file", "Component.mock.tsx", true},

		// Non-test files that might look like tests
		{"file named test but not in pattern", "test.go", false},
		{"file with test in middle", "mytest.go", false},
		{"spec in filename not suffix", "specification.ts", false},
		{"mock in filename not suffix", "mocking.go", false},

		// Edge cases
		{"empty path", "", false},
		{"just extension", ".go", false},
		{"deeply nested test", "a/b/c/d/tests/e/f/helper.go", true},
		{"windows-style path", "internal\\db\\task_test.go", true}, // filepath.ToSlash handles this
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTestFile(tt.filePath)
			if got != tt.want {
				t.Errorf("IsTestFile(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestIsTestFile_AllLanguages(t *testing.T) {
	// Comprehensive test for all supported test file patterns per language
	testCases := map[string][]string{
		"Go": {
			"foo_test.go",
			"internal/db/task_test.go",
			"cmd/cli/main_test.go",
		},
		"Python": {
			"test_foo.py",
			"foo_test.py",
			"conftest.py",
			"pytest.ini",
			"tests/test_api.py",
			"tests/conftest.py",
		},
		"TypeScript": {
			"foo.test.ts",
			"foo.spec.ts",
			"Component.test.tsx",
			"Component.spec.tsx",
			"jest.config.ts",
			"vitest.config.ts",
			"playwright.config.ts",
			"setupTests.ts",
			"__tests__/Component.tsx",
		},
		"JavaScript": {
			"foo.test.js",
			"foo.spec.js",
			"Component.test.jsx",
			"foo.test.mjs",
			"foo.test.cjs",
			"jest.config.js",
			"cypress.config.js",
			"setupTests.js",
		},
		"Ruby": {
			"foo_spec.rb",
			"user_spec.rb",
			"spec/models/user_spec.rb",
		},
		"Rust": {
			"foo_test.rs",
			"tests/integration_test.rs",
		},
	}

	for lang, files := range testCases {
		for _, file := range files {
			t.Run(lang+"/"+file, func(t *testing.T) {
				if !IsTestFile(file) {
					t.Errorf("IsTestFile(%q) should return true for %s test file", file, lang)
				}
			})
		}
	}
}

func TestIsTestFile_NonTestFiles(t *testing.T) {
	// Verify these common source files are NOT identified as test files
	sourceFiles := []string{
		// Go
		"main.go",
		"server.go",
		"handler.go",
		"internal/api/server.go",

		// Python
		"main.py",
		"app.py",
		"models.py",
		"api/views.py",

		// TypeScript/JavaScript
		"index.ts",
		"App.tsx",
		"server.js",
		"utils.mjs",
		"config.ts",
		"Component.tsx",

		// Ruby
		"app.rb",
		"server.rb",

		// Rust
		"main.rs",
		"lib.rs",

		// Config files that aren't test-related
		"package.json",
		"tsconfig.json",
		".eslintrc.js",
		"vite.config.ts",
		"next.config.js",

		// Documentation
		"README.md",
		"CLAUDE.md",
		"CHANGELOG.md",
	}

	for _, file := range sourceFiles {
		t.Run(file, func(t *testing.T) {
			if IsTestFile(file) {
				t.Errorf("IsTestFile(%q) should return false for source file", file)
			}
		})
	}
}
