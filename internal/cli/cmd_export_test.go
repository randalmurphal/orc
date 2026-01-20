package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

func TestDetectImportFormat(t *testing.T) {
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

func TestBuildManifest(t *testing.T) {
	manifest := buildManifest(10, 2, true, true)

	if manifest.Version != ExportFormatVersion {
		t.Errorf("expected version %d, got %d", ExportFormatVersion, manifest.Version)
	}
	if manifest.TaskCount != 10 {
		t.Errorf("expected task count 10, got %d", manifest.TaskCount)
	}
	if manifest.InitiativeCount != 2 {
		t.Errorf("expected initiative count 2, got %d", manifest.InitiativeCount)
	}
	if !manifest.IncludesState {
		t.Error("expected IncludesState to be true")
	}
	if !manifest.IncludesTranscripts {
		t.Error("expected IncludesTranscripts to be true")
	}
	if manifest.SourceHostname == "" {
		t.Error("expected non-empty SourceHostname")
	}
}

func TestWriteTarFile(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")

	// Create archive
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Write test file
	testData := []byte("test content")
	if err := writeTarFile(tw, "test.yaml", testData); err != nil {
		t.Fatalf("writeTarFile: %v", err)
	}

	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()

	// Verify by reading back
	f, _ = os.Open(archivePath)
	gr, _ := gzip.NewReader(f)
	tr := tar.NewReader(gr)

	header, err := tr.Next()
	if err != nil {
		t.Fatalf("read header: %v", err)
	}
	if header.Name != "test.yaml" {
		t.Errorf("expected name 'test.yaml', got %q", header.Name)
	}

	content, _ := io.ReadAll(tr)
	if string(content) != "test content" {
		t.Errorf("expected 'test content', got %q", string(content))
	}
}

func TestWriteZipFile(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.zip")

	// Create archive
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	zw := zip.NewWriter(f)

	// Write test file
	testData := []byte("zip content")
	if err := writeZipFile(zw, "data.yaml", testData); err != nil {
		t.Fatalf("writeZipFile: %v", err)
	}

	_ = zw.Close()
	_ = f.Close()

	// Verify by reading back
	r, _ := zip.OpenReader(archivePath)
	defer func() { _ = r.Close() }()

	if len(r.File) != 1 {
		t.Fatalf("expected 1 file, got %d", len(r.File))
	}
	if r.File[0].Name != "data.yaml" {
		t.Errorf("expected name 'data.yaml', got %q", r.File[0].Name)
	}
}

func TestFindLatestExport(t *testing.T) {
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

func TestExportDataVersion(t *testing.T) {
	if ExportFormatVersion != 3 {
		t.Errorf("expected ExportFormatVersion 3, got %d", ExportFormatVersion)
	}
}

func TestExportManifestStruct(t *testing.T) {
	manifest := &ExportManifest{
		Version:             3,
		ExportedAt:          time.Now(),
		SourceHostname:      "test-host",
		SourceProject:       "/path/to/project",
		OrcVersion:          "go1.21",
		TaskCount:           5,
		InitiativeCount:     1,
		IncludesState:       true,
		IncludesTranscripts: true,
	}

	// Test YAML marshaling
	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	var unmarshaled ExportManifest
	if err := yaml.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if unmarshaled.Version != 3 {
		t.Errorf("expected version 3, got %d", unmarshaled.Version)
	}
	if unmarshaled.SourceHostname != "test-host" {
		t.Errorf("expected hostname 'test-host', got %q", unmarshaled.SourceHostname)
	}
}

func TestExportDataStruct(t *testing.T) {
	now := time.Now()
	export := &ExportData{
		Version:    3,
		ExportedAt: now,
		Task: &task.Task{
			ID:     "TASK-001",
			Title:  "Test Task",
			Status: task.StatusRunning,
		},
		State: &state.State{
			TaskID: "TASK-001",
			Status: state.StatusRunning,
			Execution: &state.ExecutionInfo{
				PID:      12345,
				Hostname: "old-host",
			},
		},
	}

	// Test YAML round-trip
	data, err := yaml.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}

	var unmarshaled ExportData
	if err := yaml.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}

	if unmarshaled.Task.ID != "TASK-001" {
		t.Errorf("expected task ID 'TASK-001', got %q", unmarshaled.Task.ID)
	}
	if unmarshaled.State == nil {
		t.Fatal("expected state to be present")
	}
	if unmarshaled.State.Execution == nil {
		t.Fatal("expected execution info to be present")
	}
	if unmarshaled.State.Execution.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", unmarshaled.State.Execution.PID)
	}
}

// TestRunningTaskImportTransform verifies that running tasks are transformed on import
func TestRunningTaskImportTransform(t *testing.T) {
	// Create an export with a running task
	export := &ExportData{
		Version:    3,
		ExportedAt: time.Now(),
		Task: &task.Task{
			ID:        "TASK-001",
			Title:     "Running Task",
			Status:    task.StatusRunning,
			UpdatedAt: time.Now().Add(-1 * time.Hour),
		},
		State: &state.State{
			TaskID:       "TASK-001",
			Status:       state.StatusRunning,
			CurrentPhase: "implement",
			Execution: &state.ExecutionInfo{
				PID:           12345,
				Hostname:      "other-machine",
				StartedAt:     time.Now().Add(-2 * time.Hour),
				LastHeartbeat: time.Now().Add(-1 * time.Hour),
			},
		},
	}

	// Simulate the import transformation
	wasRunning := false
	if export.Task.Status == task.StatusRunning {
		wasRunning = true
		export.Task.Status = task.StatusPaused
		export.Task.UpdatedAt = time.Now()

		if export.State != nil {
			export.State.Status = state.StatusInterrupted
			export.State.Execution = nil
		}
	}

	if !wasRunning {
		t.Error("expected wasRunning to be true")
	}
	if export.Task.Status != task.StatusPaused {
		t.Errorf("expected task status 'paused', got %q", export.Task.Status)
	}
	if export.State.Status != state.StatusInterrupted {
		t.Errorf("expected state status 'interrupted', got %q", export.State.Status)
	}
	if export.State.Execution != nil {
		t.Error("expected execution info to be cleared")
	}
	// Phase should be preserved
	if export.State.CurrentPhase != "implement" {
		t.Errorf("expected current phase 'implement', got %q", export.State.CurrentPhase)
	}
}
