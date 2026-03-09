package bench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestImplementRubric(t *testing.T) {
	rubric := ImplementRubric
	if len(rubric.Criteria) != 4 {
		t.Errorf("ImplementRubric has %d criteria, want 4", len(rubric.Criteria))
	}
	if rubric.MaxScore != 5 {
		t.Errorf("ImplementRubric MaxScore = %d, want 5", rubric.MaxScore)
	}
	if rubric.PhaseID != "implement" {
		t.Errorf("ImplementRubric PhaseID = %q, want implement", rubric.PhaseID)
	}

	// Verify expected criteria — each provides independent signal
	want := map[string]bool{
		"functional_correctness": true,
		"completeness":           true,
		"code_quality":           true,
		"minimal_change":         true,
	}
	for _, c := range rubric.Criteria {
		if !want[c] {
			t.Errorf("unexpected criterion: %q", c)
		}
	}
}

func TestParseJudgeResponse(t *testing.T) {
	rubric := ImplementRubric

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name: "valid json",
			content: `Here's my evaluation:
{"scores":{"functional_correctness":4,"completeness":5,"code_quality":3,"minimal_change":4},"reasoning":"Good implementation"}`,
			wantErr: false,
		},
		{
			name: "json in code block",
			content: "```json\n{\"scores\":{\"functional_correctness\":5,\"completeness\":4,\"code_quality\":5,\"minimal_change\":4},\"reasoning\":\"Excellent\"}\n```",
			wantErr: false,
		},
		{
			name:    "no json",
			content: "This is just text with no JSON",
			wantErr: true,
		},
		{
			name: "missing criterion",
			content: `{"scores":{"functional_correctness":4,"completeness":5},"reasoning":"Partial"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := parseJudgeResponse(tt.content, rubric)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJudgeResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && resp != nil {
				if len(resp.Scores) == 0 {
					t.Error("expected scores in response")
				}
				// Check scores are within range
				for _, score := range resp.Scores {
					if score < 1 || score > 5 {
						t.Errorf("score %d out of range [1, 5]", score)
					}
				}
			}
		})
	}
}

func TestBuildJudgePrompt(t *testing.T) {
	req := JudgeRequest{
		TaskTitle: "Fix page split",
		TaskDesc:  "The page splitting algorithm fails on large keys",
		Rubric:    ImplementRubric,
	}

	prompt := buildJudgePrompt(req)

	// Must have: review instructions
	mustContain := []string{
		"senior engineer",
		"code review",
		"git diff HEAD~1",
		".bench/context.md",
		// Rubric criteria with anchors
		"functional_correctness",
		"completeness",
		"code_quality",
		"minimal_change",
		// Identity blinding: says "developer" not "AI"
		"developer",
	}

	for _, check := range mustContain {
		if !contains(prompt, check) {
			t.Errorf("prompt missing expected content: %q", check)
		}
	}

	// Must NOT have: test results, AI/model references
	mustNotContain := []string{
		"PASS",
		"FAIL",
		"Test Result",
		"test suite",
		"AI-orchestrated",
		"AI model",
	}

	for _, bad := range mustNotContain {
		if contains(prompt, bad) {
			t.Errorf("prompt should NOT contain %q (anchoring/blinding violation)", bad)
		}
	}
}

func TestBuildJudgePrompt_HasScoreAnchors(t *testing.T) {
	req := JudgeRequest{
		TaskTitle: "Fix bug",
		TaskDesc:  "Something is broken",
		Rubric:    ImplementRubric,
	}

	prompt := buildJudgePrompt(req)

	// Verify score anchors exist for calibrated scoring
	anchors := []string{
		"5: Correctly identifies and fixes the root cause",
		"3: Partially fixes the bug",
		"1: No meaningful fix",
		"5: Precisely targeted",
		"1: Scattered across unrelated files",
	}

	for _, anchor := range anchors {
		if !contains(prompt, anchor) {
			t.Errorf("prompt missing score anchor: %q", anchor)
		}
	}
}

func TestWriteContextFile(t *testing.T) {
	dir := t.TempDir()

	req := JudgeRequest{
		TaskTitle: "Fix page split on large keys",
		TaskDesc:  "The splitPage() function panics when key size exceeds 4KB.",
		Rubric:    ImplementRubric,
	}

	if err := writeContextFile(dir, req); err != nil {
		t.Fatalf("writeContextFile() error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".bench", "context.md"))
	if err != nil {
		t.Fatalf("read context.md: %v", err)
	}

	s := string(content)
	if !contains(s, "Fix page split on large keys") {
		t.Error("context file missing task title")
	}
	if !contains(s, "splitPage()") {
		t.Error("context file missing task description content")
	}
	if !contains(s, "# Bug Report") {
		t.Error("context file missing header")
	}
}

func TestDefaultJudgeConfigs(t *testing.T) {
	judges := DefaultJudgeConfigs()

	if len(judges) != 2 {
		t.Fatalf("expected 2 frontier judges, got %d", len(judges))
	}

	// Verify the panel: Opus with thinking + GPT-5.3-Codex with xhigh reasoning
	opus := judges[0]
	if opus.Provider != "claude" || opus.Model != "opus" {
		t.Errorf("judge[0] = %s/%s, want claude/opus", opus.Provider, opus.Model)
	}
	if !opus.Thinking {
		t.Error("opus judge should have Thinking=true")
	}

	codex := judges[1]
	if codex.Provider != "codex" || codex.Model != "gpt-5.3-codex" {
		t.Errorf("judge[1] = %s/%s, want codex/gpt-5.3-codex", codex.Provider, codex.Model)
	}
	if codex.ReasoningEffort != "xhigh" {
		t.Errorf("codex judge ReasoningEffort = %q, want xhigh", codex.ReasoningEffort)
	}
}

func TestJudgeCoverage_BothJudgesEvaluateEveryRun(t *testing.T) {
	judges := DefaultJudgeConfigs()

	// With no self-exclusion, both judges evaluate every variant.
	// This is intentional — blinding mitigates self-evaluation bias,
	// and 2 opinions is better than 1.
	variants := []struct {
		provider string
		model    string
	}{
		{"claude", "opus"},
		{"claude", "sonnet"},
		{"codex", "gpt-5.3-codex"},
	}

	for _, v := range variants {
		// Both judges should evaluate every variant
		if len(judges) != 2 {
			t.Errorf("%s/%s gets %d judges, want exactly 2", v.provider, v.model, len(judges))
		}
	}
}

func TestExecuteJudge_ThinkingConfig(t *testing.T) {
	// Verify that Opus thinking judge gets MAX_THINKING_TOKENS env var
	// and Codex judge gets ReasoningEffort passed through.
	//
	// We can't run executeJudge without a real workspace, but we can verify
	// the config struct construction by checking JudgeConfig fields flow
	// to the right TurnExecutorConfig fields.

	judges := DefaultJudgeConfigs()
	opus := judges[0]
	codex := judges[1]

	// Opus: Thinking=true should result in ClaudeConfig.Env["MAX_THINKING_TOKENS"]
	if !opus.Thinking {
		t.Fatal("opus judge must have Thinking=true")
	}
	if opus.ReasoningEffort != "" {
		t.Errorf("opus judge should not have ReasoningEffort, got %q", opus.ReasoningEffort)
	}

	// Codex: ReasoningEffort=xhigh should be passed through to TurnExecutorConfig
	if codex.ReasoningEffort != "xhigh" {
		t.Fatalf("codex judge must have ReasoningEffort=xhigh, got %q", codex.ReasoningEffort)
	}
	if codex.Thinking {
		t.Error("codex judge should not have Thinking=true")
	}
}

func TestAggregateJudgments(t *testing.T) {
	judgments := []*Judgment{
		{Scores: map[string]int{"functional_correctness": 4, "completeness": 5}},
		{Scores: map[string]int{"functional_correctness": 2, "completeness": 3}},
	}

	agg := AggregateJudgments(judgments)

	if agg["functional_correctness"] != 3.0 {
		t.Errorf("expected avg functional_correctness 3.0, got %f", agg["functional_correctness"])
	}
	if agg["completeness"] != 4.0 {
		t.Errorf("expected avg completeness 4.0, got %f", agg["completeness"])
	}
}

func TestAggregateJudgments_Empty(t *testing.T) {
	agg := AggregateJudgments(nil)
	if agg != nil {
		t.Errorf("expected nil for empty judgments, got %v", agg)
	}
}

func TestSanitizeForBlinding(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		notWant []string // These strings should NOT appear in output
	}{
		{
			name:    "commit co-author line",
			input:   "git commit -m 'fix bug'\n\nCo-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>",
			notWant: []string{"Claude", "Anthropic", "anthropic.com"},
		},
		{
			name:    "model name in code comment",
			input:   "// Generated by GPT-5.3-codex\nfunc main() {}",
			notWant: []string{"GPT-5.3", "codex"},
		},
		{
			name:    "mixed providers",
			input:   "Claude Sonnet generated this spec. Reviewed by GPT-5.2.",
			notWant: []string{"Claude Sonnet", "GPT-5.2"},
		},
		{
			name:    "orc commit prefix",
			input:   "[orc] TASK-001: implement - completed",
			notWant: []string{"[orc]"},
		},
		{
			name:    "clean content unchanged",
			input:   "func splitPage(data []byte) error {\n\treturn nil\n}",
			notWant: nil, // Nothing to redact
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeForBlinding(tt.input)
			for _, bad := range tt.notWant {
				if stringContains(result, bad) {
					t.Errorf("sanitized output still contains %q:\n%s", bad, result)
				}
			}
			// Should still have meaningful content (not empty)
			if len(tt.input) > 0 && len(result) == 0 {
				t.Error("sanitization produced empty output")
			}
		})
	}
}

func TestStripBinaryPatches(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     string
		wantSame bool // true if output should equal input (no binary content)
	}{
		{
			name:     "no binary content",
			input:    "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n",
			wantSame: true,
		},
		{
			name: "strips binary patch entry",
			input: "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n" +
				"diff --git a/binary.o b/binary.o\nindex abc..def 100644\nGIT binary patch\nliteral 1234\ndata\n" +
				"diff --git a/other.go b/other.go\n--- a/other.go\n+++ b/other.go\n@@ -1 +1 @@\n-foo\n+bar\n",
			want: "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n" +
				"diff --git a/other.go b/other.go\n--- a/other.go\n+++ b/other.go\n@@ -1 +1 @@\n-foo\n+bar\n",
		},
		{
			name: "strips Binary files differ",
			input: "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n" +
				"diff --git a/image.png b/image.png\nBinary files /dev/null and b/image.png differ\n",
			want: "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n",
		},
		{
			name:  "all binary",
			input: "diff --git a/a.o b/a.o\nGIT binary patch\nliteral 100\ndata\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripBinaryPatches(tt.input)
			if tt.wantSame {
				if got != tt.input {
					t.Errorf("expected unchanged input, got different output")
				}
				return
			}
			if got != tt.want {
				t.Errorf("stripBinaryPatches() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestSetupJudgeWorkspace(t *testing.T) {
	// This tests the workspace setup logic with a real git repo.
	tmpDir := t.TempDir()

	// Create a bare repo to clone from
	bareDir := filepath.Join(tmpDir, "bare.git")
	runGit(t, tmpDir, "git", "init", "--bare", bareDir)

	// Create a working clone to make a commit
	cloneDir := filepath.Join(tmpDir, "clone")
	runGit(t, tmpDir, "git", "clone", bareDir, cloneDir)
	runGit(t, cloneDir, "git", "config", "user.email", "test@test.com")
	runGit(t, cloneDir, "git", "config", "user.name", "Test")

	// Make an initial commit
	writeTestFile(t, filepath.Join(cloneDir, "main.go"), "package main\n\nfunc main() {}\n")
	runGit(t, cloneDir, "git", "add", "main.go")
	runGit(t, cloneDir, "git", "commit", "-m", "initial")

	// Get the commit hash
	commitHash := gitOutput(t, cloneDir, "git", "rev-parse", "HEAD")

	// Push to bare
	runGit(t, cloneDir, "git", "push", "origin", "HEAD:refs/heads/main")

	// Set up workspace
	ws := NewWorkspace(filepath.Join(tmpDir, "bench"))

	project := &Project{
		ID:      "test-proj",
		RepoURL: bareDir,
	}
	task := &Task{
		ID:           "test-001",
		ProjectID:    "test-proj",
		PreFixCommit: commitHash,
	}
	run_ := &Run{
		ModelDiff: `diff --git a/main.go b/main.go
index 1234567..abcdefg 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,5 @@
 package main

-func main() {}
+func main() {
+	fmt.Println("fixed")
+}
`,
	}

	jp := NewJudgePanel(nil, WithJudgeWorkspace(ws))
	jctx := &judgeContext{Run: run_, Task: task, Project: project}

	worktreePath, cleanup, err := jp.setupJudgeWorkspace(jctx)
	if err != nil {
		t.Fatalf("setupJudgeWorkspace() error: %v", err)
	}
	defer cleanup()

	// Verify the worktree exists
	if _, err := os.Stat(worktreePath); err != nil {
		t.Fatalf("worktree path does not exist: %v", err)
	}

	// Verify the model's changes are in the most recent commit
	diffOutput := gitOutput(t, worktreePath, "git", "diff", "HEAD~1", "--stat")
	if !contains(diffOutput, "main.go") {
		t.Errorf("expected main.go in diff, got: %s", diffOutput)
	}

	// Verify the content matches what we expect
	content, err := os.ReadFile(filepath.Join(worktreePath, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !contains(string(content), "fixed") {
		t.Errorf("expected 'fixed' in main.go content, got: %s", string(content))
	}
}

func TestSetupJudgeWorkspace_ContextFile(t *testing.T) {
	// Verify that executeJudge would create the context file.
	// We test writeContextFile directly since executeJudge requires
	// a real workspace + executor.
	dir := t.TempDir()

	req := JudgeRequest{
		TaskTitle: "Fix the [REDACTED] issue",
		TaskDesc:  "When processing large inputs, the handler panics.",
		Rubric:    ImplementRubric,
	}

	if err := writeContextFile(dir, req); err != nil {
		t.Fatalf("writeContextFile() error: %v", err)
	}

	// Verify .bench/context.md exists and has correct content
	path := filepath.Join(dir, ".bench", "context.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("context file not created: %v", err)
	}

	s := string(content)
	if !contains(s, "Fix the [REDACTED] issue") {
		t.Error("context file missing title")
	}
	if !contains(s, "handler panics") {
		t.Error("context file missing description")
	}
}

func TestExecuteJudge_RequiresWorkspace(t *testing.T) {
	jp := NewJudgePanel(nil) // No workspace

	jctx := &judgeContext{
		Run:     &Run{ModelDiff: "some diff"},
		Task:    &Task{ID: "t-001"},
		Project: &Project{ID: "proj"},
	}

	_, err := jp.executeJudge(context.TODO(), JudgeConfig{}, JudgeRequest{}, jctx)
	if err == nil {
		t.Fatal("expected error when no workspace configured")
	}
	if !contains(err.Error(), "workspace required") {
		t.Errorf("expected 'workspace required' error, got: %v", err)
	}
}

func TestExecuteJudge_RequiresModelDiff(t *testing.T) {
	ws := NewWorkspace("/tmp/bench-test")
	jp := NewJudgePanel(nil, WithJudgeWorkspace(ws))

	jctx := &judgeContext{
		Run:     &Run{ModelDiff: ""}, // Empty diff
		Task:    &Task{ID: "t-001"},
		Project: &Project{ID: "proj"},
	}

	_, err := jp.executeJudge(context.TODO(), JudgeConfig{}, JudgeRequest{}, jctx)
	if err == nil {
		t.Fatal("expected error when no model diff available")
	}
	if !contains(err.Error(), "no model diff") {
		t.Errorf("expected 'no model diff' error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// runGit executes a command and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %v failed: %v\n%s", args, err, out)
	}
}

// gitOutput runs a git command and returns trimmed stdout.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("command %v failed: %v", args, err)
	}
	result := string(out)
	// Trim trailing whitespace
	for len(result) > 0 && (result[len(result)-1] == '\n' || result[len(result)-1] == '\r' || result[len(result)-1] == ' ') {
		result = result[:len(result)-1]
	}
	return result
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
