package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// exportAllTasks exports all tasks to a directory, zip, or tar.gz archive.
func exportAllTasks(outputPath, format string, opts exportAllOptions) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	data := exportAllData{}

	// Load all tasks
	data.tasks, err = backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	// Load initiatives if requested
	if opts.withInitiatives {
		data.initiatives, err = backend.LoadAllInitiatives()
		if err != nil {
			return fmt.Errorf("load initiatives: %w", err)
		}
	}

	// Load workflows if requested
	if opts.withWorkflows {
		// Load custom phase templates (skip built-in)
		allTemplates, err := backend.ListPhaseTemplates()
		if err != nil {
			return fmt.Errorf("load phase templates: %w", err)
		}
		for _, pt := range allTemplates {
			if !pt.IsBuiltin {
				data.phaseTemplates = append(data.phaseTemplates, pt)
			}
		}

		// Load custom workflows (skip built-in)
		allWorkflows, err := backend.ListWorkflows()
		if err != nil {
			return fmt.Errorf("load workflows: %w", err)
		}
		for _, wf := range allWorkflows {
			if !wf.IsBuiltin {
				data.workflows = append(data.workflows, wf)
			}
		}

		// Load workflow runs if requested
		if opts.withRuns {
			data.workflowRuns, err = backend.ListWorkflowRuns(db.WorkflowRunListOpts{})
			if err != nil {
				return fmt.Errorf("load workflow runs: %w", err)
			}
		}
	}

	// Always load project commands (they're configuration, not optional)
	data.projectCommands, err = backend.ListProjectCommands()
	if err != nil {
		// Non-fatal - just log and continue
		fmt.Fprintf(os.Stderr, "Warning: could not load project commands: %v\n", err)
	}

	if len(data.tasks) == 0 && len(data.initiatives) == 0 && len(data.workflows) == 0 {
		fmt.Println("No tasks, initiatives, or workflows to export")
		return nil
	}

	// Detect format from filename if not using explicit format
	if format == "" {
		lower := strings.ToLower(outputPath)
		switch {
		case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
			format = "tar.gz"
		case strings.HasSuffix(lower, ".zip"):
			format = "zip"
		default:
			format = "dir"
		}
	}

	switch format {
	case "tar.gz", "tgz":
		return exportAllToTarGz(backend, data, outputPath, opts)
	case "zip":
		return exportAllToZip(backend, data, outputPath, opts)
	case "dir":
		return exportAllToDir(backend, data, outputPath, opts)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// buildManifest creates an export manifest with metadata.
func buildManifest(data exportAllData, opts exportAllOptions) *ExportManifest {
	hostname, _ := os.Hostname()
	cwd, _ := os.Getwd()

	return &ExportManifest{
		Version:             ExportFormatVersion,
		ExportedAt:          time.Now(),
		SourceHostname:      hostname,
		SourceProject:       cwd,
		OrcVersion:          runtime.Version(), // Go version as proxy for now
		TaskCount:           len(data.tasks),
		InitiativeCount:     len(data.initiatives),
		WorkflowCount:       len(data.workflows),
		PhaseTemplateCount:  len(data.phaseTemplates),
		WorkflowRunCount:    len(data.workflowRuns),
		ProjectCommandCount: len(data.projectCommands),
		IncludesState:       opts.withState,
		IncludesTranscripts: opts.withTranscripts,
		IncludesWorkflows:   opts.withWorkflows,
		IncludesRuns:        opts.withRuns,
	}
}

// exportAllToTarGz exports all data to a tar.gz archive.
func exportAllToTarGz(backend storage.Backend, data exportAllData, archivePath string, opts exportAllOptions) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(archivePath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Create the tar.gz file
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer func() { _ = file.Close() }()

	gzipWriter := gzip.NewWriter(file)
	defer func() { _ = gzipWriter.Close() }()

	tarWriter := tar.NewWriter(gzipWriter)
	defer func() { _ = tarWriter.Close() }()

	// Write manifest first
	manifest := buildManifest(data, opts)
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := writeTarFile(tarWriter, "manifest.yaml", manifestData); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Export tasks
	var tasksExported int
	for _, t := range data.tasks {
		export := buildExportDataWithBackend(backend, t, opts.withState, opts.withTranscripts)
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", t.Id, err)
			continue
		}
		if err := writeTarFile(tarWriter, filepath.Join("tasks", t.Id+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", t.Id, err)
			continue
		}
		tasksExported++
	}

	// Export initiatives
	var initExported int
	for _, init := range data.initiatives {
		export := &InitiativeExportData{
			Version:    ExportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "initiative",
			Initiative: init,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", init.ID, err)
			continue
		}
		if err := writeTarFile(tarWriter, filepath.Join("initiatives", init.ID+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", init.ID, err)
			continue
		}
		initExported++
	}

	// Export phase templates
	var templatesExported int
	for _, pt := range data.phaseTemplates {
		export := &PhaseTemplateExportData{
			Version:       ExportFormatVersion,
			ExportedAt:    time.Now(),
			Type:          "phase_template",
			PhaseTemplate: pt,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: phase template %s: marshal error: %v\n", pt.ID, err)
			continue
		}
		if err := writeTarFile(tarWriter, filepath.Join("phase_templates", pt.ID+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: phase template %s: write error: %v\n", pt.ID, err)
			continue
		}
		templatesExported++
	}

	// Export workflows with phases and variables
	var workflowsExported int
	for _, wf := range data.workflows {
		// Load phases and variables for this workflow
		phases, _ := backend.GetWorkflowPhases(wf.ID)
		variables, _ := backend.GetWorkflowVariables(wf.ID)

		export := &WorkflowExportData{
			Version:    ExportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "workflow",
			Workflow:   wf,
			Phases:     phases,
			Variables:  variables,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: workflow %s: marshal error: %v\n", wf.ID, err)
			continue
		}
		if err := writeTarFile(tarWriter, filepath.Join("workflows", wf.ID+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: workflow %s: write error: %v\n", wf.ID, err)
			continue
		}
		workflowsExported++
	}

	// Export workflow runs with phases
	var runsExported int
	for _, run := range data.workflowRuns {
		// Load phases for this run
		phases, _ := backend.GetWorkflowRunPhases(run.ID)

		export := &WorkflowRunExportData{
			Version:     ExportFormatVersion,
			ExportedAt:  time.Now(),
			Type:        "workflow_run",
			WorkflowRun: run,
			Phases:      phases,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: workflow run %s: marshal error: %v\n", run.ID, err)
			continue
		}
		if err := writeTarFile(tarWriter, filepath.Join("workflow_runs", run.ID+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: workflow run %s: write error: %v\n", run.ID, err)
			continue
		}
		runsExported++
	}

	// Export project commands (single file with all commands)
	var commandsExported int
	if len(data.projectCommands) > 0 {
		export := &ProjectCommandsExportData{
			Version:    ExportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "project_commands",
			Commands:   data.projectCommands,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: project commands: marshal error: %v\n", err)
		} else {
			if err := writeTarFile(tarWriter, "project_commands.yaml", yamlData); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: project commands: write error: %v\n", err)
			} else {
				commandsExported = len(data.projectCommands)
			}
		}
	}

	// Print summary
	fmt.Printf("Exported %d task(s)", tasksExported)
	if initExported > 0 {
		fmt.Printf(", %d initiative(s)", initExported)
	}
	if templatesExported > 0 {
		fmt.Printf(", %d phase template(s)", templatesExported)
	}
	if workflowsExported > 0 {
		fmt.Printf(", %d workflow(s)", workflowsExported)
	}
	if runsExported > 0 {
		fmt.Printf(", %d workflow run(s)", runsExported)
	}
	if commandsExported > 0 {
		fmt.Printf(", %d project command(s)", commandsExported)
	}
	fmt.Printf(" to %s\n", archivePath)
	return nil
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

// exportAllToZip exports all data to a zip archive.
func exportAllToZip(backend storage.Backend, data exportAllData, zipPath string, opts exportAllOptions) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer func() { _ = zipFile.Close() }()

	zipWriter := zip.NewWriter(zipFile)
	defer func() { _ = zipWriter.Close() }()

	// Write manifest
	manifest := buildManifest(data, opts)
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := writeZipFile(zipWriter, "manifest.yaml", manifestData); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Export tasks
	var tasksExported int
	for _, t := range data.tasks {
		export := buildExportDataWithBackend(backend, t, opts.withState, opts.withTranscripts)
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", t.Id, err)
			continue
		}
		if err := writeZipFile(zipWriter, filepath.Join("tasks", t.Id+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", t.Id, err)
			continue
		}
		tasksExported++
	}

	// Export initiatives
	var initExported int
	for _, init := range data.initiatives {
		export := &InitiativeExportData{
			Version:    ExportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "initiative",
			Initiative: init,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", init.ID, err)
			continue
		}
		if err := writeZipFile(zipWriter, filepath.Join("initiatives", init.ID+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", init.ID, err)
			continue
		}
		initExported++
	}

	// Export phase templates
	var templatesExported int
	for _, pt := range data.phaseTemplates {
		export := &PhaseTemplateExportData{
			Version:       ExportFormatVersion,
			ExportedAt:    time.Now(),
			Type:          "phase_template",
			PhaseTemplate: pt,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: phase template %s: marshal error: %v\n", pt.ID, err)
			continue
		}
		if err := writeZipFile(zipWriter, filepath.Join("phase_templates", pt.ID+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: phase template %s: write error: %v\n", pt.ID, err)
			continue
		}
		templatesExported++
	}

	// Export workflows
	var workflowsExported int
	for _, wf := range data.workflows {
		phases, _ := backend.GetWorkflowPhases(wf.ID)
		variables, _ := backend.GetWorkflowVariables(wf.ID)

		export := &WorkflowExportData{
			Version:    ExportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "workflow",
			Workflow:   wf,
			Phases:     phases,
			Variables:  variables,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: workflow %s: marshal error: %v\n", wf.ID, err)
			continue
		}
		if err := writeZipFile(zipWriter, filepath.Join("workflows", wf.ID+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: workflow %s: write error: %v\n", wf.ID, err)
			continue
		}
		workflowsExported++
	}

	// Export workflow runs
	var runsExported int
	for _, run := range data.workflowRuns {
		phases, _ := backend.GetWorkflowRunPhases(run.ID)

		export := &WorkflowRunExportData{
			Version:     ExportFormatVersion,
			ExportedAt:  time.Now(),
			Type:        "workflow_run",
			WorkflowRun: run,
			Phases:      phases,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: workflow run %s: marshal error: %v\n", run.ID, err)
			continue
		}
		if err := writeZipFile(zipWriter, filepath.Join("workflow_runs", run.ID+".yaml"), yamlData); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: workflow run %s: write error: %v\n", run.ID, err)
			continue
		}
		runsExported++
	}

	// Export project commands
	var commandsExported int
	if len(data.projectCommands) > 0 {
		export := &ProjectCommandsExportData{
			Version:    ExportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "project_commands",
			Commands:   data.projectCommands,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: project commands: marshal error: %v\n", err)
		} else {
			if err := writeZipFile(zipWriter, "project_commands.yaml", yamlData); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: project commands: write error: %v\n", err)
			} else {
				commandsExported = len(data.projectCommands)
			}
		}
	}

	// Print summary
	fmt.Printf("Exported %d task(s)", tasksExported)
	if initExported > 0 {
		fmt.Printf(", %d initiative(s)", initExported)
	}
	if templatesExported > 0 {
		fmt.Printf(", %d phase template(s)", templatesExported)
	}
	if workflowsExported > 0 {
		fmt.Printf(", %d workflow(s)", workflowsExported)
	}
	if runsExported > 0 {
		fmt.Printf(", %d workflow run(s)", runsExported)
	}
	if commandsExported > 0 {
		fmt.Printf(", %d project command(s)", commandsExported)
	}
	fmt.Printf(" to %s\n", zipPath)
	return nil
}

// writeZipFile writes a single file to a zip archive.
func writeZipFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// exportAllToDir exports all data to a directory.
func exportAllToDir(backend storage.Backend, data exportAllData, dir string, opts exportAllOptions) error {
	// Create output directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// Write manifest
	manifest := buildManifest(data, opts)
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), manifestData, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	// Create tasks subdirectory and export tasks
	var tasksExported int
	if len(data.tasks) > 0 {
		tasksDir := filepath.Join(dir, "tasks")
		if err := os.MkdirAll(tasksDir, 0755); err != nil {
			return fmt.Errorf("create tasks directory: %w", err)
		}

		for _, t := range data.tasks {
			export := buildExportDataWithBackend(backend, t, opts.withState, opts.withTranscripts)
			yamlData, err := yaml.Marshal(export)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", t.Id, err)
				continue
			}
			if err := os.WriteFile(filepath.Join(tasksDir, t.Id+".yaml"), yamlData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", t.Id, err)
				continue
			}
			tasksExported++
		}
	}

	// Export initiatives
	var initExported int
	if len(data.initiatives) > 0 {
		initDir := filepath.Join(dir, "initiatives")
		if err := os.MkdirAll(initDir, 0755); err != nil {
			return fmt.Errorf("create initiatives directory: %w", err)
		}

		for _, init := range data.initiatives {
			export := &InitiativeExportData{
				Version:    ExportFormatVersion,
				ExportedAt: time.Now(),
				Type:       "initiative",
				Initiative: init,
			}
			yamlData, err := yaml.Marshal(export)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s: marshal error: %v\n", init.ID, err)
				continue
			}
			if err := os.WriteFile(filepath.Join(initDir, init.ID+".yaml"), yamlData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: %s: write error: %v\n", init.ID, err)
				continue
			}
			initExported++
		}
	}

	// Export phase templates
	var templatesExported int
	if len(data.phaseTemplates) > 0 {
		templatesDir := filepath.Join(dir, "phase_templates")
		if err := os.MkdirAll(templatesDir, 0755); err != nil {
			return fmt.Errorf("create phase_templates directory: %w", err)
		}

		for _, pt := range data.phaseTemplates {
			export := &PhaseTemplateExportData{
				Version:       ExportFormatVersion,
				ExportedAt:    time.Now(),
				Type:          "phase_template",
				PhaseTemplate: pt,
			}
			yamlData, err := yaml.Marshal(export)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: phase template %s: marshal error: %v\n", pt.ID, err)
				continue
			}
			if err := os.WriteFile(filepath.Join(templatesDir, pt.ID+".yaml"), yamlData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: phase template %s: write error: %v\n", pt.ID, err)
				continue
			}
			templatesExported++
		}
	}

	// Export workflows
	var workflowsExported int
	if len(data.workflows) > 0 {
		workflowsDir := filepath.Join(dir, "workflows")
		if err := os.MkdirAll(workflowsDir, 0755); err != nil {
			return fmt.Errorf("create workflows directory: %w", err)
		}

		for _, wf := range data.workflows {
			phases, _ := backend.GetWorkflowPhases(wf.ID)
			variables, _ := backend.GetWorkflowVariables(wf.ID)

			export := &WorkflowExportData{
				Version:    ExportFormatVersion,
				ExportedAt: time.Now(),
				Type:       "workflow",
				Workflow:   wf,
				Phases:     phases,
				Variables:  variables,
			}
			yamlData, err := yaml.Marshal(export)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: workflow %s: marshal error: %v\n", wf.ID, err)
				continue
			}
			if err := os.WriteFile(filepath.Join(workflowsDir, wf.ID+".yaml"), yamlData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: workflow %s: write error: %v\n", wf.ID, err)
				continue
			}
			workflowsExported++
		}
	}

	// Export workflow runs
	var runsExported int
	if len(data.workflowRuns) > 0 {
		runsDir := filepath.Join(dir, "workflow_runs")
		if err := os.MkdirAll(runsDir, 0755); err != nil {
			return fmt.Errorf("create workflow_runs directory: %w", err)
		}

		for _, run := range data.workflowRuns {
			phases, _ := backend.GetWorkflowRunPhases(run.ID)

			export := &WorkflowRunExportData{
				Version:     ExportFormatVersion,
				ExportedAt:  time.Now(),
				Type:        "workflow_run",
				WorkflowRun: run,
				Phases:      phases,
			}
			yamlData, err := yaml.Marshal(export)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: workflow run %s: marshal error: %v\n", run.ID, err)
				continue
			}
			if err := os.WriteFile(filepath.Join(runsDir, run.ID+".yaml"), yamlData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: workflow run %s: write error: %v\n", run.ID, err)
				continue
			}
			runsExported++
		}
	}

	// Export project commands (single file)
	var commandsExported int
	if len(data.projectCommands) > 0 {
		export := &ProjectCommandsExportData{
			Version:    ExportFormatVersion,
			ExportedAt: time.Now(),
			Type:       "project_commands",
			Commands:   data.projectCommands,
		}
		yamlData, err := yaml.Marshal(export)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: project commands: marshal error: %v\n", err)
		} else {
			if err := os.WriteFile(filepath.Join(dir, "project_commands.yaml"), yamlData, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: project commands: write error: %v\n", err)
			} else {
				commandsExported = len(data.projectCommands)
			}
		}
	}

	// Print summary
	fmt.Printf("Exported %d task(s)", tasksExported)
	if initExported > 0 {
		fmt.Printf(", %d initiative(s)", initExported)
	}
	if templatesExported > 0 {
		fmt.Printf(", %d phase template(s)", templatesExported)
	}
	if workflowsExported > 0 {
		fmt.Printf(", %d workflow(s)", workflowsExported)
	}
	if runsExported > 0 {
		fmt.Printf(", %d workflow run(s)", runsExported)
	}
	if commandsExported > 0 {
		fmt.Printf(", %d project command(s)", commandsExported)
	}
	fmt.Printf(" to %s\n", dir)
	return nil
}

// buildExportDataWithBackend creates ExportData for a task using the backend.
// The task already contains execution state in Task.Execution (loaded by backend.LoadTask).
// withState controls whether to include gate decisions (for completeness).
// withTranscripts controls whether to include conversation history.
func buildExportDataWithBackend(backend storage.Backend, t *orcv1.Task, withState, withTranscripts bool) *ExportData {
	export := &ExportData{
		Version:    ExportFormatVersion,
		ExportedAt: time.Now(),
		Task:       t, // Task.Execution contains the execution state
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

	// Always load collaboration data (small, important for context)
	if comments, err := backend.ListTaskComments(t.Id); err == nil {
		export.TaskComments = comments
	}
	if reviews, err := backend.ListReviewComments(t.Id); err == nil {
		export.ReviewComments = reviews
	}

	// Always load attachments (with data)
	if attachments, err := backend.ListAttachments(t.Id); err == nil {
		export.Attachments = make([]AttachmentExport, 0, len(attachments))
		for _, a := range attachments {
			// Get attachment data
			_, data, err := backend.GetAttachment(t.Id, a.Filename)
			if err != nil {
				continue // Skip attachments we can't read
			}
			// Check if it's an image by content type
			isImage := strings.HasPrefix(a.ContentType, "image/")
			export.Attachments = append(export.Attachments, AttachmentExport{
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
