package executor

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

func TestEnrichContextForPhase_LoadsThreadPromptContext(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	if err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     "INIT-001",
		Title:  "Operator Control Plane",
		Status: "active",
	}); err != nil {
		t.Fatalf("SaveInitiative: %v", err)
	}

	taskItem := task.NewProtoTask("TASK-THREAD-001", "Thread prompt context")
	task.SetInitiativeProto(taskItem, "INIT-001")
	if err := backend.SaveTask(taskItem); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	thread := &db.Thread{
		Title:        "Workspace thread",
		TaskID:       taskItem.Id,
		InitiativeID: "INIT-001",
	}
	if err := backend.DB().CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if err := backend.DB().CreateThreadLink(&db.ThreadLink{
		ThreadID: thread.ID,
		LinkType: db.ThreadLinkTypeDiff,
		TargetID: "TASK-THREAD-001:web/src/components/layout/DiscussionPanel.tsx",
		Title:    "DiscussionPanel diff",
	}); err != nil {
		t.Fatalf("CreateThreadLink: %v", err)
	}
	if err := backend.DB().CreateThreadRecommendationDraft(&db.ThreadRecommendationDraft{
		ThreadID:       thread.ID,
		Kind:           db.RecommendationKindFollowUp,
		Title:          "Add resync coverage",
		Summary:        "External thread updates need coverage.",
		ProposedAction: "Add an event-driven test.",
		Evidence:       "The panel must refresh when another client mutates the thread.",
	}); err != nil {
		t.Fatalf("CreateThreadRecommendationDraft: %v", err)
	}
	if err := backend.DB().CreateThreadDecisionDraft(&db.ThreadDecisionDraft{
		ThreadID:     thread.ID,
		InitiativeID: "INIT-001",
		Decision:     "Keep discussion state separate",
		Rationale:    "Discussion context should enrich prompts without mutating execution state.",
	}); err != nil {
		t.Fatalf("CreateThreadDecisionDraft: %v", err)
	}
	if err := backend.DB().AddThreadMessage(&db.ThreadMessage{
		ThreadID: thread.ID,
		Role:     "user",
		Content:  "Remember the prior discussion.",
	}); err != nil {
		t.Fatalf("AddThreadMessage(user): %v", err)
	}
	if err := backend.DB().AddThreadMessage(&db.ThreadMessage{
		ThreadID: thread.ID,
		Role:     "assistant",
		Content:  "The workspace needs to reload after external mutations.",
	}); err != nil {
		t.Fatalf("AddThreadMessage(assistant): %v", err)
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
	)
	rctx := &variable.ResolutionContext{}

	err := we.enrichContextForPhase(rctx, "implement", taskItem, threadVariableUsage{
		ThreadID:                   true,
		ThreadTitle:                true,
		ThreadContext:              true,
		ThreadHistory:              true,
		ThreadLinkedContext:        true,
		ThreadRecommendationDrafts: true,
		ThreadDecisionDrafts:       true,
	})
	if err != nil {
		t.Fatalf("enrichContextForPhase() error = %v", err)
	}

	if rctx.ThreadID != thread.ID {
		t.Fatalf("ThreadID = %q, want %q", rctx.ThreadID, thread.ID)
	}
	if rctx.ThreadTitle != "Workspace thread" {
		t.Fatalf("ThreadTitle = %q", rctx.ThreadTitle)
	}
	if !strings.Contains(rctx.ThreadLinkedContext, "DiscussionPanel diff") {
		t.Fatalf("ThreadLinkedContext = %q", rctx.ThreadLinkedContext)
	}
	if !strings.Contains(rctx.ThreadRecommendationDrafts, "Add resync coverage") {
		t.Fatalf("ThreadRecommendationDrafts = %q", rctx.ThreadRecommendationDrafts)
	}
	if !strings.Contains(rctx.ThreadDecisionDrafts, "Keep discussion state separate") {
		t.Fatalf("ThreadDecisionDrafts = %q", rctx.ThreadDecisionDrafts)
	}
	if !strings.Contains(rctx.ThreadHistory, "Remember the prior discussion.") {
		t.Fatalf("ThreadHistory = %q", rctx.ThreadHistory)
	}
	if !strings.Contains(rctx.ThreadContext, "Recent thread history") {
		t.Fatalf("ThreadContext = %q", rctx.ThreadContext)
	}
}

func TestEnrichContextForPhase_SkipsThreadLoadingWhenUnused(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	if err := backend.DB().Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
	)
	rctx := &variable.ResolutionContext{
		ThreadContext: "stale",
	}

	if err := we.enrichContextForPhase(rctx, "implement", task.NewProtoTask("TASK-THREAD-SKIP", "skip"), threadVariableUsage{}); err != nil {
		t.Fatalf("enrichContextForPhase() error = %v, want nil when thread vars unused", err)
	}
	if rctx.ThreadContext != "" {
		t.Fatalf("ThreadContext = %q, want cleared empty value", rctx.ThreadContext)
	}
}

func TestEnrichContextForPhase_FailsWhenThreadContextLoadFails(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	taskItem := task.NewProtoTask("TASK-THREAD-FAIL", "thread failure")
	if err := backend.SaveTask(taskItem); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}
	if err := backend.DB().Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
	)

	err := we.enrichContextForPhase(&variable.ResolutionContext{}, "implement", taskItem, threadVariableUsage{
		ThreadContext: true,
	})
	if err == nil {
		t.Fatal("enrichContextForPhase() error = nil, want load failure")
	}
}
