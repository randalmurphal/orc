# TASK-020: Task Attachments and Image Support

## Overview

Enable storing attachments (images, files) in task folders with web UI rendering.

## Storage Structure

```
.orc/tasks/TASK-XXX/
├── task.yaml
├── state.yaml
├── plan.yaml
├── spec.md
├── attachments/           # General attachments
│   ├── screenshot-1.png
│   └── design-mock.pdf
└── test-results/          # Playwright test output (TASK-024)
    └── (handled separately)
```

## API Endpoints

### Upload Attachment
```
POST /api/tasks/{id}/attachments
Content-Type: multipart/form-data

Response: { filename, path, size, mime_type, created_at }
```

### List Attachments
```
GET /api/tasks/{id}/attachments

Response: [{ filename, path, size, mime_type, created_at }]
```

### Get Attachment
```
GET /api/tasks/{id}/attachments/{filename}

Response: File stream with correct Content-Type
```

### Delete Attachment
```
DELETE /api/tasks/{id}/attachments/{filename}
```

## Backend Implementation

### New File: `internal/api/handlers_attachments.go`

```go
func (s *Server) handleUploadAttachment(w, r)
func (s *Server) handleListAttachments(w, r)
func (s *Server) handleGetAttachment(w, r)
func (s *Server) handleDeleteAttachment(w, r)
```

### Attachment Model

```go
type Attachment struct {
    Filename  string    `json:"filename"`
    Path      string    `json:"path"`
    Size      int64     `json:"size"`
    MimeType  string    `json:"mime_type"`
    CreatedAt time.Time `json:"created_at"`
}
```

## Web UI

### Task Detail Page - Attachments Tab

Location: `/tasks/{id}` - new "Attachments" tab alongside Timeline/Changes/Transcript

Components:
- `AttachmentList.svelte` - Grid/list of attachments
- `AttachmentPreview.svelte` - Image preview, file download
- `AttachmentUpload.svelte` - Drag-drop upload zone

### Image Rendering

Images (png, jpg, gif, webp) render inline with:
- Thumbnail in list view
- Lightbox for full view
- Download button

### Non-Image Files

Show icon + filename + size, download on click.

## File Size Limits

- Max single file: 10MB
- Max total per task: 50MB
- Enforce in API and show in UI

## Security

- Validate file types (whitelist: images, pdf, txt, md, json, yaml)
- Sanitize filenames (no path traversal)
- Check task exists before upload

## Testing

- Unit tests for handlers
- E2E test for upload/view/delete flow
