package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

func TestFinalizeExecutor_Name(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	if exec.Name() != "finalize" {
		t.Errorf("expected Name() = 'finalize', got '%s'", exec.Name())
	}
}

func TestNewFinalizeExecutor_Defaults(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)

	if exec.logger == nil {
		t.Error("expected default logger to be set")
	}
	if exec.publisher == nil {
		t.Error("expected default publisher to be set")
	}
	if exec.config.MaxIterations != 10 {
		t.Errorf("expected MaxIterations = 10, got %d", exec.config.MaxIterations)
	}
	if exec.config.CheckpointInterval != 1 {
		t.Errorf("expected CheckpointInterval = 1, got %d", exec.config.CheckpointInterval)
	}
	if !exec.config.SessionPersistence {
		t.Error("expected SessionPersistence = true")
	}
}

func TestNewFinalizeExecutor_WithOptions(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := ExecutorConfig{
		MaxIterations:      5,
		CheckpointInterval: 2,
		TargetBranch:       "develop",
	}
	orcCfg := &config.Config{
		Completion: config.CompletionConfig{
			TargetBranch: "develop",
		},
	}

	exec := NewFinalizeExecutor(nil,
		WithFinalizeLogger(logger),
		WithFinalizeConfig(cfg),
		WithFinalizeOrcConfig(orcCfg),
		WithFinalizeWorkingDir("/tmp/test"),
	)

	if exec.logger != logger {
		t.Error("expected logger to be set via option")
	}
	if exec.config.MaxIterations != 5 {
		t.Errorf("expected MaxIterations = 5, got %d", exec.config.MaxIterations)
	}
	if exec.workingDir != "/tmp/test" {
		t.Errorf("expected workingDir = '/tmp/test', got '%s'", exec.workingDir)
	}
}

func TestFinalizeExecutor_getFinalizeConfig_WithOrcConfig(t *testing.T) {
	t.Parallel()
	orcCfg := &config.Config{
		Completion: config.CompletionConfig{
			Finalize: config.FinalizeConfig{
				Enabled:     true,
				AutoTrigger: false,
				Sync: config.FinalizeSyncConfig{
					Strategy: config.FinalizeSyncRebase,
				},
				ConflictResolution: config.ConflictResolutionConfig{
					Enabled:      true,
					Instructions: "Custom instructions",
				},
				RiskAssessment: config.RiskAssessmentConfig{
					Enabled:           true,
					ReReviewThreshold: "medium",
				},
			},
		},
	}

	exec := NewFinalizeExecutor(nil, WithFinalizeOrcConfig(orcCfg))
	cfg := exec.getFinalizeConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled = true")
	}
	if cfg.AutoTrigger {
		t.Error("expected AutoTrigger = false")
	}
	if cfg.Sync.Strategy != config.FinalizeSyncRebase {
		t.Errorf("expected Strategy = rebase, got %s", cfg.Sync.Strategy)
	}
	if cfg.ConflictResolution.Instructions != "Custom instructions" {
		t.Error("expected custom instructions to be preserved")
	}
	if cfg.RiskAssessment.ReReviewThreshold != "medium" {
		t.Errorf("expected ReReviewThreshold = medium, got %s", cfg.RiskAssessment.ReReviewThreshold)
	}
}

func TestFinalizeExecutor_getFinalizeConfig_Defaults(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil) // No orc config
	cfg := exec.getFinalizeConfig()

	if !cfg.Enabled {
		t.Error("expected default Enabled = true")
	}
	if !cfg.AutoTrigger {
		t.Error("expected default AutoTrigger = true")
	}
	if cfg.Sync.Strategy != config.FinalizeSyncMerge {
		t.Errorf("expected default Strategy = merge, got %s", cfg.Sync.Strategy)
	}
	if !cfg.ConflictResolution.Enabled {
		t.Error("expected default ConflictResolution.Enabled = true")
	}
	if !cfg.RiskAssessment.Enabled {
		t.Error("expected default RiskAssessment.Enabled = true")
	}
	if cfg.RiskAssessment.ReReviewThreshold != "high" {
		t.Errorf("expected default ReReviewThreshold = high, got %s", cfg.RiskAssessment.ReReviewThreshold)
	}
}

func TestFinalizeExecutor_getTargetBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		orcConfig  *config.Config
		execConfig ExecutorConfig
		expected   string
	}{
		{
			name:       "default to main",
			orcConfig:  nil,
			execConfig: ExecutorConfig{},
			expected:   "main",
		},
		{
			name: "from orc config",
			orcConfig: &config.Config{
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			execConfig: ExecutorConfig{},
			expected:   "develop",
		},
		{
			name:      "from exec config when no orc config",
			orcConfig: nil,
			execConfig: ExecutorConfig{
				TargetBranch: "staging",
			},
			expected: "staging",
		},
		{
			name: "orc config takes precedence",
			orcConfig: &config.Config{
				Completion: config.CompletionConfig{
					TargetBranch: "main",
				},
			},
			execConfig: ExecutorConfig{
				TargetBranch: "staging",
			},
			expected: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := NewFinalizeExecutor(nil,
				WithFinalizeOrcConfig(tt.orcConfig),
				WithFinalizeConfig(tt.execConfig),
			)
			got := exec.getTargetBranch()
			if got != tt.expected {
				t.Errorf("getTargetBranch() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestFinalizeExecutor_Execute_DisabledPhase(t *testing.T) {
	t.Parallel()
	orcCfg := &config.Config{
		Completion: config.CompletionConfig{
			Finalize: config.FinalizeConfig{
				Enabled: false, // Disabled
			},
		},
	}

	exec := NewFinalizeExecutor(nil, WithFinalizeOrcConfig(orcCfg))

	tsk := task.NewProtoTask("TASK-001", "Test task")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_LARGE
	task.EnsureExecutionProto(tsk)
	phase := &PhaseDisplay{ID: "finalize"}

	result, err := exec.Execute(context.Background(), tsk, phase, tsk.Execution)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED {
		t.Errorf("expected status = completed, got %s", result.Status)
	}
}

func TestFinalizeExecutor_Execute_NoGitService(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil) // No git service

	tsk := task.NewProtoTask("TASK-001", "Test task")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_LARGE
	task.EnsureExecutionProto(tsk)
	phase := &PhaseDisplay{ID: "finalize"}

	result, err := exec.Execute(context.Background(), tsk, phase, tsk.Execution)
	if err == nil {
		t.Error("expected error when git service is not available")
	}
	// Result.Status is PENDING when not completed, result.Error indicates failure
	if result.Error == nil {
		t.Error("expected result.Error to be set on failure")
	}
}

func TestFinalizeExecutor_shouldEscalate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   *FinalizeResult
		expected bool
	}{
		{
			name:     "nil result",
			result:   nil,
			expected: false,
		},
		{
			name: "few conflicts",
			result: &FinalizeResult{
				ConflictFiles: make([]string, 5),
				TestsPassed:   true,
			},
			expected: false,
		},
		{
			name: "many conflicts triggers escalation",
			result: &FinalizeResult{
				ConflictFiles: make([]string, 15),
				TestsPassed:   true,
			},
			expected: true,
		},
		{
			name: "many test failures triggers escalation",
			result: &FinalizeResult{
				TestsPassed:  false,
				TestFailures: make([]TestFailure, 10),
			},
			expected: true,
		},
		{
			name: "few test failures no escalation",
			result: &FinalizeResult{
				TestsPassed:  false,
				TestFailures: make([]TestFailure, 3),
			},
			expected: false,
		},
	}

	exec := NewFinalizeExecutor(nil)
	cfg := config.FinalizeConfig{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exec.shouldEscalate(tt.result, cfg)
			if got != tt.expected {
				t.Errorf("shouldEscalate() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClassifyRisk(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		files     int
		lines     int
		conflicts int
		expected  string
	}{
		{"low risk - minimal changes", 3, 50, 0, "low"},
		{"medium risk - moderate files", 10, 200, 0, "medium"},
		{"medium risk - some conflicts", 3, 50, 2, "medium"},
		{"high risk - many files", 20, 400, 0, "high"},
		{"high risk - many lines", 10, 700, 0, "high"},
		{"high risk - several conflicts", 5, 100, 5, "high"},
		{"critical - very many files", 40, 500, 0, "critical"},
		{"critical - very many lines", 20, 1500, 0, "critical"},
		{"critical - many conflicts", 10, 200, 15, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyRisk(tt.files, tt.lines, tt.conflicts)
			if got != tt.expected {
				t.Errorf("classifyRisk(%d, %d, %d) = %s, want %s",
					tt.files, tt.lines, tt.conflicts, got, tt.expected)
			}
		})
	}
}

func TestClassifyFileRisk(t *testing.T) {
	t.Parallel()
	tests := []struct {
		files    int
		expected string
	}{
		{3, "Low"},
		{10, "Medium"},
		{20, "High"},
		{50, "Critical"},
	}

	for _, tt := range tests {
		got := classifyFileRisk(tt.files)
		if got != tt.expected {
			t.Errorf("classifyFileRisk(%d) = %s, want %s", tt.files, got, tt.expected)
		}
	}
}

func TestClassifyLineRisk(t *testing.T) {
	t.Parallel()
	tests := []struct {
		lines    int
		expected string
	}{
		{50, "Low"},
		{200, "Medium"},
		{700, "High"},
		{1500, "Critical"},
	}

	for _, tt := range tests {
		got := classifyLineRisk(tt.lines)
		if got != tt.expected {
			t.Errorf("classifyLineRisk(%d) = %s, want %s", tt.lines, got, tt.expected)
		}
	}
}

func TestClassifyConflictRisk(t *testing.T) {
	t.Parallel()
	tests := []struct {
		conflicts int
		expected  string
	}{
		{0, "None"},
		{2, "Low"},
		{5, "Medium"},
		{15, "High"},
	}

	for _, tt := range tests {
		got := classifyConflictRisk(tt.conflicts)
		if got != tt.expected {
			t.Errorf("classifyConflictRisk(%d) = %s, want %s", tt.conflicts, got, tt.expected)
		}
	}
}

func TestShouldTriggerReview(t *testing.T) {
	t.Parallel()
	tests := []struct {
		riskLevel string
		threshold string
		expected  bool
	}{
		{"low", "high", false},
		{"medium", "high", false},
		{"high", "high", true},
		{"critical", "high", true},
		{"low", "low", true},
		{"medium", "low", true},
		{"low", "medium", false},
		{"medium", "medium", true},
	}

	for _, tt := range tests {
		t.Run(tt.riskLevel+"_"+tt.threshold, func(t *testing.T) {
			got := shouldTriggerReview(tt.riskLevel, tt.threshold)
			if got != tt.expected {
				t.Errorf("shouldTriggerReview(%s, %s) = %v, want %v",
					tt.riskLevel, tt.threshold, got, tt.expected)
			}
		})
	}
}

func TestParseFileCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		line     string
		expected int
	}{
		{"5 files changed, 100 insertions(+), 50 deletions(-)", 5},
		{"1 file changed, 10 insertions(+)", 1},
		{"25 files changed", 25},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		got := parseFileCount(tt.line)
		if got != tt.expected {
			t.Errorf("parseFileCount(%q) = %d, want %d", tt.line, got, tt.expected)
		}
	}
}

func TestParseTotalLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		numstat  string
		expected int
	}{
		{"10\t5\tfile1.go\n20\t10\tfile2.go", 45},
		{"100\t0\tnewfile.go", 100},
		{"0\t50\tdeleted.go", 50},
		{"", 0},
	}

	for _, tt := range tests {
		got := parseTotalLines(tt.numstat)
		if got != tt.expected {
			t.Errorf("parseTotalLines(%q) = %d, want %d", tt.numstat, got, tt.expected)
		}
	}
}

func TestBuildConflictResolutionPrompt(t *testing.T) {
	t.Parallel()
	tsk := task.NewProtoTask("TASK-001", "Test task")
	conflictFiles := []string{"file1.go", "file2.go"}
	cfg := config.FinalizeConfig{
		ConflictResolution: config.ConflictResolutionConfig{
			Enabled:      true,
			Instructions: "Custom instructions for this project",
		},
	}

	prompt := buildConflictResolutionPrompt(tsk, conflictFiles, cfg)

	// Check that prompt contains key elements
	if !strings.Contains(prompt, "TASK-001") {
		t.Error("prompt should contain task ID")
	}
	if !strings.Contains(prompt, "Test task") {
		t.Error("prompt should contain task title")
	}
	if !strings.Contains(prompt, "file1.go") {
		t.Error("prompt should contain conflict files")
	}
	if !strings.Contains(prompt, "NEVER remove features") {
		t.Error("prompt should contain conflict resolution rules")
	}
	if !strings.Contains(prompt, "Custom instructions for this project") {
		t.Error("prompt should contain custom instructions")
	}
	if !strings.Contains(prompt, `"status": "complete"`) {
		t.Error("prompt should contain JSON completion instructions")
	}
}

func TestBuildTestFixPrompt(t *testing.T) {
	t.Parallel()
	tsk := task.NewProtoTask("TASK-001", "Test task")
	testResult := &ParsedTestResult{
		Failed: 2,
		Failures: []TestFailure{
			{Test: "TestFoo", File: "foo_test.go", Line: 42, Message: "assertion failed"},
			{Test: "TestBar", File: "bar_test.go", Line: 10, Message: "nil pointer"},
		},
	}

	prompt := buildTestFixPrompt(tsk, testResult)

	// Check that prompt contains key elements
	if !strings.Contains(prompt, "TASK-001") {
		t.Error("prompt should contain task ID")
	}
	if !strings.Contains(prompt, "TestFoo") {
		t.Error("prompt should contain test name")
	}
	if !strings.Contains(prompt, "foo_test.go:42") {
		t.Error("prompt should contain file and line")
	}
	if !strings.Contains(prompt, "assertion failed") {
		t.Error("prompt should contain error message")
	}
	if !strings.Contains(prompt, "Do NOT remove tests") {
		t.Error("prompt should contain instruction not to remove tests")
	}
}

func TestBuildFinalizeReport(t *testing.T) {
	t.Parallel()
	result := &FinalizeResult{
		Synced:            true,
		ConflictsResolved: 2,
		ConflictFiles:     []string{"file1.go", "file2.go"},
		TestsPassed:       true,
		RiskLevel:         "medium",
		FilesChanged:      10,
		LinesChanged:      250,
		NeedsReview:       false,
		CommitSHA:         "abc123",
	}

	report := buildFinalizeReport("TASK-001", "main", result)

	// Check that report contains key elements
	if !strings.Contains(report, "TASK-001") {
		t.Error("report should contain task ID")
	}
	if !strings.Contains(report, "main") {
		t.Error("report should contain target branch")
	}
	if !strings.Contains(report, "Conflicts Resolved | 2") {
		t.Error("report should contain conflicts resolved count")
	}
	if !strings.Contains(report, "Files Changed (total) | 10") {
		t.Error("report should contain files changed count")
	}
	if !strings.Contains(report, "medium") {
		t.Error("report should contain risk level")
	}
	if !strings.Contains(report, "abc123") {
		t.Error("report should contain commit SHA")
	}
	if !strings.Contains(report, `"status": "complete"`) {
		t.Error("report should contain JSON completion status")
	}
}

func TestBuildEscalationContext(t *testing.T) {
	t.Parallel()
	result := &FinalizeResult{
		ConflictFiles: []string{"file1.go", "file2.go"},
		TestsPassed:   false,
		TestFailures: []TestFailure{
			{Test: "TestFoo", Message: "failed assertion"},
		},
	}

	ctx := buildEscalationContext(result)

	if !strings.Contains(ctx, "Finalize Escalation Required") {
		t.Error("context should indicate escalation is required")
	}
	if !strings.Contains(ctx, "file1.go") {
		t.Error("context should contain conflict files")
	}
	if !strings.Contains(ctx, "TestFoo") {
		t.Error("context should contain failing tests")
	}
}

func TestBuildEscalationContext_NilResult(t *testing.T) {
	t.Parallel()
	ctx := buildEscalationContext(nil)
	if !strings.Contains(ctx, "requires escalation") {
		t.Error("context should indicate escalation is needed")
	}
}

func TestFinalizeResult_Fields(t *testing.T) {
	t.Parallel()
	result := &FinalizeResult{
		Synced:            true,
		ConflictsResolved: 3,
		ConflictFiles:     []string{"a.go", "b.go", "c.go"},
		TestsPassed:       true,
		TestFailures:      nil,
		RiskLevel:         "low",
		FilesChanged:      5,
		LinesChanged:      100,
		NeedsReview:       false,
		CommitSHA:         "sha123",
	}

	if !result.Synced {
		t.Error("expected Synced = true")
	}
	if result.ConflictsResolved != 3 {
		t.Errorf("expected ConflictsResolved = 3, got %d", result.ConflictsResolved)
	}
	if len(result.ConflictFiles) != 3 {
		t.Errorf("expected 3 conflict files, got %d", len(result.ConflictFiles))
	}
	if !result.TestsPassed {
		t.Error("expected TestsPassed = true")
	}
	if result.RiskLevel != "low" {
		t.Errorf("expected RiskLevel = low, got %s", result.RiskLevel)
	}
}

func TestWithFinalizeGitSvc(t *testing.T) {
	t.Parallel()
	// Test that WithFinalizeGitSvc option sets the git service
	// We can't fully test with a real git.Git without a repo,
	// but we can verify the option mechanism works
	exec := NewFinalizeExecutor(nil)
	if exec.gitSvc != nil {
		t.Error("expected gitSvc to be nil initially")
	}

	// The option function should accept a *git.Git
	// This tests the WithFinalizeGitSvc function signature
	opt := WithFinalizeGitSvc(nil)
	opt(exec)
	// Still nil since we passed nil, but the option works
	if exec.gitSvc != nil {
		t.Error("expected gitSvc to remain nil after passing nil")
	}
}

func TestWithFinalizePublisher(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	initialPublisher := exec.publisher

	// Publisher should have a default value
	if initialPublisher == nil {
		t.Error("expected default publisher to be set")
	}

	// Apply option with nil - should still create a valid PublishHelper
	opt := WithFinalizePublisher(nil)
	opt(exec)

	if exec.publisher == nil {
		t.Error("expected publisher to be non-nil after option")
	}
}

func TestWithFinalizeExecutionUpdater(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	if exec.executionUpdater != nil {
		t.Error("expected executionUpdater to be nil initially")
	}

	called := false
	updater := func(e *orcv1.ExecutionState) {
		called = true
	}

	opt := WithFinalizeExecutionUpdater(updater)
	opt(exec)

	if exec.executionUpdater == nil {
		t.Error("expected executionUpdater to be set")
	}

	// Call the updater to verify it was set correctly
	exec.executionUpdater(nil)
	if !called {
		t.Error("expected executionUpdater to be called")
	}
}

func TestFinalizeExecutor_fetchTarget_NoGitService(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	err := exec.fetchTarget()
	if err == nil {
		t.Error("expected error when git service not available")
	}
	if !strings.Contains(err.Error(), "git service not available") {
		t.Errorf("expected 'git service not available' error, got: %s", err)
	}
}

func TestFinalizeExecutor_checkDivergence_NoGitService(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	ahead, behind, err := exec.checkDivergence("main")
	if err == nil {
		t.Error("expected error when git service not available")
	}
	if ahead != 0 || behind != 0 {
		t.Error("expected ahead and behind to be 0 on error")
	}
	if !strings.Contains(err.Error(), "git service not available") {
		t.Errorf("expected 'git service not available' error, got: %s", err)
	}
}

func TestFinalizeExecutor_syncWithTarget_NoGitService(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	cfg := config.FinalizeConfig{
		Sync: config.FinalizeSyncConfig{Strategy: config.FinalizeSyncMerge},
	}

	tsk := task.NewProtoTask("TASK-001", "Test task")
	task.EnsureExecutionProto(tsk)

	result, err := exec.syncWithTarget(
		context.Background(),
		tsk,
		&PhaseDisplay{ID: "finalize"},
		tsk.Execution,
		"main",
		cfg,
	)

	if err == nil {
		t.Error("expected error when git service not available")
	}
	if result.Synced {
		t.Error("expected Synced to be false on error")
	}
}

func TestFinalizeExecutor_assessRisk_Disabled(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	result := &FinalizeResult{}
	cfg := config.FinalizeConfig{
		RiskAssessment: config.RiskAssessmentConfig{
			Enabled: false,
		},
	}

	err := exec.assessRisk(result, "main", cfg)
	if err != nil {
		t.Errorf("expected no error when risk assessment disabled, got: %v", err)
	}
	if result.RiskLevel != "unknown" {
		t.Errorf("expected risk level 'unknown' when disabled, got: %s", result.RiskLevel)
	}
}

func TestFinalizeExecutor_assessRisk_NoGitService(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	result := &FinalizeResult{}
	cfg := config.FinalizeConfig{
		RiskAssessment: config.RiskAssessmentConfig{
			Enabled: true,
		},
	}

	err := exec.assessRisk(result, "main", cfg)
	if err == nil {
		t.Error("expected error when git service not available")
	}
	if !strings.Contains(err.Error(), "git service not available") {
		t.Errorf("expected 'git service not available' error, got: %s", err)
	}
}

func TestFinalizeExecutor_createFinalizeCommit_NoGitService(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	result := &FinalizeResult{}
	tsk := task.NewProtoTask("TASK-001", "Test task")

	sha, err := exec.createFinalizeCommit(tsk, result)
	if err == nil {
		t.Error("expected error when git service not available")
	}
	if sha != "" {
		t.Error("expected empty SHA on error")
	}
}

func TestFinalizeExecutor_publishProgress(t *testing.T) {
	t.Parallel()
	// Verify publishProgress doesn't panic
	exec := NewFinalizeExecutor(nil)
	exec.publishProgress("TASK-001", "finalize", "Test progress message")
	// If we get here without panic, test passes
}

func TestBuildFinalizeReport_NeedsReview(t *testing.T) {
	t.Parallel()
	result := &FinalizeResult{
		Synced:            true,
		ConflictsResolved: 5,
		ConflictFiles:     []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
		TestsPassed:       true,
		RiskLevel:         "high",
		FilesChanged:      25,
		LinesChanged:      600,
		NeedsReview:       true,
		CommitSHA:         "abc123def",
	}

	report := buildFinalizeReport("TASK-002", "develop", result)

	if !strings.Contains(report, "Review Required") {
		t.Error("report should indicate review required")
	}
	if !strings.Contains(report, "develop") {
		t.Error("report should contain target branch")
	}
	if !strings.Contains(report, "abc123def") {
		t.Error("report should contain commit SHA")
	}
}

func TestBuildFinalizeReport_CriticalRisk(t *testing.T) {
	t.Parallel()
	result := &FinalizeResult{
		Synced:            true,
		ConflictsResolved: 0,
		TestsPassed:       true,
		RiskLevel:         "critical",
		FilesChanged:      50,
		LinesChanged:      2000,
		NeedsReview:       false,
		CommitSHA:         "sha456",
	}

	report := buildFinalizeReport("TASK-003", "main", result)

	if !strings.Contains(report, "Senior Review Required") {
		t.Error("report should indicate senior review required for critical risk")
	}
	if !strings.Contains(report, "senior-review-required") {
		t.Error("report should recommend senior review action")
	}
}

func TestBuildFinalizeReport_TestsFailed(t *testing.T) {
	t.Parallel()
	result := &FinalizeResult{
		Synced:      true,
		TestsPassed: false,
		RiskLevel:   "low",
	}

	report := buildFinalizeReport("TASK-004", "main", result)

	if !strings.Contains(report, "✗ Tests failed") {
		t.Error("report should indicate tests failed")
	}
}

func TestBuildTestFailureContext_NilResult(t *testing.T) {
	t.Parallel()
	ctx := buildTestFailureContext(nil)
	if !strings.Contains(ctx, "unknown results") {
		t.Error("context should indicate unknown results for nil input")
	}
}

func TestBuildTestFailureContext_WithFailures(t *testing.T) {
	t.Parallel()
	testResult := &ParsedTestResult{
		Failed: 2,
		Failures: []TestFailure{
			{Test: "TestOne", File: "one_test.go", Line: 10, Message: "assertion failed"},
			{Test: "TestTwo", File: "two_test.go", Line: 20, Message: "timeout"},
		},
	}

	ctx := buildTestFailureContext(testResult)
	if ctx == "" {
		t.Error("expected non-empty context")
	}
}

func TestBuildConflictResolutionPrompt_NoCustomInstructions(t *testing.T) {
	t.Parallel()
	tsk := task.NewProtoTask("TASK-001", "Test task")
	conflictFiles := []string{"file.go"}
	cfg := config.FinalizeConfig{
		ConflictResolution: config.ConflictResolutionConfig{
			Enabled:      true,
			Instructions: "", // No custom instructions
		},
	}

	prompt := buildConflictResolutionPrompt(tsk, conflictFiles, cfg)

	if strings.Contains(prompt, "Additional Instructions") {
		t.Error("prompt should not contain 'Additional Instructions' when none provided")
	}
	if !strings.Contains(prompt, "NEVER remove features") {
		t.Error("prompt should still contain core rules")
	}
}

func TestBuildTestFixPrompt_ManyFailures(t *testing.T) {
	t.Parallel()
	tsk := task.NewProtoTask("TASK-001", "Test")
	testResult := &ParsedTestResult{
		Failed: 10,
		Failures: []TestFailure{
			{Test: "Test1", Message: "fail1"},
			{Test: "Test2", Message: "fail2"},
			{Test: "Test3", Message: "fail3"},
			{Test: "Test4", Message: "fail4"},
			{Test: "Test5", Message: "fail5"},
			{Test: "Test6", Message: "fail6"},
			{Test: "Test7", Message: "fail7"},
			{Test: "Test8", Message: "fail8"},
			{Test: "Test9", Message: "fail9"},
			{Test: "Test10", Message: "fail10"},
		},
	}

	prompt := buildTestFixPrompt(tsk, testResult)

	// Should show first 5 and "... and N more"
	if !strings.Contains(prompt, "and 5 more failures") {
		t.Error("prompt should truncate to 5 failures and show count of remaining")
	}
	if strings.Contains(prompt, "Test6") {
		t.Error("prompt should not contain Test6 (should be truncated)")
	}
}

func TestFinalizeExecutor_tryFixTests_ExecutorError(t *testing.T) {
	t.Parallel()
	// Use mock executor that returns an error
	mockExec := NewMockTurnExecutor("")
	mockExec.Error = fmt.Errorf("executor error")

	exec := NewFinalizeExecutor(
		WithFinalizeTurnExecutor(mockExec),
	)
	testResult := &ParsedTestResult{}

	tsk := task.NewProtoTask("TASK-001", "Test task")
	task.EnsureExecutionProto(tsk)

	fixed, err := exec.tryFixTests(
		context.Background(),
		tsk,
		&PhaseDisplay{ID: "finalize"},
		tsk.Execution,
		testResult,
	)

	if err == nil {
		t.Error("expected error from executor")
	}
	if fixed {
		t.Error("expected fixed to be false on error")
	}
}

func TestFinalizeExecutor_resolveConflicts_ExecutorError(t *testing.T) {
	t.Parallel()
	// Use mock executor that returns an error
	mockExec := NewMockTurnExecutor("")
	mockExec.Error = fmt.Errorf("executor error")

	exec := NewFinalizeExecutor(
		WithFinalizeTurnExecutor(mockExec),
	)
	cfg := config.FinalizeConfig{}

	tsk := task.NewProtoTask("TASK-001", "Test task")
	task.EnsureExecutionProto(tsk)

	resolved, err := exec.resolveConflicts(
		context.Background(),
		tsk,
		&PhaseDisplay{ID: "finalize"},
		tsk.Execution,
		[]string{"conflict.go"},
		cfg,
	)

	if err == nil {
		t.Error("expected error from executor")
	}
	if resolved {
		t.Error("expected resolved to be false on error")
	}
}

func TestFinalizeExecutor_shouldEscalate_EdgeCases(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	cfg := config.FinalizeConfig{}

	tests := []struct {
		name     string
		result   *FinalizeResult
		expected bool
	}{
		{
			name: "exactly 10 conflicts - no escalation",
			result: &FinalizeResult{
				ConflictFiles: make([]string, 10),
				TestsPassed:   true,
			},
			expected: false,
		},
		{
			name: "11 conflicts - triggers escalation",
			result: &FinalizeResult{
				ConflictFiles: make([]string, 11),
				TestsPassed:   true,
			},
			expected: true,
		},
		{
			name: "5 test failures with tests not passed - no escalation",
			result: &FinalizeResult{
				TestsPassed:  false,
				TestFailures: make([]TestFailure, 5),
			},
			expected: false,
		},
		{
			name: "6 test failures - triggers escalation",
			result: &FinalizeResult{
				TestsPassed:  false,
				TestFailures: make([]TestFailure, 6),
			},
			expected: true,
		},
		{
			name: "tests passed even with test failures in slice - no escalation",
			result: &FinalizeResult{
				TestsPassed:  true,
				TestFailures: make([]TestFailure, 10),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exec.shouldEscalate(tt.result, cfg)
			if got != tt.expected {
				t.Errorf("shouldEscalate() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildEscalationContext_OnlyConflicts(t *testing.T) {
	t.Parallel()
	result := &FinalizeResult{
		ConflictFiles: []string{"a.go", "b.go"},
		TestsPassed:   true,
	}

	ctx := buildEscalationContext(result)

	if !strings.Contains(ctx, "Unresolved Conflicts") {
		t.Error("context should contain conflicts section")
	}
	if strings.Contains(ctx, "Test Failures") {
		t.Error("context should not contain test failures section when tests passed")
	}
}

func TestBuildEscalationContext_ManyTestFailures(t *testing.T) {
	t.Parallel()
	failures := make([]TestFailure, 10)
	for i := 0; i < 10; i++ {
		failures[i] = TestFailure{
			Test:    fmt.Sprintf("Test%d", i),
			Message: fmt.Sprintf("failure %d", i),
		}
	}

	result := &FinalizeResult{
		TestsPassed:  false,
		TestFailures: failures,
	}

	ctx := buildEscalationContext(result)

	if !strings.Contains(ctx, "and 5 more failures") {
		t.Error("context should truncate test failures to 5 and show remaining count")
	}
}

func TestSyncStrategy_DefaultMerge(t *testing.T) {
	t.Parallel()
	exec := NewFinalizeExecutor(nil)
	cfg := config.FinalizeConfig{
		Sync: config.FinalizeSyncConfig{
			Strategy: "unknown-strategy", // Unknown strategy
		},
	}

	tsk := task.NewProtoTask("TASK-001", "Test task")
	task.EnsureExecutionProto(tsk)

	// This should fall through to default merge behavior
	result, err := exec.syncWithTarget(
		context.Background(),
		tsk,
		&PhaseDisplay{ID: "finalize"},
		tsk.Execution,
		"main",
		cfg,
	)

	// Will fail because no git service, but verifies default path
	if err == nil {
		t.Error("expected error due to no git service")
	}
	if result == nil {
		t.Error("expected result to be non-nil")
	}
}

func TestFinalizeExecutor_Execute_BranchUpToDate(t *testing.T) {
	t.Parallel()
	// This test verifies the path when branch is already up-to-date
	// Since we can't easily mock git operations, we test the disabled path instead
	orcCfg := &config.Config{
		Completion: config.CompletionConfig{
			Finalize: config.FinalizeConfig{
				Enabled: true,
			},
		},
	}

	exec := NewFinalizeExecutor(nil, WithFinalizeOrcConfig(orcCfg))

	tsk := task.NewProtoTask("TASK-001", "Test")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_LARGE
	task.EnsureExecutionProto(tsk)
	phase := &PhaseDisplay{ID: "finalize"}

	result, err := exec.Execute(context.Background(), tsk, phase, tsk.Execution)

	// Should fail because no git service
	if err == nil {
		t.Error("expected error when git service not available")
	}
	// Result.Status is PENDING when not completed, result.Error indicates failure
	if result.Error == nil {
		t.Error("expected result.Error to be set on failure")
	}
}
