// Package initiative provides initiative/feature grouping for related tasks.
// Initiatives provide shared context, vision, and decisions across multiple tasks.
package initiative

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Status represents the status of an initiative.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusArchived  Status = "archived"
)

// Identity represents the owner of an initiative.
type Identity struct {
	Initials    string `yaml:"initials" json:"initials"`
	DisplayName string `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	Email       string `yaml:"email,omitempty" json:"email,omitempty"`
}

// Decision represents a recorded decision within an initiative.
type Decision struct {
	ID        string    `yaml:"id" json:"id"`
	Date      time.Time `yaml:"date" json:"date"`
	By        string    `yaml:"by" json:"by"`
	Decision  string    `yaml:"decision" json:"decision"`
	Rationale string    `yaml:"rationale,omitempty" json:"rationale,omitempty"`
}

// TaskRef represents a reference to a task within an initiative.
type TaskRef struct {
	ID        string   `yaml:"id" json:"id"`
	Title     string   `yaml:"title" json:"title"`
	DependsOn []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Status    string   `yaml:"status" json:"status"`
}

// Initiative represents a grouping of related tasks with shared context.
type Initiative struct {
	Version      int        `yaml:"version" json:"version"`
	ID           string     `yaml:"id" json:"id"`
	Title        string     `yaml:"title" json:"title"`
	Status       Status     `yaml:"status" json:"status"`
	Owner        Identity   `yaml:"owner,omitempty" json:"owner,omitempty"`
	Vision       string     `yaml:"vision,omitempty" json:"vision,omitempty"`
	Decisions    []Decision `yaml:"decisions,omitempty" json:"decisions,omitempty"`
	ContextFiles []string   `yaml:"context_files,omitempty" json:"context_files,omitempty"`
	Tasks        []TaskRef  `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	CreatedAt    time.Time  `yaml:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `yaml:"updated_at" json:"updated_at"`
}

// Directory constants
const (
	// InitiativesDir is the subdirectory for initiatives
	InitiativesDir = "initiatives"
	// SharedDir is the shared directory for P2P mode
	SharedDir = "shared"
)

// GetInitiativesDir returns the initiatives directory path.
// In P2P mode, initiatives are stored in .orc/shared/initiatives/
// In solo mode, initiatives are stored in .orc/initiatives/
func GetInitiativesDir(shared bool) string {
	if shared {
		return filepath.Join(".orc", SharedDir, InitiativesDir)
	}
	return filepath.Join(".orc", InitiativesDir)
}

// New creates a new initiative with the given ID and title.
func New(id, title string) *Initiative {
	now := time.Now()
	return &Initiative{
		Version:   1,
		ID:        id,
		Title:     title,
		Status:    StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Load loads an initiative from disk.
func Load(id string) (*Initiative, error) {
	return LoadFrom(GetInitiativesDir(false), id)
}

// LoadShared loads a shared initiative from the shared directory.
func LoadShared(id string) (*Initiative, error) {
	return LoadFrom(GetInitiativesDir(true), id)
}

// LoadFrom loads an initiative from a specific directory.
func LoadFrom(baseDir, id string) (*Initiative, error) {
	path := filepath.Join(baseDir, id, "initiative.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read initiative %s: %w", id, err)
	}

	var init Initiative
	if err := yaml.Unmarshal(data, &init); err != nil {
		return nil, fmt.Errorf("parse initiative %s: %w", id, err)
	}

	return &init, nil
}

// Save persists the initiative to disk.
func (i *Initiative) Save() error {
	return i.SaveTo(GetInitiativesDir(false))
}

// SaveShared persists the initiative to the shared directory.
func (i *Initiative) SaveShared() error {
	return i.SaveTo(GetInitiativesDir(true))
}

// SaveTo persists the initiative to a specific directory.
func (i *Initiative) SaveTo(baseDir string) error {
	dir := filepath.Join(baseDir, i.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create initiative directory: %w", err)
	}

	i.UpdatedAt = time.Now()

	data, err := yaml.Marshal(i)
	if err != nil {
		return fmt.Errorf("marshal initiative: %w", err)
	}

	path := filepath.Join(dir, "initiative.yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write initiative: %w", err)
	}

	return nil
}

// AddTask adds a task reference to the initiative.
func (i *Initiative) AddTask(id, title string, dependsOn []string) {
	// Check if task already exists
	for idx, t := range i.Tasks {
		if t.ID == id {
			// Update existing task
			i.Tasks[idx].Title = title
			i.Tasks[idx].DependsOn = dependsOn
			i.UpdatedAt = time.Now()
			return
		}
	}

	// Add new task
	i.Tasks = append(i.Tasks, TaskRef{
		ID:        id,
		Title:     title,
		DependsOn: dependsOn,
		Status:    "pending",
	})
	i.UpdatedAt = time.Now()
}

// UpdateTaskStatus updates the status of a task in the initiative.
func (i *Initiative) UpdateTaskStatus(taskID, status string) bool {
	for idx, t := range i.Tasks {
		if t.ID == taskID {
			i.Tasks[idx].Status = status
			i.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// RemoveTask removes a task reference from the initiative.
// Returns true if the task was found and removed.
func (i *Initiative) RemoveTask(taskID string) bool {
	for idx, t := range i.Tasks {
		if t.ID == taskID {
			i.Tasks = append(i.Tasks[:idx], i.Tasks[idx+1:]...)
			i.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// AddDecision records a decision in the initiative.
func (i *Initiative) AddDecision(decision, rationale, by string) {
	id := fmt.Sprintf("DEC-%03d", len(i.Decisions)+1)
	i.Decisions = append(i.Decisions, Decision{
		ID:        id,
		Date:      time.Now(),
		By:        by,
		Decision:  decision,
		Rationale: rationale,
	})
	i.UpdatedAt = time.Now()
}

// GetTaskDependencies returns the dependencies for a specific task.
func (i *Initiative) GetTaskDependencies(taskID string) []string {
	for _, t := range i.Tasks {
		if t.ID == taskID {
			return t.DependsOn
		}
	}
	return nil
}

// GetReadyTasks returns tasks that are pending and have all dependencies completed.
func (i *Initiative) GetReadyTasks() []TaskRef {
	// Build a map of completed tasks
	completed := make(map[string]bool)
	for _, t := range i.Tasks {
		if t.Status == "completed" {
			completed[t.ID] = true
		}
	}

	// Find tasks that are pending and have all deps satisfied
	var ready []TaskRef
	for _, t := range i.Tasks {
		if t.Status != "pending" {
			continue
		}

		allDepsSatisfied := true
		for _, dep := range t.DependsOn {
			if !completed[dep] {
				allDepsSatisfied = false
				break
			}
		}

		if allDepsSatisfied {
			ready = append(ready, t)
		}
	}

	return ready
}

// Activate sets the initiative status to active.
func (i *Initiative) Activate() {
	i.Status = StatusActive
	i.UpdatedAt = time.Now()
}

// Complete sets the initiative status to completed.
func (i *Initiative) Complete() {
	i.Status = StatusCompleted
	i.UpdatedAt = time.Now()
}

// Archive sets the initiative status to archived.
func (i *Initiative) Archive() {
	i.Status = StatusArchived
	i.UpdatedAt = time.Now()
}

// List lists all initiatives in the given directory.
func List(shared bool) ([]*Initiative, error) {
	return ListFrom(GetInitiativesDir(shared))
}

// ListFrom lists all initiatives in a specific directory.
func ListFrom(baseDir string) ([]*Initiative, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read initiatives directory: %w", err)
	}

	var initiatives []*Initiative
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		init, err := LoadFrom(baseDir, entry.Name())
		if err != nil {
			continue // Skip invalid initiatives
		}
		initiatives = append(initiatives, init)
	}

	return initiatives, nil
}

// ListByStatus lists initiatives filtered by status.
func ListByStatus(status Status, shared bool) ([]*Initiative, error) {
	all, err := List(shared)
	if err != nil {
		return nil, err
	}

	var filtered []*Initiative
	for _, init := range all {
		if init.Status == status {
			filtered = append(filtered, init)
		}
	}

	return filtered, nil
}

// Exists returns true if an initiative exists.
func Exists(id string, shared bool) bool {
	path := filepath.Join(GetInitiativesDir(shared), id, "initiative.yaml")
	_, err := os.Stat(path)
	return err == nil
}

// Delete removes an initiative and all its associated files.
func Delete(id string, shared bool) error {
	dir := filepath.Join(GetInitiativesDir(shared), id)
	return os.RemoveAll(dir)
}

// NextID generates the next initiative ID.
func NextID(shared bool) (string, error) {
	initiatives, err := List(shared)
	if err != nil {
		return "", err
	}

	maxNum := 0
	for _, init := range initiatives {
		var num int
		if _, err := fmt.Sscanf(init.ID, "INIT-%d", &num); err == nil {
			if num > maxNum {
				maxNum = num
			}
		}
	}

	return fmt.Sprintf("INIT-%03d", maxNum+1), nil
}
