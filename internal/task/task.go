// Package task provides task management for orc.
package task

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/randalmurphal/orc/internal/util"
	"gopkg.in/yaml.v3"
)

const (
	// OrcDir is the default orc configuration directory
	OrcDir = ".orc"
	// TasksDir is the subdirectory for tasks
	TasksDir = "tasks"
)

// Weight represents the complexity classification of a task.
type Weight string

const (
	WeightTrivial    Weight = "trivial"
	WeightSmall      Weight = "small"
	WeightMedium     Weight = "medium"
	WeightLarge      Weight = "large"
	WeightGreenfield Weight = "greenfield"
)

// Status represents the current state of a task.
type Status string

const (
	StatusCreated     Status = "created"
	StatusClassifying Status = "classifying"
	StatusPlanned     Status = "planned"
	StatusRunning     Status = "running"
	StatusPaused      Status = "paused"
	StatusBlocked     Status = "blocked"
	StatusCompleted   Status = "completed"
	StatusFailed      Status = "failed"
)

// Task represents a unit of work to be orchestrated.
type Task struct {
	// ID is the unique identifier (e.g., TASK-001)
	ID string `yaml:"id" json:"id"`

	// Title is a short description of the task
	Title string `yaml:"title" json:"title"`

	// Description is the full task description
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Weight is the complexity classification
	Weight Weight `yaml:"weight" json:"weight"`

	// Status is the current execution state
	Status Status `yaml:"status" json:"status"`

	// CurrentPhase is the phase currently being executed
	CurrentPhase string `yaml:"current_phase,omitempty" json:"current_phase,omitempty"`

	// Branch is the git branch for this task (e.g., orc/TASK-001)
	Branch string `yaml:"branch" json:"branch"`

	// CreatedAt is when the task was created
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`

	// UpdatedAt is when the task was last updated
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`

	// StartedAt is when execution began
	StartedAt *time.Time `yaml:"started_at,omitempty" json:"started_at,omitempty"`

	// CompletedAt is when the task finished
	CompletedAt *time.Time `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`

	// Metadata holds arbitrary key-value data
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// New creates a new task with the given title.
func New(id, title string) *Task {
	now := time.Now()
	return &Task{
		ID:        id,
		Title:     title,
		Status:    StatusCreated,
		Branch:    "orc/" + id,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]string),
	}
}

// IsTerminal returns true if the task is in a terminal state.
func (t *Task) IsTerminal() bool {
	return t.Status == StatusCompleted || t.Status == StatusFailed
}

// CanRun returns true if the task can be executed.
func (t *Task) CanRun() bool {
	return t.Status == StatusCreated ||
		t.Status == StatusPlanned ||
		t.Status == StatusPaused ||
		t.Status == StatusBlocked
}

// Load loads a task from disk by ID.
func Load(id string) (*Task, error) {
	path := filepath.Join(OrcDir, TasksDir, id, "task.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task %s not found", id)
		}
		return nil, fmt.Errorf("read task %s: %w", id, err)
	}

	var t Task
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse task %s: %w", id, err)
	}

	return &t, nil
}

// Save persists the task to disk using atomic writes.
func (t *Task) Save() error {
	dir := filepath.Join(OrcDir, TasksDir, t.ID)
	return t.SaveTo(dir)
}

// LoadAll loads all tasks from disk.
func LoadAll() ([]*Task, error) {
	tasksDir := filepath.Join(OrcDir, TasksDir)

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No tasks yet
		}
		return nil, fmt.Errorf("read tasks directory: %w", err)
	}

	var tasks []*Task
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		t, err := Load(entry.Name())
		if err != nil {
			continue // Skip invalid tasks
		}
		tasks = append(tasks, t)
	}

	// Sort by creation time (newest first)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks, nil
}

// NextID generates the next task ID (TASK-001, TASK-002, etc.).
func NextID() (string, error) {
	tasksDir := filepath.Join(OrcDir, TasksDir)

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "TASK-001", nil
		}
		return "", fmt.Errorf("read tasks directory: %w", err)
	}

	// Find the highest existing ID
	taskIDRegex := regexp.MustCompile(`^TASK-(\d+)$`)
	maxNum := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		matches := taskIDRegex.FindStringSubmatch(entry.Name())
		if len(matches) == 2 {
			num, _ := strconv.Atoi(matches[1])
			if num > maxNum {
				maxNum = num
			}
		}
	}

	return fmt.Sprintf("TASK-%03d", maxNum+1), nil
}

// TaskDir returns the directory path for a task.
func TaskDir(id string) string {
	return filepath.Join(OrcDir, TasksDir, id)
}

// Exists returns true if a task exists.
func Exists(id string) bool {
	path := filepath.Join(OrcDir, TasksDir, id, "task.yaml")
	_, err := os.Stat(path)
	return err == nil
}

// Delete removes a task and all its associated files.
// Returns an error if the task is currently running.
func Delete(id string) error {
	t, err := Load(id)
	if err != nil {
		return fmt.Errorf("task %s not found", id)
	}

	if t.Status == StatusRunning {
		return fmt.Errorf("cannot delete running task %s", id)
	}

	taskDir := TaskDir(id)
	return os.RemoveAll(taskDir)
}

// SaveTo persists the task to a specific directory using atomic writes.
func (t *Task) SaveTo(dir string) error {
	t.UpdatedAt = time.Now()

	data, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}

	path := filepath.Join(dir, "task.yaml")
	if err := util.AtomicWriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write task: %w", err)
	}

	return nil
}

// LoadAllFrom loads all tasks from a specific tasks directory.
func LoadAllFrom(tasksDir string) ([]*Task, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read tasks directory: %w", err)
	}

	var tasks []*Task
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(tasksDir, entry.Name(), "task.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var t Task
		if err := yaml.Unmarshal(data, &t); err != nil {
			continue
		}
		tasks = append(tasks, &t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks, nil
}

// NextIDIn generates the next task ID in a specific tasks directory.
func NextIDIn(tasksDir string) (string, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "TASK-001", nil
		}
		return "", fmt.Errorf("read tasks directory: %w", err)
	}

	taskIDRegex := regexp.MustCompile(`^TASK-(\d+)$`)
	maxNum := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		matches := taskIDRegex.FindStringSubmatch(entry.Name())
		if len(matches) == 2 {
			num, _ := strconv.Atoi(matches[1])
			if num > maxNum {
				maxNum = num
			}
		}
	}

	return fmt.Sprintf("TASK-%03d", maxNum+1), nil
}
