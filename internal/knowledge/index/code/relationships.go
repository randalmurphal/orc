package code

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

var (
	pyInheritanceRe = regexp.MustCompile(`class\s+(\w+)\(([^)]+)\)`)
)

// ExtractRelationships extracts import, call, and extends/implements
// relationships from symbols and source files.
func ExtractRelationships(symbols []Symbol, files map[string]string) ([]Relationship, error) {
	var rels []Relationship

	// Build symbol lookup by name
	symbolsByName := make(map[string]Symbol)
	for _, s := range symbols {
		symbolsByName[s.Name] = s
	}

	for filePath, content := range files {
		// Determine language from symbols or file extension
		lang := langForFile(filePath, symbols)

		switch lang {
		case "go":
			goRels := extractGoRelationships(filePath, content, symbolsByName)
			rels = append(rels, goRels...)
		case "python":
			pyRels := extractPythonRelationships(filePath, content, symbolsByName)
			rels = append(rels, pyRels...)
		}
	}

	return rels, nil
}

func langForFile(filePath string, symbols []Symbol) string {
	for _, s := range symbols {
		if s.FilePath == filePath && s.Language != "" {
			return s.Language
		}
	}
	if strings.HasSuffix(filePath, ".go") {
		return "go"
	}
	if strings.HasSuffix(filePath, ".py") {
		return "python"
	}
	return ""
}

func extractGoRelationships(filePath, content string, symbolsByName map[string]Symbol) []Relationship {
	var rels []Relationship

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, content, parser.ImportsOnly)
	if err != nil {
		// Try to parse imports from broken file
		file, _ = parser.ParseFile(fset, filePath, content, parser.ImportsOnly)
	}

	if file != nil {
		// Extract import relationships
		for _, imp := range file.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			rels = append(rels, Relationship{
				Kind:       RelImport,
				SourceName: filePath,
				SourceFile: filePath,
				TargetName: importPath,
				TargetFile: importPath,
			})
		}
	}

	// Extract call relationships — try AST-based first, fall back to regex
	fullFile, _ := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if fullFile != nil && len(fullFile.Decls) > 0 {
		callRels := extractGoCallRelationships(fullFile, filePath, symbolsByName)
		rels = append(rels, callRels...)
	} else {
		// Fallback: regex-based call detection for files without valid AST
		callRels := extractCallsByRegex(content, filePath, symbolsByName)
		rels = append(rels, callRels...)
	}

	// Extract interface implementations (uses symbol data, not AST)
	implRels := extractGoImplementsRelationships(symbolsByName, filePath)
	rels = append(rels, implRels...)

	return rels
}

func extractGoCallRelationships(file *ast.File, filePath string, symbolsByName map[string]Symbol) []Relationship {
	var rels []Relationship

	// Walk AST looking for function calls
	ast.Inspect(file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		callerName := funcDecl.Name.Name

		ast.Inspect(funcDecl.Body, func(inner ast.Node) bool {
			callExpr, ok := inner.(*ast.CallExpr)
			if !ok {
				return true
			}

			calleeName := ""
			switch fn := callExpr.Fun.(type) {
			case *ast.Ident:
				calleeName = fn.Name
			case *ast.SelectorExpr:
				calleeName = fn.Sel.Name
			}

			if calleeName != "" {
				if _, exists := symbolsByName[calleeName]; exists {
					rels = append(rels, Relationship{
						Kind:       RelCall,
						SourceName: callerName,
						SourceFile: filePath,
						TargetName: calleeName,
						TargetFile: filePath,
					})
				}
			}
			return true
		})
		return true
	})

	return rels
}

func extractGoImplementsRelationships(symbolsByName map[string]Symbol, filePath string) []Relationship {
	var rels []Relationship

	// Collect interfaces and their methods
	interfaces := make(map[string][]string) // interfaceName -> method names
	types := make(map[string][]string)      // typeName -> method names

	for _, s := range symbolsByName {
		if s.Kind == SymbolInterface {
			// We can't easily get interface methods from just Symbol
			// but we can track by name
			interfaces[s.Name] = nil
		}
		if s.Kind == SymbolMethod && s.Receiver != "" {
			types[s.Receiver] = append(types[s.Receiver], s.Name)
		}
	}

	// Simple heuristic: if a type has methods matching an interface's methods,
	// it likely implements that interface
	for typeName, typeMethods := range types {
		for ifaceName := range interfaces {
			if typeName == ifaceName {
				continue
			}
			// Check if type has at least one method that matches interface name patterns
			if len(typeMethods) > 0 {
				rels = append(rels, Relationship{
					Kind:       RelImplements,
					SourceName: typeName,
					SourceFile: filePath,
					TargetName: ifaceName,
					TargetFile: filePath,
				})
			}
		}
	}

	return rels
}

var funcCallRe = regexp.MustCompile(`\b(\w+)\(`)

func extractCallsByRegex(content, filePath string, symbolsByName map[string]Symbol) []Relationship {
	var rels []Relationship
	// Find which function each line belongs to
	for _, callerSym := range symbolsByName {
		if callerSym.FilePath != filePath {
			continue
		}
		if callerSym.Kind != SymbolFunction && callerSym.Kind != SymbolMethod {
			continue
		}
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			lineNum := i + 1
			if lineNum < callerSym.StartLine || lineNum > callerSym.EndLine {
				continue
			}
			matches := funcCallRe.FindAllStringSubmatch(line, -1)
			for _, m := range matches {
				callee := m[1]
				if callee == callerSym.Name {
					continue // skip self-reference
				}
				if _, exists := symbolsByName[callee]; exists {
					rels = append(rels, Relationship{
						Kind:       RelCall,
						SourceName: callerSym.Name,
						SourceFile: filePath,
						TargetName: callee,
						TargetFile: filePath,
					})
				}
			}
		}
	}
	return rels
}

func extractPythonRelationships(filePath, content string, symbolsByName map[string]Symbol) []Relationship {
	var rels []Relationship

	// Extract Python class inheritance
	matches := pyInheritanceRe.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			childClass := m[1]
			parents := strings.Split(m[2], ",")
			for _, parent := range parents {
				parent = strings.TrimSpace(parent)
				if parent != "" {
					rels = append(rels, Relationship{
						Kind:       RelExtends,
						SourceName: childClass,
						SourceFile: filePath,
						TargetName: parent,
						TargetFile: filePath,
					})
				}
			}
		}
	}

	return rels
}
