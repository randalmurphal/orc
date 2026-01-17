package executor

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// mockPersister is a test mock for TranscriptPersister.
type mockPersister struct {
	mu          sync.Mutex
	transcripts []storage.Transcript
	batchCount  int
	failOnWrite bool
}

func newMockPersister() *mockPersister {
	return &mockPersister{
		transcripts: make([]storage.Transcript, 0),
	}
}

func (m *mockPersister) AddTranscriptBatch(ctx context.Context, transcripts []storage.Transcript) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failOnWrite {
		return context.DeadlineExceeded
	}

	m.batchCount++
	for i := range transcripts {
		transcripts[i].ID = int64(len(m.transcripts) + i + 1)
	}
	m.transcripts = append(m.transcripts, transcripts...)
	return nil
}

func (m *mockPersister) getTranscripts() []storage.Transcript {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]storage.Transcript, len(m.transcripts))
	copy(result, m.transcripts)
	return result
}

func (m *mockPersister) getBatchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.batchCount
}

func TestTranscriptBuffer_Add(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-001",
		DB:            mock,
		MaxBuffer:     5,
		FlushInterval: time.Hour, // Long interval to control flushing manually
	})
	defer func() { _ = buf.Close() }()

	// Add 3 lines (under threshold)
	buf.Add("implement", 1, "prompt", "Hello")
	buf.Add("implement", 1, "response", "World")
	buf.Add("implement", 2, "prompt", "Continue")

	// Should not have flushed yet
	if mock.getBatchCount() != 0 {
		t.Errorf("expected 0 batch writes, got %d", mock.getBatchCount())
	}
	if buf.LineCount() != 3 {
		t.Errorf("expected 3 buffered lines, got %d", buf.LineCount())
	}

	// Add 2 more lines to trigger threshold flush
	buf.Add("implement", 2, "response", "Done")
	buf.Add("implement", 3, "prompt", "Next")

	// Should have auto-flushed
	if mock.getBatchCount() != 1 {
		t.Errorf("expected 1 batch write after threshold, got %d", mock.getBatchCount())
	}
	if len(mock.getTranscripts()) != 5 {
		t.Errorf("expected 5 transcripts in db, got %d", len(mock.getTranscripts()))
	}
	if buf.LineCount() != 0 {
		t.Errorf("expected 0 buffered lines after flush, got %d", buf.LineCount())
	}
}

func TestTranscriptBuffer_ManualFlush(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-002",
		DB:            mock,
		MaxBuffer:     100,
		FlushInterval: time.Hour,
	})
	defer func() { _ = buf.Close() }()

	buf.Add("spec", 1, "prompt", "Test")
	buf.Add("spec", 1, "response", "Result")

	// Manual flush
	if err := buf.Flush(); err != nil {
		t.Errorf("flush failed: %v", err)
	}

	if mock.getBatchCount() != 1 {
		t.Errorf("expected 1 batch write, got %d", mock.getBatchCount())
	}
	if len(mock.getTranscripts()) != 2 {
		t.Errorf("expected 2 transcripts, got %d", len(mock.getTranscripts()))
	}

	// Second flush with no data should be no-op
	if err := buf.Flush(); err != nil {
		t.Errorf("empty flush failed: %v", err)
	}
	if mock.getBatchCount() != 1 {
		t.Errorf("expected still 1 batch write after empty flush, got %d", mock.getBatchCount())
	}
}

func TestTranscriptBuffer_PeriodicFlush(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-003",
		DB:            mock,
		MaxBuffer:     100,
		FlushInterval: 50 * time.Millisecond,
	})
	defer func() { _ = buf.Close() }()

	buf.Add("test", 1, "prompt", "Periodic test")

	// Wait for periodic flush
	time.Sleep(100 * time.Millisecond)

	if mock.getBatchCount() < 1 {
		t.Errorf("expected at least 1 periodic flush, got %d", mock.getBatchCount())
	}
	if len(mock.getTranscripts()) != 1 {
		t.Errorf("expected 1 transcript from periodic flush, got %d", len(mock.getTranscripts()))
	}
}

func TestTranscriptBuffer_AddChunk(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-004",
		DB:            mock,
		MaxBuffer:     100,
		FlushInterval: time.Hour,
	})
	defer func() { _ = buf.Close() }()

	// Add chunks that form complete lines
	buf.AddChunk("implement", 1, "Hello ")
	buf.AddChunk("implement", 1, "World\n")
	buf.AddChunk("implement", 1, "Next ")
	buf.AddChunk("implement", 1, "Line\n")

	// Should have 2 lines buffered (complete lines from chunks)
	if buf.LineCount() != 2 {
		t.Errorf("expected 2 buffered lines from chunks, got %d", buf.LineCount())
	}

	// Flush and verify
	if err := buf.Flush(); err != nil {
		t.Errorf("flush failed: %v", err)
	}

	transcripts := mock.getTranscripts()
	if len(transcripts) != 2 {
		t.Errorf("expected 2 transcripts, got %d", len(transcripts))
	}
	if transcripts[0].Role != "chunk" {
		t.Errorf("expected role 'chunk', got '%s'", transcripts[0].Role)
	}
	if transcripts[0].Content != "Hello World\n" {
		t.Errorf("expected 'Hello World\\n', got '%s'", transcripts[0].Content)
	}
}

func TestTranscriptBuffer_FlushChunks(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-005",
		DB:            mock,
		MaxBuffer:     100,
		FlushInterval: time.Hour,
	})
	defer func() { _ = buf.Close() }()

	// Add partial chunks (no newline)
	buf.AddChunk("implement", 1, "Partial ")
	buf.AddChunk("implement", 1, "content")

	// No complete lines yet
	if buf.LineCount() != 0 {
		t.Errorf("expected 0 buffered lines (incomplete), got %d", buf.LineCount())
	}

	// Flush chunks for this iteration
	buf.FlushChunks("implement", 1)

	// Now should have 1 line
	if buf.LineCount() != 1 {
		t.Errorf("expected 1 buffered line after FlushChunks, got %d", buf.LineCount())
	}

	if err := buf.Flush(); err != nil {
		t.Errorf("flush failed: %v", err)
	}

	transcripts := mock.getTranscripts()
	if len(transcripts) != 1 {
		t.Errorf("expected 1 transcript, got %d", len(transcripts))
	}
	if transcripts[0].Content != "Partial content" {
		t.Errorf("expected 'Partial content', got '%s'", transcripts[0].Content)
	}
}

func TestTranscriptBuffer_Close(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-006",
		DB:            mock,
		MaxBuffer:     100,
		FlushInterval: time.Hour,
	})

	// Add lines and partial chunks
	buf.Add("implement", 1, "prompt", "Test")
	buf.AddChunk("implement", 1, "Partial chunk")

	// Close should flush everything
	if err := buf.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}

	transcripts := mock.getTranscripts()
	if len(transcripts) != 2 {
		t.Errorf("expected 2 transcripts after close, got %d", len(transcripts))
	}

	// Verify the chunk was flushed
	found := false
	for _, tr := range transcripts {
		if tr.Content == "Partial chunk" {
			found = true
			break
		}
	}
	if !found {
		t.Error("partial chunk was not flushed on close")
	}

	// Second close should be safe (idempotent)
	if err := buf.Close(); err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestTranscriptBuffer_MultipleIterations(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-007",
		DB:            mock,
		MaxBuffer:     100,
		FlushInterval: time.Hour,
	})
	defer func() { _ = buf.Close() }()

	// Simulate multiple iterations with interleaved chunks
	buf.AddChunk("implement", 1, "Iter1 ")
	buf.AddChunk("implement", 2, "Iter2 ")
	buf.AddChunk("implement", 1, "content\n")
	buf.AddChunk("implement", 2, "content\n")

	if buf.LineCount() != 2 {
		t.Errorf("expected 2 lines from different iterations, got %d", buf.LineCount())
	}

	if err := buf.Flush(); err != nil {
		t.Errorf("flush failed: %v", err)
	}

	transcripts := mock.getTranscripts()
	if len(transcripts) != 2 {
		t.Errorf("expected 2 transcripts, got %d", len(transcripts))
	}

	// Verify both iterations are represented
	iterations := map[int]bool{}
	for _, tr := range transcripts {
		iterations[tr.Iteration] = true
	}
	if !iterations[1] || !iterations[2] {
		t.Error("expected transcripts from both iterations 1 and 2")
	}
}

func TestTranscriptBuffer_TranscriptFields(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-008",
		DB:            mock,
		MaxBuffer:     100,
		FlushInterval: time.Hour,
	})
	defer func() { _ = buf.Close() }()

	before := time.Now().Unix()
	buf.Add("spec", 3, "response", "Test content")
	after := time.Now().Unix()

	if err := buf.Flush(); err != nil {
		t.Errorf("flush failed: %v", err)
	}

	transcripts := mock.getTranscripts()
	if len(transcripts) != 1 {
		t.Fatalf("expected 1 transcript, got %d", len(transcripts))
	}

	tr := transcripts[0]
	if tr.TaskID != "TASK-008" {
		t.Errorf("expected TaskID 'TASK-008', got '%s'", tr.TaskID)
	}
	if tr.Phase != "spec" {
		t.Errorf("expected Phase 'spec', got '%s'", tr.Phase)
	}
	if tr.Iteration != 3 {
		t.Errorf("expected Iteration 3, got %d", tr.Iteration)
	}
	if tr.Role != "response" {
		t.Errorf("expected Role 'response', got '%s'", tr.Role)
	}
	if tr.Content != "Test content" {
		t.Errorf("expected Content 'Test content', got '%s'", tr.Content)
	}
	if tr.Timestamp < before || tr.Timestamp > after {
		t.Errorf("expected Timestamp between %d and %d, got %d", before, after, tr.Timestamp)
	}
	if tr.ID != 1 {
		t.Errorf("expected ID to be set by mock, got %d", tr.ID)
	}
}

func TestTranscriptBuffer_NilDB(t *testing.T) {
	ctx := context.Background()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-009",
		DB:            nil, // No database
		MaxBuffer:     3,
		FlushInterval: time.Hour,
	})
	defer func() { _ = buf.Close() }()

	// Add enough to trigger threshold
	buf.Add("test", 1, "prompt", "Line 1")
	buf.Add("test", 1, "response", "Line 2")
	buf.Add("test", 2, "prompt", "Line 3")

	// Should not panic, lines should be discarded
	if buf.LineCount() != 0 {
		t.Errorf("expected 0 lines after discard (nil db), got %d", buf.LineCount())
	}
}

func TestTranscriptBuffer_Concurrent(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-010",
		DB:            mock,
		MaxBuffer:     100,
		FlushInterval: time.Hour,
	})
	defer func() { _ = buf.Close() }()

	// Spawn multiple goroutines adding transcripts
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				buf.Add("test", iteration, "prompt", "content")
			}
		}(i)
	}
	wg.Wait()

	// All 100 lines should be buffered
	if err := buf.Flush(); err != nil {
		t.Errorf("flush failed: %v", err)
	}

	if len(mock.getTranscripts()) != 100 {
		t.Errorf("expected 100 transcripts, got %d", len(mock.getTranscripts()))
	}
}

func TestTranscriptBuffer_ChunkAggregation(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-011",
		DB:            mock,
		MaxBuffer:     100,
		FlushInterval: time.Hour,
	})
	defer func() { _ = buf.Close() }()

	// Simulate streaming: multiple small chunks forming lines
	chunks := []string{"H", "e", "l", "l", "o", " ", "W", "o", "r", "l", "d", "\n"}
	for _, chunk := range chunks {
		buf.AddChunk("implement", 1, chunk)
	}

	// Should have 1 complete line
	if buf.LineCount() != 1 {
		t.Errorf("expected 1 buffered line, got %d", buf.LineCount())
	}

	if err := buf.Flush(); err != nil {
		t.Errorf("flush failed: %v", err)
	}

	transcripts := mock.getTranscripts()
	if len(transcripts) != 1 {
		t.Fatalf("expected 1 transcript, got %d", len(transcripts))
	}
	if transcripts[0].Content != "Hello World\n" {
		t.Errorf("expected 'Hello World\\n', got '%s'", transcripts[0].Content)
	}
}

func TestTranscriptBuffer_WriteFailure(t *testing.T) {
	ctx := context.Background()
	mock := newMockPersister()
	mock.failOnWrite = true

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        "TASK-012",
		DB:            mock,
		MaxBuffer:     2,
		FlushInterval: time.Hour,
	})
	defer func() { _ = buf.Close() }()

	// Add lines to trigger auto-flush
	buf.Add("test", 1, "prompt", "Line 1")
	buf.Add("test", 1, "response", "Line 2")

	// Buffer should be cleared even on failure (to prevent infinite retry)
	if buf.LineCount() != 0 {
		t.Errorf("expected 0 lines after failed flush, got %d", buf.LineCount())
	}
}

// TestTranscriptBuffer_Integration_RealDatabase tests transcript persistence
// with a real SQLite database backend, verifying the full write and read flow.
func TestTranscriptBuffer_Integration_RealDatabase(t *testing.T) {
	// Create a temporary directory for the test database
	tmpDir, err := os.MkdirTemp("", "orc-buffer-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a real database backend
	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	taskID := "TASK-INT-001"

	// Create a task first (required due to foreign key constraint)
	testTask := task.New(taskID, "Integration test task")
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create transcript buffer using real database
	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        taskID,
		DB:            backend,
		MaxBuffer:     10,
		FlushInterval: time.Hour, // Long interval to control flushing manually
	})

	// Simulate executor transcript output
	buf.Add("implement", 1, "prompt", "Implement the feature")
	buf.Add("implement", 1, "response", "I'll implement the feature now...")
	buf.AddChunk("implement", 1, "Here is the ")
	buf.AddChunk("implement", 1, "code implementation\n")
	buf.Add("implement", 2, "prompt", "Continue with tests")
	buf.Add("implement", 2, "response", "Adding test cases...")

	// Flush pending chunks for iteration 1
	buf.FlushChunks("implement", 1)

	// Close buffer - should flush everything to database
	if err := buf.Close(); err != nil {
		t.Fatalf("close buffer: %v", err)
	}

	// Retrieve transcripts from database and verify
	transcripts, err := backend.GetTranscripts(taskID)
	if err != nil {
		t.Fatalf("get transcripts: %v", err)
	}

	// Should have: 4 Add() lines + 1 aggregated chunk line = 5 transcripts
	if len(transcripts) != 5 {
		t.Errorf("expected 5 transcripts, got %d", len(transcripts))
		for i, tr := range transcripts {
			t.Logf("  [%d] phase=%s iter=%d role=%s content=%q",
				i, tr.Phase, tr.Iteration, tr.Role, tr.Content)
		}
	}

	// Verify transcripts are in correct order (by ID)
	for i, tr := range transcripts {
		if tr.TaskID != taskID {
			t.Errorf("transcript[%d]: expected TaskID %q, got %q", i, taskID, tr.TaskID)
		}
		if tr.Phase != "implement" {
			t.Errorf("transcript[%d]: expected Phase 'implement', got %q", i, tr.Phase)
		}
	}

	// Verify specific content
	var foundPrompt, foundResponse, foundChunk bool
	for _, tr := range transcripts {
		switch {
		case tr.Role == "prompt" && tr.Content == "Implement the feature":
			foundPrompt = true
		case tr.Role == "response" && tr.Content == "I'll implement the feature now...":
			foundResponse = true
		case tr.Role == "chunk" && tr.Content == "Here is the code implementation\n":
			foundChunk = true
		}
	}

	if !foundPrompt {
		t.Error("expected to find prompt transcript")
	}
	if !foundResponse {
		t.Error("expected to find response transcript")
	}
	if !foundChunk {
		t.Error("expected to find aggregated chunk transcript")
	}

	// Verify IDs were assigned (non-zero)
	for i, tr := range transcripts {
		if tr.ID == 0 {
			t.Errorf("transcript[%d]: expected non-zero ID", i)
		}
	}
}

// TestTranscriptBuffer_Integration_OrderPreservation verifies transcripts
// are retrieved in the same order they were written.
func TestTranscriptBuffer_Integration_OrderPreservation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-buffer-order-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	taskID := "TASK-ORDER-001"

	// Create a task first (required due to foreign key constraint)
	testTask := task.New(taskID, "Order preservation test task")
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        taskID,
		DB:            backend,
		MaxBuffer:     100,
		FlushInterval: time.Hour,
	})

	// Add many transcripts across multiple phases and iterations
	phases := []string{"spec", "implement", "review", "test"}
	for _, phase := range phases {
		for iter := 1; iter <= 3; iter++ {
			buf.Add(phase, iter, "prompt", phase+"-prompt")
			buf.Add(phase, iter, "response", phase+"-response")
		}
	}

	if err := buf.Close(); err != nil {
		t.Fatalf("close buffer: %v", err)
	}

	transcripts, err := backend.GetTranscripts(taskID)
	if err != nil {
		t.Fatalf("get transcripts: %v", err)
	}

	// 4 phases * 3 iterations * 2 entries = 24 transcripts
	if len(transcripts) != 24 {
		t.Errorf("expected 24 transcripts, got %d", len(transcripts))
	}

	// Verify IDs are sequential (order preserved)
	for i := 1; i < len(transcripts); i++ {
		if transcripts[i].ID <= transcripts[i-1].ID {
			t.Errorf("transcript[%d] ID (%d) should be greater than transcript[%d] ID (%d)",
				i, transcripts[i].ID, i-1, transcripts[i-1].ID)
		}
	}
}

// TestTranscriptBuffer_Integration_ConcurrentWrites tests that concurrent
// writes to the buffer are properly serialized and persisted.
func TestTranscriptBuffer_Integration_ConcurrentWrites(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orc-buffer-concurrent-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	ctx := context.Background()
	taskID := "TASK-CONC-001"

	// Create a task first (required due to foreign key constraint)
	testTask := task.New(taskID, "Concurrent writes test task")
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("save task: %v", err)
	}

	buf := NewTranscriptBuffer(ctx, TranscriptBufferConfig{
		TaskID:        taskID,
		DB:            backend,
		MaxBuffer:     20, // Small buffer to trigger multiple flushes
		FlushInterval: time.Hour,
	})

	// Spawn multiple goroutines writing concurrently
	const numWorkers = 5
	const writesPerWorker = 20

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < writesPerWorker; i++ {
				buf.Add("implement", worker, "response", "content")
			}
		}(w)
	}
	wg.Wait()

	if err := buf.Close(); err != nil {
		t.Fatalf("close buffer: %v", err)
	}

	transcripts, err := backend.GetTranscripts(taskID)
	if err != nil {
		t.Fatalf("get transcripts: %v", err)
	}

	expected := numWorkers * writesPerWorker
	if len(transcripts) != expected {
		t.Errorf("expected %d transcripts, got %d", expected, len(transcripts))
	}
}
