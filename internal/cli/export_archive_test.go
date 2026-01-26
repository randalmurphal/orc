package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
)

func TestBuildManifest(t *testing.T) {
	t.Parallel()

	// Create test data
	testData := exportAllData{
		tasks:       make([]*orcv1.Task, 10),
		initiatives: make([]*initiative.Initiative, 2),
	}
	testOpts := exportAllOptions{
		withState:       true,
		withTranscripts: true,
	}

	manifest := buildManifest(testData, testOpts)

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
	t.Parallel()
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
	t.Parallel()
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
