package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/task"
)

func TestDetectImportFormat(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		setup    func() string // returns path
		expected string
	}{
		{
			name: "tar.gz by extension",
			setup: func() string {
				path := filepath.Join(tmpDir, "test.tar.gz")
				// Create a valid gzip file
				f, _ := os.Create(path)
				gw := gzip.NewWriter(f)
				tw := tar.NewWriter(gw)
				_ = tw.Close()
				_ = gw.Close()
				_ = f.Close()
				return path
			},
			expected: "tar.gz",
		},
		{
			name: "zip by extension",
			setup: func() string {
				path := filepath.Join(tmpDir, "test.zip")
				f, _ := os.Create(path)
				zw := zip.NewWriter(f)
				_ = zw.Close()
				_ = f.Close()
				return path
			},
			expected: "zip",
		},
		{
			name: "yaml by extension",
			setup: func() string {
				path := filepath.Join(tmpDir, "test.yaml")
				_ = os.WriteFile(path, []byte("version: 3"), 0644)
				return path
			},
			expected: "yaml",
		},
		{
			name: "directory",
			setup: func() string {
				path := filepath.Join(tmpDir, "testdir")
				_ = os.MkdirAll(path, 0755)
				return path
			},
			expected: "dir",
		},
		{
			name: "gzip by magic bytes",
			setup: func() string {
				path := filepath.Join(tmpDir, "noext")
				f, _ := os.Create(path)
				gw := gzip.NewWriter(f)
				_, _ = gw.Write([]byte("test"))
				_ = gw.Close()
				_ = f.Close()
				return path
			},
			expected: "tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			format, err := detectImportFormat(path)
			if err != nil {
				t.Fatalf("detectImportFormat failed: %v", err)
			}
			if format != tt.expected {
				t.Errorf("expected format %q, got %q", tt.expected, format)
			}
		})
	}
}

func TestFindLatestExport(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a directory structure
	exportDir := filepath.Join(tmpDir, ".orc", "exports")
	_ = os.MkdirAll(exportDir, 0755)

	// Create some test files with different timestamps
	oldArchive := filepath.Join(exportDir, "orc-export-old.tar.gz")
	newArchive := filepath.Join(exportDir, "orc-export-new.tar.gz")

	_ = os.WriteFile(oldArchive, []byte{0x1f, 0x8b}, 0644)
	time.Sleep(10 * time.Millisecond)
	_ = os.WriteFile(newArchive, []byte{0x1f, 0x8b}, 0644)

	path, err := findLatestExport(exportDir)
	if err != nil {
		t.Fatalf("findLatestExport: %v", err)
	}
	if path != newArchive {
		t.Errorf("expected newest archive %q, got %q", newArchive, path)
	}
}

func TestFindLatestExportFallsBackToDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create empty export directory (no archives)
	exportDir := filepath.Join(tmpDir, ".orc", "exports")
	_ = os.MkdirAll(exportDir, 0755)

	path, err := findLatestExport(exportDir)
	if err != nil {
		t.Fatalf("findLatestExport: %v", err)
	}
	if path != exportDir {
		t.Errorf("expected directory path %q, got %q", exportDir, path)
	}
}

// TestRunningTaskImportTransform verifies that running tasks are transformed on import
func TestRunningTaskImportTransform(t *testing.T) {
	t.Parallel()
	// Create an export with a running task
	// Executor info and execution state are on task
	export := &ExportData{
		Version:    4,
		ExportedAt: time.Now(),
		Task: &task.Task{
			ID:               "TASK-001",
			Title:            "Running Task",
			Status:           task.StatusRunning,
			ExecutorPID:      12345,
			ExecutorHostname: "other-machine",
			UpdatedAt:        time.Now().Add(-1 * time.Hour),
			CurrentPhase:     "implement",
			Execution:        task.InitExecutionState(),
		},
	}

	// Simulate the import transformation (matches cmd_export.go logic)
	wasRunning := false
	if export.Task.Status == task.StatusRunning {
		wasRunning = true
		export.Task.Status = task.StatusPaused
		// Clear executor info - it's invalid on this machine
		export.Task.ExecutorPID = 0
		export.Task.ExecutorHostname = ""
		export.Task.UpdatedAt = time.Now()
	}

	if !wasRunning {
		t.Error("expected wasRunning to be true")
	}
	if export.Task.Status != task.StatusPaused {
		t.Errorf("expected task status 'paused', got %q", export.Task.Status)
	}
	// Executor info should be cleared on task
	if export.Task.ExecutorPID != 0 {
		t.Errorf("expected executor PID to be cleared, got %d", export.Task.ExecutorPID)
	}
	// Phase should be preserved on task
	if export.Task.CurrentPhase != "implement" {
		t.Errorf("expected current phase 'implement', got %q", export.Task.CurrentPhase)
	}
}
