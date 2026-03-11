package db

import (
	"context"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// SC-1: CreateThread returns a thread with generated ID and all fields
// ============================================================================

func TestThread_Create(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	thread := &Thread{
		Title:        "Login discussion",
		TaskID:       "TASK-001",
		InitiativeID: "INIT-001",
		FileContext:  `["main.go", "auth.go"]`,
	}

	err := pdb.CreateThread(thread)
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	// ID must be generated and non-empty
	if thread.ID == "" {
		t.Error("expected non-empty thread ID")
	}
	// ID should follow THR-XXX pattern
	if !strings.HasPrefix(thread.ID, "THR-") {
		t.Errorf("expected thread ID to start with THR-, got %s", thread.ID)
	}

	// Status must default to "active"
	if thread.Status != "active" {
		t.Errorf("expected status 'active', got %q", thread.Status)
	}

	// Timestamps must be set
	if thread.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if thread.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}

	// Fields must match what was provided
	if thread.Title != "Login discussion" {
		t.Errorf("expected title 'Login discussion', got %q", thread.Title)
	}
	if thread.TaskID != "TASK-001" {
		t.Errorf("expected task_id 'TASK-001', got %q", thread.TaskID)
	}
	if thread.InitiativeID != "INIT-001" {
		t.Errorf("expected initiative_id 'INIT-001', got %q", thread.InitiativeID)
	}
	if thread.FileContext != `["main.go", "auth.go"]` {
		t.Errorf("expected file_context preserved, got %q", thread.FileContext)
	}
}

func TestThread_Create_EmptyTitle(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	thread := &Thread{
		Title: "",
	}

	err := pdb.CreateThread(thread)
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
}

func TestThread_Create_MinimalFields(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	// Only title required, no task_id or initiative_id
	thread := &Thread{
		Title: "General discussion",
	}

	err := pdb.CreateThread(thread)
	if err != nil {
		t.Fatalf("CreateThread failed: %v", err)
	}

	if thread.ID == "" {
		t.Error("expected non-empty thread ID")
	}
	if thread.TaskID != "" {
		t.Errorf("expected empty task_id, got %q", thread.TaskID)
	}
	if thread.InitiativeID != "" {
		t.Errorf("expected empty initiative_id, got %q", thread.InitiativeID)
	}
}

// ============================================================================
// SC-2: GetThread returns the thread with all messages ordered by creation time
// ============================================================================

func TestThread_GetWithMessages(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	// Create thread
	thread := &Thread{Title: "Test thread"}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// Add 3 messages with distinct creation times
	msgs := []ThreadMessage{
		{ThreadID: thread.ID, Role: "user", Content: "First message"},
		{ThreadID: thread.ID, Role: "assistant", Content: "Second message"},
		{ThreadID: thread.ID, Role: "user", Content: "Third message"},
	}
	for i := range msgs {
		if err := pdb.AddThreadMessage(&msgs[i]); err != nil {
			t.Fatalf("AddThreadMessage[%d]: %v", i, err)
		}
		// Small delay to ensure distinct timestamps
		time.Sleep(time.Millisecond)
	}

	// Get thread with messages
	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil thread")
	}

	// Must have 3 messages
	if len(got.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got.Messages))
	}

	// Messages must be in chronological order
	if got.Messages[0].Content != "First message" {
		t.Errorf("message[0] expected 'First message', got %q", got.Messages[0].Content)
	}
	if got.Messages[1].Content != "Second message" {
		t.Errorf("message[1] expected 'Second message', got %q", got.Messages[1].Content)
	}
	if got.Messages[2].Content != "Third message" {
		t.Errorf("message[2] expected 'Third message', got %q", got.Messages[2].Content)
	}

	// Verify message fields
	if got.Messages[0].Role != "user" {
		t.Errorf("message[0] role expected 'user', got %q", got.Messages[0].Role)
	}
	if got.Messages[1].Role != "assistant" {
		t.Errorf("message[1] role expected 'assistant', got %q", got.Messages[1].Role)
	}
}

func TestThread_GetWithMessages_Empty(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	// Create thread with no messages
	thread := &Thread{Title: "Empty thread"}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil thread")
	}

	// Messages must be empty slice, not nil
	if got.Messages == nil {
		t.Error("expected empty slice for messages, got nil")
	}
	if len(got.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(got.Messages))
	}
}

func TestThread_Get_NotFound(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	got, err := pdb.GetThread("THR-999")
	if err != nil {
		t.Fatalf("expected nil error for not-found, got: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil thread for not-found, got: %+v", got)
	}
}

// ============================================================================
// SC-3: ListThreads returns threads filterable by status and task_id
// ============================================================================

func TestThread_ListFilter(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	// Create 5 threads: 3 active, 2 archived
	threads := []struct {
		title  string
		taskID string
		status string
	}{
		{"Thread 1", "TASK-001", "active"},
		{"Thread 2", "TASK-001", "active"},
		{"Thread 3", "TASK-002", "active"},
		{"Thread 4", "TASK-001", "archived"},
		{"Thread 5", "TASK-002", "archived"},
	}

	for _, tt := range threads {
		thread := &Thread{Title: tt.title, TaskID: tt.taskID}
		if err := pdb.CreateThread(thread); err != nil {
			t.Fatalf("CreateThread %s: %v", tt.title, err)
		}
		// Archive threads that should be archived
		if tt.status == "archived" {
			if err := pdb.ArchiveThread(thread.ID); err != nil {
				t.Fatalf("ArchiveThread %s: %v", thread.ID, err)
			}
		}
	}

	// Filter by status="active" → 3 results
	activeThreads, err := pdb.ListThreads(ThreadListOpts{Status: "active"})
	if err != nil {
		t.Fatalf("ListThreads(active): %v", err)
	}
	if len(activeThreads) != 3 {
		t.Errorf("expected 3 active threads, got %d", len(activeThreads))
	}

	// Filter by status="archived" → 2 results
	archivedThreads, err := pdb.ListThreads(ThreadListOpts{Status: "archived"})
	if err != nil {
		t.Fatalf("ListThreads(archived): %v", err)
	}
	if len(archivedThreads) != 2 {
		t.Errorf("expected 2 archived threads, got %d", len(archivedThreads))
	}

	// Filter by task_id="TASK-001" → 3 results (2 active + 1 archived)
	task1Threads, err := pdb.ListThreads(ThreadListOpts{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("ListThreads(task_id): %v", err)
	}
	if len(task1Threads) != 3 {
		t.Errorf("expected 3 threads for TASK-001, got %d", len(task1Threads))
	}

	// Filter by both status="active" and task_id="TASK-001" → 2 results
	combined, err := pdb.ListThreads(ThreadListOpts{Status: "active", TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("ListThreads(active+task_id): %v", err)
	}
	if len(combined) != 2 {
		t.Errorf("expected 2 active threads for TASK-001, got %d", len(combined))
	}
}

func TestThread_ListFilter_NoMatch(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	// No threads exist
	threads, err := pdb.ListThreads(ThreadListOpts{Status: "active"})
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	// Must return empty slice, not nil
	if threads == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(threads) != 0 {
		t.Errorf("expected 0 threads, got %d", len(threads))
	}
}

// ============================================================================
// SC-4: ArchiveThread changes status to "archived"
// ============================================================================

func TestThread_Archive(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	thread := &Thread{Title: "To be archived"}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	if err := pdb.ArchiveThread(thread.ID); err != nil {
		t.Fatalf("ArchiveThread: %v", err)
	}

	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if got.Status != "archived" {
		t.Errorf("expected status 'archived', got %q", got.Status)
	}
}

func TestThread_Archive_Idempotent(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	thread := &Thread{Title: "Archive twice"}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// Archive twice — second should not error
	if err := pdb.ArchiveThread(thread.ID); err != nil {
		t.Fatalf("ArchiveThread (first): %v", err)
	}
	if err := pdb.ArchiveThread(thread.ID); err != nil {
		t.Fatalf("ArchiveThread (second): %v", err)
	}

	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if got.Status != "archived" {
		t.Errorf("expected status 'archived', got %q", got.Status)
	}
}

func TestThread_Archive_NotFound(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	err := pdb.ArchiveThread("THR-999")
	if err == nil {
		t.Fatal("expected error archiving non-existent thread, got nil")
	}
}

// ============================================================================
// SC-5: DeleteThread removes thread and all messages (cascade)
// ============================================================================

func TestThread_DeleteCascade(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	// Create thread with messages
	thread := &Thread{Title: "To be deleted"}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	msg := &ThreadMessage{ThreadID: thread.ID, Role: "user", Content: "Hello"}
	if err := pdb.AddThreadMessage(msg); err != nil {
		t.Fatalf("AddThreadMessage: %v", err)
	}

	// Delete thread
	if err := pdb.DeleteThread(thread.ID); err != nil {
		t.Fatalf("DeleteThread: %v", err)
	}

	// Thread should be gone
	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if got != nil {
		t.Error("expected nil thread after delete")
	}

	// Messages should also be gone (cascade)
	msgs, err := pdb.GetThreadMessages(thread.ID)
	if err != nil {
		t.Fatalf("GetThreadMessages: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 orphaned messages, got %d", len(msgs))
	}
}

func TestThread_Delete_NotFound(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	err := pdb.DeleteThread("THR-999")
	if err == nil {
		t.Fatal("expected error deleting non-existent thread, got nil")
	}
}

// ============================================================================
// Edge cases from spec
// ============================================================================

func TestThread_Create_InvalidTaskId(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	// task_id is informational, not a strict FK — should succeed
	thread := &Thread{
		Title:  "References missing task",
		TaskID: "TASK-NONEXISTENT",
	}

	err := pdb.CreateThread(thread)
	if err != nil {
		t.Fatalf("CreateThread with non-existent task_id should succeed: %v", err)
	}
}

func TestThread_LongMessage(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	thread := &Thread{Title: "Long message thread"}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// Create a very long message (100KB)
	longContent := strings.Repeat("A", 100*1024)
	msg := &ThreadMessage{ThreadID: thread.ID, Role: "user", Content: longContent}
	if err := pdb.AddThreadMessage(msg); err != nil {
		t.Fatalf("AddThreadMessage: %v", err)
	}

	// Retrieve and verify no truncation
	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if got.Messages[0].Content != longContent {
		t.Errorf("message content truncated: got %d chars, want %d",
			len(got.Messages[0].Content), len(longContent))
	}
}

func TestThread_FileContext(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	fileCtx := `["internal/api/server.go", "internal/db/thread.go", "README.md"]`
	thread := &Thread{
		Title:       "File context test",
		FileContext: fileCtx,
	}

	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if got.FileContext != fileCtx {
		t.Errorf("expected file_context %q, got %q", fileCtx, got.FileContext)
	}
}

func TestThread_SessionID_Persistence(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	thread := &Thread{Title: "Session test"}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// Initially no session ID
	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if got.SessionID != "" {
		t.Errorf("expected empty session_id initially, got %q", got.SessionID)
	}

	// Update session ID
	if err := pdb.UpdateThreadSessionID(thread.ID, "sess-abc-123"); err != nil {
		t.Fatalf("UpdateThreadSessionID: %v", err)
	}

	got, err = pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread after session update: %v", err)
	}
	if got.SessionID != "sess-abc-123" {
		t.Errorf("expected session_id 'sess-abc-123', got %q", got.SessionID)
	}
}

func TestThread_NextID_Sequential(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	ctx := context.Background()

	id1, err := pdb.GetNextThreadID(ctx)
	if err != nil {
		t.Fatalf("GetNextThreadID: %v", err)
	}
	id2, err := pdb.GetNextThreadID(ctx)
	if err != nil {
		t.Fatalf("GetNextThreadID: %v", err)
	}

	if id1 != "THR-001" {
		t.Errorf("expected THR-001, got %s", id1)
	}
	if id2 != "THR-002" {
		t.Errorf("expected THR-002, got %s", id2)
	}
}

func TestThread_Get_PopulatesTypedLinksAndDrafts(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	mustCreateThreadFixtures(t, pdb)

	thread := &Thread{
		Title:        "Workspace thread",
		TaskID:       "TASK-001",
		InitiativeID: "INIT-001",
		FileContext:  `["web/src/components/layout/DiscussionPanel.tsx"]`,
		Links: []ThreadLink{
			{LinkType: ThreadLinkTypeDiff, TargetID: "TASK-001:web/src/components/layout/DiscussionPanel.tsx", Title: "DiscussionPanel diff"},
		},
	}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if err := pdb.AddThreadMessage(&ThreadMessage{ThreadID: thread.ID, Role: "user", Content: "first"}); err != nil {
		t.Fatalf("AddThreadMessage: %v", err)
	}
	if err := pdb.CreateThreadRecommendationDraft(&ThreadRecommendationDraft{
		ThreadID:       thread.ID,
		Kind:           RecommendationKindCleanup,
		Title:          "Clean up sidebar duplication",
		Summary:        "The sidebar repeats thread context logic.",
		ProposedAction: "Extract the shared rendering path.",
		Evidence:       "The same shape is rendered in three places.",
	}); err != nil {
		t.Fatalf("CreateThreadRecommendationDraft: %v", err)
	}
	if err := pdb.CreateThreadDecisionDraft(&ThreadDecisionDraft{
		ThreadID:     thread.ID,
		InitiativeID: "INIT-001",
		Decision:     "Reuse the control-plane thread substrate",
		Rationale:    "The old work already solved history and context persistence.",
	}); err != nil {
		t.Fatalf("CreateThreadDecisionDraft: %v", err)
	}

	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if len(got.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got.Messages))
	}
	if len(got.Links) != 4 {
		t.Fatalf("expected 4 typed links, got %d", len(got.Links))
	}
	if got.Links[0].LinkType != ThreadLinkTypeTask {
		t.Fatalf("expected first link to be task, got %s", got.Links[0].LinkType)
	}
	if len(got.RecommendationDrafts) != 1 {
		t.Fatalf("expected 1 recommendation draft, got %d", len(got.RecommendationDrafts))
	}
	if len(got.DecisionDrafts) != 1 {
		t.Fatalf("expected 1 decision draft, got %d", len(got.DecisionDrafts))
	}
}

func TestThread_PromoteRecommendationDraft(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	mustCreateThreadFixtures(t, pdb)

	thread := &Thread{
		Title:  "Promotion thread",
		TaskID: "TASK-001",
	}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	draft := &ThreadRecommendationDraft{
		ThreadID:       thread.ID,
		Kind:           RecommendationKindFollowUp,
		Title:          "Follow up on thread promotion",
		Summary:        "The new promotion path needs coverage.",
		ProposedAction: "Add integration coverage for recommendation promotion.",
		Evidence:       "No test currently exercises the draft path.",
	}
	if err := pdb.CreateThreadRecommendationDraft(draft); err != nil {
		t.Fatalf("CreateThreadRecommendationDraft: %v", err)
	}

	promotedDraft, rec, err := pdb.PromoteThreadRecommendationDraft(context.Background(), draft.ID, "operator")
	if err != nil {
		t.Fatalf("PromoteThreadRecommendationDraft: %v", err)
	}
	if promotedDraft.Status != ThreadDraftStatusPromoted {
		t.Fatalf("expected promoted status, got %s", promotedDraft.Status)
	}
	if rec.Status != RecommendationStatusPending {
		t.Fatalf("expected pending recommendation, got %s", rec.Status)
	}
	if rec.SourceThreadID != thread.ID {
		t.Fatalf("expected source thread %s, got %s", thread.ID, rec.SourceThreadID)
	}
	if rec.SourceRunID != "RUN-001" {
		t.Fatalf("expected derived source run RUN-001, got %s", rec.SourceRunID)
	}
	history, err := pdb.ListRecommendationHistory(rec.ID)
	if err != nil {
		t.Fatalf("ListRecommendationHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}
	if history[0].FromStatus != "" {
		t.Fatalf("expected empty from_status for initial history, got %q", history[0].FromStatus)
	}
	if history[0].ToStatus != RecommendationStatusPending {
		t.Fatalf("expected pending history status, got %s", history[0].ToStatus)
	}

	got, err := pdb.GetThread(thread.ID)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if len(got.Links) != 2 {
		t.Fatalf("expected task link plus promoted recommendation link, got %d", len(got.Links))
	}
	if got.Links[1].LinkType != ThreadLinkTypeRecommendation {
		t.Fatalf("expected recommendation link, got %s", got.Links[1].LinkType)
	}
}

func TestThread_PromoteDecisionDraft(t *testing.T) {
	t.Parallel()
	pdb := NewTestProjectDB(t)

	mustCreateThreadFixtures(t, pdb)

	thread := &Thread{
		Title:        "Decision thread",
		InitiativeID: "INIT-001",
	}
	if err := pdb.CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	draft := &ThreadDecisionDraft{
		ThreadID:  thread.ID,
		Decision:  "Keep recommendations human-gated",
		Rationale: "Automatic backlog mutation is how you get noise with better branding.",
	}
	if err := pdb.CreateThreadDecisionDraft(draft); err != nil {
		t.Fatalf("CreateThreadDecisionDraft: %v", err)
	}

	promotedDraft, decision, err := pdb.PromoteThreadDecisionDraft(context.Background(), draft.ID, "operator")
	if err != nil {
		t.Fatalf("PromoteThreadDecisionDraft: %v", err)
	}
	if promotedDraft.Status != ThreadDraftStatusPromoted {
		t.Fatalf("expected promoted status, got %s", promotedDraft.Status)
	}
	if decision.InitiativeID != "INIT-001" {
		t.Fatalf("expected initiative INIT-001, got %s", decision.InitiativeID)
	}

	decisions, err := pdb.GetInitiativeDecisions("INIT-001")
	if err != nil {
		t.Fatalf("GetInitiativeDecisions: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 stored initiative decision, got %d", len(decisions))
	}
	if decisions[0].Decision != "Keep recommendations human-gated" {
		t.Fatalf("expected decision text to persist, got %q", decisions[0].Decision)
	}
}

func mustCreateThreadFixtures(t *testing.T, pdb *ProjectDB) {
	t.Helper()

	if err := pdb.SaveWorkflow(&Workflow{
		ID:   "wf-thread",
		Name: "Thread Workflow",
	}); err != nil {
		t.Fatalf("SaveWorkflow: %v", err)
	}
	if err := pdb.SaveTask(&Task{
		ID:         "TASK-001",
		Title:      "Thread Source Task",
		WorkflowID: "wf-thread",
		Status:     "running",
	}); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}
	taskID := "TASK-001"
	if err := pdb.SaveWorkflowRun(&WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "wf-thread",
		ContextType: "task",
		TaskID:      &taskID,
		Status:      "running",
	}); err != nil {
		t.Fatalf("SaveWorkflowRun: %v", err)
	}
	if err := pdb.SaveInitiative(&Initiative{
		ID:     "INIT-001",
		Title:  "Operator Control Plane",
		Status: "active",
	}); err != nil {
		t.Fatalf("SaveInitiative: %v", err)
	}
}
