// Package testutil provides test utilities for integration and E2E tests.
package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestRepo represents a temporary test repository with orc initialized.
type TestRepo struct {
	t       *testing.T
	RootDir string
	OrcDir  string
}

// SetupTestRepo creates a temporary git repository with orc initialized.
// The repo is cleaned up when the test completes.
func SetupTestRepo(t *testing.T) *TestRepo {
	t.Helper()

	// Create temp directory
	tmpDir := t.TempDir()

	// Initialize git repo with explicit 'main' branch
	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}

	// Configure git user for commits
	for _, cfg := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
	} {
		cmd := exec.Command("git", cfg...)
		cmd.Dir = tmpDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git config failed: %v\n%s", err, output)
		}
	}

	// Create initial commit (required for worktrees)
	readmePath := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Project\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, output)
	}
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, output)
	}

	// Create .orc directory structure
	orcDir := filepath.Join(tmpDir, ".orc")
	dirs := []string{
		orcDir,
		filepath.Join(orcDir, "tasks"),
		filepath.Join(orcDir, "local"),
		filepath.Join(orcDir, "worktrees"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create directory %s: %v", dir, err)
		}
	}

	// Create default config
	config := map[string]interface{}{
		"version": 1,
		"profile": "auto",
		"task_id": map[string]string{
			"mode":          "solo",
			"prefix_source": "initials",
		},
	}
	configPath := filepath.Join(orcDir, "config.yaml")
	WriteYAML(t, configPath, config)

	return &TestRepo{
		t:       t,
		RootDir: tmpDir,
		OrcDir:  orcDir,
	}
}

// InitSharedDir initializes the .orc/shared/ directory for P2P mode.
func (r *TestRepo) InitSharedDir() {
	r.t.Helper()

	sharedDir := filepath.Join(r.OrcDir, "shared")
	dirs := []string{
		sharedDir,
		filepath.Join(sharedDir, "prompts"),
		filepath.Join(sharedDir, "skills"),
		filepath.Join(sharedDir, "templates"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			r.t.Fatalf("create shared directory %s: %v", dir, err)
		}
	}

	// Create shared config for P2P mode
	sharedConfig := map[string]interface{}{
		"version": 1,
		"task_id": map[string]string{
			"mode":          "p2p",
			"prefix_source": "initials",
		},
		"defaults": map[string]string{
			"profile": "safe",
		},
	}
	WriteYAML(r.t, filepath.Join(sharedDir, "config.yaml"), sharedConfig)

	// Create empty team registry
	teamRegistry := map[string]interface{}{
		"version":           1,
		"members":           []interface{}{},
		"reserved_prefixes": []interface{}{},
	}
	WriteYAML(r.t, filepath.Join(sharedDir, "team.yaml"), teamRegistry)
}

// CreateTask creates a task directory with task.yaml.
func (r *TestRepo) CreateTask(taskID, title string) string {
	r.t.Helper()

	taskDir := filepath.Join(r.OrcDir, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		r.t.Fatalf("create task directory: %v", err)
	}

	task := map[string]interface{}{
		"id":     taskID,
		"title":  title,
		"weight": "small",
		"status": "pending",
	}
	WriteYAML(r.t, filepath.Join(taskDir, "task.yaml"), task)

	return taskDir
}

// SetConfig sets a value in the project config.
func (r *TestRepo) SetConfig(key string, value interface{}) {
	r.t.Helper()

	configPath := filepath.Join(r.OrcDir, "config.yaml")
	config := ReadYAML(r.t, configPath)

	// Handle nested keys
	parts := strings.Split(key, ".")
	current := config
	for i, part := range parts[:len(parts)-1] {
		if _, ok := current[part]; !ok {
			current[part] = make(map[string]interface{})
		}
		var ok bool
		current, ok = current[part].(map[string]interface{})
		if !ok {
			r.t.Fatalf("config path %s is not a map at %s", key, strings.Join(parts[:i+1], "."))
		}
	}
	current[parts[len(parts)-1]] = value

	WriteYAML(r.t, configPath, config)
}

// MockUserConfig creates a mock user config in a temp directory and returns the path.
// Returns the path to the temp user home directory (parent of .orc).
func MockUserConfig(t *testing.T, initials string) string {
	t.Helper()

	// Create temp user home
	userHome := t.TempDir()
	userOrcDir := filepath.Join(userHome, ".orc")
	if err := os.MkdirAll(userOrcDir, 0755); err != nil {
		t.Fatalf("create user .orc: %v", err)
	}

	config := map[string]interface{}{
		"identity": map[string]string{
			"initials":     initials,
			"display_name": "Test User " + initials,
		},
	}
	WriteYAML(t, filepath.Join(userOrcDir, "config.yaml"), config)

	return userHome
}

// WriteYAML writes a YAML file.
func WriteYAML(t *testing.T, path string, data interface{}) {
	t.Helper()

	bytes, err := yaml.Marshal(data)
	if err != nil {
		t.Fatalf("marshal YAML: %v", err)
	}

	if err := os.WriteFile(path, bytes, 0644); err != nil {
		t.Fatalf("write YAML file %s: %v", path, err)
	}
}

// ReadYAML reads a YAML file into a map.
func ReadYAML(t *testing.T, path string) map[string]interface{} {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read YAML file %s: %v", path, err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal YAML: %v", err)
	}

	if result == nil {
		result = make(map[string]interface{})
	}

	return result
}

// AssertBranchExists checks that a git branch exists.
func AssertBranchExists(t *testing.T, repoDir, branchName string) {
	t.Helper()

	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Errorf("branch %s does not exist", branchName)
	}
}

// AssertWorktreeExists checks that a worktree directory exists.
func AssertWorktreeExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Errorf("worktree %s does not exist", path)
		} else {
			t.Errorf("stat worktree %s: %v", path, err)
		}
		return
	}

	if !info.IsDir() {
		t.Errorf("worktree %s is not a directory", path)
	}
}

// AssertWorktreeNotExists checks that a worktree directory does not exist.
func AssertWorktreeNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Errorf("worktree %s exists but should not", path)
	} else if !os.IsNotExist(err) {
		t.Errorf("stat worktree %s: %v", path, err)
	}
}

// AssertFileExists checks that a file exists.
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("file %s does not exist", path)
		} else {
			t.Errorf("stat file %s: %v", path, err)
		}
	}
}

// AssertFileNotExists checks that a file does not exist.
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err == nil {
		t.Errorf("file %s exists but should not", path)
	} else if !os.IsNotExist(err) {
		t.Errorf("stat file %s: %v", path, err)
	}
}

// AssertFileContains checks that a file contains a specific string.
func AssertFileContains(t *testing.T, path, content string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read file %s: %v", path, err)
		return
	}

	if !strings.Contains(string(data), content) {
		t.Errorf("file %s does not contain %q\ncontents: %s", path, content, string(data))
	}
}

// CreateBranch creates a git branch in the repo.
func CreateBranch(t *testing.T, repoDir, branchName string) {
	t.Helper()

	cmd := exec.Command("git", "branch", branchName)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create branch %s: %v\n%s", branchName, err, output)
	}
}
