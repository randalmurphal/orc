package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-3: GetInitiative API returns criteria in response
// ============================================================================

// TestGetInitiative_ReturnsCriteria verifies SC-3:
// GetInitiative response includes populated criteria repeated field.
func TestGetInitiative_ReturnsCriteria(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create initiative with criteria via proto path
	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init.Criteria = []*orcv1.Criterion{
		{
			Id:          "AC-001",
			Description: "User can log in",
			Status:      "covered",
			TaskIds:     []string{"TASK-001"},
		},
		{
			Id:          "AC-002",
			Description: "User can log out",
			Status:      "uncovered",
			TaskIds:     []string{},
		},
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	resp, err := server.GetInitiative(context.Background(), connect.NewRequest(&orcv1.GetInitiativeRequest{
		InitiativeId: "INIT-001",
	}))
	if err != nil {
		t.Fatalf("GetInitiative failed: %v", err)
	}

	if resp.Msg.Initiative == nil {
		t.Fatal("response initiative is nil")
	}

	// SC-3: Criteria should be present in the response
	if len(resp.Msg.Initiative.Criteria) != 2 {
		t.Fatalf("Criteria len = %d, want 2", len(resp.Msg.Initiative.Criteria))
	}

	// Verify criteria content
	c1 := findProtoCriterionInList(resp.Msg.Initiative.Criteria, "AC-001")
	if c1 == nil {
		t.Fatal("AC-001 not found in response")
	}
	if c1.Description != "User can log in" {
		t.Errorf("AC-001 Description = %q, want %q", c1.Description, "User can log in")
	}
	if c1.Status != "covered" {
		t.Errorf("AC-001 Status = %q, want %q", c1.Status, "covered")
	}
}

// TestGetInitiative_NoCriteria verifies GetInitiative works with no criteria.
func TestGetInitiative_NoCriteria(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	resp, err := server.GetInitiative(context.Background(), connect.NewRequest(&orcv1.GetInitiativeRequest{
		InitiativeId: "INIT-001",
	}))
	if err != nil {
		t.Fatalf("GetInitiative failed: %v", err)
	}

	// No criteria should be present (empty, not nil)
	if len(resp.Msg.Initiative.Criteria) != 0 {
		t.Errorf("Criteria len = %d, want 0", len(resp.Msg.Initiative.Criteria))
	}
}

// ============================================================================
// SC-4: AddCriterion API adds a criterion and returns updated initiative
// ============================================================================

// TestAddCriterion_Success verifies SC-4:
// AddCriterion creates criterion with auto-generated ID and "uncovered" status.
func TestAddCriterion_Success(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	resp, err := server.AddCriterion(context.Background(), connect.NewRequest(&orcv1.AddCriterionRequest{
		InitiativeId: "INIT-001",
		Description:  "User can log in with JWT",
	}))
	if err != nil {
		t.Fatalf("AddCriterion failed: %v", err)
	}

	if resp.Msg.Initiative == nil {
		t.Fatal("response initiative is nil")
	}

	// Should have one criterion
	if len(resp.Msg.Initiative.Criteria) != 1 {
		t.Fatalf("Criteria len = %d, want 1", len(resp.Msg.Initiative.Criteria))
	}

	c := resp.Msg.Initiative.Criteria[0]

	// Auto-generated ID in AC-NNN format
	if c.Id != "AC-001" {
		t.Errorf("criterion ID = %q, want %q", c.Id, "AC-001")
	}

	// Description matches
	if c.Description != "User can log in with JWT" {
		t.Errorf("Description = %q, want %q", c.Description, "User can log in with JWT")
	}

	// Default status is uncovered
	if c.Status != "uncovered" {
		t.Errorf("Status = %q, want %q", c.Status, "uncovered")
	}

	// Verify persistence
	loaded, err := backend.LoadInitiativeProto("INIT-001")
	if err != nil {
		t.Fatalf("LoadInitiativeProto() error = %v", err)
	}
	if len(loaded.Criteria) != 1 {
		t.Errorf("persisted Criteria len = %d, want 1", len(loaded.Criteria))
	}
}

// TestAddCriterion_EmptyDescription verifies SC-4 error path:
// Empty description returns CodeInvalidArgument.
func TestAddCriterion_EmptyDescription(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	_, err := server.AddCriterion(context.Background(), connect.NewRequest(&orcv1.AddCriterionRequest{
		InitiativeId: "INIT-001",
		Description:  "",
	}))
	if err == nil {
		t.Fatal("expected error for empty description")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
	}
}

// TestAddCriterion_InitiativeNotFound verifies SC-4 error path:
// Non-existent initiative returns CodeNotFound.
func TestAddCriterion_InitiativeNotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewInitiativeServer(backend, nil, nil)

	_, err := server.AddCriterion(context.Background(), connect.NewRequest(&orcv1.AddCriterionRequest{
		InitiativeId: "INIT-NONEXISTENT",
		Description:  "Some criterion",
	}))
	if err == nil {
		t.Fatal("expected error for non-existent initiative")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

// TestAddCriterion_MultipleCriteria verifies sequential ID generation.
func TestAddCriterion_MultipleCriteria(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Add first criterion
	resp1, err := server.AddCriterion(context.Background(), connect.NewRequest(&orcv1.AddCriterionRequest{
		InitiativeId: "INIT-001",
		Description:  "First criterion",
	}))
	if err != nil {
		t.Fatalf("first AddCriterion failed: %v", err)
	}

	// Add second criterion
	resp2, err := server.AddCriterion(context.Background(), connect.NewRequest(&orcv1.AddCriterionRequest{
		InitiativeId: "INIT-001",
		Description:  "Second criterion",
	}))
	if err != nil {
		t.Fatalf("second AddCriterion failed: %v", err)
	}

	// Verify sequential IDs
	if len(resp1.Msg.Initiative.Criteria) != 1 {
		t.Fatalf("after first add: Criteria len = %d, want 1", len(resp1.Msg.Initiative.Criteria))
	}
	if resp1.Msg.Initiative.Criteria[0].Id != "AC-001" {
		t.Errorf("first criterion ID = %q, want AC-001", resp1.Msg.Initiative.Criteria[0].Id)
	}

	if len(resp2.Msg.Initiative.Criteria) != 2 {
		t.Fatalf("after second add: Criteria len = %d, want 2", len(resp2.Msg.Initiative.Criteria))
	}
	// Find AC-002 in the response
	found := false
	for _, c := range resp2.Msg.Initiative.Criteria {
		if c.Id == "AC-002" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AC-002 not found in response after second add")
	}
}

// ============================================================================
// SC-5: MapCriterionToTask API links task to criterion
// ============================================================================

// TestMapCriterionToTask_Success verifies SC-5:
// Mapping a task transitions status from uncovered to covered.
func TestMapCriterionToTask_Success(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init.Criteria = []*orcv1.Criterion{
		{Id: "AC-001", Description: "Test criterion", Status: "uncovered", TaskIds: []string{}},
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task to map
	tk := task.NewProtoTask("TASK-001", "Test Task")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	resp, err := server.MapCriterionToTask(context.Background(), connect.NewRequest(&orcv1.MapCriterionToTaskRequest{
		InitiativeId: "INIT-001",
		CriterionId:  "AC-001",
		TaskId:       "TASK-001",
	}))
	if err != nil {
		t.Fatalf("MapCriterionToTask failed: %v", err)
	}

	// Verify criterion has the task and status changed
	c := findProtoCriterionInList(resp.Msg.Initiative.Criteria, "AC-001")
	if c == nil {
		t.Fatal("AC-001 not found in response")
	}
	if len(c.TaskIds) != 1 || c.TaskIds[0] != "TASK-001" {
		t.Errorf("TaskIds = %v, want [TASK-001]", c.TaskIds)
	}
	if c.Status != "covered" {
		t.Errorf("Status = %q, want %q after mapping", c.Status, "covered")
	}
}

// TestMapCriterionToTask_Duplicate verifies BDD-3:
// Duplicate mapping is idempotent.
func TestMapCriterionToTask_Duplicate(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init.Criteria = []*orcv1.Criterion{
		{Id: "AC-001", Description: "Test", Status: "covered", TaskIds: []string{"TASK-001"}},
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	tk := task.NewProtoTask("TASK-001", "Test Task")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	// Map same task again
	resp, err := server.MapCriterionToTask(context.Background(), connect.NewRequest(&orcv1.MapCriterionToTaskRequest{
		InitiativeId: "INIT-001",
		CriterionId:  "AC-001",
		TaskId:       "TASK-001",
	}))
	if err != nil {
		t.Fatalf("duplicate MapCriterionToTask should succeed: %v", err)
	}

	c := findProtoCriterionInList(resp.Msg.Initiative.Criteria, "AC-001")
	if c == nil {
		t.Fatal("AC-001 not found")
	}
	// Should still have exactly one task (no duplicate)
	if len(c.TaskIds) != 1 {
		t.Errorf("TaskIds len = %d, want 1 (no duplicate)", len(c.TaskIds))
	}
}

// TestMapCriterionToTask_NotFound verifies SC-5 error path:
// Invalid criterion_id returns CodeNotFound.
func TestMapCriterionToTask_NotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	_, err := server.MapCriterionToTask(context.Background(), connect.NewRequest(&orcv1.MapCriterionToTaskRequest{
		InitiativeId: "INIT-001",
		CriterionId:  "AC-999",
		TaskId:       "TASK-001",
	}))
	if err == nil {
		t.Fatal("expected error for non-existent criterion")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

// ============================================================================
// SC-6: GetCoverageReport API returns coverage statistics
// ============================================================================

// TestGetCoverageReport_MixedStatuses verifies BDD-1 and SC-6:
// Coverage report with mixed statuses returns correct counts.
func TestGetCoverageReport_MixedStatuses(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init.Criteria = []*orcv1.Criterion{
		{Id: "AC-001", Description: "Uncovered one", Status: "uncovered", TaskIds: []string{}},
		{Id: "AC-002", Description: "Covered one", Status: "covered", TaskIds: []string{"TASK-001"}},
		{Id: "AC-003", Description: "Satisfied one", Status: "satisfied", TaskIds: []string{"TASK-002"}, Evidence: "Tests pass"},
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	resp, err := server.GetCoverageReport(context.Background(), connect.NewRequest(&orcv1.GetCoverageReportRequest{
		InitiativeId: "INIT-001",
	}))
	if err != nil {
		t.Fatalf("GetCoverageReport failed: %v", err)
	}

	report := resp.Msg.Report
	if report == nil {
		t.Fatal("report is nil")
	}

	if report.Total != 3 {
		t.Errorf("Total = %d, want 3", report.Total)
	}
	if report.Uncovered != 1 {
		t.Errorf("Uncovered = %d, want 1", report.Uncovered)
	}
	if report.Covered != 1 {
		t.Errorf("Covered = %d, want 1", report.Covered)
	}
	if report.Satisfied != 1 {
		t.Errorf("Satisfied = %d, want 1", report.Satisfied)
	}
	if report.Regressed != 0 {
		t.Errorf("Regressed = %d, want 0", report.Regressed)
	}
	if len(report.Criteria) != 3 {
		t.Errorf("Criteria in report = %d, want 3", len(report.Criteria))
	}
}

// TestGetCoverageReport_NoCriteria verifies SC-6 edge case:
// Initiative with no criteria returns zeroed report.
func TestGetCoverageReport_NoCriteria(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	resp, err := server.GetCoverageReport(context.Background(), connect.NewRequest(&orcv1.GetCoverageReportRequest{
		InitiativeId: "INIT-001",
	}))
	if err != nil {
		t.Fatalf("GetCoverageReport failed: %v", err)
	}

	report := resp.Msg.Report
	if report == nil {
		t.Fatal("report is nil")
	}

	if report.Total != 0 {
		t.Errorf("Total = %d, want 0", report.Total)
	}
	if len(report.Criteria) != 0 {
		t.Errorf("Criteria = %d, want 0", len(report.Criteria))
	}
}

// TestGetCoverageReport_InitiativeNotFound verifies error handling.
func TestGetCoverageReport_InitiativeNotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewInitiativeServer(backend, nil, nil)

	_, err := server.GetCoverageReport(context.Background(), connect.NewRequest(&orcv1.GetCoverageReportRequest{
		InitiativeId: "INIT-NONEXISTENT",
	}))
	if err == nil {
		t.Fatal("expected error for non-existent initiative")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

// ============================================================================
// VerifyCriterion API tests
// ============================================================================

// TestVerifyCriterion_Satisfied verifies that status changes to satisfied with evidence.
func TestVerifyCriterion_Satisfied(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init.Criteria = []*orcv1.Criterion{
		{Id: "AC-001", Description: "Test", Status: "covered", TaskIds: []string{"TASK-001"}},
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	resp, err := server.VerifyCriterion(context.Background(), connect.NewRequest(&orcv1.VerifyCriterionRequest{
		InitiativeId: "INIT-001",
		CriterionId:  "AC-001",
		Status:       "satisfied",
		Evidence:     "E2E test passes",
	}))
	if err != nil {
		t.Fatalf("VerifyCriterion failed: %v", err)
	}

	c := findProtoCriterionInList(resp.Msg.Initiative.Criteria, "AC-001")
	if c == nil {
		t.Fatal("AC-001 not found")
	}
	if c.Status != "satisfied" {
		t.Errorf("Status = %q, want %q", c.Status, "satisfied")
	}
	if c.Evidence != "E2E test passes" {
		t.Errorf("Evidence = %q, want %q", c.Evidence, "E2E test passes")
	}
	if c.VerifiedAt == "" {
		t.Error("VerifiedAt should be set")
	}
}

// TestVerifyCriterion_InvalidStatus verifies error path.
func TestVerifyCriterion_InvalidStatus(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Criteria = []*orcv1.Criterion{
		{Id: "AC-001", Description: "Test", Status: "uncovered", TaskIds: []string{}},
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	_, err := server.VerifyCriterion(context.Background(), connect.NewRequest(&orcv1.VerifyCriterionRequest{
		InitiativeId: "INIT-001",
		CriterionId:  "AC-001",
		Status:       "invalid_status",
		Evidence:     "test",
	}))
	if err == nil {
		t.Fatal("expected error for invalid status")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
	}
}

// TestVerifyCriterion_CriterionNotFound verifies error path.
func TestVerifyCriterion_CriterionNotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	_, err := server.VerifyCriterion(context.Background(), connect.NewRequest(&orcv1.VerifyCriterionRequest{
		InitiativeId: "INIT-001",
		CriterionId:  "AC-999",
		Status:       "satisfied",
		Evidence:     "test",
	}))
	if err == nil {
		t.Fatal("expected error for non-existent criterion")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

// ============================================================================
// RemoveCriterion API tests
// ============================================================================

// TestRemoveCriterion_Success verifies removing a criterion.
func TestRemoveCriterion_Success(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	init.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	init.Criteria = []*orcv1.Criterion{
		{Id: "AC-001", Description: "First", Status: "uncovered", TaskIds: []string{}},
		{Id: "AC-002", Description: "Second", Status: "uncovered", TaskIds: []string{}},
	}
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	resp, err := server.RemoveCriterion(context.Background(), connect.NewRequest(&orcv1.RemoveCriterionRequest{
		InitiativeId: "INIT-001",
		CriterionId:  "AC-001",
	}))
	if err != nil {
		t.Fatalf("RemoveCriterion failed: %v", err)
	}

	if len(resp.Msg.Initiative.Criteria) != 1 {
		t.Fatalf("Criteria len = %d, want 1", len(resp.Msg.Initiative.Criteria))
	}
	if resp.Msg.Initiative.Criteria[0].Id != "AC-002" {
		t.Errorf("remaining criterion ID = %q, want AC-002", resp.Msg.Initiative.Criteria[0].Id)
	}
}

// TestRemoveCriterion_NotFound verifies error path.
func TestRemoveCriterion_NotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	init := initiative.NewProtoInitiative("INIT-001", "Test")
	if err := backend.SaveInitiativeProto(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewInitiativeServer(backend, nil, nil)

	_, err := server.RemoveCriterion(context.Background(), connect.NewRequest(&orcv1.RemoveCriterionRequest{
		InitiativeId: "INIT-001",
		CriterionId:  "AC-999",
	}))
	if err == nil {
		t.Fatal("expected error for non-existent criterion")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}

// ============================================================================
// Test helpers
// ============================================================================

func findProtoCriterionInList(criteria []*orcv1.Criterion, id string) *orcv1.Criterion {
	for _, c := range criteria {
		if c.Id == id {
			return c
		}
	}
	return nil
}
