package code

import (
	"context"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TreeSitterParser parses Python, JavaScript, and TypeScript files.
type TreeSitterParser struct{}

// NewTreeSitterParser creates a new tree-sitter parser.
func NewTreeSitterParser() *TreeSitterParser {
	return &TreeSitterParser{}
}

// Parse parses source code using tree-sitter and returns symbols.
func (p *TreeSitterParser) Parse(_ context.Context, filename string, src []byte) ([]Symbol, error) {
	lang := detectLangFromFilename(filename)
	sitterLang := langToSitter(lang)
	if sitterLang == nil {
		return nil, nil
	}

	parser := sitter.NewParser()
	parser.SetLanguage(sitterLang)

	tree, err := parser.ParseCtx(context.Background(), nil, src)
	if err != nil {
		return nil, nil // graceful fallback
	}
	if tree == nil {
		return nil, nil
	}

	root := tree.RootNode()
	var symbols []Symbol
	extractTreeSitterSymbols(root, src, filename, lang, "", &symbols)

	return symbols, nil
}

func detectLangFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	default:
		return ""
	}
}

func langToSitter(lang string) *sitter.Language {
	switch lang {
	case "python":
		return python.GetLanguage()
	case "javascript":
		return javascript.GetLanguage()
	case "typescript":
		return typescript.GetLanguage()
	default:
		return nil
	}
}

func extractTreeSitterSymbols(node *sitter.Node, src []byte, filename, lang, parent string, symbols *[]Symbol) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch lang {
	case "python":
		extractPythonSymbol(node, nodeType, src, filename, lang, parent, symbols)
	case "javascript":
		extractJSSymbol(node, nodeType, src, filename, lang, parent, symbols)
	case "typescript":
		extractTSSymbol(node, nodeType, src, filename, lang, parent, symbols)
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		extractTreeSitterSymbols(child, src, filename, lang, parent, symbols)
	}
}

func extractPythonSymbol(node *sitter.Node, nodeType string, src []byte, filename, lang, parent string, symbols *[]Symbol) {
	switch nodeType {
	case "function_definition":
		name := getChildByField(node, "name", src)
		if name == "" {
			return
		}
		kind := SymbolFunction
		if parent != "" {
			kind = SymbolMethod
		}
		sym := Symbol{
			Name:      name,
			Kind:      kind,
			FilePath:  filename,
			Language:  lang,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Docstring: extractPythonDocstring(node, src),
			Parent:    parent,
		}
		*symbols = append(*symbols, sym)

	case "class_definition":
		name := getChildByField(node, "name", src)
		if name == "" {
			return
		}
		sym := Symbol{
			Name:      name,
			Kind:      SymbolClass,
			FilePath:  filename,
			Language:  lang,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Docstring: extractPythonDocstring(node, src),
		}
		*symbols = append(*symbols, sym)

		// Extract methods inside the class body
		body := node.ChildByFieldName("body")
		if body != nil {
			for i := 0; i < int(body.ChildCount()); i++ {
				child := body.Child(i)
				if child.Type() == "function_definition" {
					extractPythonSymbol(child, child.Type(), src, filename, lang, name, symbols)
				}
			}
		}
	}
}

func extractJSSymbol(node *sitter.Node, nodeType string, src []byte, filename, lang, parent string, symbols *[]Symbol) {
	switch nodeType {
	case "function_declaration":
		name := getChildByField(node, "name", src)
		if name == "" {
			return
		}
		sym := Symbol{
			Name:      name,
			Kind:      SymbolFunction,
			FilePath:  filename,
			Language:  lang,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Docstring: extractJSDocstring(node, src),
			Parent:    parent,
		}
		*symbols = append(*symbols, sym)

	case "class_declaration":
		name := getChildByField(node, "name", src)
		if name == "" {
			return
		}
		sym := Symbol{
			Name:      name,
			Kind:      SymbolClass,
			FilePath:  filename,
			Language:  lang,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Parent:    parent,
		}
		*symbols = append(*symbols, sym)

		// Extract methods inside the class body
		body := node.ChildByFieldName("body")
		if body != nil {
			for i := 0; i < int(body.ChildCount()); i++ {
				child := body.Child(i)
				if child.Type() == "method_definition" {
					mname := getChildByField(child, "name", src)
					if mname != "" {
						msym := Symbol{
							Name:      mname,
							Kind:      SymbolMethod,
							FilePath:  filename,
							Language:  lang,
							StartLine: int(child.StartPoint().Row) + 1,
							EndLine:   int(child.EndPoint().Row) + 1,
							Parent:    name,
						}
						*symbols = append(*symbols, msym)
					}
				}
			}
		}

	case "lexical_declaration", "variable_declaration":
		// Look for arrow functions: const foo = (a, b) => ...
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "variable_declarator" {
				nameNode := child.ChildByFieldName("name")
				valueNode := child.ChildByFieldName("value")
				if nameNode != nil && valueNode != nil && valueNode.Type() == "arrow_function" {
					sym := Symbol{
						Name:      nameNode.Content(src),
						Kind:      SymbolFunction,
						FilePath:  filename,
						Language:  lang,
						StartLine: int(node.StartPoint().Row) + 1,
						EndLine:   int(node.EndPoint().Row) + 1,
						Parent:    parent,
					}
					*symbols = append(*symbols, sym)
				}
			}
		}
	}
}

func extractTSSymbol(node *sitter.Node, nodeType string, src []byte, filename, lang, parent string, symbols *[]Symbol) {
	switch nodeType {
	case "function_declaration":
		name := getChildByField(node, "name", src)
		if name == "" {
			return
		}
		sym := Symbol{
			Name:      name,
			Kind:      SymbolFunction,
			FilePath:  filename,
			Language:  lang,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Parent:    parent,
		}
		*symbols = append(*symbols, sym)

	case "class_declaration":
		name := getChildByField(node, "name", src)
		if name == "" {
			return
		}
		sym := Symbol{
			Name:      name,
			Kind:      SymbolClass,
			FilePath:  filename,
			Language:  lang,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Parent:    parent,
		}
		*symbols = append(*symbols, sym)

		// Extract methods
		body := node.ChildByFieldName("body")
		if body != nil {
			for i := 0; i < int(body.ChildCount()); i++ {
				child := body.Child(i)
				if child.Type() == "method_definition" || child.Type() == "public_field_definition" {
					mname := getChildByField(child, "name", src)
					if mname != "" {
						msym := Symbol{
							Name:      mname,
							Kind:      SymbolMethod,
							FilePath:  filename,
							Language:  lang,
							StartLine: int(child.StartPoint().Row) + 1,
							EndLine:   int(child.EndPoint().Row) + 1,
							Parent:    name,
						}
						*symbols = append(*symbols, msym)
					}
				}
			}
		}

	case "interface_declaration":
		name := getChildByField(node, "name", src)
		if name == "" {
			return
		}
		sym := Symbol{
			Name:      name,
			Kind:      SymbolInterface,
			FilePath:  filename,
			Language:  lang,
			StartLine: int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Parent:    parent,
		}
		*symbols = append(*symbols, sym)
	}
}

func getChildByField(node *sitter.Node, field string, src []byte) string {
	child := node.ChildByFieldName(field)
	if child == nil {
		return ""
	}
	return child.Content(src)
}

func extractPythonDocstring(node *sitter.Node, src []byte) string {
	body := node.ChildByFieldName("body")
	if body == nil {
		return ""
	}
	if body.ChildCount() == 0 {
		return ""
	}
	first := body.Child(0)
	if first.Type() == "expression_statement" && first.ChildCount() > 0 {
		strNode := first.Child(0)
		if strNode.Type() == "string" || strNode.Type() == "concatenated_string" {
			doc := strNode.Content(src)
			doc = strings.Trim(doc, "\"'")
			doc = strings.TrimPrefix(doc, "\"\"")
			doc = strings.TrimSuffix(doc, "\"\"")
			return strings.TrimSpace(doc)
		}
	}
	return ""
}

func extractJSDocstring(node *sitter.Node, src []byte) string {
	// Look for JSDoc comment preceding the node
	if node.PrevSibling() != nil && node.PrevSibling().Type() == "comment" {
		return strings.TrimSpace(node.PrevSibling().Content(src))
	}
	return ""
}
