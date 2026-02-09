package bench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePRURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		want    PRRef
		wantErr bool
	}{
		{
			name: "bbolt PR",
			url:  "https://github.com/etcd-io/bbolt/pull/501",
			want: PRRef{Owner: "etcd-io", Repo: "bbolt", Number: 501},
		},
		{
			name: "zod PR",
			url:  "https://github.com/colinhacks/zod/pull/5555",
			want: PRRef{Owner: "colinhacks", Repo: "zod", Number: 5555},
		},
		{
			name: "catch2 PR",
			url:  "https://github.com/catchorg/Catch2/pull/2740",
			want: PRRef{Owner: "catchorg", Repo: "Catch2", Number: 2740},
		},
		{
			name: "trailing slash",
			url:  "https://github.com/encode/httpx/pull/3312/",
			want: PRRef{Owner: "encode", Repo: "httpx", Number: 3312},
		},
		{
			name:    "missing number",
			url:     "https://github.com/etcd-io/bbolt/pull/",
			wantErr: true,
		},
		{
			name:    "not a PR URL",
			url:     "https://github.com/etcd-io/bbolt/issues/501",
			wantErr: true,
		},
		{
			name:    "non-github URL",
			url:     "https://gitlab.com/foo/bar/merge_requests/1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParsePRURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParsePRURL(%q) expected error, got %+v", tt.url, ref)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePRURL(%q) unexpected error: %v", tt.url, err)
			}
			if ref.Owner != tt.want.Owner || ref.Repo != tt.want.Repo || ref.Number != tt.want.Number {
				t.Errorf("ParsePRURL(%q) = %+v, want %+v", tt.url, *ref, tt.want)
			}
		})
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path     string
		language string
		want     bool
	}{
		// Go
		{"db_test.go", "go", true},
		{"internal/freelist/freelist_test.go", "go", true},
		{"db.go", "go", false},
		{"internal/freelist/freelist.go", "go", false},
		{"testdata/foo.go", "go", false}, // testdata is not _test.go

		// Python
		{"test_headers.py", "python", true},
		{"tests/test_decoders.py", "python", true},
		{"tests/conftest.py", "python", true},
		{"conftest.py", "python", true},
		{"tests/client/test_headers.py", "python", true},
		{"httpx/_decoders.py", "python", false},
		{"setup.py", "python", false},

		// TypeScript
		{"tuple.test.ts", "typescript", true},
		{"packages/zod/src/v4/classic/tests/tuple.test.ts", "typescript", true},
		{"foo.spec.tsx", "typescript", true},
		{"__tests__/foo.ts", "typescript", true},
		{"src/index.ts", "typescript", false},

		// C++
		{"tests/SelfTest/IntrospectiveTests/ToString.tests.cpp", "cpp", true},
		{"tests/foo.cpp", "cpp", true},
		{"src/catch2/foo_test.cpp", "cpp", true},
		{"src/catch2/internal/catch_stringref.cpp", "cpp", false},

		// Unknown language
		{"foo_test.rs", "rust", false},
	}

	for _, tt := range tests {
		t.Run(tt.path+"/"+tt.language, func(t *testing.T) {
			got := IsTestFile(tt.path, tt.language)
			if got != tt.want {
				t.Errorf("IsTestFile(%q, %q) = %v, want %v", tt.path, tt.language, got, tt.want)
			}
		})
	}
}

func TestParseDiffBlocks(t *testing.T) {
	diff := `diff --git a/db.go b/db.go
index abc1234..def5678 100644
--- a/db.go
+++ b/db.go
@@ -10,6 +10,10 @@ func Open(path string) (*DB, error) {
 	existing code
+	new code
 	more existing
diff --git a/db_test.go b/db_test.go
index 111222..333444 100644
--- a/db_test.go
+++ b/db_test.go
@@ -100,3 +100,8 @@ func TestExisting(t *testing.T) {
 	existing test
+func TestNew(t *testing.T) {
+	new test code
+}
`

	blocks := ParseDiffBlocks(diff)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}

	if blocks[0].FilePath != "db.go" {
		t.Errorf("block 0 path = %q, want %q", blocks[0].FilePath, "db.go")
	}
	if blocks[0].IsNew {
		t.Error("block 0 should not be marked as new")
	}

	if blocks[1].FilePath != "db_test.go" {
		t.Errorf("block 1 path = %q, want %q", blocks[1].FilePath, "db_test.go")
	}
}

func TestParseDiffBlocks_NewFile(t *testing.T) {
	diff := `diff --git a/new_test.go b/new_test.go
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/new_test.go
@@ -0,0 +1,5 @@
+package main
+
+func TestNew(t *testing.T) {
+	// new test
+}
`

	blocks := ParseDiffBlocks(diff)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	if blocks[0].FilePath != "new_test.go" {
		t.Errorf("path = %q, want %q", blocks[0].FilePath, "new_test.go")
	}
	if !blocks[0].IsNew {
		t.Error("block should be marked as new")
	}
}

func TestParseDiffBlocks_Empty(t *testing.T) {
	blocks := ParseDiffBlocks("")
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(blocks))
	}
}

func TestSplitTestPatch(t *testing.T) {
	diff := `diff --git a/db.go b/db.go
index abc..def 100644
--- a/db.go
+++ b/db.go
@@ -1,3 +1,5 @@
 package main
+// source change
 func Open() {}
diff --git a/db_test.go b/db_test.go
index 111..222 100644
--- a/db_test.go
+++ b/db_test.go
@@ -1,3 +1,8 @@
 package main
+func TestOpen(t *testing.T) {
+	// test code
+}
diff --git a/internal/cmd_test.go b/internal/cmd_test.go
new file mode 100644
index 0000000..333444
--- /dev/null
+++ b/internal/cmd_test.go
@@ -0,0 +1,3 @@
+package internal
+func TestCmd(t *testing.T) {}
`

	testPatch, testFiles, sourceFiles := SplitTestPatch(diff, "go")

	if len(testFiles) != 2 {
		t.Fatalf("expected 2 test files, got %d: %v", len(testFiles), testFiles)
	}
	if len(sourceFiles) != 1 {
		t.Fatalf("expected 1 source file, got %d: %v", len(sourceFiles), sourceFiles)
	}
	if sourceFiles[0] != "db.go" {
		t.Errorf("source file = %q, want %q", sourceFiles[0], "db.go")
	}
	if testPatch == "" {
		t.Error("test patch should not be empty")
	}
	if !strings.Contains(testPatch, "db_test.go") {
		t.Error("test patch should contain db_test.go")
	}
	if strings.Contains(testPatch, "db.go\n") {
		t.Error("test patch should not contain source file db.go")
	}
}

func TestSplitTestPatch_NoTestFiles(t *testing.T) {
	diff := `diff --git a/db.go b/db.go
index abc..def 100644
--- a/db.go
+++ b/db.go
@@ -1,3 +1,5 @@
 package main
+// source only
`

	testPatch, testFiles, sourceFiles := SplitTestPatch(diff, "go")
	if testPatch != "" {
		t.Error("expected empty test patch for source-only diff")
	}
	if len(testFiles) != 0 {
		t.Errorf("expected 0 test files, got %d", len(testFiles))
	}
	if len(sourceFiles) != 1 {
		t.Errorf("expected 1 source file, got %d", len(sourceFiles))
	}
}

func TestSplitTestPatch_Python(t *testing.T) {
	diff := `diff --git a/httpx/_decoders.py b/httpx/_decoders.py
index abc..def 100644
--- a/httpx/_decoders.py
+++ b/httpx/_decoders.py
@@ -1,3 +1,5 @@
 class Decoder:
+    pass
diff --git a/tests/test_decoders.py b/tests/test_decoders.py
index 111..222 100644
--- a/tests/test_decoders.py
+++ b/tests/test_decoders.py
@@ -1,3 +1,8 @@
+def test_zstd():
+    pass
`

	testPatch, testFiles, _ := SplitTestPatch(diff, "python")
	if len(testFiles) != 1 {
		t.Fatalf("expected 1 test file, got %d: %v", len(testFiles), testFiles)
	}
	if testFiles[0] != "tests/test_decoders.py" {
		t.Errorf("test file = %q, want %q", testFiles[0], "tests/test_decoders.py")
	}
	if testPatch == "" {
		t.Error("test patch should not be empty")
	}
}

func TestSplitTestPatch_CppTestsDir(t *testing.T) {
	diff := `diff --git a/src/catch2/internal/foo.cpp b/src/catch2/internal/foo.cpp
index abc..def 100644
--- a/src/catch2/internal/foo.cpp
+++ b/src/catch2/internal/foo.cpp
@@ -1,3 +1,5 @@
 // source
+// change
diff --git a/tests/SelfTest/IntrospectiveTests/ToString.tests.cpp b/tests/SelfTest/IntrospectiveTests/ToString.tests.cpp
index 111..222 100644
--- a/tests/SelfTest/IntrospectiveTests/ToString.tests.cpp
+++ b/tests/SelfTest/IntrospectiveTests/ToString.tests.cpp
@@ -1,3 +1,8 @@
+TEST_CASE("nullopt") {
+    CHECK(true);
+}
`

	testPatch, testFiles, sourceFiles := SplitTestPatch(diff, "cpp")
	if len(testFiles) != 1 {
		t.Fatalf("expected 1 test file, got %d: %v", len(testFiles), testFiles)
	}
	if testFiles[0] != "tests/SelfTest/IntrospectiveTests/ToString.tests.cpp" {
		t.Errorf("test file = %q", testFiles[0])
	}
	if len(sourceFiles) != 1 {
		t.Errorf("expected 1 source file, got %d: %v", len(sourceFiles), sourceFiles)
	}
	if testPatch == "" {
		t.Error("test patch should not be empty")
	}
}

func TestValidatePatch(t *testing.T) {
	tests := []struct {
		name    string
		patch   string
		wantErr bool
	}{
		{
			name: "valid patch",
			patch: `diff --git a/db_test.go b/db_test.go
--- a/db_test.go
+++ b/db_test.go
@@ -1,3 +1,5 @@
+func TestNew(t *testing.T) {}
`,
			wantErr: false,
		},
		{
			name:    "empty",
			patch:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			patch:   "   \n\n  ",
			wantErr: true,
		},
		{
			name:    "no diff header",
			patch:   "@@ -1,3 +1,5 @@\n+some code\n",
			wantErr: true,
		},
		{
			name:    "no hunk header",
			patch:   "diff --git a/foo b/foo\n--- a/foo\n+++ b/foo\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePatch(tt.patch)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSavePatch(t *testing.T) {
	dir := t.TempDir()

	path, err := SavePatch(dir, "bbolt-002", "diff --git a/test\n")
	if err != nil {
		t.Fatalf("SavePatch: %v", err)
	}

	expected := filepath.Join(dir, "bbolt-002.patch")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "diff --git a/test\n" {
		t.Errorf("content = %q", string(data))
	}
}

func TestSavePatch_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "patches")

	_, err := SavePatch(dir, "test-001", "content")
	if err != nil {
		t.Fatalf("SavePatch with nested dir: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory should exist: %v", err)
	}
}

func TestUpdateSuiteYAML(t *testing.T) {
	suiteContent := `tasks:
  - id: bbolt-001
    project_id: bbolt
    tier: trivial
    title: "Existing task"
    reference_pr_url: "https://github.com/etcd-io/bbolt/pull/501"
    test_patch_file: "patches/bbolt-001.patch"

  - id: bbolt-002
    project_id: bbolt
    tier: trivial
    title: "Needs patch"
    reference_pr_url: "https://github.com/etcd-io/bbolt/pull/682"

  - id: zod-001
    project_id: zod
    tier: trivial
    title: "Also needs patch"
    reference_pr_url: "https://github.com/colinhacks/zod/pull/5555"
`

	dir := t.TempDir()
	suitePath := filepath.Join(dir, "suite.yaml")
	if err := os.WriteFile(suitePath, []byte(suiteContent), 0644); err != nil {
		t.Fatal(err)
	}

	results := []ExtractionResult{
		{TaskID: "bbolt-001", Status: StatusAlreadyExists},
		{TaskID: "bbolt-002", Status: StatusExtracted},
		{TaskID: "zod-001", Status: StatusExtracted},
	}

	if err := UpdateSuiteYAML(suitePath, results); err != nil {
		t.Fatalf("UpdateSuiteYAML: %v", err)
	}

	data, err := os.ReadFile(suitePath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)

	// bbolt-001 should still have exactly one test_patch_file (existing)
	if count := strings.Count(content, "bbolt-001.patch"); count != 1 {
		t.Errorf("bbolt-001.patch appears %d times, want 1", count)
	}

	// bbolt-002 should now have test_patch_file
	if !strings.Contains(content, `test_patch_file: "patches/bbolt-002.patch"`) {
		t.Error("bbolt-002 should have test_patch_file inserted")
	}

	// zod-001 should now have test_patch_file
	if !strings.Contains(content, `test_patch_file: "patches/zod-001.patch"`) {
		t.Error("zod-001 should have test_patch_file inserted")
	}

	// Original comments and formatting preserved
	if !strings.Contains(content, "Existing task") {
		t.Error("original content should be preserved")
	}
}

