package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCodexPhaseSettings_WritesAgentsMD(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := AgentsMDContent{
		PhaseContext: "Task: TEST-001\nPhase: implement",
	}
	if err := ApplyCodexPhaseSettings(dir, content, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	agentsPath := filepath.Join(dir, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	if !strings.Contains(string(data), "TEST-001") {
		t.Errorf("AGENTS.md missing phase context, got: %s", string(data))
	}
}

func TestApplyCodexPhaseSettings_WritesCodexConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := AgentsMDContent{PhaseContext: "test"}
	codexCfg := &PhaseCodexConfig{
		Instructions: "# Custom instructions\nFocus on testing.",
	}
	if err := ApplyCodexPhaseSettings(dir, content, codexCfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	instrPath := filepath.Join(dir, ".codex", "instruction.md")
	data, err := os.ReadFile(instrPath)
	if err != nil {
		t.Fatalf("instruction.md not written: %v", err)
	}
	if string(data) != codexCfg.Instructions {
		t.Errorf("instruction.md content mismatch, got: %s", string(data))
	}
}

func TestApplyCodexPhaseSettings_NilCodexConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := AgentsMDContent{PhaseContext: "test"}
	if err := ApplyCodexPhaseSettings(dir, content, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// .codex/ should not be created when config is nil
	codexDir := filepath.Join(dir, ".codex")
	if _, err := os.Stat(codexDir); err == nil {
		t.Error(".codex/ dir should not exist when config is nil")
	}
}

func TestApplyCodexPhaseSettings_InvalidPath(t *testing.T) {
	t.Parallel()
	if err := ApplyCodexPhaseSettings("/nonexistent/path", AgentsMDContent{}, nil); err == nil {
		t.Error("expected error for nonexistent path")
	}
}
