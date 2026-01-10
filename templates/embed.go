// Package templates provides embedded plan and prompt templates.
package templates

import "embed"

// Plans contains embedded plan template files.
//
//go:embed plans/*.yaml
var Plans embed.FS

// Prompts contains embedded prompt template files.
//
//go:embed prompts/*.md
var Prompts embed.FS
