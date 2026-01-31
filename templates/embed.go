// Package templates provides embedded prompt templates.
package templates

import "embed"

// Prompts contains embedded prompt template files (user prompts with task instructions).
//
//go:embed prompts/*.md
var Prompts embed.FS

// SystemPrompts contains role-framing system prompts for phase agents.
// These set behavioral context and expectations for each phase's Claude invocation.
//
//go:embed system_prompts/*.md
var SystemPrompts embed.FS

// Agents contains embedded agent definition files.
// Each file has YAML frontmatter (name, description, model, tools) and markdown body (prompt).
//
//go:embed agents/*.md
var Agents embed.FS

// Workflows contains embedded workflow definition files.
// Each file is a YAML definition of a workflow with its phases and variables.
//
//go:embed workflows/*.yaml
var Workflows embed.FS

// Phases contains embedded phase template definition files.
// Each file is a YAML definition of a phase template with its configuration.
//
//go:embed phases/*.yaml
var Phases embed.FS

// Hooks contains embedded hook script files.
// Each file is a standalone hook script (bash or python) for Claude Code hooks.
//
//go:embed hooks/*
var Hooks embed.FS
