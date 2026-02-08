package bench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyTestPatch(t *testing.T) {
	// Create a temp git repo with a file
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	run("init")
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	run("add", ".")
	run("commit", "-m", "init")

	// Apply a patch that adds a test file
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
	if len(content) == 0 {
		t.Error("test file is empty")
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
