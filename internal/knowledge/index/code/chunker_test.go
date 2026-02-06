package code

import (
	"strings"
	"testing"
)

// --- SC-4: Chunker produces AST-aware chunks ---

// SC-4: Each symbol produces exactly one chunk.
func TestChunker_OneChunkPerSymbol(t *testing.T) {
	symbols := []Symbol{
		{Name: "Add", Kind: SymbolFunction, FilePath: "math.go", StartLine: 1, EndLine: 5},
		{Name: "Sub", Kind: SymbolFunction, FilePath: "math.go", StartLine: 7, EndLine: 11},
		{Name: "Mul", Kind: SymbolFunction, FilePath: "math.go", StartLine: 13, EndLine: 17},
	}

	content := map[string]string{
		"math.go": "func Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n\nfunc Mul(a, b int) int {\n\treturn a * b\n}\n",
	}

	c := NewChunker()
	chunks, err := c.Chunk(symbols, content)
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}

	if len(chunks) != 3 {
		t.Errorf("got %d chunks, want 3 (one per symbol)", len(chunks))
	}

	names := make(map[string]bool)
	for _, ch := range chunks {
		names[ch.Symbol] = true
		if ch.Content == "" {
			t.Errorf("chunk for %s has empty content", ch.Symbol)
		}
		if ch.FilePath != "math.go" {
			t.Errorf("chunk %s.FilePath = %s, want math.go", ch.Symbol, ch.FilePath)
		}
	}
	for _, want := range []string{"Add", "Sub", "Mul"} {
		if !names[want] {
			t.Errorf("missing chunk for symbol %s", want)
		}
	}
}

// SC-4: Method chunks include context headers with file path and parent.
func TestChunker_MethodContextHeaders(t *testing.T) {
	symbols := []Symbol{
		{Name: "Server", Kind: SymbolType, FilePath: "server.go", StartLine: 1, EndLine: 4},
		{
			Name: "Start", Kind: SymbolMethod, FilePath: "server.go",
			StartLine: 6, EndLine: 10, Receiver: "Server", Parent: "Server",
		},
		{
			Name: "Stop", Kind: SymbolMethod, FilePath: "server.go",
			StartLine: 12, EndLine: 16, Receiver: "Server", Parent: "Server",
		},
	}

	content := map[string]string{
		"server.go": "type Server struct {\n\tport int\n}\n\nfunc (s *Server) Start() error {\n\treturn nil\n}\n\nfunc (s *Server) Stop() error {\n\treturn nil\n}\n",
	}

	c := NewChunker()
	chunks, err := c.Chunk(symbols, content)
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}

	for _, ch := range chunks {
		if ch.Kind == SymbolMethod {
			if ch.Context == "" {
				t.Errorf("method chunk %s should have context header", ch.Symbol)
			}
			// Context should mention the parent class/type
			if !strings.Contains(ch.Context, "Server") {
				t.Errorf("method chunk %s context %q should mention parent Server", ch.Symbol, ch.Context)
			}
			// Context should mention the file path
			if !strings.Contains(ch.Context, "server.go") {
				t.Errorf("method chunk %s context %q should mention file path", ch.Symbol, ch.Context)
			}
		}
	}
}

// SC-4: Class with >50 methods produces summary + individual method chunks.
func TestChunker_HierarchicalSplit_Above50Methods(t *testing.T) {
	// Build a class with 60 methods
	methods := make([]Symbol, 60)
	for i := range methods {
		methods[i] = Symbol{
			Name:      methodName(i),
			Kind:      SymbolMethod,
			FilePath:  "big_class.py",
			StartLine: 3 + i*3,
			EndLine:   5 + i*3,
			Parent:    "BigClass",
		}
	}

	classSymbol := Symbol{
		Name:      "BigClass",
		Kind:      SymbolClass,
		FilePath:  "big_class.py",
		StartLine: 1,
		EndLine:   183,
		Children:  methods,
	}

	allSymbols := append([]Symbol{classSymbol}, methods...)

	// Build content string
	var sb strings.Builder
	sb.WriteString("class BigClass:\n")
	for i := 0; i < 60; i++ {
		sb.WriteString("    def " + methodName(i) + "(self):\n")
		sb.WriteString("        pass\n\n")
	}

	content := map[string]string{
		"big_class.py": sb.String(),
	}

	c := NewChunker()
	chunks, err := c.Chunk(allSymbols, content)
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}

	// Should have summary chunk + 60 individual method chunks
	// (The class itself becomes a summary chunk, not a full-content chunk)
	hasSummary := false
	methodChunks := 0
	for _, ch := range chunks {
		if ch.Symbol == "BigClass" {
			hasSummary = true
			// Summary should be shorter than all methods combined
			if strings.Count(ch.Content, "def ") >= 60 {
				t.Error("BigClass summary chunk should not contain all 60 method bodies")
			}
		}
		if ch.Kind == SymbolMethod {
			methodChunks++
		}
	}

	if !hasSummary {
		t.Error("should have a BigClass summary chunk for class with >50 methods")
	}
	if methodChunks != 60 {
		t.Errorf("should have 60 individual method chunks, got %d", methodChunks)
	}
}

// SC-4: Class with exactly 50 methods is NOT split hierarchically.
func TestChunker_NoSplit_Exactly50Methods(t *testing.T) {
	methods := make([]Symbol, 50)
	for i := range methods {
		methods[i] = Symbol{
			Name:      methodName(i),
			Kind:      SymbolMethod,
			FilePath:  "medium_class.py",
			StartLine: 3 + i*3,
			EndLine:   5 + i*3,
			Parent:    "MediumClass",
		}
	}

	classSymbol := Symbol{
		Name:      "MediumClass",
		Kind:      SymbolClass,
		FilePath:  "medium_class.py",
		StartLine: 1,
		EndLine:   153,
		Children:  methods,
	}

	allSymbols := append([]Symbol{classSymbol}, methods...)

	var sb strings.Builder
	sb.WriteString("class MediumClass:\n")
	for i := 0; i < 50; i++ {
		sb.WriteString("    def " + methodName(i) + "(self):\n")
		sb.WriteString("        pass\n\n")
	}

	content := map[string]string{
		"medium_class.py": sb.String(),
	}

	c := NewChunker()
	chunks, err := c.Chunk(allSymbols, content)
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}

	// With exactly 50, the class should NOT be split into summary + methods
	// It should remain as a single class chunk (plus method chunks are optional)
	classChunks := 0
	for _, ch := range chunks {
		if ch.Symbol == "MediumClass" {
			classChunks++
		}
	}
	if classChunks != 1 {
		t.Errorf("class with exactly 50 methods should have 1 class chunk, got %d", classChunks)
	}
}

// SC-4 edge case: Empty files produce no chunks.
func TestChunker_EmptyFile(t *testing.T) {
	c := NewChunker()
	chunks, err := c.Chunk(nil, nil)
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("empty input should produce 0 chunks, got %d", len(chunks))
	}
}

// SC-4 edge case: File with no extractable symbols produces a single file-level chunk.
func TestChunker_NoSymbols_SingleFileChunk(t *testing.T) {
	content := map[string]string{
		"empty.go": "package empty\n\n// Just a package declaration and comments.\n",
	}

	c := NewChunker()
	// No symbols but content exists
	chunks, err := c.Chunk(nil, content)
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}

	if len(chunks) != 1 {
		t.Errorf("file with no symbols should produce 1 file-level chunk, got %d", len(chunks))
	}
	if len(chunks) > 0 && chunks[0].FilePath != "empty.go" {
		t.Errorf("file-level chunk should have correct FilePath, got %s", chunks[0].FilePath)
	}
}

// SC-4: Chunks preserve correct line ranges.
func TestChunker_PreservesLineRanges(t *testing.T) {
	symbols := []Symbol{
		{Name: "Foo", Kind: SymbolFunction, FilePath: "foo.go", StartLine: 3, EndLine: 7},
		{Name: "Bar", Kind: SymbolFunction, FilePath: "foo.go", StartLine: 9, EndLine: 15},
	}

	content := map[string]string{
		"foo.go": "package foo\n\nfunc Foo() {\n\t// body\n\t// more\n}\n\nfunc Bar() {\n\t// bar body\n\t// more\n\t// even more\n\t// still more\n}\n",
	}

	c := NewChunker()
	chunks, err := c.Chunk(symbols, content)
	if err != nil {
		t.Fatalf("Chunk: %v", err)
	}

	for _, ch := range chunks {
		if ch.StartLine == 0 || ch.EndLine == 0 {
			t.Errorf("chunk %s should have non-zero line numbers", ch.Symbol)
		}
	}
}

// --- Test helper ---

func methodName(i int) string {
	return "method_" + string(rune('a'+i/26)) + string(rune('a'+i%26))
}
