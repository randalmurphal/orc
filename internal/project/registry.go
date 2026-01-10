// Package project provides global project registry management.
package project

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// GlobalDir is the global orc configuration directory.
	GlobalDir = ".orc"
	// RegistryFile is the projects registry file name.
	RegistryFile = "projects.yaml"
)

// Project represents a registered orc project.
type Project struct {
	ID        string    `yaml:"id" json:"id"`
	Name      string    `yaml:"name" json:"name"`
	Path      string    `yaml:"path" json:"path"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

// Registry holds all registered projects.
type Registry struct {
	Projects []Project `yaml:"projects" json:"projects"`
}

// GlobalPath returns the path to the global orc directory.
func GlobalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, GlobalDir), nil
}

// RegistryPath returns the path to the global projects registry.
func RegistryPath() (string, error) {
	globalDir, err := GlobalPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(globalDir, RegistryFile), nil
}

// LoadRegistry loads the global project registry.
func LoadRegistry() (*Registry, error) {
	path, err := RegistryPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{Projects: []Project{}}, nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}

	var reg Registry
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}

	return &reg, nil
}

// Save saves the registry to disk.
func (r *Registry) Save() error {
	path, err := RegistryPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create global directory: %w", err)
	}

	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}

	return nil
}

// Register adds or updates a project in the registry.
func (r *Registry) Register(path string) (*Project, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("path not found: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Check if already registered
	for i, p := range r.Projects {
		if p.Path == absPath {
			// Update existing
			r.Projects[i].Name = filepath.Base(absPath)
			return &r.Projects[i], nil
		}
	}

	// Generate ID from path
	id := generateID(absPath)

	// Create new project
	proj := Project{
		ID:        id,
		Name:      filepath.Base(absPath),
		Path:      absPath,
		CreatedAt: time.Now(),
	}

	r.Projects = append(r.Projects, proj)
	return &proj, nil
}

// Unregister removes a project from the registry.
func (r *Registry) Unregister(idOrPath string) error {
	for i, p := range r.Projects {
		if p.ID == idOrPath || p.Path == idOrPath {
			r.Projects = append(r.Projects[:i], r.Projects[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("project not found: %s", idOrPath)
}

// Get returns a project by ID or path.
func (r *Registry) Get(idOrPath string) (*Project, error) {
	for _, p := range r.Projects {
		if p.ID == idOrPath || p.Path == idOrPath {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("project not found: %s", idOrPath)
}

// List returns all registered projects.
func (r *Registry) List() []Project {
	return r.Projects
}

// ValidProjects returns projects whose paths still exist.
func (r *Registry) ValidProjects() []Project {
	var valid []Project
	for _, p := range r.Projects {
		if _, err := os.Stat(p.Path); err == nil {
			valid = append(valid, p)
		}
	}
	return valid
}

// generateID creates a short unique ID from a path.
func generateID(path string) string {
	hash := sha256.Sum256([]byte(path))
	return hex.EncodeToString(hash[:])[:8]
}

// RegisterProject is a convenience function to register a project path.
func RegisterProject(path string) (*Project, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}

	proj, err := reg.Register(path)
	if err != nil {
		return nil, err
	}

	if err := reg.Save(); err != nil {
		return nil, err
	}

	return proj, nil
}

// ListProjects is a convenience function to list all projects.
func ListProjects() ([]Project, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	return reg.ValidProjects(), nil
}
