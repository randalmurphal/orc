// Package task provides task management for orc.
package task

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/randalmurphal/orc/internal/project"
	"github.com/randalmurphal/orc/internal/util"
	"gopkg.in/yaml.v3"
)

// Mode represents the coordination mode for task ID generation.
type Mode string

const (
	// ModeSolo is the default mode with no prefix (single user).
	ModeSolo Mode = "solo"
	// ModeP2P uses prefixed IDs for multi-user coordination.
	ModeP2P Mode = "p2p"
	// ModeTeam uses server-based coordination with prefixed IDs.
	ModeTeam Mode = "team"
)

// PrefixSource determines how the task ID prefix is derived.
type PrefixSource string

const (
	// PrefixNone generates IDs without a prefix (TASK-001).
	PrefixNone PrefixSource = "none"
	// PrefixInitials uses the user's configured initials (TASK-AM-001).
	PrefixInitials PrefixSource = "initials"
	// PrefixUsername uses the system username (TASK-alice-001).
	PrefixUsername PrefixSource = "username"
	// PrefixEmailHash uses first 4 chars of email hash (TASK-a1b2-001).
	PrefixEmailHash PrefixSource = "email_hash"
	// PrefixMachine uses the machine hostname (TASK-laptop-001).
	PrefixMachine PrefixSource = "machine"
)

// SequenceStore manages per-prefix sequence numbers.
type SequenceStore struct {
	path string
	mu   sync.Mutex
}

// SequenceData represents the sequences.yaml file structure.
type SequenceData struct {
	Prefixes map[string]int `yaml:"prefixes"`
}

// NewSequenceStore creates a new sequence store at the given path.
func NewSequenceStore(path string) *SequenceStore {
	return &SequenceStore{path: path}
}

// load reads the current sequence data from disk.
func (s *SequenceStore) load() (*SequenceData, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SequenceData{Prefixes: make(map[string]int)}, nil
		}
		return nil, fmt.Errorf("read sequences: %w", err)
	}

	var sd SequenceData
	if err := yaml.Unmarshal(data, &sd); err != nil {
		return nil, fmt.Errorf("parse sequences: %w", err)
	}
	if sd.Prefixes == nil {
		sd.Prefixes = make(map[string]int)
	}
	return &sd, nil
}

// save persists the sequence data to disk.
func (s *SequenceStore) save(sd *SequenceData) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("create sequences directory: %w", err)
	}

	data, err := yaml.Marshal(sd)
	if err != nil {
		return fmt.Errorf("marshal sequences: %w", err)
	}

	if err := util.AtomicWriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("write sequences: %w", err)
	}
	return nil
}

// NextSequence returns the next sequence number for the given prefix.
// The prefix is normalized to uppercase. An empty prefix uses "_solo" internally.
func (s *SequenceStore) NextSequence(prefix string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Normalize prefix
	key := strings.ToUpper(prefix)
	if key == "" {
		key = "_solo"
	}

	sd, err := s.load()
	if err != nil {
		return 0, err
	}

	// Increment and save
	current := sd.Prefixes[key]
	next := current + 1
	sd.Prefixes[key] = next

	if err := s.save(sd); err != nil {
		return 0, err
	}

	return next, nil
}

// GetSequence returns the current sequence number for the given prefix
// without incrementing it.
func (s *SequenceStore) GetSequence(prefix string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.ToUpper(prefix)
	if key == "" {
		key = "_solo"
	}

	sd, err := s.load()
	if err != nil {
		return 0, err
	}

	return sd.Prefixes[key], nil
}

// SetSequence sets the sequence number for the given prefix to a specific value.
// This is used for catch-up when existing tasks exceed the stored sequence.
func (s *SequenceStore) SetSequence(prefix string, value int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := strings.ToUpper(prefix)
	if key == "" {
		key = "_solo"
	}

	sd, err := s.load()
	if err != nil {
		return err
	}

	sd.Prefixes[key] = value

	return s.save(sd)
}

// IdentityConfig holds user identity settings for prefix generation.
type IdentityConfig struct {
	// Initials for PrefixInitials mode (e.g., "AM")
	Initials string `yaml:"initials"`
	// Email for PrefixEmailHash mode
	Email string `yaml:"email"`
	// DisplayName for team visibility
	DisplayName string `yaml:"display_name"`
}

// TaskIDConfig holds task ID generation configuration.
type TaskIDConfig struct {
	// Mode is the coordination mode (solo, p2p, team)
	Mode Mode `yaml:"mode"`
	// PrefixSource determines how the prefix is derived
	PrefixSource PrefixSource `yaml:"prefix_source"`
}

// TaskIDGenerator generates task IDs with optional prefixes.
type TaskIDGenerator struct {
	mode         Mode
	prefix       string
	store        *SequenceStore
}

// GeneratorOption configures the TaskIDGenerator.
type GeneratorOption func(*TaskIDGenerator)

// WithSequenceStore sets the sequence store for persisting sequence numbers.
func WithSequenceStore(store *SequenceStore) GeneratorOption {
	return func(g *TaskIDGenerator) {
		g.store = store
	}
}

// NewTaskIDGenerator creates a new generator with the specified mode and prefix.
// For solo mode, pass an empty prefix.
//
// Note: In solo mode, the prefix parameter is ignored and IDs are always
// generated as TASK-NNN (no prefix). This is intentional per spec - solo mode
// is for single-user workflows where prefixes add no value.
func NewTaskIDGenerator(mode Mode, prefix string, opts ...GeneratorOption) *TaskIDGenerator {
	g := &TaskIDGenerator{
		mode:   mode,
		prefix: strings.ToUpper(prefix),
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// ResolvePrefix determines the prefix based on the prefix source and identity.
func ResolvePrefix(source PrefixSource, identity *IdentityConfig) (string, error) {
	switch source {
	case PrefixNone:
		return "", nil

	case PrefixInitials:
		if identity == nil || identity.Initials == "" {
			return "", fmt.Errorf("prefix_source 'initials' requires identity.initials to be configured")
		}
		return strings.ToUpper(identity.Initials), nil

	case PrefixUsername:
		u, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("get current user: %w", err)
		}
		return strings.ToLower(u.Username), nil

	case PrefixEmailHash:
		if identity == nil || identity.Email == "" {
			return "", fmt.Errorf("prefix_source 'email_hash' requires identity.email to be configured")
		}
		hash := sha256.Sum256([]byte(strings.ToLower(identity.Email)))
		return fmt.Sprintf("%x", hash[:2]), nil // First 4 hex chars

	case PrefixMachine:
		hostname, err := os.Hostname()
		if err != nil {
			return "", fmt.Errorf("get hostname: %w", err)
		}
		// Truncate long hostnames and clean up
		name := strings.ToLower(hostname)
		name = strings.Split(name, ".")[0] // Remove domain
		if len(name) > 12 {
			name = name[:12]
		}
		return name, nil

	default:
		return "", fmt.Errorf("unknown prefix source: %s", source)
	}
}

// Next generates the next task ID.
// For solo mode: TASK-001
// For p2p/team mode with prefix: TASK-AM-001
func (g *TaskIDGenerator) Next() (string, error) {
	if g.store == nil {
		return "", fmt.Errorf("sequence store required for ID generation")
	}

	// Get next sequence from store
	seq, err := g.store.NextSequence(g.prefix)
	if err != nil {
		return "", fmt.Errorf("get next sequence: %w", err)
	}

	return g.formatID(seq), nil
}

// formatID formats a task ID with the given sequence number.
// In solo mode, the prefix is ignored and IDs are always TASK-NNN.
func (g *TaskIDGenerator) formatID(seq int) string {
	if g.prefix == "" || g.mode == ModeSolo {
		return fmt.Sprintf("TASK-%03d", seq)
	}
	return fmt.Sprintf("TASK-%s-%03d", g.prefix, seq)
}

// Prefix returns the configured prefix (empty for solo mode).
func (g *TaskIDGenerator) Prefix() string {
	return g.prefix
}

// Mode returns the configured mode.
func (g *TaskIDGenerator) Mode() Mode {
	return g.mode
}

// ParseTaskID extracts the prefix and sequence from a task ID.
// Returns prefix (empty for solo), sequence number, and ok=true if valid.
func ParseTaskID(id string) (prefix string, seq int, ok bool) {
	// Try prefixed format first: TASK-AM-001
	prefixedPattern := regexp.MustCompile(`^TASK-([A-Za-z0-9]+)-(\d+)$`)
	if matches := prefixedPattern.FindStringSubmatch(id); len(matches) == 3 {
		num, err := strconv.Atoi(matches[2])
		if err != nil {
			return "", 0, false
		}
		return strings.ToUpper(matches[1]), num, true
	}

	// Try solo format: TASK-001
	soloPattern := regexp.MustCompile(`^TASK-(\d+)$`)
	if matches := soloPattern.FindStringSubmatch(id); len(matches) == 2 {
		num, err := strconv.Atoi(matches[1])
		if err != nil {
			return "", 0, false
		}
		return "", num, true
	}

	return "", 0, false
}

// DefaultSequencePath returns the path for the sequences file.
// If projectID is provided, returns ~/.orc/projects/<id>/sequences.yaml.
// Falls back to .orc/local/sequences.yaml for unregistered projects.
func DefaultSequencePath(projectID string) string {
	if projectID != "" {
		seqPath, err := project.ProjectSequencesPath(projectID)
		if err == nil {
			return seqPath
		}
	}
	return filepath.Join(OrcDir, "local", "sequences.yaml")
}
