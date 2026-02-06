package code

import (
	"fmt"
	"strings"
)

const hierarchicalSplitThreshold = 50

// Chunker splits parsed symbols into self-contained chunks for embedding.
type Chunker struct{}

// NewChunker creates a new chunker.
func NewChunker() *Chunker {
	return &Chunker{}
}

// Chunk converts symbols into chunks with content from the source map.
// content maps file paths to their full source content.
func (c *Chunker) Chunk(symbols []Symbol, content map[string]string) ([]Chunk, error) {
	if len(symbols) == 0 && len(content) == 0 {
		return nil, nil
	}

	// If we have content but no symbols, produce file-level chunks.
	if len(symbols) == 0 {
		var chunks []Chunk
		for filePath, src := range content {
			if strings.TrimSpace(src) == "" {
				continue
			}
			chunks = append(chunks, Chunk{
				Symbol:    filePath,
				Content:   src,
				FilePath:  filePath,
				Kind:      "file",
				StartLine: 1,
				EndLine:   strings.Count(src, "\n") + 1,
			})
		}
		return chunks, nil
	}

	// Track which classes need hierarchical splitting
	classChildren := make(map[string]int) // className -> method count
	for i := range symbols {
		s := &symbols[i]
		if (s.Kind == SymbolClass || s.Kind == SymbolType) && len(s.Children) > 0 {
			classChildren[s.Name] = len(s.Children)
		}
	}

	var chunks []Chunk
	for i := range symbols {
		s := &symbols[i]
		src := content[s.FilePath]
		chunkContent := extractContent(src, s.StartLine, s.EndLine)

		// Skip methods that belong to a class — they'll be handled via class
		// but we still need to emit individual method chunks
		if s.Kind == SymbolMethod {
			ch := Chunk{
				Symbol:    s.Name,
				Content:   chunkContent,
				FilePath:  s.FilePath,
				Kind:      s.Kind,
				StartLine: s.StartLine,
				EndLine:   s.EndLine,
			}
			// Add context header for methods
			if s.Parent != "" {
				ch.Context = fmt.Sprintf("// File: %s | Parent: %s", s.FilePath, s.Parent)
			}
			chunks = append(chunks, ch)
			continue
		}

		// Check if this is a class/type needing hierarchical split
		if (s.Kind == SymbolClass || s.Kind == SymbolType) && classChildren[s.Name] > hierarchicalSplitThreshold {
			// Produce summary chunk instead of full content
			summary := buildClassSummary(s, src)
			chunks = append(chunks, Chunk{
				Symbol:    s.Name,
				Content:   summary,
				FilePath:  s.FilePath,
				Kind:      s.Kind,
				StartLine: s.StartLine,
				EndLine:   s.EndLine,
			})
			continue
		}

		// Regular symbol chunk
		chunks = append(chunks, Chunk{
			Symbol:    s.Name,
			Content:   chunkContent,
			FilePath:  s.FilePath,
			Kind:      s.Kind,
			StartLine: s.StartLine,
			EndLine:   s.EndLine,
		})
	}

	return chunks, nil
}

func extractContent(src string, startLine, endLine int) string {
	if src == "" {
		return ""
	}
	lines := strings.Split(src, "\n")
	start := startLine - 1
	end := endLine
	if start < 0 {
		start = 0
	}
	if start >= len(lines) {
		// Line numbers beyond content — return full source as fallback
		return src
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}

func buildClassSummary(s *Symbol, src string) string {
	var sb strings.Builder
	// Include the class declaration line and docstring area
	lines := strings.Split(src, "\n")
	start := s.StartLine - 1
	if start < 0 {
		start = 0
	}
	// Take first few lines as class header (declaration + docstring)
	headerEnd := start + 5
	if headerEnd > len(lines) {
		headerEnd = len(lines)
	}
	for i := start; i < headerEnd; i++ {
		sb.WriteString(lines[i])
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("\n// ... %d methods (see individual method chunks)\n", len(s.Children)))
	return sb.String()
}
