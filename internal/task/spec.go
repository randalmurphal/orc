package task

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// SpecMetadata holds metadata about a task specification.
type SpecMetadata struct {
	// Hash is the SHA-256 hash of the spec content
	Hash string `yaml:"hash"`
	// LastSyncedAt is when the spec was last synced
	LastSyncedAt time.Time `yaml:"last_synced_at"`
	// Source indicates where the spec came from (file, db, generated)
	Source string `yaml:"source,omitempty"`
}

// Spec represents a task specification with content and metadata.
type Spec struct {
	// Content is the markdown content of the spec
	Content string `yaml:"-"`
	// Metadata holds sync metadata
	Metadata SpecMetadata `yaml:"metadata"`
}

// SpecPath returns the path to the spec file for a task.
func SpecPath(taskID string) string {
	return filepath.Join(OrcDir, TasksDir, taskID, "spec.md")
}

// SpecMetadataPath returns the path to the spec metadata file.
func SpecMetadataPath(taskID string) string {
	return filepath.Join(OrcDir, TasksDir, taskID, "spec_meta.yaml")
}

// LoadSpec loads a spec from the task directory.
func LoadSpec(taskID string) (*Spec, error) {
	specPath := SpecPath(taskID)
	content, err := os.ReadFile(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No spec yet
		}
		return nil, fmt.Errorf("read spec: %w", err)
	}

	spec := &Spec{
		Content: string(content),
	}

	// Load metadata if it exists
	metaPath := SpecMetadataPath(taskID)
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		yaml.Unmarshal(metaData, &spec.Metadata)
	}

	return spec, nil
}

// SaveSpec saves a spec to the task directory.
func SaveSpec(taskID, content, source string) error {
	taskDir := filepath.Join(OrcDir, TasksDir, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	// Write spec content
	specPath := SpecPath(taskID)
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}

	// Write metadata
	meta := SpecMetadata{
		Hash:         HashContent(content),
		LastSyncedAt: time.Now(),
		Source:       source,
	}
	metaPath := SpecMetadataPath(taskID)
	metaData, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal spec metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("write spec metadata: %w", err)
	}

	return nil
}

// SpecExists returns true if a spec exists for the task.
func SpecExists(taskID string) bool {
	_, err := os.Stat(SpecPath(taskID))
	return err == nil
}

// HashContent computes SHA-256 hash of content.
func HashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// SpecChanged checks if spec content has changed since last sync.
func SpecChanged(taskID, newContent string) bool {
	spec, err := LoadSpec(taskID)
	if err != nil || spec == nil {
		return true // No existing spec, so it's "changed"
	}

	newHash := HashContent(newContent)
	return spec.Metadata.Hash != newHash
}

// LoadSpecFrom loads a spec from a specific directory.
func LoadSpecFrom(taskDir string) (*Spec, error) {
	specPath := filepath.Join(taskDir, "spec.md")
	content, err := os.ReadFile(specPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read spec: %w", err)
	}

	spec := &Spec{
		Content: string(content),
	}

	metaPath := filepath.Join(taskDir, "spec_meta.yaml")
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		yaml.Unmarshal(metaData, &spec.Metadata)
	}

	return spec, nil
}

// SaveSpecTo saves a spec to a specific directory.
func SaveSpecTo(taskDir, content, source string) error {
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	specPath := filepath.Join(taskDir, "spec.md")
	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}

	meta := SpecMetadata{
		Hash:         HashContent(content),
		LastSyncedAt: time.Now(),
		Source:       source,
	}
	metaPath := filepath.Join(taskDir, "spec_meta.yaml")
	metaData, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal spec metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("write spec metadata: %w", err)
	}

	return nil
}
