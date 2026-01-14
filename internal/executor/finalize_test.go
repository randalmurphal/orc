package executor

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

func TestFinalizeExecutor_Name(t *testing.T) {
	exec := NewFinalizeExecutor(nil)
	if exec.Name() != "finalize" {
		t.Errorf("expected Name() = 'finalize', got '%s'", exec.Name())
	}
}

func TestNewFinalizeExecutor_Defaults(t *testing.T) {
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
		WithFinalizeTaskDir("/tmp/test/task"),
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
	if exec.taskDir != "/tmp/test/task" {
		t.Errorf("expected taskDir = '/tmp/test/task', got '%s'", exec.taskDir)
	}
}

func TestFinalizeExecutor_getFinalizeConfig_WithOrcConfig(t *testing.T) {
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
	orcCfg := &config.Config{
		Completion: config.CompletionConfig{
			Finalize: config.FinalizeConfig{
				Enabled: false, // Disabled
			},
		},
	}

	exec := NewFinalizeExecutor(nil, WithFinalizeOrcConfig(orcCfg))

	tsk := &task.Task{
		ID:     "TASK-001",
		Title:  "Test task",
		Weight: task.WeightLarge,
	}
	phase := &plan.Phase{ID: "finalize"}
	s := state.New("TASK-001")

	result, err := exec.Execute(context.Background(), tsk, phase, s)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result.Status != plan.PhaseCompleted {
		t.Errorf("expected status = completed, got %s", result.Status)
	}
}

func TestFinalizeExecutor_Execute_NoGitService(t *testing.T) {
	exec := NewFinalizeExecutor(nil) // No git service

	tsk := &task.Task{
		ID:     "TASK-001",
		Title:  "Test task",
		Weight: task.WeightLarge,
	}
	phase := &plan.Phase{ID: "finalize"}
	s := state.New("TASK-001")

	result, err := exec.Execute(context.Background(), tsk, phase, s)
	if err == nil {
		t.Error("expected error when git service is not available")
	}
	if result.Status != plan.PhaseFailed {
		t.Errorf("expected status = failed, got %s", result.Status)
	}
}

func TestFinalizeExecutor_shouldEscalate(t *testing.T) {
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
	tsk := &task.Task{
		ID:    "TASK-001",
		Title: "Test task",
	}
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
	if !strings.Contains(prompt, "<phase_complete>true</phase_complete>") {
		t.Error("prompt should contain completion marker instructions")
	}
}

func TestBuildTestFixPrompt(t *testing.T) {
	tsk := &task.Task{
		ID:    "TASK-001",
		Title: "Test task",
	}
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
	if !strings.Contains(report, "<phase_complete>true</phase_complete>") {
		t.Error("report should contain phase completion marker")
	}
}

func TestBuildEscalationContext(t *testing.T) {
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
	ctx := buildEscalationContext(nil)
	if !strings.Contains(ctx, "requires escalation") {
		t.Error("context should indicate escalation is needed")
	}
}

func TestFinalizeResult_Fields(t *testing.T) {
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
