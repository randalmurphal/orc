// Package code provides AST-aware code analysis components for the indexer.
package code

// SymbolKind represents the type of a code symbol.
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolType      SymbolKind = "type"
	SymbolInterface SymbolKind = "interface"
	SymbolClass     SymbolKind = "class"
	SymbolConst     SymbolKind = "const"
	SymbolVar       SymbolKind = "var"
)

// Symbol represents a parsed code symbol (function, method, class, etc.).
type Symbol struct {
	Name      string
	Kind      SymbolKind
	FilePath  string
	Language  string
	StartLine int
	EndLine   int
	Docstring string
	Signature string
	Receiver  string
	Parent    string
	Children  []Symbol
}

// FileInfo describes a discovered source file.
type FileInfo struct {
	Path     string
	Language string
}

// Chunk is a self-contained unit of code content for embedding.
type Chunk struct {
	Symbol    string
	Content   string
	FilePath  string
	Kind      SymbolKind
	StartLine int
	EndLine   int
	Context   string
}

// RelationshipKind identifies how two symbols relate.
type RelationshipKind string

const (
	RelImport     RelationshipKind = "import"
	RelCall       RelationshipKind = "call"
	RelExtends    RelationshipKind = "extends"
	RelImplements RelationshipKind = "implements"
)

// Relationship links a source symbol to a target symbol.
type Relationship struct {
	Kind       RelationshipKind
	SourceName string
	SourceFile string
	TargetName string
	TargetFile string
}

// SecretFinding records a detected secret in source content.
type SecretFinding struct {
	Type  string
	Match string
	Line  int
}

// Pattern represents a group of structurally similar files.
type Pattern struct {
	Name          string
	MemberCount   int
	CanonicalFile string
	Members       []string
}
