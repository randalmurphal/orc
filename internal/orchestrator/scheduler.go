// Package orchestrator provides multi-task coordination for orc.
package orchestrator

import (
	"container/heap"
	"sync"
	"time"
)

// TaskPriority represents task priority level.
type TaskPriority int

const (
	PriorityBackground TaskPriority = 10
	PriorityDefault    TaskPriority = 100
	PriorityUrgent     TaskPriority = 1000
)

// ScheduledTask represents a task in the scheduler queue.
type ScheduledTask struct {
	TaskID    string
	Title     string
	Priority  TaskPriority
	DependsOn []string
	CreatedAt time.Time

	// Index in the heap (managed by heap.Interface)
	index int
}

// TaskQueue is a priority queue of scheduled tasks.
type TaskQueue []*ScheduledTask

func (pq TaskQueue) Len() int { return len(pq) }

func (pq TaskQueue) Less(i, j int) bool {
	// Higher priority first, then earlier creation time
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

func (pq TaskQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *TaskQueue) Push(x any) {
	n := len(*pq)
	item := x.(*ScheduledTask)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *TaskQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// Scheduler manages task scheduling with dependency awareness.
type Scheduler struct {
	queue         TaskQueue
	maxConcurrent int
	completed     map[string]bool // Task ID -> completed
	running       map[string]bool // Task ID -> running
	taskDeps      map[string][]string // Task ID -> dependencies
	mu            sync.RWMutex
}

// NewScheduler creates a new scheduler.
func NewScheduler(maxConcurrent int) *Scheduler {
	if maxConcurrent < 1 {
		maxConcurrent = 4
	}
	s := &Scheduler{
		queue:         make(TaskQueue, 0),
		maxConcurrent: maxConcurrent,
		completed:     make(map[string]bool),
		running:       make(map[string]bool),
		taskDeps:      make(map[string][]string),
	}
	heap.Init(&s.queue)
	return s
}

// AddTask adds a task to the scheduler.
func (s *Scheduler) AddTask(taskID, title string, dependsOn []string, priority TaskPriority) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task := &ScheduledTask{
		TaskID:    taskID,
		Title:     title,
		Priority:  priority,
		DependsOn: dependsOn,
		CreatedAt: time.Now(),
	}

	s.taskDeps[taskID] = dependsOn
	heap.Push(&s.queue, task)
}

// NextReady returns the next task(s) ready to run.
// Returns up to n tasks that have all dependencies satisfied and aren't running.
func (s *Scheduler) NextReady(n int) []*ScheduledTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n <= 0 {
		n = s.maxConcurrent
	}

	// Limit by available slots
	availableSlots := s.maxConcurrent - len(s.running)
	if availableSlots <= 0 {
		return nil
	}
	if n > availableSlots {
		n = availableSlots
	}

	var ready []*ScheduledTask
	var notReady []*ScheduledTask

	// Pop tasks and check dependencies
	for s.queue.Len() > 0 && len(ready) < n {
		task := heap.Pop(&s.queue).(*ScheduledTask)

		if s.allDepsSatisfied(task) {
			ready = append(ready, task)
			s.running[task.TaskID] = true
		} else {
			notReady = append(notReady, task)
		}
	}

	// Push back tasks that weren't ready
	for _, t := range notReady {
		heap.Push(&s.queue, t)
	}

	return ready
}

// allDepsSatisfied checks if all dependencies are completed.
// Must be called with lock held.
func (s *Scheduler) allDepsSatisfied(task *ScheduledTask) bool {
	for _, dep := range task.DependsOn {
		if !s.completed[dep] {
			return false
		}
	}
	return true
}

// MarkCompleted marks a task as completed.
func (s *Scheduler) MarkCompleted(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.running, taskID)
	s.completed[taskID] = true
}

// MarkFailed marks a task as failed (removes from running).
func (s *Scheduler) MarkFailed(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.running, taskID)
}

// Requeue adds a task back to the queue (for retry).
func (s *Scheduler) Requeue(taskID, title string, priority TaskPriority) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.running, taskID)

	task := &ScheduledTask{
		TaskID:    taskID,
		Title:     title,
		Priority:  priority,
		DependsOn: s.taskDeps[taskID],
		CreatedAt: time.Now(),
	}
	heap.Push(&s.queue, task)
}

// RunningCount returns the number of running tasks.
func (s *Scheduler) RunningCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.running)
}

// QueueLength returns the number of queued tasks.
func (s *Scheduler) QueueLength() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queue.Len()
}

// CompletedCount returns the number of completed tasks.
func (s *Scheduler) CompletedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.completed)
}

// IsComplete returns true if all tasks are completed.
func (s *Scheduler) IsComplete() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queue.Len() == 0 && len(s.running) == 0
}

// GetRunningTasks returns the IDs of currently running tasks.
func (s *Scheduler) GetRunningTasks() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]string, 0, len(s.running))
	for id := range s.running {
		tasks = append(tasks, id)
	}
	return tasks
}
