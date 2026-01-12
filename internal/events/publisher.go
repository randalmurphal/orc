package events

import (
	"sync"
)

// GlobalTaskID is the special task ID for subscribing to all task events.
// Subscribers to this ID receive events for ALL tasks.
const GlobalTaskID = "*"

// Publisher defines the interface for event publishing.
type Publisher interface {
	// Publish sends an event to all subscribers of the task.
	Publish(event Event)
	// Subscribe returns a channel that receives events for the given task.
	// Use GlobalTaskID ("*") to receive events for all tasks.
	Subscribe(taskID string) <-chan Event
	// Unsubscribe removes a subscription channel.
	Unsubscribe(taskID string, ch <-chan Event)
	// Close shuts down the publisher and all subscriptions.
	Close()
}

// MemoryPublisher is an in-memory implementation of Publisher.
type MemoryPublisher struct {
	subscribers map[string][]chan Event
	mu          sync.RWMutex
	bufferSize  int
	closed      bool
}

// PublisherOption configures a MemoryPublisher.
type PublisherOption func(*MemoryPublisher)

// WithBufferSize sets the channel buffer size for subscribers.
func WithBufferSize(size int) PublisherOption {
	return func(p *MemoryPublisher) {
		p.bufferSize = size
	}
}

// NewMemoryPublisher creates a new in-memory publisher.
func NewMemoryPublisher(opts ...PublisherOption) *MemoryPublisher {
	p := &MemoryPublisher{
		subscribers: make(map[string][]chan Event),
		bufferSize:  100, // Default buffer size
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Publish sends an event to all subscribers of the task.
// Also sends to global subscribers (those subscribed to GlobalTaskID).
// Non-blocking: skips subscribers with full buffers.
func (p *MemoryPublisher) Publish(event Event) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return
	}

	// Send to task-specific subscribers
	subs := p.subscribers[event.TaskID]
	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Skip if channel buffer is full (non-blocking)
		}
	}

	// Also send to global subscribers (if not already a global subscription)
	if event.TaskID != GlobalTaskID {
		globalSubs := p.subscribers[GlobalTaskID]
		for _, ch := range globalSubs {
			select {
			case ch <- event:
			default:
				// Skip if channel buffer is full (non-blocking)
			}
		}
	}
}

// Subscribe returns a channel that receives events for the given task.
func (p *MemoryPublisher) Subscribe(taskID string) <-chan Event {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		// Return closed channel if publisher is closed
		ch := make(chan Event)
		close(ch)
		return ch
	}

	ch := make(chan Event, p.bufferSize)
	p.subscribers[taskID] = append(p.subscribers[taskID], ch)
	return ch
}

// Unsubscribe removes a subscription channel.
func (p *MemoryPublisher) Unsubscribe(taskID string, ch <-chan Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	subs := p.subscribers[taskID]
	for i, sub := range subs {
		if sub == ch {
			// Remove from slice
			p.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
			// Close the channel
			close(sub)
			break
		}
	}

	// Clean up empty task entries
	if len(p.subscribers[taskID]) == 0 {
		delete(p.subscribers, taskID)
	}
}

// Close shuts down the publisher and closes all subscription channels.
func (p *MemoryPublisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return
	}

	p.closed = true

	// Close all subscriber channels
	for taskID, subs := range p.subscribers {
		for _, ch := range subs {
			close(ch)
		}
		delete(p.subscribers, taskID)
	}
}

// SubscriberCount returns the number of subscribers for a task.
func (p *MemoryPublisher) SubscriberCount(taskID string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.subscribers[taskID])
}

// TaskCount returns the number of tasks with subscribers.
func (p *MemoryPublisher) TaskCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.subscribers)
}

// NopPublisher is a no-op publisher for testing or when events are disabled.
type NopPublisher struct{}

// Publish does nothing.
func (p *NopPublisher) Publish(event Event) {}

// Subscribe returns a closed channel.
func (p *NopPublisher) Subscribe(taskID string) <-chan Event {
	ch := make(chan Event)
	close(ch)
	return ch
}

// Unsubscribe does nothing.
func (p *NopPublisher) Unsubscribe(taskID string, ch <-chan Event) {}

// Close does nothing.
func (p *NopPublisher) Close() {}

// NewNopPublisher creates a no-op publisher.
func NewNopPublisher() *NopPublisher {
	return &NopPublisher{}
}
