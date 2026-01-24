// Package templates provides embedded prompt templates.
package templates

import "embed"

// Prompts contains embedded prompt template files.
//
//go:embed prompts/*.md
var Prompts embed.FS

// Agents contains embedded agent definition files.
// Each file has YAML frontmatter (name, description, model, tools) and markdown body (prompt).
//
//go:embed agents/*.md
var Agents embed.FS
