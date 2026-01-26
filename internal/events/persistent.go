package events

import (
	"log/slog"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

const (
	// Buffer flushes when it reaches this size
	bufferSizeThreshold = 10
	// Buffer flushes automatically every 5 seconds
	flushInterval = 5 * time.Second
)

// PersistentPublisher wraps MemoryPublisher and adds database persistence.
// It maintains WebSocket broadcast behavior while writing events to the event_log table.
type PersistentPublisher struct {
	inner       *MemoryPublisher
	backend     storage.Backend
	source      string
	buffer      []*db.EventLog
	bufferMu    sync.Mutex
	flushTicker *time.Ticker
	phaseStarts map[string]time.Time // key: "taskID:phase"
	startsMu    sync.RWMutex
	logger      *slog.Logger
	stopCh      chan struct{}
	wg          sync.WaitGroup
	closeOnce   sync.Once
}

// NewPersistentPublisher creates a new persistent event publisher.
// The source parameter identifies where events originate (e.g., "executor", "api").
func NewPersistentPublisher(backend storage.Backend, source string, logger *slog.Logger, opts ...PublisherOption) *PersistentPublisher {
	if logger == nil {
		logger = slog.Default()
	}

	p := &PersistentPublisher{
		inner:       NewMemoryPublisher(opts...),
		backend:     backend,
		source:      source,
		buffer:      make([]*db.EventLog, 0, bufferSizeThreshold),
		phaseStarts: make(map[string]time.Time),
		logger:      logger,
		stopCh:      make(chan struct{}),
	}

	// Start background flush ticker
	p.flushTicker = time.NewTicker(flushInterval)
	p.wg.Add(1)
	go p.flushLoop()

	return p
}

// Publish sends an event to subscribers and persists it to the database.
func (p *PersistentPublisher) Publish(event Event) {
	// Always broadcast to WebSocket subscribers first (real-time delivery)
	p.inner.Publish(event)

	// Skip persistence if backend is nil (testing scenarios)
	if p.backend == nil {
		return
	}

	// Convert to EventLog and add to buffer
	eventLog := p.eventToLog(event)
	if eventLog == nil {
		return // Skip events that don't need persistence
	}

	p.bufferMu.Lock()
	p.buffer = append(p.buffer, eventLog)
	bufferSize := len(p.buffer)
	shouldFlush := bufferSize >= bufferSizeThreshold
	p.bufferMu.Unlock()

	// Flush if buffer threshold reached
	if shouldFlush {
		p.flush()
	}

	// Track phase start times for duration calculation
	p.trackPhaseStart(event)

	// Flush on phase completion to ensure duration is persisted
	if p.isPhaseCompletion(event) {
		p.flush()
	}
}

// Subscribe returns a channel that receives events for the given task.
func (p *PersistentPublisher) Subscribe(taskID string) <-chan Event {
	return p.inner.Subscribe(taskID)
}

// Unsubscribe removes a subscription channel.
func (p *PersistentPublisher) Unsubscribe(taskID string, ch <-chan Event) {
	p.inner.Unsubscribe(taskID, ch)
}

// Close shuts down the publisher, flushes remaining events, and releases resources.
// Close is idempotent and safe to call multiple times.
func (p *PersistentPublisher) Close() {
	p.closeOnce.Do(func() {
		// Signal flush loop to stop
		close(p.stopCh)

		// Stop the ticker
		p.flushTicker.Stop()

		// Wait for flush loop to finish
		p.wg.Wait()

		// Final flush
		p.flush()

		// Close inner publisher
		p.inner.Close()
	})
}

// flushLoop runs in the background and flushes the buffer every 5 seconds.
func (p *PersistentPublisher) flushLoop() {
	defer p.wg.Done()

	for {
		select {
		case <-p.flushTicker.C:
			p.flush()
		case <-p.stopCh:
			return
		}
	}
}

// flush writes buffered events to the database in a single batch.
func (p *PersistentPublisher) flush() {
	p.bufferMu.Lock()
	if len(p.buffer) == 0 {
		p.bufferMu.Unlock()
		return
	}

	// Swap buffer for new empty one
	toFlush := p.buffer
	p.buffer = make([]*db.EventLog, 0, bufferSizeThreshold)
	p.bufferMu.Unlock()

	// Write to database outside the lock
	if err := p.backend.SaveEvents(toFlush); err != nil {
		p.logger.Error("failed to persist events", "error", err, "count", len(toFlush))
		// Don't retry - just log and continue to prevent memory buildup
	}
}

// eventToLog converts an Event to an EventLog for database storage.
func (p *PersistentPublisher) eventToLog(e Event) *db.EventLog {
	var phase *string
	var iteration *int
	var durationMs *int64

	// Extract phase/iteration from typed event data
	switch data := e.Data.(type) {
	case PhaseUpdate:
		phase = &data.Phase

		// Calculate duration for completed phases
		if data.Status == "completed" {
			if dur := p.getPhaseStart(e.TaskID, data.Phase); dur != nil {
				ms := int64(e.Time.Sub(*dur).Milliseconds())
				durationMs = &ms
			}
		}

	case TranscriptLine:
		phase = &data.Phase
		iteration = &data.Iteration

	case ActivityUpdate:
		phase = &data.Phase

	case TokenUpdate:
		phase = &data.Phase

	case ErrorData:
		if data.Phase != "" {
			phase = &data.Phase
		}

	case WarningData:
		if data.Phase != "" {
			phase = &data.Phase
		}

	case HeartbeatData:
		phase = &data.Phase
		iteration = &data.Iteration

	case DecisionRequiredData:
		phase = &data.Phase

	case DecisionResolvedData:
		phase = &data.Phase

	case FilesChangedUpdate:
		// FilesChangedUpdate doesn't have phase info, just persist as-is
	}

	return &db.EventLog{
		TaskID:     e.TaskID,
		Phase:      phase,
		Iteration:  iteration,
		EventType:  string(e.Type),
		Data:       e.Data,
		Source:     p.source,
		CreatedAt:  e.Time,
		DurationMs: durationMs,
	}
}

// trackPhaseStart records when a phase starts for duration calculation.
func (p *PersistentPublisher) trackPhaseStart(e Event) {
	if phaseUpdate, ok := e.Data.(PhaseUpdate); ok && phaseUpdate.Status == "started" {
		key := e.TaskID + ":" + phaseUpdate.Phase
		p.startsMu.Lock()
		p.phaseStarts[key] = e.Time
		p.startsMu.Unlock()
	}
}

// getPhaseStart retrieves the start time for a phase and cleans it up.
func (p *PersistentPublisher) getPhaseStart(taskID, phase string) *time.Time {
	key := taskID + ":" + phase
	p.startsMu.Lock()
	defer p.startsMu.Unlock()

	if t, ok := p.phaseStarts[key]; ok {
		// Clean up the entry to prevent memory leak
		delete(p.phaseStarts, key)
		return &t
	}
	return nil
}

// isPhaseCompletion returns true if this event marks phase completion.
func (p *PersistentPublisher) isPhaseCompletion(e Event) bool {
	phaseUpdate, ok := e.Data.(PhaseUpdate)
	return ok && phaseUpdate.Status == "completed"
}
