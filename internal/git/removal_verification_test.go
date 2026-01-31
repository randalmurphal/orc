package git

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for SC-10: InjectClaudeCodeHooks calls removed from all three CreateWorktree* functions.
// Tests for SC-11: InjectClaudeCodeHooks, generateClaudeCodeSettings, ClaudeCodeHookConfig,
//   worktreeSettings, InjectMCPServersToWorktree, RemoveClaudeCodeHooks deleted from hooks.go.

// getPackageDir returns the absolute path to the current package directory.
func getPackageDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok, "should get caller info")
	return filepath.Dir(filename)
}

// TestRemoval_InjectClaudeCodeHooksNotInWorktreeFunctions verifies SC-10:
// The three CreateWorktree* functions should NOT call InjectClaudeCodeHooks.
func TestRemoval_InjectClaudeCodeHooksNotInWorktreeFunctions(t *testing.T) {
	pkgDir := getPackageDir(t)
	worktreeFile := filepath.Join(pkgDir, "git_worktree.go")

	content, err := os.ReadFile(worktreeFile)
	require.NoError(t, err, "git_worktree.go should exist")

	source := string(content)

	assert.NotContains(t, source, "InjectClaudeCodeHooks",
		"git_worktree.go should not reference InjectClaudeCodeHooks — "+
			"worktree creation no longer writes .claude/settings.json")
}

// TestRemoval_DeadCodeDeletedFromHooks verifies SC-11:
// All six symbols should be fully removed from hooks.go.
func TestRemoval_DeadCodeDeletedFromHooks(t *testing.T) {
	pkgDir := getPackageDir(t)
	hooksFile := filepath.Join(pkgDir, "hooks.go")

	content, err := os.ReadFile(hooksFile)
	require.NoError(t, err, "hooks.go should exist")

	source := string(content)

	// All these symbols should be deleted
	deletedSymbols := []string{
		"InjectClaudeCodeHooks",
		"generateClaudeCodeSettings",
		"ClaudeCodeHookConfig",
		"worktreeSettings",
		"InjectMCPServersToWorktree",
		"RemoveClaudeCodeHooks",
	}

	for _, sym := range deletedSymbols {
		assert.NotContains(t, source, sym,
			"hooks.go should not contain %s — it should be fully deleted", sym)
	}
}

// TestRemoval_NoDeadCodeDefinitions uses AST parsing to verify no function/type
// definitions remain for the deleted symbols.
func TestRemoval_NoDeadCodeDefinitions(t *testing.T) {
	pkgDir := getPackageDir(t)
	hooksFile := filepath.Join(pkgDir, "hooks.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, hooksFile, nil, parser.AllErrors)
	require.NoError(t, err, "hooks.go should parse without errors")

	deletedSymbols := map[string]bool{
		"InjectClaudeCodeHooks":      true,
		"generateClaudeCodeSettings": true,
		"ClaudeCodeHookConfig":       true,
		"worktreeSettings":           true,
		"InjectMCPServersToWorktree": true,
		"RemoveClaudeCodeHooks":      true,
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			assert.False(t, deletedSymbols[d.Name.Name],
				"function %s should be deleted from hooks.go", d.Name.Name)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					assert.False(t, deletedSymbols[ts.Name.Name],
						"type %s should be deleted from hooks.go", ts.Name.Name)
				}
			}
		}
	}
}

// TestRemoval_InjectMCPServersNotInExecutor verifies SC-4:
// InjectMCPServersToWorktree should not appear in the executor package.
func TestRemoval_InjectMCPServersNotInExecutor(t *testing.T) {
	// Walk the executor package to verify no references
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	// We're in the git package, navigate to executor
	gitDir := filepath.Dir(filename)
	executorDir := filepath.Join(filepath.Dir(gitDir), "executor")

	entries, err := os.ReadDir(executorDir)
	require.NoError(t, err, "executor directory should exist")

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		// Skip test files for the check
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(executorDir, entry.Name()))
		require.NoError(t, err)

		assert.NotContains(t, string(content), "InjectMCPServersToWorktree",
			"%s should not reference InjectMCPServersToWorktree", entry.Name())
	}
}

// TestRemoval_SkillLoaderNotInWorkflowPhase verifies SC-9:
// SkillLoader.LoadSkillsForConfig should not appear in workflow_phase.go.
func TestRemoval_SkillLoaderNotInWorkflowPhase(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	gitDir := filepath.Dir(filename)
	executorDir := filepath.Join(filepath.Dir(gitDir), "executor")
	workflowPhaseFile := filepath.Join(executorDir, "workflow_phase.go")

	content, err := os.ReadFile(workflowPhaseFile)
	require.NoError(t, err, "workflow_phase.go should exist")

	assert.NotContains(t, string(content), "LoadSkillsForConfig",
		"workflow_phase.go should not reference LoadSkillsForConfig — "+
			"skills are now written as real .claude/skills/ files by ApplyPhaseSettings")
}
