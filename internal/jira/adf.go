package jira

import (
	"fmt"
	"strings"

	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
)

// ADFToMarkdown converts an Atlassian Document Format node tree to Markdown.
// Returns empty string for nil input. Unsupported node types produce
// [unsupported: type] placeholders rather than silently dropping content.
func ADFToMarkdown(node *models.CommentNodeScheme) string {
	if node == nil {
		return ""
	}
	var b strings.Builder
	renderNode(&b, node, 0, false)
	return strings.TrimRight(b.String(), "\n")
}

func renderNode(b *strings.Builder, node *models.CommentNodeScheme, depth int, inList bool) {
	if node == nil {
		return
	}

	switch node.Type {
	case "doc":
		renderChildren(b, node, depth, false)

	case "paragraph":
		renderChildren(b, node, depth, false)
		if !inList {
			b.WriteString("\n\n")
		} else {
			b.WriteString("\n")
		}

	case "heading":
		level := attrInt(node.Attrs, "level", 1)
		b.WriteString(strings.Repeat("#", level))
		b.WriteString(" ")
		renderChildren(b, node, depth, false)
		b.WriteString("\n\n")

	case "text":
		text := node.Text
		text = applyMarks(text, node.Marks)
		b.WriteString(text)

	case "hardBreak":
		b.WriteString("  \n")

	case "bulletList":
		renderListItems(b, node, depth, "- ")

	case "orderedList":
		for i, child := range node.Content {
			prefix := fmt.Sprintf("%d. ", i+1)
			indent := strings.Repeat("  ", depth)
			b.WriteString(indent)
			b.WriteString(prefix)
			renderListItemContent(b, child, depth+1)
		}

	case "listItem":
		// Handled by parent list node
		renderChildren(b, node, depth, true)

	case "codeBlock":
		lang := attrString(node.Attrs, "language", "")
		b.WriteString("```")
		b.WriteString(lang)
		b.WriteString("\n")
		renderChildren(b, node, depth, false)
		b.WriteString("\n```\n\n")

	case "blockquote":
		var inner strings.Builder
		renderChildren(&inner, node, depth, false)
		for _, line := range strings.Split(strings.TrimRight(inner.String(), "\n"), "\n") {
			b.WriteString("> ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")

	case "rule":
		b.WriteString("---\n\n")

	case "table":
		renderTable(b, node)

	case "mediaSingle", "mediaGroup":
		// Media nodes can't be converted to markdown meaningfully
		b.WriteString("[media]\n\n")

	case "mention":
		name := attrString(node.Attrs, "text", "")
		if name == "" {
			name = "@mention"
		}
		b.WriteString(name)

	case "emoji":
		shortName := attrString(node.Attrs, "shortName", "")
		if shortName != "" {
			b.WriteString(shortName)
		}

	case "inlineCard":
		url := attrString(node.Attrs, "url", "")
		if url != "" {
			b.WriteString(url)
		}

	default:
		// Don't silently drop content
		b.WriteString(fmt.Sprintf("[unsupported: %s]", node.Type))
		renderChildren(b, node, depth, false)
	}
}

func renderChildren(b *strings.Builder, node *models.CommentNodeScheme, depth int, inList bool) {
	for _, child := range node.Content {
		renderNode(b, child, depth, inList)
	}
}

func renderListItems(b *strings.Builder, node *models.CommentNodeScheme, depth int, prefix string) {
	for _, child := range node.Content {
		indent := strings.Repeat("  ", depth)
		b.WriteString(indent)
		b.WriteString(prefix)
		renderListItemContent(b, child, depth+1)
	}
}

func renderListItemContent(b *strings.Builder, node *models.CommentNodeScheme, depth int) {
	if node == nil {
		b.WriteString("\n")
		return
	}
	for i, child := range node.Content {
		if i == 0 && child.Type == "paragraph" {
			// First paragraph inline with bullet
			renderChildren(b, child, depth, true)
			b.WriteString("\n")
		} else {
			renderNode(b, child, depth, true)
		}
	}
}

func renderTable(b *strings.Builder, table *models.CommentNodeScheme) {
	if len(table.Content) == 0 {
		return
	}

	// Collect all rows
	var rows [][]string
	for _, row := range table.Content {
		if row.Type != "tableRow" {
			continue
		}
		var cells []string
		for _, cell := range row.Content {
			var cellBuf strings.Builder
			renderChildren(&cellBuf, cell, 0, false)
			cells = append(cells, strings.TrimSpace(cellBuf.String()))
		}
		rows = append(rows, cells)
	}

	if len(rows) == 0 {
		return
	}

	// First row is header
	b.WriteString("| ")
	b.WriteString(strings.Join(rows[0], " | "))
	b.WriteString(" |\n")

	// Separator
	b.WriteString("|")
	for range rows[0] {
		b.WriteString(" --- |")
	}
	b.WriteString("\n")

	// Data rows
	for _, row := range rows[1:] {
		b.WriteString("| ")
		b.WriteString(strings.Join(row, " | "))
		b.WriteString(" |\n")
	}
	b.WriteString("\n")
}

func applyMarks(text string, marks []*models.MarkScheme) string {
	for _, mark := range marks {
		switch mark.Type {
		case "strong":
			text = "**" + text + "**"
		case "em":
			text = "*" + text + "*"
		case "code":
			text = "`" + text + "`"
		case "strike":
			text = "~~" + text + "~~"
		case "underline":
			// Markdown doesn't have underline, use emphasis
			text = "_" + text + "_"
		case "link":
			href := ""
			if mark.Attrs != nil {
				if h, ok := mark.Attrs["href"]; ok {
					if s, ok := h.(string); ok {
						href = s
					}
				}
			}
			if href != "" {
				text = "[" + text + "](" + href + ")"
			}
		case "subsup":
			// No markdown equivalent, pass through
		}
	}
	return text
}

func attrString(attrs map[string]interface{}, key, fallback string) string {
	if attrs == nil {
		return fallback
	}
	v, ok := attrs[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok {
		return fallback
	}
	return s
}

func attrInt(attrs map[string]interface{}, key string, fallback int) int {
	if attrs == nil {
		return fallback
	}
	v, ok := attrs[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return fallback
	}
}
