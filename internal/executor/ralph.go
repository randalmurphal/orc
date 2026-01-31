// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// RalphStateDir is the directory within worktree where ralph state is stored.
	RalphStateDir = ".claude"

	// RalphStateFile is the filename for ralph loop state.
	RalphStateFile = "orc-ralph.local.md"

	// DefaultMaxIterations is the default max iterations for ralph loop.
	DefaultMaxIterations = 30

	// DefaultCompletionPromise is the default completion promise text.
	DefaultCompletionPromise = "PHASE_COMPLETE"
)

// RalphState represents the state of a ralph-style iteration loop.
type RalphState struct {
	TaskID            string    `yaml:"task_id"`
	Phase             string    `yaml:"phase"`
	Iteration         int       `yaml:"iteration"`
	MaxIterations     int       `yaml:"max_iterations"`
	CompletionPromise string    `yaml:"completion_promise"`
	SessionID         string    `yaml:"session_id,omitempty"`
	StartedAt         time.Time `yaml:"started_at"`
}

// RalphStateManager handles ralph state file operations.
type RalphStateManager struct {
	worktreePath string
}

// NewRalphStateManager creates a new ralph state manager for a worktree.
func NewRalphStateManager(worktreePath string) *RalphStateManager {
	return &RalphStateManager{
		worktreePath: worktreePath,
	}
}

// statePath returns the full path to the ralph state file.
func (m *RalphStateManager) statePath() string {
	return filepath.Join(m.worktreePath, RalphStateDir, RalphStateFile)
}

// Create creates a new ralph state file with the given prompt.
func (m *RalphStateManager) Create(taskID, phase, prompt string, opts ...RalphOption) error {
	state := &RalphState{
		TaskID:            taskID,
		Phase:             phase,
		Iteration:         1,
		MaxIterations:     DefaultMaxIterations,
		CompletionPromise: DefaultCompletionPromise,
		StartedAt:         time.Now(),
	}

	// Apply options
	for _, opt := range opts {
		opt(state)
	}

	return m.write(state, prompt)
}

// Load loads the ralph state from disk.
func (m *RalphStateManager) Load() (*RalphState, string, error) {
	path := m.statePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil // No active loop
		}
		return nil, "", fmt.Errorf("read ralph state: %w", err)
	}

	content := string(data)
	state, prompt, err := parseRalphFile(content)
	if err != nil {
		return nil, "", fmt.Errorf("parse ralph state: %w", err)
	}

	return state, prompt, nil
}

// Exists checks if a ralph state file exists.
func (m *RalphStateManager) Exists() bool {
	_, err := os.Stat(m.statePath())
	return err == nil
}

// IncrementIteration increments the iteration count and updates the file.
func (m *RalphStateManager) IncrementIteration() error {
	state, prompt, err := m.Load()
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("no ralph state file found")
	}

	state.Iteration++
	return m.write(state, prompt)
}

// UpdateSessionID updates the session ID in the state file.
func (m *RalphStateManager) UpdateSessionID(sessionID string) error {
	state, prompt, err := m.Load()
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("no ralph state file found")
	}

	state.SessionID = sessionID
	return m.write(state, prompt)
}

// Remove removes the ralph state file (called on completion).
func (m *RalphStateManager) Remove() error {
	path := m.statePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove ralph state: %w", err)
	}
	return nil
}

// write writes the ralph state and prompt to the file.
func (m *RalphStateManager) write(state *RalphState, prompt string) error {
	// Ensure directory exists
	dir := filepath.Join(m.worktreePath, RalphStateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create ralph state dir: %w", err)
	}

	// Marshal frontmatter
	frontmatter, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal ralph state: %w", err)
	}

	// Build file content
	content := fmt.Sprintf("---\n%s---\n\n%s", string(frontmatter), prompt)

	path := m.statePath()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write ralph state: %w", err)
	}

	return nil
}

// parseRalphFile parses a ralph state file into state and prompt.
func parseRalphFile(content string) (*RalphState, string, error) {
	// Find frontmatter delimiters
	if !strings.HasPrefix(content, "---\n") {
		return nil, "", fmt.Errorf("invalid ralph file: missing frontmatter")
	}

	// Find end of frontmatter
	endIdx := strings.Index(content[4:], "\n---\n")
	if endIdx == -1 {
		return nil, "", fmt.Errorf("invalid ralph file: unclosed frontmatter")
	}

	frontmatter := content[4 : 4+endIdx]
	prompt := strings.TrimPrefix(content[4+endIdx+5:], "\n")

	var state RalphState
	if err := yaml.Unmarshal([]byte(frontmatter), &state); err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}

	return &state, prompt, nil
}

// RalphOption configures ralph state creation.
type RalphOption func(*RalphState)

// WithMaxIterations sets the max iterations for the loop.
func WithMaxIterations(n int) RalphOption {
	return func(s *RalphState) {
		s.MaxIterations = n
	}
}

// WithCompletionPromise sets the completion promise text.
func WithCompletionPromise(promise string) RalphOption {
	return func(s *RalphState) {
		s.CompletionPromise = promise
	}
}

// WithSessionID sets the initial session ID.
func WithSessionID(id string) RalphOption {
	return func(s *RalphState) {
		s.SessionID = id
	}
}

// IsOrcWorktree checks if the given path is within an orc worktree.
// This is used by the stop hook to determine if it should apply ralph logic.
// Supports both new global location (~/.orc/worktrees/*/orc-*) and legacy
// project-local location (.orc/worktrees/orc-*).
func IsOrcWorktree(path string) bool {
	// Check new global location: ~/.orc/worktrees/<project-id>/orc-*
	if homeDir, err := os.UserHomeDir(); err == nil {
		globalPattern := filepath.Join(homeDir, ".orc", "worktrees")
		if strings.HasPrefix(path, globalPattern) && strings.Contains(path, "/orc-") {
			return true
		}
	}
	// Legacy: check .orc/worktrees/orc-
	return strings.Contains(path, ".orc/worktrees/orc-")
}

// ExtractTaskIDFromWorktree extracts the task ID from a worktree path.
// Returns empty string if path is not an orc worktree.
// Supports both new global location (~/.orc/worktrees/<project-id>/orc-TASK-XXX)
// and legacy project-local location (.orc/worktrees/orc-TASK-XXX).
func ExtractTaskIDFromWorktree(path string) string {
	// Check new global location: ~/.orc/worktrees/<project-id>/orc-TASK-XXX
	if homeDir, err := os.UserHomeDir(); err == nil {
		globalPattern := filepath.Join(homeDir, ".orc", "worktrees")
		if strings.HasPrefix(path, globalPattern) {
			// Find /orc- in the path after the global prefix
			orcIdx := strings.Index(path, "/orc-")
			if orcIdx != -1 {
				start := orcIdx + len("/orc-")
				rest := path[start:]
				endIdx := strings.Index(rest, "/")
				if endIdx == -1 {
					return rest
				}
				return rest[:endIdx]
			}
		}
	}

	// Legacy: look for pattern .orc/worktrees/orc-TASK-XXX
	idx := strings.Index(path, ".orc/worktrees/orc-")
	if idx == -1 {
		return ""
	}

	start := idx + len(".orc/worktrees/orc-")
	rest := path[start:]

	endIdx := strings.Index(rest, "/")
	if endIdx == -1 {
		return rest
	}
	return rest[:endIdx]
}
