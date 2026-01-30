// Package api provides export/import API handlers.
package api

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
)

// ExportFormatVersion is the current version of the export format.
const exportFormatVersion = 4

// maxImportFileSize is the maximum size of a single file to import (100MB).
const maxImportFileSize = 100 * 1024 * 1024

// ExportRequest represents the JSON request body for export.
type ExportRequest struct {
	AllTasks           bool `json:"all_tasks"`
	IncludeTranscripts bool `json:"include_transcripts"`
	IncludeInitiatives bool `json:"include_initiatives"`
	Minimal            bool `json:"minimal"`
}

// ImportResult represents the JSON response for import operations.
type ImportResult struct {
	TasksImported       int      `json:"tasks_imported"`
	TasksSkipped        int      `json:"tasks_skipped"`
	InitiativesImported int      `json:"initiatives_imported"`
	InitiativesSkipped  int      `json:"initiatives_skipped"`
	Errors              []string `json:"errors,omitempty"`
	DryRun              bool     `json:"dry_run"`
}

// ExportServer handles export/import API requests.
type ExportServer struct {
	backend      storage.Backend
	projectCache *ProjectCache // Multi-project: cache of backends per project
	workDir      string
	logger       *slog.Logger
}

// NewExportServer creates a new export server.
func NewExportServer(backend storage.Backend, workDir string, logger *slog.Logger) *ExportServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &ExportServer{
		backend: backend,
		workDir: workDir,
		logger:  logger,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *ExportServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
// If projectID is provided and projectCache is available, uses the cache.
// Errors if projectID is provided but cache is not configured (prevents silent data leaks).
// Falls back to legacy single backend only when no projectID is specified.
func (s *ExportServer) getBackend(projectID string) (storage.Backend, error) {
	if projectID != "" && s.projectCache != nil {
		return s.projectCache.GetBackend(projectID)
	}
	if projectID != "" && s.projectCache == nil {
		return nil, fmt.Errorf("project_id specified but no project cache configured")
	}
	if s.backend == nil {
		return nil, fmt.Errorf("no backend available")
	}
	return s.backend, nil
}

// HandleExport handles POST /api/export requests.
// Returns a tar.gz archive as a binary download.
func (s *ExportServer) HandleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Resolve the project backend from query param
	projectID := r.URL.Query().Get("project_id")
	backend, err := s.getBackend(projectID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve backend: %v", err), http.StatusInternalServerError)
		return
	}

	// Determine export options
	withState := true
	withTranscripts := req.IncludeTranscripts
	if req.Minimal {
		withTranscripts = false
	}
	withInitiatives := req.IncludeInitiatives

	// Load all data
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to load tasks: %v", err), http.StatusInternalServerError)
		return
	}

	var initiatives []*initiative.Initiative
	if withInitiatives {
		initiatives, err = backend.LoadAllInitiatives()
		if err != nil {
			s.logger.Warn("failed to load initiatives", "error", err)
			// Continue without initiatives
		}
	}

	// Set response headers for download
	filename := fmt.Sprintf("orc-export-%s.tar.gz", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	// Create gzip writer
	gzWriter := gzip.NewWriter(w)
	defer func() { _ = gzWriter.Close() }()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer func() { _ = tarWriter.Close() }()

	// Write manifest
	manifest := s.buildManifest(len(tasks), len(initiatives), withState, withTranscripts)
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		s.logger.Error("failed to marshal manifest", "error", err)
		return
	}
	if err := writeTarFile(tarWriter, "manifest.yaml", manifestData); err != nil {
		s.logger.Error("failed to write manifest", "error", err)
		return
	}

	// Export tasks
	for _, t := range tasks {
		export := s.buildExportData(backend, t, withState, withTranscripts)
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			s.logger.Warn("failed to marshal task", "task", t.Id, "error", err)
			continue
		}
		if err := writeTarFile(tarWriter, filepath.Join("tasks", t.Id+".yaml"), yamlData); err != nil {
			s.logger.Warn("failed to write task", "task", t.Id, "error", err)
			continue
		}
	}

	// Export initiatives
	for _, init := range initiatives {
		export := &initiativeExportData{
			Version:    exportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "initiative",
			Initiative: init,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			s.logger.Warn("failed to marshal initiative", "id", init.ID, "error", err)
			continue
		}
		if err := writeTarFile(tarWriter, filepath.Join("initiatives", init.ID+".yaml"), yamlData); err != nil {
			s.logger.Warn("failed to write initiative", "id", init.ID, "error", err)
			continue
		}
	}
}

// HandleImport handles POST /api/import requests.
// Accepts multipart form with a tar.gz file upload.
func (s *ExportServer) HandleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Resolve the project backend from query param
	projectID := r.URL.Query().Get("project_id")
	backend, err := s.getBackend(projectID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to resolve backend: %v", err), http.StatusInternalServerError)
		return
	}

	// Check for dry_run query param
	dryRun := r.URL.Query().Get("dry_run") == "true"

	// Parse multipart form (max 100MB)
	if err := r.ParseMultipartForm(maxImportFileSize); err != nil {
		http.Error(w, fmt.Sprintf("failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get file: %v", err), http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	// Verify it's a tar.gz file
	filename := strings.ToLower(header.Filename)
	if !strings.HasSuffix(filename, ".tar.gz") && !strings.HasSuffix(filename, ".tgz") {
		http.Error(w, "only tar.gz files are supported", http.StatusBadRequest)
		return
	}

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read gzip: %v", err), http.StatusBadRequest)
		return
	}
	defer func() { _ = gzReader.Close() }()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Process files
	result := ImportResult{DryRun: dryRun}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("tar read error: %v", err))
			break
		}

		// Skip directories and non-YAML files
		if header.Typeflag == tar.TypeDir {
			continue
		}
		ext := filepath.Ext(header.Name)
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		// Skip manifest.yaml
		if filepath.Base(header.Name) == "manifest.yaml" {
			continue
		}

		// Read file content
		data, err := io.ReadAll(io.LimitReader(tarReader, maxImportFileSize))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: read error: %v", header.Name, err))
			continue
		}

		// Import the data
		imported, skipped, errMsg := s.importData(backend, data, header.Name, dryRun)
		if errMsg != "" {
			if strings.Contains(errMsg, "skipped") {
				// Check if it's a task or initiative
				if strings.Contains(header.Name, "initiatives/") {
					result.InitiativesSkipped++
				} else {
					result.TasksSkipped++
				}
			} else {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", header.Name, errMsg))
			}
		} else {
			if strings.Contains(header.Name, "initiatives/") {
				if imported {
					result.InitiativesImported++
				}
				if skipped {
					result.InitiativesSkipped++
				}
			} else {
				if imported {
					result.TasksImported++
				}
				if skipped {
					result.TasksSkipped++
				}
			}
		}
	}

	// Return result
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}

// buildManifest creates an export manifest.
func (s *ExportServer) buildManifest(taskCount, initiativeCount int, withState, withTranscripts bool) *exportManifest {
	hostname, _ := os.Hostname()
	return &exportManifest{
		Version:             exportFormatVersion,
		ExportedAt:          time.Now(),
		SourceHostname:      hostname,
		SourceProject:       s.workDir,
		OrcVersion:          runtime.Version(),
		TaskCount:           taskCount,
		InitiativeCount:     initiativeCount,
		IncludesState:       withState,
		IncludesTranscripts: withTranscripts,
	}
}

// buildExportData creates export data for a task.
func (s *ExportServer) buildExportData(backend storage.Backend, t *orcv1.Task, withState, withTranscripts bool) *exportData {
	export := &exportData{
		Version:    exportFormatVersion,
		ExportedAt: time.Now(),
		Task:       t,
	}

	// Always load spec
	if spec, err := backend.GetSpecForTask(t.Id); err == nil {
		export.Spec = spec
	}

	// Load gate decisions if state export is requested
	if withState {
		if decisions, err := backend.ListGateDecisions(t.Id); err == nil {
			export.GateDecisions = decisions
		}
	}

	// Load transcripts if requested
	if withTranscripts {
		if transcripts, err := backend.GetTranscripts(t.Id); err == nil {
			export.Transcripts = transcripts
		}
	}

	// Always load collaboration data
	if comments, err := backend.ListTaskComments(t.Id); err == nil {
		export.TaskComments = comments
	}
	if reviews, err := backend.ListReviewComments(t.Id); err == nil {
		export.ReviewComments = reviews
	}

	// Always load attachments
	if attachments, err := backend.ListAttachments(t.Id); err == nil {
		export.Attachments = make([]attachmentExport, 0, len(attachments))
		for _, a := range attachments {
			_, data, err := backend.GetAttachment(t.Id, a.Filename)
			if err != nil {
				continue
			}
			isImage := strings.HasPrefix(a.ContentType, "image/")
			export.Attachments = append(export.Attachments, attachmentExport{
				Filename:    a.Filename,
				ContentType: a.ContentType,
				SizeBytes:   a.Size,
				IsImage:     isImage,
				Data:        data,
			})
		}
	}

	return export
}

// importData imports task or initiative data.
// Returns (imported, skipped, errorMessage).
func (s *ExportServer) importData(backend storage.Backend, data []byte, sourceName string, dryRun bool) (bool, bool, string) {
	// First, try to detect the data type
	var typeCheck struct {
		Type string `yaml:"type"`
	}
	if err := yaml.Unmarshal(data, &typeCheck); err == nil && typeCheck.Type == "initiative" {
		return s.importInitiative(backend, data, sourceName, dryRun)
	}

	// Otherwise treat as task
	return s.importTask(backend, data, sourceName, dryRun)
}

// importTask imports a task from YAML data.
func (s *ExportServer) importTask(backend storage.Backend, data []byte, _ string, dryRun bool) (bool, bool, string) {
	var export exportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return false, false, fmt.Sprintf("parse error: %v", err)
	}

	if export.Task == nil {
		return false, false, "no task found"
	}

	// Check if task exists
	existing, _ := backend.LoadTask(export.Task.Id)
	if existing != nil {
		// Smart merge: compare timestamps
		exportTime := time.Time{}
		existingTime := time.Time{}
		if export.Task.UpdatedAt != nil {
			exportTime = export.Task.UpdatedAt.AsTime()
		}
		if existing.UpdatedAt != nil {
			existingTime = existing.UpdatedAt.AsTime()
		}
		if !exportTime.After(existingTime) {
			return false, true, "skipped (local version is newer or same)"
		}
	}

	if dryRun {
		return true, false, ""
	}

	// Handle running tasks from another machine
	if export.Task.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		export.Task.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
		export.Task.ExecutorPid = 0
		export.Task.ExecutorHostname = nil
		export.Task.UpdatedAt = timestamppb.Now()
	}

	// Save task
	if err := backend.SaveTask(export.Task); err != nil {
		return false, false, fmt.Sprintf("save error: %v", err)
	}

	// Import transcripts with deduplication
	if len(export.Transcripts) > 0 {
		existingTranscripts, _ := backend.GetTranscripts(export.Task.Id)
		transcriptKeys := make(map[string]bool)
		for _, t := range existingTranscripts {
			if t.MessageUUID != "" {
				transcriptKeys[t.MessageUUID] = true
			}
		}

		for i := range export.Transcripts {
			t := &export.Transcripts[i]
			if t.MessageUUID != "" && transcriptKeys[t.MessageUUID] {
				continue // Skip duplicate
			}
			if err := backend.AddTranscript(t); err != nil {
				s.logger.Warn("could not import transcript", "error", err)
			} else if t.MessageUUID != "" {
				transcriptKeys[t.MessageUUID] = true
			}
		}
	}

	// Import gate decisions
	for i := range export.GateDecisions {
		if err := backend.SaveGateDecision(&export.GateDecisions[i]); err != nil {
			s.logger.Warn("could not import gate decision", "error", err)
		}
	}

	// Import task comments
	for i := range export.TaskComments {
		if err := backend.SaveTaskComment(&export.TaskComments[i]); err != nil {
			s.logger.Warn("could not import task comment", "error", err)
		}
	}

	// Import review comments
	for i := range export.ReviewComments {
		if err := backend.SaveReviewComment(&export.ReviewComments[i]); err != nil {
			s.logger.Warn("could not import review comment", "error", err)
		}
	}

	// Import attachments
	for _, a := range export.Attachments {
		if _, err := backend.SaveAttachment(export.Task.Id, a.Filename, a.ContentType, a.Data); err != nil {
			s.logger.Warn("could not import attachment", "filename", a.Filename, "error", err)
		}
	}

	// Import spec
	if export.Spec != "" {
		if err := backend.SaveSpecForTask(export.Task.Id, export.Spec, "imported"); err != nil {
			s.logger.Warn("could not import spec", "error", err)
		}
	}

	return true, false, ""
}

// importInitiative imports an initiative from YAML data.
func (s *ExportServer) importInitiative(backend storage.Backend, data []byte, _ string, dryRun bool) (bool, bool, string) {
	var export initiativeExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return false, false, fmt.Sprintf("parse error: %v", err)
	}

	if export.Initiative == nil {
		return false, false, "no initiative found"
	}

	// Check if initiative exists
	existing, _ := backend.LoadInitiative(export.Initiative.ID)
	if existing != nil {
		// Smart merge: compare timestamps
		if !export.Initiative.UpdatedAt.After(existing.UpdatedAt) {
			return false, true, "skipped (local version is newer or same)"
		}
	}

	if dryRun {
		return true, false, ""
	}

	// Save initiative
	if err := backend.SaveInitiative(export.Initiative); err != nil {
		return false, false, fmt.Sprintf("save error: %v", err)
	}

	return true, false, ""
}

// writeTarFile writes a single file to a tar archive.
func writeTarFile(tw *tar.Writer, name string, data []byte) error {
	header := &tar.Header{
		Name:    name,
		Mode:    0644,
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

// Export data types (duplicated from CLI to avoid circular imports)
type exportManifest struct {
	Version             int       `yaml:"version"`
	ExportedAt          time.Time `yaml:"exported_at"`
	SourceHostname      string    `yaml:"source_hostname"`
	SourceProject       string    `yaml:"source_project,omitempty"`
	OrcVersion          string    `yaml:"orc_version,omitempty"`
	TaskCount           int       `yaml:"task_count"`
	InitiativeCount     int       `yaml:"initiative_count"`
	IncludesState       bool      `yaml:"includes_state"`
	IncludesTranscripts bool      `yaml:"includes_transcripts"`
}

type exportData struct {
	Version        int                      `yaml:"version"`
	ExportedAt     time.Time                `yaml:"exported_at"`
	Task           *orcv1.Task              `yaml:"task"`
	Spec           string                   `yaml:"spec,omitempty"`
	Transcripts    []storage.Transcript     `yaml:"transcripts,omitempty"`
	GateDecisions  []db.GateDecision        `yaml:"gate_decisions,omitempty"`
	TaskComments   []storage.TaskComment    `yaml:"task_comments,omitempty"`
	ReviewComments []storage.ReviewComment  `yaml:"review_comments,omitempty"`
	Attachments    []attachmentExport       `yaml:"attachments,omitempty"`
}

type attachmentExport struct {
	Filename    string `yaml:"filename"`
	ContentType string `yaml:"content_type"`
	SizeBytes   int64  `yaml:"size_bytes"`
	IsImage     bool   `yaml:"is_image"`
	Data        []byte `yaml:"data"`
}

type initiativeExportData struct {
	Version    int                      `yaml:"version"`
	ExportedAt time.Time                `yaml:"exported_at"`
	Type       string                   `yaml:"type"`
	Initiative *initiative.Initiative   `yaml:"initiative"`
}
