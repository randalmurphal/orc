// Package api contains tests for phase template data flow field persistence.
//
// TASK-714: QA: Phase template creation and data flow verification
//
// These TDD tests verify that data flow fields (input_variables, output_var_name,
// prompt_source) are correctly persisted through Create/Update/GET cycles.
//
// Success Criteria Coverage:
//   - SC-1: Create template with input_variables, output_var_name, prompt_source - all persisted
//   - SC-2: Update template data flow fields - GET returns updated
//   - SC-3: Prompt source toggle (inline vs file) - stored correctly
//
// Note: Frontend tests (SC-4 through SC-7) are covered in:
//   - web/src/components/workflows/EditPhaseTemplateModal.dataflow.test.tsx
//   - web/src/components/workflows/CreatePhaseTemplateModal.test.tsx
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/workflow"
)

// =============================================================================
// Test Helpers
// =============================================================================

// setupDataFlowTest creates a test server with an empty globalDB and a resolver
// configured for a temporary directory. Returns the server and globalDB.
func setupDataFlowTest(t *testing.T) (*workflowServer, *db.GlobalDB) {
	t.Helper()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Create a temporary .orc directory for the resolver
	tempDir := t.TempDir()
	orcDir := filepath.Join(tempDir, ".orc")
	phasesDir := filepath.Join(orcDir, "phases")
	if err := os.MkdirAll(phasesDir, 0755); err != nil {
		t.Fatalf("create phases dir: %v", err)
	}

	// Create resolver and cache for the temp directory
	resolver := workflow.NewResolverFromOrcDir(orcDir)
	cache := workflow.NewCacheService(resolver, globalDB)

	srv := NewWorkflowServer(backend, globalDB, resolver, nil, cache, slog.Default())
	return srv.(*workflowServer), globalDB
}

// getPhaseTemplateFromDB reads a phase template directly from the database
// for verification independent of the API layer.
func getPhaseTemplateFromDB(t *testing.T, globalDB *db.GlobalDB, id string) *db.PhaseTemplate {
	t.Helper()
	tmpl, err := globalDB.GetPhaseTemplate(id)
	if err != nil {
		t.Fatalf("get phase template from DB: %v", err)
	}
	if tmpl == nil {
		t.Fatalf("phase template %q not found in DB", id)
	}
	return tmpl
}

// parseInputVariablesJSON unmarshals the JSON-encoded input_variables column.
func parseInputVariablesJSON(t *testing.T, jsonStr string) []string {
	t.Helper()
	if jsonStr == "" {
		return nil
	}
	var vars []string
	if err := json.Unmarshal([]byte(jsonStr), &vars); err != nil {
		t.Fatalf("parse input_variables JSON: %v", err)
	}
	return vars
}

// =============================================================================
// SC-1: Create template with data flow fields — all persisted
// =============================================================================

// NOTE: CreatePhaseTemplateRequest currently lacks input_variables field in proto.
// The following test documents the EXPECTED behavior after implementation.
// It uses Update immediately after Create to set input_variables.
// Once proto is updated, this should be changed to set input_variables in Create.

func TestCreatePhaseTemplate_OutputVarName_Persisted(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	outputName := "ANALYSIS_REPORT"
	req := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:            "test-output-var",
		Name:          "Test Output Var",
		OutputVarName: &outputName,
		PromptSource:  orcv1.PromptSource_PROMPT_SOURCE_DB,
	})

	resp, err := server.CreatePhaseTemplate(ctx, req)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Verify response contains output_var_name
	if resp.Msg.Template.OutputVarName == nil || *resp.Msg.Template.OutputVarName != "ANALYSIS_REPORT" {
		t.Errorf("expected OutputVarName='ANALYSIS_REPORT' in response, got %v", resp.Msg.Template.OutputVarName)
	}

	// Verify persisted in DB
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-output-var")
	if tmpl.OutputVarName != "ANALYSIS_REPORT" {
		t.Errorf("expected OutputVarName='ANALYSIS_REPORT' in DB, got %q", tmpl.OutputVarName)
	}
}

func TestCreatePhaseTemplate_PromptSource_DB_Persisted(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	promptContent := "Analyze the code in {{WORKTREE_PATH}}"
	req := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:            "test-prompt-db",
		Name:          "Test Prompt DB",
		PromptSource:  orcv1.PromptSource_PROMPT_SOURCE_DB,
		PromptContent: &promptContent,
	})

	resp, err := server.CreatePhaseTemplate(ctx, req)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Verify response
	if resp.Msg.Template.PromptSource != orcv1.PromptSource_PROMPT_SOURCE_DB {
		t.Errorf("expected PromptSource=DB in response, got %v", resp.Msg.Template.PromptSource)
	}
	if resp.Msg.Template.PromptContent == nil || *resp.Msg.Template.PromptContent != promptContent {
		t.Errorf("expected PromptContent=%q in response, got %v", promptContent, resp.Msg.Template.PromptContent)
	}

	// Verify DB
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-prompt-db")
	if tmpl.PromptSource != "db" {
		t.Errorf("expected PromptSource='db' in DB, got %q", tmpl.PromptSource)
	}
	if tmpl.PromptContent != promptContent {
		t.Errorf("expected PromptContent=%q in DB, got %q", promptContent, tmpl.PromptContent)
	}
}

func TestCreatePhaseTemplate_PromptSource_File_Persisted(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	promptPath := "custom/analysis.md"
	req := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-prompt-file",
		Name:         "Test Prompt File",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_FILE,
		PromptPath:   &promptPath,
	})

	resp, err := server.CreatePhaseTemplate(ctx, req)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Verify response
	if resp.Msg.Template.PromptSource != orcv1.PromptSource_PROMPT_SOURCE_FILE {
		t.Errorf("expected PromptSource=FILE in response, got %v", resp.Msg.Template.PromptSource)
	}
	if resp.Msg.Template.PromptPath == nil || *resp.Msg.Template.PromptPath != promptPath {
		t.Errorf("expected PromptPath=%q in response, got %v", promptPath, resp.Msg.Template.PromptPath)
	}

	// Verify DB
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-prompt-file")
	if tmpl.PromptSource != "file" {
		t.Errorf("expected PromptSource='file' in DB, got %q", tmpl.PromptSource)
	}
	if tmpl.PromptPath != promptPath {
		t.Errorf("expected PromptPath=%q in DB, got %q", promptPath, tmpl.PromptPath)
	}
}

// TestCreateThenUpdate_InputVariables_WorksViaUpdate documents current behavior:
// input_variables can only be set via Update, not Create.
// This test verifies the UPDATE path works while CREATE path is not implemented.
// Once proto is updated to include input_variables in CreatePhaseTemplateRequest,
// a direct Create test should be added.
func TestCreateThenUpdate_InputVariables_WorksViaUpdate(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create without input variables (Create proto lacks this field)
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-input-vars-via-update",
		Name:         "Test Input Vars Via Update",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_DB,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Update with input variables (Update proto HAS this field)
	updateReq := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:             "test-input-vars-via-update",
		InputVariables: []string{"SPEC_CONTENT", "TASK_DESCRIPTION"},
	})
	updateResp, err := server.UpdatePhaseTemplate(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate failed: %v", err)
	}

	// Verify response
	if len(updateResp.Msg.Template.InputVariables) != 2 {
		t.Errorf("expected 2 input variables in response, got %d: %v",
			len(updateResp.Msg.Template.InputVariables), updateResp.Msg.Template.InputVariables)
	}
	if updateResp.Msg.Template.InputVariables[0] != "SPEC_CONTENT" {
		t.Errorf("expected InputVariables[0]='SPEC_CONTENT', got %q", updateResp.Msg.Template.InputVariables[0])
	}
	if updateResp.Msg.Template.InputVariables[1] != "TASK_DESCRIPTION" {
		t.Errorf("expected InputVariables[1]='TASK_DESCRIPTION', got %q", updateResp.Msg.Template.InputVariables[1])
	}

	// Verify persisted in DB
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-input-vars-via-update")
	vars := parseInputVariablesJSON(t, tmpl.InputVariables)
	if len(vars) != 2 {
		t.Errorf("expected 2 input variables in DB, got %d: %v", len(vars), vars)
	}
}

func TestCreatePhaseTemplate_AllDataFlowFields_Persisted(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create with output_var_name and prompt_source (the fields available in Create proto)
	outputName := "FULL_OUTPUT"
	promptContent := "Full test prompt"
	req := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:            "test-full-dataflow",
		Name:          "Test Full Data Flow",
		OutputVarName: &outputName,
		PromptSource:  orcv1.PromptSource_PROMPT_SOURCE_DB,
		PromptContent: &promptContent,
	})

	resp, err := server.CreatePhaseTemplate(ctx, req)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Verify fields in response
	if resp.Msg.Template.OutputVarName == nil || *resp.Msg.Template.OutputVarName != "FULL_OUTPUT" {
		t.Errorf("expected OutputVarName='FULL_OUTPUT', got %v", resp.Msg.Template.OutputVarName)
	}
	if resp.Msg.Template.PromptSource != orcv1.PromptSource_PROMPT_SOURCE_DB {
		t.Errorf("expected PromptSource=DB, got %v", resp.Msg.Template.PromptSource)
	}

	// Now use Update to add input_variables
	updateReq := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:             "test-full-dataflow",
		InputVariables: []string{"SPEC_CONTENT", "PROJECT_ROOT", "TASK_DESCRIPTION", "WORKTREE_PATH"},
	})
	_, err = server.UpdatePhaseTemplate(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate failed: %v", err)
	}

	// Verify all in DB
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-full-dataflow")
	vars := parseInputVariablesJSON(t, tmpl.InputVariables)
	if len(vars) != 4 {
		t.Errorf("expected 4 input variables in DB, got %d: %v", len(vars), vars)
	}
	if tmpl.OutputVarName != "FULL_OUTPUT" {
		t.Errorf("expected OutputVarName='FULL_OUTPUT' in DB, got %q", tmpl.OutputVarName)
	}
	if tmpl.PromptSource != "db" {
		t.Errorf("expected PromptSource='db' in DB, got %q", tmpl.PromptSource)
	}
}

// =============================================================================
// SC-2: Update data flow fields — GET returns updated values
// =============================================================================

func TestUpdatePhaseTemplate_InputVariables_Updated(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// First create a template without input variables
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-update-inputs",
		Name:         "Test Update Inputs",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_DB,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Update with input variables
	updateReq := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:             "test-update-inputs",
		InputVariables: []string{"SPEC_CONTENT", "WORKTREE_PATH"},
	})
	updateResp, err := server.UpdatePhaseTemplate(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate failed: %v", err)
	}

	// Verify response contains updated values
	if len(updateResp.Msg.Template.InputVariables) != 2 {
		t.Errorf("expected 2 input variables in update response, got %d: %v",
			len(updateResp.Msg.Template.InputVariables), updateResp.Msg.Template.InputVariables)
	}

	// Verify GET returns updated values
	getReq := connect.NewRequest(&orcv1.GetPhaseTemplateRequest{
		Id: "test-update-inputs",
	})
	getResp, err := server.GetPhaseTemplate(ctx, getReq)
	if err != nil {
		t.Fatalf("GetPhaseTemplate failed: %v", err)
	}
	if len(getResp.Msg.Template.InputVariables) != 2 {
		t.Errorf("expected 2 input variables from GET, got %d: %v",
			len(getResp.Msg.Template.InputVariables), getResp.Msg.Template.InputVariables)
	}

	// Verify DB
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-update-inputs")
	vars := parseInputVariablesJSON(t, tmpl.InputVariables)
	if len(vars) != 2 {
		t.Errorf("expected 2 input variables in DB, got %d: %v", len(vars), vars)
	}
}

func TestUpdatePhaseTemplate_OutputVarName_Updated(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create template without output_var_name
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-update-output",
		Name:         "Test Update Output",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_DB,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Update with output_var_name
	outputName := "NEW_OUTPUT_VAR"
	updateReq := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:            "test-update-output",
		OutputVarName: &outputName,
	})
	updateResp, err := server.UpdatePhaseTemplate(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate failed: %v", err)
	}

	// Verify response
	if updateResp.Msg.Template.OutputVarName == nil || *updateResp.Msg.Template.OutputVarName != "NEW_OUTPUT_VAR" {
		t.Errorf("expected OutputVarName='NEW_OUTPUT_VAR' in response, got %v", updateResp.Msg.Template.OutputVarName)
	}

	// Verify GET
	getReq := connect.NewRequest(&orcv1.GetPhaseTemplateRequest{
		Id: "test-update-output",
	})
	getResp, err := server.GetPhaseTemplate(ctx, getReq)
	if err != nil {
		t.Fatalf("GetPhaseTemplate failed: %v", err)
	}
	if getResp.Msg.Template.OutputVarName == nil || *getResp.Msg.Template.OutputVarName != "NEW_OUTPUT_VAR" {
		t.Errorf("expected OutputVarName='NEW_OUTPUT_VAR' from GET, got %v", getResp.Msg.Template.OutputVarName)
	}

	// Verify DB
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-update-output")
	if tmpl.OutputVarName != "NEW_OUTPUT_VAR" {
		t.Errorf("expected OutputVarName='NEW_OUTPUT_VAR' in DB, got %q", tmpl.OutputVarName)
	}
}

func TestUpdatePhaseTemplate_InputVariables_CanBeCleared(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create template
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-clear-inputs",
		Name:         "Test Clear Inputs",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_DB,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// First update with input variables
	updateReq1 := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:             "test-clear-inputs",
		InputVariables: []string{"SPEC_CONTENT"},
	})
	_, err = server.UpdatePhaseTemplate(ctx, updateReq1)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate (add) failed: %v", err)
	}

	// Update with empty input variables
	updateReq2 := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:             "test-clear-inputs",
		InputVariables: []string{}, // explicitly empty
	})
	_, err = server.UpdatePhaseTemplate(ctx, updateReq2)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate (clear) failed: %v", err)
	}

	// Verify DB has empty/cleared input variables
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-clear-inputs")
	vars := parseInputVariablesJSON(t, tmpl.InputVariables)
	if len(vars) != 0 {
		t.Errorf("expected empty input variables after clear, got %v", vars)
	}
}

// =============================================================================
// SC-3: Prompt source toggle (inline vs file) — stored correctly
// =============================================================================

func TestUpdatePhaseTemplate_PromptSource_InlineToFile(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create with inline (DB) prompt source
	promptContent := "Initial inline content"
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:            "test-toggle-source",
		Name:          "Test Toggle Source",
		PromptSource:  orcv1.PromptSource_PROMPT_SOURCE_DB,
		PromptContent: &promptContent,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Toggle to file source
	promptPath := "new/path.md"
	promptSource := orcv1.PromptSource_PROMPT_SOURCE_FILE
	updateReq := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:           "test-toggle-source",
		PromptSource: &promptSource,
		PromptPath:   &promptPath,
	})
	_, err = server.UpdatePhaseTemplate(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate failed: %v", err)
	}

	// Verify DB shows file source
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-toggle-source")
	if tmpl.PromptSource != "file" {
		t.Errorf("expected PromptSource='file' in DB, got %q", tmpl.PromptSource)
	}
	if tmpl.PromptPath != "new/path.md" {
		t.Errorf("expected PromptPath='new/path.md' in DB, got %q", tmpl.PromptPath)
	}
}

func TestUpdatePhaseTemplate_PromptSource_FileToInline(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create with file source
	promptPath := "original/path.md"
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-toggle-to-inline",
		Name:         "Test Toggle To Inline",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_FILE,
		PromptPath:   &promptPath,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Toggle to inline (DB) source
	promptSource := orcv1.PromptSource_PROMPT_SOURCE_DB
	newContent := "New inline content after toggle"
	updateReq := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:            "test-toggle-to-inline",
		PromptSource:  &promptSource,
		PromptContent: &newContent,
	})
	_, err = server.UpdatePhaseTemplate(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate failed: %v", err)
	}

	// Verify DB shows inline source
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-toggle-to-inline")
	if tmpl.PromptSource != "db" {
		t.Errorf("expected PromptSource='db' in DB, got %q", tmpl.PromptSource)
	}
	if tmpl.PromptContent != newContent {
		t.Errorf("expected PromptContent=%q in DB, got %q", newContent, tmpl.PromptContent)
	}
}

func TestGetPhaseTemplate_ReturnsCorrectPromptSource(t *testing.T) {
	t.Parallel()
	server, _ := setupDataFlowTest(t)
	ctx := context.Background()

	// Create with file source
	promptPath := "test/path.md"
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-get-source",
		Name:         "Test Get Source",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_FILE,
		PromptPath:   &promptPath,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Verify GET returns correct prompt source
	getReq := connect.NewRequest(&orcv1.GetPhaseTemplateRequest{
		Id: "test-get-source",
	})
	getResp, err := server.GetPhaseTemplate(ctx, getReq)
	if err != nil {
		t.Fatalf("GetPhaseTemplate failed: %v", err)
	}

	if getResp.Msg.Template.PromptSource != orcv1.PromptSource_PROMPT_SOURCE_FILE {
		t.Errorf("expected PromptSource=FILE from GET, got %v", getResp.Msg.Template.PromptSource)
	}
	if getResp.Msg.Template.PromptPath == nil || *getResp.Msg.Template.PromptPath != "test/path.md" {
		t.Errorf("expected PromptPath='test/path.md' from GET, got %v", getResp.Msg.Template.PromptPath)
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestCreatePhaseTemplate_NoInputVariables_NotRequired(t *testing.T) {
	t.Parallel()
	server, _ := setupDataFlowTest(t)
	ctx := context.Background()

	// Create without specifying input variables at all
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-no-inputs",
		Name:         "Test No Inputs",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_DB,
		// InputVariables intentionally omitted (not in proto anyway)
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate should succeed without InputVariables: %v", err)
	}
}

func TestUpdatePhaseTemplate_PreservesOtherFieldsWhenUpdatingDataFlow(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create with various fields
	desc := "Original description"
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-preserve-fields",
		Name:         "Test Preserve Fields",
		Description:  &desc,
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_DB,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Update only data flow fields
	outputName := "MY_OUTPUT"
	updateReq := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:             "test-preserve-fields",
		InputVariables: []string{"SPEC_CONTENT"},
		OutputVarName:  &outputName,
	})
	_, err = server.UpdatePhaseTemplate(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate failed: %v", err)
	}

	// Verify other fields are preserved
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-preserve-fields")
	if tmpl.Description != "Original description" {
		t.Errorf("expected Description='Original description' preserved, got %q", tmpl.Description)
	}
	if tmpl.Name != "Test Preserve Fields" {
		t.Errorf("expected Name='Test Preserve Fields' preserved, got %q", tmpl.Name)
	}
}

func TestCreatePhaseTemplate_PromptSource_DefaultsToDBWhenUnspecified(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create without specifying prompt source (UNSPECIFIED)
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:   "test-default-source",
		Name: "Test Default Source",
		// PromptSource intentionally UNSPECIFIED
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Verify defaults to 'db'
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-default-source")
	if tmpl.PromptSource != "db" {
		t.Errorf("expected PromptSource to default to 'db', got %q", tmpl.PromptSource)
	}
}

func TestUpdatePhaseTemplate_InputVariables_AllFourBuiltins(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create template
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-all-four-vars",
		Name:         "Test All Four Vars",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_DB,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// Update with all four built-in variables
	updateReq := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:             "test-all-four-vars",
		InputVariables: []string{"SPEC_CONTENT", "PROJECT_ROOT", "TASK_DESCRIPTION", "WORKTREE_PATH"},
	})
	updateResp, err := server.UpdatePhaseTemplate(ctx, updateReq)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate failed: %v", err)
	}

	// Verify response
	if len(updateResp.Msg.Template.InputVariables) != 4 {
		t.Errorf("expected 4 input variables, got %d: %v",
			len(updateResp.Msg.Template.InputVariables), updateResp.Msg.Template.InputVariables)
	}

	// Verify DB
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-all-four-vars")
	vars := parseInputVariablesJSON(t, tmpl.InputVariables)
	if len(vars) != 4 {
		t.Errorf("expected 4 input variables in DB, got %d: %v", len(vars), vars)
	}

	// Verify exact values in order
	expected := []string{"SPEC_CONTENT", "PROJECT_ROOT", "TASK_DESCRIPTION", "WORKTREE_PATH"}
	for i, want := range expected {
		if i >= len(vars) || vars[i] != want {
			t.Errorf("InputVariables[%d]: expected %q, got %v", i, want, vars)
			break
		}
	}
}

func TestUpdatePhaseTemplate_InputVariables_ReplaceExisting(t *testing.T) {
	t.Parallel()
	server, globalDB := setupDataFlowTest(t)
	ctx := context.Background()

	// Create template
	createReq := connect.NewRequest(&orcv1.CreatePhaseTemplateRequest{
		Id:           "test-replace-vars",
		Name:         "Test Replace Vars",
		PromptSource: orcv1.PromptSource_PROMPT_SOURCE_DB,
	})
	_, err := server.CreatePhaseTemplate(ctx, createReq)
	if err != nil {
		t.Fatalf("CreatePhaseTemplate failed: %v", err)
	}

	// First update with one set of variables
	updateReq1 := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:             "test-replace-vars",
		InputVariables: []string{"SPEC_CONTENT", "TASK_DESCRIPTION"},
	})
	_, err = server.UpdatePhaseTemplate(ctx, updateReq1)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate (first) failed: %v", err)
	}

	// Second update with different variables (should replace, not append)
	updateReq2 := connect.NewRequest(&orcv1.UpdatePhaseTemplateRequest{
		Id:             "test-replace-vars",
		InputVariables: []string{"PROJECT_ROOT", "WORKTREE_PATH"},
	})
	updateResp, err := server.UpdatePhaseTemplate(ctx, updateReq2)
	if err != nil {
		t.Fatalf("UpdatePhaseTemplate (second) failed: %v", err)
	}

	// Verify only the new variables exist (replaced, not appended)
	if len(updateResp.Msg.Template.InputVariables) != 2 {
		t.Errorf("expected 2 input variables, got %d: %v",
			len(updateResp.Msg.Template.InputVariables), updateResp.Msg.Template.InputVariables)
	}

	// Verify DB
	tmpl := getPhaseTemplateFromDB(t, globalDB, "test-replace-vars")
	vars := parseInputVariablesJSON(t, tmpl.InputVariables)
	if len(vars) != 2 {
		t.Errorf("expected 2 input variables in DB, got %d: %v", len(vars), vars)
	}
	if vars[0] != "PROJECT_ROOT" || vars[1] != "WORKTREE_PATH" {
		t.Errorf("expected [PROJECT_ROOT, WORKTREE_PATH], got %v", vars)
	}
}
