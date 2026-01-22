package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDecisionRequiredEventStruct(t *testing.T) {
	now := time.Now()
	data := DecisionRequiredData{
		DecisionID:  "gate_TASK-001_review",
		TaskID:      "TASK-001",
		TaskTitle:   "Test Task",
		Phase:       "review",
		GateType:    "human",
		Question:    "Approve?",
		Context:     "Test context",
		RequestedAt: now,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded DecisionRequiredData
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.DecisionID != "gate_TASK-001_review" {
		t.Errorf("expected DecisionID gate_TASK-001_review, got %s", decoded.DecisionID)
	}
	if decoded.TaskID != "TASK-001" {
		t.Errorf("expected TaskID TASK-001, got %s", decoded.TaskID)
	}
	if decoded.Phase != "review" {
		t.Errorf("expected Phase review, got %s", decoded.Phase)
	}
}

func TestDecisionResolvedEventStruct(t *testing.T) {
	now := time.Now()
	data := DecisionResolvedData{
		DecisionID: "gate_TASK-001_review",
		TaskID:     "TASK-001",
		Phase:      "review",
		Approved:   true,
		Reason:     "LGTM",
		ResolvedBy: "api",
		ResolvedAt: now,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Unmarshal back
	var decoded DecisionResolvedData
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify fields
	if decoded.DecisionID != "gate_TASK-001_review" {
		t.Errorf("expected DecisionID gate_TASK-001_review, got %s", decoded.DecisionID)
	}
	if decoded.Approved != true {
		t.Error("expected Approved true")
	}
	if decoded.ResolvedBy != "api" {
		t.Errorf("expected ResolvedBy api, got %s", decoded.ResolvedBy)
	}
}
