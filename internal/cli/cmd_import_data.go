package cli

import (
	"fmt"
	"os"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// importFileWithMerge imports a task with smart merge logic.
func importFileWithMerge(path string, force, skipExisting bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	return importData(data, path, force, skipExisting)
}

// importData imports task, initiative, or workflow data with smart merge logic.
func importData(data []byte, sourceName string, force, skipExisting bool) error {
	var typeCheck struct {
		Type string `yaml:"type"`
	}
	if err := yaml.Unmarshal(data, &typeCheck); err == nil {
		switch typeCheck.Type {
		case "initiative":
			return importInitiativeData(data, sourceName, force, skipExisting)
		case "phase_template":
			return importPhaseTemplateData(data, sourceName, force, skipExisting)
		case "workflow":
			return importWorkflowData(data, sourceName, force, skipExisting)
		case "workflow_run":
			return importWorkflowRunData(data, sourceName, force, skipExisting)
		case "project_commands":
			return importProjectCommandsData(data, sourceName, force, skipExisting)
		}
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	var export ExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	if export.Task == nil {
		return fmt.Errorf("no task found in %s", sourceName)
	}

	existing, _ := backend.LoadTask(export.Task.Id)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("task %s skipped (--skip-existing)", export.Task.Id)
		}
		if !force {
			exportTime := time.Time{}
			existingTime := time.Time{}
			if export.Task.UpdatedAt != nil {
				exportTime = export.Task.UpdatedAt.AsTime()
			}
			if existing.UpdatedAt != nil {
				existingTime = existing.UpdatedAt.AsTime()
			}
			if !exportTime.After(existingTime) {
				return fmt.Errorf("task %s skipped (local version is newer or same)", export.Task.Id)
			}
		}
	}

	wasRunning := false
	if export.Task.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		wasRunning = true
		export.Task.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
		export.Task.ExecutorPid = 0
		export.Task.ExecutorHostname = nil
		export.Task.UpdatedAt = timestamppb.Now()
	}

	if err := backend.SaveTask(export.Task); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	if len(export.Transcripts) > 0 {
		existingTranscripts, _ := backend.GetTranscripts(export.Task.Id)
		transcriptKeys := make(map[string]bool)
		for _, t := range existingTranscripts {
			if t.MessageUUID != "" {
				transcriptKeys[t.MessageUUID] = true
			}
		}

		var skipped int
		for i := range export.Transcripts {
			t := &export.Transcripts[i]
			if t.MessageUUID != "" && transcriptKeys[t.MessageUUID] {
				skipped++
				continue
			}
			if err := backend.AddTranscript(t); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not import transcript: %v\n", err)
			} else if t.MessageUUID != "" {
				transcriptKeys[t.MessageUUID] = true
			}
		}
		if skipped > 0 {
			fmt.Fprintf(os.Stderr, "Info: skipped %d duplicate transcript(s)\n", skipped)
		}
	}

	for i := range export.GateDecisions {
		if err := backend.SaveGateDecision(&export.GateDecisions[i]); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not import gate decision: %v\n", err)
		}
	}
	for i := range export.TaskComments {
		if err := backend.SaveTaskComment(&export.TaskComments[i]); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not import task comment: %v\n", err)
		}
	}
	for i := range export.ReviewComments {
		if err := backend.SaveReviewComment(&export.ReviewComments[i]); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not import review comment: %v\n", err)
		}
	}
	for _, a := range export.Attachments {
		if _, err := backend.SaveAttachment(export.Task.Id, a.Filename, a.ContentType, a.Data); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not import attachment %s: %v\n", a.Filename, err)
		}
	}
	if export.Spec != "" {
		if err := backend.SaveSpecForTask(export.Task.Id, export.Spec, "imported"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not import spec: %v\n", err)
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s task %s from %s", action, export.Task.Id, sourceName)
	if wasRunning {
		fmt.Printf(" (was running, now paused - use 'orc resume %s' to continue)", export.Task.Id)
	}
	fmt.Println()
	return nil
}

// importInitiativeData imports an initiative with smart merge logic.
func importInitiativeData(data []byte, sourceName string, force, skipExisting bool) error {
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	var export InitiativeExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	if export.Initiative == nil {
		return fmt.Errorf("no initiative found in %s", sourceName)
	}

	existing, _ := backend.LoadInitiative(export.Initiative.ID)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("initiative %s skipped (--skip-existing)", export.Initiative.ID)
		}
		if !force && !export.Initiative.UpdatedAt.After(existing.UpdatedAt) {
			return fmt.Errorf("initiative %s skipped (local version is newer or same)", export.Initiative.ID)
		}
	}

	deferredDeps := export.Initiative.BlockedBy
	export.Initiative.BlockedBy = nil
	if err := backend.SaveInitiative(export.Initiative); err != nil {
		return fmt.Errorf("save initiative: %w", err)
	}
	if len(deferredDeps) > 0 {
		export.Initiative.BlockedBy = deferredDeps
		if err := backend.SaveInitiative(export.Initiative); err != nil {
			registerDeferredInitiativeDeps(export.Initiative.ID, deferredDeps)
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s initiative %s from %s\n", action, export.Initiative.ID, sourceName)
	return nil
}

// importPhaseTemplateData imports a phase template with smart merge logic.
func importPhaseTemplateData(data []byte, sourceName string, force, skipExisting bool) error {
	var export PhaseTemplateExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	if export.PhaseTemplate == nil {
		return fmt.Errorf("no phase template found in %s", sourceName)
	}
	if export.PhaseTemplate.IsBuiltin {
		return fmt.Errorf("phase template %s skipped (built-in)", export.PhaseTemplate.ID)
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	existing, _ := backend.GetPhaseTemplate(export.PhaseTemplate.ID)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("phase template %s skipped (--skip-existing)", export.PhaseTemplate.ID)
		}
		if !force && !export.PhaseTemplate.UpdatedAt.After(existing.UpdatedAt) {
			return fmt.Errorf("phase template %s skipped (local version is newer or same)", export.PhaseTemplate.ID)
		}
	}

	if err := backend.SavePhaseTemplate(export.PhaseTemplate); err != nil {
		return fmt.Errorf("save phase template: %w", err)
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s phase template %s from %s\n", action, export.PhaseTemplate.ID, sourceName)
	return nil
}

// importWorkflowData imports a workflow with its phases and variables.
func importWorkflowData(data []byte, sourceName string, force, skipExisting bool) error {
	var export WorkflowExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	if export.Workflow == nil {
		return fmt.Errorf("no workflow found in %s", sourceName)
	}
	if export.Workflow.IsBuiltin {
		return fmt.Errorf("workflow %s skipped (built-in)", export.Workflow.ID)
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	existing, _ := backend.GetWorkflow(export.Workflow.ID)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("workflow %s skipped (--skip-existing)", export.Workflow.ID)
		}
		if !force && !export.Workflow.UpdatedAt.After(existing.UpdatedAt) {
			return fmt.Errorf("workflow %s skipped (local version is newer or same)", export.Workflow.ID)
		}
	}

	if err := backend.SaveWorkflow(export.Workflow); err != nil {
		return fmt.Errorf("save workflow: %w", err)
	}
	for _, phase := range export.Phases {
		if err := backend.SaveWorkflowPhase(phase); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save workflow phase %s: %v\n", phase.PhaseTemplateID, err)
		}
	}
	for _, variable := range export.Variables {
		if err := backend.SaveWorkflowVariable(variable); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save workflow variable %s: %v\n", variable.Name, err)
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s workflow %s from %s\n", action, export.Workflow.ID, sourceName)
	return nil
}

// importWorkflowRunData imports a workflow run with its phases.
func importWorkflowRunData(data []byte, sourceName string, force, skipExisting bool) error {
	var export WorkflowRunExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	if export.WorkflowRun == nil {
		return fmt.Errorf("no workflow run found in %s", sourceName)
	}

	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	existing, _ := backend.GetWorkflowRun(export.WorkflowRun.ID)
	if existing != nil {
		if skipExisting {
			return fmt.Errorf("workflow run %s skipped (--skip-existing)", export.WorkflowRun.ID)
		}
		if !force && !export.WorkflowRun.UpdatedAt.After(existing.UpdatedAt) {
			return fmt.Errorf("workflow run %s skipped (local version is newer or same)", export.WorkflowRun.ID)
		}
	}

	wasRunning := false
	if export.WorkflowRun.Status == "running" {
		wasRunning = true
		export.WorkflowRun.Status = "paused"
	}

	if err := backend.SaveWorkflowRun(export.WorkflowRun); err != nil {
		return fmt.Errorf("save workflow run: %w", err)
	}
	for _, phase := range export.Phases {
		if err := backend.SaveWorkflowRunPhase(phase); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save workflow run phase %s: %v\n", phase.PhaseTemplateID, err)
		}
	}

	action := "Imported"
	if existing != nil {
		action = "Updated"
	}
	fmt.Printf("%s workflow run %s from %s", action, export.WorkflowRun.ID, sourceName)
	if wasRunning {
		fmt.Printf(" (was running, now paused)")
	}
	fmt.Println()
	return nil
}

// importProjectCommandsData imports project commands.
func importProjectCommandsData(data []byte, sourceName string, force, skipExisting bool) error {
	var export ProjectCommandsExportData
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}
	if len(export.Commands) == 0 {
		return fmt.Errorf("no commands found in %s", sourceName)
	}

	projectRoot, err := ResolveProjectPath()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}
	pdb, err := db.OpenProject(projectRoot)
	if err != nil {
		return fmt.Errorf("open project database: %w", err)
	}
	defer func() { _ = pdb.Close() }()

	var imported, skipped int
	for _, cmd := range export.Commands {
		existing, _ := pdb.GetProjectCommand(cmd.Name)
		if existing != nil {
			if skipExisting {
				skipped++
				continue
			}
			if !force && !cmd.UpdatedAt.After(existing.UpdatedAt) {
				skipped++
				continue
			}
		}

		if err := pdb.SaveProjectCommand(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save command %s: %v\n", cmd.Name, err)
			continue
		}
		imported++
	}

	if imported > 0 || skipped > 0 {
		fmt.Printf("Imported %d project command(s) from %s", imported, sourceName)
		if skipped > 0 {
			fmt.Printf(", skipped %d", skipped)
		}
		fmt.Println()
	}
	return nil
}
