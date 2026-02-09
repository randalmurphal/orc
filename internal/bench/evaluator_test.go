package bench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo creates a git repo in dir with initial commit containing main.go.
// Returns the initial commit hash.
func initGitRepo(t *testing.T, dir string) string {
	t.Helper()
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}

	run("init")
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	run("add", ".")
	run("commit", "-m", "init")
	return run("rev-parse", "HEAD")
}

func TestApplyTestPatch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Apply a patch that adds a new test file
	patch := `diff --git a/main_test.go b/main_test.go
new file mode 100644
--- /dev/null
+++ b/main_test.go
@@ -0,0 +1,7 @@
+package main
+
+import "testing"
+
+func TestHello(t *testing.T) {
+	t.Log("hello")
+}
`

	eval := NewEvaluator()
	if err := eval.applyTestPatch(dir, patch, "HEAD"); err != nil {
		t.Fatalf("applyTestPatch: %v", err)
	}

	// Verify the test file was created
	content, err := os.ReadFile(filepath.Join(dir, "main_test.go"))
	if err != nil {
		t.Fatalf("test file not created: %v", err)
	}
	if !strings.Contains(string(content), "TestHello") {
		t.Error("test file doesn't contain expected function")
	}
}

func TestApplyTestPatchNewFileConflict(t *testing.T) {
	// Model creates a file at the same path as a new file in the test patch.
	// The patch should win — model's version gets removed.
	dir := t.TempDir()
	commitHash := initGitRepo(t, dir)

	// Simulate model creating a test file (like Claude would)
	os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main\n// model's test\n"), 0644)

	// Patch that creates the same file (new file from /dev/null)
	patch := `diff --git a/main_test.go b/main_test.go
new file mode 100644
--- /dev/null
+++ b/main_test.go
@@ -0,0 +1,7 @@
+package main
+
+import "testing"
+
+func TestReference(t *testing.T) {
+	t.Log("reference PR test")
+}
`

	eval := NewEvaluator()
	if err := eval.applyTestPatch(dir, patch, commitHash); err != nil {
		t.Fatalf("applyTestPatch should succeed despite model's conflicting file: %v", err)
	}

	// Reference version should win
	content, err := os.ReadFile(filepath.Join(dir, "main_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "TestReference") {
		t.Error("expected reference test, got model's version")
	}
	if strings.Contains(string(content), "model's test") {
		t.Error("model's file should have been replaced")
	}
}

func TestApplyTestPatchModifiedFile(t *testing.T) {
	// Model modifies an existing test file. Patch should reset it to pre-fix
	// state, then apply the reference PR's changes cleanly.
	dir := t.TempDir()
	initGitRepo(t, dir)

	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}

	// Add a test file and commit — this is the "pre-fix" state
	os.WriteFile(filepath.Join(dir, "util_test.go"), []byte("package main\n// original test\n"), 0644)
	run("add", ".")
	run("commit", "-m", "add test")
	preFixCommit := run("rev-parse", "HEAD")

	// Simulate model modifying the test file (adds its own tests)
	os.WriteFile(filepath.Join(dir, "util_test.go"), []byte("package main\n// model changed this entirely\n"), 0644)

	// Reference PR patch that modifies the original file
	patch := `diff --git a/util_test.go b/util_test.go
--- a/util_test.go
+++ b/util_test.go
@@ -1,2 +1,3 @@
 package main
 // original test
+// reference PR addition
`

	eval := NewEvaluator()
	if err := eval.applyTestPatch(dir, patch, preFixCommit); err != nil {
		t.Fatalf("applyTestPatch should succeed after resetting modified file: %v", err)
	}

	// Verify the file has the reference PR's content, not the model's
	content, err := os.ReadFile(filepath.Join(dir, "util_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "// reference PR addition") {
		t.Error("expected reference PR addition in file")
	}
	if strings.Contains(string(content), "model changed") {
		t.Error("model's modification should have been reset")
	}
}

func TestPatchFileInfo(t *testing.T) {
	patch := `diff --git a/existing.go b/existing.go
--- a/existing.go
+++ b/existing.go
@@ -1,2 +1,3 @@
 package main
+// modified
diff --git a/new_test.go b/new_test.go
new file mode 100644
--- /dev/null
+++ b/new_test.go
@@ -0,0 +1 @@
+package main
diff --git a/another.go b/another.go
--- a/another.go
+++ b/another.go
@@ -1 +1,2 @@
 package main
+// also modified
`

	files := patchFileInfo(patch)

	if len(files.modified) != 2 {
		t.Errorf("expected 2 modified files, got %d: %v", len(files.modified), files.modified)
	}
	if len(files.created) != 1 {
		t.Errorf("expected 1 created file, got %d: %v", len(files.created), files.created)
	}
	if len(files.created) > 0 && files.created[0] != "new_test.go" {
		t.Errorf("expected created file 'new_test.go', got %q", files.created[0])
	}
}

func TestPatchFileInfoDeletedFile(t *testing.T) {
	// Deleted files have +++ /dev/null — should be excluded
	patch := `diff --git a/removed.go b/removed.go
deleted file mode 100644
--- a/removed.go
+++ /dev/null
@@ -1 +0,0 @@
-package main
`
	files := patchFileInfo(patch)
	if len(files.modified) != 0 || len(files.created) != 0 {
		t.Errorf("deleted file should produce no entries, got modified=%v created=%v", files.modified, files.created)
	}
}

func TestParseGoTestCounts(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantTotal int
		wantFail  int
	}{
		{
			name: "individual results",
			output: `=== RUN   TestFoo
--- PASS: TestFoo (0.01s)
=== RUN   TestBar
--- PASS: TestBar (0.02s)
=== RUN   TestBaz
--- FAIL: TestBaz (0.03s)
FAIL
`,
			wantTotal: 3,
			wantFail:  1,
		},
		{
			name: "package level only",
			output: `ok  	go.etcd.io/bbolt	282.758s
ok  	go.etcd.io/bbolt/cmd/bbolt	1.234s
FAIL	go.etcd.io/bbolt/internal	[build failed]
?   	go.etcd.io/bbolt/errors	[no test files]
`,
			wantTotal: 3, // 2 ok + 1 fail (? is skipped)
			wantFail:  1,
		},
		{
			name: "all passing",
			output: `--- PASS: TestOne (0.01s)
--- PASS: TestTwo (0.02s)
ok  	example.com/pkg	0.03s
`,
			wantTotal: 2,
			wantFail:  0,
		},
		{
			name:      "empty output",
			output:    "",
			wantTotal: 0,
			wantFail:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, failures := parseGoTestCounts(tt.output)
			if total != tt.wantTotal {
				t.Errorf("total: got %d, want %d", total, tt.wantTotal)
			}
			if failures != tt.wantFail {
				t.Errorf("failures: got %d, want %d", failures, tt.wantFail)
			}
		})
	}
}

func TestParsePytestCounts(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantTotal int
		wantFail  int
	}{
		{
			name:      "passed and failed",
			output:    "====== 15 passed, 3 failed in 2.53s ======",
			wantTotal: 18,
			wantFail:  3,
		},
		{
			name:      "all passed",
			output:    "====== 42 passed in 1.23s ======",
			wantTotal: 42,
			wantFail:  0,
		},
		{
			name:      "empty",
			output:    "",
			wantTotal: 0,
			wantFail:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, failures := parsePytestCounts(tt.output)
			if total != tt.wantTotal {
				t.Errorf("total: got %d, want %d", total, tt.wantTotal)
			}
			if failures != tt.wantFail {
				t.Errorf("failures: got %d, want %d", failures, tt.wantFail)
			}
		})
	}
}

func TestParseJestCounts(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantTotal int
		wantFail  int
	}{
		{
			name:      "with failures",
			output:    "Tests:  3 failed, 42 passed, 45 total",
			wantTotal: 45,
			wantFail:  3,
		},
		{
			name:      "all passed",
			output:    "Tests:  45 passed, 45 total",
			wantTotal: 45,
			wantFail:  0,
		},
		{
			name:      "empty",
			output:    "",
			wantTotal: 0,
			wantFail:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, failures := parseJestCounts(tt.output)
			if total != tt.wantTotal {
				t.Errorf("total: got %d, want %d", total, tt.wantTotal)
			}
			if failures != tt.wantFail {
				t.Errorf("failures: got %d, want %d", failures, tt.wantFail)
			}
		})
	}
}

func TestParseCTestCounts(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantTotal int
		wantFail  int
	}{
		{
			name:      "all passed",
			output:    "100% tests passed, 0 tests failed out of 45",
			wantTotal: 45,
			wantFail:  0,
		},
		{
			name:      "some failed",
			output:    "93% tests passed, 3 tests failed out of 45",
			wantTotal: 45,
			wantFail:  3,
		},
		{
			name:      "single failure",
			output:    "99% tests passed, 1 test failed out of 100",
			wantTotal: 100,
			wantFail:  1,
		},
		{
			name:      "empty",
			output:    "",
			wantTotal: 0,
			wantFail:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, failures := parseCTestCounts(tt.output)
			if total != tt.wantTotal {
				t.Errorf("total: got %d, want %d", total, tt.wantTotal)
			}
			if failures != tt.wantFail {
				t.Errorf("failures: got %d, want %d", failures, tt.wantFail)
			}
		})
	}
}

func TestParseTestCountsUnknown(t *testing.T) {
	// Unknown language should return (0, 0), never error
	total, failures := parseTestCounts("some random output", "rust")
	if total != 0 || failures != 0 {
		t.Errorf("unknown language should return (0, 0), got (%d, %d)", total, failures)
	}
}

func TestCountOutputLines(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			name:   "empty",
			output: "",
			want:   0,
		},
		{
			name:   "golangci-lint warnings",
			output: "main.go:10:5: unused variable (deadcode)\nmain.go:20:1: missing doc (revive)\n",
			want:   2,
		},
		{
			name:   "with noise lines",
			output: "level=info msg=\"loading config\"\nmain.go:10:5: warning\nRun Time: 1.23s\n",
			want:   1,
		},
		{
			name:   "blank lines ignored",
			output: "\n\nfinding1\n\nfinding2\n\n",
			want:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countOutputLines(tt.output)
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRunCmd(t *testing.T) {
	dir := t.TempDir()
	eval := NewEvaluator()

	// Command that succeeds
	ok, out := eval.runCmd(dir, "echo hello")
	if !ok {
		t.Error("expected 'echo hello' to succeed")
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("expected output to contain 'hello', got %q", out)
	}

	// Command that fails
	ok, out = eval.runCmd(dir, "false")
	if ok {
		t.Error("expected 'false' to fail")
	}
	_ = out
}
