package config

import "fmt"

// ConfigLevel represents one of the 4 conceptual configuration levels.
// Higher levels override lower levels.
type ConfigLevel int

const (
	// LevelDefaults is built-in default values (lowest priority).
	LevelDefaults ConfigLevel = iota
	// LevelShared is team/project config (.orc/shared/, .orc/).
	LevelShared
	// LevelPersonal is user config (~/.orc/, .orc/local/).
	LevelPersonal
	// LevelRuntime is env vars and CLI flags (highest priority).
	LevelRuntime
)

// String returns the level name.
func (l ConfigLevel) String() string {
	return levelNames[l]
}

var levelNames = map[ConfigLevel]string{
	LevelDefaults: "default",
	LevelShared:   "shared",
	LevelPersonal: "personal",
	LevelRuntime:  "runtime",
}

// ConfigSource indicates where a configuration value came from.
// Kept as string for backward compatibility with existing code.
type ConfigSource string

const (
	// SourceDefault indicates a built-in default value.
	SourceDefault ConfigSource = "default"
	// SourceShared indicates team/project config (.orc/shared/, .orc/).
	SourceShared ConfigSource = "shared"
	// SourcePersonal indicates personal config (~/.orc/, .orc/local/).
	SourcePersonal ConfigSource = "personal"
	// SourceEnv indicates an environment variable override.
	SourceEnv ConfigSource = "env"
	// SourceFlag indicates a CLI flag override.
	SourceFlag ConfigSource = "flag"
)

// Level returns the ConfigLevel for this source.
func (s ConfigSource) Level() ConfigLevel {
	switch s {
	case SourceDefault:
		return LevelDefaults
	case SourceShared:
		return LevelShared
	case SourcePersonal:
		return LevelPersonal
	case SourceEnv, SourceFlag:
		return LevelRuntime
	default:
		return LevelDefaults
	}
}

// TrackedSource contains both the source type and the file path.
type TrackedSource struct {
	Source ConfigSource
	Path   string // File path or empty for defaults/env
}

// String returns a human-readable source description.
func (ts TrackedSource) String() string {
	if ts.Path == "" {
		return string(ts.Source)
	}
	return fmt.Sprintf("%s: %s", ts.Source, ts.Path)
}

// TrackedConfig wraps a Config with source tracking.
type TrackedConfig struct {
	// Config is the merged configuration.
	Config *Config

	// Sources maps config paths to their source type (for backward compat).
	// Examples: "profile" -> SourcePersonal, "retry.enabled" -> SourceEnv
	Sources map[string]ConfigSource

	// TrackedSources maps config paths to their full source info (source + path).
	TrackedSources map[string]TrackedSource
}

// NewTrackedConfig creates a new TrackedConfig with defaults.
func NewTrackedConfig() *TrackedConfig {
	return &TrackedConfig{
		Config:         Default(),
		Sources:        make(map[string]ConfigSource),
		TrackedSources: make(map[string]TrackedSource),
	}
}

// SetSource records the source for a config path.
func (tc *TrackedConfig) SetSource(path string, source ConfigSource) {
	tc.Sources[path] = source
	tc.TrackedSources[path] = TrackedSource{Source: source}
}

// SetSourceWithPath records the source and file path for a config path.
func (tc *TrackedConfig) SetSourceWithPath(path string, source ConfigSource, filePath string) {
	tc.Sources[path] = source
	tc.TrackedSources[path] = TrackedSource{Source: source, Path: filePath}
}

// GetSource returns the source for a config path.
// Returns SourceDefault if no source is recorded.
func (tc *TrackedConfig) GetSource(path string) ConfigSource {
	if source, ok := tc.Sources[path]; ok {
		return source
	}
	return SourceDefault
}

// GetTrackedSource returns the full source info for a config path.
func (tc *TrackedConfig) GetTrackedSource(path string) TrackedSource {
	if ts, ok := tc.TrackedSources[path]; ok {
		return ts
	}
	return TrackedSource{Source: SourceDefault}
}

