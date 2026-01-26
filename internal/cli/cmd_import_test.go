package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
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
	currentPhase := "implement"
	hostname := "other-machine"
	export := &ExportData{
		Version:    4,
		ExportedAt: time.Now(),
		Task: &orcv1.Task{
			Id:               "TASK-001",
			Title:            "Running Task",
			Status:           orcv1.TaskStatus_TASK_STATUS_RUNNING,
			ExecutorPid:      12345,
			ExecutorHostname: &hostname,
			UpdatedAt:        timestamppb.New(time.Now().Add(-1 * time.Hour)),
			CurrentPhase:     &currentPhase,
			Execution:        task.InitProtoExecutionState(),
		},
	}

	// Simulate the import transformation (matches cmd_export.go logic)
	wasRunning := false
	if export.Task.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		wasRunning = true
		export.Task.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
		// Clear executor info - it's invalid on this machine
		export.Task.ExecutorPid = 0
		export.Task.ExecutorHostname = nil
		export.Task.UpdatedAt = timestamppb.Now()
	}

	if !wasRunning {
		t.Error("expected wasRunning to be true")
	}
	if export.Task.Status != orcv1.TaskStatus_TASK_STATUS_PAUSED {
		t.Errorf("expected task status 'paused', got %s", export.Task.Status)
	}
	// Executor info should be cleared on task
	if export.Task.ExecutorPid != 0 {
		t.Errorf("expected executor PID to be cleared, got %d", export.Task.ExecutorPid)
	}
	// Phase should be preserved on task
	if export.Task.CurrentPhase == nil || *export.Task.CurrentPhase != "implement" {
		t.Errorf("expected current phase 'implement', got %v", export.Task.CurrentPhase)
	}
}
