// internal/api/project_cache_test.go
package api

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/project"
)

func TestProjectCache_GetOpensDatabase(t *testing.T) {
	// Setup: create a temp project with initialized .orc directory
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.InitAt(projectPath, false); err != nil {
		t.Fatal(err)
	}

	// Register the project
	proj, err := project.RegisterProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}

	// Create cache
	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Get should open the database
	pdb, err := cache.Get(proj.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if pdb == nil {
		t.Fatal("expected non-nil database")
	}

	// Second get should return cached instance
	pdb2, err := cache.Get(proj.ID)
	if err != nil {
		t.Fatalf("second Get failed: %v", err)
	}
	if pdb != pdb2 {
		t.Error("expected same database instance from cache")
	}
}

func TestProjectCache_LRUEviction(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 3 projects
	var projectIDs []string
	for i := 0; i < 3; i++ {
		projectPath := filepath.Join(tmpDir, fmt.Sprintf("project-%d", i))
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			t.Fatal(err)
		}
		if err := config.InitAt(projectPath, false); err != nil {
			t.Fatal(err)
		}
		proj, err := project.RegisterProject(projectPath)
		if err != nil {
			t.Fatal(err)
		}
		projectIDs = append(projectIDs, proj.ID)
	}

	// Cache with max size 2
	cache := NewProjectCache(2)
	defer func() { _ = cache.Close() }()

	// Access projects 0 and 1
	_, _ = cache.Get(projectIDs[0])
	_, _ = cache.Get(projectIDs[1])

	// Access project 2 - should evict project 0 (LRU)
	_, _ = cache.Get(projectIDs[2])

	// Project 0 should be evicted
	if cache.Contains(projectIDs[0]) {
		t.Error("project 0 should have been evicted")
	}
	if !cache.Contains(projectIDs[1]) {
		t.Error("project 1 should still be cached")
	}
	if !cache.Contains(projectIDs[2]) {
		t.Error("project 2 should be cached")
	}
}

func TestProjectCache_GetProjectPath(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.InitAt(projectPath, false); err != nil {
		t.Fatal(err)
	}

	proj, err := project.RegisterProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	// Get path without opening database (from registry)
	path, err := cache.GetProjectPath(proj.ID)
	if err != nil {
		t.Fatalf("GetProjectPath failed: %v", err)
	}
	if path != projectPath {
		t.Errorf("expected path %s, got %s", projectPath, path)
	}

	// Open database, then get path (from cache)
	_, _ = cache.Get(proj.ID)
	path2, err := cache.GetProjectPath(proj.ID)
	if err != nil {
		t.Fatalf("GetProjectPath (cached) failed: %v", err)
	}
	if path2 != projectPath {
		t.Errorf("expected cached path %s, got %s", projectPath, path2)
	}
}

func TestProjectCache_Close(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := config.InitAt(projectPath, false); err != nil {
		t.Fatal(err)
	}

	proj, err := project.RegisterProject(projectPath)
	if err != nil {
		t.Fatal(err)
	}

	cache := NewProjectCache(10)

	// Open a database
	_, err = cache.Get(proj.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Close should clean up
	if err := cache.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Cache should be empty
	if cache.Contains(proj.ID) {
		t.Error("cache should be empty after Close")
	}
}
