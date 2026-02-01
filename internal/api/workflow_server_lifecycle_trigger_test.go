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

// setupLifecycleTriggerTest creates a non-builtin workflow and a test agent
// in globalDB. Returns the server and globalDB.
func setupLifecycleTriggerTest(t *testing.T) (*workflowServer, *db.GlobalDB) {
	t.Helper()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Agent that triggers can reference
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "test-agent",
		Name: "Test Agent",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Non-builtin workflow (no triggers initially)
	if err := globalDB.SaveWorkflow(&db.Workflow{
		ID:        "wf-test",
		Name:      "Test Workflow",
		IsBuiltin: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	srv := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())
	return srv.(*workflowServer), globalDB
}

// getLifecycleTriggersFromDB reads and parses the triggers JSON from a
// workflow. Useful for verifying DB state after API calls.
func getLifecycleTriggersFromDB(t *testing.T, globalDB *db.GlobalDB, workflowID string) []db.WorkflowTrigger {
	t.Helper()
	wf, err := globalDB.GetWorkflow(workflowID)
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	if wf == nil {
		t.Fatal("workflow not found")
	}
	triggers, err := db.ParseWorkflowTriggers(wf.Triggers)
	if err != nil {
		t.Fatalf("parse workflow triggers: %v", err)
	}
	return triggers
}

// =============================================================================
// AddLifecycleTrigger
// =============================================================================

func TestAddLifecycleTrigger_Success(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
		Enabled:    true,
	})

	resp, err := server.AddLifecycleTrigger(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Msg == nil || resp.Msg.Workflow == nil {
		t.Fatal("expected response with workflow")
	}

	// Verify DB: one trigger with correct fields
	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger in DB, got %d", len(triggers))
	}
	if triggers[0].Event != "on_task_completed" {
		t.Errorf("expected event=on_task_completed, got %s", triggers[0].Event)
	}
	if triggers[0].AgentID != "test-agent" {
		t.Errorf("expected agent_id=test-agent, got %s", triggers[0].AgentID)
	}
	if triggers[0].Mode != "gate" {
		t.Errorf("expected mode=gate, got %s", triggers[0].Mode)
	}
	if !triggers[0].Enabled {
		t.Error("expected enabled=true")
	}
}

func TestAddLifecycleTrigger_AppendsToExisting(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Create a second agent
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "agent-two",
		Name: "Agent Two",
	}); err != nil {
		t.Fatalf("save second agent: %v", err)
	}

	// Add first trigger
	mode := "gate"
	req1 := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_created",
		AgentId:    "test-agent",
		Mode:       &mode,
		Enabled:    true,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), req1); err != nil {
		t.Fatalf("add first trigger: %v", err)
	}

	// Add second trigger
	reactionMode := "reaction"
	req2 := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "agent-two",
		Mode:       &reactionMode,
		Enabled:    true,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), req2); err != nil {
		t.Fatalf("add second trigger: %v", err)
	}

	// Verify DB: two triggers in order
	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}
	if triggers[0].AgentID != "test-agent" {
		t.Errorf("first trigger: expected agent_id=test-agent, got %s", triggers[0].AgentID)
	}
	if triggers[0].Event != "on_task_created" {
		t.Errorf("first trigger: expected event=on_task_created, got %s", triggers[0].Event)
	}
	if triggers[1].AgentID != "agent-two" {
		t.Errorf("second trigger: expected agent_id=agent-two, got %s", triggers[1].AgentID)
	}
	if triggers[1].Mode != "reaction" {
		t.Errorf("second trigger: expected mode=reaction, got %s", triggers[1].Mode)
	}
}

func TestAddLifecycleTrigger_DefaultMode(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Omit mode — should default to "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Enabled:    true,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].Mode != "gate" {
		t.Errorf("expected default mode=gate, got %q", triggers[0].Mode)
	}
}

func TestAddLifecycleTrigger_DefaultEnabled(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Omit enabled — proto3 bool defaults to false
	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].Enabled {
		t.Error("expected enabled=false by default")
	}
}

func TestAddLifecycleTrigger_WithConfigs(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	mode := "gate"
	inputCfg := `{"include_task":true,"include_phase_output":["spec"]}`
	outputCfg := `{"variable_name":"validation_result","on_approved":"continue","on_rejected":"fail"}`

	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		Event:        "on_task_completed",
		AgentId:      "test-agent",
		Mode:         &mode,
		Enabled:      true,
		InputConfig:  &inputCfg,
		OutputConfig: &outputCfg,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify configs stored correctly
	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
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

func TestAddLifecycleTrigger_AllValidEvents(t *testing.T) {
	t.Parallel()

	validEvents := []string{
		"on_task_created",
		"on_task_completed",
		"on_task_failed",
		"on_initiative_planned",
	}

	for _, event := range validEvents {
		t.Run(event, func(t *testing.T) {
			t.Parallel()
			server, globalDB := setupLifecycleTriggerTest(t)

			mode := "gate"
			req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
				WorkflowId: "wf-test",
				Event:      event,
				AgentId:    "test-agent",
				Mode:       &mode,
			})

			_, err := server.AddLifecycleTrigger(context.Background(), req)
			if err != nil {
				t.Fatalf("event %q should be valid, got error: %v", event, err)
			}

			triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
			if len(triggers) != 1 {
				t.Fatalf("expected 1 trigger, got %d", len(triggers))
			}
			if triggers[0].Event != event {
				t.Errorf("expected event=%s, got %s", event, triggers[0].Event)
			}
		})
	}
}

func TestAddLifecycleTrigger_InvalidEvent(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_paused",
		AgentId:    "test-agent",
		Mode:       &mode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestAddLifecycleTrigger_EmptyEvent(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "",
		AgentId:    "test-agent",
		Mode:       &mode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestAddLifecycleTrigger_InvalidMode(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	badMode := "invalid-mode"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &badMode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestAddLifecycleTrigger_MissingWorkflowID(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestAddLifecycleTrigger_MissingAgentID(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "",
		Mode:       &mode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestAddLifecycleTrigger_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-nonexistent",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestAddLifecycleTrigger_AgentNotFound(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "nonexistent-agent",
		Mode:       &mode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestAddLifecycleTrigger_BuiltinWorkflow(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

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

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-builtin",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodePermissionDenied)
}

func TestAddLifecycleTrigger_DuplicateEvent(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Create a second agent
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "agent-two",
		Name: "Agent Two",
	}); err != nil {
		t.Fatalf("save second agent: %v", err)
	}

	// Add two triggers with the same event — should be allowed
	mode := "gate"
	for _, agentID := range []string{"test-agent", "agent-two"} {
		req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
			WorkflowId: "wf-test",
			Event:      "on_task_completed",
			AgentId:    agentID,
			Mode:       &mode,
		})
		if _, err := server.AddLifecycleTrigger(context.Background(), req); err != nil {
			t.Fatalf("add trigger for %s: %v", agentID, err)
		}
	}

	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers (duplicates allowed), got %d", len(triggers))
	}
	if triggers[0].Event != "on_task_completed" || triggers[1].Event != "on_task_completed" {
		t.Error("both triggers should have event=on_task_completed")
	}
}

func TestAddLifecycleTrigger_InvalidInputConfig(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	mode := "gate"
	badJSON := `{not valid json`
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId:  "wf-test",
		Event:       "on_task_completed",
		AgentId:     "test-agent",
		Mode:        &mode,
		InputConfig: &badJSON,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestAddLifecycleTrigger_InvalidOutputConfig(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	mode := "gate"
	badJSON := `{not valid json`
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		Event:        "on_task_completed",
		AgentId:      "test-agent",
		Mode:         &mode,
		OutputConfig: &badJSON,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestAddLifecycleTrigger_CorruptTriggers(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "test-agent",
		Name: "Test Agent",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Create workflow with corrupt triggers JSON
	if err := globalDB.SaveWorkflow(&db.Workflow{
		ID:        "wf-corrupt",
		Name:      "Corrupt Workflow",
		IsBuiltin: false,
		Triggers:  "{corrupt json",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	mode := "gate"
	req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-corrupt",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})

	_, err := server.AddLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInternal)
}

// =============================================================================
// UpdateLifecycleTrigger
// =============================================================================

func TestUpdateLifecycleTrigger_PartialUpdate(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Seed a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
		Enabled:    true,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Update only mode, leave everything else unchanged
	newMode := "reaction"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	resp, err := server.UpdateLifecycleTrigger(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Msg == nil || resp.Msg.Workflow == nil {
		t.Fatal("expected response with workflow")
	}

	// Verify: mode changed, event/agent/enabled preserved
	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].Mode != "reaction" {
		t.Errorf("expected mode=reaction, got %s", triggers[0].Mode)
	}
	if triggers[0].Event != "on_task_completed" {
		t.Errorf("expected event preserved as on_task_completed, got %s", triggers[0].Event)
	}
	if triggers[0].AgentID != "test-agent" {
		t.Errorf("expected agent_id preserved as test-agent, got %s", triggers[0].AgentID)
	}
	if !triggers[0].Enabled {
		t.Error("expected enabled preserved as true")
	}
}

func TestUpdateLifecycleTrigger_AgentOnly(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Create a second agent
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "updated-agent",
		Name: "Updated Agent",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Seed a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_created",
		AgentId:    "test-agent",
		Mode:       &mode,
		Enabled:    true,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Update only agent
	newAgent := "updated-agent"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		AgentId:      &newAgent,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].AgentID != "updated-agent" {
		t.Errorf("expected agent_id=updated-agent, got %s", triggers[0].AgentID)
	}
	if triggers[0].Mode != "gate" {
		t.Errorf("expected mode preserved as gate, got %s", triggers[0].Mode)
	}
	if triggers[0].Event != "on_task_created" {
		t.Errorf("expected event preserved as on_task_created, got %s", triggers[0].Event)
	}
}

func TestUpdateLifecycleTrigger_EnabledOnly(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Seed a trigger with enabled=true
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
		Enabled:    true,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Update only enabled to false
	enabled := false
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		Enabled:      &enabled,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].Enabled {
		t.Error("expected enabled=false after update")
	}
	// Other fields should be preserved
	if triggers[0].Event != "on_task_completed" {
		t.Errorf("expected event preserved, got %s", triggers[0].Event)
	}
	if triggers[0].AgentID != "test-agent" {
		t.Errorf("expected agent preserved, got %s", triggers[0].AgentID)
	}
	if triggers[0].Mode != "gate" {
		t.Errorf("expected mode preserved, got %s", triggers[0].Mode)
	}
}

func TestUpdateLifecycleTrigger_ConfigsOnly(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Seed trigger without configs
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Update to add configs
	outputCfg := `{"on_rejected":"retry","retry_from":"spec"}`
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		OutputConfig: &outputCfg,
	})
	if _, err := server.UpdateLifecycleTrigger(context.Background(), updateReq); err != nil {
		t.Fatalf("update trigger: %v", err)
	}

	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
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

func TestUpdateLifecycleTrigger_EventChange(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Seed trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_created",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Update event
	newEvent := "on_task_failed"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		Event:        &newEvent,
	})
	if _, err := server.UpdateLifecycleTrigger(context.Background(), updateReq); err != nil {
		t.Fatalf("update trigger: %v", err)
	}

	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].Event != "on_task_failed" {
		t.Errorf("expected event=on_task_failed, got %s", triggers[0].Event)
	}
}

func TestUpdateLifecycleTrigger_InvalidEventOnUpdate(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	// Seed trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_created",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Update with invalid event
	badEvent := "on_task_paused"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		Event:        &badEvent,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateLifecycleTrigger_OutOfRange(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	// Seed one trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Try to update index 5 (out of bounds)
	newMode := "reaction"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 5,
		Mode:         &newMode,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateLifecycleTrigger_NegativeIndex(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	// Seed a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	newMode := "reaction"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: -1,
		Mode:         &newMode,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateLifecycleTrigger_EmptyArray(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	// Workflow has no triggers — updating index 0 should fail
	newMode := "reaction"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateLifecycleTrigger_MissingWorkflowID(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	newMode := "reaction"
	req := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "",
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateLifecycleTrigger_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	newMode := "reaction"
	req := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-nonexistent",
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestUpdateLifecycleTrigger_BuiltinWorkflow(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	if err := globalDB.SaveWorkflow(&db.Workflow{
		ID:        "wf-builtin",
		Name:      "Builtin",
		IsBuiltin: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	newMode := "reaction"
	req := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-builtin",
		TriggerIndex: 0,
		Mode:         &newMode,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodePermissionDenied)
}

func TestUpdateLifecycleTrigger_InvalidMode(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	// Seed a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	badMode := "invalid"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		Mode:         &badMode,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestUpdateLifecycleTrigger_AgentNotFound(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	// Seed a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	badAgent := "nonexistent-agent"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		AgentId:      &badAgent,
	})

	_, err := server.UpdateLifecycleTrigger(context.Background(), updateReq)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestUpdateLifecycleTrigger_MultiTriggerPreservesOthers(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Create second agent
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "agent-two",
		Name: "Agent Two",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Add two triggers
	mode := "gate"
	for _, pair := range []struct{ event, agent string }{
		{"on_task_created", "test-agent"},
		{"on_task_completed", "agent-two"},
	} {
		req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
			WorkflowId: "wf-test",
			Event:      pair.event,
			AgentId:    pair.agent,
			Mode:       &mode,
			Enabled:    true,
		})
		if _, err := server.AddLifecycleTrigger(context.Background(), req); err != nil {
			t.Fatalf("add trigger: %v", err)
		}
	}

	// Update trigger[0] mode to reaction
	newMode := "reaction"
	updateReq := connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
		Mode:         &newMode,
	})
	if _, err := server.UpdateLifecycleTrigger(context.Background(), updateReq); err != nil {
		t.Fatalf("update trigger: %v", err)
	}

	// Verify: trigger[0] changed, trigger[1] unchanged
	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(triggers))
	}
	if triggers[0].Mode != "reaction" {
		t.Errorf("trigger[0] expected mode=reaction, got %s", triggers[0].Mode)
	}
	if triggers[1].Mode != "gate" {
		t.Errorf("trigger[1] expected mode=gate (unchanged), got %s", triggers[1].Mode)
	}
	if triggers[1].Event != "on_task_completed" {
		t.Errorf("trigger[1] expected event=on_task_completed (unchanged), got %s", triggers[1].Event)
	}
	if triggers[1].AgentID != "agent-two" {
		t.Errorf("trigger[1] expected agent_id=agent-two (unchanged), got %s", triggers[1].AgentID)
	}
}

// =============================================================================
// RemoveLifecycleTrigger
// =============================================================================

func TestRemoveLifecycleTrigger_Success(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Create second agent
	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "agent-two",
		Name: "Agent Two",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	// Add two triggers
	mode := "gate"
	for _, pair := range []struct{ event, agent string }{
		{"on_task_created", "test-agent"},
		{"on_task_completed", "agent-two"},
	} {
		req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
			WorkflowId: "wf-test",
			Event:      pair.event,
			AgentId:    pair.agent,
			Mode:       &mode,
		})
		if _, err := server.AddLifecycleTrigger(context.Background(), req); err != nil {
			t.Fatalf("add trigger: %v", err)
		}
	}

	// Remove first trigger (index 0)
	removeReq := connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
	})

	resp, err := server.RemoveLifecycleTrigger(context.Background(), removeReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Msg == nil || resp.Msg.Workflow == nil {
		t.Fatal("expected response with workflow")
	}

	// Verify: only agent-two remains
	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger after removal, got %d", len(triggers))
	}
	if triggers[0].AgentID != "agent-two" {
		t.Errorf("expected remaining trigger agent_id=agent-two, got %s", triggers[0].AgentID)
	}
}

func TestRemoveLifecycleTrigger_MiddleIndex(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Create agents
	for _, id := range []string{"agent-a", "agent-b", "agent-c"} {
		if err := globalDB.SaveAgent(&db.Agent{ID: id, Name: id}); err != nil {
			t.Fatalf("save agent %s: %v", id, err)
		}
	}

	// Add three triggers [A, B, C]
	mode := "gate"
	for _, pair := range []struct{ event, agent string }{
		{"on_task_created", "agent-a"},
		{"on_task_completed", "agent-b"},
		{"on_task_failed", "agent-c"},
	} {
		req := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
			WorkflowId: "wf-test",
			Event:      pair.event,
			AgentId:    pair.agent,
			Mode:       &mode,
		})
		if _, err := server.AddLifecycleTrigger(context.Background(), req); err != nil {
			t.Fatalf("add trigger: %v", err)
		}
	}

	// Remove middle trigger (index 1 = agent-b)
	removeReq := connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 1,
	})
	if _, err := server.RemoveLifecycleTrigger(context.Background(), removeReq); err != nil {
		t.Fatalf("remove trigger: %v", err)
	}

	// Verify: agent-a and agent-c remain in order
	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
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

func TestRemoveLifecycleTrigger_LastTrigger(t *testing.T) {
	t.Parallel()
	server, globalDB := setupLifecycleTriggerTest(t)

	// Add one trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Remove it
	removeReq := connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
	})
	if _, err := server.RemoveLifecycleTrigger(context.Background(), removeReq); err != nil {
		t.Fatalf("remove trigger: %v", err)
	}

	// Verify: empty triggers (field should be empty string per MarshalWorkflowTriggers)
	triggers := getLifecycleTriggersFromDB(t, globalDB, "wf-test")
	if len(triggers) != 0 {
		t.Errorf("expected 0 triggers after removing last, got %d", len(triggers))
	}
}

func TestRemoveLifecycleTrigger_IndexOutOfBounds(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	// Add one trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	// Try to remove index 5
	removeReq := connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 5,
	})

	_, err := server.RemoveLifecycleTrigger(context.Background(), removeReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestRemoveLifecycleTrigger_NegativeIndex(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	// Add a trigger
	mode := "gate"
	addReq := connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-test",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	})
	if _, err := server.AddLifecycleTrigger(context.Background(), addReq); err != nil {
		t.Fatalf("add trigger: %v", err)
	}

	removeReq := connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: -1,
	})

	_, err := server.RemoveLifecycleTrigger(context.Background(), removeReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestRemoveLifecycleTrigger_EmptyArray(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	// Workflow has no triggers
	removeReq := connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "wf-test",
		TriggerIndex: 0,
	})

	_, err := server.RemoveLifecycleTrigger(context.Background(), removeReq)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestRemoveLifecycleTrigger_MissingWorkflowID(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	req := connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "",
		TriggerIndex: 0,
	})

	_, err := server.RemoveLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeInvalidArgument)
}

func TestRemoveLifecycleTrigger_WorkflowNotFound(t *testing.T) {
	t.Parallel()
	server, _ := setupLifecycleTriggerTest(t)

	req := connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "wf-nonexistent",
		TriggerIndex: 0,
	})

	_, err := server.RemoveLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodeNotFound)
}

func TestRemoveLifecycleTrigger_BuiltinWorkflow(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	if err := globalDB.SaveWorkflow(&db.Workflow{
		ID:        "wf-builtin",
		Name:      "Builtin",
		IsBuiltin: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "wf-builtin",
		TriggerIndex: 0,
	})

	_, err := server.RemoveLifecycleTrigger(context.Background(), req)
	assertConnectError(t, err, connect.CodePermissionDenied)
}

// =============================================================================
// Cross-endpoint: Builtin workflow rejection (SC-7)
// =============================================================================

func TestLifecycleTrigger_BuiltinWorkflow(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	if err := globalDB.SaveAgent(&db.Agent{
		ID:   "test-agent",
		Name: "Test Agent",
	}); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	if err := globalDB.SaveWorkflow(&db.Workflow{
		ID:        "wf-builtin",
		Name:      "Builtin",
		IsBuiltin: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	mode := "gate"

	// Add should fail
	_, err := server.AddLifecycleTrigger(context.Background(), connect.NewRequest(&orcv1.AddLifecycleTriggerRequest{
		WorkflowId: "wf-builtin",
		Event:      "on_task_completed",
		AgentId:    "test-agent",
		Mode:       &mode,
	}))
	assertConnectError(t, err, connect.CodePermissionDenied)

	// Update should fail
	newMode := "reaction"
	_, err = server.UpdateLifecycleTrigger(context.Background(), connect.NewRequest(&orcv1.UpdateLifecycleTriggerRequest{
		WorkflowId:   "wf-builtin",
		TriggerIndex: 0,
		Mode:         &newMode,
	}))
	assertConnectError(t, err, connect.CodePermissionDenied)

	// Remove should fail
	_, err = server.RemoveLifecycleTrigger(context.Background(), connect.NewRequest(&orcv1.RemoveLifecycleTriggerRequest{
		WorkflowId:   "wf-builtin",
		TriggerIndex: 0,
	}))
	assertConnectError(t, err, connect.CodePermissionDenied)
}
