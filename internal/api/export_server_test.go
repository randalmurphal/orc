// Package api provides tests for export/import API handlers.
package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
)

// =============================================================================
// SC-1: Export API returns downloadable tar.gz archive
// =============================================================================

// TestExportHandler_AllTasks_ReturnsTarGz verifies SC-1:
// POST /api/export with all_tasks=true returns a valid tar.gz archive.
func TestExportHandler_AllTasks_ReturnsTarGz(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a test task
	task := &orcv1.Task{
		Id:          "TASK-001",
		Title:       "Test Task",
		Description: ptr("A test task"),
		Status:      orcv1.TaskStatus_TASK_STATUS_CREATED,
		CreatedAt:   timestamppb.Now(),
		UpdatedAt:   timestamppb.Now(),
	}
	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewExportServer(backend, t.TempDir(), nil)

	// Create request
	reqBody := `{"all_tasks": true, "include_transcripts": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Handle request
	server.HandleExport(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify content type
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/gzip" {
		t.Errorf("expected Content-Type application/gzip, got %s", contentType)
	}

	// Verify Content-Disposition header
	disposition := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "attachment") || !strings.Contains(disposition, ".tar.gz") {
		t.Errorf("expected Content-Disposition with attachment and .tar.gz, got %s", disposition)
	}

	// Verify it's a valid tar.gz archive
	gzReader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	// Verify manifest and task are present
	var hasManifest, hasTask bool
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}

		if header.Name == "manifest.yaml" {
			hasManifest = true
			// Verify manifest content
			data, _ := io.ReadAll(tarReader)
			var manifest exportManifest
			if err := yaml.Unmarshal(data, &manifest); err != nil {
				t.Errorf("failed to parse manifest: %v", err)
			} else {
				if manifest.TaskCount != 1 {
					t.Errorf("expected TaskCount=1, got %d", manifest.TaskCount)
				}
				if manifest.Version != exportFormatVersion {
					t.Errorf("expected Version=%d, got %d", exportFormatVersion, manifest.Version)
				}
			}
		}
		if strings.HasPrefix(header.Name, "tasks/TASK-001") {
			hasTask = true
		}
	}

	if !hasManifest {
		t.Error("manifest.yaml not found in archive")
	}
	if !hasTask {
		t.Error("tasks/TASK-001.yaml not found in archive")
	}
}

// TestExportHandler_MinimalOption_ExcludesTranscripts verifies SC-1:
// POST /api/export with minimal=true excludes transcripts.
func TestExportHandler_MinimalOption_ExcludesTranscripts(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a test task with transcripts
	task := &orcv1.Task{
		Id:          "TASK-002",
		Title:       "Task with Transcripts",
		Description: ptr("A test task"),
		Status:      orcv1.TaskStatus_TASK_STATUS_CREATED,
		CreatedAt:   timestamppb.Now(),
		UpdatedAt:   timestamppb.Now(),
	}
	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Add a transcript
	transcript := &storage.Transcript{
		TaskID:      "TASK-002",
		Phase:       "implement",
		MessageUUID: "uuid-123",
		Role:        "assistant",
		Content:     "Some transcript content",
		Timestamp:   time.Now().UnixMilli(),
	}
	if err := backend.AddTranscript(transcript); err != nil {
		t.Fatalf("add transcript: %v", err)
	}

	server := NewExportServer(backend, t.TempDir(), nil)

	// Create request with minimal=true
	reqBody := `{"all_tasks": true, "minimal": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/export", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Handle request
	server.HandleExport(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Read the archive and verify transcripts are excluded
	gzReader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}

		if strings.HasPrefix(header.Name, "tasks/TASK-002") {
			data, _ := io.ReadAll(tarReader)
			var export exportData
			if err := yaml.Unmarshal(data, &export); err != nil {
				t.Fatalf("failed to parse task: %v", err)
			}
			if len(export.Transcripts) > 0 {
				t.Error("expected no transcripts with minimal=true, but found some")
			}
		}
	}
}

// =============================================================================
// SC-2: Import API accepts tar.gz upload and returns JSON results
// =============================================================================

// TestImportHandler_TarGzUpload_ReturnsResults verifies SC-2:
// POST /api/import with tar.gz file returns JSON with import counts.
func TestImportHandler_TarGzUpload_ReturnsResults(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewExportServer(backend, t.TempDir(), nil)

	// Create a tar.gz archive with a task
	archiveData := createTestArchive(t, []testTask{
		{
			ID:    "TASK-100",
			Title: "Imported Task",
		},
	}, nil)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test-export.tar.gz")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(archiveData); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/api/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	// Handle request
	server.HandleImport(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify JSON response
	var result ImportResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.TasksImported != 1 {
		t.Errorf("expected TasksImported=1, got %d", result.TasksImported)
	}
	if result.DryRun {
		t.Error("expected DryRun=false")
	}

	// Verify task was actually imported
	imported, err := backend.LoadTask("TASK-100")
	if err != nil {
		t.Fatalf("load imported task: %v", err)
	}
	if imported.Title != "Imported Task" {
		t.Errorf("expected Title='Imported Task', got %s", imported.Title)
	}
}

// TestImportHandler_DryRun_PreviewsWithoutImporting verifies SC-2:
// POST /api/import?dry_run=true previews import without actual changes.
func TestImportHandler_DryRun_PreviewsWithoutImporting(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewExportServer(backend, t.TempDir(), nil)

	// Create a tar.gz archive with a task
	archiveData := createTestArchive(t, []testTask{
		{
			ID:    "TASK-200",
			Title: "Dry Run Task",
		},
	}, nil)

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test-export.tar.gz")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(archiveData); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	// Create request with dry_run=true
	req := httptest.NewRequest(http.MethodPost, "/api/import?dry_run=true", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	// Handle request
	server.HandleImport(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify JSON response
	var result ImportResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !result.DryRun {
		t.Error("expected DryRun=true")
	}
	if result.TasksImported != 1 {
		t.Errorf("expected TasksImported=1 (would be imported), got %d", result.TasksImported)
	}

	// Verify task was NOT actually imported
	_, err = backend.LoadTask("TASK-200")
	if err == nil {
		t.Error("task should NOT have been imported in dry run mode")
	}
}

// TestImportHandler_InitiativesIncluded verifies SC-2:
// POST /api/import imports both tasks and initiatives from archive.
func TestImportHandler_InitiativesIncluded(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewExportServer(backend, t.TempDir(), nil)

	// Create a tar.gz archive with a task and an initiative
	archiveData := createTestArchive(t, []testTask{
		{
			ID:    "TASK-300",
			Title: "Task with Initiative",
		},
	}, []testInitiative{
		{
			ID:    "INIT-001",
			Title: "Test Initiative",
		},
	})

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test-export.tar.gz")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(archiveData); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/api/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	// Handle request
	server.HandleImport(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify JSON response
	var result ImportResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.TasksImported != 1 {
		t.Errorf("expected TasksImported=1, got %d", result.TasksImported)
	}
	if result.InitiativesImported != 1 {
		t.Errorf("expected InitiativesImported=1, got %d", result.InitiativesImported)
	}
}

// =============================================================================
// Helper functions
// =============================================================================

type testTask struct {
	ID    string
	Title string
}

type testInitiative struct {
	ID    string
	Title string
}

// createTestArchive creates a tar.gz archive with the given tasks and initiatives.
func createTestArchive(t *testing.T, tasks []testTask, initiatives []testInitiative) []byte {
	t.Helper()

	buf := &bytes.Buffer{}
	gzWriter := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gzWriter)

	// Write manifest
	manifest := exportManifest{
		Version:         exportFormatVersion,
		ExportedAt:      time.Now(),
		TaskCount:       len(tasks),
		InitiativeCount: len(initiatives),
		IncludesState:   true,
	}
	manifestData, _ := yaml.Marshal(manifest)
	if err := writeTarFile(tarWriter, "manifest.yaml", manifestData); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Write tasks
	for _, task := range tasks {
		taskData := exportData{
			Version:    exportFormatVersion,
			ExportedAt: time.Now(),
			Task: &orcv1.Task{
				Id:        task.ID,
				Title:     task.Title,
				Status:    orcv1.TaskStatus_TASK_STATUS_CREATED,
				CreatedAt: timestamppb.Now(),
				UpdatedAt: timestamppb.Now(),
			},
		}
		yamlData, _ := yaml.Marshal(taskData)
		if err := writeTarFile(tarWriter, "tasks/"+task.ID+".yaml", yamlData); err != nil {
			t.Fatalf("write task: %v", err)
		}
	}

	// Write initiatives
	for _, init := range initiatives {
		initData := initiativeExportData{
			Version:    exportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "initiative",
			Initiative: &initiative.Initiative{
				ID:        init.ID,
				Title:     init.Title,
				Status:    "draft",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}
		yamlData, _ := yaml.Marshal(initData)
		if err := writeTarFile(tarWriter, "initiatives/"+init.ID+".yaml", yamlData); err != nil {
			t.Fatalf("write initiative: %v", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	return buf.Bytes()
}

func ptr(s string) *string {
	return &s
}
