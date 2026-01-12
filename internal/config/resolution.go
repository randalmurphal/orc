package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ResolutionEntry represents one level in the resolution chain for a config key.
type ResolutionEntry struct {
	Level     ConfigLevel  // Level in the hierarchy (default, shared, personal, runtime)
	Source    ConfigSource // Source type (default, shared, personal, env, flag)
	Path      string       // File path or env var name
	Value     string       // Value at this level (empty if not set)
	IsSet     bool         // Whether this level has a value
	IsWinning bool         // Whether this is the winning (effective) value
}

// ResolutionChain shows all resolution levels for a config key.
type ResolutionChain struct {
	Key         string            // Config key (e.g., "model")
	FinalValue  string            // The resolved value
	WinningFrom TrackedSource     // Where the winning value came from
	Entries     []ResolutionEntry // All levels in the chain
}

// GetResolutionChain returns the full resolution chain for a config key.
// This shows values at all levels and which one "wins".
func (l *Loader) GetResolutionChain(key string) (*ResolutionChain, error) {
	chain := &ResolutionChain{
		Key:     key,
		Entries: make([]ResolutionEntry, 0),
	}

	// Get defaults first
	defaultCfg := Default()
	defaultVal, _ := defaultCfg.GetValue(key)
	chain.Entries = append(chain.Entries, ResolutionEntry{
		Level:  LevelDefaults,
		Source: SourceDefault,
		Path:   "builtin",
		Value:  defaultVal,
		IsSet:  true, // Defaults always have a value
	})

	// Get paths for shared level
	sharedPaths := []string{
		filepath.Join(l.projectDir, OrcDir, ConfigFileName),
		filepath.Join(l.projectDir, OrcDir, "shared", ConfigFileName),
	}
	for _, path := range sharedPaths {
		entry := ResolutionEntry{
			Level:  LevelShared,
			Source: SourceShared,
			Path:   path,
		}
		if val, found := getValueFromFile(path, key); found {
			entry.Value = val
			entry.IsSet = true
		}
		chain.Entries = append(chain.Entries, entry)
	}

	// Get paths for personal level
	personalPaths := []string{
		filepath.Join(l.userDir, ConfigFileName),
		filepath.Join(l.projectDir, OrcDir, "local", ConfigFileName),
	}
	for _, path := range personalPaths {
		entry := ResolutionEntry{
			Level:  LevelPersonal,
			Source: SourcePersonal,
			Path:   path,
		}
		if val, found := getValueFromFile(path, key); found {
			entry.Value = val
			entry.IsSet = true
		}
		chain.Entries = append(chain.Entries, entry)
	}

	// Check env var
	if envVar := getEnvVarForPath(key); envVar != "" {
		entry := ResolutionEntry{
			Level:  LevelRuntime,
			Source: SourceEnv,
			Path:   envVar,
		}
		if val := os.Getenv(envVar); val != "" {
			entry.Value = val
			entry.IsSet = true
		}
		chain.Entries = append(chain.Entries, entry)
	}

	// Placeholder for CLI flags - these would be set at runtime
	chain.Entries = append(chain.Entries, ResolutionEntry{
		Level:  LevelRuntime,
		Source: SourceFlag,
		Path:   "--" + key,
		IsSet:  false, // Flags are only known at runtime
	})

	// Determine winner by loading full config
	tc, err := l.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	chain.FinalValue, _ = tc.Config.GetValue(key)
	chain.WinningFrom = tc.GetTrackedSource(key)

	// Mark winning entry
	for i := range chain.Entries {
		e := &chain.Entries[i]
		if e.IsSet && e.Source == chain.WinningFrom.Source {
			if chain.WinningFrom.Path == "" || e.Path == chain.WinningFrom.Path {
				e.IsWinning = true
			}
		}
	}

	return chain, nil
}

// getEnvVarForPath returns the environment variable name for a config path.
func getEnvVarForPath(path string) string {
	for envVar, configPath := range EnvVarMapping {
		if configPath == path {
			return envVar
		}
	}
	return ""
}

// getValueFromFile reads a specific key from a config file.
func getValueFromFile(path, key string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	// Parse the file
	cfg := Default()
	if err := parseConfigYAML(data, cfg); err != nil {
		return "", false
	}

	val, err := cfg.GetValue(key)
	if err != nil {
		return "", false
	}

	// Check if the value differs from default to determine if it was actually set
	defaultCfg := Default()
	defaultVal, _ := defaultCfg.GetValue(key)
	if val != defaultVal {
		return val, true
	}

	// Need to check if the key exists in the raw YAML
	if keyExistsInYAML(data, key) {
		return val, true
	}

	return "", false
}

// parseConfigYAML parses YAML into a Config.
func parseConfigYAML(data []byte, cfg *Config) error {
	return yaml.Unmarshal(data, cfg)
}

// keyExistsInYAML checks if a key exists in YAML data.
func keyExistsInYAML(data []byte, key string) bool {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false
	}

	parts := splitKeyPath(key)
	current := raw

	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return false
		}
		if i == len(parts)-1 {
			return true
		}
		nested, ok := val.(map[string]interface{})
		if !ok {
			return false
		}
		current = nested
	}

	return false
}

// splitKeyPath splits a dot-separated key path.
func splitKeyPath(key string) []string {
	return strings.Split(key, ".")
}
