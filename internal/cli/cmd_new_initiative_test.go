package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
)

func withNewTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}
	origRoot := os.Getenv("ORC_PROJECT_ROOT")
	if err := os.Setenv("ORC_PROJECT_ROOT", tmpDir); err != nil {
		t.Fatalf("set ORC_PROJECT_ROOT: %v", err)
	}
	t.Cleanup(func() {
		if origRoot == "" {
			_ = os.Unsetenv("ORC_PROJECT_ROOT")
		} else {
			_ = os.Setenv("ORC_PROJECT_ROOT", origRoot)
		}
	})
	return tmpDir
}

func createNewTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

func TestNewCommand_InvalidInitiative(t *testing.T) {
	tmpDir := withNewTestDir(t)

	// Create backend so the database exists, then close it
	backend := createNewTestBackend(t, tmpDir)
	_ = backend.Close()

	cmd := newNewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"Test task", "--initiative", "INIT-NONEXISTENT"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent initiative")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}
}

func TestNewCommand_ValidInitiative(t *testing.T) {
	tmpDir := withNewTestDir(t)

	// Create backend and save an initiative
	backend := createNewTestBackend(t, tmpDir)
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}
	_ = backend.Close()

	// Capture stdout since cmd_new.go uses fmt.Printf for success output
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdout = w

	cmd := newNewCmd()
	var cmdOut bytes.Buffer
	cmd.SetOut(&cmdOut)
	cmd.SetErr(&cmdOut)
	cmd.SetArgs([]string{"Test task", "--initiative", "INIT-001"})

	cmdErr := cmd.Execute()

	// Restore stdout and read captured output
	_ = w.Close()
	os.Stdout = oldStdout
	captured, _ := io.ReadAll(r)

	if cmdErr != nil {
		t.Fatalf("unexpected error: %v", cmdErr)
	}

	stdout := string(captured)
	if !strings.Contains(stdout, "INIT-001") {
		t.Errorf("output should contain 'INIT-001', got: %s", stdout)
	}
}
