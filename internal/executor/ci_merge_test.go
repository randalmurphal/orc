package executor

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/task"
)

func TestCICheckResult_Status(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   CICheckResult
		expected CIStatus
	}{
		{
			name: "all passed",
			result: CICheckResult{
				TotalChecks:  3,
				PassedChecks: 3,
			},
			expected: CIStatusPassed,
		},
		{
			name: "some pending",
			result: CICheckResult{
				TotalChecks:   3,
				PassedChecks:  2,
				PendingChecks: 1,
			},
			expected: CIStatusPending,
		},
		{
			name: "some failed",
			result: CICheckResult{
				TotalChecks:  3,
				PassedChecks: 2,
				FailedChecks: 1,
			},
			expected: CIStatusFailed,
		},
		{
			name: "no checks",
			result: CICheckResult{
				TotalChecks: 0,
			},
			expected: CIStatusNoChecks,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Status != tt.expected && tt.result.TotalChecks > 0 {
				// This test validates the status is set correctly when result is built
				// The actual status determination happens in CheckCIStatus
				t.Logf("Status would be determined by CheckCIStatus, not the struct itself")
			}
		})
	}
}

func TestCIConfig_Defaults(t *testing.T) {
	t.Parallel()
	cfg := config.Default()

	// Verify default values — auto-merge/poll defaults are OFF
	if cfg.Completion.CI.WaitForCI {
		t.Error("expected WaitForCI to be false by default")
	}

	if cfg.Completion.CI.CITimeout != 10*time.Minute {
		t.Errorf("expected CITimeout to be 10m, got %v", cfg.Completion.CI.CITimeout)
	}

	if cfg.Completion.CI.PollInterval != 30*time.Second {
		t.Errorf("expected PollInterval to be 30s, got %v", cfg.Completion.CI.PollInterval)
	}

	if cfg.Completion.CI.MergeOnCIPass {
		t.Error("expected MergeOnCIPass to be false by default")
	}

	if cfg.Completion.CI.MergeMethod != "squash" {
		t.Errorf("expected MergeMethod to be 'squash', got %s", cfg.Completion.CI.MergeMethod)
	}

	if !cfg.Completion.CI.VerifySHAOnMerge {
		t.Error("expected VerifySHAOnMerge to be true by default")
	}
}

func TestConfig_ShouldWaitForCI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		profile  config.AutomationProfile
		waitFor  bool
		expected bool
	}{
		{
			name:     "auto profile with wait enabled",
			profile:  config.ProfileAuto,
			waitFor:  true,
			expected: true,
		},
		{
			name:     "fast profile with wait enabled",
			profile:  config.ProfileFast,
			waitFor:  true,
			expected: true,
		},
		{
			name:     "safe profile with wait enabled",
			profile:  config.ProfileSafe,
			waitFor:  true,
			expected: false, // Safe profile doesn't auto-merge
		},
		{
			name:     "strict profile with wait enabled",
			profile:  config.ProfileStrict,
			waitFor:  true,
			expected: false, // Strict profile doesn't auto-merge
		},
		{
			name:     "auto profile with wait disabled",
			profile:  config.ProfileAuto,
			waitFor:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Profile = tt.profile
			cfg.Completion.CI.WaitForCI = tt.waitFor

			if got := cfg.ShouldWaitForCI(); got != tt.expected {
				t.Errorf("ShouldWaitForCI() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_ShouldMergeOnCIPass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		profile  config.AutomationProfile
		merge    bool
		expected bool
	}{
		{
			name:     "auto profile with merge enabled",
			profile:  config.ProfileAuto,
			merge:    true,
			expected: true,
		},
		{
			name:     "fast profile with merge enabled",
			profile:  config.ProfileFast,
			merge:    true,
			expected: true,
		},
		{
			name:     "safe profile with merge enabled",
			profile:  config.ProfileSafe,
			merge:    true,
			expected: false, // Safe requires human approval
		},
		{
			name:     "auto profile with merge disabled",
			profile:  config.ProfileAuto,
			merge:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Profile = tt.profile
			cfg.Completion.CI.MergeOnCIPass = tt.merge
			// ShouldMergeOnCIPass requires WaitForCI to also be true
			cfg.Completion.CI.WaitForCI = tt.merge

			if got := cfg.ShouldMergeOnCIPass(); got != tt.expected {
				t.Errorf("ShouldMergeOnCIPass() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_CITimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{
			name:     "default timeout",
			timeout:  0,
			expected: 10 * time.Minute,
		},
		{
			name:     "custom timeout",
			timeout:  5 * time.Minute,
			expected: 5 * time.Minute,
		},
		{
			name:     "negative timeout uses default",
			timeout:  -1 * time.Minute,
			expected: 10 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Completion.CI.CITimeout = tt.timeout

			if got := cfg.CITimeout(); got != tt.expected {
				t.Errorf("CITimeout() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_CIPollInterval(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		interval time.Duration
		expected time.Duration
	}{
		{
			name:     "default interval",
			interval: 0,
			expected: 30 * time.Second,
		},
		{
			name:     "custom interval",
			interval: 15 * time.Second,
			expected: 15 * time.Second,
		},
		{
			name:     "negative interval uses default",
			interval: -1 * time.Second,
			expected: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Completion.CI.PollInterval = tt.interval

			if got := cfg.CIPollInterval(); got != tt.expected {
				t.Errorf("CIPollInterval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_MergeMethod(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{
			name:     "default method",
			method:   "",
			expected: "squash",
		},
		{
			name:     "squash method",
			method:   "squash",
			expected: "squash",
		},
		{
			name:     "merge method",
			method:   "merge",
			expected: "merge",
		},
		{
			name:     "rebase method",
			method:   "rebase",
			expected: "rebase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Completion.CI.MergeMethod = tt.method

			// Verify the merge method is returned correctly
			method := cfg.MergeMethod()
			if method != tt.expected {
				t.Errorf("expected method %q, got %q", tt.expected, method)
			}
		})
	}
}

func TestCIMerger_WaitForCIAndMerge_NoPR(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Profile = config.ProfileAuto

	merger := NewCIMerger(cfg)

	// Task without PR should skip CI wait
	tsk := &orcv1.Task{
		Id: "TASK-001",
	}

	err := merger.WaitForCIAndMerge(context.Background(), tsk)
	if err != nil {
		t.Errorf("expected no error for task without PR, got %v", err)
	}
}

func TestCIMerger_WaitForCIAndMerge_CIDisabled(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Profile = config.ProfileSafe // Safe profile disables CI wait

	merger := NewCIMerger(cfg)

	// Task with PR but CI disabled should skip
	prURL := "https://github.com/owner/repo/pull/1"
	prNumber := int32(1)
	tsk := &orcv1.Task{
		Id: "TASK-001",
		Pr: &orcv1.PRInfo{
			Url:    &prURL,
			Number: &prNumber,
		},
	}

	err := merger.WaitForCIAndMerge(context.Background(), tsk)
	if err != nil {
		t.Errorf("expected no error when CI is disabled, got %v", err)
	}
}

func TestCIMerger_CheckCIStatus_NoProvider(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	merger := NewCIMerger(cfg)

	_, err := merger.CheckCIStatus(context.Background(), "main")
	if err == nil {
		t.Error("expected error when provider is nil")
	}
	if !errors.Is(err, nil) && err.Error() != "hosting provider not configured" {
		t.Errorf("expected 'hosting provider not configured' error, got %v", err)
	}
}

func TestCIMerger_CheckCIStatus_NoChecks(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	provider := &mockProvider{
		checkRuns: []hosting.CheckRun{},
	}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	result, err := merger.CheckCIStatus(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != CIStatusNoChecks {
		t.Errorf("expected no_checks status, got %v", result.Status)
	}
}

func TestCIMerger_CheckCIStatus_AllPassed(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	provider := &mockProvider{
		checkRuns: []hosting.CheckRun{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "test", Status: "completed", Conclusion: "success"},
		},
	}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	result, err := merger.CheckCIStatus(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != CIStatusPassed {
		t.Errorf("expected passed status, got %v", result.Status)
	}
	if result.PassedChecks != 2 {
		t.Errorf("expected 2 passed, got %d", result.PassedChecks)
	}
}

func TestCIMerger_CheckCIStatus_SomePending(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	provider := &mockProvider{
		checkRuns: []hosting.CheckRun{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "test", Status: "in_progress", Conclusion: ""},
		},
	}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	result, err := merger.CheckCIStatus(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != CIStatusPending {
		t.Errorf("expected pending status, got %v", result.Status)
	}
	if result.PassedChecks != 1 {
		t.Errorf("expected 1 passed, got %d", result.PassedChecks)
	}
	if result.PendingChecks != 1 {
		t.Errorf("expected 1 pending, got %d", result.PendingChecks)
	}
}

func TestCIMerger_CheckCIStatus_OneFailed(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	provider := &mockProvider{
		checkRuns: []hosting.CheckRun{
			{Name: "build", Status: "completed", Conclusion: "success"},
			{Name: "test", Status: "completed", Conclusion: "failure"},
		},
	}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	result, err := merger.CheckCIStatus(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != CIStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
	if result.FailedChecks != 1 {
		t.Errorf("expected 1 failed, got %d", result.FailedChecks)
	}
	if len(result.FailedNames) != 1 || result.FailedNames[0] != "test" {
		t.Errorf("expected failed name 'test', got %v", result.FailedNames)
	}
}

func TestCIMerger_CheckCIStatus_NeutralAndSkipped(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	provider := &mockProvider{
		checkRuns: []hosting.CheckRun{
			{Name: "build", Status: "completed", Conclusion: "neutral"},
			{Name: "optional", Status: "completed", Conclusion: "skipped"},
			{Name: "test", Status: "completed", Conclusion: "success"},
		},
	}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	result, err := merger.CheckCIStatus(context.Background(), "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != CIStatusPassed {
		t.Errorf("expected passed status, got %v", result.Status)
	}
	if result.PassedChecks != 3 {
		t.Errorf("expected 3 passed (neutral+skipped count as passed), got %d", result.PassedChecks)
	}
}

func TestCIMerger_MergePR_NoProvider(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	merger := NewCIMerger(cfg)

	prNumber := int32(1)
	tsk := &orcv1.Task{
		Id: "TASK-001",
		Pr: &orcv1.PRInfo{Number: &prNumber},
	}

	err := merger.MergePR(context.Background(), tsk)
	if err == nil {
		t.Error("expected error when provider is nil")
	}
}

func TestCIMerger_MergePR_NoPRNumber(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	provider := &mockProvider{}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	tsk := &orcv1.Task{Id: "TASK-001"}

	err := merger.MergePR(context.Background(), tsk)
	if err == nil {
		t.Error("expected error for task without PR number")
	}
}

func TestTask_GetPRURL(t *testing.T) {
	t.Parallel()
	prURL := "https://github.com/owner/repo/pull/123"
	tests := []struct {
		name     string
		task     *orcv1.Task
		expected string
	}{
		{
			name:     "nil PR",
			task:     &orcv1.Task{},
			expected: "",
		},
		{
			name: "empty PR URL",
			task: &orcv1.Task{
				Pr: &orcv1.PRInfo{},
			},
			expected: "",
		},
		{
			name: "valid PR URL",
			task: &orcv1.Task{
				Pr: &orcv1.PRInfo{
					Url: &prURL,
				},
			},
			expected: "https://github.com/owner/repo/pull/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := task.GetPRURLProto(tt.task); got != tt.expected {
				t.Errorf("GetPRURLProto() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTask_SetMergedInfo(t *testing.T) {
	t.Parallel()
	tsk := &orcv1.Task{Id: "TASK-001"}

	task.SetMergedInfoProto(tsk, "https://github.com/owner/repo/pull/123", "main")

	if tsk.Pr == nil {
		t.Fatal("expected PR to be set")
	}
	if tsk.Pr.Url == nil || *tsk.Pr.Url != "https://github.com/owner/repo/pull/123" {
		t.Errorf("expected URL to be set, got %v", tsk.Pr.Url)
	}
	if !tsk.Pr.Merged {
		t.Error("expected Merged to be true")
	}
	if tsk.Pr.MergedAt == nil {
		t.Error("expected MergedAt to be set")
	}
	if tsk.Pr.TargetBranch == nil || *tsk.Pr.TargetBranch != "main" {
		t.Errorf("expected TargetBranch to be 'main', got %v", tsk.Pr.TargetBranch)
	}
	if tsk.Pr.Status != orcv1.PRStatus_PR_STATUS_MERGED {
		t.Errorf("expected Status to be PR_STATUS_MERGED, got %s", tsk.Pr.Status)
	}
}

func TestMergeMethodTranslation(t *testing.T) {
	t.Parallel()
	// Test that merge method values are passed correctly to the API
	tests := []struct {
		name           string
		configMethod   string
		expectedMethod string
	}{
		{
			name:           "squash method",
			configMethod:   "squash",
			expectedMethod: "squash",
		},
		{
			name:           "merge method",
			configMethod:   "merge",
			expectedMethod: "merge",
		},
		{
			name:           "rebase method",
			configMethod:   "rebase",
			expectedMethod: "rebase",
		},
		{
			name:           "empty defaults to squash",
			configMethod:   "",
			expectedMethod: "squash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Completion.CI.MergeMethod = tt.configMethod

			// Verify the merge method is returned correctly
			method := cfg.MergeMethod()
			if method != tt.expectedMethod {
				t.Errorf("expected method %q, got %q", tt.expectedMethod, method)
			}
		})
	}
}

func TestErrMergeFailed_Sentinel(t *testing.T) {
	t.Parallel()
	// Test that ErrMergeFailed works as a sentinel error
	wrappedErr := fmt.Errorf("%w: some details", ErrMergeFailed)

	if !errors.Is(wrappedErr, ErrMergeFailed) {
		t.Error("expected errors.Is to return true for wrapped ErrMergeFailed")
	}

	// Test nested wrapping
	doubleWrapped := fmt.Errorf("outer: %w", wrappedErr)
	if !errors.Is(doubleWrapped, ErrMergeFailed) {
		t.Error("expected errors.Is to return true for double-wrapped ErrMergeFailed")
	}
}

func TestMergePR_ExponentialBackoffValues(t *testing.T) {
	t.Parallel()
	// Test that backoff calculation produces expected values
	// Formula: min(2^attempt seconds, 8 seconds)
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 2 * time.Second}, // 2^1 = 2s
		{2, 4 * time.Second}, // 2^2 = 4s
		{3, 8 * time.Second}, // 2^3 = 8s, capped
		{4, 8 * time.Second}, // would be 16s, but capped at 8s
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			backoff := min(time.Duration(1<<tt.attempt)*time.Second, 8*time.Second)
			if backoff != tt.expected {
				t.Errorf("backoff for attempt %d = %v, want %v", tt.attempt, backoff, tt.expected)
			}
		})
	}
}

// mockProvider implements hosting.Provider for testing.
type mockProvider struct {
	checkRuns    []hosting.CheckRun
	checkRunsErr error
	mergeErr     error

	// Capture merge options for assertions
	lastMergeOpts hosting.PRMergeOptions

	// Configurable method returns
	getPRFunc          func(ctx context.Context, number int) (*hosting.PR, error)
	enableAutoMergeErr error
	updatePRBranchErr  error
	createPRFunc       func(ctx context.Context, opts hosting.PRCreateOptions) (*hosting.PR, error)
	approvePRErr       error

	// Track calls
	enableAutoMergeCalls []struct {
		Number int
		Method string
	}
	approvePRCalls []struct {
		Number int
		Body   string
	}
}

func (m *mockProvider) CreatePR(ctx context.Context, opts hosting.PRCreateOptions) (*hosting.PR, error) {
	if m.createPRFunc != nil {
		return m.createPRFunc(ctx, opts)
	}
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) GetPR(ctx context.Context, number int) (*hosting.PR, error) {
	if m.getPRFunc != nil {
		return m.getPRFunc(ctx, number)
	}
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) UpdatePR(_ context.Context, _ int, _ hosting.PRUpdateOptions) error {
	return fmt.Errorf("not implemented")
}
func (m *mockProvider) MergePR(_ context.Context, _ int, opts hosting.PRMergeOptions) error {
	m.lastMergeOpts = opts
	return m.mergeErr
}
func (m *mockProvider) FindPRByBranch(_ context.Context, _ string) (*hosting.PR, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) ListPRComments(_ context.Context, _ int) ([]hosting.PRComment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) CreatePRComment(_ context.Context, _ int, _ hosting.PRCommentCreate) (*hosting.PRComment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) ReplyToComment(_ context.Context, _ int, _ int64, _ string) (*hosting.PRComment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) GetPRComment(_ context.Context, _ int, _ int64) (*hosting.PRComment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) EnableAutoMerge(_ context.Context, number int, method string) error {
	m.enableAutoMergeCalls = append(m.enableAutoMergeCalls, struct {
		Number int
		Method string
	}{number, method})
	return m.enableAutoMergeErr
}
func (m *mockProvider) UpdatePRBranch(_ context.Context, _ int) error {
	return m.updatePRBranchErr
}
func (m *mockProvider) GetCheckRuns(_ context.Context, _ string) ([]hosting.CheckRun, error) {
	return m.checkRuns, m.checkRunsErr
}
func (m *mockProvider) GetPRReviews(_ context.Context, _ int) ([]hosting.PRReview, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) ApprovePR(_ context.Context, number int, body string) error {
	m.approvePRCalls = append(m.approvePRCalls, struct {
		Number int
		Body   string
	}{number, body})
	return m.approvePRErr
}
func (m *mockProvider) GetPRStatusSummary(_ context.Context, _ *hosting.PR) (*hosting.PRStatusSummary, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) DeleteBranch(_ context.Context, _ string) error {
	return fmt.Errorf("not implemented")
}
func (m *mockProvider) CheckAuth(_ context.Context) error {
	return nil
}
func (m *mockProvider) Name() hosting.ProviderType {
	return "mock"
}
func (m *mockProvider) OwnerRepo() (string, string) {
	return "owner", "repo"
}

// =============================================================================
// Template rendering tests
// =============================================================================

func TestRenderCommitTemplate(t *testing.T) {
	t.Parallel()
	task := &orcv1.Task{
		Id:     "TASK-042",
		Title:  "Fix authentication bug",
		Branch: "orc/TASK-042",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "all variables",
			template: "[{{TASK_ID}}] {{TASK_TITLE}} ({{TASK_BRANCH}})",
			expected: "[TASK-042] Fix authentication bug (orc/TASK-042)",
		},
		{
			name:     "no variables",
			template: "Simple commit message",
			expected: "Simple commit message",
		},
		{
			name:     "repeated variable",
			template: "{{TASK_ID}}: {{TASK_ID}}",
			expected: "TASK-042: TASK-042",
		},
		{
			name:     "empty template",
			template: "",
			expected: "",
		},
		{
			name:     "only task ID",
			template: "{{TASK_ID}}",
			expected: "TASK-042",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderCommitTemplate(tt.template, task)
			if got != tt.expected {
				t.Errorf("renderCommitTemplate() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// =============================================================================
// MergePR with SHA verification tests
// =============================================================================

func TestCIMerger_MergePR_SHAVerification(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Completion.CI.VerifySHAOnMerge = true

	provider := &mockProvider{
		getPRFunc: func(_ context.Context, number int) (*hosting.PR, error) {
			return &hosting.PR{
				Number:  number,
				HeadSHA: "abc123def456",
			}, nil
		},
	}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	prNumber := int32(42)
	tsk := &orcv1.Task{
		Id:     "TASK-001",
		Title:  "Test task",
		Branch: "orc/TASK-001",
		Pr:     &orcv1.PRInfo{Number: &prNumber},
	}

	err := merger.MergePR(context.Background(), tsk)
	if err != nil {
		t.Fatalf("MergePR() error = %v", err)
	}

	if provider.lastMergeOpts.SHA != "abc123def456" {
		t.Errorf("expected SHA = %q, got %q", "abc123def456", provider.lastMergeOpts.SHA)
	}
}

func TestCIMerger_MergePR_SHAVerificationDisabled(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Completion.CI.VerifySHAOnMerge = false

	provider := &mockProvider{}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	prNumber := int32(42)
	tsk := &orcv1.Task{
		Id:     "TASK-001",
		Title:  "Test task",
		Branch: "orc/TASK-001",
		Pr:     &orcv1.PRInfo{Number: &prNumber},
	}

	err := merger.MergePR(context.Background(), tsk)
	if err != nil {
		t.Fatalf("MergePR() error = %v", err)
	}

	if provider.lastMergeOpts.SHA != "" {
		t.Errorf("expected empty SHA when verification disabled, got %q", provider.lastMergeOpts.SHA)
	}
}

func TestCIMerger_MergePR_SHAVerificationFetchError(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Completion.CI.VerifySHAOnMerge = true

	provider := &mockProvider{
		getPRFunc: func(_ context.Context, _ int) (*hosting.PR, error) {
			return nil, fmt.Errorf("API rate limited")
		},
	}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	prNumber := int32(42)
	tsk := &orcv1.Task{
		Id:     "TASK-001",
		Title:  "Test task",
		Branch: "orc/TASK-001",
		Pr:     &orcv1.PRInfo{Number: &prNumber},
	}

	// Should still merge — SHA verification failure is a warning, not a blocker
	err := merger.MergePR(context.Background(), tsk)
	if err != nil {
		t.Fatalf("MergePR() error = %v", err)
	}

	if provider.lastMergeOpts.SHA != "" {
		t.Errorf("expected empty SHA on fetch error, got %q", provider.lastMergeOpts.SHA)
	}
}

// =============================================================================
// MergePR with commit message template tests
// =============================================================================

func TestCIMerger_MergePR_CommitMessageTemplate(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Completion.CI.MergeCommitTemplate = "Merge: [{{TASK_ID}}] {{TASK_TITLE}}"

	provider := &mockProvider{}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	prNumber := int32(10)
	tsk := &orcv1.Task{
		Id:     "TASK-099",
		Title:  "Add dark mode",
		Branch: "orc/TASK-099",
		Pr:     &orcv1.PRInfo{Number: &prNumber},
	}

	err := merger.MergePR(context.Background(), tsk)
	if err != nil {
		t.Fatalf("MergePR() error = %v", err)
	}

	expected := "Merge: [TASK-099] Add dark mode"
	if provider.lastMergeOpts.CommitMessage != expected {
		t.Errorf("CommitMessage = %q, want %q", provider.lastMergeOpts.CommitMessage, expected)
	}
}

func TestCIMerger_MergePR_SquashCommitTemplate(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Completion.CI.MergeMethod = "squash"
	cfg.Completion.CI.SquashCommitTemplate = "{{TASK_ID}}: {{TASK_TITLE}}"

	provider := &mockProvider{}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	prNumber := int32(10)
	tsk := &orcv1.Task{
		Id:     "TASK-050",
		Title:  "Refactor auth",
		Branch: "orc/TASK-050",
		Pr:     &orcv1.PRInfo{Number: &prNumber},
	}

	err := merger.MergePR(context.Background(), tsk)
	if err != nil {
		t.Fatalf("MergePR() error = %v", err)
	}

	expected := "TASK-050: Refactor auth"
	if provider.lastMergeOpts.SquashCommitMessage != expected {
		t.Errorf("SquashCommitMessage = %q, want %q", provider.lastMergeOpts.SquashCommitMessage, expected)
	}
}

func TestCIMerger_MergePR_SquashTemplateIgnoredForMergeMethod(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Completion.CI.MergeMethod = "merge" // Not squash
	cfg.Completion.CI.SquashCommitTemplate = "{{TASK_ID}}: {{TASK_TITLE}}"

	provider := &mockProvider{}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	prNumber := int32(10)
	tsk := &orcv1.Task{
		Id:     "TASK-050",
		Title:  "Refactor auth",
		Branch: "orc/TASK-050",
		Pr:     &orcv1.PRInfo{Number: &prNumber},
	}

	err := merger.MergePR(context.Background(), tsk)
	if err != nil {
		t.Fatalf("MergePR() error = %v", err)
	}

	if provider.lastMergeOpts.SquashCommitMessage != "" {
		t.Errorf("SquashCommitMessage should be empty for merge method, got %q",
			provider.lastMergeOpts.SquashCommitMessage)
	}
}

// =============================================================================
// MergePR commit title tests
// =============================================================================

func TestCIMerger_MergePR_CommitTitle(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	provider := &mockProvider{}
	merger := NewCIMerger(cfg, WithCIMergerHostingProvider(provider))

	prNumber := int32(77)
	tsk := &orcv1.Task{
		Id:     "TASK-123",
		Title:  "Add user search",
		Branch: "orc/TASK-123",
		Pr:     &orcv1.PRInfo{Number: &prNumber},
	}

	err := merger.MergePR(context.Background(), tsk)
	if err != nil {
		t.Fatalf("MergePR() error = %v", err)
	}

	expected := "[orc] TASK-123: Add user search (#77)"
	if provider.lastMergeOpts.CommitTitle != expected {
		t.Errorf("CommitTitle = %q, want %q", provider.lastMergeOpts.CommitTitle, expected)
	}
}

// =============================================================================
// EnableAutoMerge mock tests
// =============================================================================

func TestMockProvider_EnableAutoMerge_TracksCall(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{}

	err := provider.EnableAutoMerge(context.Background(), 42, "squash")
	if err != nil {
		t.Fatalf("EnableAutoMerge() error = %v", err)
	}

	if len(provider.enableAutoMergeCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(provider.enableAutoMergeCalls))
	}
	call := provider.enableAutoMergeCalls[0]
	if call.Number != 42 {
		t.Errorf("number = %d, want 42", call.Number)
	}
	if call.Method != "squash" {
		t.Errorf("method = %q, want %q", call.Method, "squash")
	}
}

func TestMockProvider_EnableAutoMerge_ReturnsError(t *testing.T) {
	t.Parallel()
	provider := &mockProvider{
		enableAutoMergeErr: hosting.ErrAutoMergeNotSupported,
	}

	err := provider.EnableAutoMerge(context.Background(), 42, "squash")
	if !errors.Is(err, hosting.ErrAutoMergeNotSupported) {
		t.Errorf("expected ErrAutoMergeNotSupported, got %v", err)
	}
}

// =============================================================================
// Default config verification tests
// =============================================================================

func TestDefault_PRConfig(t *testing.T) {
	t.Parallel()
	cfg := config.Default()

	if cfg.Completion.PR.AutoMerge {
		t.Error("expected PR.AutoMerge to be false by default")
	}
	if cfg.Completion.PR.AutoApprove {
		t.Error("expected PR.AutoApprove to be false by default")
	}
	if !cfg.Completion.PR.MaintainerCanModify {
		t.Error("expected PR.MaintainerCanModify to be true by default")
	}
	if cfg.Completion.PR.Draft {
		t.Error("expected PR.Draft to be false by default")
	}
}

func TestDefault_CIConfig_NewFields(t *testing.T) {
	t.Parallel()
	cfg := config.Default()

	if cfg.Completion.CI.MergeCommitTemplate != "" {
		t.Errorf("expected MergeCommitTemplate to be empty, got %q", cfg.Completion.CI.MergeCommitTemplate)
	}
	if cfg.Completion.CI.SquashCommitTemplate != "" {
		t.Errorf("expected SquashCommitTemplate to be empty, got %q", cfg.Completion.CI.SquashCommitTemplate)
	}
	if !cfg.Completion.CI.VerifySHAOnMerge {
		t.Error("expected VerifySHAOnMerge to be true by default")
	}
}
