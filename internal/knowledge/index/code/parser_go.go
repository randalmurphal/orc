package code

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// GoParser parses Go source files using the native go/ast package.
type GoParser struct{}

// NewGoParser creates a new Go parser.
func NewGoParser() *GoParser {
	return &GoParser{}
}

// Parse parses Go source and returns extracted symbols.
// Returns partial results and an error for syntactically invalid files.
func (p *GoParser) Parse(_ context.Context, filename string, src []byte) ([]Symbol, error) {
	fset := token.NewFileSet()
	file, parseErr := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if file == nil {
		return nil, fmt.Errorf("parse %s: %w", filename, parseErr)
	}

	var symbols []Symbol

	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sym := extractFuncDecl(fset, d, filename)
			symbols = append(symbols, sym)

		case *ast.GenDecl:
			syms := extractGenDecl(fset, d, filename)
			symbols = append(symbols, syms...)
		}
	}

	return symbols, parseErr
}

func extractFuncDecl(fset *token.FileSet, d *ast.FuncDecl, filename string) Symbol {
	sym := Symbol{
		Name:      d.Name.Name,
		Kind:      SymbolFunction,
		FilePath:  filename,
		Language:  "go",
		StartLine: fset.Position(d.Pos()).Line,
		EndLine:   fset.Position(d.End()).Line,
		Docstring: extractDocstring(d.Doc),
		Signature: buildFuncSignature(d),
	}

	if d.Recv != nil && len(d.Recv.List) > 0 {
		sym.Kind = SymbolMethod
		sym.Receiver = extractReceiverType(d.Recv.List[0].Type)
		sym.Parent = sym.Receiver
	}

	return sym
}

func extractGenDecl(fset *token.FileSet, d *ast.GenDecl, filename string) []Symbol {
	var symbols []Symbol

	switch d.Tok {
	case token.TYPE:
		for _, spec := range d.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			sym := Symbol{
				Name:      ts.Name.Name,
				FilePath:  filename,
				Language:  "go",
				StartLine: fset.Position(ts.Pos()).Line,
				EndLine:   fset.Position(ts.End()).Line,
				Docstring: extractDocstring(d.Doc),
			}
			if ts.Doc != nil {
				sym.Docstring = extractDocstring(ts.Doc)
			}
			switch ts.Type.(type) {
			case *ast.InterfaceType:
				sym.Kind = SymbolInterface
			default:
				sym.Kind = SymbolType
			}
			symbols = append(symbols, sym)
		}

	case token.CONST:
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			sym := Symbol{
				Name:      vs.Names[0].Name,
				Kind:      SymbolConst,
				FilePath:  filename,
				Language:  "go",
				StartLine: fset.Position(vs.Pos()).Line,
				EndLine:   fset.Position(vs.End()).Line,
				Docstring: extractDocstring(d.Doc),
			}
			symbols = append(symbols, sym)
		}

	case token.VAR:
		for _, spec := range d.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			sym := Symbol{
				Name:      vs.Names[0].Name,
				Kind:      SymbolVar,
				FilePath:  filename,
				Language:  "go",
				StartLine: fset.Position(vs.Pos()).Line,
				EndLine:   fset.Position(vs.End()).Line,
				Docstring: extractDocstring(d.Doc),
			}
			symbols = append(symbols, sym)
		}
	}

	return symbols
}

func extractDocstring(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	return strings.TrimSpace(doc.Text())
}

func extractReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return extractReceiverType(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return ""
	}
}

func buildFuncSignature(d *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		sb.WriteString("(")
		sb.WriteString(extractReceiverType(d.Recv.List[0].Type))
		sb.WriteString(") ")
	}
	sb.WriteString(d.Name.Name)
	sb.WriteString("(")

	if d.Type.Params != nil {
		var params []string
		for _, field := range d.Type.Params.List {
			typeName := typeString(field.Type)
			if len(field.Names) == 0 {
				params = append(params, typeName)
			} else {
				for _, name := range field.Names {
					params = append(params, name.Name+" "+typeName)
				}
			}
		}
		sb.WriteString(strings.Join(params, ", "))
	}
	sb.WriteString(")")

	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		sb.WriteString(" ")
		var results []string
		for _, field := range d.Type.Results.List {
			typeName := typeString(field.Type)
			if len(field.Names) == 0 {
				results = append(results, typeName)
			} else {
				for _, name := range field.Names {
					results = append(results, name.Name+" "+typeName)
				}
			}
		}
		if len(results) == 1 {
			sb.WriteString(results[0])
		} else {
			sb.WriteString("(" + strings.Join(results, ", ") + ")")
		}
	}

	return sb.String()
}

func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.MapType:
		return "map[" + typeString(t.Key) + "]" + typeString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + typeString(t.Elt)
	default:
		return "interface{}"
	}
}
