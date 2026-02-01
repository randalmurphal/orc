package api

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// setupTriggerCRUDTest creates a non-builtin workflow with a single phase and
// a test agent in globalDB. Returns the server, globalDB, and the phase's DB ID.
func setupTriggerCRUDTest(t *testing.T) (*workflowServer, *db.GlobalDB, int32) {
	t.Helper()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Phase template (FK constraint)
	ensurePhaseTemplatesGlobal(t, globalDB, "implement")

	// Agent that triggers can reference
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "test-agent",
		Name: "Test Agent",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Non-builtin workflow
	if err := globalDB.SaveWorkflow(&db.Workflow{
		ID:        "wf-test",
		Name:      "Test Workflow",
		IsBuiltin: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Phase linked to the workflow
	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-test",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := globalDB.SaveWorkflowPhase(phase); err != nil {
		t.Fatalf("save phase: %v", err)
	}

	srv := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())
	return srv.(*workflowServer), globalDB, int32(phase.ID)
}

// getBeforeTriggersFromDB reads and parses the before_triggers JSON from the
// first phase of a workflow. Useful for verifying DB state after API calls.
func getBeforeTriggersFromDB(t *testing.T, globalDB *db.GlobalDB, workflowID string) []db.BeforePhaseTrigger {
	t.Helper()
	phases, err := globalDB.GetWorkflowPhases(workflowID)
	if err != nil {
		t.Fatalf("get workflow phases: %v", err)
	}
	if len(phases) == 0 {
		t.Fatal("no phases found")
	}
	triggers, err := db.ParseBeforeTriggers(phases[0].BeforeTriggers)
	if err != nil {
		t.Fatalf("parse before triggers: %v", err)
	}
	return triggers
}

// assertConnectError checks that an error is a connect.Error with the expected code.
func assertConnectError(t *testing.T, err error, expectedCode connect.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %v, got nil", expectedCode)
	}
	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected *connect.Error, got %T: %v", err, err)
	}
	if connectErr.Code() != expectedCode {
		t.Errorf("expected code %v, got %v (message: %s)", expectedCode, connectErr.Code(), connectErr.Message())
	}
}

// =============================================================================
// AddBeforePhaseTrigger
// =============================================================================

func TestAddBeforePhaseTrigger_Success(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})

	resp, err := server.AddBeforePhaseTrigger(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Msg == nil || resp.Msg.Phase == nil {
		t.Fatal("expected response with phase")
	}

	// Verify DB: one trigger with correct fields
	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger in DB, got %d", len(triggers))
	}
	if triggers[0].AgentID != "test-agent" {
		t.Errorf("expected agent_id=test-agent, got %s", triggers[0].AgentID)
	}
	if triggers[0].Mode != "gate" {
		t.Errorf("expected mode=gate, got %s", triggers[0].Mode)
	}
}

func TestAddBeforePhaseTrigger_AppendsToExisting(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	// Create a second agent for the second trigger
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "agent-two",
		Name: "Agent Two",
	}); err != nil {
		t.Fatalf("save second agent: %v", err)
	}

	// Add first trigger
	mode := "gate"
	req1 := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), req1); err != nil {
		t.Fatalf("add first trigger: %v", err)
	}

	// Add second trigger
	reactionMode := "reaction"
	req2 := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "agent-two",
		Mode:       &reactionMode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), req2); err != nil {
		t.Fatalf("add second trigger: %v", err)
	}

	// Verify DB: two triggers in order
	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}
	if triggers[0].AgentID != "test-agent" {
		t.Errorf("first trigger: expected agent_id=test-agent, got %s", triggers[0].AgentID)
	}
	if triggers[1].AgentID != "agent-two" {
		t.Errorf("second trigger: expected agent_id=agent-two, got %s", triggers[1].AgentID)
	}
	if triggers[1].Mode != "reaction" {
		t.Errorf("second trigger: expected mode=reaction, got %s", triggers[1].Mode)
	}
}

func TestAddBeforePhaseTrigger_WithInputOutputConfig(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	mode := "gate"
	inputCfg := `{"include_task":true,"include_phase_output":["spec"]}`
	outputCfg := `{"variable_name":"validation_result","on_approved":"continue","on_rejected":"fail"}`

	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		AgentId:      "test-agent",
		Mode:         &mode,
		InputConfig:  &inputCfg,
		OutputConfig: &outputCfg,
	})

	_, err := server.AddBeforePhaseTrigger(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify configs stored correctly
	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].InputConfig == nil {
		t.Fatal("expected non-nil input_config")
	}
	if !triggers[0].InputConfig.IncludeTask {
		t.Error("expected include_task=true")
	}
	if len(triggers[0].InputConfig.IncludePhaseOutput) != 1 || triggers[0].InputConfig.IncludePhaseOutput[0] != "spec" {
		t.Errorf("expected include_phase_output=[spec], got %v", triggers[0].InputConfig.IncludePhaseOutput)
	}
	if triggers[0].OutputConfig == nil {
		t.Fatal("expected non-nil output_config")
	}
	if triggers[0].OutputConfig.VariableName != "validation_result" {
		t.Errorf("expected variable_name=validation_result, got %s", triggers[0].OutputConfig.VariableName)
	}
	if triggers[0].OutputConfig.OnApproved != "continue" {
		t.Errorf("expected on_approved=continue, got %s", triggers[0].OutputConfig.OnApproved)
	}
	if triggers[0].OutputConfig.OnRejected != "fail" {
		t.Errorf("expected on_rejected=fail, got %s", triggers[0].OutputConfig.OnRejected)
	}
}

func TestAddBeforePhaseTrigger_DefaultMode(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	// Omit mode — should default to "gate"
	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
	})

	_, err := server.AddBeforePhaseTrigger(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].Mode != "gate" {
		t.Errorf("expected default mode=gate, got %q", triggers[0].Mode)
	}
}

func TestAddBeforePhaseTrigger_MissingWorkflowID(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
	})

	_, err := server.AddBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestAddBeforePhaseTrigger_MissingAgentID(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "",
	})

	_, err := server.AddBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestAddBeforePhaseTrigger_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-nonexistent",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
	})

	_, err := server.AddBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestAddBeforePhaseTrigger_BuiltinWorkflow(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	ensurePhaseTemplatesGlobal(t, globalDB, "implement")

	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "test-agent",
		Name: "Test Agent",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Create builtin workflow
	if err := globalDB.SaveWorkflow(&db.Workflow{
		ID:        "wf-builtin",
		Name:      "Builtin Workflow",
		IsBuiltin: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-builtin",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := globalDB.SaveWorkflowPhase(phase); err != nil {
		t.Fatalf("save phase: %v", err)
	}

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-builtin",
		PhaseId:    int32(phase.ID),
		AgentId:    "test-agent",
	})

	_, err := server.AddBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodePermissionDenied)
}

func TestAddBeforePhaseTrigger_PhaseNotFound(t *testing.T) {
	t.Parallel()
	server, _, _ := setupTriggerCRUDTest(t)

	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    99999, // non-existent phase ID
		AgentId:    "test-agent",
	})

	_, err := server.AddBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestAddBeforePhaseTrigger_AgentNotFound(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "nonexistent-agent",
	})

	_, err := server.AddBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestAddBeforePhaseTrigger_InvalidMode(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	badMode := "invalid-mode"
	req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &badMode,
	})

	_, err := server.AddBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

// =============================================================================
// UpdateBeforePhaseTrigger
// =============================================================================

func TestUpdateBeforePhaseTrigger_Success(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	// Create a second agent for the update target
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "updated-agent",
		Name: "Updated Agent",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Seed a trigger via Add
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Update trigger at index 0
	newMode := "reaction"
	newAgent := "updated-agent"
	updateReq := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 0,
		AgentId:      &newAgent,
		Mode:         &newMode,
	})

	resp, err := server.UpdateBeforePhaseTrigger(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Msg == nil || resp.Msg.Phase == nil {
		t.Fatal("expected response with phase")
	}

	// Verify DB state
	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].AgentID != "updated-agent" {
		t.Errorf("expected agent_id=updated-agent, got %s", triggers[0].AgentID)
	}
	if triggers[0].Mode != "reaction" {
		t.Errorf("expected mode=reaction, got %s", triggers[0].Mode)
	}
}

func TestUpdateBeforePhaseTrigger_PartialUpdate(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	// Seed trigger with full config
	mode := "gate"
	inputCfg := `{"include_task":true}`
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId:  "wf-test",
		PhaseId:     phaseID,
		AgentId:     "test-agent",
		Mode:        &mode,
		InputConfig: &inputCfg,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Update only mode, leave agent_id and input_config unchanged
	newMode := "reaction"
	updateReq := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 0,
		Mode:         &newMode,
	})
	if _, err := server.UpdateBeforePhaseTrigger(context.Background(), updateReq); err != nil {
		t.Fatalf("update trigger: %v", err)
	}

	// Verify: mode changed, agent_id and input_config preserved
	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].AgentID != "test-agent" {
		t.Errorf("expected agent_id preserved as test-agent, got %s", triggers[0].AgentID)
	}
	if triggers[0].Mode != "reaction" {
		t.Errorf("expected mode=reaction, got %s", triggers[0].Mode)
	}
	if triggers[0].InputConfig == nil || !triggers[0].InputConfig.IncludeTask {
		t.Error("expected input_config preserved with include_task=true")
	}
}

func TestUpdateBeforePhaseTrigger_UpdateConfigs(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	// Seed trigger without configs
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Update to add configs
	outputCfg := `{"on_rejected":"retry","retry_from":"spec"}`
	updateReq := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 0,
		OutputConfig: &outputCfg,
	})
	if _, err := server.UpdateBeforePhaseTrigger(context.Background(), updateReq); err != nil {
		t.Fatalf("update trigger: %v", err)
	}

	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].OutputConfig == nil {
		t.Fatal("expected non-nil output_config")
	}
	if triggers[0].OutputConfig.OnRejected != "retry" {
		t.Errorf("expected on_rejected=retry, got %s", triggers[0].OutputConfig.OnRejected)
	}
	if triggers[0].OutputConfig.RetryFrom != "spec" {
		t.Errorf("expected retry_from=spec, got %s", triggers[0].OutputConfig.RetryFrom)
	}
}

func TestUpdateBeforePhaseTrigger_IndexOutOfBounds(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	// Seed one trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Try to update index 5 (out of bounds)
	newMode := "reaction"
	updateReq := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 5,
		Mode:         &newMode,
	})

	_, err := server.UpdateBeforePhaseTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateBeforePhaseTrigger_NegativeIndex(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	// Seed a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	newMode := "reaction"
	updateReq := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: -1,
		Mode:         &newMode,
	})

	_, err := server.UpdateBeforePhaseTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateBeforePhaseTrigger_EmptyTriggersArray(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	// Phase has no triggers — updating index 0 should fail
	newMode := "reaction"
	updateReq := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	_, err := server.UpdateBeforePhaseTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateBeforePhaseTrigger_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	newMode := "reaction"
	req := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-nonexistent",
		PhaseId:      phaseID,
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	_, err := server.UpdateBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestUpdateBeforePhaseTrigger_BuiltinWorkflow(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	ensurePhaseTemplatesGlobal(t, globalDB, "implement")

	if err := globalDB.SaveWorkflow(&db.Workflow{
		ID:        "wf-builtin",
		Name:      "Builtin",
		IsBuiltin: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-builtin",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := globalDB.SaveWorkflowPhase(phase); err != nil {
		t.Fatalf("save phase: %v", err)
	}

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	newMode := "reaction"
	req := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-builtin",
		PhaseId:      int32(phase.ID),
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	_, err := server.UpdateBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodePermissionDenied)
}

func TestUpdateBeforePhaseTrigger_PhaseNotFound(t *testing.T) {
	t.Parallel()
	server, _, _ := setupTriggerCRUDTest(t)

	newMode := "reaction"
	req := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      99999,
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	_, err := server.UpdateBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestUpdateBeforePhaseTrigger_InvalidMode(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	// Seed a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	badMode := "invalid"
	updateReq := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 0,
		Mode:         &badMode,
	})

	_, err := server.UpdateBeforePhaseTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateBeforePhaseTrigger_AgentNotFound(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	// Seed a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	badAgent := "nonexistent-agent"
	updateReq := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 0,
		AgentId:      &badAgent,
	})

	_, err := server.UpdateBeforePhaseTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeNotFound)
}

// =============================================================================
// RemoveBeforePhaseTrigger
// =============================================================================

func TestRemoveBeforePhaseTrigger_Success(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	// Create second agent
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "agent-two",
		Name: "Agent Two",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Add two triggers
	mode := "gate"
	for _, agentID := range []string{"test-agent", "agent-two"} {
		req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
			WorkflowId: "wf-test",
			PhaseId:    phaseID,
			AgentId:    agentID,
			Mode:       &mode,
		})
		if _, err := server.AddBeforePhaseTrigger(context.Background(), req); err != nil {
			t.Fatalf("add trigger for %s: %v", agentID, err)
		}
	}

	// Remove first trigger (index 0)
	removeReq := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 0,
	})

	resp, err := server.RemoveBeforePhaseTrigger(context.Background(), removeReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Msg == nil || resp.Msg.Phase == nil {
		t.Fatal("expected response with phase")
	}

	// Verify: only agent-two remains
	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger after removal, got %d", len(triggers))
	}
	if triggers[0].AgentID != "agent-two" {
		t.Errorf("expected remaining trigger agent_id=agent-two, got %s", triggers[0].AgentID)
	}
}

func TestRemoveBeforePhaseTrigger_RemoveFromMiddle(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	// Create agents
	for _, id := range []string{"agent-a", "agent-b", "agent-c"} {
		if err := globalDB.SaveAgent(&db.Agent{ID: id, Name: id}); err != nil {
			t.Fatalf("save agent %s: %v", id, err)
		}
	}

	// Add three triggers
	mode := "gate"
	for _, agentID := range []string{"agent-a", "agent-b", "agent-c"} {
		req := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
			WorkflowId: "wf-test",
			PhaseId:    phaseID,
			AgentId:    agentID,
			Mode:       &mode,
		})
		if _, err := server.AddBeforePhaseTrigger(context.Background(), req); err != nil {
			t.Fatalf("add trigger: %v", err)
		}
	}

	// Remove middle trigger (index 1 = agent-b)
	removeReq := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 1,
	})
	if _, err := server.RemoveBeforePhaseTrigger(context.Background(), removeReq); err != nil {
		t.Fatalf("remove trigger: %v", err)
	}

	// Verify: agent-a and agent-c remain in order
	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}
	if triggers[0].AgentID != "agent-a" {
		t.Errorf("expected first trigger agent_id=agent-a, got %s", triggers[0].AgentID)
	}
	if triggers[1].AgentID != "agent-c" {
		t.Errorf("expected second trigger agent_id=agent-c, got %s", triggers[1].AgentID)
	}
}

func TestRemoveBeforePhaseTrigger_RemoveLastTrigger(t *testing.T) {
	t.Parallel()
	server, globalDB, phaseID := setupTriggerCRUDTest(t)

	// Add one trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Remove it
	removeReq := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 0,
	})
	if _, err := server.RemoveBeforePhaseTrigger(context.Background(), removeReq); err != nil {
		t.Fatalf("remove trigger: %v", err)
	}

	// Verify: empty triggers
	triggers := getBeforeTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 0 {
		t.Errorf("expected 0 triggers after removing last, got %d", len(triggers))
	}
}

func TestRemoveBeforePhaseTrigger_IndexOutOfBounds(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	// Add one trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Try to remove index 5
	removeReq := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 5,
	})

	_, err := server.RemoveBeforePhaseTrigger(context.Background(), removeReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestRemoveBeforePhaseTrigger_NegativeIndex(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	// Add a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddBeforePhaseTriggerRequest{
		WorkflowId: "wf-test",
		PhaseId:    phaseID,
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddBeforePhaseTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	removeReq := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: -1,
	})

	_, err := server.RemoveBeforePhaseTrigger(context.Background(), removeReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestRemoveBeforePhaseTrigger_EmptyTriggersArray(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	// Phase has no triggers
	removeReq := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      phaseID,
		TriggerIndex: 0,
	})

	_, err := server.RemoveBeforePhaseTrigger(context.Background(), removeReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestRemoveBeforePhaseTrigger_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	req := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "wf-nonexistent",
		PhaseId:      phaseID,
		TriggerIndex: 0,
	})

	_, err := server.RemoveBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestRemoveBeforePhaseTrigger_BuiltinWorkflow(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)
	ensurePhaseTemplatesGlobal(t, globalDB, "implement")

	if err := globalDB.SaveWorkflow(&db.Workflow{
		ID:        "wf-builtin",
		Name:      "Builtin",
		IsBuiltin: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	phase := &db.WorkflowPhase{
		WorkflowID:      "wf-builtin",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := globalDB.SaveWorkflowPhase(phase); err != nil {
		t.Fatalf("save phase: %v", err)
	}

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "wf-builtin",
		PhaseId:      int32(phase.ID),
		TriggerIndex: 0,
	})

	_, err := server.RemoveBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodePermissionDenied)
}

func TestRemoveBeforePhaseTrigger_PhaseNotFound(t *testing.T) {
	t.Parallel()
	server, _, _ := setupTriggerCRUDTest(t)

	req := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "wf-test",
		PhaseId:      99999,
		TriggerIndex: 0,
	})

	_, err := server.RemoveBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestRemoveBeforePhaseTrigger_MissingWorkflowID(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	req := connect.NewRequest(&orcv1.RemoveBeforePhaseTriggerRequest{
		WorkflowId:   "",
		PhaseId:      phaseID,
		TriggerIndex: 0,
	})

	_, err := server.RemoveBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateBeforePhaseTrigger_MissingWorkflowID(t *testing.T) {
	t.Parallel()
	server, _, phaseID := setupTriggerCRUDTest(t)

	newMode := "reaction"
	req := connect.NewRequest(&orcv1.UpdateBeforePhaseTriggerRequest{
		WorkflowId:   "",
		PhaseId:      phaseID,
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	_, err := server.UpdateBeforePhaseTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}
