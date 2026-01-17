package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/storage"
)

// TranscriptPersister defines the interface for persisting transcripts.
// storage.Backend satisfies this interface.
type TranscriptPersister interface {
	AddTranscriptBatch(ctx context.Context, transcripts []storage.Transcript) error
}

// TranscriptBuffer accumulates transcript lines and persists them in batches.
// It provides automatic flushing based on line count or time interval, with
// explicit flush support for phase completion and shutdown.
//
// Thread-safe: All methods can be called concurrently.
type TranscriptBuffer struct {
	mu     sync.Mutex
	lines  []storage.Transcript
	chunks map[string]*chunkAccumulator // key: phase:iteration

	taskID        string
	db            TranscriptPersister
	logger        *slog.Logger
	maxBuffer     int           // Flush after this many lines
	flushInterval time.Duration // Flush after this duration

	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	doneCh   chan struct{}
}

// chunkAccumulator collects streaming chunks until a newline or flush.
type chunkAccumulator struct {
	content   strings.Builder
	phase     string
	iteration int
	timestamp time.Time
}

// TranscriptBufferConfig configures a TranscriptBuffer.
type TranscriptBufferConfig struct {
	TaskID        string
	DB            TranscriptPersister
	Logger        *slog.Logger
	MaxBuffer     int           // Default: 50
	FlushInterval time.Duration // Default: 5s
}

// NewTranscriptBuffer creates a new transcript buffer that persists to the database.
// It starts a background goroutine for periodic flushing.
// Call Close() to stop the background flusher and flush remaining lines.
func NewTranscriptBuffer(ctx context.Context, cfg TranscriptBufferConfig) *TranscriptBuffer {
	if cfg.MaxBuffer <= 0 {
		cfg.MaxBuffer = 50
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	bufCtx, cancel := context.WithCancel(ctx)
	b := &TranscriptBuffer{
		lines:         make([]storage.Transcript, 0, cfg.MaxBuffer),
		chunks:        make(map[string]*chunkAccumulator),
		taskID:        cfg.TaskID,
		db:            cfg.DB,
		logger:        cfg.Logger,
		maxBuffer:     cfg.MaxBuffer,
		flushInterval: cfg.FlushInterval,
		ctx:           bufCtx,
		cancel:        cancel,
		doneCh:        make(chan struct{}),
	}

	go b.periodicFlush()
	return b
}

// Add adds a complete transcript line to the buffer.
// If the buffer reaches maxBuffer lines, it triggers an immediate flush.
func (b *TranscriptBuffer) Add(phase string, iteration int, role, content string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.lines = append(b.lines, storage.Transcript{
		TaskID:    b.taskID,
		Phase:     phase,
		Iteration: iteration,
		Role:      role,
		Content:   content,
		Timestamp: time.Now().Unix(),
	})

	if len(b.lines) >= b.maxBuffer {
		b.flushLocked()
	}
}

// AddChunk accumulates a streaming chunk. Chunks are combined until a newline
// is encountered or FlushChunks is called (e.g., at phase completion).
func (b *TranscriptBuffer) AddChunk(phase string, iteration int, chunk string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := chunkKey(phase, iteration)
	acc, ok := b.chunks[key]
	if !ok {
		acc = &chunkAccumulator{
			phase:     phase,
			iteration: iteration,
			timestamp: time.Now(),
		}
		b.chunks[key] = acc
	}

	// Write chunk to accumulator
	acc.content.WriteString(chunk)

	// Check for complete lines (ending with newline)
	content := acc.content.String()
	if idx := strings.LastIndex(content, "\n"); idx >= 0 {
		// Flush complete lines
		completeLines := content[:idx+1]
		remaining := content[idx+1:]

		// Add complete lines as a single transcript entry
		if len(strings.TrimSpace(completeLines)) > 0 {
			b.lines = append(b.lines, storage.Transcript{
				TaskID:    b.taskID,
				Phase:     phase,
				Iteration: iteration,
				Role:      "chunk",
				Content:   completeLines,
				Timestamp: acc.timestamp.Unix(),
			})
		}

		// Keep remaining partial line
		acc.content.Reset()
		acc.content.WriteString(remaining)
		acc.timestamp = time.Now()

		// Check if buffer should flush
		if len(b.lines) >= b.maxBuffer {
			b.flushLocked()
		}
	}
}

// FlushChunks flushes any pending partial chunks for the given phase/iteration.
// Call this at phase completion to ensure no data is lost.
func (b *TranscriptBuffer) FlushChunks(phase string, iteration int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := chunkKey(phase, iteration)
	if acc, ok := b.chunks[key]; ok {
		content := strings.TrimSpace(acc.content.String())
		if len(content) > 0 {
			b.lines = append(b.lines, storage.Transcript{
				TaskID:    b.taskID,
				Phase:     phase,
				Iteration: iteration,
				Role:      "chunk",
				Content:   content,
				Timestamp: acc.timestamp.Unix(),
			})
		}
		delete(b.chunks, key)
	}
}

// Flush writes all buffered lines to the database.
// This is called automatically by the periodic flusher, but can also be
// called explicitly at phase completion.
func (b *TranscriptBuffer) Flush() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flushLocked()
}

// flushLocked performs the actual flush. Must be called with mu held.
func (b *TranscriptBuffer) flushLocked() error {
	if len(b.lines) == 0 {
		return nil
	}

	if b.db == nil {
		b.logger.Warn("transcript buffer: no database configured, discarding lines",
			"count", len(b.lines))
		b.lines = b.lines[:0]
		return nil
	}

	// Copy lines for the batch insert
	toWrite := make([]storage.Transcript, len(b.lines))
	copy(toWrite, b.lines)
	b.lines = b.lines[:0]

	// Use background context for the write to ensure it completes
	// even if the parent context is cancelled
	writeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := b.db.AddTranscriptBatch(writeCtx, toWrite); err != nil {
		b.logger.Error("transcript buffer: batch write failed",
			"error", err,
			"count", len(toWrite))
		return err
	}

	b.logger.Debug("transcript buffer: flushed lines",
		"count", len(toWrite),
		"task_id", b.taskID)
	return nil
}

// periodicFlush runs in the background, flushing the buffer periodically.
func (b *TranscriptBuffer) periodicFlush() {
	defer close(b.doneCh)

	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			if err := b.Flush(); err != nil {
				// Error already logged in flushLocked
			}
		}
	}
}

// Close stops the periodic flusher and flushes any remaining lines.
// It flushes all pending chunks before the final flush.
// Safe to call multiple times.
func (b *TranscriptBuffer) Close() error {
	var flushErr error
	b.stopOnce.Do(func() {
		// Stop the periodic flusher
		b.cancel()
		<-b.doneCh

		// Flush any remaining chunks
		b.mu.Lock()
		for key, acc := range b.chunks {
			content := strings.TrimSpace(acc.content.String())
			if len(content) > 0 {
				b.lines = append(b.lines, storage.Transcript{
					TaskID:    b.taskID,
					Phase:     acc.phase,
					Iteration: acc.iteration,
					Role:      "chunk",
					Content:   content,
					Timestamp: acc.timestamp.Unix(),
				})
			}
			delete(b.chunks, key)
		}
		b.mu.Unlock()

		// Final flush
		flushErr = b.Flush()
	})
	return flushErr
}

// LineCount returns the current number of buffered lines (for testing).
func (b *TranscriptBuffer) LineCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.lines)
}

// chunkKey generates a unique key for phase:iteration.
func chunkKey(phase string, iteration int) string {
	return fmt.Sprintf("%s:%d", phase, iteration)
}
