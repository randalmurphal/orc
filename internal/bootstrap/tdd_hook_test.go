package bootstrap

import (
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// TestTDDHookIntegration tests the bash hook against a real SQLite database.
// This ensures the hook correctly queries the database and enforces TDD discipline.
func TestTDDHookIntegration(t *testing.T) {
	// Skip if sqlite3 is not available
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 not available")
	}

	// Create temp directory for test
	tmpDir := t.TempDir()

	// Create .orc directory and database
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	dbPath := filepath.Join(orcDir, "orc.db")

	// Create a minimal database with tasks table
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()

	// Create tasks table with just the fields we need
	_, err = db.Exec(`
		CREATE TABLE tasks (
			id TEXT PRIMARY KEY,
			current_phase TEXT
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Install the hook
	hookDir := filepath.Join(tmpDir, ".claude", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	hookContent, err := embeddedHooks.ReadFile("hooks/" + TDDDisciplineHook)
	if err != nil {
		t.Fatalf("read embedded hook: %v", err)
	}

	hookPath := filepath.Join(hookDir, TDDDisciplineHook)
	if err := os.WriteFile(hookPath, hookContent, 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	tests := []struct {
		name        string
		taskID      string
		phase       string
		toolName    string
		filePath    string
		wantBlocked bool
	}{
		// During tdd_write phase
		{
			name:        "blocks non-test file during tdd_write",
			taskID:      "TASK-001",
			phase:       "tdd_write",
			toolName:    "Write",
			filePath:    "src/main.go",
			wantBlocked: true,
		},
		{
			name:        "allows test file during tdd_write",
			taskID:      "TASK-001",
			phase:       "tdd_write",
			toolName:    "Write",
			filePath:    "src/main_test.go",
			wantBlocked: false,
		},
		{
			name:        "allows file in tests dir during tdd_write",
			taskID:      "TASK-001",
			phase:       "tdd_write",
			toolName:    "Edit",
			filePath:    "tests/helper.go",
			wantBlocked: false,
		},
		{
			name:        "allows conftest.py during tdd_write",
			taskID:      "TASK-001",
			phase:       "tdd_write",
			toolName:    "Write",
			filePath:    "tests/conftest.py",
			wantBlocked: false,
		},

		// During other phases
		{
			name:        "allows non-test file during implement",
			taskID:      "TASK-002",
			phase:       "implement",
			toolName:    "Write",
			filePath:    "src/main.go",
			wantBlocked: false,
		},
		{
			name:        "allows non-test file during spec",
			taskID:      "TASK-003",
			phase:       "spec",
			toolName:    "Edit",
			filePath:    "src/server.ts",
			wantBlocked: false,
		},

		// Non-file tools are always allowed
		{
			name:        "allows Bash during tdd_write",
			taskID:      "TASK-001",
			phase:       "tdd_write",
			toolName:    "Bash",
			filePath:    "",
			wantBlocked: false,
		},
		{
			name:        "allows Read during tdd_write",
			taskID:      "TASK-001",
			phase:       "tdd_write",
			toolName:    "Read",
			filePath:    "src/main.go",
			wantBlocked: false, // Read is allowed, only Write/Edit/MultiEdit blocked
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Insert or update task in database
			_, err := db.Exec(`
				INSERT OR REPLACE INTO tasks (id, current_phase) VALUES (?, ?)
			`, tt.taskID, tt.phase)
			if err != nil {
				t.Fatalf("insert task: %v", err)
			}

			// Create hook input JSON
			input := map[string]interface{}{
				"tool_name": tt.toolName,
				"tool_input": map[string]interface{}{
					"file_path": tt.filePath,
				},
			}
			inputJSON, err := json.Marshal(input)
			if err != nil {
				t.Fatalf("marshal input: %v", err)
			}

			// Run the hook
			cmd := exec.Command("bash", hookPath)
			cmd.Env = append(os.Environ(),
				"ORC_TASK_ID="+tt.taskID,
				"ORC_DB_PATH="+dbPath,
			)
			cmd.Stdin = strings.NewReader(string(inputJSON))

			output, err := cmd.CombinedOutput()

			// Check result
			// PreToolUse hooks: exit 0 with JSON output containing decision field
			// Empty output or no JSON = allow, JSON with decision=block = block
			if err != nil {
				t.Errorf("hook exited with error: %v. Output: %s", err, output)
				return
			}

			// Parse JSON output if any
			var decision map[string]any
			hasBlockDecision := false
			if len(output) > 0 {
				if jsonErr := json.Unmarshal(output, &decision); jsonErr == nil {
					if decision["decision"] == "block" {
						hasBlockDecision = true
					}
				}
			}

			if tt.wantBlocked && !hasBlockDecision {
				t.Errorf("expected hook to block, but it allowed. Output: %s", output)
			}
			if !tt.wantBlocked && hasBlockDecision {
				t.Errorf("expected hook to allow, but it blocked. Output: %s", output)
			}
		})
	}
}

// TestTDDHookNoDatabase tests that the hook gracefully handles missing database.
func TestTDDHookNoDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	// Install the hook
	hookDir := filepath.Join(tmpDir, ".claude", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	hookContent, err := embeddedHooks.ReadFile("hooks/" + TDDDisciplineHook)
	if err != nil {
		t.Fatalf("read embedded hook: %v", err)
	}

	hookPath := filepath.Join(hookDir, TDDDisciplineHook)
	if err := os.WriteFile(hookPath, hookContent, 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	// Create hook input JSON for a non-test file
	input := map[string]interface{}{
		"tool_name": "Write",
		"tool_input": map[string]interface{}{
			"file_path": "src/main.go",
		},
	}
	inputJSON, _ := json.Marshal(input)

	// Run the hook with non-existent database
	cmd := exec.Command("bash", hookPath)
	cmd.Env = append(os.Environ(),
		"ORC_TASK_ID=TASK-001",
		"ORC_DB_PATH=/nonexistent/path/orc.db",
	)
	cmd.Stdin = strings.NewReader(string(inputJSON))

	err = cmd.Run()
	if err != nil {
		t.Errorf("hook should allow when database doesn't exist, but got error: %v", err)
	}
}

// TestTDDHookNoEnvVars tests that the hook gracefully handles missing env vars.
func TestTDDHookNoEnvVars(t *testing.T) {
	tmpDir := t.TempDir()

	// Install the hook
	hookDir := filepath.Join(tmpDir, ".claude", "hooks")
	if err := os.MkdirAll(hookDir, 0755); err != nil {
		t.Fatalf("create hooks dir: %v", err)
	}

	hookContent, err := embeddedHooks.ReadFile("hooks/" + TDDDisciplineHook)
	if err != nil {
		t.Fatalf("read embedded hook: %v", err)
	}

	hookPath := filepath.Join(hookDir, TDDDisciplineHook)
	if err := os.WriteFile(hookPath, hookContent, 0755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	// Create hook input JSON for a non-test file
	input := map[string]interface{}{
		"tool_name": "Write",
		"tool_input": map[string]interface{}{
			"file_path": "src/main.go",
		},
	}
	inputJSON, _ := json.Marshal(input)

	// Run the hook WITHOUT env vars
	cmd := exec.Command("bash", hookPath)
	// Don't set ORC_TASK_ID or ORC_DB_PATH
	cmd.Stdin = strings.NewReader(string(inputJSON))

	err = cmd.Run()
	if err != nil {
		t.Errorf("hook should allow when env vars are missing, but got error: %v", err)
	}
}
