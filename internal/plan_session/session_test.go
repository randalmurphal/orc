package plan_session

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func createTestBackend(t *testing.T) storage.Backend {
	t.Helper()
	tmpDir := t.TempDir()
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.Close()
	})
	return backend
}

func TestDetectMode(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		setup      func(t *testing.T, backend storage.Backend)
		wantMode   Mode
		wantTarget string
		wantErr    bool
	}{
		{
			name:       "empty target returns interactive mode",
			target:     "",
			setup:      nil,
			wantMode:   ModeInteractive,
			wantTarget: "",
			wantErr:    false,
		},
		{
			name:   "existing task returns task mode",
			target: "TASK-001",
			setup: func(t *testing.T, backend storage.Backend) {
				tsk := task.New("TASK-001", "Test task")
				if err := backend.SaveTask(tsk); err != nil {
					t.Fatalf("failed to save task: %v", err)
				}
			},
			wantMode:   ModeTask,
			wantTarget: "TASK-001",
			wantErr:    false,
		},
		{
			name:       "non-existent task ID pattern returns error",
			target:     "TASK-999",
			setup:      nil,
			wantMode:   "",
			wantTarget: "",
			wantErr:    true,
		},
		{
			name:       "feature title returns feature mode",
			target:     "User Authentication",
			setup:      nil,
			wantMode:   ModeFeature,
			wantTarget: "User Authentication",
			wantErr:    false,
		},
		{
			name:       "lowercase feature title returns feature mode",
			target:     "add dark mode toggle",
			setup:      nil,
			wantMode:   ModeFeature,
			wantTarget: "add dark mode toggle",
			wantErr:    false,
		},
		{
			name:       "task-like but invalid pattern returns feature mode",
			target:     "TASK001",
			setup:      nil,
			wantMode:   ModeFeature,
			wantTarget: "TASK001",
			wantErr:    false,
		},
		{
			name:       "different ID format returns feature mode",
			target:     "FEATURE-001",
			setup:      nil,
			wantMode:   ModeFeature,
			wantTarget: "FEATURE-001",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := createTestBackend(t)

			if tt.setup != nil {
				tt.setup(t, backend)
			}

			mode, target, err := DetectMode(tt.target, backend)

			if tt.wantErr {
				if err == nil {
					t.Error("DetectMode() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("DetectMode() unexpected error: %v", err)
				return
			}

			if mode != tt.wantMode {
				t.Errorf("DetectMode() mode = %q, want %q", mode, tt.wantMode)
			}

			if target != tt.wantTarget {
				t.Errorf("DetectMode() target = %q, want %q", target, tt.wantTarget)
			}
		})
	}
}

func TestDetectMode_MultipleTasksExist(t *testing.T) {
	backend := createTestBackend(t)

	// Create multiple tasks
	for _, id := range []string{"TASK-001", "TASK-002", "TASK-003"} {
		tsk := task.New(id, "Test task "+id)
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}

	// Test that we can detect each one
	for _, id := range []string{"TASK-001", "TASK-002", "TASK-003"} {
		mode, target, err := DetectMode(id, backend)
		if err != nil {
			t.Errorf("DetectMode(%q) unexpected error: %v", id, err)
			continue
		}
		if mode != ModeTask {
			t.Errorf("DetectMode(%q) mode = %q, want %q", id, mode, ModeTask)
		}
		if target != id {
			t.Errorf("DetectMode(%q) target = %q, want %q", id, target, id)
		}
	}

	// Test non-existent task in sequence
	mode, _, err := DetectMode("TASK-004", backend)
	if err == nil {
		t.Error("DetectMode(TASK-004) expected error for non-existent task")
	}
	if mode != "" {
		t.Errorf("DetectMode(TASK-004) mode = %q, want empty", mode)
	}
}

func TestFindFeatureSpecFile(t *testing.T) {
	tests := []struct {
		name         string
		initiativeID string
		setup        func(t *testing.T, tmpDir string) string // Returns expected path
		wantFound    bool
	}{
		{
			name:         "initiative-specific path exists",
			initiativeID: "INIT-001",
			setup: func(t *testing.T, tmpDir string) string {
				specPath := filepath.Join(tmpDir, ".orc", "initiatives", "INIT-001", "spec.md")
				if err := os.MkdirAll(filepath.Dir(specPath), 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				if err := os.WriteFile(specPath, []byte("# Spec"), 0644); err != nil {
					t.Fatalf("failed to write spec: %v", err)
				}
				return specPath
			},
			wantFound: true,
		},
		{
			name:         "shared path exists",
			initiativeID: "INIT-002",
			setup: func(t *testing.T, tmpDir string) string {
				specPath := filepath.Join(tmpDir, ".orc", "shared", "initiatives", "INIT-002", "spec.md")
				if err := os.MkdirAll(filepath.Dir(specPath), 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				if err := os.WriteFile(specPath, []byte("# Shared Spec"), 0644); err != nil {
					t.Fatalf("failed to write spec: %v", err)
				}
				return specPath
			},
			wantFound: true,
		},
		{
			name:         "initiative-specific takes precedence over shared",
			initiativeID: "INIT-003",
			setup: func(t *testing.T, tmpDir string) string {
				// Create both paths
				initPath := filepath.Join(tmpDir, ".orc", "initiatives", "INIT-003", "spec.md")
				sharedPath := filepath.Join(tmpDir, ".orc", "shared", "initiatives", "INIT-003", "spec.md")

				for _, p := range []string{initPath, sharedPath} {
					if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
						t.Fatalf("failed to create dir: %v", err)
					}
					if err := os.WriteFile(p, []byte("# Spec"), 0644); err != nil {
						t.Fatalf("failed to write spec: %v", err)
					}
				}
				// Initiative-specific should be returned
				return initPath
			},
			wantFound: true,
		},
		{
			name:         "default specs dir with multiple md files returns most recent",
			initiativeID: "",
			setup: func(t *testing.T, tmpDir string) string {
				specsDir := filepath.Join(tmpDir, ".orc", "specs")
				if err := os.MkdirAll(specsDir, 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}

				// Create older file
				olderPath := filepath.Join(specsDir, "older-feature.md")
				if err := os.WriteFile(olderPath, []byte("# Older"), 0644); err != nil {
					t.Fatalf("failed to write older spec: %v", err)
				}

				// Ensure time difference
				time.Sleep(10 * time.Millisecond)

				// Create newer file
				newerPath := filepath.Join(specsDir, "newer-feature.md")
				if err := os.WriteFile(newerPath, []byte("# Newer"), 0644); err != nil {
					t.Fatalf("failed to write newer spec: %v", err)
				}

				return newerPath
			},
			wantFound: true,
		},
		{
			name:         "empty specs directory",
			initiativeID: "",
			setup: func(t *testing.T, tmpDir string) string {
				specsDir := filepath.Join(tmpDir, ".orc", "specs")
				if err := os.MkdirAll(specsDir, 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				return ""
			},
			wantFound: false,
		},
		{
			name:         "non-existent specs directory",
			initiativeID: "",
			setup: func(t *testing.T, tmpDir string) string {
				return ""
			},
			wantFound: false,
		},
		{
			name:         "specs dir with only non-md files",
			initiativeID: "",
			setup: func(t *testing.T, tmpDir string) string {
				specsDir := filepath.Join(tmpDir, ".orc", "specs")
				if err := os.MkdirAll(specsDir, 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				// Create non-md files
				if err := os.WriteFile(filepath.Join(specsDir, "notes.txt"), []byte("notes"), 0644); err != nil {
					t.Fatalf("failed to write txt: %v", err)
				}
				if err := os.WriteFile(filepath.Join(specsDir, "config.yaml"), []byte("config: true"), 0644); err != nil {
					t.Fatalf("failed to write yaml: %v", err)
				}
				return ""
			},
			wantFound: false,
		},
		{
			name:         "specs dir with subdirectories containing md files",
			initiativeID: "",
			setup: func(t *testing.T, tmpDir string) string {
				specsDir := filepath.Join(tmpDir, ".orc", "specs")
				subDir := filepath.Join(specsDir, "subdir")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					t.Fatalf("failed to create subdir: %v", err)
				}
				// md file in subdirectory should be ignored
				if err := os.WriteFile(filepath.Join(subDir, "nested.md"), []byte("# Nested"), 0644); err != nil {
					t.Fatalf("failed to write nested md: %v", err)
				}
				return ""
			},
			wantFound: false,
		},
		{
			name:         "initiative ID provided but path does not exist falls back to specs dir",
			initiativeID: "INIT-MISSING",
			setup: func(t *testing.T, tmpDir string) string {
				specsDir := filepath.Join(tmpDir, ".orc", "specs")
				if err := os.MkdirAll(specsDir, 0755); err != nil {
					t.Fatalf("failed to create dir: %v", err)
				}
				specPath := filepath.Join(specsDir, "fallback.md")
				if err := os.WriteFile(specPath, []byte("# Fallback"), 0644); err != nil {
					t.Fatalf("failed to write spec: %v", err)
				}
				return specPath
			},
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			var expectedPath string
			if tt.setup != nil {
				expectedPath = tt.setup(t, tmpDir)
			}

			got := findFeatureSpecFile(tmpDir, tt.initiativeID)

			if tt.wantFound {
				if got == "" {
					t.Error("findFeatureSpecFile() returned empty, expected a path")
					return
				}
				if got != expectedPath {
					t.Errorf("findFeatureSpecFile() = %q, want %q", got, expectedPath)
				}
			} else {
				if got != "" {
					t.Errorf("findFeatureSpecFile() = %q, want empty", got)
				}
			}
		})
	}
}

func TestFindFeatureSpecFile_MultipleFilesModificationOrder(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, ".orc", "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	now := time.Now()

	// Create files with explicit modification times
	files := []struct {
		name    string
		content string
		modTime time.Time
	}{
		{"alpha.md", "# Alpha", now.Add(-2 * time.Hour)},
		{"beta.md", "# Beta", now.Add(-1 * time.Hour)},
		{"gamma.md", "# Gamma", now}, // Most recent
	}

	for _, f := range files {
		path := filepath.Join(specsDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", f.name, err)
		}
		// Explicitly set the modification time
		if err := os.Chtimes(path, f.modTime, f.modTime); err != nil {
			t.Fatalf("failed to set mtime for %s: %v", f.name, err)
		}
	}

	// The last file (gamma.md) should be returned as most recent
	got := findFeatureSpecFile(tmpDir, "")
	expected := filepath.Join(specsDir, "gamma.md")

	if got != expected {
		t.Errorf("findFeatureSpecFile() = %q, want %q", got, expected)
	}
}

func TestModeConstants(t *testing.T) {
	// Verify mode constant values
	tests := []struct {
		mode  Mode
		value string
	}{
		{ModeTask, "task"},
		{ModeFeature, "feature"},
		{ModeInteractive, "interactive"},
	}

	for _, tt := range tests {
		if string(tt.mode) != tt.value {
			t.Errorf("Mode %v = %q, want %q", tt.mode, string(tt.mode), tt.value)
		}
	}
}
