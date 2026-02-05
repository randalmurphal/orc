package executor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestLoadAndFormatInitiativeNotes_DocstringDescribesReturnValue verifies SC-1:
// The docstring must describe what the function returns (formatted markdown string,
// empty if no applicable notes).
func TestLoadAndFormatInitiativeNotes_DocstringDescribesReturnValue(t *testing.T) {
	doc := extractFunctionDoc(t, "workflow_context.go", "loadAndFormatInitiativeNotes")

	// SC-1: Must describe return value behavior
	if !strings.Contains(doc, "returns") && !strings.Contains(doc, "Returns") {
		t.Errorf("SC-1 FAILED: docstring must describe return value, got:\n%s", doc)
	}

	// SC-1: Must mention empty string case
	if !strings.Contains(doc, "empty") {
		t.Errorf("SC-1 FAILED: docstring must mention empty string case when no applicable notes, got:\n%s", doc)
	}

	// SC-1: Must describe the format (markdown)
	if !strings.Contains(strings.ToLower(doc), "markdown") && !strings.Contains(doc, "formatted") {
		t.Errorf("SC-1 FAILED: docstring should describe the output format (markdown or formatted string), got:\n%s", doc)
	}
}

// TestLoadAndFormatInitiativeNotes_DocstringExplainsFilteringCriteria verifies SC-2:
// The docstring must explain filtering criteria (human notes always included,
// agent notes only when Graduated=true).
func TestLoadAndFormatInitiativeNotes_DocstringExplainsFilteringCriteria(t *testing.T) {
	doc := extractFunctionDoc(t, "workflow_context.go", "loadAndFormatInitiativeNotes")

	// SC-2: Must explain human notes always included
	if !strings.Contains(strings.ToLower(doc), "human") {
		t.Errorf("SC-2 FAILED: docstring must mention human notes filtering, got:\n%s", doc)
	}

	// SC-2: Must explain agent notes conditional inclusion
	if !strings.Contains(strings.ToLower(doc), "agent") {
		t.Errorf("SC-2 FAILED: docstring must mention agent notes filtering, got:\n%s", doc)
	}

	// SC-2: Must explain graduated condition (explicit, not just DEC reference)
	if !strings.Contains(strings.ToLower(doc), "graduat") {
		t.Errorf("SC-2 FAILED: docstring must explain graduated condition for agent notes, got:\n%s", doc)
	}
}

// extractFunctionDoc parses a Go source file and extracts the doc comment
// for the named function (including method receivers).
func extractFunctionDoc(t *testing.T, filename, funcName string) string {
	t.Helper()

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", filename, err)
	}

	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == funcName {
			if fn.Doc == nil {
				t.Fatalf("function %s has no doc comment", funcName)
			}
			return fn.Doc.Text()
		}
	}

	t.Fatalf("function %s not found in %s", funcName, filename)
	return ""
}
