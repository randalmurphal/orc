package config

// ConfigSource indicates where a configuration value came from.
type ConfigSource string

const (
	// SourceDefault indicates a built-in default value.
	SourceDefault ConfigSource = "default"
	// SourceSystem indicates a system-wide configuration (/etc/orc/config.yaml).
	SourceSystem ConfigSource = "system"
	// SourceUser indicates a user-level configuration (~/.orc/config.yaml).
	SourceUser ConfigSource = "user"
	// SourceProject indicates a project-level configuration (.orc/config.yaml).
	SourceProject ConfigSource = "project"
	// SourceEnv indicates an environment variable override.
	SourceEnv ConfigSource = "env"
)

// TrackedConfig wraps a Config with source tracking.
type TrackedConfig struct {
	// Config is the merged configuration.
	Config *Config

	// Sources maps config paths to their source.
	// Examples: "profile" -> SourceProject, "retry.enabled" -> SourceEnv
	Sources map[string]ConfigSource
}

// NewTrackedConfig creates a new TrackedConfig with defaults.
func NewTrackedConfig() *TrackedConfig {
	return &TrackedConfig{
		Config:  Default(),
		Sources: make(map[string]ConfigSource),
	}
}

// SetSource records the source for a config path.
func (tc *TrackedConfig) SetSource(path string, source ConfigSource) {
	tc.Sources[path] = source
}

// GetSource returns the source for a config path.
// Returns SourceDefault if no source is recorded.
func (tc *TrackedConfig) GetSource(path string) ConfigSource {
	if source, ok := tc.Sources[path]; ok {
		return source
	}
	return SourceDefault
}
