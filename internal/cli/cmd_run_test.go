package cli

import (
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/task"
)

// Tests for parseConflictFilesFromError are string-only and don't need proto types.

func TestParseConflictFilesFromError_BasicFormat(t *testing.T) {
	t.Parallel()
	// Test basic bracket format: [file1 file2 file3]
	errStr := "sync conflict: files have conflicts [internal/foo.go internal/bar.go]"

	files := parseConflictFilesFromError(errStr)

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0] != "internal/foo.go" {
		t.Errorf("files[0] = %q, want 'internal/foo.go'", files[0])
	}
	if files[1] != "internal/bar.go" {
		t.Errorf("files[1] = %q, want 'internal/bar.go'", files[1])
	}
}

func TestParseConflictFilesFromError_SingleFile(t *testing.T) {
	t.Parallel()
	errStr := "sync conflict [README.md]"

	files := parseConflictFilesFromError(errStr)

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0] != "README.md" {
		t.Errorf("files[0] = %q, want 'README.md'", files[0])
	}
}

func TestParseConflictFilesFromError_EmptyBrackets(t *testing.T) {
	t.Parallel()
	errStr := "sync conflict []"

	files := parseConflictFilesFromError(errStr)

	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestParseConflictFilesFromError_NoBrackets(t *testing.T) {
	t.Parallel()
	errStr := "sync conflict without file list"

	files := parseConflictFilesFromError(errStr)

	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestParseConflictFilesFromError_EmptyString(t *testing.T) {
	t.Parallel()
	errStr := ""

	files := parseConflictFilesFromError(errStr)

	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestParseConflictFilesFromError_WithCommas(t *testing.T) {
	t.Parallel()
	// Test that commas are trimmed from file names
	errStr := "conflicts: [file1.go, file2.go, file3.go]"

	files := parseConflictFilesFromError(errStr)

	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}
	// Commas should be stripped
	if files[0] != "file1.go" {
		t.Errorf("files[0] = %q, want 'file1.go'", files[0])
	}
	if files[1] != "file2.go" {
		t.Errorf("files[1] = %q, want 'file2.go'", files[1])
	}
	if files[2] != "file3.go" {
		t.Errorf("files[2] = %q, want 'file3.go'", files[2])
	}
}

func TestParseConflictFilesFromError_MultipleSpaces(t *testing.T) {
	t.Parallel()
	// Test that multiple spaces are handled
	errStr := "conflicts: [file1.go   file2.go]"

	files := parseConflictFilesFromError(errStr)

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
}

func TestBuildBlockedContext_NilConfig(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")

	ctx := buildBlockedContextProto(tk, nil, "/tmp")

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.WorktreePath != "" {
		t.Errorf("expected empty WorktreePath with nil config, got %q", ctx.WorktreePath)
	}
	if ctx.TargetBranch != "" {
		t.Errorf("expected empty TargetBranch with nil config, got %q", ctx.TargetBranch)
	}
}

func TestBuildBlockedContext_WorktreeDisabled(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{
			Enabled: false,
		},
	}

	ctx := buildBlockedContextProto(tk, cfg, "/tmp")

	if ctx.WorktreePath != "" {
		t.Errorf("expected empty WorktreePath when worktree disabled, got %q", ctx.WorktreePath)
	}
}

func TestBuildBlockedContext_WorktreeEnabled(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{
			Enabled: true,
			Dir:     ".orc/worktrees",
		},
	}
	projectRoot := t.TempDir()

	ctx := buildBlockedContextProto(tk, cfg, projectRoot)

	// ResolveWorktreeDir joins relative Dir with projectRoot, producing an absolute path
	expected := filepath.Join(projectRoot, ".orc/worktrees") + "/orc-TASK-001"
	if ctx.WorktreePath != expected {
		t.Errorf("WorktreePath = %q, want %q", ctx.WorktreePath, expected)
	}
}

func TestBuildBlockedContext_WorktreeDefaultDir(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-002", "Test task")
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{
			Enabled: true,
			Dir:     "", // Empty triggers registry lookup, falls back to <projectRoot>/.orc/worktrees
		},
	}
	projectRoot := t.TempDir()

	ctx := buildBlockedContextProto(tk, cfg, projectRoot)

	// With empty Dir and unregistered project, falls back to <projectRoot>/.orc/worktrees
	expected := filepath.Join(projectRoot, ".orc", "worktrees") + "/orc-TASK-002"
	if ctx.WorktreePath != expected {
		t.Errorf("WorktreePath = %q, want %q", ctx.WorktreePath, expected)
	}
}

func TestBuildBlockedContext_WithConflictFiles(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Metadata = map[string]string{
		"blocked_error": "sync conflict: files [internal/foo.go internal/bar.go]",
	}
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{
			Enabled: true,
		},
	}

	ctx := buildBlockedContextProto(tk, cfg, "/tmp")

	if len(ctx.ConflictFiles) != 2 {
		t.Fatalf("expected 2 conflict files, got %d", len(ctx.ConflictFiles))
	}
	if ctx.ConflictFiles[0] != "internal/foo.go" {
		t.Errorf("ConflictFiles[0] = %q, want 'internal/foo.go'", ctx.ConflictFiles[0])
	}
}

func TestBuildBlockedContext_NoBlockedError(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Metadata = map[string]string{
		"some_other_key": "value",
	}
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{
			Enabled: true,
		},
	}

	ctx := buildBlockedContextProto(tk, cfg, "/tmp")

	if len(ctx.ConflictFiles) != 0 {
		t.Errorf("expected 0 conflict files, got %d", len(ctx.ConflictFiles))
	}
}

func TestBuildBlockedContext_NilMetadata(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Metadata = nil
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{
			Enabled: true,
		},
	}

	ctx := buildBlockedContextProto(tk, cfg, "/tmp")

	if len(ctx.ConflictFiles) != 0 {
		t.Errorf("expected 0 conflict files with nil metadata, got %d", len(ctx.ConflictFiles))
	}
}

func TestBuildBlockedContext_RebaseStrategy(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")
	cfg := &config.Config{
		Completion: config.CompletionConfig{
			Finalize: config.FinalizeConfig{
				Sync: config.FinalizeSyncConfig{
					Strategy: config.FinalizeSyncRebase,
				},
			},
		},
	}

	ctx := buildBlockedContextProto(tk, cfg, "/tmp")

	if ctx.SyncStrategy != progress.SyncStrategyRebase {
		t.Errorf("SyncStrategy = %q, want %q", ctx.SyncStrategy, progress.SyncStrategyRebase)
	}
}

func TestBuildBlockedContext_MergeStrategy(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")
	cfg := &config.Config{
		Completion: config.CompletionConfig{
			Finalize: config.FinalizeConfig{
				Sync: config.FinalizeSyncConfig{
					Strategy: config.FinalizeSyncMerge,
				},
			},
		},
	}

	ctx := buildBlockedContextProto(tk, cfg, "/tmp")

	if ctx.SyncStrategy != progress.SyncStrategyMerge {
		t.Errorf("SyncStrategy = %q, want %q", ctx.SyncStrategy, progress.SyncStrategyMerge)
	}
}

func TestBuildBlockedContext_TargetBranch(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")
	cfg := &config.Config{
		Completion: config.CompletionConfig{
			TargetBranch: "develop",
		},
	}

	ctx := buildBlockedContextProto(tk, cfg, "/tmp")

	if ctx.TargetBranch != "develop" {
		t.Errorf("TargetBranch = %q, want 'develop'", ctx.TargetBranch)
	}
}

func TestBuildBlockedContext_DefaultTargetBranch(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-001", "Test task")
	cfg := &config.Config{
		Completion: config.CompletionConfig{
			TargetBranch: "", // Empty should default to "main"
		},
	}

	ctx := buildBlockedContextProto(tk, cfg, "/tmp")

	if ctx.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want 'main'", ctx.TargetBranch)
	}
}

func TestBuildBlockedContext_FullContext(t *testing.T) {
	t.Parallel()
	tk := task.NewProtoTask("TASK-123", "Full context test")
	tk.Metadata = map[string]string{
		"blocked_error": "sync conflict: [pkg/api.go pkg/handler.go pkg/service.go]",
	}
	cfg := &config.Config{
		Worktree: config.WorktreeConfig{
			Enabled: true,
			Dir:     "custom/worktrees",
		},
		Completion: config.CompletionConfig{
			TargetBranch: "release",
			Finalize: config.FinalizeConfig{
				Sync: config.FinalizeSyncConfig{
					Strategy: config.FinalizeSyncMerge,
				},
			},
		},
	}
	projectRoot := t.TempDir()

	ctx := buildBlockedContextProto(tk, cfg, projectRoot)

	// Verify all fields are populated correctly
	expectedWT := filepath.Join(projectRoot, "custom/worktrees") + "/orc-TASK-123"
	if ctx.WorktreePath != expectedWT {
		t.Errorf("WorktreePath = %q, want %q", ctx.WorktreePath, expectedWT)
	}
	if len(ctx.ConflictFiles) != 3 {
		t.Errorf("expected 3 conflict files, got %d", len(ctx.ConflictFiles))
	}
	if ctx.SyncStrategy != progress.SyncStrategyMerge {
		t.Errorf("SyncStrategy = %q, want %q", ctx.SyncStrategy, progress.SyncStrategyMerge)
	}
	if ctx.TargetBranch != "release" {
		t.Errorf("TargetBranch = %q, want 'release'", ctx.TargetBranch)
	}
}

func TestContainsPhase(t *testing.T) {
	t.Parallel()
	phases := []string{"spec", "implement", "test", "docs"}

	tests := []struct {
		phaseID string
		want    bool
	}{
		{"spec", true},
		{"implement", true},
		{"test", true},
		{"docs", true},
		{"review", false},
		{"finalize", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.phaseID, func(t *testing.T) {
			got := containsPhase(phases, tt.phaseID)
			if got != tt.want {
				t.Errorf("containsPhase(%q) = %v, want %v", tt.phaseID, got, tt.want)
			}
		})
	}
}

func TestContainsPhase_EmptyList(t *testing.T) {
	t.Parallel()
	phases := []string{}

	if containsPhase(phases, "implement") {
		t.Error("expected false for empty phases list")
	}
}
