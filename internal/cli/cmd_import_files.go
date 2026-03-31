package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// findLatestExport finds the most recent export file or falls back to directory.
func findLatestExport(exportDir string) (string, error) {
	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		return "", fmt.Errorf("export directory not found: %s\nUse 'orc export --all-tasks' first or specify a path", exportDir)
	}

	entries, err := os.ReadDir(exportDir)
	if err != nil {
		return "", fmt.Errorf("read export directory: %w", err)
	}

	var latestArchive string
	var latestTime time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".zip") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if latestArchive == "" || info.ModTime().After(latestTime) {
				latestArchive = filepath.Join(exportDir, entry.Name())
				latestTime = info.ModTime()
			}
		}
	}
	if latestArchive != "" {
		return latestArchive, nil
	}
	return exportDir, nil
}

// detectImportFormat detects the import format from file extension and magic bytes.
func detectImportFormat(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return "dir", nil
	}

	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return "tar.gz", nil
	}
	if strings.HasSuffix(lower, ".zip") {
		return "zip", nil
	}
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		return "yaml", nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		return "", fmt.Errorf("read magic bytes: %w", err)
	}
	if magic[0] == 0x1f && magic[1] == 0x8b {
		return "tar.gz", nil
	}
	if magic[0] == 0x50 && magic[1] == 0x4b {
		return "zip", nil
	}
	return "yaml", nil
}

// importTarGz imports all tasks and initiatives from a tar.gz archive.
func importTarGz(archivePath string, force, skipExisting bool) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(gzipReader)
	var imported, skipped int
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		ext := filepath.Ext(header.Name)
		if (ext != ".yaml" && ext != ".yml") || filepath.Base(header.Name) == "manifest.yaml" {
			continue
		}

		data, err := io.ReadAll(io.LimitReader(tarReader, maxImportFileSize))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: read error: %v\n", header.Name, err)
			continue
		}
		if err := importData(data, header.Name, force, skipExisting); err != nil {
			if strings.Contains(err.Error(), "skipped") {
				skipped++
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", header.Name, err)
			continue
		}
		imported++
	}

	if imported == 0 && skipped == 0 {
		fmt.Println("No YAML files found in archive")
	} else {
		fmt.Printf("Imported %d item(s) from %s", imported, archivePath)
		if skipped > 0 {
			fmt.Printf(", skipped %d", skipped)
		}
		fmt.Println()
	}

	processDeferredInitiativeDeps()
	return nil
}

// importDryRun previews what would be imported without making changes.
func importDryRun(path, format string) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	fmt.Printf("Dry run - previewing import from %s (format: %s)\n\n", path, format)

	files, err := collectImportFiles(path, format)
	if err != nil {
		return err
	}

	var wouldImport, wouldUpdate, wouldSkip int
	for _, f := range files {
		var typeCheck struct {
			Type string `yaml:"type"`
			Task any    `yaml:"task"`
		}
		if err := yaml.Unmarshal(f.data, &typeCheck); err != nil {
			fmt.Printf("  %-20s  [ERROR: %v]\n", filepath.Base(f.name), err)
			continue
		}

		if typeCheck.Type == "initiative" {
			var export InitiativeExportData
			if err := yaml.Unmarshal(f.data, &export); err != nil {
				fmt.Printf("  %-20s  [ERROR: %v]\n", filepath.Base(f.name), err)
				continue
			}
			existing, _ := backend.LoadInitiative(export.Initiative.ID)
			if existing == nil {
				fmt.Printf("  %-20s  [WOULD IMPORT] initiative %s\n", filepath.Base(f.name), export.Initiative.ID)
				wouldImport++
			} else if export.Initiative.UpdatedAt.After(existing.UpdatedAt) {
				fmt.Printf("  %-20s  [WOULD UPDATE] initiative %s (incoming newer)\n", filepath.Base(f.name), export.Initiative.ID)
				wouldUpdate++
			} else {
				fmt.Printf("  %-20s  [WOULD SKIP]   initiative %s (local newer or same)\n", filepath.Base(f.name), export.Initiative.ID)
				wouldSkip++
			}
			continue
		}

		if typeCheck.Task != nil {
			var export ExportData
			if err := yaml.Unmarshal(f.data, &export); err != nil {
				fmt.Printf("  %-20s  [ERROR: %v]\n", filepath.Base(f.name), err)
				continue
			}
			existing, _ := backend.LoadTask(export.Task.Id)
			statusNote := ""
			if export.Task.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
				statusNote = " (running->interrupted)"
			}
			if existing == nil {
				fmt.Printf("  %-20s  [WOULD IMPORT] task %s%s\n", filepath.Base(f.name), export.Task.Id, statusNote)
				wouldImport++
			} else {
				exportTime := time.Time{}
				existingTime := time.Time{}
				if export.Task.UpdatedAt != nil {
					exportTime = export.Task.UpdatedAt.AsTime()
				}
				if existing.UpdatedAt != nil {
					existingTime = existing.UpdatedAt.AsTime()
				}
				if exportTime.After(existingTime) {
					fmt.Printf("  %-20s  [WOULD UPDATE] task %s (incoming newer)%s\n", filepath.Base(f.name), export.Task.Id, statusNote)
					wouldUpdate++
				} else {
					fmt.Printf("  %-20s  [WOULD SKIP]   task %s (local newer or same)\n", filepath.Base(f.name), export.Task.Id)
					wouldSkip++
				}
			}
		}
	}

	fmt.Printf("\nSummary: %d would import, %d would update, %d would skip\n", wouldImport, wouldUpdate, wouldSkip)
	return nil
}

type importPreviewFile struct {
	name string
	data []byte
}

func collectImportFiles(path, format string) ([]importPreviewFile, error) {
	var files []importPreviewFile

	switch format {
	case "tar.gz":
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open archive: %w", err)
		}
		defer func() { _ = file.Close() }()
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer func() { _ = gzipReader.Close() }()
		tarReader := tar.NewReader(gzipReader)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("read tar: %w", err)
			}
			if header.Typeflag == tar.TypeDir {
				continue
			}
			ext := filepath.Ext(header.Name)
			if (ext != ".yaml" && ext != ".yml") || filepath.Base(header.Name) == "manifest.yaml" {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(tarReader, maxImportFileSize))
			if err == nil {
				files = append(files, importPreviewFile{name: header.Name, data: data})
			}
		}
	case "zip":
		r, err := zip.OpenReader(path)
		if err != nil {
			return nil, fmt.Errorf("open zip: %w", err)
		}
		defer func() { _ = r.Close() }()
		for _, f := range r.File {
			if f.FileInfo().IsDir() {
				continue
			}
			ext := filepath.Ext(f.Name)
			if (ext != ".yaml" && ext != ".yml") || filepath.Base(f.Name) == "manifest.yaml" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(rc, maxImportFileSize))
			_ = rc.Close()
			if err == nil {
				files = append(files, importPreviewFile{name: f.Name, data: data})
			}
		}
	case "yaml":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}
		files = append(files, importPreviewFile{name: path, data: data})
	case "dir":
		err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := filepath.Ext(p)
			if (ext != ".yaml" && ext != ".yml") || filepath.Base(p) == "manifest.yaml" {
				return nil
			}
			data, err := os.ReadFile(p)
			if err == nil {
				files = append(files, importPreviewFile{name: p, data: data})
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk directory: %w", err)
		}
	}

	return files, nil
}

// importZip imports all tasks from a zip archive.
func importZip(zipPath string, force, skipExisting bool) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	var imported, skipped int
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		ext := filepath.Ext(f.Name)
		if (ext != ".yaml" && ext != ".yml") || filepath.Base(f.Name) == "manifest.yaml" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: open error: %v\n", f.Name, err)
			continue
		}
		data, err := io.ReadAll(io.LimitReader(rc, maxImportFileSize))
		_ = rc.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: read error: %v\n", f.Name, err)
			continue
		}

		if err := importData(data, f.Name, force, skipExisting); err != nil {
			if strings.Contains(err.Error(), "skipped") {
				skipped++
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", f.Name, err)
			continue
		}
		imported++
	}

	if imported == 0 && skipped == 0 {
		fmt.Println("No YAML files found in archive")
	} else {
		fmt.Printf("Imported %d task(s) from %s", imported, zipPath)
		if skipped > 0 {
			fmt.Printf(", skipped %d", skipped)
		}
		fmt.Println()
	}

	processDeferredInitiativeDeps()
	return nil
}

func importDirectory(dir string, force, skipExisting bool) error {
	var tasksImported, tasksSkipped int
	var initiativesImported, initiativesSkipped int

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	hasTasksDir := false
	hasInitiativesDir := false
	for _, entry := range entries {
		if entry.IsDir() {
			switch entry.Name() {
			case "tasks":
				hasTasksDir = true
			case "initiatives":
				hasInitiativesDir = true
			}
		}
	}

	if hasTasksDir {
		tasksDir := filepath.Join(dir, "tasks")
		imported, skipped := importTasksFromDir(tasksDir, force, skipExisting)
		tasksImported += imported
		tasksSkipped += skipped
	}
	if hasInitiativesDir {
		initDir := filepath.Join(dir, "initiatives")
		imported, skipped := importInitiativesFromDir(initDir, force, skipExisting)
		initiativesImported += imported
		initiativesSkipped += skipped
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if (ext != ".yaml" && ext != ".yml") || entry.Name() == "manifest.yaml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := importFileWithMerge(path, force, skipExisting); err != nil {
			if strings.Contains(err.Error(), "skipped") {
				tasksSkipped++
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", path, err)
			continue
		}
		tasksImported++
	}

	if tasksImported == 0 && tasksSkipped == 0 && initiativesImported == 0 && initiativesSkipped == 0 {
		fmt.Println("No YAML files found to import")
	} else {
		if tasksImported > 0 || tasksSkipped > 0 {
			fmt.Printf("Imported %d task(s)", tasksImported)
			if tasksSkipped > 0 {
				fmt.Printf(", skipped %d (newer local version)", tasksSkipped)
			}
			fmt.Println()
		}
		if initiativesImported > 0 || initiativesSkipped > 0 {
			fmt.Printf("Imported %d initiative(s)", initiativesImported)
			if initiativesSkipped > 0 {
				fmt.Printf(", skipped %d (newer local version)", initiativesSkipped)
			}
			fmt.Println()
		}
	}

	processDeferredInitiativeDeps()
	return nil
}

// importTasksFromDir imports all tasks from a directory.
func importTasksFromDir(dir string, force, skipExisting bool) (imported, skipped int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read tasks directory: %v\n", err)
		return 0, 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := importFileWithMerge(path, force, skipExisting); err != nil {
			if strings.Contains(err.Error(), "skipped") {
				skipped++
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", path, err)
			continue
		}
		imported++
	}
	return imported, skipped
}

// importInitiativesFromDir imports all initiatives from a directory.
func importInitiativesFromDir(dir string, force, skipExisting bool) (imported, skipped int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read initiatives directory: %v\n", err)
		return 0, 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := importFileWithMerge(path, force, skipExisting); err != nil {
			if strings.Contains(err.Error(), "skipped") {
				skipped++
				continue
			}
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", path, err)
			continue
		}
		imported++
	}
	return imported, skipped
}
