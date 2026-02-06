package code

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// --- SC-2: Go parser extracts functions, methods, types, interfaces ---

// SC-2: Go parser extracts standalone functions with correct properties.
func TestGoParser_Functions(t *testing.T) {
	src := `package example

// Add adds two integers and returns the sum.
func Add(a, b int) int {
	return a + b
}

func noDoc() {}
`
	symbols := parseGoSource(t, "funcs.go", src)

	found := findSymbol(symbols, "Add")
	if found == nil {
		t.Fatal("symbol Add not found")
	}
	if found.Kind != SymbolFunction {
		t.Errorf("Add.Kind = %s, want function", found.Kind)
	}
	if found.Docstring == "" {
		t.Error("Add.Docstring should not be empty")
	}
	if found.StartLine == 0 || found.EndLine == 0 {
		t.Error("Add should have non-zero start/end lines")
	}
	if found.EndLine <= found.StartLine {
		t.Errorf("Add.EndLine (%d) should be > StartLine (%d)", found.EndLine, found.StartLine)
	}
	if found.Signature == "" {
		t.Error("Add.Signature should not be empty")
	}
	if found.Receiver != "" {
		t.Error("Add.Receiver should be empty for standalone function")
	}

	noDocSym := findSymbol(symbols, "noDoc")
	if noDocSym == nil {
		t.Fatal("symbol noDoc not found")
	}
	if noDocSym.Docstring != "" {
		t.Errorf("noDoc.Docstring = %q, want empty", noDocSym.Docstring)
	}
}

// SC-2: Go parser extracts methods with receiver type.
func TestGoParser_Methods(t *testing.T) {
	src := `package example

type Server struct {
	port int
}

// Start starts the server on the configured port.
func (s *Server) Start() error {
	return nil
}

func (s Server) Port() int {
	return s.port
}
`
	symbols := parseGoSource(t, "methods.go", src)

	start := findSymbol(symbols, "Start")
	if start == nil {
		t.Fatal("symbol Start not found")
	}
	if start.Kind != SymbolMethod {
		t.Errorf("Start.Kind = %s, want method", start.Kind)
	}
	if start.Receiver == "" {
		t.Error("Start.Receiver should not be empty for method")
	}

	port := findSymbol(symbols, "Port")
	if port == nil {
		t.Fatal("symbol Port not found")
	}
	if port.Kind != SymbolMethod {
		t.Errorf("Port.Kind = %s, want method", port.Kind)
	}
	if port.Receiver == "" {
		t.Error("Port.Receiver should not be empty")
	}
}

// SC-2: Go parser extracts struct types.
func TestGoParser_Types(t *testing.T) {
	src := `package example

// Config holds configuration values.
type Config struct {
	Host string
	Port int
}

type Empty struct{}
`
	symbols := parseGoSource(t, "types.go", src)

	cfg := findSymbol(symbols, "Config")
	if cfg == nil {
		t.Fatal("symbol Config not found")
	}
	if cfg.Kind != SymbolType {
		t.Errorf("Config.Kind = %s, want type", cfg.Kind)
	}
	if cfg.Docstring == "" {
		t.Error("Config.Docstring should not be empty")
	}

	empty := findSymbol(symbols, "Empty")
	if empty == nil {
		t.Fatal("symbol Empty not found")
	}
}

// SC-2: Go parser extracts interfaces.
func TestGoParser_Interfaces(t *testing.T) {
	src := `package example

// Reader reads data from a source.
type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}
`
	symbols := parseGoSource(t, "interfaces.go", src)

	reader := findSymbol(symbols, "Reader")
	if reader == nil {
		t.Fatal("symbol Reader not found")
	}
	if reader.Kind != SymbolInterface {
		t.Errorf("Reader.Kind = %s, want interface", reader.Kind)
	}
	if reader.Docstring == "" {
		t.Error("Reader.Docstring should not be empty")
	}

	writer := findSymbol(symbols, "Writer")
	if writer == nil {
		t.Fatal("symbol Writer not found")
	}
}

// SC-2: Go parser extracts const and var groups.
func TestGoParser_ConstAndVarGroups(t *testing.T) {
	src := `package example

const (
	MaxRetries = 3
	Timeout    = 30
)

var (
	DefaultHost = "localhost"
	DefaultPort = 8080
)
`
	symbols := parseGoSource(t, "constvars.go", src)

	// Should find at least one const group and one var group
	hasConst := false
	hasVar := false
	for _, s := range symbols {
		if s.Kind == SymbolConst {
			hasConst = true
		}
		if s.Kind == SymbolVar {
			hasVar = true
		}
	}
	if !hasConst {
		t.Error("should find const group symbol")
	}
	if !hasVar {
		t.Error("should find var group symbol")
	}
}

// SC-2: Go parser extracts all symbol types from comprehensive test file.
func TestGoParser_AllSymbolTypes(t *testing.T) {
	src := `package example

// Config is the main configuration type.
type Config struct {
	Host string
	Port int
}

// Handler handles requests.
type Handler interface {
	Handle(req string) (string, error)
}

const MaxItems = 100

var Version = "1.0.0"

// NewConfig creates a new Config with defaults.
func NewConfig() *Config {
	return &Config{Host: "localhost", Port: 8080}
}

// String returns a string representation.
func (c *Config) String() string {
	return c.Host
}
`
	symbols := parseGoSource(t, "all.go", src)

	expectedKinds := map[string]SymbolKind{
		"Config":    SymbolType,
		"Handler":   SymbolInterface,
		"NewConfig": SymbolFunction,
		"String":    SymbolMethod,
	}

	for name, wantKind := range expectedKinds {
		sym := findSymbol(symbols, name)
		if sym == nil {
			t.Errorf("symbol %s not found", name)
			continue
		}
		if sym.Kind != wantKind {
			t.Errorf("%s.Kind = %s, want %s", name, sym.Kind, wantKind)
		}
	}

	// Must have at least the expected number of symbols
	if len(symbols) < len(expectedKinds) {
		t.Errorf("found %d symbols, want at least %d", len(symbols), len(expectedKinds))
	}
}

// SC-2 error path: Returns partial results for syntactically invalid files.
func TestGoParser_SyntaxError(t *testing.T) {
	src := `package example

// ValidFunc is fine.
func ValidFunc() {
	return
}

// This type is missing a closing brace
type Broken struct {
	Field string
`
	p := NewGoParser()
	result, err := p.Parse(context.Background(), "broken.go", []byte(src))

	// Should return partial results, not abort entirely.
	// The parser may return an error AND some symbols.
	if len(result) == 0 && err == nil {
		t.Error("should return partial results or an error for syntax error file")
	}

	// If partial results returned, ValidFunc should be among them
	if len(result) > 0 {
		found := false
		for _, s := range result {
			if s.Name == "ValidFunc" {
				found = true
				break
			}
		}
		if !found {
			t.Error("partial results should include ValidFunc (before syntax error)")
		}
	}
}

// SC-2 edge case: File with only comments returns empty symbol list.
func TestGoParser_CommentsOnly(t *testing.T) {
	src := `package example

// This file only has comments.
// No functions, types, or anything else.

/*
Multi-line comment block.
Still nothing here.
*/
`
	symbols := parseGoSource(t, "comments.go", src)

	if len(symbols) != 0 {
		t.Errorf("comments-only file should return 0 symbols, got %d", len(symbols))
	}
}

// SC-2: Go parser sets correct FilePath on all symbols.
func TestGoParser_FilePathSet(t *testing.T) {
	src := `package example

func Hello() {}
`
	symbols := parseGoSource(t, "hello.go", src)

	for _, s := range symbols {
		if s.FilePath == "" {
			t.Errorf("symbol %s should have FilePath set", s.Name)
		}
	}
}

// SC-2: Go parser sets language to "go" on all symbols.
func TestGoParser_LanguageSet(t *testing.T) {
	src := `package example

func Hello() {}
`
	symbols := parseGoSource(t, "hello.go", src)

	for _, s := range symbols {
		if s.Language != "go" {
			t.Errorf("symbol %s.Language = %s, want go", s.Name, s.Language)
		}
	}
}

// --- SC-3: Tree-sitter parser extracts Python, JavaScript, TypeScript ---

// SC-3: Tree-sitter extracts Python class, function, and method.
func TestTreeSitter_PythonClassFunctionMethod(t *testing.T) {
	src := `"""Module docstring."""

def standalone_func(x, y):
    """Add two numbers."""
    return x + y

class MyService:
    """A service class."""

    def __init__(self, name):
        self.name = name

    def process(self, data):
        """Process the given data."""
        return data
`
	p := NewTreeSitterParser()
	symbols, err := p.Parse(context.Background(), "service.py", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Should find standalone function
	fn := findSymbol(symbols, "standalone_func")
	if fn == nil {
		t.Fatal("symbol standalone_func not found")
	}
	if fn.Kind != SymbolFunction {
		t.Errorf("standalone_func.Kind = %s, want function", fn.Kind)
	}
	if fn.Docstring == "" {
		t.Error("standalone_func.Docstring should not be empty")
	}

	// Should find class
	cls := findSymbol(symbols, "MyService")
	if cls == nil {
		t.Fatal("symbol MyService not found")
	}
	if cls.Kind != SymbolClass {
		t.Errorf("MyService.Kind = %s, want class", cls.Kind)
	}

	// Should find methods
	init := findSymbol(symbols, "__init__")
	if init == nil {
		t.Fatal("symbol __init__ not found")
	}
	if init.Kind != SymbolMethod {
		t.Errorf("__init__.Kind = %s, want method", init.Kind)
	}

	process := findSymbol(symbols, "process")
	if process == nil {
		t.Fatal("symbol process not found")
	}
	if process.Kind != SymbolMethod {
		t.Errorf("process.Kind = %s, want method", process.Kind)
	}
}

// SC-3: Tree-sitter extracts JavaScript function and class.
func TestTreeSitter_JavaScriptFunctionClass(t *testing.T) {
	src := `/**
 * Add two numbers.
 */
function add(a, b) {
    return a + b;
}

class Calculator {
    constructor() {
        this.result = 0;
    }

    add(value) {
        this.result += value;
        return this;
    }
}

const multiply = (a, b) => a * b;
`
	p := NewTreeSitterParser()
	symbols, err := p.Parse(context.Background(), "calc.js", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	fn := findSymbol(symbols, "add")
	if fn == nil {
		t.Fatal("symbol add not found")
	}
	if fn.Kind != SymbolFunction {
		t.Errorf("add.Kind = %s, want function", fn.Kind)
	}

	cls := findSymbol(symbols, "Calculator")
	if cls == nil {
		t.Fatal("symbol Calculator not found")
	}
	if cls.Kind != SymbolClass {
		t.Errorf("Calculator.Kind = %s, want class", cls.Kind)
	}
}

// SC-3: Tree-sitter extracts TypeScript function, class, and interface.
func TestTreeSitter_TypeScriptFunctionClassInterface(t *testing.T) {
	src := `interface Serializable {
    serialize(): string;
    deserialize(data: string): void;
}

class User implements Serializable {
    constructor(public name: string, public email: string) {}

    serialize(): string {
        return JSON.stringify({ name: this.name, email: this.email });
    }

    deserialize(data: string): void {
        const parsed = JSON.parse(data);
        this.name = parsed.name;
    }
}

function createUser(name: string, email: string): User {
    return new User(name, email);
}
`
	p := NewTreeSitterParser()
	symbols, err := p.Parse(context.Background(), "user.ts", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	iface := findSymbol(symbols, "Serializable")
	if iface == nil {
		t.Fatal("symbol Serializable not found")
	}
	if iface.Kind != SymbolInterface {
		t.Errorf("Serializable.Kind = %s, want interface", iface.Kind)
	}

	cls := findSymbol(symbols, "User")
	if cls == nil {
		t.Fatal("symbol User not found")
	}
	if cls.Kind != SymbolClass {
		t.Errorf("User.Kind = %s, want class", cls.Kind)
	}

	fn := findSymbol(symbols, "createUser")
	if fn == nil {
		t.Fatal("symbol createUser not found")
	}
	if fn.Kind != SymbolFunction {
		t.Errorf("createUser.Kind = %s, want function", fn.Kind)
	}
}

// SC-3: Tree-sitter sets correct properties (name, kind, start/end line, docstring).
func TestTreeSitter_SymbolProperties(t *testing.T) {
	src := `"""A utility module."""

def greet(name):
    """Say hello to someone."""
    print(f"Hello, {name}")
`
	p := NewTreeSitterParser()
	symbols, err := p.Parse(context.Background(), "util.py", []byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	greet := findSymbol(symbols, "greet")
	if greet == nil {
		t.Fatal("symbol greet not found")
	}

	if greet.Name != "greet" {
		t.Errorf("Name = %s, want greet", greet.Name)
	}
	if greet.StartLine == 0 {
		t.Error("StartLine should be non-zero")
	}
	if greet.EndLine == 0 {
		t.Error("EndLine should be non-zero")
	}
	if greet.EndLine < greet.StartLine {
		t.Errorf("EndLine (%d) should be >= StartLine (%d)", greet.EndLine, greet.StartLine)
	}
	if greet.FilePath == "" {
		t.Error("FilePath should be set")
	}
}

// SC-3 error path: Returns partial results or raw content for unparseable files.
func TestTreeSitter_UnparseableFile(t *testing.T) {
	src := `This is not valid Python or JavaScript at all
	{{{{{ broken syntax everywhere
	def not_a_real_function(
`
	p := NewTreeSitterParser()
	symbols, err := p.Parse(context.Background(), "broken.py", []byte(src))

	// Should not crash. May return empty results or an error, or both.
	// The key requirement is graceful handling — no panic.
	_ = symbols
	_ = err
}

// --- Test helpers ---

func parseGoSource(t *testing.T, filename, src string) []Symbol {
	t.Helper()
	p := NewGoParser()
	result, err := p.Parse(context.Background(), filename, []byte(src))
	if err != nil {
		t.Fatalf("Parse %s: %v", filename, err)
	}
	return result
}

func findSymbol(symbols []Symbol, name string) *Symbol {
	for i := range symbols {
		if symbols[i].Name == name {
			return &symbols[i]
		}
	}
	return nil
}

// writeGoTestFile creates a Go source file in a temp dir and returns its path.
func writeGoTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
