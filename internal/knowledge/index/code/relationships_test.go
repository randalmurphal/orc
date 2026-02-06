package code

import (
	"testing"
)

// --- SC-5: Relationship extractor ---

// SC-5: Import relationships link importing file to imported module.
func TestRelationships_Imports(t *testing.T) {
	symbols := []Symbol{
		{
			Name: "main", Kind: SymbolFunction, FilePath: "main.go",
			StartLine: 5, EndLine: 10, Language: "go",
		},
	}

	// Source content for import extraction
	files := map[string]string{
		"main.go": `package main

import (
	"fmt"
	"os"
	"github.com/example/lib"
)

func main() {
	fmt.Println("hello")
}
`,
	}

	rels, err := ExtractRelationships(symbols, files)
	if err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}

	// Should find import relationships
	importRels := filterRels(rels, RelImport)
	if len(importRels) == 0 {
		t.Fatal("should find import relationships")
	}

	// Verify imports include the expected packages
	targets := relTargets(importRels)
	assertInSlice(t, targets, "fmt")
	assertInSlice(t, targets, "os")
}

// SC-5: Call relationships link caller to callee.
func TestRelationships_Calls(t *testing.T) {
	symbols := []Symbol{
		{
			Name: "processData", Kind: SymbolFunction, FilePath: "service.go",
			StartLine: 1, EndLine: 10, Language: "go",
		},
		{
			Name: "validate", Kind: SymbolFunction, FilePath: "service.go",
			StartLine: 12, EndLine: 20, Language: "go",
		},
	}

	files := map[string]string{
		"service.go": `func processData(data string) error {
	if err := validate(data); err != nil {
		return err
	}
	return nil
}

func validate(data string) error {
	if data == "" {
		return fmt.Errorf("empty data")
	}
	return nil
}
`,
	}

	rels, err := ExtractRelationships(symbols, files)
	if err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}

	callRels := filterRels(rels, RelCall)
	if len(callRels) == 0 {
		t.Fatal("should find call relationships")
	}

	// processData should call validate
	found := false
	for _, r := range callRels {
		if r.SourceName == "processData" && r.TargetName == "validate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should find call relationship: processData -> validate")
	}
}

// SC-5: Extends/implements relationships link child type to parent.
func TestRelationships_ExtendsImplements(t *testing.T) {
	symbols := []Symbol{
		{Name: "Animal", Kind: SymbolClass, FilePath: "models.py", StartLine: 1, EndLine: 5, Language: "python"},
		{Name: "Dog", Kind: SymbolClass, FilePath: "models.py", StartLine: 7, EndLine: 15, Language: "python"},
	}

	files := map[string]string{
		"models.py": `class Animal:
    def speak(self):
        pass

class Dog(Animal):
    def speak(self):
        return "Woof"
`,
	}

	rels, err := ExtractRelationships(symbols, files)
	if err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}

	extRels := filterRels(rels, RelExtends)
	if len(extRels) == 0 {
		t.Fatal("should find extends relationship")
	}

	found := false
	for _, r := range extRels {
		if r.SourceName == "Dog" && r.TargetName == "Animal" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should find extends relationship: Dog -> Animal")
	}
}

// SC-5: Unresolvable targets (external packages) stored as-is without error.
func TestRelationships_UnresolvableTargets(t *testing.T) {
	symbols := []Symbol{
		{Name: "main", Kind: SymbolFunction, FilePath: "main.go", StartLine: 3, EndLine: 8, Language: "go"},
	}

	files := map[string]string{
		"main.go": `package main

import "github.com/external/unknown"

func main() {
	unknown.DoSomething()
}
`,
	}

	rels, err := ExtractRelationships(symbols, files)
	if err != nil {
		t.Fatalf("ExtractRelationships should not error on external packages: %v", err)
	}

	// Should still have import relationship to external package
	importRels := filterRels(rels, RelImport)
	found := false
	for _, r := range importRels {
		if r.TargetName == "github.com/external/unknown" || r.TargetFile == "github.com/external/unknown" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should store external package import as-is")
	}
}

// SC-5: Source and target have proper file path and symbol name mapping.
func TestRelationships_SourceTargetMapping(t *testing.T) {
	symbols := []Symbol{
		{Name: "handler", Kind: SymbolFunction, FilePath: "handler.go", StartLine: 1, EndLine: 5, Language: "go"},
	}

	files := map[string]string{
		"handler.go": `func handler() {
	fmt.Println("handling")
}
`,
	}

	rels, err := ExtractRelationships(symbols, files)
	if err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}

	for _, r := range rels {
		if r.SourceFile == "" {
			t.Errorf("relationship %s -> %s missing SourceFile", r.SourceName, r.TargetName)
		}
		if r.SourceName == "" {
			t.Errorf("relationship from %s missing SourceName", r.SourceFile)
		}
	}
}

// --- Test helpers ---

func filterRels(rels []Relationship, kind RelationshipKind) []Relationship {
	var filtered []Relationship
	for _, r := range rels {
		if r.Kind == kind {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func relTargets(rels []Relationship) []string {
	targets := make([]string, len(rels))
	for i, r := range rels {
		targets[i] = r.TargetName
	}
	return targets
}

func assertInSlice(t *testing.T, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if s == want {
			return
		}
	}
	t.Errorf("slice %v should contain %q", slice, want)
}
