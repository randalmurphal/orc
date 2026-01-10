// Package templates provides embedded plan templates.
package templates

import "embed"

// Plans contains embedded plan template files.
//
//go:embed plans/*.yaml
var Plans embed.FS
