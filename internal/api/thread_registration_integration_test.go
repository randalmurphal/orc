// Integration tests for TASK-013: Thread backend wiring verification.
//
// These tests verify that new thread code is properly wired into EXISTING
// production code paths. Unlike unit tests (thread_server_test.go) which
// call server methods directly, these tests exercise:
//
//   1. registerConnectHandlers() includes ThreadService registration
//   2. Proto generation produces orcv1connect handler/client functions
//   3. Full HTTP round-trip works (Connect protocol serialization)
//
// WIRING POINTS TESTED:
//   - server_connect.go:registerConnectHandlers() → NewThreadServer + mux.Handle
//   - gen/proto/orc/v1/orcv1connect/thread.connect.go → handler/client functions
//   - gen/proto/orc/v1/thread.pb.go → proto message types over HTTP
//
// These tests will FAIL TO COMPILE until:
//   1. thread.proto is created and proto is regenerated
//   2. NewThreadServer is implemented in thread_server.go
//   3. ThreadService is registered in registerConnectHandlers()
package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
)

// ============================================================================
// Integration: ThreadService registered in production HTTP mux
// ============================================================================

// TestRegisterConnectHandlers_IncludesThreadService verifies that
// registerConnectHandlers() registers the ThreadService on the HTTP mux.
// This is the critical wiring test: without this registration, the
// ThreadService is dead code — unreachable from any HTTP client.
//
// WIRING POINT: server_connect.go:registerConnectHandlers()
// Must contain lines equivalent to:
//
//	threadSvc := NewThreadServer(s.backend, s.publisher, s.logger)
//	threadPath, threadHandler := orcv1connect.NewThreadServiceHandler(threadSvc, interceptors)
//	s.mux.Handle(threadPath, corsHandler(threadHandler))
//
// If those lines are missing, this test fails with a 404 or "unimplemented" error.
func TestRegisterConnectHandlers_IncludesThreadService(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	// Construct minimal Server with fields required by registerConnectHandlers.
	// This calls the PRODUCTION registration function, which creates ALL services.
	s := &Server{
		mux:              http.NewServeMux(),
		backend:          backend,
		publisher:        publisher,
		logger:           slog.Default(),
		orcConfig:        config.Default(),
		workDir:          t.TempDir(),
		projectDB:        backend.DB(),
		runningTasks:     make(map[string]context.CancelFunc),
		diffCache:        diff.NewCache(10),
		pendingDecisions: gate.NewPendingDecisionStore(),
		projectCache:     NewProjectCache(1),
	}

	s.registerConnectHandlers()

	// Start HTTP test server with the production mux
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	// Use Connect client to call ThreadService through the production mux.
	// If registerConnectHandlers doesn't register ThreadService, this call
	// returns 404 or "unimplemented" error.
	client := orcv1connect.NewThreadServiceClient(http.DefaultClient, ts.URL)

	resp, err := client.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title: "Production mux registration test",
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread via production mux failed: %v\n"+
			"ThreadService is NOT registered in registerConnectHandlers()", err)
	}
	if resp.Msg.Thread == nil {
		t.Fatal("expected non-nil thread in response")
	}
	if resp.Msg.Thread.Id == "" {
		t.Error("expected non-empty thread ID from production mux")
	}
	if resp.Msg.Thread.Status != "active" {
		t.Errorf("expected status 'active', got %q", resp.Msg.Thread.Status)
	}
}

// ============================================================================
// Integration: Full HTTP round-trip for CRUD operations
// ============================================================================

// TestThreadService_CRUD_ViaHTTP verifies that all ThreadService CRUD RPCs
// work through the HTTP transport layer (Connect protocol). This catches
// proto generation issues and Connect serialization bugs that unit tests
// miss because they call server methods directly.
//
// WIRING POINTS:
//   - orcv1connect.NewThreadServiceHandler (proto generated)
//   - orcv1connect.NewThreadServiceClient (proto generated)
//   - orcv1.CreateThreadRequest, GetThreadRequest, etc. (proto generated)
func TestThreadService_CRUD_ViaHTTP(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	// Create thread server and register on mux (mirrors registerConnectHandlers)
	threadSvc := NewThreadServer(backend, publisher, slog.Default())

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewThreadServiceHandler(threadSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Connect client — this is how the frontend talks to the API
	client := orcv1connect.NewThreadServiceClient(http.DefaultClient, ts.URL)
	ctx := context.Background()

	// --- CreateThread ---
	createResp, err := client.CreateThread(ctx, connect.NewRequest(&orcv1.CreateThreadRequest{
		Title:    "HTTP round-trip test",
		TaskId:   threadIntegrationStringPtr("TASK-001"),
		FileContext: threadIntegrationStringPtr(`["main.go"]`),
	}))
	if err != nil {
		t.Fatalf("CreateThread via HTTP: %v", err)
	}
	if createResp.Msg.Thread == nil {
		t.Fatal("expected non-nil thread")
	}
	threadID := createResp.Msg.Thread.Id
	if threadID == "" {
		t.Fatal("expected non-empty thread ID")
	}

	// --- GetThread ---
	getResp, err := client.GetThread(ctx, connect.NewRequest(&orcv1.GetThreadRequest{
		ThreadId: threadID,
	}))
	if err != nil {
		t.Fatalf("GetThread via HTTP: %v", err)
	}
	if getResp.Msg.Thread.Title != "HTTP round-trip test" {
		t.Errorf("expected title 'HTTP round-trip test', got %q", getResp.Msg.Thread.Title)
	}

	// --- ListThreads ---
	listResp, err := client.ListThreads(ctx, connect.NewRequest(&orcv1.ListThreadsRequest{}))
	if err != nil {
		t.Fatalf("ListThreads via HTTP: %v", err)
	}
	if len(listResp.Msg.Threads) != 1 {
		t.Errorf("expected 1 thread in list, got %d", len(listResp.Msg.Threads))
	}

	// --- ArchiveThread ---
	_, err = client.ArchiveThread(ctx, connect.NewRequest(&orcv1.ArchiveThreadRequest{
		ThreadId: threadID,
	}))
	if err != nil {
		t.Fatalf("ArchiveThread via HTTP: %v", err)
	}

	// Verify archive via GetThread
	getResp2, err := client.GetThread(ctx, connect.NewRequest(&orcv1.GetThreadRequest{
		ThreadId: threadID,
	}))
	if err != nil {
		t.Fatalf("GetThread after archive: %v", err)
	}
	if getResp2.Msg.Thread.Status != "archived" {
		t.Errorf("expected status 'archived', got %q", getResp2.Msg.Thread.Status)
	}

	// --- DeleteThread ---
	_, err = client.DeleteThread(ctx, connect.NewRequest(&orcv1.DeleteThreadRequest{
		ThreadId: threadID,
	}))
	if err != nil {
		t.Fatalf("DeleteThread via HTTP: %v", err)
	}

	// Verify delete via GetThread (should return not-found or empty)
	getResp3, err := client.GetThread(ctx, connect.NewRequest(&orcv1.GetThreadRequest{
		ThreadId: threadID,
	}))
	if err != nil {
		// NotFound error is expected
		connectErr, ok := err.(*connect.Error)
		if !ok || connectErr.Code() != connect.CodeNotFound {
			t.Fatalf("expected NotFound after delete, got: %v", err)
		}
	} else if getResp3.Msg.Thread != nil {
		t.Error("expected nil thread after delete")
	}
}

// ============================================================================
// Integration: SendMessage HTTP round-trip with TurnExecutor wiring
// ============================================================================

// TestThreadService_SendMessage_ViaHTTP verifies the full SendMessage flow
// through HTTP, including TurnExecutor invocation. This tests that:
//   - The proto SendThreadMessageRequest/Response serialize correctly over HTTP
//   - The TurnExecutor factory wiring works when called through HTTP layer
//   - Both user and assistant messages are returned via HTTP response
func TestThreadService_SendMessage_ViaHTTP(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mock := executor.NewMockTurnExecutor("HTTP integration response")

	threadSvc := NewThreadServer(backend, publisher, slog.Default())
	threadSvc.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewThreadServiceHandler(threadSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewThreadServiceClient(http.DefaultClient, ts.URL)
	ctx := context.Background()

	// Create thread first
	createResp, err := client.CreateThread(ctx, connect.NewRequest(&orcv1.CreateThreadRequest{
		Title: "SendMessage HTTP test",
	}))
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// Send message through HTTP
	sendResp, err := client.SendMessage(ctx, connect.NewRequest(&orcv1.SendThreadMessageRequest{
		ThreadId: threadID,
		Content:  "Hello from HTTP client",
	}))
	if err != nil {
		t.Fatalf("SendMessage via HTTP: %v", err)
	}

	// Verify user message in response
	if sendResp.Msg.UserMessage == nil {
		t.Fatal("expected user message in HTTP response")
	}
	if sendResp.Msg.UserMessage.Role != "user" {
		t.Errorf("expected user role, got %q", sendResp.Msg.UserMessage.Role)
	}
	if sendResp.Msg.UserMessage.Content != "Hello from HTTP client" {
		t.Errorf("expected user content preserved, got %q", sendResp.Msg.UserMessage.Content)
	}

	// Verify assistant message in response
	if sendResp.Msg.AssistantMessage == nil {
		t.Fatal("expected assistant message in HTTP response")
	}
	if sendResp.Msg.AssistantMessage.Content != "HTTP integration response" {
		t.Errorf("expected mock response content, got %q", sendResp.Msg.AssistantMessage.Content)
	}

	// Verify TurnExecutor was called
	if mock.CallCount() != 1 {
		t.Errorf("expected 1 TurnExecutor call, got %d", mock.CallCount())
	}

	// Verify messages persisted (via GetThread over HTTP)
	getResp, err := client.GetThread(ctx, connect.NewRequest(&orcv1.GetThreadRequest{
		ThreadId: threadID,
	}))
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if len(getResp.Msg.Thread.Messages) != 2 {
		t.Errorf("expected 2 persisted messages, got %d", len(getResp.Msg.Thread.Messages))
	}
}

// ============================================================================
// Integration: RecordDecision HTTP round-trip
// ============================================================================

// TestThreadService_RecordDecision_ViaHTTP verifies that RecordDecision
// works through the HTTP layer and actually writes to the initiative_decisions
// table via the production storage path.
func TestThreadService_RecordDecision_ViaHTTP(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	// Create initiative in storage (use db.Initiative + backend.DB() like unit tests)
	initiative := &db.Initiative{
		ID:     "INIT-001",
		Title:  "Auth System",
		Status: "active",
	}
	if err := backend.DB().SaveInitiative(initiative); err != nil {
		t.Fatalf("SaveInitiative: %v", err)
	}

	threadSvc := NewThreadServer(backend, publisher, slog.Default())

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewThreadServiceHandler(threadSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewThreadServiceClient(http.DefaultClient, ts.URL)
	ctx := context.Background()

	// Create thread linked to initiative
	createResp, err := client.CreateThread(ctx, connect.NewRequest(&orcv1.CreateThreadRequest{
		Title:        "Decision recording test",
		InitiativeId: threadIntegrationStringPtr("INIT-001"),
	}))
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// Record decision through HTTP
	_, err = client.RecordDecision(ctx, connect.NewRequest(&orcv1.RecordThreadDecisionRequest{
		ThreadId:  threadID,
		Decision:  "Use JWT tokens",
		Rationale: "Industry standard for stateless auth",
	}))
	if err != nil {
		t.Fatalf("RecordDecision via HTTP: %v", err)
	}

	// Verify decision is persisted in the initiative_decisions table
	decisions, err := backend.DB().GetInitiativeDecisions("INIT-001")
	if err != nil {
		t.Fatalf("GetInitiativeDecisions: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Decision != "Use JWT tokens" {
		t.Errorf("expected decision 'Use JWT tokens', got %q", decisions[0].Decision)
	}
}

// ============================================================================
// Integration: RecordDecision errors correctly via HTTP
// ============================================================================

// TestThreadService_RecordDecision_NoInitiative_ViaHTTP verifies that
// RecordDecision returns FailedPrecondition when the thread has no linked
// initiative, even when called through the HTTP transport layer.
func TestThreadService_RecordDecision_NoInitiative_ViaHTTP(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	threadSvc := NewThreadServer(backend, publisher, slog.Default())

	mux := http.NewServeMux()
	path, handler := orcv1connect.NewThreadServiceHandler(threadSvc)
	mux.Handle(path, corsHandler(handler))

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := orcv1connect.NewThreadServiceClient(http.DefaultClient, ts.URL)
	ctx := context.Background()

	// Create thread WITHOUT initiative link
	createResp, err := client.CreateThread(ctx, connect.NewRequest(&orcv1.CreateThreadRequest{
		Title: "No initiative thread",
	}))
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// RecordDecision should fail with FailedPrecondition
	_, err = client.RecordDecision(ctx, connect.NewRequest(&orcv1.RecordThreadDecisionRequest{
		ThreadId:  createResp.Msg.Thread.Id,
		Decision:  "Some decision",
		Rationale: "Some reason",
	}))
	if err == nil {
		t.Fatal("expected error when thread has no initiative, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}
	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", connectErr.Code())
	}
}

// ============================================================================
// Helpers
// ============================================================================

func threadIntegrationStringPtr(s string) *string {
	return &s
}
