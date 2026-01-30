package db

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// =============================================================================
// JSON Parse/Marshal Tests for Gate Config Types
// =============================================================================

// TestParseGateInputConfig_RoundTrip verifies GateInputConfig JSON serialization.
// Covers SC-1: GateInputConfig with include_phase_output, include_task, extra_vars.
func TestParseGateInputConfig_RoundTrip(t *testing.T) {
	t.Parallel()

	cfg := GateInputConfig{
		IncludePhaseOutput: []string{"spec", "tdd_write", "implement"},
		IncludeTask:        true,
		ExtraVars:          []string{"CUSTOM_VAR_1", "JIRA_CONTEXT"},
	}

	jsonStr, err := MarshalGateInputConfig(&cfg)
	if err != nil {
		t.Fatalf("MarshalGateInputConfig failed: %v", err)
	}

	parsed, err := ParseGateInputConfig(jsonStr)
	if err != nil {
		t.Fatalf("ParseGateInputConfig failed: %v", err)
	}

	if len(parsed.IncludePhaseOutput) != 3 {
		t.Errorf("IncludePhaseOutput length = %d, want 3", len(parsed.IncludePhaseOutput))
	}
	if parsed.IncludePhaseOutput[0] != "spec" {
		t.Errorf("IncludePhaseOutput[0] = %q, want spec", parsed.IncludePhaseOutput[0])
	}
	if !parsed.IncludeTask {
		t.Error("IncludeTask = false, want true")
	}
	if len(parsed.ExtraVars) != 2 {
		t.Errorf("ExtraVars length = %d, want 2", len(parsed.ExtraVars))
	}
	if parsed.ExtraVars[1] != "JIRA_CONTEXT" {
		t.Errorf("ExtraVars[1] = %q, want JIRA_CONTEXT", parsed.ExtraVars[1])
	}
}

// TestParseGateInputConfig_Empty verifies empty/null input returns nil.
// Covers edge case: Empty GateInputConfig stored as "{}".
func TestParseGateInputConfig_Empty(t *testing.T) {
	t.Parallel()

	// Empty string returns nil
	cfg, err := ParseGateInputConfig("")
	if err != nil {
		t.Fatalf("ParseGateInputConfig empty string failed: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil for empty string, got %+v", cfg)
	}

	// Empty JSON object parses as zero-value struct
	cfg2, err := ParseGateInputConfig("{}")
	if err != nil {
		t.Fatalf("ParseGateInputConfig empty JSON failed: %v", err)
	}
	if cfg2 == nil {
		t.Fatal("expected non-nil for empty JSON object")
	}
	if len(cfg2.IncludePhaseOutput) != 0 {
		t.Errorf("IncludePhaseOutput = %v, want empty", cfg2.IncludePhaseOutput)
	}
	if cfg2.IncludeTask {
		t.Error("IncludeTask = true, want false for zero-value")
	}
	if len(cfg2.ExtraVars) != 0 {
		t.Errorf("ExtraVars = %v, want empty", cfg2.ExtraVars)
	}
}

// TestParseGateInputConfig_InvalidJSON verifies error on invalid JSON.
// Covers failure mode: Invalid JSON in gate_input_config column.
func TestParseGateInputConfig_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseGateInputConfig("{invalid json")
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestParseGateOutputConfig_RoundTrip verifies GateOutputConfig JSON serialization.
// Covers SC-2: GateOutputConfig with variable_name, on_approved, on_rejected,
// retry_from, script.
func TestParseGateOutputConfig_RoundTrip(t *testing.T) {
	t.Parallel()

	cfg := GateOutputConfig{
		VariableName: "GATE_RESULT",
		OnApproved:   "continue",
		OnRejected:   "retry",
		RetryFrom:    "implement",
		Script:       "scripts/validate.sh",
	}

	jsonStr, err := MarshalGateOutputConfig(&cfg)
	if err != nil {
		t.Fatalf("MarshalGateOutputConfig failed: %v", err)
	}

	parsed, err := ParseGateOutputConfig(jsonStr)
	if err != nil {
		t.Fatalf("ParseGateOutputConfig failed: %v", err)
	}

	if parsed.VariableName != "GATE_RESULT" {
		t.Errorf("VariableName = %q, want GATE_RESULT", parsed.VariableName)
	}
	if parsed.OnApproved != "continue" {
		t.Errorf("OnApproved = %q, want continue", parsed.OnApproved)
	}
	if parsed.OnRejected != "retry" {
		t.Errorf("OnRejected = %q, want retry", parsed.OnRejected)
	}
	if parsed.RetryFrom != "implement" {
		t.Errorf("RetryFrom = %q, want implement", parsed.RetryFrom)
	}
	if parsed.Script != "scripts/validate.sh" {
		t.Errorf("Script = %q, want scripts/validate.sh", parsed.Script)
	}
}

// TestParseGateOutputConfig_RetryFromEmptyWithRetryAction verifies edge case:
// on_rejected=retry but retry_from="" is valid at schema level.
func TestParseGateOutputConfig_RetryFromEmptyWithRetryAction(t *testing.T) {
	t.Parallel()

	cfg := GateOutputConfig{
		OnRejected: "retry",
		RetryFrom:  "", // Valid at schema level, executor validates at runtime
	}

	jsonStr, err := MarshalGateOutputConfig(&cfg)
	if err != nil {
		t.Fatalf("MarshalGateOutputConfig failed: %v", err)
	}

	parsed, err := ParseGateOutputConfig(jsonStr)
	if err != nil {
		t.Fatalf("ParseGateOutputConfig failed: %v", err)
	}

	if parsed.OnRejected != "retry" {
		t.Errorf("OnRejected = %q, want retry", parsed.OnRejected)
	}
	if parsed.RetryFrom != "" {
		t.Errorf("RetryFrom = %q, want empty", parsed.RetryFrom)
	}
}

// TestParseGateOutputConfig_Empty verifies empty/null output returns nil.
func TestParseGateOutputConfig_Empty(t *testing.T) {
	t.Parallel()

	cfg, err := ParseGateOutputConfig("")
	if err != nil {
		t.Fatalf("ParseGateOutputConfig empty string failed: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil for empty string, got %+v", cfg)
	}
}

// TestParseGateOutputConfig_InvalidJSON verifies error on invalid JSON.
func TestParseGateOutputConfig_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseGateOutputConfig("not json")
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestMarshalGateInputConfig_Nil verifies nil input marshals to empty string.
func TestMarshalGateInputConfig_Nil(t *testing.T) {
	t.Parallel()

	result, err := MarshalGateInputConfig(nil)
	if err != nil {
		t.Fatalf("MarshalGateInputConfig nil failed: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for nil, got %q", result)
	}
}

// TestMarshalGateOutputConfig_Nil verifies nil output marshals to empty string.
func TestMarshalGateOutputConfig_Nil(t *testing.T) {
	t.Parallel()

	result, err := MarshalGateOutputConfig(nil)
	if err != nil {
		t.Fatalf("MarshalGateOutputConfig nil failed: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for nil, got %q", result)
	}
}

// TestParseBeforeTriggers_RoundTrip verifies BeforePhaseTrigger list JSON serialization.
// Covers SC-6: BeforePhaseTrigger list serialization for workflow_phases.before_triggers.
func TestParseBeforeTriggers_RoundTrip(t *testing.T) {
	t.Parallel()

	triggers := []BeforePhaseTrigger{
		{
			AgentID: "dep-validator",
			InputConfig: &GateInputConfig{
				IncludeTask: true,
			},
			OutputConfig: &GateOutputConfig{
				VariableName: "DEP_RESULT",
				OnApproved:   "continue",
				OnRejected:   "fail",
			},
			Mode: "gate",
		},
		{
			AgentID: "lint-checker",
			Mode:    "reaction",
		},
	}

	jsonStr, err := MarshalBeforeTriggers(triggers)
	if err != nil {
		t.Fatalf("MarshalBeforeTriggers failed: %v", err)
	}

	parsed, err := ParseBeforeTriggers(jsonStr)
	if err != nil {
		t.Fatalf("ParseBeforeTriggers failed: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("parsed length = %d, want 2", len(parsed))
	}
	if parsed[0].AgentID != "dep-validator" {
		t.Errorf("parsed[0].AgentID = %q, want dep-validator", parsed[0].AgentID)
	}
	if parsed[0].InputConfig == nil {
		t.Fatal("parsed[0].InputConfig is nil")
	}
	if !parsed[0].InputConfig.IncludeTask {
		t.Error("parsed[0].InputConfig.IncludeTask = false, want true")
	}
	if parsed[0].Mode != "gate" {
		t.Errorf("parsed[0].Mode = %q, want gate", parsed[0].Mode)
	}
	if parsed[1].AgentID != "lint-checker" {
		t.Errorf("parsed[1].AgentID = %q, want lint-checker", parsed[1].AgentID)
	}
	if parsed[1].Mode != "reaction" {
		t.Errorf("parsed[1].Mode = %q, want reaction", parsed[1].Mode)
	}
}

// TestParseBeforeTriggers_Empty verifies empty/null returns nil slice.
// Covers edge case: BeforePhaseTrigger list on WorkflowPhase with no triggers.
func TestParseBeforeTriggers_Empty(t *testing.T) {
	t.Parallel()

	// Empty string returns nil
	triggers, err := ParseBeforeTriggers("")
	if err != nil {
		t.Fatalf("ParseBeforeTriggers empty failed: %v", err)
	}
	if triggers != nil {
		t.Errorf("expected nil for empty string, got %v", triggers)
	}

	// Empty JSON array
	triggers2, err := ParseBeforeTriggers("[]")
	if err != nil {
		t.Fatalf("ParseBeforeTriggers empty array failed: %v", err)
	}
	if len(triggers2) != 0 {
		t.Errorf("expected empty slice for [], got %d items", len(triggers2))
	}
}

// TestParseBeforeTriggers_InvalidJSON verifies error on invalid JSON.
func TestParseBeforeTriggers_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseBeforeTriggers("not json")
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// TestParseWorkflowTriggers_RoundTrip verifies WorkflowTrigger list JSON serialization.
// Covers SC-10: WorkflowTrigger serialization for workflows.triggers.
func TestParseWorkflowTriggers_RoundTrip(t *testing.T) {
	t.Parallel()

	triggers := []WorkflowTrigger{
		{
			Event:   "on_task_created",
			AgentID: "init-agent",
			InputConfig: &GateInputConfig{
				IncludeTask: true,
			},
			OutputConfig: &GateOutputConfig{
				OnApproved: "continue",
				OnRejected: "fail",
			},
			Mode:    "reaction",
			Enabled: true,
		},
		{
			Event:   "on_task_failed",
			AgentID: "notify-agent",
			Mode:    "reaction",
			Enabled: false,
		},
	}

	jsonStr, err := MarshalWorkflowTriggers(triggers)
	if err != nil {
		t.Fatalf("MarshalWorkflowTriggers failed: %v", err)
	}

	parsed, err := ParseWorkflowTriggers(jsonStr)
	if err != nil {
		t.Fatalf("ParseWorkflowTriggers failed: %v", err)
	}

	if len(parsed) != 2 {
		t.Fatalf("parsed length = %d, want 2", len(parsed))
	}
	if parsed[0].Event != "on_task_created" {
		t.Errorf("parsed[0].Event = %q, want on_task_created", parsed[0].Event)
	}
	if parsed[0].AgentID != "init-agent" {
		t.Errorf("parsed[0].AgentID = %q, want init-agent", parsed[0].AgentID)
	}
	if !parsed[0].Enabled {
		t.Error("parsed[0].Enabled = false, want true")
	}
	if parsed[1].Enabled {
		t.Error("parsed[1].Enabled = true, want false")
	}
}

// TestParseWorkflowTriggers_Empty verifies empty/null returns nil.
func TestParseWorkflowTriggers_Empty(t *testing.T) {
	t.Parallel()

	triggers, err := ParseWorkflowTriggers("")
	if err != nil {
		t.Fatalf("ParseWorkflowTriggers empty failed: %v", err)
	}
	if triggers != nil {
		t.Errorf("expected nil for empty string, got %v", triggers)
	}
}

// TestParseWorkflowTriggers_UnknownEvent verifies unknown event type parses without error.
// Covers edge case: WorkflowTrigger with unknown event type.
func TestParseWorkflowTriggers_UnknownEvent(t *testing.T) {
	t.Parallel()

	jsonStr := `[{"event":"on_unknown_event","agent_id":"test","enabled":true}]`
	triggers, err := ParseWorkflowTriggers(jsonStr)
	if err != nil {
		t.Fatalf("ParseWorkflowTriggers unknown event failed: %v", err)
	}
	if len(triggers) != 1 {
		t.Fatalf("len(triggers) = %d, want 1", len(triggers))
	}
	if triggers[0].Event != "on_unknown_event" {
		t.Errorf("Event = %q, want on_unknown_event", triggers[0].Event)
	}
}

// =============================================================================
// DB Migration Tests
// =============================================================================

// TestMigration005_AddsGateConfigColumns verifies global_005.sql adds new columns.
// Covers SC-8: gate_input_config, gate_output_config, gate_mode, gate_agent_id
// on phase_templates table.
func TestMigration005_AddsGateConfigColumns(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	// Verify phase_templates has new columns
	phaseTemplateCols := []string{
		"gate_input_config",
		"gate_output_config",
		"gate_mode",
		"gate_agent_id",
	}
	for _, col := range phaseTemplateCols {
		var colCount int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM pragma_table_info('phase_templates')
			WHERE name = ?
		`, col).Scan(&colCount)
		if err != nil {
			t.Fatalf("check column %s: %v", col, err)
		}
		if colCount != 1 {
			t.Errorf("phase_templates column %s count = %d, want 1", col, colCount)
		}
	}
}

// TestMigration005_AddsBeforeTriggersColumn verifies workflow_phases gets before_triggers.
// Covers SC-9: before_triggers column on workflow_phases table.
func TestMigration005_AddsBeforeTriggersColumn(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	var colCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('workflow_phases')
		WHERE name = 'before_triggers'
	`).Scan(&colCount)
	if err != nil {
		t.Fatalf("check before_triggers column: %v", err)
	}
	if colCount != 1 {
		t.Errorf("workflow_phases before_triggers column count = %d, want 1", colCount)
	}
}

// TestMigration005_AddsWorkflowTriggersColumn verifies workflows gets triggers.
// Covers SC-10: triggers column on workflows table.
func TestMigration005_AddsWorkflowTriggersColumn(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	var colCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('workflows')
		WHERE name = 'triggers'
	`).Scan(&colCount)
	if err != nil {
		t.Fatalf("check triggers column: %v", err)
	}
	if colCount != 1 {
		t.Errorf("workflows triggers column count = %d, want 1", colCount)
	}
}

// TestMigration005_Idempotent verifies migration can run twice without error.
// Covers BDD-1: migration is idempotent.
func TestMigration005_Idempotent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("First Migrate failed: %v", err)
	}
	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Second Migrate failed: %v", err)
	}
}

// TestMigration005_PreservesExistingRows verifies existing phase_templates rows survive.
// Covers BDD-1: existing rows preserved with NULL defaults for new columns.
func TestMigration005_PreservesExistingRows(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Insert a phase template (uses the full schema including new columns)
	pt := &PhaseTemplate{
		ID:        "test-existing",
		Name:      "Test Existing",
		GateType:  "auto",
		IsBuiltin: false,
	}
	if err := gdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	// Retrieve it - new fields should be nil/empty (NULL defaults)
	got, err := gdb.GetPhaseTemplate("test-existing")
	if err != nil {
		t.Fatalf("GetPhaseTemplate failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetPhaseTemplate returned nil")
	}
	if got.Name != "Test Existing" {
		t.Errorf("Name = %q, want Test Existing", got.Name)
	}
	if got.GateType != "auto" {
		t.Errorf("GateType = %q, want auto", got.GateType)
	}
	// New fields should be zero-value/nil
	if got.GateInputConfig != "" {
		t.Errorf("GateInputConfig = %q, want empty", got.GateInputConfig)
	}
	if got.GateOutputConfig != "" {
		t.Errorf("GateOutputConfig = %q, want empty", got.GateOutputConfig)
	}
	if got.GateMode != "" && got.GateMode != "gate" {
		t.Errorf("GateMode = %q, want empty or 'gate'", got.GateMode)
	}
	if got.GateAgentID != "" {
		t.Errorf("GateAgentID = %q, want empty", got.GateAgentID)
	}
}

// =============================================================================
// PhaseTemplate CRUD with New Gate Config Fields
// =============================================================================

// TestPhaseTemplate_SaveAndGetWithGateConfig verifies full CRUD with gate config fields.
// Covers SC-5, SC-8, SC-11: PhaseTemplate CRUD with gate_input_config,
// gate_output_config, gate_mode, gate_agent_id.
func TestPhaseTemplate_SaveAndGetWithGateConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create agent first (foreign key for gate_agent_id)
	_, err = db.Exec(`INSERT INTO agents (id, name, description, prompt, is_builtin)
		VALUES ('review-agent', 'Review Agent', 'AI gate reviewer', 'Review prompt', FALSE)`)
	if err != nil {
		t.Fatalf("create agent failed: %v", err)
	}

	inputJSON := `{"include_phase_output":["implement"],"include_task":true,"extra_vars":["CUSTOM_VAR"]}`
	outputJSON := `{"variable_name":"REVIEW_RESULT","on_approved":"continue","on_rejected":"retry","retry_from":"implement"}`

	pt := &PhaseTemplate{
		ID:               "ai-review",
		Name:             "AI Review Gate",
		GateType:         "ai",
		GateMode:         "gate",
		GateAgentID:      "review-agent",
		GateInputConfig:  inputJSON,
		GateOutputConfig: outputJSON,
		IsBuiltin:        false,
	}

	if err := gdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	// Retrieve and verify
	got, err := gdb.GetPhaseTemplate("ai-review")
	if err != nil {
		t.Fatalf("GetPhaseTemplate failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetPhaseTemplate returned nil")
	}
	if got.GateType != "ai" {
		t.Errorf("GateType = %q, want ai", got.GateType)
	}
	if got.GateMode != "gate" {
		t.Errorf("GateMode = %q, want gate", got.GateMode)
	}
	if got.GateAgentID != "review-agent" {
		t.Errorf("GateAgentID = %q, want review-agent", got.GateAgentID)
	}

	// Verify JSON round-trip of gate configs
	var parsedInput struct {
		IncludePhaseOutput []string `json:"include_phase_output"`
		IncludeTask        bool     `json:"include_task"`
		ExtraVars          []string `json:"extra_vars"`
	}
	if err := json.Unmarshal([]byte(got.GateInputConfig), &parsedInput); err != nil {
		t.Fatalf("parse gate input config: %v", err)
	}
	if !parsedInput.IncludeTask {
		t.Error("parsed input IncludeTask = false, want true")
	}
	if len(parsedInput.IncludePhaseOutput) != 1 || parsedInput.IncludePhaseOutput[0] != "implement" {
		t.Errorf("parsed input IncludePhaseOutput = %v, want [implement]", parsedInput.IncludePhaseOutput)
	}

	var parsedOutput struct {
		VariableName string `json:"variable_name"`
		OnApproved   string `json:"on_approved"`
		OnRejected   string `json:"on_rejected"`
		RetryFrom    string `json:"retry_from"`
	}
	if err := json.Unmarshal([]byte(got.GateOutputConfig), &parsedOutput); err != nil {
		t.Fatalf("parse gate output config: %v", err)
	}
	if parsedOutput.VariableName != "REVIEW_RESULT" {
		t.Errorf("parsed output VariableName = %q, want REVIEW_RESULT", parsedOutput.VariableName)
	}
	if parsedOutput.OnRejected != "retry" {
		t.Errorf("parsed output OnRejected = %q, want retry", parsedOutput.OnRejected)
	}
}

// TestPhaseTemplate_AIGateWithoutAgentID verifies saving AI gate type without agent_id.
// Covers edge case: gate_type=AI but no gate_agent_id (executor validates at runtime).
func TestPhaseTemplate_AIGateWithoutAgentID(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	pt := &PhaseTemplate{
		ID:       "ai-gate-no-agent",
		Name:     "AI Gate No Agent",
		GateType: "ai",
		GateMode: "gate",
		// No GateAgentID — valid at schema level
	}

	if err := gdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	got, err := gdb.GetPhaseTemplate("ai-gate-no-agent")
	if err != nil {
		t.Fatalf("GetPhaseTemplate failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetPhaseTemplate returned nil")
	}
	if got.GateType != "ai" {
		t.Errorf("GateType = %q, want ai", got.GateType)
	}
	if got.GateAgentID != "" {
		t.Errorf("GateAgentID = %q, want empty", got.GateAgentID)
	}
}

// TestPhaseTemplate_UpdateGateConfig verifies updating gate config on existing template.
func TestPhaseTemplate_UpdateGateConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create with no gate config
	pt := &PhaseTemplate{
		ID:       "update-gate-test",
		Name:     "Update Gate Test",
		GateType: "auto",
	}
	if err := gdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate create failed: %v", err)
	}

	// Update with gate config
	inputJSON := `{"include_phase_output":["spec"],"include_task":true}`
	pt.GateType = "ai"
	pt.GateMode = "gate"
	pt.GateInputConfig = inputJSON
	if err := gdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate update failed: %v", err)
	}

	got, err := gdb.GetPhaseTemplate("update-gate-test")
	if err != nil {
		t.Fatalf("GetPhaseTemplate failed: %v", err)
	}
	if got.GateType != "ai" {
		t.Errorf("GateType after update = %q, want ai", got.GateType)
	}
	if got.GateMode != "gate" {
		t.Errorf("GateMode after update = %q, want gate", got.GateMode)
	}
	if got.GateInputConfig != inputJSON {
		t.Errorf("GateInputConfig after update = %q, want %q", got.GateInputConfig, inputJSON)
	}
}

// =============================================================================
// WorkflowPhase with BeforeTriggers
// =============================================================================

// TestWorkflowPhase_SaveAndGetWithBeforeTriggers verifies before_triggers storage.
// Covers SC-6, SC-9: WorkflowPhase before_triggers column and JSON round-trip.
func TestWorkflowPhase_SaveAndGetWithBeforeTriggers(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create prerequisites: workflow and phase template
	wf := &Workflow{
		ID:   "test-wf-bt",
		Name: "Test Workflow BT",
	}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	pt := &PhaseTemplate{
		ID:   "test-phase-bt",
		Name: "Test Phase BT",
	}
	if err := gdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	// Create workflow phase with before_triggers
	triggersJSON := `[{"agent_id":"dep-check","input_config":{"include_task":true},"mode":"gate"}]`

	phase := &WorkflowPhase{
		WorkflowID:      "test-wf-bt",
		PhaseTemplateID: "test-phase-bt",
		Sequence:        0,
		BeforeTriggers:  triggersJSON,
	}
	if err := gdb.AddWorkflowPhase(phase); err != nil {
		t.Fatalf("AddWorkflowPhase failed: %v", err)
	}

	// Retrieve workflow with phases
	gotWf, err := gdb.GetWorkflow("test-wf-bt")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if gotWf == nil {
		t.Fatal("GetWorkflow returned nil")
	}
	if len(gotWf.Phases) != 1 {
		t.Fatalf("phases count = %d, want 1", len(gotWf.Phases))
	}

	gotPhase := gotWf.Phases[0]
	if gotPhase.BeforeTriggers == "" {
		t.Fatal("BeforeTriggers is empty")
	}

	// Verify JSON round-trip
	var parsedTriggers []struct {
		AgentID string `json:"agent_id"`
		Mode    string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(gotPhase.BeforeTriggers), &parsedTriggers); err != nil {
		t.Fatalf("unmarshal before_triggers: %v", err)
	}
	if len(parsedTriggers) != 1 {
		t.Fatalf("parsed triggers count = %d, want 1", len(parsedTriggers))
	}
	if parsedTriggers[0].AgentID != "dep-check" {
		t.Errorf("trigger AgentID = %q, want dep-check", parsedTriggers[0].AgentID)
	}
}

// TestWorkflowPhase_EmptyBeforeTriggers verifies NULL/empty before_triggers.
// Covers edge case: WorkflowPhase with no before triggers.
func TestWorkflowPhase_EmptyBeforeTriggers(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create prerequisites
	wf := &Workflow{ID: "wf-empty-bt", Name: "Empty BT WF"}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}
	pt := &PhaseTemplate{ID: "pt-empty-bt", Name: "Empty BT Phase"}
	if err := gdb.SavePhaseTemplate(pt); err != nil {
		t.Fatalf("SavePhaseTemplate failed: %v", err)
	}

	// Create phase with no before_triggers
	phase := &WorkflowPhase{
		WorkflowID:      "wf-empty-bt",
		PhaseTemplateID: "pt-empty-bt",
		Sequence:        0,
		// BeforeTriggers not set
	}
	if err := gdb.AddWorkflowPhase(phase); err != nil {
		t.Fatalf("AddWorkflowPhase failed: %v", err)
	}

	gotWf, err := gdb.GetWorkflow("wf-empty-bt")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if len(gotWf.Phases) != 1 {
		t.Fatalf("phases count = %d, want 1", len(gotWf.Phases))
	}
	// Empty before_triggers should be empty string or null
	if gotWf.Phases[0].BeforeTriggers != "" {
		t.Errorf("BeforeTriggers = %q, want empty", gotWf.Phases[0].BeforeTriggers)
	}
}

// =============================================================================
// Workflow with Triggers
// =============================================================================

// TestWorkflow_SaveAndGetWithTriggers verifies workflow-level trigger storage.
// Covers SC-10: Workflow triggers field with WorkflowTrigger messages.
func TestWorkflow_SaveAndGetWithTriggers(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	triggersJSON := `[` +
		`{"event":"on_task_created","agent_id":"init-checker","mode":"reaction","enabled":true},` +
		`{"event":"on_task_completed","agent_id":"cleanup-agent","mode":"reaction","enabled":true},` +
		`{"event":"on_task_failed","agent_id":"notify-agent","mode":"reaction","enabled":false},` +
		`{"event":"on_initiative_planned","agent_id":"plan-reviewer","mode":"gate","enabled":true}` +
		`]`

	wf := &Workflow{
		ID:       "wf-with-triggers",
		Name:     "Workflow With Triggers",
		Triggers: triggersJSON,
	}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	got, err := gdb.GetWorkflow("wf-with-triggers")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetWorkflow returned nil")
	}
	if got.Triggers == "" {
		t.Fatal("Triggers is empty")
	}

	// Verify all 4 trigger events round-trip
	var parsedTriggers []struct {
		Event   string `json:"event"`
		AgentID string `json:"agent_id"`
		Mode    string `json:"mode"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(got.Triggers), &parsedTriggers); err != nil {
		t.Fatalf("unmarshal triggers: %v", err)
	}
	if len(parsedTriggers) != 4 {
		t.Fatalf("parsed triggers count = %d, want 4", len(parsedTriggers))
	}

	expectedEvents := []string{
		"on_task_created",
		"on_task_completed",
		"on_task_failed",
		"on_initiative_planned",
	}
	for i, expected := range expectedEvents {
		if parsedTriggers[i].Event != expected {
			t.Errorf("trigger[%d].Event = %q, want %q", i, parsedTriggers[i].Event, expected)
		}
	}
}

// TestWorkflow_EmptyTriggers verifies workflow with no triggers.
func TestWorkflow_EmptyTriggers(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	wf := &Workflow{
		ID:   "wf-no-triggers",
		Name: "No Triggers WF",
		// Triggers not set
	}
	if err := gdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("SaveWorkflow failed: %v", err)
	}

	got, err := gdb.GetWorkflow("wf-no-triggers")
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetWorkflow returned nil")
	}
	if got.Triggers != "" {
		t.Errorf("Triggers = %q, want empty", got.Triggers)
	}
}

// =============================================================================
// PhaseTemplate List with New Fields
// =============================================================================

// TestPhaseTemplate_ListIncludesGateConfigFields verifies ListPhaseTemplates returns
// gate config fields for all templates.
// Covers SC-11: Go structs updated, existing tests pass.
func TestPhaseTemplate_ListIncludesGateConfigFields(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create templates: one with gate config, one without
	pt1 := &PhaseTemplate{
		ID:              "with-gate-config",
		Name:            "With Gate Config",
		GateType:        "ai",
		GateMode:        "gate",
		GateInputConfig: `{"include_task":true}`,
	}
	pt2 := &PhaseTemplate{
		ID:       "without-gate-config",
		Name:     "Without Gate Config",
		GateType: "auto",
	}
	if err := gdb.SavePhaseTemplate(pt1); err != nil {
		t.Fatalf("SavePhaseTemplate pt1 failed: %v", err)
	}
	if err := gdb.SavePhaseTemplate(pt2); err != nil {
		t.Fatalf("SavePhaseTemplate pt2 failed: %v", err)
	}

	// List all templates
	templates, err := gdb.ListPhaseTemplates()
	if err != nil {
		t.Fatalf("ListPhaseTemplates failed: %v", err)
	}
	if len(templates) < 2 {
		t.Fatalf("templates count = %d, want >= 2", len(templates))
	}

	// Find our templates and verify fields
	var foundWithConfig, foundWithoutConfig bool
	for _, tmpl := range templates {
		switch tmpl.ID {
		case "with-gate-config":
			foundWithConfig = true
			if tmpl.GateMode != "gate" {
				t.Errorf("with-gate-config GateMode = %q, want gate", tmpl.GateMode)
			}
			if tmpl.GateInputConfig == "" {
				t.Error("with-gate-config GateInputConfig is empty")
			}
		case "without-gate-config":
			foundWithoutConfig = true
			if tmpl.GateMode != "" && tmpl.GateMode != "gate" {
				t.Errorf("without-gate-config GateMode = %q, want empty or 'gate'", tmpl.GateMode)
			}
			if tmpl.GateInputConfig != "" {
				t.Errorf("without-gate-config GateInputConfig = %q, want empty", tmpl.GateInputConfig)
			}
		}
	}
	if !foundWithConfig {
		t.Error("with-gate-config template not found in list")
	}
	if !foundWithoutConfig {
		t.Error("without-gate-config template not found in list")
	}
}
