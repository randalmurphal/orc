package code

import (
	"fmt"
	"strings"
	"testing"
)

// --- SC-7: Pattern detector ---

// SC-7: Clusters files with >80% AST structural similarity; 5+ members → named pattern.
func TestPatterns_ClusterSimilarFiles(t *testing.T) {
	// Create 6 structurally similar files:
	// Each has a class with __init__ and 3 methods sharing the same structure
	files := make(map[string][]Symbol)
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("service_%d.py", i)
		files[name] = []Symbol{
			{Name: fmt.Sprintf("Service%d", i), Kind: SymbolClass, FilePath: name, StartLine: 1, EndLine: 20},
			{Name: "__init__", Kind: SymbolMethod, FilePath: name, StartLine: 2, EndLine: 4, Parent: fmt.Sprintf("Service%d", i)},
			{Name: "process", Kind: SymbolMethod, FilePath: name, StartLine: 6, EndLine: 10, Parent: fmt.Sprintf("Service%d", i)},
			{Name: "validate", Kind: SymbolMethod, FilePath: name, StartLine: 12, EndLine: 16, Parent: fmt.Sprintf("Service%d", i)},
			{Name: "cleanup", Kind: SymbolMethod, FilePath: name, StartLine: 18, EndLine: 20, Parent: fmt.Sprintf("Service%d", i)},
		}
	}

	d := NewPatternDetector()
	patterns, err := d.Detect(files)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("should detect at least one pattern from 6 similar files")
	}

	// Verify the pattern has 6 members
	found := false
	for _, p := range patterns {
		if p.MemberCount >= 6 {
			found = true
			break
		}
	}
	if !found {
		t.Error("should have a pattern with member_count >= 6")
	}
}

// SC-7: Named pattern with 5+ members has canonical example file.
func TestPatterns_CanonicalFile(t *testing.T) {
	files := make(map[string][]Symbol)
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("handler_%d.go", i)
		files[name] = []Symbol{
			{Name: fmt.Sprintf("Handle%d", i), Kind: SymbolFunction, FilePath: name, StartLine: 1, EndLine: 10},
			{Name: fmt.Sprintf("validate%d", i), Kind: SymbolFunction, FilePath: name, StartLine: 12, EndLine: 20},
		}
	}

	d := NewPatternDetector()
	patterns, err := d.Detect(files)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("should detect pattern from 5 similar files")
	}

	for _, p := range patterns {
		if p.CanonicalFile == "" {
			t.Errorf("pattern %s should have a canonical_file", p.Name)
		}
		if p.Name == "" {
			t.Error("pattern should have a name")
		}
	}
}

// SC-7: Individual files in a cluster are marked with follows_pattern.
func TestPatterns_FollowsPattern(t *testing.T) {
	files := make(map[string][]Symbol)
	for i := 0; i < 7; i++ {
		name := fmt.Sprintf("worker_%d.py", i)
		files[name] = []Symbol{
			{Name: fmt.Sprintf("Worker%d", i), Kind: SymbolClass, FilePath: name, StartLine: 1, EndLine: 15},
			{Name: "run", Kind: SymbolMethod, FilePath: name, StartLine: 3, EndLine: 8, Parent: fmt.Sprintf("Worker%d", i)},
			{Name: "stop", Kind: SymbolMethod, FilePath: name, StartLine: 10, EndLine: 15, Parent: fmt.Sprintf("Worker%d", i)},
		}
	}

	d := NewPatternDetector()
	patterns, err := d.Detect(files)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("should detect pattern")
	}

	// Each pattern should list its members
	for _, p := range patterns {
		if len(p.Members) == 0 {
			t.Errorf("pattern %s should list member files", p.Name)
		}
		if p.MemberCount != len(p.Members) {
			t.Errorf("pattern %s MemberCount=%d but Members has %d entries",
				p.Name, p.MemberCount, len(p.Members))
		}
	}
}

// SC-7: Files below 80% similarity are not clustered.
func TestPatterns_BelowThreshold(t *testing.T) {
	files := map[string][]Symbol{
		"service.py": {
			{Name: "Service", Kind: SymbolClass, FilePath: "service.py", StartLine: 1, EndLine: 30},
			{Name: "__init__", Kind: SymbolMethod, FilePath: "service.py", StartLine: 2, EndLine: 5, Parent: "Service"},
			{Name: "process", Kind: SymbolMethod, FilePath: "service.py", StartLine: 7, EndLine: 15, Parent: "Service"},
			{Name: "validate", Kind: SymbolMethod, FilePath: "service.py", StartLine: 17, EndLine: 25, Parent: "Service"},
			{Name: "cleanup", Kind: SymbolMethod, FilePath: "service.py", StartLine: 27, EndLine: 30, Parent: "Service"},
		},
		"config.py": {
			// Very different structure — just a single function
			{Name: "load_config", Kind: SymbolFunction, FilePath: "config.py", StartLine: 1, EndLine: 5},
		},
	}

	d := NewPatternDetector()
	patterns, err := d.Detect(files)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	// Two very different files should not form a pattern
	for _, p := range patterns {
		if p.MemberCount >= 2 {
			// Check that both files aren't in the same cluster
			hasBoth := false
			for _, m := range p.Members {
				if strings.Contains(m, "service.py") || strings.Contains(m, "config.py") {
					hasBoth = true
				}
			}
			if hasBoth && p.MemberCount == 2 {
				t.Error("structurally different files should not be clustered together")
			}
		}
	}
}

// SC-7: Projects with fewer than 5 similar files produce no patterns.
func TestPatterns_LessThan5Similar(t *testing.T) {
	files := make(map[string][]Symbol)
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("handler_%d.go", i)
		files[name] = []Symbol{
			{Name: fmt.Sprintf("Handle%d", i), Kind: SymbolFunction, FilePath: name, StartLine: 1, EndLine: 10},
		}
	}

	d := NewPatternDetector()
	patterns, err := d.Detect(files)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("4 similar files should not produce patterns (need 5+), got %d", len(patterns))
	}
}

// SC-7 edge case: Empty project produces no patterns.
func TestPatterns_EmptyProject(t *testing.T) {
	d := NewPatternDetector()
	patterns, err := d.Detect(nil)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(patterns) != 0 {
		t.Errorf("empty project should produce 0 patterns, got %d", len(patterns))
	}
}

// SC-7 edge case: Files that fail parsing are excluded from pattern analysis.
func TestPatterns_UnparseableExcluded(t *testing.T) {
	// 5 well-formed files + 2 that would have empty symbols (unparseable)
	files := make(map[string][]Symbol)
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("good_%d.py", i)
		files[name] = []Symbol{
			{Name: fmt.Sprintf("Processor%d", i), Kind: SymbolClass, FilePath: name, StartLine: 1, EndLine: 10},
			{Name: "run", Kind: SymbolMethod, FilePath: name, StartLine: 3, EndLine: 10, Parent: fmt.Sprintf("Processor%d", i)},
		}
	}
	// Files with no symbols (failed to parse)
	files["broken_1.py"] = nil
	files["broken_2.py"] = nil

	d := NewPatternDetector()
	patterns, err := d.Detect(files)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	// Should still find pattern from the 5 good files
	if len(patterns) == 0 {
		t.Fatal("should detect pattern from 5 parseable files, excluding unparseable ones")
	}

	// Broken files should not be in any pattern's members
	for _, p := range patterns {
		for _, m := range p.Members {
			if strings.HasPrefix(m, "broken_") {
				t.Errorf("unparseable file %s should not be in pattern members", m)
			}
		}
	}
}
