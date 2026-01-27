package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

func TestShowTriggerExecutionHistory(t *testing.T) {
	// Create in-memory database
	pdb, err := db.OpenProjectInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Insert parent trigger record for foreign key constraint
	triggerID := "test-trigger"
	_, err = pdb.Exec(`
		INSERT INTO automation_triggers (id, type, enabled, config)
		VALUES (?, 'count', 1, '{}')
	`, triggerID)
	if err != nil {
		t.Fatalf("failed to insert trigger: %v", err)
	}

	// Insert test execution data
	now := time.Now()
	_, err = pdb.Exec(`
		INSERT INTO trigger_executions (trigger_id, task_id, triggered_at, trigger_reason, status)
		VALUES (?, ?, ?, ?, ?)
	`, triggerID, "AUTO-001", now.Add(-3*time.Hour).Format("2006-01-02 15:04:05"), "Manual trigger", "completed")
	if err != nil {
		t.Fatalf("failed to insert execution 1: %v", err)
	}

	_, err = pdb.Exec(`
		INSERT INTO trigger_executions (trigger_id, task_id, triggered_at, trigger_reason, status)
		VALUES (?, ?, ?, ?, ?)
	`, triggerID, "AUTO-002", now.Add(-2*time.Hour).Format("2006-01-02 15:04:05"), "Threshold reached", "completed")
	if err != nil {
		t.Fatalf("failed to insert execution 2: %v", err)
	}

	_, err = pdb.Exec(`
		INSERT INTO trigger_executions (trigger_id, task_id, triggered_at, trigger_reason, status)
		VALUES (?, ?, ?, ?, ?)
	`, triggerID, "AUTO-003", now.Add(-1*time.Hour).Format("2006-01-02 15:04:05"), "Count reached", "running")
	if err != nil {
		t.Fatalf("failed to insert execution 3: %v", err)
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = showTriggerExecutionHistory(pdb, triggerID, 5)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("showTriggerExecutionHistory failed: %v", err)
	}

	// Verify output contains expected task IDs
	if !strings.Contains(output, "AUTO-001") {
		t.Errorf("expected output to contain AUTO-001, got: %s", output)
	}
	if !strings.Contains(output, "AUTO-002") {
		t.Errorf("expected output to contain AUTO-002, got: %s", output)
	}
	if !strings.Contains(output, "AUTO-003") {
		t.Errorf("expected output to contain AUTO-003, got: %s", output)
	}

	// Verify output contains statuses
	if !strings.Contains(output, "completed") {
		t.Errorf("expected output to contain 'completed' status, got: %s", output)
	}
	if !strings.Contains(output, "running") {
		t.Errorf("expected output to contain 'running' status, got: %s", output)
	}
}

func TestShowTriggerExecutionHistory_NoExecutions(t *testing.T) {
	// Create in-memory database
	pdb, err := db.OpenProjectInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = showTriggerExecutionHistory(pdb, "nonexistent-trigger", 5)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("showTriggerExecutionHistory failed: %v", err)
	}

	// Verify "No executions" message is displayed
	if !strings.Contains(output, "No executions") {
		t.Errorf("expected output to contain 'No executions', got: %s", output)
	}
}

func TestShowTriggerExecutionHistory_LimitsResults(t *testing.T) {
	// Create in-memory database
	pdb, err := db.OpenProjectInMemory()
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Insert parent trigger record for foreign key constraint
	triggerID := "test-trigger"
	_, err = pdb.Exec(`
		INSERT INTO automation_triggers (id, type, enabled, config)
		VALUES (?, 'count', 1, '{}')
	`, triggerID)
	if err != nil {
		t.Fatalf("failed to insert trigger: %v", err)
	}

	// Insert 7 executions
	now := time.Now()
	for i := 1; i <= 7; i++ {
		taskID := fmt.Sprintf("AUTO-%03d", i)
		_, err = pdb.Exec(`
			INSERT INTO trigger_executions (trigger_id, task_id, triggered_at, trigger_reason, status)
			VALUES (?, ?, ?, ?, ?)
		`, triggerID, taskID, now.Add(-time.Duration(i)*time.Hour).Format("2006-01-02 15:04:05"), "Test reason", "completed")
		if err != nil {
			t.Fatalf("failed to insert execution %d: %v", i, err)
		}
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Request limit of 5
	err = showTriggerExecutionHistory(pdb, triggerID, 5)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("showTriggerExecutionHistory failed: %v", err)
	}

	// Count number of "AUTO-" entries in output
	count := strings.Count(output, "AUTO-")
	if count != 5 {
		t.Errorf("expected exactly 5 entries, got %d. Output: %s", count, output)
	}
}
