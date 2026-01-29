package jira

import (
	"testing"

	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
)

func TestADFToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		node     *models.CommentNodeScheme
		expected string
	}{
		{
			name:     "nil input",
			node:     nil,
			expected: "",
		},
		{
			name: "empty doc",
			node: &models.CommentNodeScheme{
				Type: "doc",
			},
			expected: "",
		},
		{
			name: "simple paragraph",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "Hello world"},
						},
					},
				},
			},
			expected: "Hello world",
		},
		{
			name: "bold text",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{
								Type: "text",
								Text: "bold",
								Marks: []*models.MarkScheme{
									{Type: "strong"},
								},
							},
						},
					},
				},
			},
			expected: "**bold**",
		},
		{
			name: "italic text",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{
								Type: "text",
								Text: "italic",
								Marks: []*models.MarkScheme{
									{Type: "em"},
								},
							},
						},
					},
				},
			},
			expected: "*italic*",
		},
		{
			name: "inline code",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{
								Type: "text",
								Text: "code",
								Marks: []*models.MarkScheme{
									{Type: "code"},
								},
							},
						},
					},
				},
			},
			expected: "`code`",
		},
		{
			name: "link",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{
								Type: "text",
								Text: "click here",
								Marks: []*models.MarkScheme{
									{
										Type:  "link",
										Attrs: map[string]interface{}{"href": "https://example.com"},
									},
								},
							},
						},
					},
				},
			},
			expected: "[click here](https://example.com)",
		},
		{
			name: "strikethrough",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{
								Type: "text",
								Text: "removed",
								Marks: []*models.MarkScheme{
									{Type: "strike"},
								},
							},
						},
					},
				},
			},
			expected: "~~removed~~",
		},
		{
			name: "heading levels",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type:  "heading",
						Attrs: map[string]interface{}{"level": float64(2)},
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "Section"},
						},
					},
				},
			},
			expected: "## Section",
		},
		{
			name: "bullet list",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "bulletList",
						Content: []*models.CommentNodeScheme{
							{
								Type: "listItem",
								Content: []*models.CommentNodeScheme{
									{
										Type: "paragraph",
										Content: []*models.CommentNodeScheme{
											{Type: "text", Text: "first"},
										},
									},
								},
							},
							{
								Type: "listItem",
								Content: []*models.CommentNodeScheme{
									{
										Type: "paragraph",
										Content: []*models.CommentNodeScheme{
											{Type: "text", Text: "second"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: "- first\n- second",
		},
		{
			name: "ordered list",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "orderedList",
						Content: []*models.CommentNodeScheme{
							{
								Type: "listItem",
								Content: []*models.CommentNodeScheme{
									{
										Type: "paragraph",
										Content: []*models.CommentNodeScheme{
											{Type: "text", Text: "alpha"},
										},
									},
								},
							},
							{
								Type: "listItem",
								Content: []*models.CommentNodeScheme{
									{
										Type: "paragraph",
										Content: []*models.CommentNodeScheme{
											{Type: "text", Text: "beta"},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: "1. alpha\n2. beta",
		},
		{
			name: "code block",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type:  "codeBlock",
						Attrs: map[string]interface{}{"language": "go"},
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "fmt.Println(\"hello\")"},
						},
					},
				},
			},
			expected: "```go\nfmt.Println(\"hello\")\n```",
		},
		{
			name: "code block without language",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "codeBlock",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "some code"},
						},
					},
				},
			},
			expected: "```\nsome code\n```",
		},
		{
			name: "blockquote",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "blockquote",
						Content: []*models.CommentNodeScheme{
							{
								Type: "paragraph",
								Content: []*models.CommentNodeScheme{
									{Type: "text", Text: "quoted text"},
								},
							},
						},
					},
				},
			},
			expected: "> quoted text",
		},
		{
			name: "horizontal rule",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{Type: "rule"},
				},
			},
			expected: "---",
		},
		{
			name: "table",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "table",
						Content: []*models.CommentNodeScheme{
							{
								Type: "tableRow",
								Content: []*models.CommentNodeScheme{
									{
										Type: "tableHeader",
										Content: []*models.CommentNodeScheme{
											{
												Type: "paragraph",
												Content: []*models.CommentNodeScheme{
													{Type: "text", Text: "Name"},
												},
											},
										},
									},
									{
										Type: "tableHeader",
										Content: []*models.CommentNodeScheme{
											{
												Type: "paragraph",
												Content: []*models.CommentNodeScheme{
													{Type: "text", Text: "Value"},
												},
											},
										},
									},
								},
							},
							{
								Type: "tableRow",
								Content: []*models.CommentNodeScheme{
									{
										Type: "tableCell",
										Content: []*models.CommentNodeScheme{
											{
												Type: "paragraph",
												Content: []*models.CommentNodeScheme{
													{Type: "text", Text: "foo"},
												},
											},
										},
									},
									{
										Type: "tableCell",
										Content: []*models.CommentNodeScheme{
											{
												Type: "paragraph",
												Content: []*models.CommentNodeScheme{
													{Type: "text", Text: "bar"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: "| Name | Value |\n| --- | --- |\n| foo | bar |",
		},
		{
			name: "multiple paragraphs",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "First paragraph."},
						},
					},
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "Second paragraph."},
						},
					},
				},
			},
			expected: "First paragraph.\n\nSecond paragraph.",
		},
		{
			name: "mixed content",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type:  "heading",
						Attrs: map[string]interface{}{"level": float64(1)},
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "Title"},
						},
					},
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "Some "},
							{
								Type: "text",
								Text: "bold",
								Marks: []*models.MarkScheme{
									{Type: "strong"},
								},
							},
							{Type: "text", Text: " text."},
						},
					},
				},
			},
			expected: "# Title\n\nSome **bold** text.",
		},
		{
			name: "hard break",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "line one"},
							{Type: "hardBreak"},
							{Type: "text", Text: "line two"},
						},
					},
				},
			},
			expected: "line one  \nline two",
		},
		{
			name: "mention",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "Hey "},
							{
								Type:  "mention",
								Attrs: map[string]interface{}{"text": "@alice"},
							},
						},
					},
				},
			},
			expected: "Hey @alice",
		},
		{
			name: "emoji",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "Done "},
							{
								Type:  "emoji",
								Attrs: map[string]interface{}{"shortName": ":thumbsup:"},
							},
						},
					},
				},
			},
			expected: "Done :thumbsup:",
		},
		{
			name: "unsupported node type",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "unknownWidget",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "inner"},
						},
					},
				},
			},
			expected: "[unsupported: unknownWidget]inner",
		},
		{
			name: "inline card",
			node: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{
								Type:  "inlineCard",
								Attrs: map[string]interface{}{"url": "https://example.com/page"},
							},
						},
					},
				},
			},
			expected: "https://example.com/page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ADFToMarkdown(tt.node)
			if got != tt.expected {
				t.Errorf("ADFToMarkdown() =\n%q\nwant:\n%q", got, tt.expected)
			}
		})
	}
}
