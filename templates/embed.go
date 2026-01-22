// Package templates provides embedded prompt templates.
package templates

import "embed"

// Prompts contains embedded prompt template files.
//
//go:embed prompts/*.md
var Prompts embed.FS
