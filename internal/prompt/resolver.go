package prompt

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/templates"
	"gopkg.in/yaml.v3"
)

// ResolvedPrompt contains the resolved prompt content and metadata.
type ResolvedPrompt struct {
	Content string `json:"content"`
	Source  Source `json:"source"`
	// InheritedFrom tracks the chain of inheritance if extends was used.
	InheritedFrom []Source `json:"inherited_from,omitempty"`
}

// PromptMeta contains frontmatter metadata for prompt inheritance.
type PromptMeta struct {
	Extends string `yaml:"extends"` // Source to inherit from: embedded, shared, project
	Prepend string `yaml:"prepend"` // Content to prepend to parent
	Append  string `yaml:"append"`  // Content to append to parent
}

// Resolver resolves prompts from multiple sources with inheritance support.
type Resolver struct {
	personalDir string   // ~/.orc/prompts/
	localDir    string   // .orc/local/prompts/
	sharedDir   string   // .orc/shared/prompts/
	projectDir  string   // .orc/prompts/
	embedded    bool     // Whether to check embedded templates
}

// ResolverOption configures a Resolver.
type ResolverOption func(*Resolver)

// WithPersonalDir sets the personal prompts directory (~/.orc/prompts/).
func WithPersonalDir(dir string) ResolverOption {
	return func(r *Resolver) {
		r.personalDir = dir
	}
}

// WithLocalDir sets the local prompts directory (.orc/local/prompts/).
func WithLocalDir(dir string) ResolverOption {
	return func(r *Resolver) {
		r.localDir = dir
	}
}

// WithSharedDir sets the shared prompts directory (.orc/shared/prompts/).
func WithSharedDir(dir string) ResolverOption {
	return func(r *Resolver) {
		r.sharedDir = dir
	}
}

// WithProjectDir sets the project prompts directory (.orc/prompts/).
func WithProjectDir(dir string) ResolverOption {
	return func(r *Resolver) {
		r.projectDir = dir
	}
}

// WithEmbedded enables or disables checking embedded templates.
func WithEmbedded(enabled bool) ResolverOption {
	return func(r *Resolver) {
		r.embedded = enabled
	}
}

// NewResolver creates a new Resolver with the given options.
func NewResolver(opts ...ResolverOption) *Resolver {
	r := &Resolver{
		embedded: true, // Default to checking embedded
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// NewResolverFromOrcDir creates a Resolver configured for a project.
func NewResolverFromOrcDir(orcDir string) *Resolver {
	homeDir, _ := os.UserHomeDir()
	return NewResolver(
		WithPersonalDir(filepath.Join(homeDir, ".orc", "prompts")),
		WithLocalDir(filepath.Join(orcDir, "local", "prompts")),
		WithSharedDir(filepath.Join(orcDir, "shared", "prompts")),
		WithProjectDir(filepath.Join(orcDir, "prompts")),
		WithEmbedded(true),
	)
}

// Resolve returns the prompt content for a phase, checking sources in priority order:
// 1. Personal (~/.orc/prompts/)
// 2. Local (.orc/local/prompts/)
// 3. Shared (.orc/shared/prompts/)
// 4. Project (.orc/prompts/)
// 5. Embedded (built-in)
//
// If the prompt has inheritance frontmatter, it will resolve the parent and combine.
func (r *Resolver) Resolve(phase string) (*ResolvedPrompt, error) {
	filename := phase + ".md"

	// Try sources in priority order
	sources := []struct {
		dir    string
		source Source
	}{
		{r.personalDir, SourcePersonalGlobal},
		{r.localDir, SourceProjectLocal},
		{r.sharedDir, SourceProjectShared},
		{r.projectDir, SourceProject},
	}

	for _, s := range sources {
		if s.dir == "" {
			continue
		}
		path := filepath.Join(s.dir, filename)
		content, err := os.ReadFile(path)
		if err != nil {
			continue // File doesn't exist, try next
		}
		return r.resolveWithInheritance(string(content), s.source, phase)
	}

	// Fall back to embedded
	if r.embedded {
		content, err := r.readEmbedded(phase)
		if err != nil {
			return nil, fmt.Errorf("prompt not found: %s", phase)
		}
		return &ResolvedPrompt{
			Content: content,
			Source:  SourceEmbedded,
		}, nil
	}

	return nil, fmt.Errorf("prompt not found: %s", phase)
}

// ResolveFromSource resolves a prompt from a specific source.
func (r *Resolver) ResolveFromSource(phase string, source Source) (*ResolvedPrompt, error) {
	filename := phase + ".md"
	var content string
	var err error

	switch source {
	case SourcePersonalGlobal:
		if r.personalDir == "" {
			return nil, fmt.Errorf("personal directory not configured")
		}
		var data []byte
		data, err = os.ReadFile(filepath.Join(r.personalDir, filename))
		content = string(data)
	case SourceProjectLocal:
		if r.localDir == "" {
			return nil, fmt.Errorf("local directory not configured")
		}
		var data []byte
		data, err = os.ReadFile(filepath.Join(r.localDir, filename))
		content = string(data)
	case SourceProjectShared:
		if r.sharedDir == "" {
			return nil, fmt.Errorf("shared directory not configured")
		}
		var data []byte
		data, err = os.ReadFile(filepath.Join(r.sharedDir, filename))
		content = string(data)
	case SourceProject:
		if r.projectDir == "" {
			return nil, fmt.Errorf("project directory not configured")
		}
		var data []byte
		data, err = os.ReadFile(filepath.Join(r.projectDir, filename))
		content = string(data)
	case SourceEmbedded:
		content, err = r.readEmbedded(phase)
	default:
		return nil, fmt.Errorf("unknown source: %s", source)
	}

	if err != nil {
		return nil, fmt.Errorf("read prompt %s from %s: %w", phase, source, err)
	}

	return r.resolveWithInheritance(content, source, phase)
}

// resolveWithInheritance handles prompt inheritance via frontmatter.
func (r *Resolver) resolveWithInheritance(content string, source Source, phase string) (*ResolvedPrompt, error) {
	meta, body := parseFrontmatter(content)

	// No inheritance, return as-is
	if meta.Extends == "" {
		return &ResolvedPrompt{
			Content: body,
			Source:  source,
		}, nil
	}

	// Determine parent source
	var parentSource Source
	switch meta.Extends {
	case "embedded":
		parentSource = SourceEmbedded
	case "shared":
		parentSource = SourceProjectShared
	case "project":
		parentSource = SourceProject
	case "local":
		parentSource = SourceProjectLocal
	case "personal":
		parentSource = SourcePersonalGlobal
	default:
		return nil, fmt.Errorf("unknown extends value: %s", meta.Extends)
	}

	// Resolve parent
	parent, err := r.ResolveFromSource(phase, parentSource)
	if err != nil {
		return nil, fmt.Errorf("resolve parent prompt: %w", err)
	}

	// Combine content
	var result strings.Builder
	if meta.Prepend != "" {
		result.WriteString(strings.TrimSpace(meta.Prepend))
		result.WriteString("\n\n")
	}
	result.WriteString(parent.Content)
	if meta.Append != "" {
		result.WriteString("\n\n")
		result.WriteString(strings.TrimSpace(meta.Append))
	}

	// Track inheritance chain
	inherited := append([]Source{parentSource}, parent.InheritedFrom...)

	return &ResolvedPrompt{
		Content:       result.String(),
		Source:        source,
		InheritedFrom: inherited,
	}, nil
}

// readEmbedded reads a prompt from embedded templates.
func (r *Resolver) readEmbedded(phase string) (string, error) {
	path := fmt.Sprintf("prompts/%s.md", phase)
	content, err := templates.Prompts.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// parseFrontmatter extracts YAML frontmatter from markdown content.
// Returns the parsed metadata and the body (content after frontmatter).
func parseFrontmatter(content string) (PromptMeta, string) {
	var meta PromptMeta

	// Check for frontmatter delimiter
	if !strings.HasPrefix(content, "---") {
		return meta, content
	}

	// Find end of frontmatter
	scanner := bufio.NewScanner(strings.NewReader(content))
	var frontmatter strings.Builder
	var body strings.Builder
	inFrontmatter := false
	frontmatterClosed := false
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if lineNum == 1 && line == "---" {
			inFrontmatter = true
			continue
		}

		if inFrontmatter && line == "---" {
			inFrontmatter = false
			frontmatterClosed = true
			continue
		}

		if inFrontmatter {
			frontmatter.WriteString(line)
			frontmatter.WriteString("\n")
		} else if frontmatterClosed {
			if body.Len() > 0 {
				body.WriteString("\n")
			}
			body.WriteString(line)
		}
	}

	// Parse YAML frontmatter
	if frontmatterClosed {
		_ = yaml.Unmarshal([]byte(frontmatter.String()), &meta)
		return meta, strings.TrimSpace(body.String())
	}

	// No valid frontmatter found, return original content
	return meta, content
}

// SourcePriority returns the priority of a source (lower = higher priority).
func SourcePriority(s Source) int {
	switch s {
	case SourcePersonalGlobal:
		return 1
	case SourceProjectLocal:
		return 2
	case SourceProjectShared:
		return 3
	case SourceProject:
		return 4
	case SourceEmbedded:
		return 5
	default:
		return 99
	}
}

// SourceDisplayName returns a human-readable name for the source.
func SourceDisplayName(s Source) string {
	switch s {
	case SourcePersonalGlobal:
		return "Personal (~/.orc/prompts/)"
	case SourceProjectLocal:
		return "Local (.orc/local/prompts/)"
	case SourceProjectShared:
		return "Shared (.orc/shared/prompts/)"
	case SourceProject:
		return "Project (.orc/prompts/)"
	case SourceEmbedded:
		return "Embedded (built-in)"
	default:
		return string(s)
	}
}
